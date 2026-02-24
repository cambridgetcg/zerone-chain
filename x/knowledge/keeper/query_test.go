package keeper_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/zerone-chain/zerone/x/knowledge/keeper"
	"github.com/zerone-chain/zerone/x/knowledge/types"
)

// ─── Params Query ────────────────────────────────────────────────────────────

func TestQuery_Params(t *testing.T) {
	k, ctx := setupKnowledgeTest(t)
	qs := keeper.NewQueryServerImpl(k)

	resp, err := qs.Params(ctx, &types.QueryParamsRequest{})
	require.NoError(t, err)
	require.NotNil(t, resp.Params)
	require.Equal(t, uint64(3), resp.Params.MinVerifiers)
}

// ─── Fact Queries ────────────────────────────────────────────────────────────

func TestQuery_Fact_Exists(t *testing.T) {
	k, ctx := setupKnowledgeTest(t)
	qs := keeper.NewQueryServerImpl(k)

	makeTestFact(t, k, ctx, "qfact-1", "Query test fact", "physics", "empirical", "zrn1sub", 700_000)

	resp, err := qs.Fact(ctx, &types.QueryFactRequest{Id: "qfact-1"})
	require.NoError(t, err)
	require.NotNil(t, resp.Fact)
	require.Equal(t, "qfact-1", resp.Fact.Id)
	require.Equal(t, "Query test fact", resp.Fact.Content)
}

func TestQuery_Fact_NotFound(t *testing.T) {
	k, ctx := setupKnowledgeTest(t)
	qs := keeper.NewQueryServerImpl(k)

	_, err := qs.Fact(ctx, &types.QueryFactRequest{Id: "nonexistent"})
	require.Error(t, err)
	require.Contains(t, err.Error(), "not found")
}

func TestQuery_Fact_EmptyID(t *testing.T) {
	k, ctx := setupKnowledgeTest(t)
	qs := keeper.NewQueryServerImpl(k)

	_, err := qs.Fact(ctx, &types.QueryFactRequest{Id: ""})
	require.Error(t, err)
	require.Contains(t, err.Error(), "required")
}

func TestQuery_Facts_All(t *testing.T) {
	k, ctx := setupKnowledgeTest(t)
	qs := keeper.NewQueryServerImpl(k)

	makeTestFact(t, k, ctx, "qf-all-1", "Fact one content here", "physics", "empirical", "zrn1sub", 700_000)
	makeTestFact(t, k, ctx, "qf-all-2", "Fact two content here", "mathematics", "formal", "zrn1sub", 800_000)

	resp, err := qs.Facts(ctx, &types.QueryFactsRequest{})
	require.NoError(t, err)
	require.GreaterOrEqual(t, len(resp.Facts), 2)
}

func TestQuery_Facts_ByDomainFilter(t *testing.T) {
	k, ctx := setupKnowledgeTest(t)
	qs := keeper.NewQueryServerImpl(k)

	makeTestFact(t, k, ctx, "qf-dom-1", "Physics fact number one", "physics", "empirical", "zrn1sub", 700_000)
	makeTestFact(t, k, ctx, "qf-dom-2", "Math fact number one here", "mathematics", "formal", "zrn1sub", 800_000)

	resp, err := qs.Facts(ctx, &types.QueryFactsRequest{Domain: "physics"})
	require.NoError(t, err)
	for _, f := range resp.Facts {
		require.Equal(t, "physics", f.Domain)
	}
}

func TestQuery_Facts_ByCategoryFilter(t *testing.T) {
	k, ctx := setupKnowledgeTest(t)
	qs := keeper.NewQueryServerImpl(k)

	makeTestFact(t, k, ctx, "qf-cat-1", "Empirical fact content one", "physics", "empirical", "zrn1sub", 700_000)
	makeTestFact(t, k, ctx, "qf-cat-2", "Formal fact content one here", "mathematics", "formal", "zrn1sub", 800_000)

	resp, err := qs.Facts(ctx, &types.QueryFactsRequest{Category: "formal"})
	require.NoError(t, err)
	for _, f := range resp.Facts {
		require.Equal(t, "formal", f.Category)
	}
}

