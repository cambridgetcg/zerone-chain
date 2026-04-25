package keeper

import (
	"context"
	"fmt"

	"github.com/zerone-chain/zerone/x/counterexamples/types"
)

type queryServer struct {
	types.UnimplementedQueryServer
	keeper Keeper
}

func NewQueryServerImpl(k Keeper) types.QueryServer { return &queryServer{keeper: k} }

var _ types.QueryServer = &queryServer{}

func (q *queryServer) Counterexample(ctx context.Context, req *types.QueryCounterexampleRequest) (*types.QueryCounterexampleResponse, error) {
	if req == nil || req.Id == "" {
		return nil, fmt.Errorf("id required")
	}
	c, ok := q.keeper.GetCounterexample(ctx, req.Id)
	if !ok {
		return nil, fmt.Errorf("%w: %s", types.ErrCounterexampleNotFound, req.Id)
	}
	return &types.QueryCounterexampleResponse{Counterexample: c}, nil
}

func (q *queryServer) CounterexamplesByFact(ctx context.Context, req *types.QueryByFactRequest) (*types.QueryByFactResponse, error) {
	if req == nil || req.FactId == "" {
		return nil, fmt.Errorf("fact_id required")
	}
	out := []*types.Counterexample{}
	_ = q.keeper.IterateCounterexamplesByFact(ctx, req.FactId, func(c *types.Counterexample) bool {
		out = append(out, c)
		return false
	})
	return &types.QueryByFactResponse{Counterexamples: out}, nil
}

func (q *queryServer) Validations(ctx context.Context, req *types.QueryValidationsRequest) (*types.QueryValidationsResponse, error) {
	if req == nil || req.CounterexampleId == "" {
		return nil, fmt.Errorf("counterexample_id required")
	}
	out := []*types.Validation{}
	_ = q.keeper.IterateValidationsByCE(ctx, req.CounterexampleId, func(v *types.Validation) bool {
		out = append(out, v)
		return false
	})
	return &types.QueryValidationsResponse{Validations: out}, nil
}

func (q *queryServer) Params(ctx context.Context, _ *types.QueryParamsRequest) (*types.QueryParamsResponse, error) {
	p := q.keeper.GetParams(ctx)
	return &types.QueryParamsResponse{Params: &p}, nil
}
