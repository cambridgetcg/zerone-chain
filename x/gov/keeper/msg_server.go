package keeper

import (
	"context"
	"fmt"
	"math/big"

	sdkmath "cosmossdk.io/math"

	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/zerone-chain/zerone/x/gov/types"
)

type msgServer struct {
	Keeper
	types.UnimplementedMsgServer
}

// NewMsgServerImpl returns a types.MsgServer implementation.
func NewMsgServerImpl(k Keeper) types.MsgServer {
	return &msgServer{Keeper: k}
}

var _ types.MsgServer = (*msgServer)(nil)

// SubmitLIP creates a new LIP in "draft" stage.
func (ms *msgServer) SubmitLIP(goCtx context.Context, msg *types.MsgSubmitLIP) (*types.MsgSubmitLIPResponse, error) {
	ctx := sdk.UnwrapSDKContext(goCtx)

	// Validate minimum stake.
	params := ms.GetParams(ctx)
	if params.MinLipStake != "" && params.MinLipStake != "0" {
		if types.CmpBigIntStrings(msg.InitialStake, params.MinLipStake) < 0 {
			return nil, types.ErrInsufficientStake
		}
	}

	// Validate category if provided.
	category := msg.Category
	if category == "" {
		category = types.CategoryText // default
	}

	// Escrow initial stake (transfer from proposer to module account).
	if ms.bankKeeper != nil {
		proposerAddr, err := sdk.AccAddressFromBech32(msg.Proposer)
		if err != nil {
			return nil, types.ErrInvalidAddress
		}
		stakeInt, ok := new(big.Int).SetString(msg.InitialStake, 10)
		if !ok || stakeInt.Sign() <= 0 {
			return nil, types.ErrInsufficientStake
		}
		coins := sdk.NewCoins(sdk.NewCoin("uzrn", sdkmath.NewIntFromBigInt(stakeInt)))
		if err := ms.bankKeeper.SendCoinsFromAccountToModule(ctx, proposerAddr, types.ModuleName, coins); err != nil {
			return nil, err
		}
	}

	// Allocate LIP number.
	num := ms.GetNextLIPNumber(ctx)
	lipID := fmt.Sprintf("LIP-%d", num)
	ms.SetNextLIPNumber(ctx, num+1)

	lip := &types.LIP{
		Id:             lipID,
		Title:          msg.Title,
		Description:    msg.Description,
		Category:       category,
		Proposer:       msg.Proposer,
		Stage:          types.StatusDraft,
		StakedAmount:   msg.InitialStake,
		YesStake:       "0",
		NoStake:        "0",
		AbstainStake:   "0",
		UniqueVoters:   0,
		CreatedAtBlock: uint64(ctx.BlockHeight()),
		ParamChanges:   msg.ParamChanges,
	}
	ms.SetLIP(ctx, lip)

	ctx.EventManager().EmitEvent(
		sdk.NewEvent("zerone.gov.lip_submitted",
			sdk.NewAttribute("lip_id", lipID),
			sdk.NewAttribute("proposer", msg.Proposer),
			sdk.NewAttribute("category", category),
			sdk.NewAttribute("initial_stake", msg.InitialStake),
		),
	)

	return &types.MsgSubmitLIPResponse{LipId: lipID}, nil
}

