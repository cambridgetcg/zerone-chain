package keeper

import (
	"context"

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
	return &types.QueryFactResponse{Fact: fact}, nil
}

func (q *queryServer) Facts(ctx context.Context, req *types.QueryFactsRequest) (*types.QueryFactsResponse, error) {
	var facts []*types.Fact

	// If domain filter is specified, use the secondary index
	if req.Domain != "" {
		q.keeper.IterateFactsByDomain(ctx, req.Domain, func(factID string) bool {
			fact, found := q.keeper.GetFact(ctx, factID)
			if found {
				if matchesFactFilters(fact, req.Status, req.Category) {
					facts = append(facts, fact)
				}
			}
			return false
		})
	} else {
		q.keeper.IterateFacts(ctx, func(fact *types.Fact) bool {
			if matchesFactFilters(fact, req.Status, req.Category) {
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

// matchesFactFilters checks if a fact passes optional status and category filters.
func matchesFactFilters(fact *types.Fact, statusFilter, categoryFilter string) bool {
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
	return true
}
