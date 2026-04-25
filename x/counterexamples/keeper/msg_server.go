package keeper

import (
	"context"
	"fmt"

	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/zerone-chain/zerone/x/counterexamples/types"
)

type msgServer struct {
	types.UnimplementedMsgServer
	keeper Keeper
}

func NewMsgServerImpl(k Keeper) types.MsgServer { return &msgServer{keeper: k} }

var _ types.MsgServer = &msgServer{}

func (m *msgServer) ProposeCounterexample(ctx context.Context, msg *types.MsgProposeCounterexample) (*types.MsgProposeCounterexampleResponse, error) {
	if err := msg.ValidateBasic(); err != nil {
		return nil, err
	}
	params := m.keeper.GetParams(ctx)
	if !params.ProposalsEnabled {
		return nil, types.ErrProposalsDisabled
	}
	if uint32(len(msg.WrongClaim)) > params.MaxReasonBytes {
		return nil, fmt.Errorf("%w: wrong_claim %d > %d", types.ErrTextTooLong, len(msg.WrongClaim), params.MaxReasonBytes)
	}
	if uint32(len(msg.Reasoning)) > params.MaxReasonBytes {
		return nil, fmt.Errorf("%w: reasoning %d > %d", types.ErrTextTooLong, len(msg.Reasoning), params.MaxReasonBytes)
	}

	// Confirm the parent fact exists. If we don't have a fact keeper
	// wired (e.g. unit tests in isolation), accept the reference; if
	// we do, the chain refuses to anchor counterexamples to
	// non-existent facts.
	if m.keeper.factKeeper != nil {
		if !m.keeper.factKeeper.FactExists(ctx, msg.FactId) {
			return nil, fmt.Errorf("%w: %s", types.ErrFactNotFound, msg.FactId)
		}
	}

	id, err := m.keeper.NextCounterexampleID(ctx)
	if err != nil {
		return nil, err
	}
	height := CurrentBlock(ctx)
	c := &types.Counterexample{
		Id:                     id,
		FactId:                 msg.FactId,
		Author:                 msg.Author,
		WrongClaim:             msg.WrongClaim,
		Reasoning:              msg.Reasoning,
		ErrorType:              msg.ErrorType,
		ViolatedMethodologyIds: msg.ViolatedMethodologyIds,
		SubmittedAtBlock:       height,
		Status:                 types.CounterexampleStatus_COUNTEREXAMPLE_STATUS_PROPOSED,
		Bond:                   params.ProposalBond,
	}
	if err := m.keeper.SetCounterexample(ctx, c); err != nil {
		return nil, err
	}

	sdkCtx := sdk.UnwrapSDKContext(ctx)
	// Commitment 15 (counterexamples are part of the corpus): every
	// proposal is publicly announced so off-chain observers can
	// surface alignment-by-structure activity.
	sdkCtx.EventManager().EmitEvent(sdk.NewEvent(
		"zerone.counterexamples.proposed",
		sdk.NewAttribute("counterexample_id", c.Id),
		sdk.NewAttribute("fact_id", c.FactId),
		sdk.NewAttribute("author", c.Author),
		sdk.NewAttribute("error_type", c.ErrorType.String()),
		sdk.NewAttribute("creed_commitment", "15"),
	))
	return &types.MsgProposeCounterexampleResponse{CounterexampleId: c.Id}, nil
}

