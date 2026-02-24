package types

import (
	"github.com/cosmos/cosmos-sdk/codec"
	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

// RegisterCodec registers module types with the legacy amino codec.
func RegisterCodec(cdc *codec.LegacyAmino) {
	cdc.RegisterConcrete(&MsgSubmitLIP{}, "zerone_gov/MsgSubmitLIP", nil)
	cdc.RegisterConcrete(&MsgStakeLIP{}, "zerone_gov/MsgStakeLIP", nil)
	cdc.RegisterConcrete(&MsgAdvanceLIPStage{}, "zerone_gov/MsgAdvanceLIPStage", nil)
	cdc.RegisterConcrete(&MsgCastVote{}, "zerone_gov/MsgCastVote", nil)
	cdc.RegisterConcrete(&MsgWithdrawLIP{}, "zerone_gov/MsgWithdrawLIP", nil)
	cdc.RegisterConcrete(&MsgUpdateParams{}, "zerone_gov/MsgUpdateParams", nil)
	cdc.RegisterConcrete(&MsgSubmitResearchSpend{}, "zerone_gov/MsgSubmitResearchSpend", nil)
	cdc.RegisterConcrete(&MsgVoteResearchSpend{}, "zerone_gov/MsgVoteResearchSpend", nil)
	cdc.RegisterConcrete(&MsgSetResearchVoters{}, "zerone_gov/MsgSetResearchVoters", nil)
	cdc.RegisterConcrete(&MsgAttachUpgradePlan{}, "zerone_gov/MsgAttachUpgradePlan", nil)
	cdc.RegisterConcrete(&MsgNominateSeatElection{}, "zerone_gov/MsgNominateSeatElection", nil)
	cdc.RegisterConcrete(&MsgAcceptSeatNomination{}, "zerone_gov/MsgAcceptSeatNomination", nil)
	cdc.RegisterConcrete(&MsgVoteSeatElection{}, "zerone_gov/MsgVoteSeatElection", nil)
}

// RegisterInterfaces registers module types with the interface registry.
func RegisterInterfaces(registry codectypes.InterfaceRegistry) {
	registry.RegisterImplementations((*sdk.Msg)(nil),
		&MsgSubmitLIP{},
		&MsgStakeLIP{},
		&MsgAdvanceLIPStage{},
		&MsgCastVote{},
		&MsgWithdrawLIP{},
		&MsgUpdateParams{},
		&MsgSubmitResearchSpend{},
		&MsgVoteResearchSpend{},
		&MsgSetResearchVoters{},
		&MsgAttachUpgradePlan{},
		&MsgNominateSeatElection{},
		&MsgAcceptSeatNomination{},
		&MsgVoteSeatElection{},
	)
}
