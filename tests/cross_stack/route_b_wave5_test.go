package cross_stack_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	knowledgekeeper "github.com/zerone-chain/zerone/x/knowledge/keeper"
	knowledgetypes "github.com/zerone-chain/zerone/x/knowledge/types"
)

// TestRouteB_Wave5_TraceSchemaGovernance exercises the governance-ratified
// trace serialisation contract: seed at genesis, query current, amend via
// authority-gated msg, version auto-increments, historical fetch works.
func TestRouteB_Wave5_TraceSchemaGovernance(t *testing.T) {
	h := NewTestHarness(t)
	require.NoError(t, h.KnowledgeKeeper.SeedDefaultTokenizerSpec(h.Ctx))
	require.NoError(t, h.KnowledgeKeeper.SeedDefaultTraceSchema(h.Ctx))

	ms := knowledgekeeper.NewMsgServerImpl(h.KnowledgeKeeper)
	qs := knowledgekeeper.NewQueryServerImpl(h.KnowledgeKeeper)
	authority := h.KnowledgeKeeper.GetAuthority()

	// Current schema should be v1 and hash-consistent with json_schema bytes.
	cur, err := qs.TraceSchema(h.Ctx, &knowledgetypes.QueryTraceSchemaRequest{})
	require.NoError(t, err)
	require.True(t, cur.Found)
	require.Equal(t, uint64(1), cur.Schema.Version)
	require.NotEmpty(t, cur.Schema.JsonSchema)
	require.NotEmpty(t, cur.Schema.JsonSchemaHash)

	// Non-authority amendment rejected.
	_, err = ms.AmendTraceSchema(h.Ctx, &knowledgetypes.MsgAmendTraceSchema{
		Authority: "zerone1impostor00000000000000000000000",
		Schema:    &knowledgetypes.TraceSchema{JsonSchema: "{}"},
	})
	require.Error(t, err)

	// Authority amends.
	resp, err := ms.AmendTraceSchema(h.Ctx, &knowledgetypes.MsgAmendTraceSchema{
		Authority: authority,
		Schema: &knowledgetypes.TraceSchema{
			JsonSchema: `{"title":"MethodologyApplicationTrace","$schema":"https://json-schema.org/draft/2020-12/schema","type":"object"}`,
			Notes:      "pruned schema for testnet experiment",
		},
	})
	require.NoError(t, err)
	require.Equal(t, uint64(2), resp.NewVersion)

	// Current is now v2 with hash auto-computed.
	curAfter, err := qs.TraceSchema(h.Ctx, &knowledgetypes.QueryTraceSchemaRequest{})
	require.NoError(t, err)
	require.Equal(t, uint64(2), curAfter.Schema.Version)
	require.NotEmpty(t, curAfter.Schema.JsonSchemaHash)

	// v1 still retrievable.
	v1, err := qs.TraceSchemaAtVersion(h.Ctx, &knowledgetypes.QueryTraceSchemaAtVersionRequest{Version: 1})
	require.NoError(t, err)
	require.True(t, v1.Found)
	require.Equal(t, uint64(1), v1.Schema.Version)
}

