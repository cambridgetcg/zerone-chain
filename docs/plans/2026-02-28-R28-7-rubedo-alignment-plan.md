# R28-7 Rubedo Alignment Activation — Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Activate the alignment module's feedback loop — real sensor data, bounded corrections, health transition responses.

**Architecture:** The alignment module already has sensors, scoring, corrections, and health categorization wired end-to-end. We add: (1) real data from knowledge adapter stubs, (2) magnitude bounds on correction application, (3) health transition responses with degraded-frequency doubling, (4) a health history query.

**Tech Stack:** Go 1.24+, Cosmos SDK v0.50.15, proto-generated types with JSON serialization.

---

### Task 1: Add MaxAutoApplyMagnitudeBps to Params

**Files:**
- Modify: `proto/zerone/alignment/v1/genesis.proto`
- Modify: `x/alignment/types/genesis.pb.go`
- Modify: `x/alignment/types/genesis.go`
- Test: `x/alignment/keeper/keeper_test.go`

**Step 1: Update proto definition**

In `proto/zerone/alignment/v1/genesis.proto`, add field 11 to `Params`:

```proto
  // Maximum correction magnitude (BPS) for auto-apply. Above this requires governance.
  uint64 max_auto_apply_magnitude_bps = 11;
```

**Step 2: Update generated Go struct**

In `x/alignment/types/genesis.pb.go`, add the field to the `Params` struct (after `Enabled`):

```go
  // Maximum correction magnitude for auto-apply (BPS).
  MaxAutoApplyMagnitudeBps uint64 `protobuf:"varint,11,opt,name=max_auto_apply_magnitude_bps,json=maxAutoApplyMagnitudeBps,proto3" json:"max_auto_apply_magnitude_bps,omitempty"`
```

Add getter method:

```go
func (x *Params) GetMaxAutoApplyMagnitudeBps() uint64 {
	if x != nil {
		return x.MaxAutoApplyMagnitudeBps
	}
	return 0
}
```

**Step 3: Update DefaultParams**

In `x/alignment/types/genesis.go`, add to `DefaultParams()`:

```go
MaxAutoApplyMagnitudeBps: 500_000, // 50% — conservative testnet default
```

Note: 500_000 BPS = 50% of the BPS range. This is a generous bound for testnet so corrections actually fire. Production would use a much lower value like 50_000 (5%).

**Step 4: Write the failing test**

In `x/alignment/keeper/keeper_test.go`, add to `TestParamValidation`:

```go
{
    name: "max_auto_apply exceeds BPS",
    modify: func(p *types.Params) {
        p.MaxAutoApplyMagnitudeBps = types.BPS + 1
    },
    errMsg: "max_auto_apply",
},
```

**Step 5: Run test to verify it fails**

Run: `go test ./x/alignment/keeper/ -run TestParamValidation -v`
Expected: FAIL — no validation for MaxAutoApplyMagnitudeBps yet.

**Step 6: Add validation**

In `x/alignment/types/genesis.go`, add to `Validate()` before `return nil`:

```go
if p.MaxAutoApplyMagnitudeBps > BPS {
    return ErrInvalidMaxAutoApply
}
```

In `x/alignment/types/errors.go`, add:

```go
var ErrInvalidMaxAutoApply = fmt.Errorf("max_auto_apply_magnitude_bps exceeds BPS")
```

**Step 7: Run test to verify it passes**

Run: `go test ./x/alignment/keeper/ -run TestParamValidation -v`
Expected: PASS

**Step 8: Commit**

```bash
git add x/alignment/types/ proto/zerone/alignment/v1/genesis.proto
git commit -m "feat(alignment): add MaxAutoApplyMagnitudeBps param"
```

---

### Task 2: Add DegradedFrequencyActive and PreviousCategory to AlignmentState

**Files:**
- Modify: `proto/zerone/alignment/v1/types.proto`
- Modify: `x/alignment/types/types.pb.go`
- Test: `x/alignment/keeper/alignment_extended_test.go`

**Step 1: Update proto definition**

In `proto/zerone/alignment/v1/types.proto`, add fields 4-5 to `AlignmentState`:

```proto
  // Whether degraded-frequency mode is active (2x observation rate).
  bool degraded_frequency_active = 4;
  // Previous health category for transition detection.
  string previous_category = 5;
```

**Step 2: Update generated Go struct**

In `x/alignment/types/types.pb.go`, add to `AlignmentState` struct (after `ObservationCount`):

