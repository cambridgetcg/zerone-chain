package keeper_test

import (
	"testing"

	sdkmath "cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/stretchr/testify/require"

	"github.com/zerone-chain/zerone/x/knowledge/keeper"
	"github.com/zerone-chain/zerone/x/knowledge/types"
)

// ─── Submitter Reputation ───────────────────────────────────────────────────

func TestReputationWiring_AcceptedSubmissionIncreasesSubmitterRep(t *testing.T) {
	k, ctx, _, roundID := setupSubmissionWithRound(t)

	// Gold-level votes from all 3 verifiers.
	votes := []*types.QualityVote{
		{OverallQuality: 850000, ConsentValid: true},
		{OverallQuality: 820000, ConsentValid: true},
		{OverallQuality: 900000, ConsentValid: true},
	}
	runFullRound(t, k, ctx, roundID, votes)
	require.NoError(t, k.AggregateQualityRound(ctx, roundID))

	// Submitter gains gold-tier reputation: base_gain(10) × 3 = 30.
	rep, found := k.GetAgentDomainReputation(ctx, testAddr, "technology")
	require.True(t, found)
	require.Equal(t, "30.000000000000000000", rep.Score)
	require.Equal(t, int64(100), rep.LastActiveHeight)
}

func TestReputationWiring_SilverSubmissionGains2x(t *testing.T) {
	k, ctx, _, roundID := setupSubmissionWithRound(t)

	// Silver-level votes.
	votes := []*types.QualityVote{
		{OverallQuality: 650000, ConsentValid: true},
		{OverallQuality: 620000, ConsentValid: true},
		{OverallQuality: 680000, ConsentValid: true},
	}
	runFullRound(t, k, ctx, roundID, votes)
	require.NoError(t, k.AggregateQualityRound(ctx, roundID))

	// Silver: base_gain(10) × 2 = 20.
	rep, found := k.GetAgentDomainReputation(ctx, testAddr, "technology")
	require.True(t, found)
	require.Equal(t, "20.000000000000000000", rep.Score)
}

func TestReputationWiring_BronzeSubmissionGains1x(t *testing.T) {
	k, ctx, _, roundID := setupSubmissionWithRound(t)

	// Bronze-level votes.
	votes := []*types.QualityVote{
		{OverallQuality: 450000, ConsentValid: true},
		{OverallQuality: 420000, ConsentValid: true},
		{OverallQuality: 480000, ConsentValid: true},
	}
	runFullRound(t, k, ctx, roundID, votes)
	require.NoError(t, k.AggregateQualityRound(ctx, roundID))

	// Bronze: base_gain(10) × 1 = 10.
	rep, found := k.GetAgentDomainReputation(ctx, testAddr, "technology")
	require.True(t, found)
	require.Equal(t, "10.000000000000000000", rep.Score)
}

func TestReputationWiring_RejectedSubmissionNoSubmitterGain(t *testing.T) {
	k, ctx, _, roundID := setupSubmissionWithRound(t)

	// All reject votes.
	votes := []*types.QualityVote{
		{OverallQuality: 100000, ConsentValid: true},
		{OverallQuality: 150000, ConsentValid: true},
		{OverallQuality: 120000, ConsentValid: true},
	}
	runFullRound(t, k, ctx, roundID, votes)
	require.NoError(t, k.AggregateQualityRound(ctx, roundID))

	// Submitter should NOT gain reputation on rejection.
	_, found := k.GetAgentDomainReputation(ctx, testAddr, "technology")
	require.False(t, found)
}

// ─── Majority Reviewer Reputation ───────────────────────────────────────────

func TestReputationWiring_MajorityReviewerGainsRep(t *testing.T) {
	k, ctx, _, roundID := setupSubmissionWithRound(t)

	// All 3 vote gold → all are majority.
	votes := []*types.QualityVote{
		{OverallQuality: 850000, ConsentValid: true},
		{OverallQuality: 820000, ConsentValid: true},
		{OverallQuality: 900000, ConsentValid: true},
	}
	runFullRound(t, k, ctx, roundID, votes)
	require.NoError(t, k.AggregateQualityRound(ctx, roundID))

	// Each reviewer gains 3.0 reputation.
	for _, v := range []string{verifier1, verifier2, verifier3} {
		rep, found := k.GetAgentDomainReputation(ctx, v, "technology")
		require.True(t, found, "verifier %s should have reputation", v)
		require.Equal(t, "3.000000000000000000", rep.Score)
		require.Equal(t, int64(100), rep.LastActiveHeight)
	}
}

