# R24-3 — Join Testnet Report

**Date:** 2026-02-26
**Localnet:** zerone-localnet, 4 validators, height ~380 at start
**Binary:** build/zeroned (commit 0d125f5)
**Final state:** 5-validator consensus, external node synced + signing blocks

---

## Summary

An external node was successfully joined to the running localnet, synced, served
queries, and was promoted to validator (both zerone_staking and SDK staking).
The 5-validator set achieved full consensus. **7 bugs found** across scripts and
documentation, **9 documentation discrepancies** in VALIDATOR-GUIDE.md.

---

## Step 1: Simulate External Node Environment

**Status:** PASS
**Time:** <1 min
**Doc Match:** N/A

Created isolated temp directory. Copied genesis from coordinator, extracted peer
info. No issues.

**Finding:** Chain ID is `zerone-localnet` but `join-testnet.sh` hardcodes
`zerone-testnet-1`. External operators joining any network other than the
hardcoded testnet cannot use the script as-is.

---

## Step 2: Try the Automated Path (join-testnet.sh)

**Status:** FAIL
**Time:** ~10 min (debugging)
**Doc Match:** VALIDATOR-GUIDE Section "Option A" references script correctly

### Bug 1: `read_seeds()` crashes on empty/comment-only seeds.txt (CRITICAL)

**File:** `scripts/join-testnet.sh:100`

The pipeline `grep -v '^\s*#' seeds.txt | grep -v '^\s*$' | tr '\n' ',' | sed 's/,$//'`
returns non-zero when seeds.txt has only comments (which is the current state).
With `set -euo pipefail`, this kills the script silently after printing the
banner.

**Fix:**
```bash
SEEDS="$(grep -v '^\s*#' "${SEEDS_FILE}" | grep -v '^\s*$' | tr '\n' ',' | sed 's/,$//' || true)"
```

### Bug 2: Hardcoded chain-id `zerone-testnet-1`

**File:** `scripts/join-testnet.sh:29`

The script hardcodes `CHAIN_ID="zerone-testnet-1"`. If the genesis file has a
different chain-id, the node is initialized with the wrong chain-id and then the
genesis is copied over, creating a mismatch between config and genesis.

**Fix:** Add a `--chain-id` flag, or read chain-id from the genesis file:
```bash
if [[ -n "${GENESIS_URL}" ]] && [[ -f "${GENESIS_URL}" ]]; then
    CHAIN_ID=$(jq -r '.chain_id' "${GENESIS_URL}")
fi
```

### Bug 3: `validate-genesis` command doesn't exist

**File:** `scripts/join-testnet.sh:150-153`

The script calls `zeroned validate-genesis` which is not a registered command.
The correct command is `zeroned genesis validate`. Note: `genesis validate`
accepts `validate-genesis` as an alias, but only as a subcommand of `genesis`.

**Fix:**
```bash
zeroned genesis validate --home "${ZERONED_HOME}" 2>/dev/null || die "Genesis validation failed"
```

### Bug 4: configure-node.sh macOS sed incompatibility

**File:** `scripts/configure-node.sh:200,208`

The ranged sed pattern `/^\[api\]/,/^\[/{s/^enable = .*/enable = true/}` fails
on macOS BSD sed with "bad flag in substitute command". This means `--enable-api`
and `--enable-grpc` flags silently fail on macOS.

**Fix:** Use a multi-line compatible approach or use the `sedi` helper with
simpler patterns, e.g.:
```bash
# Replace with awk or python for section-aware editing
python3 -c "..." # or use separate sed commands with line numbers
```

---

## Step 3: Configure Peers and Start External Node

**Status:** PASS
**Time:** ~2 min
**Doc Match:** Manual steps in guide Section "Option B" mostly correct

### Configuration Applied

- Peers: all 4 localnet validators configured as persistent_peers
- Ports: P2P=26650, RPC=26651, gRPC=9094, API=1321 (no conflicts)
- `addr_book_strict = false` and `allow_duplicate_ip = true` (needed for localhost)

### Sync Results

- Node started cleanly, no panics
- Connected to 4 peers immediately
- Block sync completed in ~10 seconds (476 blocks)
- Switched to consensus reactor at height 477
- **No app hash mismatches** — state determinism confirmed

