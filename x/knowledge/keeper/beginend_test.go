package keeper_test

import (
	"testing"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/stretchr/testify/require"

	"github.com/zerone-chain/zerone/x/knowledge/keeper"
	"github.com/zerone-chain/zerone/x/knowledge/types"
)

// ─── BeginBlocker Edge Cases ─────────────────────────────────────────────────

func TestBeginBlocker_RoundNotFound_CleansUpActiveIndex(t *testing.T) {
	k, ctx := setupKeeper(t)
	require.NoError(t, k.SetActiveRound(ctx, "nonexistent-round"))

	require.NoError(t, k.BeginBlocker(ctx))

	actives := k.GetActiveRounds(ctx)
	require.NotContains(t, actives, "nonexistent-round")
}

func TestBeginBlocker_MultipleRounds_ProcessedIndependently(t *testing.T) {
	k, ctx := setupKeeper(t)
	setupDefaultDomains(t, k, ctx)

	sub1 := &types.Submission{Id: "s1", Domain: "technology", Status: types.SubmissionStatus_SUBMISSION_STATUS_PENDING}
	sub2 := &types.Submission{Id: "s2", Domain: "science", Status: types.SubmissionStatus_SUBMISSION_STATUS_PENDING}
	require.NoError(t, k.SetSubmission(ctx, sub1))
	require.NoError(t, k.SetSubmission(ctx, sub2))

	verifiers := []string{verifier1, verifier2, verifier3}
	roundID1, _ := k.InitiateQualityRound(ctx, "s1", "", verifiers)
	roundID2, _ := k.InitiateQualityRound(ctx, "s2", "", verifiers)

	// Commit to round1 only (3 commits)
	votes := []*types.QualityVote{
		{OverallQuality: 800000, ConsentValid: true},
		{OverallQuality: 800000, ConsentValid: true},
		{OverallQuality: 800000, ConsentValid: true},
	}
	salts := [][]byte{[]byte("a"), []byte("b"), []byte("c")}
	for i, v := range verifiers {
		hash := types.ComputeQualityCommitHash(roundID1, votes[i], salts[i])
		require.NoError(t, k.SubmitCommitment(ctx, &types.MsgSubmitCommitment{
			Verifier: v, RoundId: roundID1, CommitHash: hash,
		}))
	}

	// Advance past commit deadline
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	ctx = sdkCtx.WithBlockHeight(105).WithEventManager(sdk.NewEventManager())

	require.NoError(t, k.BeginBlocker(ctx))

	r1, _ := k.GetQualityRound(ctx, roundID1)
	require.Equal(t, types.VerificationPhase_VERIFICATION_PHASE_REVEAL, r1.Phase)

	r2, _ := k.GetQualityRound(ctx, roundID2)
	require.Equal(t, types.VerificationPhase_VERIFICATION_PHASE_EXPIRED, r2.Phase)
}

// ─── EndBlocker Edge Cases ───────────────────────────────────────────────────

func TestEndBlocker_Block0_Noop(t *testing.T) {
	k, ctx := setupKeeper(t)
	params := types.DefaultParams()
	_ = k.SetParams(ctx, &params)

	sdkCtx := sdk.UnwrapSDKContext(ctx)
	ctx = sdkCtx.WithBlockHeight(0).WithEventManager(sdk.NewEventManager())

	require.NoError(t, k.EndBlocker(ctx))
}

func TestEndBlocker_PatronageNotExpired_Remains(t *testing.T) {
	k, ctx := setupKeeper(t)
	params := types.DefaultParams()
	_ = k.SetParams(ctx, &params)

	_ = k.SetSample(ctx, &types.Sample{
		Id: "1", Energy: 1_000_000, EnergyCap: 1_000_000,
		Status: types.SampleStatus_SAMPLE_STATUS_GOLD, Content: "x",
		TotalRevenue: "0", PatronageExpiryBlock: 200,
	})

	sdkCtx := sdk.UnwrapSDKContext(ctx)
	ctx = sdkCtx.WithBlockHeight(50).WithEventManager(sdk.NewEventManager())

	require.NoError(t, k.EndBlocker(ctx))

	s, _ := k.GetSample(ctx, "1")
	require.Equal(t, uint64(200), s.PatronageExpiryBlock)
}

func TestEndBlocker_BountyAlreadyClaimed_NotDeleted(t *testing.T) {
	k, ctx := setupKeeper(t)
	params := types.DefaultParams()
	_ = k.SetParams(ctx, &params)

	_ = k.SetDataBounty(ctx, &types.DataBounty{
		Id: "b1", Domain: "tech", ExpiresAtBlock: 50,
		RewardAmount: "1000000", Claimed: true,
	})

	sdkCtx := sdk.UnwrapSDKContext(ctx)
	ctx = sdkCtx.WithBlockHeight(100).WithEventManager(sdk.NewEventManager())

	require.NoError(t, k.EndBlocker(ctx))

	_, found := k.GetDataBounty(ctx, "b1")
	require.True(t, found)
}