```go
  DegradedFrequencyActive bool   `protobuf:"varint,4,opt,name=degraded_frequency_active,json=degradedFrequencyActive,proto3" json:"degraded_frequency_active,omitempty"`
  PreviousCategory        string `protobuf:"bytes,5,opt,name=previous_category,json=previousCategory,proto3" json:"previous_category,omitempty"`
```

Add getter methods:

```go
func (x *AlignmentState) GetDegradedFrequencyActive() bool {
	if x != nil {
		return x.DegradedFrequencyActive
	}
	return false
}

func (x *AlignmentState) GetPreviousCategory() string {
	if x != nil {
		return x.PreviousCategory
	}
	return ""
}
```

**Step 3: Write the failing test**

In `x/alignment/keeper/alignment_extended_test.go`, add:

```go
func TestStateRoundtripWithNewFields(t *testing.T) {
	k, _, ctx := setupKeeper(t)

	state := &types.AlignmentState{
		Enabled:                 true,
		LastObservationHeight:   500,
		ObservationCount:        42,
		DegradedFrequencyActive: true,
		PreviousCategory:        types.CategoryDegraded,
	}
	k.SetState(ctx, state)

	got := k.GetState(ctx)
	if !got.DegradedFrequencyActive {
		t.Error("expected DegradedFrequencyActive=true")
	}
	if got.PreviousCategory != types.CategoryDegraded {
		t.Errorf("expected PreviousCategory=%s, got %s", types.CategoryDegraded, got.PreviousCategory)
	}
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./x/alignment/keeper/ -run TestStateRoundtripWithNewFields -v`
Expected: PASS (JSON serialization handles new fields automatically).

**Step 5: Commit**

```bash
git add x/alignment/types/ proto/zerone/alignment/v1/types.proto
git commit -m "feat(alignment): add DegradedFrequencyActive and PreviousCategory to state"
```

---

### Task 3: Fix Knowledge Adapter GetVerificationRate

**Files:**
- Modify: `x/knowledge/keeper/alignment_adapters.go`
- Test: `x/alignment/keeper/keeper_test.go`

**Step 1: Write the failing test**

In `x/alignment/keeper/keeper_test.go`, add:

```go
func TestSensorKnowledgeReturnsRealValues(t *testing.T) {
	k, mocks, ctx := setupKeeper(t)

	// With mock returning specific values, sensors should use them.
	mocks.knowledge.verificationRate = 800_000
	mocks.knowledge.consensusDiversity = 600_000

	obs := k.ObserveAll(ctx)

	// Knowledge quality = (800k*6 + 600k*4) / 10 = (4.8M + 2.4M) / 10 = 720k
	expected := (uint64(800_000)*6 + uint64(600_000)*4) / 10
	if obs.KnowledgeQuality != expected {
		t.Errorf("expected knowledge=%d, got %d", expected, obs.KnowledgeQuality)
	}
}
```

**Step 2: Run test to verify it passes**

Run: `go test ./x/alignment/keeper/ -run TestSensorKnowledgeReturnsRealValues -v`
Expected: PASS (mocks already work; this just confirms the formula).

**Step 3: Implement real GetVerificationRate in knowledge adapter**

In `x/knowledge/keeper/alignment_adapters.go`, replace the stub:

```go
// GetVerificationRate computes accepted / terminal claims in BPS.
func (a *AlignmentKnowledgeAdapter) GetVerificationRate(ctx context.Context) uint64 {
	var accepted, terminal uint64
	a.k.IterateClaims(ctx, func(claim *types.Claim) bool {
		switch claim.Status {
		case types.ClaimStatus_CLAIM_STATUS_ACCEPTED:
			accepted++
			terminal++
		case types.ClaimStatus_CLAIM_STATUS_REJECTED,
			types.ClaimStatus_CLAIM_STATUS_MALFORMED,
			types.ClaimStatus_CLAIM_STATUS_INSUFFICIENT:
			terminal++
		}
		return false
	})
	if terminal == 0 {
		return 500_000 // NeutralBPS — no data yet
	}
	rate := accepted * 1_000_000 / terminal
	if rate > 1_000_000 {
		return 1_000_000
	}
	return rate
}
```

Also fix `GetTotalFacts`:

```go
// GetTotalFacts counts all accepted claims (facts).
func (a *AlignmentKnowledgeAdapter) GetTotalFacts(ctx context.Context) uint64 {
	var count uint64
	a.k.IterateClaims(ctx, func(claim *types.Claim) bool {
		if claim.Status == types.ClaimStatus_CLAIM_STATUS_ACCEPTED {
			count++
		}
		return false
	})
	return count
}
```

**Step 4: Run alignment tests to verify no regressions**

Run: `go test ./x/alignment/keeper/ -v`
Expected: PASS (all existing tests use mocks, unaffected by adapter changes).

