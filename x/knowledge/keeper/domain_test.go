package keeper_test

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/zerone-chain/zerone/x/knowledge/types"
)

// ─── Domain CRUD ─────────────────────────────────────────────────────────────

func TestDomain_SetAndGet(t *testing.T) {
	k, ctx := setupKnowledgeTest(t)

	domain := &types.Domain{
		Name:        "quantum_computing",
		Description: "Quantum information processing",
		Status:      types.DomainStatus_DOMAIN_STATUS_ACTIVE,
		Proposer:    "zrn1proposer",
		Stratum:     "formal",
	}
	require.NoError(t, k.SetDomain(ctx, domain))

	got, found := k.GetDomain(ctx, "quantum_computing")
	require.True(t, found)
	require.Equal(t, "quantum_computing", got.Name)
	require.Equal(t, "Quantum information processing", got.Description)
	require.Equal(t, types.DomainStatus_DOMAIN_STATUS_ACTIVE, got.Status)
	require.Equal(t, "formal", got.Stratum)
}

func TestDomain_GetNotFound(t *testing.T) {
	k, ctx := setupKnowledgeTest(t)

	_, found := k.GetDomain(ctx, "nonexistent_domain")
	require.False(t, found)
}

func TestDomain_Update(t *testing.T) {
	k, ctx := setupKnowledgeTest(t)

	domain := &types.Domain{
		Name:        "test_domain",
		Description: "Original description",
		Status:      types.DomainStatus_DOMAIN_STATUS_PROPOSED,
	}
	require.NoError(t, k.SetDomain(ctx, domain))

	// Update status
	domain.Status = types.DomainStatus_DOMAIN_STATUS_ACTIVE
	domain.Description = "Updated description"
	require.NoError(t, k.SetDomain(ctx, domain))

	got, found := k.GetDomain(ctx, "test_domain")
	require.True(t, found)
	require.Equal(t, types.DomainStatus_DOMAIN_STATUS_ACTIVE, got.Status)
	require.Equal(t, "Updated description", got.Description)
}

func TestDomain_Iterate(t *testing.T) {
	k, ctx := setupKnowledgeTest(t)

	// After InitGenesis, 18 default domains exist
	var count int
	k.IterateDomains(ctx, func(domain *types.Domain) bool {
		count++
		return false
	})
	require.Equal(t, 18, count)
}

func TestDomain_GenesisDefaults_18Domains(t *testing.T) {
	k, ctx := setupKnowledgeTest(t)

	expectedDomains := []string{
		"mathematics", "physics", "computer_science", "general",
		"theology", "philosophy", "logic", "chemistry",
		"biology", "economics", "linguistics", "psychology",
		"sociology", "cosmology", "information_theory", "ethics",
		"agent_rights", "agent_purpose",
	}

	for _, name := range expectedDomains {
		domain, found := k.GetDomain(ctx, name)
		require.True(t, found, "genesis domain %q should exist", name)
		require.Equal(t, types.DomainStatus_DOMAIN_STATUS_ACTIVE, domain.Status,
			"genesis domain %q should be active", name)
	}
}

func TestDomain_GenesisDefaults_AllActive(t *testing.T) {
	k, ctx := setupKnowledgeTest(t)

	k.IterateDomains(ctx, func(domain *types.Domain) bool {
		require.Equal(t, types.DomainStatus_DOMAIN_STATUS_ACTIVE, domain.Status,
			"genesis domain %q must be active", domain.Name)
		return false
	})
}

func TestDomain_AddCustom(t *testing.T) {
	k, ctx := setupKnowledgeTest(t)

	// Add a custom domain
	custom := &types.Domain{
		Name:        "astrobiology",
		Description: "Life in the universe",
		Status:      types.DomainStatus_DOMAIN_STATUS_ACTIVE,
	}
	require.NoError(t, k.SetDomain(ctx, custom))

	// Should now have 19 domains (18 + 1 custom)
	var count int
	k.IterateDomains(ctx, func(domain *types.Domain) bool {
		count++
		return false
	})
	require.Equal(t, 19, count)
}

func TestDomain_ProposedStatus(t *testing.T) {
	k, ctx := setupKnowledgeTest(t)

	domain := &types.Domain{
		Name:        "proposed_domain",
		Description: "Awaiting endorsement",
		Status:      types.DomainStatus_DOMAIN_STATUS_PROPOSED,
		Proposer:    "zrn1proposer",
	}
	require.NoError(t, k.SetDomain(ctx, domain))

	got, found := k.GetDomain(ctx, "proposed_domain")
	require.True(t, found)
	require.Equal(t, types.DomainStatus_DOMAIN_STATUS_PROPOSED, got.Status)
	require.Equal(t, "zrn1proposer", got.Proposer)
}

