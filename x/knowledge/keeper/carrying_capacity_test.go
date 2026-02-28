package keeper_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/zerone-chain/zerone/x/knowledge/keeper"
)

func TestDomainStats_SetGet(t *testing.T) {
	k, ctx := setupKnowledgeTest(t)
	stats := keeper.DomainStats{Domain: "physics", ActiveCount: 5, AtRiskCount: 2, TotalEnergy: 500000, LastUpdated: 100}
	k.SetDomainStats(ctx, &stats)
	got, found := k.GetDomainStats(ctx, "physics")
	require.True(t, found)
	require.Equal(t, uint64(5), got.ActiveCount)
	require.Equal(t, uint64(2), got.AtRiskCount)
	require.Equal(t, uint64(500000), got.TotalEnergy)
	require.Equal(t, uint64(100), got.LastUpdated)
}

func TestDomainStats_GetNotFound(t *testing.T) {
	k, ctx := setupKnowledgeTest(t)
	got, found := k.GetDomainStats(ctx, "nonexistent")
	require.False(t, found)
	require.Equal(t, "nonexistent", got.Domain)
	require.Equal(t, uint64(0), got.ActiveCount)
}

func TestDomainStats_IncrementDecrement(t *testing.T) {
	k, ctx := setupKnowledgeTest(t)
	k.IncrementDomainFactCount(ctx, "physics", true, 500000)  // active
	k.IncrementDomainFactCount(ctx, "physics", true, 300000)  // active
	k.IncrementDomainFactCount(ctx, "physics", false, 100000) // at-risk
	got, found := k.GetDomainStats(ctx, "physics")
	require.True(t, found)
	require.Equal(t, uint64(2), got.ActiveCount)
	require.Equal(t, uint64(1), got.AtRiskCount)
	require.Equal(t, uint64(900000), got.TotalEnergy)

	k.DecrementDomainFactCount(ctx, "physics", true, 500000)
	got, _ = k.GetDomainStats(ctx, "physics")
	require.Equal(t, uint64(1), got.ActiveCount)
	require.Equal(t, uint64(400000), got.TotalEnergy)
}

func TestDomainStats_DecrementUnderflow(t *testing.T) {
	k, ctx := setupKnowledgeTest(t)
	// Decrement when already at zero — should not underflow
	k.DecrementDomainFactCount(ctx, "physics", true, 100)
	got, _ := k.GetDomainStats(ctx, "physics")
	require.Equal(t, uint64(0), got.ActiveCount)
	require.Equal(t, uint64(0), got.TotalEnergy)
}

func TestDomainStats_Transition(t *testing.T) {
	k, ctx := setupKnowledgeTest(t)
	k.IncrementDomainFactCount(ctx, "physics", true, 500000)
	k.IncrementDomainFactCount(ctx, "physics", true, 300000)

	// Transition one from active → at-risk
	k.TransitionDomainFactStatus(ctx, "physics", false)
	got, _ := k.GetDomainStats(ctx, "physics")
	require.Equal(t, uint64(1), got.ActiveCount)
	require.Equal(t, uint64(1), got.AtRiskCount)

	// Transition it back: at-risk → active
	k.TransitionDomainFactStatus(ctx, "physics", true)
	got, _ = k.GetDomainStats(ctx, "physics")
	require.Equal(t, uint64(2), got.ActiveCount)
	require.Equal(t, uint64(0), got.AtRiskCount)
}

// ─── Pressure calculation tests ─────────────────────────────────────────────

func TestCarryingCapacity_Base(t *testing.T) {
	k, ctx := setupKnowledgeTest(t)
	cap := k.GetDomainCarryingCapacity(ctx, "physics")
	require.Equal(t, uint64(1000), cap) // DomainBaseCapacity default
}

func TestDomainPressure_Empty(t *testing.T) {
	k, ctx := setupKnowledgeTest(t)
	pressure := k.GetDomainPressure(ctx, "physics")
	require.Equal(t, uint64(0), pressure) // no facts = zero pressure
}

func TestDomainPressure_AtCapacity(t *testing.T) {
	k, ctx := setupKnowledgeTest(t)
	stats := keeper.DomainStats{Domain: "physics", ActiveCount: 1000, AtRiskCount: 0}
	k.SetDomainStats(ctx, &stats)
	pressure := k.GetDomainPressure(ctx, "physics")
	require.Equal(t, uint64(1_000_000), pressure) // exactly at capacity
}

func TestDomainPressure_Overcrowded(t *testing.T) {
	k, ctx := setupKnowledgeTest(t)
	stats := keeper.DomainStats{Domain: "physics", ActiveCount: 2000, AtRiskCount: 0}
	k.SetDomainStats(ctx, &stats)
	pressure := k.GetDomainPressure(ctx, "physics")
	require.Equal(t, uint64(2_000_000), pressure) // 2x capacity
}

func TestDomainPressure_HalfCapacity(t *testing.T) {
	k, ctx := setupKnowledgeTest(t)
	stats := keeper.DomainStats{Domain: "physics", ActiveCount: 500, AtRiskCount: 0}
	k.SetDomainStats(ctx, &stats)
	pressure := k.GetDomainPressure(ctx, "physics")
	require.Equal(t, uint64(500_000), pressure) // 50%
}

func TestPressureCategory(t *testing.T) {
	require.Equal(t, "sparse", keeper.PressureCategory(100_000))
	require.Equal(t, "sparse", keeper.PressureCategory(0))
	require.Equal(t, "normal", keeper.PressureCategory(500_000))
	require.Equal(t, "crowded", keeper.PressureCategory(900_000))
	require.Equal(t, "crowded", keeper.PressureCategory(1_000_000))
	require.Equal(t, "overcrowded", keeper.PressureCategory(1_500_000))
}
