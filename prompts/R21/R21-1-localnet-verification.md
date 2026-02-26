# R21-1 — Localnet Verification: Run It and Fix What Breaks

## Context

`scripts/localnet.sh` (563 lines) builds and boots a 4-validator localnet. `scripts/localnet-test.sh` (665 lines) runs 8 integration tests against it. Neither has been run against the current binary. This session runs both and fixes whatever breaks.

This is *not* a writing session — it's a debugging session. The scripts exist. The goal is a green run.

## Task

### Step 1: Build

```bash
cd ~/Desktop/zerone
make clean && make build
./build/zeroned version
```

Must succeed. If it doesn't, fix it before proceeding.

### Step 2: Clean State

```bash
scripts/localnet.sh clean 2>/dev/null || true
```

### Step 3: Start Localnet

```bash
scripts/localnet.sh start
```

Watch the output. Common failure modes:

- **Port conflicts** — another process on 26600-26603. Kill it or adjust `BASE_P2P_PORT`.
- **Genesis validation failure** — a module's default genesis is malformed. Fix the module's `DefaultGenesis()` or `ValidateGenesis()`.
- **Missing genesis params** — new params added in R16-R20 that aren't in the localnet genesis template. Update `scripts/localnet.sh`'s genesis patches.
- **App hash mismatch** — determinism bug. All 4 validators must compute identical state. If they diverge, check non-deterministic iteration (maps), floating point, or time-dependent logic.
- **Panic at block 1** — usually a nil pointer in BeginBlock/EndBlock. Check ABCI hooks.

If `localnet.sh` itself needs fixes (wrong CLI flags, missing subcommands, stale API paths), fix the script. Document every fix.

### Step 4: Verify Block Production

```bash
scripts/localnet.sh status
```

All 4 validators should show increasing block heights. If any are stuck:

- Check logs: `scripts/localnet.sh logs N`
- Look for consensus failures, panics, peer connection issues
- If one validator is behind, check if it's a slow catchup vs a consensus fork

### Step 5: Run Integration Tests

```bash
scripts/localnet-test.sh
```

Expected: 8 tests, all PASS. For each failure:

#### `test_block_production`
- Failure = chain isn't running. Go back to Step 3.

#### `test_validator_set`
- Failure = validators not in the active set. Check genesis staking config, min self-delegation, active validator count param.

#### `test_delegation`
- Failure = delegation tx fails. Check gas prices, fee denom, account balances. May need genesis faucet balance adjustment.

#### `test_tier_check`
- Failure = tier assignment wrong for initial stakes. Check `x/staking` tier boundary params vs `VALIDATOR_STAKES` in localnet.sh. The 4 validators have 100/1K/10K/100K ZRN stakes — tier boundaries must accommodate this spread.

#### `test_pot_round` (HIGHEST PRIORITY)
- This is the critical path. A full PoT commit/reveal/verdict cycle. Failure modes:
  - **No round created** — claim submission failed, or PoT module not triggering round creation
  - **Commit phase stuck** — vote extensions not propagating, VRF selection not working
  - **Reveal phase stuck** — committed validators not revealing, timeout too long
  - **Verdict wrong** — consensus math error, quorum not reached
- Debug with: `zeroned query knowledge rounds --status active`, check events, inspect vote extension data in block commits
- This test may need param tuning (shorter round timeouts for localnet)

#### `test_slashing`
- Stops a validator, waits for jail. If timeout: check `signed_blocks_window` and `min_signed_per_window` in genesis. Localnet may need shorter windows.

#### `test_recovery`
- Unjails a validator. If it fails: check if unjail cooldown (`downtime_jail_duration`) is shorter than the test wait time.

#### `test_governance`
- Submits a LIP, votes, checks passage. If vote fails: check voting period duration, deposit requirements, quorum params. Localnet should use very short governance periods (e.g. 30s voting, 10s deposit).

### Step 6: Fix and Re-Run

For every fix:
1. Apply the fix
2. `scripts/localnet.sh clean && scripts/localnet.sh start`
3. Re-run `scripts/localnet-test.sh`
4. Repeat until all 8 pass

### Step 7: Document

Create `docs/localnet.md` (or update if exists):

```markdown
# Running the Local Testnet

## Prerequisites
- Go 1.24+
- jq 1.6+

## Quick Start
scripts/localnet.sh start
scripts/localnet-test.sh
scripts/localnet.sh stop

## Validator Configuration
| Validator | Stake | Expected Tier |
|-----------|-------|---------------|
| val0 | 100 ZRN | Apprentice |
| val1 | 1,000 ZRN | Verified |
| val2 | 10,000 ZRN | Guardian |
| val3 | 100,000 ZRN | Architect |

## Known Issues
(document anything discovered during this session)
```

## Exit Criteria

1. `scripts/localnet.sh start` — 4 validators booting, all producing blocks
2. `scripts/localnet.sh status` — all 4 showing increasing heights
3. `scripts/localnet-test.sh` — 8/8 PASS
4. `test_pot_round` specifically passes (commit → reveal → verdict → fact created)
5. All fixes committed with descriptive messages

## Commit Convention

```
fix(localnet): <what broke and why>
fix(genesis): <param/config fix>
fix(module): <module-level fix discovered during localnet>
docs(localnet): add/update localnet documentation
```
