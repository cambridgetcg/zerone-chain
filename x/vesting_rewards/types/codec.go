package types

import (
	"github.com/cosmos/cosmos-sdk/codec"
	cdctypes "github.com/cosmos/cosmos-sdk/codec/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

func RegisterCodec(cdc *codec.LegacyAmino) {
	cdc.RegisterConcrete(&MsgCreateVesting{}, "vesting_rewards/CreateVesting", nil)
	cdc.RegisterConcrete(&MsgClaimVesting{}, "vesting_rewards/ClaimVesting", nil)
	cdc.RegisterConcrete(&MsgFalsifyVesting{}, "vesting_rewards/FalsifyVesting", nil)
	cdc.RegisterConcrete(&MsgPauseVesting{}, "vesting_rewards/PauseVesting", nil)
	cdc.RegisterConcrete(&MsgResumeVesting{}, "vesting_rewards/ResumeVesting", nil)
	cdc.RegisterConcrete(&MsgAccelerateVesting{}, "vesting_rewards/AccelerateVesting", nil)
	cdc.RegisterConcrete(&MsgCompleteVesting{}, "vesting_rewards/CompleteVesting", nil)
	cdc.RegisterConcrete(&MsgUpdateParams{}, "vesting_rewards/UpdateParams", nil)
}

func RegisterInterfaces(registry cdctypes.InterfaceRegistry) {
	registry.RegisterImplementations((*sdk.Msg)(nil),
		&MsgCreateVesting{},
		&MsgClaimVesting{},
		&MsgPauseVesting{},
		&MsgResumeVesting{},
		&MsgAccelerateVesting{},
		&MsgFalsifyVesting{},
		&MsgCompleteVesting{},
		&MsgUpdateParams{},
	)
}

var (
	Amino     = codec.NewLegacyAmino()
	ModuleCdc = codec.NewProtoCodec(cdctypes.NewInterfaceRegistry())
)
