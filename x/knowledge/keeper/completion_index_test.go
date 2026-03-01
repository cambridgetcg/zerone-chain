package keeper_test

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/zerone-chain/zerone/x/knowledge/types"
)

func TestCompletionIndex_CountCompletedRoundsInWindow(t *testing.T) {
	k, ctx := setupKnowledgeTest(t)

	// Index 5 rounds at blocks 100, 200, 300, 400, 500
	for i, block := range []uint64{100, 200, 300, 400, 500} {
		meta := &types.CompletedRoundMeta{
			Domain:         "physics",
			HasDissent:     i%2 == 0,
			DurationBlocks: 11,
		}
		err := k.IndexCompletedRound(ctx, block, fmt.Sprintf("round-%d", i), meta)
		require.NoError(t, err)
	}

	// Window [200, 500] should contain 4 rounds (blocks 200, 300, 400, 500)
	count := k.CountCompletedRoundsInWindow(ctx, 500, 300)
	require.Equal(t, uint64(4), count)

	// Window [400, 500] should contain 2 rounds (blocks 400, 500)
	count = k.CountCompletedRoundsInWindow(ctx, 500, 100)
	require.Equal(t, uint64(2), count)
}

func TestCompletionIndex_CountDisputedRoundsInWindow(t *testing.T) {
	k, ctx := setupKnowledgeTest(t)

	k.IndexCompletedRound(ctx, 100, "r1", &types.CompletedRoundMeta{Domain: "physics", HasDissent: true, DurationBlocks: 10})
	k.IndexCompletedRound(ctx, 200, "r2", &types.CompletedRoundMeta{Domain: "physics", HasDissent: false, DurationBlocks: 12})
	k.IndexCompletedRound(ctx, 300, "r3", &types.CompletedRoundMeta{Domain: "physics", HasDissent: true, DurationBlocks: 8})

	disputed := k.CountDisputedRoundsInWindow(ctx, 300, 300)
	require.Equal(t, uint64(2), disputed)
}

func TestCompletionIndex_GetAvgRoundDurationInWindow(t *testing.T) {
	k, ctx := setupKnowledgeTest(t)

	k.IndexCompletedRound(ctx, 100, "r1", &types.CompletedRoundMeta{Domain: "physics", DurationBlocks: 10})
	k.IndexCompletedRound(ctx, 200, "r2", &types.CompletedRoundMeta{Domain: "physics", DurationBlocks: 20})

	avg := k.GetAvgRoundDurationInWindow(ctx, 200, 200)
	require.Equal(t, uint64(15), avg) // (10+20)/2
}

func TestCompletionIndex_CountForDomainInWindow(t *testing.T) {
	k, ctx := setupKnowledgeTest(t)

	k.IndexCompletedRound(ctx, 100, "r1", &types.CompletedRoundMeta{Domain: "physics", DurationBlocks: 10})
	k.IndexCompletedRound(ctx, 200, "r2", &types.CompletedRoundMeta{Domain: "chemistry", DurationBlocks: 10})
	k.IndexCompletedRound(ctx, 300, "r3", &types.CompletedRoundMeta{Domain: "physics", DurationBlocks: 10})

	physics := k.CountCompletedRoundsForDomainInWindow(ctx, "physics", 300, 300)
	require.Equal(t, uint64(2), physics)

	chemistry := k.CountCompletedRoundsForDomainInWindow(ctx, "chemistry", 300, 300)
	require.Equal(t, uint64(1), chemistry)
}

func TestCompletionIndex_EmptyWindow(t *testing.T) {
	k, ctx := setupKnowledgeTest(t)

	count := k.CountCompletedRoundsInWindow(ctx, 500, 300)
	require.Equal(t, uint64(0), count)

	avg := k.GetAvgRoundDurationInWindow(ctx, 500, 300)
	require.Equal(t, uint64(0), avg)
}
