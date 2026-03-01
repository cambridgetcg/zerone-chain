# R29-4 Correction Confidence Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add correction outcome tracking and confidence scoring so the alignment module earns or loses autonomous correction authority based on demonstrated competence.

**Architecture:** New `CorrectionOutcome` type stored at `correction_outcome/{height}/{dimension}`. Outcomes evaluated in EndBlocker at each observation point by comparing pre/post-correction dimension scores. Confidence score (success rate over sliding window) modulates `MaxAutoApplyMagnitudeBps` and `ObservationIntervalBlocks` dynamically. New gRPC query and CLI command expose the confidence state.

**Tech Stack:** Go 1.24+, Cosmos SDK v0.50.15, JSON-serialized KV state, hand-edited protobuf types.

---

### Task 1: Add CorrectionOutcome Type and Storage Keys

**Files:**
- Create: `x/alignment/types/correction_confidence.go`
- Modify: `x/alignment/types/keys.go:11-20`

**Step 1: Create the CorrectionOutcome type**

Create `x/alignment/types/correction_confidence.go`:

```go
package types

// CorrectionOutcome tracks the result of a correction application.
type CorrectionOutcome struct {
	Height      uint64 `json:"height"`
	Dimension   string `json:"dimension"`
	Magnitude   uint64 `json:"magnitude"`
	Direction   string `json:"direction"`
	ScoreBefore uint64 `json:"score_before"`
	ScoreAfter  uint64 `json:"score_after"`
	Successful  bool   `json:"successful"`
}

// QueryCorrectionConfidenceRequest is the request for CorrectionConfidence query.
type QueryCorrectionConfidenceRequest struct{}

// QueryCorrectionConfidenceResponse is the response for CorrectionConfidence query.
type QueryCorrectionConfidenceResponse struct {
	ConfidenceBps              uint64               `json:"confidence_bps"`
	TotalCorrections           uint64               `json:"total_corrections"`
	SuccessfulCorrections      uint64               `json:"successful_corrections"`
	EffectiveMaxMagnitude      uint64               `json:"effective_max_magnitude"`
	EffectiveObservationInterval uint64             `json:"effective_observation_interval"`
	RecentOutcomes             []*CorrectionOutcome `json:"recent_outcomes"`
}
```

**Step 2: Add storage keys**

Add to `x/alignment/types/keys.go` after `CorrectionCountKey`:

```go
CorrectionOutcomeKeyPrefix = []byte{0x08}
```

And add a key function:

```go
// CorrectionOutcomeKey returns the store key for a correction outcome at height + dimension.
func CorrectionOutcomeKey(height uint64, dimension string) []byte {
	prefix := append(CorrectionOutcomeKeyPrefix, heightBytes(height)...)
	return append(prefix, []byte(dimension)...)
}
```

**Step 3: Run tests to verify existing tests still pass**

Run: `go test ./x/alignment/... -count=1 -v 2>&1 | tail -5`
Expected: PASS (new code not yet used)

**Step 4: Commit**

```
feat(alignment): add CorrectionOutcome type and storage keys (R29-4)
```

---

### Task 2: Add Correction Confidence Parameters

**Files:**
- Modify: `x/alignment/types/genesis.pb.go` (Params struct, ~line 109-237)
- Modify: `x/alignment/types/genesis.go:7-21` (DefaultParams)
- Modify: `x/alignment/types/genesis.go:44-72` (Validate)

**Step 1: Add new param fields to Params struct**

In `genesis.pb.go`, add after `MaxAutoApplyMagnitudeBps` field:

```go
CorrectionConfidenceWindowSize   uint64 `protobuf:"varint,12,opt,name=correction_confidence_window_size,json=correctionConfidenceWindowSize,proto3" json:"correction_confidence_window_size,omitempty"`
CorrectionConfidenceMinSamples   uint64 `protobuf:"varint,13,opt,name=correction_confidence_min_samples,json=correctionConfidenceMinSamples,proto3" json:"correction_confidence_min_samples,omitempty"`
MinConfidenceForAutoApply        uint64 `protobuf:"varint,14,opt,name=min_confidence_for_auto_apply,json=minConfidenceForAutoApply,proto3" json:"min_confidence_for_auto_apply,omitempty"`
CorrectionBoundsMinMultiplierBps uint64 `protobuf:"varint,15,opt,name=correction_bounds_min_multiplier_bps,json=correctionBoundsMinMultiplierBps,proto3" json:"correction_bounds_min_multiplier_bps,omitempty"`
CorrectionBoundsMaxMultiplierBps uint64 `protobuf:"varint,16,opt,name=correction_bounds_max_multiplier_bps,json=correctionBoundsMaxMultiplierBps,proto3" json:"correction_bounds_max_multiplier_bps,omitempty"`
```

