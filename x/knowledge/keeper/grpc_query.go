package keeper

import (
	"context"
	"fmt"
	"strconv"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/zerone-chain/zerone/x/knowledge/types"
)

type queryServer struct {
	keeper Keeper
	types.UnimplementedQueryServer
}

// NewQueryServerImpl returns a types.QueryServer backed by the given Keeper.
func NewQueryServerImpl(keeper Keeper) types.QueryServer {
	return &queryServer{keeper: keeper}
}

func (q *queryServer) Params(ctx context.Context, _ *types.QueryParamsRequest) (*types.QueryParamsResponse, error) {
	params, err := q.keeper.GetParams(ctx)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	return &types.QueryParamsResponse{Params: params}, nil
}

func (q *queryServer) Fact(ctx context.Context, req *types.QueryFactRequest) (*types.QueryFactResponse, error) {
	if req.Id == "" {
		return nil, status.Error(codes.InvalidArgument, "fact id is required")
	}
	fact, found := q.keeper.GetFact(ctx, req.Id)
	if !found {
		return nil, status.Errorf(codes.NotFound, "fact %s not found", req.Id)
	}

	// Track query — increment counter (agents set this)
	if req.TrackQuery {
		q.keeper.IncrementFactQueryCount(ctx, req.Id)
	}

	return &types.QueryFactResponse{Fact: fact}, nil
}

func (q *queryServer) Facts(ctx context.Context, req *types.QueryFactsRequest) (*types.QueryFactsResponse, error) {
	var facts []*types.Fact

	// If domain filter is specified, use the secondary index
	if req.Domain != "" {
		q.keeper.IterateFactsByDomain(ctx, req.Domain, func(factID string) bool {
			fact, found := q.keeper.GetFact(ctx, factID)
			if found {
				if matchesFactFilters(fact, req.Status, req.Category, req.ClaimType) {
					facts = append(facts, fact)
				}
			}
			return false
		})
	} else {
		q.keeper.IterateFacts(ctx, func(fact *types.Fact) bool {
			if matchesFactFilters(fact, req.Status, req.Category, req.ClaimType) {
				facts = append(facts, fact)
			}
			return false
		})
	}

	return &types.QueryFactsResponse{Facts: facts}, nil
}

func (q *queryServer) FactsByDomain(ctx context.Context, req *types.QueryFactsByDomainRequest) (*types.QueryFactsByDomainResponse, error) {
	if req.Domain == "" {
		return nil, status.Error(codes.InvalidArgument, "domain is required")
	}

	var facts []*types.Fact
	q.keeper.IterateFactsByDomain(ctx, req.Domain, func(factID string) bool {
		fact, found := q.keeper.GetFact(ctx, factID)
		if found {
			facts = append(facts, fact)
		}
		return false
	})

	return &types.QueryFactsByDomainResponse{Facts: facts}, nil
}

func (q *queryServer) FactsBySubmitter(ctx context.Context, req *types.QueryFactsBySubmitterRequest) (*types.QueryFactsBySubmitterResponse, error) {
	if req.Submitter == "" {
		return nil, status.Error(codes.InvalidArgument, "submitter is required")
	}

	var facts []*types.Fact
	q.keeper.IterateFactsBySubmitter(ctx, req.Submitter, func(factID string) bool {
		fact, found := q.keeper.GetFact(ctx, factID)
		if found {
			facts = append(facts, fact)
		}
		return false
	})

	return &types.QueryFactsBySubmitterResponse{Facts: facts}, nil
}

func (q *queryServer) Claim(ctx context.Context, req *types.QueryClaimRequest) (*types.QueryClaimResponse, error) {
	if req.Id == "" {
		return nil, status.Error(codes.InvalidArgument, "claim id is required")
	}
	claim, found := q.keeper.GetClaim(ctx, req.Id)
	if !found {
		return nil, status.Errorf(codes.NotFound, "claim %s not found", req.Id)
	}
	return &types.QueryClaimResponse{Claim: claim}, nil
}

func (q *queryServer) PendingClaims(ctx context.Context, _ *types.QueryPendingClaimsRequest) (*types.QueryPendingClaimsResponse, error) {
	var claims []*types.Claim
	q.keeper.IterateClaims(ctx, func(claim *types.Claim) bool {
		if claim.Status == types.ClaimStatus_CLAIM_STATUS_PENDING {
			claims = append(claims, claim)
		}
		return false
	})
	return &types.QueryPendingClaimsResponse{Claims: claims}, nil
}

func (q *queryServer) VerificationRound(ctx context.Context, req *types.QueryVerificationRoundRequest) (*types.QueryVerificationRoundResponse, error) {
	if req.Id == "" {
		return nil, status.Error(codes.InvalidArgument, "round id is required")
	}
	round, found := q.keeper.GetVerificationRound(ctx, req.Id)
	if !found {
		return nil, status.Errorf(codes.NotFound, "verification round %s not found", req.Id)
	}
	return &types.QueryVerificationRoundResponse{Round: round}, nil
}

