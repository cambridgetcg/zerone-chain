package keeper

import (
	"context"
	"fmt"

	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/zerone-chain/zerone/x/capture_challenge/types"
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

// Challenge returns a specific challenge by ID.
func (q queryServer) Challenge(goCtx context.Context, req *types.QueryChallengeRequest) (*types.QueryChallengeResponse, error) {
	if req == nil || req.Id == "" {
		return nil, fmt.Errorf("challenge id is required")
	}
	ctx := sdk.UnwrapSDKContext(goCtx)
	challenge, found := q.GetChallenge(ctx, req.Id)
	if !found {
		return nil, fmt.Errorf("%w: %s", types.ErrChallengeNotFound, req.Id)
	}
	return &types.QueryChallengeResponse{Challenge: challenge}, nil
}

// BountyPool returns the bounty pool for a domain.
func (q queryServer) BountyPool(goCtx context.Context, req *types.QueryBountyPoolRequest) (*types.QueryBountyPoolResponse, error) {
	if req == nil || req.Domain == "" {
		return nil, fmt.Errorf("domain is required")
	}
	ctx := sdk.UnwrapSDKContext(goCtx)
	pool, found := q.GetBountyPool(ctx, req.Domain)
	if !found {
		return nil, fmt.Errorf("%w: %s", types.ErrBountyPoolNotFound, req.Domain)
	}
	return &types.QueryBountyPoolResponse{Pool: pool}, nil
}

// ChallengesByDomain returns all challenges for a given domain.
func (q queryServer) ChallengesByDomain(goCtx context.Context, req *types.QueryChallengesByDomainRequest) (*types.QueryChallengesByDomainResponse, error) {
	if req == nil || req.Domain == "" {
		return nil, fmt.Errorf("domain is required")
	}
	ctx := sdk.UnwrapSDKContext(goCtx)

	// Use domain index to collect challenge IDs, then fetch each
	ids := q.GetChallengesByDomain(ctx, req.Domain)
	var challenges []*types.CaptureChallenge
	for _, id := range ids {
		ch, found := q.GetChallenge(ctx, id)
		if found {
			challenges = append(challenges, ch)
		}
	}

	return &types.QueryChallengesByDomainResponse{Challenges: challenges}, nil
}

// ---------- Extended Query Server (non-proto) ----------

type queryExtServer struct {
	types.UnimplementedQueryExtServer
	Keeper
}

// NewQueryExtServerImpl returns an implementation of the QueryExtServer interface.
func NewQueryExtServerImpl(keeper Keeper) types.QueryExtServer {
	return &queryExtServer{Keeper: keeper}
}

var _ types.QueryExtServer = &queryExtServer{}

// ActiveChallenges returns all challenges with an active (non-terminal) status.
func (q queryExtServer) ActiveChallenges(goCtx context.Context, _ *types.QueryActiveChallengesRequest) (*types.QueryActiveChallengesResponse, error) {
	ctx := sdk.UnwrapSDKContext(goCtx)
	var active []*types.CaptureChallenge
	q.IterateChallenges(ctx, func(ch *types.CaptureChallenge) bool {
		switch ch.Status {
		case types.ChallengeStatus_CHALLENGE_STATUS_OPEN,
			types.ChallengeStatus_CHALLENGE_STATUS_EVIDENCE,
			types.ChallengeStatus_CHALLENGE_STATUS_UNDER_REVIEW:
			active = append(active, ch)
		}
		return false
	})
	return &types.QueryActiveChallengesResponse{Challenges: active}, nil
}