**Step 5: Compile check**

Run: `go build ./x/knowledge/...`
Expected: BUILD SUCCESS

**Step 6: Commit**

```bash
git add x/knowledge/keeper/alignment_adapters.go
git commit -m "feat(knowledge): implement real GetVerificationRate and GetTotalFacts"
```

---

### Task 4: Implement Bounded Correction Application

**Files:**
- Modify: `x/alignment/keeper/corrections.go`
- Test: `x/alignment/keeper/keeper_test.go`

**Step 1: Write the failing test — small correction auto-applied**

In `x/alignment/keeper/keeper_test.go`, add:

```go
func TestBoundedCorrectionSmallAutoApplied(t *testing.T) {
	k, mocks, ctx := setupKeeper(t)

	// Set MaxAutoApplyMagnitudeBps to 500_000 (default).
	// Wire autopoiesis.
	autoMock := &mockAutopoiesisKeeper{}
	mocks.autopoiesis = autoMock
	k.SetAutopoiesisKeeper(autoMock)

	// Create a correction with small magnitude (below bounds).
	corrections := []*types.CorrectionRecord{{
		Height:    100,
		Dimension: types.DimKnowledgeQuality,
		Parameter: "knowledge.reward_multiplier",
		Direction: "increase",
		Magnitude: 100_000, // 10% — below 50% max
		Timestamp: 1000,
	}}

	k.ApplyCorrections(ctx, corrections)

	// Should be auto-applied.
	if len(autoMock.adjustments) != 1 {
		t.Fatalf("expected 1 adjustment, got %d", len(autoMock.adjustments))
	}
	stored, _ := k.GetCorrections(ctx, 100, 0)
	if len(stored) == 0 || !stored[0].Applied {
		t.Fatal("expected correction marked as applied")
	}
}
```

**Step 2: Write the failing test — large correction blocked**

```go
func TestBoundedCorrectionLargeBlocked(t *testing.T) {
	k, mocks, ctx := setupKeeper(t)

	// Set a low max bound.
	params := k.GetParams(ctx)
	params.MaxAutoApplyMagnitudeBps = 50_000 // 5%
	k.SetParams(ctx, params)

	// Wire autopoiesis.
	autoMock := &mockAutopoiesisKeeper{}
	mocks.autopoiesis = autoMock
	k.SetAutopoiesisKeeper(autoMock)

	// Create correction with large magnitude (above bounds).
	corrections := []*types.CorrectionRecord{{
		Height:    100,
		Dimension: types.DimKnowledgeQuality,
		Parameter: "knowledge.reward_multiplier",
		Direction: "increase",
		Magnitude: 200_000, // 20% — exceeds 5% max
		Timestamp: 1000,
	}}

	k.ApplyCorrections(ctx, corrections)

	// Should NOT be auto-applied.
	if len(autoMock.adjustments) != 0 {
		t.Fatalf("expected 0 adjustments for large correction, got %d", len(autoMock.adjustments))
	}
	stored, _ := k.GetCorrections(ctx, 100, 0)
	if len(stored) == 0 {
		t.Fatal("expected correction stored")
	}
	if stored[0].Applied {
		t.Fatal("expected correction NOT applied (magnitude exceeds bounds)")
	}
}
```

**Step 3: Run tests to verify they fail**

Run: `go test ./x/alignment/keeper/ -run TestBoundedCorrection -v`
Expected: FAIL — `TestBoundedCorrectionLargeBlocked` fails because current code applies all corrections.

**Step 4: Implement bounded correction application**

In `x/alignment/keeper/corrections.go`, modify `ApplyCorrections`:

