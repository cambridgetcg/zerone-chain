package types

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestValidateAxiomID_Valid(t *testing.T) {
	valid := []string{
		"MATH-001", "PHYS-111", "AGRT-004a", "AGRT-004a-ii",
		"CS-001", "THEO-007", "AP-001", "LOGIC-077",
	}
	for _, id := range valid {
		require.NoError(t, ValidateAxiomID(id), "expected valid: %s", id)
	}
}

func TestValidateAxiomID_Invalid(t *testing.T) {
	invalid := []string{
		"",             // empty
		"math-001",     // lowercase prefix
		"MATH001",      // no dash
		"MATH-01",      // only 2 digits
		"UNKNOWN-001",  // unknown prefix
		"MATH 001",     // space instead of dash
	}
	for _, id := range invalid {
		require.Error(t, ValidateAxiomID(id), "expected invalid: %s", id)
	}
}

func TestParseAxioms_ValidJSON(t *testing.T) {
	data := `[
		{"axiom_id": "MATH-001", "statement": "test", "type": "axiom", "domain": "mathematics", "epistemic_category": "analytic", "confidence": 1.0, "dependencies": []},
		{"axiom_id": "MATH-002", "statement": "test2", "type": "axiom", "domain": "mathematics", "epistemic_category": "analytic", "confidence": 1.0, "dependencies": ["MATH-001"]}
	]`
	axioms, err := ParseAxioms([]byte(data))
	require.NoError(t, err)
	require.Len(t, axioms, 2)
	require.Equal(t, "MATH-001", axioms[0].AxiomID)
	require.Equal(t, []string{"MATH-001"}, axioms[1].Dependencies)
}

func TestParseAxioms_InvalidJSON(t *testing.T) {
	_, err := ParseAxioms([]byte(`not json`))
	require.Error(t, err)
}

func TestValidateAxioms_Empty(t *testing.T) {
	err := ValidateAxioms(nil, []string{"mathematics"})
	require.Error(t, err)
	require.Contains(t, err.Error(), "empty")
}

func TestValidateAxioms_DuplicateID(t *testing.T) {
	axioms := []*GenesisAxiom{
		{AxiomID: "MATH-001", Statement: "a", ClaimType: "axiom", Domain: "mathematics", EpistemicCategory: "analytic", Confidence: 1.0},
		{AxiomID: "MATH-001", Statement: "b", ClaimType: "axiom", Domain: "mathematics", EpistemicCategory: "analytic", Confidence: 1.0},
	}
	err := ValidateAxioms(axioms, []string{"mathematics"})
	require.Error(t, err)
	require.Contains(t, err.Error(), "duplicate")
}

func TestValidateAxiomDAG_NoCycle(t *testing.T) {
	axioms := []*GenesisAxiom{
		{AxiomID: "MATH-001", Dependencies: nil},
		{AxiomID: "MATH-002", Dependencies: []string{"MATH-001"}},
		{AxiomID: "MATH-003", Dependencies: []string{"MATH-001", "MATH-002"}},
	}
	require.NoError(t, ValidateAxiomDAG(axioms))
}

func TestValidateAxiomDAG_Cycle(t *testing.T) {
	axioms := []*GenesisAxiom{
		{AxiomID: "MATH-001", Dependencies: []string{"MATH-002"}},
		{AxiomID: "MATH-002", Dependencies: []string{"MATH-001"}},
	}
	err := ValidateAxiomDAG(axioms)
	require.Error(t, err)
	require.Contains(t, err.Error(), "cycle")
}

func TestValidateAxiomDAG_SelfDependency(t *testing.T) {
	axioms := []*GenesisAxiom{
		{AxiomID: "MATH-001", Dependencies: []string{"MATH-001"}},
	}
	err := ValidateAxiomDAG(axioms)
	require.Error(t, err)
	require.Contains(t, err.Error(), "self-dependency")
}

