# R29-2 — Epistemic Temperature Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add per-domain epistemic temperature that couples the conformity/vindication systems (yin) with confidence growth/caps (yang), so that unchallenged consensus slows confidence growth and successful dissent earns faster growth.

**Architecture:** Store a `DomainEpistemicState` per domain (JSON at prefix `0x53`). At each fitness epoch, update temperature based on conformity streaks (cooling) and vindication events (heating), with natural decay toward neutral. Temperature modulates confidence caps via the existing `ClampConfidence` method and growth rates via a new `AdvanceConfidence` method that runs at `ConfidenceGrowthEpoch` intervals.

**Tech Stack:** Go 1.24, Cosmos SDK v0.50.15, existing knowledge module patterns (JSON state, BPS scale, deterministic integer math)

---

### Task 1: Add DomainEpistemicState type and store keys

**Files:**
- Modify: `x/knowledge/types/diversity.go` (add struct at end)
- Modify: `x/knowledge/types/keys.go` (add prefix + key constructors)

**Step 1: Write the failing test**

Create: `x/knowledge/keeper/epistemic_temperature_test.go`

```go
package keeper

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/zerone-chain/zerone/x/knowledge/types"
)

func TestEpistemicState_KeyConstruction(t *testing.T) {
	key := types.EpistemicStateKey("mathematics")
	require.Equal(t, byte(0x53), key[0])
	require.Contains(t, string(key[1:]), "mathematics")
}
```

**Step 2: Run test to verify it fails**

Run: `cd /Users/yournameisai/Desktop/zerone && go test ./x/knowledge/keeper/ -run TestEpistemicState_KeyConstruction -v -count=1`
Expected: FAIL — `types.EpistemicStateKey` undefined

**Step 3: Implement types and keys**

Add to `x/knowledge/types/diversity.go` (after ConformityStreak):
```go
// DomainEpistemicState tracks epistemic temperature for a knowledge domain (R29-2).
type DomainEpistemicState struct {
	Domain                string `json:"domain"`
	Temperature           uint64 `json:"temperature"`             // BPS: 500_000 = neutral
	ConformityStreak      uint64 `json:"conformity_streak"`       // consecutive high-conformity epochs
	VindicationCount      uint64 `json:"vindication_count"`       // vindications counted in current window
	LastTemperatureUpdate uint64 `json:"last_temperature_update"` // block height
}
```

Add to `x/knowledge/types/keys.go` in the const block (after `VerificationThresholdOverrideKeyPrefix`):
```go
	// ─── Epistemic temperature (R29-2) ─────────────────────────────────
	EpistemicStatePrefix = []byte{0x53} // 0x53 | domain → DomainEpistemicState (JSON)
```

Add key constructor at end of `x/knowledge/types/keys.go`:
```go
// EpistemicStateKey returns the store key for a domain's epistemic state.
func EpistemicStateKey(domain string) []byte {
	return append(EpistemicStatePrefix, []byte(domain)...)
}
```

**Step 4: Run test to verify it passes**

Run: `cd /Users/yournameisai/Desktop/zerone && go test ./x/knowledge/keeper/ -run TestEpistemicState_KeyConstruction -v -count=1`
Expected: PASS

**Step 5: Commit**

```bash
git add x/knowledge/types/diversity.go x/knowledge/types/keys.go x/knowledge/keeper/epistemic_temperature_test.go
git commit -m "feat(knowledge): add DomainEpistemicState type and store key (R29-2)"
```

---

### Task 2: Add epistemic temperature parameters

**Files:**
- Modify: `proto/zerone/knowledge/v1/genesis.proto` (add fields 115-120)
- Modify: `x/knowledge/types/genesis.pb.go` (add fields to Params struct + marshal/unmarshal)
- Modify: `x/knowledge/types/genesis.go` (add defaults)

**Step 1: Write the failing test**

Add to `x/knowledge/keeper/epistemic_temperature_test.go`:
```go
func TestEpistemicParams_Defaults(t *testing.T) {
	params := types.DefaultParams()
	require.Equal(t, uint64(995_000), params.EpistemicTemperatureDecayBps)
	require.Equal(t, uint64(50_000), params.EpistemicConformityCoolingBps)
	require.Equal(t, uint64(100_000), params.EpistemicVindicationHeatingBps)
	require.Equal(t, uint64(600_000), params.EpistemicColdConfidenceCapBps)
	require.Equal(t, uint64(1_500_000), params.EpistemicHotConfidenceGrowthBps)
	require.Equal(t, uint64(10_000), params.EpistemicTemperatureWindowBlocks)
}
```

**Step 2: Run test to verify it fails**

Run: `cd /Users/yournameisai/Desktop/zerone && go test ./x/knowledge/keeper/ -run TestEpistemicParams_Defaults -v -count=1`
Expected: FAIL — `params.EpistemicTemperatureDecayBps` undefined

**Step 3: Add proto fields and Go implementation**

Add to `proto/zerone/knowledge/v1/genesis.proto` inside `Params` (after field 114):
```protobuf
  // ─── Epistemic temperature (R29-2) ──────────────────────────────────
  uint64 epistemic_temperature_decay_bps        = 115; // Per-epoch decay toward neutral (default: 995,000 = 99.5%)
  uint64 epistemic_conformity_cooling_bps       = 116; // Cooling per high-conformity epoch (default: 50,000 = 5%)
  uint64 epistemic_vindication_heating_bps      = 117; // Heating per vindication event (default: 100,000 = 10%)
  uint64 epistemic_cold_confidence_cap_bps      = 118; // Max confidence in cold domains (default: 600,000 = 60%)
  uint64 epistemic_hot_confidence_growth_bps    = 119; // Confidence growth multiplier in hot domains (default: 1,500,000 = 150%)
  uint64 epistemic_temperature_window_blocks    = 120; // Lookback window for vindication counting (default: 10,000)
```

Add fields to `Params` struct in `x/knowledge/types/genesis.pb.go`:
```go
	// ─── Epistemic temperature (R29-2) ──────────────────────────────────
	EpistemicTemperatureDecayBps     uint64 `protobuf:"varint,115,opt,name=epistemic_temperature_decay_bps,json=epistemicTemperatureDecayBps,proto3" json:"epistemic_temperature_decay_bps,omitempty"`
	EpistemicConformityCoolingBps    uint64 `protobuf:"varint,116,opt,name=epistemic_conformity_cooling_bps,json=epistemicConformityCoolingBps,proto3" json:"epistemic_conformity_cooling_bps,omitempty"`
	EpistemicVindicationHeatingBps   uint64 `protobuf:"varint,117,opt,name=epistemic_vindication_heating_bps,json=epistemicVindicationHeatingBps,proto3" json:"epistemic_vindication_heating_bps,omitempty"`
	EpistemicColdConfidenceCapBps    uint64 `protobuf:"varint,118,opt,name=epistemic_cold_confidence_cap_bps,json=epistemicColdConfidenceCapBps,proto3" json:"epistemic_cold_confidence_cap_bps,omitempty"`
	EpistemicHotConfidenceGrowthBps  uint64 `protobuf:"varint,119,opt,name=epistemic_hot_confidence_growth_bps,json=epistemicHotConfidenceGrowthBps,proto3" json:"epistemic_hot_confidence_growth_bps,omitempty"`
	EpistemicTemperatureWindowBlocks uint64 `protobuf:"varint,120,opt,name=epistemic_temperature_window_blocks,json=epistemicTemperatureWindowBlocks,proto3" json:"epistemic_temperature_window_blocks,omitempty"`
```

