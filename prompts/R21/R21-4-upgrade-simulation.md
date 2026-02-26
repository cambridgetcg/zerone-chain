# R21-4 — Upgrade Simulation: Governance-Triggered State Migration

## Context

Zerone has the upgrade infrastructure:

- `app/upgrades.go` — `v1.0.0-testnet` handler calling `RunMigrations`
- `app/upgrades.go` — `RegisterStoreUpgrades` for added/removed store keys
- `x/*/migrations/v2/` — per-module v1→v2 migration code (alignment, auth, autopoiesis, billing+)
- Cosmos SDK's `x/upgrade` module registered in app.go

But no test verifies the full pipeline: governance proposal → upgrade plan → halt at height → restart with new binary → migrations run → chain continues.

This is the kind of bug that's catastrophic to discover after launch. Simulate it now.

## Prerequisites

- R21-1 complete (localnet boots and passes tests)

## Task

### Step 1: Create a v1.0.1-testnet Upgrade Handler

In `app/upgrades.go`, add a second handler alongside the existing one:

```go
const UpgradeNameTestnetV2 = "v1.0.1-testnet"

// v1.0.1-testnet — simulated upgrade for testing the migration pipeline.
app.UpgradeKeeper.SetUpgradeHandler(
    UpgradeNameTestnetV2,
    func(ctx context.Context, plan upgradetypes.Plan, fromVM module.VersionMap) (module.VersionMap, error) {
        app.Logger().Info(fmt.Sprintf("applying upgrade %q at height %d", plan.Name, plan.Height))
        
        // Run module migrations
        toVM, err := app.ModuleManager.RunMigrations(ctx, app.configurator, fromVM)
        if err != nil {
            return nil, err
        }
        
        // Verify: write a marker to store to prove migration ran
        store := ctx.KVStore(app.GetKey("knowledge"))
        store.Set([]byte("upgrade_marker_v1.0.1"), []byte("migrated"))
        
        return toVM, nil
    },
)
```

Also add the corresponding `RegisterStoreUpgrades` case (even if empty — proves the pattern works).

### Step 2: Create a Test Migration

Pick one module (e.g., `x/knowledge`) and create a trivial v2→v3 migration that modifies state in a verifiable way:

In `x/knowledge/migrations/v3/migrate.go` (new):

```go
package v3

import (
    "context"
    sdk "github.com/cosmos/cosmos-sdk/types"
)

// MigrateStore performs the v2 → v3 migration for the knowledge module.
// For testing: adds a "migration_marker" key to the store.
func MigrateStore(ctx context.Context, storeService store.KVStoreService) error {
    store := storeService.OpenKVStore(ctx)
    return store.Set([]byte("migration_v3_complete"), []byte("true"))
}
```

Register it in the module's `RegisterServices`:

```go
// Bump ConsensusVersion from 2 to 3
func (am AppModule) ConsensusVersion() uint64 { return 3 }

// In RegisterServices, register the migration
func (am AppModule) RegisterMigrations(cfg module.MigrationRegistrar) {
    // existing v1→v2
    cfg.Register(types.ModuleName, 1, /* v1→v2 migrator */)
    // new v2→v3
    cfg.Register(types.ModuleName, 2, func(ctx context.Context) error {
        return v3.MigrateStore(ctx, am.keeper.storeService)
    })
}
```

### Step 3: Integration Test — Full Upgrade Pipeline

Create `tests/integration/upgrade_test.go`:

```go
func TestGovernanceUpgrade(t *testing.T) {
    // 1. Boot a single-validator in-process chain (use app test helpers)
    // 2. Produce 10 blocks
    // 3. Submit a software-upgrade governance proposal
    //    - Plan name: "v1.0.1-testnet"
    //    - Plan height: current + 20
    // 4. Vote YES from the validator
    // 5. Advance blocks until the plan height
    // 6. Verify: chain halts at plan height (returns upgrade-needed error)
    // 7. Restart the app with the upgrade handler registered
    // 8. Verify: chain continues producing blocks
    // 9. Verify: migration marker exists in store
    // 10. Verify: knowledge module is at ConsensusVersion 3
}
```

If in-process testing is too complex, use the localnet approach:

### Step 3 (Alternative): Localnet Upgrade Test

Add to `scripts/localnet-test.sh` or create `scripts/upgrade-test.sh`:

