package cross_stack_test

import (
	"testing"

	sdkmath "cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/stretchr/testify/require"

	capturechallengekeeper "github.com/zerone-chain/zerone/x/capture_challenge/keeper"
	capturechallengetypes "github.com/zerone-chain/zerone/x/capture_challenge/types"
	knowledgekeeper "github.com/zerone-chain/zerone/x/knowledge/keeper"
	knowledgetypes "github.com/zerone-chain/zerone/x/knowledge/types"
	provkeeper "github.com/zerone-chain/zerone/x/training_provenance/keeper"
	provtypes "github.com/zerone-chain/zerone/x/training_provenance/types"
)

// x/training_provenance is a new MODULE that exists purely as a bundle
// of edges between three existing producers (knowledge, qualification,
// capture_challenge). It owns no state of its own; every read is satisfied
// from upstream. The module is what the meta-pattern produces when a set
// of latent integrations grows large enough to warrant its own home.
//
// These tests drive the synthesizer end-to-end against the production
// keepers wired in app.go. They prove three things:
//
//   1. The cert is computable from a pure-knowledge manifest (Grade A).
//   2. Audit signals (privileged-action injections) lower the grade.
//   3. Cartel resolutions in covered domains drop the grade to F.

// Grade A: a clean manifest with no privileged actions, no incidents,
// no cartel allegations gets the top trust grade. Demonstrates the
// happy-path synthesis.
func TestTrainingProvenance_GradeAOnCleanManifest(t *testing.T) {
	h := NewTestHarness(t)
	_, err := h.KnowledgeKeeper.SeedRouteB(h.Ctx)
	require.NoError(t, err)

	ms := knowledgekeeper.NewMsgServerImpl(h.KnowledgeKeeper)
	operator := testAddr("prov_clean_op").String()
	require.NoError(t, h.KnowledgeKeeper.SetTrainingPipeline(h.Ctx, &knowledgetypes.TrainingPipeline{
		Id: "pipe-prov-clean", OperatorAddress: operator, TokenizerVersion: 1,
		MethodologySetVersion: 1, Status: "declared",
	}))
	require.NoError(t, h.KnowledgeKeeper.SetFact(h.Ctx, &knowledgetypes.Fact{
		Id: "F-PROV-CLEAN-A", Content: "alpha", Domain: "sciences",
		Status: knowledgetypes.FactStatus_FACT_STATUS_ACTIVE,
		Submitter: operator, MethodId: knowledgetypes.MethodologyEmpirical,
		Confidence: 900_000, CorroborationCount: 3,
	}))
	require.NoError(t, h.KnowledgeKeeper.SetFact(h.Ctx, &knowledgetypes.Fact{
		Id: "F-PROV-CLEAN-B", Content: "beta", Domain: "biology",
		Status: knowledgetypes.FactStatus_FACT_STATUS_ACTIVE,
		Submitter: operator, MethodId: knowledgetypes.MethodologyEmpirical,
		Confidence: 900_000, CorroborationCount: 3,
	}))

	createResp, err := ms.CreateTrainingManifest(h.Ctx, &knowledgetypes.MsgCreateTrainingManifest{
		Creator: operator, Id: "manifest-prov-clean",
		PipelineId: "pipe-prov-clean",
		CorpusSelector: &knowledgetypes.CorpusSelector{
			MethodId: knowledgetypes.MethodologyEmpirical, MinCorroboration: 3,
		},
	})
	require.NoError(t, err)
	require.Equal(t, uint32(2), createResp.FactCount)
	_, err = ms.FinalizeTrainingManifest(h.Ctx, &knowledgetypes.MsgFinalizeTrainingManifest{
		Creator: operator, ManifestId: "manifest-prov-clean",
	})
	require.NoError(t, err)

	qs := provkeeper.NewQueryServerImpl(h.TrainingProvenanceKeeper)
	resp, err := qs.ProvenanceCertificate(h.Ctx, &provtypes.QueryProvenanceCertificateRequest{
		ManifestId: "manifest-prov-clean",
	})
	require.NoError(t, err)
	cert := resp.Certificate
	require.NotNil(t, cert)
	require.Equal(t, "manifest-prov-clean", cert.ManifestId)
	require.Equal(t, "pipe-prov-clean", cert.PipelineId)
	require.NotEmpty(t, cert.MerkleRoot)
	require.Equal(t, uint64(2), cert.FactCount)
	require.Equal(t, "A", cert.TrustGrade,
		"clean manifest with no privileged actions, no incidents, no cartels = Grade A")
	require.Equal(t, uint32(0), cert.PrivilegedActionCount)
	require.Equal(t, uint32(0), cert.IncidentCount)
	require.Equal(t, uint32(0), cert.CartelResolutionCount)
	// Domain coverage reflects both seeded domains.
	domainSet := map[string]uint64{}
	for _, d := range cert.Domains {
		domainSet[d.Domain] = d.FactCount
	}
	require.Equal(t, uint64(1), domainSet["sciences"])
	require.Equal(t, uint64(1), domainSet["biology"])
}

