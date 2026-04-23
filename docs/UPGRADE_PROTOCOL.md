# Zerone Upgrade Protocol

> **The canonical pattern every module follows to roll a new version onto a live chain.** Written for future contributors adding a new migration, new wave, or new module.

Upgrades in Zerone are engineered at four layers that reinforce each other:

1. **State-type layer** — proto messages add fields; old readers ignore, new readers see zero values → migrations backfill.
2. **Keeper layer** — a per-module `Migrator` holds one `MigrateNtoN+1` function per lineage step.
3. **Module layer** — each `AppModule` declares its `ConsensusVersion()` and calls `cfg.RegisterMigration` for every lineage step.
4. **App layer** — `app/upgrades.go` registers named upgrade handlers that call `ModuleManager.RunMigrations` and can run cross-module post-hooks (marker writes, data moves, etc.).

The test layer exercises all four, end-to-end, via `app.RunUpgradeHandlerForTests` — see `tests/cross_stack/upgrade_e2e_test.go`.

---

## How to add a migration (the 5-step recipe)

Use this recipe whenever a new wave changes state in a way old chains need to catch up to.

### Step 1 — write the migration as a package

Create `x/<module>/migrations/v<N+1>/migrate.go`. Declare a narrow interface (local to the migration package) naming only the keeper methods you call. This avoids the migrations → keeper → migrations import cycle.

```go
// x/knowledge/migrations/v4/migrate.go
package v4

type V4MigrationKeeper interface {
	GetTraceSchema(ctx context.Context) (*types.TraceSchema, bool)
	SeedDefaultTraceSchema(ctx context.Context) error
	WriteMigrationMarker(ctx context.Context, key, value string) error
}

func Migrate(ctx context.Context, k V4MigrationKeeper) error {
	if _, ok := k.GetTraceSchema(ctx); !ok {
		if err := k.SeedDefaultTraceSchema(ctx); err != nil {
			return err
		}
	}
	return k.WriteMigrationMarker(ctx, "migration_v4_complete", "true")
}
```

**Invariants:**
- **Idempotent.** Safe to run twice (e.g., if an upgrade is re-applied after a rollback). Always "set if missing", never "set blindly".
- **Marker at the end.** Write `migration_v<N>_complete` via `WriteMigrationMarker` so end-to-end tests can prove the migration ran.
- **Error rollback.** Any returned error rolls the upgrade back — be precise.

### Step 2 — wire the migrator method

Add one method to `x/<module>/keeper/migrator.go`:

```go
func (m Migrator) Migrate3to4(ctx sdk.Context) error {
	return v4.Migrate(ctx, m.keeper)
}
```

### Step 3 — register the migration

In `x/<module>/module.go`, inside `RegisterServices`:

```go
if err := cfg.RegisterMigration(types.ModuleName, 3, migrator.Migrate3to4); err != nil {
	panic(fmt.Sprintf("failed to register %s migration v3→v4: %v", types.ModuleName, err))
}
```

### Step 4 — bump the ConsensusVersion

```go
func (AppModule) ConsensusVersion() uint64 { return 4 }
```

**Important:** if you forget this step, the migration you just registered never runs — the Cosmos module manager only fires migrations when `fromVM[name] < ConsensusVersion()`.

### Step 5 — add a named upgrade handler (if this is a chain-level release)

In `app/upgrades.go`:

```go
const UpgradeNameTestnetV3 = "v1.0.2-testnet"

app.UpgradeKeeper.SetUpgradeHandler(
	UpgradeNameTestnetV3,
	func(ctx context.Context, plan upgradetypes.Plan, fromVM module.VersionMap) (module.VersionMap, error) {
		toVM, err := app.ModuleManager.RunMigrations(ctx, app.configurator, fromVM)
		if err != nil {
			return nil, err
		}
		// Optional: handler-level marker, cross-module data moves, etc.
		if err := app.KnowledgeKeeper.WriteMigrationMarker(ctx, "upgrade_marker_v1.0.2", "migrated"); err != nil {
			return nil, err
		}
		return toVM, nil
	},
)
```

Then add the lineage entry in `app/upgrade_registry.go`:

```go
{
	UpgradeName: UpgradeNameTestnetV3,
	Description: "v1.0.2-testnet — Wave 10 reference upgrade exercising knowledge v3→v4 …",
},
```

Parity between the registered handler and the lineage entry is **tested** in `TestUpgrade_LineageParityWithHandlers`. If you forget either side, the test fails.

---

## The migration marker side-channel

Every module writes its upgrade markers via `WriteMigrationMarker(ctx, key, value)`. Markers live in a dedicated sub-namespace (prefix `0x7F 0x01`) that never conflicts with domain state.