**Finding:** The VALIDATOR-GUIDE does not mention that external nodes need unique
ports when running on the same machine, nor does it mention `addr_book_strict`
or `allow_duplicate_ip` for localhost testing. This is expected for production
but a stumbling block for local testing.

---

## Step 4: Query from External Node

**Status:** PASS
**Time:** <1 min
**Doc Match:** Monitoring section mostly correct

### Query Results

| Query | External Node | Localnet Val0 | Match |
|-------|---------------|---------------|-------|
| Bank balance (val0) | 99,499,287,548 uzrn | 99,499,287,548 uzrn | YES |
| zerone_staking validators | 2 | 2 | YES |
| Knowledge params | commit=10, reveal=10, agg=5 | Same | YES |
| REST API (bank) | Works on :1321 | Works on :1317 | YES |

### Issues Found

- REST endpoint `/cosmos/base/tendermint/v1beta1/node_info` returns "Not
  Implemented" — the custom binary does not register the CometBFT service gRPC
  gateway. Standard bank/staking REST endpoints work fine.

---

## Step 5: Promote External Node to Validator

**Status:** PASS
**Time:** ~3 min
**Doc Match:** VALIDATOR-GUIDE has multiple discrepancies (see below)

### Process

1. Created key `ext-validator` on external node
2. Funded 500 ZRN from val3 (val0 only had ~99 ZRN)
3. Registered in zerone_staking: `register-validator` with JSON pubkey, raw
   integer self-delegation → Apprentice tier
4. Created SDK staking validator via `tx staking create-validator` with
   validator.json → Joined CometBFT consensus

### Results

- zerone_staking: 3 validators (was 2), external validator as Apprentice
- CometBFT: 5 validators (was 4), external validator power=100000
- Block 578: all 5 validators signing
- No consensus issues with 5-validator set

### Bug 5: Dual registration required (ARCHITECTURE)

An external validator must register in **both** zerone_staking (for PoT
participation) and SDK staking (for CometBFT consensus). The VALIDATOR-GUIDE
only documents zerone_staking registration. An operator following the guide
would have a PoT-registered validator that never signs blocks.

**Fix:** Document the dual registration requirement clearly. Either:
- Add SDK `create-validator` step to the guide, or
- Implement automatic SDK validator creation when registering in zerone_staking

### Bug 6: Self-delegation format in VALIDATOR-GUIDE

**Guide says:** `111000uzrn` (with denomination)
**Reality:** `register-validator` expects a raw integer (`111000`), not a coin
string. Passing `111000uzrn` returns "invalid self delegation amount".

### Bug 7: Consensus pubkey format inconsistency

When registering via CLI with JSON pubkey from `comet show-validator`, the
pubkey is stored as JSON in zerone_staking:
```
consensus_pubkey: '{"@type":"/cosmos.crypto.ed25519.PubKey","key":"CtOW..."}'
```
But genesis validators have hex-encoded pubkeys:
```
consensus_pubkey: 02fb5a4619b9d3ccc929d2e75165ebe03a1ffae6094bf20a18d194637ecb938b14
```
This cosmetic inconsistency may cause issues with tooling that expects a
consistent format.

---

## Step 6: Test Cosmovisor Setup

**Status:** PASS (manual)
**Time:** ~2 min
**Doc Match:** Cosmovisor section in guide is correct

### Results

- Directory structure `cosmovisor/genesis/bin/` created correctly
- Binary copied successfully
- Environment file generated correctly
- Cosmovisor binary not installed (expected — needs `go install`)

**Note:** The join-testnet.sh `--cosmovisor` flag cannot be tested because the
script crashes at `read_seeds()` (Bug 1). Manual setup works.

---

## Step 7: Validate VALIDATOR-GUIDE.md

### Discrepancy 1: Go version requirement

**Line 32:** Guide says "Go 1.22+"
**Reality:** `go.mod` requires Go 1.24.0. Building with Go 1.22 would fail.
**Fix:** Change to "Go 1.24+"

### Discrepancy 2: `validate-genesis` command

**Line 98:** `zeroned validate-genesis`
**Reality:** Command is `zeroned genesis validate`
**Fix:** `zeroned genesis validate`

