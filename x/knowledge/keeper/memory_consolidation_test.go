package keeper_test

import (
	"context"
	"testing"

	sdkmath "cosmossdk.io/math"
	"github.com/stretchr/testify/require"

	"github.com/zerone-chain/zerone/x/knowledge/keeper"
	"github.com/zerone-chain/zerone/x/knowledge/types"
)

// ─── Helpers ────────────────────────────────────────────────────────────────

func setupConsolidationTest(t *testing.T) (keeper.Keeper, context.Context) {
	t.Helper()
	k, ctx := setupKeeper(t)
	setupDefaultDomains(t, k, ctx)
	// Set fast consolidation params for testing.
	require.NoError(t, k.SetConsolidationParams(ctx, types.ConsolidationParams{
		ConsolidationInterval:      10,
		ActiveMinActivations:       3,
		ConsolidatedMinActivations: 8,
		ConsolidatedMinSpacing:     "0.400000000000000000",
		ConsolidatedMinPerformance: "0.500000000000000000",
		CanonicalMinActivations:    15,
		CanonicalMinStrength:       "0.600000000000000000",
		CanonicalMinAge:            20,
		HebbianThreshold:           3,
	}))
	return k, ctx
}

// ─── Test: Record Activation — Basic ────────────────────────────────────────

func TestRecordActivation_Basic(t *testing.T) {
	k, ctx := setupConsolidationTest(t)

	// Record first activation.
	delta := sdkmath.LegacyNewDecWithPrec(1, 1) // +0.1 model improvement
	err := k.RecordActivation(ctx, "tdu-hello", 1, delta)
	require.NoError(t, err)

	record, found := k.GetActivationRecord(ctx, "tdu-hello")
	require.True(t, found)
	require.Equal(t, uint64(1), record.TotalActivations)
	require.Equal(t, uint64(1), record.UniqueCycles)
	require.Equal(t, uint64(1), record.FirstActivation)
	require.Equal(t, uint64(1), record.LastActivation)
	require.Equal(t, uint64(1), record.PositiveOutcomes)
	require.Equal(t, uint64(0), record.NegativeOutcomes)
	require.Equal(t, int(types.MemoryTierWorking), record.MemoryTier)
}

// ─── Test: Record Activation — Spaced Repetition ────────────────────────────

func TestRecordActivation_SpacedRepetition(t *testing.T) {
	k, ctx := setupConsolidationTest(t)

	delta := sdkmath.LegacyNewDecWithPrec(5, 2) // +0.05

	// Activate across different cycles (well-spaced).
	for cycle := uint64(1); cycle <= 5; cycle++ {
		err := k.RecordActivation(ctx, "tdu-spaced", cycle, delta)
		require.NoError(t, err)
	}

	record, found := k.GetActivationRecord(ctx, "tdu-spaced")
	require.True(t, found)
	require.Equal(t, uint64(5), record.TotalActivations)
	require.Equal(t, uint64(5), record.UniqueCycles)

	// Perfect spacing: unique_cycles / total_activations = 1.0
	spacing := record.GetSpacingFactor()
	require.True(t, spacing.Equal(sdkmath.LegacyOneDec()), "spacing should be 1.0, got %s", spacing)
}

func TestRecordActivation_CrammedRepetition(t *testing.T) {
	k, ctx := setupConsolidationTest(t)

	delta := sdkmath.LegacyNewDecWithPrec(5, 2) // +0.05

	// 5 activations all in the same cycle (cramming).
	for i := 0; i < 5; i++ {
		err := k.RecordActivation(ctx, "tdu-crammed", 1, delta)
		require.NoError(t, err)
	}

	record, found := k.GetActivationRecord(ctx, "tdu-crammed")
	require.True(t, found)
	require.Equal(t, uint64(5), record.TotalActivations)
	require.Equal(t, uint64(1), record.UniqueCycles)

	// Poor spacing: 1/5 = 0.2
	spacing := record.GetSpacingFactor()
	expected := sdkmath.LegacyNewDecWithPrec(2, 1) // 0.2
	require.True(t, spacing.Equal(expected), "spacing should be 0.2, got %s", spacing)
}

