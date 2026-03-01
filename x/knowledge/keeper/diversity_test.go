package keeper_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/zerone-chain/zerone/x/knowledge/keeper"
)

// ─── ComputeRoundEntropy (pure function) ─────────────────────────────────────

func TestComputeRoundEntropy_UnanimousAccept(t *testing.T) {
	// All accept → entropy = 0
	got := keeper.ComputeRoundEntropy(5, 0)
	require.Equal(t, uint64(0), got, "unanimous accept should yield 0 entropy")
}

func TestComputeRoundEntropy_UnanimousReject(t *testing.T) {
	// All reject → entropy = 0
	got := keeper.ComputeRoundEntropy(0, 5)
	require.Equal(t, uint64(0), got, "unanimous reject should yield 0 entropy")
}

func TestComputeRoundEntropy_PerfectSplit(t *testing.T) {
	// 50/50 split → max entropy = 1,000,000
	got := keeper.ComputeRoundEntropy(5, 5)
	require.Equal(t, uint64(1_000_000), got, "50/50 split should yield max entropy")
}

func TestComputeRoundEntropy_80_20_Split(t *testing.T) {
	// 80/20 split → ~721,928 BPS
	got := keeper.ComputeRoundEntropy(8, 2)
	require.InDelta(t, 721_928, float64(got), 50_000, "80/20 split should be ~721,928 (tolerance 50K)")
}

func TestComputeRoundEntropy_NoVoters(t *testing.T) {
	// 0 voters → entropy = 0
	got := keeper.ComputeRoundEntropy(0, 0)
	require.Equal(t, uint64(0), got, "no voters should yield 0 entropy")
}

func TestComputeRoundEntropy_SingleVoter(t *testing.T) {
	// 1 voter → entropy = 0 (unanimous)
	got := keeper.ComputeRoundEntropy(1, 0)
	require.Equal(t, uint64(0), got, "single voter should yield 0 entropy")
}

func TestComputeRoundEntropy_3_1_Split(t *testing.T) {
	// 3-1 split → ~811,278 BPS
	got := keeper.ComputeRoundEntropy(3, 1)
	require.InDelta(t, 811_278, float64(got), 50_000, "3-1 split should be ~811,278 (tolerance 50K)")
}

// ─── Set/Get RoundDiversity ──────────────────────────────────────────────────

func TestSetGetRoundDiversity(t *testing.T) {
	k, ctx := setupKnowledgeTest(t)

	rec := keeper.RoundDiversityRecord{
		RoundID:     "round001",
		Entropy:     500_000,
		AcceptCount: 3,
		RejectCount: 2,
		TotalVoters: 5,
		Domain:      "physics",
		Epoch:       1,
	}

	err := k.SetRoundDiversity(ctx, "round001", rec)
	require.NoError(t, err)

	got, found, err := k.GetRoundDiversity(ctx, "round001")
	require.NoError(t, err)
	require.True(t, found)
	require.Equal(t, rec.Entropy, got.Entropy)
	require.Equal(t, rec.AcceptCount, got.AcceptCount)
	require.Equal(t, rec.RejectCount, got.RejectCount)
	require.Equal(t, rec.Domain, got.Domain)
}

// ─── Set/Get DomainDiversity ─────────────────────────────────────────────────

func TestSetGetDomainDiversity(t *testing.T) {
	k, ctx := setupKnowledgeTest(t)

	rec := keeper.DomainDiversityRecord{
		Domain:         "mathematics",
		Epoch:          5,
		AvgEntropy:     300_000,
		RoundCount:     10,
		UnanimousCount: 3,
	}

	err := k.SetDomainDiversity(ctx, "mathematics", 5, rec)
	require.NoError(t, err)

	got, found, err := k.GetDomainDiversity(ctx, "mathematics", 5)
	require.NoError(t, err)
	require.True(t, found)
	require.Equal(t, rec.AvgEntropy, got.AvgEntropy)
	require.Equal(t, rec.RoundCount, got.RoundCount)
	require.Equal(t, rec.UnanimousCount, got.UnanimousCount)
}

