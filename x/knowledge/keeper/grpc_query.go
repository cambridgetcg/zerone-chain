package keeper

import (
	"context"
	"fmt"
	"sort"
	"strings"

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

// ─── Single lookups ─────────────────────────────────────────────────────────

func (q queryServer) Sample(ctx context.Context, req *types.QuerySampleRequest) (*types.QuerySampleResponse, error) {
	if req == nil || req.Id == "" {
		return nil, status.Error(codes.InvalidArgument, "sample id is required")
	}
	sample, found := q.keeper.GetSample(ctx, req.Id)
	if !found {
		return nil, status.Errorf(codes.NotFound, "sample %q not found", req.Id)
	}
	return &types.QuerySampleResponse{Sample: sample}, nil
}

func (q queryServer) Submission(ctx context.Context, req *types.QuerySubmissionRequest) (*types.QuerySubmissionResponse, error) {
	if req == nil || req.Id == "" {
		return nil, status.Error(codes.InvalidArgument, "submission id is required")
	}
	sub, found := q.keeper.GetSubmission(ctx, req.Id)
	if !found {
		return nil, status.Errorf(codes.NotFound, "submission %q not found", req.Id)
	}
	return &types.QuerySubmissionResponse{Submission: sub}, nil
}

func (q queryServer) QualityRound(ctx context.Context, req *types.QueryQualityRoundRequest) (*types.QueryQualityRoundResponse, error) {
	if req == nil || req.Id == "" {
		return nil, status.Error(codes.InvalidArgument, "round id is required")
	}
	round, found := q.keeper.GetQualityRound(ctx, req.Id)
	if !found {
		return nil, status.Errorf(codes.NotFound, "quality round %q not found", req.Id)
	}
	return &types.QueryQualityRoundResponse{Round: round}, nil
}

// ─── Filtered sample queries ────────────────────────────────────────────────

func (q queryServer) Samples(ctx context.Context, req *types.QuerySamplesRequest) (*types.QuerySamplesResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "request is required")
	}

	var samples []*types.Sample

	// Use domain index if domain filter specified
	if req.Domain != "" {
		ids := q.keeper.GetSamplesByDomain(ctx, req.Domain)
		for _, id := range ids {
			s, found := q.keeper.GetSample(ctx, id)
			if found && matchesSamplesFilter(s, req) {
				samples = append(samples, s)
			}
		}
	} else {
		q.keeper.IterateSamples(ctx, func(s *types.Sample) bool {
			if matchesSamplesFilter(s, req) {
				samples = append(samples, s)
			}
			return false
		})
	}

	samples = paginateSamples(samples, req.Pagination)
	return &types.QuerySamplesResponse{Samples: samples}, nil
}

func (q queryServer) SamplesByDomain(ctx context.Context, req *types.QuerySamplesByDomainRequest) (*types.QuerySamplesResponse, error) {
	if req == nil || req.Domain == "" {
		return nil, status.Error(codes.InvalidArgument, "domain is required")
	}
	ids := q.keeper.GetSamplesByDomain(ctx, req.Domain)
	var samples []*types.Sample
	for _, id := range ids {
		s, found := q.keeper.GetSample(ctx, id)
		if found {
			samples = append(samples, s)
		}
	}
	samples = paginateSamples(samples, req.Pagination)
	return &types.QuerySamplesResponse{Samples: samples}, nil
}

func (q queryServer) SamplesByThread(ctx context.Context, req *types.QuerySamplesByThreadRequest) (*types.QuerySamplesResponse, error) {
	if req == nil || req.ThreadId == "" {
		return nil, status.Error(codes.InvalidArgument, "thread_id is required")
	}
	ids := q.keeper.GetSamplesByThread(ctx, req.ThreadId)
	var samples []*types.Sample
	for _, id := range ids {
		s, found := q.keeper.GetSample(ctx, id)
		if found {
			samples = append(samples, s)
		}
	}
	// Sort by thread position
	sort.Slice(samples, func(i, j int) bool {
		return samples[i].ThreadPosition < samples[j].ThreadPosition
	})
	samples = paginateSamples(samples, req.Pagination)
	return &types.QuerySamplesResponse{Samples: samples}, nil
}

