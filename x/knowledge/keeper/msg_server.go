package keeper

import (
	"context"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/zerone-chain/zerone/x/knowledge/types"
)

var _ types.MsgServer = msgServer{}

type msgServer struct {
	types.UnimplementedMsgServer
	keeper Keeper
}

// NewMsgServerImpl returns an implementation of MsgServer for the knowledge module.
func NewMsgServerImpl(keeper Keeper) types.MsgServer {
	return &msgServer{keeper: keeper}
}

func (m msgServer) SubmitData(ctx context.Context, msg *types.MsgSubmitData) (*types.MsgSubmitDataResponse, error) {
	return m.keeper.SubmitData(ctx, msg)
}

func (m msgServer) SubmitThread(ctx context.Context, msg *types.MsgSubmitThread) (*types.MsgSubmitThreadResponse, error) {
	return m.keeper.SubmitThread(ctx, msg)
}

func (m msgServer) SubmitCommitment(ctx context.Context, msg *types.MsgSubmitCommitment) (*types.MsgSubmitCommitmentResponse, error) {
	if err := m.keeper.SubmitCommitment(ctx, msg); err != nil {
		return nil, err
	}
	return &types.MsgSubmitCommitmentResponse{}, nil
}

func (m msgServer) SubmitReveal(ctx context.Context, msg *types.MsgSubmitReveal) (*types.MsgSubmitRevealResponse, error) {
	if err := m.keeper.SubmitReveal(ctx, msg); err != nil {
		return nil, err
	}
	return &types.MsgSubmitRevealResponse{}, nil
}

func (m msgServer) ContestSample(ctx context.Context, msg *types.MsgContestSample) (*types.MsgContestSampleResponse, error) {
	return m.keeper.ContestSample(ctx, msg)
}

func (m msgServer) SponsorSample(ctx context.Context, msg *types.MsgSponsorSample) (*types.MsgSponsorSampleResponse, error) {
	return m.keeper.SponsorSample(ctx, msg)
}

func (m msgServer) ProposeDomain(ctx context.Context, msg *types.MsgProposeDomain) (*types.MsgProposeDomainResponse, error) {
	return m.keeper.ProposeDomain(ctx, msg)
}

func (m msgServer) EndorseDomainProposal(ctx context.Context, msg *types.MsgEndorseDomainProposal) (*types.MsgEndorseDomainProposalResponse, error) {
	return m.keeper.EndorseDomainProposal(ctx, msg)
}

func (m msgServer) CreateDataset(ctx context.Context, msg *types.MsgCreateDataset) (*types.MsgCreateDatasetResponse, error) {
	return m.keeper.CreateDataset(ctx, msg)
}

func (m msgServer) AccessDataset(ctx context.Context, msg *types.MsgAccessDataset) (*types.MsgAccessDatasetResponse, error) {
	return m.keeper.AccessDataset(ctx, msg)
}

func (m msgServer) AccessSample(ctx context.Context, msg *types.MsgAccessSample) (*types.MsgAccessSampleResponse, error) {
	return m.keeper.AccessSample(ctx, msg)
}

func (m msgServer) ReportDemand(ctx context.Context, msg *types.MsgReportDemand) (*types.MsgReportDemandResponse, error) {
	return m.keeper.ReportDemand(ctx, msg)
}

func (m msgServer) FundBounty(ctx context.Context, msg *types.MsgFundBounty) (*types.MsgFundBountyResponse, error) {
	return m.keeper.FundBounty(ctx, msg)
}

func (m msgServer) RateSample(_ context.Context, _ *types.MsgRateSample) (*types.MsgRateSampleResponse, error) {
	return nil, status.Error(codes.Unimplemented, "RateSample not implemented (R37)")
}

func (m msgServer) UpdateParams(_ context.Context, _ *types.MsgUpdateParams) (*types.MsgUpdateParamsResponse, error) {
	return nil, status.Error(codes.Unimplemented, "UpdateParams not implemented (R37)")
}

func (m msgServer) AddScrapedSource(ctx context.Context, msg *types.MsgAddScrapedSource) (*types.MsgAddScrapedSourceResponse, error) {
	return m.keeper.AddScrapedSource(ctx, msg)
}

func (m msgServer) RemoveScrapedSource(ctx context.Context, msg *types.MsgRemoveScrapedSource) (*types.MsgRemoveScrapedSourceResponse, error) {
	return m.keeper.RemoveScrapedSource(ctx, msg)
}

func (m msgServer) ProposeResearchFund(_ context.Context, _ *types.MsgProposeResearchFund) (*types.MsgProposeResearchFundResponse, error) {
	return nil, status.Error(codes.Unimplemented, "ProposeResearchFund not implemented (R37)")
}

func (m msgServer) VoteResearchProposal(_ context.Context, _ *types.MsgVoteResearchProposal) (*types.MsgVoteResearchProposalResponse, error) {
	return nil, status.Error(codes.Unimplemented, "VoteResearchProposal not implemented (R37)")
}

func (m msgServer) ExecuteResearchProposal(_ context.Context, _ *types.MsgExecuteResearchProposal) (*types.MsgExecuteResearchProposalResponse, error) {
	return nil, status.Error(codes.Unimplemented, "ExecuteResearchProposal not implemented (R37)")
}

func (m msgServer) AddSample(_ context.Context, _ *types.MsgAddSample) (*types.MsgAddSampleResponse, error) {
	return nil, status.Error(codes.Unimplemented, "AddSample not implemented (R37)")
}

func (m msgServer) RevokeConsent(ctx context.Context, msg *types.MsgRevokeConsent) (*types.MsgRevokeConsentResponse, error) {
	if err := m.keeper.RevokeConsent(ctx, msg); err != nil {
		return nil, err
	}
	return &types.MsgRevokeConsentResponse{}, nil
}

func (m msgServer) UpgradeConsent(ctx context.Context, msg *types.MsgUpgradeConsent) (*types.MsgUpgradeConsentResponse, error) {
	return m.keeper.UpgradeConsent(ctx, msg)
}
