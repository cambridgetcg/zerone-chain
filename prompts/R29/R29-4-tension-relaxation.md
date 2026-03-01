# R29-4 — 張弛 (Tension and Relaxation): Correction Confidence

## Context

R28-7 activated the alignment module with bounded corrections — corrections below `MaxAutoApplyMagnitudeBps` are auto-applied via autopoiesis, larger ones emit governance-required events. But the bounds are static. A system that has successfully auto-applied 50 corrections and improved its health every time still operates under the same magnitude cap as one that just booted.

The system can't learn from its own correction history. It can't earn greater autonomy through demonstrated competence, nor lose it through demonstrated failure.

## Polarity

- **Yang (張 tension):** Correction application, frequency increase, auto-apply
- **Yin (弛 relaxation):** Magnitude bounds, governance-required events, observation intervals
- **Coupling:** Correction confidence — the system's track record of successful corrections modulates how much autonomous correction authority it earns

## Architecture

### 1. Correction Outcome Tracking

Extend `AlignmentState` with correction history:

```
CorrectionOutcome {
    Height           uint64
    Dimension        string
    Magnitude        uint64
    Direction        string  // "increase" or "decrease"
    ScoreBefore      uint64  // dimension score before correction
    ScoreAfter       uint64  // dimension score at next observation after correction
    Successful       bool    // ScoreAfter closer to healthy than ScoreBefore
}
```

Store: `correction_outcome/{height}/{dimension}`

### 2. Recording Outcomes

In the alignment EndBlocker, after computing scores, check if any previous corrections are pending outcome evaluation:

```go
func (k Keeper) EvaluatePendingCorrections(ctx context.Context, currentScores *types.DimensionScores) {
    // Get corrections applied at the previous observation height
    prevHeight := state.LastObservationHeight
    if prevHeight == 0 { return }
    
    outcomes := k.GetCorrectionsAtHeight(ctx, prevHeight)
    for _, correction := range outcomes {
        if correction.ScoreAfter > 0 { continue } // already evaluated
        
        scoreBefore := correction.ScoreBefore
        scoreAfter := getDimensionScore(currentScores, correction.Dimension)
        
        // Determine if correction was successful
        // Success = score moved toward healthy threshold
        params := k.GetParams(ctx)
        distBefore := absDistance(scoreBefore, params.HealthyThreshold)
        distAfter := absDistance(scoreAfter, params.HealthyThreshold)
        
        correction.ScoreAfter = scoreAfter
        correction.Successful = distAfter < distBefore
        
        k.SetCorrectionOutcome(ctx, correction)
    }
}
```

**In ApplyCorrections**, record the pre-correction state:

```go
func (k Keeper) ApplyCorrections(ctx context.Context, corrections []*types.CorrectionRecord) {
    currentScores := k.ComputeScores(ctx, k.GetLastObservation(ctx))
    
    for _, c := range corrections {
        // Record outcome tracking before applying
        outcome := &types.CorrectionOutcome{
            Height:      uint64(sdkCtx.BlockHeight()),
            Dimension:   string(c.Dimension),
            Magnitude:   c.Magnitude,
            Direction:   c.Direction,
            ScoreBefore: getDimensionScore(currentScores, c.Dimension),
        }
        k.SetCorrectionOutcome(ctx, outcome)
        
        // ... existing bounded application logic ...
    }
}
```

### 3. Correction Confidence Score

```go
func (k Keeper) GetCorrectionConfidence(ctx context.Context) uint64 {
    // Look at last N correction outcomes
    params := k.GetParams(ctx)
    outcomes := k.GetRecentCorrectionOutcomes(ctx, params.CorrectionConfidenceWindowSize)
    
    if len(outcomes) < params.CorrectionConfidenceMinSamples {
        return 500_000 // neutral — not enough data
    }
    
    successes := uint64(0)
    for _, o := range outcomes {
        if o.Successful {
            successes++
        }
    }
    
    // Confidence = success rate in BPS
    return successes * BPS / uint64(len(outcomes))
}
```

### 4. Dynamic Bounds Modulation

In `ApplyCorrections`, the magnitude check becomes dynamic:

