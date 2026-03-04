package keeper_test

import (
	"testing"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/stretchr/testify/require"

	"github.com/zerone-chain/zerone/x/knowledge/types"
)

// TestBeginBlocker_CommitDeadline_NoCommits_ExpiresRound verifies that a round
// with 0 commits (below MinValidatorsPerRound=3) is expired when the commit
// deadline passes, rather than transitioning to REVEAL.
func TestBeginBlocker_CommitDeadline_NoCommits_ExpiresRound(t *testing.T) {
	k, ctx := setupKeeper(t)
	setupDefaultDomains(t, k, ctx)

	sub := &types.Submission{Id: "s1", Domain: "technology", Status: types.SubmissionStatus_SUBMISSION_STATUS_PENDING}
	require.NoError(t, k.SetSubmission(ctx, sub))

	verifiers := []string{verifier1, verifier2, verifier3}
	roundID, _ := k.InitiateQualityRound(ctx, "s1", "", verifiers)

	// Block 100 → commit deadline 104. Advance to block 105.
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	ctx = sdkCtx.WithBlockHeight(105).WithEventManager(sdk.NewEventManager())

	require.NoError(t, k.BeginBlocker(ctx))

	round, found := k.GetQualityRound(ctx, roundID)
	require.True(t, found)
	require.Equal(t, types.VerificationPhase_VERIFICATION_PHASE_EXPIRED, round.Phase)

	actives := k.GetActiveRounds(ctx)
	require.NotContains(t, actives, roundID)

	// Submission should be reset to PENDING
	sub2, found := k.GetSubmission(ctx, "s1")
	require.True(t, found)
	require.Equal(t, types.SubmissionStatus_SUBMISSION_STATUS_PENDING, sub2.Status)
}

// TestBeginBlocker_CommitToReveal_InsufficientCommits_ExpiresRound tests that
// a round with fewer commits than MinValidatorsPerRound is expired, with stake
// returned to the submitter.
func TestBeginBlocker_CommitToReveal_InsufficientCommits_ExpiresRound(t *testing.T) {
	k, ctx, bk := setupKeeperWithBank(t)
	setupDefaultDomains(t, k, ctx)

	sub := &types.Submission{
		Id: "s1", Domain: "technology", Submitter: testAddr,
		Stake:  "1000000",
		Status: types.SubmissionStatus_SUBMISSION_STATUS_PENDING,
	}
	require.NoError(t, k.SetSubmission(ctx, sub))

	verifiers := []string{verifier1, verifier2, verifier3}
	roundID, _ := k.InitiateQualityRound(ctx, "s1", "", verifiers)

	// Only 1 commit (below MinValidatorsPerRound=3)
	vote := &types.QualityVote{OverallQuality: 800000, ConsentValid: true}
	salt := []byte("s1")
	hash := types.ComputeQualityCommitHash(roundID, vote, salt)
	require.NoError(t, k.SubmitCommitment(ctx, &types.MsgSubmitCommitment{
		Verifier: verifier1, RoundId: roundID, CommitHash: hash,
	}))

	// Advance past commit deadline
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	ctx = sdkCtx.WithBlockHeight(105).WithEventManager(sdk.NewEventManager())

	require.NoError(t, k.BeginBlocker(ctx))

	round, found := k.GetQualityRound(ctx, roundID)
	require.True(t, found)
	require.Equal(t, types.VerificationPhase_VERIFICATION_PHASE_EXPIRED, round.Phase)

	actives := k.GetActiveRounds(ctx)
	require.NotContains(t, actives, roundID)

	// Stake should be returned
	require.True(t, len(bk.moduleToAccountCalls) > 0, "expected stake return")
}

// TestBeginBlocker_CommitToReveal_EnoughCommits_TransitionsToReveal verifies that
// when enough validators commit (>= MinValidatorsPerRound), the round transitions
// to REVEAL phase as expected.
func TestBeginBlocker_CommitToReveal_EnoughCommits_TransitionsToReveal(t *testing.T) {
	k, ctx := setupKeeper(t)
	setupDefaultDomains(t, k, ctx)

	sub := &types.Submission{Id: "s1", Domain: "technology", Status: types.SubmissionStatus_SUBMISSION_STATUS_PENDING}
	require.NoError(t, k.SetSubmission(ctx, sub))

	verifiers := []string{verifier1, verifier2, verifier3}
	roundID, _ := k.InitiateQualityRound(ctx, "s1", "", verifiers)

	// All 3 commit
	votes := []*types.QualityVote{
		{OverallQuality: 800000, ConsentValid: true},
		{OverallQuality: 800000, ConsentValid: true},
		{OverallQuality: 800000, ConsentValid: true},
	}
	salts := [][]byte{[]byte("s1"), []byte("s2"), []byte("s3")}
	for i, v := range verifiers {
		hash := types.ComputeQualityCommitHash(roundID, votes[i], salts[i])
		require.NoError(t, k.SubmitCommitment(ctx, &types.MsgSubmitCommitment{
			Verifier: v, RoundId: roundID, CommitHash: hash,
		}))
	}

	// Advance past commit deadline
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	ctx = sdkCtx.WithBlockHeight(105).WithEventManager(sdk.NewEventManager())

	require.NoError(t, k.BeginBlocker(ctx))

	round, found := k.GetQualityRound(ctx, roundID)
	require.True(t, found)
	require.Equal(t, types.VerificationPhase_VERIFICATION_PHASE_REVEAL, round.Phase)
}

