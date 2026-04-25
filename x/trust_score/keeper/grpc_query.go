package keeper

import (
	"context"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/zerone-chain/zerone/x/trust_score/types"
)

var _ types.QueryServer = queryServer{}

type queryServer struct {
	types.UnimplementedQueryServer
	keeper Keeper
}

func NewQueryServerImpl(k Keeper) types.QueryServer {
	return queryServer{keeper: k}
}

func (q queryServer) TrustScore(ctx context.Context, req *types.QueryTrustScoreRequest) (*types.QueryTrustScoreResponse, error) {
	if req == nil || req.Address == "" {
		return nil, status.Error(codes.InvalidArgument, "address required")
	}
	score, err := q.keeper.BuildScore(ctx, req.Address)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	return &types.QueryTrustScoreResponse{Score: score}, nil
}