```go
func (k Keeper) getEffectiveMaxMagnitude(ctx context.Context) uint64 {
    params := k.GetParams(ctx)
    baseMax := params.MaxAutoApplyMagnitudeBps
    confidence := k.GetCorrectionConfidence(ctx)
    
    // Scale bounds based on confidence:
    // - 50% confidence (neutral) → 100% of base max
    // - 80% confidence → 160% of base max (earned autonomy)
    // - 30% confidence → 60% of base max (lost autonomy)
    // - Below MinConfidenceForAutoApply → 0 (all corrections require governance)
    
    if confidence < params.MinConfidenceForAutoApply {
        return 0 // system has proven it can't self-correct — governance only
    }
    
    // Linear scaling: confidence maps to [MinBoundsMultiplier, MaxBoundsMultiplier]
    multiplier := params.CorrectionBoundsMinMultiplierBps + 
        (confidence * (params.CorrectionBoundsMaxMultiplierBps - params.CorrectionBoundsMinMultiplierBps) / BPS)
    
    return baseMax * multiplier / BPS
}
```

### 5. Observation Frequency Modulation

The alignment observation interval should also respond to correction confidence:

```go
func (k Keeper) getEffectiveObservationInterval(ctx context.Context) uint64 {
    state := k.GetState(ctx)
    params := k.GetParams(ctx)
    
    // Health-based override from R28-7 takes priority
    if state.OverrideIntervalBlocks > 0 {
        return state.OverrideIntervalBlocks
    }
    
    confidence := k.GetCorrectionConfidence(ctx)
    baseInterval := params.ObservationIntervalBlocks
    
    // High confidence → can observe less frequently (system is stable)
    // Low confidence → observe more frequently (system needs monitoring)
    if confidence > 800_000 {
        return baseInterval * 3 / 2 // 150% interval (less frequent)
    } else if confidence < 300_000 {
        return baseInterval * 2 / 3 // 67% interval (more frequent)
    }
    
    return baseInterval
}
```

### 6. Parameters

Add to alignment params:
```
CorrectionConfidenceWindowSize     uint64  // default: 50 — last 50 corrections
CorrectionConfidenceMinSamples     uint64  // default: 5 — need at least 5 outcomes
MinConfidenceForAutoApply          uint64  // default: 200_000 — below 20% success → governance only
CorrectionBoundsMinMultiplierBps   uint64  // default: 300_000 — 30% of base at worst
CorrectionBoundsMaxMultiplierBps   uint64  // default: 2_000_000 — 200% of base at best
```

### 7. Pruning

Correction outcomes older than `CorrectionConfidenceWindowSize * 2` observations can be pruned in EndBlocker to prevent unbounded growth.

### 8. Events

```
correction_confidence_updated {
    confidence_bps: uint64
    total_corrections: uint64
    successful_corrections: uint64
    effective_max_magnitude: uint64
    category: "restricted" | "cautious" | "normal" | "confident" | "autonomous"
}

correction_outcome_recorded {
    height: uint64
    dimension: string
    magnitude: uint64
    score_before: uint64
    score_after: uint64
    successful: bool
}
```

### 9. Query

Add `CorrectionConfidence` to alignment query:
```
rpc CorrectionConfidence(QueryCorrectionConfidenceRequest) returns (QueryCorrectionConfidenceResponse)

QueryCorrectionConfidenceRequest {}
QueryCorrectionConfidenceResponse {
    confidence_bps: uint64
    total_corrections: uint64
    successful_corrections: uint64
    effective_max_magnitude: uint64
    effective_observation_interval: uint64
    recent_outcomes: repeated CorrectionOutcome
}
```

## Tests

1. **Outcome recording:** Apply correction, advance blocks, observe — outcome recorded with correct before/after scores.
2. **Success determination:** Correction that moves score toward healthy = successful.
3. **Confidence calculation:** 8/10 successful → 800,000 confidence.
4. **Bounds widening:** High confidence → effective max magnitude > base max.
5. **Bounds tightening:** Low confidence → effective max magnitude < base max.
6. **Governance lockout:** Below MinConfidenceForAutoApply → all corrections require governance.
7. **Frequency modulation:** High confidence → longer observation intervals.
8. **Min samples:** Fewer than 5 outcomes → neutral confidence (500,000).
9. **Integration:** Boot → neutral bounds → successful corrections → bounds widen → system earns autonomy.

## What This Changes

Before R29-4: The system has fixed correction authority. A fresh chain and a battle-tested chain operate under identical bounds.

After R29-4: The system earns autonomy. Successful corrections widen the bounds. Failed corrections tighten them. A system that consistently makes things worse loses the right to self-correct and must go through governance. A system that consistently makes things better earns broader correction authority.

The yin-yang: tension (張) and relaxation (弛) aren't opposites — they're a rhythm. The bow must be drawn (tension) to release the arrow (relaxation). The system must prove itself (tension) to earn freedom (relaxation). And freedom earned through competence can be lost through incompetence.