Add getter methods after existing getters:

```go
func (x *Params) GetCorrectionConfidenceWindowSize() uint64 {
	if x != nil { return x.CorrectionConfidenceWindowSize }
	return 0
}
func (x *Params) GetCorrectionConfidenceMinSamples() uint64 {
	if x != nil { return x.CorrectionConfidenceMinSamples }
	return 0
}
func (x *Params) GetMinConfidenceForAutoApply() uint64 {
	if x != nil { return x.MinConfidenceForAutoApply }
	return 0
}
func (x *Params) GetCorrectionBoundsMinMultiplierBps() uint64 {
	if x != nil { return x.CorrectionBoundsMinMultiplierBps }
	return 0
}
func (x *Params) GetCorrectionBoundsMaxMultiplierBps() uint64 {
	if x != nil { return x.CorrectionBoundsMaxMultiplierBps }
	return 0
}
```

**Step 2: Set defaults in DefaultParams**

In `genesis.go`, add to `DefaultParams()`:

```go
CorrectionConfidenceWindowSize:   50,
CorrectionConfidenceMinSamples:   5,
MinConfidenceForAutoApply:        200_000,  // 20%
CorrectionBoundsMinMultiplierBps: 300_000,  // 30%
CorrectionBoundsMaxMultiplierBps: 2_000_000, // 200%
```

**Step 3: Add validation**

In `genesis.go` `Validate()`, add before `return nil`:

```go
if p.CorrectionBoundsMinMultiplierBps > p.CorrectionBoundsMaxMultiplierBps {
	return ErrInvalidConfidenceBounds
}
```

**Step 4: Add error**

In `errors.go`, add:

```go
ErrInvalidConfidenceBounds = errors.Register(ModuleName, 9, "min bounds multiplier exceeds max bounds multiplier")
```

**Step 5: Run tests**

Run: `go test ./x/alignment/... -count=1 -v 2>&1 | tail -5`
Expected: PASS

**Step 6: Commit**

```
feat(alignment): add correction confidence parameters (R29-4)
```

---

### Task 3: Add Correction Outcome State Management

**Files:**
- Modify: `x/alignment/keeper/state.go` (add outcome CRUD methods)
- Modify: `x/alignment/types/correction_confidence.go` (add helper)

**Step 1: Add getDimensionScore helper**

In `x/alignment/types/correction_confidence.go`, add:

```go
// GetDimensionScore extracts a dimension's score from DimensionScores by name.
func GetDimensionScore(scores *DimensionScores, dimension string) uint64 {
	switch dimension {
	case DimKnowledgeQuality:
		return scores.KnowledgeQuality
	case DimEconomicStability:
		return scores.EconomicStability
	case DimGovernanceParticipation:
		return scores.GovernanceParticipation
	case DimNetworkSecurity:
		return scores.NetworkSecurity
	case DimStakingRatio:
		return scores.StakingRatio
	default:
		return 0
	}
}
```

**Step 2: Add outcome state methods to keeper**

In `x/alignment/keeper/state.go`, add after the corrections section:

```go
// --- Correction Outcomes ---

func (k Keeper) SetCorrectionOutcome(ctx context.Context, outcome *types.CorrectionOutcome) {
	st := k.storeService.OpenKVStore(ctx)
	bz, err := json.Marshal(outcome)
	if err != nil {
		panic("failed to marshal correction outcome: " + err.Error())
	}
	if err := st.Set(types.CorrectionOutcomeKey(outcome.Height, outcome.Dimension), bz); err != nil {
		panic("failed to set correction outcome: " + err.Error())
	}
}

func (k Keeper) GetCorrectionOutcome(ctx context.Context, height uint64, dimension string) (*types.CorrectionOutcome, bool) {
	st := k.storeService.OpenKVStore(ctx)
	bz, err := st.Get(types.CorrectionOutcomeKey(height, dimension))
	if err != nil || bz == nil {
		return nil, false
	}
	var outcome types.CorrectionOutcome
	if err := json.Unmarshal(bz, &outcome); err != nil {
		return nil, false
	}
	return &outcome, true
}

// GetCorrectionsAtHeight returns all correction outcomes recorded at a given height.
func (k Keeper) GetCorrectionsAtHeight(ctx context.Context, height uint64) []*types.CorrectionOutcome {
	st := k.storeService.OpenKVStore(ctx)
	prefix := append(types.CorrectionOutcomeKeyPrefix, types.HeightBytes(height)...)
	iter, err := st.Iterator(prefix, prefixEndBytes(prefix))
	if err != nil {
		return nil
	}
	defer iter.Close()

	var outcomes []*types.CorrectionOutcome
	for ; iter.Valid(); iter.Next() {
		var o types.CorrectionOutcome
		if json.Unmarshal(iter.Value(), &o) == nil {
			outcomes = append(outcomes, &o)
		}
	}
	return outcomes
}

// GetRecentCorrectionOutcomes returns the most recent N evaluated correction outcomes.
func (k Keeper) GetRecentCorrectionOutcomes(ctx context.Context, windowSize uint64) []*types.CorrectionOutcome {
	st := k.storeService.OpenKVStore(ctx)
	iter, err := st.ReverseIterator(types.CorrectionOutcomeKeyPrefix, prefixEndBytes(types.CorrectionOutcomeKeyPrefix))
	if err != nil {
		return nil
	}
	defer iter.Close()

	var outcomes []*types.CorrectionOutcome
	maxIter := 10_000
	count := 0
	for ; iter.Valid() && uint64(len(outcomes)) < windowSize && count < maxIter; iter.Next() {
		count++
		var o types.CorrectionOutcome
		if json.Unmarshal(iter.Value(), &o) == nil {
			if o.ScoreAfter > 0 { // only include evaluated outcomes
				outcomes = append(outcomes, &o)
			}
		}
	}
	return outcomes
}
```

