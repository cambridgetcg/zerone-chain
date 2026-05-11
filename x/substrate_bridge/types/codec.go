package types

import (
	"github.com/cosmos/cosmos-sdk/codec"
	cdctypes "github.com/cosmos/cosmos-sdk/codec/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

func RegisterInterfaces(reg cdctypes.InterfaceRegistry) {
	reg.RegisterImplementations((*sdk.Msg)(nil),
		&MsgRegisterAdapter{},
		&MsgSuspendAdapter{},
		&MsgTombstoneAdapter{},
		&MsgSubmitExternalAttestation{},
	)
}

func RegisterLegacyAminoCodec(cdc *codec.LegacyAmino) {
	cdc.RegisterConcrete(&MsgRegisterAdapter{}, "zerone_substrate_bridge/RegisterAdapter", nil)
	cdc.RegisterConcrete(&MsgSuspendAdapter{}, "zerone_substrate_bridge/SuspendAdapter", nil)
	cdc.RegisterConcrete(&MsgTombstoneAdapter{}, "zerone_substrate_bridge/TombstoneAdapter", nil)
	cdc.RegisterConcrete(&MsgSubmitExternalAttestation{}, "zerone_substrate_bridge/SubmitExternalAttestation", nil)
}
