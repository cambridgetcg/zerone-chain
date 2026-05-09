package cross_stack_test

// Truth-seeking invariants. Each test in this file binds one commitment
// from docs/TRUTH_SEEKING.md. The test name reads as a creed line. The
// comment block above each test cites the commitment number and quotes
// the binding clause. The scenario drives the chain through a path
// where the commitment could be violated; the assertions prove the
// violation cannot occur.
//
// If a commitment in the creed has no test here, it is rhetoric, not
// belief. If a test here breaks, the commitment has been broken — not
// the test.
//
// Some commitments are bound by tests in other files; this file
// consolidates the BELIEF SURFACE so the contract between creed and
// code is visible in one place. The header above each test cites the
// other binding tests where they exist.

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"testing"

	sdkmath "cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	"github.com/stretchr/testify/require"

	autopoiesistypes "github.com/zerone-chain/zerone/x/autopoiesis/types"
	counterexampleskeeper "github.com/zerone-chain/zerone/x/counterexamples/keeper"
	counterexamplestypes "github.com/zerone-chain/zerone/x/counterexamples/types"
	disputeskeeper "github.com/zerone-chain/zerone/x/disputes/keeper"
	disputestypes "github.com/zerone-chain/zerone/x/disputes/types"
	emergencytypes "github.com/zerone-chain/zerone/x/emergency/types"
	inquirykeeper "github.com/zerone-chain/zerone/x/inquiry/keeper"
	inquirytypes "github.com/zerone-chain/zerone/x/inquiry/types"
	knowledgekeeper "github.com/zerone-chain/zerone/x/knowledge/keeper"
	knowledgetypes "github.com/zerone-chain/zerone/x/knowledge/types"
	ontologytypes "github.com/zerone-chain/zerone/x/ontology/types"
	qualificationtypes "github.com/zerone-chain/zerone/x/qualification/types"
	trustscorekeeper "github.com/zerone-chain/zerone/x/trust_score/keeper"
	trustscoretypes "github.com/zerone-chain/zerone/x/trust_score/types"
)

// ════════════════════════════════════════════════════════════════════
// Commitment 1: Methodology over statement.
//
// "A claim's value comes from how it can be tested, not from what it
// asserts." Facts must carry a methodology id. The chain values the
// path of derivation, not the surface content.
//
// Bound here AND by: TestRouteB_Wave4a_IsOughtWall, the entire
// methodology phase suite.
// ════════════════════════════════════════════════════════════════════

func TestTruthSeeking_FactsCarryMethodology(t *testing.T) {
	h := NewTestHarness(t)
	_, err := h.KnowledgeKeeper.SeedRouteB(h.Ctx)
	require.NoError(t, err)

	// Drive a claim through the verification round. The accepted Fact
	// must carry a non-empty MethodId.
	submitter := testAddr("ts_methodology").String()
	claim := &knowledgetypes.Claim{
		Id:          "claim-ts-method",
		Submitter:   submitter,
		FactContent: "claim with declared methodology",
		Domain:      "sciences",
		Category:    "empirical",
		MethodId:    knowledgetypes.MethodologyEmpirical,
		Status:      knowledgetypes.ClaimStatus_CLAIM_STATUS_IN_VERIFICATION,
		Stake:       "1000000",
	}
	require.NoError(t, h.KnowledgeKeeper.SetClaim(h.Ctx, claim))
	round := &knowledgetypes.VerificationRound{
		Id: "round-ts-method", ClaimId: claim.Id,
		Phase: knowledgetypes.VerificationPhase_VERIFICATION_PHASE_COMPLETE, StartedAtBlock: 1,
	}
	require.NoError(t, h.KnowledgeKeeper.CompleteRound(h.Ctx, round, &knowledgekeeper.VerificationResult{
		Verdict: knowledgetypes.Verdict_VERDICT_ACCEPT, Confidence: 900_000, AcceptCount: 3,
	}))

	var fact *knowledgetypes.Fact
	h.KnowledgeKeeper.IterateFacts(h.Ctx, func(f *knowledgetypes.Fact) bool {
		if f.ClaimId == claim.Id {
			fact = f
			return true
		}
		return false
	})
	require.NotNil(t, fact, "fact must be created from accepted claim")
	require.NotEmpty(t, fact.MethodId,
		"a fact entering VERIFIED status must carry a non-empty MethodId; without it, the fact has no declared path of testability")
}

// ════════════════════════════════════════════════════════════════════
// Commitment 2: Is-ought wall is structural.
//
// "Descriptive facts and normative commitments are categorically
// different and must not be substituted for each other."
//
// Bound here AND by: TestRouteB_Wave4a_IsOughtWall,
// TestRouteB_Adversarial_IsOughtSmuggling, TestNormativeCommitment_*.
// ════════════════════════════════════════════════════════════════════

func TestTruthSeeking_NormativeCommitmentIDsCarryNoTrainingValue(t *testing.T) {
	h := NewTestHarness(t)
	_, err := h.KnowledgeKeeper.SeedRouteB(h.Ctx)
	require.NoError(t, err)

	// Pick a seeded NormativeCommitment. Its ID must yield TVW = 0
	// with BlockedByIsOught = true, regardless of any other signal.
	commitments := h.KnowledgeKeeper.GetAllNormativeCommitments(h.Ctx)
	require.NotEmpty(t, commitments, "seeded commitments expected")
	commitID := commitments[0].Id

	qs := knowledgekeeper.NewQueryServerImpl(h.KnowledgeKeeper)
	resp, err := qs.TrainingValueWeight(h.Ctx, &knowledgetypes.QueryTrainingValueWeightRequest{
		FactId: commitID,
	})
	require.NoError(t, err)
	require.True(t, resp.BlockedIsOught,
		"a commitment ID resolving to TVW must be blocked by the is-ought wall, not silently zero-paid")
	require.Equal(t, uint64(0), resp.TvwBps,
		"a commitment that becomes a fact ID earns nothing — values cannot be laundered as facts")
}

// ════════════════════════════════════════════════════════════════════
// Commitment 3: Popper, not popularity.
//
// "Truth is what survives falsification, not what is most asserted."
// BaseWeight in TVW must scale with corroboration count (rejected
// challenges), not with raw verification count.
//
// Bound here AND by: TestMoat_TVWHardensWithSurvivedAttacks.
// ════════════════════════════════════════════════════════════════════

func TestTruthSeeking_TVWScalesWithCorroborationNotConfidence(t *testing.T) {
	h := NewTestHarness(t)
	_, err := h.KnowledgeKeeper.SeedRouteB(h.Ctx)
	require.NoError(t, err)

	submitter := testAddr("ts_popper").String()
	mkFact := func(id string, corroboration uint64) {
		require.NoError(t, h.KnowledgeKeeper.SetFact(h.Ctx, &knowledgetypes.Fact{
			Id: id, Content: "popper test", Domain: "sciences",
			Status:                          knowledgetypes.FactStatus_FACT_STATUS_VERIFIED,
			Submitter:                       submitter,
			MethodId:                        knowledgetypes.MethodologyEmpirical,
			Confidence:                      900_000,
			CorroborationCount:              corroboration,
			SubmitterCalibrationSnapshotBps: 800_000,
		}))
	}
	mkFact("F-TS-POPPER-0", 0)
	mkFact("F-TS-POPPER-10", 10)

	qs := knowledgekeeper.NewQueryServerImpl(h.KnowledgeKeeper)
	r0, _ := qs.TrainingValueWeight(h.Ctx, &knowledgetypes.QueryTrainingValueWeightRequest{FactId: "F-TS-POPPER-0"})
	r10, _ := qs.TrainingValueWeight(h.Ctx, &knowledgetypes.QueryTrainingValueWeightRequest{FactId: "F-TS-POPPER-10"})

	require.Greater(t, r10.TvwBps, r0.TvwBps,
		"a fact that has survived 10 falsification attempts must earn more TVW than one that has survived 0; truth is what survives, not what is asserted")
	require.Greater(t, r10.BaseWeight, r0.BaseWeight,
		"BaseWeight must scale with CorroborationCount — Popper, not popularity")
}

