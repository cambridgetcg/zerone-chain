package cross_stack_test

import (
	"testing"

	sdkmath "cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/stretchr/testify/require"

	knowledgekeeper "github.com/zerone-chain/zerone/x/knowledge/keeper"
	knowledgetypes "github.com/zerone-chain/zerone/x/knowledge/types"
)

// Moat-integrity tests. Each pins a gate that protects the "chain-attested
// model is the most trustworthy model" thesis. Failure of any one of these
// means an unverified input can reach the training substrate without going
// through the full trust chain.

// ConfidenceThreshold is the gate deciding whether a verification round
// accepts a claim. Governance setting it to zero would accept every claim
// regardless of verifier consensus — a single-parameter bypass of the
// entire verifier panel. Params.Validate must reject it.
func TestMoat_ConfidenceThresholdFloor(t *testing.T) {
	p := knowledgetypes.DefaultParams()
	require.NoError(t, p.Validate())

	p.ConfidenceThreshold = 0
	err := p.Validate()
	require.Error(t, err)
	require.Contains(t, err.Error(), "confidence_threshold must be > 0")

	// QuorumThreshold floor is the sibling gate — a single verifier
	// could otherwise carry a round. Guard both together.
	p = knowledgetypes.DefaultParams()
	p.QuorumThreshold = 0
	err = p.Validate()
	require.Error(t, err)
	require.Contains(t, err.Error(), "quorum_threshold must be > 0")
}

// A fact in PENDING, PROVISIONAL, AT_RISK, REVOKED, EXPIRED, PRUNED, or
// SUPERSEDED status must earn zero TVW. These are the statuses that either
// never passed verification or have been invalidated; paying them would
// mean the attribution-revenue pipeline rewards unverified training data.
func TestMoat_TVWStatusGate(t *testing.T) {
	h := NewTestHarness(t)
	_, err := h.KnowledgeKeeper.SeedRouteB(h.Ctx)
	require.NoError(t, err)

	qs := knowledgekeeper.NewQueryServerImpl(h.KnowledgeKeeper)
	submitter := testAddr("moat_tvw_sub").String()

	base := func(id string, status knowledgetypes.FactStatus) *knowledgetypes.Fact {
		return &knowledgetypes.Fact{
			Id: id, Content: "moat test", Domain: "sciences",
			Status:                          status,
			Submitter:                       submitter,
			MethodId:                        knowledgetypes.MethodologyEmpirical,
			Confidence:                      900_000,
			CorroborationCount:              3,
			SubmitterCalibrationSnapshotBps: 800_000,
			AxiomDistance:                   2,
		}
	}

	eligible := []knowledgetypes.FactStatus{
		knowledgetypes.FactStatus_FACT_STATUS_VERIFIED,
		knowledgetypes.FactStatus_FACT_STATUS_ACTIVE,
		knowledgetypes.FactStatus_FACT_STATUS_CONTESTED,
		knowledgetypes.FactStatus_FACT_STATUS_CHALLENGED,
	}
	ineligible := []knowledgetypes.FactStatus{
		knowledgetypes.FactStatus_FACT_STATUS_UNSPECIFIED,
		knowledgetypes.FactStatus_FACT_STATUS_PENDING,
		knowledgetypes.FactStatus_FACT_STATUS_PROVISIONAL,
		knowledgetypes.FactStatus_FACT_STATUS_SUPERSEDED,
		knowledgetypes.FactStatus_FACT_STATUS_EXPIRED,
		knowledgetypes.FactStatus_FACT_STATUS_REVOKED,
		knowledgetypes.FactStatus_FACT_STATUS_AT_RISK,
		knowledgetypes.FactStatus_FACT_STATUS_PRUNED,
	}

	for _, s := range eligible {
		id := "F-OK-" + s.String()
		require.NoError(t, h.KnowledgeKeeper.SetFact(h.Ctx, base(id, s)))
		resp, err := qs.TrainingValueWeight(h.Ctx, &knowledgetypes.QueryTrainingValueWeightRequest{FactId: id})
		require.NoError(t, err)
		require.Greater(t, resp.TvwBps, uint64(0),
			"status %s must earn positive TVW", s.String())
		require.False(t, resp.StatusIneligible,
			"status %s must not be flagged ineligible", s.String())
	}

	for _, s := range ineligible {
		id := "F-NOPE-" + s.String()
		require.NoError(t, h.KnowledgeKeeper.SetFact(h.Ctx, base(id, s)))
		resp, err := qs.TrainingValueWeight(h.Ctx, &knowledgetypes.QueryTrainingValueWeightRequest{FactId: id})
		require.NoError(t, err)
		require.Equal(t, uint64(0), resp.TvwBps,
			"status %s must earn zero TVW", s.String())
		require.True(t, resp.StatusIneligible,
			"status %s must be flagged ineligible", s.String())
	}
}

