package keeper_test

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/zerone-chain/zerone/x/knowledge/types"
)

func TestGetVerificationHealth_Throughput(t *testing.T) {
	k, ctx := setupKnowledgeTest(t)

	// Advance to height 1000 so all indexed rounds fall inside the observation window.
	ctx = advanceBlocks(ctx, 900) // 100 + 900 = 1000

	// Index 5 rounds at blocks 100..500
	for i := 0; i < 5; i++ {
		err := k.IndexCompletedRound(ctx, uint64(100+i*100), fmt.Sprintf("r%d", i), &types.CompletedRoundMeta{
			Domain: "physics", HasDissent: false, DurationBlocks: 11,
		})
		require.NoError(t, err)
	}

	// Default params: ObservationWindowBlocks=10,000; roundCycle=200+200+50=450
	// theoreticalMax = 10,000 / 450 = 22
	// throughput = 5 * 1,000,000 / 22 = 227,272
	throughput, disputeRate, avgDuration := k.GetVerificationHealth(ctx)
	require.Greater(t, throughput, uint64(0), "throughput should be > 0 with completed rounds")
	require.Equal(t, uint64(0), disputeRate, "no dissent = 0 dispute rate")
	require.Equal(t, uint64(11), avgDuration)
}

func TestGetVerificationHealth_DisputeRate(t *testing.T) {
	k, ctx := setupKnowledgeTest(t)

	// Advance so all indexed blocks are visible.
	ctx = advanceBlocks(ctx, 900)

	k.IndexCompletedRound(ctx, 100, "r1", &types.CompletedRoundMeta{Domain: "physics", HasDissent: true, DurationBlocks: 10})
	k.IndexCompletedRound(ctx, 200, "r2", &types.CompletedRoundMeta{Domain: "physics", HasDissent: false, DurationBlocks: 10})
	k.IndexCompletedRound(ctx, 300, "r3", &types.CompletedRoundMeta{Domain: "physics", HasDissent: true, DurationBlocks: 10})

	_, disputeRate, _ := k.GetVerificationHealth(ctx)
	// 2/3 disputed = 666,666 BPS
	require.Greater(t, disputeRate, uint64(600_000))
}

func TestGetVerificationHealth_NoRounds(t *testing.T) {
	k, ctx := setupKnowledgeTest(t)

	throughput, disputeRate, avgDuration := k.GetVerificationHealth(ctx)
	require.Equal(t, uint64(0), throughput)
	require.Equal(t, uint64(0), disputeRate)
	require.Equal(t, uint64(0), avgDuration)
}

func TestGetEffectiveMinVerifiers_NilPartnershipKeeper(t *testing.T) {
	k, ctx := setupKnowledgeTest(t)
	// setupKnowledgeTest does NOT set partnershipKeeper, so it remains nil.
	// With nil partnership keeper, GetEffectiveMinVerifiers returns base + 1.
	effective := k.GetEffectiveMinVerifiers(ctx, "physics")
	params, err := k.GetParams(ctx)
	require.NoError(t, err)
	require.Equal(t, uint32(params.MinVerifiers+1), effective,
		"nil partnership keeper = base + 1 verifiers required")
}

func TestGetDomainVerificationActivity_WithRounds(t *testing.T) {
	k, ctx := setupKnowledgeTest(t)

	// Advance to height 1000 so all indexed rounds are in the window.
	ctx = advanceBlocks(ctx, 900)

	// Index 5 rounds for physics
	for i := 0; i < 5; i++ {
		k.IndexCompletedRound(ctx, uint64(100+i*100), fmt.Sprintf("r%d", i),
			&types.CompletedRoundMeta{Domain: "physics", DurationBlocks: 11})
	}

	// activity = 5 * BPS / 10 = 500,000 (50%)
	activity := k.GetDomainVerificationActivity(ctx, "physics")
	require.Greater(t, activity, uint64(0))
	require.Equal(t, uint64(500_000), activity)
}

func TestGetDomainVerificationActivity_NoRounds(t *testing.T) {
	k, ctx := setupKnowledgeTest(t)

	activity := k.GetDomainVerificationActivity(ctx, "physics")
	require.Equal(t, uint64(0), activity)
}
