package keeper_test

import (
	"context"
	"testing"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/stretchr/testify/require"

	"github.com/zerone-chain/zerone/x/knowledge/keeper"
	"github.com/zerone-chain/zerone/x/knowledge/types"
)

const (
	verifier1 = "zrn1verifier1qqqqqqqqqqqqqqqqqqpvxfez"
	verifier2 = "zrn1verifier2qqqqqqqqqqqqqqqqqqpt5ev5"
	verifier3 = "zrn1verifier3qqqqqqqqqqqqqqqqqqpkf4jc"
)

func TestInitiateQualityRound(t *testing.T) {
	k, ctx := setupKeeper(t)
	setupDefaultDomains(t, k, ctx)

	sub := &types.Submission{
		Id:     "s1",
		Domain: "technology",
		Status: types.SubmissionStatus_SUBMISSION_STATUS_PENDING,
	}
	require.NoError(t, k.SetSubmission(ctx, sub))

	verifiers := []string{verifier1, verifier2, verifier3}
	roundID, err := k.InitiateQualityRound(ctx, "s1", "", verifiers)
	require.NoError(t, err)
	require.NotEmpty(t, roundID)

	round, found := k.GetQualityRound(ctx, roundID)
	require.True(t, found)
	require.Equal(t, "s1", round.SubmissionId)
	require.Equal(t, types.VerificationPhase_VERIFICATION_PHASE_COMMIT, round.Phase)
	require.Equal(t, uint64(100), round.StartedAtBlock)
	require.Equal(t, uint64(104), round.CommitDeadline)
	require.Equal(t, uint64(108), round.RevealDeadline)
	require.Len(t, round.SelectedVerifiers, 3)

	updatedSub, found := k.GetSubmission(ctx, "s1")
	require.True(t, found)
	require.Equal(t, roundID, updatedSub.QualityRoundId)
	require.Equal(t, types.SubmissionStatus_SUBMISSION_STATUS_PENDING_REVIEW, updatedSub.Status)

	actives := k.GetActiveRounds(ctx)
	require.Contains(t, actives, roundID)

	gotRoundID, found := k.GetRoundBySubmission(ctx, "s1")
	require.True(t, found)
	require.Equal(t, roundID, gotRoundID)

	sdkCtx := sdk.UnwrapSDKContext(ctx)
	events := sdkCtx.EventManager().Events()
	eventFound := false
	for _, e := range events {
		if e.Type == "quality_round_started" {
			eventFound = true
		}
	}
	require.True(t, eventFound)
}

func TestInitiateQualityRound_SubmissionNotFound(t *testing.T) {
	k, ctx := setupKeeper(t)
	_, err := k.InitiateQualityRound(ctx, "nonexistent", "", []string{verifier1})
	require.ErrorIs(t, err, types.ErrSubmissionNotFound)
}

func TestInitiateQualityRound_Thread(t *testing.T) {
	k, ctx := setupKeeper(t)
	setupDefaultDomains(t, k, ctx)

	for i, id := range []string{"s1", "s2", "s3"} {
		sub := &types.Submission{
			Id:       id,
			Domain:   "technology",
			ThreadId: "thread-1",
			Status:   types.SubmissionStatus_SUBMISSION_STATUS_PENDING,
		}
		if i > 0 {
			sub.ParentSubmissionId = []string{"s1", "s2", "s3"}[i-1]
		}
		require.NoError(t, k.SetSubmission(ctx, sub))
	}

	verifiers := []string{verifier1, verifier2, verifier3}
	roundID, err := k.InitiateQualityRound(ctx, "s1", "thread-1", verifiers)
	require.NoError(t, err)

	for _, sid := range []string{"s1", "s2", "s3"} {
		gotRoundID, found := k.GetRoundBySubmission(ctx, sid)
		require.True(t, found)
		require.Equal(t, roundID, gotRoundID)
	}
}

// ─── SubmitCommitment tests ─────────────────────────────────────────────────

