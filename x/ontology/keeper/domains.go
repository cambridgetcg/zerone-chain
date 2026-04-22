package keeper

import (
	"context"
	"fmt"

	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/zerone-chain/zerone/x/ontology/types"
)

// IncrementClaimCount atomically increments a domain's claim count.
// Called by the knowledge module when a new claim is submitted.
func (k Keeper) IncrementClaimCount(ctx sdk.Context, domainName string) error {
	domain, found := k.GetDomain(ctx, domainName)
	if !found {
		return fmt.Errorf("%w: %s", types.ErrDomainNotFound, domainName)
	}
	domain.ClaimCount++
	domain.UpdatedAt = uint64(ctx.BlockHeight())
	k.SetDomain(ctx, domain)
	return nil
}

// IncrementFactCount atomically increments a domain's verified fact count.
// Called by the knowledge module when a claim is accepted.
func (k Keeper) IncrementFactCount(ctx sdk.Context, domainName string) error {
	domain, found := k.GetDomain(ctx, domainName)
	if !found {
		return fmt.Errorf("%w: %s", types.ErrDomainNotFound, domainName)
	}
	domain.FactCount++
	domain.UpdatedAt = uint64(ctx.BlockHeight())
	k.SetDomain(ctx, domain)
	return nil
}

// ValidateDomainForClaim checks whether a domain is valid for claim submission.
// Returns an error if the domain does not exist or is not active.
func (k Keeper) ValidateDomainForClaim(ctx sdk.Context, domainName string) error {
	domain, found := k.GetDomain(ctx, domainName)
	if !found {
		return fmt.Errorf("%w: %s", types.ErrDomainNotFound, domainName)
	}
	if domain.Status != "active" {
		return fmt.Errorf("%w: domain %s has status %s", types.ErrDomainInactive, domainName, domain.Status)
	}
	return nil
}

// GetDomainConfidenceCeiling returns the maximum confidence value allowed for a domain
// based on the stratum it belongs to. Also returns the decay rate.
func (k Keeper) GetDomainConfidenceCeiling(ctx sdk.Context, domainName string) (maxConfidence uint64, decayRate uint64, err error) {
	domain, found := k.GetDomain(ctx, domainName)
	if !found {
		return 0, 0, fmt.Errorf("%w: %s", types.ErrDomainNotFound, domainName)
	}

	stratum, found := k.GetStratum(ctx, types.Stratum(domain.Stratum))
	if !found {
		return 0, 0, fmt.Errorf("%w: stratum %d for domain %s",
			types.ErrInvalidStratum, domain.Stratum, domainName)
	}

	return stratum.MaxConfidence, stratum.DecayRate, nil
}

// GetStratumPropsForDomain returns the stratum properties for a given domain.
func (k Keeper) GetStratumPropsForDomain(ctx sdk.Context, domainName string) (*types.StratumProperties, error) {
	domain, found := k.GetDomain(ctx, domainName)
	if !found {
		return nil, fmt.Errorf("%w: %s", types.ErrDomainNotFound, domainName)
	}

	stratum, found := k.GetStratum(ctx, types.Stratum(domain.Stratum))
	if !found {
		return nil, fmt.Errorf("%w: stratum %d for domain %s",
			types.ErrInvalidStratum, domain.Stratum, domainName)
	}

	return stratum, nil
}

// IsGoedelConstrained returns whether claims in this domain are subject to
// Goedel's incompleteness theorems, which means the system acknowledges that
// some true statements may be unprovable.
func (k Keeper) IsGoedelConstrained(ctx sdk.Context, domainName string) (bool, error) {
	stratum, err := k.GetStratumPropsForDomain(ctx, domainName)
	if err != nil {
		return false, err
	}
	return stratum.GoedelApplies, nil
}

// CountDomainsInStratum returns the number of domains belonging to a stratum.
func (k Keeper) CountDomainsInStratum(ctx sdk.Context, stratum types.Stratum) uint32 {
	domains := k.GetDomainsByStratum(ctx, stratum)
	return uint32(len(domains))
}