Also add getter methods to `genesis.pb.go` (follow existing pattern):
```go
func (m *Params) GetEpistemicTemperatureDecayBps() uint64 { if m != nil { return m.EpistemicTemperatureDecayBps }; return 0 }
func (m *Params) GetEpistemicConformityCoolingBps() uint64 { if m != nil { return m.EpistemicConformityCoolingBps }; return 0 }
func (m *Params) GetEpistemicVindicationHeatingBps() uint64 { if m != nil { return m.EpistemicVindicationHeatingBps }; return 0 }
func (m *Params) GetEpistemicColdConfidenceCapBps() uint64 { if m != nil { return m.EpistemicColdConfidenceCapBps }; return 0 }
func (m *Params) GetEpistemicHotConfidenceGrowthBps() uint64 { if m != nil { return m.EpistemicHotConfidenceGrowthBps }; return 0 }
func (m *Params) GetEpistemicTemperatureWindowBlocks() uint64 { if m != nil { return m.EpistemicTemperatureWindowBlocks }; return 0 }
```

Add marshal/unmarshal support in `genesis.pb.go` — find the `Params` `MarshalToSizedBuffer` and `Unmarshal` methods and add entries for fields 115-120 following the pattern of fields 110-114 (simple varint encode/decode).

Add defaults to `x/knowledge/types/genesis.go` in `DefaultParams()`:
```go
		// ─── Epistemic temperature (R29-2) ───────────────────────────────
		EpistemicTemperatureDecayBps:     995_000,   // 99.5% per epoch — slow drift to neutral
		EpistemicConformityCoolingBps:    50_000,    // -5% per high-conformity epoch
		EpistemicVindicationHeatingBps:   100_000,   // +10% per vindication event
		EpistemicColdConfidenceCapBps:    600_000,   // 60% max confidence in cold domains
		EpistemicHotConfidenceGrowthBps:  1_500_000, // 150% growth rate in hot domains
		EpistemicTemperatureWindowBlocks: 10_000,    // lookback window for vindication counting
```

**Step 4: Run test to verify it passes**

Run: `cd /Users/yournameisai/Desktop/zerone && go test ./x/knowledge/keeper/ -run TestEpistemicParams_Defaults -v -count=1`
Expected: PASS

**Step 5: Run full build to ensure pb.go edits are valid**

Run: `cd /Users/yournameisai/Desktop/zerone && go build ./...`
Expected: SUCCESS

**Step 6: Commit**

```bash
git add proto/zerone/knowledge/v1/genesis.proto x/knowledge/types/genesis.pb.go x/knowledge/types/genesis.go
git commit -m "feat(knowledge): add epistemic temperature governance params (R29-2)"
```

---

### Task 3: Add Get/Set DomainEpistemicState keeper methods

**Files:**
- Create: `x/knowledge/keeper/epistemic_temperature.go`
- Modify: `x/knowledge/keeper/epistemic_temperature_test.go`

**Step 1: Write the failing test**

Add to `x/knowledge/keeper/epistemic_temperature_test.go`:
```go
func TestEpistemicState_SetGetRoundTrip(t *testing.T) {
	k, ctx := setupKnowledgeTest(t)

	state := &types.DomainEpistemicState{
		Domain:                "mathematics",
		Temperature:           500_000,
		ConformityStreak:      3,
		VindicationCount:      2,
		LastTemperatureUpdate: 100,
	}
	require.NoError(t, k.SetDomainEpistemicState(ctx, state))

	got, found, err := k.GetDomainEpistemicState(ctx, "mathematics")
	require.NoError(t, err)
	require.True(t, found)
	require.Equal(t, uint64(500_000), got.Temperature)
	require.Equal(t, uint64(3), got.ConformityStreak)
	require.Equal(t, uint64(2), got.VindicationCount)
	require.Equal(t, uint64(100), got.LastTemperatureUpdate)
}

func TestEpistemicState_NotFound(t *testing.T) {
	k, ctx := setupKnowledgeTest(t)

	_, found, err := k.GetDomainEpistemicState(ctx, "nonexistent")
	require.NoError(t, err)
	require.False(t, found)
}
```

**Step 2: Run test to verify it fails**

Run: `cd /Users/yournameisai/Desktop/zerone && go test ./x/knowledge/keeper/ -run TestEpistemicState_SetGet -v -count=1`
Expected: FAIL — `k.SetDomainEpistemicState` undefined

**Step 3: Implement CRUD methods**

Create `x/knowledge/keeper/epistemic_temperature.go`:
```go
package keeper

import (
	"context"
	"encoding/json"
	"fmt"

	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/zerone-chain/zerone/x/knowledge/types"
)

// ─── CRUD: DomainEpistemicState (R29-2) ──────────────────────────────────────

// SetDomainEpistemicState stores the epistemic temperature state for a domain.
func (k Keeper) SetDomainEpistemicState(ctx context.Context, state *types.DomainEpistemicState) error {
	store := k.storeService.OpenKVStore(ctx)
	bz, err := json.Marshal(state)
	if err != nil {
		return fmt.Errorf("failed to marshal DomainEpistemicState: %w", err)
	}
	return store.Set(types.EpistemicStateKey(state.Domain), bz)
}

// GetDomainEpistemicState retrieves the epistemic temperature state for a domain.
func (k Keeper) GetDomainEpistemicState(ctx context.Context, domain string) (types.DomainEpistemicState, bool, error) {
	store := k.storeService.OpenKVStore(ctx)
	bz, err := store.Get(types.EpistemicStateKey(domain))
	if err != nil {
		return types.DomainEpistemicState{}, false, err
	}
	if bz == nil {
		return types.DomainEpistemicState{}, false, nil
	}
	var state types.DomainEpistemicState
	if err := json.Unmarshal(bz, &state); err != nil {
		return types.DomainEpistemicState{}, false, fmt.Errorf("failed to unmarshal DomainEpistemicState: %w", err)
	}
	return state, true, nil
}

// GetOrInitDomainEpistemicState returns existing state or creates neutral state.
func (k Keeper) GetOrInitDomainEpistemicState(ctx context.Context, domain string) (types.DomainEpistemicState, error) {
	state, found, err := k.GetDomainEpistemicState(ctx, domain)
	if err != nil {
		return types.DomainEpistemicState{}, err
	}
	if !found {
		return types.DomainEpistemicState{
			Domain:      domain,
			Temperature: NeutralBPS, // 500,000 = neutral
		}, nil
	}
	return state, nil
}
```

