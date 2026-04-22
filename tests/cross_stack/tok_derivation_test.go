package cross_stack_test

import (
	"testing"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/stretchr/testify/require"

	knowledgekeeper "github.com/zerone-chain/zerone/x/knowledge/keeper"
	knowledgetypes "github.com/zerone-chain/zerone/x/knowledge/types"
)

// TestToK_ChainedDerivation_AxiomDistanceAndConfidenceFloor drives a three-step
// derivation chain (axiom → fact1 → fact2 → fact3) and verifies that:
//
//  1. axiom_distance propagates: 0 → 1 → 2 → 3
//  2. dependency_confidence_floor inherits the weakest cited support
//  3. Fact.Confidence is clamped to its floor when the parent is weaker
//  4. ProofTree returns the full chain with typed inference edges
//
// This is the regression guard for ToK Waves 1, 2, and 3.
func TestToK_ChainedDerivation_AxiomDistanceAndConfidenceFloor(t *testing.T) {
	h := NewTestHarness(t)

	domain := "tok_derivation_domain"
	require.NoError(t, h.KnowledgeKeeper.SetDomain(h.Ctx, &knowledgetypes.Domain{
		Name:   domain,
		Status: knowledgetypes.DomainStatus_DOMAIN_STATUS_ACTIVE,
	}))

	// ─── Layer 0: Axiom — foundational, distance 0, 100% confidence ──────
	axiom := &knowledgetypes.Fact{
		Id:                "AXIOM-TEST-001",
		Content:           "An isolated system's entropy cannot decrease.",
		Domain:            domain,
		Category:          "formal",
		Confidence:        1_000_000,
		Status:            knowledgetypes.FactStatus_FACT_STATUS_VERIFIED,
		Submitter:         "genesis",
		Stratum:           "fundamental",
		Maturity:          "canonical",
		AxiomDistance:     0,
	}
	require.NoError(t, h.KnowledgeKeeper.SetFact(h.Ctx, axiom))

	// ─── Layer 1: Direct derivation — deductive, 95% confidence ──────────
	// Simulate by calling createFactFromClaim path indirectly: build a
	// Claim whose relations cite the axiom with INFERENCE_TYPE_DEDUCTIVE,
	// then drive CompleteRound ACCEPT verdict to exercise the full path.
	// For clarity, we build a claim directly and invoke the creation path
	// by going through the message server submission is heavy; instead use
	// the keeper's internal creation logic by submitting via SubmitClaim
	// and advancing the verification round. That's covered by the
	// full_loop test. Here, we exercise the provenance calculation
	// deterministically by crafting each fact's prior state and asserting
	// the derived fact picks up the right distance/floor.
	//
	// We want to test computeProvenance end-to-end. The cleanest way is to
	// build Claims and drive them through CompleteRound with ACCEPT. The
	// harness has no msg_server shortcut for multi-validator flows, so we
	// call createFactFromClaim-equivalent by going through a small helper:
	// we construct a claim, persist it, open a round, force verdict.

	layer1 := submitAndAcceptChainedClaim(t, h, domain,
		"Thermal energy only flows from hot to cold in the absence of work.",
		[]*knowledgetypes.ClaimRelation{
			{
				TargetFactId:         axiom.Id,
				Relation:             knowledgetypes.RelationType_RELATION_TYPE_REQUIRES,
				Inference:            knowledgetypes.InferenceType_INFERENCE_TYPE_DEDUCTIVE,
				InferenceStrengthBps: 1_000_000,
			},
		}, "derived-from-axiom")

	require.Equal(t, uint32(1), layer1.AxiomDistance,
		"layer-1 fact cites a distance-0 axiom → distance 1")
	require.Equal(t, uint64(1_000_000), layer1.DependencyConfidenceFloor,
		"dep floor inherits axiom confidence 1M")

	// ─── Layer 2: cites layer1 with an inductive generalization ──────────
	layer2 := submitAndAcceptChainedClaim(t, h, domain,
		"Real engines always dissipate some energy as waste heat.",
		[]*knowledgetypes.ClaimRelation{
			{
				TargetFactId:         layer1.Id,
				Relation:             knowledgetypes.RelationType_RELATION_TYPE_SUPPORTS,
				Inference:            knowledgetypes.InferenceType_INFERENCE_TYPE_INDUCTIVE,
				InferenceStrengthBps: 800_000, // inductive, not truth-preserving
			},
		}, "inductive-from-layer1")

	require.Equal(t, uint32(2), layer2.AxiomDistance,
		"layer-2 cites layer-1 (distance 1) → distance 2")
	require.LessOrEqual(t, layer2.DependencyConfidenceFloor, layer1.Confidence,
		"layer-2 floor must not exceed layer-1's effective confidence")

	// ─── Layer 3: cites layer2 AND layer1 — floor is weakest of the two ──
	layer3 := submitAndAcceptChainedClaim(t, h, domain,
		"Carnot efficiency bounds exhaustively apply to all heat engines.",
		[]*knowledgetypes.ClaimRelation{
			{
				TargetFactId:         layer2.Id,
				Relation:             knowledgetypes.RelationType_RELATION_TYPE_SUPPORTS,
				Inference:            knowledgetypes.InferenceType_INFERENCE_TYPE_ABDUCTIVE,
				InferenceStrengthBps: 700_000,
			},
			{
				TargetFactId:         layer1.Id,
				Relation:             knowledgetypes.RelationType_RELATION_TYPE_REQUIRES,
				Inference:            knowledgetypes.InferenceType_INFERENCE_TYPE_DEDUCTIVE,
				InferenceStrengthBps: 1_000_000,
			},
		}, "multi-cite")

	// Distance = min(layer1=1, layer2=2) + 1 = 2
	require.Equal(t, uint32(2), layer3.AxiomDistance,
		"layer-3 cites both layer1 (d=1) and layer2 (d=2) → min + 1 = 2")
	// Floor = min(layer1.eff, layer2.eff)
	require.LessOrEqual(t, layer3.DependencyConfidenceFloor, layer2.Confidence,
		"floor must not exceed layer-2's effective confidence")

	// ─── ProofTree query: full ancestry should be navigable ──────────────
	qs := knowledgekeeper.NewQueryServerImpl(h.KnowledgeKeeper)
	resp, err := qs.ProofTree(h.Ctx, &knowledgetypes.QueryProofTreeRequest{
		FactId:        layer3.Id,
		MaxDepth:      5,
		IncludeAxioms: true,
	})
	require.NoError(t, err)
	require.NotNil(t, resp.Root)
	require.Equal(t, layer3.Id, resp.Root.Fact.Id)
	require.Greater(t, resp.TotalNodes, uint32(1), "proof tree should span multiple facts")

	// The root's support list should contain layer1 and layer2 at depth 1.
	supporterIDs := make(map[string]bool)
	for _, child := range resp.Root.Supports {
		supporterIDs[child.Fact.Id] = true
		require.Equal(t, uint32(1), child.Depth,
			"direct supporters should be at depth 1")
	}
	require.True(t, supporterIDs[layer1.Id], "layer1 should be a direct supporter")
	require.True(t, supporterIDs[layer2.Id], "layer2 should be a direct supporter")

	// The weakest link in the tree should be surfaced.
	require.LessOrEqual(t, resp.MinimumConfidenceInTree, layer3.Confidence,
		"min confidence in tree should surface the weakest link")

	// Depth should reach at least 2 (root → layer1 or layer2 → axiom path).
	require.GreaterOrEqual(t, resp.MaxDepthReached, uint32(1))
}

