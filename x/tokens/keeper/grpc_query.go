package keeper

import (
	"context"
	"math/big"

	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/zerone-chain/zerone/x/tokens/types"
)

type queryServer struct {
	Keeper
	types.UnimplementedQueryServer
}

// NewQueryServerImpl returns an implementation of the QueryServer interface.
func NewQueryServerImpl(keeper Keeper) types.QueryServer {
	return &queryServer{Keeper: keeper}
}

var _ types.QueryServer = (*queryServer)(nil)

func (q *queryServer) Params(goCtx context.Context, req *types.QueryParamsRequest) (*types.QueryParamsResponse, error) {
	ctx := sdk.UnwrapSDKContext(goCtx)
	return &types.QueryParamsResponse{Params: q.GetParams(ctx)}, nil
}

func (q *queryServer) TokenConfig(goCtx context.Context, req *types.QueryTokenConfigRequest) (*types.QueryTokenConfigResponse, error) {
	ctx := sdk.UnwrapSDKContext(goCtx)
	token := q.GetToken(ctx, req.TokenId)
	if token == nil {
		return nil, types.ErrTokenNotFound
	}
	return &types.QueryTokenConfigResponse{Token: token}, nil
}

func (q *queryServer) Tokens(goCtx context.Context, req *types.QueryTokensRequest) (*types.QueryTokensResponse, error) {
	ctx := sdk.UnwrapSDKContext(goCtx)
	var tokens []*types.TokenDefinition
	q.IterateTokens(ctx, func(token *types.TokenDefinition) bool {
		tokens = append(tokens, token)
		return false
	})
	return &types.QueryTokensResponse{Tokens: tokens}, nil
}

func (q *queryServer) TokenBySymbol(goCtx context.Context, req *types.QueryTokenBySymbolRequest) (*types.QueryTokenBySymbolResponse, error) {
	ctx := sdk.UnwrapSDKContext(goCtx)
	token := q.GetTokenBySymbol(ctx, req.Symbol)
	if token == nil {
		return nil, types.ErrTokenNotFound
	}
	return &types.QueryTokenBySymbolResponse{Token: token}, nil
}

func (q *queryServer) Balance(goCtx context.Context, req *types.QueryBalanceRequest) (*types.QueryBalanceResponse, error) {
	ctx := sdk.UnwrapSDKContext(goCtx)
	bal := q.GetBalance(ctx, req.TokenId, req.Address)
	return &types.QueryBalanceResponse{Balance: bal.String()}, nil
}

func (q *queryServer) TotalSupply(goCtx context.Context, req *types.QueryTotalSupplyRequest) (*types.QueryTotalSupplyResponse, error) {
	ctx := sdk.UnwrapSDKContext(goCtx)
	token := q.GetToken(ctx, req.TokenId)
	if token == nil {
		return nil, types.ErrTokenNotFound
	}
	return &types.QueryTotalSupplyResponse{
		TotalSupply: token.TotalSupply,
		MaxSupply:   token.MaxSupply,
		Minted:      token.TotalSupply, // total minted = current supply (burns reduce it)
		Burned:      "0",               // not separately tracked
	}, nil
}

func (q *queryServer) Allowance(goCtx context.Context, req *types.QueryAllowanceRequest) (*types.QueryAllowanceResponse, error) {
	ctx := sdk.UnwrapSDKContext(goCtx)
	al := q.GetAllowance(ctx, req.TokenId, req.Owner, req.Spender)
	return &types.QueryAllowanceResponse{Allowance: al.String()}, nil
}

func (q *queryServer) DelegatedPower(goCtx context.Context, req *types.QueryDelegatedPowerRequest) (*types.QueryDelegatedPowerResponse, error) {
	ctx := sdk.UnwrapSDKContext(goCtx)

	delegatedTotal := q.GetDelegatorTotal(ctx, req.TokenId, req.Address)

	// Sum received delegations
	receivedTotal := new(big.Int)
	q.IterateDelegationsByToken(ctx, req.TokenId, func(delegator, delegate string, amount *big.Int) bool {
		if delegate == req.Address {
			receivedTotal.Add(receivedTotal, amount)
		}
		return false
	})

	undelegated := q.GetUndelegatedBalance(ctx, req.TokenId, req.Address)

	return &types.QueryDelegatedPowerResponse{
		DelegatedTotal:     delegatedTotal.String(),
		ReceivedTotal:      receivedTotal.String(),
		UndelegatedBalance: undelegated.String(),
	}, nil
}

func (q *queryServer) WrappedToken(goCtx context.Context, req *types.QueryWrappedTokenRequest) (*types.QueryWrappedTokenResponse, error) {
	ctx := sdk.UnwrapSDKContext(goCtx)
	wrappedDenom := q.GetWrappedDenom(ctx, req.TokenId)
	if wrappedDenom == "" {
		return nil, types.ErrWrapRecordNotFound
	}
	return &types.QueryWrappedTokenResponse{
		TokenId:      req.TokenId,
		WrappedDenom: wrappedDenom,
		TotalWrapped: "0",
	}, nil
}

func (q *queryServer) WrappedTokens(goCtx context.Context, req *types.QueryWrappedTokensRequest) (*types.QueryWrappedTokensResponse, error) {
	ctx := sdk.UnwrapSDKContext(goCtx)
	var records []*types.QueryWrappedTokenResponse
	q.IterateWrapRecords(ctx, func(tokenId, wrappedDenom string) bool {
		records = append(records, &types.QueryWrappedTokenResponse{
			TokenId:      tokenId,
			WrappedDenom: wrappedDenom,
			TotalWrapped: "0",
		})
		return false
	})
	return &types.QueryWrappedTokensResponse{Records: records}, nil
}

func (q *queryServer) EmissionPeriod(goCtx context.Context, req *types.QueryEmissionPeriodRequest) (*types.QueryEmissionPeriodResponse, error) {
	ctx := sdk.UnwrapSDKContext(goCtx)
	emission := q.GetEmissionPeriod(ctx, req.EmissionId)
	if emission == nil {
		return nil, types.ErrEmissionNotFound
	}
	return &types.QueryEmissionPeriodResponse{Emission: emission}, nil
}

func (q *queryServer) EmissionPeriods(goCtx context.Context, req *types.QueryEmissionPeriodsRequest) (*types.QueryEmissionPeriodsResponse, error) {
	ctx := sdk.UnwrapSDKContext(goCtx)
	var emissions []*types.EmissionPeriod
	q.IterateEmissionPeriods(ctx, func(emission *types.EmissionPeriod) bool {
		if req.ActiveOnly && !emission.Active {
			return false
		}
		emissions = append(emissions, emission)
		return false
	})
	return &types.QueryEmissionPeriodsResponse{Emissions: emissions}, nil
}