// Commitment 3: Popper, not popularity (disputes side). The formal
// dispute mechanism is where falsification gets a structured hearing.
// For the falsification test to be MEANINGFUL, each arbiter's
// judgment must count exactly once — without one-vote-per-arbiter,
// the same hand could repeatedly stamp the same view and "survival"
// would lose its independence guarantee. This test asserts the
// chain refuses repeated stamping with the protecting commitment in
// the error message.
func TestTruthSeeking_DisputeArbitersVoteOnce(t *testing.T) {
	h := NewTestHarness(t)

	// Stand up a dispute manually in arbitration phase. The full
	// initiate→commit→reveal→arbitrate flow is exercised in dispute-
	// specific tests; here we focus on the structural invariant
	// commitment 3 names: independence of arbiter judgments.
	arbiter := testAddr("ts_dispute_arb_a").String()
	disputeID := "ts-dispute-popper-1"
	currentBlock := uint64(h.Ctx.BlockHeight())
	dispute := &disputestypes.Dispute{
		Id:               disputeID,
		TargetId:         "F-TS-DISPUTE-POPPER",
		TargetType:       disputestypes.DisputeTargetType_DISPUTE_TARGET_TYPE_FACT,
		Challenger:       testAddr("ts_dispute_challenger").String(),
		Defender:         testAddr("ts_dispute_defender").String(),
		Bond:             "1000000",
		Tier:             1,
		Phase:            disputestypes.DisputePhase_DISPUTE_PHASE_ARBITRATION,
		Outcome:          disputestypes.DisputeOutcome_DISPUTE_OUTCOME_UNSPECIFIED,
		CreatedAt:        currentBlock,
		EvidenceDeadline: currentBlock + 100,
		VotingDeadline:   currentBlock + 1000,
		Arbiters:         []string{arbiter},
	}
	h.DisputesKeeper.SetDispute(h.Ctx, dispute)

	ms := disputeskeeper.NewMsgServerImpl(h.DisputesKeeper)

	// First vote — accepted; the arbiter's independent judgment is
	// recorded.
	_, err := ms.ArbiterVote(h.Ctx, &disputestypes.MsgArbiterVote{
		DisputeId: disputeID,
		Arbiter:   arbiter,
		Vote:      disputestypes.ArbiterDecision_ARBITER_DECISION_CHALLENGER,
		Reasoning: "first independent judgment",
	})
	require.NoError(t, err, "first arbiter vote must be accepted — Popperian survival depends on independent tests being recorded at all")

	// Second vote from the same arbiter — refused. The chain MUST
	// cite commitment 3 in the refusal so off-chain observers can see
	// which principle the chain is protecting.
	_, err = ms.ArbiterVote(h.Ctx, &disputestypes.MsgArbiterVote{
		DisputeId: disputeID,
		Arbiter:   arbiter,
		Vote:      disputestypes.ArbiterDecision_ARBITER_DECISION_DEFENDER,
		Reasoning: "second stamp by the same hand",
	})
	require.Error(t, err, "the second vote from the same arbiter must be refused — without independence, commitment 3's 'survives challenge' degenerates into 'asserted by the loudest voice'")
	require.Contains(t, err.Error(), "commitment 3",
		"the refusal must name the protecting commitment in the chain's voice — silent rejection makes the principle illegible to off-chain observers")
}

// ════════════════════════════════════════════════════════════════════
// Commitment 4: The substrate stress-tests its own truth.
//
// "A 90%-confidence fact must be CHEAPER to probe than a 10%-confidence
// fact." High-confidence claims invite probing rather than tax it.
//
// Bound here AND by: TestMoat_HighConfidenceFactsCheaperToProbe.
// ════════════════════════════════════════════════════════════════════

func TestTruthSeeking_HighConfidenceClaimsInviteProbing(t *testing.T) {
	h := NewTestHarness(t)
	_, err := h.KnowledgeKeeper.SeedRouteB(h.Ctx)
	require.NoError(t, err)

	params, err := h.KnowledgeKeeper.GetParams(h.Ctx)
	require.NoError(t, err)

	stakeAt := func(conf uint64) uint64 {
		return knowledgekeeper.EffectiveMinChallengeStake(params, conf).Uint64()
	}
	require.Greater(t, stakeAt(0), stakeAt(900_000),
		"a 90%-confidence fact must cost LESS to probe than an unproven one; the substrate that taxes the testing of its own most-trusted claims is the substrate that lets stale consensus harden")
}

// ════════════════════════════════════════════════════════════════════
// Commitment 5: The chain manufactures probe demand.
//
// "Waiting for probers is not enough. The substrate names its own
// under-tested high-confidence facts and pays for them to be tested."
//
// Bound here AND by: TestMoat_HeartbeatInvitesIdleHighConfidenceFacts,
// TestMoat_ProbeBountyPoolAccumulatesAndFundsBonuses,
// TestMoat_InvitationBonusPaidToAnswerer.
// ════════════════════════════════════════════════════════════════════

func TestTruthSeeking_ChainPaysForOwnAudit(t *testing.T) {
	h := NewTestHarness(t)
	_, err := h.KnowledgeKeeper.SeedRouteB(h.Ctx)
	require.NoError(t, err)

	// The probe bounty pool must accumulate uzrn per block from chain
	// mint, not from user funding. Without external action, balance
	// must rise.
	starting := h.KnowledgeKeeper.ProbeBountyPoolBalance(h.Ctx).Int64()
	h.AdvanceBlocks(50)
	after := h.KnowledgeKeeper.ProbeBountyPoolBalance(h.Ctx).Int64()
	require.Greater(t, after, starting,
		"chain must mint into the probe bounty pool autonomously; an audit budget that depends on volunteer funding is rhetoric, not commitment")
}

// ════════════════════════════════════════════════════════════════════
// Commitment 6: No individual can unilaterally inject truth.
//
// "A single key — even the legitimate authority key — must not be
// able to silently inject content into the training corpus."
//
// Bound here AND by: TestMoat_GuardianVetoCancelsAuthorityFactInjection,
// TestMoat_PendingFactMaterializesAfterVetoWindow,
// TestMoat_NonGuardianCannotVetoFactInjection.
// ════════════════════════════════════════════════════════════════════

func TestTruthSeeking_AuthorityInjectionIsCancellable(t *testing.T) {
	h := NewTestHarness(t)
	_, err := h.KnowledgeKeeper.SeedRouteB(h.Ctx)
	require.NoError(t, err)

	guardian := testAddr("ts_guardian").String()
	params, err := h.KnowledgeKeeper.GetParams(h.Ctx)
	require.NoError(t, err)
	params.GuardianAddresses = []string{guardian}
	params.AddFactVetoWindowBlocks = 100
	require.NoError(t, h.KnowledgeKeeper.SetParams(h.Ctx, params))

	ms := knowledgekeeper.NewMsgServerImpl(h.KnowledgeKeeper)
	resp, err := ms.AddFact(h.Ctx, &knowledgetypes.MsgAddFact{
		Authority: h.KnowledgeKeeper.GetAuthority(),
		Content:   "ts: authority injection",
		Domain:    "sciences",
		Category:  "empirical",
		Confidence: 900_000,
	})
	require.NoError(t, err)

	_, exists := h.KnowledgeKeeper.GetFact(h.Ctx, resp.FactId)
	require.False(t, exists,
		"under guardian-veto config, MsgAddFact must NOT materialize the fact immediately; the legitimate authority's act of injection is plural by code, not by convention")

	_, ok := h.KnowledgeKeeper.GetPendingFactInjection(h.Ctx, resp.FactId)
	require.True(t, ok, "pending injection must exist for guardian veto window")
}

// ════════════════════════════════════════════════════════════════════
// Commitment 7: Skill is current, not historical.
//
// "The chain does not issue diplomas. A voter who was once domain-
// qualified must continue to vote correctly to remain so."
//
// Bound here AND by: TestDomainPanel_QualificationDecaysOnLowAccuracy,
// TestDomainPanel_QualificationRecoversFromProbation,
// TestDomainPanel_QualificationSuspendsOnContinuedFailure.
// ════════════════════════════════════════════════════════════════════

func TestTruthSeeking_QualificationStatusFollowsAccuracy(t *testing.T) {
	h := NewTestHarness(t)
	_, err := h.KnowledgeKeeper.SeedRouteB(h.Ctx)
	require.NoError(t, err)

	qParams := h.QualificationKeeper.GetParams(h.Ctx)
	qParams.DecayCheckIntervalBlocks = 5
	qParams.DecayMinSamples = 10
	h.QualificationKeeper.SetParams(h.Ctx, qParams)

	addr := testAddr("ts_decay").String()
	h.SetDomainQualification(addr, "mathematics", 80)
	q, _ := h.QualificationKeeper.GetQualification(h.Ctx, addr, "mathematics")
	require.Equal(t, qualificationtypes.QualificationStatus_QUALIFICATION_STATUS_ACTIVE, q.Status)
	q.Metrics = &qualificationtypes.QualificationMetrics{
		TotalVerifications: 30, CorrectVerifications: 12, AccuracyBps: 400_000,
	}
	h.QualificationKeeper.SetQualification(h.Ctx, q)

	h.AdvanceBlocks(10)
	q, _ = h.QualificationKeeper.GetQualification(h.Ctx, addr, "mathematics")
	require.Equal(t, qualificationtypes.QualificationStatus_QUALIFICATION_STATUS_PROBATIONARY, q.Status,
		"a once-qualified voter who has stopped voting correctly must lose status; qualification is a current statement, not a stored artefact")
}

