package keeper

import (
	"context"
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
	return (rate*6 + diversity*4) / 10
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

// senseGovernanceParticipation uses domain count as a governance proxy.
// Normalized: count / 100 (target 100 domains = 100% participation).
// Nil-safe: returns NeutralBPS if keeper is nil.
func (k Keeper) senseGovernanceParticipation(ctx context.Context) uint64 {
	if k.ontologyKeeper == nil {
		return types.NeutralBPS
	}
	count := k.ontologyKeeper.GetDomainCount(ctx)
	// Normalize: 100 domains = 100% (1,000,000 BPS).
	const targetDomains = 100
	if count >= targetDomains {
		return types.BPS
	}
	return count * types.BPS / targetDomains
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
