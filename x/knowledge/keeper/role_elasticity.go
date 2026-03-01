package keeper

import (
	"context"
	"encoding/json"
	"fmt"

	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/zerone-chain/zerone/x/knowledge/types"
)

// ─── CRUD ────────────────────────────────────────────────────────────────────

func (k Keeper) SetDomainRoleRecord(ctx context.Context, record *types.DomainRoleRecord) error {
	store := k.storeService.OpenKVStore(ctx)
	bz, err := json.Marshal(record)
	if err != nil {
		return fmt.Errorf("failed to marshal DomainRoleRecord: %w", err)
	}
	return store.Set(types.DomainRoleRecordKey(record.Domain), bz)
}

func (k Keeper) GetDomainRoleRecord(ctx context.Context, domain string) (*types.DomainRoleRecord, bool) {
	store := k.storeService.OpenKVStore(ctx)
	bz, err := store.Get(types.DomainRoleRecordKey(domain))
	if err != nil || bz == nil {
		return nil, false
	}
	var record types.DomainRoleRecord
	if err := json.Unmarshal(bz, &record); err != nil {
		return nil, false
	}
	return &record, true
}

func (k Keeper) IterateDomainRoleRecords(ctx context.Context, cb func(record *types.DomainRoleRecord) bool) {
	store := k.storeService.OpenKVStore(ctx)
	iter, err := store.Iterator(types.DomainRoleRecordPrefix, prefixEndBytes(types.DomainRoleRecordPrefix))
	if err != nil {
		return
	}
	defer iter.Close()

	for ; iter.Valid(); iter.Next() {
		var record types.DomainRoleRecord
		if err := json.Unmarshal(iter.Value(), &record); err != nil {
			continue
		}
		if cb(&record) {
			break
		}
	}
}

// ─── Elasticity Calculation ──────────────────────────────────────────────────

// roleElasticityParams fills in defaults for role elasticity params that may
// be zero due to proto raw descriptor not yet including fields 125-128.
func roleElasticityParams(params *types.Params) (minCalls, maxMul, minMul uint64) {
	defaults := types.DefaultParams()
	minCalls = params.RoleElasticityMinCalls
	if minCalls == 0 {
		minCalls = defaults.RoleElasticityMinCalls
	}
	maxMul = params.RoleElasticityMaxMultiplierBps
	if maxMul == 0 {
		maxMul = defaults.RoleElasticityMaxMultiplierBps
	}
	minMul = params.RoleElasticityMinMultiplierBps
	if minMul == 0 {
		minMul = defaults.RoleElasticityMinMultiplierBps
	}
	return
}

func clampUint64(val, minVal, maxVal uint64) uint64 {
	if val < minVal {
		return minVal
	}
	if val > maxVal {
		return maxVal
	}
	return val
}

func (k Keeper) GetRoleElasticity(ctx context.Context, domain string) (agentBonusBps, humanBonusBps uint64) {
	params, err := k.GetParams(ctx)
	if err != nil {
		return 0, 0
	}

	agentBase := params.AgentVerificationBonusBps
	humanBase := params.HumanPatronageBonusBps

	record, found := k.GetDomainRoleRecord(ctx, domain)
	if !found {
		return agentBase, humanBase
	}

	minCalls, maxMul, minMul := roleElasticityParams(params)

	agentTotal := record.AgentCorrectCalls + record.AgentIncorrectCalls
	humanTotal := record.HumanCorrectCalls + record.HumanIncorrectCalls

	if agentTotal < minCalls || humanTotal < minCalls {
		return agentBase, humanBase
	}

	agentAccuracy := safeMulDiv(record.AgentCorrectCalls, BPS, agentTotal)
	humanAccuracy := safeMulDiv(record.HumanCorrectCalls, BPS, humanTotal)

	total := agentAccuracy + humanAccuracy
	if total == 0 {
		return agentBase, humanBase
	}

	agentMultiplier := clampUint64(
		safeMulDiv(agentAccuracy*2, BPS, total),
		minMul,
		maxMul,
	)
	humanMultiplier := clampUint64(
		safeMulDiv(humanAccuracy*2, BPS, total),
		minMul,
		maxMul,
	)

	return safeMulDiv(agentBase, agentMultiplier, BPS), safeMulDiv(humanBase, humanMultiplier, BPS)
}

func (k Keeper) GetRoleAccuracies(ctx context.Context, domain string) (agentAccBps, humanAccBps uint64) {
	record, found := k.GetDomainRoleRecord(ctx, domain)
	if !found {
		return 0, 0
	}

	agentTotal := record.AgentCorrectCalls + record.AgentIncorrectCalls
	humanTotal := record.HumanCorrectCalls + record.HumanIncorrectCalls

	if agentTotal > 0 {
		agentAccBps = safeMulDiv(record.AgentCorrectCalls, BPS, agentTotal)
	}
	if humanTotal > 0 {
		humanAccBps = safeMulDiv(record.HumanCorrectCalls, BPS, humanTotal)
	}
	return
}

// ─── Vote Counting ───────────────────────────────────────────────────────────

