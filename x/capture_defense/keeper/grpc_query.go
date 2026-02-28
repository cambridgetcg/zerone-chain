package keeper

import (
	"context"
	"fmt"

	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/zerone-chain/zerone/x/capture_defense/types"
)

type queryServer struct {
	types.UnimplementedQueryServer
	Keeper
}

// NewQueryServerImpl returns an implementation of the QueryServer interface.
func NewQueryServerImpl(keeper Keeper) types.QueryServer {
	return &queryServer{Keeper: keeper}
}

var _ types.QueryServer = &queryServer{}

// Params returns the module parameters.
func (q queryServer) Params(goCtx context.Context, req *types.QueryParamsRequest) (*types.QueryParamsResponse, error) {
	if req == nil {
		return nil, fmt.Errorf("request cannot be nil")
	}
	ctx := sdk.UnwrapSDKContext(goCtx)
	params := q.GetParams(ctx)
	return &types.QueryParamsResponse{Params: params}, nil
}

// Reputation returns the multi-layer reputation for a validator, optionally scoped to a domain.
func (q queryServer) Reputation(goCtx context.Context, req *types.QueryReputationRequest) (*types.QueryReputationResponse, error) {
	if req == nil || req.Validator == "" {
		return nil, fmt.Errorf("validator is required")
	}
	ctx := sdk.UnwrapSDKContext(goCtx)

	resp := &types.QueryReputationResponse{}

	// Global reputation
	global, found := q.GetGlobalReputation(ctx, req.Validator)
	if found {
		resp.Global = global
	}

	// Try to find stratum reputation for domain's stratum (use domain as stratum hint)
	if req.Domain != "" {
		// Domain reputation
		dr, found := q.GetDomainReputation(ctx, req.Domain, req.Validator)
		if found {
			resp.Domain = dr
		}
	}

	// Compute effective score
	params := q.GetParams(ctx)
	stratum := "" // no stratum in request; pass empty
	resp.EffectiveScore = q.GetEffectiveReputation(ctx, req.Validator, req.Domain, stratum, params)

	return resp, nil
}

// CaptureMetrics returns the capture analysis metrics for a domain.
func (q queryServer) CaptureMetrics(goCtx context.Context, req *types.QueryCaptureMetricsRequest) (*types.QueryCaptureMetricsResponse, error) {
	if req == nil || req.Domain == "" {
		return nil, fmt.Errorf("domain is required")
	}
	ctx := sdk.UnwrapSDKContext(goCtx)

	metrics, found := q.GetCaptureMetrics(ctx, req.Domain)
	if !found {
		return nil, fmt.Errorf("%w: %s", types.ErrMetricsNotFound, req.Domain)
	}

	return &types.QueryCaptureMetricsResponse{Metrics: metrics}, nil
}

// CrossStratumRequirements returns all configured cross-stratum requirements.
func (q queryServer) CrossStratumRequirements(goCtx context.Context, req *types.QueryCrossStratumRequirementsRequest) (*types.QueryCrossStratumRequirementsResponse, error) {
	if req == nil {
		return nil, fmt.Errorf("request cannot be nil")
	}
	ctx := sdk.UnwrapSDKContext(goCtx)
	reqs := q.GetAllCrossStratumRequirements(ctx)
	return &types.QueryCrossStratumRequirementsResponse{Requirements: reqs}, nil
}

// FlaggedDomains returns all domains with active capture flags.
func (q queryServer) FlaggedDomains(goCtx context.Context, _ *types.QueryFlaggedDomainsRequest) (*types.QueryFlaggedDomainsResponse, error) {
	ctx := sdk.UnwrapSDKContext(goCtx)
	var flagged []*types.CaptureMetrics
	q.IterateCaptureMetrics(ctx, func(m *types.CaptureMetrics) bool {
		if m.Flagged {
			flagged = append(flagged, m)
		}
		return false
	})
	return &types.QueryFlaggedDomainsResponse{Metrics: flagged}, nil
}