// emitDomainStatusTransition emits the unified domain_status_transition event
// so indexers can follow the full lifecycle without subscribing to each
// named event individually (L1).
func emitDomainStatusTransition(ctx sdk.Context, domainName, fromStatus, toStatus, reason string) {
	ctx.EventManager().EmitEvent(
		sdk.NewEvent(
			"zerone.ontology.domain_status_transition",
			sdk.NewAttribute("domain", domainName),
			sdk.NewAttribute("from_status", fromStatus),
			sdk.NewAttribute("to_status", toStatus),
			sdk.NewAttribute("reason", reason),
			sdk.NewAttribute("height", fmt.Sprintf("%d", ctx.BlockHeight())),
		),
	)
}

// DeprecateDomain marks a domain as deprecated. Deprecated domains cannot
// accept new claims but existing facts remain queryable.
func (k Keeper) DeprecateDomain(ctx sdk.Context, domainName string) error {
	domain, found := k.GetDomain(ctx, domainName)
	if !found {
		return fmt.Errorf("%w: %s", types.ErrDomainNotFound, domainName)
	}
	fromStatus := domain.Status
	domain.Status = "deprecated"
	domain.UpdatedAt = uint64(ctx.BlockHeight())
	k.SetDomain(ctx, domain)

	ctx.EventManager().EmitEvent(
		sdk.NewEvent(
			"zerone.ontology.domain_deprecated",
			sdk.NewAttribute("domain", domainName),
			sdk.NewAttribute("height", fmt.Sprintf("%d", ctx.BlockHeight())),
		),
	)
	emitDomainStatusTransition(ctx, domainName, fromStatus, "deprecated", "deprecate_call")

	return nil
}

// ArchiveDomain marks a domain as archived. Archived domains cannot accept new claims
// and are hidden from default listings, but existing facts remain queryable for historical reference.
func (k Keeper) ArchiveDomain(ctx sdk.Context, domainName string) error {
	domain, found := k.GetDomain(ctx, domainName)
	if !found {
		return fmt.Errorf("%w: %s", types.ErrDomainNotFound, domainName)
	}
	fromStatus := domain.Status
	domain.Status = "archived"
	domain.UpdatedAt = uint64(ctx.BlockHeight())
	k.SetDomain(ctx, domain)

	ctx.EventManager().EmitEvent(
		sdk.NewEvent(
			"zerone.ontology.domain_archived",
			sdk.NewAttribute("domain", domainName),
			sdk.NewAttribute("height", fmt.Sprintf("%d", ctx.BlockHeight())),
		),
	)
	emitDomainStatusTransition(ctx, domainName, fromStatus, "archived", "archive_call")

	return nil
}

// MergeDomains merges a source domain into a target domain. The source domain is archived
// and its claim/fact counts are added to the target. Both domains must exist.
// For merge proposals, the target domain name is stored in proposal.Domain.Description
// using the convention "merge_into:<target_name>".
func (k Keeper) MergeDomains(ctx sdk.Context, proposal *types.DomainProposal) error {
	targetName := types.ParseMergeTarget(proposal.Domain.Description)
	if targetName == "" {
		return fmt.Errorf("merge proposal missing merge target (expected 'merge_into:<name>' in description)")
	}

	sourceName := proposal.Domain.Name

	source, found := k.GetDomain(ctx, sourceName)
	if !found {
		return fmt.Errorf("%w: source %s", types.ErrDomainNotFound, sourceName)
	}
	target, found := k.GetDomain(ctx, targetName)
	if !found {
		return fmt.Errorf("%w: target %s", types.ErrDomainNotFound, targetName)
	}

	// Transfer counts
	target.ClaimCount += source.ClaimCount
	target.FactCount += source.FactCount
	target.UpdatedAt = uint64(ctx.BlockHeight())
	k.SetDomain(ctx, target)

	// Archive source
	sourceFromStatus := source.Status
	source.Status = "archived"
	source.UpdatedAt = uint64(ctx.BlockHeight())
	k.SetDomain(ctx, source)

	ctx.EventManager().EmitEvent(
		sdk.NewEvent(
			"zerone.ontology.domains_merged",
			sdk.NewAttribute("source", sourceName),
			sdk.NewAttribute("target", targetName),
			sdk.NewAttribute("height", fmt.Sprintf("%d", ctx.BlockHeight())),
		),
	)
	emitDomainStatusTransition(ctx, sourceName, sourceFromStatus, "archived", "merged_into:"+targetName)

	return nil
}

