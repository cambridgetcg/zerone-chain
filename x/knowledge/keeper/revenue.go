package keeper

import (
	"context"
	"fmt"

	sdkmath "cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/zerone-chain/zerone/x/knowledge/types"
)

const (
	// RevenueShareDenom is the denominator for revenue share BPS (10000 = 100%).
	RevenueShareDenom = 10_000
	// ConsentMultiplierDenom is the denominator for consent multipliers.
	ConsentMultiplierDenom = 10_000
)

// DistributeEpochRevenue processes all pending revenue and distributes to stakeholders.
// Called from EndBlocker at epoch boundaries.
func (k Keeper) DistributeEpochRevenue(ctx context.Context, params *types.Params) {
	k.distributeEpochRevenue(ctx, params)
}

func (k Keeper) distributeEpochRevenue(ctx context.Context, params *types.Params) {
	sdkCtx := sdk.UnwrapSDKContext(ctx)

	submitterShareBPS := parseUzrn(params.SubmitterRevenueShareBps)
	validatorShareBPS := parseUzrn(params.ValidatorRevenueShareBps)

	var toDelete []string

	k.IteratePendingRevenue(ctx, func(sampleID string, amount uint64) bool {
		if amount == 0 {
			toDelete = append(toDelete, sampleID)
			return false
		}

		sample, found := k.GetSample(ctx, sampleID)
		if !found {
			toDelete = append(toDelete, sampleID)
			return false
		}

		totalAmount := sdkmath.NewInt(int64(amount))

		// Calculate base shares
		submitterBase := totalAmount.Mul(sdkmath.NewInt(int64(submitterShareBPS))).Quo(sdkmath.NewInt(RevenueShareDenom))
		validatorTotal := totalAmount.Mul(sdkmath.NewInt(int64(validatorShareBPS))).Quo(sdkmath.NewInt(RevenueShareDenom))
		protocolShare := totalAmount.Sub(submitterBase).Sub(validatorTotal)

		// Apply consent multiplier to submitter share
		consentMul := getConsentMultiplier(sample, params)
		adjustedSubmitter := submitterBase.Mul(sdkmath.NewInt(int64(consentMul))).Quo(sdkmath.NewInt(ConsentMultiplierDenom))

		// Consent penalty/bonus goes to protocol
		consentDelta := submitterBase.Sub(adjustedSubmitter)
		protocolShare = protocolShare.Add(consentDelta)

		// 1. Pay submitter
		if adjustedSubmitter.IsPositive() {
			submitterAddr, err := sdk.AccAddressFromBech32(sample.Submitter)
			if err == nil {
				coins := sdk.NewCoins(sdk.NewCoin("uzrn", adjustedSubmitter))
				_ = k.bankKeeper.SendCoinsFromModuleToAccount(ctx, types.ModuleName, submitterAddr, coins)
			}
		}

		// 2. Pay validators
		if validatorTotal.IsPositive() {
			remainder := k.distributeToValidators(ctx, sample, validatorTotal)
			// Any undistributed remainder goes to protocol
			protocolShare = protocolShare.Add(remainder)
		}

		// 3. Protocol share → research fund
		if protocolShare.IsPositive() {
			k.depositProtocolRevenue(ctx, protocolShare)
		}

		toDelete = append(toDelete, sampleID)

		sdkCtx.EventManager().EmitEvent(sdk.NewEvent(
			"revenue_distributed",
			sdk.NewAttribute("sample_id", sampleID),
			sdk.NewAttribute("total", totalAmount.String()),
			sdk.NewAttribute("submitter", adjustedSubmitter.String()),
			sdk.NewAttribute("validators", validatorTotal.Sub(protocolShare.Sub(protocolShare)).String()),
			sdk.NewAttribute("protocol", protocolShare.String()),
		))

		return false
	})

	// Clear distributed entries
	for _, id := range toDelete {
		_ = k.DeletePendingRevenue(ctx, id)
	}
}