```bash
test_upgrade_simulation() {
    info "Testing governance-triggered upgrade..."
    
    # Get current height
    HEIGHT=$(curl -s http://localhost:26657/status | jq -r '.result.sync_info.latest_block_height')
    UPGRADE_HEIGHT=$((HEIGHT + 30))
    
    # Submit upgrade proposal
    ./build/zeroned tx upgrade software-upgrade "v1.0.1-testnet" \
        --title "Test Upgrade" \
        --description "Upgrade simulation" \
        --upgrade-height $UPGRADE_HEIGHT \
        --deposit 10000000uzrn \
        --from val0 $TX_FLAGS
    
    # Vote from all validators
    PROPOSAL_ID=$(./build/zeroned query gov proposals --status voting_period $COMMON_FLAGS | jq -r '.proposals[-1].id')
    for i in 0 1 2 3; do
        ./build/zeroned tx gov vote $PROPOSAL_ID yes \
            --from val$i $TX_FLAGS
    done
    
    # Wait for upgrade height
    info "Waiting for upgrade height $UPGRADE_HEIGHT..."
    while true; do
        H=$(curl -s http://localhost:26657/status 2>/dev/null | jq -r '.result.sync_info.latest_block_height' 2>/dev/null || echo "0")
        [ "$H" -ge "$UPGRADE_HEIGHT" ] && break
        sleep 2
    done
    
    # Verify chain halted (nodes should have stopped or be returning errors)
    sleep 5
    STATUS=$(curl -s http://localhost:26657/status 2>/dev/null | jq -r '.result.sync_info.latest_block_height' 2>/dev/null || echo "halted")
    
    if [ "$STATUS" = "halted" ] || [ "$STATUS" = "$UPGRADE_HEIGHT" ]; then
        ok "Chain halted at upgrade height"
    else
        fail "Chain did not halt at upgrade height (got $STATUS)"
        return
    fi
    
    # Rebuild binary (simulating "new version" — same binary since handler is already registered)
    # In production, this would be a new binary. For testing, just restart.
    scripts/localnet.sh stop
    sleep 3
    scripts/localnet.sh start --skip-init  # Resume from existing state
    
    # Verify chain continues
    sleep 10
    NEW_HEIGHT=$(curl -s http://localhost:26657/status | jq -r '.result.sync_info.latest_block_height')
    if [ "$NEW_HEIGHT" -gt "$UPGRADE_HEIGHT" ]; then
        ok "Chain resumed after upgrade (height: $NEW_HEIGHT)"
    else
        fail "Chain did not resume after upgrade"
    fi
    
    # Verify migration marker (query via custom endpoint or state dump)
    # This depends on whether we expose the marker via gRPC query
}
```

**Note:** `scripts/localnet.sh` may need a `--skip-init` flag (or equivalent) to restart validators from existing state without re-initializing genesis. If it doesn't have this, add it.

### Step 4: Verify Migration Ran

The migration marker written in Step 2 needs to be queryable. Options:

1. **Custom gRPC query** — add a `QueryMigrationStatus` endpoint (overkill for a test)
2. **State export** — `zeroned export` and check the JSON
3. **Module version query** — `zeroned query upgrade module_versions` should show knowledge at version 3

Option 3 is simplest:

```bash
./build/zeroned query upgrade module_versions $COMMON_FLAGS | jq '.module_versions[] | select(.name == "knowledge")'
# Expected: { "name": "knowledge", "version": "3" }
```

### Step 5: Test Rollback Safety

Verify that if the upgrade handler is *not* registered (simulating running old binary after upgrade height), the chain refuses to start:

```go
func TestUpgradeHaltsWithoutHandler(t *testing.T) {
    // 1. Advance chain to upgrade height
    // 2. Restart WITHOUT the upgrade handler registered
    // 3. Verify: app refuses to start with "upgrade needed" error
}
```

This validates that the chain can't accidentally continue with an old binary.

## Exit Criteria

1. `v1.0.1-testnet` upgrade handler registered and compiles
2. Knowledge module has v2→v3 migration (trivial marker write)
3. Governance upgrade proposal passes and chain halts at upgrade height
4. Chain resumes after restart with handler registered
5. `zeroned query upgrade module_versions` shows knowledge at version 3
6. Chain refuses to start without upgrade handler (old binary protection)
7. All existing tests still pass (`go test ./...`)

## Commit Convention

```
feat(upgrade): v1.0.1-testnet upgrade handler for migration testing
feat(knowledge): v2→v3 test migration
test(upgrade): governance-triggered upgrade simulation
test(upgrade): rollback safety — chain halts without handler
```

## Cleanup Note

After testnet launch, the test migration (v2→v3 marker write) can be removed or left as documentation. The `v1.0.1-testnet` handler should be kept as a template for real future upgrades.