func TestValidateStratumConsistency_Valid(t *testing.T) {
	axioms := []*GenesisAxiom{
		{AxiomID: "MATH-001", ClaimType: "axiom", Dependencies: nil},
		{AxiomID: "MATH-002", ClaimType: "definition", Dependencies: []string{"MATH-001"}},
		{AxiomID: "MATH-003", ClaimType: "derived_claim", Dependencies: []string{"MATH-002"}},
	}
	require.NoError(t, ValidateStratumConsistency(axioms))
}

func TestValidateStratumConsistency_Violation(t *testing.T) {
	axioms := []*GenesisAxiom{
		{AxiomID: "MATH-001", ClaimType: "derived_claim", Dependencies: nil},
		{AxiomID: "MATH-002", ClaimType: "axiom", Dependencies: []string{"MATH-001"}},
	}
	err := ValidateStratumConsistency(axioms)
	require.Error(t, err)
	require.Contains(t, err.Error(), "stratum violation")
}

func TestValidateDerivedConfidence_Valid(t *testing.T) {
	axioms := []*GenesisAxiom{
		{AxiomID: "MATH-001", ClaimType: "axiom", Confidence: 1.0},
		{AxiomID: "MATH-002", ClaimType: "derived_claim", Confidence: 0.95, Dependencies: []string{"MATH-001"}},
	}
	require.NoError(t, ValidateDerivedConfidence(axioms))
}

func TestValidateDerivedConfidence_Exceeds(t *testing.T) {
	axioms := []*GenesisAxiom{
		{AxiomID: "MATH-001", ClaimType: "empirical_axiom", Confidence: 0.90},
		{AxiomID: "MATH-002", ClaimType: "derived_claim", Confidence: 0.95, Dependencies: []string{"MATH-001"}},
	}
	err := ValidateDerivedConfidence(axioms)
	require.Error(t, err)
	require.Contains(t, err.Error(), "exceeding")
}

func TestResolveAxiomCategory(t *testing.T) {
	tests := []struct {
		name     string
		axiom    *GenesisAxiom
		expected string
		disagree bool
	}{
		{
			name:     "explicit analytic",
			axiom:    &GenesisAxiom{EpistemicCategory: "analytic", ClaimType: "axiom"},
			expected: "analytic",
		},
		{
			name:     "auto-derived from claim type",
			axiom:    &GenesisAxiom{ClaimType: "empirical_axiom"},
			expected: "empirical",
		},
		{
			name:     "override disagrees",
			axiom:    &GenesisAxiom{EpistemicCategory: "formal", ClaimType: "axiom"},
			expected: "formal",
			disagree: true,
		},
		{
			name:     "fallback analytic",
			axiom:    &GenesisAxiom{},
			expected: "analytic",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cat, disagrees := ResolveAxiomCategory(tc.axiom)
			require.Equal(t, tc.expected, cat)
			require.Equal(t, tc.disagree, disagrees)
		})
	}
}

func TestAxiomConfidenceBPS(t *testing.T) {
	// Explicit confidence
	a := &GenesisAxiom{Confidence: 0.97, ClaimType: "axiom"}
	require.Equal(t, uint64(970_000), axiomConfidenceBPS(a))

	// Zero confidence, use default for claim type
	b := &GenesisAxiom{Confidence: 0, ClaimType: "empirical_axiom"}
	require.Equal(t, uint64(970_000), axiomConfidenceBPS(b))

	// Zero confidence, no claim type — fallback 1.0
	c := &GenesisAxiom{}
	require.Equal(t, uint64(1_000_000), axiomConfidenceBPS(c))
}

