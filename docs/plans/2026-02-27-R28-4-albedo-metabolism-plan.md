# R28-4 Albedo: Knowledge Metabolism Refinement — Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Refine the knowledge fact lifecycle: fix patronage energy recovery, enforce confidence caps, rescale energy to 0–1M, add multi-level thresholds, activate confidence growth, add metabolism dashboard query.

**Architecture:** Keep the existing cost/income metabolism engine. Rescale to 0–1M BPS scale. Add multi-level thresholds (ACTIVE/AT_RISK/EXTINCT). Fix the MsgAddFact confidence bypass. Wire up unused confidence growth params. Add unified lifecycle events.

**Tech Stack:** Go 1.24+, Cosmos SDK v0.50.15, CometBFT v0.38.20

---

### Task 1: Rescale Energy Params + Add Threshold Params

**Files:**
- Modify: `x/knowledge/types/genesis.go:100-112` (DefaultParams metabolism section)
- Modify: `x/knowledge/types/genesis.go:379-394` (Validate metabolism section)
- Modify: `x/knowledge/types/genesis.pb.go` (add new param fields)

**Step 1: Add new param fields to genesis.pb.go**

Add three new fields after the existing metabolism params (after field 60 `MetabolismExpiredToPrunedEpochs`). Find the Params struct and add:

```go
MetabolismActiveThreshold     uint64 `protobuf:"varint,85,opt,name=metabolism_active_threshold,json=metabolismActiveThreshold,proto3" json:"metabolism_active_threshold,omitempty"`
MetabolismExtinctionThreshold uint64 `protobuf:"varint,86,opt,name=metabolism_extinction_threshold,json=metabolismExtinctionThreshold,proto3" json:"metabolism_extinction_threshold,omitempty"`
MaxConfidence                 uint64 `protobuf:"varint,87,opt,name=max_confidence,json=maxConfidence,proto3" json:"max_confidence,omitempty"`
```

Add getter methods following the existing pattern (e.g. near `GetMetabolismAtRiskEpochs`):

```go
func (x *Params) GetMetabolismActiveThreshold() uint64 {
	if x != nil {
		return x.MetabolismActiveThreshold
	}
	return 0
}

func (x *Params) GetMetabolismExtinctionThreshold() uint64 {
	if x != nil {
		return x.MetabolismExtinctionThreshold
	}
	return 0
}

func (x *Params) GetMaxConfidence() uint64 {
	if x != nil {
		return x.MaxConfidence
	}
	return 0
}
```

**Step 2: Rescale defaults in genesis.go**

Replace the metabolism defaults block at lines 100-112:

```go
// ─── Metabolism ─────────────────────────────────────────────────────
MetabolismBaseCost:                10_000,     // 10,000 energy drain per epoch (1% of cap)
MetabolismContentLengthBps:        10_000,     // +1% base cost per 100 chars
MetabolismDomainCompetitionBps:    5_000,      // +0.5% base cost per 100 domain facts
MetabolismEnergyPerQuery:          1_000,      // 1,000 energy per agent query
MetabolismEnergyPerCitation:       5_000,      // 5,000 energy per new citation
MetabolismEnergyPerPatronage:      20_000,     // 20,000 energy per patronage epoch
MetabolismEnergyChallengeSurvival: 100_000,    // 100,000 energy for surviving challenge
MetabolismEnergyCap:               1_000_000,  // Max 1,000,000 energy (matches BPS scale)
MetabolismInitialEnergy:           500_000,    // Born with 50% of cap
MetabolismAtRiskEpochs:            5,          // 5 epochs at low energy before expiry
MetabolismExpiredToPrunedEpochs:   20,         // 20 epochs after expiry before archive
MetabolismActiveThreshold:         300_000,    // 30% — below this → AT_RISK
MetabolismExtinctionThreshold:     10_000,     // 1% — below this for N epochs → EXTINCT
MaxConfidence:                     880_000,    // Hard cap on confidence (matches SurvivedChallengeConfidenceCap)
```

**Step 3: Add validation for new params**

In Validate() after line 393 (`MetabolismExpiredToPrunedEpochs`), add:

```go
if p.MetabolismActiveThreshold == 0 {
	return fmt.Errorf("metabolism_active_threshold must be > 0")
}
if p.MetabolismActiveThreshold > p.MetabolismEnergyCap {
	return fmt.Errorf("metabolism_active_threshold (%d) must be <= metabolism_energy_cap (%d)", p.MetabolismActiveThreshold, p.MetabolismEnergyCap)
}
if p.MetabolismExtinctionThreshold >= p.MetabolismActiveThreshold {
	return fmt.Errorf("metabolism_extinction_threshold (%d) must be < metabolism_active_threshold (%d)", p.MetabolismExtinctionThreshold, p.MetabolismActiveThreshold)
}
if p.MaxConfidence == 0 {
	return fmt.Errorf("max_confidence must be > 0")
}
if p.MaxConfidence > 1_000_000 {
	return fmt.Errorf("max_confidence must be <= 1,000,000")
}
```

**Step 4: Run tests**

Run: `go build ./x/knowledge/...`
Expected: Compiles cleanly

**Step 5: Commit**

```
feat(knowledge): rescale energy params to 0-1M and add threshold params (R28-4)
```

---

### Task 2: Multi-Level Status Transitions in Metabolism

**Files:**
- Modify: `x/knowledge/keeper/metabolism.go:57-76` (status transitions)
- Test: `x/knowledge/keeper/metabolism_test.go`

**Step 1: Write failing tests for multi-level thresholds**

Add to `metabolism_test.go`. Update `makeEnergyFact` helper to use new scale:

```go
func makeEnergyFact(id, content, domain string, energy uint64, status types.FactStatus) *types.Fact {
	return &types.Fact{
		Id:        id,
		Content:   content,
		Domain:    domain,
		Status:    status,
		Energy:    energy,
		EnergyCap: 1_000_000,
		Submitter: "zrn1test",
	}
}
```

Add new tests:

```go
func TestMetabolism_MultiLevelThresholds_ActiveToAtRisk(t *testing.T) {
	k, ctx := setupKnowledgeTest(t)

	// Fact with energy just above active threshold — drain should push below
	// ActiveThreshold = 300,000. BaseCost = 10,000. Start at 305,000 → drains to 295,000 → AT_RISK
	fact := makeEnergyFact("fact-ml1", "Multi-level threshold test!!!", "physics", 305_000, types.FactStatus_FACT_STATUS_VERIFIED)
	require.NoError(t, k.SetFact(ctx, fact))

	require.NoError(t, k.ProcessMetabolism(ctx, 1))

	updated, found := k.GetFact(ctx, "fact-ml1")
	require.True(t, found)
	require.Equal(t, uint64(295_000), updated.Energy)
	require.Equal(t, types.FactStatus_FACT_STATUS_AT_RISK, updated.Status)
}

func TestMetabolism_MultiLevelThresholds_AtRiskRecovery(t *testing.T) {
	k, ctx := setupKnowledgeTest(t)

	// AT_RISK fact with enough query income to push above threshold
	// 400 queries * 1000 = 400,000 income. BaseCost 10,000. Start at 0.
	// Net: 0 + 400,000 - 10,000 = 390,000 > 300,000 → ACTIVE
	fact := makeEnergyFact("fact-ml2", "Recovery threshold test!!!!!", "physics", 0, types.FactStatus_FACT_STATUS_AT_RISK)
	fact.AtRiskSinceEpoch = 1
	fact.QueryCountEpoch = 400
	require.NoError(t, k.SetFact(ctx, fact))

	require.NoError(t, k.ProcessMetabolism(ctx, 2))

	updated, found := k.GetFact(ctx, "fact-ml2")
	require.True(t, found)
	require.Equal(t, types.FactStatus_FACT_STATUS_ACTIVE, updated.Status)
	require.Equal(t, uint64(0), updated.AtRiskSinceEpoch)
}

func TestMetabolism_MultiLevelThresholds_ExtinctionZone(t *testing.T) {
	k, ctx := setupKnowledgeTest(t)

	// Fact at extinction threshold — should stay AT_RISK (not instantly extinct)
	fact := makeEnergyFact("fact-ml3", "Extinction zone test fact!!", "physics", 10_100, types.FactStatus_FACT_STATUS_AT_RISK)
	fact.AtRiskSinceEpoch = 1
	require.NoError(t, k.SetFact(ctx, fact))

	// After drain: 10,100 - 10,000 = 100. Still > 0 but < ExtinctionThreshold.
	// AT_RISK since epoch 1, now epoch 2 — not expired yet
	require.NoError(t, k.ProcessMetabolism(ctx, 2))

	updated, found := k.GetFact(ctx, "fact-ml3")
	require.True(t, found)
	require.Equal(t, types.FactStatus_FACT_STATUS_AT_RISK, updated.Status)
}
```

**Step 2: Run tests to verify they fail**

Run: `go test ./x/knowledge/keeper/ -run TestMetabolism_MultiLevel -v`
Expected: FAIL — existing logic uses binary energy=0 check

**Step 3: Update status transitions in metabolism.go**

Replace lines 57-76 (the status transitions block):

```go
// ─── State transitions (multi-level thresholds) ──────────
oldStatus := fact.Status

if newEnergy >= params.MetabolismActiveThreshold {
	// Healthy — clear any at-risk state
	if fact.AtRiskSinceEpoch > 0 {
		fact.AtRiskSinceEpoch = 0
		fact.Status = types.FactStatus_FACT_STATUS_ACTIVE
	}
} else if newEnergy < params.MetabolismActiveThreshold {
	// Below active threshold
	if fact.AtRiskSinceEpoch == 0 {
		// Just entered at-risk zone
		fact.AtRiskSinceEpoch = epoch
		fact.Status = types.FactStatus_FACT_STATUS_AT_RISK
	} else {
		// Already at risk — check for expiry/pruning
		atRiskDuration := epoch - fact.AtRiskSinceEpoch
		if atRiskDuration >= params.MetabolismAtRiskEpochs+params.MetabolismExpiredToPrunedEpochs {
			fact.Status = types.FactStatus_FACT_STATUS_PRUNED
		} else if atRiskDuration >= params.MetabolismAtRiskEpochs {
			fact.Status = types.FactStatus_FACT_STATUS_EXPIRED
		}
	}
}
```

**Step 4: Run tests to verify they pass**

Run: `go test ./x/knowledge/keeper/ -run TestMetabolism -v`
Expected: All metabolism tests pass

**Step 5: Fix existing tests for new scale**

Update all existing metabolism tests to use 1M-scale energy values. Key changes:
- `TestMetabolism_BaseDrain`: energy 500_000, expect 490_000 after base cost 10_000
- `TestMetabolism_QueryIncome`: energy 100_000, 50 queries * 1000 = 50_000 income, cost 10_000, expect 140_000
- `TestMetabolism_CitationIncome`: energy 100_000, 3 citations * 5000 = 15_000, cost 10_000, expect 105_000
- `TestMetabolism_PatronageIncome`: energy 100_000, patronage income 20_000, cost 10_000, expect 110_000
- `TestMetabolism_AtRiskTransition`: energy 10_000 (just at base cost), drains to 0, below 300K → AT_RISK
- `TestMetabolism_ExpiredTransition`: energy 0, at_risk since epoch 1, epoch 6 → EXPIRED
- `TestMetabolism_PrunedTransition`: energy 0, at_risk since epoch 1, epoch 26 → PRUNED
- `TestMetabolism_Recovery`: energy 0, 400 queries → 400_000 income → above 300K → ACTIVE
- `TestMetabolism_EnergyCap`: energy 990_000, income 100_000, cost 10_000, cap at 1_000_000
- `TestMetabolism_InitialEnergy`: expect 500_000 initial, 1_000_000 cap

