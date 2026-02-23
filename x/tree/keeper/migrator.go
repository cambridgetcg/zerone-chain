package keeper

import (
	sdk "github.com/cosmos/cosmos-sdk/types"
)

// Migrator handles module state migrations.
type Migrator struct {
	keeper Keeper
}

// NewMigrator returns a new Migrator.
func NewMigrator(k Keeper) Migrator {
	return Migrator{keeper: k}
}

// Migrate1to2 is a stub for future state migration.
func (m Migrator) Migrate1to2(_ sdk.Context) error {
	return nil
}