// ─── Minority Reviewer Reputation ───────────────────────────────────────────

func TestReputationWiring_MinorityReviewerLosesRep(t *testing.T) {
	k, ctx, _, roundID := setupSubmissionWithRound(t)

	// Give verifier3 initial reputation so we can see the decrease.
	require.NoError(t, k.SetAgentDomainReputation(ctx, types.NewAgentDomainReputation(
		verifier3, "technology", sdkmath.LegacyNewDec(20), 50)))

	// 2 accept, 1 reject → verifier3 is minority.
	votes := []*types.QualityVote{
		{OverallQuality: 850000, ConsentValid: true},
		{OverallQuality: 820000, ConsentValid: true},
		{OverallQuality: 200000, ConsentValid: true}, // below bronze → reject side
	}
	runFullRound(t, k, ctx, roundID, votes)
	require.NoError(t, k.AggregateQualityRound(ctx, roundID))

	// Verifier3: 20 - 5 (penalty) = 15.
	rep, found := k.GetAgentDomainReputation(ctx, verifier3, "technology")
	require.True(t, found)
	require.Equal(t, "15.000000000000000000", rep.Score)
}

// ─── Timer Behavior ─────────────────────────────────────────────────────────

func TestReputationWiring_SuccessfulReviewResetsTimer(t *testing.T) {
	k, ctx, _, roundID := setupSubmissionWithRound(t)

	// Set verifier1 with old last-active height.
	require.NoError(t, k.SetAgentDomainReputation(ctx, types.NewAgentDomainReputation(
		verifier1, "technology", sdkmath.LegacyNewDec(10), 10)))

	// Gold votes → all majority.
	votes := []*types.QualityVote{
		{OverallQuality: 850000, ConsentValid: true},
		{OverallQuality: 820000, ConsentValid: true},
		{OverallQuality: 900000, ConsentValid: true},
	}
	runFullRound(t, k, ctx, roundID, votes)
	require.NoError(t, k.AggregateQualityRound(ctx, roundID))

	// verifier1's timer should be reset to current block (100).
	rep, found := k.GetAgentDomainReputation(ctx, verifier1, "technology")
	require.True(t, found)
	require.Equal(t, int64(100), rep.LastActiveHeight)
}

func TestReputationWiring_BadReviewDoesNotResetTimer(t *testing.T) {
	k, ctx, _, roundID := setupSubmissionWithRound(t)

	// Set verifier3 with old last-active height.
	require.NoError(t, k.SetAgentDomainReputation(ctx, types.NewAgentDomainReputation(
		verifier3, "technology", sdkmath.LegacyNewDec(20), 10)))

	// 2 accept, 1 reject → verifier3 is minority.
	votes := []*types.QualityVote{
		{OverallQuality: 850000, ConsentValid: true},
		{OverallQuality: 820000, ConsentValid: true},
		{OverallQuality: 200000, ConsentValid: true}, // reject
	}
	runFullRound(t, k, ctx, roundID, votes)
	require.NoError(t, k.AggregateQualityRound(ctx, roundID))

	// verifier3's timer should NOT be reset (stays at 10).
	rep, found := k.GetAgentDomainReputation(ctx, verifier3, "technology")
	require.True(t, found)
	require.Equal(t, int64(10), rep.LastActiveHeight)
}

// ─── Weighted Voting ────────────────────────────────────────────────────────

func TestReputationWiring_HighRepVoterWeightsAggregation(t *testing.T) {
	k, ctx, _, roundID := setupSubmissionWithRound(t)

	// Give verifier3 high reputation (100 → max weight 3.0x).
	require.NoError(t, k.SetAgentDomainReputation(ctx, types.NewAgentDomainReputation(
		verifier3, "technology", sdkmath.LegacyNewDec(100), 50)))

	// 2 low-rep voters score below bronze, 1 high-rep voter scores gold.
	// Without weighting: median([300000, 300000, 900000]) = 300000 → reject.
	// With weighting:    weights [1.0, 1.0, 3.0], half=2.5
	//   300000(cum=1.0), 300000(cum=2.0), 900000(cum=5.0>2.5) → 900000 → gold.
	votes := []*types.QualityVote{
		{OverallQuality: 300000, ConsentValid: true},
		{OverallQuality: 300000, ConsentValid: true},
		{OverallQuality: 900000, ConsentValid: true},
	}
	runFullRound(t, k, ctx, roundID, votes)
	require.NoError(t, k.AggregateQualityRound(ctx, roundID))

	round, found := k.GetQualityRound(ctx, roundID)
	require.True(t, found)
	// High-rep voter's score dominates → gold.
	require.Equal(t, types.QualityVerdict_QUALITY_VERDICT_GOLD, round.Verdict)
	require.Equal(t, uint64(900000), round.AggregateScores.OverallQuality)
}