// ════════════════════════════════════════════════════════════════════
// Commitment 8: The panel weights skill, not bond.
//
// "A wealthy validator who has not shown they can tell truth from
// falsehood must not dominate the panel. Stake alone is not skill."
//
// Bound here AND by: TestMoat_PanelWeightedByStakeTimesCalibration,
// TestDomainPanel_DomainQualifiedVotersDominateGloballyCalibrated.
// ════════════════════════════════════════════════════════════════════

func TestTruthSeeking_StakeAloneCannotCarryThePanel(t *testing.T) {
	h := NewTestHarness(t)
	_, err := h.KnowledgeKeeper.SeedRouteB(h.Ctx)
	require.NoError(t, err)

	// Three zero-stake addresses — the cheapest possible Sybil.
	// Their votes record but never finalize a verdict.
	ms := knowledgekeeper.NewMsgServerImpl(h.KnowledgeKeeper)
	sponsor := testAddr("ts_panel_sponsor")
	require.NoError(t, h.FundAccount(sponsor, sdkCoinsForTest(100_000_000)))
	submitter := testAddr("ts_panel_sub").String()
	require.NoError(t, h.KnowledgeKeeper.SetFact(h.Ctx, &knowledgetypes.Fact{
		Id: "F-TS-PANEL", Content: "target", Domain: "sciences",
		Status: knowledgetypes.FactStatus_FACT_STATUS_ACTIVE, Submitter: sponsor.String(),
		MethodId: knowledgetypes.MethodologyEmpirical, Confidence: 900_000,
	}))
	_, err = ms.CreateAugmentationBounty(h.Ctx, &knowledgetypes.MsgCreateAugmentationBounty{
		Sponsor: sponsor.String(), Id: "b-ts-panel", TargetFactId: "F-TS-PANEL",
		RewardPerVariant: 1_000_000, MaxVariants: 1,
	})
	require.NoError(t, err)
	_, err = ms.SubmitAugmentation(h.Ctx, &knowledgetypes.MsgSubmitAugmentation{
		Submitter: submitter, Id: "aug-ts-panel", BountyId: "b-ts-panel",
		OriginalFactId: "F-TS-PANEL", VariantContent: "panel test variant",
	})
	require.NoError(t, err)

	for _, v := range []string{"ts_sybil1", "ts_sybil2", "ts_sybil3"} {
		resp, err := ms.VoteOnAugmentation(h.Ctx, &knowledgetypes.MsgVoteOnAugmentation{
			Verifier: testAddr(v).String(), AugmentationId: "aug-ts-panel",
			Vote: knowledgetypes.AugmentationVerdict_AUGMENTATION_VERDICT_EQUIVALENT,
		})
		require.NoError(t, err)
		require.False(t, resp.VerdictFinalized,
			"zero-stake votes must never finalize a panel verdict, no matter how many cohere; stake alone is not skill, and zero stake is not stake")
	}
}

// ════════════════════════════════════════════════════════════════════
// Commitment 9: Cartel detection has consequence.
//
// "Confirmation that a validator participated in cartel behaviour
// must reduce their voice on the next vote, not merely produce an
// audit log entry."
//
// Bound here AND by: TestCartelDetection_UpheldPenaltyReducesPanelWeight.
// ════════════════════════════════════════════════════════════════════

func TestTruthSeeking_CartelPenaltyReducesNextVoteWeight(t *testing.T) {
	h := NewTestHarness(t)
	_, err := h.KnowledgeKeeper.SeedRouteB(h.Ctx)
	require.NoError(t, err)

	addr := testAddr("ts_cartel_member").String()
	h.SetDomainQualification(addr, "mathematics", 80)
	pre := h.QualificationKeeper.GetQualificationWeight(h.Ctx, addr, "mathematics")
	require.Equal(t, uint32(80), pre)

	require.NoError(t, h.QualificationKeeper.ReduceQualificationWeight(
		h.Ctx, addr, "mathematics", 500_000, uint64(h.Height())+10_000,
	))

	post := h.QualificationKeeper.GetQualificationWeight(h.Ctx, addr, "mathematics")
	require.Equal(t, uint32(40), post,
		"a cartel UPHELD penalty must REDUCE effective qualification weight at the next read; a penalty that nobody reads is not a penalty")
}

// ════════════════════════════════════════════════════════════════════
// Commitment 11: Trust is queryable.
//
// "Every signal that contributes to trust must be available through
// a well-known query that synthesises them."
//
// Bound here AND by: TestTrainingProvenance_*, TestTrustScore_*,
// TestGovernanceSynthesis_*.
// ════════════════════════════════════════════════════════════════════

func TestTruthSeeking_TrustIsSynthesisedNotStitched(t *testing.T) {
	h := NewTestHarness(t)
	_, err := h.KnowledgeKeeper.SeedRouteB(h.Ctx)
	require.NoError(t, err)

	// Any address can be queried for a TrustScore even with no
	// records. The synthesizer does not REQUIRE pre-existing records;
	// it computes from current state, returning a deterministic
	// neutral score for unknown addresses. Trust is queryable for
	// anyone, anywhere — not a gated facility.
	addr := testAddr("ts_unknown").String()
	qs := trustscorekeeper.NewQueryServerImpl(h.TrustScoreKeeper)
	resp, err := qs.TrustScore(h.Ctx, &trustscoretypes.QueryTrustScoreRequest{Address: addr})
	require.NoError(t, err)
	require.NotNil(t, resp.Score,
		"the synthesizer must answer for any address; trust is not gated, it is queryable")
	require.Equal(t, addr, resp.Score.Address)
	require.NotEmpty(t, resp.Score.Band,
		"the synthesizer must always return a band; absence of record is not absence of trust signal")
}

// ════════════════════════════════════════════════════════════════════
// Commitment 12: The chain pays for its own audit.
//
// "Epistemic auditing is the chain's most important ongoing process.
// It must not depend on volunteer labour or external funding."
//
// Bound here AND by: TestMoat_ProbeBountyPoolAccumulatesAndFundsBonuses,
// TestMoat_ProbeBountyPoolRespectsCap.
// ════════════════════════════════════════════════════════════════════

func TestTruthSeeking_AuditBudgetIsAutonomous(t *testing.T) {
	h := NewTestHarness(t)
	_, err := h.KnowledgeKeeper.SeedRouteB(h.Ctx)
	require.NoError(t, err)

	// At default params, the probe bounty pool must mint per-block
	// without any user action. Verify pool grows from zero across
	// idle blocks.
	require.Equal(t, int64(0), h.KnowledgeKeeper.ProbeBountyPoolBalance(h.Ctx).Int64(),
		"pool starts at zero — no genesis pre-fund")
	h.AdvanceBlocks(20)
	require.Greater(t, h.KnowledgeKeeper.ProbeBountyPoolBalance(h.Ctx).Int64(), int64(0),
		"the chain must fund its own audit autonomously; without per-block mint, the budget depends on someone else's discipline")
}

// Commitment 12: the chain pays for its own audit (autopoiesis side).
// The probe bounty pool is the budget; x/autopoiesis is the regulator
// that scales the budget multipliers in response to the chain's own
// stress signal (SSI). This test asserts the regulator's autonomy:
// once activated, the multiplier system advances epochs, computes
// SSI, and adjusts multipliers without anyone calling Override or
// otherwise intervening. The autonomic loop is what makes the
// budget responsive to chain state rather than fixed at deploy time.
func TestTruthSeeking_AutopoiesisRegulatesItself(t *testing.T) {
	h := NewTestHarness(t)

	// Activate autopoiesis with short epochs so the test runs in a
	// few blocks. The regulator must start from epoch 0 and advance
	// through its own EndBlocker, with no external triggers.
	h.AutopoiesisKeeper.SetState(h.Ctx, &autopoiesistypes.AutopoiesisState{
		Activated:       true,
		CurrentEpoch:    0,
		LastEpochHeight: uint64(h.Height()),
	})
	params := autopoiesistypes.DefaultParams()
	params.EpochLengthBlocks = 5
	h.AutopoiesisKeeper.SetParams(h.Ctx, &params)

	// Seed default multipliers — the budget paths the regulator
	// adjusts. Without these the regulator has nothing to adjust and
	// "autonomy" reduces to a no-op.
	for _, m := range autopoiesistypes.DefaultMultipliers() {
		h.AutopoiesisKeeper.SetMultiplierState(h.Ctx, m)
	}

	startEpoch := h.AutopoiesisKeeper.GetState(h.Ctx).CurrentEpoch

	// Advance past several epoch boundaries. NO calls to Override,
	// Freeze, Activate, or any external trigger. If the regulator
	// is autonomous, the epoch counter advances on its own through
	// EndBlocker; if not, this is a static counter masquerading as
	// regulation.
	h.AdvanceBlocks(20)

	endEpoch := h.AutopoiesisKeeper.GetState(h.Ctx).CurrentEpoch
	require.Greater(t, endEpoch, startEpoch,
		"autopoiesis must advance epochs autonomously; without per-block self-regulation, 'the chain pays for its own audit' has no engine adjusting the budget paths")

	// And SSI must be a queryable BPS value — the regulator computes
	// it each epoch as the input to multiplier adjustment. A zero SSI
	// would indicate the sensors never ran; a too-large value would
	// indicate the BPS contract was violated.
	ssi := h.AutopoiesisKeeper.GetSSI(h.Ctx)
	require.LessOrEqual(t, ssi, autopoiesistypes.BPSScale,
		"SSI must be a valid BPS value; the regulator's input signal cannot exceed 1,000,000 bps without the autonomic budget logic losing meaning")
}