```go
func (k Keeper) ApplyCorrections(ctx context.Context, corrections []*types.CorrectionRecord) {
	params := k.GetParams(ctx)
	sdkCtx := sdk.UnwrapSDKContext(ctx)

	for _, c := range corrections {
		// Check magnitude bounds.
		if params.MaxAutoApplyMagnitudeBps > 0 && c.Magnitude > params.MaxAutoApplyMagnitudeBps {
			k.Logger(ctx).Info("correction exceeds auto-apply bounds, requires governance",
				"dimension", c.Dimension,
				"parameter", c.Parameter,
				"magnitude", c.Magnitude,
				"max_auto_apply", params.MaxAutoApplyMagnitudeBps,
			)
			sdkCtx.EventManager().EmitEvent(
				sdk.NewEvent("zerone.alignment.correction_governance_required",
					sdk.NewAttribute("dimension", c.Dimension),
					sdk.NewAttribute("parameter", c.Parameter),
					sdk.NewAttribute("direction", c.Direction),
					sdk.NewAttribute("magnitude", fmt.Sprintf("%d", c.Magnitude)),
					sdk.NewAttribute("max_auto_apply", fmt.Sprintf("%d", params.MaxAutoApplyMagnitudeBps)),
				),
			)
			c.Applied = false
			k.AddCorrection(ctx, c)
			continue
		}

		if k.autopoiesisKeeper != nil {
			err := k.autopoiesisKeeper.SuggestAdjustment(ctx, c.Parameter, c.Direction, c.Magnitude)
			if err == nil {
				c.Applied = true
			} else {
				k.Logger(ctx).Error("failed to apply correction",
					"dimension", c.Dimension,
					"parameter", c.Parameter,
					"error", err,
				)
			}
		} else {
			k.Logger(ctx).Info("correction logged (autopoiesis not wired)",
				"dimension", c.Dimension,
				"parameter", c.Parameter,
				"direction", c.Direction,
				"magnitude", c.Magnitude,
			)
		}
		k.AddCorrection(ctx, c)
	}
}
```

Note: The `fmt` import is needed — add `"fmt"` to the imports in `corrections.go`.

**Step 5: Run tests to verify they pass**

Run: `go test ./x/alignment/keeper/ -run TestBoundedCorrection -v`
Expected: PASS

**Step 6: Run all existing tests for regressions**

Run: `go test ./x/alignment/keeper/ -v`
Expected: PASS — existing tests use default MaxAutoApplyMagnitudeBps=500_000 which is generous enough that all test corrections (max magnitude ~600k for critical doubled) may exceed it. If `TestCorrectionsAppliedWithAutopoiesis` fails because critical corrections exceed 500k, that's actually correct behavior. Adjust the test by setting `params.MaxAutoApplyMagnitudeBps = types.BPS` (disable bounds) in that test.

**Step 7: Commit**

```bash
git add x/alignment/keeper/corrections.go x/alignment/keeper/keeper_test.go
git commit -m "feat(alignment): implement bounded correction application"
```

---

### Task 5: Implement Health Transition Responses

**Files:**
- Modify: `x/alignment/module.go`
- Test: `x/alignment/keeper/keeper_test.go`

**Step 1: Write the failing test — degraded frequency doubling**

In `x/alignment/keeper/keeper_test.go`, add:

```go
func TestHealthTransitionDegradedDoubleFrequency(t *testing.T) {
	k, mocks, ctx := setupKeeper(t)

	// Set initial state: healthy, no frequency override.
	k.SetState(ctx, &types.AlignmentState{
		Enabled:          true,
		PreviousCategory: types.CategoryHealthy,
	})

	// Set dimensions to produce degraded health (composite < 700k).
	mocks.knowledge.verificationRate = 300_000
	mocks.staking.totalStaked = big.NewInt(400_000_000_000)
	mocks.staking.activeValidators = 50
	mocks.ontology.domainCount = 30
	mocks.vestingRewards.totalSupply = big.NewInt(1_000_000_000_000)

	// Run EndBlock at height 100.
	ctx = ctx.WithBlockHeight(100)
	am := alignment.NewAppModule(nil, k)
	if err := am.EndBlock(ctx); err != nil {
		t.Fatalf("EndBlock failed: %v", err)
	}

	state := k.GetState(ctx)
	if !state.DegradedFrequencyActive {
		t.Error("expected DegradedFrequencyActive=true after transition to degraded")
	}
	if state.PreviousCategory != types.CategoryDegraded {
		t.Errorf("expected PreviousCategory=degraded, got %s", state.PreviousCategory)
	}
}

func TestHealthTransitionRecoveryResetsFrequency(t *testing.T) {
	k, mocks, ctx := setupKeeper(t)

	// Set initial state: degraded with frequency override active.
	k.SetState(ctx, &types.AlignmentState{
		Enabled:                 true,
		PreviousCategory:        types.CategoryDegraded,
		DegradedFrequencyActive: true,
	})

	// Set dimensions to produce healthy composite (>= 700k).
	mocks.knowledge.verificationRate = 900_000
	mocks.knowledge.consensusDiversity = 900_000
	mocks.staking.totalStaked = big.NewInt(800_000_000_000)
	mocks.staking.activeValidators = 111
	mocks.ontology.domainCount = 100
	mocks.vestingRewards.totalSupply = big.NewInt(1_000_000_000_000)

	// Run EndBlock at height 100.
	ctx = ctx.WithBlockHeight(100)
	am := alignment.NewAppModule(nil, k)
	if err := am.EndBlock(ctx); err != nil {
		t.Fatalf("EndBlock failed: %v", err)
	}

	state := k.GetState(ctx)
	if state.DegradedFrequencyActive {
		t.Error("expected DegradedFrequencyActive=false after recovery")
	}
	if state.PreviousCategory != types.CategoryHealthy {
		t.Errorf("expected PreviousCategory=healthy, got %s", state.PreviousCategory)
	}
}

func TestDegradedFrequencyAffectsInterval(t *testing.T) {
	k, mocks, ctx := setupKeeper(t)

	// Set degraded frequency active + params interval = 100.
	k.SetState(ctx, &types.AlignmentState{
		Enabled:                 true,
		DegradedFrequencyActive: true,
		PreviousCategory:        types.CategoryDegraded,
	})

	// Degraded frequency: effective interval = 100/2 = 50.
	// Height 50 should trigger observation (50 % 50 == 0).
	mocks.knowledge.verificationRate = 300_000
	mocks.staking.totalStaked = big.NewInt(400_000_000_000)
	mocks.staking.activeValidators = 50
	mocks.ontology.domainCount = 30
	mocks.vestingRewards.totalSupply = big.NewInt(1_000_000_000_000)

	ctx = ctx.WithBlockHeight(50)
	am := alignment.NewAppModule(nil, k)
	if err := am.EndBlock(ctx); err != nil {
		t.Fatalf("EndBlock failed: %v", err)
	}

	_, found := k.GetObservation(ctx, 50)
	if !found {
		t.Error("expected observation at height 50 (degraded frequency: interval=50)")
	}
}
```