func (q *queryServer) Domain(ctx context.Context, req *types.QueryDomainRequest) (*types.QueryDomainResponse, error) {
	if req.Name == "" {
		return nil, status.Error(codes.InvalidArgument, "domain name is required")
	}
	domain, found := q.keeper.GetDomain(ctx, req.Name)
	if !found {
		return nil, status.Errorf(codes.NotFound, "domain %s not found", req.Name)
	}
	return &types.QueryDomainResponse{Domain: domain}, nil
}

func (q *queryServer) Domains(ctx context.Context, _ *types.QueryDomainsRequest) (*types.QueryDomainsResponse, error) {
	var domains []*types.Domain
	q.keeper.IterateDomains(ctx, func(domain *types.Domain) bool {
		domains = append(domains, domain)
		return false
	})
	return &types.QueryDomainsResponse{Domains: domains}, nil
}

func (q *queryServer) FactConfidence(ctx context.Context, req *types.QueryFactConfidenceRequest) (*types.QueryFactConfidenceResponse, error) {
	if req.Id == "" {
		return nil, status.Error(codes.InvalidArgument, "fact id is required")
	}
	fact, found := q.keeper.GetFact(ctx, req.Id)
	if !found {
		return nil, status.Errorf(codes.NotFound, "fact %s not found", req.Id)
	}
	return &types.QueryFactConfidenceResponse{Confidence: fact.Confidence}, nil
}

func (q *queryServer) FactCitationCount(ctx context.Context, req *types.QueryFactCitationCountRequest) (*types.QueryFactCitationCountResponse, error) {
	if req.Id == "" {
		return nil, status.Error(codes.InvalidArgument, "fact id is required")
	}
	fact, found := q.keeper.GetFact(ctx, req.Id)
	if !found {
		return nil, status.Errorf(codes.NotFound, "fact %s not found", req.Id)
	}
	return &types.QueryFactCitationCountResponse{
		Count: fact.CitationCount + fact.IncomingCitationCount,
	}, nil
}

func (q *queryServer) FactRelations(ctx context.Context, req *types.QueryFactRelationsRequest) (*types.QueryFactRelationsResponse, error) {
	if req.FactId == "" {
		return nil, status.Error(codes.InvalidArgument, "fact_id is required")
	}

	// Verify fact exists
	if _, found := q.keeper.GetFact(ctx, req.FactId); !found {
		return nil, status.Errorf(codes.NotFound, "fact %s not found", req.FactId)
	}

	direction := req.Direction
	if direction == "" {
		direction = "both"
	}

	var relations []*types.FactRelation

	if direction == "outgoing" || direction == "both" {
		outgoing, err := q.keeper.GetFactRelations(ctx, req.FactId)
		if err != nil {
			return nil, status.Error(codes.Internal, err.Error())
		}
		relations = append(relations, outgoing...)
	}

	if direction == "incoming" || direction == "both" {
		incoming, err := q.keeper.GetIncomingRelations(ctx, req.FactId)
		if err != nil {
			return nil, status.Error(codes.Internal, err.Error())
		}
		relations = append(relations, incoming...)
	}

	// Apply optional type filter
	if req.Relation != types.RelationType_RELATION_TYPE_UNSPECIFIED {
		var filtered []*types.FactRelation
		for _, rel := range relations {
			if rel.Relation == req.Relation {
				filtered = append(filtered, rel)
			}
		}
		relations = filtered
	}

	return &types.QueryFactRelationsResponse{Relations: relations}, nil
}

func (q *queryServer) FactsBySubject(ctx context.Context, req *types.QueryFactsBySubjectRequest) (*types.QueryFactsBySubjectResponse, error) {
	if req.Domain == "" {
		return nil, status.Error(codes.InvalidArgument, "domain is required")
	}
	if req.Subject == "" {
		return nil, status.Error(codes.InvalidArgument, "subject is required")
	}

	factID := q.keeper.FindFactBySubjectPredicate(ctx, req.Domain, req.Subject, "")
	if factID == "" {
		return &types.QueryFactsBySubjectResponse{}, nil
	}

	fact, found := q.keeper.GetFact(ctx, factID)
	if !found {
		return &types.QueryFactsBySubjectResponse{}, nil
	}

	return &types.QueryFactsBySubjectResponse{Facts: []*types.Fact{fact}}, nil
}

func (q *queryServer) FactsByTag(ctx context.Context, req *types.QueryFactsByTagRequest) (*types.QueryFactsByTagResponse, error) {
	if req.Tag == "" {
		return nil, status.Error(codes.InvalidArgument, "tag is required")
	}

	factIDs, err := q.keeper.FindFactsByTag(ctx, req.Tag)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	var facts []*types.Fact
	for _, id := range factIDs {
		fact, found := q.keeper.GetFact(ctx, id)
		if found {
			facts = append(facts, fact)
		}
	}

	return &types.QueryFactsByTagResponse{Facts: facts}, nil
}

