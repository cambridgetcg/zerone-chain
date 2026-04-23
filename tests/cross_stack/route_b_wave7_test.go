package cross_stack_test

import (
	"testing"

	sdkmath "cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/stretchr/testify/require"

	knowledgekeeper "github.com/zerone-chain/zerone/x/knowledge/keeper"
	knowledgetypes "github.com/zerone-chain/zerone/x/knowledge/types"
)

// TestRouteB_Wave7_SeedRouteBBootstrap exercises the one-shot
// initialization entry point. First call writes everything; second call
// is a no-op (all seeds idempotent).
func TestRouteB_Wave7_SeedRouteBBootstrap(t *testing.T) {
	h := NewTestHarness(t)

	first, err := h.KnowledgeKeeper.SeedRouteB(h.Ctx)
	require.NoError(t, err)
	require.True(t, first.MethodologiesWritten, "first run seeds methodologies")
	require.True(t, first.TokenizerSpecWritten, "first run seeds tokenizer spec")
	require.True(t, first.TraceSchemaWritten, "first run seeds trace schema")
	require.True(t, first.CommitmentsWritten, "first run seeds commitments")

	second, err := h.KnowledgeKeeper.SeedRouteB(h.Ctx)
	require.NoError(t, err)
	require.False(t, second.MethodologiesWritten, "second run is idempotent")
	require.False(t, second.TokenizerSpecWritten, "second run is idempotent")
	require.False(t, second.TraceSchemaWritten, "second run is idempotent")
	require.False(t, second.CommitmentsWritten, "second run is idempotent")
}

// TestRouteB_Wave7_RouteBCapabilities exercises the chain's self-description.
func TestRouteB_Wave7_RouteBCapabilities(t *testing.T) {
	h := NewTestHarness(t)
	_, err := h.KnowledgeKeeper.SeedRouteB(h.Ctx)
	require.NoError(t, err)

	qs := knowledgekeeper.NewQueryServerImpl(h.KnowledgeKeeper)
	resp, err := qs.RouteBCapabilities(h.Ctx, &knowledgetypes.QueryRouteBCapabilitiesRequest{})
	require.NoError(t, err)
	caps := resp.Capabilities

	// Versions present after seed.
	require.Greater(t, caps.CurrentTokenizerVersion, uint64(0))
	require.Greater(t, caps.CurrentTraceSchemaVersion, uint64(0))
	require.Greater(t, caps.MethodologyCount, uint64(0))

	// Seed status all true after a seed run.
	require.True(t, caps.SeedStatus.MethodologiesSeeded)
	require.True(t, caps.SeedStatus.TokenizerSpecSeeded)
	require.True(t, caps.SeedStatus.TraceSchemaSeeded)
	require.True(t, caps.SeedStatus.CommitmentsSeeded)

	// All expected corpora announced.
	require.Contains(t, caps.AvailableCorpora, "MethodologyApplicationTrace")
	require.Contains(t, caps.AvailableCorpora, "ContrastivePair")
	require.Contains(t, caps.AvailableCorpora, "DriftCorpus")
	require.Contains(t, caps.AvailableCorpora, "NormativeCorpus")

	// Counts start at zero before any facts/bounties/etc exist.
	require.Equal(t, uint64(0), caps.FactCount)
	require.Equal(t, uint64(0), caps.FinalizedManifestCount)
	require.Equal(t, uint64(0), caps.ActiveBountyCount)

	// Financials exposed even with zero balance.
	require.Equal(t, "0", caps.TrainingFundBalanceUzrn)
	require.Equal(t, "0", caps.TrainingFundEscrowedUzrn)
	require.Equal(t, "0", caps.TrainingFundVestingUzrn)
}

