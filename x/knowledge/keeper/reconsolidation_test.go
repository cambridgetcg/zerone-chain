package keeper_test

import (
	"context"
	"fmt"
	"testing"

	sdkmath "cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/stretchr/testify/require"

	"github.com/zerone-chain/zerone/x/knowledge/keeper"
	"github.com/zerone-chain/zerone/x/knowledge/types"
)

// advanceBlockHeight returns a new context with the block height set.
func advanceBlockHeight(ctx context.Context, height int64) context.Context {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	return sdkCtx.WithBlockHeight(height)
}

// ─── Helpers ────────────────────────────────────────────────────────────────

func setupReconsolidationTest(t *testing.T) (keeper.Keeper, context.Context) {
	t.Helper()
	k, ctx := setupKeeper(t)
	setupDefaultDomains(t, k, ctx)
	require.NoError(t, k.SetReconsolidationParams(ctx, types.DefaultReconsolidationParams()))
	require.NoError(t, k.SetConsolidationParams(ctx, types.DefaultConsolidationParams()))
	return k, ctx
}

func seedFitnessRecord(t *testing.T, k keeper.Keeper, ctx context.Context, sampleID string, fitness string) {
	t.Helper()
	score, _ := sdkmath.LegacyNewDecFromStr(fitness)
	rec := types.NewTDUFitnessRecord(sampleID, sdkmath.NewInt(1000000), 0)
	rec.SetFitnessScore(score)
	require.NoError(t, k.SetFitnessRecord(ctx, rec))
}

// ─── Test: Trigger Reconsolidation — Happy Path ─────────────────────────────

func TestTriggerReconsolidation_HappyPath(t *testing.T) {
	k, ctx := setupReconsolidationTest(t)
	seedFitnessRecord(t, k, ctx, "tdu-wrong", "0.6")

	modelDelta := sdkmath.LegacyNewDecWithPrec(-1, 1) // -0.1
	windowID, err := k.TriggerReconsolidation(ctx, "tdu-wrong", modelDelta, 10)
	require.NoError(t, err)
	require.NotEmpty(t, windowID)

	window, found := k.GetWindow(ctx, windowID)
	require.True(t, found)
	require.Equal(t, "tdu-wrong", window.SampleID)
	require.Equal(t, types.ReconsolidationOpen, window.Status)
	require.Equal(t, "0.600000000000000000", window.OriginalFitness)

	// History should be updated.
	history := k.GetReconsolidationHistory(ctx, "tdu-wrong")
	require.Equal(t, uint64(1), history.TotalWindows)
	require.Equal(t, windowID, history.ActiveWindowID)
}

// ─── Test: Trigger — Disabled ───────────────────────────────────────────────

func TestTriggerReconsolidation_Disabled(t *testing.T) {
	k, ctx := setupReconsolidationTest(t)
	params := types.DefaultReconsolidationParams()
	params.Enabled = false
	require.NoError(t, k.SetReconsolidationParams(ctx, params))

	_, err := k.TriggerReconsolidation(ctx, "tdu-x", sdkmath.LegacyNewDecWithPrec(-1, 1), 1)
	require.ErrorIs(t, err, types.ErrReconsolidationDisabled)
}

// ─── Test: Trigger — Already Open ───────────────────────────────────────────

func TestTriggerReconsolidation_AlreadyOpen(t *testing.T) {
	k, ctx := setupReconsolidationTest(t)
	seedFitnessRecord(t, k, ctx, "tdu-dup", "0.5")

	delta := sdkmath.LegacyNewDecWithPrec(-1, 1)
	_, err := k.TriggerReconsolidation(ctx, "tdu-dup", delta, 1)
	require.NoError(t, err)

	// Second trigger should fail.
	_, err = k.TriggerReconsolidation(ctx, "tdu-dup", delta, 2)
	require.ErrorIs(t, err, types.ErrReconsolidationAlreadyOpen)
}

// ─── Test: Window Duration by Tier ──────────────────────────────────────────

