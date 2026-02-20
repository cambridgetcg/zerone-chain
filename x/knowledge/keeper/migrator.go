package keeper

// Migrator handles module state migrations for x/upgrade compatibility.
type Migrator struct {
	keeper Keeper
}

// NewMigrator creates a new Migrator.
func NewMigrator(keeper Keeper) Migrator {
	return Migrator{keeper: keeper}
}

// Migrate1to2 will handle v1→v2 state migration when needed.
// func (m Migrator) Migrate1to2(ctx sdk.Context) error { ... }