**Step 2: Run tests to verify they fail**

Run: `go test ./x/alignment/keeper/ -run TestHealthTransition -v`
Expected: FAIL

Run: `go test ./x/alignment/keeper/ -run TestDegradedFrequencyAffectsInterval -v`
Expected: FAIL

**Step 3: Implement health transition logic in EndBlock**

In `x/alignment/module.go`, modify `EndBlock`:

```go
func (am AppModule) EndBlock(ctx context.Context) error {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	height := uint64(sdkCtx.BlockHeight())

	if !am.keeper.IsEnabled(ctx) {
		return nil
	}
	if am.keeper.IsHalted(ctx) {
		return nil
	}

	params := am.keeper.GetParams(ctx)
	if params.ObservationIntervalBlocks == 0 {
		return nil
	}

	// Compute effective interval (halved when degraded).
	state := am.keeper.GetState(ctx)
	effectiveInterval := params.ObservationIntervalBlocks
	if state.DegradedFrequencyActive && effectiveInterval > 1 {
		effectiveInterval = effectiveInterval / 2
	}

	if height%effectiveInterval != 0 {
		return nil
	}

	// 1. Observe
	obs := am.keeper.ObserveAll(ctx)
	am.keeper.SetObservation(ctx, obs)

	// 2. Score
	scores := am.keeper.ComputeScores(ctx, obs)
	am.keeper.SetScores(ctx, scores)

	// 3. Corrections
	corrections := am.keeper.GenerateCorrections(ctx, scores)
	am.keeper.ApplyCorrections(ctx, corrections)

	// 4. Health index
	category := am.keeper.CategorizeHealth(ctx, scores.Composite)
	hi := am.keeper.BuildHealthIndex(scores, category, uint32(len(corrections)))
	am.keeper.SetHealthIndex(ctx, hi)

	// 5. Health transition responses
	previousCategory := state.PreviousCategory
	if previousCategory != category {
		switch {
		case previousCategory == types.CategoryHealthy && category == types.CategoryDegraded:
			state.DegradedFrequencyActive = true
			sdkCtx.EventManager().EmitEvent(
				sdk.NewEvent("zerone.alignment.network_health_degraded",
					sdk.NewAttribute("height", fmt.Sprintf("%d", height)),
					sdk.NewAttribute("composite", fmt.Sprintf("%d", scores.Composite)),
				),
			)
		case previousCategory == types.CategoryHealthy && category == types.CategoryCritical:
			state.DegradedFrequencyActive = true
			sdkCtx.EventManager().EmitEvent(
				sdk.NewEvent("zerone.alignment.network_health_critical",
					sdk.NewAttribute("height", fmt.Sprintf("%d", height)),
					sdk.NewAttribute("composite", fmt.Sprintf("%d", scores.Composite)),
				),
			)
		case previousCategory == types.CategoryDegraded && category == types.CategoryCritical:
			sdkCtx.EventManager().EmitEvent(
				sdk.NewEvent("zerone.alignment.network_health_critical",
					sdk.NewAttribute("height", fmt.Sprintf("%d", height)),
					sdk.NewAttribute("composite", fmt.Sprintf("%d", scores.Composite)),
				),
			)
		case (previousCategory == types.CategoryDegraded || previousCategory == types.CategoryCritical) && category == types.CategoryHealthy:
			state.DegradedFrequencyActive = false
			sdkCtx.EventManager().EmitEvent(
				sdk.NewEvent("zerone.alignment.network_health_recovered",
					sdk.NewAttribute("height", fmt.Sprintf("%d", height)),
					sdk.NewAttribute("composite", fmt.Sprintf("%d", scores.Composite)),
				),
			)
		}
		state.PreviousCategory = category
	}

	// 6. Update state
	state.LastObservationHeight = height
	state.ObservationCount++
	am.keeper.SetState(ctx, state)

	sdkCtx.EventManager().EmitEvent(
		sdk.NewEvent("zerone.alignment.observation_recorded",
			sdk.NewAttribute("height", fmt.Sprintf("%d", height)),
			sdk.NewAttribute("composite_score", fmt.Sprintf("%d", scores.Composite)),
			sdk.NewAttribute("category", category),
			sdk.NewAttribute("correction_count", fmt.Sprintf("%d", len(corrections))),
			sdk.NewAttribute("observation_count", fmt.Sprintf("%d", state.ObservationCount)),
		),
	)

	am.keeper.Logger(ctx).Info("alignment observation complete",
		"height", height,
		"composite", scores.Composite,
		"category", category,
		"corrections", len(corrections),
	)

	return nil
}
```

