# R12-1 — Wire RegisterMigration + Migrator for 25 Modules

## Context

Every Zerone module has a `migrations/v2/migrate.go` stub, but only 5 modules (`ontology`, `partnerships`, `staking`, `tree`, `vesting_rewards`) actually call `cfg.RegisterMigration()` in `RegisterServices` and have a `keeper/migrator.go`. The other 25 modules have the migration files but no wiring — so `app.ModuleManager.RunMigrations()` will silently skip them during a chain upgrade.

## Affected Modules (25)

```
alignment       autopoiesis     billing         bvm
capture_challenge  capture_defense  channels     claiming_pot
compute_pool    discovery       disputes        emergency
evidence_mgmt   gov             home            ibcratelimit
icaauth         knowledge       liquiditypool   qualification
research        schedule        tokens          toolbox
```

Note: `auth` also needs wiring — it has `keeper/migrator.go` but does NOT call `RegisterMigration` in `module.go`.

## Task

For each of the 25 modules listed above (plus `auth`):

### 1. Create `x/<module>/keeper/migrator.go` (if it doesn't exist)

6 modules already have this file (`auth`, `knowledge`, `ontology`, `partnerships`, `staking`, `vesting_rewards`). For the other 19, create it following the exact pattern from the working modules.

Use this template:

```go
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
```

### 2. Wire `RegisterMigration` in `x/<module>/module.go`

In each module's `RegisterServices` method, add the migration registration **after** the existing `RegisterMsgServer`/`RegisterQueryServer` calls:

```go
func (am AppModule) RegisterServices(cfg module.Configurator) {
	types.RegisterMsgServer(cfg.MsgServer(), keeper.NewMsgServerImpl(am.keeper))
	types.RegisterQueryServer(cfg.QueryServer(), keeper.NewQueryServerImpl(am.keeper))

	// ADD THESE LINES:
	migrator := keeper.NewMigrator(am.keeper)
	if err := cfg.RegisterMigration(types.ModuleName, 1, migrator.Migrate1to2); err != nil {
		panic(fmt.Sprintf("failed to register %s migration: %v", types.ModuleName, err))
	}
}
```

Ensure `"fmt"` is in the import block if not already present.

### 3. Verify each module's `migrations/v2/migrate.go` exists

All 30 should already have this file. Verify it compiles. The existing stubs call `keeper.Keeper` methods — make sure the keeper import path is correct (`github.com/zerone-chain/zerone/x/<module>/keeper`).

If any migration stub references methods that don't exist on the keeper, fix the import or simplify to a no-op:

```go
package v2

import (
	sdk "github.com/cosmos/cosmos-sdk/types"
)

// Migrate performs the v2 migration for the <module> module.
func Migrate(_ sdk.Context) error {
	return nil
}
```

## Reference Pattern

Working example — `x/ontology/module.go`:
```go
func (am AppModule) RegisterServices(cfg module.Configurator) {
	types.RegisterMsgServer(cfg.MsgServer(), keeper.NewMsgServerImpl(am.keeper))
	types.RegisterQueryServer(cfg.QueryServer(), keeper.NewQueryServerImpl(am.keeper))

	migrator := keeper.NewMigrator(am.keeper)
	if err := cfg.RegisterMigration(types.ModuleName, 1, migrator.Migrate1to2); err != nil {
		panic(fmt.Sprintf("failed to register %s migration: %v", types.ModuleName, err))
	}
}
```

Working example — `x/ontology/keeper/migrator.go`:
```go
package keeper

import (
	sdk "github.com/cosmos/cosmos-sdk/types"
)

type Migrator struct {
	keeper Keeper
}

func NewMigrator(keeper Keeper) Migrator {
	return Migrator{keeper: keeper}
}

func (m Migrator) Migrate1to2(_ sdk.Context) error {
	return nil
}
```

## Verification

```bash
# Must compile
go build ./...

# All module tests must pass
go test ./x/...

# Verify all 30 modules are wired (should output 30 lines)
grep -rn "RegisterMigration" x/*/module.go | wc -l
```

## Constraints

- Do NOT change `ConsensusVersion()` — it stays at 1 for all modules
- Do NOT add actual migration logic — stubs only (no-op `return nil`)
- Do NOT modify the existing 5 wired modules (`ontology`, `partnerships`, `staking`, `tree`, `vesting_rewards`)
- The `fmt` import may need to be added to some module.go files for the `panic(fmt.Sprintf(...))` call