func TestReputationWiring_EqualRepWeightsMatchSimpleMedian(t *testing.T) {
	k, ctx, _, roundID := setupSubmissionWithRound(t)

	// No reputation set → all weights = 1.0 (base).
	votes := []*types.QualityVote{
		{OverallQuality: 850000, ReasoningDepth: 800000, Novelty: 750000, Toxicity: 10000, FactualAccuracy: 900000, ConsentValid: true},
		{OverallQuality: 820000, ReasoningDepth: 780000, Novelty: 700000, Toxicity: 15000, FactualAccuracy: 880000, ConsentValid: true},
		{OverallQuality: 900000, ReasoningDepth: 850000, Novelty: 800000, Toxicity: 5000, FactualAccuracy: 950000, ConsentValid: true},
	}
	runFullRound(t, k, ctx, roundID, votes)
	require.NoError(t, k.AggregateQualityRound(ctx, roundID))

	round, found := k.GetQualityRound(ctx, roundID)
	require.True(t, found)
	// Same as simple median: 850000.
	require.Equal(t, uint64(850000), round.AggregateScores.OverallQuality)
	require.Equal(t, types.QualityVerdict_QUALITY_VERDICT_GOLD, round.Verdict)
}

// ─── Decay via Ecology ──────────────────────────────────────────────────────

func TestReputationWiring_InactivityDecayApplied(t *testing.T) {
	k, ctx := setupKeeper(t)

	// Set short decay interval for testing.
	params := types.ReputationDecayParams{
		DecayRateBps:        500,   // 5%
		DecayIntervalBlocks: 100,   // ~every 100 blocks
		FloorRatioBps:       2500,  // 25% floor
	}
	require.NoError(t, k.SetReputationDecayParams(ctx, params))

	// Agent active at block 0, reputation = 100.
	require.NoError(t, k.SetAgentDomainReputation(ctx, types.NewAgentDomainReputation(
		"agent1", "tech", sdkmath.LegacyNewDec(100), 0)))

	// Apply decay at block 500 (5 intervals of inactivity).
	// 100 × 0.95^5 ≈ 77.38
	k.ApplyReputationDecay(ctx, 500)

	rep, found := k.GetAgentDomainReputation(ctx, "agent1", "tech")
	require.True(t, found)
	require.True(t, rep.GetScore().LT(sdkmath.LegacyNewDec(80)))
	require.True(t, rep.GetScore().GT(sdkmath.LegacyNewDec(75)))
}

func TestReputationWiring_NeverBelowFloor(t *testing.T) {
	k, ctx := setupKeeper(t)

	params := types.ReputationDecayParams{
		DecayRateBps:        5000,  // 50% — aggressive
		DecayIntervalBlocks: 100,
		FloorRatioBps:       2500,  // 25% floor
	}
	require.NoError(t, k.SetReputationDecayParams(ctx, params))

	require.NoError(t, k.SetAgentDomainReputation(ctx, types.NewAgentDomainReputation(
		"agent1", "tech", sdkmath.LegacyNewDec(100), 0)))

	// Many periods of decay → should hit floor at 25.
	k.ApplyReputationDecay(ctx, 100000)

	rep, found := k.GetAgentDomainReputation(ctx, "agent1", "tech")
	require.True(t, found)
	floor := sdkmath.LegacyNewDec(25)
	require.True(t, rep.GetScore().GTE(floor), "score %s should be >= floor %s", rep.GetScore(), floor)
}

// ─── Domain Resolution ──────────────────────────────────────────────────────

func TestReputationWiring_DomainResolution(t *testing.T) {
	require.Equal(t, "technology", keeper.ResolveDomain("technology"))
	require.Equal(t, "general", keeper.ResolveDomain(""))
}

// ─── Vote Weight Computation ────────────────────────────────────────────────