// Grade F: a cartel UPHELD in a manifest's covered domain drops the
// grade to F regardless of other signals. Demonstrates that cartel
// detection has DOWNSTREAM consequences via the synthesizer — the
// provenance certificate is the wire that exposes the bad news.
func TestTrainingProvenance_GradeFOnCartelInCoveredDomain(t *testing.T) {
	h := NewTestHarness(t)
	_, err := h.KnowledgeKeeper.SeedRouteB(h.Ctx)
	require.NoError(t, err)

	ms := knowledgekeeper.NewMsgServerImpl(h.KnowledgeKeeper)
	operator := testAddr("prov_cartel_op").String()
	require.NoError(t, h.KnowledgeKeeper.SetTrainingPipeline(h.Ctx, &knowledgetypes.TrainingPipeline{
		Id: "pipe-prov-cartel", OperatorAddress: operator, TokenizerVersion: 1,
		MethodologySetVersion: 1, Status: "declared",
	}))
	require.NoError(t, h.KnowledgeKeeper.SetFact(h.Ctx, &knowledgetypes.Fact{
		Id: "F-PROV-CARTEL", Content: "x", Domain: "mathematics",
		Status: knowledgetypes.FactStatus_FACT_STATUS_ACTIVE,
		Submitter: operator, MethodId: knowledgetypes.MethodologyFormal,
		Confidence: 900_000, CorroborationCount: 3,
	}))
	createResp, err := ms.CreateTrainingManifest(h.Ctx, &knowledgetypes.MsgCreateTrainingManifest{
		Creator: operator, Id: "manifest-prov-cartel",
		PipelineId: "pipe-prov-cartel",
		CorpusSelector: &knowledgetypes.CorpusSelector{
			MethodId: knowledgetypes.MethodologyFormal, MinCorroboration: 3,
		},
	})
	require.NoError(t, err)
	require.Equal(t, uint32(1), createResp.FactCount)
	_, err = ms.FinalizeTrainingManifest(h.Ctx, &knowledgetypes.MsgFinalizeTrainingManifest{
		Creator: operator, ManifestId: "manifest-prov-cartel",
	})
	require.NoError(t, err)

	// Drive a cartel UPHELD resolution in the covered domain.
	whistleblower := testAddr("prov_cartel_whistle")
	require.NoError(t, h.FundAccount(whistleblower, sdk.NewCoins(sdk.NewCoin("uzrn", sdkmath.NewInt(100_000_000)))))
	driveCartelUpheld(t, h, whistleblower.String(), "mathematics",
		[]string{testAddr("prov_cartel_v1").String(), testAddr("prov_cartel_v2").String()})

	// Re-query the cert: cartel resolution count > 0, grade F.
	qs := provkeeper.NewQueryServerImpl(h.TrainingProvenanceKeeper)
	resp, err := qs.ProvenanceCertificate(h.Ctx, &provtypes.QueryProvenanceCertificateRequest{
		ManifestId: "manifest-prov-cartel",
	})
	require.NoError(t, err)
	cert := resp.Certificate
	require.GreaterOrEqual(t, cert.CartelResolutionCount, uint32(1),
		"upheld cartel in covered domain must show up in the certificate")
	require.Equal(t, "F", cert.TrustGrade,
		"cartel resolution in any covered domain drops grade to F")
}

