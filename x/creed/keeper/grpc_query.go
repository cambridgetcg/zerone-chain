package keeper

import (
	"context"

	"github.com/zerone-chain/zerone/x/creed/types"
)

type queryServer struct {
	types.UnimplementedQueryServer
	keeper Keeper
}

func NewQueryServerImpl(k Keeper) types.QueryServer { return &queryServer{keeper: k} }

var _ types.QueryServer = &queryServer{}

func (q *queryServer) Pinned(ctx context.Context, _ *types.QueryPinnedRequest) (*types.QueryPinnedResponse, error) {
	pin, ok := q.keeper.GetCurrentPin(ctx)
	if !ok {
		return nil, types.ErrPinNotFound.Wrap("no pin recorded yet")
	}
	return &types.QueryPinnedResponse{Pin: pin}, nil
}

func (q *queryServer) PinAtVersion(ctx context.Context, req *types.QueryPinAtVersionRequest) (*types.QueryPinAtVersionResponse, error) {
	if req.Version == 0 {
		return nil, types.ErrPinNotFound.Wrap("version must be ≥ 1")
	}
	pin, ok := q.keeper.GetPin(ctx, req.Version)
	if !ok {
		return nil, types.ErrPinNotFound.Wrapf("no pin at version %d", req.Version)
	}
	return &types.QueryPinAtVersionResponse{Pin: pin}, nil
}

func (q *queryServer) PinHistory(ctx context.Context, req *types.QueryPinHistoryRequest) (*types.QueryPinHistoryResponse, error) {
	limit := req.Limit
	if limit == 0 || limit > 100 {
		limit = 50
	}
	cur := q.keeper.GetCurrentVersion(ctx)
	startBefore := req.StartBeforeVersion
	if startBefore == 0 || startBefore > cur {
		startBefore = cur + 1
	}

	pins := []*types.PinnedCreed{}
	var nextStart uint32
	count := uint32(0)
	for v := startBefore - 1; v > 0; v-- {
		if count >= limit {
			nextStart = v + 1
			break
		}
		p, ok := q.keeper.GetPin(ctx, v)
		if !ok {
			continue
		}
		pins = append(pins, p)
		count++
	}
	return &types.QueryPinHistoryResponse{
		Pins:                   pins,
		NextStartBeforeVersion: nextStart,
	}, nil
}

func (q *queryServer) Commitment(ctx context.Context, req *types.QueryCommitmentRequest) (*types.QueryCommitmentResponse, error) {
	entry, ok := q.keeper.CurrentCommitment(ctx, req.Number)
	if !ok {
		return nil, types.ErrCommitmentNotFound.Wrapf("commitment %d not currently active", req.Number)
	}
	return &types.QueryCommitmentResponse{Entry: entry}, nil
}

func (q *queryServer) Params(ctx context.Context, _ *types.QueryParamsRequest) (*types.QueryParamsResponse, error) {
	p := q.keeper.GetParams(ctx)
	return &types.QueryParamsResponse{Params: &p}, nil
}
