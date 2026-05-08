package keeper

import (
	"context"
	"fmt"
	"math/big"

	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/zerone-chain/zerone/x/alignment/types"
)

// ObserveAll reads all 5 dimension sensors and returns an observation.
func (k Keeper) ObserveAll(ctx context.Context) *types.AlignmentObservation {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	height := uint64(sdkCtx.BlockHeight())

	return &types.AlignmentObservation{
		Height:                  height,
		Timestamp:               sdkCtx.BlockTime().Unix(),
		KnowledgeQuality:        k.senseKnowledgeQuality(ctx),
		EconomicStability:       k.senseEconomicStability(ctx),
		GovernanceParticipation: k.senseGovernanceParticipation(ctx),
		NetworkSecurity:         k.senseNetworkSecurity(ctx),
		StakingRatio:            k.senseStakingRatio(ctx),
	}
}

// senseKnowledgeQuality reads verification rate and consensus diversity from x/knowledge.
// Weighted: 60% verification rate, 40% diversity.
// A system that verifies everything unanimously scores LOWER on knowledge quality.
// Growth pressure (R31-1): if pending/active ratio exceeds 150%, apply 20% penalty (Wood overwhelming Earth).
// Returns BPS. Nil-safe: returns NeutralBPS if keeper is nil.
func (k Keeper) senseKnowledgeQuality(ctx context.Context) uint64 {
	if k.knowledgeKeeper == nil {
		return types.NeutralBPS
	}
	rate := k.knowledgeKeeper.GetVerificationRate(ctx)
	if rate > types.BPS {
		rate = types.BPS
	}
	diversity := k.knowledgeKeeper.GetConsensusDiversity(ctx)
	if diversity > types.BPS {
		diversity = types.BPS
	}
	// Weighted: 60% verification rate, 40% diversity
	qualityScore := (rate*6 + diversity*4) / 10

	// Growth pressure penalty (R31-1: Wood controls Earth)
	pendingRatio := k.knowledgeKeeper.GetPendingVerificationRatio(ctx)
	if pendingRatio > 1_500_000 { // 150% — verification backlog
		qualityScore = qualityScore * 800_000 / types.BPS // 20% penalty
	}

	return qualityScore
}

// senseEconomicStability computes staked/supply ratio as BPS.
// Nil-safe: returns NeutralBPS if either keeper is nil.
func (k Keeper) senseEconomicStability(ctx context.Context) uint64 {
	if k.stakingKeeper == nil || k.vestingRewardsKeeper == nil {
		return types.NeutralBPS
	}
	totalStaked := k.stakingKeeper.GetTotalStaked(ctx)
	totalSupply := k.vestingRewardsKeeper.GetTotalSupply(ctx)
	return ratioBPS(totalStaked, totalSupply)
}

// senseGovernanceParticipation uses domain count and verification health as governance proxies.
// Weighted: 70% domain count, 30% verification health (R31-2: Fire -> Earth).
// Nil-safe: returns NeutralBPS if keepers are nil.
func (k Keeper) senseGovernanceParticipation(ctx context.Context) uint64 {
	if k.ontologyKeeper == nil {
		return types.NeutralBPS
	}

	// Domain count component (70% weight)
	count := k.ontologyKeeper.GetDomainCount(ctx)
	const targetDomains = 100
	domainScore := count * types.BPS / targetDomains
	if domainScore > types.BPS {
		domainScore = types.BPS
	}

	// Verification health component (30% weight) — R31-2: Fire -> Earth
	var verificationHealth uint64
	if k.knowledgeKeeper != nil {
		throughput, disputeRate, _ := k.knowledgeKeeper.GetVerificationHealth(ctx)

		verificationHealth = throughput
		// Extreme dispute rate (>30%) penalises verification health
		if disputeRate > 300_000 {
			verificationHealth = verificationHealth * 700_000 / types.BPS
		}

		sdkCtx := sdk.UnwrapSDKContext(ctx)
		// Commitment 4 (the substrate stress-tests its own truth): the
		// verification subsystem's throughput and dispute-rate are
		// stress signals — extreme dispute rate shows the substrate is
		// already under test, and the chain announces what it sees.
		sdkCtx.EventManager().EmitEvent(sdk.NewEvent(
			"zerone.alignment.verification_health_observed",
			sdk.NewAttribute("throughput_bps", fmt.Sprintf("%d", throughput)),
			sdk.NewAttribute("dispute_rate_bps", fmt.Sprintf("%d", disputeRate)),
			sdk.NewAttribute("creed_commitment", "4"),
		))
	}

	// Blend: 70% domain count + 30% verification health
	score := domainScore*700_000/types.BPS + verificationHealth*300_000/types.BPS

	return score
}

