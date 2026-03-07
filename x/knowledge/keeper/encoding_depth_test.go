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

func setupEncodingTest(t *testing.T) (keeper.Keeper, context.Context) {
	t.Helper()
	k, ctx := setupKeeper(t)
	setupDefaultDomains(t, k, ctx)
	require.NoError(t, k.SetConsolidationParams(ctx, types.DefaultConsolidationParams()))
	require.NoError(t, k.SetReconsolidationParams(ctx, types.DefaultReconsolidationParams()))
	return k, ctx
}

// ─── Test: Encoding Depth — Deep vs Shallow ─────────────────────────────────

func TestEncodingDepth_DeepVsShallow(t *testing.T) {
	// Deep encoding: unanimous, expert reviewers, high stake, procedural.
	deep := types.EncodingDepth(
		sdkmath.LegacyOneDec(),                        // consensus: 1.0 (unanimous)
		sdkmath.LegacyNewDecWithPrec(9, 1),            // reviewer rep: 0.9 (expert)
		sdkmath.LegacyNewDec(5),                       // stake ratio: 5× (max difficulty)
		types.MemoryClassProcedural,                    // skills = deepest processing
	)

	// Shallow encoding: bare majority, unknown reviewers, min stake, episodic.
	shallow := types.EncodingDepth(
		sdkmath.LegacyNewDecWithPrec(6, 1),            // consensus: 0.6 (bare majority)
		sdkmath.LegacyNewDecWithPrec(2, 1),            // reviewer rep: 0.2 (newbie)
		sdkmath.LegacyOneDec(),                        // stake ratio: 1× (minimum)
		types.MemoryClassEpisodic,                      // events = shallowest
	)

	require.True(t, deep.GT(shallow),
		"deep encoding (%s) should be higher than shallow (%s)", deep, shallow)

	// Deep should be near 0.8 (max).
	require.True(t, deep.GTE(sdkmath.LegacyNewDecWithPrec(7, 1)),
		"deep encoding should be >= 0.7, got %s", deep)

	// Shallow should be near 0.3 (min).
	require.True(t, shallow.LTE(sdkmath.LegacyNewDecWithPrec(5, 1)),
		"shallow encoding should be <= 0.5, got %s", shallow)
}

// ─── Test: Encoding Depth — Clamped Range ───────────────────────────────────

func TestEncodingDepth_ClampedRange(t *testing.T) {
	min := sdkmath.LegacyNewDecWithPrec(3, 1)  // 0.3
	max := sdkmath.LegacyNewDecWithPrec(8, 1)  // 0.8

	// All zeros → minimum.
	lowest := types.EncodingDepth(
		sdkmath.LegacyZeroDec(),
		sdkmath.LegacyZeroDec(),
		sdkmath.LegacyOneDec(),
		types.MemoryClassEpisodic,
	)
	require.True(t, lowest.GTE(min), "should be >= 0.3, got %s", lowest)

	// All maxed → maximum.
	highest := types.EncodingDepth(
		sdkmath.LegacyOneDec(),
		sdkmath.LegacyOneDec(),
		sdkmath.LegacyNewDec(5),
		types.MemoryClassProcedural,
	)
	require.True(t, highest.LTE(max), "should be <= 0.8, got %s", highest)
}

// ─── Test: Memory Class Classification ──────────────────────────────────────