**Step 3: Export HeightBytes for cross-package use**

In `keys.go`, rename `heightBytes` to `HeightBytes` (exported) and update all callers in keys.go to use `HeightBytes`.

Actually, since `GetCorrectionsAtHeight` lives in the keeper package, not the types package, it can't call the unexported `heightBytes`. Instead, the prefix should be constructed in the types package. Add a function:

```go
// CorrectionOutcomeHeightPrefix returns the key prefix for all outcomes at a height.
func CorrectionOutcomeHeightPrefix(height uint64) []byte {
	return append(CorrectionOutcomeKeyPrefix, heightBytes(height)...)
}
```

Then in state.go, use `types.CorrectionOutcomeHeightPrefix(height)` instead of manual prefix construction.

**Step 4: Run tests**

Run: `go test ./x/alignment/... -count=1 -v 2>&1 | tail -5`
Expected: PASS

**Step 5: Commit**

```
feat(alignment): add correction outcome state management (R29-4)
```

---

### Task 4: Implement Correction Confidence Calculation

**Files:**
- Create: `x/alignment/keeper/correction_confidence.go`

**Step 1: Write the test**

Add to `x/alignment/keeper/keeper_test.go`:

```go
func TestCorrectionConfidenceNeutralWithoutData(t *testing.T) {
	k, _, ctx := setupKeeper(t)
	confidence := k.GetCorrectionConfidence(ctx)
	if confidence != 500_000 {
		t.Fatalf("expected neutral confidence 500000, got %d", confidence)
	}
}

func TestCorrectionConfidenceCalculation(t *testing.T) {
	k, _, ctx := setupKeeper(t)

	// Record 10 outcomes: 8 successful, 2 failed.
	for i := uint64(0); i < 10; i++ {
		outcome := &types.CorrectionOutcome{
			Height:      100 + i*100,
			Dimension:   types.DimKnowledgeQuality,
			Magnitude:   100_000,
			Direction:   "increase",
			ScoreBefore: 300_000,
			ScoreAfter:  400_000,
			Successful:  i < 8,
		}
		k.SetCorrectionOutcome(ctx, outcome)
	}

	confidence := k.GetCorrectionConfidence(ctx)
	expected := uint64(8) * types.BPS / 10 // 800,000
	if confidence != expected {
		t.Fatalf("expected confidence %d, got %d", expected, confidence)
	}
}

func TestCorrectionConfidenceMinSamples(t *testing.T) {
	k, _, ctx := setupKeeper(t)

	// Record only 3 outcomes (below min_samples=5).
	for i := uint64(0); i < 3; i++ {
		k.SetCorrectionOutcome(ctx, &types.CorrectionOutcome{
			Height: 100 + i*100, Dimension: types.DimKnowledgeQuality,
			ScoreBefore: 300_000, ScoreAfter: 400_000, Successful: true,
		})
	}

	confidence := k.GetCorrectionConfidence(ctx)
	if confidence != 500_000 {
		t.Fatalf("expected neutral 500000 (below min samples), got %d", confidence)
	}
}
```

**Step 2: Run tests to see them fail**

Run: `go test ./x/alignment/keeper/ -run TestCorrectionConfidence -v 2>&1 | tail -10`
Expected: FAIL (method not defined)

**Step 3: Implement GetCorrectionConfidence**

Create `x/alignment/keeper/correction_confidence.go`:

```go
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

// getEffectiveMaxMagnitude returns the dynamic max auto-apply magnitude based on correction confidence.
func (k Keeper) GetEffectiveMaxMagnitude(ctx context.Context) uint64 {
	params := k.GetParams(ctx)
	baseMax := params.MaxAutoApplyMagnitudeBps
	if baseMax == 0 {
		return 0 // auto-apply disabled
	}

	confidence := k.GetCorrectionConfidence(ctx)

	if params.MinConfidenceForAutoApply > 0 && confidence < params.MinConfidenceForAutoApply {
		return 0 // system has proven it can't self-correct
	}

	minMul := params.CorrectionBoundsMinMultiplierBps
	maxMul := params.CorrectionBoundsMaxMultiplierBps
	if minMul == 0 || maxMul == 0 || maxMul <= minMul {
		return baseMax // no modulation configured
	}

	// Linear scaling: confidence maps to [minMul, maxMul]
	multiplier := minMul + (confidence * (maxMul - minMul) / types.BPS)

	return baseMax * multiplier / types.BPS
}

// GetEffectiveObservationInterval returns the observation interval modulated by correction confidence.
// Health-based overrides (DegradedFrequencyActive) are applied separately in EndBlock.
func (k Keeper) GetEffectiveObservationInterval(ctx context.Context) uint64 {
	params := k.GetParams(ctx)
	baseInterval := params.ObservationIntervalBlocks

	confidence := k.GetCorrectionConfidence(ctx)

	if confidence > 800_000 {
		return baseInterval * 3 / 2 // 150% (less frequent)
	} else if confidence < 300_000 {
		return baseInterval * 2 / 3 // 67% (more frequent)
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
```

**Step 4: Run tests to verify pass**

Run: `go test ./x/alignment/keeper/ -run TestCorrectionConfidence -v 2>&1 | tail -10`
Expected: PASS

**Step 5: Commit**

```
feat(alignment): implement correction confidence calculation and dynamic bounds (R29-4)
```

---

### Task 5: Add Effective Max Magnitude and Observation Interval Tests

**Files:**
- Modify: `x/alignment/keeper/keeper_test.go` (add tests)

**Step 1: Write tests for dynamic bounds**

```go
func TestEffectiveMaxMagnitudeHighConfidence(t *testing.T) {
	k, _, ctx := setupKeeper(t)

	// Record 10 successful outcomes.
	for i := uint64(0); i < 10; i++ {
		k.SetCorrectionOutcome(ctx, &types.CorrectionOutcome{
			Height: 100 + i*100, Dimension: types.DimKnowledgeQuality,
			ScoreBefore: 300_000, ScoreAfter: 400_000, Successful: true,
		})
	}

	effectiveMax := k.GetEffectiveMaxMagnitude(ctx)
	params := k.GetParams(ctx)
	baseMax := params.MaxAutoApplyMagnitudeBps

	if effectiveMax <= baseMax {
		t.Fatalf("expected effective max > base max with high confidence, got %d <= %d", effectiveMax, baseMax)
	}
}

func TestEffectiveMaxMagnitudeLowConfidence(t *testing.T) {
	k, _, ctx := setupKeeper(t)

	// Record 10 outcomes: all failed.
	for i := uint64(0); i < 10; i++ {
		k.SetCorrectionOutcome(ctx, &types.CorrectionOutcome{
			Height: 100 + i*100, Dimension: types.DimKnowledgeQuality,
			ScoreBefore: 300_000, ScoreAfter: 200_000, Successful: false,
		})
	}

	effectiveMax := k.GetEffectiveMaxMagnitude(ctx)
	// 0% confidence < 20% MinConfidenceForAutoApply → governance only
	if effectiveMax != 0 {
		t.Fatalf("expected effective max = 0 (governance only) with 0%% confidence, got %d", effectiveMax)
	}
}

func TestEffectiveObservationIntervalHighConfidence(t *testing.T) {
	k, _, ctx := setupKeeper(t)

	for i := uint64(0); i < 10; i++ {
		k.SetCorrectionOutcome(ctx, &types.CorrectionOutcome{
			Height: 100 + i*100, Dimension: types.DimKnowledgeQuality,
			ScoreBefore: 300_000, ScoreAfter: 400_000, Successful: true,
		})
	}

	interval := k.GetEffectiveObservationInterval(ctx)
	params := k.GetParams(ctx)
	expected := params.ObservationIntervalBlocks * 3 / 2
	if interval != expected {
		t.Fatalf("expected interval %d (150%%), got %d", expected, interval)
	}
}

func TestEffectiveObservationIntervalLowConfidence(t *testing.T) {
	k, _, ctx := setupKeeper(t)

	for i := uint64(0); i < 10; i++ {
		k.SetCorrectionOutcome(ctx, &types.CorrectionOutcome{
			Height: 100 + i*100, Dimension: types.DimKnowledgeQuality,
			ScoreBefore: 300_000, ScoreAfter: 200_000, Successful: false,
		})
	}

	interval := k.GetEffectiveObservationInterval(ctx)
	params := k.GetParams(ctx)
	expected := params.ObservationIntervalBlocks * 2 / 3
	if interval != expected {
		t.Fatalf("expected interval %d (67%%), got %d", expected, interval)
	}
}
```

