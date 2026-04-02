package cross_stack_test

import (
	"fmt"
	"testing"

	sdkmath "cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/stretchr/testify/require"

	knowledgetypes "github.com/zerone-chain/zerone/x/knowledge/types"
)

// ─── Scenario E: DOKIMANT Partial Rewards ────────────────────────────────────

// TestEdimanceE_DokimantPartialRetention verifies the DOKIMANT constitutive
// reward mechanism at integration level:
//
//   - Minority verifier receives 20% (MinorityRetentionBps) of their escrowed
//     stake back even on the losing side.
//   - Revenue distribution is quality-weighted: the verifier whose scores are
//     closest to the aggregate receives a larger share than one far from it.
//
// Setup: 5 verifiers. 4 vote ACCEPT (high quality). 1 votes REJECT (minority).
// DOKIMANT: minority gets 60K back (20% of 300K reviewer stake).
func TestEdimanceE_DokimantPartialRetention(t *testing.T) {
	h := NewTestHarness(t)

	// Pre-fund the knowledge module with enough uzrn to cover all payouts.
	// Submitter stake: 1_000_000. Reviewer stakes: 5 × 300_000 = 1_500_000.
	// Total distribution budget ~ 2_500_000; add buffer.
	fundKnowledgeModule(t, h, 3_000_000)

	verifiers := setupDistributedVerifierSet(t, h, "edimance_e", 5)
	submitter := testAddr("submitter_e_____")
	require.NoError(t, h.FundAccount(submitter, sdk.NewCoins(sdk.NewInt64Coin("uzrn", 100))))

	// Create submission with 1 ZRN stake.
	sub := &knowledgetypes.Submission{
		Id:          "edimance-e-sub",
		Domain:      "technology",
		Submitter:   submitter.String(),
		Content:     "EDIMANCE Scenario E: DOKIMANT partial retention",
		Stake:       "1000000",
		Status:      knowledgetypes.SubmissionStatus_SUBMISSION_STATUS_PENDING,
		Consent:     &knowledgetypes.ConsentProof{Type: knowledgetypes.ConsentType_CONSENT_TYPE_SELF_AUTHORED},
		ContentHash: "edimance_e_hash_001",
	}
	require.NoError(t, h.KnowledgeKeeper.SetSubmission(h.Ctx, sub))

	verifierStrs := make([]string, 5)
	for i, v := range verifiers {
		verifierStrs[i] = v.String()
	}
	roundID, err := h.KnowledgeKeeper.InitiateQualityRound(h.Ctx, "edimance-e-sub", "", verifierStrs)
	require.NoError(t, err)

	// v0-v3: accept (overall_quality ≥ 400_000).
	// v4: reject — minority (overall_quality < 400_000).
	votes := []*knowledgetypes.QualityVote{
		{OverallQuality: 810_000, ReasoningDepth: 700_000, Novelty: 650_000, Toxicity: 80_000, FactualAccuracy: 760_000, ConsentValid: true},
		{OverallQuality: 790_000, ReasoningDepth: 680_000, Novelty: 630_000, Toxicity: 90_000, FactualAccuracy: 740_000, ConsentValid: true},
		{OverallQuality: 830_000, ReasoningDepth: 720_000, Novelty: 670_000, Toxicity: 75_000, FactualAccuracy: 780_000, ConsentValid: true},
		{OverallQuality: 800_000, ReasoningDepth: 710_000, Novelty: 640_000, Toxicity: 85_000, FactualAccuracy: 750_000, ConsentValid: true},
		{OverallQuality: 190_000, ReasoningDepth: 160_000, Novelty: 110_000, Toxicity: 720_000, FactualAccuracy: 160_000, ConsentValid: true}, // minority
	}
	runCrossStackVerificationRound(t, h, roundID, verifiers, votes)

	// Snapshot minority verifier balance after commit (post-escrow, pre-distribution).
	// Expected: 500_000 funded - 300_000 escrowed = 200_000.
	balMinorityPostEscrow := h.GetBalance(verifiers[4], "uzrn")
	require.Equal(t, sdkmath.NewInt(200_000), balMinorityPostEscrow.Amount,
		"minority verifier should have 200K after staking escrow (500K - 300K)")

	// Run aggregation (triggers distributeAccept + DOKIMANT partial retention).
	require.NoError(t, h.KnowledgeKeeper.AggregateQualityRound(h.Ctx, roundID))

	// ── Assert: ACCEPT staking event with minority_count=1 ───────────────────
	events := h.Ctx.EventManager().Events()
	require.True(t, hasStakingEvent(events, roundID, "accept", 4, 1),
		"expected reviewer_staking event: outcome=accept majority_count=4 minority_count=1")

	// ── Assert: DOKIMANT partial retention for minority ───────────────────────
	// MinorityRetentionBps=2000 (20%). minority_retention = 300_000 × 20% = 60_000.
	// Minority balance: 200_000 (post-escrow) + 60_000 (retention) = 260_000.
	balMinorityFinal := h.GetBalance(verifiers[4], "uzrn")
	require.Equal(t, sdkmath.NewInt(260_000), balMinorityFinal.Amount,
		"minority verifier must receive 20%% DOKIMANT partial retention (200K + 60K)")

	// ── Assert: majority verifiers received rewards ───────────────────────────
	// Majority stake: 4 × 300_000 = 1_200_000 (already in module from EscrowReviewerStake).
	// effectiveMinorityPot = 300_000 - 60_000 = 240_000.
	// showUpPool = 240_000 × 10% = 24_000.
	// acceptReward = min(1_000_000 × 30%, 240_000 - 24_000) = min(300_000, 216_000) = 216_000.
	// per majority (showUp+remaining)/4 = (24_000 + 0) / 4 = 6_000.
	// Each majority final: 200_000 (post-escrow) + 300_000 (own stake back) + 6_000 = 506_000.
	for i := 0; i < 4; i++ {
		bal := h.GetBalance(verifiers[i], "uzrn")
		require.Equal(t, sdkmath.NewInt(506_000), bal.Amount,
			"majority verifier %d must receive 506K (own stake + show-up reward)", i)
	}

	// ── Assert: minority strictly less than majority (penalization < majority reward) ─
	require.True(t, balMinorityFinal.Amount.LT(h.GetBalance(verifiers[0], "uzrn").Amount),
		"minority verifier payout must be strictly less than majority verifier payout")

	// ── Assert: submission accepted ──────────────────────────────────────────
	finalSub, found := h.KnowledgeKeeper.GetSubmission(h.Ctx, "edimance-e-sub")
	require.True(t, found)
	require.Equal(t, knowledgetypes.SubmissionStatus_SUBMISSION_STATUS_ACCEPTED, finalSub.Status)
}