func (q *queryServer) FactByCanonical(ctx context.Context, req *types.QueryFactByCanonicalRequest) (*types.QueryFactByCanonicalResponse, error) {
	canonicalHash := req.CanonicalHash
	if canonicalHash == "" && req.CanonicalForm != "" {
		// Hash the provided form server-side
		normalized := types.NormalizeCanonicalForm(req.CanonicalForm)
		canonicalHash = types.HashCanonicalForm(normalized)
	}
	if canonicalHash == "" {
		return nil, status.Error(codes.InvalidArgument, "canonical_hash or canonical_form is required")
	}

	id, found := q.keeper.GetClaimByCanonicalHash(ctx, canonicalHash)
	if !found {
		return nil, status.Errorf(codes.NotFound, "no fact/claim found for canonical hash %s", canonicalHash)
	}

	// Try fact first, then claim's provisional fact
	fact, found := q.keeper.GetFact(ctx, id)
	if found {
		return &types.QueryFactByCanonicalResponse{Fact: fact}, nil
	}

	// The index might point to a claim ID — check if that claim has a fact
	claim, found := q.keeper.GetClaim(ctx, id)
	if found && claim.ProvisionalFactId != "" {
		fact, found = q.keeper.GetFact(ctx, claim.ProvisionalFactId)
		if found {
			return &types.QueryFactByCanonicalResponse{Fact: fact}, nil
		}
	}

	// Search for fact with matching canonical hash
	var matchedFact *types.Fact
	q.keeper.IterateFacts(ctx, func(f *types.Fact) bool {
		if f.CanonicalHash == canonicalHash {
			matchedFact = f
			return true
		}
		return false
	})
	if matchedFact != nil {
		return &types.QueryFactByCanonicalResponse{Fact: matchedFact}, nil
	}

	return nil, status.Errorf(codes.NotFound, "no fact found for canonical hash %s", canonicalHash)
}

func (q *queryServer) FactsByFitness(ctx context.Context, req *types.QueryFactsByFitnessRequest) (*types.QueryFactsByFitnessResponse, error) {
	ascending := req.Order == "asc"
	facts := q.keeper.GetFactsByFitness(ctx, req.Domain, req.MinFitness, req.Limit, ascending)
	return &types.QueryFactsByFitnessResponse{Facts: facts}, nil
}

// matchesFactFilters checks if a fact passes optional status, category, and claim type filters.
func matchesFactFilters(fact *types.Fact, statusFilter, categoryFilter string, claimTypeFilter types.ClaimType) bool {
	if statusFilter != "" {
		if fact.Status.String() != statusFilter {
			return false
		}
	}
	if categoryFilter != "" {
		if fact.Category != categoryFilter {
			return false
		}
	}
	if claimTypeFilter != types.ClaimType_CLAIM_TYPE_UNSPECIFIED {
		if fact.ClaimType != claimTypeFilter {
			return false
		}
	}
	return true
}

func (q *queryServer) BootstrapFundStatus(ctx context.Context, _ *types.QueryBootstrapFundStatusRequest) (*types.QueryBootstrapFundStatusResponse, error) {
	params, err := q.keeper.GetParams(ctx)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	balance := q.keeper.GetBootstrapFundBalance(ctx)

	// Calculate total claims funded (sum of all per-address counts would be expensive,
	// so we track via epoch counts across all epochs — approximate via current epoch)
	epoch := q.keeper.CurrentEpoch(ctx)
	var totalClaims uint64
	for e := uint64(0); e <= epoch; e++ {
		totalClaims += q.keeper.GetBootstrapEpochCount(ctx, e)
	}

	// Calculate remaining per epoch
	maxPerEpoch, _ := strconv.ParseUint(params.BootstrapFundMaxPerEpoch, 10, 64)
	currentEpochCount := q.keeper.GetBootstrapEpochCount(ctx, epoch)
	remaining := uint64(0)
	if maxPerEpoch > currentEpochCount {
		remaining = maxPerEpoch - currentEpochCount
	}

	return &types.QueryBootstrapFundStatusResponse{
		Balance:            balance.Amount.String(),
		Enabled:            params.BootstrapFundEnabled,
		TotalClaimsFunded:  fmt.Sprintf("%d", totalClaims),
		TotalAmountSpent:   "0", // Not tracked separately — can be derived from genesis allocation minus balance
		RemainingPerEpoch:  fmt.Sprintf("%d", remaining),
	}, nil
}

func (q *queryServer) FactsAtRisk(ctx context.Context, req *types.QueryFactsAtRiskRequest) (*types.QueryFactsAtRiskResponse, error) {
	limit := req.Limit
	if limit == 0 {
		limit = 50
	}

	var facts []*types.Fact
	q.keeper.IterateFacts(ctx, func(fact *types.Fact) bool {
		if fact.Status != types.FactStatus_FACT_STATUS_AT_RISK &&
			fact.Status != types.FactStatus_FACT_STATUS_EXPIRED {
			return false
		}
		if req.Domain != "" && fact.Domain != req.Domain {
			return false
		}
		facts = append(facts, fact)
		return uint64(len(facts)) >= limit
	})

	return &types.QueryFactsAtRiskResponse{Facts: facts}, nil
}
