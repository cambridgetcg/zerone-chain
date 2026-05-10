package keeper

import (
	"context"

	"github.com/zerone-chain/zerone/x/sponsorship/types"
)

type queryServer struct {
	types.UnimplementedQueryServer
	Keeper
}

func NewQueryServerImpl(k Keeper) types.QueryServer { return &queryServer{Keeper: k} }

func (q queryServer) Params(ctx context.Context, _ *types.QueryParamsRequest) (*types.QueryParamsResponse, error) {
	return &types.QueryParamsResponse{Params: q.GetParams(ctx)}, nil
}

func (q queryServer) BountyOrder(ctx context.Context, req *types.QueryBountyOrderRequest) (*types.QueryBountyOrderResponse, error) {
	o, ok := q.GetBountyOrder(ctx, req.Id)
	if !ok {
		return nil, types.ErrBountyNotFound
	}
	return &types.QueryBountyOrderResponse{Order: o}, nil
}

func (q queryServer) BountyOrders(ctx context.Context, _ *types.QueryBountyOrdersRequest) (*types.QueryBountyOrdersResponse, error) {
	return &types.QueryBountyOrdersResponse{Orders: q.GetAllBountyOrders(ctx)}, nil
}
