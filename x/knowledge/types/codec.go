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
	// Route B: training infrastructure
	cdc.RegisterConcrete(&MsgRegisterTrainingPipeline{}, "zerone_knowledge/RegisterTrainingPipeline", nil)
	cdc.RegisterConcrete(&MsgUpdateTrainingPipeline{}, "zerone_knowledge/UpdateTrainingPipeline", nil)
	cdc.RegisterConcrete(&MsgRegisterModelCard{}, "zerone_knowledge/RegisterModelCard", nil)
	cdc.RegisterConcrete(&MsgUpdateModelCard{}, "zerone_knowledge/UpdateModelCard", nil)
	cdc.RegisterConcrete(&MsgRetireModelCard{}, "zerone_knowledge/RetireModelCard", nil)
	// Route B Wave 3
	cdc.RegisterConcrete(&MsgAmendTokenizerSpec{}, "zerone_knowledge/AmendTokenizerSpec", nil)
	cdc.RegisterConcrete(&MsgAttributeContributions{}, "zerone_knowledge/AttributeContributions", nil)
	cdc.RegisterConcrete(&MsgAttestTraining{}, "zerone_knowledge/AttestTraining", nil)
	cdc.RegisterConcrete(&MsgCreateAugmentationBounty{}, "zerone_knowledge/CreateAugmentationBounty", nil)
	cdc.RegisterConcrete(&MsgSubmitAugmentation{}, "zerone_knowledge/SubmitAugmentation", nil)
	cdc.RegisterConcrete(&MsgAcceptAugmentation{}, "zerone_knowledge/AcceptAugmentation", nil)
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
		// Route B: training infrastructure
		&MsgRegisterTrainingPipeline{},
		&MsgUpdateTrainingPipeline{},
		&MsgRegisterModelCard{},
		&MsgUpdateModelCard{},
		&MsgRetireModelCard{},
		// Route B Wave 3
		&MsgAmendTokenizerSpec{},
		&MsgAttributeContributions{},
		&MsgAttestTraining{},
		&MsgCreateAugmentationBounty{},
		&MsgSubmitAugmentation{},
		&MsgAcceptAugmentation{},
	)
}

var (
	Amino     = codec.NewLegacyAmino()
	ModuleCdc = codec.NewProtoCodec(cdctypes.NewInterfaceRegistry())
)
