package keeper

import (
	"context"

	sbridgetypes "github.com/zerone-chain/zerone/x/substrate_bridge/types"
	qtypes "github.com/zerone-chain/zerone/x/qualification/types"
)

// SubstrateBridgeQualificationAdapter bridges x/qualification to the narrow
// QualificationKeeper interface expected by x/substrate_bridge.
// It wraps GetQualification (returns *DomainQualification) into the
// GetDomainQualification signature (returns DomainQualification value).
type SubstrateBridgeQualificationAdapter struct {
	k Keeper
}

// NewSubstrateBridgeQualificationAdapter returns an adapter satisfying
// substrate_bridge/types.QualificationKeeper.
func NewSubstrateBridgeQualificationAdapter(k Keeper) *SubstrateBridgeQualificationAdapter {
	return &SubstrateBridgeQualificationAdapter{k: k}
}

// GetDomainQualification returns the DomainQualification for (address, domain).
// Implements substrate_bridge/types.QualificationKeeper.
func (a *SubstrateBridgeQualificationAdapter) GetDomainQualification(
	ctx context.Context, address, domain string,
) (qtypes.DomainQualification, bool) {
	ptr, found := a.k.GetQualification(ctx, address, domain)
	if !found || ptr == nil {
		return qtypes.DomainQualification{}, false
	}
	return *ptr, true
}

// Compile-time interface assertion.
var _ sbridgetypes.QualificationKeeper = (*SubstrateBridgeQualificationAdapter)(nil)
