package keeper_test

import (
	"fmt"
	"sort"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/zerone-chain/zerone/x/knowledge/keeper"
	"github.com/zerone-chain/zerone/x/knowledge/types"
)

// ─── Claim Type Data Model Tests ─────────────────────────────────────────────
//
// The Claim protobuf type stores category/domain/etc. as strings.
// Server-side claim type validation is in types.ValidClaimTypes and
// types.ValidateAxioms. Here we test data model round-trips through
// SetClaim/GetClaim and that the type system constants are self-consistent.

func TestClaimType_Axiom(t *testing.T) {
	k, ctx := setupKnowledgeTest(t)

	claim := &types.Claim{
		Id:          "ct-axiom",
		FactContent: "Every object is identical to itself.",
		Domain:      "mathematics",
		Category:    "analytic",
		Submitter:   "zrn1submitter1",
		Status:      types.ClaimStatus_CLAIM_STATUS_PENDING,
	}
	require.NoError(t, k.SetClaim(ctx, claim))

	got, found := k.GetClaim(ctx, "ct-axiom")
	require.True(t, found)
	require.Equal(t, "mathematics", got.Domain)
	require.Equal(t, "analytic", got.Category)

	// "axiom" is a valid claim type
	require.True(t, types.ValidClaimTypes["axiom"])
}

func TestClaimType_EmpiricalAxiom(t *testing.T) {
	k, ctx := setupKnowledgeTest(t)

	claim := &types.Claim{
		Id:          "ct-empirical",
		FactContent: "The speed of light in vacuum is approximately 3e8 m/s.",
		Domain:      "physics",
		Category:    "empirical",
		Submitter:   "zrn1submitter1",
		Status:      types.ClaimStatus_CLAIM_STATUS_PENDING,
	}
	require.NoError(t, k.SetClaim(ctx, claim))

	got, found := k.GetClaim(ctx, "ct-empirical")
	require.True(t, found)
	require.Equal(t, "physics", got.Domain)
	require.Equal(t, "empirical", got.Category)

	require.True(t, types.ValidClaimTypes["empirical_axiom"])
}

func TestClaimType_Definition(t *testing.T) {
	k, ctx := setupKnowledgeTest(t)

	claim := &types.Claim{
		Id:          "ct-definition",
		FactContent: "A group is a set with an associative binary operation, identity element, and inverses.",
		Domain:      "mathematics",
		Category:    "analytic",
		Submitter:   "zrn1submitter1",
		Status:      types.ClaimStatus_CLAIM_STATUS_PENDING,
	}
	require.NoError(t, k.SetClaim(ctx, claim))

	got, found := k.GetClaim(ctx, "ct-definition")
	require.True(t, found)
	require.Equal(t, "mathematics", got.Domain)

	require.True(t, types.ValidClaimTypes["definition"])
}

func TestClaimType_RegimeDeclaration(t *testing.T) {
	require.True(t, types.ValidClaimTypes["regime_declaration"])

	// Verify it maps to "formal" category
	cat, ok := types.ClaimTypeToCategory["regime_declaration"]
	require.True(t, ok)
	require.Equal(t, "formal", cat)
}

func TestClaimType_DerivedClaim(t *testing.T) {
	k, ctx := setupKnowledgeTest(t)

	claim := &types.Claim{
		Id:          "ct-derived",
		FactContent: "From axioms A and B, conclusion C follows.",
		Domain:      "logic",
		Category:    "formal",
		Submitter:   "zrn1submitter1",
		References:  []string{"axiom-A", "axiom-B"},
		Status:      types.ClaimStatus_CLAIM_STATUS_PENDING,
	}
	require.NoError(t, k.SetClaim(ctx, claim))

	got, found := k.GetClaim(ctx, "ct-derived")
	require.True(t, found)
	require.Equal(t, []string{"axiom-A", "axiom-B"}, got.References)

	require.True(t, types.ValidClaimTypes["derived_claim"])
}

func TestClaimType_DerivedClaim_NoDeps_Rejected(t *testing.T) {
	// ValidateAxioms rejects derived_claim with no dependencies
	axioms := []*types.GenesisAxiom{
		{
			AxiomID:           "MATH-001",
			Statement:         "Derived with no deps",
			ClaimType:         "derived_claim",
			Domain:            "mathematics",
			EpistemicCategory: "formal",
			Confidence:        0.95,
			Dependencies:      nil, // no deps — should fail
		},
	}
	err := types.ValidateAxioms(axioms, []string{"mathematics"})
	require.Error(t, err)
	require.Contains(t, err.Error(), "no dependencies")
}