func TestBeginBlocker_RevealToAggregation(t *testing.T) {
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

	votes := []*types.QualityVote{
		{OverallQuality: 850000, ConsentValid: true},
		{OverallQuality: 850000, ConsentValid: true},
		{OverallQuality: 850000, ConsentValid: true},
	}
	salts := [][]byte{[]byte("s1"), []byte("s2"), []byte("s3")}
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

	// Advance past reveal deadline (108)
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	ctx = sdkCtx.WithBlockHeight(109).WithEventManager(sdk.NewEventManager())

	require.NoError(t, k.BeginBlocker(ctx))

	round, found := k.GetQualityRound(ctx, roundID)
	require.True(t, found)
	require.Equal(t, types.VerificationPhase_VERIFICATION_PHASE_COMPLETE, round.Phase)
	require.Equal(t, types.QualityVerdict_QUALITY_VERDICT_GOLD, round.Verdict)
}

func TestBeginBlocker_NoActiveRounds(t *testing.T) {
	k, ctx := setupKeeper(t)
	require.NoError(t, k.BeginBlocker(ctx))
}

// TestBeginBlocker_ExpiredRound_NoReveals verifies that a round with no commits
// at all is expired in a single BeginBlocker call (the min-validators check
// catches it at commit deadline, before ever entering REVEAL phase).
func TestBeginBlocker_ExpiredRound_NoReveals(t *testing.T) {
	k, ctx := setupKeeper(t)
	setupDefaultDomains(t, k, ctx)

	sub := &types.Submission{Id: "s1", Domain: "technology", Status: types.SubmissionStatus_SUBMISSION_STATUS_PENDING}
	require.NoError(t, k.SetSubmission(ctx, sub))

	verifiers := []string{verifier1, verifier2, verifier3}
	roundID, _ := k.InitiateQualityRound(ctx, "s1", "", verifiers)

	// No commits, no reveals. Advance way past deadlines.
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	ctx = sdkCtx.WithBlockHeight(200).WithEventManager(sdk.NewEventManager())

	// Single BeginBlocker: commit deadline passed with 0 commits → expired directly
	require.NoError(t, k.BeginBlocker(ctx))

	round, found := k.GetQualityRound(ctx, roundID)
	require.True(t, found)
	require.Equal(t, types.VerificationPhase_VERIFICATION_PHASE_EXPIRED, round.Phase)

	actives := k.GetActiveRounds(ctx)
	require.NotContains(t, actives, roundID)
}

// ─── EndBlocker Tests ───────────────────────────────────────────────────────

func TestEndBlocker_NoError_NoParams(t *testing.T) {
	k, ctx := setupKeeper(t)
	require.NoError(t, k.EndBlocker(ctx))
}

func TestEndBlocker_EcologyEpochBoundary(t *testing.T) {
	k, ctx := setupKeeper(t)
	params := types.DefaultParams()
	_ = k.SetParams(ctx, &params)

	_ = k.SetSample(ctx, &types.Sample{
		Id: "1", Energy: 1_000_000, EnergyCap: 1_000_000,
		Status: types.SampleStatus_SAMPLE_STATUS_GOLD, Content: "x",
		TotalRevenue: "0", QualityScore: 500_000,
	})

	sdkCtx := sdk.UnwrapSDKContext(ctx)
	ctx = sdkCtx.WithBlockHeight(100).WithEventManager(sdk.NewEventManager())

	require.NoError(t, k.EndBlocker(ctx))

	s, _ := k.GetSample(ctx, "1")
	require.Less(t, s.Energy, uint64(1_000_000))
	require.Greater(t, s.FitnessScore, uint64(0))
}