**Step 4: Run test to verify it passes**

Run: `cd /Users/yournameisai/Desktop/zerone && go test ./x/knowledge/keeper/ -run TestEpistemicState_ -v -count=1`
Expected: PASS (both tests)

**Step 5: Commit**

```bash
git add x/knowledge/keeper/epistemic_temperature.go x/knowledge/keeper/epistemic_temperature_test.go
git commit -m "feat(knowledge): add Get/Set DomainEpistemicState keeper methods (R29-2)"
```

---

### Task 4: Add CountVindicationsInWindow

**Files:**
- Modify: `x/knowledge/keeper/epistemic_temperature.go`
- Modify: `x/knowledge/keeper/epistemic_temperature_test.go`

**Step 1: Write the failing test**

Add to `x/knowledge/keeper/epistemic_temperature_test.go`:
```go
func TestCountVindicationsInWindow(t *testing.T) {
	k, ctx := setupKnowledgeTest(t)

	// Create vindication records at various heights for domain "physics"
	// Need facts in the "physics" domain to attach records to
	makeTestFact(t, k, ctx, "f1", "fact one", "physics", "general")
	makeTestFact(t, k, ctx, "f2", "fact two", "physics", "general")
	makeTestFact(t, k, ctx, "f3", "fact three", "mathematics", "general")

	// Records for physics domain facts
	require.NoError(t, k.SetVindicationRecord(ctx, "f1", &types.VindicationRecord{
		Verifier: "v1", FactId: "f1", VindicatedAt: 5000,
	}))
	require.NoError(t, k.SetVindicationRecord(ctx, "f1", &types.VindicationRecord{
		Verifier: "v2", FactId: "f1", VindicatedAt: 6000,
	}))
	require.NoError(t, k.SetVindicationRecord(ctx, "f2", &types.VindicationRecord{
		Verifier: "v3", FactId: "f2", VindicatedAt: 9000,
	}))

	// Record for mathematics domain (should not count)
	require.NoError(t, k.SetVindicationRecord(ctx, "f3", &types.VindicationRecord{
		Verifier: "v4", FactId: "f3", VindicatedAt: 8000,
	}))

	// Count vindications for physics within window [5000, 10000]
	count := k.CountVindicationsInWindow(ctx, "physics", 10000, 5000)
	require.Equal(t, uint64(2), count) // f1 and f2 are two distinct vindication EVENTS (fact-level, not per-verifier)

	// Window that excludes early vindications
	count = k.CountVindicationsInWindow(ctx, "physics", 10000, 2000)
	require.Equal(t, uint64(1), count) // Only f2 at height 9000 within [8000, 10000]
}
```

**Step 2: Run test to verify it fails**

Run: `cd /Users/yournameisai/Desktop/zerone && go test ./x/knowledge/keeper/ -run TestCountVindicationsInWindow -v -count=1`
Expected: FAIL — `k.CountVindicationsInWindow` undefined

**Step 3: Implement CountVindicationsInWindow**

CountVindicationsInWindow counts vindication events (fact-level, not per-verifier) in a domain within a block window. A "vindication event" is a distinct factId that has any VindicationRecord with height in the window.

Add to `x/knowledge/keeper/epistemic_temperature.go`:
```go
// CountVindicationsInWindow counts distinct vindication events (disproven facts)
// in the given domain within [currentHeight-windowBlocks, currentHeight].
// A vindication event is a fact that was disproven (has vindication records).
func (k Keeper) CountVindicationsInWindow(ctx context.Context, domain string, currentHeight, windowBlocks uint64) uint64 {
	startHeight := uint64(0)
	if currentHeight > windowBlocks {
		startHeight = currentHeight - windowBlocks
	}

	// Iterate all facts in the domain and check for vindication records in window.
	// A vindication is per-fact (one disproven fact = one vindication event),
	// regardless of how many minority voters were vindicated.
	count := uint64(0)
	k.IterateFactsByDomain(ctx, domain, func(fact *types.Fact) bool {
		records := k.GetVindicationRecordsForFact(ctx, fact.Id)
		for _, rec := range records {
			if rec.VindicatedAt >= startHeight && rec.VindicatedAt <= currentHeight {
				count++ // Count this fact as one vindication event
				break   // Don't double-count multiple verifiers on same fact
			}
		}
		return false
	})
	return count
}
```

**Step 4: Run test to verify it passes**

Run: `cd /Users/yournameisai/Desktop/zerone && go test ./x/knowledge/keeper/ -run TestCountVindicationsInWindow -v -count=1`
Expected: PASS

Note: The test may need adjustment depending on how `makeTestFact` works and whether `GetVindicationRecordsForFact` returns the records correctly. Debug and fix as needed.

**Step 5: Commit**

```bash
git add x/knowledge/keeper/epistemic_temperature.go x/knowledge/keeper/epistemic_temperature_test.go
git commit -m "feat(knowledge): add CountVindicationsInWindow for epistemic temperature (R29-2)"
```

---

### Task 5: Implement UpdateEpistemicTemperature

**Files:**
- Modify: `x/knowledge/keeper/epistemic_temperature.go`
- Modify: `x/knowledge/keeper/epistemic_temperature_test.go`

**Step 1: Write the failing tests**

Add to `x/knowledge/keeper/epistemic_temperature_test.go`:
```go
func TestUpdateEpistemicTemperature_DecayToNeutral(t *testing.T) {
	k, ctx := setupKnowledgeTest(t)
	ctx = advanceBlocks(ctx, 10_000) // at first fitness epoch

	// Start hot
	require.NoError(t, k.SetDomainEpistemicState(ctx, &types.DomainEpistemicState{
		Domain:      "physics",
		Temperature: 800_000,
	}))

	// Update — should decay toward 500,000
	require.NoError(t, k.UpdateEpistemicTemperature(ctx, "physics"))

	state, found, err := k.GetDomainEpistemicState(ctx, "physics")
	require.NoError(t, err)
	require.True(t, found)
	// Decay: 800,000 → neutral + (800,000 - 500,000) * 995,000 / 1,000,000
	// = 500,000 + 300,000 * 0.995 = 500,000 + 298,500 = 798,500
	require.Equal(t, uint64(798_500), state.Temperature)
}

func TestUpdateEpistemicTemperature_ConformityCooling(t *testing.T) {
	k, ctx := setupKnowledgeTest(t)
	ctx = advanceBlocks(ctx, 10_000)

	// Set initial neutral state
	require.NoError(t, k.SetDomainEpistemicState(ctx, &types.DomainEpistemicState{
		Domain:      "physics",
		Temperature: 500_000,
	}))

	// Create low-diversity epoch data (below ConformityAlertThreshold of 50,000)
	epoch := uint64(1)
	require.NoError(t, k.SetDomainDiversity(ctx, "physics", epoch, DomainDiversityRecord{
		Domain:     "physics",
		Epoch:      epoch,
		AvgEntropy: 10_000, // Very low entropy = high conformity
		RoundCount: 5,
	}))

	require.NoError(t, k.UpdateEpistemicTemperature(ctx, "physics"))

	state, found, err := k.GetDomainEpistemicState(ctx, "physics")
	require.NoError(t, err)
	require.True(t, found)
	require.Less(t, state.Temperature, uint64(500_000)) // Cooled below neutral
	require.Equal(t, uint64(1), state.ConformityStreak)
}

func TestUpdateEpistemicTemperature_NewDomainStartsNeutral(t *testing.T) {
	k, ctx := setupKnowledgeTest(t)
	ctx = advanceBlocks(ctx, 10_000)

	// No existing state — should initialize at neutral
	require.NoError(t, k.UpdateEpistemicTemperature(ctx, "new_domain"))

	state, found, err := k.GetDomainEpistemicState(ctx, "new_domain")
	require.NoError(t, err)
	require.True(t, found)
	require.Equal(t, uint64(500_000), state.Temperature)
}
```