// ─── Set/Get ValidatorIndependence ───────────────────────────────────────────

func TestSetGetValidatorIndependence(t *testing.T) {
	k, ctx := setupKnowledgeTest(t)

	rec := keeper.ValidatorIndependenceRecord{
		Validator:     "zrn1validator1",
		TotalVotes:    20,
		MinorityVotes: 3,
		LastEpoch:     7,
	}

	err := k.SetValidatorIndependence(ctx, "zrn1validator1", rec)
	require.NoError(t, err)

	got, found, err := k.GetValidatorIndependence(ctx, "zrn1validator1")
	require.NoError(t, err)
	require.True(t, found)
	require.Equal(t, rec.TotalVotes, got.TotalVotes)
	require.Equal(t, rec.MinorityVotes, got.MinorityVotes)
	require.Equal(t, rec.LastEpoch, got.LastEpoch)
}

// ─── Set/Get ConformityStreak ────────────────────────────────────────────────

func TestSetGetConformityStreak(t *testing.T) {
	k, ctx := setupKnowledgeTest(t)

	rec := keeper.ConformityStreakRecord{
		Domain:            "logic",
		ConsecutiveEpochs: 4,
		LastEpoch:         10,
	}

	err := k.SetConformityStreak(ctx, "logic", rec)
	require.NoError(t, err)

	got, found, err := k.GetConformityStreak(ctx, "logic")
	require.NoError(t, err)
	require.True(t, found)
	require.Equal(t, rec.ConsecutiveEpochs, got.ConsecutiveEpochs)
	require.Equal(t, rec.LastEpoch, got.LastEpoch)
}

// ─── RecordRoundDiversity ────────────────────────────────────────────────────

func TestRecordRoundDiversity_Unanimous(t *testing.T) {
	k, ctx := setupKnowledgeTest(t)

	err := k.RecordRoundDiversity(ctx, "round_unan", "physics", 5, 0)
	require.NoError(t, err)

	rec, found, err := k.GetRoundDiversity(ctx, "round_unan")
	require.NoError(t, err)
	require.True(t, found)
	require.Equal(t, uint64(0), rec.Entropy, "unanimous round should have 0 entropy")
}

func TestRecordRoundDiversity_Split(t *testing.T) {
	k, ctx := setupKnowledgeTest(t)

	err := k.RecordRoundDiversity(ctx, "round_split", "physics", 3, 2)
	require.NoError(t, err)

	rec, found, err := k.GetRoundDiversity(ctx, "round_split")
	require.NoError(t, err)
	require.True(t, found)
	require.Greater(t, rec.Entropy, uint64(0), "split round should have non-zero entropy")
}

// ─── UpdateValidatorIndependence ─────────────────────────────────────────────

func TestUpdateValidatorIndependence_MajorityVoter(t *testing.T) {
	k, ctx := setupKnowledgeTest(t)

	// Validator votes with the majority → 0 minority votes
	err := k.UpdateValidatorIndependence(ctx, "val1", "accept", "accept")
	require.NoError(t, err)

	rec, found, err := k.GetValidatorIndependence(ctx, "val1")
	require.NoError(t, err)
	require.True(t, found)
	require.Equal(t, uint64(1), rec.TotalVotes)
	require.Equal(t, uint64(0), rec.MinorityVotes)
}

func TestUpdateValidatorIndependence_MinorityVoter(t *testing.T) {
	k, ctx := setupKnowledgeTest(t)

	// Validator votes against the majority → 1 minority vote
	err := k.UpdateValidatorIndependence(ctx, "val2", "reject", "accept")
	require.NoError(t, err)

	rec, found, err := k.GetValidatorIndependence(ctx, "val2")
	require.NoError(t, err)
	require.True(t, found)
	require.Equal(t, uint64(1), rec.TotalVotes)
	require.Equal(t, uint64(1), rec.MinorityVotes)
}