**Step 2: Run tests**

Run: `go test ./x/alignment/keeper/ -run TestEffective -v 2>&1 | tail -10`
Expected: PASS

**Step 3: Commit**

```
test(alignment): add correction confidence bounds and interval tests (R29-4)
```

---

### Task 6: Wire Outcome Recording into ApplyCorrections

**Files:**
- Modify: `x/alignment/keeper/corrections.go:98-146`

**Step 1: Write test for outcome recording**

Add to `keeper_test.go`:

```go
func TestApplyCorrectionsRecordsOutcomes(t *testing.T) {
	k, mocks, ctx := setupKeeper(t)

	autoMock := &mockAutopoiesisKeeper{}
	mocks.autopoiesis = autoMock
	k.SetAutopoiesisKeeper(autoMock)

	// Set scores so we know pre-correction state.
	k.SetScores(ctx, &types.DimensionScores{
		Height:           100,
		KnowledgeQuality: 300_000,
	})

	corrections := []*types.CorrectionRecord{{
		Height:    100,
		Dimension: types.DimKnowledgeQuality,
		Parameter: "knowledge.reward_multiplier",
		Direction: "increase",
		Magnitude: 100_000,
		Timestamp: 1000,
	}}

	k.ApplyCorrections(ctx, corrections)

	// Verify outcome was recorded.
	outcome, found := k.GetCorrectionOutcome(ctx, 100, types.DimKnowledgeQuality)
	if !found {
		t.Fatal("expected correction outcome to be recorded")
	}
	if outcome.ScoreBefore != 300_000 {
		t.Fatalf("expected score_before=300000, got %d", outcome.ScoreBefore)
	}
	if outcome.ScoreAfter != 0 {
		t.Fatal("expected score_after=0 (not yet evaluated)")
	}
}
```

**Step 2: Run test to see it fail**

Run: `go test ./x/alignment/keeper/ -run TestApplyCorrectionsRecordsOutcomes -v 2>&1 | tail -10`
Expected: FAIL

**Step 3: Modify ApplyCorrections**

In `corrections.go`, update `ApplyCorrections` to record outcomes. Before the existing `for` loop, get current scores. Inside the loop, after storing the correction, also store an outcome:

```go
func (k Keeper) ApplyCorrections(ctx context.Context, corrections []*types.CorrectionRecord) {
	params := k.GetParams(ctx)
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	height := uint64(sdkCtx.BlockHeight())

	// Get current scores for outcome tracking.
	currentScores, _ := k.GetScores(ctx, height)

	// Use dynamic effective max magnitude instead of static param.
	effectiveMax := k.GetEffectiveMaxMagnitude(ctx)

	for _, c := range corrections {
		// Record pre-correction outcome for tracking (R29-4).
		if currentScores != nil {
			outcome := &types.CorrectionOutcome{
				Height:      height,
				Dimension:   c.Dimension,
				Magnitude:   c.Magnitude,
				Direction:   c.Direction,
				ScoreBefore: types.GetDimensionScore(currentScores, c.Dimension),
			}
			k.SetCorrectionOutcome(ctx, outcome)
		}

		// Check magnitude bounds (dynamic via correction confidence).
		if effectiveMax > 0 && c.Magnitude > effectiveMax {
			// ... existing governance-required logic, but use effectiveMax ...
		} else if effectiveMax == 0 && params.MaxAutoApplyMagnitudeBps > 0 {
			// Confidence too low — all corrections require governance.
			// ... emit governance-required event ...
		}

		// ... rest of existing auto-apply logic ...
	}
}
```

The key changes:
1. Replace `params.MaxAutoApplyMagnitudeBps` check with `effectiveMax` from `GetEffectiveMaxMagnitude`
2. Add outcome recording before the bounds check
3. Handle `effectiveMax == 0` (governance lockout due to low confidence)

**Step 4: Run all tests**

Run: `go test ./x/alignment/... -count=1 -v 2>&1 | tail -10`
Expected: PASS (existing tests should still pass — effectiveMax returns same as base when no outcomes exist)

**Step 5: Commit**

```
feat(alignment): wire outcome recording into ApplyCorrections with dynamic bounds (R29-4)
```

---

### Task 7: Wire Outcome Evaluation into EndBlocker

