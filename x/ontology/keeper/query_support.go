package keeper

import (
	"context"
	"fmt"

	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/zerone-chain/zerone/x/ontology/types"
)

// ---------- OntologyKeeper interface methods ----------
// These methods satisfy the OntologyKeeper interface defined in
// x/knowledge/types/expected_keepers.go, providing confidence ceilings,
// logic zone validation, and stratum definitions for the knowledge module.

// GetConfidenceCeiling returns the max confidence for a stratum by name.
// Satisfies: OntologyKeeper.GetConfidenceCeiling(ctx context.Context, stratum string) (uint64, error)
func (k Keeper) GetConfidenceCeiling(goCtx context.Context, stratumName string) (uint64, error) {
	ctx := sdk.UnwrapSDKContext(goCtx)

	// Look up stratum by name
	allStrata := k.GetAllStrata(ctx)
	for _, s := range allStrata {
		if s.Name == stratumName {
			return s.MaxConfidence, nil
		}
	}
	return 0, fmt.Errorf("%w: stratum %s not found", types.ErrInvalidStratum, stratumName)
}

// IsValidLogicZone checks if a given zone name is a registered logic zone.
// Satisfies: OntologyKeeper.IsValidLogicZone(ctx context.Context, domain string) (bool, error)
func (k Keeper) IsValidLogicZone(goCtx context.Context, zoneName string) (bool, error) {
	ctx := sdk.UnwrapSDKContext(goCtx)
	_, found := k.GetLogicZone(ctx, types.LogicZone(zoneName))
	return found, nil
}

// AcknowledgesIncompleteness checks if a domain's stratum acknowledges Gödelian limits.
// Satisfies: OntologyKeeper.AcknowledgesIncompleteness(ctx context.Context, domain string) (bool, error)
func (k Keeper) AcknowledgesIncompleteness(goCtx context.Context, domainName string) (bool, error) {
	ctx := sdk.UnwrapSDKContext(goCtx)
	stratum, err := k.GetStratumPropsForDomain(ctx, domainName)
	if err != nil {
		return false, err
	}
	return stratum.GoedelApplies, nil
}

// GetStratumForDomain returns the stratum name string for a domain.
// Satisfies: OntologyKeeper.GetStratumForDomain(ctx context.Context, domain string) (string, error)
func (k Keeper) GetStratumForDomain(goCtx context.Context, domainName string) (string, error) {
	ctx := sdk.UnwrapSDKContext(goCtx)
	stratum, err := k.GetStratumPropsForDomain(ctx, domainName)
	if err != nil {
		return "", err
	}
	return stratum.Name, nil
}

// GetDepthForDomain returns the tree depth of a domain.
// Satisfies cross-module OntologyKeeper interfaces.
func (k Keeper) GetDepthForDomain(goCtx context.Context, domainName string) (uint32, error) {
	ctx := sdk.UnwrapSDKContext(goCtx)
	return k.GetDomainDepth(ctx, domainName)
}