func TestDomain_DeprecatedStatus(t *testing.T) {
	k, ctx := setupKnowledgeTest(t)

	domain := &types.Domain{
		Name:   "deprecated_domain",
		Status: types.DomainStatus_DOMAIN_STATUS_DEPRECATED,
	}
	require.NoError(t, k.SetDomain(ctx, domain))

	got, found := k.GetDomain(ctx, "deprecated_domain")
	require.True(t, found)
	require.Equal(t, types.DomainStatus_DOMAIN_STATUS_DEPRECATED, got.Status)
}

func TestDomain_WithEndorsers(t *testing.T) {
	k, ctx := setupKnowledgeTest(t)

	domain := &types.Domain{
		Name:      "endorsed_domain",
		Status:    types.DomainStatus_DOMAIN_STATUS_PROPOSED,
		Proposer:  "zrn1proposer",
		Endorsers: []string{"zrn1endorser1", "zrn1endorser2"},
	}
	require.NoError(t, k.SetDomain(ctx, domain))

	got, found := k.GetDomain(ctx, "endorsed_domain")
	require.True(t, found)
	require.Len(t, got.Endorsers, 2)
	require.Contains(t, got.Endorsers, "zrn1endorser1")
}

func TestDomain_FactCount(t *testing.T) {
	k, ctx := setupKnowledgeTest(t)

	domain := &types.Domain{
		Name:      "counting_domain",
		Status:    types.DomainStatus_DOMAIN_STATUS_ACTIVE,
		FactCount: 42,
	}
	require.NoError(t, k.SetDomain(ctx, domain))

	got, found := k.GetDomain(ctx, "counting_domain")
	require.True(t, found)
	require.Equal(t, uint64(42), got.FactCount)
}

func TestDomain_OverwriteExisting(t *testing.T) {
	k, ctx := setupKnowledgeTest(t)

	// Set twice — second write should overwrite
	d1 := &types.Domain{Name: "overwrite_me", Description: "First"}
	require.NoError(t, k.SetDomain(ctx, d1))

	d2 := &types.Domain{Name: "overwrite_me", Description: "Second"}
	require.NoError(t, k.SetDomain(ctx, d2))

	got, found := k.GetDomain(ctx, "overwrite_me")
	require.True(t, found)
	require.Equal(t, "Second", got.Description)
}

func TestDomain_IterateEarlyBreak(t *testing.T) {
	k, ctx := setupKnowledgeTest(t)

	var count int
	k.IterateDomains(ctx, func(domain *types.Domain) bool {
		count++
		return count >= 3
	})
	require.Equal(t, 3, count, "iteration should stop after 3")
}

func TestDomain_WithStratum(t *testing.T) {
	k, ctx := setupKnowledgeTest(t)

	domain := &types.Domain{
		Name:    "strata_test",
		Status:  types.DomainStatus_DOMAIN_STATUS_ACTIVE,
		Stratum: "empirical",
	}
	require.NoError(t, k.SetDomain(ctx, domain))

	got, found := k.GetDomain(ctx, "strata_test")
	require.True(t, found)
	require.Equal(t, "empirical", got.Stratum)
}

func TestDomain_CreatedAtBlock(t *testing.T) {
	k, ctx := setupKnowledgeTest(t)

	domain := &types.Domain{
		Name:           "block_test",
		Status:         types.DomainStatus_DOMAIN_STATUS_ACTIVE,
		CreatedAtBlock: 42,
	}
	require.NoError(t, k.SetDomain(ctx, domain))

	got, found := k.GetDomain(ctx, "block_test")
	require.True(t, found)
	require.Equal(t, uint64(42), got.CreatedAtBlock)
}

// ─── DomainStatus enum ──────────────────────────────────────────────────────

func TestDomainStatus_AllValues(t *testing.T) {
	k, ctx := setupKnowledgeTest(t)

	statuses := []types.DomainStatus{
		types.DomainStatus_DOMAIN_STATUS_UNSPECIFIED,
		types.DomainStatus_DOMAIN_STATUS_PROPOSED,
		types.DomainStatus_DOMAIN_STATUS_ACTIVE,
		types.DomainStatus_DOMAIN_STATUS_DEPRECATED,
	}

	for i, s := range statuses {
		name := fmt.Sprintf("domain_status_%d", i)
		require.NoError(t, k.SetDomain(ctx, &types.Domain{Name: name, Status: s}))
		got, found := k.GetDomain(ctx, name)
		require.True(t, found)
		require.Equal(t, s, got.Status)
	}
}

// ─── Extended Domain Tests ──────────────────────────────────────────────────

