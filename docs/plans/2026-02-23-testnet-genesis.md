# R13-3 Testnet Genesis Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Create `scripts/testnet-genesis.sh` that explicitly sets ALL module parameters for 30 custom modules (no reliance on `DefaultParams()`) and a companion `scripts/testnet-genesis-config.json` reference document.

**Architecture:** Port the prototype's `legible-money/scripts/testnet-genesis.sh` pattern (863 lines) to Zerone, translating LGM→ZRN economics and using Zerone's module names (`zerone_auth`, `zerone_gov`, `zerone_staking`). Use `jq` for atomic genesis patching. Follow the existing `scripts/genesis-ceremony.sh` and `scripts/localnet.sh` code style exactly.

**Tech Stack:** Bash, jq, zeroned CLI

---

## Key Conventions (from existing codebase)

- **JSON field names are snake_case** (amino encoding, matching proto field names) — confirmed by working `localnet.sh` and `genesis-ceremony.sh`
- **BPS scale:** 1,000,000 = 100%
- **Block time:** 2521ms → ~34,272 blocks/day
- **Denom:** `uzrn` (micro-ZRN, 6 decimals)
- **Module genesis keys** use the mapping from `prompts/R13/README.md`:
  - `x/auth` → `zerone_auth`
  - `x/gov` → `zerone_gov`
  - `x/staking` → `zerone_staking`
  - All others → directory name (e.g., `x/knowledge` → `knowledge`)

## Module Parameter Sources

For each module, the plan uses:
1. **Zerone's actual DefaultParams()** — discovered from `x/*/types/genesis.go` and `x/*/types/types.go`
2. **Prototype testnet values** — from `legible-money/scripts/testnet-genesis.sh` (translated LGM→ZRN)
3. **Testnet overrides** — marked `[TESTNET]` where values differ from Zerone defaults for faster iteration

---

### Task 1: Create testnet-genesis.sh scaffold with constants and helpers

**Files:**
- Create: `scripts/testnet-genesis.sh`

**Step 1: Write the script scaffold**

Create `scripts/testnet-genesis.sh` with:
- Shebang, header comment block (matching `genesis-ceremony.sh` style)
- `set -euo pipefail`
- Constants: `CHAIN_ID="zerone-testnet-1"`, `DENOM="uzrn"`, `BINARY`, `TESTNET_HOME`, `GENESIS`, `PROJECT_ROOT`
- Testnet economics constants (from spec):
  ```bash
  FOUNDATION_BALANCE="1000000000000"    # 1,000,000 ZRN
  RESEARCH_BALANCE="500000000000"       #   500,000 ZRN
  FAUCET_BALANCE="100000000000"         #   100,000 ZRN
  TEST_BALANCE="10000000000"            #    10,000 ZRN
  VALIDATOR_BALANCE="100000000000"      #   100,000 ZRN
  VALIDATOR_STAKE="10000000000"         #    10,000 ZRN
  ```
- Helper functions: `die()`, `info()`, `ok()`, `warn()`, `check_deps()`, `patch()` (atomic jq), `validate_genesis()`
- Main `case` dispatch for subcommands: `init`, `add-validator`, `finalize`, `export`, `verify`, `help`
- Empty function stubs for each subcommand
- `chmod +x`

**Step 2: Verify scaffold runs**

Run: `bash scripts/testnet-genesis.sh help`
Expected: Usage message printed without errors

**Step 3: Commit**

```bash
git add scripts/testnet-genesis.sh
git commit -m "feat(R13-3): scaffold testnet-genesis.sh with constants and subcommands"
```

---

### Task 2: Implement cmd_init — consensus + SDK module patches

**Files:**
- Modify: `scripts/testnet-genesis.sh`

**Step 1: Write cmd_init header + initialization**

In `cmd_init()`, implement:
1. Banner display (chain ID, denom, block time)
2. `check_deps`
3. Clean previous state (`rm -rf "${TESTNET_HOME}"`)
4. `${BINARY} init testnet-coordinator --chain-id "${CHAIN_ID}" --default-denom "${DENOM}" --home "${TESTNET_HOME}" 2>/dev/null`

**Step 2: Add consensus parameter patches**

```bash
patch '
  .consensus.params.block.max_gas = "33333333" |
  .consensus.params.block.max_bytes = "4194304" |
  .consensus.params.abci.vote_extensions_enable_height = "1"
'
```

**Step 3: Add SDK module overrides**

```bash
# SDK staking — bond_denom
patch '
  .app_state.staking.params.bond_denom = "uzrn"
'

# SDK slashing
patch '
  .app_state.slashing.params.signed_blocks_window = "100" |
  .app_state.slashing.params.slash_fraction_downtime = "0.010000000000000000"
'
```

**Step 4: Add bank denom metadata**

```bash
patch '
  .app_state.bank.denom_metadata = [{
    "description": "Zerone - the currency of verified truth",
    "denom_units": [
      {"denom": "uzrn",  "exponent": 0, "aliases": ["microzerone"]},
      {"denom": "mzrn",  "exponent": 3, "aliases": ["millizerone"]},
      {"denom": "zrn",   "exponent": 6, "aliases": ["zerone"]}
    ],
    "base": "uzrn",
    "display": "zrn",
    "name": "Zerone",
    "symbol": "ZRN"
  }]
'
```

**Step 5: Commit**

```bash
git add scripts/testnet-genesis.sh
git commit -m "feat(R13-3): add consensus, SDK, and denom patches to testnet-genesis init"
```

---

### Task 3: Implement cmd_init — alignment through capture_defense module patches

**Files:**
- Modify: `scripts/testnet-genesis.sh`

**Step 1: Add alignment module patch**

All params from Zerone's `x/alignment/types/genesis.go` DefaultParams(), with testnet values ported from prototype:

```bash
info "alignment..."
patch '
  .app_state.alignment.params.observation_interval_blocks = 1111 |
  .app_state.alignment.params.weight_knowledge_quality = 200000 |
  .app_state.alignment.params.weight_economic_stability = 200000 |
  .app_state.alignment.params.weight_governance_participation = 200000 |
  .app_state.alignment.params.weight_network_security = 200000 |
  .app_state.alignment.params.weight_staking_ratio = 200000 |
  .app_state.alignment.params.critical_threshold = 200000 |
  .app_state.alignment.params.degraded_threshold = 400000 |
  .app_state.alignment.params.healthy_threshold = 700000 |
  .app_state.alignment.params.enabled = true
'
```

