package keeper

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"math/big"
	"sort"

	sdkmath "cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"google.golang.org/protobuf/proto"

	"github.com/zerone-chain/zerone/x/knowledge/types"
)

// ─── Demand Signal CRUD ─────────────────────────────────────────────────────

// hashSubject returns the SHA-256 hex hash of a subject for use in store keys.
func hashSubject(subject string) string {
	h := sha256.Sum256([]byte(normalizeSubject(subject)))
	return hex.EncodeToString(h[:])
}

// GetDemandSignal retrieves a demand signal by domain and subject.
func (k Keeper) GetDemandSignal(ctx context.Context, domain, subject string) (*types.DemandSignal, bool) {
	store := k.storeService.OpenKVStore(ctx)
	key := types.DemandSignalKey(domain, hashSubject(subject))
	bz, err := store.Get(key)
	if err != nil || bz == nil {
		return nil, false
	}
	var signal types.DemandSignal
	if err := proto.Unmarshal(bz, &signal); err != nil {
		return nil, false
	}
	return &signal, true
}

// GetOrCreateDemandSignal retrieves a demand signal or initializes a new one.
// Returns the signal and true if it already existed, false if newly created.
func (k Keeper) GetOrCreateDemandSignal(ctx context.Context, domain, subject string) (*types.DemandSignal, bool) {
	signal, found := k.GetDemandSignal(ctx, domain, subject)
	if found {
		return signal, true
	}
	return &types.DemandSignal{
		Domain:  domain,
		Subject: subject,
	}, false
}

// SetDemandSignal stores a demand signal.
func (k Keeper) SetDemandSignal(ctx context.Context, signal *types.DemandSignal) error {
	store := k.storeService.OpenKVStore(ctx)
	bz, err := marshalOpts.Marshal(signal)
	if err != nil {
		return fmt.Errorf("failed to marshal demand signal: %w", err)
	}
	return store.Set(types.DemandSignalKey(signal.Domain, hashSubject(signal.Subject)), bz)
}

// IterateDemandSignals iterates all demand signals. Return true from cb to stop.
func (k Keeper) IterateDemandSignals(ctx context.Context, cb func(*types.DemandSignal) bool) {
	store := k.storeService.OpenKVStore(ctx)
	iter, err := store.Iterator(types.DemandSignalPrefix, prefixEndBytes(types.DemandSignalPrefix))
	if err != nil {
		return
	}
	defer iter.Close()
	for ; iter.Valid(); iter.Next() {
		var signal types.DemandSignal
		if err := proto.Unmarshal(iter.Value(), &signal); err != nil {
			continue
		}
		if cb(&signal) {
			break
		}
	}
}

// ResetDemandEpochCounters resets epoch_query_count and epoch_unfulfilled on all demand signals.
// Collects keys before modifying the store to avoid iterator invalidation.
func (k Keeper) ResetDemandEpochCounters(ctx context.Context) {
	var signals []*types.DemandSignal
	k.IterateDemandSignals(ctx, func(signal *types.DemandSignal) bool {
		if signal.EpochQueryCount > 0 || signal.EpochUnfulfilled > 0 {
			signals = append(signals, signal)
		}
		return false
	})
	for _, signal := range signals {
		signal.EpochQueryCount = 0
		signal.EpochUnfulfilled = 0
		if err := k.SetDemandSignal(ctx, signal); err != nil {
			k.Logger(ctx).Error("failed to reset demand epoch counters",
				"domain", signal.Domain, "subject", signal.Subject, "error", err)
		}
	}
}

// IsAuthorizedDemandReporter checks if the given address is in the authorized_demand_reporters param.
func (k Keeper) IsAuthorizedDemandReporter(ctx context.Context, reporter string) bool {
	params, err := k.GetParams(ctx)
	if err != nil {
		return false
	}
	for _, addr := range params.AuthorizedDemandReporters {
		if addr == reporter {
			return true
		}
	}
	return false
}