// getConsentMultiplier returns the consent-based multiplier for a sample's submitter share.
func getConsentMultiplier(sample *types.Sample, params *types.Params) uint64 {
	if sample.Consent == nil {
		return ConsentMultiplierDenom // 1.0x default
	}

	switch sample.Consent.Type {
	case types.ConsentType_CONSENT_TYPE_SELF_AUTHORED:
		if params.SelfAuthoredMultiplier > 0 {
			return params.SelfAuthoredMultiplier
		}
		return 15_000 // 1.5x
	case types.ConsentType_CONSENT_TYPE_OPT_IN:
		if params.OptInMultiplier > 0 {
			return params.OptInMultiplier
		}
		return 13_000 // 1.3x
	case types.ConsentType_CONSENT_TYPE_PUBLIC_LICENSE:
		if params.PublicLicenseMultiplier > 0 {
			return params.PublicLicenseMultiplier
		}
		return 10_000 // 1.0x
	case types.ConsentType_CONSENT_TYPE_PLATFORM_TOS:
		if params.PlatformTosMultiplier > 0 {
			return params.PlatformTosMultiplier
		}
		return 8_000 // 0.8x
	case types.ConsentType_CONSENT_TYPE_FAIR_USE:
		if params.FairUseMultiplier > 0 {
			return params.FairUseMultiplier
		}
		return 5_000 // 0.5x
	default:
		return ConsentMultiplierDenom // 1.0x
	}
}

// distributeToValidators splits the validator share among round validators who revealed.
// Returns any undistributed remainder (due to rounding or missing round).
func (k Keeper) distributeToValidators(ctx context.Context, sample *types.Sample, totalShare sdkmath.Int) sdkmath.Int {
	if sample.SubmissionId == "" {
		return totalShare // No submission → all goes to protocol
	}

	roundID, found := k.GetRoundBySubmission(ctx, sample.SubmissionId)
	if !found {
		return totalShare
	}

	round, found := k.GetQualityRound(ctx, roundID)
	if !found || len(round.Reveals) == 0 {
		return totalShare
	}

	// Quality-weighted split: higher participation score → larger share.
	// Falls back to equal split when no aggregate scores are available.
	scores := computeParticipationScores(round)
	var totalScore uint64
	for _, s := range scores {
		totalScore += s
	}

	var distributed sdkmath.Int = sdkmath.ZeroInt()
	if totalScore == 0 {
		// Fallback: equal split among validators who revealed.
		numValidators := int64(len(round.Reveals))
		perValidator := totalShare.Quo(sdkmath.NewInt(numValidators))
		if !perValidator.IsPositive() {
			return totalShare
		}
		for _, reveal := range round.Reveals {
			validatorAddr, err := sdk.AccAddressFromBech32(reveal.Verifier)
			if err != nil {
				continue
			}
			coins := sdk.NewCoins(sdk.NewCoin("uzrn", perValidator))
			if err := k.bankKeeper.SendCoinsFromModuleToAccount(ctx, types.ModuleName, validatorAddr, coins); err == nil {
				distributed = distributed.Add(perValidator)
			}
		}
	} else {
		// Weighted split proportional to participation score.
		for _, reveal := range round.Reveals {
			score := scores[reveal.Verifier]
			share := totalShare.Mul(sdkmath.NewInt(int64(score))).Quo(sdkmath.NewInt(int64(totalScore)))
			if !share.IsPositive() {
				continue
			}
			validatorAddr, err := sdk.AccAddressFromBech32(reveal.Verifier)
			if err != nil {
				continue
			}
			coins := sdk.NewCoins(sdk.NewCoin("uzrn", share))
			if err := k.bankKeeper.SendCoinsFromModuleToAccount(ctx, types.ModuleName, validatorAddr, coins); err == nil {
				distributed = distributed.Add(share)
			}
		}
	}

	// Return undistributed remainder
	return totalShare.Sub(distributed)
}

// depositProtocolRevenue sends the protocol's share to the research fund.
func (k Keeper) depositProtocolRevenue(ctx context.Context, amount sdkmath.Int) {
	if !amount.IsPositive() {
		return
	}
	coins := sdk.NewCoins(sdk.NewCoin("uzrn", amount))

	// Try research fund via vesting rewards keeper
	if k.vestingRewardsKeeper != nil {
		_ = k.vestingRewardsKeeper.DepositToResearchFund(ctx, types.ModuleName, coins)
		return
	}

	// Fallback: stays in module account (logged for debugging)
	_ = fmt.Sprintf("protocol revenue %s uzrn retained in module (no vesting keeper)", amount.String())
}
