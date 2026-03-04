package keeper

import (
	"context"

	sdkmath "cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/zerone-chain/zerone/x/knowledge/types"
)

// BeginBlocker processes active quality rounds, transitioning phases based on block deadlines.
func (k Keeper) BeginBlocker(ctx context.Context) error {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	block := uint64(sdkCtx.BlockHeight())
	params, _ := k.GetParams(ctx)

	activeRoundIDs := k.GetActiveRounds(ctx)
	for _, roundID := range activeRoundIDs {
		round, found := k.GetQualityRound(ctx, roundID)
		if !found {
			_ = k.DeleteActiveRound(ctx, roundID)
			continue
		}

		switch round.Phase {
		case types.VerificationPhase_VERIFICATION_PHASE_COMMIT:
			if block > round.CommitDeadline {
				minValidators := uint64(3)
				if params != nil && params.MinValidatorsPerRound > 0 {
					minValidators = params.MinValidatorsPerRound
				}
				if uint64(len(round.Commits)) >= minValidators {
					round.Phase = types.VerificationPhase_VERIFICATION_PHASE_REVEAL
					_ = k.SetQualityRound(ctx, round)
				} else {
					k.expireRound(ctx, round)
				}
			}

		case types.VerificationPhase_VERIFICATION_PHASE_REVEAL:
			if block > round.RevealDeadline {
				if len(round.Reveals) > 0 {
					_ = k.AggregateQualityRound(ctx, roundID)
				} else {
					k.expireRound(ctx, round)
				}
			}
		}
	}

	return nil
}

// expireRound marks a round as expired, removes from active index, and returns stake to submitter.
func (k Keeper) expireRound(ctx context.Context, round *types.QualityRound) {
	sdkCtx := sdk.UnwrapSDKContext(ctx)

	round.Phase = types.VerificationPhase_VERIFICATION_PHASE_EXPIRED
	_ = k.SetQualityRound(ctx, round)
	_ = k.DeleteActiveRound(ctx, round.Id)

	// Return stake to submitter and reset submission status
	sub, found := k.GetSubmission(ctx, round.SubmissionId)
	if found {
		if sub.Submitter != "" && sub.Stake != "" {
			submitterAddr, addrErr := sdk.AccAddressFromBech32(sub.Submitter)
			if addrErr == nil {
				stakeAmt, ok := sdkmath.NewIntFromString(sub.Stake)
				if ok && stakeAmt.IsPositive() {
					stakeCoin := sdk.NewCoin("uzrn", stakeAmt)
					_ = k.bankKeeper.SendCoinsFromModuleToAccount(sdkCtx, types.ModuleName, submitterAddr, sdk.NewCoins(stakeCoin))
				}
			}
		}
		sub.Status = types.SubmissionStatus_SUBMISSION_STATUS_PENDING
		_ = k.SetSubmission(ctx, sub)
	}

	sdkCtx.EventManager().EmitEvent(sdk.NewEvent(
		"quality_round_expired",
		sdk.NewAttribute("round_id", round.Id),
		sdk.NewAttribute("submission_id", round.SubmissionId),
	))
}