func TestQuery_Facts_Empty(t *testing.T) {
	k, ctx := setupKnowledgeTest(t)
	qs := keeper.NewQueryServerImpl(k)

	resp, err := qs.Facts(ctx, &types.QueryFactsRequest{})
	require.NoError(t, err)
	require.Empty(t, resp.Facts)
}

// ─── FactsByDomain Query ────────────────────────────────────────────────────

func TestQuery_FactsByDomain_Exists(t *testing.T) {
	k, ctx := setupKnowledgeTest(t)
	qs := keeper.NewQueryServerImpl(k)

	makeTestFact(t, k, ctx, "qfbd-1", "Domain query test content", "physics", "empirical", "zrn1sub", 700_000)

	resp, err := qs.FactsByDomain(ctx, &types.QueryFactsByDomainRequest{Domain: "physics"})
	require.NoError(t, err)
	require.NotEmpty(t, resp.Facts)
	require.Equal(t, "physics", resp.Facts[0].Domain)
}

func TestQuery_FactsByDomain_EmptyDomain(t *testing.T) {
	k, ctx := setupKnowledgeTest(t)
	qs := keeper.NewQueryServerImpl(k)

	_, err := qs.FactsByDomain(ctx, &types.QueryFactsByDomainRequest{Domain: ""})
	require.Error(t, err)
	require.Contains(t, err.Error(), "required")
}

func TestQuery_FactsByDomain_NoResults(t *testing.T) {
	k, ctx := setupKnowledgeTest(t)
	qs := keeper.NewQueryServerImpl(k)

	resp, err := qs.FactsByDomain(ctx, &types.QueryFactsByDomainRequest{Domain: "physics"})
	require.NoError(t, err)
	require.Empty(t, resp.Facts)
}

// ─── FactsBySubmitter Query ─────────────────────────────────────────────────

func TestQuery_FactsBySubmitter_Exists(t *testing.T) {
	k, ctx := setupKnowledgeTest(t)
	qs := keeper.NewQueryServerImpl(k)

	makeTestFact(t, k, ctx, "qfbs-1", "Submitter query test fact", "physics", "empirical", "zrn1alice", 700_000)

	resp, err := qs.FactsBySubmitter(ctx, &types.QueryFactsBySubmitterRequest{Submitter: "zrn1alice"})
	require.NoError(t, err)
	require.NotEmpty(t, resp.Facts)
	require.Equal(t, "zrn1alice", resp.Facts[0].Submitter)
}

func TestQuery_FactsBySubmitter_EmptySubmitter(t *testing.T) {
	k, ctx := setupKnowledgeTest(t)
	qs := keeper.NewQueryServerImpl(k)

	_, err := qs.FactsBySubmitter(ctx, &types.QueryFactsBySubmitterRequest{Submitter: ""})
	require.Error(t, err)
	require.Contains(t, err.Error(), "required")
}

func TestQuery_FactsBySubmitter_NoResults(t *testing.T) {
	k, ctx := setupKnowledgeTest(t)
	qs := keeper.NewQueryServerImpl(k)

	resp, err := qs.FactsBySubmitter(ctx, &types.QueryFactsBySubmitterRequest{Submitter: "zrn1nobody"})
	require.NoError(t, err)
	require.Empty(t, resp.Facts)
}

// ─── Claim Query ────────────────────────────────────────────────────────────

func TestQuery_Claim_Exists(t *testing.T) {
	k, ctx := setupKnowledgeTest(t)
	qs := keeper.NewQueryServerImpl(k)

	claim := &types.Claim{
		Id:          "qclaim-1",
		FactContent: "Query claim test claim content",
		Domain:      "physics",
		Submitter:   "zrn1sub",
		Status:      types.ClaimStatus_CLAIM_STATUS_PENDING,
		Stake:       "1000000",
	}
	require.NoError(t, k.SetClaim(ctx, claim))

	resp, err := qs.Claim(ctx, &types.QueryClaimRequest{Id: "qclaim-1"})
	require.NoError(t, err)
	require.Equal(t, "qclaim-1", resp.Claim.Id)
}

