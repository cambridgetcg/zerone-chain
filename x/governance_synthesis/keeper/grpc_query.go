package keeper

import (
	"context"

	"github.com/zerone-chain/zerone/x/governance_synthesis/types"
)

var _ types.QueryServer = queryServer{}

type queryServer struct {
	types.UnimplementedQueryServer
	keeper Keeper
}

func NewQueryServerImpl(k Keeper) types.QueryServer {
	return queryServer{keeper: k}
}

func (q queryServer) SystemHealth(ctx context.Context, _ *types.QuerySystemHealthRequest) (*types.QuerySystemHealthResponse, error) {
	return &types.QuerySystemHealthResponse{Health: q.keeper.BuildHealth(ctx)}, nil
}

func (q queryServer) Frontier(ctx context.Context, req *types.QueryFrontierRequest) (*types.QueryFrontierResponse, error) {
	limit := uint32(0)
	if req != nil {
		limit = req.Limit
	}
	const maxLimit = uint32(100)
	if limit == 0 || limit > maxLimit {
		limit = maxLimit
	}
	f := q.keeper.ComposeFrontier(ctx, limit)
	return &types.QueryFrontierResponse{Frontier: &f}, nil
}