**Step 2: Run tests to verify they fail**

Run: `cd /Users/yournameisai/Desktop/zerone && go test ./x/knowledge/keeper/ -run TestUpdateEpistemicTemperature -v -count=1`
Expected: FAIL — `k.UpdateEpistemicTemperature` undefined

**Step 3: Implement UpdateEpistemicTemperature**

Add to `x/knowledge/keeper/epistemic_temperature.go`:
```go
// UpdateEpistemicTemperature recalculates a domain's epistemic temperature.
// Called from BeginBlocker at fitness epoch boundaries.
func (k Keeper) UpdateEpistemicTemperature(ctx context.Context, domain string) error {
	state, err := k.GetOrInitDomainEpistemicState(ctx, domain)
	if err != nil {
		return err
	}
	params, err := k.GetParams(ctx)
	if err != nil {
		return err
	}
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	height := uint64(sdkCtx.BlockHeight())
	neutral := uint64(NeutralBPS)

	// 1. Decay toward neutral (500,000)
	if state.Temperature > neutral {
		diff := state.Temperature - neutral
		state.Temperature = neutral + safeMulDiv(diff, params.EpistemicTemperatureDecayBps, BPS)
	} else if state.Temperature < neutral {
		diff := neutral - state.Temperature
		state.Temperature = neutral - safeMulDiv(diff, params.EpistemicTemperatureDecayBps, BPS)
	}

	// 2. Conformity cooling — check current epoch diversity
	epoch := uint64(0)
	if params.FitnessEpochBlocks > 0 {
		epoch = height / params.FitnessEpochBlocks
	}
	rec, found, err := k.GetDomainDiversity(ctx, domain, epoch)
	if err != nil {
		return err
	}
	if found && rec.RoundCount > 0 && rec.AvgEntropy < params.DiversityConformityAlertThreshold {
		state.ConformityStreak++
		// Scale cooling by streak (capped at 10 for max effect)
		streak := state.ConformityStreak
		if streak > 10 {
			streak = 10
		}
		cooling := safeMulDiv(params.EpistemicConformityCoolingBps, streak, 10)
		if state.Temperature > cooling {
			state.Temperature -= cooling
		} else {
			state.Temperature = 0
		}
	} else {
		state.ConformityStreak = 0
	}

	// 3. Vindication heating
	windowBlocks := params.EpistemicTemperatureWindowBlocks
	if windowBlocks == 0 {
		windowBlocks = 10_000
	}
	recentVindications := k.CountVindicationsInWindow(ctx, domain, height, windowBlocks)
	if recentVindications > state.VindicationCount {
		newVindications := recentVindications - state.VindicationCount
		heating := params.EpistemicVindicationHeatingBps * newVindications
		state.Temperature += heating
		if state.Temperature > BPS {
			state.Temperature = BPS
		}
	}
	state.VindicationCount = recentVindications

	state.LastTemperatureUpdate = height
	return k.SetDomainEpistemicState(ctx, &state)
}
```

**Step 4: Run tests to verify they pass**

Run: `cd /Users/yournameisai/Desktop/zerone && go test ./x/knowledge/keeper/ -run TestUpdateEpistemicTemperature -v -count=1`
Expected: PASS

**Step 5: Commit**

```bash
git add x/knowledge/keeper/epistemic_temperature.go x/knowledge/keeper/epistemic_temperature_test.go
git commit -m "feat(knowledge): implement UpdateEpistemicTemperature core logic (R29-2)"
```

---

### Task 6: Add temperature category helper and event emission

**Files:**
- Modify: `x/knowledge/keeper/epistemic_temperature.go`
- Modify: `x/knowledge/keeper/epistemic_temperature_test.go`

**Step 1: Write the failing test**

Add to `x/knowledge/keeper/epistemic_temperature_test.go`:
```go
func TestTemperatureCategory(t *testing.T) {
	tests := []struct {
		temp     uint64
		expected string
	}{
		{0, "cold"},
		{200_000, "cold"},
		{300_000, "cool"},
		{400_000, "cool"},
		{500_000, "neutral"},
		{600_000, "neutral"},
		{700_000, "neutral"},
		{750_000, "warm"},
		{800_000, "hot"},
		{1_000_000, "hot"},
	}
	for _, tt := range tests {
		require.Equal(t, tt.expected, TemperatureCategory(tt.temp), "temp=%d", tt.temp)
	}
}
```

**Step 2: Run test to verify it fails**

Run: `cd /Users/yournameisai/Desktop/zerone && go test ./x/knowledge/keeper/ -run TestTemperatureCategory -v -count=1`
Expected: FAIL — `TemperatureCategory` undefined

**Step 3: Implement category helper and event**

Add to `x/knowledge/keeper/epistemic_temperature.go`:
```go
// TemperatureCategory returns a human-readable category for a temperature value.
func TemperatureCategory(temp uint64) string {
	switch {
	case temp < 300_000:
		return "cold"
	case temp < 500_000:
		return "cool"
	case temp <= 700_000:
		return "neutral"
	case temp < 800_000:
		return "warm"
	default:
		return "hot"
	}
}

// emitTemperatureEvent emits an event when epistemic temperature is updated.
func (k Keeper) emitTemperatureEvent(ctx context.Context, domain string, state types.DomainEpistemicState) {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	sdkCtx.EventManager().EmitEvent(sdk.NewEvent(
		"zerone.knowledge.epistemic_temperature_changed",
		sdk.NewAttribute("domain", domain),
		sdk.NewAttribute("temperature_bps", fmt.Sprintf("%d", state.Temperature)),
		sdk.NewAttribute("category", TemperatureCategory(state.Temperature)),
		sdk.NewAttribute("conformity_streak", fmt.Sprintf("%d", state.ConformityStreak)),
		sdk.NewAttribute("recent_vindications", fmt.Sprintf("%d", state.VindicationCount)),
	))
}
```

