package keeper_test

import (
	"testing"

	cmtproto "github.com/cometbft/cometbft/proto/tendermint/types"

	"github.com/stretchr/testify/require"

	"github.com/zerone-chain/zerone/x/knowledge/keeper"
	"github.com/zerone-chain/zerone/x/knowledge/types"
)

// ─── Fitness Score Tests ─────────────────────────────────────────────────────

func TestCalculateFitness_HighQuery(t *testing.T) {
	k, ctx := setupKnowledgeTest(t)

	fact := &types.Fact{
		Id:              "fact-hq",
		Content:         "Heavily queried fact",
		Domain:          "physics",
		Status:          types.FactStatus_FACT_STATUS_VERIFIED,
		QueryCountEpoch: 1000, // Max queries
		EpochBorn:       0,
	}
	require.NoError(t, k.SetFact(ctx, fact))

	score := k.CalculateFitness(ctx, fact, 1) // epoch 1, within grace period

	// With 1000 queries, query component = 1,000,000 * 300,000/1,000,000 = 300,000
	// Plus uniqueness component (1,000,000 * 100,000/1,000,000 = 100,000)
	// Score should be substantial
	require.Greater(t, score, uint64(300_000), "heavily queried fact should score high")
}

func TestCalculateFitness_HighCitation(t *testing.T) {
	k, ctx := setupKnowledgeTest(t)

	fact := &types.Fact{
		Id:                   "fact-hc",
		Content:              "Well-cited foundational fact",
		Domain:               "mathematics",
		Status:               types.FactStatus_FACT_STATUS_VERIFIED,
		IncomingCitationCount: 10, // Max citation score
		EpochBorn:            0,
	}
	require.NoError(t, k.SetFact(ctx, fact))

	score := k.CalculateFitness(ctx, fact, 1)

	// Citation component: min(10*100,000, 1,000,000) = 1,000,000
	// Weighted: 1,000,000 * 250,000/1,000,000 = 250,000
	require.Greater(t, score, uint64(250_000), "well-cited fact should score high")
}

func TestCalculateFitness_ZeroUsage(t *testing.T) {
	k, ctx := setupKnowledgeTest(t)

	fact := &types.Fact{
		Id:        "fact-zu",
		Content:   "Unused fact",
		Domain:    "general",
		Status:    types.FactStatus_FACT_STATUS_VERIFIED,
		EpochBorn: 0,
	}
	require.NoError(t, k.SetFact(ctx, fact))

	// Epoch 30 — well past grace period (10 epochs), 20 penalty epochs
	score := k.CalculateFitness(ctx, fact, 30)

	// Age penalty: (30-10)*50,000 = 1,000,000 (max)
	// Weighted: 1,000,000 * 100,000/1,000,000 = 100,000
	// Only uniqueness remains: 100,000
	// Fitness = uniqueness(100,000) - age_penalty(100,000) = 0
	require.Less(t, score, uint64(150_000), "unused old fact should decay")
}

func TestCalculateFitness_AgeResistance(t *testing.T) {
	k, ctx := setupKnowledgeTest(t)

	fact := &types.Fact{
		Id:                   "fact-ar",
		Content:              "2+2=4",
		Domain:               "mathematics",
		Status:               types.FactStatus_FACT_STATUS_VERIFIED,
		IncomingCitationCount: 10, // Well-cited
		EpochBorn:            0,
	}
	require.NoError(t, k.SetFact(ctx, fact))

	// Old fact (epoch 30), but well-cited
	score := k.CalculateFitness(ctx, fact, 30)

	// Age penalty: (30-10)*50,000 = 1,000,000
	// Citation resistance: min(10*100,000, 1,000,000) = 1,000,000
	// Net age penalty = 0
	// Citation still contributes positively
	require.Greater(t, score, uint64(200_000), "cited old fact should resist aging")
}

func TestCalculateFitness_BridgeBonus(t *testing.T) {
	k, ctx := setupKnowledgeTest(t)

	fact := &types.Fact{
		Id:          "fact-bb",
		Content:     "Cross-domain bridging fact",
		Domain:      "physics",
		Status:      types.FactStatus_FACT_STATUS_VERIFIED,
		BridgeScore: 800_000, // High bridge score
		EpochBorn:   0,
	}
	require.NoError(t, k.SetFact(ctx, fact))

	score := k.CalculateFitness(ctx, fact, 1) // Within grace period

	// Bridge: 800,000 * 100,000/1,000,000 = 80,000
	// Plus uniqueness: 100,000
	require.Greater(t, score, uint64(150_000), "cross-domain fact should get bridge bonus")
}

func TestCalculateFitness_PatronageKeepsAlive(t *testing.T) {
	k, ctx := setupKnowledgeTest(t)

	// Set block height to something reasonable
	ctx = ctx.WithBlockHeader(cmtproto.Header{Height: 500})

	fact := &types.Fact{
		Id:                  "fact-pk",
		Content:             "Patronized fact",
		Domain:              "general",
		Status:              types.FactStatus_FACT_STATUS_VERIFIED,
		PatronageAmount:     "1000000", // 1 ZRN
		PatronageExpiryBlock: 10000,    // Far in the future
		EpochBorn:           0,
	}
	require.NoError(t, k.SetFact(ctx, fact))

	// Old epoch but with active patronage
	score := k.CalculateFitness(ctx, fact, 30)

	// Patronage: 1,000,000 * 50,000/1,000,000 = 50,000
	// Even with age penalty, patronage contributes
	require.Greater(t, score, uint64(0), "patronized fact should not die")
}