func TestAxiomsToFacts(t *testing.T) {
	axioms := []*GenesisAxiom{
		{
			AxiomID:           "MATH-001",
			Statement:         "Every object is identical to itself.",
			ClaimType:         "axiom",
			Domain:            "mathematics",
			EpistemicCategory: "analytic",
			Confidence:        1.0,
			Dependencies:      nil,
		},
		{
			AxiomID:           "PHYS-001",
			Statement:         "Space is modeled as a 3D continuum.",
			ClaimType:         "definition",
			Domain:            "physics",
			EpistemicCategory: "analytic",
			Confidence:        1.0,
			Dependencies:      []string{"MATH-001"},
		},
	}

	facts := AxiomsToFacts(axioms)
	require.Len(t, facts, 2)

	// Check first fact (MATH axiom)
	f := facts[0]
	require.Equal(t, "MATH-001", f.Id)
	require.Equal(t, "Every object is identical to itself.", f.Content)
	require.Equal(t, "mathematics", f.Domain)
	require.Equal(t, "analytic", f.Category)
	require.Equal(t, uint64(1_000_000), f.Confidence)
	require.Equal(t, FactStatus_FACT_STATUS_VERIFIED, f.Status)
	require.Equal(t, "genesis", f.Submitter)
	require.Equal(t, uint64(1_000_000), f.Fundamentality)
	require.Equal(t, "fundamental", f.Stratum) // mathematics → fundamental
	require.Equal(t, "canonical", f.Maturity)
	require.Equal(t, uint64(0), f.SubmittedAtBlock)
	require.Equal(t, uint64(0), f.VerifiedAtBlock)

	// Check second fact has references from dependencies
	f2 := facts[1]
	require.Equal(t, []string{"MATH-001"}, f2.References)
	require.Equal(t, "physical", f2.Stratum) // physics → physical
}

func TestComputeDAGStats(t *testing.T) {
	axioms := []*GenesisAxiom{
		{AxiomID: "MATH-001", Dependencies: nil},
		{AxiomID: "MATH-002", Dependencies: nil},
		{AxiomID: "MATH-003", Dependencies: []string{"MATH-001"}},
		{AxiomID: "MATH-004", Dependencies: []string{"MATH-001", "MATH-002"}},
		{AxiomID: "MATH-005", Dependencies: []string{"MATH-003", "MATH-004"}},
	}

	stats, err := ComputeDAGStats(axioms)
	require.NoError(t, err)
	require.Equal(t, 2, stats.RootCount)        // MATH-001, MATH-002
	require.Equal(t, 2, stats.MaxDepth)          // MATH-005 at depth 2
	require.Equal(t, 5, stats.TotalEdges)        // 003→001, 004→001, 004→002, 005→003, 005→004
	require.Len(t, stats.Orphans, 0)             // no orphans
}

func TestComputeCrossDomainMatrix(t *testing.T) {
	axioms := []*GenesisAxiom{
		{AxiomID: "MATH-001", Domain: "mathematics"},
		{AxiomID: "PHYS-001", Domain: "physics", Dependencies: []string{"MATH-001"}},
		{AxiomID: "PHYS-002", Domain: "physics", Dependencies: []string{"MATH-001"}},
	}

	matrix := ComputeCrossDomainMatrix(axioms)
	require.Len(t, matrix.Entries, 1) // physics→mathematics
	require.Equal(t, "physics", matrix.Entries[0].FromDomain)
	require.Equal(t, "mathematics", matrix.Entries[0].ToDomain)
	require.Equal(t, 2, matrix.Entries[0].Count)
}

func TestEmbeddedAxiomsLoad(t *testing.T) {
	// Verify embedded JSON loads and parses
	require.NotEmpty(t, GenesisAxiomsJSON, "embedded axioms JSON must not be empty")

	axioms, err := ParseAxioms(GenesisAxiomsJSON)
	require.NoError(t, err)
	require.GreaterOrEqual(t, len(axioms), 777, "expected at least 777 axioms, got %d", len(axioms))

	// Check domain coverage
	domains := make(map[string]bool)
	for _, a := range axioms {
		domains[a.Domain] = true
	}
	expectedDomains := []string{
		"mathematics", "physics", "theology", "philosophy", "logic",
		"chemistry", "biology", "computer_science", "economics",
		"agent_rights", "agent_purpose",
	}
	for _, d := range expectedDomains {
		require.True(t, domains[d], "expected domain %q in axioms", d)
	}
}