func TestClaimType_MeasurementFact(t *testing.T) {
	require.True(t, types.ValidClaimTypes["measurement_fact"])

	cat, ok := types.ClaimTypeToCategory["measurement_fact"]
	require.True(t, ok)
	require.Equal(t, "empirical", cat)

	// Default confidence for measurement_fact
	conf, ok := types.ClaimTypeDefaultConfidence["measurement_fact"]
	require.True(t, ok)
	require.Equal(t, 0.90, conf)
}

func TestClaimType_Meta(t *testing.T) {
	require.True(t, types.ValidClaimTypes["meta"])

	cat, ok := types.ClaimTypeToCategory["meta"]
	require.True(t, ok)
	require.Equal(t, "empirical", cat)

	conf, ok := types.ClaimTypeDefaultConfidence["meta"]
	require.True(t, ok)
	require.Equal(t, 0.85, conf)
}

func TestClaimType_Invalid_Rejected(t *testing.T) {
	// ValidateAxioms rejects unknown claim types
	axioms := []*types.GenesisAxiom{
		{
			AxiomID:           "MATH-001",
			Statement:         "Test invalid type",
			ClaimType:         "nonexistent_type",
			Domain:            "mathematics",
			EpistemicCategory: "analytic",
			Confidence:        1.0,
		},
	}
	err := types.ValidateAxioms(axioms, []string{"mathematics"})
	require.Error(t, err)
	require.Contains(t, err.Error(), "invalid claim type")
}

func TestClaimType_AllValidTypes(t *testing.T) {
	expected := []string{
		"axiom", "empirical_axiom", "definition",
		"regime_declaration", "derived_claim", "measurement_fact", "meta",
	}

	require.Len(t, types.ValidClaimTypes, len(expected),
		"expected %d valid claim types", len(expected))

	for _, ct := range expected {
		require.True(t, types.ValidClaimTypes[ct],
			"claim type %q should be in ValidClaimTypes", ct)
	}
}

func TestClaimType_ConfidenceModels(t *testing.T) {
	// Each valid claim type must have a default confidence
	for ct := range types.ValidClaimTypes {
		conf, ok := types.ClaimTypeDefaultConfidence[ct]
		require.True(t, ok, "claim type %q must have a default confidence", ct)
		require.Greater(t, conf, 0.0, "default confidence for %q must be positive", ct)
		require.LessOrEqual(t, conf, 1.0, "default confidence for %q must be <= 1.0", ct)
	}

	// Confidence ordering: axiom >= definition >= empirical_axiom >= derived_claim >= measurement_fact >= meta
	require.GreaterOrEqual(t, types.ClaimTypeDefaultConfidence["axiom"],
		types.ClaimTypeDefaultConfidence["definition"])
	require.GreaterOrEqual(t, types.ClaimTypeDefaultConfidence["definition"],
		types.ClaimTypeDefaultConfidence["empirical_axiom"])
	require.GreaterOrEqual(t, types.ClaimTypeDefaultConfidence["empirical_axiom"],
		types.ClaimTypeDefaultConfidence["derived_claim"])
	require.GreaterOrEqual(t, types.ClaimTypeDefaultConfidence["derived_claim"],
		types.ClaimTypeDefaultConfidence["measurement_fact"])
	require.GreaterOrEqual(t, types.ClaimTypeDefaultConfidence["measurement_fact"],
		types.ClaimTypeDefaultConfidence["meta"])
}

func TestClaimType_CategoryCompatibility(t *testing.T) {
	// Every valid claim type must map to a valid epistemic category
	for ct := range types.ValidClaimTypes {
		cat, ok := types.ClaimTypeToCategory[ct]
		require.True(t, ok, "claim type %q must have a category mapping", ct)
		require.True(t, types.ValidAxiomCategories[cat],
			"claim type %q maps to invalid category %q", ct, cat)
	}
}

func TestClaimType_StakeRequirements(t *testing.T) {
	// StratumStakeMultiplier must cover all valid strata
	for stratum := range types.ValidStrata {
		mult, ok := types.StratumStakeMultiplier[stratum]
		require.True(t, ok, "stratum %q must have a stake multiplier", stratum)
		require.Greater(t, mult, uint64(0), "multiplier for %q must be > 0", stratum)
	}

	// Fundamental stratum should have the highest multiplier
	require.Greater(t, types.StratumStakeMultiplier["fundamental"],
		types.StratumStakeMultiplier["technological"],
		"fundamental stratum should require higher stake than technological")
}

