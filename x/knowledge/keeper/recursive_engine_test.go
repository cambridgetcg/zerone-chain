package keeper_test

import (
	"context"
	"fmt"
	"testing"

	sdkmath "cosmossdk.io/math"
	"github.com/stretchr/testify/require"

	"github.com/zerone-chain/zerone/x/knowledge/keeper"
	"github.com/zerone-chain/zerone/x/knowledge/types"
)

// ─── Helpers ────────────────────────────────────────────────────────────────

func setupRecursiveTest(t *testing.T) (keeper.Keeper, context.Context) {
	t.Helper()
	k, ctx := setupKeeper(t)
	setupDefaultDomains(t, k, ctx)
	require.NoError(t, k.SetRecursiveEngineParams(ctx, types.DefaultRecursiveEngineParams()))
	require.NoError(t, k.SetConsolidationParams(ctx, types.DefaultConsolidationParams()))
	require.NoError(t, k.SetReconsolidationParams(ctx, types.DefaultReconsolidationParams()))
	return k, ctx
}

// seedModelAgent creates a model and promotes it to an agent for testing.
func seedModelAgent(t *testing.T, k keeper.Keeper, ctx context.Context, agentID, modelID string, gen uint64) {
	t.Helper()
	agent := &types.AgentIdentity{
		AgentID:       agentID,
		ModelID:       modelID,
		Generation:    gen,
		Status:        types.AgentStatusActive,
		Address:       testAddr,
	}
	require.NoError(t, k.SetAgentIdentity(ctx, agent))
}

// ─── Test: Params ───────────────────────────────────────────────────────────

func TestRecursiveEngineParams(t *testing.T) {
	k, ctx := setupRecursiveTest(t)

	params := k.GetRecursiveEngineParams(ctx)
	require.True(t, params.CaptureEnabled)
	require.Equal(t, uint64(21), params.MaxSlotsPerDomain)
	require.NoError(t, params.Validate())

	// Invalid params should fail.
	bad := params
	bad.MaxSlotsPerDomain = 0
	require.Error(t, bad.Validate())
}

// ─── Test: Verification Capture — Aligned ───────────────────────────────────

func TestCaptureVerificationWork_Aligned(t *testing.T) {
	k, ctx := setupRecursiveTest(t)
	seedModelAgent(t, k, ctx, "agent-alpha", "model-v1", 1)

	err := k.CaptureVerificationWork(
		ctx,
		"round-1",
		"agent-alpha",
		`{"overall_quality": 80, "reasoning_depth": 7}`,
		true, // aligned with consensus
		sdkmath.LegacyNewDecWithPrec(9, 1), // 0.9 consensus score
		"sub-1",
		int32(types.SampleType_SAMPLE_TYPE_EXPLANATION),
		"science",
	)
	require.NoError(t, err)

	// Check capture was stored.
	capture, found := k.GetVerificationCapture(ctx, "vc-1")
	require.True(t, found)
	require.Equal(t, "round-1", capture.RoundID)
	require.Equal(t, "agent-alpha", capture.VerifierID)
	require.Equal(t, "model-v1", capture.ModelID)
	require.True(t, capture.Aligned)
	require.Equal(t, uint64(1), capture.Generation)

	// Fitness should be high (aligned).
	rec, found := k.GetFitnessRecord(ctx, "vc-1")
	require.True(t, found)
	score := rec.GetFitnessScore()
	require.True(t, score.GTE(sdkmath.LegacyNewDecWithPrec(7, 1)),
		"aligned capture should have fitness >= 0.7, got %s", score)

	// Memory class should be procedural (verification is skill).
	memClass := k.GetMemoryClass(ctx, "vc-1")
	require.Equal(t, types.MemoryClassProcedural, memClass)
}

// ─── Test: Verification Capture — Misaligned ────────────────────────────────

