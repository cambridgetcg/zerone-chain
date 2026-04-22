package keeper

import (
	"math/big"

	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/zerone-chain/zerone/x/staking/types"
)

// ComputeValidatorTier evaluates the highest tier a validator qualifies for.
// Evaluates from highest (Guardian) to lowest (Apprentice).
func ComputeValidatorTier(
	ctx sdk.Context,
	k Keeper,
	stake *big.Int,
	verifications, correctVerifications uint64,
	slashCount, contestedCorrect, lastSlashHeight uint64,
) types.ValidatorTier {
	params := k.GetParams(ctx)
	configs := params.TierConfigs
	if len(configs) != 4 {
		return types.TierApprentice
	}

	currentHeight := uint64(ctx.BlockHeight())
	epochLength := params.SlashDecayPeriodBlocks
	if epochLength == 0 {
		epochLength = 34_272
	}

	var accuracy uint64
	if verifications > 0 {
		accuracy = (correctVerifications * types.BPSScale) / verifications
	}

	// Guardian check (configs[3])
	guardianCfg := configs[3]
	guardianMinStake, _ := new(big.Int).SetString(guardianCfg.MinStake, 10)
	if guardianMinStake != nil && stake.Cmp(guardianMinStake) >= 0 {
		windowedSlashes := getWindowedSlashCount(slashCount, lastSlashHeight, currentHeight, epochLength, guardianCfg.SlashWindowEpochs)
		effectiveVer := getEffectiveVerifications(verifications, contestedCorrect, guardianCfg.ContestedVerificationMultiplier)
		if effectiveVer >= guardianCfg.MinVerifications &&
			accuracy >= guardianCfg.MinAccuracy &&
			windowedSlashes <= 0 &&
			contestedCorrect >= guardianCfg.MinContestedVerifications {
			return types.TierGuardian
		}
	}

	// Scholar check (configs[2])
	scholarCfg := configs[2]
	scholarMinStake, _ := new(big.Int).SetString(scholarCfg.MinStake, 10)
	if scholarMinStake != nil && stake.Cmp(scholarMinStake) >= 0 &&
		verifications >= scholarCfg.MinVerifications &&
		accuracy >= scholarCfg.MinAccuracy {
		return types.TierScholar
	}

	// Verified check (configs[1])
	verifiedCfg := configs[1]
	verifiedMinStake, _ := new(big.Int).SetString(verifiedCfg.MinStake, 10)
	if verifiedMinStake != nil && stake.Cmp(verifiedMinStake) >= 0 &&
		verifications >= verifiedCfg.MinVerifications &&
		accuracy >= verifiedCfg.MinAccuracy {
		return types.TierVerified
	}

	return types.TierApprentice
}

// getWindowedSlashCount returns the effective slash count within the window.
func getWindowedSlashCount(slashCount, lastSlashHeight, currentHeight, epochLength, slashWindowEpochs uint64) uint64 {
	if slashWindowEpochs == 0 || lastSlashHeight == 0 {
		return slashCount
	}
	windowBlocks := epochLength * slashWindowEpochs
	if currentHeight > lastSlashHeight && (currentHeight-lastSlashHeight) > windowBlocks {
		return 0
	}
	return slashCount
}

// getEffectiveVerifications computes effective verifications with contested multiplier.
func getEffectiveVerifications(totalVerifications, contestedCorrect, multiplier uint64) uint64 {
	if multiplier <= 1 {
		return totalVerifications
	}
	nonContested := totalVerifications - contestedCorrect
	return nonContested + contestedCorrect*multiplier
}

// CheckTierTransition checks if a validator should change tiers.
func (k Keeper) CheckTierTransition(ctx sdk.Context, val *types.Validator) (types.ValidatorTier, bool) {
	stake, ok := new(big.Int).SetString(val.TotalStake, 10)
	if !ok {
		stake = new(big.Int)
	}
	newTier := ComputeValidatorTier(
		ctx, k,
		stake,
		val.TotalVerifications,
		val.CorrectVerifications,
		val.SlashCount,
		val.ContestedVerificationsCorrect,
		val.LastSlashHeight,
	)
	if newTier != val.Tier {
		return newTier, true
	}
	return val.Tier, false
}

