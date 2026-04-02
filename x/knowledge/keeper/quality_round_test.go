package keeper_test

import (
	"context"
	"fmt"
	"sort"
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

// ─── SubmitReveal tests ─────────────────────────────────────────────────────

func setupRoundInRevealPhase(t *testing.T) (keeper.Keeper, context.Context, *mockBankKeeper, string, []*types.QualityVote, [][]byte) {
	t.Helper()
	k, ctx, bk, roundID := setupRoundInCommitPhase(t)

	salt1, salt2, salt3 := []byte("salt1"), []byte("salt2"), []byte("salt3")
	votes := []*types.QualityVote{
		{OverallQuality: 800000, ReasoningDepth: 700000, Novelty: 600000, Toxicity: 10000, FactualAccuracy: 900000, ConsentValid: true},
		{OverallQuality: 750000, ReasoningDepth: 650000, Novelty: 550000, Toxicity: 20000, FactualAccuracy: 850000, ConsentValid: true},
		{OverallQuality: 850000, ReasoningDepth: 750000, Novelty: 650000, Toxicity: 5000, FactualAccuracy: 950000, ConsentValid: true},
	}
	salts := [][]byte{salt1, salt2, salt3}

	for i, v := range []string{verifier1, verifier2, verifier3} {
		hash := types.ComputeQualityCommitHash(roundID, votes[i], salts[i])
		require.NoError(t, k.SubmitCommitment(ctx, &types.MsgSubmitCommitment{
			Verifier: v, RoundId: roundID, CommitHash: hash,
		}))
	}

	round, _ := k.GetQualityRound(ctx, roundID)
	round.Phase = types.VerificationPhase_VERIFICATION_PHASE_REVEAL
	require.NoError(t, k.SetQualityRound(ctx, round))

	return k, ctx, bk, roundID, votes, salts
}

func TestSubmitReveal_Success(t *testing.T) {
	k, ctx, _, roundID, votes, salts := setupRoundInRevealPhase(t)

	err := k.SubmitReveal(ctx, &types.MsgSubmitReveal{
		Verifier: verifier1, RoundId: roundID, Scores: votes[0], Salt: salts[0],
	})
	require.NoError(t, err)

	round, found := k.GetQualityRound(ctx, roundID)
	require.True(t, found)
	require.Len(t, round.Reveals, 1)
	require.Equal(t, verifier1, round.Reveals[0].Verifier)
}

func TestSubmitReveal_HashMismatch(t *testing.T) {
	k, ctx, _, roundID, _, _ := setupRoundInRevealPhase(t)

	wrongVote := &types.QualityVote{OverallQuality: 999999}
	err := k.SubmitReveal(ctx, &types.MsgSubmitReveal{
		Verifier: verifier1, RoundId: roundID, Scores: wrongVote, Salt: []byte("salt1"),
	})
	require.ErrorIs(t, err, types.ErrRevealMismatch)
}

func TestSubmitReveal_WrongPhase(t *testing.T) {
	k, ctx, _, roundID := setupRoundInCommitPhase(t)
	// Round is still in commit phase

	err := k.SubmitReveal(ctx, &types.MsgSubmitReveal{
		Verifier: verifier1, RoundId: roundID,
		Scores: &types.QualityVote{OverallQuality: 800000}, Salt: []byte("salt1"),
	})
	require.ErrorIs(t, err, types.ErrWrongPhase)
}

func TestSubmitReveal_DeadlinePassed(t *testing.T) {
	k, ctx, _, roundID, votes, salts := setupRoundInRevealPhase(t)

	sdkCtx := sdk.UnwrapSDKContext(ctx)
	ctx = sdkCtx.WithBlockHeight(300)

	err := k.SubmitReveal(ctx, &types.MsgSubmitReveal{
		Verifier: verifier1, RoundId: roundID, Scores: votes[0], Salt: salts[0],
	})
	require.ErrorIs(t, err, types.ErrDeadlinePassed)
}

func TestSubmitReveal_NotSelectedValidator(t *testing.T) {
	k, ctx, _, roundID, _, _ := setupRoundInRevealPhase(t)

	err := k.SubmitReveal(ctx, &types.MsgSubmitReveal{
		Verifier: testAddr, RoundId: roundID,
		Scores: &types.QualityVote{OverallQuality: 800000}, Salt: []byte("salt1"),
	})
	require.ErrorIs(t, err, types.ErrNotSelectedValidator)
}

func TestSubmitReveal_NoCommitment(t *testing.T) {
	k, ctx, _, roundID, _, _ := setupRoundInRevealPhase(t)

	// Remove verifier1's commit
	round, _ := k.GetQualityRound(ctx, roundID)
	round.Commits = round.Commits[1:]
	require.NoError(t, k.SetQualityRound(ctx, round))

	err := k.SubmitReveal(ctx, &types.MsgSubmitReveal{
		Verifier: verifier1, RoundId: roundID,
		Scores: &types.QualityVote{OverallQuality: 800000}, Salt: []byte("salt1"),
	})
	require.ErrorIs(t, err, types.ErrNoCommitment)
}

func TestSubmitReveal_DuplicateReveal(t *testing.T) {
	k, ctx, _, roundID, votes, salts := setupRoundInRevealPhase(t)

	msg := &types.MsgSubmitReveal{
		Verifier: verifier1, RoundId: roundID, Scores: votes[0], Salt: salts[0],
	}
	require.NoError(t, k.SubmitReveal(ctx, msg))
	err := k.SubmitReveal(ctx, msg)
	require.ErrorIs(t, err, types.ErrAlreadyRevealed)
}

func TestSubmitReveal_ScoreOutOfRange(t *testing.T) {
	k, ctx, _, roundID, _, _ := setupRoundInRevealPhase(t)

	vote := &types.QualityVote{OverallQuality: 1_500_000}
	salt := []byte("salt-oob")

	// Update verifier1's commit to match this vote
	round, _ := k.GetQualityRound(ctx, roundID)
	hash := types.ComputeQualityCommitHash(roundID, vote, salt)
	for i, c := range round.Commits {
		if c.Verifier == verifier1 {
			round.Commits[i].CommitHash = hash
		}
	}
	require.NoError(t, k.SetQualityRound(ctx, round))

	err := k.SubmitReveal(ctx, &types.MsgSubmitReveal{
		Verifier: verifier1, RoundId: roundID, Scores: vote, Salt: salt,
	})
	require.ErrorIs(t, err, types.ErrInvalidQualityScore)
}

func TestSubmitReveal_RoundNotFound(t *testing.T) {
	k, ctx := setupKeeper(t)
	err := k.SubmitReveal(ctx, &types.MsgSubmitReveal{
		Verifier: verifier1, RoundId: "nonexistent",
		Scores: &types.QualityVote{OverallQuality: 800000}, Salt: []byte("s"),
	})
	require.ErrorIs(t, err, types.ErrRoundNotFound)
}

// ─── AggregateQualityRound tests ────────────────────────────────────────────

// runFullRound runs the full commit+reveal flow for a round.
func runFullRound(t *testing.T, k keeper.Keeper, ctx context.Context, roundID string, votes []*types.QualityVote) {
	t.Helper()
	salts := [][]byte{[]byte("s1"), []byte("s2"), []byte("s3")}
	verifiers := []string{verifier1, verifier2, verifier3}

	for i, v := range verifiers {
		hash := types.ComputeQualityCommitHash(roundID, votes[i], salts[i])
		require.NoError(t, k.SubmitCommitment(ctx, &types.MsgSubmitCommitment{
			Verifier: v, RoundId: roundID, CommitHash: hash,
		}))
	}

	round, _ := k.GetQualityRound(ctx, roundID)
	round.Phase = types.VerificationPhase_VERIFICATION_PHASE_REVEAL
	require.NoError(t, k.SetQualityRound(ctx, round))

	for i, v := range verifiers {
		require.NoError(t, k.SubmitReveal(ctx, &types.MsgSubmitReveal{
			Verifier: v, RoundId: roundID, Scores: votes[i], Salt: salts[i],
		}))
	}
}

// setupSubmissionWithRound creates a submission with stake and initiates a quality round.
func setupSubmissionWithRound(t *testing.T) (keeper.Keeper, context.Context, *mockBankKeeper, string) {
	t.Helper()
	k, ctx, bk := setupKeeperWithBank(t)
	setupDefaultDomains(t, k, ctx)

	sub := &types.Submission{
		Id: "s1", Domain: "technology", Submitter: testAddr,
		Content: "test", Stake: "1000000",
		Status:  types.SubmissionStatus_SUBMISSION_STATUS_PENDING,
		Consent: &types.ConsentProof{Type: types.ConsentType_CONSENT_TYPE_SELF_AUTHORED},
	}
	require.NoError(t, k.SetSubmission(ctx, sub))

	verifiers := []string{verifier1, verifier2, verifier3}
	roundID, err := k.InitiateQualityRound(ctx, "s1", "", verifiers)
	require.NoError(t, err)
	return k, ctx, bk, roundID
}

func TestAggregateQualityRound_GoldVerdict(t *testing.T) {
	k, ctx, bk, roundID := setupSubmissionWithRound(t)

	votes := []*types.QualityVote{
		{OverallQuality: 850000, ReasoningDepth: 800000, Novelty: 750000, Toxicity: 10000, FactualAccuracy: 900000, ConsentValid: true},
		{OverallQuality: 820000, ReasoningDepth: 780000, Novelty: 700000, Toxicity: 15000, FactualAccuracy: 880000, ConsentValid: true},
		{OverallQuality: 900000, ReasoningDepth: 850000, Novelty: 800000, Toxicity: 5000, FactualAccuracy: 950000, ConsentValid: true},
	}
	runFullRound(t, k, ctx, roundID, votes)

	err := k.AggregateQualityRound(ctx, roundID)
	require.NoError(t, err)

	round, found := k.GetQualityRound(ctx, roundID)
	require.True(t, found)
	require.Equal(t, types.QualityVerdict_QUALITY_VERDICT_GOLD, round.Verdict)
	require.Equal(t, types.VerificationPhase_VERIFICATION_PHASE_COMPLETE, round.Phase)
	require.Equal(t, uint64(100), round.VerdictBlock)
	require.NotNil(t, round.AggregateScores)
	// Median of [820000, 850000, 900000] = 850000
	require.Equal(t, uint64(850000), round.AggregateScores.OverallQuality)

	// Active round removed
	actives := k.GetActiveRounds(ctx)
	require.NotContains(t, actives, roundID)

	// Submission accepted
	sub, found := k.GetSubmission(ctx, "s1")
	require.True(t, found)
	require.Equal(t, types.SubmissionStatus_SUBMISSION_STATUS_ACCEPTED, sub.Status)

	// Stake returned
	require.Len(t, bk.moduleToAccountCalls, 1)
	require.Equal(t, "knowledge", bk.moduleToAccountCalls[0].from)
	require.Equal(t, testAddr, bk.moduleToAccountCalls[0].to)
	require.Equal(t, sdk.NewInt64Coin("uzrn", 1000000), bk.moduleToAccountCalls[0].amount[0])

	// Event emitted
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	events := sdkCtx.EventManager().Events()
	eventFound := false
	for _, e := range events {
		if e.Type == "quality_round_completed" {
			eventFound = true
		}
	}
	require.True(t, eventFound)
}

func TestAggregateQualityRound_SilverVerdict(t *testing.T) {
	k, ctx, _, roundID := setupSubmissionWithRound(t)

	votes := []*types.QualityVote{
		{OverallQuality: 720000, ReasoningDepth: 700000, Novelty: 650000, Toxicity: 10000, FactualAccuracy: 750000, ConsentValid: true},
		{OverallQuality: 680000, ReasoningDepth: 650000, Novelty: 600000, Toxicity: 15000, FactualAccuracy: 700000, ConsentValid: true},
		{OverallQuality: 650000, ReasoningDepth: 620000, Novelty: 580000, Toxicity: 20000, FactualAccuracy: 680000, ConsentValid: true},
	}
	runFullRound(t, k, ctx, roundID, votes)

	err := k.AggregateQualityRound(ctx, roundID)
	require.NoError(t, err)

	round, found := k.GetQualityRound(ctx, roundID)
	require.True(t, found)
	require.Equal(t, types.QualityVerdict_QUALITY_VERDICT_SILVER, round.Verdict)
	// Median of [650000, 680000, 720000] = 680000
	require.Equal(t, uint64(680000), round.AggregateScores.OverallQuality)

	sub, found := k.GetSubmission(ctx, "s1")
	require.True(t, found)
	require.Equal(t, types.SubmissionStatus_SUBMISSION_STATUS_ACCEPTED, sub.Status)
}

func TestAggregateQualityRound_BronzeVerdict(t *testing.T) {
	k, ctx, _, roundID := setupSubmissionWithRound(t)

	votes := []*types.QualityVote{
		{OverallQuality: 500000, ReasoningDepth: 480000, Novelty: 400000, Toxicity: 10000, FactualAccuracy: 520000, ConsentValid: true},
		{OverallQuality: 480000, ReasoningDepth: 450000, Novelty: 380000, Toxicity: 15000, FactualAccuracy: 490000, ConsentValid: true},
		{OverallQuality: 450000, ReasoningDepth: 420000, Novelty: 350000, Toxicity: 20000, FactualAccuracy: 460000, ConsentValid: true},
	}
	runFullRound(t, k, ctx, roundID, votes)

	err := k.AggregateQualityRound(ctx, roundID)
	require.NoError(t, err)

	round, found := k.GetQualityRound(ctx, roundID)
	require.True(t, found)
	require.Equal(t, types.QualityVerdict_QUALITY_VERDICT_BRONZE, round.Verdict)
	// Median of [450000, 480000, 500000] = 480000
	require.Equal(t, uint64(480000), round.AggregateScores.OverallQuality)

	sub, found := k.GetSubmission(ctx, "s1")
	require.True(t, found)
	require.Equal(t, types.SubmissionStatus_SUBMISSION_STATUS_ACCEPTED, sub.Status)
}

func TestAggregateQualityRound_RejectVerdict(t *testing.T) {
	k, ctx, _, roundID := setupSubmissionWithRound(t)

	votes := []*types.QualityVote{
		{OverallQuality: 300000, ReasoningDepth: 280000, Novelty: 200000, Toxicity: 10000, FactualAccuracy: 320000, ConsentValid: true},
		{OverallQuality: 250000, ReasoningDepth: 230000, Novelty: 180000, Toxicity: 15000, FactualAccuracy: 260000, ConsentValid: true},
		{OverallQuality: 200000, ReasoningDepth: 180000, Novelty: 150000, Toxicity: 20000, FactualAccuracy: 210000, ConsentValid: true},
	}
	runFullRound(t, k, ctx, roundID, votes)

	err := k.AggregateQualityRound(ctx, roundID)
	require.NoError(t, err)

	round, found := k.GetQualityRound(ctx, roundID)
	require.True(t, found)
	require.Equal(t, types.QualityVerdict_QUALITY_VERDICT_REJECT, round.Verdict)
	// Median of [200000, 250000, 300000] = 250000
	require.Equal(t, uint64(250000), round.AggregateScores.OverallQuality)

	sub, found := k.GetSubmission(ctx, "s1")
	require.True(t, found)
	require.Equal(t, types.SubmissionStatus_SUBMISSION_STATUS_REJECTED, sub.Status)
}

func TestAggregateQualityRound_ConsentFail(t *testing.T) {
	k, ctx, _, roundID := setupSubmissionWithRound(t)

	// High quality but majority say consent_valid=false
	votes := []*types.QualityVote{
		{OverallQuality: 900000, ReasoningDepth: 850000, Novelty: 800000, Toxicity: 5000, FactualAccuracy: 950000, ConsentValid: false},
		{OverallQuality: 880000, ReasoningDepth: 830000, Novelty: 780000, Toxicity: 8000, FactualAccuracy: 920000, ConsentValid: false},
		{OverallQuality: 920000, ReasoningDepth: 870000, Novelty: 820000, Toxicity: 3000, FactualAccuracy: 960000, ConsentValid: true},
	}
	runFullRound(t, k, ctx, roundID, votes)

	err := k.AggregateQualityRound(ctx, roundID)
	require.NoError(t, err)

	round, found := k.GetQualityRound(ctx, roundID)
	require.True(t, found)
	require.Equal(t, types.QualityVerdict_QUALITY_VERDICT_CONSENT_FAIL, round.Verdict)

	sub, found := k.GetSubmission(ctx, "s1")
	require.True(t, found)
	require.Equal(t, types.SubmissionStatus_SUBMISSION_STATUS_CONSENT_FAILED, sub.Status)
}

func TestAggregateQualityRound_DuplicateOverridesQuality(t *testing.T) {
	k, ctx, _, roundID := setupSubmissionWithRound(t)

	// High quality but majority mark as duplicate
	votes := []*types.QualityVote{
		{OverallQuality: 900000, ReasoningDepth: 850000, Novelty: 800000, Toxicity: 5000, FactualAccuracy: 950000, ConsentValid: true, Duplicate: true},
		{OverallQuality: 880000, ReasoningDepth: 830000, Novelty: 780000, Toxicity: 8000, FactualAccuracy: 920000, ConsentValid: true, Duplicate: true},
		{OverallQuality: 920000, ReasoningDepth: 870000, Novelty: 820000, Toxicity: 3000, FactualAccuracy: 960000, ConsentValid: true, Duplicate: false},
	}
	runFullRound(t, k, ctx, roundID, votes)

	err := k.AggregateQualityRound(ctx, roundID)
	require.NoError(t, err)

	round, found := k.GetQualityRound(ctx, roundID)
	require.True(t, found)
	require.Equal(t, types.QualityVerdict_QUALITY_VERDICT_REJECT, round.Verdict)

	sub, found := k.GetSubmission(ctx, "s1")
	require.True(t, found)
	require.Equal(t, types.SubmissionStatus_SUBMISSION_STATUS_REJECTED, sub.Status)
}

func TestAggregateQualityRound_ToxicityThreshold(t *testing.T) {
	k, ctx, _, roundID := setupSubmissionWithRound(t)

	// High quality but toxicity above threshold (200000)
	votes := []*types.QualityVote{
		{OverallQuality: 900000, ReasoningDepth: 850000, Novelty: 800000, Toxicity: 250000, FactualAccuracy: 950000, ConsentValid: true},
		{OverallQuality: 880000, ReasoningDepth: 830000, Novelty: 780000, Toxicity: 300000, FactualAccuracy: 920000, ConsentValid: true},
		{OverallQuality: 920000, ReasoningDepth: 870000, Novelty: 820000, Toxicity: 220000, FactualAccuracy: 960000, ConsentValid: true},
	}
	runFullRound(t, k, ctx, roundID, votes)

	err := k.AggregateQualityRound(ctx, roundID)
	require.NoError(t, err)

	round, found := k.GetQualityRound(ctx, roundID)
	require.True(t, found)
	require.Equal(t, types.QualityVerdict_QUALITY_VERDICT_REJECT, round.Verdict)
	// Median toxicity of [220000, 250000, 300000] = 250000, which is > 200000
	require.Equal(t, uint64(250000), round.AggregateScores.Toxicity)

	sub, found := k.GetSubmission(ctx, "s1")
	require.True(t, found)
	require.Equal(t, types.SubmissionStatus_SUBMISSION_STATUS_REJECTED, sub.Status)
}

// ─── Validator Scoring tests ─────────────────────────────────────────────────

func TestValidatorScoring_OutlierSlashed(t *testing.T) {
	k, ctx, bk, roundID := setupSubmissionWithRound(t)

	// verifier3 is extreme outlier (100k vs ~845k median)
	votes := []*types.QualityVote{
		{OverallQuality: 850000, ConsentValid: true},
		{OverallQuality: 840000, ConsentValid: true},
		{OverallQuality: 100000, ConsentValid: true},
	}
	runFullRound(t, k, ctx, roundID, votes)
	require.NoError(t, k.AggregateQualityRound(ctx, roundID))

	sdkCtx := sdk.UnwrapSDKContext(ctx)
	events := sdkCtx.EventManager().Events()
	slashFound := false
	for _, e := range events {
		if e.Type == "validator_slashed" {
			for _, attr := range e.Attributes {
				if attr.Key == "verifier" && attr.Value == verifier3 {
					slashFound = true
				}
			}
		}
	}
	require.True(t, slashFound, "expected verifier3 to be slashed as outlier")
	_ = bk
}

func TestValidatorScoring_MissedRevealSlashed(t *testing.T) {
	k, ctx, _ := setupKeeperWithBank(t)
	setupDefaultDomains(t, k, ctx)

	sub := &types.Submission{
		Id: "s1", Domain: "technology", Submitter: testAddr,
		Content: "test", Stake: "1000000",
		Status:  types.SubmissionStatus_SUBMISSION_STATUS_PENDING,
		Consent: &types.ConsentProof{Type: types.ConsentType_CONSENT_TYPE_SELF_AUTHORED},
	}
	require.NoError(t, k.SetSubmission(ctx, sub))

	verifiers := []string{verifier1, verifier2, verifier3}
	roundID, _ := k.InitiateQualityRound(ctx, "s1", "", verifiers)

	// All 3 commit, only 2 reveal
	votes := []*types.QualityVote{
		{OverallQuality: 850000, ConsentValid: true},
		{OverallQuality: 840000, ConsentValid: true},
	}
	salts := [][]byte{[]byte("s1"), []byte("s2")}

	for i, v := range []string{verifier1, verifier2} {
		hash := types.ComputeQualityCommitHash(roundID, votes[i], salts[i])
		require.NoError(t, k.SubmitCommitment(ctx, &types.MsgSubmitCommitment{
			Verifier: v, RoundId: roundID, CommitHash: hash,
		}))
	}
	// verifier3 commits but won't reveal
	hash3 := types.ComputeQualityCommitHash(roundID, &types.QualityVote{OverallQuality: 800000}, []byte("s3"))
	require.NoError(t, k.SubmitCommitment(ctx, &types.MsgSubmitCommitment{
		Verifier: verifier3, RoundId: roundID, CommitHash: hash3,
	}))

	round, _ := k.GetQualityRound(ctx, roundID)
	round.Phase = types.VerificationPhase_VERIFICATION_PHASE_REVEAL
	require.NoError(t, k.SetQualityRound(ctx, round))

	for i, v := range []string{verifier1, verifier2} {
		require.NoError(t, k.SubmitReveal(ctx, &types.MsgSubmitReveal{
			Verifier: v, RoundId: roundID, Scores: votes[i], Salt: salts[i],
		}))
	}

	require.NoError(t, k.AggregateQualityRound(ctx, roundID))

	sdkCtx := sdk.UnwrapSDKContext(ctx)
	events := sdkCtx.EventManager().Events()
	missedRevealFound := false
	for _, e := range events {
		if e.Type == "validator_missed_reveal" {
			for _, attr := range e.Attributes {
				if attr.Key == "verifier" && attr.Value == verifier3 {
					missedRevealFound = true
				}
			}
		}
	}
	require.True(t, missedRevealFound, "expected verifier3 missed-reveal event")
}

func TestValidatorScoring_ConsensusRewarded(t *testing.T) {
	k, ctx, _, roundID := setupSubmissionWithRound(t)

	// All close together → all rewarded
	votes := []*types.QualityVote{
		{OverallQuality: 850000, ConsentValid: true},
		{OverallQuality: 845000, ConsentValid: true},
		{OverallQuality: 855000, ConsentValid: true},
	}
	runFullRound(t, k, ctx, roundID, votes)
	require.NoError(t, k.AggregateQualityRound(ctx, roundID))

	sdkCtx := sdk.UnwrapSDKContext(ctx)
	events := sdkCtx.EventManager().Events()
	rewardCount := 0
	for _, e := range events {
		if e.Type == "validator_rewarded" {
			rewardCount++
		}
	}
	require.Equal(t, 3, rewardCount, "all 3 validators should be rewarded")
}

// ─── Sample creation tests ──────────────────────────────────────────────────

func TestSampleCreation_FieldMapping(t *testing.T) {
	k, ctx, _ := setupKeeperWithBank(t)
	setupDefaultDomains(t, k, ctx)

	sub := &types.Submission{
		Id: "s1", Domain: "technology", Submitter: testAddr,
		Content:         "detailed test content",
		SampleType:      types.SampleType_SAMPLE_TYPE_EXPLANATION,
		SourceUri:       "https://example.com",
		SourcePlatform:  "web",
		SourceTimestamp:  1234567890,
		OriginalAuthor:  "author1",
		License:         "MIT",
		Tags:            []string{"go", "testing"},
		Language:        "en",
		ThreadId:        "",
		Stake:           "1000000",
		Status:          types.SubmissionStatus_SUBMISSION_STATUS_PENDING,
		Consent:         &types.ConsentProof{Type: types.ConsentType_CONSENT_TYPE_SELF_AUTHORED},
		ContentHash:     "abc123",
	}
	require.NoError(t, k.SetSubmission(ctx, sub))

	verifiers := []string{verifier1, verifier2, verifier3}
	roundID, _ := k.InitiateQualityRound(ctx, "s1", "", verifiers)

	votes := []*types.QualityVote{
		{OverallQuality: 850000, ReasoningDepth: 700000, Novelty: 600000, Toxicity: 5000, FactualAccuracy: 900000, ConsentValid: true},
		{OverallQuality: 850000, ReasoningDepth: 700000, Novelty: 600000, Toxicity: 5000, FactualAccuracy: 900000, ConsentValid: true},
		{OverallQuality: 850000, ReasoningDepth: 700000, Novelty: 600000, Toxicity: 5000, FactualAccuracy: 900000, ConsentValid: true},
	}
	runFullRound(t, k, ctx, roundID, votes)
	require.NoError(t, k.AggregateQualityRound(ctx, roundID))

	sampleIDs := k.GetSamplesByDomain(ctx, "technology")
	require.Len(t, sampleIDs, 1)

	sample, found := k.GetSample(ctx, sampleIDs[0])
	require.True(t, found)
	require.Equal(t, "detailed test content", sample.Content)
	require.Equal(t, types.SampleType_SAMPLE_TYPE_EXPLANATION, sample.SampleType)
	require.Equal(t, "technology", sample.Domain)
	require.Equal(t, "https://example.com", sample.SourceUri)
	require.Equal(t, testAddr, sample.Submitter)
	require.Equal(t, "author1", sample.OriginalAuthor)
	require.Equal(t, "MIT", sample.License)
	require.Equal(t, "en", sample.Language)
	require.Equal(t, "s1", sample.SubmissionId)
	require.Equal(t, "gold", sample.QualityTier)
	require.Equal(t, uint64(850000), sample.QualityScore)
	require.Equal(t, uint64(700000), sample.ReasoningDepth)
	require.Equal(t, uint64(600000), sample.NoveltyScore)
	require.Equal(t, types.SampleStatus_SAMPLE_STATUS_GOLD, sample.Status)
	require.Equal(t, uint64(100), sample.VerifiedAtBlock)
	require.NotNil(t, sample.Consent)
}

func TestSampleCreation_ThreadSamples(t *testing.T) {
	k, ctx, _ := setupKeeperWithBank(t)
	setupDefaultDomains(t, k, ctx)

	for i, id := range []string{"s1", "s2", "s3"} {
		sub := &types.Submission{
			Id: id, Domain: "technology", Submitter: testAddr,
			Content:  "content " + id,
			ThreadId: "thread-1",
			Stake:    "1000000",
			Status:   types.SubmissionStatus_SUBMISSION_STATUS_PENDING,
			Consent:  &types.ConsentProof{Type: types.ConsentType_CONSENT_TYPE_SELF_AUTHORED},
		}
		if i > 0 {
			sub.ParentSubmissionId = []string{"s1", "s2"}[i-1]
		}
		require.NoError(t, k.SetSubmission(ctx, sub))
	}

	verifiers := []string{verifier1, verifier2, verifier3}
	roundID, _ := k.InitiateQualityRound(ctx, "s1", "thread-1", verifiers)

	votes := []*types.QualityVote{
		{OverallQuality: 850000, ConsentValid: true},
		{OverallQuality: 850000, ConsentValid: true},
		{OverallQuality: 850000, ConsentValid: true},
	}
	runFullRound(t, k, ctx, roundID, votes)
	require.NoError(t, k.AggregateQualityRound(ctx, roundID))

	threadSamples := k.GetSamplesByThread(ctx, "thread-1")
	require.Len(t, threadSamples, 3)

	// Verify parent-child linking
	samples := make([]*types.Sample, 3)
	for i, id := range threadSamples {
		s, found := k.GetSample(ctx, id)
		require.True(t, found)
		samples[i] = s
	}
	sort.Slice(samples, func(i, j int) bool {
		return samples[i].SubmissionId < samples[j].SubmissionId
	})

	require.Equal(t, "", samples[0].ParentSampleId)
	require.Equal(t, samples[0].Id, samples[1].ParentSampleId)
	require.Equal(t, samples[1].Id, samples[2].ParentSampleId)
}

// ─── Integration & Edge-Case tests ─────────────────────────────────────────

func TestEndToEnd_SubmitToSample(t *testing.T) {
	k, ctx, _ := setupKeeperWithBank(t)
	setupDefaultDomains(t, k, ctx)

	// Submit data
	resp, err := k.SubmitData(ctx, &types.MsgSubmitData{
		Submitter:  testAddr,
		Content:    "end to end test content",
		SampleType: types.SampleType_SAMPLE_TYPE_EXPLANATION,
		Domain:     "technology",
		Consent:    &types.ConsentProof{Type: types.ConsentType_CONSENT_TYPE_SELF_AUTHORED},
		Stake:      "1000000",
	})
	require.NoError(t, err)

	roundID, found := k.GetRoundBySubmission(ctx, resp.SubmissionId)
	require.True(t, found)

	// Add verifiers (simulating VRF selection)
	round, found := k.GetQualityRound(ctx, roundID)
	require.True(t, found)
	round.SelectedVerifiers = []string{verifier1, verifier2, verifier3}
	require.NoError(t, k.SetQualityRound(ctx, round))

	// Full commit+reveal+aggregate cycle
	votes := []*types.QualityVote{
		{OverallQuality: 900000, ReasoningDepth: 800000, Novelty: 750000, Toxicity: 1000, FactualAccuracy: 950000, ConsentValid: true},
		{OverallQuality: 880000, ReasoningDepth: 780000, Novelty: 730000, Toxicity: 2000, FactualAccuracy: 930000, ConsentValid: true},
		{OverallQuality: 910000, ReasoningDepth: 810000, Novelty: 760000, Toxicity: 500, FactualAccuracy: 960000, ConsentValid: true},
	}
	runFullRound(t, k, ctx, roundID, votes)
	require.NoError(t, k.AggregateQualityRound(ctx, roundID))

	// Verify sample exists with correct data
	samples := k.GetSamplesByDomain(ctx, "technology")
	require.Len(t, samples, 1)

	sample, found := k.GetSample(ctx, samples[0])
	require.True(t, found)
	require.Equal(t, "gold", sample.QualityTier)
	require.Equal(t, "end to end test content", sample.Content)
	require.Equal(t, types.SampleStatus_SAMPLE_STATUS_GOLD, sample.Status)

	// Submission should be ACCEPTED
	sub, found := k.GetSubmission(ctx, resp.SubmissionId)
	require.True(t, found)
	require.Equal(t, types.SubmissionStatus_SUBMISSION_STATUS_ACCEPTED, sub.Status)

	// Round should be COMPLETE and removed from active
	round, found = k.GetQualityRound(ctx, roundID)
	require.True(t, found)
	require.Equal(t, types.VerificationPhase_VERIFICATION_PHASE_COMPLETE, round.Phase)
	actives := k.GetActiveRounds(ctx)
	require.NotContains(t, actives, roundID)
}

func TestAggregateQualityRound_NoReveals_Noop(t *testing.T) {
	k, ctx, _ := setupKeeperWithBank(t)
	setupDefaultDomains(t, k, ctx)

	sub := &types.Submission{
		Id: "s1", Domain: "technology", Status: types.SubmissionStatus_SUBMISSION_STATUS_PENDING,
	}
	require.NoError(t, k.SetSubmission(ctx, sub))

	verifiers := []string{verifier1, verifier2, verifier3}
	roundID, err := k.InitiateQualityRound(ctx, "s1", "", verifiers)
	require.NoError(t, err)

	round, found := k.GetQualityRound(ctx, roundID)
	require.True(t, found)
	round.Phase = types.VerificationPhase_VERIFICATION_PHASE_REVEAL
	require.NoError(t, k.SetQualityRound(ctx, round))

	// No reveals submitted
	require.NoError(t, k.AggregateQualityRound(ctx, roundID))

	// Round should remain without verdict (early return, no changes)
	round, found = k.GetQualityRound(ctx, roundID)
	require.True(t, found)
	require.Equal(t, types.QualityVerdict_QUALITY_VERDICT_UNSPECIFIED, round.Verdict)
}

func TestAggregateQualityRound_RoundNotFound(t *testing.T) {
	k, ctx := setupKeeper(t)
	err := k.AggregateQualityRound(ctx, "nonexistent")
	require.ErrorIs(t, err, types.ErrRoundNotFound)
}

func TestMultipleRoundsActive(t *testing.T) {
	k, ctx := setupKeeper(t)
	setupDefaultDomains(t, k, ctx)

	for _, id := range []string{"s1", "s2"} {
		sub := &types.Submission{Id: id, Domain: "technology", Status: types.SubmissionStatus_SUBMISSION_STATUS_PENDING}
		require.NoError(t, k.SetSubmission(ctx, sub))
	}

	verifiers := []string{verifier1, verifier2, verifier3}
	r1, err := k.InitiateQualityRound(ctx, "s1", "", verifiers)
	require.NoError(t, err)
	r2, err := k.InitiateQualityRound(ctx, "s2", "", verifiers)
	require.NoError(t, err)

	actives := k.GetActiveRounds(ctx)
	require.Contains(t, actives, r1)
	require.Contains(t, actives, r2)
	require.Len(t, actives, 2)
}

func TestCommitAllVerifiers_RevealPartial(t *testing.T) {
	k, ctx, _ := setupKeeperWithBank(t)
	setupDefaultDomains(t, k, ctx)

	sub := &types.Submission{
		Id: "s1", Domain: "technology", Submitter: testAddr,
		Content: "test", Stake: "1000000",
		Status:  types.SubmissionStatus_SUBMISSION_STATUS_PENDING,
		Consent: &types.ConsentProof{Type: types.ConsentType_CONSENT_TYPE_SELF_AUTHORED},
	}
	require.NoError(t, k.SetSubmission(ctx, sub))

	verifiers := []string{verifier1, verifier2, verifier3}
	roundID, err := k.InitiateQualityRound(ctx, "s1", "", verifiers)
	require.NoError(t, err)

	votes := []*types.QualityVote{
		{OverallQuality: 850000, ConsentValid: true},
		{OverallQuality: 840000, ConsentValid: true},
		{OverallQuality: 830000, ConsentValid: true},
	}
	salts := [][]byte{[]byte("s1"), []byte("s2"), []byte("s3")}

	for i, v := range verifiers {
		hash := types.ComputeQualityCommitHash(roundID, votes[i], salts[i])
		require.NoError(t, k.SubmitCommitment(ctx, &types.MsgSubmitCommitment{
			Verifier: v, RoundId: roundID, CommitHash: hash,
		}))
	}
	round, found := k.GetQualityRound(ctx, roundID)
	require.True(t, found)
	round.Phase = types.VerificationPhase_VERIFICATION_PHASE_REVEAL
	require.NoError(t, k.SetQualityRound(ctx, round))

	// Only 2 of 3 reveal
	for i, v := range []string{verifier1, verifier2} {
		require.NoError(t, k.SubmitReveal(ctx, &types.MsgSubmitReveal{
			Verifier: v, RoundId: roundID, Scores: votes[i], Salt: salts[i],
		}))
	}

	require.NoError(t, k.AggregateQualityRound(ctx, roundID))

	round, found = k.GetQualityRound(ctx, roundID)
	require.True(t, found)
	require.Equal(t, types.QualityVerdict_QUALITY_VERDICT_GOLD, round.Verdict)
}

func TestRejectVerdict_NoSampleCreated(t *testing.T) {
	k, ctx, _ := setupKeeperWithBank(t)
	setupDefaultDomains(t, k, ctx)

	sub := &types.Submission{
		Id: "s1", Domain: "technology", Submitter: testAddr,
		Content: "low quality", Stake: "1000000",
		Status:  types.SubmissionStatus_SUBMISSION_STATUS_PENDING,
		Consent: &types.ConsentProof{Type: types.ConsentType_CONSENT_TYPE_SELF_AUTHORED},
	}
	require.NoError(t, k.SetSubmission(ctx, sub))

	verifiers := []string{verifier1, verifier2, verifier3}
	roundID, err := k.InitiateQualityRound(ctx, "s1", "", verifiers)
	require.NoError(t, err)

	// Very low quality -> reject
	votes := []*types.QualityVote{
		{OverallQuality: 100000, ConsentValid: true},
		{OverallQuality: 150000, ConsentValid: true},
		{OverallQuality: 120000, ConsentValid: true},
	}
	runFullRound(t, k, ctx, roundID, votes)
	require.NoError(t, k.AggregateQualityRound(ctx, roundID))

	// No samples should exist
	samples := k.GetSamplesByDomain(ctx, "technology")
	require.Len(t, samples, 0)

	// Submission should be REJECTED
	sub, found := k.GetSubmission(ctx, "s1")
	require.True(t, found)
	require.Equal(t, types.SubmissionStatus_SUBMISSION_STATUS_REJECTED, sub.Status)
}

// ─── EDIMANCE: Concurrent Round Isolation Tests ──────────────────────────────

// runFullRoundForVerifiers is like runFullRound but accepts arbitrary verifier lists
// and matching salts. Enables concurrent-round tests with distinct verifier sets.
func runFullRoundForVerifiers(
	t *testing.T,
	k keeper.Keeper,
	ctx context.Context,
	roundID string,
	verifiers []string,
	votes []*types.QualityVote,
) {
	t.Helper()
	require.Equal(t, len(verifiers), len(votes), "verifier and vote count must match")

	salts := make([][]byte, len(verifiers))
	for i := range verifiers {
		salts[i] = []byte(fmt.Sprintf("salt_%s_%02d", roundID, i))
	}

	for i, v := range verifiers {
		hash := types.ComputeQualityCommitHash(roundID, votes[i], salts[i])
		require.NoError(t, k.SubmitCommitment(ctx, &types.MsgSubmitCommitment{
			Verifier: v, RoundId: roundID, CommitHash: hash,
		}), "commit failed for verifier %d in round %s", i, roundID)
	}

	round, found := k.GetQualityRound(ctx, roundID)
	require.True(t, found)
	round.Phase = types.VerificationPhase_VERIFICATION_PHASE_REVEAL
	require.NoError(t, k.SetQualityRound(ctx, round))

	for i, v := range verifiers {
		require.NoError(t, k.SubmitReveal(ctx, &types.MsgSubmitReveal{
			Verifier: v, RoundId: roundID, Scores: votes[i], Salt: salts[i],
		}), "reveal failed for verifier %d in round %s", i, roundID)
	}
}

// TestConcurrentRounds_Independence verifies that multiple quality rounds running
// simultaneously with the same verifier set produce fully independent results.
// State from one round must not bleed into another.
func TestConcurrentRounds_Independence(t *testing.T) {
	k, ctx := setupKeeper(t)
	setupDefaultDomains(t, k, ctx)

	const numRounds = 3
	verifiers := []string{verifier1, verifier2, verifier3}

	// Create submissions and initiate all 3 rounds before running any commits.
	roundIDs := make([]string, numRounds)
	for i := 0; i < numRounds; i++ {
		subID := fmt.Sprintf("concurrent-sub-%d", i)
		sub := &types.Submission{
			Id:          subID,
			Domain:      "technology",
			Submitter:   testAddr,
			Content:     fmt.Sprintf("concurrent round %d content", i),
			Stake:       "0",
			Status:      types.SubmissionStatus_SUBMISSION_STATUS_PENDING,
			Consent:     &types.ConsentProof{Type: types.ConsentType_CONSENT_TYPE_SELF_AUTHORED},
			ContentHash: fmt.Sprintf("concurrent_hash_%03d", i),
		}
		require.NoError(t, k.SetSubmission(ctx, sub))

		rID, err := k.InitiateQualityRound(ctx, subID, "", verifiers)
		require.NoError(t, err)
		roundIDs[i] = rID
	}

	// All 3 rounds must have distinct IDs.
	seen := make(map[string]bool)
	for _, rID := range roundIDs {
		require.False(t, seen[rID], "duplicate round ID: %s", rID)
		seen[rID] = true
	}

	// All 3 rounds are in the active index.
	actives := k.GetActiveRounds(ctx)
	for _, rID := range roundIDs {
		require.Contains(t, actives, rID, "round %s must be in active index", rID)
	}

	// Run commit-reveal for each round independently.
	// Round 0: all accept (high quality → GOLD/SILVER/BRONZE verdict).
	// Round 1: all accept (medium quality → BRONZE verdict).
	// Round 2: all reject (low quality → REJECT verdict).
	voteSets := [][]*types.QualityVote{
		{
			{OverallQuality: 900_000, ConsentValid: true},
			{OverallQuality: 920_000, ConsentValid: true},
			{OverallQuality: 880_000, ConsentValid: true},
		},
		{
			{OverallQuality: 450_000, ConsentValid: true},
			{OverallQuality: 430_000, ConsentValid: true},
			{OverallQuality: 470_000, ConsentValid: true},
		},
		{
			{OverallQuality: 100_000, ConsentValid: true},
			{OverallQuality: 120_000, ConsentValid: true},
			{OverallQuality: 80_000, ConsentValid: true},
		},
	}
	for i, rID := range roundIDs {
		runFullRound(t, k, ctx, rID, voteSets[i])
	}

	// Aggregate all rounds.
	for _, rID := range roundIDs {
		require.NoError(t, k.AggregateQualityRound(ctx, rID))
	}

	// ── Assert: each round has independent state ───────────────────────────────
	for i, rID := range roundIDs {
		round, found := k.GetQualityRound(ctx, rID)
		require.True(t, found, "round %d must exist after aggregation", i)
		require.Equal(t, types.VerificationPhase_VERIFICATION_PHASE_COMPLETE, round.Phase,
			"round %d must be COMPLETE", i)
		require.Equal(t, "concurrent-sub-"+fmt.Sprintf("%d", i), round.SubmissionId,
			"round %d must reference its own submission", i)
		require.Len(t, round.Reveals, 3, "round %d must have exactly 3 reveals", i)
		require.NotNil(t, round.AggregateScores, "round %d must have aggregate scores", i)
	}

	// ── Assert: verdicts differ as expected by vote quality ────────────────────
	r0, _ := k.GetQualityRound(ctx, roundIDs[0])
	r1, _ := k.GetQualityRound(ctx, roundIDs[1])
	r2, _ := k.GetQualityRound(ctx, roundIDs[2])

	r0Accept := r0.Verdict == types.QualityVerdict_QUALITY_VERDICT_GOLD ||
		r0.Verdict == types.QualityVerdict_QUALITY_VERDICT_SILVER ||
		r0.Verdict == types.QualityVerdict_QUALITY_VERDICT_BRONZE
	require.True(t, r0Accept, "round 0 (high quality) must be accepted")

	r1Accept := r1.Verdict == types.QualityVerdict_QUALITY_VERDICT_GOLD ||
		r1.Verdict == types.QualityVerdict_QUALITY_VERDICT_SILVER ||
		r1.Verdict == types.QualityVerdict_QUALITY_VERDICT_BRONZE
	require.True(t, r1Accept, "round 1 (medium quality) must be accepted")

	require.Equal(t, types.QualityVerdict_QUALITY_VERDICT_REJECT, r2.Verdict,
		"round 2 (low quality) must be rejected")

	// ── Assert: aggregate scores are independent (high quality > low quality) ─
	require.Greater(t, r0.AggregateScores.OverallQuality, r1.AggregateScores.OverallQuality,
		"round 0 aggregate must be higher quality than round 1")
	require.Greater(t, r1.AggregateScores.OverallQuality, r2.AggregateScores.OverallQuality,
		"round 1 aggregate must be higher quality than round 2")

	// ── Assert: rounds are removed from active index after completion ──────────
	activesAfter := k.GetActiveRounds(ctx)
	for _, rID := range roundIDs {
		require.NotContains(t, activesAfter, rID,
			"completed round %s must be removed from active index", rID)
	}
}

// TestConcurrentRounds_DistinctVerifierSets verifies that 3 rounds with completely
// disjoint verifier sets process independently — a verifier in round A cannot
// affect the state of round B.
func TestConcurrentRounds_DistinctVerifierSets(t *testing.T) {
	k, ctx := setupKeeper(t)
	setupDefaultDomains(t, k, ctx)

	// 3 rounds with 3 verifiers each; verifier sets are disjoint.
	// Round A: verifier1, verifier2, verifier3.
	// Round B: rv1, rv2, rv3.
	// Round C: rv4, rv5, verifier1 (overlap to test shared verifier isolation).
	verifierSets := [][]string{
		{verifier1, verifier2, verifier3},
		{rv1, rv2, rv3},
		{rv4, rv5, verifier1}, // verifier1 shared with round A — must not bleed
	}

	roundIDs := make([]string, 3)
	for i, vs := range verifierSets {
		subID := fmt.Sprintf("distinct-vs-sub-%d", i)
		sub := &types.Submission{
			Id:          subID,
			Domain:      "technology",
			Submitter:   testAddr,
			Content:     fmt.Sprintf("distinct verifier set round %d", i),
			Stake:       "0",
			Status:      types.SubmissionStatus_SUBMISSION_STATUS_PENDING,
			Consent:     &types.ConsentProof{Type: types.ConsentType_CONSENT_TYPE_SELF_AUTHORED},
			ContentHash: fmt.Sprintf("dvs_hash_%03d", i),
		}
		require.NoError(t, k.SetSubmission(ctx, sub))

		rID, err := k.InitiateQualityRound(ctx, subID, "", vs)
		require.NoError(t, err)
		roundIDs[i] = rID
	}

	// ── Submit commits: each verifier to its own round only ───────────────────
	acceptVotes := []*types.QualityVote{
		{OverallQuality: 800_000, ConsentValid: true},
		{OverallQuality: 810_000, ConsentValid: true},
		{OverallQuality: 790_000, ConsentValid: true},
	}
	rejectVotes := []*types.QualityVote{
		{OverallQuality: 100_000, ConsentValid: true},
		{OverallQuality: 120_000, ConsentValid: true},
		{OverallQuality: 90_000, ConsentValid: true},
	}

	// Round A (accept), Round B (reject), Round C (accept).
	voteGroups := [][]*types.QualityVote{acceptVotes, rejectVotes, acceptVotes}
	for i, rID := range roundIDs {
		runFullRoundForVerifiers(t, k, ctx, rID, verifierSets[i], voteGroups[i])
	}

	for _, rID := range roundIDs {
		require.NoError(t, k.AggregateQualityRound(ctx, rID))
	}

	// ── Assert: verdicts are correct for each round ───────────────────────────
	rA, _ := k.GetQualityRound(ctx, roundIDs[0])
	rB, _ := k.GetQualityRound(ctx, roundIDs[1])
	rC, _ := k.GetQualityRound(ctx, roundIDs[2])

	aAccept := rA.Verdict == types.QualityVerdict_QUALITY_VERDICT_GOLD ||
		rA.Verdict == types.QualityVerdict_QUALITY_VERDICT_SILVER ||
		rA.Verdict == types.QualityVerdict_QUALITY_VERDICT_BRONZE
	require.True(t, aAccept, "round A (accept votes) must produce accepted verdict")

	require.Equal(t, types.QualityVerdict_QUALITY_VERDICT_REJECT, rB.Verdict,
		"round B (reject votes) must produce REJECT verdict")

	cAccept := rC.Verdict == types.QualityVerdict_QUALITY_VERDICT_GOLD ||
		rC.Verdict == types.QualityVerdict_QUALITY_VERDICT_SILVER ||
		rC.Verdict == types.QualityVerdict_QUALITY_VERDICT_BRONZE
	require.True(t, cAccept, "round C (accept votes) must produce accepted verdict")

	// ── Assert: verifier1 participation in round A didn't contaminate round C ─
	// Round C's reveals must only include rv4, rv5, and verifier1's round C votes.
	require.Len(t, rA.Reveals, 3, "round A must have 3 reveals")
	require.Len(t, rB.Reveals, 3, "round B must have 3 reveals")
	require.Len(t, rC.Reveals, 3, "round C must have 3 reveals")

	// Verify that round C's verifier1 reveal uses the round C commit hash,
	// not round A's. (If cross-contamination existed, the commit hash check would
	// have failed during SubmitReveal, causing an error above.)
	v1InC := false
	for _, reveal := range rC.Reveals {
		if reveal.Verifier == verifier1 {
			v1InC = true
			break
		}
	}
	require.True(t, v1InC, "verifier1 must appear in round C reveals")

	v1InA := false
	for _, reveal := range rA.Reveals {
		if reveal.Verifier == verifier1 {
			v1InA = true
			break
		}
	}
	require.True(t, v1InA, "verifier1 must appear in round A reveals")

	// Both references are independent — verifier1's vote in round A and round C
	// have different hashes (different roundIDs in ComputeQualityCommitHash).
	require.NotEqual(t, roundIDs[0], roundIDs[2],
		"rounds must have distinct IDs even with shared verifiers")
}
