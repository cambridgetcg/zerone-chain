package keeper

import (
	"context"
	"fmt"

	sdkmath "cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/zerone-chain/zerone/x/knowledge/types"
)

// ─── Memory ↔ Training Bridge ───────────────────────────────────────────────
//
// Connects the training pipeline to the memory consolidation (R50) and
// reconsolidation (R51) systems. When a training run completes, this bridge:
//
// 1. Records activation for each TDU used in the training set
// 2. Records co-activations for Hebbian learning
// 3. Triggers reconsolidation for TDUs correlated with negative outcomes
// 4. Emits fitness signals based on training influence
//
// This is the nervous system connecting the training enclave (the body)
// to the memory system (the brain).

// TrainingOutcome represents the result of a training run for memory processing.
type TrainingOutcome struct {
	// TDUIDs are the TDU sample IDs that were in the training set.
	TDUIDs []string

	// OverallDelta is the benchmark score change from the training run.
	// Positive = model improved, negative = model degraded.
	OverallDelta sdkmath.LegacyDec

	// PerTDUDeltas maps individual TDUs to their influence on the outcome.
	// If not available, OverallDelta is distributed evenly.
	PerTDUDeltas map[string]sdkmath.LegacyDec

	// CurrentCycle is the fitness cycle when this outcome was produced.
	CurrentCycle uint64

	// AttestationHash links back to the on-chain training record.
	AttestationHash string
}

// ProcessTrainingOutcome integrates training results with the memory system.
// This is the core bridge between training and memory consolidation/reconsolidation.
//
// For each TDU in the training set:
//   - Records activation (R50) — retrieval strengthening
//   - Tracks performance correlation — was this TDU helpful?
//   - Triggers reconsolidation (R51) — if TDU correlated with negative outcome
//
// Also records co-activations for Hebbian learning across the batch.
func (k Keeper) ProcessTrainingOutcome(ctx context.Context, outcome TrainingOutcome) error {
	sdkCtx := sdk.UnwrapSDKContext(ctx)

	if len(outcome.TDUIDs) == 0 {
		return nil
	}

	// ── 1. Record individual activations ─────────────────────────────────
	negativeThreshold := sdkmath.LegacyNewDecWithPrec(-5, 2) // -0.05

	for _, tduID := range outcome.TDUIDs {
		// Determine per-TDU delta.
		delta := outcome.OverallDelta
		if outcome.PerTDUDeltas != nil {
			if perTDU, ok := outcome.PerTDUDeltas[tduID]; ok {
				delta = perTDU
			}
		}

		// Record activation (R50: retrieval strengthening).
		if err := k.RecordActivation(ctx, tduID, outcome.CurrentCycle, delta); err != nil {
			// Non-fatal: log and continue.
			sdkCtx.Logger().Error("failed to record activation",
				"tdu_id", tduID, "error", err)
			continue
		}

		// Trigger reconsolidation if this TDU is correlated with negative outcome (R51).
		if delta.LT(negativeThreshold) {
			_, err := k.TriggerReconsolidation(ctx, tduID, delta, outcome.CurrentCycle)
			if err != nil {
				// Expected errors: already open, disabled, canonical gate.
				// These are normal — just means this TDU doesn't need reconsolidation.
				if !isExpectedReconsolidationError(err) {
					sdkCtx.Logger().Error("unexpected reconsolidation error",
						"tdu_id", tduID, "error", err)
				}
			}
		}

		// Emit fitness signal from training influence.
		k.emitTrainingFitnessSignal(ctx, tduID, delta, outcome.CurrentCycle)
	}

	// ── 2. Record co-activations for Hebbian learning ────────────────────
	// Only for batches with multiple TDUs (cap at 20 to avoid O(n²) explosion).
	if len(outcome.TDUIDs) > 1 && len(outcome.TDUIDs) <= 20 {
		if err := k.RecordCoActivation(ctx, outcome.TDUIDs, outcome.CurrentCycle); err != nil {
			sdkCtx.Logger().Error("failed to record co-activations", "error", err)
		}
	}

	sdkCtx.EventManager().EmitEvent(sdk.NewEvent(
		"training_outcome_processed",
		sdk.NewAttribute("tdu_count", fmt.Sprintf("%d", len(outcome.TDUIDs))),
		sdk.NewAttribute("overall_delta", outcome.OverallDelta.String()),
		sdk.NewAttribute("attestation_hash", outcome.AttestationHash),
	))

	return nil
}

// emitTrainingFitnessSignal converts a training delta into a fitness signal.
func (k Keeper) emitTrainingFitnessSignal(ctx context.Context, tduID string, delta sdkmath.LegacyDec, currentCycle uint64) {
	// Convert delta to [0, 1] training influence score.
	// delta > 0 → influence > 0.5 (beneficial)
	// delta = 0 → influence = 0.5 (neutral)
	// delta < 0 → influence < 0.5 (harmful)
	influence := sdkmath.LegacyNewDecWithPrec(5, 1).Add(delta) // 0.5 + delta
	if influence.GT(sdkmath.LegacyOneDec()) {
		influence = sdkmath.LegacyOneDec()
	}
	if influence.IsNegative() {
		influence = sdkmath.LegacyZeroDec()
	}

	signal := types.FitnessSignal{
		TrainingInfluence: influence,
		UsageCorrelation:  sdkmath.LegacyNewDecWithPrec(5, 1), // neutral usage (updated separately)
		Redundancy:        sdkmath.LegacyNewDecWithPrec(5, 1), // neutral redundancy (updated separately)
	}

	// Non-fatal: best effort fitness signal.
	_ = k.UpdateFitnessScoreWithEvent(ctx, tduID, signal, currentCycle)
}

// isExpectedReconsolidationError returns true for errors that are normal
// (not bugs) during reconsolidation triggering.
func isExpectedReconsolidationError(err error) bool {
	return types.ErrReconsolidationAlreadyOpen.Is(err) ||
		types.ErrReconsolidationDisabled.Is(err) ||
		types.ErrCanonicalNotEnoughNegatives.Is(err)
}
