package keeper

import (
	"context"
	"fmt"

	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/zerone-chain/zerone/x/alignment/types"
)

type msgServer struct {
	types.UnimplementedMsgServer
	Keeper
}

// NewMsgServerImpl returns a Msg service implementation.
func NewMsgServerImpl(keeper Keeper) types.MsgServer {
	return &msgServer{Keeper: keeper}
}

// UpdateParams updates the module parameters (authority-gated).
func (m msgServer) UpdateParams(ctx context.Context, msg *types.MsgUpdateParams) (*types.MsgUpdateParamsResponse, error) {
	if msg.Authority != m.Keeper.GetAuthority() {
		return nil, types.ErrUnauthorized
	}
	if msg.Params != nil {
		if err := msg.Params.Validate(); err != nil {
			return nil, err
		}
		m.Keeper.SetParams(ctx, msg.Params)
	}

	sdkCtx := sdk.UnwrapSDKContext(ctx)
	sdkCtx.EventManager().EmitEvent(
		sdk.NewEvent("zerone.alignment.params_updated",
			sdk.NewAttribute("authority", msg.Authority),
		),
	)

	return &types.MsgUpdateParamsResponse{}, nil
}

// Activate enables or disables the alignment module (authority-gated).
func (m msgServer) Activate(ctx context.Context, msg *types.MsgActivate) (*types.MsgActivateResponse, error) {
	if msg.Authority != m.Keeper.GetAuthority() {
		return nil, types.ErrUnauthorized
	}
	state := m.Keeper.GetState(ctx)
	state.Enabled = msg.Enabled
	m.Keeper.SetState(ctx, state)

	sdkCtx := sdk.UnwrapSDKContext(ctx)
	// Commitment 11 (trust is queryable) AND commitment 12 (the chain
	// pays for its own audit): activation is the chain turning on the
	// observational upstream of the alignment surface — the sensor
	// outputs and correction records that downstream synthesisers and
	// the audit-budget regulator both consume.
	sdkCtx.EventManager().EmitEvent(
		sdk.NewEvent("zerone.alignment.activated",
			sdk.NewAttribute("authority", msg.Authority),
			sdk.NewAttribute("enabled", fmt.Sprintf("%t", msg.Enabled)),
			sdk.NewAttribute("creed_commitment", "11,12"),
		),
	)

	return &types.MsgActivateResponse{}, nil
}