**Step 6: Run full test suite**

Run: `go test ./x/knowledge/keeper/ -run TestMetabolism -v`
Expected: All PASS

**Step 7: Commit**

```
feat(knowledge): implement multi-level energy thresholds for metabolism (R28-4)
```

---

### Task 3: Unified Fact Lifecycle Events

**Files:**
- Modify: `x/knowledge/keeper/metabolism.go:200-239` (emitMetabolismStatusEvent)
- Test: `x/knowledge/keeper/metabolism_test.go`

**Step 1: Write failing test for unified events**

```go
func TestMetabolism_UnifiedStatusEvent(t *testing.T) {
	k, ctx := setupKnowledgeTest(t)

	fact := makeEnergyFact("fact-ev", "Event test fact content!!", "physics", 305_000, types.FactStatus_FACT_STATUS_VERIFIED)
	require.NoError(t, k.SetFact(ctx, fact))

	require.NoError(t, k.ProcessMetabolism(ctx, 1))

	// Check for unified event
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	events := sdkCtx.EventManager().Events()
	found := false
	for _, event := range events {
		if event.Type == "zerone.knowledge.fact_status_changed" {
			found = true
			attrs := make(map[string]string)
			for _, attr := range event.Attributes {
				attrs[attr.Key] = attr.Value
			}
			require.Equal(t, "fact-ev", attrs["fact_id"])
			require.Equal(t, "FACT_STATUS_VERIFIED", attrs["old_status"])
			require.Equal(t, "FACT_STATUS_AT_RISK", attrs["new_status"])
			require.Equal(t, "decay", attrs["reason"])
			require.NotEmpty(t, attrs["energy"])
		}
	}
	require.True(t, found, "should emit fact_status_changed event")
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./x/knowledge/keeper/ -run TestMetabolism_UnifiedStatusEvent -v`
Expected: FAIL — old event types don't match

**Step 3: Replace emitMetabolismStatusEvent**

Replace lines 200-239:

```go
// emitMetabolismStatusEvent emits a unified fact_status_changed event.
func (k Keeper) emitMetabolismStatusEvent(ctx context.Context, fact *types.Fact, oldStatus types.FactStatus, epoch uint64) {
	sdkCtx := sdk.UnwrapSDKContext(ctx)

	reason := "decay"
	if fact.Status == types.FactStatus_FACT_STATUS_ACTIVE &&
		(oldStatus == types.FactStatus_FACT_STATUS_AT_RISK || oldStatus == types.FactStatus_FACT_STATUS_EXPIRED) {
		reason = "recovery"
	} else if fact.Status == types.FactStatus_FACT_STATUS_EXPIRED {
		reason = "extinction"
	} else if fact.Status == types.FactStatus_FACT_STATUS_PRUNED {
		reason = "extinction"
	}

	sdkCtx.EventManager().EmitEvent(sdk.NewEvent(
		"zerone.knowledge.fact_status_changed",
		sdk.NewAttribute("fact_id", fact.Id),
		sdk.NewAttribute("old_status", oldStatus.String()),
		sdk.NewAttribute("new_status", fact.Status.String()),
		sdk.NewAttribute("energy", fmt.Sprintf("%d", fact.Energy)),
		sdk.NewAttribute("reason", reason),
		sdk.NewAttribute("epoch", fmt.Sprintf("%d", epoch)),
	))
}
```

**Step 4: Run tests**

Run: `go test ./x/knowledge/keeper/ -run TestMetabolism -v`
Expected: All PASS

**Step 5: Commit**

```
feat(knowledge): unify fact lifecycle events into fact_status_changed (R28-4)
```

---

### Task 4: Immediate Patronage Energy Recovery

**Files:**
- Modify: `x/knowledge/keeper/msg_server.go:847-887` (PatronizeFact handler)
- Test: `x/knowledge/keeper/metabolism_test.go`

**Step 1: Write failing tests**

```go
func TestPatronage_ImmediateEnergyBoost(t *testing.T) {
	k, ctx := setupKnowledgeTest(t)
	ctx = ctx.WithBlockHeader(cmtproto.Header{Height: 500})

	params, _ := k.GetParams(ctx)

	// Fact with low energy — no patronage yet
	fact := makeEnergyFact("fact-ipe", "Patronage immediate test!!", "physics", 200_000, types.FactStatus_FACT_STATUS_VERIFIED)
	require.NoError(t, k.SetFact(ctx, fact))

	// Duration: 10,000 blocks. FitnessEpochBlocks = 1000. So ~10 epochs.
	// Boost = MetabolismEnergyPerPatronage * durationEpochs / 10 = 20,000 * 10 / 10 = 20,000
	durationBlocks := uint64(10_000)
	expectedBoost := params.MetabolismEnergyPerPatronage * (durationBlocks / params.FitnessEpochBlocks) / 10

	// Simulate patronage (skip bank — no bankKeeper in test)
	k.ApplyPatronageEnergyBoost(ctx, fact, durationBlocks)

	updated, found := k.GetFact(ctx, "fact-ipe")
	require.True(t, found)
	require.Equal(t, 200_000+expectedBoost, updated.Energy)
}

func TestPatronage_AtRiskRecovery(t *testing.T) {
	k, ctx := setupKnowledgeTest(t)
	ctx = ctx.WithBlockHeader(cmtproto.Header{Height: 500})

	// AT_RISK fact — patronage should push above threshold and recover
	fact := makeEnergyFact("fact-prec", "Patronage recovery test!!!", "physics", 250_000, types.FactStatus_FACT_STATUS_AT_RISK)
	fact.AtRiskSinceEpoch = 1
	require.NoError(t, k.SetFact(ctx, fact))

	// Long patronage: 50,000 blocks = 50 epochs. Boost = 20,000 * 50 / 10 = 100,000
	k.ApplyPatronageEnergyBoost(ctx, fact, 50_000)

	updated, found := k.GetFact(ctx, "fact-prec")
	require.True(t, found)
	// 250,000 + 100,000 = 350,000 > ActiveThreshold (300,000) → ACTIVE
	require.Equal(t, types.FactStatus_FACT_STATUS_ACTIVE, updated.Status)
	require.Equal(t, uint64(0), updated.AtRiskSinceEpoch)
}
```

