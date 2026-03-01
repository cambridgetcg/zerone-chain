package keeper

import (
	"context"

	sdk "github.com/cosmos/cosmos-sdk/types"

	govtypes "github.com/zerone-chain/zerone/x/gov/types"
)

// GovPartnershipsAdapter wraps the partnerships Keeper to satisfy the gov
// module's PartnershipsKeeper interface (context.Context → sdk.Context conversion).
type GovPartnershipsAdapter struct {
	k Keeper
}

// NewGovPartnershipsAdapter creates a new adapter.
func NewGovPartnershipsAdapter(k Keeper) *GovPartnershipsAdapter {
	return &GovPartnershipsAdapter{k: k}
}

// Ensure compile-time interface compliance.
var _ govtypes.PartnershipsKeeper = (*GovPartnershipsAdapter)(nil)

// SetDomainFormationFreeze implements govtypes.PartnershipsKeeper.
func (a *GovPartnershipsAdapter) SetDomainFormationFreeze(ctx context.Context, domain string, expiryHeight uint64, reason string) {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	a.k.SetDomainFormationFreeze(sdkCtx, domain, expiryHeight, reason)
}
