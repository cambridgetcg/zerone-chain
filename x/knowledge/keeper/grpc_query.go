package keeper

import (
	"context"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/zerone-chain/zerone/x/knowledge/types"
)

var _ types.QueryServer = queryServer{}

type queryServer struct {
	types.UnimplementedQueryServer
	keeper Keeper
}

// NewQueryServerImpl returns an implementation of QueryServer for the knowledge module.
func NewQueryServerImpl(keeper Keeper) types.QueryServer {
	return &queryServer{keeper: keeper}
}

func (q queryServer) Sample(_ context.Context, _ *types.QuerySampleRequest) (*types.QuerySampleResponse, error) {
	return nil, status.Error(codes.Unimplemented, "Sample not implemented (R37)")
}

func (q queryServer) Samples(_ context.Context, _ *types.QuerySamplesRequest) (*types.QuerySamplesResponse, error) {
	return nil, status.Error(codes.Unimplemented, "Samples not implemented (R37)")
}

func (q queryServer) SamplesByDomain(_ context.Context, _ *types.QuerySamplesByDomainRequest) (*types.QuerySamplesResponse, error) {
	return nil, status.Error(codes.Unimplemented, "SamplesByDomain not implemented (R37)")
}

func (q queryServer) SamplesByThread(_ context.Context, _ *types.QuerySamplesByThreadRequest) (*types.QuerySamplesResponse, error) {
	return nil, status.Error(codes.Unimplemented, "SamplesByThread not implemented (R37)")
}

func (q queryServer) SamplesBySubmitter(_ context.Context, _ *types.QuerySamplesBySubmitterRequest) (*types.QuerySamplesResponse, error) {
	return nil, status.Error(codes.Unimplemented, "SamplesBySubmitter not implemented (R37)")
}

func (q queryServer) Submission(_ context.Context, _ *types.QuerySubmissionRequest) (*types.QuerySubmissionResponse, error) {
	return nil, status.Error(codes.Unimplemented, "Submission not implemented (R37)")
}

func (q queryServer) PendingSubmissions(_ context.Context, _ *types.QueryPendingSubmissionsRequest) (*types.QuerySubmissionsResponse, error) {
	return nil, status.Error(codes.Unimplemented, "PendingSubmissions not implemented (R37)")
}

func (q queryServer) QualityRound(_ context.Context, _ *types.QueryQualityRoundRequest) (*types.QueryQualityRoundResponse, error) {
	return nil, status.Error(codes.Unimplemented, "QualityRound not implemented (R37)")
}

func (q queryServer) Dataset(_ context.Context, _ *types.QueryDatasetRequest) (*types.QueryDatasetResponse, error) {
	return nil, status.Error(codes.Unimplemented, "Dataset not implemented (R37)")
}

func (q queryServer) Datasets(_ context.Context, _ *types.QueryDatasetsRequest) (*types.QueryDatasetsResponse, error) {
	return nil, status.Error(codes.Unimplemented, "Datasets not implemented (R37)")
}

func (q queryServer) TrainingDemand(_ context.Context, _ *types.QueryTrainingDemandRequest) (*types.QueryTrainingDemandResponse, error) {
	return nil, status.Error(codes.Unimplemented, "TrainingDemand not implemented (R37)")
}

func (q queryServer) DataBounties(_ context.Context, _ *types.QueryDataBountiesRequest) (*types.QueryDataBountiesResponse, error) {
	return nil, status.Error(codes.Unimplemented, "DataBounties not implemented (R37)")
}

func (q queryServer) Domain(_ context.Context, _ *types.QueryDomainRequest) (*types.QueryDomainResponse, error) {
	return nil, status.Error(codes.Unimplemented, "Domain not implemented (R37)")
}

func (q queryServer) Domains(_ context.Context, _ *types.QueryDomainsRequest) (*types.QueryDomainsResponse, error) {
	return nil, status.Error(codes.Unimplemented, "Domains not implemented (R37)")
}

func (q queryServer) DomainStats(_ context.Context, _ *types.QueryDomainStatsRequest) (*types.QueryDomainStatsResponse, error) {
	return nil, status.Error(codes.Unimplemented, "DomainStats not implemented (R37)")
}

func (q queryServer) ProtocolStats(_ context.Context, _ *types.QueryProtocolStatsRequest) (*types.QueryProtocolStatsResponse, error) {
	return nil, status.Error(codes.Unimplemented, "ProtocolStats not implemented (R37)")
}

func (q queryServer) Params(_ context.Context, _ *types.QueryParamsRequest) (*types.QueryParamsResponse, error) {
	return nil, status.Error(codes.Unimplemented, "Params not implemented (R37)")
}