### Discrepancy 3: Register-account module name

**Line 160:** `zeroned tx auth register-account`
**Reality:** Module is `zerone_auth`, not `auth`. Correct command:
`zeroned tx zerone_auth register-account`

### Discrepancy 4: Register-validator module name

**Line 183:** `zeroned tx staking register-validator`
**Reality:** Module is `zerone_staking`, not `staking`. Correct command:
`zeroned tx zerone_staking register-validator`

### Discrepancy 5: Register-validator argument format

**Line 185:** `111000uzrn` (self-delegation with denom)
**Reality:** Expects raw integer `111000` without denomination.

### Discrepancy 6: Register-validator `--commission` flag

**Line 186:** `--commission 500`
**Reality:** Flag is `--commission 500` — this is actually correct! The CLI
accepts BPS. However, the guide uses positional args format that doesn't match
the actual CLI: `[pubkey-hex] [self-delegation]` are positional, not
`[consensus-pubkey]`.

### Discrepancy 7: Missing SDK staking registration step

The guide only covers zerone_staking registration. For CometBFT consensus
participation, operators also need `tx staking create-validator` with a
validator.json file. This critical step is completely missing.

### Discrepancy 8: Update-stake command module

**Line 267:** `zeroned tx staking update-stake`
**Reality:** `zeroned tx zerone_staking update-stake`

### Discrepancy 9: Knowledge module phase durations

**Line 284-286:** "Commit phase (4 blocks)", "Reveal phase (4 blocks)",
"Aggregation phase (3 blocks)"
**Reality:** commit_phase_blocks=10, reveal_phase_blocks=10,
aggregation_phase_blocks=5 (from on-chain params). The guide values don't match
the genesis defaults.

### Discrepancy 10: Monitoring status query

**Line 376-379:** `zeroned status | jq '.sync_info'` and
`zeroned query staking validators`
**Reality:** `zeroned status` works correctly with `jq '.sync_info'`.
`zeroned query staking validators` shows SDK validators (genesis set), not
zerone_staking validators. Guide should mention both query paths.

---

## Bug Summary

| # | Severity | Component | Description |
|---|----------|-----------|-------------|
| 1 | CRITICAL | join-testnet.sh | `read_seeds()` crashes on comment-only seeds.txt |
| 2 | HIGH | join-testnet.sh | Hardcoded chain-id prevents use with any other network |
| 3 | MEDIUM | join-testnet.sh | `validate-genesis` command doesn't exist |
| 4 | MEDIUM | configure-node.sh | macOS sed fails for API/gRPC section editing |
| 5 | HIGH | Architecture/Docs | Dual registration (zerone_staking + SDK staking) not documented |
| 6 | MEDIUM | Docs | Self-delegation format: `111000uzrn` should be `111000` |
| 7 | LOW | zerone_staking | Consensus pubkey stored as JSON vs hex inconsistency |

## Documentation Discrepancies

| # | Section | Issue |
|---|---------|-------|
| 1 | Prerequisites | Go 1.22+ should be Go 1.24+ |
| 2 | Manual Setup §2 | `validate-genesis` → `genesis validate` |
| 3 | Becoming a Validator §3 | `tx auth` → `tx zerone_auth` |
| 4 | Becoming a Validator §4 | `tx staking` → `tx zerone_staking` |
| 5 | Becoming a Validator §4 | Self-delegation `111000uzrn` → `111000` |
| 6 | Becoming a Validator | Missing SDK staking create-validator step |
| 7 | Tier Progression | `tx staking update-stake` → `tx zerone_staking update-stake` |
| 8 | PoT Participation | Phase durations 4/4/3 → 10/10/5 |
| 9 | Monitoring | Missing zerone_staking query path |

---

## Exit Criteria Checklist

- [x] External node joins running localnet and syncs
- [x] External node serves queries matching localnet state
- [x] External node promoted to validator and signs blocks
- [x] join-testnet.sh tested (FAIL — bugs documented)
- [x] VALIDATOR-GUIDE accuracy validated (9 discrepancies found)
- [x] Every documentation discrepancy recorded
- [x] Report written to `docs/join-testnet-report.md`
