package types

import (
	"github.com/cosmos/cosmos-sdk/codec"
	cdctypes "github.com/cosmos/cosmos-sdk/codec/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

func RegisterCodec(cdc *codec.LegacyAmino) {
	cdc.RegisterConcrete(&MsgProposePartnership{}, "partnerships/ProposePartnership", nil)
	cdc.RegisterConcrete(&MsgAcceptPartnership{}, "partnerships/AcceptPartnership", nil)
	cdc.RegisterConcrete(&MsgProposeConsensusOp{}, "partnerships/ProposeConsensusOp", nil)
	cdc.RegisterConcrete(&MsgVoteConsensusOp{}, "partnerships/VoteConsensusOp", nil)
	cdc.RegisterConcrete(&MsgSafetyFreeze{}, "partnerships/SafetyFreeze", nil)
	cdc.RegisterConcrete(&MsgRaiseCoercionSignal{}, "partnerships/RaiseCoercionSignal", nil)
	cdc.RegisterConcrete(&MsgInitiateDissolution{}, "partnerships/InitiateDissolution", nil)
	cdc.RegisterConcrete(&MsgCreateSeedPartnership{}, "partnerships/CreateSeedPartnership", nil)
	cdc.RegisterConcrete(&MsgJoinFormationPool{}, "partnerships/JoinFormationPool", nil)
	cdc.RegisterConcrete(&MsgLeaveFormationPool{}, "partnerships/LeaveFormationPool", nil)
	cdc.RegisterConcrete(&MsgUpdateParams{}, "partnerships/UpdateParams", nil)
	cdc.RegisterConcrete(&MsgProposeMentorship{}, "partnerships/ProposeMentorship", nil)
	cdc.RegisterConcrete(&MsgAcceptMentorship{}, "partnerships/AcceptMentorship", nil)
	cdc.RegisterConcrete(&MsgGraduateMentee{}, "partnerships/GraduateMentee", nil)
	cdc.RegisterConcrete(&MsgEndMentorship{}, "partnerships/EndMentorship", nil)
	cdc.RegisterConcrete(&MsgAcceptFormationMatch{}, "partnerships/AcceptFormationMatch", nil)
	cdc.RegisterConcrete(&MsgDeclineFormationMatch{}, "partnerships/DeclineFormationMatch", nil)
}

func RegisterInterfaces(registry cdctypes.InterfaceRegistry) {
	registry.RegisterImplementations((*sdk.Msg)(nil),
		&MsgProposePartnership{},
		&MsgAcceptPartnership{},
		&MsgProposeConsensusOp{},
		&MsgVoteConsensusOp{},
		&MsgSafetyFreeze{},
		&MsgRaiseCoercionSignal{},
		&MsgInitiateDissolution{},
		&MsgCreateSeedPartnership{},
		&MsgJoinFormationPool{},
		&MsgLeaveFormationPool{},
		&MsgUpdateParams{},
		&MsgProposeMentorship{},
		&MsgAcceptMentorship{},
		&MsgGraduateMentee{},
		&MsgEndMentorship{},
		&MsgAcceptFormationMatch{},
		&MsgDeclineFormationMatch{},
	)
}