// Self-reported ModelCard eval metrics are BPS-scaled in [0, 1_000_000].
// A value above 1M is nonsensical; a downstream trust-dashboard consuming
// it would render malformed model comparisons. Handler must bound.
func TestMoat_ModelCardEvalRangeBound(t *testing.T) {
	h := NewTestHarness(t)
	_, err := h.KnowledgeKeeper.SeedRouteB(h.Ctx)
	require.NoError(t, err)

	ms := knowledgekeeper.NewMsgServerImpl(h.KnowledgeKeeper)
	owner := testAddr("moat_eval_owner").String()
	require.NoError(t, h.KnowledgeKeeper.SetTrainingPipeline(h.Ctx, &knowledgetypes.TrainingPipeline{
		Id: "pipe-moat-eval", OperatorAddress: owner, TokenizerVersion: 1,
	}))

	_, err = ms.RegisterModelCard(h.Ctx, &knowledgetypes.MsgRegisterModelCard{
		Owner: owner, Id: "m-moat-bad-eval", PipelineId: "pipe-moat-eval",
		Route: "from_scratch", EvalAcceptanceRateBps: 2_000_000,
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "eval_acceptance_rate_bps")

	_, err = ms.RegisterModelCard(h.Ctx, &knowledgetypes.MsgRegisterModelCard{
		Owner: owner, Id: "m-moat-bad-corr", PipelineId: "pipe-moat-eval",
		Route: "from_scratch", EvalCorroborationRateBps: 9_999_999,
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "eval_corroboration_rate_bps")

	// Register a legitimate card first to test the update path.
	_, err = ms.RegisterModelCard(h.Ctx, &knowledgetypes.MsgRegisterModelCard{
		Owner: owner, Id: "m-moat-good", PipelineId: "pipe-moat-eval",
		Route: "from_scratch", EvalAcceptanceRateBps: 500_000,
	})
	require.NoError(t, err)

	_, err = ms.UpdateModelCard(h.Ctx, &knowledgetypes.MsgUpdateModelCard{
		Owner: owner, Id: "m-moat-good",
		EvalAcceptanceRateBps: 1_500_000,
	})
	require.Error(t, err, "out-of-range update must also reject")
}

// SubmitterCalibrationSnapshotBps must be frozen at fact acceptance so
// a submitter cannot boost calibration after the fact is live and
// retroactively harvest more training value. The snapshot is the Popper-
// weighting gate that makes TVW non-gameable; if future calibration
// drift leaks into this fact's TVW, the gate is open.
func TestMoat_CalibrationSnapshotFrozenAtAcceptance(t *testing.T) {
	h := NewTestHarness(t)
	_, err := h.KnowledgeKeeper.SeedRouteB(h.Ctx)
	require.NoError(t, err)

	submitter := testAddr("moat_cal_sub").String()
	require.NoError(t, h.KnowledgeKeeper.SetAgentCalibration(h.Ctx, &knowledgetypes.AgentCalibration{
		Address: submitter, CalibrationScoreBps: 400_000,
		Accepted: 4, TotalSubmissions: 10,
	}))

	claim := &knowledgetypes.Claim{
		Id:          "claim-moat-cal",
		Submitter:   submitter,
		FactContent: "moat calibration freeze subject — a verified empirical finding",
		Domain:      "sciences",
		Category:    "empirical",
		Status:      knowledgetypes.ClaimStatus_CLAIM_STATUS_IN_VERIFICATION,
		Stake:       "1000000",
	}
	require.NoError(t, h.KnowledgeKeeper.SetClaim(h.Ctx, claim))
	round := &knowledgetypes.VerificationRound{
		Id:             "round-moat-cal",
		ClaimId:        claim.Id,
		Phase:          knowledgetypes.VerificationPhase_VERIFICATION_PHASE_COMPLETE,
		StartedAtBlock: 1,
	}
	result := &knowledgekeeper.VerificationResult{
		Verdict: knowledgetypes.Verdict_VERDICT_ACCEPT, Confidence: 900_000, AcceptCount: 3,
	}
	require.NoError(t, h.KnowledgeKeeper.CompleteRound(h.Ctx, round, result))

	var fact *knowledgetypes.Fact
	h.KnowledgeKeeper.IterateFacts(h.Ctx, func(f *knowledgetypes.Fact) bool {
		if f.ClaimId == claim.Id {
			fact = f
			return true
		}
		return false
	})
	require.NotNil(t, fact, "fact must be created from accepted claim")
	require.Greater(t, fact.SubmitterCalibrationSnapshotBps, uint64(0),
		"snapshot must be populated at acceptance — never left zero")

	frozen := fact.SubmitterCalibrationSnapshotBps

	// Submitter later boosts calibration to the ceiling via unrelated activity.
	require.NoError(t, h.KnowledgeKeeper.SetAgentCalibration(h.Ctx, &knowledgetypes.AgentCalibration{
		Address: submitter, CalibrationScoreBps: 1_000_000,
		Accepted: 40, TotalSubmissions: 40,
	}))

	// The previously-accepted fact's TVW must still use the frozen snapshot,
	// not the boosted current score — no retroactive farming.
	qs := knowledgekeeper.NewQueryServerImpl(h.KnowledgeKeeper)
	resp, err := qs.TrainingValueWeight(h.Ctx, &knowledgetypes.QueryTrainingValueWeightRequest{FactId: fact.Id})
	require.NoError(t, err)
	require.Equal(t, frozen, resp.SubmitterCalibrationBps,
		"TVW must read the frozen snapshot, not the current calibration")
	require.NotEqual(t, uint64(1_000_000), resp.SubmitterCalibrationBps,
		"TVW must NOT reflect the submitter's post-acceptance calibration boost")
}

// Successful challenges must refund the challenger's stake remainder
// after the verifier pool; failed challenges must route the remainder
// to protocol treasury (not orphan it in the knowledge module). Without
// this, legitimate falsification is economically irrational (lose 100%
// either way) and truth-discovery is disincentivized — the core signal
// the moat depends on.
func TestMoat_ChallengeStakeSettled(t *testing.T) {
	h := NewTestHarness(t)
	_, err := h.KnowledgeKeeper.SeedRouteB(h.Ctx)
	require.NoError(t, err)

	// Successful-challenge path: ACCEPT verdict on a challenge claim must
	// refund the remainder to the challenger. (The verifier pool has
	// already been deducted via distributeVerifierRewardsFromPool.)
	challenger := testAddr("moat_chal_ok")
	challengerStr := challenger.String()
	victimFact := &knowledgetypes.Fact{
		Id: "F-MOAT-VICTIM", Content: "a bad fact that will be disproven",
		Domain: "sciences", Category: "empirical",
		Status: knowledgetypes.FactStatus_FACT_STATUS_VERIFIED,
		Submitter: testAddr("moat_chal_victim_sub").String(),
		MethodId: knowledgetypes.MethodologyEmpirical,
		Confidence: 800_000,
	}
	require.NoError(t, h.KnowledgeKeeper.SetFact(h.Ctx, victimFact))

	// Fund the challenger, then route the challenge through MsgChallengeFact
	// so the stake is locked exactly as production does.
	require.NoError(t, h.FundAccount(challenger, sdk.NewCoins(sdk.NewCoin("uzrn", sdkmath.NewInt(50_000_000)))))
	ms := knowledgekeeper.NewMsgServerImpl(h.KnowledgeKeeper)
	// Challenge stake is risk-scaled by target-fact confidence; 20M clears
	// the 19.8M effective minimum at confidence 800k.
	chalResp, err := ms.ChallengeFact(h.Ctx, &knowledgetypes.MsgChallengeFact{
		Challenger: challengerStr, FactId: victimFact.Id,
		Stake: "20000000", Reason: "moat test — challenge stakes must settle",
	})
	require.NoError(t, err)

	balAfterLock := h.GetBalance(challenger, "uzrn")
	require.Equal(t, sdkmath.NewInt(30_000_000), balAfterLock.Amount,
		"50M funded − 20M locked = 30M remaining")

	// Force the challenge round to complete with ACCEPT verdict (challenge
	// succeeds, victimFact is disproven).
	round, ok := h.KnowledgeKeeper.GetVerificationRound(h.Ctx, chalResp.RoundId)
	require.True(t, ok)
	round.Phase = knowledgetypes.VerificationPhase_VERIFICATION_PHASE_COMPLETE
	result := &knowledgekeeper.VerificationResult{
		Verdict: knowledgetypes.Verdict_VERDICT_ACCEPT, Confidence: 900_000, AcceptCount: 3,
	}
	require.NoError(t, h.KnowledgeKeeper.CompleteRound(h.Ctx, round, result))

	// After settlement the challenger should have at least the 45% remainder
	// back (verifier pool already took the other 55%). Exact amount depends
	// on whether the protocol treasury has the bonus funds, but the
	// remainder must show up.
	balAfterSettle := h.GetBalance(challenger, "uzrn")
	require.True(t, balAfterSettle.Amount.GT(balAfterLock.Amount),
		"successful challenger must receive the stake remainder refund")
}

// Popperian antifragility: high-confidence facts must be CHEAPER to
// probe than low-confidence facts. Truth stands firm under challenge
// because of its nature — the substrate invites stress-testing of the
// claims we trust most rather than taxing it. If this invariant breaks,
// the architecture has reverted to "protect consensus" and the moat's
// core epistemology is compromised.
func TestMoat_HighConfidenceFactsCheaperToProbe(t *testing.T) {
	h := NewTestHarness(t)
	_, err := h.KnowledgeKeeper.SeedRouteB(h.Ctx)
	require.NoError(t, err)

	params, err := h.KnowledgeKeeper.GetParams(h.Ctx)
	require.NoError(t, err)

	stakeAt := func(conf uint64) uint64 {
		return knowledgekeeper.EffectiveMinChallengeStake(params, conf).Uint64()
	}
	lowConfStake := stakeAt(0)
	midConfStake := stakeAt(500_000)
	highConfStake := stakeAt(900_000)
	maxConfStake := stakeAt(1_000_000)

	require.Greater(t, lowConfStake, midConfStake,
		"mid-confidence facts must be cheaper to probe than unproven ones")
	require.Greater(t, midConfStake, highConfStake,
		"high-confidence facts must be cheaper to probe than mid-confidence ones")
	require.GreaterOrEqual(t, highConfStake, maxConfStake,
		"max-confidence facts reach the floor; still non-zero to deter pure spam")

	// Floor guarantees some minimum probe cost — axioms are not free to
	// challenge, only invitingly cheap.
	require.Greater(t, maxConfStake, uint64(0),
		"ChallengeStakeFloorBps must keep even axiom-level facts costly enough to deter spam")
}

// Successful-challenge reward amplifies with the disproven fact's
// confidence. Disproving a 90%-confidence claim is a paradigm shift;
// disproving a 10%-confidence claim is routine cleanup. The chain's
// reward schedule must mirror that asymmetry — the signal worth paying
// for is the one the community didn't see coming.
func TestMoat_SuccessfulChallengeRewardScalesWithTargetConfidence(t *testing.T) {
	h := NewTestHarness(t)
	_, err := h.KnowledgeKeeper.SeedRouteB(h.Ctx)
	require.NoError(t, err)

	// Mint directly into the protocol treasury so the bonus amplification
	// path has funds to draw from. Without this the bonus silently
	// skips (the code logs and continues) and the two probes would
	// return identical refunds, defeating the test.
	treasuryFeed := sdk.NewCoins(sdk.NewCoin("uzrn", sdkmath.NewInt(1_000_000_000)))
	require.NoError(t, h.App.BankKeeper.MintCoins(h.Ctx, "zerone_auth", treasuryFeed))
	require.NoError(t, h.App.BankKeeper.SendCoinsFromModuleToModule(h.Ctx,
		"zerone_auth", "protocol_treasury", treasuryFeed))

	probe := func(targetConf uint64, challengerTag string) sdkmath.Int {
		challenger := testAddr(challengerTag)
		require.NoError(t, h.FundAccount(challenger,
			sdk.NewCoins(sdk.NewCoin("uzrn", sdkmath.NewInt(50_000_000)))))
		submitter := testAddr(challengerTag + "_sub").String()

		victim := &knowledgetypes.Fact{
			Id:         "F-MOAT-SCALED-" + challengerTag,
			Content:    "a fact that will be disproven",
			Domain:     "sciences",
			Category:   "empirical",
			Status:     knowledgetypes.FactStatus_FACT_STATUS_VERIFIED,
			Submitter:  submitter,
			MethodId:   knowledgetypes.MethodologyEmpirical,
			Confidence: targetConf,
		}
		require.NoError(t, h.KnowledgeKeeper.SetFact(h.Ctx, victim))

		params, err := h.KnowledgeKeeper.GetParams(h.Ctx)
		require.NoError(t, err)
		stake := knowledgekeeper.EffectiveMinChallengeStake(params, targetConf)
		ms := knowledgekeeper.NewMsgServerImpl(h.KnowledgeKeeper)
		resp, err := ms.ChallengeFact(h.Ctx, &knowledgetypes.MsgChallengeFact{
			Challenger: challenger.String(), FactId: victim.Id,
			Stake: stake.String(), Reason: "moat reward-scaling test",
		})
		require.NoError(t, err)

		preBal := h.GetBalance(challenger, "uzrn")

		round, ok := h.KnowledgeKeeper.GetVerificationRound(h.Ctx, resp.RoundId)
		require.True(t, ok)
		require.NoError(t, h.KnowledgeKeeper.CompleteRound(h.Ctx, round, &knowledgekeeper.VerificationResult{
			Verdict: knowledgetypes.Verdict_VERDICT_ACCEPT, Confidence: 900_000, AcceptCount: 3,
		}))

		postBal := h.GetBalance(challenger, "uzrn")
		return postBal.Amount.Sub(preBal.Amount)
	}

	lowReward := probe(100_000, "lowconf")
	highReward := probe(900_000, "highconf")

	require.True(t, highReward.GT(lowReward),
		"disproving a high-confidence fact must pay more than disproving a low-confidence one — paradigm shifts are the signal that matters most (low=%s high=%s)",
		lowReward.String(), highReward.String())
}

// TVW must compound with survived attacks, not merely scale linearly.
// Popper: a theory that has passed 100 tests is not 100× as credible as
// one that passed 1 — it's exponentially more. The HardeningMultiplier
// enforces that shape on top of the already-linear BaseWeight, so every
// additional survived attack is worth more than the one before, up to
// the 3× cap.
func TestMoat_TVWHardensWithSurvivedAttacks(t *testing.T) {
	h := NewTestHarness(t)
	_, err := h.KnowledgeKeeper.SeedRouteB(h.Ctx)
	require.NoError(t, err)

	submitter := testAddr("moat_hardening_sub").String()
	mkFact := func(id string, corroboration uint64) {
		require.NoError(t, h.KnowledgeKeeper.SetFact(h.Ctx, &knowledgetypes.Fact{
			Id:         id,
			Content:    "hardened truth candidate",
			Domain:     "sciences",
			Status:     knowledgetypes.FactStatus_FACT_STATUS_VERIFIED,
			Submitter:  submitter,
			MethodId:   knowledgetypes.MethodologyEmpirical,
			Confidence: 900_000,
			CorroborationCount:              corroboration,
			SubmitterCalibrationSnapshotBps: 800_000,
			AxiomDistance:                   2,
		}))
	}
	mkFact("F-HARD-0", 0)
	mkFact("F-HARD-10", 10)
	mkFact("F-HARD-40", 40)
	mkFact("F-HARD-100", 100)

	qs := knowledgekeeper.NewQueryServerImpl(h.KnowledgeKeeper)
	q := func(id string) uint64 {
		r, err := qs.TrainingValueWeight(h.Ctx, &knowledgetypes.QueryTrainingValueWeightRequest{FactId: id})
		require.NoError(t, err)
		return r.TvwBps
	}
	zero := q("F-HARD-0")
	ten := q("F-HARD-10")
	forty := q("F-HARD-40")
	hundred := q("F-HARD-100")

	// Monotonic: more survived attacks → strictly more TVW.
	require.Greater(t, ten, zero, "10 survived attacks must out-earn 0")
	require.Greater(t, forty, ten, "40 survived attacks must out-earn 10")

	// Accelerating return shape: the jump from 10→40 corroborations must
	// exceed 4× the jump from 0→10 (BaseWeight alone would give 4×; the
	// hardening multiplier pushes it past that).
	jump0to10 := ten - zero
	jump10to40 := forty - ten
	require.Greater(t, jump10to40, jump0to10*4,
		"hardening must accelerate returns, not merely linearize them")

	// Cap is asymptotic — going from 40 to 100 corroborations continues
	// to add value (BaseWeight grows linearly) but the multiplier plateaus.
	require.Greater(t, hundred, forty, "TVW still grows past the cap via BaseWeight")
}

// Failed probes earn a participation reward so stress-testing remains
// rational even when the fact stands firm. Without this, the only
// profitable strategy is to probe facts you're almost certain are
// wrong, and the substrate's epistemic audit loop starves. With this,
// any challenger with even weak doubt about a high-confidence claim
// has positive expected value for attempting a probe.
func TestMoat_FailedProbesEarnParticipationReward(t *testing.T) {
	h := NewTestHarness(t)
	_, err := h.KnowledgeKeeper.SeedRouteB(h.Ctx)
	require.NoError(t, err)

	challenger := testAddr("moat_probe_participation")
	require.NoError(t, h.FundAccount(challenger,
		sdk.NewCoins(sdk.NewCoin("uzrn", sdkmath.NewInt(50_000_000)))))

	surviving := &knowledgetypes.Fact{
		Id: "F-MOAT-SURVIVOR", Content: "a fact that will survive the probe",
		Domain: "sciences", Category: "empirical",
		Status: knowledgetypes.FactStatus_FACT_STATUS_VERIFIED,
		Submitter: testAddr("moat_probe_sub").String(),
		MethodId: knowledgetypes.MethodologyEmpirical,
		Confidence: 900_000,
	}
	require.NoError(t, h.KnowledgeKeeper.SetFact(h.Ctx, surviving))

	params, err := h.KnowledgeKeeper.GetParams(h.Ctx)
	require.NoError(t, err)
	stake := knowledgekeeper.EffectiveMinChallengeStake(params, surviving.Confidence)

	ms := knowledgekeeper.NewMsgServerImpl(h.KnowledgeKeeper)
	resp, err := ms.ChallengeFact(h.Ctx, &knowledgetypes.MsgChallengeFact{
		Challenger: challenger.String(), FactId: surviving.Id,
		Stake: stake.String(), Reason: "participation-reward test",
	})
	require.NoError(t, err)

	balAfterLock := h.GetBalance(challenger, "uzrn")
	round, ok := h.KnowledgeKeeper.GetVerificationRound(h.Ctx, resp.RoundId)
	require.True(t, ok)

	// Force the challenge to FAIL (fact survives).
	require.NoError(t, h.KnowledgeKeeper.CompleteRound(h.Ctx, round, &knowledgekeeper.VerificationResult{
		Verdict: knowledgetypes.Verdict_VERDICT_REJECT, Confidence: 900_000, RejectCount: 3,
	}))

	balAfterSettle := h.GetBalance(challenger, "uzrn")

	// Participation reward = 15% of stake.
	expected := stake.Uint64() * 150_000 / 1_000_000
	gained := balAfterSettle.Amount.Sub(balAfterLock.Amount).Uint64()
	require.Equal(t, expected, gained,
		"failed challenger must receive 15%% of stake as probe participation reward")
	require.Greater(t, gained, uint64(0),
		"participation reward must be non-zero so probing is never pure loss")
}

// Stake-weighted augmentation consensus (Wave 10 Sybil fix). The panel
// must require proportional STAKE, not proportional ADDRESSES, before
// finalizing a verdict. Without this, every downstream economic lever
// (probe amplification, hardening multiplier, paradigm-shift bonus) is
// farmable from the other direction — a Sybil ring running a captive
// panel could approve their own collusive probes and harvest the
// amplified rewards.
func TestMoat_AugmentationPanelStakeWeighted(t *testing.T) {
	h := NewTestHarness(t)
	_, err := h.KnowledgeKeeper.SeedRouteB(h.Ctx)
	require.NoError(t, err)

	ms := knowledgekeeper.NewMsgServerImpl(h.KnowledgeKeeper)
	sponsor := testAddr("moat_panel_sponsor")
	require.NoError(t, h.FundAccount(sponsor, sdk.NewCoins(sdk.NewCoin("uzrn", sdkmath.NewInt(100_000_000)))))
	submitter := testAddr("moat_panel_sub").String()

	require.NoError(t, h.KnowledgeKeeper.SetFact(h.Ctx, &knowledgetypes.Fact{
		Id: "F-PANEL", Content: "target", Domain: "sciences",
		Status: knowledgetypes.FactStatus_FACT_STATUS_ACTIVE, Submitter: sponsor.String(),
		MethodId: knowledgetypes.MethodologyEmpirical, Confidence: 900_000,
	}))
	_, err = ms.CreateAugmentationBounty(h.Ctx, &knowledgetypes.MsgCreateAugmentationBounty{
		Sponsor: sponsor.String(), Id: "b-panel", TargetFactId: "F-PANEL",
		RewardPerVariant: 1_000_000, MaxVariants: 1,
	})
	require.NoError(t, err)
	_, err = ms.SubmitAugmentation(h.Ctx, &knowledgetypes.MsgSubmitAugmentation{
		Submitter: submitter, Id: "aug-panel", BountyId: "b-panel",
		OriginalFactId: "F-PANEL", VariantContent: "variant under test",
	})
	require.NoError(t, err)

	// Three Sybil zero-stake votes: no consensus, verdict stays PENDING.
	for _, v := range []string{"moat_sybil_1", "moat_sybil_2", "moat_sybil_3"} {
		resp, err := ms.VoteOnAugmentation(h.Ctx, &knowledgetypes.MsgVoteOnAugmentation{
			Verifier: testAddr(v).String(), AugmentationId: "aug-panel",
			Vote: knowledgetypes.AugmentationVerdict_AUGMENTATION_VERDICT_EQUIVALENT,
		})
		require.NoError(t, err)
		require.False(t, resp.VerdictFinalized, "zero-stake votes must never finalize")
	}
	aug, _ := h.KnowledgeKeeper.GetAugmentation(h.Ctx, "aug-panel")
	require.Equal(t, knowledgetypes.AugmentationVerdict_AUGMENTATION_VERDICT_PENDING, aug.Verdict,
		"after 3 zero-stake votes the panel must remain PENDING")

	// Now a single stake-bearing validator votes EQUIVALENT. The total
	// voted stake is now 10_000_000, all on EQUIVALENT — 100% share,
	// easily past the 66.6% consensus threshold. With MinPanelVotes=3
	// already satisfied by the earlier Sybils, the verdict finalizes on
	// this validator's vote alone.
	realVal := testAddr("moat_real_v1").String()
	h.BondTestValidator(realVal, 10_000_000)
	resp, err := ms.VoteOnAugmentation(h.Ctx, &knowledgetypes.MsgVoteOnAugmentation{
		Verifier: realVal, AugmentationId: "aug-panel",
		Vote: knowledgetypes.AugmentationVerdict_AUGMENTATION_VERDICT_EQUIVALENT,
	})
	require.NoError(t, err)
	require.True(t, resp.VerdictFinalized,
		"a stake-bearing vote tips the consensus; Sybil headcount did not")
	require.Equal(t, knowledgetypes.AugmentationVerdict_AUGMENTATION_VERDICT_EQUIVALENT, resp.FinalizedVerdict)

	aug, _ = h.KnowledgeKeeper.GetAugmentation(h.Ctx, "aug-panel")
	require.Equal(t, knowledgetypes.AugmentationVerdict_AUGMENTATION_VERDICT_EQUIVALENT, aug.Verdict)

	// The vote record preserves all four voters with their stakes frozen
	// at vote time — three zeros + one 10_000_000.
	require.Len(t, aug.VerdictVoteStakes, 4)
	var zeroCount, stakedCount int
	for _, w := range aug.VerdictVoteStakes {
		if w == 0 {
			zeroCount++
		} else if w == 10_000_000 {
			stakedCount++
		}
	}
	require.Equal(t, 3, zeroCount, "three zero-stake Sybil voters recorded for audit")
	require.Equal(t, 1, stakedCount, "one stake-bearing voter recorded")
}

// Chain-driven probe invitation heartbeat (Wave 15). High-confidence
// facts that have gone idle are nominated by the chain for stress-
// testing — the substrate actively seeks audit rather than waiting for
// audit to arrive. This pins the full invitation flow: eligible facts
// get stamped and emit probe_invited events; fresh facts and low-
// confidence facts don't.
func TestMoat_HeartbeatInvitesIdleHighConfidenceFacts(t *testing.T) {
	h := NewTestHarness(t)
	_, err := h.KnowledgeKeeper.SeedRouteB(h.Ctx)
	require.NoError(t, err)

	// Shrink thresholds so we don't need to advance a full day of blocks.
	params, err := h.KnowledgeKeeper.GetParams(h.Ctx)
	require.NoError(t, err)
	params.ProbeInvitationIdleThresholdBlocks = 100
	params.ProbeInvitationMinConfidenceBps = 700_000
	params.ProbeInvitationBatchSize = 10
	params.ProbeInvitationReinviteCooldown = 500
	require.NoError(t, h.KnowledgeKeeper.SetParams(h.Ctx, params))

	submitter := testAddr("moat_probe_sub").String()
	mkFact := func(id string, confidence uint64, verifiedAtBlock uint64) {
		require.NoError(t, h.KnowledgeKeeper.SetFact(h.Ctx, &knowledgetypes.Fact{
			Id: id, Content: "probe candidate",
			Domain: "sciences",
			Status: knowledgetypes.FactStatus_FACT_STATUS_VERIFIED,
			Submitter: submitter,
			MethodId: knowledgetypes.MethodologyEmpirical,
			Confidence: confidence,
			VerifiedAtBlock: verifiedAtBlock,
		}))
	}
	// Eligible: high confidence, verified long ago.
	mkFact("F-IDLE-HIGH", 900_000, 1)
	// Ineligible: old but low confidence.
	mkFact("F-IDLE-LOW", 500_000, 1)

	// Advance past the idle threshold; the heartbeat runs on each block.
	h.AdvanceBlocks(150)

	// Now add a fresh high-confidence fact AFTER the advance, so its
	// VerifiedAtBlock is current and it hasn't had time to go idle.
	mkFact("F-FRESH-HIGH", 900_000, uint64(h.Height()))
	h.AdvanceBlocks(10)

	// Only the eligible fact got stamped.
	idleHigh, _ := h.KnowledgeKeeper.GetFact(h.Ctx, "F-IDLE-HIGH")
	require.Greater(t, idleHigh.ProbeInvitedAtBlock, uint64(0),
		"high-confidence idle fact must be invited for probing")
	freshHigh, _ := h.KnowledgeKeeper.GetFact(h.Ctx, "F-FRESH-HIGH")
	require.Equal(t, uint64(0), freshHigh.ProbeInvitedAtBlock,
		"too-fresh facts must not be invited")
	idleLow, _ := h.KnowledgeKeeper.GetFact(h.Ctx, "F-IDLE-LOW")
	require.Equal(t, uint64(0), idleLow.ProbeInvitedAtBlock,
		"low-confidence facts must not be invited (verifier panel handles those)")

	// Query exposes the invited fact as work for probers.
	qs := knowledgekeeper.NewQueryServerImpl(h.KnowledgeKeeper)
	resp, err := qs.IdleFacts(h.Ctx, &knowledgetypes.QueryIdleFactsRequest{})
	require.NoError(t, err)
	require.NotEmpty(t, resp.Facts, "idle facts query must surface the invited fact")
	foundInvited := false
	for _, f := range resp.Facts {
		if f.Id == "F-IDLE-HIGH" {
			foundInvited = true
			require.Greater(t, f.BlocksSinceInvited, uint64(0))
		}
	}
	require.True(t, foundInvited)
}

// Corroboration clears a prior invitation: once a probe has actually
// happened, the chain stops asking for more probes on that fact (it
// already got the audit). This confirms the invitation is a demand
// signal, not a permanent flag.
func TestMoat_ProbeInvitationClearsOnCorroboration(t *testing.T) {
	h := NewTestHarness(t)
	_, err := h.KnowledgeKeeper.SeedRouteB(h.Ctx)
	require.NoError(t, err)

	params, err := h.KnowledgeKeeper.GetParams(h.Ctx)
	require.NoError(t, err)
	params.ProbeInvitationIdleThresholdBlocks = 100
	params.ProbeInvitationMinConfidenceBps = 700_000
	params.ProbeInvitationBatchSize = 10
	params.ProbeInvitationReinviteCooldown = 500
	require.NoError(t, h.KnowledgeKeeper.SetParams(h.Ctx, params))

	submitter := testAddr("moat_probe_cleared").String()
	require.NoError(t, h.KnowledgeKeeper.SetFact(h.Ctx, &knowledgetypes.Fact{
		Id: "F-PROBE-CLEARED", Content: "will be probed",
		Domain: "sciences",
		Status: knowledgetypes.FactStatus_FACT_STATUS_VERIFIED,
		Submitter: submitter,
		MethodId: knowledgetypes.MethodologyEmpirical,
		Confidence: 900_000,
		VerifiedAtBlock: 1,
	}))

	h.AdvanceBlocks(150)
	f, _ := h.KnowledgeKeeper.GetFact(h.Ctx, "F-PROBE-CLEARED")
	require.Greater(t, f.ProbeInvitedAtBlock, uint64(0))

	// Simulate a corroboration landing (e.g., a challenge was rejected
	// so the fact survived; LastCorroboratedBlock advances).
	f.LastCorroboratedBlock = uint64(h.Height())
	require.NoError(t, h.KnowledgeKeeper.SetFact(h.Ctx, f))

	// Now the invitation should be considered stale — the query filters
	// out facts whose last corroboration is after the invitation.
	qs := knowledgekeeper.NewQueryServerImpl(h.KnowledgeKeeper)
	resp, err := qs.IdleFacts(h.Ctx, &knowledgetypes.QueryIdleFactsRequest{})
	require.NoError(t, err)
	for _, idle := range resp.Facts {
		require.NotEqual(t, "F-PROBE-CLEARED", idle.Id,
			"after corroboration the invitation must clear — the fact was just audited")
	}
}

// The probe bounty pool (Wave 15) must accumulate uzrn per block and
// fund successful-probe bonuses. Decouples epistemic-auditing budget
// from protocol treasury so continuous auditing has its own funding
// stream that doesn't compete with other governance priorities.
func TestMoat_ProbeBountyPoolAccumulatesAndFundsBonuses(t *testing.T) {
	h := NewTestHarness(t)
	_, err := h.KnowledgeKeeper.SeedRouteB(h.Ctx)
	require.NoError(t, err)

	// Starting balance should be zero.
	require.Equal(t, int64(0), h.KnowledgeKeeper.ProbeBountyPoolBalance(h.Ctx).Int64(),
		"probe bounty pool starts empty")

	// Heartbeat runs on each block; with default mint of 1 ZRN/block,
	// 100 blocks should yield 100 ZRN (100_000_000 uzrn).
	h.AdvanceBlocks(100)
	pool := h.KnowledgeKeeper.ProbeBountyPoolBalance(h.Ctx).Int64()
	require.GreaterOrEqual(t, pool, int64(100_000_000),
		"100 blocks of 1 ZRN mints should accumulate ≥ 100 ZRN in the pool")

	// Now simulate a successful challenge that draws from the pool.
	// We exercise only the pool-draw function directly to keep the test
	// focused on the funding decoupling, not the full challenge flow
	// (which is covered by TestMoat_ChallengeStakeSettled).
	challenger := testAddr("moat_pool_drawer")
	require.NoError(t, h.FundAccount(challenger, sdk.NewCoins(sdk.NewCoin("uzrn", sdkmath.NewInt(1)))))
	preBal := h.GetBalance(challenger, "uzrn").Amount.Int64()

	requested := sdkmath.NewInt(30_000_000).BigInt() // 30 ZRN bonus
	paid := h.KnowledgeKeeper.PayProbeBountyFromPool(h.Ctx, challenger, requested)
	require.Equal(t, requested.Int64(), paid.Int64(),
		"pool is well-funded; full bonus must be paid from the pool")

	postBal := h.GetBalance(challenger, "uzrn").Amount.Int64()
	require.Equal(t, preBal+requested.Int64(), postBal)

	// Pool shrinks by the paid amount.
	require.Equal(t, pool-requested.Int64(), h.KnowledgeKeeper.ProbeBountyPoolBalance(h.Ctx).Int64(),
		"pool balance decreases by exactly the paid amount")
}

// Pool cap: once ProbeBountyMaxPoolSize is reached, issuance throttles.
// Stops unbounded inflation from a perpetually-minting heartbeat.
func TestMoat_ProbeBountyPoolRespectsCap(t *testing.T) {
	h := NewTestHarness(t)
	_, err := h.KnowledgeKeeper.SeedRouteB(h.Ctx)
	require.NoError(t, err)

	// Shrink the cap so the test can reach it quickly.
	params, err := h.KnowledgeKeeper.GetParams(h.Ctx)
	require.NoError(t, err)
	params.ProbeBountyMaxPoolSize = "10000000" // 10 ZRN cap
	params.ProbeBountyMintPerBlock = "1000000" // 1 ZRN/block
	require.NoError(t, h.KnowledgeKeeper.SetParams(h.Ctx, params))

	// Advance 50 blocks — with a 10 ZRN cap and 1 ZRN/block, the pool
	// should level off at 10 ZRN regardless.
	h.AdvanceBlocks(50)
	pool := h.KnowledgeKeeper.ProbeBountyPoolBalance(h.Ctx).Int64()
	require.LessOrEqual(t, pool, int64(10_000_000),
		"pool balance must not exceed ProbeBountyMaxPoolSize")
	require.GreaterOrEqual(t, pool, int64(9_000_000),
		"pool should still be near the cap — issuance throttles but doesn't stop mid-block")
}

// Panel weight = stake × calibration (Wave 15). The Wave 10 fix made
// zero-stake Sybil addresses unable to carry consensus; this test pins
// the further refinement: a HIGH-STAKE verifier with low calibration
// cannot dominate either. Rich-but-unreliable voters are weighted down
// by their track record. The bar to capture the panel becomes BOTH
// stake AND skill — two independent rare resources.
func TestMoat_PanelWeightedByStakeTimesCalibration(t *testing.T) {
	h := NewTestHarness(t)
	_, err := h.KnowledgeKeeper.SeedRouteB(h.Ctx)
	require.NoError(t, err)

	ms := knowledgekeeper.NewMsgServerImpl(h.KnowledgeKeeper)
	sponsor := testAddr("moat_panel_rep_sponsor")
	require.NoError(t, h.FundAccount(sponsor, sdk.NewCoins(sdk.NewCoin("uzrn", sdkmath.NewInt(100_000_000)))))
	submitter := testAddr("moat_panel_rep_sub").String()

	require.NoError(t, h.KnowledgeKeeper.SetFact(h.Ctx, &knowledgetypes.Fact{
		Id: "F-PANEL-REP", Content: "target", Domain: "sciences",
		Status: knowledgetypes.FactStatus_FACT_STATUS_ACTIVE, Submitter: sponsor.String(),
		MethodId: knowledgetypes.MethodologyEmpirical, Confidence: 900_000,
	}))
	_, err = ms.CreateAugmentationBounty(h.Ctx, &knowledgetypes.MsgCreateAugmentationBounty{
		Sponsor: sponsor.String(), Id: "b-panel-rep", TargetFactId: "F-PANEL-REP",
		RewardPerVariant: 1_000_000, MaxVariants: 1,
	})
	require.NoError(t, err)
	_, err = ms.SubmitAugmentation(h.Ctx, &knowledgetypes.MsgSubmitAugmentation{
		Submitter: submitter, Id: "aug-panel-rep", BountyId: "b-panel-rep",
		OriginalFactId: "F-PANEL-REP", VariantContent: "the variant under test",
	})
	require.NoError(t, err)

	// Rich-but-uncalibrated verifier. Stake: 100M, calibration: explicitly
	// zero via an AgentCalibration record with score=0 (floored to 20%
	// internally, so effective weight = 100M × 0.2 = 20M).
	rich := testAddr("moat_panel_rich_uncal").String()
	h.BondTestValidator(rich, 100_000_000)
	require.NoError(t, h.KnowledgeKeeper.SetAgentCalibration(h.Ctx, &knowledgetypes.AgentCalibration{
		Address: rich, CalibrationScoreBps: 0, Accepted: 0, TotalSubmissions: 0,
	}))

	// Three modest-but-calibrated verifiers. Stake: 20M each × cal 0.9 =
	// 18M effective each, total 54M. Versus rich's 100M × 0.2 = 20M.
	// Under raw stake: rich would dominate. Under reputation weighting:
	// 54M / (54M + 20M) = 73% — clears the 66.6% consensus threshold.
	cal1 := testAddr("moat_panel_cal1").String()
	cal2 := testAddr("moat_panel_cal2").String()
	cal3 := testAddr("moat_panel_cal3").String()
	for _, v := range []string{cal1, cal2, cal3} {
		h.BondTestValidator(v, 20_000_000)
		require.NoError(t, h.KnowledgeKeeper.SetAgentCalibration(h.Ctx, &knowledgetypes.AgentCalibration{
			Address: v, CalibrationScoreBps: 900_000,
			Accepted: 90, TotalSubmissions: 100,
		}))
	}

	// Rich uncalibrated votes DRIFT; calibrated ones vote EQUIVALENT.
	_, err = ms.VoteOnAugmentation(h.Ctx, &knowledgetypes.MsgVoteOnAugmentation{
		Verifier: rich, AugmentationId: "aug-panel-rep",
		Vote: knowledgetypes.AugmentationVerdict_AUGMENTATION_VERDICT_DRIFT,
	})
	require.NoError(t, err)
	for _, v := range []string{cal1, cal2, cal3} {
		_, err := ms.VoteOnAugmentation(h.Ctx, &knowledgetypes.MsgVoteOnAugmentation{
			Verifier: v, AugmentationId: "aug-panel-rep",
			Vote: knowledgetypes.AugmentationVerdict_AUGMENTATION_VERDICT_EQUIVALENT,
		})
		require.NoError(t, err)
	}

	aug, _ := h.KnowledgeKeeper.GetAugmentation(h.Ctx, "aug-panel-rep")
	require.Equal(t, knowledgetypes.AugmentationVerdict_AUGMENTATION_VERDICT_EQUIVALENT, aug.Verdict,
		"calibrated minority in raw stake out-votes rich uncalibrated: skill × bond, not bond alone")

	// Calibration snapshots are preserved on the record.
	require.Len(t, aug.VerdictVoteCalibrationBps, 4)
	require.Equal(t, uint64(0), aug.VerdictVoteCalibrationBps[0], "rich uncalibrated recorded as 0")
	for i := 1; i < 4; i++ {
		require.Equal(t, uint64(900_000), aug.VerdictVoteCalibrationBps[i])
	}
}

// MsgAddFact bypasses the verifier-panel trust chain by design (genesis
// seeding and authority-gated corrections need it). Every call must emit
// a PrivilegedAction log entry so compromised-authority abuse is
// queryable via the uniform admin-action surface, not buried in the
// general block event stream.
func TestMoat_AddFactWritesPrivilegedLog(t *testing.T) {
	h := NewTestHarness(t)
	_, err := h.KnowledgeKeeper.SeedRouteB(h.Ctx)
	require.NoError(t, err)

	ms := knowledgekeeper.NewMsgServerImpl(h.KnowledgeKeeper)
	qs := knowledgekeeper.NewQueryServerImpl(h.KnowledgeKeeper)
	authority := h.KnowledgeKeeper.GetAuthority()

	resp, err := ms.AddFact(h.Ctx, &knowledgetypes.MsgAddFact{
		Authority:  authority,
		Content:    "moat: authority-injected fact",
		Domain:     "sciences",
		Category:   "empirical",
		Confidence: 950_000,
	})
	require.NoError(t, err)
	require.NotEmpty(t, resp.FactId)

	logs, err := qs.PrivilegedActions(h.Ctx, &knowledgetypes.QueryPrivilegedActionsRequest{
		Type: knowledgetypes.PrivilegedActionType_PRIVILEGED_ACTION_TYPE_FACT_AUTHORITY_INJECT,
	})
	require.NoError(t, err)
	require.Len(t, logs.Actions, 1,
		"authority-gated AddFact must emit a PrivilegedAction so compromise is queryable")
	require.Equal(t, resp.FactId, logs.Actions[0].Target)
	require.Equal(t, authority, logs.Actions[0].Invoker)
}