**Step 4: Run tests**

Run: `go test ./x/alignment/keeper/ -run "TestHealthTransition|TestDegradedFrequency" -v`
Expected: PASS

Run: `go test ./x/alignment/keeper/ -v`
Expected: PASS (all existing tests)

**Step 5: Commit**

```bash
git add x/alignment/module.go x/alignment/keeper/keeper_test.go
git commit -m "feat(alignment): implement health transition responses with degraded frequency"
```

---

### Task 6: Add Health History Query

**Files:**
- Modify: `proto/zerone/alignment/v1/query.proto`
- Modify: `x/alignment/types/query.pb.go`
- Modify: `x/alignment/types/query_grpc.pb.go`
- Modify: `x/alignment/keeper/grpc_query.go`
- Modify: `x/alignment/keeper/state.go`
- Modify: `x/alignment/client/cli/query.go`
- Test: `x/alignment/keeper/keeper_test.go`

**Step 1: Add query types to proto**

In `proto/zerone/alignment/v1/query.proto`, add:

```proto
  rpc HealthHistory(QueryHealthHistoryRequest) returns (QueryHealthHistoryResponse) {
    option (google.api.http).get = "/zerone/alignment/v1/health-history";
  }
```

And the message types:

```proto
message QueryHealthHistoryRequest {
  uint32 limit = 1;
}
message QueryHealthHistoryResponse {
  repeated AlignmentHealthIndex entries = 1;
}
```

**Step 2: Add Go types to query.pb.go**

In `x/alignment/types/query.pb.go`, add the request/response structs with json tags:

```go
type QueryHealthHistoryRequest struct {
	Limit uint32 `protobuf:"varint,1,opt,name=limit,proto3" json:"limit,omitempty"`
}

type QueryHealthHistoryResponse struct {
	Entries []*AlignmentHealthIndex `protobuf:"bytes,1,rep,name=entries,proto3" json:"entries,omitempty"`
}
```

Add `ProtoMessage()`, `Reset()`, and getter methods following the existing pattern. These types just need to satisfy the gRPC interface and JSON marshaling.

**Step 3: Add gRPC service method to query_grpc.pb.go**

In `x/alignment/types/query_grpc.pb.go`, add `HealthHistory` to the `QueryServer` interface and `UnimplementedQueryServer`.

**Step 4: Implement GetRecentHealthIndices in state.go**

In `x/alignment/keeper/state.go`, add:

```go
// GetRecentHealthIndices returns the most recent health indices, up to limit.
// Iterates in reverse order. Max iteration capped at 10,000 entries.
func (k Keeper) GetRecentHealthIndices(ctx context.Context, limit uint32) []*types.AlignmentHealthIndex {
	if limit == 0 || limit > 100 {
		limit = 20
	}

	st := k.storeService.OpenKVStore(ctx)
	iter, err := st.ReverseIterator(types.HealthIndexKeyPrefix, prefixEndBytes(types.HealthIndexKeyPrefix))
	if err != nil {
		return nil
	}
	defer iter.Close()

	var results []*types.AlignmentHealthIndex
	maxIter := 10_000
	count := 0
	for ; iter.Valid() && uint32(len(results)) < limit && count < maxIter; iter.Next() {
		count++
		var hi types.AlignmentHealthIndex
		if err := json.Unmarshal(iter.Value(), &hi); err != nil {
			continue
		}
		results = append(results, &hi)
	}
	return results
}
```

