package cross_stack_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	knowledgekeeper "github.com/zerone-chain/zerone/x/knowledge/keeper"
	knowledgetypes "github.com/zerone-chain/zerone/x/knowledge/types"
)

// TestToK_FalsificationCascade drives:
//   axiom  ──SUPPORTS──  factB  ──SUPPORTS──  factC
// then disproves axiom via challenge. Expects:
//   · axiom flipped to DISPROVEN
//   · factB flipped to CONTESTED (direct descendant)
//   · factC NOT touched (transitive — not cascaded automatically)
//   · DescendantTree(axiom) finds both B and C
func TestToK_FalsificationCascade(t *testing.T) {
	h := NewTestHarness(t)

	domain := "cascade_test_domain"
	require.NoError(t, h.KnowledgeKeeper.SetDomain(h.Ctx, &knowledgetypes.Domain{
		Name:   domain,
		Status: knowledgetypes.DomainStatus_DOMAIN_STATUS_ACTIVE,
	}))

	// Seed the axiom.
	axiom := &knowledgetypes.Fact{
		Id:            "CASCADE-AXIOM",
		Content:       "Caloric fluid is a substance (historical pre-kinetic-theory claim).",
		Domain:        domain,
		Category:      "empirical",
		Confidence:    900_000,
		Status:        knowledgetypes.FactStatus_FACT_STATUS_VERIFIED,
		Submitter:     "genesis",
		Stratum:       "physical",
		Maturity:      "established",
		AxiomDistance: 0,
	}
	require.NoError(t, h.KnowledgeKeeper.SetFact(h.Ctx, axiom))

	// Seed factB, which cites the axiom.
	factB := submitAndAcceptChainedClaim(t, h, domain,
		"Heat flow obeys caloric conservation in closed systems.",
		[]*knowledgetypes.ClaimRelation{
			{
				TargetFactId:         axiom.Id,
				Relation:             knowledgetypes.RelationType_RELATION_TYPE_REQUIRES,
				Inference:            knowledgetypes.InferenceType_INFERENCE_TYPE_DEDUCTIVE,
				InferenceStrengthBps: 1_000_000,
			},
		}, "factB-cascade")
	require.Equal(t, knowledgetypes.FactStatus_FACT_STATUS_VERIFIED, factB.Status)

	// Seed factC, which cites factB (transitively depends on axiom).
	factC := submitAndAcceptChainedClaim(t, h, domain,
		"Calorimeters accurately measure the flow of caloric.",
		[]*knowledgetypes.ClaimRelation{
			{
				TargetFactId:         factB.Id,
				Relation:             knowledgetypes.RelationType_RELATION_TYPE_SUPPORTS,
				Inference:            knowledgetypes.InferenceType_INFERENCE_TYPE_EMPIRICAL,
				InferenceStrengthBps: 800_000,
			},
		}, "factC-cascade")
	require.Equal(t, knowledgetypes.FactStatus_FACT_STATUS_VERIFIED, factC.Status)

	// ─── Sanity: DescendantTree should find B (depth 1) and C (depth 2) ──
	qs := knowledgekeeper.NewQueryServerImpl(h.KnowledgeKeeper)
	descBefore, err := qs.DescendantTree(h.Ctx, &knowledgetypes.QueryDescendantTreeRequest{
		FactId:   axiom.Id,
		MaxDepth: 5,
	})
	require.NoError(t, err)
	require.GreaterOrEqual(t, descBefore.TotalNodes, uint32(3), "axiom + B + C")
	// Level-1 descendant should be factB.
	require.Len(t, descBefore.Descendants, 1)
	require.Equal(t, factB.Id, descBefore.Descendants[0].Fact.Id)
	// Under B, at depth 2, should be C.
	require.Len(t, descBefore.Descendants[0].Descendants, 1)
	require.Equal(t, factC.Id, descBefore.Descendants[0].Descendants[0].Fact.Id)

	// ─── Disprove the axiom via challenge claim ──────────────────────────
	// Build a challenge claim that was ACCEPTED (the axiom is now disproven).
	// This mirrors the R26-7 vindication path.
	challengeClaim := &knowledgetypes.Claim{
		Id:                "challenge-axiom",
		Submitter:         "challenger",
		FactContent:       "Caloric does not exist; heat is molecular motion.",
		Domain:            domain,
		Category:          "empirical",
		Status:            knowledgetypes.ClaimStatus_CLAIM_STATUS_IN_VERIFICATION,
		Stake:             "11000000",
		ProvisionalFactId: axiom.Id, // challenges the axiom
		Relations: []*knowledgetypes.ClaimRelation{
			{
				TargetFactId: axiom.Id,
				Relation:     knowledgetypes.RelationType_RELATION_TYPE_CONTRADICTS,
			},
		},
	}
	require.NoError(t, h.KnowledgeKeeper.SetClaim(h.Ctx, challengeClaim))

	challengeRound := &knowledgetypes.VerificationRound{
		Id:             "round-challenge-axiom",
		ClaimId:        challengeClaim.Id,
		Phase:          knowledgetypes.VerificationPhase_VERIFICATION_PHASE_COMPLETE,
		StartedAtBlock: 1,
	}
	acceptResult := &knowledgekeeper.VerificationResult{
		Verdict:     knowledgetypes.Verdict_VERDICT_ACCEPT,
		Confidence:  900_000,
		AcceptCount: 3,
	}
	// CompleteRound on an ACCEPT verdict of a challenge claim triggers
	// handleChallengeDisproven → falsification cascade.
	require.NoError(t, h.KnowledgeKeeper.CompleteRound(h.Ctx, challengeRound, acceptResult))

	// ─── Verify cascade ──────────────────────────────────────────────────
	afterAxiom, _ := h.KnowledgeKeeper.GetFact(h.Ctx, axiom.Id)
	require.Equal(t, knowledgetypes.FactStatus_FACT_STATUS_DISPROVEN, afterAxiom.Status,
		"axiom should be DISPROVEN")

	afterB, _ := h.KnowledgeKeeper.GetFact(h.Ctx, factB.Id)
	require.Equal(t, knowledgetypes.FactStatus_FACT_STATUS_CONTESTED, afterB.Status,
		"direct descendant B should be CONTESTED (falsification cascade)")

	afterC, _ := h.KnowledgeKeeper.GetFact(h.Ctx, factC.Id)
	require.Equal(t, knowledgetypes.FactStatus_FACT_STATUS_VERIFIED, afterC.Status,
		"transitive descendant C must NOT auto-cascade — only direct descendants")

	// ─── Verify descendant tree still reachable post-disproof ────────────
	descAfter, err := qs.DescendantTree(h.Ctx, &knowledgetypes.QueryDescendantTreeRequest{
		FactId:   axiom.Id,
		MaxDepth: 5,
	})
	require.NoError(t, err)
	// Root is the (now disproven) axiom — descendants chain still exists.
	require.GreaterOrEqual(t, descAfter.TotalNodes, uint32(3))
	require.Equal(t, knowledgetypes.FactStatus_FACT_STATUS_DISPROVEN, descAfter.Root.Status)
	require.Equal(t, knowledgetypes.FactStatus_FACT_STATUS_CONTESTED,
		descAfter.Descendants[0].Fact.Status)
}