func TestCalculateFitness_GracePeriod(t *testing.T) {
	k, ctx := setupKnowledgeTest(t)

	fact := &types.Fact{
		Id:        "fact-gp",
		Content:   "Brand new fact",
		Domain:    "physics",
		Status:    types.FactStatus_FACT_STATUS_VERIFIED,
		EpochBorn: 5,
	}
	require.NoError(t, k.SetFact(ctx, fact))

	// Epoch 10 — only 5 epochs old, within 10-epoch grace period
	scoreInGrace := k.CalculateFitness(ctx, fact, 10)

	// Epoch 20 — 15 epochs old, 5 past grace
	scorePostGrace := k.CalculateFitness(ctx, fact, 20)

	// Within grace period, no age penalty
	// Post grace, age penalty kicks in
	require.GreaterOrEqual(t, scoreInGrace, scorePostGrace,
		"fact within grace period should score >= post-grace")
}

func TestUpdateAllFitness_EpochBoundary(t *testing.T) {
	k, ctx := setupKnowledgeTest(t)

	// Set block height to a fitness epoch boundary
	params := types.DefaultParams()
	ctx = ctx.WithBlockHeader(cmtproto.Header{Height: int64(params.FitnessEpochBlocks)})

	// Create a verified fact with some query activity
	fact := &types.Fact{
		Id:              "fact-ub",
		Content:         "Epoch boundary test fact",
		Domain:          "physics",
		Status:          types.FactStatus_FACT_STATUS_VERIFIED,
		QueryCountEpoch: 500,
		EpochBorn:       0,
		FitnessScore:    500_000,
	}
	require.NoError(t, k.SetFact(ctx, fact))

	// Run epoch update
	err := k.UpdateAllFitnessScores(ctx)
	require.NoError(t, err)

	// Fetch updated fact
	updated, found := k.GetFact(ctx, "fact-ub")
	require.True(t, found)

	// Fitness should be recalculated
	require.Equal(t, uint64(params.FitnessEpochBlocks), updated.FitnessUpdatedBlock)
	// Epoch query counter should be reset
	require.Equal(t, uint64(0), updated.QueryCountEpoch)
}

func TestQueryByFitness_Sorted(t *testing.T) {
	k, ctx := setupKnowledgeTest(t)

	// Create facts with different fitness scores
	facts := []*types.Fact{
		{Id: "low", Content: "Low fitness", Domain: "physics", Status: types.FactStatus_FACT_STATUS_VERIFIED, FitnessScore: 100_000},
		{Id: "high", Content: "High fitness", Domain: "physics", Status: types.FactStatus_FACT_STATUS_VERIFIED, FitnessScore: 800_000},
		{Id: "mid", Content: "Mid fitness", Domain: "physics", Status: types.FactStatus_FACT_STATUS_VERIFIED, FitnessScore: 500_000},
	}
	for _, f := range facts {
		require.NoError(t, k.SetFact(ctx, f))
	}

	// Query sorted descending
	result := k.GetFactsByFitness(ctx, "", 0, 50, false)
	require.Len(t, result, 3)
	require.Equal(t, "high", result[0].Id)
	require.Equal(t, "mid", result[1].Id)
	require.Equal(t, "low", result[2].Id)

	// Query sorted ascending
	resultAsc := k.GetFactsByFitness(ctx, "", 0, 50, true)
	require.Len(t, resultAsc, 3)
	require.Equal(t, "low", resultAsc[0].Id)

	// Query with min_fitness filter
	resultFiltered := k.GetFactsByFitness(ctx, "", 400_000, 50, false)
	require.Len(t, resultFiltered, 2)
	require.Equal(t, "high", resultFiltered[0].Id)
	require.Equal(t, "mid", resultFiltered[1].Id)

	// Query with domain filter
	resultDomain := k.GetFactsByFitness(ctx, "physics", 0, 50, false)
	require.Len(t, resultDomain, 3)

	resultEmpty := k.GetFactsByFitness(ctx, "mathematics", 0, 50, false)
	require.Len(t, resultEmpty, 0)
}

func TestFitnessLabel(t *testing.T) {
	tests := []struct {
		score uint64
		label string
	}{
		{0, "critical"},
		{50_000, "critical"},
		{100_000, "low"},
		{200_000, "low"},
		{300_000, "healthy"},
		{500_000, "healthy"},
		{600_000, "thriving"},
		{750_000, "thriving"},
		{800_000, "keystone"},
		{1_000_000, "keystone"},
	}

	for _, tt := range tests {
		t.Run(tt.label, func(t *testing.T) {
			require.Equal(t, tt.label, keeper.FitnessLabel(tt.score))
		})
	}
}
