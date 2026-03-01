package keeper

import (
	"context"

	sdk "github.com/cosmos/cosmos-sdk/types"

	govtypes "github.com/zerone-chain/zerone/x/gov/types"

	"github.com/zerone-chain/zerone/x/emergency/types"
)

// GovEmergencyAdapter wraps the emergency Keeper to satisfy the
// gov module's EmergencyKeeper interface.
type GovEmergencyAdapter struct {
	k Keeper
}

// NewGovEmergencyAdapter returns an adapter for the gov module.
func NewGovEmergencyAdapter(k Keeper) *GovEmergencyAdapter {
	return &GovEmergencyAdapter{k: k}
}

// Compile-time interface check.
var _ govtypes.EmergencyKeeper = (*GovEmergencyAdapter)(nil)

// CountHaltsForReason counts the number of finalized halt ceremonies.
// The reason parameter is reserved for future filtering; currently all
// finalized halts are counted.
func (a *GovEmergencyAdapter) CountHaltsForReason(ctx context.Context, _ string) uint64 {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	var count uint64
	a.k.IterateCeremonies(sdkCtx, func(c *types.EmergencyCeremony) bool {
		if c.Type == string(types.CeremonyHalt) && c.Phase == string(types.PhaseFinalized) {
			count++
		}
		return false
	})
	return count
}