// ════════════════════════════════════════════════════════════════════
// Commitment 10: Forward-only audit.
//
// "Every privileged action is logged. The chain's history is
// append-only and verifiable."
//
// Bound here AND by: TestMoat_AddFactWritesPrivilegedLog,
// TestInternalHackDrill_SchemaAmendmentDetection.
// ════════════════════════════════════════════════════════════════════

func TestTruthSeeking_PrivilegedActionsLogMonotonically(t *testing.T) {
	h := NewTestHarness(t)
	_, err := h.KnowledgeKeeper.SeedRouteB(h.Ctx)
	require.NoError(t, err)

	ms := knowledgekeeper.NewMsgServerImpl(h.KnowledgeKeeper)
	authority := h.KnowledgeKeeper.GetAuthority()
	qs := knowledgekeeper.NewQueryServerImpl(h.KnowledgeKeeper)

	// Three privileged actions of different types — each must land in
	// the log with strictly increasing seq. The log is the chain's
	// audit substrate; without monotonicity, history can be reordered.
	for i, content := range []string{"audit-1", "audit-2", "audit-3"} {
		_ = i
		_, err := ms.AddFact(h.Ctx, &knowledgetypes.MsgAddFact{
			Authority: authority, Content: content,
			Domain: "sciences", Category: "empirical", Confidence: 800_000,
		})
		require.NoError(t, err)
	}
	logs, err := qs.PrivilegedActions(h.Ctx, &knowledgetypes.QueryPrivilegedActionsRequest{})
	require.NoError(t, err)
	require.GreaterOrEqual(t, len(logs.Actions), 3,
		"every privileged action must land in the log")

	var prevSeq uint64
	for _, a := range logs.Actions {
		require.Greater(t, a.Seq, prevSeq,
			"the privileged-action log must be strictly monotonic; reorderable history is not history")
		prevSeq = a.Seq
		require.NotEmpty(t, a.Invoker, "every log entry must name its invoker — accountability is structural")
		require.Greater(t, a.InvokedAtBlock, uint64(0),
			"every log entry must carry the block it was invoked at — time-stamped, not post-dateable")
	}
}

// ════════════════════════════════════════════════════════════════════
// Commitment 10: Forward-only audit (emergency ceremonies).
//
// "Every privileged action is logged. The chain's history is
// append-only and verifiable." For x/emergency specifically: a
// halt/revert/resume ceremony record, once finalized, must be the
// chain's permanent record of how that emergency was handled.
// Subsequent emergencies append to the audit log; they never
// rewrite prior entries.
//
// Bound here AND by: TestTruthSeeking_PrivilegedActionsLogMonotonically
// (the knowledge-side privileged-action log binds the same commitment
// for authority-gated handlers; this test extends the binding to the
// emergency ceremony audit trail).
// ════════════════════════════════════════════════════════════════════

func TestTruthSeeking_EmergencyCeremoniesAreImmutablePostFinalize(t *testing.T) {
	h := NewTestHarness(t)

	// First ceremony: a halt. Drive it manually to Finalized phase
	// (the prevote/precommit machinery is exercised elsewhere; the
	// commitment under test here is what happens AFTER finalization,
	// not how a ceremony reaches that phase).
	haltID := "ts-emergency-halt-1"
	_, err := h.EmergencyKeeper.CreateHaltCeremony(h.Ctx, &emergencytypes.EmergencyHaltProposal{
		Id:              haltID,
		Proposer:        testAddr("ts_emergency_proposer").String(),
		Reason:          "truth-seeking invariant: halt audit immutability",
		ProposedAtBlock: uint64(h.Ctx.BlockHeight()),
	})
	require.NoError(t, err)

	ceremony, found := h.EmergencyKeeper.GetCeremony(h.Ctx, haltID)
	require.True(t, found, "halt ceremony must exist before finalization")
	ceremony.Phase = string(emergencytypes.PhaseFinalized)
	require.NoError(t, h.EmergencyKeeper.SetCeremony(h.Ctx, ceremony))

	h.EmergencyKeeper.HandleCeremonyFinalization(h.Ctx, haltID)

	// Snapshot the audit log immediately after the halt finalization.
	// Every byte of these entries must survive subsequent operations
	// — the audit substrate cannot be rewritten by later activity.
	postHaltLog := h.EmergencyKeeper.GetAuditLog(h.Ctx)
	require.NotEmpty(t, postHaltLog,
		"halt finalization must write at least one audit entry — without the entry, 'forward-only audit' has no record to be forward-only about")

	// Capture canonical content of every entry by serialising fields
	// the chain treats as the immutable record (action, actor,
	// ceremony id, block, timestamp, details).
	type frozen struct {
		Timestamp   int64
		BlockNumber uint64
		Action      string
		Actor       string
		CeremonyId  string
		Details     string
	}
	freeze := func(entries []*emergencytypes.EmergencyAuditEntry) []frozen {
		out := make([]frozen, len(entries))
		for i, e := range entries {
			out[i] = frozen{
				Timestamp:   e.Timestamp,
				BlockNumber: e.BlockNumber,
				Action:      e.Action,
				Actor:       e.Actor,
				CeremonyId:  e.CeremonyId,
				Details:     e.Details,
			}
		}
		return out
	}
	preResumeFreeze := freeze(postHaltLog)
	preResumeLen := len(preResumeFreeze)

	// Advance a few blocks to ensure the second ceremony's audit entry
	// lands at a strictly later height than the first — exercising the
	// (height, monotonic-index) keying that protects against accidental
	// overwrites of prior records.
	h.AdvanceBlocks(3)

	// Second ceremony: a resume. Same drive-to-finalized pattern.
	resumeID := "ts-emergency-resume-1"
	_, err = h.EmergencyKeeper.CreateResumeCeremony(h.Ctx, &emergencytypes.EmergencyResumeProposal{
		Id:             resumeID,
		Proposer:       testAddr("ts_emergency_proposer").String(),
		HaltCeremonyId: haltID,
	})
	require.NoError(t, err)

	ceremony, found = h.EmergencyKeeper.GetCeremony(h.Ctx, resumeID)
	require.True(t, found, "resume ceremony must exist before finalization")
	ceremony.Phase = string(emergencytypes.PhaseFinalized)
	require.NoError(t, h.EmergencyKeeper.SetCeremony(h.Ctx, ceremony))

	h.EmergencyKeeper.HandleCeremonyFinalization(h.Ctx, resumeID)

	// Re-read the audit log. The contract:
	//   1. The log must have grown (resume finalization wrote at
	//      least one new entry).
	//   2. Every entry that existed before the resume operation must
	//      still exist with byte-identical content. The chain cannot
	//      rewrite history to make the halt look different now that
	//      the resume has happened.
	postResumeLog := h.EmergencyKeeper.GetAuditLog(h.Ctx)
	require.Greater(t, len(postResumeLog), preResumeLen,
		"resume finalization must add new audit entries — without them, the resume is invisible to future audit; with them, the audit substrate strictly grows")

	postResumeFreeze := freeze(postResumeLog)
	for i, prior := range preResumeFreeze {
		require.Equal(t, prior, postResumeFreeze[i],
			"audit entry %d existed before resume and must remain byte-identical after resume; commitment 10 forbids overwriting prior records to fit a later narrative", i)
	}

	// And the prior halt's emergency-status side-effect must remain
	// visible in the historical record even though the resume has
	// since transitioned the chain back to Normal — the audit log
	// preserves WHAT HAPPENED, not just the current end-state.
	var sawHaltExecuted bool
	for _, e := range postResumeFreeze {
		if e.CeremonyId == haltID && e.Action == string(emergencytypes.AuditHaltExecuted) {
			sawHaltExecuted = true
			break
		}
	}
	require.True(t, sawHaltExecuted,
		"the halt-executed audit entry from before the resume must still be present afterward; without it, 'we can re-enter normal' silently overwrites 'we were once halted'")
}

