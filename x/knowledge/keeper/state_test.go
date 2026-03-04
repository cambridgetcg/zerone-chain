package keeper_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/zerone-chain/zerone/x/knowledge/types"
)

func TestSubmissionCRUD(t *testing.T) {
	k, ctx := setupKeeper(t)

	// Get missing returns false
	_, ok := k.GetSubmission(ctx, "nonexistent")
	require.False(t, ok)

	// Set and get
	sub := &types.Submission{
		Id:        "1",
		Submitter: testAddr,
		Content:   "hello world",
		Domain:    "science",
	}
	err := k.SetSubmission(ctx, sub)
	require.NoError(t, err)

	got, ok := k.GetSubmission(ctx, "1")
	require.True(t, ok)
	require.Equal(t, "1", got.Id)
	require.Equal(t, testAddr, got.Submitter)
	require.Equal(t, "hello world", got.Content)
	require.Equal(t, "science", got.Domain)

	// Delete
	err = k.DeleteSubmission(ctx, "1")
	require.NoError(t, err)

	_, ok = k.GetSubmission(ctx, "1")
	require.False(t, ok)
}

func TestContentHashIndex(t *testing.T) {
	k, ctx := setupKeeper(t)

	// Missing hash
	require.False(t, k.HasContentHash(ctx, "abc123"))

	// Set and check
	err := k.SetContentHash(ctx, "abc123", "sub-1")
	require.NoError(t, err)
	require.True(t, k.HasContentHash(ctx, "abc123"))

	// Different hash still missing
	require.False(t, k.HasContentHash(ctx, "def456"))
}

func TestNextSubmissionID(t *testing.T) {
	k, ctx := setupKeeper(t)

	// First call returns "1"
	id1 := k.NextSubmissionID(ctx)
	require.Equal(t, "1", id1)

	// Second call returns "2"
	id2 := k.NextSubmissionID(ctx)
	require.Equal(t, "2", id2)

	// Third call returns "3"
	id3 := k.NextSubmissionID(ctx)
	require.Equal(t, "3", id3)
}

func TestSubmissionIterator(t *testing.T) {
	k, ctx := setupKeeper(t)

	subs := []*types.Submission{
		{Id: "a", Submitter: testAddr, Domain: "science"},
		{Id: "b", Submitter: testAddr, Domain: "math"},
		{Id: "c", Submitter: testAddr, Domain: "art"},
	}
	for _, s := range subs {
		require.NoError(t, k.SetSubmission(ctx, s))
	}

	var collected []string
	k.IterateSubmissions(ctx, func(sub *types.Submission) bool {
		collected = append(collected, sub.Id)
		return false
	})
	require.Len(t, collected, 3)
	require.Contains(t, collected, "a")
	require.Contains(t, collected, "b")
	require.Contains(t, collected, "c")
}

func TestSubmissionsByDomain(t *testing.T) {
	k, ctx := setupKeeper(t)

	// Two in "science", one in "math"
	require.NoError(t, k.SetSubmissionDomainIndex(ctx, "science", "s1"))
	require.NoError(t, k.SetSubmissionDomainIndex(ctx, "science", "s2"))
	require.NoError(t, k.SetSubmissionDomainIndex(ctx, "math", "s3"))

	scienceIDs := k.GetSubmissionsByDomain(ctx, "science")
	require.Len(t, scienceIDs, 2)
	require.Contains(t, scienceIDs, "s1")
	require.Contains(t, scienceIDs, "s2")

	mathIDs := k.GetSubmissionsByDomain(ctx, "math")
	require.Len(t, mathIDs, 1)
	require.Equal(t, "s3", mathIDs[0])

	// Empty domain
	emptyIDs := k.GetSubmissionsByDomain(ctx, "history")
	require.Empty(t, emptyIDs)
}

