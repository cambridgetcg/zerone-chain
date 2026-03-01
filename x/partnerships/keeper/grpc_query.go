package keeper

import (
	"context"
	"fmt"

	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/zerone-chain/zerone/x/partnerships/types"
)

type queryServer struct {
	Keeper
	types.UnimplementedQueryServer
}

// NewQueryServerImpl returns an implementation of the QueryServer interface.
func NewQueryServerImpl(keeper Keeper) types.QueryServer {
	return &queryServer{Keeper: keeper}
}

var _ types.QueryServer = queryServer{}

func (qs queryServer) Params(goCtx context.Context, req *types.QueryParamsRequest) (*types.QueryParamsResponse, error) {
	if req == nil {
		return nil, fmt.Errorf("request cannot be nil")
	}
	ctx := sdk.UnwrapSDKContext(goCtx)
	params := qs.GetParams(ctx)
	return &types.QueryParamsResponse{Params: params}, nil
}

func (qs queryServer) Partnership(goCtx context.Context, req *types.QueryPartnershipRequest) (*types.QueryPartnershipResponse, error) {
	if req == nil || req.Id == "" {
		return nil, fmt.Errorf("partnership id is required")
	}
	ctx := sdk.UnwrapSDKContext(goCtx)
	p, found := qs.GetPartnership(ctx, req.Id)
	if !found {
		return nil, fmt.Errorf("%w: %s", types.ErrPartnershipNotFound, req.Id)
	}
	return &types.QueryPartnershipResponse{Partnership: p}, nil
}

func (qs queryServer) PartnershipsByAddress(goCtx context.Context, req *types.QueryByAddressRequest) (*types.QueryByAddressResponse, error) {
	if req == nil || req.Address == "" {
		return nil, fmt.Errorf("address is required")
	}
	ctx := sdk.UnwrapSDKContext(goCtx)
	partnerships := qs.GetPartnershipsByParticipant(ctx, req.Address)
	return &types.QueryByAddressResponse{Partnerships: partnerships}, nil
}

func (qs queryServer) PendingOps(goCtx context.Context, req *types.QueryPendingOpsRequest) (*types.QueryPendingOpsResponse, error) {
	if req == nil || req.PartnershipId == "" {
		return nil, fmt.Errorf("partnership_id is required")
	}
	ctx := sdk.UnwrapSDKContext(goCtx)
	ops := qs.GetPendingOpsForPartnership(ctx, req.PartnershipId)
	return &types.QueryPendingOpsResponse{Operations: ops}, nil
}

func (qs queryServer) FormationPool(goCtx context.Context, req *types.QueryFormationPoolRequest) (*types.QueryFormationPoolResponse, error) {
	if req == nil {
		return nil, fmt.Errorf("request cannot be nil")
	}
	ctx := sdk.UnwrapSDKContext(goCtx)
	entries := qs.GetAllPoolEntries(ctx)
	return &types.QueryFormationPoolResponse{Entries: entries}, nil
}

func (qs queryServer) Mentorship(goCtx context.Context, req *types.QueryMentorshipRequest) (*types.QueryMentorshipResponse, error) {
	if req == nil || req.Id == "" {
		return nil, fmt.Errorf("mentorship id is required")
	}
	ctx := sdk.UnwrapSDKContext(goCtx)
	m, found := qs.GetMentorship(ctx, req.Id)
	if !found {
		return nil, fmt.Errorf("%w: %s", types.ErrMentorshipNotFound, req.Id)
	}
	return &types.QueryMentorshipResponse{Mentorship: m}, nil
}

func (qs queryServer) MentorshipsByAddress(goCtx context.Context, req *types.QueryMentorshipsByAddressRequest) (*types.QueryMentorshipsByAddressResponse, error) {
	if req == nil || req.Address == "" {
		return nil, fmt.Errorf("address is required")
	}
	ctx := sdk.UnwrapSDKContext(goCtx)
	byMentor := qs.GetMentorshipsByMentor(ctx, req.Address)
	byMentee := qs.GetMentorshipsByMentee(ctx, req.Address)

	seen := make(map[string]bool)
	var all []*types.Mentorship
	for _, m := range byMentor {
		if !seen[m.Id] {
			seen[m.Id] = true
			all = append(all, m)
		}
	}
	for _, m := range byMentee {
		if !seen[m.Id] {
			seen[m.Id] = true
			all = append(all, m)
		}
	}
	return &types.QueryMentorshipsByAddressResponse{Mentorships: all}, nil
}

func (qs queryServer) FormationMatches(goCtx context.Context, req *types.QueryFormationMatchesRequest) (*types.QueryFormationMatchesResponse, error) {
	if req == nil {
		return nil, fmt.Errorf("request cannot be nil")
	}
	ctx := sdk.UnwrapSDKContext(goCtx)
	matches := qs.GetAllFormationMatches(ctx)
	return &types.QueryFormationMatchesResponse{Matches: matches}, nil
}