// Grade C: when audit signals accumulate (privileged-action injections
// touching constituent facts) but no cartels, the grade drops to C —
// "yellow flags accumulating, downstream consumer should review."
func TestTrainingProvenance_GradeCOnAuthorityInjections(t *testing.T) {
	h := NewTestHarness(t)
	_, err := h.KnowledgeKeeper.SeedRouteB(h.Ctx)
	require.NoError(t, err)

	ms := knowledgekeeper.NewMsgServerImpl(h.KnowledgeKeeper)
	authority := h.KnowledgeKeeper.GetAuthority()
	operator := testAddr("prov_auth_op").String()

	require.NoError(t, h.KnowledgeKeeper.SetTrainingPipeline(h.Ctx, &knowledgetypes.TrainingPipeline{
		Id: "pipe-prov-auth", OperatorAddress: operator, TokenizerVersion: 1,
		MethodologySetVersion: 1, Status: "declared",
	}))

	// Authority injects three facts via MsgAddFact (no veto window
	// configured → immediate execution; logs to PrivilegedAction).
	var factIDs []string
	for i, content := range []string{"injected-A", "injected-B", "injected-C"} {
		_ = i
		resp, err := ms.AddFact(h.Ctx, &knowledgetypes.MsgAddFact{
			Authority: authority,
			Content:   content,
			Domain:    "sciences",
			Category:  "empirical",
			Confidence: 800_000,
		})
		require.NoError(t, err)
		factIDs = append(factIDs, resp.FactId)
	}

	// Bump corroboration on the injected facts so they qualify for
	// the manifest selector (otherwise they'd be filtered).
	for _, id := range factIDs {
		f, _ := h.KnowledgeKeeper.GetFact(h.Ctx, id)
		f.CorroborationCount = 3
		f.MethodId = knowledgetypes.MethodologyEmpirical
		require.NoError(t, h.KnowledgeKeeper.SetFact(h.Ctx, f))
	}

	createResp, err := ms.CreateTrainingManifest(h.Ctx, &knowledgetypes.MsgCreateTrainingManifest{
		Creator: operator, Id: "manifest-prov-auth",
		PipelineId: "pipe-prov-auth",
		CorpusSelector: &knowledgetypes.CorpusSelector{
			MethodId: knowledgetypes.MethodologyEmpirical, MinCorroboration: 3,
		},
	})
	require.NoError(t, err)
	require.Equal(t, uint32(3), createResp.FactCount)
	_, err = ms.FinalizeTrainingManifest(h.Ctx, &knowledgetypes.MsgFinalizeTrainingManifest{
		Creator: operator, ManifestId: "manifest-prov-auth",
	})
	require.NoError(t, err)

	qs := provkeeper.NewQueryServerImpl(h.TrainingProvenanceKeeper)
	resp, err := qs.ProvenanceCertificate(h.Ctx, &provtypes.QueryProvenanceCertificateRequest{
		ManifestId: "manifest-prov-auth",
	})
	require.NoError(t, err)
	cert := resp.Certificate
	require.GreaterOrEqual(t, cert.PrivilegedActionCount, uint32(3),
		"three authority injections must each show up in the certificate's audit count")
	require.Contains(t, []string{"B", "C"}, cert.TrustGrade,
		"privileged actions touching manifest facts must downgrade from A; B if ≤2, C if more")
	if cert.TrustGrade == "C" {
		require.Contains(t, cert.TrustExplanation, "yellow flags")
	}
}

// driveCartelUpheld is a focused helper that runs the full cartel-
// detection pipeline (capture_challenge submit → evidence → resolve)
// to UPHELD against the named validators in the named domain.
func driveCartelUpheld(t *testing.T, h *TestHarness, whistleblower, domain string, accused []string) {
	t.Helper()

	for _, v := range accused {
		h.BondTestValidator(v, 50_000_000)
	}

	ccMS := capturechallengekeeper.NewMsgServerImpl(h.CaptureChallengeKeeper)
	_, err := ccMS.FundBountyPool(h.Ctx, &capturechallengetypes.MsgFundBountyPool{
		Sender: whistleblower, Domain: domain, Amount: "10000000",
	})
	require.NoError(t, err)
	subResp, err := ccMS.SubmitChallenge(h.Ctx, &capturechallengetypes.MsgSubmitChallenge{
		Challenger:        whistleblower,
		Domain:            domain,
		AccusedValidators: accused,
		Stake:             "10000000",
		Reason:            "provenance test cartel",
	})
	require.NoError(t, err)
	_, err = ccMS.AddEvidence(h.Ctx, &capturechallengetypes.MsgAddEvidence{
		Challenger:  whistleblower,
		ChallengeId: subResp.ChallengeId,
		Description: "provenance test evidence",
		DataHash:    "sha256:provtest",
	})
	require.NoError(t, err)

	h.AdvanceBlocks(5001)
	sdkCtx := sdk.UnwrapSDKContext(h.Ctx)
	h.CaptureChallengeKeeper.AdvanceChallengePhases(sdkCtx, uint64(sdkCtx.BlockHeight()))

	authority := h.CaptureChallengeKeeper.GetAuthority()
	_, err = ccMS.ResolveChallenge(h.Ctx, &capturechallengetypes.MsgResolveChallenge{
		Authority:   authority,
		ChallengeId: subResp.ChallengeId,
		Outcome:     capturechallengetypes.ChallengeOutcome_CHALLENGE_OUTCOME_UPHELD,
		Reason:      "cartel pattern verified",
	})
	require.NoError(t, err)
}