func setupRoundInCommitPhase(t *testing.T) (keeper.Keeper, context.Context, *mockBankKeeper, string) {
	t.Helper()
	k, ctx, bk := setupKeeperWithBank(t)
	setupDefaultDomains(t, k, ctx)

	sub := &types.Submission{
		Id:     "s1",
		Domain: "technology",
		Status: types.SubmissionStatus_SUBMISSION_STATUS_PENDING,
	}
	require.NoError(t, k.SetSubmission(ctx, sub))

	verifiers := []string{verifier1, verifier2, verifier3}
	roundID, err := k.InitiateQualityRound(ctx, "s1", "", verifiers)
	require.NoError(t, err)
	return k, ctx, bk, roundID
}

func TestSubmitCommitment_Success(t *testing.T) {
	k, ctx, _, roundID := setupRoundInCommitPhase(t)

	commitHash := types.ComputeQualityCommitHash(roundID, &types.QualityVote{
		OverallQuality: 800000,
	}, []byte("salt1"))

	err := k.SubmitCommitment(ctx, &types.MsgSubmitCommitment{
		Verifier:   verifier1,
		RoundId:    roundID,
		CommitHash: commitHash,
	})
	require.NoError(t, err)

	round, found := k.GetQualityRound(ctx, roundID)
	require.True(t, found)
	require.Len(t, round.Commits, 1)
	require.Equal(t, verifier1, round.Commits[0].Verifier)
	require.Equal(t, commitHash, round.Commits[0].CommitHash)
}

func TestSubmitCommitment_NotSelectedValidator(t *testing.T) {
	k, ctx, _, roundID := setupRoundInCommitPhase(t)

	err := k.SubmitCommitment(ctx, &types.MsgSubmitCommitment{
		Verifier:   testAddr,
		RoundId:    roundID,
		CommitHash: []byte("fake"),
	})
	require.ErrorIs(t, err, types.ErrNotSelectedValidator)
}

func TestSubmitCommitment_WrongPhase(t *testing.T) {
	k, ctx, _, roundID := setupRoundInCommitPhase(t)

	round, _ := k.GetQualityRound(ctx, roundID)
	round.Phase = types.VerificationPhase_VERIFICATION_PHASE_REVEAL
	require.NoError(t, k.SetQualityRound(ctx, round))

	err := k.SubmitCommitment(ctx, &types.MsgSubmitCommitment{
		Verifier:   verifier1,
		RoundId:    roundID,
		CommitHash: []byte("hash"),
	})
	require.ErrorIs(t, err, types.ErrWrongPhase)
}

func TestSubmitCommitment_DeadlinePassed(t *testing.T) {
	k, ctx, _, roundID := setupRoundInCommitPhase(t)

	sdkCtx := sdk.UnwrapSDKContext(ctx)
	ctx = sdkCtx.WithBlockHeight(200)

	err := k.SubmitCommitment(ctx, &types.MsgSubmitCommitment{
		Verifier:   verifier1,
		RoundId:    roundID,
		CommitHash: []byte("hash"),
	})
	require.ErrorIs(t, err, types.ErrDeadlinePassed)
}

func TestSubmitCommitment_DuplicateCommit(t *testing.T) {
	k, ctx, _, roundID := setupRoundInCommitPhase(t)

	msg := &types.MsgSubmitCommitment{
		Verifier:   verifier1,
		RoundId:    roundID,
		CommitHash: []byte("hash"),
	}
	require.NoError(t, k.SubmitCommitment(ctx, msg))
	err := k.SubmitCommitment(ctx, msg)
	require.ErrorIs(t, err, types.ErrAlreadyCommitted)
}

func TestSubmitCommitment_RoundNotFound(t *testing.T) {
	k, ctx := setupKeeper(t)

	err := k.SubmitCommitment(ctx, &types.MsgSubmitCommitment{
		Verifier:   verifier1,
		RoundId:    "nonexistent",
		CommitHash: []byte("hash"),
	})
	require.ErrorIs(t, err, types.ErrRoundNotFound)
}