// MaxDomainDepth is the maximum allowed nesting depth for domains.
const MaxDomainDepth = 5

// ComputeDepth computes the depth of a domain from its parent chain.
// Root domains (no parent) have depth 1.
func (k Keeper) ComputeDepth(ctx sdk.Context, parentDomain string) (uint32, error) {
	if parentDomain == "" {
		return 1, nil
	}
	parent, found := k.GetDomain(ctx, parentDomain)
	if !found {
		return 0, fmt.Errorf("%w: parent %s", types.ErrDomainNotFound, parentDomain)
	}
	depth := parent.Depth + 1
	if depth > MaxDomainDepth {
		return 0, fmt.Errorf("%w: depth %d exceeds max %d", types.ErrInvalidHierarchy, depth, MaxDomainDepth)
	}
	return depth, nil
}

// GetDomainDepth returns the depth of a domain. Returns 1 if the domain has no depth set (legacy).
func (k Keeper) GetDomainDepth(ctx sdk.Context, domainName string) (uint32, error) {
	domain, found := k.GetDomain(ctx, domainName)
	if !found {
		return 0, fmt.Errorf("%w: %s", types.ErrDomainNotFound, domainName)
	}
	if domain.Depth == 0 {
		return 1, nil // legacy domains default to depth 1
	}
	return domain.Depth, nil
}

// GetRelatedStrata returns stratum names of domains related to the given domain
// via cross-links (R31-4). This enables cross-stratum partnership matching.
// Accepts context.Context so it directly satisfies the partnerships OntologyKeeper interface.
func (k Keeper) GetRelatedStrata(ctx context.Context, domain string) []string {
	sdkCtx := sdk.UnwrapSDKContext(ctx)

	links := k.GetLinksBySource(sdkCtx, domain)
	strataSet := make(map[string]bool)

	// Include the source domain's own stratum name
	if srcDomain, found := k.GetDomain(sdkCtx, domain); found {
		if stratum, found := k.GetStratum(sdkCtx, types.Stratum(srcDomain.Stratum)); found {
			strataSet[stratum.Name] = true
		}
	}

	// Add strata of all linked target domains
	for _, link := range links {
		targetDomain, found := k.GetDomain(sdkCtx, link.TargetDomain)
		if found {
			if stratum, found := k.GetStratum(sdkCtx, types.Stratum(targetDomain.Stratum)); found {
				strataSet[stratum.Name] = true
			}
		}
	}

	strata := make([]string, 0, len(strataSet))
	for s := range strataSet {
		strata = append(strata, s)
	}
	return strata
}

// ActivateDomain transitions a proposed domain to active status.
func (k Keeper) ActivateDomain(ctx sdk.Context, domainName string) error {
	domain, found := k.GetDomain(ctx, domainName)
	if !found {
		return fmt.Errorf("%w: %s", types.ErrDomainNotFound, domainName)
	}
	fromStatus := domain.Status
	domain.Status = "active"
	domain.UpdatedAt = uint64(ctx.BlockHeight())
	k.SetDomain(ctx, domain)

	ctx.EventManager().EmitEvent(
		sdk.NewEvent(
			"zerone.ontology.domain_activated",
			sdk.NewAttribute("domain", domainName),
			sdk.NewAttribute("height", fmt.Sprintf("%d", ctx.BlockHeight())),
		),
	)
	emitDomainStatusTransition(ctx, domainName, fromStatus, "active", "activate_call")

	return nil
}