func TestWindowDuration_ByTier(t *testing.T) {
	k, ctx := setupReconsolidationTest(t)

	tests := []struct {
		name     string
		tier     types.MemoryTier
		expected int64
	}{
		{"working", types.MemoryTierWorking, 222},
		{"active", types.MemoryTierActive, 111},
		{"consolidated", types.MemoryTierConsolidated, 55},
		{"canonical", types.MemoryTierCanonical, 27},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sampleID := "tdu-" + tt.name
			seedFitnessRecord(t, k, ctx, sampleID, "0.5")

			// Set activation record at this tier.
			k.SetActivationRecord(ctx, &types.ActivationRecord{
				SampleID:         sampleID,
				MemoryTier:       int(tt.tier),
				TotalActivations: 30, // enough for canonical
				NegativeOutcomes: 5,  // enough for canonical gate
			})

			delta := sdkmath.LegacyNewDecWithPrec(-1, 1)
			windowID, err := k.TriggerReconsolidation(ctx, sampleID, delta, 1)
			require.NoError(t, err)

			window, _ := k.GetWindow(ctx, windowID)
			duration := window.ExpiresAt - window.TriggeredAt
			require.Equal(t, tt.expected, duration, "tier %s should have %d block window", tt.name, tt.expected)
		})
	}
}

// ─── Test: Canonical Gate ───────────────────────────────────────────────────

func TestCanonicalGate(t *testing.T) {
	k, ctx := setupReconsolidationTest(t)
	seedFitnessRecord(t, k, ctx, "tdu-canon", "0.9")

	// Set as canonical with only 1 negative outcome.
	k.SetActivationRecord(ctx, &types.ActivationRecord{
		SampleID:         "tdu-canon",
		MemoryTier:       int(types.MemoryTierCanonical),
		TotalActivations: 30,
		NegativeOutcomes: 1, // below threshold of 3
	})

	delta := sdkmath.LegacyNewDecWithPrec(-1, 1)
	_, err := k.TriggerReconsolidation(ctx, "tdu-canon", delta, 1)
	require.ErrorIs(t, err, types.ErrCanonicalNotEnoughNegatives)

	// Now set enough negatives.
	k.SetActivationRecord(ctx, &types.ActivationRecord{
		SampleID:         "tdu-canon",
		MemoryTier:       int(types.MemoryTierCanonical),
		TotalActivations: 30,
		NegativeOutcomes: 3,
	})
	_, err = k.TriggerReconsolidation(ctx, "tdu-canon", delta, 1)
	require.NoError(t, err) // now it works
}

// ─── Test: Submit Correction ────────────────────────────────────────────────

func TestSubmitCorrection(t *testing.T) {
	k, ctx := setupReconsolidationTest(t)
	seedFitnessRecord(t, k, ctx, "tdu-stale", "0.6")

	delta := sdkmath.LegacyNewDecWithPrec(-15, 2) // -0.15
	windowID, _ := k.TriggerReconsolidation(ctx, "tdu-stale", delta, 1)

	// Submit correction during window.
	err := k.SubmitCorrection(ctx, windowID, "tdu-corrected", testAddr)
	require.NoError(t, err)

	window, _ := k.GetWindow(ctx, windowID)
	require.Len(t, window.CorrectionIDs, 1)
	require.Equal(t, "tdu-corrected", window.CorrectionIDs[0])
}

// ─── Test: Submit Correction — Window Closed ────────────────────────────────

func TestSubmitCorrection_WindowClosed(t *testing.T) {
	k, ctx := setupReconsolidationTest(t)
	seedFitnessRecord(t, k, ctx, "tdu-late", "0.6")

	delta := sdkmath.LegacyNewDecWithPrec(-1, 1)
	windowID, _ := k.TriggerReconsolidation(ctx, "tdu-late", delta, 1)

	// Resolve the window first.
	_ = k.ResolveWindow(ctx, windowID, "tdu-fix")

	// Try to submit after resolution.
	err := k.SubmitCorrection(ctx, windowID, "tdu-too-late", testAddr)
	require.ErrorIs(t, err, types.ErrReconsolidationWindowClosed)
}

// ─── Test: Resolve Window — Full Flow ───────────────────────────────────────

