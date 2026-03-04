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