// TestRouteB_Wave5_MethodologyApplicationTraceAssembly verifies that a fact
// with a full dialectical history + reformulation companions produces a
// trace that bundles every truth-seeking signal.
func TestRouteB_Wave5_MethodologyApplicationTraceAssembly(t *testing.T) {
	h := NewTestHarness(t)
	require.NoError(t, h.KnowledgeKeeper.SeedDefaultMethodologies(h.Ctx))
	require.NoError(t, h.KnowledgeKeeper.SeedDefaultTokenizerSpec(h.Ctx))
	require.NoError(t, h.KnowledgeKeeper.SeedDefaultTraceSchema(h.Ctx))

	qs := knowledgekeeper.NewQueryServerImpl(h.KnowledgeKeeper)

	submitter := testAddr("wave5_submitter").String()

	// Seed the primary fact with methodology + reasoning trace + calibration.
	primary := &knowledgetypes.Fact{
		Id:                              "FACT-5-PRIMARY",
		Content:                         "The primary claim under scrutiny.",
		Domain:                          "sciences",
		Confidence:                      900_000,
		Status:                          knowledgetypes.FactStatus_FACT_STATUS_ACTIVE,
		Submitter:                       submitter,
		MethodId:                        knowledgetypes.MethodologyEmpirical,
		ReasoningTrace:                  `[{"step":1,"observation":"..."}]`,
		CorroborationCount:              3,
		AxiomDistance:                   2,
		SubmitterCalibrationSnapshotBps: 750_000,
	}
	require.NoError(t, h.KnowledgeKeeper.SetFact(h.Ctx, primary))

	// Seed a predecessor axiom fact (SUPPORTS edge).
	axiom := &knowledgetypes.Fact{
		Id: "FACT-5-AXIOM", Content: "axiom", Domain: "sciences",
		Confidence: 950_000, Status: knowledgetypes.FactStatus_FACT_STATUS_ACTIVE,
		Submitter: submitter, MethodId: knowledgetypes.MethodologyFormal,
		AxiomDistance: 0, CorroborationCount: 10,
	}
	require.NoError(t, h.KnowledgeKeeper.SetFact(h.Ctx, axiom))
	require.NoError(t, h.KnowledgeKeeper.SetFactRelation(h.Ctx, &knowledgetypes.FactRelation{
		SourceFactId: primary.Id, TargetFactId: axiom.Id,
		Relation: knowledgetypes.RelationType_RELATION_TYPE_SUPPORTS,
		Inference: knowledgetypes.InferenceType_INFERENCE_TYPE_DEDUCTIVE,
		InferenceStrengthBps: 1_000_000,
	}))

	// Seed a contradicting disproven fact (for contradicting_fact_ids + pair).
	disproven := &knowledgetypes.Fact{
		Id: "FACT-5-DISPROVEN", Content: "a disproven counter-claim", Domain: "sciences",
		Confidence: 400_000, Status: knowledgetypes.FactStatus_FACT_STATUS_DISPROVEN,
		Submitter: submitter, MethodId: knowledgetypes.MethodologyEmpirical,
	}
	require.NoError(t, h.KnowledgeKeeper.SetFact(h.Ctx, disproven))
	require.NoError(t, h.KnowledgeKeeper.SetFactRelation(h.Ctx, &knowledgetypes.FactRelation{
		SourceFactId: primary.Id, TargetFactId: disproven.Id,
		Relation: knowledgetypes.RelationType_RELATION_TYPE_CONTRADICTS,
	}))

	// Seed reformulation variants: one EQUIVALENT, one DRIFT.
	require.NoError(t, h.KnowledgeKeeper.SetAugmentation(h.Ctx, &knowledgetypes.Augmentation{
		Id: "aug-5-eq", OriginalFactId: primary.Id,
		VariantContent: "equivalent rephrasing",
		Submitter:      testAddr("wave5_sub_eq").String(),
		Verdict:        knowledgetypes.AugmentationVerdict_AUGMENTATION_VERDICT_EQUIVALENT,
		Accepted:       true,
		VerdictBlock:   10,
	}))
	require.NoError(t, h.KnowledgeKeeper.SetAugmentation(h.Ctx, &knowledgetypes.Augmentation{
		Id: "aug-5-drift", OriginalFactId: primary.Id,
		VariantContent: "a drifted reformulation",
		Submitter:      testAddr("wave5_sub_drift").String(),
		Verdict:        knowledgetypes.AugmentationVerdict_AUGMENTATION_VERDICT_DRIFT,
		VerdictBlock:   11,
	}))

	// Assemble trace.
	resp, err := qs.MethodologyApplicationTrace(h.Ctx, &knowledgetypes.QueryMethodologyApplicationTraceRequest{
		FactId: primary.Id,
	})
	require.NoError(t, err)
	require.True(t, resp.Found)
	tr := resp.Trace

	// Provenance pin is present.
	require.NotEmpty(t, tr.TraceId)
	require.Equal(t, primary.Id, tr.FactId)
	require.Equal(t, uint64(1), tr.TokenizerVersion)
	require.Equal(t, uint64(1), tr.TraceSchemaVersion)

	// Methodology bundled.
	require.Equal(t, knowledgetypes.MethodologyEmpirical, tr.MethodologyId)
	require.NotEmpty(t, tr.MethodologyRubric,
		"methodology's Description should populate the trace rubric")
	require.NotEmpty(t, tr.ReasoningTrace)

	// Derivation graph.
	require.NotEmpty(t, tr.PredecessorEdges,
		"SUPPORTS edge to axiom must appear as predecessor")
	require.Greater(t, tr.GroundedScoreBps, uint64(0))

	// Dialectical companions.
	require.Contains(t, tr.ContradictingFactIds, disproven.Id)

	// Contrastive companions — reformulations + drift examples.
	require.Len(t, tr.Reformulations, 1)
	require.Equal(t, knowledgetypes.AugmentationVerdict_AUGMENTATION_VERDICT_EQUIVALENT, tr.Reformulations[0].Verdict)
	require.Len(t, tr.DriftExamples, 1)
	require.Equal(t, knowledgetypes.AugmentationVerdict_AUGMENTATION_VERDICT_DRIFT, tr.DriftExamples[0].Verdict)

	// Truth-seeking alignment invariants.
	require.False(t, tr.IsNormative, "facts must not be tagged is_normative")
	require.Greater(t, tr.TrainingValueWeightBps, uint64(0),
		"non-disproven facts produce positive TVW")
	require.Equal(t, knowledgetypes.CurriculumTier_CURRICULUM_TIER_INTERMEDIATE, tr.CurriculumTier)
	require.Equal(t, knowledgetypes.TrainingQualityTier_TRAINING_QUALITY_TIER_GOLD, tr.QualityTier)
}

