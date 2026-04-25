package keeper

import (
	"context"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/zerone-chain/zerone/x/training_provenance/types"
)

var _ types.QueryServer = queryServer{}

type queryServer struct {
	types.UnimplementedQueryServer
	keeper Keeper
}

// NewQueryServerImpl returns a query server for the training_provenance module.
func NewQueryServerImpl(k Keeper) types.QueryServer {
	return queryServer{keeper: k}
}

// ProvenanceCertificate is the module's only query: synthesise the cert
// for the named manifest from the keepers' current state.
func (q queryServer) ProvenanceCertificate(ctx context.Context, req *types.QueryProvenanceCertificateRequest) (*types.QueryProvenanceCertificateResponse, error) {
	if req == nil || req.ManifestId == "" {
		return nil, status.Error(codes.InvalidArgument, "manifest_id required")
	}
	cert, err := q.keeper.BuildCertificate(ctx, req.ManifestId)
	if err != nil {
		return nil, status.Error(codes.NotFound, err.Error())
	}
	return &types.QueryProvenanceCertificateResponse{Certificate: cert}, nil
}
