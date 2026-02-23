# R14 Boot Verification Report

**Date:** 2026-02-23
**Binary version:** a85254c
**Chain ID:** zerone-boot-1
**Moniker:** boot-verify

## Verification Results

| Check | Status |
|-------|--------|
| `make build` | PASS |
| `zeroned version` | PASS (a85254c) |
| `zeroned init` | PASS (genesis.json, node_key.json, priv_validator_key.json) |
| Genesis validation | PASS (valid genesis file) |
| Axiom injection | PASS (777 axioms, 16 domains, 0 cycles, max depth 33) |
| Chain start | PASS (all 46 IAVL stores initialized) |
| Block production | PASS (height: 43+, ~2.5s block time) |
| RPC API (26657) | PASS (status, net_info, consensus_state, genesis, validators) |
| REST API (1317) | PARTIAL (API server enabled, but module query endpoints return "Not Implemented") |
| Module CLI queries | FAIL (query/tx subcommands not registered: `query bank`, `tx bank send`, etc.) |
| Transaction | SKIPPED (tx subcommands not available) |
| Graceful shutdown | PASS (clean service stop, no errors) |
| `go vet ./...` | PASS (0 issues) |
| `go test ./...` | PASS (41 packages, 0 failures) |
| `make pr-check` | PASS (lint + test + build) |

## Test Coverage Summary

- Total individual tests: 2,385
- Passing: 2,385
- Failing: 0
- Packages with tests: 41
- Packages with no test files: 125

## Findings

### Critical: Module Query/TX CLI Not Wired

The `zeroned query` and `zeroned tx` commands do not register module subcommands (bank, staking, gov, etc.). This means:

- `zeroned query bank balances <addr>` -> "unknown command"
- `zeroned tx bank send` -> not available
- REST endpoints like `/cosmos/bank/v1beta1/balances/` return state loading errors
- REST endpoints for custom modules return "Not Implemented"

**Root cause:** Module AutoCLI and/or gRPC-gateway registration is missing from `cmd/zeroned/cmd/root.go`. The modules are correctly wired in `app/app.go` (all 30 custom keepers present), but their CLI query servers and REST routes need explicit registration.

**Impact:** Chain runs and produces blocks correctly, but operators cannot query state or submit transactions via CLI or REST. Only raw CometBFT RPC (port 26657) works.

**Recommendation:** Wire module AutoCLI in root command setup, register gRPC query servers in `app.go`, and add gRPC-gateway routes. This is a **P0 blocker** for any testnet or mainnet deployment.

### What Works

- Binary compiles and runs cleanly
- Genesis initialization with all 30+ custom modules
- Validator setup (keys, funding, gentx, collect)
- Axiom DAG injection (777 axioms across 16 domains)
- CometBFT consensus (single validator producing blocks)
- IAVL storage for all module stores
- Graceful shutdown
- Full test suite (2,385 tests, 0 failures)
- `go vet` clean