func TestSubmissionsBySubmitter(t *testing.T) {
	k, ctx := setupKeeper(t)

	addr2 := "zrn1qqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqq5z5r7e"

	require.NoError(t, k.SetSubmissionSubmitterIndex(ctx, testAddr, "s1"))
	require.NoError(t, k.SetSubmissionSubmitterIndex(ctx, testAddr, "s2"))
	require.NoError(t, k.SetSubmissionSubmitterIndex(ctx, addr2, "s3"))

	ids := k.GetSubmissionsBySubmitter(ctx, testAddr)
	require.Len(t, ids, 2)
	require.Contains(t, ids, "s1")
	require.Contains(t, ids, "s2")

	ids2 := k.GetSubmissionsBySubmitter(ctx, addr2)
	require.Len(t, ids2, 1)
	require.Equal(t, "s3", ids2[0])
}

// ─── QualityRound CRUD ─────────────────────────────────────────────────────

func TestQualityRoundCRUD(t *testing.T) {
	k, ctx := setupKeeper(t)

	// Get missing returns false
	_, ok := k.GetQualityRound(ctx, "nonexistent")
	require.False(t, ok)

	// Set and get
	round := &types.QualityRound{
		Id:              "1",
		SubmissionId:    "sub-1",
		StartedAtBlock:  100,
		Phase:           types.VerificationPhase_VERIFICATION_PHASE_COMMIT,
		CommitDeadline:  110,
		RevealDeadline:  120,
	}
	err := k.SetQualityRound(ctx, round)
	require.NoError(t, err)

	got, ok := k.GetQualityRound(ctx, "1")
	require.True(t, ok)
	require.Equal(t, "1", got.Id)
	require.Equal(t, "sub-1", got.SubmissionId)
	require.Equal(t, uint64(100), got.StartedAtBlock)
	require.Equal(t, types.VerificationPhase_VERIFICATION_PHASE_COMMIT, got.Phase)

	// Delete
	err = k.DeleteQualityRound(ctx, "1")
	require.NoError(t, err)

	_, ok = k.GetQualityRound(ctx, "1")
	require.False(t, ok)
}

// ─── Sample CRUD ────────────────────────────────────────────────────────────

func TestSampleCRUD(t *testing.T) {
	k, ctx := setupKeeper(t)

	// Get missing returns false
	_, ok := k.GetSample(ctx, "nonexistent")
	require.False(t, ok)

	// Set and get
	sample := &types.Sample{
		Id:           "1",
		Content:      "validated training data",
		Domain:       "science",
		Submitter:    testAddr,
		QualityScore: 85,
		QualityTier:  "gold",
		SubmissionId: "sub-1",
	}
	err := k.SetSample(ctx, sample)
	require.NoError(t, err)

	got, ok := k.GetSample(ctx, "1")
	require.True(t, ok)
	require.Equal(t, "1", got.Id)
	require.Equal(t, "validated training data", got.Content)
	require.Equal(t, "science", got.Domain)
	require.Equal(t, testAddr, got.Submitter)
	require.Equal(t, uint64(85), got.QualityScore)
	require.Equal(t, "gold", got.QualityTier)

	// Delete
	err = k.DeleteSample(ctx, "1")
	require.NoError(t, err)

	_, ok = k.GetSample(ctx, "1")
	require.False(t, ok)
}

// ─── Sequences ──────────────────────────────────────────────────────────────

func TestNextRoundID(t *testing.T) {
	k, ctx := setupKeeper(t)

	id1 := k.NextRoundID(ctx)
	require.Equal(t, "1", id1)

	id2 := k.NextRoundID(ctx)
	require.Equal(t, "2", id2)

	id3 := k.NextRoundID(ctx)
	require.Equal(t, "3", id3)
}

func TestNextSampleID(t *testing.T) {
	k, ctx := setupKeeper(t)

	id1 := k.NextSampleID(ctx)
	require.Equal(t, "1", id1)

	id2 := k.NextSampleID(ctx)
	require.Equal(t, "2", id2)
}