// TestRouteB_Wave7_ManifestCreateFinalizeVerifyBundle — the crown-jewel
// test. Full lifecycle: seed → pipeline → facts → manifest create →
// finalize with Merkle root → bundle download → root re-verifies.
func TestRouteB_Wave7_ManifestCreateFinalizeVerifyBundle(t *testing.T) {
	h := NewTestHarness(t)
	_, err := h.KnowledgeKeeper.SeedRouteB(h.Ctx)
	require.NoError(t, err)

	ms := knowledgekeeper.NewMsgServerImpl(h.KnowledgeKeeper)
	qs := knowledgekeeper.NewQueryServerImpl(h.KnowledgeKeeper)

	operatorAddr := testAddr("wave7_operator")
	operator := operatorAddr.String()

	// 1. Register a training pipeline.
	require.NoError(t, h.KnowledgeKeeper.SetTrainingPipeline(h.Ctx, &knowledgetypes.TrainingPipeline{
		Id: "pipe-7", OperatorAddress: operator, TokenizerVersion: 1,
		MethodologySetVersion: 1, Status: "declared",
	}))

	// 2. Seed facts that the selector will pick up. Three gold-tier empirical
	//    facts from distinct domains; one disproven fact to exclude.
	require.NoError(t, h.KnowledgeKeeper.SetFact(h.Ctx, &knowledgetypes.Fact{
		Id: "F-7-A", Content: "a", Domain: "sciences",
		Status: knowledgetypes.FactStatus_FACT_STATUS_ACTIVE,
		Submitter: operator, MethodId: knowledgetypes.MethodologyEmpirical,
		Confidence: 900_000, CorroborationCount: 5,
	}))
	require.NoError(t, h.KnowledgeKeeper.SetFact(h.Ctx, &knowledgetypes.Fact{
		Id: "F-7-B", Content: "b", Domain: "sciences",
		Status: knowledgetypes.FactStatus_FACT_STATUS_ACTIVE,
		Submitter: operator, MethodId: knowledgetypes.MethodologyEmpirical,
		Confidence: 900_000, CorroborationCount: 4,
	}))
	require.NoError(t, h.KnowledgeKeeper.SetFact(h.Ctx, &knowledgetypes.Fact{
		Id: "F-7-C", Content: "c", Domain: "biology",
		Status: knowledgetypes.FactStatus_FACT_STATUS_ACTIVE,
		Submitter: operator, MethodId: knowledgetypes.MethodologyEmpirical,
		Confidence: 900_000, CorroborationCount: 3,
	}))
	require.NoError(t, h.KnowledgeKeeper.SetFact(h.Ctx, &knowledgetypes.Fact{
		Id: "F-7-DISPROVEN", Content: "d", Domain: "sciences",
		Status: knowledgetypes.FactStatus_FACT_STATUS_DISPROVEN,
		Submitter: operator, MethodId: knowledgetypes.MethodologyEmpirical,
	}))

	// 3. Create manifest in DRAFT.
	selector := &knowledgetypes.CorpusSelector{
		MethodId:         knowledgetypes.MethodologyEmpirical,
		MinCorroboration: 3,
		IncludeDisproven: false,
	}
	createResp, err := ms.CreateTrainingManifest(h.Ctx, &knowledgetypes.MsgCreateTrainingManifest{
		Creator:        operator,
		Id:             "manifest-7",
		PipelineId:     "pipe-7",
		CorpusSelector: selector,
		Description:    "first full-lifecycle manifest",
	})
	require.NoError(t, err)
	require.Equal(t, uint32(3), createResp.FactCount,
		"three qualifying facts; disproven excluded")
	require.Equal(t, createResp.FactCount, createResp.TraceCount,
		"trace count equals fact count")

	// Fetch the DRAFT manifest.
	mResp, err := qs.TrainingManifest(h.Ctx, &knowledgetypes.QueryTrainingManifestRequest{Id: "manifest-7"})
	require.NoError(t, err)
	require.True(t, mResp.Found)
	require.Equal(t, knowledgetypes.ManifestStatus_MANIFEST_STATUS_DRAFT, mResp.Manifest.Status)
	require.Empty(t, mResp.Manifest.MerkleRoot, "root not yet committed on DRAFT")

	// 4. Non-creator cannot finalize.
	_, err = ms.FinalizeTrainingManifest(h.Ctx, &knowledgetypes.MsgFinalizeTrainingManifest{
		Creator:    "zerone1impostor00000000000000000000000",
		ManifestId: "manifest-7",
	})
	require.Error(t, err)

	// 5. Creator finalizes.
	finResp, err := ms.FinalizeTrainingManifest(h.Ctx, &knowledgetypes.MsgFinalizeTrainingManifest{
		Creator:    operator,
		ManifestId: "manifest-7",
	})
	require.NoError(t, err)
	require.NotEmpty(t, finResp.MerkleRoot, "Merkle root materialised")

	// Status updated.
	mResp2, err := qs.TrainingManifest(h.Ctx, &knowledgetypes.QueryTrainingManifestRequest{Id: "manifest-7"})
	require.NoError(t, err)
	require.Equal(t, knowledgetypes.ManifestStatus_MANIFEST_STATUS_FINALIZED, mResp2.Manifest.Status)
	require.Equal(t, finResp.MerkleRoot, mResp2.Manifest.MerkleRoot)
	require.Greater(t, mResp2.Manifest.FinalizedAtBlock, uint64(0))

	// 6. Double-finalize rejected.
	_, err = ms.FinalizeTrainingManifest(h.Ctx, &knowledgetypes.MsgFinalizeTrainingManifest{
		Creator:    operator,
		ManifestId: "manifest-7",
	})
	require.Error(t, err)

	// 7. Download the bundle and verify the Merkle root re-derives.
	bundle, err := qs.TrainingManifestBundle(h.Ctx, &knowledgetypes.QueryTrainingManifestBundleRequest{
		Id: "manifest-7",
	})
	require.NoError(t, err)
	require.NotNil(t, bundle.Manifest)
	require.Equal(t, finResp.MerkleRoot, bundle.DerivedMerkleRoot,
		"bundle re-derivation of Merkle root matches the committed value")
	require.True(t, bundle.MerkleRootValid,
		"merkle_root_valid flag reports verified")
	require.Len(t, bundle.Traces, 3, "bundle traces match manifest.FactCount")
	for _, tr := range bundle.Traces {
		require.Equal(t, uint64(1), tr.TraceSchemaVersion,
			"each trace pins the manifest's trace_schema_version")
	}

	// 8. Bind to a training attestation.
	_, err = ms.AttestTraining(h.Ctx, &knowledgetypes.MsgAttestTraining{
		Attester: operator, PipelineId: "pipe-7",
		FlopsEstimate: 1_000_000, WallclockSeconds: 3600,
		EvalHash: "sha256:eval-bundle",
	})
	require.NoError(t, err)

	_, err = ms.BindManifestToAttestation(h.Ctx, &knowledgetypes.MsgBindManifestToAttestation{
		Creator:       operator,
		ManifestId:    "manifest-7",
		AttestationId: "pipe-7",
	})
	require.NoError(t, err)

	// Status → ATTESTED.
	mResp3, err := qs.TrainingManifest(h.Ctx, &knowledgetypes.QueryTrainingManifestRequest{Id: "manifest-7"})
	require.NoError(t, err)
	require.Equal(t, knowledgetypes.ManifestStatus_MANIFEST_STATUS_ATTESTED, mResp3.Manifest.Status)
	require.Equal(t, "pipe-7", mResp3.Manifest.AttestationId)

	// 9. Capabilities now reports 1 finalized (ATTESTED counts as finalized too).
	caps, err := qs.RouteBCapabilities(h.Ctx, &knowledgetypes.QueryRouteBCapabilitiesRequest{})
	require.NoError(t, err)
	require.Equal(t, uint64(1), caps.Capabilities.FinalizedManifestCount)
	require.Equal(t, uint64(4), caps.Capabilities.FactCount,
		"FactCount spans all statuses (3 active + 1 disproven)")

	// 10. List manifests — all 3 status filters return what they should.
	draftsList, err := qs.TrainingManifests(h.Ctx, &knowledgetypes.QueryTrainingManifestsRequest{
		Status: knowledgetypes.ManifestStatus_MANIFEST_STATUS_DRAFT,
	})
	require.NoError(t, err)
	require.Empty(t, draftsList.Manifests, "no DRAFTs left")
	attested, err := qs.TrainingManifests(h.Ctx, &knowledgetypes.QueryTrainingManifestsRequest{
		Status: knowledgetypes.ManifestStatus_MANIFEST_STATUS_ATTESTED,
	})
	require.NoError(t, err)
	require.Len(t, attested.Manifests, 1)
	byPipeline, err := qs.TrainingManifests(h.Ctx, &knowledgetypes.QueryTrainingManifestsRequest{
		PipelineId: "pipe-7",
	})
	require.NoError(t, err)
	require.Len(t, byPipeline.Manifests, 1)
}