// ─── Bounty CRUD ─────────────────────────────────────────────────────────────

// GetBounty retrieves a bounty by ID.
func (k Keeper) GetBounty(ctx context.Context, id string) (*types.KnowledgeBounty, bool) {
	store := k.storeService.OpenKVStore(ctx)
	bz, err := store.Get(types.BountyKey(id))
	if err != nil || bz == nil {
		return nil, false
	}
	var bounty types.KnowledgeBounty
	if err := proto.Unmarshal(bz, &bounty); err != nil {
		return nil, false
	}
	return &bounty, true
}

// SetBounty stores a bounty and maintains the domain/subject index.
// If the bounty is claimed, the domain/subject index entry is removed.
func (k Keeper) SetBounty(ctx context.Context, bounty *types.KnowledgeBounty) error {
	store := k.storeService.OpenKVStore(ctx)
	bz, err := marshalOpts.Marshal(bounty)
	if err != nil {
		return fmt.Errorf("failed to marshal bounty: %w", err)
	}
	if err := store.Set(types.BountyKey(bounty.Id), bz); err != nil {
		return err
	}

	// Maintain domain/subject index for active bounty lookup
	indexKey := types.BountyByDomainSubjectKey(bounty.Domain, hashSubject(bounty.Subject))
	if bounty.Claimed {
		// Remove from active index when claimed
		_ = store.Delete(indexKey)
	} else {
		_ = store.Set(indexKey, []byte(bounty.Id))
	}
	return nil
}

// HasActiveBounty checks if an unclaimed bounty exists for the given domain/subject.
func (k Keeper) HasActiveBounty(ctx context.Context, domain, subject string) bool {
	store := k.storeService.OpenKVStore(ctx)
	indexKey := types.BountyByDomainSubjectKey(domain, hashSubject(subject))
	bz, err := store.Get(indexKey)
	if err != nil || bz == nil {
		return false
	}
	// Verify the bounty still exists and is not claimed
	bounty, found := k.GetBounty(ctx, string(bz))
	if !found || bounty.Claimed {
		return false
	}
	return true
}

// FindMatchingBounty finds an active (unclaimed, unexpired) bounty for the given domain/subject.
func (k Keeper) FindMatchingBounty(ctx context.Context, domain, subject string) (*types.KnowledgeBounty, bool) {
	store := k.storeService.OpenKVStore(ctx)
	indexKey := types.BountyByDomainSubjectKey(domain, hashSubject(subject))
	bz, err := store.Get(indexKey)
	if err != nil || bz == nil {
		return nil, false
	}
	bounty, found := k.GetBounty(ctx, string(bz))
	if !found || bounty.Claimed {
		return nil, false
	}
	// Check expiry
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	if bounty.ExpiresAtBlock > 0 && uint64(sdkCtx.BlockHeight()) > bounty.ExpiresAtBlock {
		return nil, false
	}
	return bounty, true
}

// IterateBounties iterates all bounties. Return true from cb to stop.
func (k Keeper) IterateBounties(ctx context.Context, cb func(*types.KnowledgeBounty) bool) {
	store := k.storeService.OpenKVStore(ctx)
	iter, err := store.Iterator(types.BountyPrefix, prefixEndBytes(types.BountyPrefix))
	if err != nil {
		return
	}
	defer iter.Close()
	for ; iter.Valid(); iter.Next() {
		var bounty types.KnowledgeBounty
		if err := proto.Unmarshal(iter.Value(), &bounty); err != nil {
			continue
		}
		if cb(&bounty) {
			break
		}
	}
}

// GenerateBountyID generates a deterministic hex bounty ID from domain, subject, and epoch.
func GenerateBountyID(domain, subject string, epoch uint64) string {
	h := sha256.New()
	h.Write([]byte("ZRN.bounty.id.v1:"))
	h.Write([]byte(domain))
	h.Write([]byte(":"))
	h.Write([]byte(subject))
	h.Write([]byte(fmt.Sprintf(":%d", epoch)))
	return hex.EncodeToString(h.Sum(nil))[:32]
}

