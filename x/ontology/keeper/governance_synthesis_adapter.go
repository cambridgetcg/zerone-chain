package keeper

import (
	"context"

	sdk "github.com/cosmos/cosmos-sdk/types"

	govsynthtypes "github.com/zerone-chain/zerone/x/governance_synthesis/types"
)

// GovernanceSynthesisAdapter exposes the narrow read x/governance_synthesis
// needs for its Frontier query: just the list of domain names.
type GovernanceSynthesisAdapter struct {
	k Keeper
}

func NewGovernanceSynthesisAdapter(k Keeper) *GovernanceSynthesisAdapter {
	return &GovernanceSynthesisAdapter{k: k}
}

// IterateDomainNames calls cb with each registered domain. Walks
// the existing GetAllDomains; bounded by the chain's
// MaxDomainsPerStratum cap.
func (a *GovernanceSynthesisAdapter) IterateDomainNames(ctx context.Context, cb func(name string) bool) {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	for _, d := range a.k.GetAllDomains(sdkCtx) {
		if d == nil {
			continue
		}
		if cb(d.Name) {
			return
		}
	}
}

var _ govsynthtypes.OntologyKeeper = (*GovernanceSynthesisAdapter)(nil)