**Semantics:**
- `key` is namespaced per-migration (`migration_v4_complete`) or per-handler (`upgrade_marker_v1.0.2`).
- `value` is a free-form string — usually `"true"` or `"migrated"`.
- Writing the same `(key, value)` twice is idempotent.
- Writing a different `value` for an existing key is **rejected silently with a warning log** — first writer wins. This prevents a later migration from overwriting an earlier marker's verification trail.

**Reading:** `ReadMigrationMarker(ctx, key)` returns `""` if absent; the stored value otherwise.

---

## Cross-module coordination

When an upgrade touches multiple modules, the right order matters. Rules:

1. **Module manager already orders migrations** per module independently — each module's full v1→v2→…→vN chain runs in one pass.
2. **Cross-module dependencies** (e.g., module B's v5 migration needs module A's v5 state present) are the handler's responsibility. Handle them in the `SetUpgradeHandler` function **after** `RunMigrations` has returned successfully.
3. **Never depend on a specific migration ordering across modules.** The module manager can reorder; the only guaranteed sequence is: all migrations complete → handler post-hook runs → toVM returned.
4. **Idempotent everything.** Any migration or post-hook must be safe to run twice. Chain rollbacks and re-applications happen; state invariants must hold under replay.

---

## Testing an upgrade end-to-end

Use `app.RunUpgradeHandlerForTests` to drive the full pipeline in-harness:

```go
current := h.App.CurrentModuleVersionMap()
fromVM := make(module.VersionMap, len(current))
for name, ver := range current {
	fromVM[name] = ver
}
fromVM["knowledge"] = 3 // downshift the module being tested

toVM, err := h.App.RunUpgradeHandlerForTests(h.Ctx, UpgradeNameTestnetV3, fromVM, h.Height())
require.NoError(t, err)
require.Equal(t, uint64(4), toVM["knowledge"])
require.Equal(t, "true", h.KnowledgeKeeper.ReadMigrationMarker(h.Ctx, "migration_v4_complete"))
require.Equal(t, "migrated", h.KnowledgeKeeper.ReadMigrationMarker(h.Ctx, "upgrade_marker_v1.0.2"))
```

**Important caveats:**
- Downshift only the module(s) whose migration you're testing. Cosmos SDK modules (bank, staking, distribution…) have their own v1→v2 migrations that require test fixtures the cross-stack harness doesn't provide. Downshifting them triggers SDK-level migration code paths that may panic.
- Assert **both** a per-module marker and a handler-level marker so you know both layers ran — not just one of them.

---

## Observability

### Chain-level version report

```go
report := app.BuildChainVersionReport()
// report.Modules          — every module's current ConsensusVersion (sorted)
// report.KnownUpgrades    — every upgrade name this binary can run
```

**Use case:** a dashboard or CLI that answers "what upgrades is this chain prepared to execute?" at any moment.

### Lineage-parity guard

`TestUpgrade_LineageParityWithHandlers` asserts that every advertised upgrade has a registered handler, and vice versa. If they drift, the test fails.

---

## What this protocol does NOT handle

- **Hard forks / state rewrites.** Upgrades assume in-place state migration. If a rewrite is needed (e.g., a proto message fundamentally changes shape), a genesis export → rewrite → import flow is the path — see `x/knowledge/keeper/genesis.go` (Wave 8 round-trip).
- **Rollback after a partial migration.** If a migration panics mid-way, the block is aborted and state reverts to pre-block. A migration that wants "transactional" semantics beyond this (e.g., "revert only my changes if module B's migration fails later") must wrap its own writes in a cache context.
- **Cross-chain upgrade coordination.** IBC state, light-client attestations, etc. are out of scope. Chains upgrading in concert must coordinate governance separately.

---

## When to cut a new upgrade name

A new `UpgradeName<X>` constant is warranted when:

- Any module's `ConsensusVersion()` increases, **and**
- That version needs to be applied to a running chain (vs. just waiting for the next genesis).

For purely additive proto changes with no state shape change, no new upgrade name is needed — the `omitempty` default makes old state readable by new code without action.

---

## The test guard

`tests/cross_stack/upgrade_e2e_test.go` exercises:
1. `BuildChainVersionReport` — sorting, completeness, non-drift.
2. V1→V2 full chain (knowledge downshifted → Migrate1to2 + Migrate2to3 + Migrate3to4 fire in sequence).
3. V3→V4 single step (knowledge downshifted from 3 → backfills TraceSchema → writes marker).
4. Unknown handler rejection — no silent success on a non-existent upgrade name.
5. Marker idempotency — first writer wins; conflicting writes preserve the original.
6. Lineage parity — handlers ↔ lineage entries ↔ constants all align.

**Run it before every upgrade release.** If this passes, the mechanism works; if a new wave's migration is correct, it's part of this test; if it isn't, the wave has no verified upgrade path.

---

— **Route B, Wave 10 · The Upgrade Protocol** · 2026-04-23
