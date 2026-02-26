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
// Writes a verifiable marker to confirm the migration ran successfully.
func (m Migrator) Migrate1to2(ctx sdk.Context) error {
	store := m.keeper.storeService.OpenKVStore(ctx)
	return store.Set([]byte("migration_v2_complete"), []byte("true"))
}
