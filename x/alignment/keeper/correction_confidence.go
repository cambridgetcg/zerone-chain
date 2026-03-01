package keeper

import (
	"context"
	"fmt"

	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/zerone-chain/zerone/x/alignment/types"
)

// GetCorrectionConfidence calculates the correction success rate over the confidence window.
// Returns confidence in BPS (0-1,000,000). Returns 500,000 (neutral) if insufficient data.
func (k Keeper) GetCorrectionConfidence(ctx context.Context) uint64 {
	params := k.GetParams(ctx)
	windowSize := params.CorrectionConfidenceWindowSize
	if windowSize == 0 {
		windowSize = 50
	}

	outcomes := k.GetRecentCorrectionOutcomes(ctx, windowSize)

	minSamples := params.CorrectionConfidenceMinSamples
	if minSamples == 0 {
		minSamples = 5
	}
	if uint64(len(outcomes)) < minSamples {
		return 500_000 // neutral
	}

	successes := uint64(0)
	for _, o := range outcomes {
		if o.Successful {
			successes++
		}
	}

	return successes * types.BPS / uint64(len(outcomes))
}

// GetEffectiveMaxMagnitude returns the dynamic max auto-apply magnitude based on correction confidence.
func (k Keeper) GetEffectiveMaxMagnitude(ctx context.Context) uint64 {
	params := k.GetParams(ctx)
	baseMax := params.MaxAutoApplyMagnitudeBps
	if baseMax == 0 {
		return 0
	}

	confidence := k.GetCorrectionConfidence(ctx)

	if params.MinConfidenceForAutoApply > 0 && confidence < params.MinConfidenceForAutoApply {
		return 0 // governance only
	}

	minMul := params.CorrectionBoundsMinMultiplierBps
	maxMul := params.CorrectionBoundsMaxMultiplierBps
	if minMul == 0 || maxMul == 0 || maxMul <= minMul {
		return baseMax
	}

	// Linear scaling: confidence maps to [minMul, maxMul]
	multiplier := minMul + (confidence * (maxMul - minMul) / types.BPS)

	return baseMax * multiplier / types.BPS
}

// GetEffectiveObservationInterval returns the observation interval modulated by correction confidence.
func (k Keeper) GetEffectiveObservationInterval(ctx context.Context) uint64 {
	params := k.GetParams(ctx)
	baseInterval := params.ObservationIntervalBlocks

	confidence := k.GetCorrectionConfidence(ctx)

	if confidence > 800_000 {
		return baseInterval * 3 / 2 // 150%
	} else if confidence < 300_000 {
		return baseInterval * 2 / 3 // 67%
	}

	return baseInterval
}

// EvaluatePendingCorrections checks outcomes from the previous observation
// and determines if corrections were successful.
func (k Keeper) EvaluatePendingCorrections(ctx context.Context, currentScores *types.DimensionScores) {
	state := k.GetState(ctx)
	prevHeight := state.LastObservationHeight
	if prevHeight == 0 {
		return
	}

	params := k.GetParams(ctx)
	outcomes := k.GetCorrectionsAtHeight(ctx, prevHeight)
	for _, outcome := range outcomes {
		if outcome.ScoreAfter > 0 {
			continue // already evaluated
		}

		scoreAfter := types.GetDimensionScore(currentScores, outcome.Dimension)
		distBefore := absDistance(outcome.ScoreBefore, params.HealthyThreshold)
		distAfter := absDistance(scoreAfter, params.HealthyThreshold)

		outcome.ScoreAfter = scoreAfter
		outcome.Successful = distAfter < distBefore

		k.SetCorrectionOutcome(ctx, outcome)

		sdkCtx := sdk.UnwrapSDKContext(ctx)
		sdkCtx.EventManager().EmitEvent(
			sdk.NewEvent("zerone.alignment.correction_outcome_recorded",
				sdk.NewAttribute("height", fmt.Sprintf("%d", outcome.Height)),
				sdk.NewAttribute("dimension", outcome.Dimension),
				sdk.NewAttribute("magnitude", fmt.Sprintf("%d", outcome.Magnitude)),
				sdk.NewAttribute("score_before", fmt.Sprintf("%d", outcome.ScoreBefore)),
				sdk.NewAttribute("score_after", fmt.Sprintf("%d", outcome.ScoreAfter)),
				sdk.NewAttribute("successful", fmt.Sprintf("%t", outcome.Successful)),
			),
		)
	}
}

func absDistance(a, b uint64) uint64 {
	if a > b {
		return a - b
	}
	return b - a
}

// PruneOldOutcomes removes correction outcomes older than windowSize*2 observations.
func (k Keeper) PruneOldOutcomes(ctx context.Context) {
	params := k.GetParams(ctx)
	state := k.GetState(ctx)
	windowSize := params.CorrectionConfidenceWindowSize
	if windowSize == 0 {
		windowSize = 50
	}

	// Only prune every windowSize observations.
	if state.ObservationCount%windowSize != 0 {
		return
	}

	cutoffObservations := windowSize * 2
	baseInterval := params.ObservationIntervalBlocks
	if baseInterval == 0 || state.LastObservationHeight == 0 {
		return
	}

	cutoffHeight := uint64(0)
	totalBlocksBack := cutoffObservations * baseInterval
	if state.LastObservationHeight > totalBlocksBack {
		cutoffHeight = state.LastObservationHeight - totalBlocksBack
	}

	if cutoffHeight == 0 {
		return
	}

	st := k.storeService.OpenKVStore(ctx)
	endKey := types.CorrectionOutcomeHeightPrefix(cutoffHeight)
	iter, err := st.Iterator(types.CorrectionOutcomeKeyPrefix, endKey)
	if err != nil {
		return
	}
	defer iter.Close()

	var keysToDelete [][]byte
	for ; iter.Valid(); iter.Next() {
		keysToDelete = append(keysToDelete, append([]byte{}, iter.Key()...))
	}

	for _, key := range keysToDelete {
		_ = st.Delete(key)
	}
}

// CategorizeConfidence returns a human-readable category for the confidence level.
func CategorizeConfidence(confidence uint64) string {
	switch {
	case confidence < 200_000:
		return "restricted"
	case confidence < 400_000:
		return "cautious"
	case confidence < 600_000:
		return "normal"
	case confidence < 800_000:
		return "confident"
	default:
		return "autonomous"
	}
}