**Files:**
- Modify: `x/alignment/module.go:131-231`
- Modify: `x/alignment/keeper/correction_confidence.go` (add EvaluatePendingCorrections)

**Step 1: Add EvaluatePendingCorrections**

In `correction_confidence.go`, add:

```go
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
	}
}

func absDistance(a, b uint64) uint64 {
	if a > b {
		return a - b
	}
	return b - a
}
```

**Step 2: Wire into EndBlocker**

In `module.go`, after step 1 (Observe) and step 2 (Score), add evaluation before step 3 (Corrections):

```go
// 2.5 Evaluate pending correction outcomes from previous observation (R29-4)
am.keeper.EvaluatePendingCorrections(ctx, scores)
```

Also modify the effective interval calculation to use confidence-modulated interval:

```go
// Replace:
effectiveInterval := params.ObservationIntervalBlocks
// With:
effectiveInterval := am.keeper.GetEffectiveObservationInterval(ctx)
```

Keep the degraded frequency override (halving) applied on top.

**Step 3: Add confidence event emission**

After step 4 (Health index), emit the confidence event:

```go
// 4.5 Emit correction confidence event (R29-4)
confidence := am.keeper.GetCorrectionConfidence(ctx)
effectiveMax := am.keeper.GetEffectiveMaxMagnitude(ctx)
sdkCtx.EventManager().EmitEvent(
	sdk.NewEvent("zerone.alignment.correction_confidence_updated",
		sdk.NewAttribute("confidence_bps", fmt.Sprintf("%d", confidence)),
		sdk.NewAttribute("effective_max_magnitude", fmt.Sprintf("%d", effectiveMax)),
		sdk.NewAttribute("category", keeper.CategorizeConfidence(confidence)),
	),
)
```

**Step 4: Run all tests**

Run: `go test ./x/alignment/... -count=1 2>&1 | tail -5`
Expected: PASS

**Step 5: Commit**

```
feat(alignment): wire outcome evaluation and confidence events into EndBlocker (R29-4)
```

---

### Task 8: Add Pruning

**Files:**
- Modify: `x/alignment/keeper/correction_confidence.go`
- Modify: `x/alignment/module.go` (EndBlocker)

**Step 1: Add pruning method**

In `correction_confidence.go`:

```go
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
	endKey := append(types.CorrectionOutcomeKeyPrefix, types.HeightBytes(cutoffHeight)...)
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
```

**Step 2: Wire into EndBlocker**

After the confidence event in EndBlocker, add:

```go
// 4.6 Prune old correction outcomes (R29-4)
am.keeper.PruneOldOutcomes(ctx)
```

**Step 3: Run tests**

Run: `go test ./x/alignment/... -count=1 2>&1 | tail -5`
Expected: PASS

**Step 4: Commit**

```
feat(alignment): add correction outcome pruning (R29-4)
```

---

### Task 9: Add CorrectionConfidence gRPC Query

**Files:**
- Modify: `x/alignment/types/query_grpc.pb.go` (QueryServer interface, handler, service desc)
- Modify: `x/alignment/keeper/grpc_query.go` (implementation)

**Step 1: Extend QueryServer interface**

In `query_grpc.pb.go`:

1. Add constant (after `Query_HealthHistory_FullMethodName`):
```go
Query_CorrectionConfidence_FullMethodName = "/zerone.alignment.v1.Query/CorrectionConfidence"
```

2. Add to `QueryClient` interface:
```go
CorrectionConfidence(ctx context.Context, in *QueryCorrectionConfidenceRequest, opts ...grpc.CallOption) (*QueryCorrectionConfidenceResponse, error)
```

3. Add client method:
```go
func (c *queryClient) CorrectionConfidence(ctx context.Context, in *QueryCorrectionConfidenceRequest, opts ...grpc.CallOption) (*QueryCorrectionConfidenceResponse, error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	out := new(QueryCorrectionConfidenceResponse)
	err := c.cc.Invoke(ctx, Query_CorrectionConfidence_FullMethodName, in, out, cOpts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}
```

4. Add to `QueryServer` interface:
```go
CorrectionConfidence(context.Context, *QueryCorrectionConfidenceRequest) (*QueryCorrectionConfidenceResponse, error)
```

5. Add Unimplemented stub:
```go
func (UnimplementedQueryServer) CorrectionConfidence(context.Context, *QueryCorrectionConfidenceRequest) (*QueryCorrectionConfidenceResponse, error) {
	return nil, status.Error(codes.Unimplemented, "method CorrectionConfidence not implemented")
}
```

