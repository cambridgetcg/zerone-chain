package app

import (
	"context"

	upgradekeeper "cosmossdk.io/x/upgrade/keeper"
	upgradetypes "cosmossdk.io/x/upgrade/types"

	govtypes "github.com/zerone-chain/zerone/x/gov/types"
)

// GovUpgradeAdapter bridges the Zerone governance module's UpgradeKeeper
// interface to the real Cosmos SDK x/upgrade keeper.
type GovUpgradeAdapter struct {
	keeper *upgradekeeper.Keeper
}

func NewGovUpgradeAdapter(k *upgradekeeper.Keeper) *GovUpgradeAdapter {
	return &GovUpgradeAdapter{keeper: k}
}

func (a *GovUpgradeAdapter) ScheduleUpgrade(ctx context.Context, plan *govtypes.UpgradePlan) error {
	return a.keeper.ScheduleUpgrade(ctx, upgradetypes.Plan{
		Name:   plan.Name,
		Height: plan.Height,
		Info:   plan.Info,
	})
}

var _ govtypes.UpgradeKeeper = (*GovUpgradeAdapter)(nil)