func TestEndBlocker_MultipleBountiesExpire(t *testing.T) {
	k, ctx := setupKeeper(t)
	params := types.DefaultParams()
	_ = k.SetParams(ctx, &params)

	_ = k.SetDataBounty(ctx, &types.DataBounty{Id: "b1", ExpiresAtBlock: 50})
	_ = k.SetDataBounty(ctx, &types.DataBounty{Id: "b2", ExpiresAtBlock: 80})
	_ = k.SetDataBounty(ctx, &types.DataBounty{Id: "b3", ExpiresAtBlock: 200})

	sdkCtx := sdk.UnwrapSDKContext(ctx)
	ctx = sdkCtx.WithBlockHeight(100).WithEventManager(sdk.NewEventManager())

	require.NoError(t, k.EndBlocker(ctx))

	_, f1 := k.GetDataBounty(ctx, "b1")
	require.False(t, f1)
	_, f2 := k.GetDataBounty(ctx, "b2")
	require.False(t, f2)
	_, f3 := k.GetDataBounty(ctx, "b3")
	require.True(t, f3)
}

func TestEndBlocker_PruningAtGracePeriod(t *testing.T) {
	k, ctx := setupKeeper(t)
	params := types.DefaultParams() // PruneGraceEpochs = 10
	_ = k.SetParams(ctx, &params)

	_ = k.SetSample(ctx, &types.Sample{
		Id: "1", Content: "will die", Energy: 0, EnergyCap: 1_000_000,
		AtRiskSinceEpoch: 1, Status: types.SampleStatus_SAMPLE_STATUS_GOLD,
		TotalRevenue: "0",
	})
	_ = k.SetAtRiskIndex(ctx, "1")

	sdkCtx := sdk.UnwrapSDKContext(ctx)
	ctx = sdkCtx.WithBlockHeight(1200).WithEventManager(sdk.NewEventManager())

	require.NoError(t, k.EndBlocker(ctx))

	s, _ := k.GetSample(ctx, "1")
	require.Equal(t, types.SampleStatus_SAMPLE_STATUS_PRUNED, s.Status)
	require.Empty(t, s.Content)
}

func TestEndBlocker_AtRiskTransitionAtEpoch(t *testing.T) {
	k, ctx := setupKeeper(t)
	params := types.DefaultParams()
	_ = k.SetParams(ctx, &params)

	_ = k.SetSample(ctx, &types.Sample{
		Id: "1", Energy: 1, EnergyCap: 1_000_000,
		Status: types.SampleStatus_SAMPLE_STATUS_GOLD, Content: "x",
		TotalRevenue: "0",
	})

	sdkCtx := sdk.UnwrapSDKContext(ctx)
	ctx = sdkCtx.WithBlockHeight(100).WithEventManager(sdk.NewEventManager())

	require.NoError(t, k.EndBlocker(ctx))

	s, _ := k.GetSample(ctx, "1")
	require.Equal(t, uint64(0), s.Energy)
	require.Equal(t, uint64(1), s.AtRiskSinceEpoch)
}

func TestEndBlocker_SponsoredSample_FitnessStillComputed(t *testing.T) {
	k, ctx := setupKeeper(t)
	params := types.DefaultParams()
	_ = k.SetParams(ctx, &params)

	_ = k.SetSample(ctx, &types.Sample{
		Id: "1", Energy: 1_000_000, EnergyCap: 1_000_000,
		QualityScore: 800_000,
		Status: types.SampleStatus_SAMPLE_STATUS_GOLD, Content: "x",
		TotalRevenue: "0", PatronageExpiryBlock: 200,
	})

	sdkCtx := sdk.UnwrapSDKContext(ctx)
	ctx = sdkCtx.WithBlockHeight(100).WithEventManager(sdk.NewEventManager())

	require.NoError(t, k.EndBlocker(ctx))

	s, _ := k.GetSample(ctx, "1")
	require.Greater(t, s.FitnessScore, uint64(0))
	require.Equal(t, uint64(1_000_000), s.Energy)
}