**Step 2: Add autopoiesis module patch**

All params from `x/autopoiesis/types/genesis.go`:

```bash
info "autopoiesis..."
patch '
  .app_state.autopoiesis.params.epoch_length_blocks = 1111 |
  .app_state.autopoiesis.params.max_change_per_epoch_bps = 50000 |
  .app_state.autopoiesis.params.slash_multiplier_min = 250000 |
  .app_state.autopoiesis.params.slash_multiplier_max = 4000000 |
  .app_state.autopoiesis.params.ssi_critical_threshold = 800000 |
  .app_state.autopoiesis.params.ssi_stressed_threshold = 500000 |
  .app_state.autopoiesis.params.ssi_healthy_threshold = 750000 |
  .app_state.autopoiesis.params.enabled = true
'
```

**Step 3: Add billing module patch**

All params from `x/billing/types/types.go` (Zerone has different params than prototype — use Zerone's actual fields):

```bash
info "billing..."
patch '
  .app_state.billing.params.base_query_price = "1000000" |
  .app_state.billing.params.confidence_weight_bps = 200000 |
  .app_state.billing.params.novelty_weight_bps = 100000 |
  .app_state.billing.params.freshness_weight_bps = 100000 |
  .app_state.billing.params.revenue_split = {"contributor_bps": 550000, "protocol_bps": 220000, "research_bps": 130000, "burn_bps": 100000} |
  .app_state.billing.params.dynamic_pricing = {"enabled": false, "target_query_cost_usd": "0", "manual_zrn_price_usd": "0", "twap_window_blocks": 1000, "staleness_blocks": 100, "min_cost_per_fact": "100000", "max_cost_per_fact": "100000000"} |
  .app_state.billing.params.min_provider_stake = "100000000" |
  .app_state.billing.params.confidence_threshold = 500000 |
  .app_state.billing.params.freshness_window_blocks = 1000 |
  .app_state.billing.params.quote_validity_blocks = 100
'
```

**Step 4: Add bvm module patch**

All params from `x/bvm/types/genesis.go`:

```bash
info "bvm..."
patch '
  .app_state.bvm.params.max_bytecode_size = 65536 |
  .app_state.bvm.params.max_gas_per_call = 10000000 |
  .app_state.bvm.params.max_gas_per_block = 100000000 |
  .app_state.bvm.params.max_contracts_per_creator = 100 |
  .app_state.bvm.params.max_state_entries = 100000 |
  .app_state.bvm.params.deploy_cost = "5000000" |
  .app_state.bvm.params.max_schedule_gas = 1000000 |
  .app_state.bvm.params.schedule_horizon_blocks = 100000 |
  .app_state.bvm.params.current_bvm_version = 1 |
  .app_state.bvm.params.max_schedules_per_contract = 100
'
```

**Step 5: Add capture_challenge module patch**

All params from `x/capture_challenge/types/types.go`:

```bash
info "capture_challenge..."
patch '
  .app_state.capture_challenge.params.min_challenge_stake = "10000000" |
  .app_state.capture_challenge.params.evidence_period_blocks = 777 |
  .app_state.capture_challenge.params.review_period_blocks = 7777 |
  .app_state.capture_challenge.params.domain_pause_blocks = 777 |
  .app_state.capture_challenge.params.reward_rate_bps = 100000 |
  .app_state.capture_challenge.params.slash_rate_bps = 50000 |
  .app_state.capture_challenge.params.bounty_contribution_per_fact = "1000" |
  .app_state.capture_challenge.params.risk_analysis_interval = 1000
'
```

**Step 6: Add capture_defense module patch**

All params from `x/capture_defense/types/types.go`:

```bash
info "capture_defense..."
patch '
  .app_state.capture_defense.params.decay_epoch_blocks = 7777 |
  .app_state.capture_defense.params.min_verifications_for_score = 5 |
  .app_state.capture_defense.params.hhi_threshold = 400000 |
  .app_state.capture_defense.params.risk_analysis_interval = 777 |
  .app_state.capture_defense.params.history_retention_blocks = 50000 |
  .app_state.capture_defense.params.base_reputation_score = 500000 |
  .app_state.capture_defense.params.max_history_per_domain = 100
'
```

**Step 7: Commit**

```bash
git add scripts/testnet-genesis.sh
git commit -m "feat(R13-3): add alignment through capture_defense module patches"
```

---

### Task 4: Implement cmd_init — channels through emergency module patches

**Files:**
- Modify: `scripts/testnet-genesis.sh`

**Step 1: Add channels module patch**

All params from `x/channels/types/types.go`:

```bash
info "channels..."
patch '
  .app_state.channels.params.min_deposit = "1000000" |
  .app_state.channels.params.min_timeout_blocks = 100 |
  .app_state.channels.params.max_timeout_blocks = 1036800 |
  .app_state.channels.params.dispute_window_blocks = 1000 |
  .app_state.channels.params.default_settlement_freq = 100 |
  .app_state.channels.params.max_channels_per_pair = 10 |
  .app_state.channels.params.channel_open_fee = "100000"
'
```

**Step 2: Add claiming_pot module patch**

All params from `x/claiming_pot/types/types.go`:

```bash
info "claiming_pot..."
patch '
  .app_state.claiming_pot.params.max_pots_active = 10 |
  .app_state.claiming_pot.params.min_claim_amount = "1000"
'
```

**Step 3: Add compute_pool module patch**

All params from `x/compute_pool/types/genesis.go`:

```bash
info "compute_pool..."
patch '
  .app_state.compute_pool.params.compute_pool_share_bps = 300000 |
  .app_state.compute_pool.params.base_cu_per_verification = 100 |
  .app_state.compute_pool.params.min_provider_stake = "100000000" |
  .app_state.compute_pool.params.min_uptime_bps = 950000 |
  .app_state.compute_pool.params.heartbeat_interval_blocks = 1000 |
  .app_state.compute_pool.params.max_price_per_cu = "1000000" |
  .app_state.compute_pool.params.provider_unbonding_blocks = 34272 |
  .app_state.compute_pool.params.price_change_delay_blocks = 1000 |
  .app_state.compute_pool.params.max_latency_ms = 30000 |
  .app_state.compute_pool.params.sla_window_blocks = 1000 |
  .app_state.compute_pool.params.target_utilization_low_bps = 200000 |
  .app_state.compute_pool.params.target_utilization_high_bps = 800000
'
```

**Step 4: Add discovery module patch**

All params from `x/discovery/types/genesis.go`:

```bash
info "discovery..."
patch '
  .app_state.discovery.params.min_registration_stake = "100000000" |
  .app_state.discovery.params.max_capabilities_per_agent = 50 |
  .app_state.discovery.params.profile_expiry_blocks = 342720
'
```

**Step 5: Add disputes module patch (with tier_configs)**

All params from `x/disputes/types/types.go`, including 3 TierConfig objects:

```bash
info "disputes..."
patch '
  .app_state.disputes.params.max_active_disputes = 3 |
  .app_state.disputes.params.escalation_delay = 17136 |
  .app_state.disputes.params.slash_rate_loser_bps = 500000 |
  .app_state.disputes.params.reward_rate_winner_bps = 300000 |
  .app_state.disputes.params.arbiter_reward_bps = 200000 |
  .app_state.disputes.params.tier_configs = [
    {"tier": 0, "arbiter_count": 3, "min_bond": "111000000", "evidence_period": 34272, "voting_period": 68544, "quorum_bps": 660000, "majority_bps": 770000},
    {"tier": 1, "arbiter_count": 7, "min_bond": "555000000", "evidence_period": 68544, "voting_period": 137088, "quorum_bps": 660000, "majority_bps": 770000},
    {"tier": 2, "arbiter_count": 15, "min_bond": "1111000000", "evidence_period": 102816, "voting_period": 239904, "quorum_bps": 660000, "majority_bps": 770000}
  ]
'
```

**Step 6: Add emergency module patch**

All 23 params from `x/emergency/types/genesis.go`:

```bash
info "emergency..."
patch '
  .app_state.emergency.params.halt_quorum = 750000 |
  .app_state.emergency.params.revert_quorum = 800000 |
  .app_state.emergency.params.resume_quorum = 800000 |
  .app_state.emergency.params.halt_prevote_blocks = 11 |
  .app_state.emergency.params.halt_precommit_blocks = 11 |
  .app_state.emergency.params.halt_timeout_blocks = 44 |
  .app_state.emergency.params.revert_prevote_blocks = 22 |
  .app_state.emergency.params.revert_precommit_blocks = 22 |
  .app_state.emergency.params.revert_timeout_blocks = 111 |
  .app_state.emergency.params.resume_prevote_blocks = 22 |
  .app_state.emergency.params.resume_precommit_blocks = 22 |
  .app_state.emergency.params.resume_timeout_blocks = 111 |
  .app_state.emergency.params.max_proposals_per_epoch = 3 |
  .app_state.emergency.params.max_proposals_per_guardian_per_epoch = 1 |
  .app_state.emergency.params.cooldown_blocks = 111 |
  .app_state.emergency.params.min_guardian_stake = "111111000000" |
  .app_state.emergency.params.min_distinct_voters = 3 |
  .app_state.emergency.params.max_revert_depth = 111111 |
  .app_state.emergency.params.epoch_blocks = 34272 |
  .app_state.emergency.params.max_halt_duration_blocks = 34272 |
  .app_state.emergency.params.genesis_council = [] |
  .app_state.emergency.params.council_expiry_block = 0 |
  .app_state.emergency.params.council_virtual_stake = "11111000000"
'
```

**Step 7: Commit**

```bash
git add scripts/testnet-genesis.sh
git commit -m "feat(R13-3): add channels through emergency module patches"
```

---

### Task 5: Implement cmd_init — evidence_mgmt through knowledge module patches

**Files:**
- Modify: `scripts/testnet-genesis.sh`

**Step 1: Add evidence_mgmt module patch**

All params from `x/evidence_mgmt/types/types.go`:

```bash
info "evidence_mgmt..."
patch '
  .app_state.evidence_mgmt.params.min_verifier_tier = 2 |
  .app_state.evidence_mgmt.params.verification_quorum = 3 |
  .app_state.evidence_mgmt.params.challenge_bond = "500000" |
  .app_state.evidence_mgmt.params.challenge_window_blocks = 50000
'
```

**Step 2: Add home module patch**

All params from `x/home/types/genesis.go`:

```bash
info "home..."
patch '
  .app_state.home.params.max_keys_per_home = 20 |
  .app_state.home.params.max_sessions_per_home = 5 |
  .app_state.home.params.session_timeout_blocks = 34272 |
  .app_state.home.params.deadman_min_threshold = 34272 |
  .app_state.home.params.deadman_max_threshold = 100000 |
  .app_state.home.params.max_alerts_per_home = 100 |
  .app_state.home.params.home_creation_fee = "1000000" |
  .app_state.home.params.max_recovery_addresses = 5
'
```

**Step 3: Add ibcratelimit module patch**

```bash
info "ibcratelimit..."
patch '
  .app_state.ibcratelimit.params.enabled = true
'
```

**Step 4: Add icaauth module patch**

All params from `x/icaauth/types/types.go`:

```bash
info "icaauth..."
patch '
  .app_state.icaauth.params.max_remote_accounts_per_owner = 5 |
  .app_state.icaauth.params.allowed_host_msg_types = [
    "/cosmos.bank.v1beta1.MsgSend",
    "/cosmos.staking.v1beta1.MsgDelegate",
    "/cosmos.staking.v1beta1.MsgUndelegate",
    "/cosmos.distribution.v1beta1.MsgWithdrawDelegatorReward",
    "/cosmos.gov.v1.MsgVote",
    "/cosmos.gov.v1.MsgSubmitProposal"
  ] |
  .app_state.icaauth.params.registration_cooldown = 100 |
  .app_state.icaauth.params.max_messages_per_tx = 5
'
```

**Step 5: Add knowledge module patch (largest — all 48+ params)**

All params from `x/knowledge/types/genesis.go`. This is the PoT core module with the most parameters. Use `[TESTNET]` overrides from prototype for phase durations:

```bash
info "knowledge (PoT core)..."
patch '
  .app_state.knowledge.params.min_verifiers = 3 |
  .app_state.knowledge.params.max_verifiers = 22 |
  .app_state.knowledge.params.commit_phase_blocks = 50 |
  .app_state.knowledge.params.reveal_phase_blocks = 50 |
  .app_state.knowledge.params.aggregation_phase_blocks = 25 |
  .app_state.knowledge.params.claim_cooldown_blocks = 50 |
  .app_state.knowledge.params.initial_confidence = 500000 |
  .app_state.knowledge.params.confidence_boost_per_verification = 50000 |
  .app_state.knowledge.params.confidence_threshold = 770000 |
  .app_state.knowledge.params.quorum_threshold = 660000 |
  .app_state.knowledge.params.wrong_verification_slash_bps = 50000 |
  .app_state.knowledge.params.missed_reveal_slash_bps = 100000 |
  .app_state.knowledge.params.equivocation_slash_bps = 200000 |
  .app_state.knowledge.params.invalid_claim_slash_bps = 220000 |
  .app_state.knowledge.params.verification_reward = "3000000" |
  .app_state.knowledge.params.verification_reward_decay_bps = 999000 |
  .app_state.knowledge.params.min_claim_text_length = 20 |
  .app_state.knowledge.params.max_claim_text_length = 10000 |
  .app_state.knowledge.params.min_claim_stake = "1000000" |
  .app_state.knowledge.params.adversarial_verification_enabled = true |
  .app_state.knowledge.params.provisional_threshold = 500000 |
  .app_state.knowledge.params.reject_threshold = 300000 |
  .app_state.knowledge.params.challenge_duration_blocks = 34272 |
  .app_state.knowledge.params.min_challenge_stake = "11000000" |
  .app_state.knowledge.params.failed_challenge_slash_bps = 220000 |
  .app_state.knowledge.params.successful_challenge_reward_bps = 300000 |
  .app_state.knowledge.params.max_concurrent_challenges = 3 |
  .app_state.knowledge.params.citation_share_bps = 150000 |
  .app_state.knowledge.params.cross_domain_bonus_bps = 200000 |
  .app_state.knowledge.params.max_facts_per_domain = 100000 |
  .app_state.knowledge.params.fact_expiry_blocks = 0 |
  .app_state.knowledge.params.cross_stratum_discount_bps = 0 |
  .app_state.knowledge.params.novelty_bonus_bps = 0 |
  .app_state.knowledge.params.max_validators_per_round = 22 |
  .app_state.knowledge.params.max_citations_per_claim = 50 |
  .app_state.knowledge.params.citation_decay_per_level = 500000 |
  .app_state.knowledge.params.self_citation_discount_bps = 500000 |
  .app_state.knowledge.params.confidence_growth_epoch = 1111 |
  .app_state.knowledge.params.confidence_growth_per_epoch_bps = 11000 |
  .app_state.knowledge.params.max_survival_confidence = 770000 |
  .app_state.knowledge.params.survived_challenge_confidence_cap = 880000 |
  .app_state.knowledge.params.max_apprentice_validators = 111 |
  .app_state.knowledge.params.conformity_threshold_bps = 950000 |
  .app_state.knowledge.params.calibration_trivial_threshold = 950000 |
  .app_state.knowledge.params.misbehavior_rejection_threshold = 300000 |
  .app_state.knowledge.params.min_domain_contributors_for_novelty = 3 |
  .app_state.knowledge.params.min_participation_rate_bps = 500000 |
  .app_state.knowledge.params.challenge_stake_ratio_min_bps = 500000 |
  .app_state.knowledge.params.research_fund_share_bps = 130000
'
```

**Step 6: Commit**

```bash
git add scripts/testnet-genesis.sh
git commit -m "feat(R13-3): add evidence_mgmt through knowledge module patches"
```

---

### Task 6: Implement cmd_init — liquiditypool through partnerships module patches

**Files:**
- Modify: `scripts/testnet-genesis.sh`

**Step 1: Add liquiditypool module patch**

All params from `x/liquiditypool/types/genesis.go`:

```bash
info "liquiditypool..."
patch '
  .app_state.liquiditypool.params.default_swap_fee_bps = 3000 |
  .app_state.liquiditypool.params.max_pools = 3 |
  .app_state.liquiditypool.params.min_initial_liquidity = "10000000000" |
  .app_state.liquiditypool.params.twap_window_blocks = 1000 |
  .app_state.liquiditypool.params.protocol_fee_bps = 450000 |
  .app_state.liquiditypool.params.min_reserve = "1"
'
```

**Step 2: Add ontology module patch**

All params from `x/ontology/types/genesis.go`:

```bash
info "ontology..."
patch '
  .app_state.ontology.params.min_proposal_stake = "1000000" |
  .app_state.ontology.params.proposal_voting_period = 34272 |
  .app_state.ontology.params.min_endorsements = 3 |
  .app_state.ontology.params.cross_stratum_discount = 50000 |
  .app_state.ontology.params.max_domains_per_stratum = 100 |
  .app_state.ontology.params.allow_new_strata = false
'
```

**Step 3: Add partnerships module patch**

All params from `x/partnerships/types/genesis.go`:

```bash
info "partnerships..."
patch '
  .app_state.partnerships.params.formation_window_blocks = 77 |
  .app_state.partnerships.params.cooling_period_blocks = 222222 |
  .app_state.partnerships.params.common_pot_share_bps = 220000 |
  .app_state.partnerships.params.safety_freeze_duration_blocks = 222 |
  .app_state.partnerships.params.max_freezes_per_epoch = 3 |
  .app_state.partnerships.params.coercion_review_blocks = 1111 |
  .app_state.partnerships.params.base_cooldown_blocks = 77 |
  .app_state.partnerships.params.max_counter_proposal_depth = 3 |
  .app_state.partnerships.params.default_human_split_bps = 500000 |
  .app_state.partnerships.params.default_agent_split_bps = 500000 |
  .app_state.partnerships.params.min_partnership_stake = "1000000" |
  .app_state.partnerships.params.seed_partnership_duration = 10000 |
  .app_state.partnerships.params.seed_common_pot_cap = "100000000"
'
```

**Step 4: Commit**

```bash
git add scripts/testnet-genesis.sh
git commit -m "feat(R13-3): add liquiditypool through partnerships module patches"
```

---

### Task 7: Implement cmd_init — qualification through tree module patches

**Files:**
- Modify: `scripts/testnet-genesis.sh`

**Step 1: Add qualification module patch**

All params from `x/qualification/types/types.go`:

```bash
info "qualification..."
patch '
  .app_state.qualification.params.min_stake_amount = "100000000" |
  .app_state.qualification.params.stake_lock_period = 7777 |
  .app_state.qualification.params.min_verifications = 19 |
  .app_state.qualification.params.min_accuracy_bps = 770000 |
  .app_state.qualification.params.min_reputation_score = 500000 |
  .app_state.qualification.params.qualification_period = 1209600 |
  .app_state.qualification.params.probation_period = 302400 |
  .app_state.qualification.params.renewal_window = 100800 |
  .app_state.qualification.params.max_endorsements = 50 |
  .app_state.qualification.params.cross_ref_min_weight = 30 |
  .app_state.qualification.params.cross_ref_weight_discount_bps = 200000 |
  .app_state.qualification.params.inheritance_weight_discount_bps = 300000
'
```

**Step 2: Add research module patch**

All params from `x/research/types/types.go`:

```bash
info "research..."
patch '
  .app_state.research.params.min_research_stake = "1000000" |
  .app_state.research.params.min_challenge_stake = "1000000" |
  .app_state.research.params.review_period_blocks = 68544 |
  .app_state.research.params.min_reviewer_count = 3 |
  .app_state.research.params.acceptance_score_threshold = 70 |
  .app_state.research.params.rejection_slash_bps = 330000 |
  .app_state.research.params.max_bounty_reward = "10000000000" |
  .app_state.research.params.bounty_min_deadline_blocks = 34272
'
```

**Step 3: Add schedule module patch**

All params from `x/schedule/types/genesis.go`:

```bash
info "schedule..."
patch '
  .app_state.schedule.params.max_active_per_account = 100 |
  .app_state.schedule.params.max_gas_per_block = 2000000 |
  .app_state.schedule.params.min_interval_blocks = 1 |
  .app_state.schedule.params.min_fee_per_execution = "10000" |
  .app_state.schedule.params.max_compound_depth = 3
'
```

**Step 4: Add tokens module patch**

All params from `x/tokens/types/genesis.go`:

```bash
info "tokens..."
patch '
  .app_state.tokens.params.emission_epoch_blocks = 0 |
  .app_state.tokens.params.default_fee_bps = ""
'
```

**Step 5: Add toolbox module patch**

All 26 params from `x/toolbox/types/types.go`:

```bash
info "toolbox..."
patch '
  .app_state.toolbox.params.max_contributors = 22 |
  .app_state.toolbox.params.max_dependency_depth = 10 |
  .app_state.toolbox.params.max_dependencies = 20 |
  .app_state.toolbox.params.min_tool_stake = 11000000 |
  .app_state.toolbox.params.share_lock_cooldown_blocks = 34272 |
  .app_state.toolbox.params.deprecation_grace_blocks = 240000 |
  .app_state.toolbox.params.blocks_per_trust_update = 1000 |
  .app_state.toolbox.params.verified_grace_period_blocks = 10000 |
  .app_state.toolbox.params.tool_gas_limit = 1000000 |
  .app_state.toolbox.params.demand_window_size = 1000 |
  .app_state.toolbox.params.target_calls_per_block_per_tool = 10 |
  .app_state.toolbox.params.target_global_calls_per_block = 100 |
  .app_state.toolbox.params.surge_threshold_bps = 500000 |
  .app_state.toolbox.params.surge_critical_bps = 800000 |
  .app_state.toolbox.params.max_surge_multiplier_bps = 10000000 |
  .app_state.toolbox.params.surge_enabled = true |
  .app_state.toolbox.params.free_calls_per_epoch = 50 |
  .app_state.toolbox.params.min_home_age_blocks = 10000 |
  .app_state.toolbox.params.free_calls_enabled = true |
  .app_state.toolbox.params.tool_revenue_bps = 550000 |
  .app_state.toolbox.params.protocol_bps = 220000 |
  .app_state.toolbox.params.research_bps = 130000 |
  .app_state.toolbox.params.burn_bps = 100000 |
  .app_state.toolbox.params.protocol_citation_bps = 500000 |
  .app_state.toolbox.params.protocol_verification_bps = 300000 |
  .app_state.toolbox.params.protocol_treasury_bps = 200000
'
```

**Step 6: Add tree module patch**

All params from `x/tree/types/genesis.go`:

```bash
info "tree..."
patch '
  .app_state.tree.params.min_budget = "1000000" |
  .app_state.tree.params.max_tasks_per_project = 200 |
  .app_state.tree.params.max_contributors = 50 |
  .app_state.tree.params.max_applications = 100 |
  .app_state.tree.params.task_deadline_min_blocks = 100 |
  .app_state.tree.params.task_deadline_max_blocks = 1036800 |
  .app_state.tree.params.max_rejections = 3 |
  .app_state.tree.params.seed_expiry_blocks = 172800 |
  .app_state.tree.params.min_contributors_to_start = 1 |
  .app_state.tree.params.contributors_bp = 550000 |
  .app_state.tree.params.protocol_treasury_bp = 220000 |
  .app_state.tree.params.research_fund_bp = 130000 |
  .app_state.tree.params.burn_bp = 100000 |
  .app_state.tree.params.evidence_tax_bp = 220000
'
```

**Step 7: Commit**

```bash
git add scripts/testnet-genesis.sh
git commit -m "feat(R13-3): add qualification through tree module patches"
```

---

### Task 8: Implement cmd_init — vesting_rewards, zerone_auth, zerone_gov, zerone_staking patches

**Files:**
- Modify: `scripts/testnet-genesis.sh`

**Step 1: Add vesting_rewards module patch (with category configs)**

All params from `x/vesting_rewards/types/genesis.go` plus 10 epistemic release curves:

```bash
info "vesting_rewards..."
patch '
  .app_state.vesting_rewards.params.block_reward = "3000000" |
  .app_state.vesting_rewards.params.reward_decay_bps = 850000 |
  .app_state.vesting_rewards.params.blocks_per_reward_epoch = 240000 |
  .app_state.vesting_rewards.params.revenue_split = {"contributor_bps": 550000, "protocol_bps": 220000, "research_bps": 130000, "burn_bps": 100000} |
  .app_state.vesting_rewards.params.protocol_sub_split = {"citation_bps": 500000, "verification_bps": 300000, "treasury_bps": 200000} |
  .app_state.vesting_rewards.params.founder_share_bps = 70000 |
  .app_state.vesting_rewards.params.founder_address = "" |
  .app_state.vesting_rewards.params.governance_activation_height = 0 |
  .app_state.vesting_rewards.params.category_reward_configs = [
    {"category": "axiomatic",     "multiplier_bps": 2000000},
    {"category": "formal_proof",  "multiplier_bps": 1500000},
    {"category": "on_chain",      "multiplier_bps": 1200000},
    {"category": "cryptographic", "multiplier_bps": 1200000},
    {"category": "computational", "multiplier_bps": 1000000},
    {"category": "peer_reviewed", "multiplier_bps": 900000},
    {"category": "replicated",    "multiplier_bps": 800000},
    {"category": "oracle_feed",   "multiplier_bps": 600000},
    {"category": "attestation",   "multiplier_bps": 500000},
    {"category": "contested",     "multiplier_bps": 300000}
  ] |
  .app_state.vesting_rewards.params.research_fund_module_account = "research_fund" |
  .app_state.vesting_rewards.params.vesting_enabled = true |
  .app_state.vesting_rewards.params.released_clawback_rate = 3300 |
  .app_state.vesting_rewards.params.min_validators_for_full_reward = 3 |
  .app_state.vesting_rewards.params.empty_block_reward_rate = 0 |
  .app_state.vesting_rewards.params.floor_reward = "100000" |
  .app_state.vesting_rewards.params.initial_fund_balance = "0"
'
```

Note: `[TESTNET]` `min_validators_for_full_reward = 3` (production: 22), `block_reward = "3000000"` (lower than production `"10000000"`)

**Step 2: Add vesting category release curves (top-level genesis state)**

```bash
# Category configs (epistemic release curves)
patch '
  .app_state.vesting_rewards.category_configs = [
    {"category": "axiomatic",     "half_life_blocks": 1111111, "cliff_blocks": 11111, "max_release": 950000},
    {"category": "formal_proof",  "half_life_blocks": 555555,  "cliff_blocks": 5555,  "max_release": 920000},
    {"category": "on_chain",      "half_life_blocks": 222222,  "cliff_blocks": 1111,  "max_release": 900000},
    {"category": "cryptographic", "half_life_blocks": 222222,  "cliff_blocks": 3333,  "max_release": 900000},
    {"category": "computational", "half_life_blocks": 333333,  "cliff_blocks": 2222,  "max_release": 880000},
    {"category": "peer_reviewed", "half_life_blocks": 111111,  "cliff_blocks": 5555,  "max_release": 850000},
    {"category": "replicated",    "half_life_blocks": 111111,  "cliff_blocks": 3333,  "max_release": 880000},
    {"category": "oracle_feed",   "half_life_blocks": 55555,   "cliff_blocks": 555,   "max_release": 800000},
    {"category": "attestation",   "half_life_blocks": 77777,   "cliff_blocks": 2222,  "max_release": 800000},
    {"category": "contested",     "half_life_blocks": 22222,   "cliff_blocks": 1111,  "max_release": 600000}
  ]
'
```

**Step 3: Add zerone_auth module patch**

All params from `x/auth/types/genesis.go` (genesis key: `zerone_auth`):

```bash
info "zerone_auth..."
patch '
  .app_state.zerone_auth.params.max_session_keys = 5 |
  .app_state.zerone_auth.params.max_session_duration = 34272 |
  .app_state.zerone_auth.params.key_rotation_cooldown = 111 |
  .app_state.zerone_auth.params.recovery_delay_blocks = 1000 |
  .app_state.zerone_auth.params.challenge_period_blocks = 500 |
  .app_state.zerone_auth.params.bootstrap_enabled = false |
  .app_state.zerone_auth.params.bootstrap_amount = "0" |
  .app_state.zerone_auth.params.max_metadata_length = 1024 |
  .app_state.zerone_auth.params.require_did = false |
  .app_state.zerone_auth.params.max_recovery_shards = 10 |
  .app_state.zerone_auth.params.recovery_challenge_period_blocks = 500 |
  .app_state.zerone_auth.params.recovery_execution_delay_blocks = 1000
'
```

**Step 4: Add zerone_gov module patch**

All params from `x/gov/types/genesis.go` (genesis key: `zerone_gov`). `[TESTNET]` shortened voting periods:

```bash
info "zerone_gov..."
patch '
  .app_state.zerone_gov.params.voting_period_blocks = 17136 |
  .app_state.zerone_gov.params.discussion_period_blocks = 8568 |
  .app_state.zerone_gov.params.quorum_threshold_bps = 334000 |
  .app_state.zerone_gov.params.support_threshold_bps = 500000 |
  .app_state.zerone_gov.params.min_lip_stake = "1000000" |
  .app_state.zerone_gov.params.min_vote_stake = "0" |
  .app_state.zerone_gov.params.category_configs = [
    {"category": "consensus",  "required_stake_bps": 1000000, "review_blocks": 34272},
    {"category": "economics",  "required_stake_bps": 800000,  "review_blocks": 17136},
    {"category": "interface",  "required_stake_bps": 400000,  "review_blocks": 8568},
    {"category": "process",    "required_stake_bps": 200000,  "review_blocks": 8568}
  ] |
  .app_state.zerone_gov.params.research_fund_voters = [] |
  .app_state.zerone_gov.params.research_discussion_blocks = 8568 |
  .app_state.zerone_gov.params.research_voting_blocks = 17136
'
```

**Step 5: Add zerone_staking module patch (with 4 tier configs)**

All params from `x/staking/types/genesis.go` (genesis key: `zerone_staking`). `[TESTNET]` unbonding shortened to ~3 days:

```bash
info "zerone_staking..."
patch '
  .app_state.zerone_staking.params.unbonding_period = 102672 |
  .app_state.zerone_staking.params.virtual_stake = "11000000" |
  .app_state.zerone_staking.params.max_validators = 100 |
  .app_state.zerone_staking.params.min_self_delegation = "111000" |
  .app_state.zerone_staking.params.max_slashes_per_epoch = 2 |
  .app_state.zerone_staking.params.slash_decay_period_blocks = 34272 |
  .app_state.zerone_staking.params.max_slash_count_deactivate = 3 |
  .app_state.zerone_staking.params.min_stake_for_verification = "111000" |
  .app_state.zerone_staking.params.slash_escalation_bps = 100000 |
  .app_state.zerone_staking.params.reputation_correct_delta = 100 |
  .app_state.zerone_staking.params.reputation_incorrect_delta = 200 |
  .app_state.zerone_staking.params.reputation_slash_delta = 10000 |
  .app_state.zerone_staking.params.redelegation_cooldown_blocks = 1111 |
  .app_state.zerone_staking.params.tier_configs = [
    {
      "tier": 0, "name": "apprentice",
      "min_stake": "111000", "min_reputation": 0, "min_verifications": 0, "min_accuracy": 0,
      "max_slash_count": -1,
      "allowed_categories": ["protocol", "computational", "formal"],
      "reward_multiplier_bps": 100, "selection_weight_bps": 100,
      "slash_window_epochs": 0, "min_contested_verifications": 0, "contested_verification_multiplier": 1,
      "slash_multiplier_bps": 1500
    },
    {
      "tier": 1, "name": "verified",
      "min_stake": "1110000", "min_reputation": 770000, "min_verifications": 22, "min_accuracy": 770000,
      "max_slash_count": -1,
      "allowed_categories": ["protocol", "computational", "formal", "empirical"],
      "reward_multiplier_bps": 500, "selection_weight_bps": 500,
      "slash_window_epochs": 0, "min_contested_verifications": 0, "contested_verification_multiplier": 1,
      "slash_multiplier_bps": 1200
    },
    {
      "tier": 2, "name": "scholar",
      "min_stake": "1111000000", "min_reputation": 500000, "min_verifications": 11, "min_accuracy": 500000,
      "max_slash_count": -1,
      "allowed_categories": ["protocol", "computational", "formal", "empirical", "oracle", "attestation"],
      "reward_multiplier_bps": 1000, "selection_weight_bps": 1000,
      "slash_window_epochs": 0, "min_contested_verifications": 0, "contested_verification_multiplier": 1,
      "slash_multiplier_bps": 1000
    },
    {
      "tier": 3, "name": "guardian",
      "min_stake": "11111000000", "min_reputation": 770000, "min_verifications": 333, "min_accuracy": 770000,
      "max_slash_count": 0,
      "allowed_categories": ["protocol", "computational", "formal", "empirical", "oracle", "attestation", "predictive", "social", "contested"],
      "reward_multiplier_bps": 2000, "selection_weight_bps": 1500,
      "slash_window_epochs": 10, "min_contested_verifications": 33, "contested_verification_multiplier": 3,
      "slash_multiplier_bps": 1000
    }
  ]
'
```

Also set top-level tier_configs:
```bash
patch '
  .app_state.zerone_staking.tier_configs = .app_state.zerone_staking.params.tier_configs
'
```

**Step 6: Commit**

```bash
git add scripts/testnet-genesis.sh
git commit -m "feat(R13-3): add vesting_rewards, zerone_auth, zerone_gov, zerone_staking patches"
```

---

### Task 9: Implement cmd_init — axiom injection, bootstrap accounts, validation

**Files:**
- Modify: `scripts/testnet-genesis.sh`

**Step 1: Add axiom injection hook**

After all module patches, add axiom injection (identical pattern to `genesis-ceremony.sh:196-205`):

```bash
# Axiom injection hook
if command -v go &>/dev/null && [ -f "${PROJECT_ROOT}/x/knowledge/types/genesis_axioms.json" ]; then
  if [ -d "${PROJECT_ROOT}/tools/axiom-loader" ]; then
    info "Injecting 777 genesis axioms..."
    (cd "${PROJECT_ROOT}" && go run tools/axiom-loader/main.go inject \
      x/knowledge/types/genesis_axioms.json \
      "${GENESIS}") || warn "Axiom injection failed (non-fatal)"
  else
    warn "axiom-loader not found — skipping axiom injection"
  fi
fi
```

**Step 2: Add genesis validation**

```bash
validate_genesis "module param patching"
```

**Step 3: Add bootstrap accounts**

Create foundation, research, faucet, and 3 test accounts (matching prototype pattern):

```bash
info "Creating bootstrap accounts..."

${BINARY} keys add foundation --keyring-backend test --home "${TESTNET_HOME}" 2>/dev/null
FOUNDATION_ADDR=$(${BINARY} keys show foundation -a --keyring-backend test --home "${TESTNET_HOME}")
${BINARY} add-genesis-account "${FOUNDATION_ADDR}" "${FOUNDATION_BALANCE}${DENOM}" --home "${TESTNET_HOME}"
info "  Foundation: ${FOUNDATION_ADDR} (1M ZRN)"

${BINARY} keys add research --keyring-backend test --home "${TESTNET_HOME}" 2>/dev/null
RESEARCH_ADDR=$(${BINARY} keys show research -a --keyring-backend test --home "${TESTNET_HOME}")
${BINARY} add-genesis-account "${RESEARCH_ADDR}" "${RESEARCH_BALANCE}${DENOM}" --home "${TESTNET_HOME}"
info "  Research: ${RESEARCH_ADDR} (500K ZRN)"

${BINARY} keys add faucet --keyring-backend test --home "${TESTNET_HOME}" 2>/dev/null
FAUCET_ADDR=$(${BINARY} keys show faucet -a --keyring-backend test --home "${TESTNET_HOME}")
${BINARY} add-genesis-account "${FAUCET_ADDR}" "${FAUCET_BALANCE}${DENOM}" --home "${TESTNET_HOME}"
info "  Faucet: ${FAUCET_ADDR} (100K ZRN)"

for i in 1 2 3; do
  ${BINARY} keys add "test${i}" --keyring-backend test --home "${TESTNET_HOME}" 2>/dev/null
  TEST_ADDR=$(${BINARY} keys show "test${i}" -a --keyring-backend test --home "${TESTNET_HOME}")
  ${BINARY} add-genesis-account "${TEST_ADDR}" "${TEST_BALANCE}${DENOM}" --home "${TESTNET_HOME}"
  info "  Test${i}: ${TEST_ADDR} (10K ZRN)"
done

validate_genesis "bootstrap accounts"
```

**Step 4: Add init summary banner**

Print summary showing chain ID, testnet home, genesis path, module count, next steps.

**Step 5: Commit**

```bash
git add scripts/testnet-genesis.sh
git commit -m "feat(R13-3): add axiom injection, bootstrap accounts, and validation to init"
```

---

### Task 10: Implement add-validator, finalize, export subcommands

**Files:**
- Modify: `scripts/testnet-genesis.sh`

**Step 1: Implement cmd_add_validator**

Port from prototype's `cmd_add_validator` (lines 696-752), translating LGM→ZRN. Same pattern as `genesis-ceremony.sh:cmd_add_validator` but using testnet constants.

**Step 2: Implement cmd_finalize**

Port from prototype's `cmd_finalize` (lines 756-781). Collect gentxs, validate, print summary with genesis hash.

**Step 3: Implement cmd_export**

Port from prototype's `cmd_export` (lines 785-806). Copy genesis.json to project root, print chain info.

**Step 4: Commit**

```bash
git add scripts/testnet-genesis.sh
git commit -m "feat(R13-3): implement add-validator, finalize, export subcommands"
```

---

### Task 11: Implement verify subcommand

**Files:**
- Modify: `scripts/testnet-genesis.sh`

**Step 1: Implement cmd_verify**

Query params from running node for ALL 30 custom modules + SDK modules. Use the correct query module names:

```bash
cmd_verify() {
  echo "═══════════════════════════════════════════════════════════════"
  echo "  Zerone — Testnet Genesis: VERIFY PARAMS"
  echo "═══════════════════════════════════════════════════════════════"
  echo ""

  check_deps

  local modules=(
    alignment autopoiesis billing bvm
    capture_challenge capture_defense channels claiming_pot
    compute_pool discovery disputes emergency
    evidence_mgmt home ibcratelimit icaauth
    knowledge liquiditypool ontology partnerships
    qualification research schedule tokens
    toolbox tree vesting_rewards
    zerone_auth zerone_gov zerone_staking
  )

  local ok_count=0
  local fail_count=0

  for mod in "${modules[@]}"; do
    info "Querying ${mod} params..."
    if ${BINARY} query "${mod}" params --output json 2>/dev/null | jq '.params' 2>/dev/null; then
      ok_count=$((ok_count + 1))
    else
      warn "  ${mod}: not responding"
      fail_count=$((fail_count + 1))
    fi
    echo ""
  done

  echo "═══════════════════════════════════════════════════════════════"
  echo "  Verified: ${ok_count} modules OK, ${fail_count} failed"
  echo "═══════════════════════════════════════════════════════════════"
}
```

**Step 2: Commit**

```bash
git add scripts/testnet-genesis.sh
git commit -m "feat(R13-3): implement verify subcommand for all 30 modules"
```

---

### Task 12: Create testnet-genesis-config.json

**Files:**
- Create: `scripts/testnet-genesis-config.json`

**Step 1: Write the complete JSON reference**

Create the companion reference JSON document with:
- `_meta` section (chain_id, generated date, block_time_ms, blocks_per_day, bps_scale, denom)
- `consensus` section
- `modules` section with all 30 modules, each containing:
  - `_note` explaining module purpose
  - `params` with all values
  - `[TESTNET]` annotations where values differ from production defaults
- `testnet_overrides` section listing all testnet-specific deviations
- `bootstrap_accounts` section
- `validators` section with commission defaults

Port structure from `legible-money/scripts/testnet-genesis-config.json` (745 lines), translating all LGM→ZRN values and updating module names. Use the exact same parameter values set in `testnet-genesis.sh`.

**Step 2: Validate JSON**

Run: `jq . scripts/testnet-genesis-config.json > /dev/null`
Expected: No errors

**Step 3: Commit**

```bash
git add scripts/testnet-genesis-config.json
git commit -m "feat(R13-3): create testnet-genesis-config.json parameter reference"
```

---

### Task 13: Verify the complete script with shellcheck

**Files:**
- Modify: `scripts/testnet-genesis.sh` (if fixes needed)

**Step 1: Run shellcheck**

Run: `shellcheck scripts/testnet-genesis.sh`
Expected: No errors (warnings OK)

**Step 2: Run help subcommand**

Run: `bash scripts/testnet-genesis.sh help`
Expected: Clean usage output

**Step 3: Verify script line count**

Run: `wc -l scripts/testnet-genesis.sh`
Expected: ~700-900 lines (comparable to prototype's 863)

**Step 4: Fix any issues found**

Address shellcheck warnings and ensure consistent style with existing scripts.

**Step 5: Final commit**

```bash
git add scripts/testnet-genesis.sh scripts/testnet-genesis-config.json
git commit -m "feat(R13-3): finalize testnet genesis script and config reference"
```

---

## Testnet Override Summary

These values differ from `DefaultParams()` for faster testnet iteration:

| Module | Param | Testnet | Default | Reason |
|--------|-------|---------|---------|--------|
| knowledge | commit_phase_blocks | 50 | 4 | Longer for testnet observability |
| knowledge | reveal_phase_blocks | 50 | 4 | Longer for testnet observability |
| knowledge | aggregation_phase_blocks | 25 | 3 | Longer for testnet observability |
| zerone_staking | unbonding_period | 102672 | 268560 | ~3 days vs ~7 days |
| zerone_gov | voting_period_blocks | 17136 | 102816 | ~12h vs ~3 days |
| zerone_gov | discussion_period_blocks | 8568 | 68544 | ~6h vs ~2 days |
| zerone_gov | research_discussion_blocks | 8568 | 68544 | ~6h vs ~2 days |
| zerone_gov | research_voting_blocks | 17136 | 102816 | ~12h vs ~3 days |
| vesting_rewards | min_validators_for_full_reward | 3 | 22 | Fewer validators needed |
| vesting_rewards | block_reward | "3000000" | "10000000" | Lower emission for testnet |
| emergency | min_distinct_voters | 3 | 4 | Fewer guardians needed |
