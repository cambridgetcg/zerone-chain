package keeper

import (
	"crypto/sha256"
	"fmt"
	"math/big"

	sdkmath "cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/zerone-chain/zerone/x/ontology/types"
)

// CreateProposal creates a new domain proposal and stores it.
func (k Keeper) CreateProposal(ctx sdk.Context, proposal *types.DomainProposal) error {
	// Validate the domain doesn't already exist
	if _, found := k.GetDomain(ctx, proposal.Domain.Name); found {
		return fmt.Errorf("%w: %s", types.ErrDomainExists, proposal.Domain.Name)
	}

	// Validate stratum is valid
	if !types.Stratum(proposal.Domain.Stratum).IsValid() {
		return fmt.Errorf("%w: %d", types.ErrInvalidStratum, proposal.Domain.Stratum)
	}

	// Check max domains per stratum
	params := k.GetParams(ctx)
	domainCount := k.CountDomainsInStratum(ctx, types.Stratum(proposal.Domain.Stratum))
	if domainCount >= params.MaxDomainsPerStratum {
		return fmt.Errorf("%w: stratum %d has %d domains (max %d)",
			types.ErrMaxDomainsReached, proposal.Domain.Stratum, domainCount, params.MaxDomainsPerStratum)
	}

	// Store the proposal
	k.SetProposal(ctx, proposal)

	// Also store the domain in "proposed" status
	proposal.Domain.Status = "proposed"
	proposal.Domain.CreatedAt = uint64(ctx.BlockHeight())
	k.SetDomain(ctx, proposal.Domain)

	k.Logger(ctx).Info("domain proposal created",
		"proposal_id", proposal.Id,
		"domain", proposal.Domain.Name,
		"stratum", proposal.Domain.Stratum,
		"proposer", proposal.Proposer,
	)

	return nil
}

// VoteProposal records a vote on a domain proposal.
func (k Keeper) VoteProposal(ctx sdk.Context, proposalId string, voter string, approve bool) error {
	proposal, found := k.GetProposal(ctx, proposalId)
	if !found {
		return fmt.Errorf("%w: %s", types.ErrProposalNotFound, proposalId)
	}

	if proposal.Status != "active" {
		return fmt.Errorf("%w: proposal %s has status %s",
			types.ErrProposalNotActive, proposalId, proposal.Status)
	}

	// Check expiry
	currentHeight := uint64(ctx.BlockHeight())
	if currentHeight > proposal.ExpiresAt {
		return fmt.Errorf("%w: proposal %s expired at height %d",
			types.ErrProposalExpired, proposalId, proposal.ExpiresAt)
	}

	// Check for duplicate votes
	if proposal.HasVoted(voter) {
		return fmt.Errorf("%w: %s on proposal %s",
			types.ErrAlreadyVoted, voter, proposalId)
	}

	// Record vote
	proposal.Voters = append(proposal.Voters, voter)
	if approve {
		proposal.VotesFor++
	} else {
		proposal.VotesAgainst++
	}

	// Check if proposal has passed
	params := k.GetParams(ctx)
	if proposal.VotesFor >= params.MinEndorsements {
		proposal.Status = "passed"

		// Execute the proposal immediately
		if err := k.ExecuteProposal(ctx, proposal); err != nil {
			k.Logger(ctx).Error("failed to execute passed proposal",
				"proposal_id", proposalId, "error", err)
			// Revert status if execution fails
			proposal.Status = "active"
			proposal.VotesFor--
			proposal.Voters = proposal.Voters[:len(proposal.Voters)-1]
		} else {
			// Refund stake on successful execution
			if err := k.RefundProposalStake(ctx, proposal); err != nil {
				k.Logger(ctx).Error("failed to refund proposal stake",
					"proposal_id", proposalId, "error", err)
			}
		}
	}

	k.SetProposal(ctx, proposal)

	ctx.EventManager().EmitEvent(
		sdk.NewEvent(
			"proposal_voted",
			sdk.NewAttribute("proposal_id", proposalId),
			sdk.NewAttribute("voter", voter),
			sdk.NewAttribute("approve", fmt.Sprintf("%t", approve)),
			sdk.NewAttribute("votes_for", fmt.Sprintf("%d", proposal.VotesFor)),
			sdk.NewAttribute("votes_against", fmt.Sprintf("%d", proposal.VotesAgainst)),
		),
	)

	return nil
}

