package keeper

import (
	"context"
	"fmt"

	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/zerone-chain/zerone/x/knowledge/types"
)

const (
	// MinDomainEndorsements is the number of endorsements required to activate a proposed domain.
	MinDomainEndorsements = 3
)

// ProposeDomain creates a new domain proposal. The domain starts in PROPOSED status
// and activates when it receives MinDomainEndorsements endorsements.
func (k Keeper) ProposeDomain(ctx context.Context, msg *types.MsgProposeDomain) (*types.MsgProposeDomainResponse, error) {
	sdkCtx := sdk.UnwrapSDKContext(ctx)

	// Check domain doesn't already exist
	if _, found := k.GetDomain(ctx, msg.Name); found {
		return nil, types.ErrDomainExists.Wrapf("domain %q already exists", msg.Name)
	}

	domain := &types.Domain{
		Name:           msg.Name,
		Description:    msg.Description,
		Status:         types.DomainStatus_DOMAIN_STATUS_PROPOSED,
		CreatedAtBlock: uint64(sdkCtx.BlockHeight()),
		Proposer:       msg.Proposer,
		Endorsers:      []string{msg.Proposer}, // proposer auto-endorses
		Stratum:        msg.Stratum,
		Depth:          1,
	}

	if err := k.SetDomain(ctx, domain); err != nil {
		return nil, err
	}

	sdkCtx.EventManager().EmitEvent(sdk.NewEvent(
		"propose_domain",
		sdk.NewAttribute("domain", msg.Name),
		sdk.NewAttribute("proposer", msg.Proposer),
	))

	return &types.MsgProposeDomainResponse{ProposalId: msg.Name}, nil
}

// EndorseDomainProposal adds an endorsement to a proposed domain.
// If the endorsement count reaches MinDomainEndorsements, the domain activates.
func (k Keeper) EndorseDomainProposal(ctx context.Context, msg *types.MsgEndorseDomainProposal) (*types.MsgEndorseDomainProposalResponse, error) {
	sdkCtx := sdk.UnwrapSDKContext(ctx)

	domain, found := k.GetDomain(ctx, msg.ProposalId)
	if !found {
		return nil, types.ErrDomainNotFound.Wrapf("domain %q not found", msg.ProposalId)
	}

	if domain.Status != types.DomainStatus_DOMAIN_STATUS_PROPOSED {
		return nil, types.ErrInvalidDomain.Wrapf("domain %q is not in PROPOSED status", msg.ProposalId)
	}

	// Check for duplicate endorsement
	for _, e := range domain.Endorsers {
		if e == msg.Endorser {
			return nil, types.ErrInvalidDomain.Wrap("already endorsed")
		}
	}

	domain.Endorsers = append(domain.Endorsers, msg.Endorser)

	// Activate if enough endorsements
	if len(domain.Endorsers) >= MinDomainEndorsements {
		domain.Status = types.DomainStatus_DOMAIN_STATUS_ACTIVE
		sdkCtx.EventManager().EmitEvent(sdk.NewEvent(
			"activate_domain",
			sdk.NewAttribute("domain", domain.Name),
			sdk.NewAttribute("endorsement_count", fmt.Sprintf("%d", len(domain.Endorsers))),
		))
	}

	if err := k.SetDomain(ctx, domain); err != nil {
		return nil, err
	}

	sdkCtx.EventManager().EmitEvent(sdk.NewEvent(
		"endorse_domain",
		sdk.NewAttribute("domain", msg.ProposalId),
		sdk.NewAttribute("endorser", msg.Endorser),
	))

	return &types.MsgEndorseDomainProposalResponse{}, nil
}
