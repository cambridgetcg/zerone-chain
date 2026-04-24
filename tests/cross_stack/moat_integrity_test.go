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
