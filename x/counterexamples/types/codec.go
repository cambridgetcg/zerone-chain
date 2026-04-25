package types

import (
	"github.com/cosmos/cosmos-sdk/codec"
	cdctypes "github.com/cosmos/cosmos-sdk/codec/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

func RegisterCodec(cdc *codec.LegacyAmino) {
	cdc.RegisterConcrete(&MsgProposeCounterexample{}, "zerone_counterexamples/Propose", nil)
	cdc.RegisterConcrete(&MsgValidate{}, "zerone_counterexamples/Validate", nil)
	cdc.RegisterConcrete(&MsgUpdateParams{}, "zerone_counterexamples/UpdateParams", nil)
}

func RegisterInterfaces(registry cdctypes.InterfaceRegistry) {
	registry.RegisterImplementations((*sdk.Msg)(nil),
		&MsgProposeCounterexample{},
		&MsgValidate{},
		&MsgUpdateParams{},
	)
}
