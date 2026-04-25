package keeper

import (
	"context"
	"fmt"
	"math/big"

	sdkmath "cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/zerone-chain/zerone/x/ontology/types"
)

type msgServer struct {
	types.UnimplementedMsgServer
	Keeper
}

// NewMsgServerImpl returns an implementation of the MsgServer interface.
func NewMsgServerImpl(keeper Keeper) types.MsgServer {
	return &msgServer{Keeper: keeper}
}

var _ types.MsgServer = msgServer{}

// ProposeDomain handles MsgProposeDomain — submits a new domain proposal.
func (k msgServer) ProposeDomain(goCtx context.Context, msg *types.MsgProposeDomain) (*types.MsgProposeDomainResponse, error) {
	ctx := sdk.UnwrapSDKContext(goCtx)
	params := k.GetParams(ctx)

	// Validate stratum exists
	if _, found := k.GetStratum(ctx, types.Stratum(msg.Stratum)); !found {
		return nil, fmt.Errorf("%w: stratum %d not registered (commitment 2: domains are bound to a registered stratum at proposal time — empirical and normative claims must remain structurally distinguishable)", types.ErrInvalidStratum, msg.Stratum)
	}

	// Check domain doesn't already exist
	if _, found := k.GetDomain(ctx, msg.Name); found {
		return nil, fmt.Errorf("%w: %s", types.ErrDomainExists, msg.Name)
	}

	// Validate and deduct stake
	stakeAmount := new(big.Int)
	if _, ok := stakeAmount.SetString(msg.Stake, 10); !ok || stakeAmount.Sign() <= 0 {
		return nil, fmt.Errorf("%w: invalid stake amount", types.ErrInsufficientStake)
	}

	minStake := new(big.Int)
	if _, ok := minStake.SetString(params.MinProposalStake, 10); !ok {
		minStake.SetInt64(1000000) // fallback default
	}
	if stakeAmount.Cmp(minStake) < 0 {
		return nil, fmt.Errorf("%w: stake %s below minimum %s",
			types.ErrInsufficientStake, msg.Stake, params.MinProposalStake)
	}

	proposerAddr, err := sdk.AccAddressFromBech32(msg.Proposer)
	if err != nil {
		return nil, fmt.Errorf("invalid proposer address: %w", err)
	}

	stakeCoin := sdk.NewCoin("uzrn", sdkmath.NewIntFromBigInt(stakeAmount))
	if err := k.bankKeeper.SendCoinsFromAccountToModule(
		ctx, proposerAddr, types.ModuleName, sdk.NewCoins(stakeCoin),
	); err != nil {
		return nil, fmt.Errorf("%w: %v", types.ErrInsufficientStake, err)
	}

	// Generate proposal
	height := uint64(ctx.BlockHeight())
	proposalID := GenerateProposalID(msg.Proposer, msg.Name, height)

	proposal := &types.DomainProposal{
		Id: proposalID,
		Domain: &types.Domain{
			Name:        msg.Name,
			DisplayName: msg.DisplayName,
			Description: msg.Description,
			Stratum:     uint32(msg.Stratum),
			Status:      "proposed",
			ProposedBy:  msg.Proposer,
		},
		Proposer:     msg.Proposer,
		ProposalType: "add",
		Stake:        msg.Stake,
		VotesFor:     0,
		VotesAgainst: 0,
		Voters:       []string{},
		Status:       "active",
		CreatedAt:    height,
		ExpiresAt:    height + params.ProposalVotingPeriod,
	}

	if err := k.CreateProposal(ctx, proposal); err != nil {
		return nil, err
	}

	ctx.EventManager().EmitEvent(
		sdk.NewEvent(
			"zerone.ontology.domain_proposed",
			sdk.NewAttribute("proposal_id", proposalID),
			sdk.NewAttribute("domain", msg.Name),
			sdk.NewAttribute("stratum", fmt.Sprintf("%d", msg.Stratum)),
			sdk.NewAttribute("proposer", msg.Proposer),
			sdk.NewAttribute("stake", msg.Stake),
		),
	)

	return &types.MsgProposeDomainResponse{ProposalId: proposalID}, nil
}

// VoteDomainProposal handles MsgVoteDomainProposal — votes on an existing proposal.
func (k msgServer) VoteDomainProposal(goCtx context.Context, msg *types.MsgVoteDomainProposal) (*types.MsgVoteDomainProposalResponse, error) {
	ctx := sdk.UnwrapSDKContext(goCtx)

	if err := k.VoteProposal(ctx, msg.ProposalId, msg.Voter, msg.Approve); err != nil {
		return nil, err
	}

	return &types.MsgVoteDomainProposalResponse{}, nil
}