// StakeLIP adds stake to an existing LIP. If the category's required stake
// threshold is met, the LIP advances from "draft" to "review".
func (ms *msgServer) StakeLIP(goCtx context.Context, msg *types.MsgStakeLIP) (*types.MsgStakeLIPResponse, error) {
	ctx := sdk.UnwrapSDKContext(goCtx)

	lip, found := ms.GetLIP(ctx, msg.LipId)
	if !found {
		return nil, types.ErrLIPNotFound
	}

	if types.IsTerminal(lip.Stage) {
		return nil, types.ErrTerminalState
	}

	// Escrow stake.
	if ms.bankKeeper != nil {
		stakerAddr, err := sdk.AccAddressFromBech32(msg.Staker)
		if err != nil {
			return nil, types.ErrInvalidAddress
		}
		amt, ok := new(big.Int).SetString(msg.Amount, 10)
		if !ok || amt.Sign() <= 0 {
			return nil, types.ErrInsufficientStake
		}
		coins := sdk.NewCoins(sdk.NewCoin("uzrn", sdkmath.NewIntFromBigInt(amt)))
		if err := ms.bankKeeper.SendCoinsFromAccountToModule(ctx, stakerAddr, types.ModuleName, coins); err != nil {
			return nil, err
		}
	}

	// Add to staked amount.
	lip.StakedAmount = types.AddBigIntStrings(lip.StakedAmount, msg.Amount)

	// Check if stake threshold met for draft → review transition.
	if lip.Stage == types.StatusDraft {
		params := ms.GetParams(ctx)
		catCfg := types.GetCategoryConfig(params, lip.Category)
		if catCfg != nil && types.CmpBigIntStrings(lip.StakedAmount, catCfg.RequiredStakeBps) >= 0 {
			lip.Stage = types.StatusReview
			lip.ReviewStartedBlock = uint64(ctx.BlockHeight())
		}
	}

	ms.SetLIP(ctx, lip)

	ctx.EventManager().EmitEvent(
		sdk.NewEvent("zerone.gov.lip_staked",
			sdk.NewAttribute("lip_id", msg.LipId),
			sdk.NewAttribute("staker", msg.Staker),
			sdk.NewAttribute("amount", msg.Amount),
			sdk.NewAttribute("new_stage", lip.Stage),
			sdk.NewAttribute("total_staked", lip.StakedAmount),
		),
	)

	return &types.MsgStakeLIPResponse{}, nil
}

// AdvanceLIPStage advances a LIP to the next stage (proposer only).
// review → last_call: requires review_blocks elapsed.
// last_call → voting: sets voting_end_block.
func (ms *msgServer) AdvanceLIPStage(goCtx context.Context, msg *types.MsgAdvanceLIPStage) (*types.MsgAdvanceLIPStageResponse, error) {
	ctx := sdk.UnwrapSDKContext(goCtx)

	lip, found := ms.GetLIP(ctx, msg.LipId)
	if !found {
		return nil, types.ErrLIPNotFound
	}

	// Only proposer can advance.
	if lip.Proposer != msg.Authority {
		return nil, types.ErrNotProposer
	}

	if types.IsTerminal(lip.Stage) {
		return nil, types.ErrTerminalState
	}

	params := ms.GetParams(ctx)
	currentHeight := uint64(ctx.BlockHeight())

	switch lip.Stage {
	case types.StatusReview:
		// Check review period elapsed.
		catCfg := types.GetCategoryConfig(params, lip.Category)
		reviewBlocks := uint64(0)
		if catCfg != nil {
			reviewBlocks = catCfg.ReviewBlocks
		}
		if currentHeight < lip.ReviewStartedBlock+reviewBlocks {
			return nil, types.ErrInvalidStatus
		}
		lip.Stage = types.StatusLastCall
		lip.LastCallStartedBlock = currentHeight

	case types.StatusLastCall:
		lip.Stage = types.StatusVoting
		lip.VotingEndBlock = currentHeight + params.VotingPeriodBlocks

	default:
		return nil, types.ErrInvalidStatus
	}

	ms.SetLIP(ctx, lip)

	ctx.EventManager().EmitEvent(
		sdk.NewEvent("zerone.gov.lip_stage_advanced",
			sdk.NewAttribute("lip_id", msg.LipId),
			sdk.NewAttribute("authority", msg.Authority),
			sdk.NewAttribute("new_stage", lip.Stage),
		),
	)

	return &types.MsgAdvanceLIPStageResponse{NewStage: lip.Stage}, nil
}

