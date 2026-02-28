package keeper_test

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/zerone-chain/zerone/x/knowledge/keeper"
	"github.com/zerone-chain/zerone/x/knowledge/types"
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

// ─── Birth pressure tests ───────────────────────────────────────────────────

func TestBirthPressure_SparseBonus(t *testing.T) {
	k, ctx := setupKnowledgeTest(t)
	// Empty domain = sparse = energy bonus
	params, _ := k.GetParams(ctx)
	energy := k.ApplyBirthPressure(ctx, "physics", params.MetabolismInitialEnergy)
	require.Greater(t, energy, params.MetabolismInitialEnergy)
}

func TestBirthPressure_OvercrowdedNoBonus(t *testing.T) {
	k, ctx := setupKnowledgeTest(t)
	stats := keeper.DomainStats{Domain: "physics", ActiveCount: 2000}
	k.SetDomainStats(ctx, &stats)
	params, _ := k.GetParams(ctx)
	energy := k.ApplyBirthPressure(ctx, "physics", params.MetabolismInitialEnergy)
	require.Equal(t, params.MetabolismInitialEnergy, energy) // no bonus when overcrowded
}

func TestBirthPressure_AtCapacityNoBonus(t *testing.T) {
	k, ctx := setupKnowledgeTest(t)
	stats := keeper.DomainStats{Domain: "physics", ActiveCount: 1000}
	k.SetDomainStats(ctx, &stats)
	params, _ := k.GetParams(ctx)
	energy := k.ApplyBirthPressure(ctx, "physics", params.MetabolismInitialEnergy)
	require.Equal(t, params.MetabolismInitialEnergy, energy) // no bonus at exact capacity
}

func TestBirthPressure_HalfCapacity(t *testing.T) {
	k, ctx := setupKnowledgeTest(t)
	stats := keeper.DomainStats{Domain: "physics", ActiveCount: 500}
	k.SetDomainStats(ctx, &stats)
	params, _ := k.GetParams(ctx)
	energy := k.ApplyBirthPressure(ctx, "physics", params.MetabolismInitialEnergy)
	// 50% sparseness, 20% bonus cap → ~10% bonus
	require.Greater(t, energy, params.MetabolismInitialEnergy)
	// But less than full sparse bonus
	fullSparseEnergy := k.ApplyBirthPressure(ctx, "theology", params.MetabolismInitialEnergy) // empty domain
	require.Less(t, energy, fullSparseEnergy)
}

// ─── Death pressure tests ───────────────────────────────────────────────────

func TestDeathPressure_Overcrowded(t *testing.T) {
	k, ctx := setupKnowledgeTest(t)
	stats := keeper.DomainStats{Domain: "physics", ActiveCount: 2000}
	k.SetDomainStats(ctx, &stats)
	multiplier := k.GetDeathPressureMultiplier(ctx, "physics")
	require.Greater(t, multiplier, uint64(keeper.BPSCapacity)) // > 100% = faster decay
}

func TestDeathPressure_Sparse(t *testing.T) {
	k, ctx := setupKnowledgeTest(t)
	stats := keeper.DomainStats{Domain: "physics", ActiveCount: 100}
	k.SetDomainStats(ctx, &stats)
	multiplier := k.GetDeathPressureMultiplier(ctx, "physics")
	require.Less(t, multiplier, uint64(keeper.BPSCapacity)) // < 100% = slower decay
}

func TestDeathPressure_Normal(t *testing.T) {
	k, ctx := setupKnowledgeTest(t)
	stats := keeper.DomainStats{Domain: "physics", ActiveCount: 600}
	k.SetDomainStats(ctx, &stats)
	multiplier := k.GetDeathPressureMultiplier(ctx, "physics")
	require.Equal(t, uint64(keeper.BPSCapacity), multiplier) // 100% = normal
}

func TestDeathPressure_Empty(t *testing.T) {
	k, ctx := setupKnowledgeTest(t)
	// Empty domain = 0 pressure < 50% threshold → 75% decay
	multiplier := k.GetDeathPressureMultiplier(ctx, "physics")
	require.Equal(t, uint64(750_000), multiplier) // 75% = slower decay
}

// ─── Integration test ───────────────────────────────────────────────────────

func TestCarryingCapacity_Integration(t *testing.T) {
	k, ctx := setupKnowledgeTest(t)
	params, _ := k.GetParams(ctx)

	// 1. Create 1200 facts in "physics" (over capacity of 1000)
	for i := 0; i < 1200; i++ {
		factID := fmt.Sprintf("fact-%04d", i)
		fact := &types.Fact{
			Id:                factID,
			Content:           fmt.Sprintf("Physics fact number %d with enough content", i),
			Domain:            "physics",
			Category:          "observation",
			Confidence:        500_000,
			Submitter:         "zrn1test",
			SubmittedAtBlock:  100,
			VerifiedAtBlock:   100,
			LastVerifiedBlock: 100,
			Status:            types.FactStatus_FACT_STATUS_ACTIVE,
			Energy:            params.MetabolismInitialEnergy,
			EnergyCap:         params.MetabolismEnergyCap,
		}
		require.NoError(t, k.SetFact(ctx, fact))
		k.IncrementDomainFactCount(ctx, fact.Domain, true, fact.Energy)
	}

	// 2. Verify pressure > 1M BPS (overcrowded)
	pressure := k.GetDomainPressure(ctx, "physics")
	require.Greater(t, pressure, uint64(keeper.BPSCapacity))
	require.Equal(t, "overcrowded", keeper.PressureCategory(pressure))

	// 3. Verify death pressure multiplier > 1M (faster decay)
	multiplier := k.GetDeathPressureMultiplier(ctx, "physics")
	require.Greater(t, multiplier, uint64(keeper.BPSCapacity))

	// 4. Verify birth pressure gives no bonus
	energy := k.ApplyBirthPressure(ctx, "physics", params.MetabolismInitialEnergy)
	require.Equal(t, params.MetabolismInitialEnergy, energy)

	// 5. Create fact in sparse domain — verify bonus energy
	sparseEnergy := k.ApplyBirthPressure(ctx, "theology", params.MetabolismInitialEnergy)
	require.Greater(t, sparseEnergy, params.MetabolismInitialEnergy)

	// 6. Verify stats are correct
	stats, found := k.GetDomainStats(ctx, "physics")
	require.True(t, found)
	require.Equal(t, uint64(1200), stats.ActiveCount)
	require.Equal(t, uint64(0), stats.AtRiskCount)

	// 7. Verify capacity
	cap := k.GetDomainCarryingCapacity(ctx, "physics")
	require.Equal(t, uint64(1000), cap)

	// 8. Verify sparse domain
	sparseStats, found := k.GetDomainStats(ctx, "theology")
	require.False(t, found) // not yet populated
	require.Equal(t, uint64(0), sparseStats.ActiveCount)
	sparsePressure := k.GetDomainPressure(ctx, "theology")
	require.Equal(t, uint64(0), sparsePressure)
	require.Equal(t, "sparse", keeper.PressureCategory(sparsePressure))
}
