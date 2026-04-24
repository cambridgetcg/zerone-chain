package cross_stack_test

import (
	"testing"

	sdkmath "cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/stretchr/testify/require"

	knowledgekeeper "github.com/zerone-chain/zerone/x/knowledge/keeper"
	knowledgetypes "github.com/zerone-chain/zerone/x/knowledge/types"
)

// Wave 4 wires the augmentation accept path through a verifier panel,
// escrows bounty rewards in a module account, routes contribution revenue
// through a Popper-weighted TVW formula, adds an is-ought wall in money,
// and introduces post-hoc calibration-gated training-fund disbursements.
//
// These tests exercise every economic payout; every assertion names the
// alignment principle being verified.

// TestRouteB_Wave4a_IsOughtWall — NormativeCommitment fact_ids must not
// reach a ContributionRecord. Any that do are reported under
// rejected_commitment_count and ALL payout math skips them.
func TestRouteB_Wave4a_IsOughtWall(t *testing.T) {
	h := NewTestHarness(t)
	require.NoError(t, h.KnowledgeKeeper.SeedDefaultMethodologies(h.Ctx))
	require.NoError(t, h.KnowledgeKeeper.SeedDefaultTokenizerSpec(h.Ctx))
	require.NoError(t, h.KnowledgeKeeper.SeedDefaultCommitments(h.Ctx))

	ms := knowledgekeeper.NewMsgServerImpl(h.KnowledgeKeeper)
	qs := knowledgekeeper.NewQueryServerImpl(h.KnowledgeKeeper)

	owner := testAddr("wave4a_owner").String()
	operator := testAddr("wave4a_op").String()

	// Seed pipeline + model.
	require.NoError(t, h.KnowledgeKeeper.SetTrainingPipeline(h.Ctx, &knowledgetypes.TrainingPipeline{
		Id: "pipe-4a", OperatorAddress: operator, TokenizerVersion: 1, Status: "completed",
	}))
	require.NoError(t, h.KnowledgeKeeper.SetModelCard(h.Ctx, &knowledgetypes.ModelCard{
		Id: "model-4a", PipelineId: "pipe-4a", OwnerAddress: owner, Route: "from_scratch", Active: true,
	}))

	// Seed a legitimate fact.
	require.NoError(t, h.KnowledgeKeeper.SetFact(h.Ctx, &knowledgetypes.Fact{
		Id: "FACT-4a-GOOD", Content: "A real fact", Domain: "sciences",
		Confidence: 900_000, Status: knowledgetypes.FactStatus_FACT_STATUS_ACTIVE,
		Submitter: owner, MethodId: knowledgetypes.MethodologyEmpirical,
		CorroborationCount: 5,
	}))

	// Pick a commitment ID seeded at genesis.
	commitments := h.KnowledgeKeeper.GetAllNormativeCommitments(h.Ctx)
	require.NotEmpty(t, commitments, "seed commitments expected")
	commitmentID := commitments[0].Id

	// Attribute a mix: one real fact, one normative commitment id (must be dropped).
	_, err := ms.AttributeContributions(h.Ctx, &knowledgetypes.MsgAttributeContributions{
		Owner:   owner,
		ModelId: "model-4a",
		FactIds: []string{"FACT-4a-GOOD", commitmentID},
	})
	require.NoError(t, err)

	rec, err := qs.ModelContributions(h.Ctx, &knowledgetypes.QueryModelContributionsRequest{ModelId: "model-4a"})
	require.NoError(t, err)
	require.True(t, rec.Found)
	require.Equal(t, []string{"FACT-4a-GOOD"}, rec.Record.FactIds,
		"normative commitment ids must not be recorded into ContributionRecord.fact_ids")
	require.Equal(t, uint32(1), rec.Record.RejectedCommitmentCount,
		"the attempted ought-claim laundering must be reported")
	require.Greater(t, rec.Record.ComputedTvw, uint64(0), "the legitimate fact must still yield TVW")

	// And TrainingValueWeight on a commitment id explicitly returns BlockedIsOught.
	tvwCommit, err := qs.TrainingValueWeight(h.Ctx, &knowledgetypes.QueryTrainingValueWeightRequest{FactId: commitmentID})
	require.NoError(t, err)
	require.True(t, tvwCommit.BlockedIsOught)
	require.Equal(t, uint64(0), tvwCommit.TvwBps)

	// NormativeCorpus export returns the commitments tagged is_normative=true.
	corpus, err := qs.NormativeCorpus(h.Ctx, &knowledgetypes.QueryNormativeCorpusRequest{Limit: 50})
	require.NoError(t, err)
	require.NotEmpty(t, corpus.Entries)
	for _, e := range corpus.Entries {
		require.True(t, e.IsNormative)
	}
}