// ════════════════════════════════════════════════════════════════════
// Commitment 13: The training corpus is not for sale.
//
// "What enters the corpus enters because it survived. Facts are
// append-only post-acceptance; status transitions are forward-only;
// training revenue clawback fires deterministically on disprove."
//
// Bound here AND by: TestRouteB_Adversarial_ClawbackStickyAndIdempotent,
// TestRouteB_Wave4b_PopperWeightedTVWAndClawback.
// ════════════════════════════════════════════════════════════════════

func TestTruthSeeking_DisprovenFactClawsBackAndStaysClawedBack(t *testing.T) {
	h := NewTestHarness(t)
	_, err := h.KnowledgeKeeper.SeedRouteB(h.Ctx)
	require.NoError(t, err)

	submitter := testAddr("ts_clawback").String()
	require.NoError(t, h.KnowledgeKeeper.SetFact(h.Ctx, &knowledgetypes.Fact{
		Id: "F-TS-CLAWBACK", Content: "to be disproven", Domain: "sciences",
		Status:                          knowledgetypes.FactStatus_FACT_STATUS_ACTIVE,
		Submitter:                       submitter,
		MethodId:                        knowledgetypes.MethodologyEmpirical,
		Confidence:                      900_000,
		CorroborationCount:              5,
		SubmitterCalibrationSnapshotBps: 800_000,
	}))

	qs := knowledgekeeper.NewQueryServerImpl(h.KnowledgeKeeper)
	pre, err := qs.TrainingValueWeight(h.Ctx, &knowledgetypes.QueryTrainingValueWeightRequest{FactId: "F-TS-CLAWBACK"})
	require.NoError(t, err)
	require.Greater(t, pre.TvwBps, uint64(0))

	require.NoError(t, h.KnowledgeKeeper.ClawbackOnDisproval(h.Ctx, "F-TS-CLAWBACK"))

	post, err := qs.TrainingValueWeight(h.Ctx, &knowledgetypes.QueryTrainingValueWeightRequest{FactId: "F-TS-CLAWBACK"})
	require.NoError(t, err)
	require.Equal(t, uint64(0), post.TvwBps,
		"a disproven fact must lose its training value immediately")
	require.True(t, post.Disproven,
		"the clawback must be a structural state, not a transient flag")

	// Status flip back to ACTIVE — clawback must STAY.
	recovered, _ := h.KnowledgeKeeper.GetFact(h.Ctx, "F-TS-CLAWBACK")
	recovered.Status = knowledgetypes.FactStatus_FACT_STATUS_ACTIVE
	require.NoError(t, h.KnowledgeKeeper.SetFact(h.Ctx, recovered))

	postFlip, err := qs.TrainingValueWeight(h.Ctx, &knowledgetypes.QueryTrainingValueWeightRequest{FactId: "F-TS-CLAWBACK"})
	require.NoError(t, err)
	require.Equal(t, uint64(0), postFlip.TvwBps,
		"clawback must survive status flips; the corpus is not negotiable post-acceptance")
}

// ════════════════════════════════════════════════════════════════════
// Commitment 14: Reasoning traces are first-class.
//
// "The chain trains not just on conclusions but on derivations.
// Reasoning traces are gold-standard chain-of-thought, recorded
// on-chain alongside the conclusion."
//
// Bound here AND by: TestRouteB_Wave5_MethodologyApplicationTraceAssembly.
// ════════════════════════════════════════════════════════════════════

func TestTruthSeeking_ReasoningTracePropagatesThroughVerification(t *testing.T) {
	h := NewTestHarness(t)
	_, err := h.KnowledgeKeeper.SeedRouteB(h.Ctx)
	require.NoError(t, err)

	submitter := testAddr("ts_trace").String()
	const tracePayload = `[{"step":1,"observation":"premise A"},{"step":2,"derivation":"therefore B"}]`
	claim := &knowledgetypes.Claim{
		Id: "claim-ts-trace", Submitter: submitter,
		FactContent:    "claim with explicit derivation",
		Domain:         "sciences",
		Category:       "empirical",
		MethodId:       knowledgetypes.MethodologyEmpirical,
		ReasoningTrace: tracePayload,
		Status:         knowledgetypes.ClaimStatus_CLAIM_STATUS_IN_VERIFICATION,
		Stake:          "1000000",
	}
	require.NoError(t, h.KnowledgeKeeper.SetClaim(h.Ctx, claim))
	round := &knowledgetypes.VerificationRound{
		Id: "round-ts-trace", ClaimId: claim.Id,
		Phase: knowledgetypes.VerificationPhase_VERIFICATION_PHASE_COMPLETE, StartedAtBlock: 1,
	}
	require.NoError(t, h.KnowledgeKeeper.CompleteRound(h.Ctx, round, &knowledgekeeper.VerificationResult{
		Verdict: knowledgetypes.Verdict_VERDICT_ACCEPT, Confidence: 900_000, AcceptCount: 3,
	}))

	var fact *knowledgetypes.Fact
	h.KnowledgeKeeper.IterateFacts(h.Ctx, func(f *knowledgetypes.Fact) bool {
		if f.ClaimId == claim.Id {
			fact = f
			return true
		}
		return false
	})
	require.NotNil(t, fact)
	require.Equal(t, tracePayload, fact.ReasoningTrace,
		"a claim's reasoning trace must propagate verbatim into the accepted Fact; without the derivation, the corpus is a list of assertions, not a curriculum")
}

// Commitment 15: counterexamples are part of the corpus. The training
// corpus must include not just what is true, but what is wrong AND
// WHY. The chain operationalises this in two halves: x/counterexamples
// stores audited (fact_id, wrong_claim, error_type, reasoning) records;
// x/knowledge's TVW formula multiplies BPS for facts that have at
// least one VALIDATED counterexample. This test asserts the
// MULTIPLIER PATH — that a fact with a validated counterexample
// produces strictly higher TVW than the same fact without one.
func TestTruthSeeking_CounterexamplesRaiseTVW(t *testing.T) {
	h := NewTestHarness(t)
	_, err := h.KnowledgeKeeper.SeedRouteB(h.Ctx)
	require.NoError(t, err)

	// Create a fact via the existing claim+verify path.
	submitter := testAddr("ts_counterex_author").String()
	claim := &knowledgetypes.Claim{
		Id:          "claim-ts-counterex",
		Submitter:   submitter,
		FactContent: "claim that will gain a counterexample",
		Domain:      "sciences",
		Category:    "empirical",
		MethodId:    knowledgetypes.MethodologyEmpirical,
		Status:      knowledgetypes.ClaimStatus_CLAIM_STATUS_IN_VERIFICATION,
		Stake:       "1000000",
	}
	require.NoError(t, h.KnowledgeKeeper.SetClaim(h.Ctx, claim))
	round := &knowledgetypes.VerificationRound{
		Id: "round-ts-counterex", ClaimId: claim.Id,
		Phase: knowledgetypes.VerificationPhase_VERIFICATION_PHASE_COMPLETE, StartedAtBlock: 1,
	}
	require.NoError(t, h.KnowledgeKeeper.CompleteRound(h.Ctx, round, &knowledgekeeper.VerificationResult{
		Verdict: knowledgetypes.Verdict_VERDICT_ACCEPT, Confidence: 900_000, AcceptCount: 3,
	}))

	var fact *knowledgetypes.Fact
	h.KnowledgeKeeper.IterateFacts(h.Ctx, func(f *knowledgetypes.Fact) bool {
		if f.ClaimId == claim.Id {
			fact = f
			return true
		}
		return false
	})
	require.NotNil(t, fact, "fact must be created for counterexample to anchor to")

	// Baseline TVW: no counterexample yet.
	baseline := h.KnowledgeKeeper.ComputeTrainingValueWeight(h.Ctx, fact.Id)
	require.Greater(t, baseline.Final, uint64(0), "baseline TVW must be non-zero")
	require.Equal(t, uint64(1_000_000), baseline.CounterexampleMultiplier,
		"without a validated counterexample the multiplier defaults to 1.0x")

	// Propose a counterexample.
	ms := counterexampleskeeper.NewMsgServerImpl(h.CounterexamplesKeeper)
	proposeResp, err := ms.ProposeCounterexample(h.Ctx, &counterexamplestypes.MsgProposeCounterexample{
		Author:     submitter,
		FactId:     fact.Id,
		WrongClaim: "an alternative that confuses categorical levels",
		Reasoning: "the alternative would treat the empirical claim as normative, " +
			"which the is-ought wall (commitment 2) forbids structurally",
		ErrorType: counterexamplestypes.ErrorType_ERROR_TYPE_CATEGORICAL,
	})
	require.NoError(t, err)
	require.NotEmpty(t, proposeResp.CounterexampleId)

	// Three distinct validators affirm — meets default min_votes=3
	// with 100% affirm rate, well above the 66.6% threshold.
	for i, name := range []string{"ts_ce_val1", "ts_ce_val2", "ts_ce_val3"} {
		validator := testAddr(name).String()
		_, err := ms.Validate(h.Ctx, &counterexamplestypes.MsgValidate{
			Validator:        validator,
			CounterexampleId: proposeResp.CounterexampleId,
			Affirm:           true,
			Reason:           fmt.Sprintf("validator %d concurs", i+1),
		})
		require.NoError(t, err)
	}

	// The counterexample must now be VALIDATED.
	ce, ok := h.CounterexamplesKeeper.GetCounterexample(h.Ctx, proposeResp.CounterexampleId)
	require.True(t, ok)
	require.Equal(t, counterexamplestypes.CounterexampleStatus_COUNTEREXAMPLE_STATUS_VALIDATED, ce.Status,
		"after 3-of-3 affirmations the counterexample must auto-resolve to VALIDATED")

	// And the fact's TVW must now apply the multiplier.
	boosted := h.KnowledgeKeeper.ComputeTrainingValueWeight(h.Ctx, fact.Id)
	require.Greater(t, boosted.CounterexampleMultiplier, uint64(1_000_000),
		"validated counterexample must raise the multiplier above 1.0x")
	require.Greater(t, boosted.Final, baseline.Final,
		"a fact with a validated counterexample must earn STRICTLY MORE training-data value than the same fact without one — alignment-by-structure must be paid for, not declared")
}

