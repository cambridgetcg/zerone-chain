package cross_stack_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	knowledgekeeper "github.com/zerone-chain/zerone/x/knowledge/keeper"
	knowledgetypes "github.com/zerone-chain/zerone/x/knowledge/types"
)

// TestMethodologyPhase2_ThirteenMethodologiesSeeded verifies that Phase 2
// expands the registry to 13: the original seven plus the six philosophical
// additions (pragmatic, coherentist, phenomenological, historical, ecological,
// practice).
func TestMethodologyPhase2_ThirteenMethodologiesSeeded(t *testing.T) {
	h := NewTestHarness(t)
	require.NoError(t, h.KnowledgeKeeper.SeedDefaultMethodologies(h.Ctx))

	qs := knowledgekeeper.NewQueryServerImpl(h.KnowledgeKeeper)
	resp, err := qs.Methodologies(h.Ctx, &knowledgetypes.QueryMethodologiesRequest{})
	require.NoError(t, err)
	require.Len(t, resp.Methodologies, 13,
		"13 methodologies expected (6 bootstrap + 6 philosophical + legacy)")

	expected := []string{
		knowledgetypes.MethodologyFormal,
		knowledgetypes.MethodologyEmpirical,
		knowledgetypes.MethodologyComputational,
		knowledgetypes.MethodologyTestimonial,
		knowledgetypes.MethodologyAnalogical,
		knowledgetypes.MethodologyDialectical,
		knowledgetypes.MethodologyPragmatic,
		knowledgetypes.MethodologyCoherentist,
		knowledgetypes.MethodologyPhenomenologic,
		knowledgetypes.MethodologyHistorical,
		knowledgetypes.MethodologyEcological,
		knowledgetypes.MethodologyPractice,
		knowledgetypes.MethodologyLegacy,
	}
	present := make(map[string]bool)
	for _, m := range resp.Methodologies {
		present[m.Id] = true
	}
	for _, id := range expected {
		require.True(t, present[id], "methodology %s must be seeded", id)
	}
}

// TestMethodologyPhase2_PhilosophicalDiscountShape pins specific cross-method
// discounts that reflect the philosophical design:
//   · phenomenology cannot ground formal proof (very weak cross-discount)
//   · ecological knowledge cannot ground computational claims
//   · pragmatism citing empirical is strong (both about consequences)
//   · coherentism citing historical is strong (both about fit within a corpus)
func TestMethodologyPhase2_PhilosophicalDiscountShape(t *testing.T) {
	h := NewTestHarness(t)
	require.NoError(t, h.KnowledgeKeeper.SeedDefaultMethodologies(h.Ctx))

	pheno, found := h.KnowledgeKeeper.GetMethodology(h.Ctx, knowledgetypes.MethodologyPhenomenologic)
	require.True(t, found)
	require.LessOrEqual(t, pheno.CrossMethodDiscountBps[knowledgetypes.MethodologyFormal], uint64(300_000),
		"phenomenology → formal must be heavily discounted")

	eco, found := h.KnowledgeKeeper.GetMethodology(h.Ctx, knowledgetypes.MethodologyEcological)
	require.True(t, found)
	require.LessOrEqual(t, eco.CrossMethodDiscountBps[knowledgetypes.MethodologyComputational], uint64(300_000),
		"ecological → computational must be heavily discounted")

	prag, found := h.KnowledgeKeeper.GetMethodology(h.Ctx, knowledgetypes.MethodologyPragmatic)
	require.True(t, found)
	require.GreaterOrEqual(t, prag.CrossMethodDiscountBps[knowledgetypes.MethodologyEmpirical], uint64(700_000),
		"pragmatism → empirical should be strong (both concerned with consequences)")

	coh, found := h.KnowledgeKeeper.GetMethodology(h.Ctx, knowledgetypes.MethodologyCoherentist)
	require.True(t, found)
	require.GreaterOrEqual(t, coh.CrossMethodDiscountBps[knowledgetypes.MethodologyHistorical], uint64(700_000),
		"coherentism → historical should be strong (both about fit within a corpus)")
}