func TestResolveWindow_FullFlow(t *testing.T) {
	k, ctx := setupReconsolidationTest(t)
	seedFitnessRecord(t, k, ctx, "tdu-resolve", "0.7")

	// Create activation record for inheritance.
	k.SetActivationRecord(ctx, &types.ActivationRecord{
		SampleID:         "tdu-resolve",
		TotalActivations: 10,
		UniqueCycles:     8,
		PositiveOutcomes: 7,
		NegativeOutcomes: 3,
		FirstActivation:  1,
		LastActivation:   10,
		MemoryTier:       int(types.MemoryTierActive),
	})

	// Trigger reconsolidation.
	delta := sdkmath.LegacyNewDecWithPrec(-2, 1) // -0.2
	windowID, _ := k.TriggerReconsolidation(ctx, "tdu-resolve", delta, 10)

	// Submit and resolve with correction.
	k.SubmitCorrection(ctx, windowID, "tdu-fix-resolve", testAddr)
	err := k.ResolveWindow(ctx, windowID, "tdu-fix-resolve")
	require.NoError(t, err)

	// Check window status.
	window, _ := k.GetWindow(ctx, windowID)
	require.Equal(t, types.ReconsolidationResolved, window.Status)

	// Original fitness should have dropped (0.7 - 0.2 = 0.5).
	fitnessRec, found := k.GetFitnessRecord(ctx, "tdu-resolve")
	require.True(t, found)
	require.Equal(t, "0.500000000000000000", fitnessRec.FitnessScore)

	// Correction should have inherited 50% of activation history.
	corrActivation, found := k.GetActivationRecord(ctx, "tdu-fix-resolve")
	require.True(t, found)
	require.Equal(t, uint64(5), corrActivation.TotalActivations)  // 10 * 0.5
	require.Equal(t, uint64(4), corrActivation.UniqueCycles)      // 8 * 0.5
	require.Equal(t, uint64(3), corrActivation.PositiveOutcomes)  // 7 * 0.5 truncated
	require.Equal(t, int(types.MemoryTierActive), corrActivation.MemoryTier)

	// History should record correction.
	history := k.GetReconsolidationHistory(ctx, "tdu-resolve")
	require.Equal(t, uint64(1), history.CorrectedCount)
	require.Equal(t, "", history.ActiveWindowID) // cleared
	require.Contains(t, history.CorrectionChain, "tdu-fix-resolve")
}

// ─── Test: Expire Windows ───────────────────────────────────────────────────

func TestExpireWindows(t *testing.T) {
	k, ctx := setupReconsolidationTest(t)
	seedFitnessRecord(t, k, ctx, "tdu-expire", "0.6")

	// Set as working tier (222 block window).
	delta := sdkmath.LegacyNewDecWithPrec(-1, 1)
	windowID, _ := k.TriggerReconsolidation(ctx, "tdu-expire", delta, 1)

	window, _ := k.GetWindow(ctx, windowID)

	// Advance past expiration.
	ctx = advanceBlockHeight(ctx, window.ExpiresAt+1)

	expired, err := k.ExpireWindows(ctx)
	require.NoError(t, err)
	require.Equal(t, uint64(1), expired)

	// Check fitness penalty applied (0.6 - 0.05 = 0.55).
	fitnessRec, _ := k.GetFitnessRecord(ctx, "tdu-expire")
	require.Equal(t, "0.550000000000000000", fitnessRec.FitnessScore)

	// History should record uncorrected.
	history := k.GetReconsolidationHistory(ctx, "tdu-expire")
	require.Equal(t, uint64(1), history.UncorrectedCount)
	require.Equal(t, "", history.ActiveWindowID)
}

// ─── Test: Reconsolidation Penalty Compounds ────────────────────────────────

func TestReconsolidationPenaltyCompounds(t *testing.T) {
	history := &types.ReconsolidationHistory{
		UncorrectedCount: 0,
	}
	// 0 uncorrected → penalty 1.0 (no extra decay).
	require.True(t, history.GetReconsolidationPenalty().Equal(sdkmath.LegacyOneDec()))

	// 3 uncorrected → penalty 1.3.
	history.UncorrectedCount = 3
	expected := sdkmath.LegacyNewDecWithPrec(13, 1) // 1.3
	require.True(t, history.GetReconsolidationPenalty().Equal(expected),
		"want %s, got %s", expected, history.GetReconsolidationPenalty())

	// 5 uncorrected → penalty 1.5.
	history.UncorrectedCount = 5
	expected = sdkmath.LegacyNewDecWithPrec(15, 1) // 1.5
	require.True(t, history.GetReconsolidationPenalty().Equal(expected))
}

// ─── Test: Effective Decay Rate — Full Stack ────────────────────────────────

func TestEffectiveDecayRate_FullStack(t *testing.T) {
	k, ctx := setupReconsolidationTest(t)

	baseDecay := sdkmath.LegacyNewDecWithPrec(2, 2) // 0.02

	// Active tier (0.7×) + 2 uncorrected reconsolidations (1.2×).
	k.SetActivationRecord(ctx, &types.ActivationRecord{
		SampleID: "tdu-stack", MemoryTier: int(types.MemoryTierActive),
	})
	k.SetReconsolidationHistory(ctx, &types.ReconsolidationHistory{
		SampleID: "tdu-stack", UncorrectedCount: 2,
	})

	effective := k.GetEffectiveDecayRate(ctx, "tdu-stack", baseDecay)
	// 0.02 × 0.7 × 1.2 = 0.0168
	expected := sdkmath.LegacyNewDecWithPrec(2, 2).
		Mul(sdkmath.LegacyNewDecWithPrec(7, 1)).
		Mul(sdkmath.LegacyNewDecWithPrec(12, 1))
	require.True(t, effective.Equal(expected),
		"effective decay: want %s, got %s", expected, effective)
}

