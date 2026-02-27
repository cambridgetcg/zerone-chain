package keeper_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/zerone-chain/zerone/x/knowledge/types"
)

func TestVindicationPending_SetGetDelete(t *testing.T) {
	k, ctx := setupKnowledgeTest(t)

	factId := "fact_abc123"

	// Initially empty
	got := k.GetVindicationPending(ctx, factId)
	require.Nil(t, got)

	// Set entries
	entries := []types.VindicationEntry{
		{
			Verifier:    "zrn1validator1",
			Vote:        "REJECT",
			SlashAmount: "5000",
			SlashBps:    50000,
			RoundId:     "round_001",
			FactId:      factId,
			Height:      100,
		},
		{
			Verifier:    "zrn1validator2",
			Vote:        "REJECT",
			SlashAmount: "3000",
			SlashBps:    30000,
			RoundId:     "round_001",
			FactId:      factId,
			Height:      100,
		},
	}
	require.NoError(t, k.SetVindicationPending(ctx, factId, entries))

	// Get them back
	got = k.GetVindicationPending(ctx, factId)
	require.Len(t, got, 2)
	require.Equal(t, "zrn1validator1", got[0].Verifier)
	require.Equal(t, "REJECT", got[0].Vote)
	require.Equal(t, "5000", got[0].SlashAmount)
	require.Equal(t, uint64(50000), got[0].SlashBps)
	require.Equal(t, "zrn1validator2", got[1].Verifier)
	require.Equal(t, "3000", got[1].SlashAmount)

	// Delete
	k.DeleteVindicationPending(ctx, factId)

	// Verify empty after delete
	got = k.GetVindicationPending(ctx, factId)
	require.Nil(t, got)
}

func TestVindicationRecord_SetGet(t *testing.T) {
	k, ctx := setupKnowledgeTest(t)

	factId := "fact_xyz789"
	verifier := "zrn1validator1"

	// Non-existent record returns false
	_, found := k.GetVindicationRecord(ctx, factId, verifier)
	require.False(t, found)

	// Store a record
	record := types.VindicationRecord{
		Verifier:     verifier,
		FactId:       factId,
		RefundAmount: "5000",
		BonusAmount:  "1000",
		VindicatedAt: 500,
		DisprovenBy:  "challenge_round_002",
		RoundId:      "round_001",
	}
	require.NoError(t, k.SetVindicationRecord(ctx, factId, record))

	// Retrieve it
	got, found := k.GetVindicationRecord(ctx, factId, verifier)
	require.True(t, found)
	require.Equal(t, verifier, got.Verifier)
	require.Equal(t, factId, got.FactId)
	require.Equal(t, "5000", got.RefundAmount)
	require.Equal(t, "1000", got.BonusAmount)
	require.Equal(t, uint64(500), got.VindicatedAt)
	require.Equal(t, "challenge_round_002", got.DisprovenBy)
	require.Equal(t, "round_001", got.RoundId)

	// Different verifier returns false
	_, found = k.GetVindicationRecord(ctx, factId, "zrn1validator99")
	require.False(t, found)
}

func TestVindicationPending_GetAllPending(t *testing.T) {
	k, ctx := setupKnowledgeTest(t)

	factId1 := "fact_aaa"
	factId2 := "fact_bbb"

	entries1 := []types.VindicationEntry{
		{
			Verifier:    "zrn1validator1",
			Vote:        "REJECT",
			SlashAmount: "5000",
			SlashBps:    50000,
			RoundId:     "round_001",
			FactId:      factId1,
			Height:      100,
		},
	}
	entries2 := []types.VindicationEntry{
		{
			Verifier:    "zrn1validator3",
			Vote:        "REJECT",
			SlashAmount: "8000",
			SlashBps:    80000,
			RoundId:     "round_002",
			FactId:      factId2,
			Height:      200,
		},
		{
			Verifier:    "zrn1validator4",
			Vote:        "REJECT",
			SlashAmount: "2000",
			SlashBps:    20000,
			RoundId:     "round_002",
			FactId:      factId2,
			Height:      200,
		},
	}

	require.NoError(t, k.SetVindicationPending(ctx, factId1, entries1))
	require.NoError(t, k.SetVindicationPending(ctx, factId2, entries2))

	// GetAll returns both
	all := k.GetAllVindicationPending(ctx)
	require.Len(t, all, 2)

	require.Contains(t, all, factId1)
	require.Contains(t, all, factId2)
	require.Len(t, all[factId1], 1)
	require.Len(t, all[factId2], 2)
	require.Equal(t, "zrn1validator1", all[factId1][0].Verifier)
	require.Equal(t, "zrn1validator3", all[factId2][0].Verifier)
	require.Equal(t, "zrn1validator4", all[factId2][1].Verifier)
}