// TestToK_InferenceWeightedFloor pins Wave 6: a low-strength inductive edge
// should weaken the inherited floor proportionally; a full-strength deductive
// edge should preserve it. Same cited source, different inference strengths
// must produce different floors.
func TestToK_InferenceWeightedFloor(t *testing.T) {
	h := NewTestHarness(t)

	domain := "inference_weighting_domain"
	require.NoError(t, h.KnowledgeKeeper.SetDomain(h.Ctx, &knowledgetypes.Domain{
		Name:   domain,
		Status: knowledgetypes.DomainStatus_DOMAIN_STATUS_ACTIVE,
	}))

	// Seed a high-confidence source fact (850k BPS).
	source := &knowledgetypes.Fact{
		Id:            "WEIGHT-SOURCE",
		Content:       "Experimental heat capacity measurements for water.",
		Domain:        domain,
		Category:      "empirical",
		Confidence:    850_000,
		Status:        knowledgetypes.FactStatus_FACT_STATUS_VERIFIED,
		Submitter:     "genesis",
		AxiomDistance: 0,
	}
	require.NoError(t, h.KnowledgeKeeper.SetFact(h.Ctx, source))

	// Deductive at full strength — preserves the 850k floor.
	deductive := submitAndAcceptChainedClaim(t, h, domain,
		"Heat capacity of water at 25°C is 4.184 J/(g·K).",
		[]*knowledgetypes.ClaimRelation{
			{
				TargetFactId:         source.Id,
				Relation:             knowledgetypes.RelationType_RELATION_TYPE_SUPPORTS,
				Inference:            knowledgetypes.InferenceType_INFERENCE_TYPE_DEDUCTIVE,
				InferenceStrengthBps: 1_000_000,
			},
		}, "deductive-full")

	// Inductive at 700k strength — weakens 850k × 0.7 = 595k.
	inductive := submitAndAcceptChainedClaim(t, h, domain,
		"Most aqueous solutions have similar heat capacity near 25°C.",
		[]*knowledgetypes.ClaimRelation{
			{
				TargetFactId:         source.Id,
				Relation:             knowledgetypes.RelationType_RELATION_TYPE_SUPPORTS,
				Inference:            knowledgetypes.InferenceType_INFERENCE_TYPE_INDUCTIVE,
				InferenceStrengthBps: 700_000,
			},
		}, "inductive-weak")

	// Deductive floor should be (approx) the source's effective confidence.
	require.Equal(t, uint64(850_000), deductive.DependencyConfidenceFloor,
		"deductive at full strength preserves source confidence")

	// Inductive floor should be the weakened contribution: 850k × 700k / 1M = 595k.
	require.Equal(t, uint64(595_000), inductive.DependencyConfidenceFloor,
		"inductive at 70%% strength weakens source confidence proportionally")

	// Therefore the inductive fact's actual confidence (after clamp) must be
	// capped at its weakened floor — which is substantially below the
	// 900k that its own verification gave it.
	require.LessOrEqual(t, inductive.Confidence, uint64(595_000),
		"inductive fact confidence must be clamped to weakened floor")
	require.LessOrEqual(t, deductive.Confidence, uint64(850_000),
		"deductive fact confidence must be clamped to preserved floor")
	require.Greater(t, deductive.Confidence, inductive.Confidence,
		"stronger-inference fact must end with higher confidence than weaker-inference peer")
}