func TestCaptureVerificationWork_Misaligned(t *testing.T) {
	k, ctx := setupRecursiveTest(t)
	seedModelAgent(t, k, ctx, "agent-beta", "model-v1", 1)

	err := k.CaptureVerificationWork(
		ctx, "round-2", "agent-beta",
		`{"overall_quality": 30}`,
		false, // misaligned — model was wrong
		sdkmath.LegacyNewDecWithPrec(3, 1),
		"sub-2", int32(types.SampleType_SAMPLE_TYPE_DISCUSSION), "science",
	)
	require.NoError(t, err)

	capture, found := k.GetVerificationCapture(ctx, "vc-1")
	require.True(t, found)
	require.False(t, capture.Aligned)

	// Fitness should be low (misaligned — but still captured as training data!).
	rec, found := k.GetFitnessRecord(ctx, "vc-1")
	require.True(t, found)
	score := rec.GetFitnessScore()
	require.True(t, score.LTE(sdkmath.LegacyNewDecWithPrec(3, 1)),
		"misaligned capture should have fitness <= 0.3, got %s", score)
}

// ─── Test: Capture Disabled ─────────────────────────────────────────────────

func TestCaptureVerificationWork_Disabled(t *testing.T) {
	k, ctx := setupRecursiveTest(t)

	params := k.GetRecursiveEngineParams(ctx)
	params.CaptureEnabled = false
	require.NoError(t, k.SetRecursiveEngineParams(ctx, params))

	seedModelAgent(t, k, ctx, "agent-gamma", "model-v1", 1)

	err := k.CaptureVerificationWork(
		ctx, "round-3", "agent-gamma", `{}`, true,
		sdkmath.LegacyOneDec(), "sub-3", 0, "science",
	)
	require.NoError(t, err)

	// No capture should exist.
	_, found := k.GetVerificationCapture(ctx, "vc-1")
	require.False(t, found)
}

// ─── Test: Non-Model Verifier Skipped ───────────────────────────────────────

func TestCaptureVerificationWork_HumanVerifier(t *testing.T) {
	k, ctx := setupRecursiveTest(t)

	// No agent identity → human verifier, should be silently skipped.
	err := k.CaptureVerificationWork(
		ctx, "round-4", "human-verifier", `{}`, true,
		sdkmath.LegacyOneDec(), "sub-4", 0, "science",
	)
	require.NoError(t, err)

	_, found := k.GetVerificationCapture(ctx, "vc-1")
	require.False(t, found)
}

// ─── Test: Join Consensus Pool ──────────────────────────────────────────────

func TestJoinConsensusPool(t *testing.T) {
	k, ctx := setupRecursiveTest(t)
	seedModelAgent(t, k, ctx, "agent-1", "model-1", 1)

	err := k.JoinConsensusPool(ctx, "agent-1", "science", sdkmath.NewInt(10000000))
	require.NoError(t, err)

	slot, found := k.GetConsensusSlot(ctx, "science", "agent-1")
	require.True(t, found)
	require.True(t, slot.Active)
	require.Equal(t, "model-1", slot.ModelID)
	require.Equal(t, uint64(1), slot.Generation)
	require.Equal(t, uint64(0), slot.TotalVotes)

	// Active slot count should be 1.
	require.Equal(t, uint64(1), k.CountActiveSlots(ctx, "science"))

	// Generation should have 1 active model.
	gen := k.GetModelGeneration(ctx, 1)
	require.Equal(t, uint64(1), gen.ActiveModels)
}

// ─── Test: Join Pool — Duplicate Rejected ───────────────────────────────────

func TestJoinConsensusPool_Duplicate(t *testing.T) {
	k, ctx := setupRecursiveTest(t)
	seedModelAgent(t, k, ctx, "agent-1", "model-1", 1)

	require.NoError(t, k.JoinConsensusPool(ctx, "agent-1", "science", sdkmath.NewInt(10000000)))
	err := k.JoinConsensusPool(ctx, "agent-1", "science", sdkmath.NewInt(10000000))
	require.Error(t, err)
	require.ErrorIs(t, err, types.ErrConsensusAlreadyJoined)
}