func TestExpireRound_ReturnsStake(t *testing.T) {
	k, ctx, bk := setupKeeperWithBank(t)
	setupDefaultDomains(t, k, ctx)

	sub := &types.Submission{
		Id: "s1", Domain: "technology", Submitter: testAddr,
		Stake:  "5000000",
		Status: types.SubmissionStatus_SUBMISSION_STATUS_PENDING,
	}
	require.NoError(t, k.SetSubmission(ctx, sub))

	verifiers := []string{verifier1, verifier2, verifier3}
	_, err := k.InitiateQualityRound(ctx, "s1", "", verifiers)
	require.NoError(t, err)

	sdkCtx := sdk.UnwrapSDKContext(ctx)
	ctx = sdkCtx.WithBlockHeight(105).WithEventManager(sdk.NewEventManager())
	require.NoError(t, k.BeginBlocker(ctx))

	require.GreaterOrEqual(t, len(bk.moduleToAccountCalls), 1)
	lastCall := bk.moduleToAccountCalls[len(bk.moduleToAccountCalls)-1]
	require.Equal(t, types.ModuleName, lastCall.from)
}

func TestExpireRound_SubmissionResetToPending(t *testing.T) {
	k, ctx, _ := setupKeeperWithBank(t)
	setupDefaultDomains(t, k, ctx)

	sub := &types.Submission{
		Id: "s1", Domain: "technology", Submitter: testAddr,
		Stake:  "1000000",
		Status: types.SubmissionStatus_SUBMISSION_STATUS_PENDING,
	}
	require.NoError(t, k.SetSubmission(ctx, sub))

	verifiers := []string{verifier1, verifier2, verifier3}
	_, err := k.InitiateQualityRound(ctx, "s1", "", verifiers)
	require.NoError(t, err)

	sdkCtx := sdk.UnwrapSDKContext(ctx)
	ctx = sdkCtx.WithBlockHeight(105).WithEventManager(sdk.NewEventManager())
	require.NoError(t, k.BeginBlocker(ctx))

	updatedSub, _ := k.GetSubmission(ctx, "s1")
	require.Equal(t, types.SubmissionStatus_SUBMISSION_STATUS_PENDING, updatedSub.Status)
}

func TestEndBlocker_MultipleEpochsOfDecay(t *testing.T) {
	k, ctx := setupKeeper(t)
	params := types.DefaultParams()
	_ = k.SetParams(ctx, &params)

	_ = k.SetSample(ctx, &types.Sample{
		Id: "1", Energy: 1_000_000, EnergyCap: 1_000_000,
		Status: types.SampleStatus_SAMPLE_STATUS_GOLD, Content: "x",
		TotalRevenue: "0",
	})

	for epoch := uint64(1); epoch <= 3; epoch++ {
		block := int64(epoch * keeper.EcologyEpochBlocks)
		sdkCtx := sdk.UnwrapSDKContext(ctx)
		ctx = sdkCtx.WithBlockHeight(block).WithEventManager(sdk.NewEventManager())
		require.NoError(t, k.EndBlocker(ctx))
	}

	s, _ := k.GetSample(ctx, "1")
	require.Equal(t, uint64(857_375), s.Energy)
}

func TestEndBlocker_BountyExpiryOnlyAtEpoch(t *testing.T) {
	k, ctx := setupKeeper(t)
	params := types.DefaultParams()
	_ = k.SetParams(ctx, &params)

	_ = k.SetDataBounty(ctx, &types.DataBounty{Id: "b1", ExpiresAtBlock: 50})

	sdkCtx := sdk.UnwrapSDKContext(ctx)
	ctx = sdkCtx.WithBlockHeight(99).WithEventManager(sdk.NewEventManager())
	require.NoError(t, k.EndBlocker(ctx))

	_, found := k.GetDataBounty(ctx, "b1")
	require.True(t, found)

	ctx = sdkCtx.WithBlockHeight(100).WithEventManager(sdk.NewEventManager())
	require.NoError(t, k.EndBlocker(ctx))

	_, found = k.GetDataBounty(ctx, "b1")
	require.False(t, found)
}

// ─── Full Lifecycle Integration Test ─────────────────────────────────────────