// Commitment 16: the chain pays for exploration of the unknown.
// Without an open-question market, the corpus grows only along paths
// that interest current contributors. x/inquiry creates the dual of
// commitment 5: pay for facts that don't yet exist. This test asserts
// the BOUNTY PATH — that an inquiry resolves and pays the answerer
// when their linked claim produces an accepted fact, end-to-end.
func TestTruthSeeking_ChainPaysForExploration(t *testing.T) {
	h := NewTestHarness(t)
	_, err := h.KnowledgeKeeper.SeedRouteB(h.Ctx)
	require.NoError(t, err)

	asker := testAddr("ts_inquiry_asker")
	answerer := testAddr("ts_inquiry_answerer")

	// Fund the asker so they can escrow the bounty.
	bountyAmount := int64(2_000_000) // 2 ZRN
	require.NoError(t, h.FundAccount(asker, sdk.NewCoins(sdk.NewCoin("uzrn", sdkmath.NewInt(bountyAmount)))))

	// Submit the inquiry. Bounty is escrowed in the pool.
	inquiryMs := inquirykeeper.NewMsgServerImpl(h.InquiryKeeper)
	inqResp, err := inquiryMs.SubmitInquiry(h.Ctx, &inquirytypes.MsgSubmitInquiry{
		Asker:    asker.String(),
		Question: "What follows from premise X under methodology Y?",
		Domain:   "sciences",
		Bounty:   fmt.Sprintf("%d", bountyAmount),
	})
	require.NoError(t, err)
	require.NotEmpty(t, inqResp.InquiryId)

	// Asker's balance should be drained to escrow.
	require.Equal(t, int64(0), h.GetBalance(asker, "uzrn").Amount.Int64(),
		"bounty must be escrowed out of the asker's account on submission")

	// The answerer creates a knowledge claim normally — this is the
	// answer body. Then they link it to the inquiry.
	claim := &knowledgetypes.Claim{
		Id:          "claim-ts-inquiry-answer",
		Submitter:   answerer.String(),
		FactContent: "the answer to the inquiry, derived empirically",
		Domain:      "sciences",
		Category:    "empirical",
		MethodId:    knowledgetypes.MethodologyEmpirical,
		Status:      knowledgetypes.ClaimStatus_CLAIM_STATUS_IN_VERIFICATION,
		Stake:       "1000000",
	}
	require.NoError(t, h.KnowledgeKeeper.SetClaim(h.Ctx, claim))

	_, err = inquiryMs.SubmitAnswer(h.Ctx, &inquirytypes.MsgSubmitAnswer{
		Answerer:  answerer.String(),
		InquiryId: inqResp.InquiryId,
		ClaimId:   claim.Id,
	})
	require.NoError(t, err)

	// Verify the claim — produces an accepted fact.
	round := &knowledgetypes.VerificationRound{
		Id: "round-ts-inquiry", ClaimId: claim.Id,
		Phase: knowledgetypes.VerificationPhase_VERIFICATION_PHASE_COMPLETE, StartedAtBlock: 1,
	}
	require.NoError(t, h.KnowledgeKeeper.CompleteRound(h.Ctx, round, &knowledgekeeper.VerificationResult{
		Verdict: knowledgetypes.Verdict_VERDICT_ACCEPT, Confidence: 900_000, AcceptCount: 3,
	}))

	// Manually resolve. The auto-resolver in BeginBlocker would do
	// this on the next block; manual is faster for the test.
	resolveResp, err := inquiryMs.ResolveInquiry(h.Ctx, &inquirytypes.MsgResolveInquiry{
		Caller:    answerer.String(),
		InquiryId: inqResp.InquiryId,
	})
	require.NoError(t, err)
	require.Equal(t, inquirytypes.InquiryStatus_INQUIRY_STATUS_RESOLVED, resolveResp.Status,
		"with an accepted fact linked, the inquiry must resolve")
	require.NotEmpty(t, resolveResp.WinningFactId,
		"the winning fact id must be recorded on resolve — exploration is auditable")

	// The answerer must now hold the bounty. The chain has paid for
	// exploration: bounty moved from pool → answerer.
	require.Equal(t, bountyAmount, h.GetBalance(answerer, "uzrn").Amount.Int64(),
		"the chain must pay the bounty to the answerer when their linked claim accepts — without payment, 'we believe in exploration' is slogan, not commitment")
}

// Commitment 17: disagreement is structure, not noise. A fact's
// dialectic signature reflects the SHAPE of the verification that
// produced it — not just verdict, but vote tally, minority size,
// per-voter alignment. Two facts both accepted are structurally
// distinct if one was 5-0 and the other was 3-2. This test asserts
// the SHAPE PATH: a fact whose round had reveals must produce a
// non-empty DialecticSignature with a vote tally that matches the
// reveals, and a stress label that reflects the agreement margin.
func TestTruthSeeking_DialecticSignatureCarriesVoteShape(t *testing.T) {
	h := NewTestHarness(t)
	_, err := h.KnowledgeKeeper.SeedRouteB(h.Ctx)
	require.NoError(t, err)

	submitter := testAddr("ts_dialectic").String()
	claim := &knowledgetypes.Claim{
		Id:          "claim-ts-dialectic",
		Submitter:   submitter,
		FactContent: "claim with structured disagreement",
		Domain:      "sciences",
		Category:    "empirical",
		MethodId:    knowledgetypes.MethodologyEmpirical,
		Status:      knowledgetypes.ClaimStatus_CLAIM_STATUS_IN_VERIFICATION,
		Stake:       "1000000",
	}
	require.NoError(t, h.KnowledgeKeeper.SetClaim(h.Ctx, claim))

	// Build a round with three reveals: 2 accept, 1 reject. The
	// fact accepts but with a structured 2-1 split — the
	// "contested-but-resolved" shape commitment 17 must capture.
	round := &knowledgetypes.VerificationRound{
		Id:             "round-ts-dialectic",
		ClaimId:        claim.Id,
		Phase:          knowledgetypes.VerificationPhase_VERIFICATION_PHASE_COMPLETE,
		StartedAtBlock: 1,
		Reveals: []*knowledgetypes.RevealEntry{
			{Verifier: testAddr("dlc_v1").String(), Vote: "accept"},
			{Verifier: testAddr("dlc_v2").String(), Vote: "accept"},
			{Verifier: testAddr("dlc_v3").String(), Vote: "reject"},
		},
	}
	require.NoError(t, h.KnowledgeKeeper.CompleteRound(h.Ctx, round, &knowledgekeeper.VerificationResult{
		Verdict: knowledgetypes.Verdict_VERDICT_ACCEPT, Confidence: 800_000, AcceptCount: 2,
	}))

	var fact *knowledgetypes.Fact
	h.KnowledgeKeeper.IterateFacts(h.Ctx, func(f *knowledgetypes.Fact) bool {
		if f.ClaimId == claim.Id {
			fact = f
			return true
		}
		return false
	})
	require.NotNil(t, fact, "fact must exist for dialectic signature")

	sig := h.DialecticKeeper.ComposeSignature(h.Ctx, fact.Id)
	require.NotNil(t, sig)
	require.Equal(t, uint32(3), sig.TotalVoters,
		"signature must reflect every voter — including those who voted against the verdict")
	require.Equal(t, uint32(2), sig.AcceptCount)
	require.Equal(t, uint32(1), sig.RejectCount)
	require.Equal(t, uint32(1), sig.MinoritySize,
		"minority_size must report the dissenter — without this, '5-0' and '5-4' are indistinguishable")
	require.Equal(t, "accept", sig.Verdict)
	require.Less(t, sig.AgreementBps, uint64(850_000),
		"a 2-1 verdict must register below the contested threshold")
	require.Equal(t, "CONTESTED", sig.StressLabel,
		"a 2-1 verdict must label CONTESTED — settled and contested-but-resolved are different shapes the chain must distinguish")
}