// ─── Test: IsInReconsolidation ──────────────────────────────────────────────

func TestIsInReconsolidation(t *testing.T) {
	k, ctx := setupReconsolidationTest(t)
	seedFitnessRecord(t, k, ctx, "tdu-check", "0.5")

	// Not in reconsolidation initially.
	inRecon, window := k.IsInReconsolidation(ctx, "tdu-check")
	require.False(t, inRecon)
	require.Nil(t, window)

	// Trigger reconsolidation.
	delta := sdkmath.LegacyNewDecWithPrec(-1, 1)
	k.TriggerReconsolidation(ctx, "tdu-check", delta, 1)

	inRecon, window = k.IsInReconsolidation(ctx, "tdu-check")
	require.True(t, inRecon)
	require.NotNil(t, window)
	require.Equal(t, "tdu-check", window.SampleID)
}

// ─── Test: Open Window Count ────────────────────────────────────────────────

func TestOpenWindowCount(t *testing.T) {
	k, ctx := setupReconsolidationTest(t)

	for i := 0; i < 5; i++ {
		sampleID := fmt.Sprintf("tdu-count-%d", i)
		seedFitnessRecord(t, k, ctx, sampleID, "0.5")
		k.TriggerReconsolidation(ctx, sampleID, sdkmath.LegacyNewDecWithPrec(-1, 1), uint64(i))
	}

	count := k.GetOpenWindowCount(ctx)
	require.Equal(t, uint64(5), count)
}

// ─── Test: Full Reconsolidation Lifecycle ───────────────────────────────────

func TestFullReconsolidationLifecycle(t *testing.T) {
	k, ctx := setupReconsolidationTest(t)
	seedFitnessRecord(t, k, ctx, "tdu-lifecycle", "0.7")

	k.SetActivationRecord(ctx, &types.ActivationRecord{
		SampleID:         "tdu-lifecycle",
		TotalActivations: 8,
		UniqueCycles:     8,
		PositiveOutcomes: 5,
		NegativeOutcomes: 3,
		FirstActivation:  1,
		MemoryTier:       int(types.MemoryTierActive),
	})

	// 1. Negative training outcome triggers reconsolidation.
	delta := sdkmath.LegacyNewDecWithPrec(-15, 2) // -0.15
	windowID, err := k.TriggerReconsolidation(ctx, "tdu-lifecycle", delta, 10)
	require.NoError(t, err)

	// 2. Verify TDU is in reconsolidation.
	inRecon, _ := k.IsInReconsolidation(ctx, "tdu-lifecycle")
	require.True(t, inRecon)

	// 3. Agent submits correction during window.
	err = k.SubmitCorrection(ctx, windowID, "tdu-better-data", testAddr)
	require.NoError(t, err)

	// 4. Correction passes quality review → resolve window.
	err = k.ResolveWindow(ctx, windowID, "tdu-better-data")
	require.NoError(t, err)

	// 5. Verify original fitness dropped (0.7 - 0.15 = 0.55).
	fitnessRec, _ := k.GetFitnessRecord(ctx, "tdu-lifecycle")
	require.Equal(t, "0.550000000000000000", fitnessRec.FitnessScore)

	// 6. Verify correction inherited activation history.
	corrActivation, found := k.GetActivationRecord(ctx, "tdu-better-data")
	require.True(t, found)
	require.Equal(t, uint64(4), corrActivation.TotalActivations) // 8 * 0.5
	require.Equal(t, int(types.MemoryTierActive), corrActivation.MemoryTier)

	// 7. Verify TDU is no longer in reconsolidation.
	inRecon, _ = k.IsInReconsolidation(ctx, "tdu-lifecycle")
	require.False(t, inRecon)

	// 8. Verify correction chain.
	history := k.GetReconsolidationHistory(ctx, "tdu-lifecycle")
	require.Equal(t, uint64(1), history.CorrectedCount)
	require.Equal(t, uint64(0), history.UncorrectedCount)
	require.Contains(t, history.CorrectionChain, "tdu-better-data")
}
