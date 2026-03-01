package keeper

import (
	"context"

	"github.com/zerone-chain/zerone/x/alignment/types"
)

// GetGlobalPacingMultiplier returns creation and analysis pacing multipliers
// based on the current health category. Values are in BPS (1,000,000 = 100%).
// Degraded health slows creation (longer cooldowns) and speeds analysis (shorter intervals).
// Critical health doubles these effects.
func (k Keeper) GetGlobalPacingMultiplier(ctx context.Context) (creationBps, analysisBps uint64) {
	state := k.GetState(ctx)
	if state == nil || !state.Enabled {
		return types.BPS, types.BPS
	}

	switch state.PreviousCategory {
	case types.CategoryHealthy:
		return types.BPS, types.BPS
	case types.CategoryDegraded:
		return 750_000, 1_500_000
	case types.CategoryCritical:
		return 500_000, 2_000_000
	default:
		return types.BPS, types.BPS
	}
}