func TestEndBlocker_NonEpoch_NoDecay(t *testing.T) {
	k, ctx := setupKeeper(t)
	params := types.DefaultParams()
	_ = k.SetParams(ctx, &params)

	_ = k.SetSample(ctx, &types.Sample{
		Id: "1", Energy: 1_000_000, EnergyCap: 1_000_000,
		Status: types.SampleStatus_SAMPLE_STATUS_GOLD, Content: "x",
		TotalRevenue: "0",
	})

	sdkCtx := sdk.UnwrapSDKContext(ctx)
	ctx = sdkCtx.WithBlockHeight(101).WithEventManager(sdk.NewEventManager())

	require.NoError(t, k.EndBlocker(ctx))

	s, _ := k.GetSample(ctx, "1")
	require.Equal(t, uint64(1_000_000), s.Energy)
}

func TestEndBlocker_SponsoredSampleSkipsDecay(t *testing.T) {
	k, ctx := setupKeeper(t)
	params := types.DefaultParams()
	_ = k.SetParams(ctx, &params)

	_ = k.SetSample(ctx, &types.Sample{
		Id: "1", Energy: 1_000_000, EnergyCap: 1_000_000,
		Status: types.SampleStatus_SAMPLE_STATUS_GOLD, Content: "x",
		TotalRevenue:         "0",
		PatronageExpiryBlock: 200,
	})

	sdkCtx := sdk.UnwrapSDKContext(ctx)
	ctx = sdkCtx.WithBlockHeight(100).WithEventManager(sdk.NewEventManager())

	require.NoError(t, k.EndBlocker(ctx))

	s, _ := k.GetSample(ctx, "1")
	require.Equal(t, uint64(1_000_000), s.Energy)
}

func TestEndBlocker_PatronageExpiry(t *testing.T) {
	k, ctx := setupKeeper(t)
	params := types.DefaultParams()
	_ = k.SetParams(ctx, &params)

	_ = k.SetSample(ctx, &types.Sample{
		Id: "1", Energy: 1_000_000, EnergyCap: 1_000_000,
		Status: types.SampleStatus_SAMPLE_STATUS_GOLD, Content: "x",
		TotalRevenue:         "0",
		PatronageExpiryBlock: 50,
	})

	sdkCtx := sdk.UnwrapSDKContext(ctx)
	ctx = sdkCtx.WithBlockHeight(100).WithEventManager(sdk.NewEventManager())

	require.NoError(t, k.EndBlocker(ctx))

	s, _ := k.GetSample(ctx, "1")
	require.Equal(t, uint64(0), s.PatronageExpiryBlock)
}

func TestEndBlocker_BountyExpiry(t *testing.T) {
	k, ctx := setupKeeper(t)
	params := types.DefaultParams()
	_ = k.SetParams(ctx, &params)

	_ = k.SetDataBounty(ctx, &types.DataBounty{
		Id: "b1", Domain: "tech", ExpiresAtBlock: 50,
		RewardAmount: "1000000",
	})
	_ = k.SetDataBounty(ctx, &types.DataBounty{
		Id: "b2", Domain: "sci", ExpiresAtBlock: 200,
		RewardAmount: "2000000",
	})

	sdkCtx := sdk.UnwrapSDKContext(ctx)
	ctx = sdkCtx.WithBlockHeight(100).WithEventManager(sdk.NewEventManager())

	require.NoError(t, k.EndBlocker(ctx))

	_, found := k.GetDataBounty(ctx, "b1")
	require.False(t, found)

	_, found2 := k.GetDataBounty(ctx, "b2")
	require.True(t, found2)
}

func TestEndBlocker_NicheRankingUpdate(t *testing.T) {
	k, ctx := setupKeeper(t)
	params := types.DefaultParams()
	_ = k.SetParams(ctx, &params)

	_ = k.SetSample(ctx, &types.Sample{
		Id: "1", NicheKey: "niche_a", Energy: 1_000_000, EnergyCap: 1_000_000,
		QualityScore: 500_000, Status: types.SampleStatus_SAMPLE_STATUS_GOLD,
		Content: "a", TotalRevenue: "0",
	})
	_ = k.SetSample(ctx, &types.Sample{
		Id: "2", NicheKey: "niche_a", Energy: 1_000_000, EnergyCap: 1_000_000,
		QualityScore: 900_000, Status: types.SampleStatus_SAMPLE_STATUS_GOLD,
		Content: "b", TotalRevenue: "0",
	})
	_ = k.SetNicheIndex(ctx, "niche_a", "1")
	_ = k.SetNicheIndex(ctx, "niche_a", "2")

	sdkCtx := sdk.UnwrapSDKContext(ctx)
	ctx = sdkCtx.WithBlockHeight(100).WithEventManager(sdk.NewEventManager())

	require.NoError(t, k.EndBlocker(ctx))

	s2, _ := k.GetSample(ctx, "2")
	require.True(t, s2.NicheLeader)
}
