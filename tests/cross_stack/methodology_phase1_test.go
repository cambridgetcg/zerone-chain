package cross_stack_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	knowledgekeeper "github.com/zerone-chain/zerone/x/knowledge/keeper"
	knowledgetypes "github.com/zerone-chain/zerone/x/knowledge/types"
)

// TestMethodologyPhase1_RegistrySeeded checks the seven bootstrap methodologies
// are present at genesis and queryable. The registry is the bedrock of the
// "methodology over statement" model.
func TestMethodologyPhase1_RegistrySeeded(t *testing.T) {
	h := NewTestHarness(t)
	// This harness's test context does not surface InitChain state (see
	// full_loop_test.go for the same pattern on domain seeding). Trigger
	// the same seeding path keeper.InitGenesis uses.
	require.NoError(t, h.KnowledgeKeeper.SeedDefaultMethodologies(h.Ctx))

	qs := knowledgekeeper.NewQueryServerImpl(h.KnowledgeKeeper)

	resp, err := qs.Methodologies(h.Ctx, &knowledgetypes.QueryMethodologiesRequest{})
	require.NoError(t, err)
	require.Len(t, resp.Methodologies, 7,
		"seven methodologies (6 + legacy) expected in registry")

	ids := make(map[string]bool)
	for _, m := range resp.Methodologies {
		ids[m.Id] = true
	}
	for _, expected := range []string{
		knowledgetypes.MethodologyFormal,
		knowledgetypes.MethodologyEmpirical,
		knowledgetypes.MethodologyComputational,
		knowledgetypes.MethodologyTestimonial,
		knowledgetypes.MethodologyAnalogical,
		knowledgetypes.MethodologyDialectical,
		knowledgetypes.MethodologyLegacy,
	} {
		require.True(t, ids[expected], "methodology %s must be seeded at genesis", expected)
	}

	// Single-fetch sanity.
	one, err := qs.Methodology(h.Ctx, &knowledgetypes.QueryMethodologyRequest{
		Id: knowledgetypes.MethodologyFormal,
	})
	require.NoError(t, err)
	require.True(t, one.Found)
	require.Equal(t, "Formal derivation", one.Methodology.Name)
	require.NotEmpty(t, one.Methodology.ComplianceCriteria)
	require.NotEmpty(t, one.Methodology.FalsificationPaths)

	// Unknown method returns found=false, no error.
	missing, err := qs.Methodology(h.Ctx, &knowledgetypes.QueryMethodologyRequest{Id: "M-DOES-NOT-EXIST"})
	require.NoError(t, err)
	require.False(t, missing.Found)
}

// TestMethodologyPhase1_CrossMethodDiscountsShape asserts a sample of
// cross-method discounts match the design: testimony cannot ground formal
// claims at full strength; analogy cannot prove formal or computational
// claims at full strength.
func TestMethodologyPhase1_CrossMethodDiscountsShape(t *testing.T) {
	h := NewTestHarness(t)
	require.NoError(t, h.KnowledgeKeeper.SeedDefaultMethodologies(h.Ctx))

	testimonial, found := h.KnowledgeKeeper.GetMethodology(h.Ctx, knowledgetypes.MethodologyTestimonial)
	require.True(t, found)
	require.Less(t, testimonial.CrossMethodDiscountBps[knowledgetypes.MethodologyFormal], uint64(600_000),
		"testimony → formal must be heavily discounted")

	analogical, found := h.KnowledgeKeeper.GetMethodology(h.Ctx, knowledgetypes.MethodologyAnalogical)
	require.True(t, found)
	require.Less(t, analogical.CrossMethodDiscountBps[knowledgetypes.MethodologyFormal], uint64(500_000),
		"analogy → formal must be heavily discounted")

	// Formal citing computational: full strength (both strict).
	formal, _ := h.KnowledgeKeeper.GetMethodology(h.Ctx, knowledgetypes.MethodologyFormal)
	require.Equal(t, uint64(1_000_000), formal.CrossMethodDiscountBps[knowledgetypes.MethodologyComputational])
}

