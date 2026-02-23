# R13-3 — Testnet Genesis: Explicit Params for All Modules + Config Reference

## Context

Zerone's genesis scripts rely on `DefaultParams()` for most modules. The prototype's `testnet-genesis.sh` explicitly set **every parameter** for every module, with a companion `testnet-genesis-config.json` documenting each value.

This matters because:
- `DefaultParams()` are tuned for development, not production
- Any future change to `DefaultParams()` silently alters genesis behaviour
- Explicit params create a permanent, auditable record of genesis decisions
- The config reference JSON documents the *reasoning* behind each value

## Task

### 1. Create `scripts/testnet-genesis.sh`

A comprehensive script that initializes a testnet genesis with ALL module parameters explicitly configured. Subcommands: `init`, `add-validator`, `finalize`, `export`, `verify`.

Use `jq` for all genesis patching (atomic, reproducible):

```bash
patch() {
  jq "$1" "$GENESIS" > "${GENESIS}.tmp" && mv "${GENESIS}.tmp" "$GENESIS"
}
```

#### Constants

```bash
CHAIN_ID="zerone-testnet-1"
DENOM="uzrn"
BINARY="build/zeroned"
TESTNET_HOME="${HOME}/.zeroned/testnet"

# Testnet economics (lower than mainnet)
FOUNDATION_BALANCE="1000000000000"    # 1,000,000 ZRN
RESEARCH_BALANCE="500000000000"       #   500,000 ZRN
FAUCET_BALANCE="100000000000"         #   100,000 ZRN
TEST_BALANCE="10000000000"            #    10,000 ZRN
VALIDATOR_BALANCE="100000000000"      #   100,000 ZRN
VALIDATOR_STAKE="10000000000"         #    10,000 ZRN
```

#### Module Parameters

Patch every module in `cmd_init()`. Use the prototype's testnet-genesis.sh as the reference for parameter values, translating from LGM economics to ZRN.

**CRITICAL:** Use the correct genesis key for each module (see README module name mapping). The key differences from the prototype:
- `legible_auth` → `zerone_auth`
- `legible_staking` → `zerone_staking`
- `lgm_gov` → `zerone_gov`

For each module, read the current `DefaultParams()` in `x/<module>/types/genesis.go` (or `params.go`) to discover all available parameters and their types. Then set each one explicitly in the `jq` patch.

Modules to configure (alphabetical):

1. **alignment** — epoch_length_blocks, max_corrections, sensor enables, override thresholds
2. **autopoiesis** — epoch_length_blocks, max_change_per_epoch_bps, SSI thresholds, enabled
3. **billing** — base_cost_per_fact, confidence curves, freshness window, research share
4. **bvm** — max_bytecode_size, gas costs, max_schedules, current_bvm_version
5. **capture_challenge** — evidence_period, review_period, domain_pause_duration
6. **capture_defense** — stake_lock, min_verifications, accuracy, reputation decay
7. **channels** — min_deposit, max_duration, dispute_period, fraud_penalty
8. **claiming_pot** — max_pots, min_deposit, max_allowlist, min_claim
9. **compute_pool** — share BPS, CU rates, provider requirements, SLA, pricing
10. **discovery** — min_stake, max_capabilities, profile_expiry
11. **disputes** — max_active, escalation, slash/reward rates, tier_configs (3 tiers)
12. **emergency** — halt/revert/resume quorums, voting phases, guardian requirements
13. **evidence_mgmt** — audit_stake, audit_window, accuracy_threshold, oracle params
14. **home** — max_homes, min_creation_stake, session limits, deadman, alert retention
15. **ibcratelimit** — (may have no params or minimal — check DefaultParams)
16. **icaauth** — max_remote_accounts, allowed_host_msg_types, registration_cooldown
17. **knowledge** — ALL 29+ params: PoT lifecycle, thresholds, adversarial verification, challenge params
18. **liquiditypool** — max_pools, fee params
19. **ontology** — min_proposal_stake, voting_period, endorsements, stratum limits
20. **partnerships** — formation/cooling windows, pot shares, freeze limits, coercion review
21. **qualification** — stake tiers, lock period, accuracy, endorsers, decay
22. **research** — min_stakes, review_period, reviewer_count, acceptance score, bounty limits
23. **schedule** — max_active, gas_per_block, interval limits, prepaid, cleanup
24. **tokens** — (check DefaultParams)
25. **toolbox** — demand tracking, pricing params
26. **tree** — min_budget, task limits, contributor caps, revenue split BPS
27. **vesting_rewards** — research share, block_reward, decay, category_configs (10 epistemic curves)
28. **zerone_auth** — session keys, rotation cooldown, recovery delay, bootstrap
29. **zerone_gov** — voting/review/last_call periods, thresholds, disbursement params, LIP stake categories
30. **zerone_staking** — max_validators, unbonding, tier_configs (4 tiers: apprentice→guardian)