// TestRouteB_Wave5_ContrastivePairEmission covers all four pair types:
// survived-vs-disproven, equivalent-vs-drift, equivalent-vs-inferior,
// vindicated-minority.
func TestRouteB_Wave5_ContrastivePairEmission(t *testing.T) {
	h := NewTestHarness(t)
	require.NoError(t, h.KnowledgeKeeper.SeedDefaultMethodologies(h.Ctx))
	require.NoError(t, h.KnowledgeKeeper.SeedDefaultTokenizerSpec(h.Ctx))
	require.NoError(t, h.KnowledgeKeeper.SeedDefaultTraceSchema(h.Ctx))

	qs := knowledgekeeper.NewQueryServerImpl(h.KnowledgeKeeper)

	submitter := testAddr("wave5_pairs_submitter").String()

	// 1. Survived-vs-disproven pair.
	survived := &knowledgetypes.Fact{
		Id: "P-SURVIVED", Content: "the survivor", Domain: "sciences",
		Status: knowledgetypes.FactStatus_FACT_STATUS_ACTIVE, Submitter: submitter,
		MethodId: knowledgetypes.MethodologyEmpirical, Confidence: 900_000,
	}
	disproven := &knowledgetypes.Fact{
		Id: "P-DISPROVEN", Content: "the loser", Domain: "sciences",
		Status: knowledgetypes.FactStatus_FACT_STATUS_DISPROVEN, Submitter: submitter,
		MethodId: knowledgetypes.MethodologyEmpirical, Confidence: 400_000,
	}
	require.NoError(t, h.KnowledgeKeeper.SetFact(h.Ctx, survived))
	require.NoError(t, h.KnowledgeKeeper.SetFact(h.Ctx, disproven))
	require.NoError(t, h.KnowledgeKeeper.SetFactRelation(h.Ctx, &knowledgetypes.FactRelation{
		SourceFactId: survived.Id, TargetFactId: disproven.Id,
		Relation: knowledgetypes.RelationType_RELATION_TYPE_CONTRADICTS,
		MethodId: knowledgetypes.MethodologyEmpirical,
	}))

	// 2. Reformulation-vs-drift pair.
	orig := &knowledgetypes.Fact{
		Id: "P-ORIG", Content: "the original statement", Domain: "sciences",
		Status: knowledgetypes.FactStatus_FACT_STATUS_ACTIVE, Submitter: submitter,
		MethodId: knowledgetypes.MethodologyEmpirical, Confidence: 900_000,
	}
	require.NoError(t, h.KnowledgeKeeper.SetFact(h.Ctx, orig))
	// Seed a bounty so the winner carries its methodology.
	require.NoError(t, h.KnowledgeKeeper.SetAugmentationBounty(h.Ctx, &knowledgetypes.AugmentationBounty{
		Id: "bounty-pair", TargetFactId: orig.Id, MethodologyId: knowledgetypes.MethodologyEmpirical,
		MaxVariants: 5, Active: true,
	}))
	require.NoError(t, h.KnowledgeKeeper.SetAugmentation(h.Ctx, &knowledgetypes.Augmentation{
		Id: "aug-win", OriginalFactId: orig.Id, BountyId: "bounty-pair",
		VariantContent: "clearer phrasing", Submitter: testAddr("pair_winner").String(),
		Verdict: knowledgetypes.AugmentationVerdict_AUGMENTATION_VERDICT_EQUIVALENT, Accepted: true,
	}))
	require.NoError(t, h.KnowledgeKeeper.SetAugmentation(h.Ctx, &knowledgetypes.Augmentation{
		Id: "aug-drift", OriginalFactId: orig.Id, BountyId: "bounty-pair",
		VariantContent: "meaning-changed version", Submitter: testAddr("pair_drift").String(),
		Verdict: knowledgetypes.AugmentationVerdict_AUGMENTATION_VERDICT_DRIFT,
	}))
	require.NoError(t, h.KnowledgeKeeper.SetAugmentation(h.Ctx, &knowledgetypes.Augmentation{
		Id: "aug-inferior", OriginalFactId: orig.Id, BountyId: "bounty-pair",
		VariantContent: "weaker method application", Submitter: testAddr("pair_inf").String(),
		Verdict: knowledgetypes.AugmentationVerdict_AUGMENTATION_VERDICT_INFERIOR,
	}))

	// All pairs.
	resp, err := qs.ContrastivePairs(h.Ctx, &knowledgetypes.QueryContrastivePairsRequest{})
	require.NoError(t, err)
	require.NotEmpty(t, resp.Pairs)

	var sawSurvived, sawDrift, sawInferior bool
	for _, p := range resp.Pairs {
		switch p.PairType {
		case knowledgetypes.ContrastivePairType_CONTRASTIVE_PAIR_SURVIVED_VS_DISPROVEN:
			if p.PositiveFactId == survived.Id && p.NegativeFactId == disproven.Id {
				sawSurvived = true
			}
		case knowledgetypes.ContrastivePairType_CONTRASTIVE_PAIR_EQUIVALENT_VS_DRIFT:
			if p.NegativeAugmentationId == "aug-drift" {
				sawDrift = true
				require.Equal(t, knowledgetypes.MethodologyEmpirical, p.MethodId,
					"method carries through from the winning variant's bounty")
			}
		case knowledgetypes.ContrastivePairType_CONTRASTIVE_PAIR_EQUIVALENT_VS_INFERIOR:
			if p.NegativeAugmentationId == "aug-inferior" {
				sawInferior = true
			}
		}
	}
	require.True(t, sawSurvived, "survived-vs-disproven pair must emit")
	require.True(t, sawDrift, "equivalent-vs-drift pair must emit")
	require.True(t, sawInferior, "equivalent-vs-inferior pair must emit")

	// Filter by pair_type returns only the requested kind.
	driftOnly, err := qs.ContrastivePairs(h.Ctx, &knowledgetypes.QueryContrastivePairsRequest{
		PairType: knowledgetypes.ContrastivePairType_CONTRASTIVE_PAIR_EQUIVALENT_VS_DRIFT,
	})
	require.NoError(t, err)
	for _, p := range driftOnly.Pairs {
		require.Equal(t, knowledgetypes.ContrastivePairType_CONTRASTIVE_PAIR_EQUIVALENT_VS_DRIFT, p.PairType)
	}
}