6. Add handler:
```go
func _Query_CorrectionConfidence_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(QueryCorrectionConfidenceRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(QueryServer).CorrectionConfidence(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: Query_CorrectionConfidence_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(QueryServer).CorrectionConfidence(ctx, req.(*QueryCorrectionConfidenceRequest))
	}
	return interceptor(ctx, in, info, handler)
}
```

7. Add to `Query_ServiceDesc.Methods`:
```go
{
	MethodName: "CorrectionConfidence",
	Handler:    _Query_CorrectionConfidence_Handler,
},
```

**Step 2: Implement query in grpc_query.go**

```go
func (q queryServer) CorrectionConfidence(ctx context.Context, req *types.QueryCorrectionConfidenceRequest) (*types.QueryCorrectionConfidenceResponse, error) {
	confidence := q.Keeper.GetCorrectionConfidence(ctx)
	effectiveMax := q.Keeper.GetEffectiveMaxMagnitude(ctx)
	effectiveInterval := q.Keeper.GetEffectiveObservationInterval(ctx)

	params := q.Keeper.GetParams(ctx)
	windowSize := params.CorrectionConfidenceWindowSize
	if windowSize == 0 {
		windowSize = 50
	}
	outcomes := q.Keeper.GetRecentCorrectionOutcomes(ctx, windowSize)

	total := uint64(len(outcomes))
	successful := uint64(0)
	for _, o := range outcomes {
		if o.Successful {
			successful++
		}
	}

	// Cap recent outcomes for response to 20.
	recentCap := 20
	if len(outcomes) > recentCap {
		outcomes = outcomes[:recentCap]
	}

	return &types.QueryCorrectionConfidenceResponse{
		ConfidenceBps:                confidence,
		TotalCorrections:             total,
		SuccessfulCorrections:        successful,
		EffectiveMaxMagnitude:        effectiveMax,
		EffectiveObservationInterval: effectiveInterval,
		RecentOutcomes:               outcomes,
	}, nil
}
```

**Step 3: Run tests**

Run: `go test ./x/alignment/... -count=1 2>&1 | tail -5`
Expected: PASS

**Step 4: Commit**

```
feat(alignment): add CorrectionConfidence gRPC query (R29-4)
```

---

### Task 10: Add CLI Command

**Files:**
- Modify: `x/alignment/client/cli/query.go`

**Step 1: Add CLI command**

In `query.go`, add `NewQueryCorrectionConfidenceCmd()` to `queryCmd.AddCommand(...)`, then add the function:

```go
func NewQueryCorrectionConfidenceCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "correction-confidence",
		Short: "Query correction confidence score and effective bounds",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientQueryContext(cmd)
			if err != nil {
				return err
			}
			req := &types.QueryCorrectionConfidenceRequest{}
			resp := &types.QueryCorrectionConfidenceResponse{}
			if err := clientCtx.Invoke(cmd.Context(), "/zerone.alignment.v1.Query/CorrectionConfidence", req, resp); err != nil {
				return fmt.Errorf("failed to query correction confidence: %w", err)
			}
			return clientCtx.PrintObjectLegacy(resp)
		},
	}
	flags.AddQueryFlagsToCmd(cmd)
	return cmd
}
```

**Step 2: Run build**

Run: `go build ./cmd/zeroned/ 2>&1 | tail -10`
Expected: Build succeeds

**Step 3: Commit**

```
feat(alignment): add correction-confidence CLI query command (R29-4)
```

---

### Task 11: Write Integration Test

**Files:**
- Modify: `x/alignment/keeper/keeper_test.go` (add integration test)

**Step 1: Write full lifecycle integration test**