// ─── Bounty Processing ──────────────────────────────────────────────────────

// ProcessDemandBounties creates bounties for demand signals that exceed the threshold.
// Called at epoch boundaries. Funds bounties from protocol_treasury → knowledge module.
// Resets epoch counters after processing.
func (k Keeper) ProcessDemandBounties(ctx context.Context, epoch uint64) error {
	params, err := k.GetParams(ctx)
	if err != nil {
		return err
	}

	// Skip if demand tracking is disabled
	if !params.DemandTrackingEnabled {
		return nil
	}

	sdkCtx := sdk.UnwrapSDKContext(ctx)
	height := uint64(sdkCtx.BlockHeight())

	// Calculate expiry block from epoch length
	epochBlocks := params.BootstrapFundEpochBlocks
	if epochBlocks == 0 {
		epochBlocks = 1000 // safe default
	}
	expiresAtBlock := height + params.DemandBountyExpiryEpochs*epochBlocks

	// Parse reward params
	baseReward := new(big.Int)
	if _, ok := baseReward.SetString(params.DemandBountyBaseReward, 10); !ok {
		baseReward.SetUint64(10_000_000) // fallback 10 ZRN
	}
	perQueryBonus := new(big.Int)
	if _, ok := perQueryBonus.SetString(params.DemandBountyPerQueryBonus, 10); !ok {
		perQueryBonus.SetUint64(100_000) // fallback 0.1 ZRN
	}

	// Collect signals exceeding threshold (avoid modifying store during iteration)
	var qualifying []*types.DemandSignal
	k.IterateDemandSignals(ctx, func(signal *types.DemandSignal) bool {
		if signal.EpochUnfulfilled >= params.DemandBountyThreshold {
			qualifying = append(qualifying, signal)
		}
		return false
	})

	bountiesCreated := 0
	for _, signal := range qualifying {
		// Skip if an active bounty already exists for this domain/subject
		if k.HasActiveBounty(ctx, signal.Domain, signal.Subject) {
			continue
		}

		// Calculate reward: base + (unfulfilled * per_query_bonus)
		reward := new(big.Int).Set(baseReward)
		bonus := new(big.Int).Mul(
			new(big.Int).SetUint64(signal.EpochUnfulfilled),
			perQueryBonus,
		)
		reward.Add(reward, bonus)

		// Fund bounty: protocol_treasury → knowledge module
		coins := sdk.NewCoins(sdk.NewCoin("uzrn", sdkmath.NewIntFromBigInt(reward)))
		if err := k.bankKeeper.SendCoinsFromModuleToModule(ctx, protocolTreasuryModule, types.ModuleName, coins); err != nil {
			k.Logger(ctx).Error("failed to fund bounty from treasury",
				"domain", signal.Domain, "subject", signal.Subject, "error", err)
			continue
		}

		bountyID := GenerateBountyID(signal.Domain, signal.Subject, epoch)
		bounty := &types.KnowledgeBounty{
			Id:             bountyID,
			Domain:         signal.Domain,
			Subject:        signal.Subject,
			RewardAmount:   reward.String(),
			CreatedAtBlock: height,
			ExpiresAtBlock: expiresAtBlock,
			Claimed:        false,
			DemandCount:    signal.EpochUnfulfilled,
		}
		if err := k.SetBounty(ctx, bounty); err != nil {
			k.Logger(ctx).Error("failed to store bounty",
				"bounty_id", bountyID, "error", err)
			continue
		}

		sdkCtx.EventManager().EmitEvent(sdk.NewEvent(
			"zerone.knowledge.bounty_created",
			sdk.NewAttribute("bounty_id", bountyID),
			sdk.NewAttribute("domain", signal.Domain),
			sdk.NewAttribute("subject", signal.Subject),
			sdk.NewAttribute("reward_amount", reward.String()),
			sdk.NewAttribute("demand_count", fmt.Sprintf("%d", signal.EpochUnfulfilled)),
		))
		bountiesCreated++
	}

	// Reset epoch counters after processing
	k.ResetDemandEpochCounters(ctx)

	if bountiesCreated > 0 {
		k.Logger(ctx).Info("demand bounties created",
			"count", bountiesCreated,
			"epoch", epoch,
		)
	}

	return nil
}