// TestMethodologyPhase1_MethodIdPropagates checks that a claim declaring
// method_id carries it through to the resulting Fact after acceptance.
func TestMethodologyPhase1_MethodIdPropagates(t *testing.T) {
	h := NewTestHarness(t)

	domain := "methodology_propagation_domain"
	require.NoError(t, h.KnowledgeKeeper.SetDomain(h.Ctx, &knowledgetypes.Domain{
		Name:   domain,
		Status: knowledgetypes.DomainStatus_DOMAIN_STATUS_ACTIVE,
	}))

	// Submit claim declaring M-EMPIRICAL.
	claim := &knowledgetypes.Claim{
		Id:          "method-claim-empirical",
		Submitter:   "tester",
		FactContent: "Water's heat capacity is ~4.184 J/(g·K) at 25°C.",
		Domain:      domain,
		Category:    "empirical",
		Status:      knowledgetypes.ClaimStatus_CLAIM_STATUS_IN_VERIFICATION,
		Stake:       "1000000",
		MethodId:    knowledgetypes.MethodologyEmpirical,
	}
	require.NoError(t, h.KnowledgeKeeper.SetClaim(h.Ctx, claim))

	round := &knowledgetypes.VerificationRound{
		Id:             "round-method",
		ClaimId:        claim.Id,
		Phase:          knowledgetypes.VerificationPhase_VERIFICATION_PHASE_COMPLETE,
		StartedAtBlock: 1,
	}
	result := &knowledgekeeper.VerificationResult{
		Verdict:     knowledgetypes.Verdict_VERDICT_ACCEPT,
		Confidence:  900_000,
		AcceptCount: 3,
	}
	require.NoError(t, h.KnowledgeKeeper.CompleteRound(h.Ctx, round, result))

	// Locate the created fact and assert method_id was carried.
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
	require.NotNil(t, createdFact)
	require.Equal(t, knowledgetypes.MethodologyEmpirical, createdFact.MethodId,
		"Fact must carry the methodology declared on the originating Claim")
}

// TestMethodologyPhase1_LegacyDefaultForUnspecified asserts that a claim
// submitted WITHOUT declaring method_id gets M-LEGACY on its Fact. This is
// the transitional behavior until the legacy sunset.
func TestMethodologyPhase1_LegacyDefaultForUnspecified(t *testing.T) {
	h := NewTestHarness(t)

	domain := "legacy_default_domain"
	require.NoError(t, h.KnowledgeKeeper.SetDomain(h.Ctx, &knowledgetypes.Domain{
		Name:   domain,
		Status: knowledgetypes.DomainStatus_DOMAIN_STATUS_ACTIVE,
	}))

	// Claim with NO method_id declared.
	claim := &knowledgetypes.Claim{
		Id:          "method-claim-unspecified",
		Submitter:   "tester",
		FactContent: "A claim submitted without declaring a methodology.",
		Domain:      domain,
		Category:    "empirical",
		Status:      knowledgetypes.ClaimStatus_CLAIM_STATUS_IN_VERIFICATION,
		Stake:       "1000000",
		// MethodId: "" — not set
	}
	require.NoError(t, h.KnowledgeKeeper.SetClaim(h.Ctx, claim))

	round := &knowledgetypes.VerificationRound{
		Id:             "round-legacy-default",
		ClaimId:        claim.Id,
		Phase:          knowledgetypes.VerificationPhase_VERIFICATION_PHASE_COMPLETE,
		StartedAtBlock: 1,
	}
	result := &knowledgekeeper.VerificationResult{
		Verdict:     knowledgetypes.Verdict_VERDICT_ACCEPT,
		Confidence:  800_000,
		AcceptCount: 3,
	}
	require.NoError(t, h.KnowledgeKeeper.CompleteRound(h.Ctx, round, result))

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
	require.NotNil(t, createdFact)
	require.Equal(t, knowledgetypes.MethodologyLegacy, createdFact.MethodId,
		"undeclared method must default to M-LEGACY (transitional)")
}