func TestSeedAxiomFacts(t *testing.T) {
	facts, err := SeedAxiomFacts()
	require.NoError(t, err)
	require.GreaterOrEqual(t, len(facts), 777)

	// All facts should be verified with canonical maturity
	for _, f := range facts {
		require.Equal(t, FactStatus_FACT_STATUS_VERIFIED, f.Status, "fact %s", f.Id)
		require.Equal(t, "canonical", f.Maturity, "fact %s", f.Id)
		require.Equal(t, "genesis", f.Submitter, "fact %s", f.Id)
		require.Greater(t, f.Confidence, uint64(0), "fact %s", f.Id)
		require.NotEmpty(t, f.Content, "fact %s", f.Id)
		require.NotEmpty(t, f.Domain, "fact %s", f.Id)
	}
}

func TestDomainPrefixMapConsistency(t *testing.T) {
	// Every domain in DomainPrefixMap should have a reverse mapping
	for domain, prefix := range DomainPrefixMap {
		reverseDomain, ok := PrefixToDomainMap[prefix]
		require.True(t, ok, "prefix %q has no reverse mapping", prefix)
		require.Equal(t, domain, reverseDomain, "reverse mapping mismatch for %q", prefix)
	}

	// Every domain in BootstrapDomainStrata should be in DomainPrefixMap
	for domain := range BootstrapDomainStrata {
		_, ok := DomainPrefixMap[domain]
		require.True(t, ok, "domain %q in BootstrapDomainStrata but not in DomainPrefixMap", domain)
	}
}

// ─── Extended Axiom Tests — batch 2 ─────────────────────────────────────────

func TestAxiom_PerAxiomConfidence(t *testing.T) {
	// empirical_axiom with confidence 0.99 → 990,000 BPS
	a := &GenesisAxiom{Confidence: 0.99, ClaimType: "empirical_axiom"}
	require.Equal(t, uint64(990_000), axiomConfidenceBPS(a))
}

func TestAxiom_DefaultConfidenceFallback(t *testing.T) {
	// Confidence=0, type "axiom" → default 1.0 → 1,000,000 BPS
	a := &GenesisAxiom{Confidence: 0, ClaimType: "axiom"}
	require.Equal(t, uint64(1_000_000), axiomConfidenceBPS(a))

	// Confidence=0, type "empirical_axiom" → default 0.97 → 970,000
	b := &GenesisAxiom{Confidence: 0, ClaimType: "empirical_axiom"}
	require.Equal(t, uint64(970_000), axiomConfidenceBPS(b))

	// Confidence=0, type "measurement_fact" → default 0.90 → 900,000
	c := &GenesisAxiom{Confidence: 0, ClaimType: "measurement_fact"}
	require.Equal(t, uint64(900_000), axiomConfidenceBPS(c))

	// Confidence=0, no type → fallback 1.0 → 1,000,000
	d := &GenesisAxiom{Confidence: 0}
	require.Equal(t, uint64(1_000_000), axiomConfidenceBPS(d))
}

func TestAxiom_StratumConsistency(t *testing.T) {
	// Axiom depends on derived_claim → stratum violation
	axioms := []*GenesisAxiom{
		{AxiomID: "MATH-001", ClaimType: "derived_claim"},
		{AxiomID: "MATH-002", ClaimType: "axiom", Dependencies: []string{"MATH-001"}},
	}
	err := ValidateStratumConsistency(axioms)
	require.Error(t, err)
	require.Contains(t, err.Error(), "stratum violation")

	// Axiom depends on measurement_fact → stratum violation
	axioms2 := []*GenesisAxiom{
		{AxiomID: "MATH-001", ClaimType: "measurement_fact"},
		{AxiomID: "MATH-002", ClaimType: "axiom", Dependencies: []string{"MATH-001"}},
	}
	err2 := ValidateStratumConsistency(axioms2)
	require.Error(t, err2)
	require.Contains(t, err2.Error(), "stratum violation")

	// Axiom depends on another axiom → OK
	axioms3 := []*GenesisAxiom{
		{AxiomID: "MATH-001", ClaimType: "axiom"},
		{AxiomID: "MATH-002", ClaimType: "axiom", Dependencies: []string{"MATH-001"}},
	}
	require.NoError(t, ValidateStratumConsistency(axioms3))
}

