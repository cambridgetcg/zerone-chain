package keeper

import sdk "github.com/cosmos/cosmos-sdk/types"

// Migrator is the module state migrator.
type Migrator struct {
	keeper Keeper
}

// NewMigrator creates a new Migrator.
func NewMigrator(keeper Keeper) Migrator {
	return Migrator{keeper: keeper}
}

// Migrate1to2 migrates from version 1 to 2 (stub).
func (m Migrator) Migrate1to2(_ sdk.Context) error {
	return nil
}