func TestQuery_Claim_NotFound(t *testing.T) {
	k, ctx := setupKnowledgeTest(t)
	qs := keeper.NewQueryServerImpl(k)

	_, err := qs.Claim(ctx, &types.QueryClaimRequest{Id: "nonexistent"})
	require.Error(t, err)
	require.Contains(t, err.Error(), "not found")
}

func TestQuery_Claim_EmptyID(t *testing.T) {
	k, ctx := setupKnowledgeTest(t)
	qs := keeper.NewQueryServerImpl(k)

	_, err := qs.Claim(ctx, &types.QueryClaimRequest{Id: ""})
	require.Error(t, err)
	require.Contains(t, err.Error(), "required")
}

// ─── PendingClaims Query ────────────────────────────────────────────────────

func TestQuery_PendingClaims(t *testing.T) {
	k, ctx := setupKnowledgeTest(t)
	qs := keeper.NewQueryServerImpl(k)

	pending := &types.Claim{
		Id:          "qpc-1",
		FactContent: "Pending claim test content one",
		Status:      types.ClaimStatus_CLAIM_STATUS_PENDING,
	}
	inVerification := &types.Claim{
		Id:          "qpc-2",
		FactContent: "In verification claim content",
		Status:      types.ClaimStatus_CLAIM_STATUS_IN_VERIFICATION,
	}
	require.NoError(t, k.SetClaim(ctx, pending))
	require.NoError(t, k.SetClaim(ctx, inVerification))

	resp, err := qs.PendingClaims(ctx, &types.QueryPendingClaimsRequest{})
	require.NoError(t, err)
	require.Len(t, resp.Claims, 1)
	require.Equal(t, "qpc-1", resp.Claims[0].Id)
}

func TestQuery_PendingClaims_Empty(t *testing.T) {
	k, ctx := setupKnowledgeTest(t)
	qs := keeper.NewQueryServerImpl(k)

	resp, err := qs.PendingClaims(ctx, &types.QueryPendingClaimsRequest{})
	require.NoError(t, err)
	require.Empty(t, resp.Claims)
}

// ─── VerificationRound Query ────────────────────────────────────────────────

func TestQuery_VerificationRound_Exists(t *testing.T) {
	k, ctx := setupKnowledgeTest(t)
	qs := keeper.NewQueryServerImpl(k)

	round := makeRoundInPhase("qround-1", "c1", types.VerificationPhase_VERIFICATION_PHASE_COMMIT, 100)
	require.NoError(t, k.SetVerificationRound(ctx, round))

	resp, err := qs.VerificationRound(ctx, &types.QueryVerificationRoundRequest{Id: "qround-1"})
	require.NoError(t, err)
	require.Equal(t, "qround-1", resp.Round.Id)
	require.Equal(t, types.VerificationPhase_VERIFICATION_PHASE_COMMIT, resp.Round.Phase)
}

func TestQuery_VerificationRound_NotFound(t *testing.T) {
	k, ctx := setupKnowledgeTest(t)
	qs := keeper.NewQueryServerImpl(k)

	_, err := qs.VerificationRound(ctx, &types.QueryVerificationRoundRequest{Id: "nonexistent"})
	require.Error(t, err)
	require.Contains(t, err.Error(), "not found")
}

func TestQuery_VerificationRound_EmptyID(t *testing.T) {
	k, ctx := setupKnowledgeTest(t)
	qs := keeper.NewQueryServerImpl(k)

	_, err := qs.VerificationRound(ctx, &types.QueryVerificationRoundRequest{Id: ""})
	require.Error(t, err)
	require.Contains(t, err.Error(), "required")
}