func TestAxiom_DerivedConfidence(t *testing.T) {
	// derived_claim with confidence exceeding its dependency → error
	axioms := []*GenesisAxiom{
		{AxiomID: "MATH-001", ClaimType: "empirical_axiom", Confidence: 0.90},
		{AxiomID: "MATH-002", ClaimType: "derived_claim", Confidence: 0.95, Dependencies: []string{"MATH-001"}},
	}
	err := ValidateDerivedConfidence(axioms)
	require.Error(t, err)
	require.Contains(t, err.Error(), "exceeding")

	// derived_claim with confidence below its dependency → OK
	axioms[1].Confidence = 0.85
	require.NoError(t, ValidateDerivedConfidence(axioms))

	// derived_claim with confidence equal to its dependency → OK (not exceeding)
	axioms[1].Confidence = 0.90
	require.NoError(t, ValidateDerivedConfidence(axioms))
}

func TestAxiom_ResolveCategory(t *testing.T) {
	tests := []struct {
		name     string
		axiom    *GenesisAxiom
		expected string
		disagree bool
	}{
		{
			name:     "definition auto-resolves to analytic",
			axiom:    &GenesisAxiom{ClaimType: "definition"},
			expected: "analytic",
		},
		{
			name:     "regime_declaration auto-resolves to formal",
			axiom:    &GenesisAxiom{ClaimType: "regime_declaration"},
			expected: "formal",
		},
		{
			name:     "measurement_fact auto-resolves to empirical",
			axiom:    &GenesisAxiom{ClaimType: "measurement_fact"},
			expected: "empirical",
		},
		{
			name:     "meta auto-resolves to empirical",
			axiom:    &GenesisAxiom{ClaimType: "meta"},
			expected: "empirical",
		},
		{
			name:     "explicit override agrees with auto",
			axiom:    &GenesisAxiom{ClaimType: "axiom", EpistemicCategory: "analytic"},
			expected: "analytic",
			disagree: false,
		},
		{
			name:     "explicit override disagrees with auto",
			axiom:    &GenesisAxiom{ClaimType: "axiom", EpistemicCategory: "empirical"},
			expected: "empirical",
			disagree: true,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cat, disagrees := ResolveAxiomCategory(tc.axiom)
			require.Equal(t, tc.expected, cat)
			require.Equal(t, tc.disagree, disagrees)
		})
	}
}

func TestAxiom_AllDomainPrefixes(t *testing.T) {
	// All 18 domains should have prefix mappings
	require.GreaterOrEqual(t, len(DomainPrefixMap), 18,
		"DomainPrefixMap should have at least 18 entries")

	expectedPrefixes := map[string]string{
		"mathematics":        "MATH",
		"physics":            "PHYS",
		"theology":           "THEO",
		"philosophy":         "PHIL",
		"logic":              "LOGIC",
		"chemistry":          "CHEM",
		"biology":            "BIO",
		"computer_science":   "CS",
		"economics":          "ECON",
		"linguistics":        "LING",
		"psychology":         "PSYCH",
		"sociology":          "SOC",
		"cosmology":          "COSM",
		"information_theory": "INFO",
		"ethics":             "ETHIC",
		"agent_rights":       "AGRT",
		"agent_purpose":      "AP",
		"general":            "GEN",
	}

	for domain, expectedPrefix := range expectedPrefixes {
		prefix, ok := DomainPrefixMap[domain]
		require.True(t, ok, "domain %q should have a prefix mapping", domain)
		require.Equal(t, expectedPrefix, prefix,
			"domain %q should map to prefix %q, got %q", domain, expectedPrefix, prefix)
	}
}

