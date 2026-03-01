package keeper

import (
	"context"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

// GetDomainVerificationActivity returns the verification activity level for a domain
// as a BPS value (0-1,000,000). Uses the completion index for accurate window-based counting.
// 10 rounds per window = 100% activity (R31-2 / R31-4).
func (k Keeper) GetDomainVerificationActivity(ctx context.Context, domain string) uint64 {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	height := uint64(sdkCtx.BlockHeight())

	const windowBlocks = 10_000
	rounds := k.CountCompletedRoundsForDomainInWindow(ctx, domain, height, windowBlocks)

	// Normalise: 10 rounds per window = BPS (100% activity)
	activity := rounds * BPS / 10
	if activity > BPS {
		activity = BPS
	}
	return activity
}