// ApplyTierTransition mutates val.Tier and emits a unified tier-transition
// event (L3). All call sites that previously silently mutated val.Tier should
// route through this helper so every tier change is observable.
//
// `trigger` is a free-form string naming the message or condition that caused
// the transition (e.g. "stake_delegate", "reward_recorded", "slash"). It flows
// into the event's `trigger` attribute.
func (k Keeper) ApplyTierTransition(ctx sdk.Context, val *types.Validator, newTier types.ValidatorTier, trigger string) {
	if val == nil || val.Tier == newTier {
		return
	}
	oldTier := val.Tier
	val.Tier = newTier

	direction := "promotion"
	if newTier < oldTier {
		direction = "demotion"
	}

	ctx.EventManager().EmitEvent(sdk.NewEvent(
		"zerone.staking.tier_transitioned",
		sdk.NewAttribute("validator", val.OperatorAddress),
		sdk.NewAttribute("from_tier", types.ValidatorTierString(oldTier)),
		sdk.NewAttribute("to_tier", types.ValidatorTierString(newTier)),
		sdk.NewAttribute("direction", direction),
		sdk.NewAttribute("trigger", trigger),
	))
}

// IsTierEligibleForCategory checks if a tier allows a specific verification category.
func (k Keeper) IsTierEligibleForCategory(ctx sdk.Context, tier types.ValidatorTier, category string) bool {
	tc, found := k.GetTierConfig(ctx, tier)
	if !found {
		return false
	}
	for _, cat := range tc.AllowedCategories {
		if cat == category {
			return true
		}
	}
	return false
}

// GetEffectiveSelectionStake returns the stake used for VRF selection.
// Tier 0/1 get virtual stake; tier 2/3 use real stake (if above verification minimum).
func (k Keeper) GetEffectiveSelectionStake(ctx sdk.Context, val *types.Validator) *big.Int {
	params := k.GetParams(ctx)

	totalStake, ok := new(big.Int).SetString(val.TotalStake, 10)
	if !ok {
		totalStake = new(big.Int)
	}

	minStake, ok := new(big.Int).SetString(params.MinStakeForVerification, 10)
	if !ok {
		minStake = new(big.Int)
	}

	// R3: below minimum → "0" (not eligible for VRF)
	if totalStake.Cmp(minStake) < 0 {
		zero := new(big.Int)
		return zero
	}

	// Tier 0/1: virtual stake for fair VRF probability
	if val.Tier <= types.TierVerified {
		vs, ok := new(big.Int).SetString(params.VirtualStake, 10)
		if ok {
			return vs
		}
		return new(big.Int)
	}

	// Tier 2/3: real total stake
	return totalStake
}

// CalculateTierReward applies the tier reward multiplier.
func (k Keeper) CalculateTierReward(ctx sdk.Context, tier types.ValidatorTier, baseReward *big.Int) *big.Int {
	tc, found := k.GetTierConfig(ctx, tier)
	if !found {
		return baseReward
	}
	result := new(big.Int).Mul(baseReward, new(big.Int).SetUint64(tc.RewardMultiplierBps))
	result.Div(result, big.NewInt(1000))
	return result
}

// CalculateTierSlash applies the tier slash multiplier (P2-1: exported but not used in SlashValidator).
func (k Keeper) CalculateTierSlash(ctx sdk.Context, tier types.ValidatorTier, baseSlash *big.Int) *big.Int {
	tc, found := k.GetTierConfig(ctx, tier)
	if !found {
		return baseSlash
	}
	result := new(big.Int).Mul(baseSlash, new(big.Int).SetUint64(tc.SlashMultiplierBps))
	result.Div(result, big.NewInt(1000))
	return result
}