```go
func TestCorrectionConfidenceFullLifecycle(t *testing.T) {
	k, mocks, ctx := setupKeeper(t)

	autoMock := &mockAutopoiesisKeeper{}
	k.SetAutopoiesisKeeper(autoMock)

	am := alignment.NewAppModule(nil, k)

	// --- Phase 1: Boot — neutral confidence, base bounds ---
	confidence := k.GetCorrectionConfidence(ctx)
	if confidence != 500_000 {
		t.Fatalf("expected neutral confidence at boot, got %d", confidence)
	}

	params := k.GetParams(ctx)
	effectiveMax := k.GetEffectiveMaxMagnitude(ctx)
	if effectiveMax != params.MaxAutoApplyMagnitudeBps {
		t.Fatalf("expected base max at boot, got %d", effectiveMax)
	}

	// --- Phase 2: Run observations with degraded dimensions to generate corrections ---
	mocks.knowledge.verificationRate = 300_000  // below degraded (400k)
	mocks.knowledge.consensusDiversity = 300_000
	mocks.staking.totalStaked = big.NewInt(400_000_000_000)
	mocks.staking.activeValidators = 50
	mocks.staking.targetValidators = 111
	mocks.ontology.domainCount = 30
	mocks.vestingRewards.totalSupply = big.NewInt(1_000_000_000_000)

	// Observation at block 100: generates corrections, records outcomes.
	ctx = ctx.WithBlockHeight(100)
	if err := am.EndBlock(ctx); err != nil {
		t.Fatalf("EndBlock at 100 failed: %v", err)
	}

	// --- Phase 3: Improve dimensions — corrections "succeed" ---
	mocks.knowledge.verificationRate = 800_000
	mocks.knowledge.consensusDiversity = 700_000
	mocks.staking.totalStaked = big.NewInt(800_000_000_000)
	mocks.staking.activeValidators = 100
	mocks.ontology.domainCount = 80

	// Observation at block 200: evaluates previous outcomes.
	ctx = ctx.WithBlockHeight(200)
	if err := am.EndBlock(ctx); err != nil {
		t.Fatalf("EndBlock at 200 failed: %v", err)
	}

	// Check that outcomes were evaluated.
	outcomes := k.GetCorrectionsAtHeight(ctx, 100)
	evaluatedCount := 0
	successCount := 0
	for _, o := range outcomes {
		if o.ScoreAfter > 0 {
			evaluatedCount++
			if o.Successful {
				successCount++
			}
		}
	}
	if evaluatedCount == 0 {
		t.Fatal("expected at least one evaluated outcome")
	}

	// Query the confidence.
	qs := keeper.NewQueryServerImpl(k)
	resp, err := qs.CorrectionConfidence(ctx, &types.QueryCorrectionConfidenceRequest{})
	if err != nil {
		t.Fatalf("CorrectionConfidence query failed: %v", err)
	}
	t.Logf("Confidence: %d BPS, Total: %d, Successful: %d, EffectiveMax: %d",
		resp.ConfidenceBps, resp.TotalCorrections, resp.SuccessfulCorrections, resp.EffectiveMaxMagnitude)
}
```

**Step 2: Run the integration test**

Run: `go test ./x/alignment/keeper/ -run TestCorrectionConfidenceFullLifecycle -v 2>&1 | tail -20`
Expected: PASS

**Step 3: Commit**

```
test(alignment): add correction confidence full lifecycle integration test (R29-4)
```

---

### Task 12: Add Outcome Recording Event

**Files:**
- Modify: `x/alignment/keeper/correction_confidence.go` (EvaluatePendingCorrections)

**Step 1: Emit correction_outcome_recorded events**

In `EvaluatePendingCorrections`, after setting the outcome, emit an event:

```go
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
```

**Step 2: Run all tests**

Run: `go test ./x/alignment/... -count=1 2>&1 | tail -5`
Expected: PASS

**Step 3: Commit**

```
feat(alignment): emit correction_outcome_recorded events (R29-4)
```

---

### Task 13: Final Verification

**Step 1: Run full test suite**

Run: `go test ./x/alignment/... -count=1 -v 2>&1 | tail -30`
Expected: All tests PASS

**Step 2: Build binary**

Run: `go build ./cmd/zeroned/ 2>&1`
Expected: Clean build

**Step 3: Run broader test suite to check for regressions**

Run: `go test ./x/... -count=1 2>&1 | tail -20`
Expected: No regressions

**Step 4: Commit with final message**

```
feat(alignment): complete R29-4 correction confidence implementation
```

---

## Summary of Files Changed

| File | Action | Purpose |
|------|--------|---------|
| `x/alignment/types/correction_confidence.go` | Create | CorrectionOutcome type, query types, GetDimensionScore helper |
| `x/alignment/types/keys.go` | Modify | Add CorrectionOutcomeKeyPrefix (0x08) and key functions |
| `x/alignment/types/genesis.pb.go` | Modify | Add 5 confidence params to Params struct + getters |
| `x/alignment/types/genesis.go` | Modify | Default values + validation for new params |
| `x/alignment/types/errors.go` | Modify | Add ErrInvalidConfidenceBounds |
| `x/alignment/keeper/correction_confidence.go` | Create | Confidence calc, dynamic bounds, outcome eval, pruning |
| `x/alignment/keeper/corrections.go` | Modify | Record outcomes + use dynamic effective max |
| `x/alignment/keeper/state.go` | Modify | Outcome CRUD methods |
| `x/alignment/keeper/grpc_query.go` | Modify | CorrectionConfidence query implementation |
| `x/alignment/types/query_grpc.pb.go` | Modify | gRPC service extension |
| `x/alignment/client/cli/query.go` | Modify | CLI command |
| `x/alignment/module.go` | Modify | EndBlocker: evaluate outcomes, confidence interval, events, pruning |
| `x/alignment/keeper/keeper_test.go` | Modify | All new tests |