// senseNetworkSecurity computes active/target validator ratio as BPS,
// then applies a capture risk penalty based on flagged domain ratio.
// Nil-safe: returns NeutralBPS if staking keeper is nil.
func (k Keeper) senseNetworkSecurity(ctx context.Context) uint64 {
	if k.stakingKeeper == nil {
		return types.NeutralBPS
	}
	active := k.stakingKeeper.GetActiveValidatorCount(ctx)
	target := k.stakingKeeper.GetTargetValidatorCount(ctx)
	if target == 0 {
		return types.NeutralBPS
	}
	baseSecurity := active * types.BPS / target
	if baseSecurity > types.BPS {
		baseSecurity = types.BPS
	}

	// Apply capture risk penalty (R28-8).
	if k.captureDefenseKeeper != nil {
		flaggedCount := k.captureDefenseKeeper.GetFlaggedDomainCount(ctx)
		if flaggedCount > 0 && k.ontologyKeeper != nil {
			totalDomains := k.ontologyKeeper.GetDomainCount(ctx)
			if totalDomains > 0 {
				captureRatio := flaggedCount * types.BPS / totalDomains
				if captureRatio > types.BPS {
					captureRatio = types.BPS
				}
				// security = baseSecurity * (1 - captureRatio)
				baseSecurity = baseSecurity * (types.BPS - captureRatio) / types.BPS
			}
		}
	}

	return baseSecurity
}

// senseStakingRatio computes staked/supply from the staking angle.
// Same underlying calculation as economic stability but measured independently.
// Nil-safe: returns NeutralBPS if either keeper is nil.
func (k Keeper) senseStakingRatio(ctx context.Context) uint64 {
	if k.stakingKeeper == nil || k.vestingRewardsKeeper == nil {
		return types.NeutralBPS
	}
	totalStaked := k.stakingKeeper.GetTotalStaked(ctx)
	totalSupply := k.vestingRewardsKeeper.GetTotalSupply(ctx)
	return ratioBPS(totalStaked, totalSupply)
}

// EmitGrowthPressureEvent emits a growth_pressure_detected event when verification backlog is high (R31-1).
func (k Keeper) EmitGrowthPressureEvent(ctx sdk.Context, qualityScore uint64) {
	if k.knowledgeKeeper == nil {
		return
	}
	pendingRatio := k.knowledgeKeeper.GetPendingVerificationRatio(ctx)
	if pendingRatio <= 1_500_000 {
		return // no pressure
	}

	// Commitment 4 (the substrate stress-tests its own truth): a high
	// pending-verification ratio means the substrate's testing capacity
	// is under pressure. The chain announces the pressure rather than
	// hiding it behind an apparently-healthy throughput score.
	ctx.EventManager().EmitEvent(sdk.NewEvent(
		"zerone.alignment.growth_pressure_detected",
		sdk.NewAttribute("pending_ratio_bps", fmt.Sprintf("%d", pendingRatio)),
		sdk.NewAttribute("quality_penalty_applied", "true"),
		sdk.NewAttribute("creed_commitment", "4"),
	))
}

// ratioBPS computes (numerator / denominator) * BPS, capped at BPS.
func ratioBPS(numerator, denominator *big.Int) uint64 {
	if denominator == nil || denominator.Sign() == 0 {
		return types.NeutralBPS
	}
	if numerator == nil || numerator.Sign() == 0 {
		return 0
	}
	// (numerator * BPS) / denominator
	num := new(big.Int).Mul(numerator, big.NewInt(int64(types.BPS)))
	result := new(big.Int).Div(num, denominator)
	r := result.Uint64()
	if r > types.BPS {
		return types.BPS
	}
	return r
}