func TestFullLifecycle_SubmitToDecayToAccessToPrune(t *testing.T) {
	k, ctx, _ := setupKeeperWithBank(t)
	setupDefaultDomains(t, k, ctx)
	params := types.DefaultParams()
	_ = k.SetParams(ctx, &params)

	// Block 1: Submit data
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	ctx = sdkCtx.WithBlockHeight(1).WithEventManager(sdk.NewEventManager())
	sub := &types.Submission{
		Id: "s1", Content: "integration test data", Domain: "technology",
		Submitter: testAddr, SampleType: types.SampleType_SAMPLE_TYPE_TUTORIAL,
		Tags: []string{"golang"}, Stake: "1000000",
		Consent: &types.ConsentProof{Type: types.ConsentType_CONSENT_TYPE_SELF_AUTHORED},
		License: "MIT",
		Status:  types.SubmissionStatus_SUBMISSION_STATUS_PENDING,
	}
	require.NoError(t, k.SetSubmission(ctx, sub))

	// Block 2: Quality round starts, validators commit
	ctx = sdkCtx.WithBlockHeight(2).WithEventManager(sdk.NewEventManager())
	verifiers := []string{verifier1, verifier2, verifier3}
	roundID, err := k.InitiateQualityRound(ctx, "s1", "", verifiers)
	require.NoError(t, err)

	votes := []*types.QualityVote{
		{OverallQuality: 850_000, Novelty: 700_000, ReasoningDepth: 600_000, ConsentValid: true},
		{OverallQuality: 860_000, Novelty: 710_000, ReasoningDepth: 590_000, ConsentValid: true},
		{OverallQuality: 840_000, Novelty: 690_000, ReasoningDepth: 610_000, ConsentValid: true},
	}
	salts := [][]byte{[]byte("salt1"), []byte("salt2"), []byte("salt3")}
	for i, v := range verifiers {
		hash := types.ComputeQualityCommitHash(roundID, votes[i], salts[i])
		require.NoError(t, k.SubmitCommitment(ctx, &types.MsgSubmitCommitment{
			Verifier: v, RoundId: roundID, CommitHash: hash,
		}))
	}

	// Commit deadline passes -> REVEAL phase
	round, _ := k.GetQualityRound(ctx, roundID)
	ctx = sdkCtx.WithBlockHeight(int64(round.CommitDeadline + 1)).WithEventManager(sdk.NewEventManager())
	require.NoError(t, k.BeginBlocker(ctx))

	round, _ = k.GetQualityRound(ctx, roundID)
	require.Equal(t, types.VerificationPhase_VERIFICATION_PHASE_REVEAL, round.Phase)

	// Validators reveal
	for i, v := range verifiers {
		require.NoError(t, k.SubmitReveal(ctx, &types.MsgSubmitReveal{
			Verifier: v, RoundId: roundID, Scores: votes[i], Salt: salts[i],
		}))
	}

	// Reveal deadline passes -> AGGREGATION -> Sample created
	round, _ = k.GetQualityRound(ctx, roundID)
	ctx = sdkCtx.WithBlockHeight(int64(round.RevealDeadline + 1)).WithEventManager(sdk.NewEventManager())
	require.NoError(t, k.BeginBlocker(ctx))

	round, _ = k.GetQualityRound(ctx, roundID)
	require.Equal(t, types.VerificationPhase_VERIFICATION_PHASE_COMPLETE, round.Phase)
	require.Equal(t, types.QualityVerdict_QUALITY_VERDICT_GOLD, round.Verdict)

	// Verify sample created
	sampleIDs := k.GetSamplesByDomain(ctx, "technology")
	require.GreaterOrEqual(t, len(sampleIDs), 1)
	sample, ok := k.GetSample(ctx, sampleIDs[0])
	require.True(t, ok)
	require.Equal(t, keeper.DefaultEnergyCap, sample.Energy)
	sampleID := sampleIDs[0]

	// Epoch 1: energy decays
	ctx = sdkCtx.WithBlockHeight(100).WithEventManager(sdk.NewEventManager())
	require.NoError(t, k.EndBlocker(ctx))

	sample, _ = k.GetSample(ctx, sampleID)
	require.Less(t, sample.Energy, keeper.DefaultEnergyCap)
	energyAfterOneDecay := sample.Energy

	// Access -> energy restored
	k.RestoreEnergyOnAccess(ctx, sample, &params)
	_ = k.SetSample(ctx, sample)
	require.Greater(t, sample.Energy, energyAfterOneDecay)

	// Many epochs without access -> energy drops to 0 -> at-risk
	for epoch := uint64(2); epoch <= 300; epoch++ {
		block := int64(epoch * keeper.EcologyEpochBlocks)
		ctx = sdkCtx.WithBlockHeight(block).WithEventManager(sdk.NewEventManager())
		require.NoError(t, k.EndBlocker(ctx))
	}

	sample, _ = k.GetSample(ctx, sampleID)
	require.Equal(t, uint64(0), sample.Energy)
	require.Greater(t, sample.AtRiskSinceEpoch, uint64(0))

	// Continue past grace period -> sample pruned
	atRiskEpoch := sample.AtRiskSinceEpoch
	pruneEpoch := atRiskEpoch + params.PruneGraceEpochs + 1
	ctx = sdkCtx.WithBlockHeight(int64(pruneEpoch * keeper.EcologyEpochBlocks)).WithEventManager(sdk.NewEventManager())
	require.NoError(t, k.EndBlocker(ctx))

	sample, ok = k.GetSample(ctx, sampleID)
	require.True(t, ok, "record should still exist")
	require.Equal(t, types.SampleStatus_SAMPLE_STATUS_PRUNED, sample.Status)
	require.Empty(t, sample.Content, "content should be cleared after pruning")
}