// TestRouteB_Wave4b_PopperWeightedTVWAndClawback — TVW follows survived
// falsification, methodology normalization, vindication, calibration
// snapshot, and axiom proximity. Disproved facts earn zero.
func TestRouteB_Wave4b_PopperWeightedTVWAndClawback(t *testing.T) {
	h := NewTestHarness(t)
	require.NoError(t, h.KnowledgeKeeper.SeedDefaultMethodologies(h.Ctx))
	require.NoError(t, h.KnowledgeKeeper.SeedDefaultTokenizerSpec(h.Ctx))

	qs := knowledgekeeper.NewQueryServerImpl(h.KnowledgeKeeper)

	baseFact := &knowledgetypes.Fact{
		Id: "FACT-4b-BASE", Content: "base", Domain: "sciences",
		Confidence: 900_000, Status: knowledgetypes.FactStatus_FACT_STATUS_ACTIVE,
		Submitter: testAddr("wave4b_sub").String(), MethodId: knowledgetypes.MethodologyEmpirical,
		CorroborationCount: 0, // unchallenged — survived 0 falsifications
		SubmitterCalibrationSnapshotBps: 800_000,
		AxiomDistance: 3,
	}
	require.NoError(t, h.KnowledgeKeeper.SetFact(h.Ctx, baseFact))

	// A highly-corroborated fact earns more (Popperian: more failed falsifications).
	challengedFact := &knowledgetypes.Fact{
		Id: "FACT-4b-CHALLENGED", Content: "well-challenged", Domain: "sciences",
		Confidence: 900_000, Status: knowledgetypes.FactStatus_FACT_STATUS_ACTIVE,
		Submitter: testAddr("wave4b_sub").String(), MethodId: knowledgetypes.MethodologyEmpirical,
		CorroborationCount: 10, // survived 10 falsification attempts
		SubmitterCalibrationSnapshotBps: 800_000,
		AxiomDistance: 3,
	}
	require.NoError(t, h.KnowledgeKeeper.SetFact(h.Ctx, challengedFact))

	baseTVW, err := qs.TrainingValueWeight(h.Ctx, &knowledgetypes.QueryTrainingValueWeightRequest{FactId: baseFact.Id})
	require.NoError(t, err)
	challengedTVW, err := qs.TrainingValueWeight(h.Ctx, &knowledgetypes.QueryTrainingValueWeightRequest{FactId: challengedFact.Id})
	require.NoError(t, err)
	require.Greater(t, challengedTVW.TvwBps, baseTVW.TvwBps,
		"survived falsifications must out-weigh mere existence (Popper over popularity)")

	// Methodology normalization: phenomenological earns more per unit corroboration.
	phenomFact := &knowledgetypes.Fact{
		Id: "FACT-4b-PHENOM", Content: "lived-experience claim", Domain: "phenomenology",
		Confidence: 900_000, Status: knowledgetypes.FactStatus_FACT_STATUS_ACTIVE,
		Submitter: testAddr("wave4b_sub").String(),
		MethodId:  knowledgetypes.MethodologyPhenomenologic,
		CorroborationCount: 0,
		SubmitterCalibrationSnapshotBps: 800_000,
		AxiomDistance: 3,
	}
	require.NoError(t, h.KnowledgeKeeper.SetFact(h.Ctx, phenomFact))
	phenomTVW, err := qs.TrainingValueWeight(h.Ctx, &knowledgetypes.QueryTrainingValueWeightRequest{FactId: phenomFact.Id})
	require.NoError(t, err)
	require.Greater(t, phenomTVW.MethodologyMultiplierBps, uint64(1_000_000),
		"phenomenological methodology must have >1× normalization")
	require.Greater(t, phenomTVW.TvwBps, baseTVW.TvwBps,
		"frontier methodologies mustn't be starved at equal corroboration")

	// Axiom proximity: closer to axioms earns more.
	axiomFact := &knowledgetypes.Fact{
		Id: "FACT-4b-AXIOM", Content: "axiom adjacent", Domain: "sciences",
		Confidence: 900_000, Status: knowledgetypes.FactStatus_FACT_STATUS_ACTIVE,
		Submitter: testAddr("wave4b_sub").String(), MethodId: knowledgetypes.MethodologyEmpirical,
		CorroborationCount: 0,
		SubmitterCalibrationSnapshotBps: 800_000,
		AxiomDistance: 0,
	}
	require.NoError(t, h.KnowledgeKeeper.SetFact(h.Ctx, axiomFact))
	axiomTVW, err := qs.TrainingValueWeight(h.Ctx, &knowledgetypes.QueryTrainingValueWeightRequest{FactId: axiomFact.Id})
	require.NoError(t, err)
	require.Greater(t, axiomTVW.TvwBps, baseTVW.TvwBps,
		"axiom-proximate facts must earn more per use (foundational weighting)")

	// Clawback on disproval: future TVW drops to zero; clawback block stamped.
	require.NoError(t, h.KnowledgeKeeper.ClawbackOnDisproval(h.Ctx, challengedFact.Id))
	postDisproval, err := qs.TrainingValueWeight(h.Ctx, &knowledgetypes.QueryTrainingValueWeightRequest{FactId: challengedFact.Id})
	require.NoError(t, err)
	require.True(t, postDisproval.Disproven)
	require.Equal(t, uint64(0), postDisproval.TvwBps,
		"disproven facts must earn zero future revenue")
}