// TestRouteB_Wave7_ManifestMerkleRootCollisionFree — sanity: swapping a
// TRACE id for a PAIR id does NOT yield the same Merkle root, because
// domain separation is enforced.
func TestRouteB_Wave7_ManifestMerkleRootCollisionFree(t *testing.T) {
	a := knowledgekeeper.SelectedManifestIDs{
		TraceIDs: []string{"x"},
	}
	b := knowledgekeeper.SelectedManifestIDs{
		PairIDs: []string{"x"},
	}
	ra := knowledgekeeper.ComputeManifestMerkleRoot(a)
	rb := knowledgekeeper.ComputeManifestMerkleRoot(b)
	require.NotEqual(t, ra, rb,
		"swapping an ID between sets must change the root (domain separation)")

	// Reordering within a sorted set is impossible (we sort); but passing
	// the same IDs in same sets yields the same root.
	ra2 := knowledgekeeper.ComputeManifestMerkleRoot(a)
	require.Equal(t, ra, ra2, "determinism: same input → same root")
}

// TestRouteB_Wave7_ManifestEndToEndWithContrastivePairs — selector that
// includes contrastive pairs causes pair_count > 0 and the bundle to
// materialise the pairs.
func TestRouteB_Wave7_ManifestEndToEndWithContrastivePairs(t *testing.T) {
	h := NewTestHarness(t)
	_, err := h.KnowledgeKeeper.SeedRouteB(h.Ctx)
	require.NoError(t, err)

	ms := knowledgekeeper.NewMsgServerImpl(h.KnowledgeKeeper)
	qs := knowledgekeeper.NewQueryServerImpl(h.KnowledgeKeeper)

	operator := testAddr("wave7_contrastive").String()
	require.NoError(t, h.KnowledgeKeeper.SetTrainingPipeline(h.Ctx, &knowledgetypes.TrainingPipeline{
		Id: "pipe-7c", OperatorAddress: operator, TokenizerVersion: 1,
	}))

	// Survivor + disproven counter-fact yields a SURVIVED_VS_DISPROVEN pair.
	surv := &knowledgetypes.Fact{
		Id: "F7c-SURV", Content: "x", Domain: "sciences",
		Status: knowledgetypes.FactStatus_FACT_STATUS_ACTIVE, Submitter: operator,
		MethodId: knowledgetypes.MethodologyEmpirical, Confidence: 900_000, CorroborationCount: 3,
	}
	dis := &knowledgetypes.Fact{
		Id: "F7c-DIS", Content: "not x", Domain: "sciences",
		Status: knowledgetypes.FactStatus_FACT_STATUS_DISPROVEN, Submitter: operator,
		MethodId: knowledgetypes.MethodologyEmpirical,
	}
	require.NoError(t, h.KnowledgeKeeper.SetFact(h.Ctx, surv))
	require.NoError(t, h.KnowledgeKeeper.SetFact(h.Ctx, dis))
	require.NoError(t, h.KnowledgeKeeper.SetFactRelation(h.Ctx, &knowledgetypes.FactRelation{
		SourceFactId: surv.Id, TargetFactId: dis.Id,
		Relation: knowledgetypes.RelationType_RELATION_TYPE_CONTRADICTS,
		MethodId: knowledgetypes.MethodologyEmpirical,
	}))

	createResp, err := ms.CreateTrainingManifest(h.Ctx, &knowledgetypes.MsgCreateTrainingManifest{
		Creator:    operator,
		Id:         "manifest-7c",
		PipelineId: "pipe-7c",
		CorpusSelector: &knowledgetypes.CorpusSelector{
			MethodId:                knowledgetypes.MethodologyEmpirical,
			MinCorroboration:        3,
			IncludeContrastivePairs: true,
		},
	})
	require.NoError(t, err)
	require.GreaterOrEqual(t, createResp.PairCount, uint32(1),
		"SURVIVED_VS_DISPROVEN pair should be included")

	_, err = ms.FinalizeTrainingManifest(h.Ctx, &knowledgetypes.MsgFinalizeTrainingManifest{
		Creator: operator, ManifestId: "manifest-7c",
	})
	require.NoError(t, err)

	bundle, err := qs.TrainingManifestBundle(h.Ctx, &knowledgetypes.QueryTrainingManifestBundleRequest{Id: "manifest-7c"})
	require.NoError(t, err)
	require.NotEmpty(t, bundle.ContrastivePairs,
		"bundle materialises the included pairs alongside traces")
	require.True(t, bundle.MerkleRootValid,
		"even with pairs, root re-derives cleanly")
}

