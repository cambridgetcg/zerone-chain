package keeper_test

import (
	"strings"
	"testing"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/stretchr/testify/require"

	"github.com/zerone-chain/zerone/x/knowledge/keeper"
	"github.com/zerone-chain/zerone/x/knowledge/types"
)

// ─── 1. TestRecordQueryReceipt ───────────────────────────────────────────────

func TestRecordQueryReceipt(t *testing.T) {
	k, ctx := setupKnowledgeTest(t)
	rater := "zrn1rater1"
	factID := "fact-abc"

	// No receipt initially
	require.False(t, k.HasQueryReceipt(ctx, rater, factID))

	// Record receipt
	require.NoError(t, k.RecordQueryReceipt(ctx, rater, factID))

	// Receipt exists
	require.True(t, k.HasQueryReceipt(ctx, rater, factID))

	// Different rater has no receipt
	require.False(t, k.HasQueryReceipt(ctx, "zrn1other", factID))

	// Different fact has no receipt
	require.False(t, k.HasQueryReceipt(ctx, rater, "fact-xyz"))
}

// ─── 2. TestConsumeQueryReceipt ──────────────────────────────────────────────

func TestConsumeQueryReceipt(t *testing.T) {
	k, ctx := setupKnowledgeTest(t)
	rater := "zrn1rater1"
	factID := "fact-abc"

	// Record and verify
	require.NoError(t, k.RecordQueryReceipt(ctx, rater, factID))
	require.True(t, k.HasQueryReceipt(ctx, rater, factID))

	// Consume
	require.NoError(t, k.ConsumeQueryReceipt(ctx, rater, factID))

	// Receipt gone after consumption
	require.False(t, k.HasQueryReceipt(ctx, rater, factID))
}

// ─── 3. TestClearQueryReceipts ───────────────────────────────────────────────

func TestClearQueryReceipts(t *testing.T) {
	k, ctx := setupKnowledgeTest(t)

	// Create multiple receipts
	require.NoError(t, k.RecordQueryReceipt(ctx, "zrn1a", "fact-1"))
	require.NoError(t, k.RecordQueryReceipt(ctx, "zrn1b", "fact-2"))
	require.NoError(t, k.RecordQueryReceipt(ctx, "zrn1c", "fact-3"))

	// All exist
	require.True(t, k.HasQueryReceipt(ctx, "zrn1a", "fact-1"))
	require.True(t, k.HasQueryReceipt(ctx, "zrn1b", "fact-2"))
	require.True(t, k.HasQueryReceipt(ctx, "zrn1c", "fact-3"))

	// Clear all
	k.ClearQueryReceipts(ctx)

	// All gone
	require.False(t, k.HasQueryReceipt(ctx, "zrn1a", "fact-1"))
	require.False(t, k.HasQueryReceipt(ctx, "zrn1b", "fact-2"))
	require.False(t, k.HasQueryReceipt(ctx, "zrn1c", "fact-3"))
}

// ─── 4. TestRateFactPositive ─────────────────────────────────────────────────

func TestRateFactPositive(t *testing.T) {
	k, ctx := setupKnowledgeTest(t)
	rater := "zrn1rater1"

	fact := makeTestFact(t, k, ctx, "fact-pos", "Positive fact content here", "physics", "empirical", "zrn1sub", 800_000)

	// Record query receipt
	require.NoError(t, k.RecordQueryReceipt(ctx, rater, fact.Id))

	// Rate as useful
	msgServer := keeper.NewMsgServerImpl(k)
	_, err := msgServer.RateFact(ctx, &types.MsgRateFact{
		Rater:  rater,
		FactId: fact.Id,
		Useful: true,
	})
	require.NoError(t, err)

	// Check satisfaction counters incremented
	updated, found := k.GetFact(ctx, fact.Id)
	require.True(t, found)
	require.Equal(t, uint64(1), updated.SatisfactionUp)
	require.Equal(t, uint64(1), updated.SatisfactionUpEpoch)
	require.Equal(t, uint64(0), updated.SatisfactionDown)
	require.Equal(t, uint64(0), updated.SatisfactionDownEpoch)
}

// ─── 5. TestRateFactNegative ─────────────────────────────────────────────────

