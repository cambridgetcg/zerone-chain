# R9-2 — Genesis Validation + Ante Integration Tests + Boot Fix

## Goal

Comprehensive genesis validation for all 32 modules, full round-trip testing, ante handler
integration tests, and fix the boot-test script so it works on macOS.

## Working Directory

`/Users/yuai/Desktop/Zerone`

## Part 1: Genesis Validation

### Tests

1. **DefaultGenesis for all modules** — every module's `DefaultGenesis()` passes `ValidateGenesis()`
2. **Full genesis round-trip** — init → export → reimport → export → compare (byte-identical)
3. **Genesis with all modules populated** — non-default state in every module, validate, round-trip
4. **Invalid genesis rejection** — bad params (BPS > 1M, negative values, zero slash) rejected
5. **Token supply consistency** — sum of all genesis balances == total supply in bank genesis
6. **Module account permissions** — all module accounts present with correct permissions

### Implementation

Extend `tests/cross_stack/genesis_roundtrip_test.go` or create `tests/genesis/`:
```
tests/genesis/
├── default_test.go      (DefaultGenesis for all 32 modules)
├── roundtrip_test.go    (export/import round-trip)
├── validation_test.go   (invalid genesis rejection)
└── supply_test.go       (token supply consistency)
```

## Part 2: Ante Handler Integration Tests

### Reference

- `/Users/yuai/Desktop/legible_money/app/ante_integration_test.go` — 1604 LOC, comprehensive

### Tests to Port / Create

1. **Emergency halt blocks normal txs** — send MsgSend during halt → rejected
2. **Emergency halt allows emergency txs** — MsgProposeResume during halt → accepted
3. **Frozen account blocked** — frozen account tries MsgSend → rejected
4. **Frozen account unfreeze + retry** — unfreeze then MsgSend → accepted
5. **Session key capability enforcement** — session key with "transfer_only" tries MsgVote → rejected
6. **Session key within capability** — session key with "transfer_only" tries MsgSend → accepted
7. **Bootstrap gas-free for PoT** — MsgSubmitClaim at height 1 → no gas charged
8. **Bootstrap gas-free expires** — MsgSubmitClaim after bootstrap → normal gas
9. **Gas overflow protection** — tx with 1000 messages → saturating addition, no panic
10. **Fee router denomination check** — fee in wrong denom → rejected
11. **ZRN gas table coverage** — every registered message type has correct gas cost
12. **DID resolution from memo** — tx with DID in memo → context annotated

### Implementation

```
app/ante_integration_test.go   (new file, port from draft)
```

These tests need a running app instance (use the in-process test app pattern).

## Part 3: Boot Test Fix

The `scripts/boot-test.sh` script fails because it uses `zeroned genesis add-genesis-account`
but the binary uses `zeroned add-genesis-account` (Cosmos SDK v0.50 direct subcommand style).

Fix the script to use the correct command syntax. Also replace `timeout` (not available on
macOS by default) with a background process + sleep + kill pattern.

Verify the fixed script runs end-to-end:
```bash
bash scripts/boot-test.sh
```

## Verification

```bash
# All tests pass
go test ./tests/... ./app/...

# Full suite still green
go test ./...

# Boot test works
bash scripts/boot-test.sh
```

## Constraints

- Genesis round-trip must be byte-identical (not just structurally equivalent)
- Ante tests must use the real ante handler chain (not individual decorators)
- Boot test must work on macOS (no GNU-only tools)
- Every test must clean up after itself (no leaked state)