// TestRouteB_Wave7_EscrowVisibleInCapabilities — after a sponsor locks
// bounty escrow, RouteBCapabilities reports a non-zero escrowed balance.
func TestRouteB_Wave7_EscrowVisibleInCapabilities(t *testing.T) {
	h := NewTestHarness(t)
	_, err := h.KnowledgeKeeper.SeedRouteB(h.Ctx)
	require.NoError(t, err)

	ms := knowledgekeeper.NewMsgServerImpl(h.KnowledgeKeeper)
	qs := knowledgekeeper.NewQueryServerImpl(h.KnowledgeKeeper)

	sponsorAddr := testAddr("wave7_sponsor_escrow")
	sponsor := sponsorAddr.String()
	require.NoError(t, h.FundAccount(sponsorAddr, sdk.NewCoins(sdk.NewCoin("uzrn", sdkmath.NewInt(100_000_000)))))

	require.NoError(t, h.KnowledgeKeeper.SetFact(h.Ctx, &knowledgetypes.Fact{
		Id: "F-CAP-TARGET", Content: "target", Domain: "sciences",
		Status: knowledgetypes.FactStatus_FACT_STATUS_ACTIVE, Submitter: sponsor,
		MethodId: knowledgetypes.MethodologyEmpirical, Confidence: 900_000,
	}))

	_, err = ms.CreateAugmentationBounty(h.Ctx, &knowledgetypes.MsgCreateAugmentationBounty{
		Sponsor: sponsor, Id: "bounty-cap", TargetFactId: "F-CAP-TARGET",
		RewardPerVariant: 1_000_000, MaxVariants: 2,
	})
	require.NoError(t, err)

	caps, err := qs.RouteBCapabilities(h.Ctx, &knowledgetypes.QueryRouteBCapabilitiesRequest{})
	require.NoError(t, err)
	require.Equal(t, "3000000", caps.Capabilities.TrainingFundBalanceUzrn,
		"2 × 1M base + 50% SUPERIOR padding = 3M locked in training fund")
	require.Equal(t, "3000000", caps.Capabilities.TrainingFundEscrowedUzrn,
		"escrowed surface reports the bounty lock")
	require.Equal(t, uint64(1), caps.Capabilities.ActiveBountyCount)
}