func (m *msgServer) Validate(ctx context.Context, msg *types.MsgValidate) (*types.MsgValidateResponse, error) {
	if err := msg.ValidateBasic(); err != nil {
		return nil, err
	}
	params := m.keeper.GetParams(ctx)
	if uint32(len(msg.Reason)) > params.MaxReasonBytes {
		return nil, fmt.Errorf("%w: reason %d > %d", types.ErrTextTooLong, len(msg.Reason), params.MaxReasonBytes)
	}

	c, ok := m.keeper.GetCounterexample(ctx, msg.CounterexampleId)
	if !ok {
		return nil, fmt.Errorf("%w: %s", types.ErrCounterexampleNotFound, msg.CounterexampleId)
	}
	if c.Status != types.CounterexampleStatus_COUNTEREXAMPLE_STATUS_PROPOSED {
		return nil, types.ErrAlreadyResolved
	}
	if m.keeper.HasValidatorVoted(ctx, c.Id, msg.Validator) {
		return nil, types.ErrAlreadyVoted
	}

	id, err := m.keeper.NextValidationID(ctx)
	if err != nil {
		return nil, err
	}
	height := CurrentBlock(ctx)
	v := &types.Validation{
		Id:                id,
		CounterexampleId:  c.Id,
		Validator:         msg.Validator,
		Affirm:            msg.Affirm,
		Reason:            msg.Reason,
		SubmittedAtBlock:  height,
	}
	if err := m.keeper.SetValidation(ctx, v); err != nil {
		return nil, err
	}
	if err := m.keeper.markValidatorVoted(ctx, c.Id, msg.Validator); err != nil {
		return nil, err
	}

	if msg.Affirm {
		c.Validations++
	} else {
		c.Rejections++
	}

	resolved := false
	totalVotes := uint32(c.Validations + c.Rejections)
	if totalVotes >= params.MinVotes {
		// Affirm threshold: affirms / total >= threshold_bps / 1_000_000
		// equivalently: affirms * 1_000_000 >= total * threshold_bps
		affirmScore := uint64(c.Validations) * 1_000_000
		needed := uint64(totalVotes) * params.AffirmThresholdBps
		if affirmScore >= needed {
			c.Status = types.CounterexampleStatus_COUNTEREXAMPLE_STATUS_VALIDATED
		} else {
			c.Status = types.CounterexampleStatus_COUNTEREXAMPLE_STATUS_REJECTED
		}
		c.ResolvedAtBlock = height
		resolved = true
	}

	if err := m.keeper.SetCounterexample(ctx, c); err != nil {
		return nil, err
	}

	sdkCtx := sdk.UnwrapSDKContext(ctx)
	// Validation events stay tagged with commitment 15 so off-chain
	// observers can compose a per-fact "alignment-structure score" from
	// just the event log.
	sdkCtx.EventManager().EmitEvent(sdk.NewEvent(
		"zerone.counterexamples.validation_recorded",
		sdk.NewAttribute("validation_id", fmt.Sprintf("%d", v.Id)),
		sdk.NewAttribute("counterexample_id", c.Id),
		sdk.NewAttribute("validator", v.Validator),
		sdk.NewAttribute("affirm", fmt.Sprintf("%t", v.Affirm)),
		sdk.NewAttribute("creed_commitment", "15"),
	))

	if resolved {
		sdkCtx.EventManager().EmitEvent(sdk.NewEvent(
			"zerone.counterexamples.resolved",
			sdk.NewAttribute("counterexample_id", c.Id),
			sdk.NewAttribute("status", c.Status.String()),
			sdk.NewAttribute("validations", fmt.Sprintf("%d", c.Validations)),
			sdk.NewAttribute("rejections", fmt.Sprintf("%d", c.Rejections)),
			sdk.NewAttribute("resolved_at_block", fmt.Sprintf("%d", height)),
			sdk.NewAttribute("creed_commitment", "15"),
		))
	}

	return &types.MsgValidateResponse{
		ValidationId: v.Id,
		Resolved:     resolved,
		Status:       c.Status,
	}, nil
}

func (m *msgServer) UpdateParams(ctx context.Context, msg *types.MsgUpdateParams) (*types.MsgUpdateParamsResponse, error) {
	if err := msg.ValidateBasic(); err != nil {
		return nil, err
	}
	if msg.Authority != m.keeper.Authority() {
		return nil, fmt.Errorf("%w: expected %s, got %s", types.ErrInvalidAuthority, m.keeper.Authority(), msg.Authority)
	}
	if msg.Params == nil {
		return nil, fmt.Errorf("params required")
	}
	if err := m.keeper.SetParams(ctx, *msg.Params); err != nil {
		return nil, err
	}
	return &types.MsgUpdateParamsResponse{}, nil
}
