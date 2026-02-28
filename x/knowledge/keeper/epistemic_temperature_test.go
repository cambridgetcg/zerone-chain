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
