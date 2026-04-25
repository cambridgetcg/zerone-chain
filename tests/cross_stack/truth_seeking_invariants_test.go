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
	"testing"

	sdkmath "cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/stretchr/testify/require"

	knowledgekeeper "github.com/zerone-chain/zerone/x/knowledge/keeper"
	knowledgetypes "github.com/zerone-chain/zerone/x/knowledge/types"
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

// sdkCoinsForTest keeps the panel test readable; mirrors the inlined
// pattern other tests use.
func sdkCoinsForTest(amt int64) sdk.Coins {
	return sdk.NewCoins(sdk.NewCoin("uzrn", sdkmath.NewInt(amt)))
}