// UpdateDomain handles MsgUpdateDomain — authority-only domain metadata update.
func (k msgServer) UpdateDomain(goCtx context.Context, msg *types.MsgUpdateDomain) (*types.MsgUpdateDomainResponse, error) {
	ctx := sdk.UnwrapSDKContext(goCtx)

	if k.GetAuthority() != msg.Authority {
		return nil, fmt.Errorf("unauthorized: expected %s, got %s", k.GetAuthority(), msg.Authority)
	}

	domain, found := k.GetDomain(ctx, msg.DomainName)
	if !found {
		return nil, fmt.Errorf("%w: %s", types.ErrDomainNotFound, msg.DomainName)
	}

	// Apply updates
	if msg.DisplayName != "" {
		domain.DisplayName = msg.DisplayName
	}
	if msg.Description != "" {
		domain.Description = msg.Description
	}
	if msg.Status != "" {
		domain.Status = msg.Status
	}
	domain.UpdatedAt = uint64(ctx.BlockHeight())

	k.SetDomain(ctx, domain)

	ctx.EventManager().EmitEvent(
		sdk.NewEvent(
			"zerone.ontology.domain_updated",
			sdk.NewAttribute("domain", msg.DomainName),
			sdk.NewAttribute("authority", msg.Authority),
		),
	)

	return &types.MsgUpdateDomainResponse{}, nil
}

// UpdateParams handles MsgUpdateParams — authority-only parameter update.
func (k msgServer) UpdateParams(goCtx context.Context, msg *types.MsgUpdateParams) (*types.MsgUpdateParamsResponse, error) {
	if k.GetAuthority() != msg.Authority {
		return nil, fmt.Errorf("unauthorized: expected %s, got %s", k.GetAuthority(), msg.Authority)
	}

	ctx := sdk.UnwrapSDKContext(goCtx)

	if msg.Params == nil {
		return nil, fmt.Errorf("params cannot be nil")
	}
	if err := msg.Params.Validate(); err != nil {
		return nil, fmt.Errorf("invalid params: %w", err)
	}

	k.SetParams(ctx, msg.Params)

	ctx.EventManager().EmitEvent(
		sdk.NewEvent(
			"zerone.ontology.params_updated",
			sdk.NewAttribute("authority", msg.Authority),
		),
	)

	return &types.MsgUpdateParamsResponse{}, nil
}

// RegisterLogicZone handles MsgRegisterLogicZone — authority-only zone registration.
func (k msgServer) RegisterLogicZone(goCtx context.Context, msg *types.MsgRegisterLogicZone) (*types.MsgRegisterLogicZoneResponse, error) {
	if k.GetAuthority() != msg.Authority {
		return nil, fmt.Errorf("unauthorized: expected %s, got %s", k.GetAuthority(), msg.Authority)
	}

	ctx := sdk.UnwrapSDKContext(goCtx)

	if msg.ZoneProperties == nil {
		return nil, fmt.Errorf("zone properties cannot be nil")
	}

	// Check if zone already exists
	if _, found := k.GetLogicZone(ctx, types.LogicZone(msg.ZoneProperties.Zone)); found {
		return nil, fmt.Errorf("%w: %s", types.ErrLogicZoneExists, msg.ZoneProperties.Zone)
	}

	k.SetLogicZone(ctx, msg.ZoneProperties)

	ctx.EventManager().EmitEvent(
		sdk.NewEvent(
			"zerone.ontology.logic_zone_registered",
			sdk.NewAttribute("zone", msg.ZoneProperties.Zone),
			sdk.NewAttribute("complete", fmt.Sprintf("%v", msg.ZoneProperties.Complete)),
			sdk.NewAttribute("goedel_applies", fmt.Sprintf("%v", msg.ZoneProperties.GoedelApplies)),
			sdk.NewAttribute("max_confidence_bps", fmt.Sprintf("%d", msg.ZoneProperties.MaxConfidenceBps)),
			sdk.NewAttribute("authority", msg.Authority),
		),
	)

	return &types.MsgRegisterLogicZoneResponse{}, nil
}

// AcknowledgeIncompleteness handles MsgAcknowledgeIncompleteness — records Gödelian acknowledgment.
func (k msgServer) AcknowledgeIncompleteness(goCtx context.Context, msg *types.MsgAcknowledgeIncompleteness) (*types.MsgAcknowledgeIncompletenessResponse, error) {
	ctx := sdk.UnwrapSDKContext(goCtx)

	// Validate zone exists and Gödel applies
	zone, found := k.GetLogicZone(ctx, types.LogicZone(msg.Zone))
	if !found {
		return nil, fmt.Errorf("%w: %s", types.ErrLogicZoneNotFound, msg.Zone)
	}
	if !zone.GoedelApplies {
		return nil, fmt.Errorf("%w: zone %s is complete, no acknowledgment needed",
			types.ErrGoedelInconsistency, msg.Zone)
	}

	// Store acknowledgment
	ack := types.IncompletenessAcknowledgment{
		FactId:         msg.FactId,
		Zone:           types.LogicZone(msg.Zone),
		Reason:         msg.Reason,
		AcknowledgedAt: uint64(ctx.BlockHeight()),
		AcknowledgedBy: msg.Submitter,
	}
	k.SetIncompletenessAck(ctx, ack)

	ctx.EventManager().EmitEvent(
		sdk.NewEvent(
			"zerone.ontology.incompleteness_acknowledged",
			sdk.NewAttribute("fact_id", msg.FactId),
			sdk.NewAttribute("zone", msg.Zone),
			sdk.NewAttribute("submitter", msg.Submitter),
		),
	)

	return &types.MsgAcknowledgeIncompletenessResponse{}, nil
}
