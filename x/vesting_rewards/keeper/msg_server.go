package keeper

import (
	"context"

	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/zerone-chain/zerone/x/vesting_rewards/types"
)

type msgServer struct {
	types.UnimplementedMsgServer
	Keeper
}

// NewMsgServerImpl returns a msg server implementation.
func NewMsgServerImpl(keeper Keeper) types.MsgServer {
	return &msgServer{Keeper: keeper}
}

var _ types.MsgServer = msgServer{}

// CreateVesting creates a new truth-linked vesting schedule.
// Only callable by module authority (governance/knowledge module).
func (m msgServer) CreateVesting(
	goCtx context.Context,
	msg *types.MsgCreateVesting,
) (*types.MsgCreateVestingResponse, error) {
	ctx := sdk.UnwrapSDKContext(goCtx)

	if msg.Authority != m.Keeper.GetAuthority() {
		return nil, types.ErrUnauthorized
	}

	categoryStr := mapProtoCategoryToStr(msg.Category)

	schedule, err := m.Keeper.CreateVestingSchedule(
		ctx, msg.LinkedFactId, msg.LinkedFactId, msg.Beneficiary,
		msg.Amount, categoryStr, types.SourceVerification,
	)
	if err != nil {
		return nil, err
	}

	ctx.EventManager().EmitEvent(
		sdk.NewEvent(
			"vesting_created",
			sdk.NewAttribute("vesting_id", schedule.Id),
			sdk.NewAttribute("beneficiary", msg.Beneficiary),
			sdk.NewAttribute("amount", msg.Amount),
		),
	)

	return &types.MsgCreateVestingResponse{VestingId: schedule.Id}, nil
}

// ClaimVesting claims available vested rewards for the sender.
func (m msgServer) ClaimVesting(
	goCtx context.Context,
	msg *types.MsgClaimVesting,
) (*types.MsgClaimVestingResponse, error) {
	ctx := sdk.UnwrapSDKContext(goCtx)

	vestingId := ""
	if len(msg.VestingIds) == 1 {
		vestingId = msg.VestingIds[0]
	}

	claimed, err := m.Keeper.ClaimRewards(ctx, msg.Claimer, vestingId)
	if err != nil {
		return nil, err
	}

	ctx.EventManager().EmitEvent(
		sdk.NewEvent(
			"claim_vested_rewards",
			sdk.NewAttribute("claimer", msg.Claimer),
			sdk.NewAttribute("claimed_amount", claimed),
		),
	)

	scheduleCount := uint32(1)
	if vestingId == "" {
		schedules := m.Keeper.GetVestingSchedulesByRecipient(ctx, msg.Claimer)
		scheduleCount = uint32(len(schedules))
	}

	return &types.MsgClaimVestingResponse{
		TotalClaimed:    claimed,
		VestingsClaimed: scheduleCount,
	}, nil
}

// FalsifyVesting triggers clawback for a falsified vesting schedule.
func (m msgServer) FalsifyVesting(
	goCtx context.Context,
	msg *types.MsgFalsifyVesting,
) (*types.MsgFalsifyVestingResponse, error) {
	ctx := sdk.UnwrapSDKContext(goCtx)

	schedule, found := m.Keeper.GetVestingSchedule(ctx, msg.VestingId)
	if !found {
		return nil, types.ErrScheduleNotFound
	}

	_, err := m.Keeper.FalsifyClaim(ctx, schedule.ClaimId, msg.Challenger)
	if err != nil {
		return nil, err
	}

	ctx.EventManager().EmitEvent(
		sdk.NewEvent(
			"falsify_vesting",
			sdk.NewAttribute("vesting_id", msg.VestingId),
			sdk.NewAttribute("challenger", msg.Challenger),
			sdk.NewAttribute("reason", msg.Reason),
		),
	)

	return &types.MsgFalsifyVestingResponse{
		VestingPaused: true,
	}, nil
}

// PauseVesting pauses a vesting schedule due to active challenge.
// Only callable by module authority (governance/dispute module).
func (m msgServer) PauseVesting(
	goCtx context.Context,
	msg *types.MsgPauseVesting,
) (*types.MsgPauseVestingResponse, error) {
	ctx := sdk.UnwrapSDKContext(goCtx)

	if msg.Authority != m.Keeper.GetAuthority() {
		return nil, types.ErrUnauthorized
	}

	if err := m.Keeper.PauseVesting(ctx, msg.VestingId); err != nil {
		return nil, err
	}

	ctx.EventManager().EmitEvent(
		sdk.NewEvent(
			"vesting_paused",
			sdk.NewAttribute("vesting_id", msg.VestingId),
			sdk.NewAttribute("reason", msg.Reason),
		),
	)

	return &types.MsgPauseVestingResponse{}, nil
}

// ResumeVesting resumes a paused vesting schedule.
// Only callable by module authority (governance/dispute module).
func (m msgServer) ResumeVesting(
	goCtx context.Context,
	msg *types.MsgResumeVesting,
) (*types.MsgResumeVestingResponse, error) {
	ctx := sdk.UnwrapSDKContext(goCtx)

	if msg.Authority != m.Keeper.GetAuthority() {
		return nil, types.ErrUnauthorized
	}

	if err := m.Keeper.ResumeVesting(ctx, msg.VestingId); err != nil {
		return nil, err
	}

	ctx.EventManager().EmitEvent(
		sdk.NewEvent(
			"vesting_resumed",
			sdk.NewAttribute("vesting_id", msg.VestingId),
		),
	)

	return &types.MsgResumeVestingResponse{}, nil
}