Then add event emission to the end of `UpdateEpistemicTemperature`, just before the return:
```go
	// Emit temperature event
	k.emitTemperatureEvent(ctx, domain, state)

	state.LastTemperatureUpdate = height
	return k.SetDomainEpistemicState(ctx, &state)
```

**Step 4: Run test to verify it passes**

Run: `cd /Users/yournameisai/Desktop/zerone && go test ./x/knowledge/keeper/ -run TestTemperatureCategory -v -count=1`
Expected: PASS

**Step 5: Commit**

```bash
git add x/knowledge/keeper/epistemic_temperature.go x/knowledge/keeper/epistemic_temperature_test.go
git commit -m "feat(knowledge): add temperature category helper and events (R29-2)"
```

---

### Task 7: Confidence cap modulation via ClampConfidence

**Files:**
- Modify: `x/knowledge/keeper/confidence.go` (modify `ClampConfidence`)
- Modify: `x/knowledge/keeper/epistemic_temperature_test.go`

**Step 1: Write the failing test**

Add to `x/knowledge/keeper/epistemic_temperature_test.go`:
```go
func TestClampConfidence_ColdDomainCap(t *testing.T) {
	k, ctx := setupKnowledgeTest(t)

	// Set cold temperature for physics
	require.NoError(t, k.SetDomainEpistemicState(ctx, &types.DomainEpistemicState{
		Domain:      "physics",
		Temperature: 200_000, // Cold
	}))

	// Default MaxConfidence is 880,000, but cold cap is 600,000
	clamped := k.ClampConfidence(ctx, 750_000, "physics")
	require.Equal(t, uint64(600_000), clamped)
}

func TestClampConfidence_HotDomainAllowsHigher(t *testing.T) {
	k, ctx := setupKnowledgeTest(t)

	// Set very hot temperature
	require.NoError(t, k.SetDomainEpistemicState(ctx, &types.DomainEpistemicState{
		Domain:      "physics",
		Temperature: 850_000, // Very hot (> 800,000)
	}))

	// Hot domains get SurvivedChallengeConfidenceCap (880,000) even without surviving a challenge
	clamped := k.ClampConfidence(ctx, 860_000, "physics")
	require.Equal(t, uint64(860_000), clamped) // 860,000 <= 880,000, passes through
}

func TestClampConfidence_NeutralDomainUnchanged(t *testing.T) {
	k, ctx := setupKnowledgeTest(t)

	// No epistemic state (neutral by default)
	clamped := k.ClampConfidence(ctx, 750_000, "physics")
	// Default MaxConfidence is 880,000, so 750,000 passes through normally
	require.Equal(t, uint64(750_000), clamped)
}
```

**Step 2: Run test to verify it fails**

Run: `cd /Users/yournameisai/Desktop/zerone && go test ./x/knowledge/keeper/ -run TestClampConfidence_ -v -count=1`
Expected: FAIL — the cold cap is not applied yet

**Step 3: Add epistemic temperature cap modulation to ClampConfidence**

Modify `ClampConfidence` in `x/knowledge/keeper/confidence.go` to add epistemic temperature modulation after the stratum ceiling but before the global hard cap:

```go
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

	// Apply epistemic temperature cap modulation (R29-2)
	if domain != "" {
		epistemicState, found, err := k.GetDomainEpistemicState(ctx, domain)
		if err == nil && found {
			effectiveCap := params.MaxConfidence
			if effectiveCap == 0 {
				effectiveCap = 880_000
			}

			// Cold domains: lower cap — untested consensus shouldn't be highly confident
			if epistemicState.Temperature < 300_000 && params.EpistemicColdConfidenceCapBps > 0 {
				if params.EpistemicColdConfidenceCapBps < effectiveCap {
					effectiveCap = params.EpistemicColdConfidenceCapBps
				}
			}

			// Very hot domains: allow up to SurvivedChallengeConfidenceCap
			if epistemicState.Temperature > 800_000 && params.SurvivedChallengeConfidenceCap > effectiveCap {
				effectiveCap = params.SurvivedChallengeConfidenceCap
			}

			if confidence > effectiveCap {
				confidence = effectiveCap
			}
			return confidence
		}
	}

	// Apply global hard cap (no epistemic state)
	if params.MaxConfidence > 0 && confidence > params.MaxConfidence {
		confidence = params.MaxConfidence
	}

	return confidence
}
```

**Step 4: Run test to verify it passes**

Run: `cd /Users/yournameisai/Desktop/zerone && go test ./x/knowledge/keeper/ -run TestClampConfidence_ -v -count=1`
Expected: PASS

**Step 5: Run existing confidence tests to ensure no regression**

Run: `cd /Users/yournameisai/Desktop/zerone && go test ./x/knowledge/keeper/ -run TestAggregate -v -count=1`
Expected: PASS (all existing tests still pass)

**Step 6: Commit**

```bash
git add x/knowledge/keeper/confidence.go x/knowledge/keeper/epistemic_temperature_test.go
git commit -m "feat(knowledge): add epistemic temperature cap modulation to ClampConfidence (R29-2)"
```

---

### Task 8: Implement AdvanceConfidence with temperature growth modulation

**Files:**
- Modify: `x/knowledge/keeper/epistemic_temperature.go`
- Modify: `x/knowledge/keeper/epistemic_temperature_test.go`

**Step 1: Write the failing test**

Add to `x/knowledge/keeper/epistemic_temperature_test.go`:
```go
func TestAdvanceConfidence_NeutralDomain(t *testing.T) {
	k, ctx := setupKnowledgeTest(t)

	// Create a verified fact in physics
	makeTestFact(t, k, ctx, "f1", "verified fact", "physics", "general")
	fact, _ := k.GetFact(ctx, "f1")
	fact.Status = types.FactStatus_FACT_STATUS_VERIFIED
	fact.Confidence = 500_000
	require.NoError(t, k.SetFact(ctx, fact))

	// No epistemic state → neutral → normal growth
	require.NoError(t, k.AdvanceConfidence(ctx))

	updated, _ := k.GetFact(ctx, "f1")
	// Default growth: 11,000 BPS (1.1%) of 500,000 = 5,500
	// New confidence = 500,000 + 5,500 = 505,500
	require.Equal(t, uint64(505_500), updated.Confidence)
}

func TestAdvanceConfidence_HotDomainFasterGrowth(t *testing.T) {
	k, ctx := setupKnowledgeTest(t)

	makeTestFact(t, k, ctx, "f1", "verified fact", "physics", "general")
	fact, _ := k.GetFact(ctx, "f1")
	fact.Status = types.FactStatus_FACT_STATUS_VERIFIED
	fact.Confidence = 500_000
	require.NoError(t, k.SetFact(ctx, fact))

	// Set hot temperature
	require.NoError(t, k.SetDomainEpistemicState(ctx, &types.DomainEpistemicState{
		Domain:      "physics",
		Temperature: 800_000,
	}))

	require.NoError(t, k.AdvanceConfidence(ctx))

	updated, _ := k.GetFact(ctx, "f1")
	// Hot growth: 11,000 * 1,500,000 / 1,000,000 = 16,500 BPS
	// Growth amount: 500,000 * 16,500 / 1,000,000 = 8,250
	// New confidence = 500,000 + 8,250 = 508,250
	require.Equal(t, uint64(508_250), updated.Confidence)
}

func TestAdvanceConfidence_ColdDomainSlowerGrowth(t *testing.T) {
	k, ctx := setupKnowledgeTest(t)

	makeTestFact(t, k, ctx, "f1", "verified fact", "physics", "general")
	fact, _ := k.GetFact(ctx, "f1")
	fact.Status = types.FactStatus_FACT_STATUS_VERIFIED
	fact.Confidence = 500_000
	require.NoError(t, k.SetFact(ctx, fact))

	// Set cold temperature
	require.NoError(t, k.SetDomainEpistemicState(ctx, &types.DomainEpistemicState{
		Domain:      "physics",
		Temperature: 200_000,
	}))

	require.NoError(t, k.AdvanceConfidence(ctx))

	updated, _ := k.GetFact(ctx, "f1")
	// Cold growth: 11,000 * 500,000 / 1,000,000 = 5,500 BPS (50% rate)
	// Growth amount: 500,000 * 5,500 / 1,000,000 = 2,750
	// New confidence = 500,000 + 2,750 = 502,750
	require.Equal(t, uint64(502_750), updated.Confidence)
}
```