// ─── Test: Immediate Promotion — Working → Active ───────────────────────────

func TestImmediatePromotion_WorkingToActive(t *testing.T) {
	k, ctx := setupConsolidationTest(t)

	delta := sdkmath.LegacyNewDecWithPrec(1, 1) // +0.1

	// 2 activations: still Working.
	k.RecordActivation(ctx, "tdu-promote", 1, delta)
	k.RecordActivation(ctx, "tdu-promote", 2, delta)

	record, _ := k.GetActivationRecord(ctx, "tdu-promote")
	require.Equal(t, int(types.MemoryTierWorking), record.MemoryTier)

	// 3rd activation: promoted to Active immediately.
	k.RecordActivation(ctx, "tdu-promote", 3, delta)

	record, _ = k.GetActivationRecord(ctx, "tdu-promote")
	require.Equal(t, int(types.MemoryTierActive), record.MemoryTier)
}

// ─── Test: Consolidation — Active → Consolidated ────────────────────────────

func TestConsolidation_ActiveToConsolidated(t *testing.T) {
	k, ctx := setupConsolidationTest(t)

	delta := sdkmath.LegacyNewDecWithPrec(1, 1) // +0.1 (positive)

	// Create 10 well-spaced activations with positive outcomes.
	for cycle := uint64(1); cycle <= 10; cycle++ {
		k.RecordActivation(ctx, "tdu-consolidate", cycle, delta)
	}

	record, _ := k.GetActivationRecord(ctx, "tdu-consolidate")
	require.Equal(t, int(types.MemoryTierActive), record.MemoryTier) // promoted by immediate check

	// Run consolidation.
	promoted, err := k.RunConsolidation(ctx, 10)
	require.NoError(t, err)
	require.Equal(t, uint64(1), promoted)

	record, _ = k.GetActivationRecord(ctx, "tdu-consolidate")
	require.Equal(t, int(types.MemoryTierConsolidated), record.MemoryTier)
	require.Equal(t, uint64(10), record.ConsolidatedAt)
}

// ─── Test: Consolidation — Poor Performance Blocks Promotion ────────────────

func TestConsolidation_PoorPerformanceBlocks(t *testing.T) {
	k, ctx := setupConsolidationTest(t)

	// Mix of positive and negative outcomes (mostly negative).
	for cycle := uint64(1); cycle <= 10; cycle++ {
		delta := sdkmath.LegacyNewDecWithPrec(-1, 1) // -0.1 (negative)
		if cycle <= 2 {
			delta = sdkmath.LegacyNewDecWithPrec(1, 1) // +0.1 (positive) — only 2 out of 10
		}
		k.RecordActivation(ctx, "tdu-poor", cycle, delta)
	}

	promoted, _ := k.RunConsolidation(ctx, 10)
	require.Equal(t, uint64(0), promoted) // should NOT be promoted — 20% positive < 50% threshold

	record, _ := k.GetActivationRecord(ctx, "tdu-poor")
	require.Equal(t, int(types.MemoryTierActive), record.MemoryTier) // stays Active
}

// ─── Test: Consolidation — Canonical Promotion ──────────────────────────────

func TestConsolidation_CanonicalPromotion(t *testing.T) {
	k, ctx := setupConsolidationTest(t)

	delta := sdkmath.LegacyNewDecWithPrec(2, 1) // +0.2 (strong positive)

	// 20 well-spaced activations, all positive.
	for cycle := uint64(1); cycle <= 20; cycle++ {
		k.RecordActivation(ctx, "tdu-canonical", cycle, delta)
	}

	// First consolidation: Active → Consolidated.
	k.RunConsolidation(ctx, 20)
	record, _ := k.GetActivationRecord(ctx, "tdu-canonical")
	require.Equal(t, int(types.MemoryTierConsolidated), record.MemoryTier)

	// More activations to reach canonical threshold (25 total).
	for cycle := uint64(21); cycle <= 30; cycle++ {
		k.RecordActivation(ctx, "tdu-canonical", cycle, delta)
	}

	// Second consolidation at cycle 30: Consolidated → Canonical (age 29 ≥ 20).
	promoted, _ := k.RunConsolidation(ctx, 30)
	require.GreaterOrEqual(t, promoted, uint64(1))

	record, _ = k.GetActivationRecord(ctx, "tdu-canonical")
	require.Equal(t, int(types.MemoryTierCanonical), record.MemoryTier)
	require.Equal(t, uint64(30), record.CanonicalAt)
}

