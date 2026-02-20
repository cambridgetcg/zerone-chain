package keeper

import (
	"context"

	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/zerone-chain/zerone/x/staking/types"
)

type queryServer struct {
	Keeper
	types.UnimplementedQueryServer
}

// NewQueryServerImpl returns a QueryServer implementation.
func NewQueryServerImpl(k Keeper) types.QueryServer {
	return &queryServer{Keeper: k}
}

var _ types.QueryServer = &queryServer{}

func (qs *queryServer) Validator(goCtx context.Context, req *types.QueryValidatorRequest) (*types.QueryValidatorResponse, error) {
	ctx := sdk.UnwrapSDKContext(goCtx)
	val, found := qs.GetValidator(ctx, req.Address)
	if !found {
		return nil, types.ErrValidatorNotFound
	}
	return &types.QueryValidatorResponse{Validator: val}, nil
}

func (qs *queryServer) Validators(goCtx context.Context, req *types.QueryValidatorsRequest) (*types.QueryValidatorsResponse, error) {
	ctx := sdk.UnwrapSDKContext(goCtx)

	limit := req.Limit
	if limit == 0 || limit > 100 {
		limit = 100
	}

	var all []*types.Validator
	qs.IterateValidators(ctx, func(val *types.Validator) bool {
		if req.ActiveOnly && !val.IsActive {
			return false
		}
		if req.Tier >= 0 && val.Tier != types.ValidatorTier(req.Tier) {
			return false
		}
		all = append(all, val)
		return false
	})

	total := uint64(len(all))

	// Apply offset and limit.
	start := req.Offset
	if start > total {
		start = total
	}
	end := start + limit
	if end > total {
		end = total
	}
	page := all[start:end]

	return &types.QueryValidatorsResponse{
		Validators: page,
		Total:      total,
	}, nil
}

func (qs *queryServer) Delegation(goCtx context.Context, req *types.QueryDelegationRequest) (*types.QueryDelegationResponse, error) {
	ctx := sdk.UnwrapSDKContext(goCtx)
	del, found := qs.GetDelegation(ctx, req.Delegator, req.Validator)
	if !found {
		return nil, types.ErrDelegationNotFound
	}
	return &types.QueryDelegationResponse{Delegation: del}, nil
}

func (qs *queryServer) DelegatorDelegations(goCtx context.Context, req *types.QueryDelegatorDelegationsRequest) (*types.QueryDelegatorDelegationsResponse, error) {
	ctx := sdk.UnwrapSDKContext(goCtx)

	var delegations []*types.Delegation
	qs.IterateDelegations(ctx, func(del *types.Delegation) bool {
		if del.DelegatorAddress == req.Delegator {
			delegations = append(delegations, del)
		}
		return false
	})
	return &types.QueryDelegatorDelegationsResponse{Delegations: delegations}, nil
}

func (qs *queryServer) ValidatorDelegations(goCtx context.Context, req *types.QueryValidatorDelegationsRequest) (*types.QueryValidatorDelegationsResponse, error) {
	ctx := sdk.UnwrapSDKContext(goCtx)
	delegations := qs.GetDelegationsForValidator(ctx, req.Validator)
	return &types.QueryValidatorDelegationsResponse{Delegations: delegations}, nil
}

func (qs *queryServer) Params(goCtx context.Context, _ *types.QueryParamsRequest) (*types.QueryParamsResponse, error) {
	ctx := sdk.UnwrapSDKContext(goCtx)
	params := qs.GetParams(ctx)
	configs := qs.GetAllTierConfigs(ctx)
	return &types.QueryParamsResponse{
		Params:      params,
		TierConfigs: configs,
	}, nil
}

func (qs *queryServer) TierConfig(goCtx context.Context, req *types.QueryTierConfigRequest) (*types.QueryTierConfigResponse, error) {
	ctx := sdk.UnwrapSDKContext(goCtx)
	tc, found := qs.GetTierConfig(ctx, types.ValidatorTier(req.Tier))
	if !found {
		return nil, types.ErrValidatorNotFound.Wrap("tier config not found")
	}
	return &types.QueryTierConfigResponse{TierConfig: tc}, nil
}