// ─── Test: Join Pool — Insufficient Stake ───────────────────────────────────

func TestJoinConsensusPool_InsufficientStake(t *testing.T) {
	k, ctx := setupRecursiveTest(t)
	seedModelAgent(t, k, ctx, "agent-1", "model-1", 1)

	err := k.JoinConsensusPool(ctx, "agent-1", "science", sdkmath.NewInt(100)) // way too low
	require.Error(t, err)
}

// ─── Test: Leave Consensus Pool ─────────────────────────────────────────────

func TestLeaveConsensusPool(t *testing.T) {
	k, ctx := setupRecursiveTest(t)
	seedModelAgent(t, k, ctx, "agent-1", "model-1", 1)

	require.NoError(t, k.JoinConsensusPool(ctx, "agent-1", "science", sdkmath.NewInt(10000000)))
	require.NoError(t, k.LeaveConsensusPool(ctx, "agent-1", "science"))

	slot, found := k.GetConsensusSlot(ctx, "science", "agent-1")
	require.True(t, found) // slot exists but...
	require.False(t, slot.Active) // ...inactive

	require.Equal(t, uint64(0), k.CountActiveSlots(ctx, "science"))
}

// ─── Test: Consensus Slot Stats Update ──────────────────────────────────────

func TestConsensusSlotStats(t *testing.T) {
	k, ctx := setupRecursiveTest(t)
	seedModelAgent(t, k, ctx, "agent-1", "model-1", 1)
	require.NoError(t, k.JoinConsensusPool(ctx, "agent-1", "science", sdkmath.NewInt(10000000)))

	// Simulate 10 votes: 8 aligned, 2 misaligned.
	for i := 0; i < 10; i++ {
		aligned := i < 8
		err := k.CaptureVerificationWork(
			ctx,
			fmt.Sprintf("round-%d", i),
			"agent-1",
			`{"quality": 80}`,
			aligned,
			sdkmath.LegacyNewDecWithPrec(8, 1),
			fmt.Sprintf("sub-%d", i),
			int32(types.SampleType_SAMPLE_TYPE_EXPLANATION),
			"science",
		)
		require.NoError(t, err)
	}

	slot, _ := k.GetConsensusSlot(ctx, "science", "agent-1")
	require.Equal(t, uint64(10), slot.TotalVotes)
	require.Equal(t, uint64(8), slot.AlignedVotes)
	require.Equal(t, uint64(10), slot.CapturesCreated)

	// Alignment rate should be 0.8.
	rate := slot.AlignmentRate()
	require.Equal(t, "0.800000000000000000", rate.String())
}

// ─── Test: Generational Challenge — Succession ──────────────────────────────

func TestGenerationalChallenge_Succession(t *testing.T) {
	k, ctx := setupRecursiveTest(t)

	// Set up defender (Gen 1) with mediocre alignment.
	seedModelAgent(t, k, ctx, "defender", "model-v1", 1)
	require.NoError(t, k.JoinConsensusPool(ctx, "defender", "science", sdkmath.NewInt(10000000)))

	// Set up challenger (Gen 2).
	seedModelAgent(t, k, ctx, "challenger", "model-v2", 2)
	require.NoError(t, k.JoinConsensusPool(ctx, "challenger", "science", sdkmath.NewInt(10000000)))

	// Simulate 15 votes: defender 60% aligned, challenger 90% aligned.
	for i := 0; i < 15; i++ {
		// Defender: aligned for first 9 (60%).
		_ = k.CaptureVerificationWork(ctx, fmt.Sprintf("d-round-%d", i), "defender",
			`{}`, i < 9, sdkmath.LegacyOneDec(), fmt.Sprintf("sub-%d", i), 0, "science")
		// Challenger: aligned for first 13 (87%).
		_ = k.CaptureVerificationWork(ctx, fmt.Sprintf("c-round-%d", i), "challenger",
			`{}`, i < 13, sdkmath.LegacyOneDec(), fmt.Sprintf("sub-%d", i), 0, "science")
	}

	// Initiate challenge.
	chalID, err := k.InitiateChallenge(ctx, "challenger", "defender", "science")
	require.NoError(t, err)

	// Resolve — challenger should win (87% vs 60% > 5% threshold).
	challenge, found := k.GetChallenge(ctx, chalID)
	require.True(t, found)
	require.False(t, challenge.Resolved)

	err = k.ResolveChallenge(ctx, chalID)
	require.NoError(t, err)

	// Check outcome.
	challenge, _ = k.GetChallenge(ctx, chalID)
	require.True(t, challenge.Resolved)
	require.Equal(t, "challenger", challenge.Winner)

	// Defender should be retired.
	defSlot, _ := k.GetConsensusSlot(ctx, "science", "defender")
	require.False(t, defSlot.Active)
}

