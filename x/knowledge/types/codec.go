package types

import (
	"github.com/cosmos/cosmos-sdk/codec"
	cdctypes "github.com/cosmos/cosmos-sdk/codec/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

// RegisterCodec registers the knowledge module's types with the legacy amino codec.
func RegisterCodec(cdc *codec.LegacyAmino) {
	cdc.RegisterConcrete(&MsgSubmitClaim{}, "zerone_knowledge/SubmitClaim", nil)
	cdc.RegisterConcrete(&MsgSubmitCommitment{}, "zerone_knowledge/SubmitCommitment", nil)
	cdc.RegisterConcrete(&MsgSubmitReveal{}, "zerone_knowledge/SubmitReveal", nil)
	cdc.RegisterConcrete(&MsgChallengeFact{}, "zerone_knowledge/ChallengeFact", nil)
	cdc.RegisterConcrete(&MsgAddFact{}, "zerone_knowledge/AddFact", nil)
	cdc.RegisterConcrete(&MsgSubmitContradiction{}, "zerone_knowledge/SubmitContradiction", nil)
	cdc.RegisterConcrete(&MsgPatronizeFact{}, "zerone_knowledge/PatronizeFact", nil)
	cdc.RegisterConcrete(&MsgProposeDomain{}, "zerone_knowledge/ProposeDomain", nil)
	cdc.RegisterConcrete(&MsgEndorseDomainProposal{}, "zerone_knowledge/EndorseDomainProposal", nil)
	cdc.RegisterConcrete(&MsgChallengeDomainProposal{}, "zerone_knowledge/ChallengeDomainProposal", nil)
	cdc.RegisterConcrete(&MsgRegisterStratum{}, "zerone_knowledge/RegisterStratum", nil)
	cdc.RegisterConcrete(&MsgChallengeProvisionalFact{}, "zerone_knowledge/ChallengeProvisionalFact", nil)
	cdc.RegisterConcrete(&MsgUpdateParams{}, "zerone_knowledge/UpdateParams", nil)
	cdc.RegisterConcrete(&MsgUpdateExtendedParams{}, "zerone_knowledge/UpdateExtendedParams", nil)
	cdc.RegisterConcrete(&MsgProposeResearchFund{}, "zerone_knowledge/ProposeResearchFund", nil)
	cdc.RegisterConcrete(&MsgVoteResearchProposal{}, "zerone_knowledge/VoteResearchProposal", nil)
	cdc.RegisterConcrete(&MsgExecuteResearchProposal{}, "zerone_knowledge/ExecuteResearchProposal", nil)
	cdc.RegisterConcrete(&MsgAddCommonKnowledge{}, "zerone_knowledge/AddCommonKnowledge", nil)
	cdc.RegisterConcrete(&MsgRemoveCommonKnowledge{}, "zerone_knowledge/RemoveCommonKnowledge", nil)
	cdc.RegisterConcrete(&MsgRateFact{}, "zerone_knowledge/RateFact", nil)
}

// RegisterInterfaces registers the knowledge module's interface implementations.
func RegisterInterfaces(registry cdctypes.InterfaceRegistry) {
	registry.RegisterImplementations((*sdk.Msg)(nil),
		&MsgSubmitClaim{},
		&MsgSubmitCommitment{},
		&MsgSubmitReveal{},
		&MsgChallengeFact{},
		&MsgAddFact{},
		&MsgSubmitContradiction{},
		&MsgPatronizeFact{},
		&MsgProposeDomain{},
		&MsgEndorseDomainProposal{},
		&MsgChallengeDomainProposal{},
		&MsgRegisterStratum{},
		&MsgChallengeProvisionalFact{},
		&MsgUpdateParams{},
		&MsgUpdateExtendedParams{},
		&MsgProposeResearchFund{},
		&MsgVoteResearchProposal{},
		&MsgExecuteResearchProposal{},
		&MsgAddCommonKnowledge{},
		&MsgRemoveCommonKnowledge{},
		&MsgReportDemand{},
		&MsgRateFact{},
	)
}

var (
	Amino     = codec.NewLegacyAmino()
	ModuleCdc = codec.NewProtoCodec(cdctypes.NewInterfaceRegistry())
)