**Step 2: Run tests to verify they fail**

Run: `go test ./x/knowledge/keeper/ -run TestPatronage_ -v`
Expected: FAIL — `ApplyPatronageEnergyBoost` doesn't exist

**Step 3: Add ApplyPatronageEnergyBoost to metabolism.go**

Add at end of file:

```go
// ApplyPatronageEnergyBoost gives an immediate energy boost when patronage is set.
// Boost is proportional to patronage duration: MetabolismEnergyPerPatronage * epochs / 10.
func (k Keeper) ApplyPatronageEnergyBoost(ctx context.Context, fact *types.Fact, durationBlocks uint64) {
	params, err := k.GetParams(ctx)
	if err != nil {
		return
	}

	durationEpochs := uint64(1)
	if params.FitnessEpochBlocks > 0 {
		durationEpochs = durationBlocks / params.FitnessEpochBlocks
		if durationEpochs == 0 {
			durationEpochs = 1
		}
	}

	boost := params.MetabolismEnergyPerPatronage * durationEpochs / 10
	if boost == 0 {
		boost = params.MetabolismEnergyPerPatronage // minimum one epoch worth
	}

	oldStatus := fact.Status
	fact.Energy += boost
	if fact.Energy > params.MetabolismEnergyCap {
		fact.Energy = params.MetabolismEnergyCap
	}

	// Recover from AT_RISK if energy is above active threshold
	if (fact.Status == types.FactStatus_FACT_STATUS_AT_RISK || fact.Status == types.FactStatus_FACT_STATUS_EXPIRED) &&
		fact.Energy >= params.MetabolismActiveThreshold {
		fact.AtRiskSinceEpoch = 0
		fact.Status = types.FactStatus_FACT_STATUS_ACTIVE
	}

	sdkCtx := sdk.UnwrapSDKContext(ctx)
	fact.EnergyLastUpdated = uint64(sdkCtx.BlockHeight())
	_ = k.SetFact(ctx, fact)

	if oldStatus != fact.Status {
		sdkCtx.EventManager().EmitEvent(sdk.NewEvent(
			"zerone.knowledge.fact_status_changed",
			sdk.NewAttribute("fact_id", fact.Id),
			sdk.NewAttribute("old_status", oldStatus.String()),
			sdk.NewAttribute("new_status", fact.Status.String()),
			sdk.NewAttribute("energy", fmt.Sprintf("%d", fact.Energy)),
			sdk.NewAttribute("reason", "patronage_recovery"),
			sdk.NewAttribute("epoch", "0"),
		))
	}
}
```

**Step 4: Wire into PatronizeFact handler in msg_server.go**

After line 874 (`fact.PatronageExpiryBlock = height + msg.DurationBlocks`), before `_ = m.keeper.SetFact(ctx, fact)`, add:

```go
// Apply immediate energy boost (before SetFact below)
m.keeper.ApplyPatronageEnergyBoost(ctx, fact, msg.DurationBlocks)
// Re-read fact since ApplyPatronageEnergyBoost already saved it
fact, _ = m.keeper.GetFact(ctx, msg.FactId)
```

Remove the duplicate `_ = m.keeper.SetFact(ctx, fact)` line since ApplyPatronageEnergyBoost already saves.

**Step 5: Run tests**

Run: `go test ./x/knowledge/keeper/ -run "TestPatronage_|TestMetabolism_" -v`
Expected: All PASS

**Step 6: Commit**

```
feat(knowledge): add immediate patronage energy recovery (R28-4)
```

---

### Task 5: Fix Confidence Cap — MsgAddFact Bypass + clampConfidence

**Files:**
- Modify: `x/knowledge/keeper/confidence.go` (add clampConfidence helper)
- Modify: `x/knowledge/keeper/msg_server.go:424-464` (fix MsgAddFact)
- Modify: `x/knowledge/keeper/rounds.go:266-276` (apply clampConfidence)
- Test: `x/knowledge/keeper/metabolism_test.go`

**Step 1: Write failing tests**

