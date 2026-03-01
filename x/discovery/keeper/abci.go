package keeper

import (
	"context"
	"fmt"

	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/zerone-chain/zerone/x/discovery/types"
)

// BeginBlocker runs at the start of each block.
// Every 100 blocks it checks for expired profiles and marks them.
func (k Keeper) BeginBlocker(ctx context.Context) error {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	currentBlock := uint64(sdkCtx.BlockHeight())

	// Adaptive expiry check interval (R29-6)
	expiryCheckInterval := uint64(100)
	if k.pacingKeeper != nil {
		creationPacing, _ := k.pacingKeeper.GetGlobalPacingMultiplier(ctx)
		if creationPacing > 0 && creationPacing != 1_000_000 {
			expiryCheckInterval = 100 * 1_000_000 / creationPacing
		}
	}
	if expiryCheckInterval == 0 {
		expiryCheckInterval = 100
	}
	if currentBlock%expiryCheckInterval != 0 {
		return nil
	}

	params := k.GetParams(ctx)
	expiryBlocks := params.ProfileExpiryBlocks

	var expiredCount int
	k.IterateProfiles(ctx, func(profile *types.AgentProfile) bool {
		if profile.Status == "active" && profile.LastActiveBlock+expiryBlocks < currentBlock {
			profile.Status = "expired"
			k.SetProfile(ctx, profile)
			expiredCount++

			sdkCtx.EventManager().EmitEvent(
				sdk.NewEvent("zerone.discovery.profile_expired",
					sdk.NewAttribute("address", profile.Address),
					sdk.NewAttribute("last_active_block", fmt.Sprintf("%d", profile.LastActiveBlock)),
				),
			)
		}
		return false
	})

	return nil
}