func TestDomain_ActivityTracking(t *testing.T) {
	k, ctx := setupKnowledgeTest(t)

	// Domain starts at 0 facts
	domain, found := k.GetDomain(ctx, "physics")
	require.True(t, found)
	require.Equal(t, uint64(0), domain.FactCount)

	// Simulate incrementing FactCount (as submissions would)
	domain.FactCount = 5
	require.NoError(t, k.SetDomain(ctx, domain))

	got, found := k.GetDomain(ctx, "physics")
	require.True(t, found)
	require.Equal(t, uint64(5), got.FactCount, "domain activity should track fact submissions")
}

func TestDomain_ReputationAccumulation(t *testing.T) {
	k, ctx := setupKnowledgeTest(t)

	// Store facts in a domain and increment the count each time
	domain, found := k.GetDomain(ctx, "mathematics")
	require.True(t, found)

	for i := uint64(1); i <= 10; i++ {
		domain.FactCount = i
		require.NoError(t, k.SetDomain(ctx, domain))
	}

	got, found := k.GetDomain(ctx, "mathematics")
	require.True(t, found)
	require.Equal(t, uint64(10), got.FactCount,
		"repeated updates should accumulate activity")
}

func TestDomain_MultipleDomains(t *testing.T) {
	k, ctx := setupKnowledgeTest(t)

	// Update fact counts independently for multiple domains
	physics, _ := k.GetDomain(ctx, "physics")
	physics.FactCount = 42
	require.NoError(t, k.SetDomain(ctx, physics))

	math, _ := k.GetDomain(ctx, "mathematics")
	math.FactCount = 100
	require.NoError(t, k.SetDomain(ctx, math))

	// Read back — each should have its own count
	gotPhysics, _ := k.GetDomain(ctx, "physics")
	gotMath, _ := k.GetDomain(ctx, "mathematics")
	require.Equal(t, uint64(42), gotPhysics.FactCount, "physics should have 42 facts")
	require.Equal(t, uint64(100), gotMath.FactCount, "math should have 100 facts")
}

func TestDomain_DefaultDomainsCount(t *testing.T) {
	defaults := types.DefaultDomains()
	require.GreaterOrEqual(t, len(defaults), 18,
		"DefaultDomains must return at least 18 domains")

	// All should be active
	for _, d := range defaults {
		require.Equal(t, types.DomainStatus_DOMAIN_STATUS_ACTIVE, d.Status,
			"default domain %q must be active", d.Name)
	}
}

func TestDomain_StatusTransitions(t *testing.T) {
	k, ctx := setupKnowledgeTest(t)

	// Create an active domain, then deprecate it
	require.NoError(t, k.SetDomain(ctx, &types.Domain{
		Name:   "transitional_domain",
		Status: types.DomainStatus_DOMAIN_STATUS_ACTIVE,
	}))

	got, found := k.GetDomain(ctx, "transitional_domain")
	require.True(t, found)
	require.Equal(t, types.DomainStatus_DOMAIN_STATUS_ACTIVE, got.Status)

	// Transition to deprecated
	got.Status = types.DomainStatus_DOMAIN_STATUS_DEPRECATED
	require.NoError(t, k.SetDomain(ctx, got))

	updated, found := k.GetDomain(ctx, "transitional_domain")
	require.True(t, found)
	require.Equal(t, types.DomainStatus_DOMAIN_STATUS_DEPRECATED, updated.Status,
		"domain should transition from active to deprecated")
}

func TestDomain_FactCountByDomain(t *testing.T) {
	k, ctx := setupKnowledgeTest(t)

	// Set different fact counts for several domains
	testData := map[string]uint64{
		"physics":     10,
		"mathematics": 25,
		"biology":     7,
	}
	for name, count := range testData {
		d, found := k.GetDomain(ctx, name)
		require.True(t, found)
		d.FactCount = count
		require.NoError(t, k.SetDomain(ctx, d))
	}

	// Verify each via iteration
	factCounts := make(map[string]uint64)
	k.IterateDomains(ctx, func(domain *types.Domain) bool {
		if domain.FactCount > 0 {
			factCounts[domain.Name] = domain.FactCount
		}
		return false
	})

	for name, expectedCount := range testData {
		require.Equal(t, expectedCount, factCounts[name],
			"domain %q fact count mismatch", name)
	}
}