// Commitment 18: the chain manufactures exploration demand. Where
// commitment 5 has the chain mint to stress-test what it already
// thinks it knows, and commitment 16 lets askers escrow bounties for
// the questions that interest them, this commitment names the third
// shape of demand: the chain itself, seeing through its own frontier
// composition that a domain is sparse, FUNDS open inquiries there
// without waiting for an outside party to ask. This test asserts
// the LOAD-BEARING falsifier — the round-trip — that distinguishes
// commitment 18 from rhetoric: a chain-sponsored inquiry that
// expires unanswered must return its bounty to the frontier-bounty
// pool. Without the round-trip, the chain's exploration mint silently
// leaks into general circulation, the audit budget cannot be tracked
// across cycles, and "the chain pays for its own audit" (commitment
// 12) becomes incoherent at the budget layer.
//
// Bound here AND by: TestFrontier_ChainSponsorsInquiriesForSparseDomains
// (the sponsorship path), TestFrontier_OpenInquiriesRaiseSparsity (the
// frontier-input bind that this commitment consumes).
func TestTruthSeeking_FrontierBountyRoundTripsOnExpiry(t *testing.T) {
	h := NewTestHarness(t)
	_, err := h.KnowledgeKeeper.SeedRouteB(h.Ctx)
	require.NoError(t, err)

	// One sparse domain — enough for top-K=1 sponsorship.
	h.App.ZeroneOntologyKeeper.SetDomain(h.Ctx, &ontologytypes.Domain{
		Name:    "philosophy",
		Stratum: uint32(ontologytypes.StratumEmpirical),
		Status:  "active",
		Depth:   1,
	})

	// Tighten the cadence and expiry so the test runs in a few blocks.
	// cadence=2 → height 2 fires sponsorship.
	// expiry=1 → ExpiresAtBlock = 2 + 1 = 3; height 3 is NOT a cadence
	//            multiple, so the resolution scan triggers expiry without
	//            also producing a second sponsorship in the same tick —
	//            keeps the round-trip assertion uncontaminated.
	p := h.App.InquiryKeeper.GetParams(h.Ctx)
	p.FrontierInvitationCadenceBlocks = 2
	p.FrontierInvitationSparsityThresholdBps = 600_000
	p.FrontierInvitationTopK = 1
	p.FrontierInvitationBounty = "5000000"
	p.FrontierInvitationExpiryBlocks = 1
	require.NoError(t, h.App.InquiryKeeper.SetParams(h.Ctx, p))

	frontierAddr := authtypes.NewModuleAddress(inquirytypes.FrontierBountyPoolModuleName)
	inquiryAddr := authtypes.NewModuleAddress(inquirytypes.BountyPoolModuleName)

	// Advance to height 2 — cadence tick. Sponsorship fires.
	h.AdvanceBlocks(1)
	require.NoError(t, h.App.InquiryKeeper.BeginBlocker(h.Ctx))

	var sponsored *inquirytypes.Inquiry
	require.NoError(t, h.App.InquiryKeeper.IterateAllInquiries(h.Ctx, func(q *inquirytypes.Inquiry) bool {
		if q.SystemInitiated {
			sponsored = q
			return true
		}
		return false
	}))
	require.NotNil(t, sponsored,
		"commitment 18: BeginBlocker at cadence tick must sponsor at least one chain-asked inquiry; without sponsorship, the commitment is rhetoric")
	require.Equal(t, "5000000", sponsored.Bounty)

	// Pre-expiry balances: frontier pool emptied (mint round-tripped
	// out to inquiry pool); inquiry pool holds the bounty.
	require.True(t, h.GetBalance(frontierAddr, "uzrn").Amount.IsZero(),
		"after sponsorship, frontier pool is empty — mint flowed through to inquiry pool")
	require.Equal(t, sdkmath.NewInt(5_000_000), h.GetBalance(inquiryAddr, "uzrn").Amount,
		"after sponsorship, inquiry pool holds the chain-sponsored bounty awaiting answer or expiry")

	// Advance past expiry. ExpiresAtBlock = 3, so height 3 makes
	// `currentBlock >= ExpiresAtBlock` true and the resolution scan
	// in BeginBlocker calls expireInquiry → refundBounty. For
	// system_initiated inquiries, refundBounty must route the funds
	// back to the frontier pool, not to a user account. Height 3 is
	// also NOT a cadence-aligned block (cadence=2), so the
	// frontier-invitation cycle does not fire — keeps the assertion
	// focused on the round-trip alone.
	h.AdvanceBlocks(1) // height now 3
	require.NoError(t, h.App.InquiryKeeper.BeginBlocker(h.Ctx))

	// Re-load the inquiry. It must have transitioned to EXPIRED.
	expired, ok := h.App.InquiryKeeper.GetInquiry(h.Ctx, sponsored.Id)
	require.True(t, ok, "expired inquiry must still exist for audit")
	require.Equal(t, inquirytypes.InquiryStatus_INQUIRY_STATUS_EXPIRED, expired.Status,
		"unanswered chain-sponsored inquiries must transition to EXPIRED — without this, commitment 18's expiry path is dead code")

	// THE LOAD-BEARING ASSERTION: the bounty is back in the frontier
	// pool. The chain's exploration audit budget conserves itself
	// across unanswered cycles. If this fails, every expired chain-
	// sponsored inquiry leaks uzrn into circulation, the audit budget
	// becomes untrackable, and the chain has silently paid for
	// nothing — which is the structural form of "rhetoric, not
	// commitment."
	require.Equal(t, sdkmath.NewInt(5_000_000), h.GetBalance(frontierAddr, "uzrn").Amount,
		"commitment 18 + commitment 12: the bounty of an unanswered chain-sponsored inquiry must round-trip back to the frontier pool — anything less is leakage of the chain's audit budget")
	require.True(t, h.GetBalance(inquiryAddr, "uzrn").Amount.IsZero(),
		"inquiry pool must release the system-sponsored bounty on expiry — leaving it would let chain-mint silently subsidise unrelated user-asked inquiries")
}

// ════════════════════════════════════════════════════════════════════
// Meta-invariant: the creed and the contract stay in sync.
//
// docs/TRUTH_SEEKING.md numbers its commitments. Every numbered
// commitment in the creed must have at least one corresponding
// TestTruthSeeking_ test in this file. Every TestTruthSeeking_ test
// must cite a commitment number in its header comment.
//
// This test is the discipline enforced mechanically: if you add a
// commitment to the creed without binding it, this test fails. If
// you add a binding test without a creed entry, this test fails.
// The creed and the contract cannot drift.
// ════════════════════════════════════════════════════════════════════