// ─── Test: Decay Modifier by Tier ───────────────────────────────────────────

func TestDecayModifiedRate(t *testing.T) {
	k, ctx := setupConsolidationTest(t)

	baseDecay := sdkmath.LegacyNewDecWithPrec(2, 2) // 0.02

	// No activation record → full decay.
	rate := k.GetDecayModifiedRate(ctx, "tdu-unknown", baseDecay)
	require.True(t, rate.Equal(baseDecay))

	// Working tier → full decay (1.0×).
	k.SetActivationRecord(ctx, &types.ActivationRecord{
		SampleID: "tdu-working", MemoryTier: int(types.MemoryTierWorking),
	})
	rate = k.GetDecayModifiedRate(ctx, "tdu-working", baseDecay)
	require.True(t, rate.Equal(baseDecay))

	// Active tier → 0.7× decay.
	k.SetActivationRecord(ctx, &types.ActivationRecord{
		SampleID: "tdu-active", MemoryTier: int(types.MemoryTierActive),
	})
	rate = k.GetDecayModifiedRate(ctx, "tdu-active", baseDecay)
	expected := baseDecay.Mul(sdkmath.LegacyNewDecWithPrec(7, 1)) // 0.014
	require.True(t, rate.Equal(expected), "active decay: want %s, got %s", expected, rate)

	// Consolidated tier → 0.2× decay.
	k.SetActivationRecord(ctx, &types.ActivationRecord{
		SampleID: "tdu-consolidated", MemoryTier: int(types.MemoryTierConsolidated),
	})
	rate = k.GetDecayModifiedRate(ctx, "tdu-consolidated", baseDecay)
	expected = baseDecay.Mul(sdkmath.LegacyNewDecWithPrec(2, 1)) // 0.004
	require.True(t, rate.Equal(expected), "consolidated decay: want %s, got %s", expected, rate)

	// Canonical tier → 0× decay (immune).
	k.SetActivationRecord(ctx, &types.ActivationRecord{
		SampleID: "tdu-canonical", MemoryTier: int(types.MemoryTierCanonical),
	})
	rate = k.GetDecayModifiedRate(ctx, "tdu-canonical", baseDecay)
	require.True(t, rate.IsZero(), "canonical should have zero decay, got %s", rate)
}

// ─── Test: Co-Activation (Hebbian Learning) ─────────────────────────────────

func TestCoActivation_HebbianLearning(t *testing.T) {
	k, ctx := setupConsolidationTest(t)

	delta := sdkmath.LegacyNewDecWithPrec(1, 1) // +0.1

	// Create activation records for 3 TDUs.
	for _, id := range []string{"tdu-a", "tdu-b", "tdu-c"} {
		k.RecordActivation(ctx, id, 1, delta)
	}

	// Co-activate A and B together 5 times (above Hebbian threshold of 3).
	for i := 0; i < 5; i++ {
		err := k.RecordCoActivation(ctx, []string{"tdu-a", "tdu-b"}, uint64(i+1))
		require.NoError(t, err)
	}

	// A should have B as a Hebbian associate.
	associates := k.GetHebbianAssociates(ctx, "tdu-a")
	require.Len(t, associates, 1)
	require.Equal(t, "tdu-b", associates[0].SampleID)
	require.Equal(t, uint64(5), associates[0].CoActivations)

	// B should have A as a Hebbian associate.
	associates = k.GetHebbianAssociates(ctx, "tdu-b")
	require.Len(t, associates, 1)
	require.Equal(t, "tdu-a", associates[0].SampleID)

	// C should have no associates (never co-activated above threshold).
	associates = k.GetHebbianAssociates(ctx, "tdu-c")
	require.Len(t, associates, 0)
}