// CastVote casts a stake-weighted vote on a LIP in voting stage.
func (ms *msgServer) CastVote(goCtx context.Context, msg *types.MsgCastVote) (*types.MsgCastVoteResponse, error) {
	ctx := sdk.UnwrapSDKContext(goCtx)

	lip, found := ms.GetLIP(ctx, msg.LipId)
	if !found {
		return nil, types.ErrLIPNotFound
	}

	if lip.Stage != types.StatusVoting {
		return nil, types.ErrNotInVotingStage
	}

	// Check voting period not expired.
	if lip.VotingEndBlock > 0 && uint64(ctx.BlockHeight()) > lip.VotingEndBlock {
		return nil, types.ErrVotingPeriodEnded
	}

	// Check dedupe.
	if ms.HasVoted(ctx, msg.LipId, msg.Voter) {
		return nil, types.ErrAlreadyVoted
	}

	// Get voter's total bonded stake as vote weight.
	weight := "0"
	if ms.stakingKeeper != nil {
		bonded, err := ms.stakingKeeper.GetDelegatorTotalBonded(ctx, msg.Voter)
		if err != nil {
			return nil, err
		}
		weight = bonded
	}

	// Validate minimum vote stake.
	params := ms.GetParams(ctx)
	if params.MinVoteStake != "" && params.MinVoteStake != "0" {
		if types.CmpBigIntStrings(weight, params.MinVoteStake) < 0 {
			return nil, types.ErrInsufficientStake
		}
	}

	// Record vote.
	vote := &types.Vote{
		LipId:  msg.LipId,
		Voter:  msg.Voter,
		Option: msg.Option,
		Weight: weight,
	}
	ms.SetVote(ctx, vote)

	// Track distinct governance participants for phase exit conditions.
	ms.RecordDistinctVoter(ctx, msg.Voter)

	// Accumulate tally.
	switch msg.Option {
	case types.VoteYes:
		lip.YesStake = types.AddBigIntStrings(lip.YesStake, weight)
	case types.VoteNo:
		lip.NoStake = types.AddBigIntStrings(lip.NoStake, weight)
	case types.VoteAbstain:
		lip.AbstainStake = types.AddBigIntStrings(lip.AbstainStake, weight)
	}
	lip.UniqueVoters++
	ms.SetLIP(ctx, lip)

	ctx.EventManager().EmitEvent(
		sdk.NewEvent("zerone.gov.vote_cast",
			sdk.NewAttribute("lip_id", msg.LipId),
			sdk.NewAttribute("voter", msg.Voter),
			sdk.NewAttribute("option", msg.Option),
			sdk.NewAttribute("weight", weight),
		),
	)

	return &types.MsgCastVoteResponse{EffectiveWeight: weight}, nil
}

// WithdrawLIP withdraws a LIP (proposer only, not terminal, not in voting).
func (ms *msgServer) WithdrawLIP(goCtx context.Context, msg *types.MsgWithdrawLIP) (*types.MsgWithdrawLIPResponse, error) {
	ctx := sdk.UnwrapSDKContext(goCtx)

	lip, found := ms.GetLIP(ctx, msg.LipId)
	if !found {
		return nil, types.ErrLIPNotFound
	}

	if lip.Proposer != msg.Proposer {
		return nil, types.ErrNotProposer
	}

	if types.IsTerminal(lip.Stage) {
		return nil, types.ErrTerminalState
	}

	if lip.Stage == types.StatusVoting {
		return nil, types.ErrInvalidStatus
	}

	lip.Stage = types.StatusWithdrawn
	ms.SetLIP(ctx, lip)

	ctx.EventManager().EmitEvent(
		sdk.NewEvent("zerone.gov.lip_withdrawn",
			sdk.NewAttribute("lip_id", msg.LipId),
			sdk.NewAttribute("proposer", msg.Proposer),
		),
	)

	return &types.MsgWithdrawLIPResponse{}, nil
}

