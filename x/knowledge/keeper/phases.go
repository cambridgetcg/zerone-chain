package keeper

import (
	"context"
	"fmt"

	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/zerone-chain/zerone/x/knowledge/types"
)

// BeginBlocker runs knowledge module begin-block logic.
// Advances verification round phases by deadline.
func (k Keeper) BeginBlocker(ctx context.Context) error {
	return k.AdvanceRoundPhases(ctx)
}

// AdvanceRoundPhases iterates all active rounds and transitions phases by deadline.
func (k Keeper) AdvanceRoundPhases(ctx context.Context) error {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	height := uint64(sdkCtx.BlockHeight())

	params, err := k.GetParams(ctx)
	if err != nil {
		return err
	}

	// Collect rounds to process (avoid modifying store during iteration)
	var roundsToProcess []*types.VerificationRound
	k.IterateActiveRounds(ctx, func(round *types.VerificationRound) bool {
		roundsToProcess = append(roundsToProcess, round)
		return false
	})

	for _, round := range roundsToProcess {
		expectedPhase := GetExpectedPhase(round, height, params)

		if expectedPhase == round.Phase {
			continue // no transition needed
		}

		switch expectedPhase {
		case types.VerificationPhase_VERIFICATION_PHASE_REVEAL:
			// COMMIT → REVEAL transition
			round.Phase = types.VerificationPhase_VERIFICATION_PHASE_REVEAL
			if err := k.SetVerificationRound(ctx, round); err != nil {
				k.Logger(ctx).Error("failed to transition round to REVEAL", "round_id", round.Id, "error", err)
			}
			sdkCtx.EventManager().EmitEvent(sdk.NewEvent(
				"zerone.knowledge.round_phase_changed",
				sdk.NewAttribute("round_id", round.Id),
				sdk.NewAttribute("phase", "REVEAL"),
			))

		case types.VerificationPhase_VERIFICATION_PHASE_AGGREGATION:
			// REVEAL → AGGREGATION transition
			round.Phase = types.VerificationPhase_VERIFICATION_PHASE_AGGREGATION
			if err := k.SetVerificationRound(ctx, round); err != nil {
				k.Logger(ctx).Error("failed to transition round to AGGREGATION", "round_id", round.Id, "error", err)
				continue
			}
			sdkCtx.EventManager().EmitEvent(sdk.NewEvent(
				"zerone.knowledge.round_phase_changed",
				sdk.NewAttribute("round_id", round.Id),
				sdk.NewAttribute("phase", "AGGREGATION"),
				sdk.NewAttribute("reveal_count", fmt.Sprintf("%d", len(round.Reveals))),
			))
			// Perform aggregation immediately
			if err := k.performAggregation(ctx, round); err != nil {
				k.Logger(ctx).Error("aggregation failed", "round_id", round.Id, "error", err)
			}

		case types.VerificationPhase_VERIFICATION_PHASE_EXPIRED:
			// Round has expired — check if we can still aggregate
			if uint64(len(round.Reveals)) >= params.MinVerifiers {
				// Enough reveals — aggregate
				round.Phase = types.VerificationPhase_VERIFICATION_PHASE_AGGREGATION
				if err := k.SetVerificationRound(ctx, round); err != nil {
					continue
				}
				if err := k.performAggregation(ctx, round); err != nil {
					k.Logger(ctx).Error("late aggregation failed", "round_id", round.Id, "error", err)
				}
			} else {
				// Insufficient reveals — mark as expired
				round.Phase = types.VerificationPhase_VERIFICATION_PHASE_EXPIRED
				round.Verdict = types.Verdict_VERDICT_INCONCLUSIVE
				if err := k.SetVerificationRound(ctx, round); err != nil {
					continue
				}
				// Review fee is non-refundable — mark claim as insufficient
				claim, found := k.GetClaim(ctx, round.ClaimId)
				if found {
					claim.Status = types.ClaimStatus_CLAIM_STATUS_INSUFFICIENT
					_ = k.SetClaim(ctx, claim)
				}
				sdkCtx.EventManager().EmitEvent(sdk.NewEvent(
					"zerone.knowledge.round_expired",
					sdk.NewAttribute("round_id", round.Id),
					sdk.NewAttribute("reveals", fmt.Sprintf("%d", len(round.Reveals))),
				))
			}
		}
	}

	return nil
}

// GetExpectedPhase is a pure function that maps a block height to the expected phase.
func GetExpectedPhase(round *types.VerificationRound, height uint64, params *types.Params) types.VerificationPhase {
	if round.Phase == types.VerificationPhase_VERIFICATION_PHASE_COMPLETE ||
		round.Phase == types.VerificationPhase_VERIFICATION_PHASE_EXPIRED {
		return round.Phase
	}

	if height >= round.AggregationDeadline {
		return types.VerificationPhase_VERIFICATION_PHASE_EXPIRED
	}
	if height >= round.RevealDeadline {
		return types.VerificationPhase_VERIFICATION_PHASE_AGGREGATION
	}
	if height >= round.CommitDeadline {
		return types.VerificationPhase_VERIFICATION_PHASE_REVEAL
	}
	return types.VerificationPhase_VERIFICATION_PHASE_COMMIT
}

// performAggregation aggregates votes and completes the round.
func (k Keeper) performAggregation(ctx context.Context, round *types.VerificationRound) error {
	result, err := k.AggregateVerificationResult(ctx, round)
	if err != nil {
		return err
	}
	return k.CompleteRound(ctx, round, result)
}