```go
func TestConfidence_MsgAddFactCeiling(t *testing.T) {
	k, ctx := setupKnowledgeTest(t)

	params, _ := k.GetParams(ctx)

	// Simulate adding a fact with confidence exceeding MaxConfidence
	fact := &types.Fact{
		Id:         "fact-cap1",
		Content:    "Governance fact with high confidence",
		Domain:     "physics",
		Confidence: 950_000, // above MaxConfidence (880,000)
		Status:     types.FactStatus_FACT_STATUS_VERIFIED,
		Submitter:  "zrn1authority",
	}

	// Apply clampConfidence
	fact.Confidence = k.ClampConfidence(ctx, fact.Confidence, fact.Domain)

	require.NoError(t, k.SetFact(ctx, fact))
	updated, _ := k.GetFact(ctx, "fact-cap1")
	require.LessOrEqual(t, updated.Confidence, params.MaxConfidence,
		"confidence should not exceed MaxConfidence")
}

func TestConfidence_NeverExceedsCap(t *testing.T) {
	k, ctx := setupKnowledgeTest(t)

	params, _ := k.GetParams(ctx)

	// Test various confidence values — all should be capped
	testCases := []struct {
		input    uint64
		expected uint64
	}{
		{500_000, 500_000},  // below cap — unchanged
		{880_000, 880_000},  // at cap — unchanged
		{950_000, 880_000},  // above cap — clamped
		{1_000_000, 880_000}, // way above — clamped
	}

	for _, tc := range testCases {
		result := k.ClampConfidence(ctx, tc.input, "physics")
		require.Equal(t, tc.expected, result,
			"ClampConfidence(%d) should return %d", tc.input, tc.expected)
		_ = params // suppress unused
	}
}
```

**Step 2: Run tests to verify they fail**

Run: `go test ./x/knowledge/keeper/ -run TestConfidence_ -v`
Expected: FAIL — `ClampConfidence` doesn't exist

**Step 3: Add ClampConfidence to confidence.go**

Add at end of confidence.go:

```go
// ClampConfidence enforces the MaxConfidence hard cap and optional stratum ceiling.
func (k Keeper) ClampConfidence(ctx context.Context, confidence uint64, domain string) uint64 {
	params, err := k.GetParams(ctx)
	if err != nil {
		return confidence
	}

	// Apply stratum ceiling if ontology keeper is available
	if k.ontologyKeeper != nil && domain != "" {
		stratum, err := k.ontologyKeeper.GetStratumForDomain(ctx, domain)
		if err == nil && stratum != "" {
			ceiling, err := k.ontologyKeeper.GetConfidenceCeiling(ctx, stratum)
			if err == nil && ceiling > 0 && confidence > ceiling {
				confidence = ceiling
			}
		}
	}

	// Apply global hard cap
	if params.MaxConfidence > 0 && confidence > params.MaxConfidence {
		confidence = params.MaxConfidence
	}

	return confidence
}
```

**Step 4: Fix MsgAddFact in msg_server.go**

After line 440 (`Confidence: msg.Confidence,`), before `SetFact`, add the clamp call. Replace lines 435-451:

```go
	fact := &types.Fact{
		Id:               factID,
		Content:          msg.Content,
		Domain:           msg.Domain,
		Category:         msg.Category,
		Confidence:       m.keeper.ClampConfidence(ctx, msg.Confidence, msg.Domain),
		Submitter:        msg.Authority,
		SubmittedAtBlock: height,
		VerifiedAtBlock:  height,
		LastVerifiedBlock: height,
		References:       msg.References,
		Status:           types.FactStatus_FACT_STATUS_VERIFIED,
		// Initialize metabolism fields
		Energy:           params.MetabolismInitialEnergy,
		EnergyCap:        params.MetabolismEnergyCap,
		EnergyLastUpdated: height,
	}
```

Note: also need to read params at top of AddFact. Add after the height calculation:

```go
	params, err := m.keeper.GetParams(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get params: %w", err)
	}
```

**Step 5: Simplify createFactFromClaim ceiling logic in rounds.go**

Replace lines 266-276 (stratum ceiling block) with:

```go
	// Apply confidence ceiling (stratum + global MaxConfidence hard cap)
	fact.Confidence = k.ClampConfidence(ctx, fact.Confidence, claim.Domain)
	if k.ontologyKeeper != nil && claim.Domain != "" {
		stratum, err := k.ontologyKeeper.GetStratumForDomain(ctx, claim.Domain)
		if err == nil && stratum != "" {
			fact.Stratum = stratum
		}
	}
```

**Step 6: Apply clamp in AggregateVerificationResult too**

In confidence.go, after line 124 (end of stratum ceiling block), add:

```go
	// Apply global MaxConfidence hard cap
	if params.MaxConfidence > 0 && result.Confidence > params.MaxConfidence {
		result.Confidence = params.MaxConfidence
	}
```

**Step 7: Run tests**

Run: `go test ./x/knowledge/keeper/ -run "TestConfidence_|TestMetabolism_" -v`
Expected: All PASS

**Step 8: Commit**

```
fix(knowledge): enforce confidence cap in MsgAddFact and all paths (R28-4)
```

---

### Task 6: Activate Confidence Growth

**Files:**
- Modify: `x/knowledge/keeper/fitness.go:102-169` (UpdateAllFitnessScores)
- Test: `x/knowledge/keeper/metabolism_test.go`

**Step 1: Write failing test**