**Step 2: Run test to verify it fails**

Run: `cd /Users/yournameisai/Desktop/zerone && go test ./x/knowledge/keeper/ -run TestAdvanceConfidence -v -count=1`
Expected: FAIL — `k.AdvanceConfidence` undefined

**Step 3: Implement AdvanceConfidence**

Add to `x/knowledge/keeper/epistemic_temperature.go`:
```go
// AdvanceConfidence grows confidence for all active/verified facts by
// ConfidenceGrowthPerEpochBps, modulated by epistemic temperature.
// Called from BeginBlocker at ConfidenceGrowthEpoch intervals.
func (k Keeper) AdvanceConfidence(ctx context.Context) error {
	params, err := k.GetParams(ctx)
	if err != nil {
		return err
	}

	baseGrowthRate := params.ConfidenceGrowthPerEpochBps
	if baseGrowthRate == 0 {
		return nil // growth disabled
	}

	// Cache epistemic states per domain to avoid repeated lookups
	domainGrowthRates := make(map[string]uint64)

	k.IterateFacts(ctx, func(fact *types.Fact) bool {
		// Only grow confidence for active/verified facts
		switch fact.Status {
		case types.FactStatus_FACT_STATUS_VERIFIED,
			types.FactStatus_FACT_STATUS_ACTIVE,
			types.FactStatus_FACT_STATUS_PROVISIONAL:
		default:
			return false
		}

		growthRate, ok := domainGrowthRates[fact.Domain]
		if !ok {
			growthRate = baseGrowthRate
			epistemicState, found, err := k.GetDomainEpistemicState(ctx, fact.Domain)
			if err == nil && found {
				// Hot domains: confidence grows faster
				if epistemicState.Temperature > 700_000 && params.EpistemicHotConfidenceGrowthBps > 0 {
					growthRate = safeMulDiv(growthRate, params.EpistemicHotConfidenceGrowthBps, BPS)
				}
				// Cold domains: confidence grows slower (50% rate)
				if epistemicState.Temperature < 300_000 {
					growthRate = safeMulDiv(growthRate, 500_000, BPS)
				}
			}
			domainGrowthRates[fact.Domain] = growthRate
		}

		// Apply growth: confidence += confidence * growthRate / BPS
		growth := safeMulDiv(fact.Confidence, growthRate, BPS)
		if growth == 0 {
			growth = 1 // minimum 1 BPS growth per epoch
		}
		fact.Confidence += growth

		// Clamp to effective cap (includes epistemic temperature modulation)
		fact.Confidence = k.ClampConfidence(ctx, fact.Confidence, fact.Domain)

		if err := k.SetFact(ctx, fact); err != nil {
			return false
		}
		return false
	})

	return nil
}
```

**Step 4: Run test to verify it passes**

Run: `cd /Users/yournameisai/Desktop/zerone && go test ./x/knowledge/keeper/ -run TestAdvanceConfidence -v -count=1`
Expected: PASS

**Step 5: Commit**

```bash
git add x/knowledge/keeper/epistemic_temperature.go x/knowledge/keeper/epistemic_temperature_test.go
git commit -m "feat(knowledge): implement AdvanceConfidence with temperature growth modulation (R29-2)"
```

---

### Task 9: Wire into BeginBlocker

**Files:**
- Modify: `x/knowledge/keeper/phases.go`
- Modify: `x/knowledge/keeper/epistemic_temperature_test.go`

**Step 1: Write the failing test**

Add to `x/knowledge/keeper/epistemic_temperature_test.go`:
```go
func TestBeginBlocker_UpdatesTemperatureAtFitnessEpoch(t *testing.T) {
	k, ctx := setupKnowledgeTest(t)

	// Set a domain with hot temperature
	require.NoError(t, k.SetDomainEpistemicState(ctx, &types.DomainEpistemicState{
		Domain:      "physics",
		Temperature: 800_000,
	}))

	// Advance to a fitness epoch boundary (default FitnessEpochBlocks = 10,000)
	ctx = advanceBlocks(ctx, 10_000)

	err := k.BeginBlocker(ctx)
	require.NoError(t, err)

	// Temperature should have been updated (decayed toward neutral)
	state, found, err := k.GetDomainEpistemicState(ctx, "physics")
	require.NoError(t, err)
	require.True(t, found)
	require.Less(t, state.Temperature, uint64(800_000))
}
```

**Step 2: Run test to verify it fails**

Run: `cd /Users/yournameisai/Desktop/zerone && go test ./x/knowledge/keeper/ -run TestBeginBlocker_UpdatesTemperature -v -count=1`
Expected: FAIL — temperature not updated

**Step 3: Wire UpdateEpistemicTemperature and AdvanceConfidence into BeginBlocker**

Modify `x/knowledge/keeper/phases.go` `BeginBlocker`:

After step 8 (ProcessDiversity), add:
```go
		// 9. Update epistemic temperature for all domains (R29-2)
		k.IterateDomains(ctx, func(domain *types.Domain) bool {
			if dErr := k.UpdateEpistemicTemperature(ctx, domain.Name); dErr != nil {
				k.Logger(ctx).Error("epistemic temperature update failed", "domain", domain.Name, "error", dErr)
			}
			return false
		})
```

Add a separate check for ConfidenceGrowthEpoch (outside the fitness epoch block, but inside the BeginBlocker):