func TestClassifyFromSampleType(t *testing.T) {
	tests := []struct {
		sampleType types.SampleType
		expected   types.MemoryClass
	}{
		// Semantic.
		{types.SampleType_SAMPLE_TYPE_EXPLANATION, types.MemoryClassSemantic},
		{types.SampleType_SAMPLE_TYPE_Q_AND_A, types.MemoryClassSemantic},
		{types.SampleType_SAMPLE_TYPE_ANNOTATION, types.MemoryClassSemantic},

		// Episodic.
		{types.SampleType_SAMPLE_TYPE_DISCUSSION, types.MemoryClassEpisodic},
		{types.SampleType_SAMPLE_TYPE_DEBATE, types.MemoryClassEpisodic},
		{types.SampleType_SAMPLE_TYPE_NARRATIVE, types.MemoryClassEpisodic},
		{types.SampleType_SAMPLE_TYPE_OPINION, types.MemoryClassEpisodic},

		// Procedural.
		{types.SampleType_SAMPLE_TYPE_TUTORIAL, types.MemoryClassProcedural},
		{types.SampleType_SAMPLE_TYPE_TROUBLESHOOT, types.MemoryClassProcedural},
		{types.SampleType_SAMPLE_TYPE_CREATIVE, types.MemoryClassProcedural},
		{types.SampleType_SAMPLE_TYPE_CORRECTION, types.MemoryClassProcedural},
	}

	for _, tt := range tests {
		result := types.ClassifyFromSampleType(tt.sampleType)
		require.Equal(t, tt.expected, result,
			"SampleType %v should classify as %s", tt.sampleType, tt.expected)
	}
}

// ─── Test: Type-Specific Decay Modifiers ────────────────────────────────────

func TestDecayModifiers(t *testing.T) {
	semantic := types.MemoryClassSemantic.DecayModifier()
	episodic := types.MemoryClassEpisodic.DecayModifier()
	procedural := types.MemoryClassProcedural.DecayModifier()

	// Procedural < Semantic < Episodic (slower decay → more durable).
	require.True(t, procedural.LT(semantic), "procedural (%s) should decay slower than semantic (%s)", procedural, semantic)
	require.True(t, semantic.LT(episodic), "semantic (%s) should decay slower than episodic (%s)", semantic, episodic)

	// Exact values.
	require.Equal(t, "0.600000000000000000", procedural.String())
	require.Equal(t, "0.800000000000000000", semantic.String())
	require.Equal(t, "1.200000000000000000", episodic.String())
}

// ─── Test: Initialize Fitness with Depth ────────────────────────────────────

func TestInitializeFitnessWithDepth(t *testing.T) {
	k, ctx := setupEncodingTest(t)

	err := k.InitializeFitnessWithDepth(
		ctx,
		"tdu-deep",
		sdkmath.NewInt(5000000), // 5 ZRN stake
		1,
		sdkmath.LegacyOneDec(),                     // unanimous
		sdkmath.LegacyNewDecWithPrec(8, 1),         // 0.8 reputation
		sdkmath.LegacyNewDec(3),                    // 3× stake ratio
		types.SampleType_SAMPLE_TYPE_TUTORIAL,       // procedural
	)
	require.NoError(t, err)

	// Check fitness record exists with depth-based score (not flat 0.5).
	rec, found := k.GetFitnessRecord(ctx, "tdu-deep")
	require.True(t, found)
	score := rec.GetFitnessScore()
	require.True(t, score.GT(sdkmath.LegacyNewDecWithPrec(5, 1)),
		"deeply encoded TDU should have fitness > 0.5, got %s", score)

	// Check memory class was stored.
	memClass := k.GetMemoryClass(ctx, "tdu-deep")
	require.Equal(t, types.MemoryClassProcedural, memClass)
}

// ─── Test: Full Effective Decay — Complete Stack ────────────────────────────

