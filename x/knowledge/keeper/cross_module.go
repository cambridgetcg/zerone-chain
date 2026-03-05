package keeper

import (
	"context"
	"encoding/hex"
	"fmt"
	"strings"

	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/zerone-chain/zerone/x/knowledge/types"
	toolboxtypes "github.com/zerone-chain/zerone/x/toolbox/types"
)

// Compile-time interface compliance check.
var _ toolboxtypes.KnowledgeKeeper = (*ToolboxKnowledgeAdapter)(nil)

// ─── Tree integration: Data collection campaigns ────────────────────────────

// CreateProjectBounty creates a DataBounty linked to a tree project.
func (k Keeper) CreateProjectBounty(ctx context.Context, domain string, targetCount, minQuality uint64, budget sdk.Coins, projectID string) error {
	sdkCtx := sdk.UnwrapSDKContext(ctx)

	// Generate bounty ID
	seq, err := k.nextBountySeq(ctx)
	if err != nil {
		return err
	}
	bountyID := hex.EncodeToString([]byte(fmt.Sprintf("proj-%s-%d", projectID, seq)))

	var rewardAmount string
	if budget.IsZero() {
		rewardAmount = "0"
	} else {
		rewardAmount = budget.AmountOf("uzrn").String()
	}

	bounty := &types.DataBounty{
		Id:             bountyID,
		Domain:         domain,
		Subject:        fmt.Sprintf("project:%s:target:%d:minq:%d", projectID, targetCount, minQuality),
		RewardAmount:   rewardAmount,
		CreatedAtBlock: uint64(sdkCtx.BlockHeight()),
		Claimed:        false,
		DemandCount:    targetCount,
	}

	return k.SetDataBounty(ctx, bounty)
}

// GetBountyProgress returns the number of accepted samples vs target for a project bounty.
func (k Keeper) GetBountyProgress(ctx context.Context, projectID string) (current uint64, target uint64, found bool) {
	prefix := fmt.Sprintf("project:%s:", projectID)
	var bounty *types.DataBounty

	k.IterateDataBounties(ctx, func(b *types.DataBounty) bool {
		if strings.HasPrefix(b.Subject, prefix) {
			bounty = b
			return true // stop
		}
		return false
	})

	if bounty == nil {
		return 0, 0, false
	}

	// Count accepted samples in the bounty's domain
	var accepted uint64
	ids := k.GetSamplesByDomain(ctx, bounty.Domain)
	for _, id := range ids {
		s, ok := k.GetSample(ctx, id)
		if ok && s.Status != types.SampleStatus_SAMPLE_STATUS_PENDING &&
			s.Status != types.SampleStatus_SAMPLE_STATUS_REJECTED &&
			s.Status != types.SampleStatus_SAMPLE_STATUS_PRUNED &&
			s.Status != types.SampleStatus_SAMPLE_STATUS_EXPIRED {
			accepted++
		}
	}

	return accepted, bounty.DemandCount, true
}

// nextBountySeq returns an auto-incrementing sequence for bounty IDs.
func (k Keeper) nextBountySeq(ctx context.Context) (uint64, error) {
	store := k.storeService.OpenKVStore(ctx)
	key := []byte("bounty_seq")
	bz, err := store.Get(key)
	if err != nil {
		return 0, err
	}
	var seq uint64
	if bz != nil && len(bz) >= 8 {
		seq = uint64(bz[0])<<56 | uint64(bz[1])<<48 | uint64(bz[2])<<40 | uint64(bz[3])<<32 |
			uint64(bz[4])<<24 | uint64(bz[5])<<16 | uint64(bz[6])<<8 | uint64(bz[7])
	}
	seq++
	buf := make([]byte, 8)
	buf[0] = byte(seq >> 56)
	buf[1] = byte(seq >> 48)
	buf[2] = byte(seq >> 40)
	buf[3] = byte(seq >> 32)
	buf[4] = byte(seq >> 24)
	buf[5] = byte(seq >> 16)
	buf[6] = byte(seq >> 8)
	buf[7] = byte(seq)
	return seq, store.Set(key, buf)
}

// ─── Vesting integration: Protocol revenue routing ──────────────────────────

// SendProtocolRevenue routes the protocol's share of access revenue to vesting_rewards.
func (k Keeper) SendProtocolRevenue(ctx context.Context, amount sdk.Coins) error {
	if k.vestingRewardsKeeper == nil {
		return nil // no-op if not wired
	}
	return k.vestingRewardsKeeper.DepositToResearchFund(ctx, types.ModuleName, amount)
}

// ─── Toolbox integration: Knowledge adapter ─────────────────────────────────

// ToolboxKnowledgeAdapter adapts the knowledge Keeper to satisfy
// the toolbox module's KnowledgeKeeper interface.
type ToolboxKnowledgeAdapter struct {
	k Keeper
}

// NewToolboxKnowledgeAdapter returns an adapter for the toolbox module.
func NewToolboxKnowledgeAdapter(k Keeper) *ToolboxKnowledgeAdapter {
	return &ToolboxKnowledgeAdapter{k: k}
}

// GetFactConfidence returns a sample's quality score as confidence.
func (a *ToolboxKnowledgeAdapter) GetFactConfidence(ctx context.Context, factID string) (uint64, bool) {
	sample, found := a.k.GetSample(ctx, factID)
	if !found {
		return 0, false
	}
	return sample.QualityScore, true
}

// SearchFactsByContent searches samples by domain and content terms.
func (a *ToolboxKnowledgeAdapter) SearchFactsByContent(ctx context.Context, domain string, terms []string, maxResults uint64) ([]string, error) {
	ids := a.k.GetSamplesByDomain(ctx, domain)
	var results []string

	for _, id := range ids {
		if uint64(len(results)) >= maxResults {
			break
		}
		s, found := a.k.GetSample(ctx, id)
		if !found || s.Content == "" || s.Content == "[consent revoked]" {
			continue
		}
		// Simple term matching
		lower := strings.ToLower(s.Content)
		matched := true
		for _, term := range terms {
			if !strings.Contains(lower, strings.ToLower(term)) {
				matched = false
				break
			}
		}
		if matched {
			results = append(results, id)
		}
	}
	return results, nil
}

// GetFactDetails returns content, confidence, and access count for a sample.
func (a *ToolboxKnowledgeAdapter) GetFactDetails(ctx context.Context, factID string) (content string, confidence uint64, citations uint64, err error) {
	sample, found := a.k.GetSample(ctx, factID)
	if !found {
		return "", 0, 0, fmt.Errorf("sample %q not found", factID)
	}
	return sample.Content, sample.QualityScore, sample.AccessCount, nil
}

// RecordFactCitation records a citation/access of a sample by a tool.
func (a *ToolboxKnowledgeAdapter) RecordFactCitation(ctx context.Context, factID string, _ string) error {
	sample, found := a.k.GetSample(ctx, factID)
	if !found {
		return fmt.Errorf("sample %q not found", factID)
	}
	sample.AccessCount++
	return a.k.SetSample(ctx, sample)
}