// TestMethodologyPhase2_PopperianCorroboration exercises the core Phase 2
// mechanism: a fact that survives a falsification attempt accrues a
// corroboration count. Truth in the Popperian sense is not "confidence at
// verification" but "tests survived."
func TestMethodologyPhase2_PopperianCorroboration(t *testing.T) {
	h := NewTestHarness(t)
	require.NoError(t, h.KnowledgeKeeper.SeedDefaultMethodologies(h.Ctx))

	domain := "corroboration_domain"
	require.NoError(t, h.KnowledgeKeeper.SetDomain(h.Ctx, &knowledgetypes.Domain{
		Name:   domain,
		Status: knowledgetypes.DomainStatus_DOMAIN_STATUS_ACTIVE,
	}))

	// Seed a fact that will be challenged.
	targetFact := &knowledgetypes.Fact{
		Id:         "CORROBORATION-TARGET",
		Content:    "Entropy of an isolated system never decreases.",
		Domain:     domain,
		Category:   "empirical",
		Confidence: 900_000,
		Status:     knowledgetypes.FactStatus_FACT_STATUS_VERIFIED,
		Submitter:  "genesis",
		MethodId:   knowledgetypes.MethodologyEmpirical,
	}
	require.NoError(t, h.KnowledgeKeeper.SetFact(h.Ctx, targetFact))
	require.Equal(t, uint64(0), targetFact.CorroborationCount,
		"freshly-created fact has not yet been corroborated")

	// Helper: submit a challenge claim against the target, complete its round
	// with REJECT verdict (the challenge failed — the fact survived).
	issueChallenge := func(id string) {
		challenge := &knowledgetypes.Claim{
			Id:                id,
			Submitter:         "challenger",
			FactContent:       "Attempted falsification of " + targetFact.Id,
			Domain:            domain,
			Category:          "empirical",
			Status:            knowledgetypes.ClaimStatus_CLAIM_STATUS_IN_VERIFICATION,
			Stake:             "11000000",
			ProvisionalFactId: targetFact.Id,
			MethodId:          knowledgetypes.MethodologyDialectical,
			Relations: []*knowledgetypes.ClaimRelation{
				{
					TargetFactId: targetFact.Id,
					Relation:     knowledgetypes.RelationType_RELATION_TYPE_CONTRADICTS,
				},
			},
		}
		require.NoError(t, h.KnowledgeKeeper.SetClaim(h.Ctx, challenge))

		round := &knowledgetypes.VerificationRound{
			Id:             "round-" + id,
			ClaimId:        challenge.Id,
			Phase:          knowledgetypes.VerificationPhase_VERIFICATION_PHASE_COMPLETE,
			StartedAtBlock: 1,
		}
		// REJECT verdict on a challenge claim = the challenge failed = fact survived.
		rejectResult := &knowledgekeeper.VerificationResult{
			Verdict:     knowledgetypes.Verdict_VERDICT_REJECT,
			Confidence:  850_000,
			RejectCount: 3,
		}
		require.NoError(t, h.KnowledgeKeeper.CompleteRound(h.Ctx, round, rejectResult))
	}

	// Three challenges, all rejected — the fact should accrue three
	// corroboration counts.
	issueChallenge("challenge-1")
	issueChallenge("challenge-2")
	issueChallenge("challenge-3")

	updatedFact, found := h.KnowledgeKeeper.GetFact(h.Ctx, targetFact.Id)
	require.True(t, found)
	require.Equal(t, uint64(3), updatedFact.CorroborationCount,
		"three failed falsifications → corroboration count 3")
	require.Greater(t, updatedFact.LastCorroboratedBlock, uint64(0),
		"last_corroborated_block must be set to the most recent survival")

	// TrustProfile surfaces corroboration.
	qs := knowledgekeeper.NewQueryServerImpl(h.KnowledgeKeeper)
	profile, err := qs.TrustProfile(h.Ctx, &knowledgetypes.QueryTrustProfileRequest{
		FactId: targetFact.Id,
	})
	require.NoError(t, err)
	require.Equal(t, uint64(3), profile.CorroborationCount,
		"TrustProfile must surface the Popperian survival count")
	require.Equal(t, knowledgetypes.MethodologyEmpirical, profile.MethodId,
		"TrustProfile must echo the fact's declared methodology")
}

// TestMethodologyPhase2_CorroborationBoostsGroundedScore verifies the
// Popperian move: a corroborated fact earns a higher grounded_score than
// an identical-otherwise uncorroborated fact. This is the mechanism that
// makes survived-falsification count in ranking.
func TestMethodologyPhase2_CorroborationBoostsGroundedScore(t *testing.T) {
	h := NewTestHarness(t)
	require.NoError(t, h.KnowledgeKeeper.SeedDefaultMethodologies(h.Ctx))

	domain := "corroboration_score_domain"
	require.NoError(t, h.KnowledgeKeeper.SetDomain(h.Ctx, &knowledgetypes.Domain{
		Name:   domain,
		Status: knowledgetypes.DomainStatus_DOMAIN_STATUS_ACTIVE,
	}))

	// Two facts, identical except corroboration_count.
	uncorroborated := &knowledgetypes.Fact{
		Id:                 "FACT-UNCORROBORATED",
		Content:            "Uncorroborated claim.",
		Domain:             domain,
		Category:           "empirical",
		Confidence:         700_000,
		Status:             knowledgetypes.FactStatus_FACT_STATUS_VERIFIED,
		Submitter:          "test",
		AxiomDistance:      1,
		CorroborationCount: 0,
	}
	corroborated := &knowledgetypes.Fact{
		Id:                 "FACT-CORROBORATED",
		Content:            "Same claim, but survived five challenges.",
		Domain:             domain,
		Category:           "empirical",
		Confidence:         700_000,
		Status:             knowledgetypes.FactStatus_FACT_STATUS_VERIFIED,
		Submitter:          "test",
		AxiomDistance:      1,
		CorroborationCount: 5,
	}
	require.NoError(t, h.KnowledgeKeeper.SetFact(h.Ctx, uncorroborated))
	require.NoError(t, h.KnowledgeKeeper.SetFact(h.Ctx, corroborated))

	qs := knowledgekeeper.NewQueryServerImpl(h.KnowledgeKeeper)
	uncProfile, err := qs.TrustProfile(h.Ctx, &knowledgetypes.QueryTrustProfileRequest{
		FactId: uncorroborated.Id,
	})
	require.NoError(t, err)
	corrProfile, err := qs.TrustProfile(h.Ctx, &knowledgetypes.QueryTrustProfileRequest{
		FactId: corroborated.Id,
	})
	require.NoError(t, err)

	require.Greater(t, corrProfile.GroundedScoreBps, uncProfile.GroundedScoreBps,
		"corroborated fact must outrank its uncorroborated twin")

	// A corroborated fact's grounded_score can exceed its initial confidence
	// — Popperian evidence accumulation above the initial verification.
	require.Greater(t, corrProfile.GroundedScoreBps, corroborated.Confidence,
		"5 corroborations should push grounded_score above 700_000 own_confidence")

	// But still bounded at BPS (100%).
	require.LessOrEqual(t, corrProfile.GroundedScoreBps, uint64(1_000_000),
		"grounded_score is absolutely capped at BPS regardless of corroboration")
}