After the fitness epoch block, before the `return nil`, add:
```go
	// Advance fact confidence at ConfidenceGrowthEpoch intervals (R29-2)
	if params.ConfidenceGrowthEpochBlocks > 0 && height > 0 && height%params.ConfidenceGrowthEpochBlocks == 0 {
		if err := k.AdvanceConfidence(ctx); err != nil {
			k.Logger(ctx).Error("confidence growth failed", "error", err)
		}
	}
```

Note: Check the actual param name — it might be `ConfidenceGrowthEpoch` not `ConfidenceGrowthEpochBlocks`. Look at the proto field name in genesis.pb.go for the exact field name.

**Step 4: Run test to verify it passes**

Run: `cd /Users/yournameisai/Desktop/zerone && go test ./x/knowledge/keeper/ -run TestBeginBlocker_UpdatesTemperature -v -count=1`
Expected: PASS

**Step 5: Commit**

```bash
git add x/knowledge/keeper/phases.go x/knowledge/keeper/epistemic_temperature_test.go
git commit -m "feat(knowledge): wire epistemic temperature and confidence growth into BeginBlocker (R29-2)"
```

---

### Task 10: Add EpistemicTemperature query

**Files:**
- Modify: `proto/zerone/knowledge/v1/query.proto` (add RPC + messages)
- Modify: `x/knowledge/types/query.pb.go` (add request/response types)
- Modify: `x/knowledge/keeper/grpc_query.go` (implement query)
- Modify: `x/knowledge/client/cli/query.go` (add CLI command)

**Step 1: Add proto definition** (documentation only, may not regenerate)

Add to `proto/zerone/knowledge/v1/query.proto` in the Query service:
```protobuf
  // EpistemicTemperature queries a domain's epistemic temperature state.
  rpc EpistemicTemperature(QueryEpistemicTemperatureRequest) returns (QueryEpistemicTemperatureResponse) {
    option (google.api.http).get = "/zerone/knowledge/v1/epistemic_temperature/{domain}";
  }
```

Add request/response messages:
```protobuf
message QueryEpistemicTemperatureRequest {
  string domain = 1;
}

message QueryEpistemicTemperatureResponse {
  string domain                    = 1;
  uint64 temperature_bps           = 2;
  string category                  = 3;
  uint64 conformity_streak         = 4;
  uint64 recent_vindications       = 5;
  uint64 effective_confidence_cap  = 6;
  uint64 effective_growth_rate     = 7;
}
```

**Step 2: Add Go types manually**

Since proto regeneration is complex, add the query types manually. Create or modify a file to add the request/response structs that match the proto definition. If `make proto-gen` works, run it. Otherwise, add them manually to `query.pb.go` or create a separate `query_epistemic.go` in the types package.

Practical approach: Add minimal Go types to `x/knowledge/types/diversity.go` (they share the epistemic domain):
```go
// QueryEpistemicTemperatureRequest is the request for the EpistemicTemperature query.
type QueryEpistemicTemperatureRequest struct {
	Domain string `json:"domain"`
}

// QueryEpistemicTemperatureResponse is the response for the EpistemicTemperature query.
type QueryEpistemicTemperatureResponse struct {
	Domain                 string `json:"domain"`
	TemperatureBps         uint64 `json:"temperature_bps"`
	Category               string `json:"category"`
	ConformityStreak       uint64 `json:"conformity_streak"`
	RecentVindications     uint64 `json:"recent_vindications"`
	EffectiveConfidenceCap uint64 `json:"effective_confidence_cap"`
	EffectiveGrowthRate    uint64 `json:"effective_growth_rate"`
}
```

**Step 3: Implement gRPC query server method**

Add to `x/knowledge/keeper/grpc_query.go`:
```go
// EpistemicTemperature queries a domain's epistemic temperature state.
func (q *queryServer) EpistemicTemperature(ctx context.Context, req *types.QueryEpistemicTemperatureRequest) (*types.QueryEpistemicTemperatureResponse, error) {
	if req.Domain == "" {
		return nil, status.Error(codes.InvalidArgument, "domain is required")
	}

	params, err := q.keeper.GetParams(ctx)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	state, err := q.keeper.GetOrInitDomainEpistemicState(ctx, req.Domain)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	// Calculate effective confidence cap
	effectiveCap := params.MaxConfidence
	if effectiveCap == 0 {
		effectiveCap = 880_000
	}
	if state.Temperature < 300_000 && params.EpistemicColdConfidenceCapBps > 0 {
		if params.EpistemicColdConfidenceCapBps < effectiveCap {
			effectiveCap = params.EpistemicColdConfidenceCapBps
		}
	}
	if state.Temperature > 800_000 && params.SurvivedChallengeConfidenceCap > effectiveCap {
		effectiveCap = params.SurvivedChallengeConfidenceCap
	}

	// Calculate effective growth rate
	growthRate := params.ConfidenceGrowthPerEpochBps
	if state.Temperature > 700_000 && params.EpistemicHotConfidenceGrowthBps > 0 {
		growthRate = safeMulDiv(growthRate, params.EpistemicHotConfidenceGrowthBps, BPS)
	}
	if state.Temperature < 300_000 {
		growthRate = safeMulDiv(growthRate, 500_000, BPS)
	}

	return &types.QueryEpistemicTemperatureResponse{
		Domain:                 req.Domain,
		TemperatureBps:         state.Temperature,
		Category:               TemperatureCategory(state.Temperature),
		ConformityStreak:       state.ConformityStreak,
		RecentVindications:     state.VindicationCount,
		EffectiveConfidenceCap: effectiveCap,
		EffectiveGrowthRate:    growthRate,
	}, nil
}
```

Note: This method won't be auto-registered on the gRPC service since it's not in the proto-generated interface. It will be accessible via CLI using `clientCtx.Invoke` with a custom path, or as a direct keeper query. The CLI command below will use a direct keeper approach.

**Step 4: Add CLI command**

Add to `x/knowledge/client/cli/query.go`:

In the `GetQueryCmd()` function, add to the `AddCommand` list:
```go
		NewQueryEpistemicTemperatureCmd(),
```

Add the command function:
```go
// NewQueryEpistemicTemperatureCmd queries a domain's epistemic temperature.
func NewQueryEpistemicTemperatureCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "epistemic-temperature [domain]",
		Short: "Query epistemic temperature for a domain",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientQueryContext(cmd)
			if err != nil {
				return err
			}
			req := &types.QueryEpistemicTemperatureRequest{Domain: args[0]}
			resp := &types.QueryEpistemicTemperatureResponse{}
			if err := clientCtx.Invoke(cmd.Context(), "/zerone.knowledge.v1.Query/EpistemicTemperature", req, resp); err != nil {
				return fmt.Errorf("failed to query epistemic temperature: %w", err)
			}
			return clientCtx.PrintObjectLegacy(resp)
		},
	}
	flags.AddQueryFlagsToCmd(cmd)
	return cmd
}
```

**Step 5: Build to verify compilation**

Run: `cd /Users/yournameisai/Desktop/zerone && go build ./...`
Expected: SUCCESS