// AccelerateVesting records evidence that accelerates a vesting schedule.
// Only callable by module authority (knowledge/dispute module).
func (m msgServer) AccelerateVesting(
	goCtx context.Context,
	msg *types.MsgAccelerateVesting,
) (*types.MsgAccelerateVestingResponse, error) {
	ctx := sdk.UnwrapSDKContext(goCtx)

	if msg.Authority != m.Keeper.GetAuthority() {
		return nil, types.ErrUnauthorized
	}

	var accelType string
	switch {
	case msg.AccelerationFactor <= 250000:
		accelType = "citation"
	case msg.AccelerationFactor <= 500000:
		accelType = "corroboration"
	case msg.AccelerationFactor <= 750000:
		accelType = "replication"
	default:
		accelType = "defense"
	}

	switch accelType {
	case "defense":
		if err := m.Keeper.RecordDefense(ctx, msg.VestingId); err != nil {
			return nil, err
		}
	case "replication":
		if err := m.Keeper.RecordReplication(ctx, msg.VestingId); err != nil {
			return nil, err
		}
	case "corroboration":
		if err := m.Keeper.RecordCorroboration(ctx, msg.VestingId); err != nil {
			return nil, err
		}
	case "citation":
		if err := m.Keeper.RecordCitation(ctx, msg.VestingId); err != nil {
			return nil, err
		}
	default:
		return nil, types.ErrInvalidAccelerationType
	}

	ctx.EventManager().EmitEvent(
		sdk.NewEvent(
			"vesting_accelerated",
			sdk.NewAttribute("vesting_id", msg.VestingId),
			sdk.NewAttribute("acceleration_type", accelType),
		),
	)

	return &types.MsgAccelerateVestingResponse{}, nil
}

// CompleteVesting marks a vesting schedule as completed.
func (m msgServer) CompleteVesting(
	goCtx context.Context,
	msg *types.MsgCompleteVesting,
) (*types.MsgCompleteVestingResponse, error) {
	ctx := sdk.UnwrapSDKContext(goCtx)

	schedule, found := m.Keeper.GetVestingSchedule(ctx, msg.VestingId)
	if !found {
		return nil, types.ErrScheduleNotFound
	}

	if msg.Authority != schedule.Recipient && msg.Authority != m.Keeper.GetAuthority() {
		return nil, types.ErrNotRecipientOrAuthority
	}

	if schedule.Status == string(types.VestingStatusCompleted) {
		return nil, types.ErrScheduleAlreadyCompleted
	}

	if schedule.Status == string(types.VestingStatusFalsified) || schedule.Status == string(types.VestingStatusAbandoned) {
		return nil, types.ErrScheduleNotActive
	}

	if err := m.Keeper.UpdateClaimableAmount(ctx, msg.VestingId); err != nil {
		return nil, err
	}

	schedule, _ = m.Keeper.GetVestingSchedule(ctx, msg.VestingId)

	schedule.Status = string(types.VestingStatusCompleted)
	schedule.UpdatedAt = uint64(ctx.BlockHeight())
	m.Keeper.SetVestingSchedule(ctx, schedule)

	ctx.EventManager().EmitEvent(
		sdk.NewEvent(
			"vesting_completed",
			sdk.NewAttribute("vesting_id", msg.VestingId),
			sdk.NewAttribute("released_amount", schedule.ReleasedAmount),
		),
	)

	return &types.MsgCompleteVestingResponse{
		RemainingAmount: schedule.ReleasedAmount,
	}, nil
}

// UpdateParams updates the module parameters.
// Only callable by module authority (governance).
func (m msgServer) UpdateParams(
	goCtx context.Context,
	msg *types.MsgUpdateParams,
) (*types.MsgUpdateParamsResponse, error) {
	ctx := sdk.UnwrapSDKContext(goCtx)

	if msg.Authority != m.Keeper.GetAuthority() {
		return nil, types.ErrUnauthorized
	}

	if msg.Params != nil {
		m.Keeper.SetParams(ctx, msg.Params)
	}

	return &types.MsgUpdateParamsResponse{}, nil
}

// mapProtoCategoryToStr maps the proto VestingCategory enum to the string-based VestingCategoryStr.
func mapProtoCategoryToStr(cat types.VestingCategory) types.VestingCategoryStr {
	switch cat {
	case types.VestingCategory_VESTING_CATEGORY_VERIFICATION_REWARD:
		return types.CategoryFormalProof
	case types.VestingCategory_VESTING_CATEGORY_BLOCK_REWARD:
		return types.CategoryAxiomatic
	case types.VestingCategory_VESTING_CATEGORY_BOUNTY_REWARD:
		return types.CategoryComputational
	case types.VestingCategory_VESTING_CATEGORY_DISPUTE_REWARD:
		return types.CategoryCryptographic
	case types.VestingCategory_VESTING_CATEGORY_RESEARCH_GRANT:
		return types.CategoryPeerReviewed
	case types.VestingCategory_VESTING_CATEGORY_BOOTSTRAP:
		return types.CategoryAxiomatic
	default:
		return types.CategoryPeerReviewed
	}
}
