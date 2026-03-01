# R32-2 — Genesis Lifecycle E2E (CLI-Based)

## Objective

Test the real binary's genesis export/import path end-to-end via Docker. The cross-stack tests (TestScenario8/9/14/15/16) already validate in-process round-trip and invalid rejection. This session tests what they can't: the actual `zeroned export` CLI → JSON file → `zeroned start` pipeline that operators will use in production.

## Non-Goals

- Detailed module-state field comparison (covered by cross_stack tests)
- Invalid genesis rejection (covered by TestScenario16)
- In-process genesis manipulation (covered by existing tests)

## Tasks

### 1. CLI export round-trip

The core test — the thing that catches real production bugs:

```go
func TestGenesis_CLIExportImportRoundTrip(t *testing.T) {
    chain := SetupChain(t, 1)
    ctx := context.Background()
    
    // Run for 50 blocks to accumulate real state across modules
    WaitBlocks(chain, ctx, 50)
    
    // Submit some transactions to populate state:
    // - A knowledge claim (creates knowledge + verification state)
    // - A delegation (creates staking state)
    // - A governance proposal (creates gov state)
    PopulateChainState(chain, ctx)
    WaitBlocks(chain, ctx, 20)
    
    // Export genesis via the real CLI binary
    exported := ExecExportGenesis(chain, ctx)  // runs `zeroned export` inside Docker
    
    // Start a fresh chain from the exported genesis
    chain2 := SetupChainFromGenesis(t, exported)
    WaitBlocks(chain2, ctx, 10)
    
    // Verify chain2 is functional — produces blocks, serves queries
    height, err := chain2.Height(ctx)
    require.NoError(t, err)
    require.Greater(t, height, int64(0))
    
    // Verify key state survived the round-trip:
    // - Knowledge fact still queryable
    // - Delegation still exists
    // - Governance proposal still present
    VerifyStatePresent(chain2, ctx)
}
```

### 2. All 32 modules initialize from genesis

```go
func TestGenesis_AllModulesQueryable(t *testing.T) {
    chain := SetupChain(t, 1)
    ctx := context.Background()
    
    // Wait for BeginBlock/EndBlock hooks to all fire at least once
    WaitBlocks(chain, ctx, 5)
    
    // Query params for every custom module via gRPC
    for _, mod := range AllCustomModules() {
        result := QueryParams(chain, ctx, mod)
        require.NotNil(t, result, "module %s must have queryable params after genesis", mod)
    }
}
```

### 3. Export determinism

```go
func TestGenesis_ExportDeterministic(t *testing.T) {
    chain := SetupChain(t, 1)
    ctx := context.Background()
    WaitBlocks(chain, ctx, 30)
    
    // Export twice at the same height
    export1 := ExecExportGenesis(chain, ctx)
    export2 := ExecExportGenesis(chain, ctx)
    
    // Must be byte-identical
    require.Equal(t, export1, export2, "genesis export must be deterministic")
}
```

### 4. Export after upgrade (placeholder)

Wire this to R34-2 (upgrade path) — after a Cosmovisor binary swap, verify `zeroned export` still works with the new binary. Just create the test skeleton here; R34-2 fills in the upgrade logic.

## Acceptance Criteria

- [ ] `zeroned export` → new chain → produces blocks (the real CLI path)
- [ ] All 32 modules queryable after genesis init
- [ ] Exported genesis is deterministic (same height → same bytes)
- [ ] Key state (facts, delegations, proposals) survives round-trip
- [ ] Test completes in < 3 minutes