// UpdateParams updates governance parameters (authority only).
func (ms *msgServer) UpdateParams(goCtx context.Context, msg *types.MsgUpdateParams) (*types.MsgUpdateParamsResponse, error) {
	ctx := sdk.UnwrapSDKContext(goCtx)

	if ms.GetAuthority() != msg.Authority {
		return nil, types.ErrUnauthorized
	}

	ms.SetParams(ctx, msg.Params)

	ctx.EventManager().EmitEvent(
		sdk.NewEvent("zerone.gov.params_updated",
			sdk.NewAttribute("authority", msg.Authority),
		),
	)

	return &types.MsgUpdateParamsResponse{}, nil
}

// AttachUpgradePlan attaches a software upgrade plan to an upgrade-category LIP.
func (ms *msgServer) AttachUpgradePlan(goCtx context.Context, msg *types.MsgAttachUpgradePlan) (*types.MsgAttachUpgradePlanResponse, error) {
	ctx := sdk.UnwrapSDKContext(goCtx)

	lip, found := ms.GetLIP(ctx, msg.LipId)
	if !found {
		return nil, types.ErrLIPNotFound
	}

	// Only the proposer can attach an upgrade plan.
	if lip.Proposer != msg.Proposer {
		return nil, fmt.Errorf("only the LIP proposer can attach an upgrade plan")
	}

	// Only upgrade-category LIPs can carry upgrade plans.
	if lip.Category != types.CategoryUpgrade {
		return nil, fmt.Errorf("only upgrade-category LIPs can carry upgrade plans")
	}

	// Cannot attach to terminal LIPs.
	if types.IsTerminal(lip.Stage) {
		return nil, fmt.Errorf("cannot attach upgrade plan to terminal LIP")
	}

	// Check if an upgrade plan already exists for this LIP.
	if _, exists := ms.GetUpgradePlan(ctx, msg.LipId); exists {
		return nil, fmt.Errorf("upgrade plan already attached to LIP %s", msg.LipId)
	}

	// Validate upgrade plan fields.
	if msg.Height <= 0 {
		return nil, fmt.Errorf("upgrade height must be positive")
	}

	plan := &types.UpgradePlan{
		Name:   msg.UpgradeName,
		Height: msg.Height,
		Info:   msg.Info,
	}
	ms.SetUpgradePlan(ctx, msg.LipId, plan)

	ctx.EventManager().EmitEvent(
		sdk.NewEvent("zerone.gov.upgrade_plan_attached",
			sdk.NewAttribute("lip_id", msg.LipId),
			sdk.NewAttribute("upgrade_name", msg.UpgradeName),
			sdk.NewAttribute("height", fmt.Sprintf("%d", msg.Height)),
		),
	)

	return &types.MsgAttachUpgradePlanResponse{}, nil
}

// --- Research Spend Message Handlers ---

// SubmitResearchSpend delegates to the keeper's SubmitResearchSpend method.
func (ms *msgServer) SubmitResearchSpend(goCtx context.Context, msg *types.MsgSubmitResearchSpend) (*types.MsgSubmitResearchSpendResponse, error) {
	ctx := sdk.UnwrapSDKContext(goCtx)
	return ms.Keeper.SubmitResearchSpend(ctx, msg)
}

// VoteResearchSpend delegates to the keeper's VoteResearchSpend method.
func (ms *msgServer) VoteResearchSpend(goCtx context.Context, msg *types.MsgVoteResearchSpend) (*types.MsgVoteResearchSpendResponse, error) {
	ctx := sdk.UnwrapSDKContext(goCtx)
	return ms.Keeper.VoteResearchSpend(ctx, msg)
}

// SetResearchVoters delegates to the keeper's SetResearchVoters method.
func (ms *msgServer) SetResearchVoters(goCtx context.Context, msg *types.MsgSetResearchVoters) (*types.MsgSetResearchVotersResponse, error) {
	ctx := sdk.UnwrapSDKContext(goCtx)
	return ms.Keeper.SetResearchVoters(ctx, msg)
}

