# R32-2 Genesis Lifecycle E2E — Design

## Objective

Validate that ZERONE's genesis with all custom modules initializes correctly and exports cleanly on a real running chain via Docker/interchaintest.

## Approach

CLI-based E2E using the existing interchaintest harness (`SetupChain`, `WaitBlocks`, `QueryModule`). All tests in `tests/e2e/genesis_lifecycle_test.go`.

## Tests

### 1. TestGenesis_AllModulesInitialize

- Start single-validator chain via `SetupChain(t, 1)`
- Wait 5 blocks (BeginBlock/EndBlock hooks fire)
- Query params for every custom module via `QueryModule`
- Assert each returns valid JSON (not error)
- Module list: all custom modules registered in the app

### 2. TestGenesis_ExportRoundTrip

- Start chain, wait 20 blocks to accumulate state
- Run `zeroned export` via `chain.GetNode().Exec()` to get genesis JSON
- Parse exported genesis, validate structure:
  - All expected module keys present
  - Module param values match what was set in `testGenesisKV()`
  - Knowledge, alignment, partnerships modules have expected param structure
- Marshal back to JSON, verify re-parseable (structural round-trip)

### 3. TestGenesis_GenesisCheckTool

- After export, run `tools/genesis-check` against the exported genesis file
- Assert exit code 0 (all invariant checks pass)

## Out of Scope

- **Genesis migration test**: No previous version snapshot exists yet
- **Invalid genesis rejection E2E**: Already covered in cross_stack TestScenario16; interchaintest makes negative chain-start tests awkward
- **Second chain boot from export**: Complex with interchaintest validator set handling; structural validation of export sufficient for now

## Existing Coverage (not duplicated)

| Test | Location | What it covers |
|------|----------|---------------|
| TestScenario8_AppBootSmokeTest | cross_stack | In-process boot, export, re-import |
| TestScenario9_R7GenesisRoundTrip | cross_stack | Module genesis JSON round-trip |
| TestScenario16_InvalidGenesisRejection | cross_stack | Bad params rejected |
| TestSmoke_ChainStarts | e2e | Basic chain boot + funding |

## New Value Added

- Tests the **real binary** export path via CLI (not in-process)
- Validates **all 32+ modules** initialize on a real Docker chain
- Runs `tools/genesis-check` against **live-exported** genesis
- Confirms exported genesis is structurally complete and re-parseable