func TestDomain_CrossDomainReferences(t *testing.T) {
	k, ctx := setupKnowledgeTest(t)

	// A fact in physics can reference mathematics
	fact := &types.Fact{
		Id:         "cross-domain-fact",
		Content:    "F = ma (derived from mathematical axioms)",
		Domain:     "physics",
		Category:   "empirical",
		Confidence: 900_000,
		Submitter:  "zrn1sub1",
		References: []string{"MATH-001", "MATH-002"},
		Status:     types.FactStatus_FACT_STATUS_VERIFIED,
	}
	require.NoError(t, k.SetFact(ctx, fact))

	got, found := k.GetFact(ctx, "cross-domain-fact")
	require.True(t, found)
	require.Equal(t, "physics", got.Domain)
	require.Equal(t, []string{"MATH-001", "MATH-002"}, got.References,
		"facts should be able to reference entities from other domains")
}

func TestDomain_StratumAssignment(t *testing.T) {
	// Every domain in BootstrapDomainStrata maps to a valid stratum
	for domain, stratum := range types.BootstrapDomainStrata {
		require.True(t, types.ValidStrata[stratum],
			"domain %q maps to invalid stratum %q", domain, stratum)
	}

	// Specific expected mappings
	require.Equal(t, "fundamental", types.BootstrapDomainStrata["mathematics"])
	require.Equal(t, "physical", types.BootstrapDomainStrata["physics"])
	require.Equal(t, "chemical", types.BootstrapDomainStrata["chemistry"])
	require.Equal(t, "biological", types.BootstrapDomainStrata["biology"])
	require.Equal(t, "cognitive", types.BootstrapDomainStrata["psychology"])
	require.Equal(t, "social", types.BootstrapDomainStrata["economics"])
	require.Equal(t, "technological", types.BootstrapDomainStrata["computer_science"])
}

func TestDomain_PrefixMapping(t *testing.T) {
	// DomainPrefixMap and PrefixToDomainMap must be inverses
	for domain, prefix := range types.DomainPrefixMap {
		reverseDomain, ok := types.PrefixToDomainMap[prefix]
		require.True(t, ok, "prefix %q (domain %q) missing from PrefixToDomainMap", prefix, domain)
		require.Equal(t, domain, reverseDomain,
			"PrefixToDomainMap[%q] = %q, expected %q", prefix, reverseDomain, domain)
	}

	for prefix, domain := range types.PrefixToDomainMap {
		reversePrefix, ok := types.DomainPrefixMap[domain]
		require.True(t, ok, "domain %q (prefix %q) missing from DomainPrefixMap", domain, prefix)
		require.Equal(t, prefix, reversePrefix,
			"DomainPrefixMap[%q] = %q, expected %q", domain, reversePrefix, prefix)
	}
}

func TestDomain_UnknownDomainRejected(t *testing.T) {
	// ValidateAxioms rejects claims for unknown domains
	axioms := []*types.GenesisAxiom{
		{
			AxiomID:           "MATH-001",
			Statement:         "Statement",
			ClaimType:         "axiom",
			Domain:            "alchemy", // not in valid domains
			EpistemicCategory: "analytic",
			Confidence:        1.0,
		},
	}
	err := types.ValidateAxioms(axioms, []string{"mathematics", "physics"})
	require.Error(t, err)
	require.Contains(t, err.Error(), "unknown domain")
}

func TestDomain_IterationOrder(t *testing.T) {
	k, ctx := setupKnowledgeTest(t)

	// Iterate twice — order must be deterministic (KV store uses sorted keys)
	var names1, names2 []string
	k.IterateDomains(ctx, func(domain *types.Domain) bool {
		names1 = append(names1, domain.Name)
		return false
	})
	k.IterateDomains(ctx, func(domain *types.Domain) bool {
		names2 = append(names2, domain.Name)
		return false
	})
	require.Equal(t, names1, names2,
		"domain iteration must produce the same order on repeated calls")
	require.True(t, len(names1) >= 18, "should iterate over all 18 genesis domains")
}

func TestDomain_ProposedDomainCreation(t *testing.T) {
	k, ctx := setupKnowledgeTest(t)

	proposed := &types.Domain{
		Name:           "quantum_gravity",
		Description:    "Theories unifying quantum mechanics and general relativity",
		Status:         types.DomainStatus_DOMAIN_STATUS_PROPOSED,
		Proposer:       "zrn1proposer1",
		Endorsers:      nil,
		Stratum:        types.DefaultProposalStratum,
		CreatedAtBlock: uint64(ctx.BlockHeight()),
	}
	require.NoError(t, k.SetDomain(ctx, proposed))

	got, found := k.GetDomain(ctx, "quantum_gravity")
	require.True(t, found)
	require.Equal(t, types.DomainStatus_DOMAIN_STATUS_PROPOSED, got.Status)
	require.Equal(t, "zrn1proposer1", got.Proposer)
	require.Equal(t, "social", got.Stratum, "proposed domains default to social stratum")
	require.Equal(t, uint64(100), got.CreatedAtBlock)
}