// ─── Domain Query ───────────────────────────────────────────────────────────

func TestQuery_Domain_Exists(t *testing.T) {
	k, ctx := setupKnowledgeTest(t)
	qs := keeper.NewQueryServerImpl(k)

	resp, err := qs.Domain(ctx, &types.QueryDomainRequest{Name: "physics"})
	require.NoError(t, err)
	require.Equal(t, "physics", resp.Domain.Name)
	require.Equal(t, types.DomainStatus_DOMAIN_STATUS_ACTIVE, resp.Domain.Status)
}

func TestQuery_Domain_NotFound(t *testing.T) {
	k, ctx := setupKnowledgeTest(t)
	qs := keeper.NewQueryServerImpl(k)

	_, err := qs.Domain(ctx, &types.QueryDomainRequest{Name: "nonexistent"})
	require.Error(t, err)
	require.Contains(t, err.Error(), "not found")
}

func TestQuery_Domain_EmptyName(t *testing.T) {
	k, ctx := setupKnowledgeTest(t)
	qs := keeper.NewQueryServerImpl(k)

	_, err := qs.Domain(ctx, &types.QueryDomainRequest{Name: ""})
	require.Error(t, err)
	require.Contains(t, err.Error(), "required")
}

// ─── Domains Query ──────────────────────────────────────────────────────────

func TestQuery_Domains_All(t *testing.T) {
	k, ctx := setupKnowledgeTest(t)
	qs := keeper.NewQueryServerImpl(k)

	resp, err := qs.Domains(ctx, &types.QueryDomainsRequest{})
	require.NoError(t, err)
	require.Len(t, resp.Domains, 18)
}

// ─── FactConfidence Query ───────────────────────────────────────────────────

func TestQuery_FactConfidence_Exists(t *testing.T) {
	k, ctx := setupKnowledgeTest(t)
	qs := keeper.NewQueryServerImpl(k)

	makeTestFact(t, k, ctx, "qfc-1", "Confidence query test fact", "physics", "empirical", "zrn1sub", 850_000)

	resp, err := qs.FactConfidence(ctx, &types.QueryFactConfidenceRequest{Id: "qfc-1"})
	require.NoError(t, err)
	require.Equal(t, uint64(850_000), resp.Confidence)
}

func TestQuery_FactConfidence_NotFound(t *testing.T) {
	k, ctx := setupKnowledgeTest(t)
	qs := keeper.NewQueryServerImpl(k)

	_, err := qs.FactConfidence(ctx, &types.QueryFactConfidenceRequest{Id: "nonexistent"})
	require.Error(t, err)
	require.Contains(t, err.Error(), "not found")
}

func TestQuery_FactConfidence_EmptyID(t *testing.T) {
	k, ctx := setupKnowledgeTest(t)
	qs := keeper.NewQueryServerImpl(k)

	_, err := qs.FactConfidence(ctx, &types.QueryFactConfidenceRequest{Id: ""})
	require.Error(t, err)
	require.Contains(t, err.Error(), "required")
}

// ─── FactCitationCount Query ────────────────────────────────────────────────

func TestQuery_FactCitationCount_Exists(t *testing.T) {
	k, ctx := setupKnowledgeTest(t)
	qs := keeper.NewQueryServerImpl(k)

	fact := &types.Fact{
		Id:                   "qfcc-1",
		Content:              "Citation count query test",
		Domain:               "physics",
		Status:               types.FactStatus_FACT_STATUS_VERIFIED,
		CitationCount:        5,
		IncomingCitationCount: 3,
	}
	require.NoError(t, k.SetFact(ctx, fact))

	resp, err := qs.FactCitationCount(ctx, &types.QueryFactCitationCountRequest{Id: "qfcc-1"})
	require.NoError(t, err)
	require.Equal(t, uint64(8), resp.Count) // 5 + 3
}

