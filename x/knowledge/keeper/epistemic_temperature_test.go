package keeper_test

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

func TestEpistemicParams_Defaults(t *testing.T) {
	params := types.DefaultParams()
	require.Equal(t, uint64(995_000), params.EpistemicTemperatureDecayBps)
	require.Equal(t, uint64(50_000), params.EpistemicConformityCoolingBps)
	require.Equal(t, uint64(100_000), params.EpistemicVindicationHeatingBps)
	require.Equal(t, uint64(600_000), params.EpistemicColdConfidenceCapBps)
	require.Equal(t, uint64(1_500_000), params.EpistemicHotConfidenceGrowthBps)
	require.Equal(t, uint64(10_000), params.EpistemicTemperatureWindowBlocks)
}

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

func TestEpistemicState_GetOrInit(t *testing.T) {
	k, ctx := setupKnowledgeTest(t)

	// No existing state — should return neutral
	state, err := k.GetOrInitDomainEpistemicState(ctx, "new_domain")
	require.NoError(t, err)
	require.Equal(t, "new_domain", state.Domain)
	require.Equal(t, uint64(500_000), state.Temperature)

	// Set a state, then GetOrInit should return it
	require.NoError(t, k.SetDomainEpistemicState(ctx, &types.DomainEpistemicState{
		Domain:      "existing",
		Temperature: 800_000,
	}))
	state, err = k.GetOrInitDomainEpistemicState(ctx, "existing")
	require.NoError(t, err)
	require.Equal(t, uint64(800_000), state.Temperature)
}

func TestCountVindicationsInWindow(t *testing.T) {
	k, ctx := setupKnowledgeTest(t)

	// Create facts in different domains using the existing makeTestFact helper
	makeTestFact(t, k, ctx, "f1", "fact one", "physics", "general", "zrn1submitter1", 700_000)
	makeTestFact(t, k, ctx, "f2", "fact two", "physics", "general", "zrn1submitter1", 700_000)
	makeTestFact(t, k, ctx, "f3", "fact three", "mathematics", "general", "zrn1submitter1", 700_000)

	// Add vindication records for physics domain facts
	require.NoError(t, k.SetVindicationRecord(ctx, "f1", types.VindicationRecord{
		Verifier: "v1", FactId: "f1", VindicatedAt: 5000,
	}))
	require.NoError(t, k.SetVindicationRecord(ctx, "f1", types.VindicationRecord{
		Verifier: "v2", FactId: "f1", VindicatedAt: 5000,
	}))
	require.NoError(t, k.SetVindicationRecord(ctx, "f2", types.VindicationRecord{
		Verifier: "v3", FactId: "f2", VindicatedAt: 9000,
	}))

	// Record for mathematics domain (should not count for physics)
	require.NoError(t, k.SetVindicationRecord(ctx, "f3", types.VindicationRecord{
		Verifier: "v4", FactId: "f3", VindicatedAt: 8000,
	}))

	// Count vindications for physics within window [0, 10000]
	count := k.CountVindicationsInWindow(ctx, "physics", 10000, 10000)
	require.Equal(t, uint64(2), count) // f1 and f2 are two distinct facts (events)

	// Narrower window that excludes f1's vindication
	count = k.CountVindicationsInWindow(ctx, "physics", 10000, 2000)
	require.Equal(t, uint64(1), count) // Only f2 at height 9000 within [8000, 10000]

	// Empty domain
	count = k.CountVindicationsInWindow(ctx, "logic", 10000, 10000)
	require.Equal(t, uint64(0), count)
}