func TestRateFactNegative(t *testing.T) {
	k, ctx := setupKnowledgeTest(t)
	rater := "zrn1rater2"

	fact := makeTestFact(t, k, ctx, "fact-neg", "Negative fact content here", "physics", "empirical", "zrn1sub", 800_000)

	// Record query receipt
	require.NoError(t, k.RecordQueryReceipt(ctx, rater, fact.Id))

	// Rate as not useful
	msgServer := keeper.NewMsgServerImpl(k)
	_, err := msgServer.RateFact(ctx, &types.MsgRateFact{
		Rater:  rater,
		FactId: fact.Id,
		Useful: false,
	})
	require.NoError(t, err)

	// Check satisfaction counters
	updated, found := k.GetFact(ctx, fact.Id)
	require.True(t, found)
	require.Equal(t, uint64(0), updated.SatisfactionUp)
	require.Equal(t, uint64(0), updated.SatisfactionUpEpoch)
	require.Equal(t, uint64(1), updated.SatisfactionDown)
	require.Equal(t, uint64(1), updated.SatisfactionDownEpoch)
}

// ─── 6. TestRateFactNoReceipt ────────────────────────────────────────────────

func TestRateFactNoReceipt(t *testing.T) {
	k, ctx := setupKnowledgeTest(t)

	fact := makeTestFact(t, k, ctx, "fact-norec", "No receipt fact content", "physics", "empirical", "zrn1sub", 800_000)

	// Try to rate without querying first
	msgServer := keeper.NewMsgServerImpl(k)
	_, err := msgServer.RateFact(ctx, &types.MsgRateFact{
		Rater:  "zrn1norec",
		FactId: fact.Id,
		Useful: true,
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "no query receipt")
}

// ─── 7. TestRateFactDoubleRate ───────────────────────────────────────────────

func TestRateFactDoubleRate(t *testing.T) {
	k, ctx := setupKnowledgeTest(t)
	rater := "zrn1double"

	fact := makeTestFact(t, k, ctx, "fact-dbl", "Double rate fact content", "physics", "empirical", "zrn1sub", 800_000)

	// Record receipt and rate once
	require.NoError(t, k.RecordQueryReceipt(ctx, rater, fact.Id))

	msgServer := keeper.NewMsgServerImpl(k)
	_, err := msgServer.RateFact(ctx, &types.MsgRateFact{
		Rater:  rater,
		FactId: fact.Id,
		Useful: true,
	})
	require.NoError(t, err)

	// Second rating fails (receipt consumed)
	_, err = msgServer.RateFact(ctx, &types.MsgRateFact{
		Rater:  rater,
		FactId: fact.Id,
		Useful: false,
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "no query receipt")

	// Only the first rating was applied
	updated, found := k.GetFact(ctx, fact.Id)
	require.True(t, found)
	require.Equal(t, uint64(1), updated.SatisfactionUp)
	require.Equal(t, uint64(0), updated.SatisfactionDown)
}

// ─── 8. TestSatisfactionFitnessImpact ────────────────────────────────────────

func TestSatisfactionFitnessImpact(t *testing.T) {
	k, ctx := setupKnowledgeTest(t)

	// Create two facts with similar base stats but different satisfaction
	highSat := makeTestFact(t, k, ctx, "fact-highsat", "High satisfaction fact content", "physics", "empirical", "zrn1sub", 800_000)
	highSat.QueryCountEpoch = 100
	highSat.SatisfactionUpEpoch = 10
	highSat.SatisfactionDownEpoch = 0
	require.NoError(t, k.SetFact(ctx, highSat))

	lowSat := makeTestFact(t, k, ctx, "fact-lowsat", "Low satisfaction fact content here", "physics", "empirical", "zrn1sub", 800_000)
	lowSat.QueryCountEpoch = 100
	lowSat.SatisfactionUpEpoch = 0
	lowSat.SatisfactionDownEpoch = 10
	require.NoError(t, k.SetFact(ctx, lowSat))

	// Calculate fitness for both
	highFitness := k.CalculateFitness(ctx, highSat, 1)
	lowFitness := k.CalculateFitness(ctx, lowSat, 1)

	// High satisfaction should score better
	require.Greater(t, highFitness, lowFitness,
		"high satisfaction (%d) should beat low satisfaction (%d)", highFitness, lowFitness)
}

// ─── 9. TestSatisfactionMinRatings ───────────────────────────────────────────

func TestSatisfactionMinRatings(t *testing.T) {
	k, ctx := setupKnowledgeTest(t)

	// Fact with fewer ratings than min threshold (default 3)
	fewRatings := makeTestFact(t, k, ctx, "fact-few", "Few ratings fact content here", "physics", "empirical", "zrn1sub", 800_000)
	fewRatings.QueryCountEpoch = 100
	fewRatings.SatisfactionUpEpoch = 0
	fewRatings.SatisfactionDownEpoch = 2 // Below min (3)
	require.NoError(t, k.SetFact(ctx, fewRatings))

	// Fact with no ratings at all
	noRatings := makeTestFact(t, k, ctx, "fact-none", "No ratings fact content here.", "physics", "empirical", "zrn1sub", 800_000)
	noRatings.QueryCountEpoch = 100
	require.NoError(t, k.SetFact(ctx, noRatings))

	// Both should have the same fitness (neutral satisfaction = 500k for both)
	fewFitness := k.CalculateFitness(ctx, fewRatings, 1)
	noneFitness := k.CalculateFitness(ctx, noRatings, 1)

	require.Equal(t, fewFitness, noneFitness,
		"below min ratings (%d) and no ratings (%d) should both use neutral default", fewFitness, noneFitness)
}

// ─── 10. TestSatisfactionEpochReset ──────────────────────────────────────────

func TestSatisfactionEpochReset(t *testing.T) {
	k, ctx := setupKnowledgeTest(t)

	// Create fact with satisfaction data
	fact := makeTestFact(t, k, ctx, "fact-epoch", "Epoch reset test fact content", "physics", "empirical", "zrn1sub", 800_000)
	fact.SatisfactionUp = 10
	fact.SatisfactionDown = 3
	fact.SatisfactionUpEpoch = 5
	fact.SatisfactionDownEpoch = 2
	fact.QueryCountEpoch = 50
	require.NoError(t, k.SetFact(ctx, fact))

	// Also add some query receipts
	require.NoError(t, k.RecordQueryReceipt(ctx, "zrn1a", fact.Id))
	require.NoError(t, k.RecordQueryReceipt(ctx, "zrn1b", fact.Id))

	// Advance to epoch boundary and run update
	params, _ := k.GetParams(ctx)
	epochHeight := int64(params.FitnessEpochBlocks)
	ctx = ctx.WithBlockHeight(epochHeight)

	require.NoError(t, k.UpdateAllFitnessScores(ctx))

	// Epoch counters reset, lifetime counters preserved
	updated, found := k.GetFact(ctx, fact.Id)
	require.True(t, found)
	require.Equal(t, uint64(10), updated.SatisfactionUp, "lifetime up should be preserved")
	require.Equal(t, uint64(3), updated.SatisfactionDown, "lifetime down should be preserved")
	require.Equal(t, uint64(0), updated.SatisfactionUpEpoch, "epoch up should be reset")
	require.Equal(t, uint64(0), updated.SatisfactionDownEpoch, "epoch down should be reset")
	require.Equal(t, uint64(0), updated.QueryCountEpoch, "query count epoch should be reset")

	// Clear receipts (tested separately — called in BeginBlocker)
	k.ClearQueryReceipts(ctx)
	require.False(t, k.HasQueryReceipt(ctx, "zrn1a", fact.Id))
	require.False(t, k.HasQueryReceipt(ctx, "zrn1b", fact.Id))
}

// ─── 11. TestMemoTooLong ─────────────────────────────────────────────────────

func TestMemoTooLong(t *testing.T) {
	k, ctx := setupKnowledgeTest(t)

	fact := makeTestFact(t, k, ctx, "fact-memo", "Memo test fact content here.", "physics", "empirical", "zrn1sub", 800_000)

	// Record receipt
	require.NoError(t, k.RecordQueryReceipt(ctx, "zrn1memoer", fact.Id))

	// Rate with memo > 256 chars
	msgServer := keeper.NewMsgServerImpl(k)
	_, err := msgServer.RateFact(ctx, &types.MsgRateFact{
		Rater:  "zrn1memoer",
		FactId: fact.Id,
		Useful: true,
		Memo:   strings.Repeat("x", 257),
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "memo exceeds 256 characters")

	// Receipt should NOT be consumed on validation failure
	require.True(t, k.HasQueryReceipt(ctx, "zrn1memoer", fact.Id))

	// Valid memo succeeds
	_, err = msgServer.RateFact(sdk.WrapSDKContext(ctx), &types.MsgRateFact{
		Rater:  "zrn1memoer",
		FactId: fact.Id,
		Useful: true,
		Memo:   strings.Repeat("x", 256), // exactly at limit
	})
	require.NoError(t, err)
}
