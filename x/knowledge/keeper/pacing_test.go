package keeper_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

// ─── Mock PacingKeeper ──────────────────────────────────────────────────────

type mockPacingKeeper struct {
	creationBps uint64
	analysisBps uint64
}

func (m *mockPacingKeeper) GetGlobalPacingMultiplier(_ context.Context) (creationBps, analysisBps uint64) {
	return m.creationBps, m.analysisBps
}

// ─── GetEffectiveCooldown tests ─────────────────────────────────────────────

func TestPacing_EffectiveCooldown_NoPacingKeeper(t *testing.T) {
	k, ctx := setupKnowledgeTest(t)

	// Without pacing keeper, cooldown should equal the base param (50).
	cooldown := k.GetEffectiveCooldown(ctx, "physics")
	require.Equal(t, uint64(50), cooldown, "no pacing keeper should return base cooldown")
}

func TestPacing_EffectiveCooldown_HealthyNetwork(t *testing.T) {
	k, ctx := setupKnowledgeTest(t)

	pk := &mockPacingKeeper{creationBps: 1_000_000, analysisBps: 1_000_000}
	k.SetPacingKeeper(pk)

	cooldown := k.GetEffectiveCooldown(ctx, "physics")
	require.Equal(t, uint64(50), cooldown, "healthy pacing should return base cooldown")
}

func TestPacing_EffectiveCooldown_DegradedNetwork(t *testing.T) {
	k, ctx := setupKnowledgeTest(t)

	// 75% pacing → cooldown = 50 * 1_000_000 / 750_000 = 66
	pk := &mockPacingKeeper{creationBps: 750_000, analysisBps: 750_000}
	k.SetPacingKeeper(pk)

	cooldown := k.GetEffectiveCooldown(ctx, "physics")
	require.Equal(t, uint64(66), cooldown, "degraded pacing (75%%) should increase cooldown to 66")
}

func TestPacing_EffectiveCooldown_CriticalNetwork(t *testing.T) {
	k, ctx := setupKnowledgeTest(t)

	// 50% pacing → cooldown = 50 * 1_000_000 / 500_000 = 100
	pk := &mockPacingKeeper{creationBps: 500_000, analysisBps: 500_000}
	k.SetPacingKeeper(pk)

	cooldown := k.GetEffectiveCooldown(ctx, "physics")
	require.Equal(t, uint64(100), cooldown, "critical pacing (50%%) should double cooldown to 100")
}

func TestPacing_EffectiveCooldown_DomainPressure(t *testing.T) {
	k, ctx := setupKnowledgeTest(t)

	pk := &mockPacingKeeper{creationBps: 1_000_000, analysisBps: 1_000_000}
	k.SetPacingKeeper(pk)

	// Overcrowd the physics domain: add more facts than base capacity (1000).
	for i := 0; i < 1500; i++ {
		k.IncrementDomainFactCount(ctx, "physics", true, 500_000)
	}

	cooldown := k.GetEffectiveCooldown(ctx, "physics")
	require.Greater(t, cooldown, uint64(50),
		"overcrowded domain should increase cooldown above base")
}

// ─── GetEffectiveMinReviewFee tests ─────────────────────────────────────────

func TestPacing_EffectiveMinReviewFee_NoPacingKeeper(t *testing.T) {
	k, ctx := setupKnowledgeTest(t)

	fee := k.GetEffectiveMinReviewFee(ctx)
	require.Equal(t, "100000", fee, "no pacing keeper should return base fee")
}

func TestPacing_EffectiveMinReviewFee_HealthyNetwork(t *testing.T) {
	k, ctx := setupKnowledgeTest(t)

	pk := &mockPacingKeeper{creationBps: 1_000_000, analysisBps: 1_000_000}
	k.SetPacingKeeper(pk)

	fee := k.GetEffectiveMinReviewFee(ctx)
	require.Equal(t, "100000", fee, "healthy pacing should return base fee")
}

func TestPacing_EffectiveMinReviewFee_DegradedNetwork(t *testing.T) {
	k, ctx := setupKnowledgeTest(t)

	// 75% pacing → fee = 100000 * 1_000_000 / 750_000 = 133333
	pk := &mockPacingKeeper{creationBps: 750_000, analysisBps: 750_000}
	k.SetPacingKeeper(pk)

	fee := k.GetEffectiveMinReviewFee(ctx)
	require.Equal(t, "133333", fee, "degraded pacing (75%%) should increase fee to 133333")
}

func TestPacing_EffectiveMinReviewFee_CriticalNetwork(t *testing.T) {
	k, ctx := setupKnowledgeTest(t)

	// 50% pacing → fee = 100000 * 1_000_000 / 500_000 = 200000
	pk := &mockPacingKeeper{creationBps: 500_000, analysisBps: 500_000}
	k.SetPacingKeeper(pk)

	fee := k.GetEffectiveMinReviewFee(ctx)
	require.Equal(t, "200000", fee, "critical pacing (50%%) should double fee to 200000")
}

// ─── LastClaimHeight tests ──────────────────────────────────────────────────

func TestPacing_LastClaimHeight_RoundTrip(t *testing.T) {
	k, ctx := setupKnowledgeTest(t)

	submitter := "zrn1submitter_pacing"

	// Initially zero
	height := k.GetLastClaimHeight(ctx, submitter)
	require.Equal(t, uint64(0), height, "should return 0 for unknown submitter")

	// Set and retrieve
	k.SetLastClaimHeight(ctx, submitter, 42)
	height = k.GetLastClaimHeight(ctx, submitter)
	require.Equal(t, uint64(42), height, "should return 42 after set")

	// Overwrite
	k.SetLastClaimHeight(ctx, submitter, 999)
	height = k.GetLastClaimHeight(ctx, submitter)
	require.Equal(t, uint64(999), height, "should return 999 after overwrite")
}

func TestPacing_LastClaimHeight_IsolatedPerSubmitter(t *testing.T) {
	k, ctx := setupKnowledgeTest(t)

	k.SetLastClaimHeight(ctx, "alice", 100)
	k.SetLastClaimHeight(ctx, "bob", 200)

	require.Equal(t, uint64(100), k.GetLastClaimHeight(ctx, "alice"))
	require.Equal(t, uint64(200), k.GetLastClaimHeight(ctx, "bob"))
	require.Equal(t, uint64(0), k.GetLastClaimHeight(ctx, "carol"), "unknown submitter should be 0")
}