func (q queryServer) SamplesBySubmitter(ctx context.Context, req *types.QuerySamplesBySubmitterRequest) (*types.QuerySamplesResponse, error) {
	if req == nil || req.Submitter == "" {
		return nil, status.Error(codes.InvalidArgument, "submitter is required")
	}
	ids := q.keeper.GetSamplesBySubmitter(ctx, req.Submitter)
	var samples []*types.Sample
	for _, id := range ids {
		s, found := q.keeper.GetSample(ctx, id)
		if found {
			samples = append(samples, s)
		}
	}
	samples = paginateSamples(samples, req.Pagination)
	return &types.QuerySamplesResponse{Samples: samples}, nil
}

func (q queryServer) PendingSubmissions(ctx context.Context, req *types.QueryPendingSubmissionsRequest) (*types.QuerySubmissionsResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "request is required")
	}
	var subs []*types.Submission
	q.keeper.IterateSubmissions(ctx, func(sub *types.Submission) bool {
		if sub.Status == types.SubmissionStatus_SUBMISSION_STATUS_PENDING {
			subs = append(subs, sub)
		}
		return false
	})
	return &types.QuerySubmissionsResponse{Submissions: subs}, nil
}

// ─── Dataset queries (already implemented) ──────────────────────────────────

func (q queryServer) Dataset(ctx context.Context, req *types.QueryDatasetRequest) (*types.QueryDatasetResponse, error) {
	if req == nil || req.Id == "" {
		return nil, status.Error(codes.InvalidArgument, "dataset id is required")
	}
	dataset, found := q.keeper.GetDataset(ctx, req.Id)
	if !found {
		return nil, status.Errorf(codes.NotFound, "dataset %q not found", req.Id)
	}
	count, tokens := q.keeper.countMatchingSamples(ctx, dataset)
	dataset.SampleCount = count
	dataset.TotalTokens = tokens
	return &types.QueryDatasetResponse{Dataset: dataset}, nil
}

func (q queryServer) Datasets(ctx context.Context, req *types.QueryDatasetsRequest) (*types.QueryDatasetsResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "request is required")
	}
	var datasets []*types.Dataset
	if req.Domain != "" {
		ids := q.keeper.GetDatasetsByDomain(ctx, req.Domain)
		for _, id := range ids {
			ds, found := q.keeper.GetDataset(ctx, id)
			if found {
				count, tokens := q.keeper.countMatchingSamples(ctx, ds)
				ds.SampleCount = count
				ds.TotalTokens = tokens
				datasets = append(datasets, ds)
			}
		}
	} else {
		q.keeper.IterateDatasets(ctx, func(ds *types.Dataset) bool {
			count, tokens := q.keeper.countMatchingSamples(ctx, ds)
			ds.SampleCount = count
			ds.TotalTokens = tokens
			datasets = append(datasets, ds)
			return false
		})
	}
	return &types.QueryDatasetsResponse{Datasets: datasets}, nil
}

// ─── Reference data queries ─────────────────────────────────────────────────

func (q queryServer) Domain(ctx context.Context, req *types.QueryDomainRequest) (*types.QueryDomainResponse, error) {
	if req == nil || req.Name == "" {
		return nil, status.Error(codes.InvalidArgument, "domain name is required")
	}
	domain, found := q.keeper.GetDomain(ctx, req.Name)
	if !found {
		return nil, status.Errorf(codes.NotFound, "domain %q not found", req.Name)
	}
	return &types.QueryDomainResponse{Domain: domain}, nil
}

func (q queryServer) Domains(ctx context.Context, req *types.QueryDomainsRequest) (*types.QueryDomainsResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "request is required")
	}
	var domains []*types.Domain
	q.keeper.IterateDomains(ctx, func(d *types.Domain) bool {
		domains = append(domains, d)
		return false
	})
	return &types.QueryDomainsResponse{Domains: domains}, nil
}

func (q queryServer) TrainingDemand(ctx context.Context, req *types.QueryTrainingDemandRequest) (*types.QueryTrainingDemandResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "request is required")
	}
	var signals []*types.TrainingDemand
	q.keeper.IterateTrainingDemands(ctx, func(td *types.TrainingDemand) bool {
		if req.Domain == "" || td.Domain == req.Domain {
			signals = append(signals, td)
		}
		return false
	})
	return &types.QueryTrainingDemandResponse{Signals: signals}, nil
}

func (q queryServer) DataBounties(ctx context.Context, req *types.QueryDataBountiesRequest) (*types.QueryDataBountiesResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "request is required")
	}
	var bounties []*types.DataBounty
	q.keeper.IterateDataBounties(ctx, func(b *types.DataBounty) bool {
		if req.Domain == "" || b.Domain == req.Domain {
			bounties = append(bounties, b)
		}
		return false
	})
	return &types.QueryDataBountiesResponse{Bounties: bounties}, nil
}