**Step 6: Commit**

```bash
git add proto/zerone/knowledge/v1/query.proto x/knowledge/types/diversity.go x/knowledge/keeper/grpc_query.go x/knowledge/client/cli/query.go
git commit -m "feat(knowledge): add EpistemicTemperature query and CLI command (R29-2)"
```

---

### Task 11: Integration tests — full lifecycle

**Files:**
- Modify: `x/knowledge/keeper/epistemic_temperature_test.go`

**Step 1: Write integration test**

Add to `x/knowledge/keeper/epistemic_temperature_test.go`:
```go
func TestEpistemicTemperature_FullCycle(t *testing.T) {
	k, ctx := setupKnowledgeTest(t)

	domain := "physics"

	// 1. New domain starts at neutral (500,000)
	state, err := k.GetOrInitDomainEpistemicState(ctx, domain)
	require.NoError(t, err)
	require.Equal(t, uint64(500_000), state.Temperature)

	// 2. Simulate conformity cooling over 5 epochs
	params, _ := k.GetParams(ctx)
	for i := uint64(1); i <= 5; i++ {
		height := i * params.FitnessEpochBlocks
		ctx = advanceBlocks(ctx, params.FitnessEpochBlocks)

		// Record low-diversity epoch
		require.NoError(t, k.SetDomainDiversity(ctx, domain, i, DomainDiversityRecord{
			Domain:     domain,
			Epoch:      i,
			AvgEntropy: 10_000, // Very low (below 50,000 threshold)
			RoundCount: 3,
		}))

		_ = height // used by advanceBlocks
		require.NoError(t, k.UpdateEpistemicTemperature(ctx, domain))
	}

	state, _, err = k.GetDomainEpistemicState(ctx, domain)
	require.NoError(t, err)
	require.Less(t, state.Temperature, uint64(300_000), "Should be cold after 5 conformity epochs")

	// 3. Cold domain: confidence capped at 600,000
	capped := k.ClampConfidence(ctx, 750_000, domain)
	require.Equal(t, uint64(600_000), capped)

	// 4. Simulate vindication event — create a fact and vindication records
	makeTestFact(t, k, ctx, "f-vind", "disproven fact", domain, "general")
	require.NoError(t, k.SetVindicationRecord(ctx, "f-vind", &types.VindicationRecord{
		Verifier:     "v1",
		FactId:       "f-vind",
		VindicatedAt: uint64(sdk.UnwrapSDKContext(ctx).BlockHeight()),
		DisprovenBy:  "f-new",
	}))

	// Update temperature — vindication should heat it
	ctx = advanceBlocks(ctx, params.FitnessEpochBlocks)
	// Add diversity data for this epoch to avoid conformity cooling
	epoch := uint64(sdk.UnwrapSDKContext(ctx).BlockHeight()) / params.FitnessEpochBlocks
	require.NoError(t, k.SetDomainDiversity(ctx, domain, epoch, DomainDiversityRecord{
		Domain:     domain,
		Epoch:      epoch,
		AvgEntropy: 500_000, // Healthy diversity
		RoundCount: 3,
	}))
	require.NoError(t, k.UpdateEpistemicTemperature(ctx, domain))

	state, _, err = k.GetDomainEpistemicState(ctx, domain)
	require.NoError(t, err)
	require.Greater(t, state.Temperature, uint64(300_000), "Vindication should have heated the domain")

	// 5. After many epochs with no events, temperature drifts back to neutral
	for i := 0; i < 50; i++ {
		ctx = advanceBlocks(ctx, params.FitnessEpochBlocks)
		ep := uint64(sdk.UnwrapSDKContext(ctx).BlockHeight()) / params.FitnessEpochBlocks
		require.NoError(t, k.SetDomainDiversity(ctx, domain, ep, DomainDiversityRecord{
			Domain:     domain,
			Epoch:      ep,
			AvgEntropy: 500_000, // Healthy
			RoundCount: 3,
		}))
		require.NoError(t, k.UpdateEpistemicTemperature(ctx, domain))
	}

	state, _, err = k.GetDomainEpistemicState(ctx, domain)
	require.NoError(t, err)
	diff := int64(state.Temperature) - 500_000
	if diff < 0 {
		diff = -diff
	}
	require.Less(t, diff, int64(10_000), "Temperature should be near neutral after many quiet epochs")
}
```

**Step 2: Run integration test**

Run: `cd /Users/yournameisai/Desktop/zerone && go test ./x/knowledge/keeper/ -run TestEpistemicTemperature_FullCycle -v -count=1`
Expected: PASS

**Step 3: Run all epistemic temperature tests**

Run: `cd /Users/yournameisai/Desktop/zerone && go test ./x/knowledge/keeper/ -run "TestEpistemicState_|TestEpistemicParams_|TestUpdateEpistemicTemperature|TestTemperatureCategory|TestClampConfidence_|TestAdvanceConfidence|TestBeginBlocker_UpdatesTemperature|TestCountVindications|TestEpistemicTemperature_FullCycle" -v -count=1`
Expected: All PASS

**Step 4: Run full knowledge module tests for regression check**

Run: `cd /Users/yournameisai/Desktop/zerone && go test ./x/knowledge/... -count=1 -timeout 300s`
Expected: All PASS

**Step 5: Commit**

```bash
git add x/knowledge/keeper/epistemic_temperature_test.go
git commit -m "test(knowledge): add epistemic temperature integration tests (R29-2)"
```

---

### Task 12: Final build verification and summary commit

**Step 1: Full build**

Run: `cd /Users/yournameisai/Desktop/zerone && go build ./...`
Expected: SUCCESS

**Step 2: Full test suite**

Run: `cd /Users/yournameisai/Desktop/zerone && go test ./x/knowledge/... -count=1 -timeout 300s`
Expected: All PASS

**Step 3: Summary review**

Files created:
- `x/knowledge/keeper/epistemic_temperature.go` — Core keeper logic (CRUD, temperature update, confidence growth, events)
- `x/knowledge/keeper/epistemic_temperature_test.go` — Unit + integration tests

Files modified:
- `x/knowledge/types/diversity.go` — DomainEpistemicState struct + query types
- `x/knowledge/types/keys.go` — EpistemicStatePrefix (0x53) + key constructor
- `x/knowledge/types/genesis.go` — 6 new default params
- `x/knowledge/types/genesis.pb.go` — 6 new Params fields + getters + marshal/unmarshal
- `proto/zerone/knowledge/v1/genesis.proto` — Param fields 115-120
- `proto/zerone/knowledge/v1/query.proto` — EpistemicTemperature RPC + messages
- `x/knowledge/keeper/confidence.go` — ClampConfidence epistemic cap modulation
- `x/knowledge/keeper/phases.go` — BeginBlocker wiring
- `x/knowledge/keeper/grpc_query.go` — EpistemicTemperature query handler
- `x/knowledge/client/cli/query.go` — epistemic-temperature CLI command