// ─── Test: Retrieval Strength Computation ───────────────────────────────────

func TestRetrievalStrength(t *testing.T) {
	// Zero activations → zero strength.
	record := &types.ActivationRecord{}
	require.True(t, record.GetRetrievalStrength().IsZero())

	// 5 well-spaced activations, all positive.
	record = &types.ActivationRecord{
		TotalActivations: 5,
		UniqueCycles:     5,
		PositiveOutcomes: 5,
		NegativeOutcomes: 0,
	}
	strength := record.GetRetrievalStrength()
	// activation = 5/10 = 0.5, spacing = 1.0, perf = 1.0
	// 0.5 * 0.4 + 1.0 * 0.3 + 1.0 * 0.3 = 0.2 + 0.3 + 0.3 = 0.8
	expected := sdkmath.LegacyNewDecWithPrec(8, 1)
	require.True(t, strength.Equal(expected), "want %s, got %s", expected, strength)

	// 10 activations (cap), perfect spacing, 50/50 outcomes.
	record = &types.ActivationRecord{
		TotalActivations: 10,
		UniqueCycles:     10,
		PositiveOutcomes: 5,
		NegativeOutcomes: 5,
	}
	strength = record.GetRetrievalStrength()
	// activation = 10/10 = 1.0, spacing = 1.0, perf = 0.5
	// 1.0 * 0.4 + 1.0 * 0.3 + 0.5 * 0.3 = 0.4 + 0.3 + 0.15 = 0.85
	expected = sdkmath.LegacyNewDecWithPrec(85, 2)
	require.True(t, strength.Equal(expected), "want %s, got %s", expected, strength)
}

// ─── Test: Performance Tracking ─────────────────────────────────────────────

func TestPerformanceTracking(t *testing.T) {
	k, ctx := setupConsolidationTest(t)

	// Mix of positive and negative deltas.
	k.RecordActivation(ctx, "tdu-perf", 1, sdkmath.LegacyNewDecWithPrec(2, 1))  // +0.2
	k.RecordActivation(ctx, "tdu-perf", 2, sdkmath.LegacyNewDecWithPrec(-1, 1)) // -0.1
	k.RecordActivation(ctx, "tdu-perf", 3, sdkmath.LegacyNewDecWithPrec(3, 1))  // +0.3

	record, found := k.GetActivationRecord(ctx, "tdu-perf")
	require.True(t, found)
	require.Equal(t, uint64(2), record.PositiveOutcomes)
	require.Equal(t, uint64(1), record.NegativeOutcomes)
}

// ─── Test: Get TDUs by Tier ─────────────────────────────────────────────────

func TestGetTDUsByTier(t *testing.T) {
	k, ctx := setupConsolidationTest(t)

	k.SetActivationRecord(ctx, &types.ActivationRecord{SampleID: "w1", MemoryTier: int(types.MemoryTierWorking)})
	k.SetActivationRecord(ctx, &types.ActivationRecord{SampleID: "w2", MemoryTier: int(types.MemoryTierWorking)})
	k.SetActivationRecord(ctx, &types.ActivationRecord{SampleID: "a1", MemoryTier: int(types.MemoryTierActive)})
	k.SetActivationRecord(ctx, &types.ActivationRecord{SampleID: "c1", MemoryTier: int(types.MemoryTierConsolidated)})

	working := k.GetTDUsByTier(ctx, types.MemoryTierWorking)
	require.Len(t, working, 2)

	active := k.GetTDUsByTier(ctx, types.MemoryTierActive)
	require.Len(t, active, 1)

	consolidated := k.GetTDUsByTier(ctx, types.MemoryTierConsolidated)
	require.Len(t, consolidated, 1)

	canonical := k.GetTDUsByTier(ctx, types.MemoryTierCanonical)
	require.Len(t, canonical, 0)
}

