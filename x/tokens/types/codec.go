package types

import (
	"github.com/cosmos/cosmos-sdk/codec"
	cdctypes "github.com/cosmos/cosmos-sdk/codec/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

func RegisterCodec(cdc *codec.LegacyAmino) {
	cdc.RegisterConcrete(&MsgCreateToken{}, "zerone_tokens/CreateToken", nil)
	cdc.RegisterConcrete(&MsgMintToken{}, "zerone_tokens/MintToken", nil)
	cdc.RegisterConcrete(&MsgBurnToken{}, "zerone_tokens/BurnToken", nil)
	cdc.RegisterConcrete(&MsgTransferToken{}, "zerone_tokens/TransferToken", nil)
	cdc.RegisterConcrete(&MsgApproveToken{}, "zerone_tokens/ApproveToken", nil)
	cdc.RegisterConcrete(&MsgTransferFrom{}, "zerone_tokens/TransferFrom", nil)
	cdc.RegisterConcrete(&MsgPauseToken{}, "zerone_tokens/PauseToken", nil)
	cdc.RegisterConcrete(&MsgUnpauseToken{}, "zerone_tokens/UnpauseToken", nil)
	cdc.RegisterConcrete(&MsgDelegatePower{}, "zerone_tokens/DelegatePower", nil)
	cdc.RegisterConcrete(&MsgUndelegatePower{}, "zerone_tokens/UndelegatePower", nil)
	cdc.RegisterConcrete(&MsgWrapToken{}, "zerone_tokens/WrapToken", nil)
	cdc.RegisterConcrete(&MsgUnwrapToken{}, "zerone_tokens/UnwrapToken", nil)
	cdc.RegisterConcrete(&MsgCreateEmissionPeriod{}, "zerone_tokens/CreateEmissionPeriod", nil)
	cdc.RegisterConcrete(&MsgCancelEmissionPeriod{}, "zerone_tokens/CancelEmissionPeriod", nil)
	cdc.RegisterConcrete(&MsgUpdateParams{}, "zerone_tokens/UpdateParams", nil)
}

func RegisterInterfaces(registry cdctypes.InterfaceRegistry) {
	registry.RegisterImplementations((*sdk.Msg)(nil),
		&MsgCreateToken{},
		&MsgMintToken{},
		&MsgBurnToken{},
		&MsgTransferToken{},
		&MsgApproveToken{},
		&MsgTransferFrom{},
		&MsgPauseToken{},
		&MsgUnpauseToken{},
		&MsgDelegatePower{},
		&MsgUndelegatePower{},
		&MsgWrapToken{},
		&MsgUnwrapToken{},
		&MsgCreateEmissionPeriod{},
		&MsgCancelEmissionPeriod{},
		&MsgUpdateParams{},
	)
}

var (
	Amino     = codec.NewLegacyAmino()
	ModuleCdc = codec.NewProtoCodec(cdctypes.NewInterfaceRegistry())
)
