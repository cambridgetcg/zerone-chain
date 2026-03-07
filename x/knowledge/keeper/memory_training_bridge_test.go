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

func setupBridgeTest(t *testing.T) (keeper.Keeper, context.Context) {
	t.Helper()
	k, ctx := setupKeeper(t)
	setupDefaultDomains(t, k, ctx)
	require.NoError(t, k.SetReconsolidationParams(ctx, types.DefaultReconsolidationParams()))
	require.NoError(t, k.SetConsolidationParams(ctx, types.DefaultConsolidationParams()))
	// Seed fitness records for test TDUs.
	for _, id := range []string{"tdu-a", "tdu-b", "tdu-c", "tdu-d", "tdu-e"} {
		rec := types.NewTDUFitnessRecord(id, sdkmath.NewInt(1000000), 0)
		require.NoError(t, k.SetFitnessRecord(ctx, rec))
	}
	return k, ctx
}

// ─── Test: Process Training Outcome — Positive ──────────────────────────────

func TestProcessTrainingOutcome_Positive(t *testing.T) {
	k, ctx := setupBridgeTest(t)

	outcome := keeper.TrainingOutcome{
		TDUIDs:          []string{"tdu-a", "tdu-b", "tdu-c"},
		OverallDelta:    sdkmath.LegacyNewDecWithPrec(15, 2), // +0.15 (positive)
		CurrentCycle:    5,
		AttestationHash: "hash-positive",
	}

	err := k.ProcessTrainingOutcome(ctx, outcome)
	require.NoError(t, err)

	// All TDUs should have activation records.
	for _, id := range outcome.TDUIDs {
		record, found := k.GetActivationRecord(ctx, id)
		require.True(t, found, "activation record should exist for %s", id)
		require.Equal(t, uint64(1), record.TotalActivations)
		require.Equal(t, uint64(1), record.PositiveOutcomes)
		require.Equal(t, uint64(0), record.NegativeOutcomes)
	}

	// No reconsolidation windows should be open (positive outcome).
	require.Equal(t, uint64(0), k.GetOpenWindowCount(ctx))
}

// ─── Test: Process Training Outcome — Negative Triggers Reconsolidation ─────

func TestProcessTrainingOutcome_NegativeTriggersReconsolidation(t *testing.T) {
	k, ctx := setupBridgeTest(t)

	outcome := keeper.TrainingOutcome{
		TDUIDs:          []string{"tdu-a", "tdu-b"},
		OverallDelta:    sdkmath.LegacyNewDecWithPrec(-2, 1), // -0.2 (negative)
		CurrentCycle:    5,
		AttestationHash: "hash-negative",
	}

	err := k.ProcessTrainingOutcome(ctx, outcome)
	require.NoError(t, err)

	// Negative TDUs should have reconsolidation windows.
	require.Equal(t, uint64(2), k.GetOpenWindowCount(ctx))

	// Check activation records tracked negative outcomes.
	for _, id := range outcome.TDUIDs {
		record, found := k.GetActivationRecord(ctx, id)
		require.True(t, found)
		require.Equal(t, uint64(1), record.NegativeOutcomes)
	}
}

// ─── Test: Process Training Outcome — Co-Activations ────────────────────────

func TestProcessTrainingOutcome_CoActivations(t *testing.T) {
	k, ctx := setupBridgeTest(t)

	// Process same batch 5 times to build co-activation history.
	for cycle := uint64(1); cycle <= 5; cycle++ {
		outcome := keeper.TrainingOutcome{
			TDUIDs:       []string{"tdu-a", "tdu-b"},
			OverallDelta: sdkmath.LegacyNewDecWithPrec(1, 1), // +0.1
			CurrentCycle: cycle,
		}
		require.NoError(t, k.ProcessTrainingOutcome(ctx, outcome))
	}

	// tdu-a and tdu-b should be Hebbian associates.
	associates := k.GetHebbianAssociates(ctx, "tdu-a")
	require.Len(t, associates, 1)
	require.Equal(t, "tdu-b", associates[0].SampleID)
	require.Equal(t, uint64(5), associates[0].CoActivations)
}

// ─── Test: Process Training Outcome — Per-TDU Deltas ────────────────────────