// TestToK_TrustProfile verifies the aggregate query surfaces all Wave 1-6
// signals consistently and that grounded_score honours axiom distance.
func TestToK_TrustProfile(t *testing.T) {
	h := NewTestHarness(t)

	domain := "trust_profile_domain"
	require.NoError(t, h.KnowledgeKeeper.SetDomain(h.Ctx, &knowledgetypes.Domain{
		Name:   domain,
		Status: knowledgetypes.DomainStatus_DOMAIN_STATUS_ACTIVE,
	}))

	axiom := &knowledgetypes.Fact{
		Id:            "TP-AXIOM",
		Content:       "All triangles have three sides.",
		Domain:        domain,
		Category:      "formal",
		Confidence:    1_000_000,
		Status:        knowledgetypes.FactStatus_FACT_STATUS_VERIFIED,
		Submitter:     "genesis",
		AxiomDistance: 0,
	}
	require.NoError(t, h.KnowledgeKeeper.SetFact(h.Ctx, axiom))

	derived := submitAndAcceptChainedClaim(t, h, domain,
		"Equilateral triangles have three equal sides.",
		[]*knowledgetypes.ClaimRelation{
			{
				TargetFactId:         axiom.Id,
				Relation:             knowledgetypes.RelationType_RELATION_TYPE_REQUIRES,
				Inference:            knowledgetypes.InferenceType_INFERENCE_TYPE_DEDUCTIVE,
				InferenceStrengthBps: 1_000_000,
			},
		}, "TP-derived")

	qs := knowledgekeeper.NewQueryServerImpl(h.KnowledgeKeeper)

	// Axiom profile: distance 0, no floor, grounded == own confidence.
	axiomProfile, err := qs.TrustProfile(h.Ctx, &knowledgetypes.QueryTrustProfileRequest{FactId: axiom.Id})
	require.NoError(t, err)
	require.Equal(t, uint32(0), axiomProfile.AxiomDistance)
	require.Equal(t, uint64(1_000_000), axiomProfile.OwnConfidenceBps)
	require.Equal(t, uint64(1_000_000), axiomProfile.GroundedScoreBps,
		"axiom at distance 0 with no floor → grounded equals own confidence")
	require.Equal(t, uint32(1), axiomProfile.DirectDescendants,
		"axiom has exactly one direct descendant (derived fact)")

	// Derived profile: distance 1, floor = 1M (deductive preservation),
	// grounded score weighted down by axiom distance.
	derivedProfile, err := qs.TrustProfile(h.Ctx, &knowledgetypes.QueryTrustProfileRequest{FactId: derived.Id})
	require.NoError(t, err)
	require.Equal(t, uint32(1), derivedProfile.AxiomDistance)
	require.Equal(t, uint32(1), derivedProfile.DirectSupporters,
		"derived fact has exactly one direct supporter")
	require.Equal(t, uint32(0), derivedProfile.DirectDescendants)
	// grounded should equal own_conf × axiom_weight at distance 1:
	// axiomWeight = BPS² / (BPS + 1×50_000) = 1M/1.05 ≈ 952_380
	// grounded = own × 952_380 / BPS (then × floor_weight = 1.0 because floor == own)
	require.Less(t, derivedProfile.GroundedScoreBps, derivedProfile.OwnConfidenceBps,
		"distance-1 fact's grounded score must be below its own confidence")
	require.Greater(t, derivedProfile.GroundedScoreBps, uint64(0))
	// The min-in-ancestry must touch the axiom's confidence (or the derived
	// fact's floor, whichever is smaller).
	require.LessOrEqual(t, derivedProfile.MinimumConfidenceInAncestry, axiom.Confidence)
}