// ProcessExpiredBounties marks expired bounties and returns funds to protocol_treasury.
func (k Keeper) ProcessExpiredBounties(ctx context.Context) {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	height := uint64(sdkCtx.BlockHeight())

	// Collect expired bounties (avoid modifying store during iteration)
	var expired []*types.KnowledgeBounty
	k.IterateBounties(ctx, func(bounty *types.KnowledgeBounty) bool {
		if !bounty.Claimed && bounty.ExpiresAtBlock > 0 && height > bounty.ExpiresAtBlock {
			expired = append(expired, bounty)
		}
		return false
	})

	for _, bounty := range expired {
		// Return funds: knowledge module → protocol_treasury
		reward := new(big.Int)
		if _, ok := reward.SetString(bounty.RewardAmount, 10); !ok || reward.Sign() <= 0 {
			// Invalid reward amount — just mark claimed to clean up
			bounty.Claimed = true
			_ = k.SetBounty(ctx, bounty)
			continue
		}

		coins := sdk.NewCoins(sdk.NewCoin("uzrn", sdkmath.NewIntFromBigInt(reward)))
		if err := k.bankKeeper.SendCoinsFromModuleToModule(ctx, types.ModuleName, protocolTreasuryModule, coins); err != nil {
			k.Logger(ctx).Error("failed to return expired bounty funds",
				"bounty_id", bounty.Id, "error", err)
			continue
		}

		bounty.Claimed = true // Mark as claimed to remove from active index
		if err := k.SetBounty(ctx, bounty); err != nil {
			k.Logger(ctx).Error("failed to update expired bounty",
				"bounty_id", bounty.Id, "error", err)
			continue
		}

		sdkCtx.EventManager().EmitEvent(sdk.NewEvent(
			"zerone.knowledge.bounty_expired",
			sdk.NewAttribute("bounty_id", bounty.Id),
			sdk.NewAttribute("domain", bounty.Domain),
			sdk.NewAttribute("subject", bounty.Subject),
			sdk.NewAttribute("returned_amount", bounty.RewardAmount),
		))
	}

	if len(expired) > 0 {
		k.Logger(ctx).Info("expired bounties processed", "count", len(expired))
	}
}