func TestProcessTrainingOutcome_PerTDUDeltas(t *testing.T) {
	k, ctx := setupBridgeTest(t)

	outcome := keeper.TrainingOutcome{
		TDUIDs:       []string{"tdu-a", "tdu-b"},
		OverallDelta: sdkmath.LegacyZeroDec(), // neutral overall
		PerTDUDeltas: map[string]sdkmath.LegacyDec{
			"tdu-a": sdkmath.LegacyNewDecWithPrec(3, 1),  // +0.3 (positive)
			"tdu-b": sdkmath.LegacyNewDecWithPrec(-3, 1), // -0.3 (negative)
		},
		CurrentCycle: 5,
	}

	err := k.ProcessTrainingOutcome(ctx, outcome)
	require.NoError(t, err)

	// tdu-a should have positive outcome.
	recA, _ := k.GetActivationRecord(ctx, "tdu-a")
	require.Equal(t, uint64(1), recA.PositiveOutcomes)

	// tdu-b should have negative outcome AND reconsolidation.
	recB, _ := k.GetActivationRecord(ctx, "tdu-b")
	require.Equal(t, uint64(1), recB.NegativeOutcomes)

	inRecon, _ := k.IsInReconsolidation(ctx, "tdu-b")
	require.True(t, inRecon)

	inReconA, _ := k.IsInReconsolidation(ctx, "tdu-a")
	require.False(t, inReconA) // positive outcome, no reconsolidation
}

// ─── Test: Memory-Aware Decay in EndBlocker ─────────────────────────────────

func TestDecayUnscoredWithMemory(t *testing.T) {
	k, ctx := setupBridgeTest(t)

	// Set up 3 TDUs at different memory tiers.
	// Working tier (full decay).
	k.SetActivationRecord(ctx, &types.ActivationRecord{
		SampleID: "tdu-a", MemoryTier: int(types.MemoryTierWorking),
	})
	// Consolidated tier (0.2× decay).
	k.SetActivationRecord(ctx, &types.ActivationRecord{
		SampleID: "tdu-b", MemoryTier: int(types.MemoryTierConsolidated),
	})
	// Canonical tier (0× decay, immune).
	k.SetActivationRecord(ctx, &types.ActivationRecord{
		SampleID: "tdu-c", MemoryTier: int(types.MemoryTierCanonical),
	})

	// All start at 0.5 fitness, last signal at cycle 0, threshold is 5.
	// Apply decay at cycle 10 (well past threshold).
	k.DecayUnscoredWithMemory(ctx, 10)

	// Working: decays by full 0.02 → 0.48.
	recA, _ := k.GetFitnessRecord(ctx, "tdu-a")
	require.Equal(t, "0.480000000000000000", recA.FitnessScore)

	// Consolidated: decays by 0.02 × 0.2 = 0.004 → 0.496.
	recB, _ := k.GetFitnessRecord(ctx, "tdu-b")
	require.Equal(t, "0.496000000000000000", recB.FitnessScore)

	// Canonical: immune, stays at 0.5.
	recC, _ := k.GetFitnessRecord(ctx, "tdu-c")
	require.Equal(t, "0.500000000000000000", recC.FitnessScore)
}

// ─── Test: Dataset Fingerprint Mapping ──────────────────────────────────────

func TestDatasetFingerprintMapping(t *testing.T) {
	k, ctx := setupBridgeTest(t)

	// Register mapping.
	tduIDs := []string{"tdu-a", "tdu-b", "tdu-c"}
	err := k.RegisterDatasetFingerprint(ctx, "abc123hash", tduIDs)
	require.NoError(t, err)

	// Retrieve via internal method.
	retrieved := k.GetTDUIDsFromDataset(ctx, "abc123hash")
	require.Equal(t, tduIDs, retrieved)

	// Non-existent fingerprint returns nil.
	missing := k.GetTDUIDsFromDataset(ctx, "nonexistent")
	require.Nil(t, missing)
}

// ─── Test: Empty Outcome is No-Op ──────────────────────────────────────────

func TestProcessTrainingOutcome_Empty(t *testing.T) {
	k, ctx := setupBridgeTest(t)

	err := k.ProcessTrainingOutcome(ctx, keeper.TrainingOutcome{})
	require.NoError(t, err)
	require.Equal(t, uint64(0), k.GetOpenWindowCount(ctx))
}
