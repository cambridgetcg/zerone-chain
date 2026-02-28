package keeper

import (
	"context"

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
