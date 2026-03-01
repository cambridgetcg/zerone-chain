# R33-1 — State Machine Simulation (Cosmos SimApp Pattern)

## Objective

Integrate the Cosmos SDK simulation framework to fuzz-test all 32 modules with random message sequences. This is the standard approach used by every production Cosmos chain.

## Background

The Cosmos SDK simulation framework:
- Generates random valid messages for registered module operations
- Executes them in random order over hundreds of blocks
- Checks registered invariants after every operation
- Catches state corruption, panics, and invariant violations that unit tests miss

We already have `tests/simulation/` with economic and adversarial sims, but these use mock keepers. This session adds **full app-level simulation** using the real `ZeroneApp`.

## Tasks

### 1. Create simulation operations for each custom module

For each module in `x/`, create `x/<module>/simulation/operations.go`:

```go
func SimulateMsgSubmitClaim(ak types.AccountKeeper, bk types.BankKeeper, k keeper.Keeper) simtypes.Operation {
    return func(r *rand.Rand, app *baseapp.BaseApp, ctx sdk.Context, accs []simtypes.Account, chainID string) (simtypes.OperationMsg, []simtypes.FutureOperation, error) {
        // Generate random valid claim
        // Return operation message
    }
}
```

Priority modules (most complex state machines):
1. `knowledge` — claim/commit/reveal/round lifecycle
2. `partnerships` — formation/operation/exit
3. `governance` — proposal/vote/execute
4. `capture_defense` — flag/unflag/metric updates
5. `alignment` — observation/correction
6. `vesting_rewards` — minting/distribution

Lower priority (simpler or already covered):
7-32. Other modules — at minimum register a no-op operation so simulation doesn't skip them

### 2. Register weighted operations

Create `app/sim_test.go`:

```go
func TestFullAppSimulation(t *testing.T) {
    config := simcli.NewConfigFromFlags()
    config.NumBlocks = 500
    config.BlockSize = 50
    
    db := dbm.NewMemDB()
    app := NewZeroneApp(...)
    
    _, simParams, simErr := simulation.SimulateFromSeed(
        t, os.Stdout, app.BaseApp, 
        AppStateFn(app.AppCodec(), app.SimulationManager()),
        simtypes.RandomAccounts,
        simcli.SimulationOperations(app, app.AppCodec(), config),
        app.ModuleAccountAddrs(),
        config,
        app.AppCodec(),
    )
    require.NoError(t, simErr)
}
```

### 3. Operation weights

Use realistic weights — more claims than governance proposals, more partnership operations than emergency halts:

| Module | Operation | Weight |
|--------|-----------|--------|
| knowledge | SubmitClaim | 100 |
| knowledge | SubmitCommitment | 80 |
| knowledge | SubmitReveal | 80 |
| partnerships | ProposePartnership | 40 |
| partnerships | AcceptPartnership | 30 |
| governance | SubmitProposal | 10 |
| governance | Vote | 20 |
| capture_defense | (triggered by BeginBlock) | 0 (passive) |
| alignment | (triggered by EndBlock) | 0 (passive) |

### 4. AppStateFn — random genesis generation

Create `app/sim_state.go` that generates random but valid genesis state for all modules. This must:
- Respect parameter bounds (no negative capacities)
- Create consistent cross-module references
- Ensure minimum staking requirements met

### 5. Deterministic simulation

Add a deterministic mode with fixed seed:

```bash
go test -run TestFullAppSimulation -Seed=42 -NumBlocks=500 -BlockSize=50 -v
```

This ensures reproducibility when a failure is found.

### 6. CI integration

Add simulation to CI (nightly, not per-PR — too slow):

```yaml
  simulation:
    name: "Simulation (nightly)"
    runs-on: ubuntu-latest
    timeout-minutes: 60
    if: github.event_name == 'schedule'
    steps:
      - name: Run simulation
        run: go test -run TestFullAppSimulation -Seed=42 -NumBlocks=200 -BlockSize=30 -timeout 45m ./app/...
```

## Acceptance Criteria

- [ ] All 32 modules have at least one simulation operation registered
- [ ] 6 priority modules have comprehensive operation coverage
- [ ] `TestFullAppSimulation` runs 500 blocks without panic or invariant failure
- [ ] Deterministic mode produces identical results with same seed
- [ ] Random genesis generation respects all module constraints
- [ ] CI nightly job configured
