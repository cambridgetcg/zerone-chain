package types

import (
	"github.com/cosmos/cosmos-sdk/codec"
	cdctypes "github.com/cosmos/cosmos-sdk/codec/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

func RegisterCodec(cdc *codec.LegacyAmino) {
	cdc.RegisterConcrete(&MsgCreateBountyOrder{}, "zerone_sponsorship/CreateBountyOrder", nil)
	cdc.RegisterConcrete(&MsgFulfillBounty{}, "zerone_sponsorship/FulfillBounty", nil)
	cdc.RegisterConcrete(&MsgCancelBountyOrder{}, "zerone_sponsorship/CancelBountyOrder", nil)
}

func RegisterInterfaces(registry cdctypes.InterfaceRegistry) {
	registry.RegisterImplementations((*sdk.Msg)(nil),
		&MsgCreateBountyOrder{},
		&MsgFulfillBounty{},
		&MsgCancelBountyOrder{},
	)
}
