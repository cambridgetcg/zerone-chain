package types

import (
	"github.com/cosmos/cosmos-sdk/codec"
	cdctypes "github.com/cosmos/cosmos-sdk/codec/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/msgservice"
)

// RegisterLegacyAminoCodec registers concrete types on the LegacyAmino codec.
func RegisterLegacyAminoCodec(cdc *codec.LegacyAmino) {
	cdc.RegisterConcrete(&MsgSubmitContribution{}, "zerone/contribution/MsgSubmitContribution", nil)
}

// RegisterInterfaces registers the module's interface types.
func RegisterInterfaces(reg cdctypes.InterfaceRegistry) {
	reg.RegisterImplementations(
		(*sdk.Msg)(nil),
		&MsgSubmitContribution{},
	)
	msgservice.RegisterMsgServiceDesc(reg, &Msg_ServiceDesc)
}