func TestUpdateValidatorIndependence_HealthyDissenter(t *testing.T) {
	k, ctx := setupKnowledgeTest(t)

	// Simulate 20 votes, 3 of which are minority
	for i := 0; i < 17; i++ {
		err := k.UpdateValidatorIndependence(ctx, "val3", "accept", "accept")
		require.NoError(t, err)
	}
	for i := 0; i < 3; i++ {
		err := k.UpdateValidatorIndependence(ctx, "val3", "reject", "accept")
		require.NoError(t, err)
	}

	rec, found, err := k.GetValidatorIndependence(ctx, "val3")
	require.NoError(t, err)
	require.True(t, found)
	require.Equal(t, uint64(20), rec.TotalVotes)
	require.Equal(t, uint64(3), rec.MinorityVotes)
}

// ─── AggregateDomainDiversity ────────────────────────────────────────────────

func TestAggregateDomainDiversity_AllUnanimous(t *testing.T) {
	k, ctx := setupKnowledgeTest(t)

	// Record 3 unanimous rounds in the same domain/epoch
	for i := 0; i < 3; i++ {
		roundID := "round_unan_" + string(rune('a'+i))
		err := k.RecordRoundDiversity(ctx, roundID, "physics", 5, 0)
		require.NoError(t, err)
	}

	err := k.AggregateDomainDiversity(ctx, "physics", 0)
	require.NoError(t, err)

	rec, found, err := k.GetDomainDiversity(ctx, "physics", 0)
	require.NoError(t, err)
	require.True(t, found)
	require.Equal(t, uint64(0), rec.AvgEntropy, "all unanimous rounds should yield avg entropy 0")
	require.Equal(t, uint64(3), rec.UnanimousCount, "should count 3 unanimous rounds")
	require.Equal(t, uint64(3), rec.RoundCount)
}

func TestAggregateDomainDiversity_Mixed(t *testing.T) {
	k, ctx := setupKnowledgeTest(t)

	// 1 unanimous round + 1 split round
	err := k.RecordRoundDiversity(ctx, "round_m1", "physics", 5, 0)
	require.NoError(t, err)

	err = k.RecordRoundDiversity(ctx, "round_m2", "physics", 3, 2)
	require.NoError(t, err)

	err = k.AggregateDomainDiversity(ctx, "physics", 0)
	require.NoError(t, err)

	rec, found, err := k.GetDomainDiversity(ctx, "physics", 0)
	require.NoError(t, err)
	require.True(t, found)
	require.Greater(t, rec.AvgEntropy, uint64(0), "mixed rounds should yield non-zero avg entropy")
	require.Equal(t, uint64(2), rec.RoundCount)
}

// ─── Conformity streak ──────────────────────────────────────────────────────

func TestConformityStreak_IncrementAndReset(t *testing.T) {
	k, ctx := setupKnowledgeTest(t)

	// Increment 3 times
	err := k.IncrementConformityStreak(ctx, "physics", 1)
	require.NoError(t, err)
	err = k.IncrementConformityStreak(ctx, "physics", 2)
	require.NoError(t, err)
	err = k.IncrementConformityStreak(ctx, "physics", 3)
	require.NoError(t, err)

	rec, found, err := k.GetConformityStreak(ctx, "physics")
	require.NoError(t, err)
	require.True(t, found)
	require.Equal(t, uint64(3), rec.ConsecutiveEpochs)
	require.Equal(t, uint64(3), rec.LastEpoch)

	// Reset
	err = k.ResetConformityStreak(ctx, "physics")
	require.NoError(t, err)

	rec, found, err = k.GetConformityStreak(ctx, "physics")
	require.NoError(t, err)
	require.True(t, found)
	require.Equal(t, uint64(0), rec.ConsecutiveEpochs)
}

// ─── GetGlobalConsensusDiversity ─────────────────────────────────────────────

func TestGetGlobalConsensusDiversity_NoData(t *testing.T) {
	k, ctx := setupKnowledgeTest(t)

	score := k.GetGlobalConsensusDiversity(ctx)
	require.Equal(t, uint64(500_000), score, "no data should return neutral BPS (500,000)")
}
