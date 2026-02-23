package keeper

import (
	sdk "github.com/cosmos/cosmos-sdk/types"
)

// Migrator handles in-place store migrations.
type Migrator struct {
	keeper Keeper
}

// NewMigrator returns a new Migrator.
func NewMigrator(keeper Keeper) Migrator {
	return Migrator{keeper: keeper}
}

// Migrate1to2 migrates from version 1 to version 2.
// Stub — implement when v2 state changes are needed.
func (m Migrator) Migrate1to2(_ sdk.Context) error {
	return nil
}