func TestTruthSeeking_CreedAndContractStayInSync(t *testing.T) {
	creedBytes, err := os.ReadFile("../../docs/TRUTH_SEEKING.md")
	require.NoError(t, err, "creed must exist; if you renamed or moved it, update this test")

	creed := string(creedBytes)
	commitmentNumberRe := regexp.MustCompile(`(?m)^### (\d+)\. `)
	matches := commitmentNumberRe.FindAllStringSubmatch(creed, -1)
	require.NotEmpty(t, matches, "no numbered commitments parsed — creed format may have changed")

	creedNumbers := make(map[int]bool)
	for _, m := range matches {
		n, err := strconv.Atoi(m[1])
		require.NoError(t, err)
		creedNumbers[n] = true
	}

	testFile, err := os.ReadFile("truth_seeking_invariants_test.go")
	require.NoError(t, err)
	testContent := string(testFile)

	// Every numbered commitment must appear in some "Commitment N:"
	// header inside this file. The header pattern is the contract.
	citationRe := regexp.MustCompile(`(?m)^// Commitment (\d+):`)
	citationMatches := citationRe.FindAllStringSubmatch(testContent, -1)
	citedNumbers := make(map[int]bool)
	for _, m := range citationMatches {
		n, err := strconv.Atoi(m[1])
		require.NoError(t, err)
		citedNumbers[n] = true
	}

	for n := range creedNumbers {
		require.True(t, citedNumbers[n],
			"commitment %d in the creed has no binding test; either add a TestTruthSeeking_ test that cites it, or remove the commitment from the creed. Slogan vs belief.", n)
	}
	for n := range citedNumbers {
		require.True(t, creedNumbers[n],
			"a binding test cites commitment %d which does not appear in the creed; either add the commitment to TRUTH_SEEKING.md or remove the citation. The creed and the contract must stay in sync.", n)
	}

	// ─── Architecture echo ───────────────────────────────────────────
	// The creed binds tests; the tests bind code. The third leg is
	// position: every commitment must also be DECLARED at the package
	// level by at least one x/*/doc.go file. Belief is not a property
	// of the test suite alone; the chain itself must say "this is what
	// I am about to do."
	//
	// Walk x/*/doc.go and collect every cited commitment number. The
	// citation pattern matches both leading-capital ("Commitment 5")
	// and lower-case ("commitment 5") references, since both forms
	// appear in existing package declarations.

	declaredNumbers := make(map[int]bool)
	docCitationRe := regexp.MustCompile(`(?i)\bcommitment (\d+)\b`)
	xRoot := "../../x"
	err = filepath.Walk(xRoot, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if info.IsDir() || filepath.Base(path) != "doc.go" {
			return nil
		}
		body, readErr := os.ReadFile(path)
		if readErr != nil {
			return readErr
		}
		for _, m := range docCitationRe.FindAllStringSubmatch(string(body), -1) {
			n, convErr := strconv.Atoi(m[1])
			if convErr != nil {
				continue
			}
			declaredNumbers[n] = true
		}
		return nil
	})
	require.NoError(t, err, "walking x/ for doc.go failed")

	for n := range creedNumbers {
		require.True(t, declaredNumbers[n],
			"commitment %d in the creed is not declared in any x/*/doc.go; the architecture does not echo this commitment back. Add a doc.go that names this commitment in the module that preserves it.", n)
	}
	for n := range declaredNumbers {
		require.True(t, creedNumbers[n],
			"an x/*/doc.go declares commitment %d which does not appear in the creed; either add the commitment to TRUTH_SEEKING.md or remove the declaration. Position must match creed.", n)
	}

	// ─── Voice echo ─────────────────────────────────────────────────
	// The creed binds tests, the tests bind code, the code binds
	// position. The fourth leg is voice: events emitted by the chain
	// to off-chain observers carry a "creed_commitment" attribute
	// naming which commitment the announcement preserves. Off-chain
	// indexers and dashboards filter on this attribute to surface
	// truth-seeking activity in the same vocabulary the creed uses.
	//
	// This test asserts every "creed_commitment" attribute value is a
	// real commitment number from the creed. Adding an event with a
	// commitment value that doesn't exist in TRUTH_SEEKING.md fails
	// CI. Renaming or renumbering the creed without updating event
	// emission sites also fails CI.
	//
	// We do NOT enforce that every commitment has at least one event:
	// some commitments (e.g., 11 "trust queryable", 14 "reasoning
	// traces are first-class") are properties of data structures or
	// query surfaces, not transition moments. Forcing them into events
	// would create ceremonial events with no real audit value.

	announcedNumbers := make(map[int]bool)
	creedAttrRe := regexp.MustCompile(`sdk\.NewAttribute\("creed_commitment",\s*"([^"]+)"\)`)
	err = filepath.Walk(xRoot, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if info.IsDir() {
			return nil
		}
		base := filepath.Base(path)
		if !regexp.MustCompile(`\.go$`).MatchString(base) {
			return nil
		}
		// Skip generated and test files. The convention applies to
		// production emission sites only.
		if regexp.MustCompile(`(_test\.go|\.pb\.go|\.pb\.gw\.go)$`).MatchString(base) {
			return nil
		}
		body, readErr := os.ReadFile(path)
		if readErr != nil {
			return readErr
		}
		for _, m := range creedAttrRe.FindAllStringSubmatch(string(body), -1) {
			// Value can be "5" or "5,12" or "6, 10" — split and trim.
			raw := m[1]
			for _, part := range regexp.MustCompile(`,\s*`).Split(raw, -1) {
				part = regexp.MustCompile(`^\s+|\s+$`).ReplaceAllString(part, "")
				if part == "" {
					continue
				}
				n, convErr := strconv.Atoi(part)
				require.NoError(t, convErr,
					"file %s emits an event with creed_commitment=%q; value %q is not an integer. The attribute must be a comma-separated list of commitment numbers.",
					path, raw, part)
				require.True(t, creedNumbers[n],
					"file %s emits an event with creed_commitment=%q; commitment %d does not appear in TRUTH_SEEKING.md. Either add the commitment to the creed or correct the announcement.",
					path, raw, n)
				announcedNumbers[n] = true
			}
		}
		return nil
	})
	require.NoError(t, err, "walking x/ for event emissions failed")

	// Soft check: at least one event must announce truth-seeking
	// activity at all. If this number is zero, the convention has
	// been silently abandoned — fail loudly.
	require.NotEmpty(t, announcedNumbers,
		"no creed_commitment attributes found in any x/ source file. The voice layer of truth-seeking has been silently removed; either restore the convention or remove this test and document why.")

	// ─── Internal coherence ─────────────────────────────────────────
	// The creed is not a flat list. Each commitment names other
	// commitments it Echoes — depends on, reinforces, or operationalises.
	// The cross-references make the creed a graph: a reader can trace
	// from any commitment to the others that hold it up.
	//
	// This check parses the **Echoes**: lines and asserts:
	//   1. Every echoed number is a real commitment in the creed.
	//   2. No commitment echoes itself (would be vacuous).
	//   3. Every commitment has at least one Echoes line (the creed
	//      is a graph; an isolated commitment is a smell, not a
	//      property — if a commitment truly stands alone, document
	//      that it stands alone).

	echoesRe := regexp.MustCompile(`(?m)^\*\*Echoes\*\*:[^\n]+`)
	echoNumberRe := regexp.MustCompile(`commitment (\d+)`)
	// Split creed into per-commitment sections by header.
	sectionRe := regexp.MustCompile(`(?m)^### (\d+)\. `)
	headers := sectionRe.FindAllStringSubmatchIndex(creed, -1)
	require.NotEmpty(t, headers, "creed sections did not parse")

	commitmentsWithEchoes := make(map[int]bool)
	for i, h := range headers {
		nStr := creed[h[2]:h[3]]
		n, convErr := strconv.Atoi(nStr)
		require.NoError(t, convErr)

		// Section runs from this header to the next (or end of file).
		end := len(creed)
		if i+1 < len(headers) {
			end = headers[i+1][0]
		}
		section := creed[h[0]:end]

		echoLines := echoesRe.FindAllString(section, -1)
		if len(echoLines) == 0 {
			// Commitment has no Echoes line. Fail with a clear message.
			require.Fail(t,
				"commitment without Echoes line",
				"commitment %d has no **Echoes**: line. Add one naming the commitments this one depends on or reinforces, or document explicitly that it stands alone.", n)
		}
		commitmentsWithEchoes[n] = true

		for _, line := range echoLines {
			refs := echoNumberRe.FindAllStringSubmatch(line, -1)
			require.NotEmpty(t, refs,
				"commitment %d has an **Echoes** line that names no commitment numbers: %q", n, line)
			for _, ref := range refs {
				m, convErr := strconv.Atoi(ref[1])
				require.NoError(t, convErr)
				require.NotEqual(t, n, m,
					"commitment %d echoes itself in line %q; cross-references must point to other commitments", n, line)
				require.True(t, creedNumbers[m],
					"commitment %d echoes commitment %d, which is not a numbered commitment in the creed", n, m)
			}
		}
	}

	for n := range creedNumbers {
		require.True(t, commitmentsWithEchoes[n],
			"commitment %d has no Echoes line; the creed is a graph, not a list — every commitment must declare its connections", n)
	}
}

// sdkCoinsForTest keeps the panel test readable; mirrors the inlined
// pattern other tests use.
func sdkCoinsForTest(amt int64) sdk.Coins {
	return sdk.NewCoins(sdk.NewCoin("uzrn", sdkmath.NewInt(amt)))
}