// TestEdimanceE_QualityWeightedRevenue verifies that the DOKIMANT quality-weighted
// revenue distribution pays the high-quality verifier more than the low-quality
// verifier, even if they are on the same (majority) side.
//
// Setup: 3 verifiers all accept. v_good's scores are very close to the aggregate;
// v_bad1 and v_bad2's scores deviate significantly on multiple dimensions.
// After revenue distribution, v_good must receive a larger share than v_bad1 or v_bad2.
func TestEdimanceE_QualityWeightedRevenue(t *testing.T) {
	h := NewTestHarness(t)

	// Pre-fund knowledge module for revenue distribution.
	fundKnowledgeModule(t, h, 5_000_000)

	// 3 verifiers: v_good is close to aggregate; v_bad1, v_bad2 are far.
	verifiers := setupDistributedVerifierSet(t, h, "edimance_e_rev", 3)
	submitter := testAddr("submitter_e_rev_")
	require.NoError(t, h.FundAccount(submitter, sdk.NewCoins(sdk.NewInt64Coin("uzrn", 100))))

	sub := &knowledgetypes.Submission{
		Id:          "edimance-e-rev-sub",
		Domain:      "technology",
		Submitter:   submitter.String(),
		Content:     "EDIMANCE Scenario E: quality-weighted revenue",
		Stake:       "0", // no staking for this test — focus on revenue
		Status:      knowledgetypes.SubmissionStatus_SUBMISSION_STATUS_PENDING,
		Consent:     &knowledgetypes.ConsentProof{Type: knowledgetypes.ConsentType_CONSENT_TYPE_SELF_AUTHORED},
		ContentHash: "edimance_e_rev_hash",
	}
	require.NoError(t, h.KnowledgeKeeper.SetSubmission(h.Ctx, sub))

	verifierStrs := make([]string, 3)
	for i, v := range verifiers {
		verifierStrs[i] = v.String()
	}
	roundID, err := h.KnowledgeKeeper.InitiateQualityRound(h.Ctx, "edimance-e-rev-sub", "", verifierStrs)
	require.NoError(t, err)

	// Votes deliberately designed so that after aggregation:
	//   aggregate ≈ {overall:800K, reasoning:700K, novelty:600K, tox:100K, factual:750K}
	// v_good (verifiers[0]): votes nearly identical to the aggregate → near-zero deviation.
	// v_bad1 (verifiers[1]): reasoning/novelty wildly different → high deviation.
	// v_bad2 (verifiers[2]): factual/overall far off → high deviation.
	votes := []*knowledgetypes.QualityVote{
		// v_good: close to final aggregate (deviation ≈ 0)
		{OverallQuality: 800_000, ReasoningDepth: 700_000, Novelty: 600_000, Toxicity: 100_000, FactualAccuracy: 750_000, ConsentValid: true},
		// v_bad1: reasoning and novelty far from consensus
		{OverallQuality: 810_000, ReasoningDepth: 200_000, Novelty: 100_000, Toxicity: 90_000, FactualAccuracy: 760_000, ConsentValid: true},
		// v_bad2: overall and factual far from consensus
		{OverallQuality: 600_000, ReasoningDepth: 710_000, Novelty: 610_000, Toxicity: 110_000, FactualAccuracy: 300_000, ConsentValid: true},
	}
	runCrossStackVerificationRound(t, h, roundID, verifiers, votes)
	require.NoError(t, h.KnowledgeKeeper.AggregateQualityRound(h.Ctx, roundID))

	// Verify the round has aggregate scores set (AggregateScores populated).
	round, found := h.KnowledgeKeeper.GetQualityRound(h.Ctx, roundID)
	require.True(t, found)
	require.NotNil(t, round.AggregateScores, "aggregate scores must be set after aggregation")
	require.Equal(t, knowledgetypes.VerificationPhase_VERIFICATION_PHASE_COMPLETE, round.Phase)

	// Create a sample linked to this submission with pending revenue.
	sampleID := "edimance-e-rev-sample"
	sample := &knowledgetypes.Sample{
		Id:           sampleID,
		SubmissionId: "edimance-e-rev-sub",
		Domain:       "technology",
		QualityTier:  "bronze",
		Status:       knowledgetypes.SampleStatus_SAMPLE_STATUS_BRONZE,
		Submitter:    submitter.String(),
		Content:      "EDIMANCE Scenario E: quality-weighted revenue sample",
		Energy:       500_000,
		EnergyCap:    1_000_000,
		Consent:      &knowledgetypes.ConsentProof{Type: knowledgetypes.ConsentType_CONSENT_TYPE_SELF_AUTHORED},
	}
	require.NoError(t, h.KnowledgeKeeper.SetSample(h.Ctx, sample))

	// Set the submission-to-round index so distributeToValidators can find the round.
	require.NoError(t, h.KnowledgeKeeper.SetSubmissionRoundIndex(h.Ctx, "edimance-e-rev-sub", roundID))

	// Enqueue pending revenue: 1_000_000 uzrn total.
	const totalRevenue = 1_000_000
	require.NoError(t, h.KnowledgeKeeper.SetPendingRevenue(h.Ctx, sampleID, totalRevenue))

	// Snapshot verifier balances before revenue distribution.
	preBals := make([]sdkmath.Int, 3)
	for i, v := range verifiers {
		preBals[i] = h.GetBalance(v, "uzrn").Amount
	}

	// Trigger epoch revenue distribution.
	params, err := h.KnowledgeKeeper.GetParams(h.Ctx)
	require.NoError(t, err)
	h.KnowledgeKeeper.DistributeEpochRevenue(h.Ctx, params)

	// Compute per-verifier revenue received.
	revReceived := make([]sdkmath.Int, 3)
	for i, v := range verifiers {
		postBal := h.GetBalance(v, "uzrn").Amount
		revReceived[i] = postBal.Sub(preBals[i])
	}

	// ── Assert: v_good (index 0) received more revenue than v_bad1 or v_bad2 ──
	// v_good's scores are at the aggregate median → near-zero deviation → max quality score.
	// v_bad1 and v_bad2 deviate significantly → lower quality scores → smaller share.
	require.True(t, revReceived[0].GT(revReceived[1]),
		"high-quality verifier (v_good) must receive more revenue than low-quality verifier (v_bad1): %s vs %s",
		revReceived[0], revReceived[1])
	require.True(t, revReceived[0].GT(revReceived[2]),
		"high-quality verifier (v_good) must receive more revenue than low-quality verifier (v_bad2): %s vs %s",
		revReceived[0], revReceived[2])

	// All verifiers received some revenue (quality-weighted, not zero-sum).
	for i, rev := range revReceived {
		require.True(t, rev.IsPositive(),
			fmt.Sprintf("verifier %d must receive some revenue even with lower quality score", i))
	}
}