// ─── Test: Full Memory Lifecycle ────────────────────────────────────────────

func TestFullMemoryLifecycle(t *testing.T) {
	k, ctx := setupConsolidationTest(t)

	sampleID := "tdu-lifecycle"
	delta := sdkmath.LegacyNewDecWithPrec(15, 2) // +0.15

	// Phase 1: Initial activations — Working tier.
	k.RecordActivation(ctx, sampleID, 1, delta)
	r, _ := k.GetActivationRecord(ctx, sampleID)
	require.Equal(t, int(types.MemoryTierWorking), r.MemoryTier)

	// Phase 2: More activations — promote to Active (immediate at 3).
	k.RecordActivation(ctx, sampleID, 2, delta)
	k.RecordActivation(ctx, sampleID, 3, delta)
	r, _ = k.GetActivationRecord(ctx, sampleID)
	require.Equal(t, int(types.MemoryTierActive), r.MemoryTier)

	// Phase 3: Build up to consolidated threshold (8+ activations, well-spaced).
	for cycle := uint64(4); cycle <= 10; cycle++ {
		k.RecordActivation(ctx, sampleID, cycle, delta)
	}

	// Run consolidation → Active → Consolidated.
	promoted, _ := k.RunConsolidation(ctx, 10)
	require.Equal(t, uint64(1), promoted)
	r, _ = k.GetActivationRecord(ctx, sampleID)
	require.Equal(t, int(types.MemoryTierConsolidated), r.MemoryTier)

	// Phase 4: Continue activating toward canonical (need 15+, age 20+, strength 0.6+).
	for cycle := uint64(11); cycle <= 25; cycle++ {
		k.RecordActivation(ctx, sampleID, cycle, delta)
	}

	// Run consolidation at cycle 25 (age = 24 ≥ 20) → Consolidated → Canonical.
	promoted, _ = k.RunConsolidation(ctx, 25)
	require.GreaterOrEqual(t, promoted, uint64(1))
	r, _ = k.GetActivationRecord(ctx, sampleID)
	require.Equal(t, int(types.MemoryTierCanonical), r.MemoryTier)

	// Phase 5: Verify canonical has zero decay.
	baseDecay := sdkmath.LegacyNewDecWithPrec(2, 2) // 0.02
	modifiedDecay := k.GetDecayModifiedRate(ctx, sampleID, baseDecay)
	require.True(t, modifiedDecay.IsZero(), "canonical TDU should have zero decay")
}

// ─── Test: Canonical Pins Fitness Floor ─────────────────────────────────────

func TestCanonicalPinsFitnessFloor(t *testing.T) {
	k, ctx := setupConsolidationTest(t)

	// Create a fitness record with low score.
	record := types.NewTDUFitnessRecord("tdu-pinned", sdkmath.NewInt(1000000), 0)
	record.SetFitnessScore(sdkmath.LegacyNewDecWithPrec(3, 1)) // 0.3 — low
	require.NoError(t, k.SetFitnessRecord(ctx, record))

	// Create activation record at canonical tier.
	k.SetActivationRecord(ctx, &types.ActivationRecord{
		SampleID:         "tdu-pinned",
		MemoryTier:       int(types.MemoryTierConsolidated),
		TotalActivations: 20,
		UniqueCycles:     20,
		FirstActivation:  1,
		PositiveOutcomes: 20,
	})

	// Run consolidation to promote to canonical.
	k.RunConsolidation(ctx, 30)

	// Fitness should be pinned to at least 0.8.
	fitnessRec, found := k.GetFitnessRecord(ctx, "tdu-pinned")
	require.True(t, found)
	score := fitnessRec.GetFitnessScore()
	require.True(t, score.GTE(sdkmath.LegacyNewDecWithPrec(8, 1)),
		"canonical TDU fitness should be at least 0.8, got %s", score)
}