**Step 5: Implement query handler in grpc_query.go**

In `x/alignment/keeper/grpc_query.go`, add:

```go
func (q queryServer) HealthHistory(ctx context.Context, req *types.QueryHealthHistoryRequest) (*types.QueryHealthHistoryResponse, error) {
	entries := q.Keeper.GetRecentHealthIndices(ctx, req.Limit)
	return &types.QueryHealthHistoryResponse{Entries: entries}, nil
}
```

**Step 6: Add CLI command in query.go**

In `x/alignment/client/cli/query.go`, add `NewQueryHealthHistoryCmd()` to the `queryCmd.AddCommand(...)` list, and implement:

```go
func NewQueryHealthHistoryCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "history",
		Short: "Query alignment health history (most recent observations)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientQueryContext(cmd)
			if err != nil {
				return err
			}
			limit, _ := cmd.Flags().GetUint32("limit")
			req := &types.QueryHealthHistoryRequest{Limit: limit}
			resp := &types.QueryHealthHistoryResponse{}
			if err := clientCtx.Invoke(cmd.Context(), "/zerone.alignment.v1.Query/HealthHistory", req, resp); err != nil {
				return fmt.Errorf("failed to query health history: %w", err)
			}
			return clientCtx.PrintObjectLegacy(resp)
		},
	}
	cmd.Flags().Uint32("limit", 20, "Maximum number of entries to return (max 100)")
	flags.AddQueryFlagsToCmd(cmd)
	return cmd
}
```

**Step 7: Write tests**

In `x/alignment/keeper/keeper_test.go`, add:

```go
func TestQueryHealthHistory(t *testing.T) {
	k, _, ctx := setupKeeper(t)
	qs := keeper.NewQueryServerImpl(k)

	// Store health indices at multiple heights.
	for i := uint64(100); i <= 500; i += 100 {
		k.SetHealthIndex(ctx, &types.AlignmentHealthIndex{
			Height:         i,
			CompositeScore: 700_000 + i,
			Category:       types.CategoryHealthy,
		})
	}

	resp, err := qs.HealthHistory(ctx, &types.QueryHealthHistoryRequest{Limit: 3})
	if err != nil {
		t.Fatalf("query HealthHistory failed: %v", err)
	}
	if len(resp.Entries) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(resp.Entries))
	}
	// Should be in reverse order (most recent first).
	if resp.Entries[0].Height != 500 {
		t.Errorf("expected first entry height=500, got %d", resp.Entries[0].Height)
	}
}

func TestGetRecentHealthIndicesDefaultLimit(t *testing.T) {
	k, _, ctx := setupKeeper(t)

	// No entries — should not error.
	results := k.GetRecentHealthIndices(ctx, 0)
	if len(results) != 0 {
		t.Errorf("expected 0 entries, got %d", len(results))
	}
}
```

**Step 8: Run all tests**

Run: `go test ./x/alignment/keeper/ -v`
Expected: PASS

**Step 9: Commit**

```bash
git add x/alignment/ proto/zerone/alignment/v1/query.proto
git commit -m "feat(alignment): add health history query"
```

---

### Task 7: Integration Test — Full EndBlocker Cycle

**Files:**
- Test: `x/alignment/keeper/keeper_test.go`

**Step 1: Write integration test**

In `x/alignment/keeper/keeper_test.go`, add:

```go
func TestEndBlockerFullCycle(t *testing.T) {
	k, mocks, ctx := setupKeeper(t)

	// Wire autopoiesis.
	autoMock := &mockAutopoiesisKeeper{}
	k.SetAutopoiesisKeeper(autoMock)

	// Set all dimensions to healthy values.
	mocks.knowledge.verificationRate = 800_000
	mocks.knowledge.consensusDiversity = 700_000
	mocks.staking.totalStaked = big.NewInt(800_000_000_000)
	mocks.staking.activeValidators = 100
	mocks.staking.targetValidators = 111
	mocks.ontology.domainCount = 80
	mocks.vestingRewards.totalSupply = big.NewInt(1_000_000_000_000)

	am := alignment.NewAppModule(nil, k)

	// --- Block 100: First observation (healthy) ---
	ctx = ctx.WithBlockHeight(100)
	if err := am.EndBlock(ctx); err != nil {
		t.Fatalf("EndBlock at 100 failed: %v", err)
	}

	obs, found := k.GetObservation(ctx, 100)
	if !found {
		t.Fatal("expected observation at height 100")
	}
	if obs.KnowledgeQuality == 0 {
		t.Error("expected non-zero knowledge quality")
	}

	hi, found := k.GetHealthIndex(ctx, 100)
	if !found {
		t.Fatal("expected health index at height 100")
	}
	if hi.Category != types.CategoryHealthy {
		t.Errorf("expected healthy at block 100, got %s", hi.Category)
	}

	state := k.GetState(ctx)
	if state.ObservationCount != 1 {
		t.Errorf("expected observation_count=1, got %d", state.ObservationCount)
	}

	// --- Degrade dimensions for next observation ---
	mocks.knowledge.verificationRate = 200_000
	mocks.knowledge.consensusDiversity = 200_000
	mocks.staking.totalStaked = big.NewInt(200_000_000_000)
	mocks.staking.activeValidators = 30
	mocks.ontology.domainCount = 10

	// --- Block 200: Second observation (degraded) ---
	ctx = ctx.WithBlockHeight(200)
	if err := am.EndBlock(ctx); err != nil {
		t.Fatalf("EndBlock at 200 failed: %v", err)
	}

	hi2, found := k.GetHealthIndex(ctx, 200)
	if !found {
		t.Fatal("expected health index at height 200")
	}
	if hi2.Category == types.CategoryHealthy {
		t.Error("expected NOT healthy with degraded dimensions")
	}

	state2 := k.GetState(ctx)
	if state2.ObservationCount != 2 {
		t.Errorf("expected observation_count=2, got %d", state2.ObservationCount)
	}
	if state2.PreviousCategory != hi2.Category {
		t.Errorf("expected PreviousCategory=%s, got %s", hi2.Category, state2.PreviousCategory)
	}

	// If degraded, frequency should be active.
	if hi2.Category == types.CategoryDegraded || hi2.Category == types.CategoryCritical {
		if !state2.DegradedFrequencyActive {
			t.Error("expected DegradedFrequencyActive=true after degradation")
		}
	}

	// --- Recover dimensions ---
	mocks.knowledge.verificationRate = 900_000
	mocks.knowledge.consensusDiversity = 900_000
	mocks.staking.totalStaked = big.NewInt(900_000_000_000)
	mocks.staking.activeValidators = 111
	mocks.ontology.domainCount = 100

	// --- Block 250 or 300: Recovery observation ---
	// If frequency is doubled (interval=50), 250 triggers. Otherwise 300.
	recoveryHeight := int64(300)
	if state2.DegradedFrequencyActive {
		recoveryHeight = 250
	}
	ctx = ctx.WithBlockHeight(recoveryHeight)
	if err := am.EndBlock(ctx); err != nil {
		t.Fatalf("EndBlock at %d failed: %v", recoveryHeight, err)
	}

	hi3, found := k.GetHealthIndex(ctx, uint64(recoveryHeight))
	if !found {
		t.Fatalf("expected health index at height %d", recoveryHeight)
	}
	if hi3.Category != types.CategoryHealthy {
		t.Errorf("expected healthy after recovery, got %s (composite=%d)", hi3.Category, hi3.CompositeScore)
	}

	state3 := k.GetState(ctx)
	if state3.DegradedFrequencyActive {
		t.Error("expected DegradedFrequencyActive=false after recovery")
	}
}
```

**Step 2: Run integration test**

Run: `go test ./x/alignment/keeper/ -run TestEndBlockerFullCycle -v`
Expected: PASS

**Step 3: Run full test suite**

Run: `go test ./x/alignment/... -v`
Expected: PASS

**Step 4: Build check**

Run: `go build ./...`
Expected: BUILD SUCCESS

**Step 5: Commit**

```bash
git add x/alignment/keeper/keeper_test.go
git commit -m "test(alignment): add full EndBlocker lifecycle integration test"
```

---

### Task 8: Final Verification and Cleanup

**Files:**
- Review: all modified files

**Step 1: Run full test suite**

Run: `go test ./x/alignment/... -v -count=1`
Expected: PASS

**Step 2: Run knowledge tests (adapter changed)**

Run: `go test ./x/knowledge/... -v -count=1`
Expected: PASS

**Step 3: Build entire project**

Run: `go build ./cmd/zeroned`
Expected: BUILD SUCCESS

**Step 4: Verify no vet issues**

Run: `go vet ./x/alignment/...`
Expected: No issues

**Step 5: Final commit (if any cleanup)**

```bash
git add -A
git commit -m "feat(alignment): R28-7 rubedo alignment activation complete"
```
