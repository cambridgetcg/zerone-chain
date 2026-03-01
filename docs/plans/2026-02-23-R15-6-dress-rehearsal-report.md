# Testnet Dress Rehearsal Report

**Date:** 2026-02-23
**Network:** zerone-testnet-1
**Validators:** 4
**Binary version:** c92af33

## Pipeline

| Step | Status | Notes |
|------|--------|-------|
| Clean build | :white_check_mark: | `make clean && make build` — binary at `./build/zeroned` |
| Genesis ceremony | :white_check_mark: | 4 validators, 100K ZRN each (10K staked) |
| Axiom injection (777) | :white_check_mark: | 777 axioms loaded via axiom-loader |
| Genesis validation | :white_check_mark: | `zeroned genesis validate-genesis` passed (after fixing `genesis` subcommand namespace) |
| Node configuration (4x) | :white_check_mark: | Unique ports per node, persistent_peers wired, localhost networking configured |
| Chain start (4 validators) | :white_check_mark: | All 4 validators online and signing |
| Block production | :white_check_mark: | Height reached: 141 |
| 100 blocks | :white_check_mark: | 271 seconds (~2.71s/block, target 2.521s) |
| REST API | :warning: | Bank queries work; custom module REST endpoints did not respond |
| Module params queryable | :white_check_mark: | knowledge, staking params verified via CLI |
| Validator set correct | :white_check_mark: | Count: 4 |
| Axioms in knowledge | :white_check_mark: | 777 axioms confirmed in genesis state |
| LIP submitted | :x: | BLOCKED — `zerone-gov` CLI tx/query commands not registered |
| LIP voted | :x: | BLOCKED — same as above |
| LIP passed | :x: | BLOCKED — same as above |
| Param change applied | :x: | BLOCKED — no governance CLI |
| Bank transfer | :x: | BLOCKED — `tx bank send` not registered in CLI |
| Graceful shutdown | :white_check_mark: | All 4 nodes stopped cleanly at height 141 |
| `go test ./...` | :white_check_mark: | 2551 tests, 0 failures |
| `make pr-check` | :white_check_mark: | `go vet` + `go test` + `go build` all green |

## Script Fixes Applied

Three systematic fixes to `testnet-genesis.sh` and `genesis-ceremony.sh`:

1. **validate_genesis()**: Changed `${BINARY} validate` to `${BINARY} genesis validate-genesis` (Cosmos SDK v0.50 moved command under `genesis` subcommand)
2. **gentx**: Changed `${BINARY} gentx` to `${BINARY} genesis gentx`
3. **collect-gentxs**: Changed `${BINARY} collect-gentxs` to `${BINARY} genesis collect-gentxs`

Additional fix:
4. **disputes tier_configs**: Added missing `"tier":N` field to each tier config object (validation required tier 1-4, got 0)

## Test Fix Applied

- **`TestAnteIntegration_BootstrapGasFreeAtHeight1`**: Updated test to expect `BlockGasLimit` (33,333,333) instead of `math.MaxUint64`. The `BootstrapGasFreeDecorator` intentionally uses `BlockGasLimit` because CometBFT's mempool rejects txs with `gas_wanted` exceeding `ConsensusParams.Block.MaxGas`.

## Test Coverage

- Total test functions: 2551
- Passing: 2551
- Failing: 0

## Findings

### P0 — Must fix before testnet

1. **CLI module registration incomplete**: Only `knowledge` and `zerone_staking` tx/query commands are registered. Standard Cosmos SDK modules (`bank`, `gov`, `staking`, `distribution`) and zerone custom modules (`zerone-gov`, `emergency`, etc.) are missing from CLI. This blocks governance, bank transfers, and most operational CLI commands.

### P1 — Should fix

2. **Custom module REST endpoints not responding**: While bank REST queries work, custom module endpoints (knowledge, staking, etc.) did not respond on the REST API. May be related to module registration.

3. **Block time slightly above target**: Observed ~2.71s/block vs 2.521s target. This is expected for a 4-validator localhost setup and should normalize on a real network.

## Verdict

**BLOCKED** — CLI module registration is incomplete. Bank transfers and governance operations cannot be performed via CLI. The chain itself produces blocks correctly with 4 validators, axioms load, and all 2551 unit/integration tests pass. The genesis ceremony scripts work after the subcommand namespace fixes. Once CLI registration is resolved, the pipeline is ready for testnet.
