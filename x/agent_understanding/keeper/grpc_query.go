package keeper

import (
	"context"
	"fmt"

	"github.com/zerone-chain/zerone/x/agent_understanding/types"
)

type queryServer struct {
	types.UnimplementedQueryServer
	keeper Keeper
}

func NewQueryServerImpl(k Keeper) types.QueryServer { return &queryServer{keeper: k} }

var _ types.QueryServer = &queryServer{}

func (q *queryServer) AgentProfile(ctx context.Context, req *types.QueryAgentProfileRequest) (*types.QueryAgentProfileResponse, error) {
	if req == nil || req.Agent == "" {
		return nil, fmt.Errorf("agent required")
	}
	p := q.keeper.ComposeProfile(ctx, req.Agent)
	return &types.QueryAgentProfileResponse{Profile: p}, nil
}

func (q *queryServer) AgentDomainProfile(ctx context.Context, req *types.QueryAgentDomainProfileRequest) (*types.QueryAgentDomainProfileResponse, error) {
	if req == nil || req.Agent == "" || req.Domain == "" {
		return nil, fmt.Errorf("agent and domain required")
	}
	dp := q.keeper.ComposeDomainProfile(ctx, req.Agent, req.Domain)
	return &types.QueryAgentDomainProfileResponse{Profile: dp}, nil
}

func (q *queryServer) Params(ctx context.Context, _ *types.QueryParamsRequest) (*types.QueryParamsResponse, error) {
	p := q.keeper.GetParams(ctx)
	return &types.QueryParamsResponse{Params: &p}, nil
}