Also patch:
- **SDK staking** — bond_denom = "uzrn"
- **SDK slashing** — signed_blocks_window, slash_fraction_downtime
- **Bank denom metadata** — ZRN display units

#### `verify` subcommand

Query params from a running node for each module:
```bash
zeroned query <module> params --output json | jq '.params'
```

### 2. Create `scripts/testnet-genesis-config.json`

A comprehensive JSON reference document with every parameter value and annotations:

```json
{
  "_meta": {
    "description": "Complete parameter reference for Zerone testnet genesis",
    "chain_id": "zerone-testnet-1",
    "generated": "2026-02-23",
    "block_time_ms": 2521,
    "blocks_per_day": 34272,
    "bps_scale": "1000000 = 100%",
    "denom": "uzrn (micro-ZRN, 6 decimals)"
  },
  "consensus": {
    "block_max_gas": "33333333",
    "block_max_bytes": "4194304",
    "vote_extensions_enable_height": "1",
    "_note": "vote_extensions MANDATORY for PoT/VRF validator selection"
  },
  "modules": {
    "alignment": {
      "_note": "Protocol self-correction via 5 dimensional sensors (TDI, DCI, HI, FI, NI)",
      "params": { ... }
    },
    ...
  }
}
```

For each module, include:
- `_note` explaining the module's purpose
- All params with their values
- `[TESTNET]` annotations where testnet values differ from production defaults

### 3. Staking Tier Configs

The 4-tier staking system needs explicit tier configs in genesis. Port from prototype:

```json
[
  {
    "tier": "apprentice",
    "min_stake": "111000",
    "min_verifications": 0,
    "min_accuracy": 0,
    "allowed_categories": ["protocol", "computational", "formal"],
    "reward_multiplier": 100,
    "selection_weight": 100,
    "slash_multiplier_bps": 1500
  },
  {
    "tier": "verified", ... },
  {
    "tier": "bonded", ... },
  {
    "tier": "guardian", ... }
]
```

### 4. Vesting Rewards Category Configs

The epistemic release curves for knowledge categories:

```json
[
  {"category": "axiomatic",     "half_life_blocks": 1111111, "cliff_blocks": 11111, "max_release": 950000},
  {"category": "formal_proof",  "half_life_blocks": 555555,  "cliff_blocks": 5555,  "max_release": 920000},
  ...
]
```

### 5. Dispute Tier Configs

3-tier dispute resolution with escalating requirements:

```json
[
  {"tier": 0, "arbiter_count": 3,  "min_bond": "111000000",  ...},
  {"tier": 1, "arbiter_count": 7,  "min_bond": "555000000",  ...},
  {"tier": 2, "arbiter_count": 15, "min_bond": "1111000000", ...}
]
```

## Reference

- Prototype: `legible_money/scripts/testnet-genesis.sh` (complete — 600+ lines)
- Prototype config ref: `legible_money/scripts/testnet-genesis-config.json`
- Prototype ceremony: `legible_money/scripts/genesis-ceremony.sh`
- Zerone DefaultParams: `x/*/types/genesis.go` or `x/*/types/params.go`
- Zerone localnet params: `scripts/localnet.sh` (partial reference)
- Module name mapping: see R13 README.md

## Verification

```bash
# Full testnet flow
./scripts/testnet-genesis.sh init
./scripts/testnet-genesis.sh add-validator val1
./scripts/testnet-genesis.sh finalize
./scripts/testnet-genesis.sh export

# Validate
zeroned validate-genesis

# Start and verify
zeroned start --home $HOME/.zeroned/testnet --minimum-gas-prices 0.025uzrn &
sleep 10
./scripts/testnet-genesis.sh verify
```
