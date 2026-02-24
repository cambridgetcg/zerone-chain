package keeper

import (
	"context"

	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/zerone-chain/zerone/x/gov/types"
)

type queryServer struct {
	Keeper
	types.UnimplementedQueryServer
}

// NewQueryServerImpl returns a types.QueryServer implementation.
func NewQueryServerImpl(k Keeper) types.QueryServer {
	return &queryServer{Keeper: k}
}

var _ types.QueryServer = (*queryServer)(nil)

// LIP returns a single LIP with its votes.
func (qs *queryServer) LIP(goCtx context.Context, req *types.QueryLIPRequest) (*types.QueryLIPResponse, error) {
	ctx := sdk.UnwrapSDKContext(goCtx)

	lip, found := qs.GetLIP(ctx, req.LipId)
	if !found {
		return nil, types.ErrLIPNotFound
	}

	votes := qs.GetVotesForLIP(ctx, req.LipId)
	return &types.QueryLIPResponse{Lip: lip, Votes: votes}, nil
}

// LIPs returns a filtered, paginated list of LIPs.
func (qs *queryServer) LIPs(goCtx context.Context, req *types.QueryLIPsRequest) (*types.QueryLIPsResponse, error) {
	ctx := sdk.UnwrapSDKContext(goCtx)

	var all []*types.LIP
	qs.IterateLIPs(ctx, func(lip *types.LIP) bool {
		if req.Status != "" && lip.Stage != req.Status {
			return false
		}
		if req.Category != "" && lip.Category != req.Category {
			return false
		}
		all = append(all, lip)
		return false
	})

	total := uint64(len(all))

	// Pagination.
	limit := req.Limit
	if limit == 0 || limit > 100 {
		limit = 100
	}
	start := req.Offset
	if start > total {
		start = total
	}
	end := start + limit
	if end > total {
		end = total
	}
	page := all[start:end]

	return &types.QueryLIPsResponse{Lips: page, Total: total}, nil
}

// Vote returns a single vote.
func (qs *queryServer) Vote(goCtx context.Context, req *types.QueryVoteRequest) (*types.QueryVoteResponse, error) {
	ctx := sdk.UnwrapSDKContext(goCtx)

	vote, found := qs.GetVote(ctx, req.LipId, req.Voter)
	if !found {
		return nil, types.ErrVoteNotFound
	}

	return &types.QueryVoteResponse{Vote: vote}, nil
}

// Votes returns all votes for a LIP.
func (qs *queryServer) Votes(goCtx context.Context, req *types.QueryVotesRequest) (*types.QueryVotesResponse, error) {
	ctx := sdk.UnwrapSDKContext(goCtx)

	votes := qs.GetVotesForLIP(ctx, req.LipId)
	return &types.QueryVotesResponse{Votes: votes}, nil
}

// TallyResult computes the current tally for a LIP.
func (qs *queryServer) TallyResult(goCtx context.Context, req *types.QueryTallyResultRequest) (*types.QueryTallyResultResponse, error) {
	ctx := sdk.UnwrapSDKContext(goCtx)

	lip, found := qs.GetLIP(ctx, req.LipId)
	if !found {
		return nil, types.ErrLIPNotFound
	}

	params := qs.GetParams(ctx)
	quorumMet, passed := qs.checkQuorumAndSupport(ctx, lip, params)

	return &types.QueryTallyResultResponse{
		YesStake:     lip.YesStake,
		NoStake:      lip.NoStake,
		AbstainStake: lip.AbstainStake,
		QuorumMet:    quorumMet,
		Passed:       passed,
	}, nil
}

// Params returns the current governance parameters.
func (qs *queryServer) Params(goCtx context.Context, _ *types.QueryParamsRequest) (*types.QueryParamsResponse, error) {
	ctx := sdk.UnwrapSDKContext(goCtx)
	params := qs.GetParams(ctx)
	return &types.QueryParamsResponse{Params: params}, nil
}

// --- Research Spend Query Handlers ---

// ResearchSpend returns a single research spend proposal by ID.
func (qs *queryServer) ResearchSpend(goCtx context.Context, req *types.QueryResearchSpendRequest) (*types.QueryResearchSpendResponse, error) {
	ctx := sdk.UnwrapSDKContext(goCtx)

	prop, found := qs.GetResearchSpendProposal(ctx, req.ProposalId)
	if !found {
		return nil, types.ErrResearchProposalNotFound
	}

	return &types.QueryResearchSpendResponse{Proposal: prop}, nil
}

// ResearchSpends returns a filtered, paginated list of research spend proposals.
func (qs *queryServer) ResearchSpends(goCtx context.Context, req *types.QueryResearchSpendsRequest) (*types.QueryResearchSpendsResponse, error) {
	ctx := sdk.UnwrapSDKContext(goCtx)

	all := qs.GetAllResearchSpendProposals(ctx)

	// Filter by stage if specified.
	var filtered []*types.ResearchSpendProposal
	for _, prop := range all {
		if req.Stage != "" && prop.Stage != req.Stage {
			continue
		}
		filtered = append(filtered, prop)
	}

	total := uint64(len(filtered))

	// Pagination.
	limit := req.Limit
	if limit == 0 || limit > 100 {
		limit = 100
	}
	start := req.Offset
	if start > total {
		start = total
	}
	end := start + limit
	if end > total {
		end = total
	}
	page := filtered[start:end]

	return &types.QueryResearchSpendsResponse{Proposals: page, Total: total}, nil
}

// ResearchVoters returns the current 2-of-2 voter configuration.
func (qs *queryServer) ResearchVoters(goCtx context.Context, _ *types.QueryResearchVotersRequest) (*types.QueryResearchVotersResponse, error) {
	ctx := sdk.UnwrapSDKContext(goCtx)
	voters := qs.GetResearchFundVoters(ctx)
	return &types.QueryResearchVotersResponse{Voters: voters}, nil
}

// ResearchFundGovernance returns the current governance state and a live snapshot of exit conditions.
func (qs *queryServer) ResearchFundGovernance(goCtx context.Context, _ *types.QueryResearchFundGovernanceRequest) (*types.QueryResearchFundGovernanceResponse, error) {
	ctx := sdk.UnwrapSDKContext(goCtx)
	state := qs.GetResearchFundGovernanceState(ctx)
	conditions, _ := qs.CheckPhaseExitConditions(ctx)

	return &types.QueryResearchFundGovernanceResponse{
		State:             state,
		CurrentConditions: conditions,
	}, nil
}