func (k Keeper) CountVotesByAccountType(ctx context.Context, round *types.VerificationRound) (agentVotes, humanVotes uint64) {
	for _, reveal := range round.Reveals {
		accountType := k.getAccountType(ctx, reveal.Verifier)
		switch accountType {
		case "agent":
			agentVotes++
		case "human":
			humanVotes++
		}
	}
	return
}

// ─── Track Record Updates ────────────────────────────────────────────────────

func (k Keeper) RecordVindicationRoleImpact(ctx context.Context, round *types.VerificationRound, domain string) {
	if domain == "" {
		return
	}
	agentVotes, humanVotes := k.CountVotesByAccountType(ctx, round)
	if agentVotes == humanVotes {
		return
	}

	record, found := k.GetDomainRoleRecord(ctx, domain)
	if !found {
		record = &types.DomainRoleRecord{Domain: domain}
	}

	if agentVotes > humanVotes {
		record.AgentIncorrectCalls++
	} else {
		record.HumanIncorrectCalls++
	}

	sdkCtx := sdk.UnwrapSDKContext(ctx)
	record.LastUpdated = uint64(sdkCtx.BlockHeight())
	_ = k.SetDomainRoleRecord(ctx, record)

	k.emitRoleElasticityEvent(ctx, domain)
}

func (k Keeper) RecordChallengeRoleImpact(ctx context.Context, factId, domain string, upheld bool) error {
	round := k.GetVerificationRoundForFact(ctx, factId)
	if round == nil {
		return nil
	}

	agentVotes, humanVotes := k.CountVotesByAccountType(ctx, round)
	if agentVotes == humanVotes {
		return nil
	}

	record, found := k.GetDomainRoleRecord(ctx, domain)
	if !found {
		record = &types.DomainRoleRecord{Domain: domain}
	}

	if upheld {
		if agentVotes > humanVotes {
			record.AgentIncorrectCalls++
		} else {
			record.HumanIncorrectCalls++
		}
	} else {
		if agentVotes > humanVotes {
			record.AgentCorrectCalls++
		} else {
			record.HumanCorrectCalls++
		}
	}

	sdkCtx := sdk.UnwrapSDKContext(ctx)
	record.LastUpdated = uint64(sdkCtx.BlockHeight())
	_ = k.SetDomainRoleRecord(ctx, record)

	k.emitRoleElasticityEvent(ctx, domain)
	return nil
}

func (k Keeper) GetVerificationRoundForFact(ctx context.Context, factId string) *types.VerificationRound {
	fact, found := k.GetFact(ctx, factId)
	if !found || fact.ClaimId == "" {
		return nil
	}
	claim, found := k.GetClaim(ctx, fact.ClaimId)
	if !found || claim.VerificationRoundId == "" {
		return nil
	}
	round, found := k.GetVerificationRound(ctx, claim.VerificationRoundId)
	if !found {
		return nil
	}
	return round
}

// ─── Helpers ─────────────────────────────────────────────────────────────────

func (k Keeper) getDomainForFact(ctx context.Context, factId string) string {
	fact, found := k.GetFact(ctx, factId)
	if !found {
		return ""
	}
	return fact.Domain
}

func (k Keeper) getDomainForRound(ctx context.Context, round *types.VerificationRound) string {
	if round == nil || round.ClaimId == "" {
		return ""
	}
	claim, found := k.GetClaim(ctx, round.ClaimId)
	if !found {
		return ""
	}
	return claim.Domain
}

// ─── Decay ───────────────────────────────────────────────────────────────────

func (k Keeper) DecayRoleRecords(ctx context.Context) {
	var records []*types.DomainRoleRecord
	k.IterateDomainRoleRecords(ctx, func(record *types.DomainRoleRecord) bool {
		records = append(records, record)
		return false
	})

	for _, record := range records {
		record.AgentCorrectCalls = safeMulDiv(record.AgentCorrectCalls, 950_000, BPS)
		record.AgentIncorrectCalls = safeMulDiv(record.AgentIncorrectCalls, 950_000, BPS)
		record.HumanCorrectCalls = safeMulDiv(record.HumanCorrectCalls, 950_000, BPS)
		record.HumanIncorrectCalls = safeMulDiv(record.HumanIncorrectCalls, 950_000, BPS)
		_ = k.SetDomainRoleRecord(ctx, record)
	}
}

// ─── Events ──────────────────────────────────────────────────────────────────

func (k Keeper) emitRoleElasticityEvent(ctx context.Context, domain string) {
	agentBonus, humanBonus := k.GetRoleElasticity(ctx, domain)
	agentAcc, humanAcc := k.GetRoleAccuracies(ctx, domain)

	sdkCtx := sdk.UnwrapSDKContext(ctx)
	sdkCtx.EventManager().EmitEvent(sdk.NewEvent(
		"zerone.knowledge.role_elasticity_updated",
		sdk.NewAttribute("domain", domain),
		sdk.NewAttribute("agent_bonus_bps", fmt.Sprintf("%d", agentBonus)),
		sdk.NewAttribute("human_bonus_bps", fmt.Sprintf("%d", humanBonus)),
		sdk.NewAttribute("agent_accuracy_bps", fmt.Sprintf("%d", agentAcc)),
		sdk.NewAttribute("human_accuracy_bps", fmt.Sprintf("%d", humanAcc)),
	))
}
