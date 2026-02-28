package keeper

import (
	"context"

	"github.com/zerone-chain/zerone/x/alignment/types"
)

type queryServer struct {
	types.UnimplementedQueryServer
	Keeper
}

// NewQueryServerImpl returns a Query service implementation.
func NewQueryServerImpl(keeper Keeper) types.QueryServer {
	return &queryServer{Keeper: keeper}
}

func (q queryServer) Params(ctx context.Context, req *types.QueryParamsRequest) (*types.QueryParamsResponse, error) {
	params := q.Keeper.GetParams(ctx)
	return &types.QueryParamsResponse{Params: params}, nil
}

func (q queryServer) State(ctx context.Context, req *types.QueryStateRequest) (*types.QueryStateResponse, error) {
	state := q.Keeper.GetState(ctx)
	return &types.QueryStateResponse{State: state}, nil
}

func (q queryServer) Observation(ctx context.Context, req *types.QueryObservationRequest) (*types.QueryObservationResponse, error) {
	obs, found := q.Keeper.GetObservation(ctx, req.Height)
	return &types.QueryObservationResponse{Observation: obs, Found: found}, nil
}

func (q queryServer) Scores(ctx context.Context, req *types.QueryScoresRequest) (*types.QueryScoresResponse, error) {
	scores, found := q.Keeper.GetScores(ctx, req.Height)
	return &types.QueryScoresResponse{Scores: scores, Found: found}, nil
}

func (q queryServer) HealthIndex(ctx context.Context, req *types.QueryHealthIndexRequest) (*types.QueryHealthIndexResponse, error) {
	hi, found := q.Keeper.GetHealthIndex(ctx, req.Height)
	return &types.QueryHealthIndexResponse{HealthIndex: hi, Found: found}, nil
}

func (q queryServer) CorrectionHistory(ctx context.Context, req *types.QueryCorrectionHistoryRequest) (*types.QueryCorrectionHistoryResponse, error) {
	limit := req.Limit
	if limit == 0 || limit > 100 {
		limit = 100
	}
	corrections, total := q.Keeper.GetCorrections(ctx, limit, req.Offset)
	return &types.QueryCorrectionHistoryResponse{
		Corrections: corrections,
		Total:       total,
	}, nil
}

func (q queryServer) HealthHistory(ctx context.Context, req *types.QueryHealthHistoryRequest) (*types.QueryHealthHistoryResponse, error) {
	entries := q.Keeper.GetRecentHealthIndices(ctx, req.Limit)
	return &types.QueryHealthHistoryResponse{Entries: entries}, nil
}
