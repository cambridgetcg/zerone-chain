package types

import (
	"github.com/cosmos/cosmos-sdk/codec"
	cdctypes "github.com/cosmos/cosmos-sdk/codec/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

func RegisterCodec(cdc *codec.LegacyAmino) {
	cdc.RegisterConcrete(&MsgProposeDomain{}, "zerone_ontology/ProposeDomain", nil)
	cdc.RegisterConcrete(&MsgVoteDomainProposal{}, "zerone_ontology/VoteDomainProposal", nil)
	cdc.RegisterConcrete(&MsgUpdateDomain{}, "zerone_ontology/UpdateDomain", nil)
	cdc.RegisterConcrete(&MsgUpdateParams{}, "zerone_ontology/UpdateParams", nil)
	cdc.RegisterConcrete(&MsgRegisterLogicZone{}, "zerone_ontology/RegisterLogicZone", nil)
	cdc.RegisterConcrete(&MsgAcknowledgeIncompleteness{}, "zerone_ontology/AcknowledgeIncompleteness", nil)
}

func RegisterInterfaces(registry cdctypes.InterfaceRegistry) {
	registry.RegisterImplementations((*sdk.Msg)(nil),
		&MsgProposeDomain{},
		&MsgVoteDomainProposal{},
		&MsgUpdateDomain{},
		&MsgUpdateParams{},
		&MsgRegisterLogicZone{},
		&MsgAcknowledgeIncompleteness{},
	)
}

var (
	Amino     = codec.NewLegacyAmino()
	ModuleCdc = codec.NewProtoCodec(cdctypes.NewInterfaceRegistry())
)