func TestQuery_FactCitationCount_ZeroCitations(t *testing.T) {
	k, ctx := setupKnowledgeTest(t)
	qs := keeper.NewQueryServerImpl(k)

	makeTestFact(t, k, ctx, "qfcc-zero", "Zero citation test fact", "physics", "empirical", "zrn1sub", 700_000)

	resp, err := qs.FactCitationCount(ctx, &types.QueryFactCitationCountRequest{Id: "qfcc-zero"})
	require.NoError(t, err)
	require.Equal(t, uint64(0), resp.Count)
}

func TestQuery_FactCitationCount_NotFound(t *testing.T) {
	k, ctx := setupKnowledgeTest(t)
	qs := keeper.NewQueryServerImpl(k)

	_, err := qs.FactCitationCount(ctx, &types.QueryFactCitationCountRequest{Id: "nonexistent"})
	require.Error(t, err)
	require.Contains(t, err.Error(), "not found")
}

func TestQuery_FactCitationCount_EmptyID(t *testing.T) {
	k, ctx := setupKnowledgeTest(t)
	qs := keeper.NewQueryServerImpl(k)

	_, err := qs.FactCitationCount(ctx, &types.QueryFactCitationCountRequest{Id: ""})
	require.Error(t, err)
	require.Contains(t, err.Error(), "required")
}

// ─── ClaimType Filter Query ─────────────────────────────────────────────────

func TestQuery_Facts_FilterByClaimType(t *testing.T) {
	k, ctx := setupKnowledgeTest(t)
	qs := keeper.NewQueryServerImpl(k)

	// Create facts with different claim types
	assertionFact := &types.Fact{
		Id:        "qfct-assertion",
		Content:   "Water freezes at zero degrees Celsius",
		Domain:    "physics",
		Category:  "empirical",
		Status:    types.FactStatus_FACT_STATUS_VERIFIED,
		ClaimType: types.ClaimType_CLAIM_TYPE_ASSERTION,
	}
	definitionFact := &types.Fact{
		Id:        "qfct-definition",
		Content:   "Entropy means the measure of disorder in a system",
		Domain:    "physics",
		Category:  "empirical",
		Status:    types.FactStatus_FACT_STATUS_VERIFIED,
		ClaimType: types.ClaimType_CLAIM_TYPE_DEFINITION,
	}
	constraintFact := &types.Fact{
		Id:        "qfct-constraint",
		Content:   "Energy must be conserved in all physical processes",
		Domain:    "physics",
		Category:  "empirical",
		Status:    types.FactStatus_FACT_STATUS_VERIFIED,
		ClaimType: types.ClaimType_CLAIM_TYPE_CONSTRAINT,
	}
	require.NoError(t, k.SetFact(ctx, assertionFact))
	require.NoError(t, k.SetFact(ctx, definitionFact))
	require.NoError(t, k.SetFact(ctx, constraintFact))

	// Filter by DEFINITION — should return only the definition fact
	resp, err := qs.Facts(ctx, &types.QueryFactsRequest{
		ClaimType: types.ClaimType_CLAIM_TYPE_DEFINITION,
	})
	require.NoError(t, err)
	require.Len(t, resp.Facts, 1)
	require.Equal(t, "qfct-definition", resp.Facts[0].Id)
	require.Equal(t, types.ClaimType_CLAIM_TYPE_DEFINITION, resp.Facts[0].ClaimType)

	// Filter by ASSERTION — should return only the assertion fact
	resp, err = qs.Facts(ctx, &types.QueryFactsRequest{
		ClaimType: types.ClaimType_CLAIM_TYPE_ASSERTION,
	})
	require.NoError(t, err)
	require.Len(t, resp.Facts, 1)
	require.Equal(t, "qfct-assertion", resp.Facts[0].Id)

	// No filter (UNSPECIFIED) — should return all 3
	resp, err = qs.Facts(ctx, &types.QueryFactsRequest{})
	require.NoError(t, err)
	require.Len(t, resp.Facts, 3)
}
