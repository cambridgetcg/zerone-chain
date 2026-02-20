package keeper

import (
	"context"
	"fmt"

	"github.com/zerone-chain/zerone/x/knowledge/types"
)

type msgServer struct {
	keeper Keeper
	types.UnimplementedMsgServer
}

// NewMsgServerImpl returns a types.MsgServer backed by the given Keeper.
func NewMsgServerImpl(keeper Keeper) types.MsgServer {
	return &msgServer{keeper: keeper}
}

func notImplemented(method string) error {
	return fmt.Errorf("knowledge: %s not implemented — see R2-2", method)
}

func (m *msgServer) SubmitClaim(_ context.Context, _ *types.MsgSubmitClaim) (*types.MsgSubmitClaimResponse, error) {
	return nil, notImplemented("SubmitClaim")
}

func (m *msgServer) SubmitCommitment(_ context.Context, _ *types.MsgSubmitCommitment) (*types.MsgSubmitCommitmentResponse, error) {
	return nil, notImplemented("SubmitCommitment")
}

func (m *msgServer) SubmitReveal(_ context.Context, _ *types.MsgSubmitReveal) (*types.MsgSubmitRevealResponse, error) {
	return nil, notImplemented("SubmitReveal")
}

func (m *msgServer) ChallengeFact(_ context.Context, _ *types.MsgChallengeFact) (*types.MsgChallengeFactResponse, error) {
	return nil, notImplemented("ChallengeFact")
}

func (m *msgServer) AddFact(_ context.Context, _ *types.MsgAddFact) (*types.MsgAddFactResponse, error) {
	return nil, notImplemented("AddFact")
}

func (m *msgServer) SubmitContradiction(_ context.Context, _ *types.MsgSubmitContradiction) (*types.MsgSubmitContradictionResponse, error) {
	return nil, notImplemented("SubmitContradiction")
}

func (m *msgServer) PatronizeFact(_ context.Context, _ *types.MsgPatronizeFact) (*types.MsgPatronizeFactResponse, error) {
	return nil, notImplemented("PatronizeFact")
}

func (m *msgServer) ProposeDomain(_ context.Context, _ *types.MsgProposeDomain) (*types.MsgProposeDomainResponse, error) {
	return nil, notImplemented("ProposeDomain")
}

func (m *msgServer) EndorseDomainProposal(_ context.Context, _ *types.MsgEndorseDomainProposal) (*types.MsgEndorseDomainProposalResponse, error) {
	return nil, notImplemented("EndorseDomainProposal")
}

func (m *msgServer) ChallengeDomainProposal(_ context.Context, _ *types.MsgChallengeDomainProposal) (*types.MsgChallengeDomainProposalResponse, error) {
	return nil, notImplemented("ChallengeDomainProposal")
}

func (m *msgServer) RegisterStratum(_ context.Context, _ *types.MsgRegisterStratum) (*types.MsgRegisterStratumResponse, error) {
	return nil, notImplemented("RegisterStratum")
}

func (m *msgServer) ChallengeProvisionalFact(_ context.Context, _ *types.MsgChallengeProvisionalFact) (*types.MsgChallengeProvisionalFactResponse, error) {
	return nil, notImplemented("ChallengeProvisionalFact")
}

func (m *msgServer) UpdateParams(_ context.Context, _ *types.MsgUpdateParams) (*types.MsgUpdateParamsResponse, error) {
	return nil, notImplemented("UpdateParams")
}

func (m *msgServer) UpdateExtendedParams(_ context.Context, _ *types.MsgUpdateExtendedParams) (*types.MsgUpdateExtendedParamsResponse, error) {
	return nil, notImplemented("UpdateExtendedParams")
}

func (m *msgServer) ProposeResearchFund(_ context.Context, _ *types.MsgProposeResearchFund) (*types.MsgProposeResearchFundResponse, error) {
	return nil, notImplemented("ProposeResearchFund")
}

func (m *msgServer) VoteResearchProposal(_ context.Context, _ *types.MsgVoteResearchProposal) (*types.MsgVoteResearchProposalResponse, error) {
	return nil, notImplemented("VoteResearchProposal")
}

func (m *msgServer) ExecuteResearchProposal(_ context.Context, _ *types.MsgExecuteResearchProposal) (*types.MsgExecuteResearchProposalResponse, error) {
	return nil, notImplemented("ExecuteResearchProposal")
}