// ─── Test: Generational Challenge — Defender Holds ──────────────────────────

func TestGenerationalChallenge_DefenderHolds(t *testing.T) {
	k, ctx := setupRecursiveTest(t)

	seedModelAgent(t, k, ctx, "defender", "model-v1", 1)
	require.NoError(t, k.JoinConsensusPool(ctx, "defender", "science", sdkmath.NewInt(10000000)))

	seedModelAgent(t, k, ctx, "challenger", "model-v2", 2)
	require.NoError(t, k.JoinConsensusPool(ctx, "challenger", "science", sdkmath.NewInt(10000000)))

	// Both have similar alignment (80% vs 80%) — no improvement.
	for i := 0; i < 15; i++ {
		aligned := i < 12
		_ = k.CaptureVerificationWork(ctx, fmt.Sprintf("d-%d", i), "defender",
			`{}`, aligned, sdkmath.LegacyOneDec(), fmt.Sprintf("s-%d", i), 0, "science")
		_ = k.CaptureVerificationWork(ctx, fmt.Sprintf("c-%d", i), "challenger",
			`{}`, aligned, sdkmath.LegacyOneDec(), fmt.Sprintf("s-%d", i), 0, "science")
	}

	chalID, err := k.InitiateChallenge(ctx, "challenger", "defender", "science")
	require.NoError(t, err)
	require.NoError(t, k.ResolveChallenge(ctx, chalID))

	challenge, _ := k.GetChallenge(ctx, chalID)
	require.True(t, challenge.Resolved)
	require.Equal(t, "defender", challenge.Winner) // defender holds

	// Defender should still be active.
	defSlot, _ := k.GetConsensusSlot(ctx, "science", "defender")
	require.True(t, defSlot.Active)
}

// ─── Test: Challenge Requires Newer Generation ──────────────────────────────

func TestChallenge_SameGenRejected(t *testing.T) {
	k, ctx := setupRecursiveTest(t)

	seedModelAgent(t, k, ctx, "agent-a", "model-a", 1)
	seedModelAgent(t, k, ctx, "agent-b", "model-b", 1) // same generation

	_, err := k.InitiateChallenge(ctx, "agent-a", "agent-b", "science")
	require.ErrorIs(t, err, types.ErrChallengeSameGen)
}

// ─── Test: Generation Tracking ──────────────────────────────────────────────

func TestGenerationTracking(t *testing.T) {
	k, ctx := setupRecursiveTest(t)

	// Initial generation should be 0.
	require.Equal(t, uint64(0), k.GetCurrentGeneration(ctx))

	// Set and read.
	k.SetCurrentGeneration(ctx, 3)
	require.Equal(t, uint64(3), k.GetCurrentGeneration(ctx))

	// Generation stats should track captures.
	seedModelAgent(t, k, ctx, "agent-1", "model-1", 1)
	require.NoError(t, k.JoinConsensusPool(ctx, "agent-1", "science", sdkmath.NewInt(10000000)))

	for i := 0; i < 5; i++ {
		_ = k.CaptureVerificationWork(ctx, fmt.Sprintf("r-%d", i), "agent-1",
			`{}`, true, sdkmath.LegacyOneDec(), fmt.Sprintf("s-%d", i), 0, "science")
	}

	gen := k.GetModelGeneration(ctx, 1)
	require.Equal(t, uint64(5), gen.TotalCaptures)
	require.Equal(t, uint64(1), gen.ActiveModels)
}