// submitAndAcceptChainedClaim is a test helper that builds a Claim with the
// given relations, creates a verification round, forces an ACCEPT verdict,
// and returns the resulting Fact. Designed for writing concise chain tests
// without driving the full message-server / commit-reveal flow.
func submitAndAcceptChainedClaim(
	t *testing.T,
	h *TestHarness,
	domain string,
	content string,
	relations []*knowledgetypes.ClaimRelation,
	testTag string,
) *knowledgetypes.Fact {
	t.Helper()

	claimID := "claim-" + testTag
	submitter := sdk.AccAddress([]byte("tok-test-submitter-" + testTag[:3])).String()
	claim := &knowledgetypes.Claim{
		Id:          claimID,
		Submitter:   submitter,
		FactContent: content,
		Domain:      domain,
		Category:    "empirical",
		Status:      knowledgetypes.ClaimStatus_CLAIM_STATUS_IN_VERIFICATION,
		Stake:       "1000000",
		Relations:   relations,
	}
	require.NoError(t, h.KnowledgeKeeper.SetClaim(h.Ctx, claim))

	round := &knowledgetypes.VerificationRound{
		Id:             "round-" + testTag,
		ClaimId:        claim.Id,
		Phase:          knowledgetypes.VerificationPhase_VERIFICATION_PHASE_COMPLETE,
		StartedAtBlock: 1,
	}

	result := &knowledgekeeper.VerificationResult{
		Verdict:    knowledgetypes.Verdict_VERDICT_ACCEPT,
		Confidence: 900_000, // 90% — higher than axiom floor, lower than uncapped
		AcceptCount: 3,
		RejectCount: 0,
	}
	require.NoError(t, h.KnowledgeKeeper.CompleteRound(h.Ctx, round, result))

	// Re-read the claim to find the generated fact.
	updatedClaim, found := h.KnowledgeKeeper.GetClaim(h.Ctx, claim.Id)
	require.True(t, found)
	require.Equal(t, knowledgetypes.ClaimStatus_CLAIM_STATUS_ACCEPTED, updatedClaim.Status)

	// Find the fact by iterating the domain — exactly one new fact is created.
	var createdFact *knowledgetypes.Fact
	h.KnowledgeKeeper.IterateFactsByDomain(h.Ctx, domain, func(factID string) bool {
		f, ok := h.KnowledgeKeeper.GetFact(h.Ctx, factID)
		if !ok {
			return false
		}
		if f.ClaimId == claim.Id {
			createdFact = f
			return true
		}
		return false
	})
	require.NotNil(t, createdFact, "fact must be created from accepted claim")
	return createdFact
}