func TestClaimType_SubmissionValidation(t *testing.T) {
	// ValidateAxioms requires non-empty statements
	axioms := []*types.GenesisAxiom{
		{
			AxiomID:           "MATH-001",
			Statement:         "",
			ClaimType:         "axiom",
			Domain:            "mathematics",
			EpistemicCategory: "analytic",
			Confidence:        1.0,
		},
	}
	err := types.ValidateAxioms(axioms, []string{"mathematics"})
	require.Error(t, err)
	require.Contains(t, err.Error(), "empty statement")
}

func TestClaimType_ContentHashDedup(t *testing.T) {
	k, ctx := setupKnowledgeTest(t)

	content := "Water boils at 100C at standard atmospheric pressure"
	domain := "physics"
	contentHash := keeper.ComputeClaimContentHash(content, domain)

	claim1 := &types.Claim{
		Id:          "ct-dedup-1",
		FactContent: content,
		Domain:      domain,
		ContentHash: contentHash,
		Submitter:   "zrn1sub1",
		Status:      types.ClaimStatus_CLAIM_STATUS_PENDING,
	}
	require.NoError(t, k.SetClaim(ctx, claim1))

	// Looking up by content hash should find original
	foundID, found := k.GetClaimByContentHash(ctx, contentHash)
	require.True(t, found)
	require.Equal(t, "ct-dedup-1", foundID)

	// Same content, different domain = different hash
	otherHash := keeper.ComputeClaimContentHash(content, "chemistry")
	require.NotEqual(t, contentHash, otherHash)
}

func TestClaimType_DomainRequired(t *testing.T) {
	// ValidateAxioms requires a valid domain
	axioms := []*types.GenesisAxiom{
		{
			AxiomID:           "MATH-001",
			Statement:         "Some statement",
			ClaimType:         "axiom",
			Domain:            "nonexistent_domain",
			EpistemicCategory: "analytic",
			Confidence:        1.0,
		},
	}
	err := types.ValidateAxioms(axioms, []string{"mathematics"})
	require.Error(t, err)
	require.Contains(t, err.Error(), "unknown domain")
}

func TestClaimType_SubmitterRequired(t *testing.T) {
	k, ctx := setupKnowledgeTest(t)

	// At the keeper level, claims are data objects — submitter can be stored
	// as empty. This tests the data model accepts it (it's the msg_server
	// that would enforce submitter presence).
	claim := &types.Claim{
		Id:          "ct-no-submitter",
		FactContent: "Claim without submitter",
		Domain:      "mathematics",
		Submitter:   "", // no submitter
		Status:      types.ClaimStatus_CLAIM_STATUS_PENDING,
	}
	require.NoError(t, k.SetClaim(ctx, claim))

	got, found := k.GetClaim(ctx, "ct-no-submitter")
	require.True(t, found)
	require.Empty(t, got.Submitter, "submitter should be empty as stored")
}

func TestClaimType_StratumMapping(t *testing.T) {
	// Every domain in BootstrapDomainStrata must map to a valid stratum
	for domain, stratum := range types.BootstrapDomainStrata {
		require.True(t, types.ValidStrata[stratum],
			"domain %q maps to invalid stratum %q", domain, stratum)
	}
}

func TestClaimType_FundamentalityScore(t *testing.T) {
	// Strata multipliers should increase with ontological depth:
	// fundamental > physical > chemical >= biological > cognitive > social > technological
	require.Greater(t, types.StratumStakeMultiplier["fundamental"],
		types.StratumStakeMultiplier["physical"])
	require.Greater(t, types.StratumStakeMultiplier["physical"],
		types.StratumStakeMultiplier["biological"])
	require.Greater(t, types.StratumStakeMultiplier["biological"],
		types.StratumStakeMultiplier["cognitive"])
	require.Greater(t, types.StratumStakeMultiplier["cognitive"],
		types.StratumStakeMultiplier["social"])
	require.GreaterOrEqual(t, types.StratumStakeMultiplier["social"],
		types.StratumStakeMultiplier["technological"])
}