```go
func TestConfidence_GrowthAtEpoch(t *testing.T) {
	k, ctx := setupKnowledgeTest(t)

	params, _ := k.GetParams(ctx)

	// ACTIVE fact with 500K confidence — should grow by 1.1% per epoch
	fact := &types.Fact{
		Id:         "fact-cg",
		Content:    "Confidence growth test fact!",
		Domain:     "physics",
		Status:     types.FactStatus_FACT_STATUS_ACTIVE,
		Confidence: 500_000,
		Energy:     500_000,
		EnergyCap:  1_000_000,
		EpochBorn:  0,
		FitnessScore: 500_000,
		Submitter:  "zrn1test",
	}
	require.NoError(t, k.SetFact(ctx, fact))

	// Run fitness epoch update
	require.NoError(t, k.UpdateAllFitnessScores(ctx))

	updated, found := k.GetFact(ctx, "fact-cg")
	require.True(t, found)

	// Growth = 500,000 * 11,000 / 1,000,000 = 5,500
	expectedConfidence := uint64(500_000 + safeMulDiv(500_000, params.ConfidenceGrowthPerEpochBps, 1_000_000))
	require.Equal(t, expectedConfidence, updated.Confidence)
}

func TestConfidence_GrowthCappedAtMax(t *testing.T) {
	k, ctx := setupKnowledgeTest(t)

	params, _ := k.GetParams(ctx)

	// Fact near MaxConfidence — growth should be capped
	fact := &types.Fact{
		Id:         "fact-cgc",
		Content:    "Confidence growth cap test!!",
		Domain:     "physics",
		Status:     types.FactStatus_FACT_STATUS_VERIFIED,
		Confidence: 875_000, // near MaxConfidence (880,000)
		Energy:     500_000,
		EnergyCap:  1_000_000,
		EpochBorn:  0,
		FitnessScore: 500_000,
		Submitter:  "zrn1test",
	}
	require.NoError(t, k.SetFact(ctx, fact))

	require.NoError(t, k.UpdateAllFitnessScores(ctx))

	updated, found := k.GetFact(ctx, "fact-cgc")
	require.True(t, found)
	require.LessOrEqual(t, updated.Confidence, params.MaxConfidence,
		"confidence growth should not exceed MaxConfidence")
}

func TestConfidence_NoGrowthWhenAtRisk(t *testing.T) {
	k, ctx := setupKnowledgeTest(t)

	// AT_RISK fact should NOT grow confidence
	fact := &types.Fact{
		Id:         "fact-cng",
		Content:    "No growth when at risk!!!!",
		Domain:     "physics",
		Status:     types.FactStatus_FACT_STATUS_AT_RISK,
		Confidence: 500_000,
		Energy:     100_000,
		EnergyCap:  1_000_000,
		EpochBorn:  0,
		FitnessScore: 500_000,
		AtRiskSinceEpoch: 1,
		Submitter:  "zrn1test",
	}
	require.NoError(t, k.SetFact(ctx, fact))

	require.NoError(t, k.UpdateAllFitnessScores(ctx))

	updated, found := k.GetFact(ctx, "fact-cng")
	require.True(t, found)
	// AT_RISK facts are not iterated by UpdateAllFitnessScores (only VERIFIED/ACTIVE/PROVISIONAL)
	require.Equal(t, uint64(500_000), updated.Confidence, "AT_RISK fact should not grow confidence")
}
```

**Step 2: Run tests to verify they fail**

Run: `go test ./x/knowledge/keeper/ -run TestConfidence_Growth -v`
Expected: FAIL — no confidence growth logic exists

**Step 3: Add confidence growth to UpdateAllFitnessScores**

In fitness.go, inside the `for _, fact := range factsToUpdate` loop (around line 130), after `fact.FitnessScore = newFitness`, add:

```go
		// Confidence growth for healthy facts
		if params.ConfidenceGrowthPerEpochBps > 0 && fact.Confidence > 0 {
			growth := safeMulDiv(fact.Confidence, params.ConfidenceGrowthPerEpochBps, 1_000_000)
			if growth > 0 {
				fact.Confidence += growth
				fact.Confidence = k.ClampConfidence(ctx, fact.Confidence, fact.Domain)
			}
		}
```

**Step 4: Run tests**

Run: `go test ./x/knowledge/keeper/ -run "TestConfidence_|TestMetabolism_" -v`
Expected: All PASS

**Step 5: Commit**

```
feat(knowledge): activate confidence growth at fitness epochs (R28-4)
```

---

### Task 7: Metabolism Dashboard Query

**Files:**
- Modify: `x/knowledge/types/query.pb.go` (add request/response types)
- Modify: `x/knowledge/keeper/grpc_query.go` (add query handler)
- Modify: `x/knowledge/client/cli/query.go` (add CLI command)
- Test: `x/knowledge/keeper/metabolism_test.go`

**Step 1: Add proto types to query.pb.go**

Add the request/response structs (find the pattern from QueryFactsAtRiskRequest). Place near the end of the message type definitions:

```go
type QueryMetabolismStatusRequest struct {
	state         protoimpl.MessageState `protogen:"open.v1"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *QueryMetabolismStatusRequest) Reset()         { *x = QueryMetabolismStatusRequest{} }
func (x *QueryMetabolismStatusRequest) String() string  { return fmt.Sprintf("%+v", *x) }
func (*QueryMetabolismStatusRequest) ProtoMessage()     {}

type QueryMetabolismStatusResponse struct {
	state              protoimpl.MessageState `protogen:"open.v1"`
	TotalFacts         uint64 `protobuf:"varint,1,opt,name=total_facts,json=totalFacts,proto3" json:"total_facts,omitempty"`
	ActiveCount        uint64 `protobuf:"varint,2,opt,name=active_count,json=activeCount,proto3" json:"active_count,omitempty"`
	AtRiskCount        uint64 `protobuf:"varint,3,opt,name=at_risk_count,json=atRiskCount,proto3" json:"at_risk_count,omitempty"`
	ExpiredCount       uint64 `protobuf:"varint,4,opt,name=expired_count,json=expiredCount,proto3" json:"expired_count,omitempty"`
	PrunedCount        uint64 `protobuf:"varint,5,opt,name=pruned_count,json=prunedCount,proto3" json:"pruned_count,omitempty"`
	AvgEnergy          uint64 `protobuf:"varint,6,opt,name=avg_energy,json=avgEnergy,proto3" json:"avg_energy,omitempty"`
	CurrentEpoch       uint64 `protobuf:"varint,7,opt,name=current_epoch,json=currentEpoch,proto3" json:"current_epoch,omitempty"`
	NextEpochBlock     uint64 `protobuf:"varint,8,opt,name=next_epoch_block,json=nextEpochBlock,proto3" json:"next_epoch_block,omitempty"`
	RecentRecoveries   uint64 `protobuf:"varint,9,opt,name=recent_recoveries,json=recentRecoveries,proto3" json:"recent_recoveries,omitempty"`
	RecentExtinctions  uint64 `protobuf:"varint,10,opt,name=recent_extinctions,json=recentExtinctions,proto3" json:"recent_extinctions,omitempty"`
	unknownFields      protoimpl.UnknownFields
	sizeCache          protoimpl.SizeCache
}

