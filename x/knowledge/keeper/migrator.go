package keeper

import (
	sdk "github.com/cosmos/cosmos-sdk/types"

	v3 "github.com/zerone-chain/zerone/x/knowledge/migrations/v3"
	v4 "github.com/zerone-chain/zerone/x/knowledge/migrations/v4"
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
	return m.keeper.WriteMigrationMarker(ctx, "migration_v2_complete", "true")
}

// Migrate2to3 migrates from version 2 to version 3.
// Backfills R29 param defaults for zero-valued fields after upgrade.
func (m Migrator) Migrate2to3(ctx sdk.Context) error {
	return v3.Migrate(ctx, m.keeper)
}

// Migrate3to4 migrates from version 3 to version 4 (Route B Wave 10).
// Backfills TraceSchema if missing; records a verifiable marker.
// See docs/UPGRADE_PROTOCOL.md for the canonical pattern this follows.
func (m Migrator) Migrate3to4(ctx sdk.Context) error {
	return v4.Migrate(ctx, m.keeper)
}