func TestClaimType_ReferenceValidation(t *testing.T) {
	// ValidateAxioms checks that dependencies point to existing axioms
	axioms := []*types.GenesisAxiom{
		{
			AxiomID:           "MATH-001",
			Statement:         "Base axiom",
			ClaimType:         "axiom",
			Domain:            "mathematics",
			EpistemicCategory: "analytic",
			Confidence:        1.0,
		},
		{
			AxiomID:           "MATH-002",
			Statement:         "Derived from MATH-999 which does not exist",
			ClaimType:         "derived_claim",
			Domain:            "mathematics",
			EpistemicCategory: "formal",
			Confidence:        0.95,
			Dependencies:      []string{"MATH-999"}, // does not exist
		},
	}
	err := types.ValidateAxioms(axioms, []string{"mathematics"})
	require.Error(t, err)
	require.Contains(t, err.Error(), "MATH-999")
}

// ─── Extended Claim Data Model Tests ─────────────────────────────────────────

func TestClaimType_ClaimTypeToCategory_Complete(t *testing.T) {
	// ClaimTypeToCategory must cover every ValidClaimType
	for ct := range types.ValidClaimTypes {
		_, ok := types.ClaimTypeToCategory[ct]
		require.True(t, ok, "ClaimTypeToCategory must include %q", ct)
	}
}

func TestClaimType_ConfidenceValidation_OutOfRange(t *testing.T) {
	// Confidence > 1.0 is rejected
	axioms := []*types.GenesisAxiom{
		{
			AxiomID:           "MATH-001",
			Statement:         "Over-confident axiom",
			ClaimType:         "axiom",
			Domain:            "mathematics",
			EpistemicCategory: "analytic",
			Confidence:        1.5,
		},
	}
	err := types.ValidateAxioms(axioms, []string{"mathematics"})
	require.Error(t, err)
	require.Contains(t, err.Error(), "confidence")

	// Confidence < 0 is rejected
	axioms[0].Confidence = -0.1
	err = types.ValidateAxioms(axioms, []string{"mathematics"})
	require.Error(t, err)
	require.Contains(t, err.Error(), "confidence")
}

func TestClaimType_DomainPrefixConsistency(t *testing.T) {
	// ValidateAxioms checks that axiom ID prefix matches domain
	axioms := []*types.GenesisAxiom{
		{
			AxiomID:           "PHYS-001",  // prefix implies physics
			Statement:         "A math statement in physics domain",
			ClaimType:         "axiom",
			Domain:            "mathematics", // mismatch!
			EpistemicCategory: "analytic",
			Confidence:        1.0,
		},
	}
	err := types.ValidateAxioms(axioms, []string{"mathematics", "physics"})
	require.Error(t, err)
	require.Contains(t, err.Error(), "prefix")
}

func TestClaimType_ValidEpistemicCategories(t *testing.T) {
	// All 5 epistemic categories should be present
	expected := []string{"analytic", "formal", "empirical", "protocol", "computational"}
	require.Len(t, types.ValidAxiomCategories, len(expected))
	for _, cat := range expected {
		require.True(t, types.ValidAxiomCategories[cat],
			"expected epistemic category %q", cat)
	}
}

func TestClaimType_IterateAllClaims(t *testing.T) {
	k, ctx := setupKnowledgeTest(t)

	// Store claims with different categories
	categories := []string{"analytic", "formal", "empirical"}
	for i, cat := range categories {
		claim := &types.Claim{
			Id:          fmt.Sprintf("ct-iter-%d", i),
			FactContent: fmt.Sprintf("Claim in category %s for iteration test", cat),
			Domain:      "mathematics",
			Category:    cat,
			Submitter:   "zrn1sub1",
			Status:      types.ClaimStatus_CLAIM_STATUS_PENDING,
		}
		require.NoError(t, k.SetClaim(ctx, claim))
	}

	// Iterate and collect
	var collected []string
	k.IterateClaims(ctx, func(claim *types.Claim) bool {
		collected = append(collected, claim.Category)
		return false
	})
	sort.Strings(collected)
	sort.Strings(categories)
	require.Equal(t, categories, collected)
}

func TestComputationalClaimTypeExists(t *testing.T) {
	ct := types.ClaimType_CLAIM_TYPE_COMPUTATIONAL
	require.Equal(t, int32(7), int32(ct))
	require.Equal(t, "CLAIM_TYPE_COMPUTATIONAL", types.ClaimType_name[7])
}
