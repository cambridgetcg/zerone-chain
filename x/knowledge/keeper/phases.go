package keeper

import (
	"context"

	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/zerone-chain/zerone/x/knowledge/types"
)

// BeginBlocker processes active quality rounds, transitioning phases based on block deadlines.
func (k Keeper) BeginBlocker(ctx context.Context) error {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	block := uint64(sdkCtx.BlockHeight())

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
				round.Phase = types.VerificationPhase_VERIFICATION_PHASE_REVEAL
				_ = k.SetQualityRound(ctx, round)
			}

		case types.VerificationPhase_VERIFICATION_PHASE_REVEAL:
			if block > round.RevealDeadline {
				if len(round.Reveals) > 0 {
					_ = k.AggregateQualityRound(ctx, roundID)
				} else {
					round.Phase = types.VerificationPhase_VERIFICATION_PHASE_EXPIRED
					_ = k.SetQualityRound(ctx, round)
					_ = k.DeleteActiveRound(ctx, roundID)
				}
			}
		}
	}

	return nil
}