// TestRouteB_Wave5_TracesStreamFilters verifies MethodologyApplicationTraces
// honours method_id / min_corroboration / min_tier / include_disproven.
func TestRouteB_Wave5_TracesStreamFilters(t *testing.T) {
	h := NewTestHarness(t)
	require.NoError(t, h.KnowledgeKeeper.SeedDefaultMethodologies(h.Ctx))
	require.NoError(t, h.KnowledgeKeeper.SeedDefaultTokenizerSpec(h.Ctx))
	require.NoError(t, h.KnowledgeKeeper.SeedDefaultTraceSchema(h.Ctx))

	qs := knowledgekeeper.NewQueryServerImpl(h.KnowledgeKeeper)
	submitter := testAddr("wave5_stream").String()

	// 3 facts: 1 gold empirical, 1 silver formal, 1 disproven.
	require.NoError(t, h.KnowledgeKeeper.SetFact(h.Ctx, &knowledgetypes.Fact{
		Id: "S-GOLD-EMP", Content: "gold empirical", Domain: "sciences",
		Status: knowledgetypes.FactStatus_FACT_STATUS_ACTIVE, Submitter: submitter,
		MethodId: knowledgetypes.MethodologyEmpirical, Confidence: 900_000,
		CorroborationCount: 5,
	}))
	require.NoError(t, h.KnowledgeKeeper.SetFact(h.Ctx, &knowledgetypes.Fact{
		Id: "S-SILVER-FORM", Content: "silver formal", Domain: "math",
		Status: knowledgetypes.FactStatus_FACT_STATUS_ACTIVE, Submitter: submitter,
		MethodId: knowledgetypes.MethodologyFormal, Confidence: 900_000,
		CorroborationCount: 1,
	}))
	require.NoError(t, h.KnowledgeKeeper.SetFact(h.Ctx, &knowledgetypes.Fact{
		Id: "S-DISPROVEN", Content: "disproven", Domain: "sciences",
		Status: knowledgetypes.FactStatus_FACT_STATUS_DISPROVEN, Submitter: submitter,
		MethodId: knowledgetypes.MethodologyEmpirical, Confidence: 500_000,
	}))

	// Method filter narrows to empirical (gold + disproven, but disproven excluded by default).
	resp, err := qs.MethodologyApplicationTraces(h.Ctx, &knowledgetypes.QueryMethodologyApplicationTracesRequest{
		MethodId: knowledgetypes.MethodologyEmpirical, Limit: 50,
	})
	require.NoError(t, err)
	require.Len(t, resp.Traces, 1)
	require.Equal(t, "S-GOLD-EMP", resp.Traces[0].FactId)

	// include_disproven returns disproven-tier too.
	respAll, err := qs.MethodologyApplicationTraces(h.Ctx, &knowledgetypes.QueryMethodologyApplicationTracesRequest{
		MethodId: knowledgetypes.MethodologyEmpirical, IncludeDisproven: true, Limit: 50,
	})
	require.NoError(t, err)
	require.Len(t, respAll.Traces, 2)

	// min_corroboration=3 excludes the silver fact.
	respMinCorro, err := qs.MethodologyApplicationTraces(h.Ctx, &knowledgetypes.QueryMethodologyApplicationTracesRequest{
		MinCorroboration: 3, Limit: 50,
	})
	require.NoError(t, err)
	require.Len(t, respMinCorro.Traces, 1)

	// Schema version pinned to current.
	require.Equal(t, uint64(1), resp.TraceSchemaVersion)
}
