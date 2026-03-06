package types

import (
	"github.com/cosmos/cosmos-sdk/codec"
	cdctypes "github.com/cosmos/cosmos-sdk/codec/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

// RegisterCodec registers the knowledge module's types with the legacy amino codec.
func RegisterCodec(cdc *codec.LegacyAmino) {
	cdc.RegisterConcrete(&MsgSubmitData{}, "zerone_knowledge/SubmitData", nil)
	cdc.RegisterConcrete(&MsgSubmitThread{}, "zerone_knowledge/SubmitThread", nil)
	cdc.RegisterConcrete(&MsgSubmitCommitment{}, "zerone_knowledge/SubmitCommitment", nil)
	cdc.RegisterConcrete(&MsgSubmitReveal{}, "zerone_knowledge/SubmitReveal", nil)
	cdc.RegisterConcrete(&MsgContestSample{}, "zerone_knowledge/ContestSample", nil)
	cdc.RegisterConcrete(&MsgSponsorSample{}, "zerone_knowledge/SponsorSample", nil)
	cdc.RegisterConcrete(&MsgProposeDomain{}, "zerone_knowledge/ProposeDomain", nil)
	cdc.RegisterConcrete(&MsgEndorseDomainProposal{}, "zerone_knowledge/EndorseDomainProposal", nil)
	cdc.RegisterConcrete(&MsgCreateDataset{}, "zerone_knowledge/CreateDataset", nil)
	cdc.RegisterConcrete(&MsgAccessDataset{}, "zerone_knowledge/AccessDataset", nil)
	cdc.RegisterConcrete(&MsgAccessSample{}, "zerone_knowledge/AccessSample", nil)
	cdc.RegisterConcrete(&MsgReportDemand{}, "zerone_knowledge/ReportDemand", nil)
	cdc.RegisterConcrete(&MsgFundBounty{}, "zerone_knowledge/FundBounty", nil)
	cdc.RegisterConcrete(&MsgRateSample{}, "zerone_knowledge/RateSample", nil)
	cdc.RegisterConcrete(&MsgUpdateParams{}, "zerone_knowledge/UpdateParams", nil)
	cdc.RegisterConcrete(&MsgAddScrapedSource{}, "zerone_knowledge/AddScrapedSource", nil)
	cdc.RegisterConcrete(&MsgRemoveScrapedSource{}, "zerone_knowledge/RemoveScrapedSource", nil)
	cdc.RegisterConcrete(&MsgProposeResearchFund{}, "zerone_knowledge/ProposeResearchFund", nil)
	cdc.RegisterConcrete(&MsgVoteResearchProposal{}, "zerone_knowledge/VoteResearchProposal", nil)
	cdc.RegisterConcrete(&MsgExecuteResearchProposal{}, "zerone_knowledge/ExecuteResearchProposal", nil)
	cdc.RegisterConcrete(&MsgAddSample{}, "zerone_knowledge/AddSample", nil)
	cdc.RegisterConcrete(&MsgRevokeConsent{}, "zerone_knowledge/RevokeConsent", nil)
	cdc.RegisterConcrete(&MsgUpgradeConsent{}, "zerone_knowledge/UpgradeConsent", nil)
	cdc.RegisterConcrete(&MsgAttestStorage{}, "zerone_knowledge/AttestStorage", nil)
	cdc.RegisterConcrete(&MsgRegisterEnclave{}, "zerone_knowledge/RegisterEnclave", nil)
	cdc.RegisterConcrete(&MsgVerifyAttestation{}, "zerone_knowledge/VerifyAttestation", nil)
	cdc.RegisterConcrete(&MsgSuspendEnclave{}, "zerone_knowledge/SuspendEnclave", nil)
	cdc.RegisterConcrete(&MsgRevokeEnclave{}, "zerone_knowledge/RevokeEnclave", nil)
	// API revenue (R44-1)
	cdc.RegisterConcrete(&MsgCreateAPIKey{}, "zerone_knowledge/CreateAPIKey", nil)
	cdc.RegisterConcrete(&MsgRevokeAPIKey{}, "zerone_knowledge/RevokeAPIKey", nil)
	cdc.RegisterConcrete(&MsgDepositAPICredits{}, "zerone_knowledge/DepositAPICredits", nil)
	cdc.RegisterConcrete(&MsgWithdrawAPICredits{}, "zerone_knowledge/WithdrawAPICredits", nil)
	cdc.RegisterConcrete(&MsgRecordAPIUsage{}, "zerone_knowledge/RecordAPIUsage", nil)
}

// RegisterInterfaces registers the knowledge module's interface implementations.
func RegisterInterfaces(registry cdctypes.InterfaceRegistry) {
	registry.RegisterImplementations((*sdk.Msg)(nil),
		&MsgSubmitData{},
		&MsgSubmitThread{},
		&MsgSubmitCommitment{},
		&MsgSubmitReveal{},
		&MsgContestSample{},
		&MsgSponsorSample{},
		&MsgProposeDomain{},
		&MsgEndorseDomainProposal{},
		&MsgCreateDataset{},
		&MsgAccessDataset{},
		&MsgAccessSample{},
		&MsgReportDemand{},
		&MsgFundBounty{},
		&MsgRateSample{},
		&MsgUpdateParams{},
		&MsgAddScrapedSource{},
		&MsgRemoveScrapedSource{},
		&MsgProposeResearchFund{},
		&MsgVoteResearchProposal{},
		&MsgExecuteResearchProposal{},
		&MsgAddSample{},
		&MsgRevokeConsent{},
		&MsgUpgradeConsent{},
		// MsgAttestStorage: proto-gen required for full registration.
		// Proto definition in tx.proto; run `make proto-gen` when BSR is available.
		// TEE attestation messages (T6-1).
		// Proto-gen will supersede tee_msgs.go definitions.
		&MsgRegisterEnclave{},
		&MsgVerifyAttestation{},
		&MsgSuspendEnclave{},
		&MsgRevokeEnclave{},
		// API revenue (R44-1)
		&MsgCreateAPIKey{},
		&MsgRevokeAPIKey{},
		&MsgDepositAPICredits{},
		&MsgWithdrawAPICredits{},
		&MsgRecordAPIUsage{},
	)
}

var (
	Amino     = codec.NewLegacyAmino()
	ModuleCdc = codec.NewProtoCodec(cdctypes.NewInterfaceRegistry())
)
