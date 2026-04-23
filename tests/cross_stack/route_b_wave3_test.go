package cross_stack_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	knowledgekeeper "github.com/zerone-chain/zerone/x/knowledge/keeper"
	knowledgetypes "github.com/zerone-chain/zerone/x/knowledge/types"
)

// TestRouteB_Wave3a_TokenizerAmendment exercises governance-gated amendment
// of the tokenizer contract: unauthorized actors are rejected, the authority
// may amend, version auto-increments monotonically, and history remains
// queryable at each version.
func TestRouteB_Wave3a_TokenizerAmendment(t *testing.T) {
	h := NewTestHarness(t)
	require.NoError(t, h.KnowledgeKeeper.SeedDefaultTokenizerSpec(h.Ctx))

	ms := knowledgekeeper.NewMsgServerImpl(h.KnowledgeKeeper)
	qs := knowledgekeeper.NewQueryServerImpl(h.KnowledgeKeeper)
	authority := h.KnowledgeKeeper.GetAuthority()

	// Non-authority rejected.
	_, err := ms.AmendTokenizerSpec(h.Ctx, &knowledgetypes.MsgAmendTokenizerSpec{
		Authority: "zerone1impostor0000000000000000000000",
		Spec: &knowledgetypes.TokenizerSpec{
			MethodTokenPrefix:             "<method:",
			FactBeginToken:                "<fact>",
			FactEndToken:                  "</fact>",
			DisproofMarkerToken:           "<disproved/>",
			CanonicalSerialisationVersion: 2,
		},
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "unauthorized")

	// Authority amends — version auto-increments from 1 → 2.
	resp, err := ms.AmendTokenizerSpec(h.Ctx, &knowledgetypes.MsgAmendTokenizerSpec{
		Authority: authority,
		Spec: &knowledgetypes.TokenizerSpec{
			Version:                       999, // caller-supplied version must be ignored
			MethodTokenPrefix:             "<method:",
			FactBeginToken:                "<fact>",
			FactEndToken:                  "</fact>",
			DisproofMarkerToken:           "<disproved/>",
			CanonicalSerialisationVersion: 2,
		},
	})
	require.NoError(t, err)
	require.Equal(t, uint64(2), resp.NewVersion)

	// Current spec reflects v2.
	curr, err := qs.TokenizerSpec(h.Ctx, &knowledgetypes.QueryTokenizerSpecRequest{})
	require.NoError(t, err)
	require.True(t, curr.Found)
	require.Equal(t, uint64(2), curr.Spec.Version)
	require.Equal(t, uint64(2), curr.Spec.CanonicalSerialisationVersion)

	// v1 history still retrievable.
	v1, err := qs.TokenizerSpecAtVersion(h.Ctx, &knowledgetypes.QueryTokenizerSpecAtVersionRequest{Version: 1})
	require.NoError(t, err)
	require.True(t, v1.Found)
	require.Equal(t, uint64(1), v1.Spec.Version)

	// Amend again to guarantee monotonic increment.
	resp2, err := ms.AmendTokenizerSpec(h.Ctx, &knowledgetypes.MsgAmendTokenizerSpec{
		Authority: authority,
		Spec: &knowledgetypes.TokenizerSpec{
			MethodTokenPrefix:             "<method:",
			FactBeginToken:                "<fact>",
			FactEndToken:                  "</fact>",
			DisproofMarkerToken:           "<disproved/>",
			CanonicalSerialisationVersion: 3,
		},
	})
	require.NoError(t, err)
	require.Equal(t, uint64(3), resp2.NewVersion)
}

// TestRouteB_Wave3b_ContributionAttribution exercises the attribution flow:
// only the model owner may post, dedup is honoured, total_weight sums
// cited-fact corroboration, and the reverse fact→model index serves the
// "which models used me?" query.
func TestRouteB_Wave3b_ContributionAttribution(t *testing.T) {
	h := NewTestHarness(t)
	require.NoError(t, h.KnowledgeKeeper.SeedDefaultMethodologies(h.Ctx))
	require.NoError(t, h.KnowledgeKeeper.SeedDefaultTokenizerSpec(h.Ctx))

	ms := knowledgekeeper.NewMsgServerImpl(h.KnowledgeKeeper)
	qs := knowledgekeeper.NewQueryServerImpl(h.KnowledgeKeeper)

	owner := "zerone1modelowner3b0000000000000000000"
	operator := "zerone1operator3b000000000000000000000"
	impostor := "zerone1impostor3b000000000000000000000"

	// Seed pipeline + model card.
	pipelineID := "pipe-3b-1"
	require.NoError(t, h.KnowledgeKeeper.SetTrainingPipeline(h.Ctx, &knowledgetypes.TrainingPipeline{
		Id:                    pipelineID,
		OperatorAddress:       operator,
		CorpusSnapshotHeight:  1,
		TokenizerVersion:      1,
		MethodologySetVersion: 1,
		RecipeHash:            "sha256:3b",
		Status:                "completed",
	}))
	modelID := "model-3b-1"
	require.NoError(t, h.KnowledgeKeeper.SetModelCard(h.Ctx, &knowledgetypes.ModelCard{
		Id:             modelID,
		Name:           "Wave3b-test-model",
		PipelineId:     pipelineID,
		OwnerAddress:   owner,
		Route:          "from_scratch",
		ParameterCount: 1,
		Active:         true,
	}))

	// Seed three facts; f1 has corroboration 5, f2 has 2, f3 is unknown fact id.
	f1 := &knowledgetypes.Fact{
		Id:                 "FACT-3b-1",
		Content:            "empirical anchor",
		Domain:             "sciences",
		Confidence:         900_000,
		Status:             knowledgetypes.FactStatus_FACT_STATUS_ACTIVE,
		Submitter:          owner,
		MethodId:           knowledgetypes.MethodologyEmpirical,
		CorroborationCount: 5,
	}
	f2 := &knowledgetypes.Fact{
		Id:                 "FACT-3b-2",
		Content:            "formal result",
		Domain:             "mathematics",
		Confidence:         950_000,
		Status:             knowledgetypes.FactStatus_FACT_STATUS_ACTIVE,
		Submitter:          owner,
		MethodId:           knowledgetypes.MethodologyFormal,
		CorroborationCount: 2,
	}
	require.NoError(t, h.KnowledgeKeeper.SetFact(h.Ctx, f1))
	require.NoError(t, h.KnowledgeKeeper.SetFact(h.Ctx, f2))

	// Impostor rejected.
	_, err := ms.AttributeContributions(h.Ctx, &knowledgetypes.MsgAttributeContributions{
		Owner:   impostor,
		ModelId: modelID,
		FactIds: []string{f1.Id, f2.Id},
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "only the model owner")

	// Unknown model rejected.
	_, err = ms.AttributeContributions(h.Ctx, &knowledgetypes.MsgAttributeContributions{
		Owner:   owner,
		ModelId: "no-such-model",
		FactIds: []string{f1.Id},
	})
	require.Error(t, err)

	// Owner posts — duplicate fact IDs deduplicated; total_weight = (5+1)+(2+1)+1 = 10
	// when we include a ghost fact (unknown id counts as weight 1).
	resp, err := ms.AttributeContributions(h.Ctx, &knowledgetypes.MsgAttributeContributions{
		Owner:   owner,
		ModelId: modelID,
		FactIds: []string{f1.Id, f2.Id, f1.Id, "GHOST-ID", ""},
	})
	require.NoError(t, err)
	require.Equal(t, uint32(3), resp.Recorded, "expected 3 unique non-empty fact ids")

	contrib, err := qs.ModelContributions(h.Ctx, &knowledgetypes.QueryModelContributionsRequest{ModelId: modelID})
	require.NoError(t, err)
	require.True(t, contrib.Found)
	require.Equal(t, uint64(6+3+1), contrib.Record.TotalWeight,
		"total weight: (f1.corro+1) + (f2.corro+1) + (ghost=1)")
	require.ElementsMatch(t, []string{f1.Id, f2.Id, "GHOST-ID"}, contrib.Record.FactIds)

	// Reverse index: f1 and f2 now point at modelID.
	c1, err := qs.FactContributors(h.Ctx, &knowledgetypes.QueryFactContributorsRequest{FactId: f1.Id})
	require.NoError(t, err)
	require.ElementsMatch(t, []string{modelID}, c1.ModelIds)
	c2, err := qs.FactContributors(h.Ctx, &knowledgetypes.QueryFactContributorsRequest{FactId: f2.Id})
	require.NoError(t, err)
	require.ElementsMatch(t, []string{modelID}, c2.ModelIds)
	cnone, err := qs.FactContributors(h.Ctx, &knowledgetypes.QueryFactContributorsRequest{FactId: "nope"})
	require.NoError(t, err)
	require.Empty(t, cnone.ModelIds)

	// A second model also citing f1 shows up in the fact→model index.
	modelID2 := "model-3b-2"
	require.NoError(t, h.KnowledgeKeeper.SetModelCard(h.Ctx, &knowledgetypes.ModelCard{
		Id: modelID2, Name: "m2", PipelineId: pipelineID, OwnerAddress: owner, Route: "from_scratch", Active: true,
	}))
	_, err = ms.AttributeContributions(h.Ctx, &knowledgetypes.MsgAttributeContributions{
		Owner: owner, ModelId: modelID2, FactIds: []string{f1.Id},
	})
	require.NoError(t, err)
	c1Both, err := qs.FactContributors(h.Ctx, &knowledgetypes.QueryFactContributorsRequest{FactId: f1.Id})
	require.NoError(t, err)
	require.ElementsMatch(t, []string{modelID, modelID2}, c1Both.ModelIds)
}

// TestRouteB_Wave3c_TrainingAttestation: only pipeline operator may attest;
// the attestation is queryable by pipeline id afterwards.
func TestRouteB_Wave3c_TrainingAttestation(t *testing.T) {
	h := NewTestHarness(t)
	require.NoError(t, h.KnowledgeKeeper.SeedDefaultTokenizerSpec(h.Ctx))

	ms := knowledgekeeper.NewMsgServerImpl(h.KnowledgeKeeper)
	qs := knowledgekeeper.NewQueryServerImpl(h.KnowledgeKeeper)

	operator := "zerone1operator3c000000000000000000000"
	impostor := "zerone1impostor3c000000000000000000000"
	pipelineID := "pipe-3c-1"
	require.NoError(t, h.KnowledgeKeeper.SetTrainingPipeline(h.Ctx, &knowledgetypes.TrainingPipeline{
		Id: pipelineID, OperatorAddress: operator, TokenizerVersion: 1, Status: "completed",
	}))

	// Impostor rejected.
	_, err := ms.AttestTraining(h.Ctx, &knowledgetypes.MsgAttestTraining{
		Attester: impostor, PipelineId: pipelineID,
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "only the pipeline operator")

	// Unknown pipeline rejected.
	_, err = ms.AttestTraining(h.Ctx, &knowledgetypes.MsgAttestTraining{
		Attester: operator, PipelineId: "no-such",
	})
	require.Error(t, err)

	// Operator attests.
	_, err = ms.AttestTraining(h.Ctx, &knowledgetypes.MsgAttestTraining{
		Attester:         operator,
		PipelineId:       pipelineID,
		FlopsEstimate:    1_234_567_890,
		WallclockSeconds: 86400,
		EvalHash:         "sha256:evalbundle",
		Signature:        "ed25519:sig",
		Notes:            "first attested run",
	})
	require.NoError(t, err)

	got, err := qs.TrainingAttestation(h.Ctx, &knowledgetypes.QueryTrainingAttestationRequest{PipelineId: pipelineID})
	require.NoError(t, err)
	require.True(t, got.Found)
	require.Equal(t, uint64(1_234_567_890), got.Attestation.FlopsEstimate)
	require.Equal(t, uint64(86400), got.Attestation.WallclockSeconds)
	require.Equal(t, "sha256:evalbundle", got.Attestation.EvalHash)
	require.Equal(t, operator, got.Attestation.AttesterAddress)
}

// TestRouteB_Wave3d_ModelLineage walks a 3-generation predecessor chain and
// verifies ordering, max_depth truncation, and missing-model handling.
func TestRouteB_Wave3d_ModelLineage(t *testing.T) {
	h := NewTestHarness(t)
	require.NoError(t, h.KnowledgeKeeper.SeedDefaultTokenizerSpec(h.Ctx))

	qs := knowledgekeeper.NewQueryServerImpl(h.KnowledgeKeeper)

	owner := "zerone1modelowner3d0000000000000000000"
	// Pipeline binding required.
	require.NoError(t, h.KnowledgeKeeper.SetTrainingPipeline(h.Ctx, &knowledgetypes.TrainingPipeline{
		Id: "pipe-3d-1", OperatorAddress: owner, TokenizerVersion: 1, Status: "completed",
	}))

	// Build chain: m-root ← m-v2 ← m-v3
	root := &knowledgetypes.ModelCard{
		Id: "m-root", Name: "root", PipelineId: "pipe-3d-1",
		OwnerAddress: owner, Route: "from_scratch", Active: true,
	}
	v2 := &knowledgetypes.ModelCard{
		Id: "m-v2", Name: "v2", PipelineId: "pipe-3d-1",
		OwnerAddress: owner, Route: "from_scratch", Active: true,
		PredecessorModelId: "m-root",
	}
	v3 := &knowledgetypes.ModelCard{
		Id: "m-v3", Name: "v3", PipelineId: "pipe-3d-1",
		OwnerAddress: owner, Route: "from_scratch", Active: true,
		PredecessorModelId: "m-v2",
	}
	require.NoError(t, h.KnowledgeKeeper.SetModelCard(h.Ctx, root))
	require.NoError(t, h.KnowledgeKeeper.SetModelCard(h.Ctx, v2))
	require.NoError(t, h.KnowledgeKeeper.SetModelCard(h.Ctx, v3))

	// Full walk from v3 returns [root, v2, v3] oldest-first.
	lin, err := qs.ModelLineage(h.Ctx, &knowledgetypes.QueryModelLineageRequest{
		ModelId: "m-v3", MaxDepth: 10,
	})
	require.NoError(t, err)
	require.True(t, lin.RootReached)
	require.False(t, lin.Truncated)
	require.Len(t, lin.Ancestry, 3)
	require.Equal(t, "m-root", lin.Ancestry[0].Id)
	require.Equal(t, "m-v2", lin.Ancestry[1].Id)
	require.Equal(t, "m-v3", lin.Ancestry[2].Id)

	// Truncated walk at depth=2 only walks v3,v2 then stops before root.
	truncLin, err := qs.ModelLineage(h.Ctx, &knowledgetypes.QueryModelLineageRequest{
		ModelId: "m-v3", MaxDepth: 2,
	})
	require.NoError(t, err)
	require.True(t, truncLin.Truncated)
	require.False(t, truncLin.RootReached)
	require.Len(t, truncLin.Ancestry, 2)

	// Unknown model returns empty ancestry, not an error.
	mis, err := qs.ModelLineage(h.Ctx, &knowledgetypes.QueryModelLineageRequest{
		ModelId: "no-such", MaxDepth: 5,
	})
	require.NoError(t, err)
	require.Empty(t, mis.Ancestry)
}

// TestRouteB_Wave3e_AugmentationMarketplace exercises the bounty lifecycle:
// create → submit variants → accept as sponsor → saturate → volunteer path.
func TestRouteB_Wave3e_AugmentationMarketplace(t *testing.T) {
	h := NewTestHarness(t)
	require.NoError(t, h.KnowledgeKeeper.SeedDefaultMethodologies(h.Ctx))
	require.NoError(t, h.KnowledgeKeeper.SeedDefaultTokenizerSpec(h.Ctx))

	ms := knowledgekeeper.NewMsgServerImpl(h.KnowledgeKeeper)
	qs := knowledgekeeper.NewQueryServerImpl(h.KnowledgeKeeper)

	sponsor := "zerone1sponsor3e000000000000000000000a"
	submitterA := "zerone1submittera3e00000000000000000aa"
	submitterB := "zerone1submitterb3e00000000000000000bb"
	originalSubmitter := "zerone1origsubmitter3e0000000000000aaa"
	impostor := "zerone1impostor3e000000000000000000000"

	targetFact := &knowledgetypes.Fact{
		Id:         "FACT-3e-TARGET",
		Content:    "The target fact wanting reformulations.",
		Domain:     "sciences",
		Confidence: 900_000,
		Status:     knowledgetypes.FactStatus_FACT_STATUS_ACTIVE,
		Submitter:  originalSubmitter,
		MethodId:   knowledgetypes.MethodologyEmpirical,
	}
	volunteerFact := &knowledgetypes.Fact{
		Id:         "FACT-3e-VOLUNTEER",
		Content:    "A fact reformulated voluntarily (no bounty).",
		Domain:     "sciences",
		Confidence: 800_000,
		Status:     knowledgetypes.FactStatus_FACT_STATUS_ACTIVE,
		Submitter:  originalSubmitter,
		MethodId:   knowledgetypes.MethodologyEmpirical,
	}
	require.NoError(t, h.KnowledgeKeeper.SetFact(h.Ctx, targetFact))
	require.NoError(t, h.KnowledgeKeeper.SetFact(h.Ctx, volunteerFact))

	// Bounty for an unknown fact rejected.
	_, err := ms.CreateAugmentationBounty(h.Ctx, &knowledgetypes.MsgCreateAugmentationBounty{
		Sponsor: sponsor, Id: "bounty-bad", TargetFactId: "nope", MaxVariants: 1,
	})
	require.Error(t, err)

	// max_variants must be > 0.
	_, err = ms.CreateAugmentationBounty(h.Ctx, &knowledgetypes.MsgCreateAugmentationBounty{
		Sponsor: sponsor, Id: "bounty-zero", TargetFactId: targetFact.Id, MaxVariants: 0,
	})
	require.Error(t, err)

	// Legitimate bounty: max 2 variants.
	_, err = ms.CreateAugmentationBounty(h.Ctx, &knowledgetypes.MsgCreateAugmentationBounty{
		Sponsor:          sponsor,
		Id:               "bounty-1",
		TargetFactId:     targetFact.Id,
		RewardPerVariant: 1000,
		MaxVariants:      2,
		Description:      "Rephrase in plain language",
	})
	require.NoError(t, err)

	// Duplicate bounty rejected.
	_, err = ms.CreateAugmentationBounty(h.Ctx, &knowledgetypes.MsgCreateAugmentationBounty{
		Sponsor: sponsor, Id: "bounty-1", TargetFactId: targetFact.Id, MaxVariants: 1,
	})
	require.Error(t, err)

	// Submit A.
	_, err = ms.SubmitAugmentation(h.Ctx, &knowledgetypes.MsgSubmitAugmentation{
		Submitter: submitterA, Id: "aug-A", BountyId: "bounty-1",
		OriginalFactId: targetFact.Id, VariantContent: "First rephrasing",
	})
	require.NoError(t, err)

	// Mismatched target fact rejected.
	_, err = ms.SubmitAugmentation(h.Ctx, &knowledgetypes.MsgSubmitAugmentation{
		Submitter: submitterA, Id: "aug-mismatch", BountyId: "bounty-1",
		OriginalFactId: volunteerFact.Id, VariantContent: "wrong target",
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "bounty target")

	// Submit B.
	_, err = ms.SubmitAugmentation(h.Ctx, &knowledgetypes.MsgSubmitAugmentation{
		Submitter: submitterB, Id: "aug-B", BountyId: "bounty-1",
		OriginalFactId: targetFact.Id, VariantContent: "Second rephrasing",
	})
	require.NoError(t, err)

	// Non-sponsor rejected from accepting.
	_, err = ms.AcceptAugmentation(h.Ctx, &knowledgetypes.MsgAcceptAugmentation{
		Acceptor: impostor, AugmentationId: "aug-A",
	})
	require.Error(t, err)

	// Sponsor accepts A.
	_, err = ms.AcceptAugmentation(h.Ctx, &knowledgetypes.MsgAcceptAugmentation{
		Acceptor: sponsor, AugmentationId: "aug-A", Note: "great",
	})
	require.NoError(t, err)

	// Sponsor accepts B — this saturates the bounty (max 2).
	_, err = ms.AcceptAugmentation(h.Ctx, &knowledgetypes.MsgAcceptAugmentation{
		Acceptor: sponsor, AugmentationId: "aug-B",
	})
	require.NoError(t, err)

	bnt, err := qs.AugmentationBounty(h.Ctx, &knowledgetypes.QueryAugmentationBountyRequest{Id: "bounty-1"})
	require.NoError(t, err)
	require.True(t, bnt.Found)
	require.False(t, bnt.Bounty.Active, "bounty should deactivate on saturation")
	require.Equal(t, uint32(2), bnt.Bounty.AcceptedVariants)

	// A third submission against the saturated bounty is rejected.
	_, err = ms.SubmitAugmentation(h.Ctx, &knowledgetypes.MsgSubmitAugmentation{
		Submitter: submitterA, Id: "aug-C", BountyId: "bounty-1",
		OriginalFactId: targetFact.Id, VariantContent: "too late",
	})
	require.Error(t, err)

	// Volunteer path — no bounty, original fact's submitter accepts.
	_, err = ms.SubmitAugmentation(h.Ctx, &knowledgetypes.MsgSubmitAugmentation{
		Submitter: submitterA, Id: "aug-volunteer",
		OriginalFactId: volunteerFact.Id, VariantContent: "Volunteer variant",
	})
	require.NoError(t, err)

	// Only original fact submitter may accept volunteer variant.
	_, err = ms.AcceptAugmentation(h.Ctx, &knowledgetypes.MsgAcceptAugmentation{
		Acceptor: impostor, AugmentationId: "aug-volunteer",
	})
	require.Error(t, err)
	_, err = ms.AcceptAugmentation(h.Ctx, &knowledgetypes.MsgAcceptAugmentation{
		Acceptor: originalSubmitter, AugmentationId: "aug-volunteer",
	})
	require.NoError(t, err)

	// Query augmentations by fact and ensure both paths queryable.
	byTarget, err := qs.AugmentationsByFact(h.Ctx, &knowledgetypes.QueryAugmentationsByFactRequest{
		FactId: targetFact.Id,
	})
	require.NoError(t, err)
	require.Len(t, byTarget.Augmentations, 2)

	acceptedOnly, err := qs.AugmentationsByFact(h.Ctx, &knowledgetypes.QueryAugmentationsByFactRequest{
		FactId: targetFact.Id, AcceptedOnly: true,
	})
	require.NoError(t, err)
	require.Len(t, acceptedOnly.Augmentations, 2)

	byVol, err := qs.AugmentationsByFact(h.Ctx, &knowledgetypes.QueryAugmentationsByFactRequest{
		FactId: volunteerFact.Id, AcceptedOnly: true,
	})
	require.NoError(t, err)
	require.Len(t, byVol.Augmentations, 1)

	// Bounty listings with filters.
	all, err := qs.AugmentationBounties(h.Ctx, &knowledgetypes.QueryAugmentationBountiesRequest{})
	require.NoError(t, err)
	require.Len(t, all.Bounties, 1)
	active, err := qs.AugmentationBounties(h.Ctx, &knowledgetypes.QueryAugmentationBountiesRequest{ActiveOnly: true})
	require.NoError(t, err)
	require.Empty(t, active.Bounties, "saturated bounty should drop from active-only filter")
	bySponsor, err := qs.AugmentationBounties(h.Ctx, &knowledgetypes.QueryAugmentationBountiesRequest{SponsorAddress: sponsor})
	require.NoError(t, err)
	require.Len(t, bySponsor.Bounties, 1)
}
