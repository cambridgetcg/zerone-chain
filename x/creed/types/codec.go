package types

import (
	"github.com/cosmos/cosmos-sdk/codec"
	cdctypes "github.com/cosmos/cosmos-sdk/codec/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

func RegisterCodec(cdc *codec.LegacyAmino) {
	cdc.RegisterConcrete(&MsgAnchorPin{}, "zerone_creed/AnchorPin", nil)
	cdc.RegisterConcrete(&MsgUpdateParams{}, "zerone_creed/UpdateParams", nil)
	cdc.RegisterConcrete(&MsgUpdateCouncilMember{}, "zerone_creed/UpdateCouncilMember", nil)
}

func RegisterInterfaces(registry cdctypes.InterfaceRegistry) {
	registry.RegisterImplementations((*sdk.Msg)(nil),
		&MsgAnchorPin{},
		&MsgUpdateParams{},
		&MsgUpdateCouncilMember{},
	)
}