// ─── Active round index ─────────────────────────────────────────────────────

func TestActiveRoundIndex(t *testing.T) {
	k, ctx := setupKeeper(t)

	// Initially empty
	rounds := k.GetActiveRounds(ctx)
	require.Empty(t, rounds)

	// Set two active rounds
	err := k.SetActiveRound(ctx, "r1")
	require.NoError(t, err)
	err = k.SetActiveRound(ctx, "r2")
	require.NoError(t, err)

	rounds = k.GetActiveRounds(ctx)
	require.Len(t, rounds, 2)
	require.Contains(t, rounds, "r1")
	require.Contains(t, rounds, "r2")

	// Delete one
	err = k.DeleteActiveRound(ctx, "r1")
	require.NoError(t, err)

	rounds = k.GetActiveRounds(ctx)
	require.Len(t, rounds, 1)
	require.Equal(t, "r2", rounds[0])
}

// ─── Submission → Round index ───────────────────────────────────────────────

func TestSubmissionRoundIndex(t *testing.T) {
	k, ctx := setupKeeper(t)

	// Nonexistent returns empty, false
	_, ok := k.GetRoundBySubmission(ctx, "nonexistent")
	require.False(t, ok)

	// Set and get
	err := k.SetSubmissionRoundIndex(ctx, "sub-1", "round-1")
	require.NoError(t, err)

	roundID, ok := k.GetRoundBySubmission(ctx, "sub-1")
	require.True(t, ok)
	require.Equal(t, "round-1", roundID)

	// Different submission still returns false
	_, ok = k.GetRoundBySubmission(ctx, "sub-2")
	require.False(t, ok)
}

// ─── Sample indexes ─────────────────────────────────────────────────────────

func TestSampleIndexes(t *testing.T) {
	k, ctx := setupKeeper(t)

	// Set indexes for a sample
	require.NoError(t, k.SetSampleDomainIndex(ctx, "science", "s1"))
	require.NoError(t, k.SetSampleDomainIndex(ctx, "science", "s2"))
	require.NoError(t, k.SetSampleDomainIndex(ctx, "math", "s3"))

	require.NoError(t, k.SetSampleSubmitterIndex(ctx, testAddr, "s1"))
	require.NoError(t, k.SetSampleSubmitterIndex(ctx, testAddr, "s2"))

	require.NoError(t, k.SetSampleThreadIndex(ctx, "thread-1", "s1"))
	require.NoError(t, k.SetSampleThreadIndex(ctx, "thread-1", "s2"))
	require.NoError(t, k.SetSampleThreadIndex(ctx, "thread-2", "s3"))

	// Verify domain index
	scienceIDs := k.GetSamplesByDomain(ctx, "science")
	require.Len(t, scienceIDs, 2)
	require.Contains(t, scienceIDs, "s1")
	require.Contains(t, scienceIDs, "s2")

	mathIDs := k.GetSamplesByDomain(ctx, "math")
	require.Len(t, mathIDs, 1)
	require.Equal(t, "s3", mathIDs[0])

	emptyIDs := k.GetSamplesByDomain(ctx, "history")
	require.Empty(t, emptyIDs)

	// Verify submitter index
	submitterIDs := k.GetSamplesBySubmitter(ctx, testAddr)
	require.Len(t, submitterIDs, 2)
	require.Contains(t, submitterIDs, "s1")
	require.Contains(t, submitterIDs, "s2")

	// Verify thread index
	thread1IDs := k.GetSamplesByThread(ctx, "thread-1")
	require.Len(t, thread1IDs, 2)
	require.Contains(t, thread1IDs, "s1")
	require.Contains(t, thread1IDs, "s2")

	thread2IDs := k.GetSamplesByThread(ctx, "thread-2")
	require.Len(t, thread2IDs, 1)
	require.Equal(t, "s3", thread2IDs[0])
}