func TestComputeVoteWeight(t *testing.T) {
	base := sdkmath.LegacyOneDec()
	mult := sdkmath.LegacyNewDec(2)

	tests := []struct {
		name       string
		repScore   sdkmath.LegacyDec
		expectMin  string
		expectMax  string
	}{
		{"zero rep", sdkmath.LegacyZeroDec(), "1.0", "1.0"},
		{"rep 50 (half)", sdkmath.LegacyNewDec(50), "2.0", "2.0"},
		{"rep 100 (max)", sdkmath.LegacyNewDec(100), "3.0", "3.0"},
		{"rep 200 (capped)", sdkmath.LegacyNewDec(200), "3.0", "3.0"},
		{"negative rep", sdkmath.LegacyNewDec(-10), "1.0", "1.0"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			w := keeper.ComputeVoteWeight(tc.repScore, base, mult)
			minW, _ := sdkmath.LegacyNewDecFromStr(tc.expectMin)
			maxW, _ := sdkmath.LegacyNewDecFromStr(tc.expectMax)
			require.True(t, w.GTE(minW), "weight %s < min %s", w, minW)
			require.True(t, w.LTE(maxW), "weight %s > max %s", w, maxW)
		})
	}
}

// ─── Decay Interval Gating in Ecology ───────────────────────────────────────

func TestReputationWiring_DecayGatingInEcologyEpoch(t *testing.T) {
	k, ctx := setupKeeper(t)

	// Set decay interval to 1000 blocks. EcologyEpochBlocks = 100.
	params := types.ReputationDecayParams{
		DecayRateBps:        500,
		DecayIntervalBlocks: 1000,
		FloorRatioBps:       2500,
	}
	require.NoError(t, k.SetReputationDecayParams(ctx, params))

	// Agent active at block 0, reputation = 100.
	require.NoError(t, k.SetAgentDomainReputation(ctx, types.NewAgentDomainReputation(
		"agent1", "tech", sdkmath.LegacyNewDec(100), 0)))

	// Run ecology epoch at block 500 — should NOT trigger decay (500 % 1000 = 500 >= 100).
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	ctx500 := sdkCtx.WithBlockHeight(500).WithEventManager(sdk.NewEventManager()).WithMultiStore(&mockCacheMultiStore{})
	k.RunEcologyEpoch(ctx500, 5)

	rep, found := k.GetAgentDomainReputation(ctx500, "agent1", "tech")
	require.True(t, found)
	require.Equal(t, sdkmath.LegacyNewDec(100), rep.GetScore(), "decay should not fire at block 500")

	// Run ecology epoch at block 2000 — SHOULD trigger decay (2000 % 1000 = 0 < 100).
	ctx2000 := sdkCtx.WithBlockHeight(2000).WithEventManager(sdk.NewEventManager()).WithMultiStore(&mockCacheMultiStore{})
	k.RunEcologyEpoch(ctx2000, 20)

	rep, found = k.GetAgentDomainReputation(ctx2000, "agent1", "tech")
	require.True(t, found)
	// 2000 blocks inactive / 1000 interval = 2 periods → 100 × 0.95^2 ≈ 90.25
	require.True(t, rep.GetScore().LT(sdkmath.LegacyNewDec(100)), "decay should fire at block 2000")
}

// ─── Params Getters ─────────────────────────────────────────────────────────

func TestReputationParamsGetters(t *testing.T) {
	p := types.DefaultReputationDecayParams()

	require.Equal(t, sdkmath.LegacyNewDec(10), p.GetSubmitterGain())
	require.Equal(t, sdkmath.LegacyNewDec(3), p.GetReviewerGain())
	require.Equal(t, sdkmath.LegacyNewDec(5), p.GetReviewerPenalty())
	require.Equal(t, sdkmath.LegacyNewDec(2), p.GetReputationMultiplier())
	require.Equal(t, sdkmath.LegacyOneDec(), p.GetBaseVoteWeight())
}

func TestReputationParamsGettersDefaults(t *testing.T) {
	// Zero-value struct should return defaults via getters.
	p := types.ReputationDecayParams{}

	require.Equal(t, sdkmath.LegacyNewDec(10), p.GetSubmitterGain())
	require.Equal(t, sdkmath.LegacyNewDec(3), p.GetReviewerGain())
	require.Equal(t, sdkmath.LegacyNewDec(5), p.GetReviewerPenalty())
	require.Equal(t, sdkmath.LegacyNewDec(2), p.GetReputationMultiplier())
	require.Equal(t, sdkmath.LegacyOneDec(), p.GetBaseVoteWeight())
}
