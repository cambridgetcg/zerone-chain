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