// TestRouteB_Wave4cd_AugmentationEscrowAndVerdict — bounty escrow locks coins
// in the training fund module account; accept requires a verifier panel
// verdict, not the sponsor.
func TestRouteB_Wave4cd_AugmentationEscrowAndVerdict(t *testing.T) {
	h := NewTestHarness(t)
	require.NoError(t, h.KnowledgeKeeper.SeedDefaultMethodologies(h.Ctx))
	require.NoError(t, h.KnowledgeKeeper.SeedDefaultTokenizerSpec(h.Ctx))

	ms := knowledgekeeper.NewMsgServerImpl(h.KnowledgeKeeper)
	qs := knowledgekeeper.NewQueryServerImpl(h.KnowledgeKeeper)

	sponsorAddr := testAddr("wave4_sponsor")
	sponsor := sponsorAddr.String()
	submitter := testAddr("wave4_submitter").String()
	verifier1 := testAddr("wave4_v1").String()
	verifier2 := testAddr("wave4_v2").String()
	verifier3 := testAddr("wave4_v3").String()

	// Fund sponsor.
	require.NoError(t, h.FundAccount(sponsorAddr, sdk.NewCoins(sdk.NewCoin("uzrn", sdkmath.NewInt(100_000_000)))))

	// Wave 10 Sybil fix: the augmentation panel is now stake-weighted.
	// Bond each verifier with a modest self-delegation so their votes
	// carry non-zero weight in the consensus tally. Without this, the
	// panel would wait forever for stake-bearing voters.
	for _, v := range []string{verifier1, verifier2, verifier3} {
		h.BondTestValidator(v, 10_000_000)
	}

	targetFact := &knowledgetypes.Fact{
		Id: "FACT-4cd-TARGET", Content: "the original", Domain: "sciences",
		Confidence: 900_000, Status: knowledgetypes.FactStatus_FACT_STATUS_ACTIVE,
		Submitter: sponsor, MethodId: knowledgetypes.MethodologyEmpirical,
		CorroborationCount: 2,
	}
	require.NoError(t, h.KnowledgeKeeper.SetFact(h.Ctx, targetFact))

	// Create bounty — locks escrow into module account.
	_, err := ms.CreateAugmentationBounty(h.Ctx, &knowledgetypes.MsgCreateAugmentationBounty{
		Sponsor: sponsor, Id: "bounty-4cd", TargetFactId: targetFact.Id,
		RewardPerVariant: 1_000_000, MaxVariants: 2, Description: "rephrase me",
	})
	require.NoError(t, err)

	// Escrow balance check: fund holds 2 × 1_000_000 + 50% SUPERIOR padding = 3_000_000
	bal, err := qs.TrainingFundBalance(h.Ctx, &knowledgetypes.QueryTrainingFundBalanceRequest{})
	require.NoError(t, err)
	require.Equal(t, "3000000", bal.Balance,
		"module account must hold escrow (base + SUPERIOR-bonus pad)")
	require.Equal(t, "3000000", bal.Escrowed)

	// Submitter posts variant.
	_, err = ms.SubmitAugmentation(h.Ctx, &knowledgetypes.MsgSubmitAugmentation{
		Submitter: submitter, Id: "aug-4cd-1", BountyId: "bounty-4cd",
		OriginalFactId: targetFact.Id, VariantContent: "Rephrased version",
	})
	require.NoError(t, err)

	// Sponsor CANNOT self-accept under Wave 4.
	_, err = ms.AcceptAugmentation(h.Ctx, &knowledgetypes.MsgAcceptAugmentation{
		Acceptor: sponsor, AugmentationId: "aug-4cd-1",
	})
	require.Error(t, err, "sponsor must not self-judge (is-judge separation)")
	require.Contains(t, err.Error(), "verifier-panel")

	// Sponsor can't vote.
	_, err = ms.VoteOnAugmentation(h.Ctx, &knowledgetypes.MsgVoteOnAugmentation{
		Verifier: sponsor, AugmentationId: "aug-4cd-1",
		Vote: knowledgetypes.AugmentationVerdict_AUGMENTATION_VERDICT_EQUIVALENT,
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "sponsor may not vote")

	// Submitter can't vote.
	_, err = ms.VoteOnAugmentation(h.Ctx, &knowledgetypes.MsgVoteOnAugmentation{
		Verifier: submitter, AugmentationId: "aug-4cd-1",
		Vote: knowledgetypes.AugmentationVerdict_AUGMENTATION_VERDICT_EQUIVALENT,
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "submitter may not vote")

	// Verifier panel: 2 EQUIVALENT + 1 EQUIVALENT = finalized (3 votes, 100% consensus).
	for _, v := range []string{verifier1, verifier2, verifier3} {
		resp, err := ms.VoteOnAugmentation(h.Ctx, &knowledgetypes.MsgVoteOnAugmentation{
			Verifier: v, AugmentationId: "aug-4cd-1",
			Vote: knowledgetypes.AugmentationVerdict_AUGMENTATION_VERDICT_EQUIVALENT,
		})
		require.NoError(t, err)
		if v == verifier3 {
			require.True(t, resp.VerdictFinalized)
			require.Equal(t, knowledgetypes.AugmentationVerdict_AUGMENTATION_VERDICT_EQUIVALENT, resp.FinalizedVerdict)
		} else {
			require.False(t, resp.VerdictFinalized)
		}
	}

	// Submitter received payout.
	submitterBal := h.GetBalance(sdk.MustAccAddressFromBech32(submitter), "uzrn")
	require.Equal(t, sdkmath.NewInt(1_000_000), submitterBal.Amount,
		"EQUIVALENT verdict pays base reward_per_variant")

	// Augmentation state reflects verdict.
	aug, found := h.KnowledgeKeeper.GetAugmentation(h.Ctx, "aug-4cd-1")
	require.True(t, found)
	require.Equal(t, knowledgetypes.AugmentationVerdict_AUGMENTATION_VERDICT_EQUIVALENT, aug.Verdict)
	require.True(t, aug.Accepted)

	// Second variant — let's test the DRIFT path: no payout, archived for drift corpus.
	_, err = ms.SubmitAugmentation(h.Ctx, &knowledgetypes.MsgSubmitAugmentation{
		Submitter: submitter, Id: "aug-4cd-drift", BountyId: "bounty-4cd",
		OriginalFactId: targetFact.Id, VariantContent: "A variant that changes meaning",
	})
	require.NoError(t, err)
	for _, v := range []string{verifier1, verifier2, verifier3} {
		_, _ = ms.VoteOnAugmentation(h.Ctx, &knowledgetypes.MsgVoteOnAugmentation{
			Verifier: v, AugmentationId: "aug-4cd-drift",
			Vote: knowledgetypes.AugmentationVerdict_AUGMENTATION_VERDICT_DRIFT,
		})
	}
	driftAug, found := h.KnowledgeKeeper.GetAugmentation(h.Ctx, "aug-4cd-drift")
	require.True(t, found)
	require.Equal(t, knowledgetypes.AugmentationVerdict_AUGMENTATION_VERDICT_DRIFT, driftAug.Verdict)
	require.False(t, driftAug.Accepted, "DRIFT pays nothing")

	// Submitter balance unchanged after DRIFT.
	submitterBalAfterDrift := h.GetBalance(sdk.MustAccAddressFromBech32(submitter), "uzrn")
	require.Equal(t, submitterBal.Amount, submitterBalAfterDrift.Amount)

	// Drift corpus export contains the failed variant.
	drift, err := qs.DriftCorpus(h.Ctx, &knowledgetypes.QueryDriftCorpusRequest{Limit: 50})
	require.NoError(t, err)
	require.NotEmpty(t, drift.Entries)
	foundDrift := false
	for _, e := range drift.Entries {
		if e.AugmentationId == "aug-4cd-drift" {
			foundDrift = true
			require.Equal(t, knowledgetypes.AugmentationVerdict_AUGMENTATION_VERDICT_DRIFT, e.Verdict)
		}
	}
	require.True(t, foundDrift, "DRIFT-verdict variant must appear in drift corpus")
}

// TestRouteB_Wave4e_AttributionChallenge — fact submitter disputes a
// ContributionRecord with a bond; authority-resolver settles; winner
// gets bond × 2, loser forfeits.
func TestRouteB_Wave4e_AttributionChallenge(t *testing.T) {
	h := NewTestHarness(t)
	require.NoError(t, h.KnowledgeKeeper.SeedDefaultMethodologies(h.Ctx))
	require.NoError(t, h.KnowledgeKeeper.SeedDefaultTokenizerSpec(h.Ctx))

	ms := knowledgekeeper.NewMsgServerImpl(h.KnowledgeKeeper)
	qs := knowledgekeeper.NewQueryServerImpl(h.KnowledgeKeeper)

	ownerAddr := testAddr("wave4e_owner")
	owner := ownerAddr.String()
	challengerAddr := testAddr("wave4e_challenger")
	challenger := challengerAddr.String()

	// Fund challenger (bond default 5_000_000).
	require.NoError(t, h.FundAccount(challengerAddr, sdk.NewCoins(sdk.NewCoin("uzrn", sdkmath.NewInt(10_000_000)))))

	// Seed pipeline + model + contribution record.
	require.NoError(t, h.KnowledgeKeeper.SetTrainingPipeline(h.Ctx, &knowledgetypes.TrainingPipeline{
		Id: "pipe-4e", OperatorAddress: owner, TokenizerVersion: 1, Status: "completed",
	}))
	require.NoError(t, h.KnowledgeKeeper.SetModelCard(h.Ctx, &knowledgetypes.ModelCard{
		Id: "model-4e", PipelineId: "pipe-4e", OwnerAddress: owner, Route: "from_scratch", Active: true,
	}))
	require.NoError(t, h.KnowledgeKeeper.SetContributionRecord(h.Ctx, &knowledgetypes.ContributionRecord{
		ModelId: "model-4e", FactIds: []string{"FACT-4e-KNOWN"}, AttributedBy: owner,
	}))

	// Open challenge — "missing" (claims model trained on a fact that owner didn't list).
	resp, err := ms.ChallengeContribution(h.Ctx, &knowledgetypes.MsgChallengeContribution{
		Challenger:     challenger,
		ModelId:        "model-4e",
		DisputedFactId: "FACT-4e-ALLEGEDLY-USED",
		DisputeType:    "missing",
		Evidence:       `{"prompt":"X","response":"...verbatim excerpt..."}`,
		Id:             "chal-4e-1",
	})
	require.NoError(t, err)
	require.Equal(t, "5000000", resp.BondEscrowed)

	// Challenger balance decreased by bond.
	balAfterBond := h.GetBalance(challengerAddr, "uzrn")
	require.Equal(t, sdkmath.NewInt(5_000_000), balAfterBond.Amount,
		"bond escrowed (10M - 5M = 5M)")

	// Open list.
	open, err := qs.OpenContributionChallenges(h.Ctx, &knowledgetypes.QueryOpenContributionChallengesRequest{ModelId: "model-4e"})
	require.NoError(t, err)
	require.Len(t, open.Challenges, 1)

	// Non-authority cannot resolve.
	_, err = ms.ResolveContributionChallenge(h.Ctx, &knowledgetypes.MsgResolveContributionChallenge{
		Resolver:    "zerone1impostor0000000000000000000000",
		ChallengeId: "chal-4e-1",
		Uphold:      true,
	})
	require.Error(t, err)

	// Authority upholds — challenger gets bond × 2 = 10M.
	_, err = ms.ResolveContributionChallenge(h.Ctx, &knowledgetypes.MsgResolveContributionChallenge{
		Resolver:    h.KnowledgeKeeper.GetAuthority(),
		ChallengeId: "chal-4e-1",
		Uphold:      true,
		Note:        "model showed verbatim excerpt",
	})
	require.NoError(t, err)

	// Challenger now holds original balance (5M left) + 10M reward = 15M.
	finalBal := h.GetBalance(challengerAddr, "uzrn")
	require.Equal(t, sdkmath.NewInt(15_000_000), finalBal.Amount,
		"upheld challenge: winner takes bond × 2")

	// Challenge no longer appears in open list.
	openAfter, err := qs.OpenContributionChallenges(h.Ctx, &knowledgetypes.QueryOpenContributionChallengesRequest{})
	require.NoError(t, err)
	require.Empty(t, openAfter.Challenges)
}

// TestRouteB_Wave4f_CalibrationGatedDisbursement — pipeline operator claims
// post-hoc reward; calibration floor gates; 50/50 released/vesting split.
func TestRouteB_Wave4f_CalibrationGatedDisbursement(t *testing.T) {
	h := NewTestHarness(t)
	require.NoError(t, h.KnowledgeKeeper.SeedDefaultMethodologies(h.Ctx))
	require.NoError(t, h.KnowledgeKeeper.SeedDefaultTokenizerSpec(h.Ctx))

	ms := knowledgekeeper.NewMsgServerImpl(h.KnowledgeKeeper)
	qs := knowledgekeeper.NewQueryServerImpl(h.KnowledgeKeeper)

	operatorAddr := testAddr("wave4f_operator")
	operator := operatorAddr.String()
	deploymentAddr := testAddr("wave4f_deploy").String()

	require.NoError(t, h.KnowledgeKeeper.SetTrainingPipeline(h.Ctx, &knowledgetypes.TrainingPipeline{
		Id: "pipe-4f", OperatorAddress: operator, TokenizerVersion: 1, Status: "completed",
	}))
	require.NoError(t, h.KnowledgeKeeper.SetModelCard(h.Ctx, &knowledgetypes.ModelCard{
		Id: "model-4f", PipelineId: "pipe-4f", OwnerAddress: operator, Route: "from_scratch",
		Active: true, DeploymentAddress: deploymentAddr,
	}))

	// Calibration ABOVE floor → disbursement succeeds.
	require.NoError(t, h.KnowledgeKeeper.SetAgentCalibration(h.Ctx, &knowledgetypes.AgentCalibration{
		Address: deploymentAddr, CalibrationScoreBps: 800_000,
		Accepted: 50, TotalSubmissions: 50,
	}))

	claimResp, err := ms.ClaimTrainingFundDisbursement(h.Ctx, &knowledgetypes.MsgClaimTrainingFundDisbursement{
		Claimant: operator, ModelId: "model-4f", Id: "disb-4f-1",
	})
	require.NoError(t, err)

	// Assert 50/50 split.
	total, _ := sdkmath.NewIntFromString(claimResp.TotalAmount)
	released, _ := sdkmath.NewIntFromString(claimResp.ReleasedAmount)
	vesting, _ := sdkmath.NewIntFromString(claimResp.VestingAmount)
	require.True(t, released.Equal(total.Quo(sdkmath.NewInt(2))), "50% immediate release")
	require.True(t, vesting.Equal(total.Sub(released)), "50% vesting")
	require.Greater(t, claimResp.VestingEndBlock, uint64(0))

	// Operator received the released portion.
	opBal := h.GetBalance(operatorAddr, "uzrn")
	require.True(t, opBal.Amount.Equal(released),
		"released amount credited to operator immediately")

	// Re-claim rejected (idempotency on id).
	_, err = ms.ClaimTrainingFundDisbursement(h.Ctx, &knowledgetypes.MsgClaimTrainingFundDisbursement{
		Claimant: operator, ModelId: "model-4f", Id: "disb-4f-1",
	})
	require.Error(t, err)

	// Disbursement queryable.
	disb, err := qs.TrainingFundDisbursement(h.Ctx, &knowledgetypes.QueryTrainingFundDisbursementRequest{Id: "disb-4f-1"})
	require.NoError(t, err)
	require.True(t, disb.Found)
	require.Equal(t, uint64(800_000), disb.Disbursement.CalibrationScoreAtClaimBps)

	// Calibration BELOW floor → rejected.
	require.NoError(t, h.KnowledgeKeeper.SetAgentCalibration(h.Ctx, &knowledgetypes.AgentCalibration{
		Address: deploymentAddr, CalibrationScoreBps: 300_000, // below 500,000 floor
	}))
	_, err = ms.ClaimTrainingFundDisbursement(h.Ctx, &knowledgetypes.MsgClaimTrainingFundDisbursement{
		Claimant: operator, ModelId: "model-4f", Id: "disb-4f-2",
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "below floor")
}

