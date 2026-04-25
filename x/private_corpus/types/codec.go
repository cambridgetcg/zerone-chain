package types

import (
	"github.com/cosmos/cosmos-sdk/codec"
	cdctypes "github.com/cosmos/cosmos-sdk/codec/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

func RegisterCodec(cdc *codec.LegacyAmino) {
	cdc.RegisterConcrete(&MsgRegisterVault{}, "zerone_private_corpus/RegisterVault", nil)
	cdc.RegisterConcrete(&MsgUpdateVault{}, "zerone_private_corpus/UpdateVault", nil)
	cdc.RegisterConcrete(&MsgPublishManifest{}, "zerone_private_corpus/PublishManifest", nil)
	cdc.RegisterConcrete(&MsgWithdrawManifest{}, "zerone_private_corpus/WithdrawManifest", nil)
	cdc.RegisterConcrete(&MsgRecordAccess{}, "zerone_private_corpus/RecordAccess", nil)
	cdc.RegisterConcrete(&MsgUpdateParams{}, "zerone_private_corpus/UpdateParams", nil)
}

func RegisterInterfaces(registry cdctypes.InterfaceRegistry) {
	registry.RegisterImplementations((*sdk.Msg)(nil),
		&MsgRegisterVault{},
		&MsgUpdateVault{},
		&MsgPublishManifest{},
		&MsgWithdrawManifest{},
		&MsgRecordAccess{},
		&MsgUpdateParams{},
	)
}