// ProcessExpiredProposals checks all active proposals and expires those past their deadline.
// Called by EndBlocker.
func (k Keeper) ProcessExpiredProposals(ctx sdk.Context) error {
	currentHeight := uint64(ctx.BlockHeight())

	var toExpire []*types.DomainProposal

	k.IterateProposals(ctx, func(p *types.DomainProposal) bool {
		if p.Status == "active" && currentHeight > p.ExpiresAt {
			toExpire = append(toExpire, p)
		}
		return false
	})

	for _, proposal := range toExpire {
		proposal.Status = "expired"
		k.SetProposal(ctx, proposal)

		// Remove the proposed domain since it was never activated
		domain, found := k.GetDomain(ctx, proposal.Domain.Name)
		if found && domain.Status == "proposed" {
			k.DeleteDomain(ctx, proposal.Domain.Name)
		}

		ctx.EventManager().EmitEvent(
			sdk.NewEvent(
				"proposal_expired",
				sdk.NewAttribute("proposal_id", proposal.Id),
				sdk.NewAttribute("domain", proposal.Domain.Name),
				sdk.NewAttribute("votes_for", fmt.Sprintf("%d", proposal.VotesFor)),
				sdk.NewAttribute("votes_against", fmt.Sprintf("%d", proposal.VotesAgainst)),
			),
		)

		k.Logger(ctx).Info("proposal expired",
			"proposal_id", proposal.Id,
			"domain", proposal.Domain.Name,
		)
	}

	return nil
}

// ExecuteProposal activates or modifies a domain based on the proposal type.
func (k Keeper) ExecuteProposal(ctx sdk.Context, proposal *types.DomainProposal) error {
	switch proposal.ProposalType {
	case "add":
		return k.ActivateDomain(ctx, proposal.Domain.Name)
	case "deprecate":
		return k.DeprecateDomain(ctx, proposal.Domain.Name)
	case "archive":
		return k.ArchiveDomain(ctx, proposal.Domain.Name)
	case "modify":
		domain, found := k.GetDomain(ctx, proposal.Domain.Name)
		if !found {
			return fmt.Errorf("%w: %s", types.ErrDomainNotFound, proposal.Domain.Name)
		}
		// Apply modifications from the proposal domain
		if proposal.Domain != nil && proposal.Domain.DisplayName != "" {
			domain.DisplayName = proposal.Domain.DisplayName
		}
		if proposal.Domain != nil && proposal.Domain.Description != "" {
			domain.Description = proposal.Domain.Description
		}
		domain.UpdatedAt = uint64(ctx.BlockHeight())
		k.SetDomain(ctx, domain)
		return nil
	case "merge":
		return k.MergeDomains(ctx, proposal)
	default:
		return fmt.Errorf("unknown proposal type: %s", proposal.ProposalType)
	}
}

// RefundProposalStake returns the staked tokens to the proposer.
// Called when a proposal is resolved (passed or expired).
func (k Keeper) RefundProposalStake(ctx sdk.Context, proposal *types.DomainProposal) error {
	if proposal.Stake == "" || proposal.Stake == "0" || proposal.Proposer == "" {
		return nil
	}

	proposerAddr, err := sdk.AccAddressFromBech32(proposal.Proposer)
	if err != nil {
		return fmt.Errorf("invalid proposer address for refund: %w", err)
	}

	stakeAmount := new(big.Int)
	if _, ok := stakeAmount.SetString(proposal.Stake, 10); !ok || stakeAmount.Sign() <= 0 {
		return nil // no valid stake to refund
	}

	stakeCoin := sdk.NewCoin("uzrn", sdkmath.NewIntFromBigInt(stakeAmount))
	if err := k.bankKeeper.SendCoinsFromModuleToAccount(
		ctx, types.ModuleName, proposerAddr, sdk.NewCoins(stakeCoin),
	); err != nil {
		return fmt.Errorf("failed to refund stake: %w", err)
	}

	k.Logger(ctx).Info("proposal stake refunded",
		"proposal_id", proposal.Id,
		"proposer", proposal.Proposer,
		"amount", proposal.Stake,
	)

	return nil
}

// GenerateProposalID generates a deterministic proposal ID from the proposer and domain.
func GenerateProposalID(proposer, domainName string, height uint64) string {
	h := sha256.New()
	h.Write([]byte("ZRN.ontology.proposal.v1:"))
	h.Write([]byte(proposer))
	h.Write([]byte(":"))
	h.Write([]byte(domainName))
	h.Write([]byte(fmt.Sprintf(":%d", height)))
	return fmt.Sprintf("%x", h.Sum(nil))[:32]
}