func TestFullEffectiveDecayRate(t *testing.T) {
	k, ctx := setupEncodingTest(t)
	baseDecay := sdkmath.LegacyNewDecWithPrec(2, 2) // 0.02

	// Consolidated procedural TDU, no reconsolidation issues.
	k.SetActivationRecord(ctx, &types.ActivationRecord{
		SampleID: "tdu-skill", MemoryTier: int(types.MemoryTierConsolidated),
	})
	k.SetMemoryClass(ctx, "tdu-skill", types.MemoryClassProcedural)

	rate := k.GetFullEffectiveDecayRate(ctx, "tdu-skill", baseDecay)
	// 0.02 × 0.2 (consolidated) × 1.0 (no reconsolidation) × 0.6 (procedural) = 0.0024
	expected := sdkmath.LegacyNewDecWithPrec(2, 2).
		Mul(sdkmath.LegacyNewDecWithPrec(2, 1)).
		Mul(sdkmath.LegacyNewDecWithPrec(6, 1))
	require.True(t, rate.Equal(expected),
		"consolidated procedural: want %s, got %s", expected, rate)

	// Working episodic TDU with 2 uncorrected reconsolidation events.
	k.SetActivationRecord(ctx, &types.ActivationRecord{
		SampleID: "tdu-chat", MemoryTier: int(types.MemoryTierWorking),
	})
	k.SetReconsolidationHistory(ctx, &types.ReconsolidationHistory{
		SampleID: "tdu-chat", UncorrectedCount: 2,
	})
	k.SetMemoryClass(ctx, "tdu-chat", types.MemoryClassEpisodic)

	rate = k.GetFullEffectiveDecayRate(ctx, "tdu-chat", baseDecay)
	// 0.02 × 1.0 (working) × 1.2 (2 uncorrected) × 1.2 (episodic) = 0.0288
	expected = sdkmath.LegacyNewDecWithPrec(2, 2).
		Mul(sdkmath.LegacyOneDec()).
		Mul(sdkmath.LegacyNewDecWithPrec(12, 1)).
		Mul(sdkmath.LegacyNewDecWithPrec(12, 1))
	require.True(t, rate.Equal(expected),
		"working episodic w/ recon: want %s, got %s", expected, rate)
}

// ─── Test: Type-Specific Decay in Practice ──────────────────────────────────

func TestTypeSpecificDecay_InPractice(t *testing.T) {
	k, ctx := setupEncodingTest(t)

	// Create 3 TDUs at same tier, different memory classes.
	for _, tt := range []struct {
		id    string
		class types.MemoryClass
	}{
		{"tdu-fact", types.MemoryClassSemantic},
		{"tdu-event", types.MemoryClassEpisodic},
		{"tdu-skill", types.MemoryClassProcedural},
	} {
		rec := types.NewTDUFitnessRecord(tt.id, sdkmath.NewInt(1000000), 0)
		require.NoError(t, k.SetFitnessRecord(ctx, rec))
		k.SetActivationRecord(ctx, &types.ActivationRecord{
			SampleID: tt.id, MemoryTier: int(types.MemoryTierActive),
		})
		k.SetMemoryClass(ctx, tt.id, tt.class)
	}

	// Apply memory-aware decay at cycle 10 (past threshold).
	k.DecayUnscoredWithMemory(ctx, 10)

	factRec, _ := k.GetFitnessRecord(ctx, "tdu-fact")
	eventRec, _ := k.GetFitnessRecord(ctx, "tdu-event")
	skillRec, _ := k.GetFitnessRecord(ctx, "tdu-skill")

	factScore := factRec.GetFitnessScore()
	eventScore := eventRec.GetFitnessScore()
	skillScore := skillRec.GetFitnessScore()

	// Skills should have highest fitness (least decay).
	require.True(t, skillScore.GT(factScore),
		"procedural (%s) should decay less than semantic (%s)", skillScore, factScore)

	// Facts should have higher fitness than events.
	require.True(t, factScore.GT(eventScore),
		"semantic (%s) should decay less than episodic (%s)", factScore, eventScore)

	// Verify order: skill > fact > event.
	t.Logf("Fitness after decay — skill: %s, fact: %s, event: %s", skillScore, factScore, eventScore)
}

// ─── Test: Memory Class Persistence ─────────────────────────────────────────

func TestMemoryClassPersistence(t *testing.T) {
	k, ctx := setupEncodingTest(t)

	// Default is semantic.
	require.Equal(t, types.MemoryClassSemantic, k.GetMemoryClass(ctx, "unknown-tdu"))

	// Set and retrieve each class.
	for _, class := range []types.MemoryClass{
		types.MemoryClassSemantic,
		types.MemoryClassEpisodic,
		types.MemoryClassProcedural,
	} {
		id := "tdu-" + class.String()
		require.NoError(t, k.SetMemoryClass(ctx, id, class))
		require.Equal(t, class, k.GetMemoryClass(ctx, id))
	}
}
