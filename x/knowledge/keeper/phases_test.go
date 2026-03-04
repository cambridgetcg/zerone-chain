package keeper_test

import (
	"testing"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/stretchr/testify/require"

	"github.com/zerone-chain/zerone/x/knowledge/types"
)

func TestBeginBlocker_CommitToRevealTransition(t *testing.T) {
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

	// First BeginBlocker: commit → reveal transition
	require.NoError(t, k.BeginBlocker(ctx))

	// The round should now be in reveal phase (commit deadline passed)
	// Run BeginBlocker again for reveal → expired transition
	require.NoError(t, k.BeginBlocker(ctx))

	round, found := k.GetQualityRound(ctx, roundID)
	require.True(t, found)
	require.Equal(t, types.VerificationPhase_VERIFICATION_PHASE_EXPIRED, round.Phase)

	actives := k.GetActiveRounds(ctx)
	require.NotContains(t, actives, roundID)
}
