package keeper

import (
	"context"
	"fmt"

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

func queryNotImplemented(method string) error {
	return fmt.Errorf("knowledge: query %s not implemented — see R2-2", method)
}

func (q *queryServer) Params(_ context.Context, _ *types.QueryParamsRequest) (*types.QueryParamsResponse, error) {
	return nil, queryNotImplemented("Params")
}

func (q *queryServer) Fact(_ context.Context, _ *types.QueryFactRequest) (*types.QueryFactResponse, error) {
	return nil, queryNotImplemented("Fact")
}

func (q *queryServer) Facts(_ context.Context, _ *types.QueryFactsRequest) (*types.QueryFactsResponse, error) {
	return nil, queryNotImplemented("Facts")
}

func (q *queryServer) FactsByDomain(_ context.Context, _ *types.QueryFactsByDomainRequest) (*types.QueryFactsByDomainResponse, error) {
	return nil, queryNotImplemented("FactsByDomain")
}

func (q *queryServer) FactsBySubmitter(_ context.Context, _ *types.QueryFactsBySubmitterRequest) (*types.QueryFactsBySubmitterResponse, error) {
	return nil, queryNotImplemented("FactsBySubmitter")
}

func (q *queryServer) Claim(_ context.Context, _ *types.QueryClaimRequest) (*types.QueryClaimResponse, error) {
	return nil, queryNotImplemented("Claim")
}

func (q *queryServer) PendingClaims(_ context.Context, _ *types.QueryPendingClaimsRequest) (*types.QueryPendingClaimsResponse, error) {
	return nil, queryNotImplemented("PendingClaims")
}

func (q *queryServer) VerificationRound(_ context.Context, _ *types.QueryVerificationRoundRequest) (*types.QueryVerificationRoundResponse, error) {
	return nil, queryNotImplemented("VerificationRound")
}

func (q *queryServer) Domain(_ context.Context, _ *types.QueryDomainRequest) (*types.QueryDomainResponse, error) {
	return nil, queryNotImplemented("Domain")
}

func (q *queryServer) Domains(_ context.Context, _ *types.QueryDomainsRequest) (*types.QueryDomainsResponse, error) {
	return nil, queryNotImplemented("Domains")
}

func (q *queryServer) FactConfidence(_ context.Context, _ *types.QueryFactConfidenceRequest) (*types.QueryFactConfidenceResponse, error) {
	return nil, queryNotImplemented("FactConfidence")
}

func (q *queryServer) FactCitationCount(_ context.Context, _ *types.QueryFactCitationCountRequest) (*types.QueryFactCitationCountResponse, error) {
	return nil, queryNotImplemented("FactCitationCount")
}