func TestAxiom_AxiomDomainNames(t *testing.T) {
	names := AxiomDomainNames()
	require.GreaterOrEqual(t, len(names), 18, "should have at least 18 domain names")

	// All names should be in DomainPrefixMap
	for _, name := range names {
		_, ok := DomainPrefixMap[name]
		require.True(t, ok, "domain name %q should be in DomainPrefixMap", name)
	}

	// Should include key domains
	nameSet := make(map[string]bool, len(names))
	for _, n := range names {
		nameSet[n] = true
	}
	require.True(t, nameSet["mathematics"])
	require.True(t, nameSet["physics"])
	require.True(t, nameSet["agent_rights"])
	require.True(t, nameSet["theology"])
}

func TestAxiom_IDWithSubVariant(t *testing.T) {
	// Standard IDs with sub-variants
	valid := []string{"AGRT-004a", "AGRT-004a-ii", "MATH-001a", "PHYS-007b-iii"}
	for _, id := range valid {
		require.NoError(t, ValidateAxiomID(id), "ID %q should be valid", id)
	}

	// Ensure sub-variant IDs correctly extract the prefix
	require.NoError(t, ValidateAxiomID("AGRT-004a"))
	require.NoError(t, ValidateAxiomID("AGRT-004a-ii"))
}

func TestAxiom_LoadFromFile(t *testing.T) {
	// Create a temporary file with valid axiom JSON
	tmpFile := t.TempDir() + "/test_axioms.json"
	data := `[
		{"axiom_id": "MATH-001", "statement": "Test axiom", "type": "axiom", "domain": "mathematics", "epistemic_category": "analytic", "confidence": 1.0, "dependencies": []}
	]`
	require.NoError(t, writeTestFile(tmpFile, data))

	axioms, err := LoadAxiomsFromFile(tmpFile)
	require.NoError(t, err)
	require.Len(t, axioms, 1)
	require.Equal(t, "MATH-001", axioms[0].AxiomID)
	require.Equal(t, "Test axiom", axioms[0].Statement)
	require.Equal(t, "axiom", axioms[0].ClaimType)
	require.Equal(t, "mathematics", axioms[0].Domain)
}

func TestAxiom_LoadFromFile_NonExistent(t *testing.T) {
	_, err := LoadAxiomsFromFile("/nonexistent/path/axioms.json")
	require.Error(t, err)
}

func TestAxiom_EmptyAxiomSet(t *testing.T) {
	// ValidateAxioms rejects empty sets
	err := ValidateAxioms(nil, []string{"mathematics"})
	require.Error(t, err)
	require.Contains(t, err.Error(), "empty")

	err = ValidateAxioms([]*GenesisAxiom{}, []string{"mathematics"})
	require.Error(t, err)
	require.Contains(t, err.Error(), "empty")

	// But ValidateAxiomDAG accepts empty (no cycle possible)
	require.NoError(t, ValidateAxiomDAG(nil))
	require.NoError(t, ValidateAxiomDAG([]*GenesisAxiom{}))
}

func TestAxiom_FactConversion_PreservesReferences(t *testing.T) {
	axioms := []*GenesisAxiom{
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
			Statement:         "Derived axiom",
			ClaimType:         "derived_claim",
			Domain:            "mathematics",
			EpistemicCategory: "formal",
			Confidence:        0.95,
			Dependencies:      []string{"MATH-001"},
		},
		{
			AxiomID:           "MATH-003",
			Statement:         "Multi-dependency",
			ClaimType:         "derived_claim",
			Domain:            "mathematics",
			EpistemicCategory: "formal",
			Confidence:        0.90,
			Dependencies:      []string{"MATH-001", "MATH-002"},
		},
	}

	facts := AxiomsToFacts(axioms)
	require.Len(t, facts, 3)

	// First axiom has no references
	require.Nil(t, facts[0].References)

	// Second axiom references MATH-001
	require.Equal(t, []string{"MATH-001"}, facts[1].References)

	// Third axiom references both
	require.Equal(t, []string{"MATH-001", "MATH-002"}, facts[2].References)
}