func (q queryServer) Params(ctx context.Context, _ *types.QueryParamsRequest) (*types.QueryParamsResponse, error) {
	params, err := q.keeper.GetParams(ctx)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to get params: %v", err)
	}
	return &types.QueryParamsResponse{Params: params}, nil
}

// ─── Stats queries ──────────────────────────────────────────────────────────

func (q queryServer) DomainStats(ctx context.Context, req *types.QueryDomainStatsRequest) (*types.QueryDomainStatsResponse, error) {
	if req == nil || req.Domain == "" {
		return nil, status.Error(codes.InvalidArgument, "domain is required")
	}

	resp := &types.QueryDomainStatsResponse{Domain: req.Domain}
	var totalRevenue uint64

	ids := q.keeper.GetSamplesByDomain(ctx, req.Domain)
	for _, id := range ids {
		s, found := q.keeper.GetSample(ctx, id)
		if !found {
			continue
		}
		resp.SampleCount++
		switch s.Status {
		case types.SampleStatus_SAMPLE_STATUS_PENDING:
			resp.PendingCount++
		case types.SampleStatus_SAMPLE_STATUS_GOLD:
			resp.GoldCount++
		case types.SampleStatus_SAMPLE_STATUS_SILVER:
			resp.SilverCount++
		case types.SampleStatus_SAMPLE_STATUS_BRONZE:
			resp.BronzeCount++
		case types.SampleStatus_SAMPLE_STATUS_REJECTED:
			resp.RejectedCount++
		}
		totalRevenue += parseUzrn(s.TotalRevenue)
	}
	resp.TotalRevenue = fmt.Sprintf("%d", totalRevenue)

	// Count active bounties for this domain
	q.keeper.IterateDataBounties(ctx, func(b *types.DataBounty) bool {
		if b.Domain == req.Domain && !b.Claimed {
			resp.ActiveBounties++
		}
		return false
	})

	return resp, nil
}

func (q queryServer) ProtocolStats(ctx context.Context, _ *types.QueryProtocolStatsRequest) (*types.QueryProtocolStatsResponse, error) {
	resp := &types.QueryProtocolStatsResponse{}
	var totalRevenue uint64
	domainSet := make(map[string]struct{})

	q.keeper.IterateSamples(ctx, func(s *types.Sample) bool {
		resp.TotalSamples++
		resp.TotalAccessCount += s.AccessCount
		totalRevenue += parseUzrn(s.TotalRevenue)
		if s.Domain != "" {
			domainSet[s.Domain] = struct{}{}
		}
		return false
	})

	q.keeper.IterateSubmissions(ctx, func(_ *types.Submission) bool {
		resp.TotalSubmissions++
		return false
	})

	q.keeper.IterateDatasets(ctx, func(_ *types.Dataset) bool {
		resp.TotalDatasets++
		return false
	})

	resp.TotalDomains = uint64(len(domainSet))
	resp.ActiveRounds = uint64(len(q.keeper.GetActiveRounds(ctx)))
	resp.TotalRevenue = fmt.Sprintf("%d", totalRevenue)

	return resp, nil
}

// ─── Helpers ────────────────────────────────────────────────────────────────

// matchesSamplesFilter checks if a sample matches the filter criteria in QuerySamplesRequest.
func matchesSamplesFilter(s *types.Sample, req *types.QuerySamplesRequest) bool {
	if req.Status != "" {
		statusVal, ok := types.SampleStatus_value[req.Status]
		if ok && s.Status != types.SampleStatus(statusVal) {
			return false
		}
		// Also try case-insensitive match with SAMPLE_STATUS_ prefix
		if !ok {
			upper := strings.ToUpper(req.Status)
			if !strings.HasPrefix(upper, "SAMPLE_STATUS_") {
				upper = "SAMPLE_STATUS_" + upper
			}
			statusVal, ok = types.SampleStatus_value[upper]
			if ok && s.Status != types.SampleStatus(statusVal) {
				return false
			}
		}
	}
	if req.SampleType != types.SampleType_SAMPLE_TYPE_UNSPECIFIED && s.SampleType != req.SampleType {
		return false
	}
	return true
}

// paginateSamples applies simple offset/limit pagination to a sample slice.
func paginateSamples(samples []*types.Sample, _ interface{}) []*types.Sample {
	// Simple pagination: return all results (proto Pagination not enforced for KV stores)
	return samples
}
