# R32-2 — Genesis Lifecycle E2E

## Objective

Validate that ZERONE's genesis with all 32 custom modules initializes correctly, exports cleanly, and re-imports without data loss — on a real running chain.

## Tasks

### 1. Full genesis validation test

```go
func TestGenesis_AllModulesInitialize(t *testing.T) {
    chain := SetupChain(t, 1)
    ctx := context.Background()
    
    // Wait for 5 blocks to ensure all BeginBlock/EndBlock hooks run
    WaitBlocks(chain, ctx, 5)
    
    // Query each custom module's params to verify initialization
    modules := []string{
        "knowledge", "alignment", "capture_defense", "capture_challenge",
        "partnerships", "discovery", "home", "qualification", "ontology",
        "vesting_rewards", "pacing", "autopoiesis", "schedule",
        // ... all 32
    }
    for _, mod := range modules {
        params := QueryParams(chain, ctx, mod)
        require.NotNil(t, params, "module %s params must be queryable", mod)
    }
}
```

### 2. Genesis export/import round-trip

```go
func TestGenesis_ExportImportRoundTrip(t *testing.T) {
    chain := SetupChain(t, 1)
    ctx := context.Background()
    
    // Run for 20 blocks to accumulate some state
    WaitBlocks(chain, ctx, 20)
    
    // Export genesis
    exported := ExportGenesis(chain, ctx)
    
    // Start a new chain from the exported genesis
    chain2 := SetupChainFromGenesis(t, exported)
    WaitBlocks(chain2, ctx, 5)
    
    // Export again and compare
    reexported := ExportGenesis(chain2, ctx)
    
    // Compare module states (ignoring block-height-dependent fields)
    CompareGenesisModules(t, exported, reexported, []string{
        "knowledge", "alignment", "partnerships", "vesting_rewards",
    })
}
```

### 3. Genesis migration test

Test that `zeroned genesis migrate` works correctly for future upgrades:

```go
func TestGenesis_MigrateFromPreviousVersion(t *testing.T) {
    // Load a saved genesis snapshot from testdata/
    oldGenesis := LoadTestGenesis(t, "testdata/genesis_v0.1.0.json")
    
    // Run migration
    migrated := MigrateGenesis(chain, ctx, oldGenesis)
    
    // Verify migrated genesis starts a chain
    chain := SetupChainFromGenesis(t, migrated)
    WaitBlocks(chain, ctx, 5)
}
```

### 4. Invalid genesis rejection

Test that malformed genesis configurations are rejected at init:

- Missing required module genesis
- Invalid param values (negative capacities, zero voting period)
- Inconsistent cross-module references

## Acceptance Criteria

- [ ] Chain starts with all 32 modules and produces blocks
- [ ] Genesis export after 20 blocks re-imports cleanly
- [ ] Re-exported genesis matches original (module-state level)
- [ ] Invalid genesis configurations fail fast with clear errors
- [ ] `tools/genesis-check` passes on all generated genesis files