// ClaimBountyForFact checks if a newly created fact matches a bounty and claims it.
// If fact.Structure.Subject matches a bounty, the bounty is claimed and reward is paid to the submitter.
func (k Keeper) ClaimBountyForFact(ctx context.Context, fact *types.Fact, claim *types.Claim) {
	if fact.Structure == nil || fact.Structure.Subject == "" {
		return
	}

	bounty, found := k.FindMatchingBounty(ctx, fact.Domain, fact.Structure.Subject)
	if !found {
		return
	}

	// Parse reward amount
	reward := new(big.Int)
	if _, ok := reward.SetString(bounty.RewardAmount, 10); !ok || reward.Sign() <= 0 {
		return
	}

	// Pay submitter: knowledge module → submitter account
	submitterAddr, err := sdk.AccAddressFromBech32(claim.Submitter)
	if err != nil {
		k.Logger(ctx).Error("invalid submitter address for bounty claim",
			"submitter", claim.Submitter, "error", err)
		return
	}

	coins := sdk.NewCoins(sdk.NewCoin("uzrn", sdkmath.NewIntFromBigInt(reward)))
	if err := k.bankKeeper.SendCoinsFromModuleToAccount(ctx, types.ModuleName, submitterAddr, coins); err != nil {
		k.Logger(ctx).Error("failed to pay bounty to submitter",
			"bounty_id", bounty.Id, "submitter", claim.Submitter, "error", err)
		return
	}

	// Mark bounty as claimed
	bounty.Claimed = true
	bounty.ClaimedByFactId = fact.Id
	if err := k.SetBounty(ctx, bounty); err != nil {
		k.Logger(ctx).Error("failed to update claimed bounty",
			"bounty_id", bounty.Id, "error", err)
		return
	}

	sdkCtx := sdk.UnwrapSDKContext(ctx)
	sdkCtx.EventManager().EmitEvent(sdk.NewEvent(
		"zerone.knowledge.bounty_claimed",
		sdk.NewAttribute("bounty_id", bounty.Id),
		sdk.NewAttribute("fact_id", fact.Id),
		sdk.NewAttribute("submitter", claim.Submitter),
		sdk.NewAttribute("reward_amount", bounty.RewardAmount),
		sdk.NewAttribute("domain", bounty.Domain),
		sdk.NewAttribute("subject", bounty.Subject),
	))

	k.Logger(ctx).Info("bounty claimed",
		"bounty_id", bounty.Id,
		"fact_id", fact.Id,
		"submitter", claim.Submitter,
		"reward", bounty.RewardAmount,
	)
}

// ─── Demand-Weighted Energy ─────────────────────────────────────────────────

// GetDemandMultiplier returns the demand-weighted energy multiplier in BPS.
// Base 1× (1,000,000) + 0.1× (100,000) per 10 epoch queries (linear).
// Capped by DemandMultiplierCap param. Returns 1,000,000 if no data.
func (k Keeper) GetDemandMultiplier(ctx context.Context, domain, subject string) uint64 {
	const baseBps = 1_000_000       // 1×
	const bonusPerTen = 100_000     // 0.1× per 10 queries
	const queriesPerStep uint64 = 10

	signal, found := k.GetDemandSignal(ctx, domain, subject)
	if !found {
		return baseBps
	}

	// Linear: base + (epoch_queries / 10) * 100,000 BPS
	steps := signal.EpochQueryCount / queriesPerStep
	multiplier := baseBps + steps*bonusPerTen

	// Apply cap
	params, err := k.GetParams(ctx)
	if err != nil {
		return baseBps
	}
	cap := params.DemandMultiplierCap
	if cap == 0 {
		cap = 10_000_000 // fallback: 10×
	}
	if multiplier > cap {
		multiplier = cap
	}

	return multiplier
}

// ─── Query Helpers ──────────────────────────────────────────────────────────

// GetActiveBounties returns all active (unclaimed, unexpired) bounties for a domain.
func (k Keeper) GetActiveBounties(ctx context.Context, domain string) []*types.KnowledgeBounty {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	height := uint64(sdkCtx.BlockHeight())

	var active []*types.KnowledgeBounty
	k.IterateBounties(ctx, func(bounty *types.KnowledgeBounty) bool {
		if bounty.Domain == domain && !bounty.Claimed {
			if bounty.ExpiresAtBlock == 0 || height <= bounty.ExpiresAtBlock {
				active = append(active, bounty)
			}
		}
		return false
	})
	return active
}

// GetTopDemandGaps returns demand signals sorted by unfulfilled_count descending,
// limited to the specified number.
func (k Keeper) GetTopDemandGaps(ctx context.Context, limit uint64) []*types.DemandSignal {
	var all []*types.DemandSignal
	k.IterateDemandSignals(ctx, func(signal *types.DemandSignal) bool {
		if signal.UnfulfilledCount > 0 {
			all = append(all, signal)
		}
		return false
	})

	// Sort by unfulfilled_count descending
	sort.Slice(all, func(i, j int) bool {
		return all[i].UnfulfilledCount > all[j].UnfulfilledCount
	})

	if uint64(len(all)) > limit {
		all = all[:limit]
	}
	return all
}