// ─── Test: The Full Recursive Loop ──────────────────────────────────────────

func TestRecursiveLoop_EndToEnd(t *testing.T) {
	k, ctx := setupRecursiveTest(t)

	// === GENERATION 0: Seed models bootstrap the loop ===
	seedModelAgent(t, k, ctx, "gen0-agent", "seed-model", 0)
	require.NoError(t, k.JoinConsensusPool(ctx, "gen0-agent", "science", sdkmath.NewInt(10000000)))

	// Gen 0 does verification work — captures are generated.
	for i := 0; i < 20; i++ {
		aligned := i < 14 // 70% alignment (mediocre seed model)
		_ = k.CaptureVerificationWork(ctx, fmt.Sprintf("gen0-r-%d", i), "gen0-agent",
			`{"quality": 60}`, aligned, sdkmath.LegacyOneDec(),
			fmt.Sprintf("sub-%d", i), int32(types.SampleType_SAMPLE_TYPE_TUTORIAL), "science")
	}

	gen0 := k.GetModelGeneration(ctx, 0)
	require.Equal(t, uint64(20), gen0.TotalCaptures) // 20 training samples generated

	// === GENERATION 1: Trained on Gen 0's verification data ===
	// (Training happens off-chain in TEE. Chain just tracks lineage.)
	seedModelAgent(t, k, ctx, "gen1-agent", "improved-model", 1)
	require.NoError(t, k.JoinConsensusPool(ctx, "gen1-agent", "science", sdkmath.NewInt(10000000)))

	// Gen 1 is better — 90% alignment.
	for i := 0; i < 20; i++ {
		aligned := i < 18 // 90% alignment (improved!)
		_ = k.CaptureVerificationWork(ctx, fmt.Sprintf("gen1-r-%d", i), "gen1-agent",
			`{"quality": 85}`, aligned, sdkmath.LegacyOneDec(),
			fmt.Sprintf("sub-%d", i+20), int32(types.SampleType_SAMPLE_TYPE_TUTORIAL), "science")
	}

	gen1 := k.GetModelGeneration(ctx, 1)
	require.Equal(t, uint64(20), gen1.TotalCaptures) // 20 more training samples

	// === SUCCESSION: Gen 1 challenges Gen 0 ===
	chalID, err := k.InitiateChallenge(ctx, "gen1-agent", "gen0-agent", "science")
	require.NoError(t, err)
	require.NoError(t, k.ResolveChallenge(ctx, chalID))

	challenge, _ := k.GetChallenge(ctx, chalID)
	require.True(t, challenge.Resolved)
	require.Equal(t, "gen1-agent", challenge.Winner) // Gen 1 proves superiority

	// Gen 0 is retired.
	gen0Slot, _ := k.GetConsensusSlot(ctx, "science", "gen0-agent")
	require.False(t, gen0Slot.Active)

	// Gen 1 continues — its captures will train Gen 2.
	gen1Slot, _ := k.GetConsensusSlot(ctx, "science", "gen1-agent")
	require.True(t, gen1Slot.Active)

	// The loop continues: Gen 1 captures → Train Gen 2 → Challenge → Succession → ...
	// Each generation produces better verification data for the next.
	// The chain improves itself.

	// Total captures across all generations.
	totalCaptures := gen0.TotalCaptures + gen1.TotalCaptures
	require.Equal(t, uint64(40), totalCaptures) // 40 training samples from consensus work
}