// TestEdimanceE_MinorityRetentionScalesWithRetentionBps verifies that the
// partial retention amount scales correctly with the governance parameter.
func TestEdimanceE_MinorityRetentionScalesWithRetentionBps(t *testing.T) {
	h := NewTestHarness(t)
	fundKnowledgeModule(t, h, 3_000_000)

	// ── 10% retention: minority gets 10% of stake ─────────────────────────────
	rp := knowledgetypes.DefaultReviewerStakingParams()
	rp.MinorityRetentionBps = 1000 // 10%
	require.NoError(t, h.KnowledgeKeeper.SetReviewerStakingParams(h.Ctx, rp))

	verifiers10 := setupDistributedVerifierSet(t, h, "ret10", 3)
	sub10 := &knowledgetypes.Submission{
		Id: "ret10-sub", Domain: "technology",
		Submitter:   testAddr("ret10_submitter_").String(),
		Content:     "retention 10% test", Stake: "1000000",
		Status:  knowledgetypes.SubmissionStatus_SUBMISSION_STATUS_PENDING,
		Consent: &knowledgetypes.ConsentProof{Type: knowledgetypes.ConsentType_CONSENT_TYPE_SELF_AUTHORED},
		ContentHash: "ret10_hash",
	}
	require.NoError(t, h.KnowledgeKeeper.SetSubmission(h.Ctx, sub10))
	vs10 := []string{verifiers10[0].String(), verifiers10[1].String(), verifiers10[2].String()}
	rID10, err := h.KnowledgeKeeper.InitiateQualityRound(h.Ctx, "ret10-sub", "", vs10)
	require.NoError(t, err)
	votes10 := []*knowledgetypes.QualityVote{
		{OverallQuality: 800_000, ConsentValid: true},
		{OverallQuality: 820_000, ConsentValid: true},
		{OverallQuality: 200_000, ConsentValid: true}, // minority
	}
	runCrossStackVerificationRound(t, h, rID10, verifiers10, votes10)
	require.NoError(t, h.KnowledgeKeeper.AggregateQualityRound(h.Ctx, rID10))

	// 10% of 300K = 30K. Balance: 500K - 300K + 30K = 230K.
	bal10 := h.GetBalance(verifiers10[2], "uzrn")
	require.Equal(t, sdkmath.NewInt(230_000), bal10.Amount,
		"10%% retention: minority should have 230K (500K - 300K + 30K)")

	// ── 50% retention: minority gets 50% of stake ─────────────────────────────
	rp50 := knowledgetypes.DefaultReviewerStakingParams()
	rp50.MinorityRetentionBps = 5000 // 50%
	require.NoError(t, h.KnowledgeKeeper.SetReviewerStakingParams(h.Ctx, rp50))

	fundKnowledgeModule(t, h, 3_000_000)
	verifiers50 := setupDistributedVerifierSet(t, h, "ret50", 3)
	sub50 := &knowledgetypes.Submission{
		Id: "ret50-sub", Domain: "technology",
		Submitter:   testAddr("ret50_submitter_").String(),
		Content:     "retention 50% test", Stake: "1000000",
		Status:  knowledgetypes.SubmissionStatus_SUBMISSION_STATUS_PENDING,
		Consent: &knowledgetypes.ConsentProof{Type: knowledgetypes.ConsentType_CONSENT_TYPE_SELF_AUTHORED},
		ContentHash: "ret50_hash",
	}
	require.NoError(t, h.KnowledgeKeeper.SetSubmission(h.Ctx, sub50))
	vs50 := []string{verifiers50[0].String(), verifiers50[1].String(), verifiers50[2].String()}
	rID50, err := h.KnowledgeKeeper.InitiateQualityRound(h.Ctx, "ret50-sub", "", vs50)
	require.NoError(t, err)
	votes50 := []*knowledgetypes.QualityVote{
		{OverallQuality: 800_000, ConsentValid: true},
		{OverallQuality: 820_000, ConsentValid: true},
		{OverallQuality: 200_000, ConsentValid: true}, // minority
	}
	runCrossStackVerificationRound(t, h, rID50, verifiers50, votes50)
	require.NoError(t, h.KnowledgeKeeper.AggregateQualityRound(h.Ctx, rID50))

	// 50% of 300K = 150K. Balance: 500K - 300K + 150K = 350K.
	bal50 := h.GetBalance(verifiers50[2], "uzrn")
	require.Equal(t, sdkmath.NewInt(350_000), bal50.Amount,
		"50%% retention: minority should have 350K (500K - 300K + 150K)")

	// 50% retention > 10% retention.
	require.True(t, bal50.Amount.GT(bal10.Amount),
		"higher MinorityRetentionBps must produce higher minority payout")
}
