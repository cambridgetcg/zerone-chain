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