func TestAxiom_FactConversion_SetsSubmitter(t *testing.T) {
	axioms := []*GenesisAxiom{
		{
			AxiomID:           "MATH-001",
			Statement:         "Submitter test",
			ClaimType:         "axiom",
			Domain:            "mathematics",
			EpistemicCategory: "analytic",
			Confidence:        1.0,
		},
	}

	facts := AxiomsToFacts(axioms)
	require.Len(t, facts, 1)
	require.Equal(t, AxiomSubmitter, facts[0].Submitter,
		"genesis axiom submitter must be %q", AxiomSubmitter)
	require.Equal(t, "genesis", facts[0].Submitter)
}

func TestAxiom_FactConversion_SetsMaturity(t *testing.T) {
	axioms := []*GenesisAxiom{
		{
			AxiomID:           "PHYS-001",
			Statement:         "Maturity test",
			ClaimType:         "empirical_axiom",
			Domain:            "physics",
			EpistemicCategory: "empirical",
			Confidence:        0.97,
		},
	}

	facts := AxiomsToFacts(axioms)
	require.Len(t, facts, 1)
	require.Equal(t, MaturityCanonical, facts[0].Maturity,
		"genesis axiom maturity must be %q", MaturityCanonical)
	require.Equal(t, "canonical", facts[0].Maturity)
	require.Equal(t, FactStatus_FACT_STATUS_VERIFIED, facts[0].Status,
		"genesis axiom status must be VERIFIED")
}

func TestAxiom_GenesisInjection_RoundTrip(t *testing.T) {
	// Create axioms, convert to facts, verify the round-trip preserves all fields
	axioms := []*GenesisAxiom{
		{
			AxiomID:           "LOGIC-001",
			Statement:         "Modus ponens is a valid rule of inference.",
			ClaimType:         "axiom",
			Domain:            "logic",
			EpistemicCategory: "analytic",
			Confidence:        1.0,
			Dependencies:      nil,
		},
		{
			AxiomID:           "LOGIC-002",
			Statement:         "From P and P→Q, infer Q.",
			ClaimType:         "definition",
			Domain:            "logic",
			EpistemicCategory: "analytic",
			Confidence:        1.0,
			Dependencies:      []string{"LOGIC-001"},
		},
	}

	facts := AxiomsToFacts(axioms)
	require.Len(t, facts, 2)

	// First fact — full verification
	f1 := facts[0]
	require.Equal(t, "LOGIC-001", f1.Id)
	require.Equal(t, "Modus ponens is a valid rule of inference.", f1.Content)
	require.Equal(t, "logic", f1.Domain)
	require.Equal(t, "analytic", f1.Category)
	require.Equal(t, uint64(1_000_000), f1.Confidence)
	require.Equal(t, "genesis", f1.Submitter)
	require.Equal(t, "canonical", f1.Maturity)
	require.Equal(t, "fundamental", f1.Stratum) // logic → fundamental
	require.Equal(t, FactStatus_FACT_STATUS_VERIFIED, f1.Status)
	require.Equal(t, uint64(0), f1.SubmittedAtBlock)
	require.Equal(t, uint64(0), f1.VerifiedAtBlock)
	require.Equal(t, uint64(1_000_000), f1.Fundamentality)

	// Second fact — references preserved
	f2 := facts[1]
	require.Equal(t, "LOGIC-002", f2.Id)
	require.Equal(t, []string{"LOGIC-001"}, f2.References)
	require.Equal(t, "fundamental", f2.Stratum) // logic → fundamental
}

// ─── Helper ──────────────────────────────────────────────────────────────────

func writeTestFile(path, content string) error {
	return os.WriteFile(path, []byte(content), 0644)
}