func (x *QueryMetabolismStatusResponse) Reset()         { *x = QueryMetabolismStatusResponse{} }
func (x *QueryMetabolismStatusResponse) String() string  { return fmt.Sprintf("%+v", *x) }
func (*QueryMetabolismStatusResponse) ProtoMessage()     {}

func (x *QueryMetabolismStatusResponse) GetTotalFacts() uint64 {
	if x != nil { return x.TotalFacts }
	return 0
}
func (x *QueryMetabolismStatusResponse) GetActiveCount() uint64 {
	if x != nil { return x.ActiveCount }
	return 0
}
func (x *QueryMetabolismStatusResponse) GetAtRiskCount() uint64 {
	if x != nil { return x.AtRiskCount }
	return 0
}
func (x *QueryMetabolismStatusResponse) GetExpiredCount() uint64 {
	if x != nil { return x.ExpiredCount }
	return 0
}
func (x *QueryMetabolismStatusResponse) GetPrunedCount() uint64 {
	if x != nil { return x.PrunedCount }
	return 0
}
func (x *QueryMetabolismStatusResponse) GetAvgEnergy() uint64 {
	if x != nil { return x.AvgEnergy }
	return 0
}
func (x *QueryMetabolismStatusResponse) GetCurrentEpoch() uint64 {
	if x != nil { return x.CurrentEpoch }
	return 0
}
func (x *QueryMetabolismStatusResponse) GetNextEpochBlock() uint64 {
	if x != nil { return x.NextEpochBlock }
	return 0
}
func (x *QueryMetabolismStatusResponse) GetRecentRecoveries() uint64 {
	if x != nil { return x.RecentRecoveries }
	return 0
}
func (x *QueryMetabolismStatusResponse) GetRecentExtinctions() uint64 {
	if x != nil { return x.RecentExtinctions }
	return 0
}
```

**Step 2: Add query handler to grpc_query.go**

Add after FactsAtRisk function (after line 405):

```go
// MetabolismStatus returns aggregate metabolism health statistics.
func (q *queryServer) MetabolismStatus(ctx context.Context, req *types.QueryMetabolismStatusRequest) (*types.QueryMetabolismStatusResponse, error) {
	params, err := q.keeper.GetParams(ctx)
	if err != nil {
		return nil, err
	}

	sdkCtx := sdk.UnwrapSDKContext(ctx)
	height := uint64(sdkCtx.BlockHeight())

	var totalFacts, activeCount, atRiskCount, expiredCount, prunedCount uint64
	var totalEnergy uint64

	q.keeper.IterateFacts(ctx, func(fact *types.Fact) bool {
		totalFacts++
		totalEnergy += fact.Energy
		switch fact.Status {
		case types.FactStatus_FACT_STATUS_VERIFIED, types.FactStatus_FACT_STATUS_ACTIVE, types.FactStatus_FACT_STATUS_PROVISIONAL:
			activeCount++
		case types.FactStatus_FACT_STATUS_AT_RISK:
			atRiskCount++
		case types.FactStatus_FACT_STATUS_EXPIRED:
			expiredCount++
		case types.FactStatus_FACT_STATUS_PRUNED:
			prunedCount++
		}
		return false
	})

	avgEnergy := uint64(0)
	if totalFacts > 0 {
		avgEnergy = totalEnergy / totalFacts
	}

	currentEpoch := uint64(0)
	nextEpochBlock := uint64(0)
	if params.FitnessEpochBlocks > 0 {
		currentEpoch = height / params.FitnessEpochBlocks
		nextEpochBlock = (currentEpoch + 1) * params.FitnessEpochBlocks
	}

	return &types.QueryMetabolismStatusResponse{
		TotalFacts:        totalFacts,
		ActiveCount:       activeCount,
		AtRiskCount:       atRiskCount,
		ExpiredCount:      expiredCount,
		PrunedCount:       prunedCount,
		AvgEnergy:         avgEnergy,
		CurrentEpoch:      currentEpoch,
		NextEpochBlock:    nextEpochBlock,
	}, nil
}
```

**Step 3: Add CLI command to query.go**

Add a new command function:

```go
func NewQueryMetabolismStatusCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "metabolism-status",
		Short: "Query aggregate metabolism health statistics",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientQueryContext(cmd)
			if err != nil {
				return err
			}

			req := &types.QueryMetabolismStatusRequest{}
			resp := &types.QueryMetabolismStatusResponse{}

			if err := clientCtx.Invoke(cmd.Context(), "/zerone.knowledge.v1.Query/MetabolismStatus", req, resp); err != nil {
				return err
			}

			return clientCtx.PrintObjectLegacy(resp)
		},
	}
	flags.AddQueryFlagsToCmd(cmd)
	return cmd
}
```

Register it in `GetQueryCmd()` — add `NewQueryMetabolismStatusCmd()` to the command list alongside the other commands.

**Step 4: Write test for metabolism dashboard query**

```go
func TestMetabolismStatus_Query(t *testing.T) {
	k, ctx := setupKnowledgeTest(t)

	// Create facts in various states
	active1 := makeEnergyFact("fact-ms-a1", "Active fact one content!!", "physics", 500_000, types.FactStatus_FACT_STATUS_ACTIVE)
	active2 := makeEnergyFact("fact-ms-a2", "Active fact two content!!", "physics", 700_000, types.FactStatus_FACT_STATUS_VERIFIED)
	atRisk := makeEnergyFact("fact-ms-ar", "At risk fact for query!!!", "physics", 100_000, types.FactStatus_FACT_STATUS_AT_RISK)
	atRisk.AtRiskSinceEpoch = 1
	expired := makeEnergyFact("fact-ms-ex", "Expired fact for query!!!", "physics", 5_000, types.FactStatus_FACT_STATUS_EXPIRED)
	expired.AtRiskSinceEpoch = 1

	for _, f := range []*types.Fact{active1, active2, atRisk, expired} {
		require.NoError(t, k.SetFact(ctx, f))
	}

	qs := keeper.NewQueryServerImpl(k)
	resp, err := qs.MetabolismStatus(ctx, &types.QueryMetabolismStatusRequest{})
	require.NoError(t, err)

	require.Equal(t, uint64(4), resp.TotalFacts)
	require.Equal(t, uint64(2), resp.ActiveCount)
	require.Equal(t, uint64(1), resp.AtRiskCount)
	require.Equal(t, uint64(1), resp.ExpiredCount)
	// Avg energy = (500K + 700K + 100K + 5K) / 4 = 326,250
	require.Equal(t, uint64(326_250), resp.AvgEnergy)
}
```

**Step 5: Run tests**

Run: `go test ./x/knowledge/keeper/ -run "TestMetabolismStatus_|TestMetabolism_" -v`
Expected: All PASS

**Step 6: Commit**

```
feat(knowledge): add metabolism dashboard query and CLI (R28-4)
```

---

### Task 8: Update Existing Tests + Full Build Verification

**Files:**
- Modify: `x/knowledge/keeper/metabolism_test.go` (update all test values for 1M scale)
- All files from previous tasks

**Step 1: Run full knowledge module tests**

Run: `go test ./x/knowledge/... -v -count=1 2>&1 | tail -50`
Expected: Identify any remaining failures from scale change

**Step 2: Fix any broken tests outside metabolism_test.go**

Search for hardcoded energy values in other test files:

```
grep -rn "Energy.*10_000\|EnergyCap.*10_000\|energy.*5000\|energy.*10000" --include="*_test.go" x/knowledge/
```

Update all references to use 1M-scale values.

**Step 3: Run full build**

Run: `go build ./...`
Expected: Clean compile

**Step 4: Run all tests**

Run: `go test ./x/knowledge/... -v -count=1`
Expected: All PASS

**Step 5: Commit**

```
test(knowledge): update all tests for 1M energy scale (R28-4)
```

---

### Task 9: Final Integration Test + Commit

**Files:**
- Modify: `x/knowledge/keeper/metabolism_test.go` (add integration test)

**Step 1: Write full lifecycle integration test**

```go
func TestMetabolism_FullLifecycle(t *testing.T) {
	k, ctx := setupKnowledgeTest(t)

	// 1. Create a fact with initial energy
	fact := makeEnergyFact("fact-life", "Full lifecycle test fact!!", "physics", 500_000, types.FactStatus_FACT_STATUS_VERIFIED)
	require.NoError(t, k.SetFact(ctx, fact))

	// 2. Drain over several epochs — should eventually become AT_RISK
	for epoch := uint64(1); epoch <= 25; epoch++ {
		require.NoError(t, k.ProcessMetabolism(ctx, epoch))
	}
	updated, _ := k.GetFact(ctx, "fact-life")
	require.Equal(t, types.FactStatus_FACT_STATUS_AT_RISK, updated.Status,
		"fact should be AT_RISK after sustained drain")

	// 3. Patronage saves it
	ctx = ctx.WithBlockHeader(cmtproto.Header{Height: 500})
	k.ApplyPatronageEnergyBoost(ctx, updated, 50_000) // 50 epochs of patronage
	updated, _ = k.GetFact(ctx, "fact-life")
	require.Equal(t, types.FactStatus_FACT_STATUS_ACTIVE, updated.Status,
		"patronage should recover fact to ACTIVE")

	// 4. Verify confidence never exceeds cap
	params, _ := k.GetParams(ctx)
	require.LessOrEqual(t, updated.Confidence, params.MaxConfidence)
}
```

**Step 2: Run the integration test**

Run: `go test ./x/knowledge/keeper/ -run TestMetabolism_FullLifecycle -v`
Expected: PASS

**Step 3: Run full module test suite one last time**

Run: `go test ./x/knowledge/... -count=1`
Expected: All PASS

**Step 4: Final commit**

```
test(knowledge): add full lifecycle integration test (R28-4)
```

---

## Execution Summary

| Task | Description | Files |
|------|-------------|-------|
| 1 | Rescale energy params + add threshold params | genesis.go, genesis.pb.go |
| 2 | Multi-level status transitions | metabolism.go, metabolism_test.go |
| 3 | Unified lifecycle events | metabolism.go, metabolism_test.go |
| 4 | Immediate patronage energy recovery | msg_server.go, metabolism.go, metabolism_test.go |
| 5 | Fix confidence cap (MsgAddFact bypass + clampConfidence) | confidence.go, msg_server.go, rounds.go, metabolism_test.go |
| 6 | Activate confidence growth | fitness.go, metabolism_test.go |
| 7 | Metabolism dashboard query + CLI | query.pb.go, grpc_query.go, query.go, metabolism_test.go |
| 8 | Update all existing tests for 1M scale | metabolism_test.go, other test files |
| 9 | Full lifecycle integration test | metabolism_test.go |
