#!/usr/bin/env bash
# ═══════════════════════════════════════════════════════════════════════════
# Zerone — Testnet Genesis Pipeline
# ═══════════════════════════════════════════════════════════════════════════
#
# Multi-step genesis pipeline for coordinated testnet launches.
# Produces a validated genesis.json with bootstrap accounts, patched
# consensus/module params (testnet-tuned), and collected validator gentxs.
#
# All 30 custom modules are configured with testnet-appropriate values:
# shorter periods, lower thresholds, reduced balances for rapid iteration.
#
# Usage:
#   scripts/testnet-genesis.sh init                # Initialize testnet genesis
#   scripts/testnet-genesis.sh add-validator NAME  # Add a validator
#   scripts/testnet-genesis.sh finalize            # Collect gentxs, validate
#   scripts/testnet-genesis.sh export              # Export genesis + keys
#   scripts/testnet-genesis.sh verify              # Verify params on running node
#
# Requires: jq >= 1.6, go (1.24+), curl (for verify)
# ═══════════════════════════════════════════════════════════════════════════

set -euo pipefail

# ── Constants ────────────────────────────────────────────────────────────

CHAIN_ID="zerone-testnet-1"
GENESIS_TIME="2026-07-01T00:00:00Z"  # placeholder — set at ceremony time
DENOM="uzrn"
PROJECT_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
BINARY="${PROJECT_ROOT}/build/zeroned"
CEREMONY_HOME="${HOME}/.zeroned/testnet-genesis"
KEYRING="test"

# Economics: Testnet — lower bootstrap balances for rapid iteration
FOUNDATION_BALANCE="1000000000000${DENOM}"    # 1,000,000 ZRN  (1M)
RESEARCH_BALANCE="500000000000${DENOM}"       #   500,000 ZRN (500K)
FAUCET_BALANCE="100000000000${DENOM}"         #   100,000 ZRN (100K)
VALIDATOR_BALANCE="100000000000${DENOM}"      #   100,000 ZRN (100K)
VALIDATOR_STAKE="10000000000${DENOM}"         #    10,000 ZRN  (10K)

# ── Helpers ──────────────────────────────────────────────────────────────

die()  { echo -e "\033[1;31mFATAL:\033[0m $*" >&2; exit 1; }
info() { echo -e "\033[1;34m  ->\033[0m $*"; }
ok()   { echo -e "\033[1;32m  OK\033[0m $*"; }
warn() { echo -e "\033[1;33m  !!\033[0m $*"; }

check_deps() {
  command -v jq >/dev/null 2>&1 || die "jq >= 1.6 required. Install: brew install jq"
  command -v go >/dev/null 2>&1 || die "go >= 1.24 required."
}

check_binary() {
  [ -x "${BINARY}" ] || die "Binary not found at ${BINARY}. Run 'init' first or build manually."
}

# Atomic jq patch on ceremony genesis
patch() {
  local genesis="${CEREMONY_HOME}/config/genesis.json"
  jq "$1" "$genesis" > "${genesis}.tmp" && mv "${genesis}.tmp" "$genesis"
}

# Validate genesis after a mutation
validate_genesis() {
  if ! ${BINARY} genesis validate-genesis --home "${CEREMONY_HOME}" 2>/dev/null; then
    if ! ${BINARY} genesis validate --home "${CEREMONY_HOME}" 2>/dev/null; then
      die "Genesis validation FAILED after: $1"
    fi
  fi
}

zeroned_coord() {
  "${BINARY}" "$@" --home "${CEREMONY_HOME}"
}

# ── cmd_init ─────────────────────────────────────────────────────────────

cmd_init() {
  echo "═══════════════════════════════════════════════════════════════"
  echo "  Zerone — Testnet Genesis: INIT"
  echo "  Chain: ${CHAIN_ID} | Genesis: ${GENESIS_TIME}"
  echo "═══════════════════════════════════════════════════════════════"
  echo ""

  check_deps

  # ── Step 1: Build binary if not present ──────────────────────────────
  if [ ! -x "${BINARY}" ]; then
    info "Building zeroned binary..."
    mkdir -p "${PROJECT_ROOT}/build"
    (cd "${PROJECT_ROOT}" && go build \
      -ldflags "-X github.com/cosmos/cosmos-sdk/version.Name=zerone -X github.com/cosmos/cosmos-sdk/version.AppName=zeroned" \
      -o build/zeroned ./cmd/zeroned) || die "Build failed"
    ok "Binary built: ${BINARY}"
  else
    ok "Binary exists: ${BINARY}"
  fi

  # ── Step 2: Clean previous ceremony state ────────────────────────────
  if [ -d "${CEREMONY_HOME}" ]; then
    warn "Removing previous testnet genesis state at ${CEREMONY_HOME}"
    rm -rf "${CEREMONY_HOME}"
  fi

  # ── Step 3: Init coordinator ─────────────────────────────────────────
  info "Initializing testnet genesis coordinator..."
  ${BINARY} init testnet-coordinator \
    --chain-id "${CHAIN_ID}" \
    --default-denom "${DENOM}" \
    --home "${CEREMONY_HOME}" 2>/dev/null
  ok "Coordinator initialized"

  # ── Step 4: Set genesis time ─────────────────────────────────────────
  info "Setting genesis time: ${GENESIS_TIME}"
  patch ".genesis_time = \"${GENESIS_TIME}\""

  # ── Step 5: Patch consensus params ───────────────────────────────────
  info "Patching consensus parameters..."

  # Group 1: Consensus params
  patch '
    .consensus.params.block.max_gas = "33333333" |
    .consensus.params.block.max_bytes = "4194304" |
    .consensus.params.abci.vote_extensions_enable_height = "1"
  '
  ok "Consensus params: max_gas=33333333, max_bytes=4MB, vote_extensions@1"

  # ── Step 6: Patch module params ──────────────────────────────────────
  info "Patching module parameters (30 custom modules + SDK overrides)..."

  # Group 2: SDK staking + slashing
  patch '
    .app_state.staking.params.bond_denom = "uzrn" |
    .app_state.slashing.params.signed_blocks_window = "100" |
    .app_state.slashing.params.slash_fraction_downtime = "0.010000000000000000"
  '
  ok "SDK overrides: staking bond_denom, slashing params"

  # Group 3: Bank denom metadata
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
  ok "Bank denom metadata: ZRN (uzrn/mzrn/zrn)"

  # Group 4: Knowledge module [TESTNET: min_verifiers=2, challenge_duration halved]
  patch '
    .app_state.knowledge.params.min_verifiers = 2 |
    .app_state.knowledge.params.max_verifiers = 22 |
    .app_state.knowledge.params.commit_phase_blocks = 4 |
    .app_state.knowledge.params.reveal_phase_blocks = 4 |
    .app_state.knowledge.params.aggregation_phase_blocks = 3 |
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
    .app_state.knowledge.params.challenge_duration_blocks = 17136 |
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
  ok "Knowledge module: 48 params (min_verifiers=2, challenge_duration=17136)"

  # Group 5: Governance [TESTNET: faster periods — voting halved, discussion halved]
  patch '
    .app_state.zerone_gov.params.voting_period_blocks = 34272 |
    .app_state.zerone_gov.params.discussion_period_blocks = 17136 |
    .app_state.zerone_gov.params.quorum_threshold_bps = 334000 |
    .app_state.zerone_gov.params.support_threshold_bps = 500000 |
    .app_state.zerone_gov.params.min_lip_stake = "1000000" |
    .app_state.zerone_gov.params.min_vote_stake = "0" |
    .app_state.zerone_gov.params.research_discussion_blocks = 17136 |
    .app_state.zerone_gov.params.research_voting_blocks = 34272 |
    .app_state.zerone_gov.params.category_configs = [
      {"category": "parameter", "required_stake_bps": "1000000000", "review_blocks": 17136},
      {"category": "upgrade", "required_stake_bps": "800000000", "review_blocks": 17136},
      {"category": "text", "required_stake_bps": "400000000", "review_blocks": 8568},
      {"category": "research_spend", "required_stake_bps": "200000000", "review_blocks": 8568}
    ]
  '
  ok "Governance: voting=34272 discussion=17136 (halved from prod)"

  # Group 6: Emergency [TESTNET: shorter cooldowns and durations]
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
    .app_state.emergency.params.cooldown_blocks = 55 |
    .app_state.emergency.params.min_guardian_stake = "111111000000" |
    .app_state.emergency.params.min_distinct_voters = 4 |
    .app_state.emergency.params.max_revert_depth = 111111 |
    .app_state.emergency.params.epoch_blocks = 17136 |
    .app_state.emergency.params.council_expiry_block = 0 |
    .app_state.emergency.params.council_virtual_stake = "11111000000" |
    .app_state.emergency.params.max_halt_duration_blocks = 17136
  '
  ok "Emergency: cooldown=55 epoch=17136 max_halt=17136 (halved from prod)"

  # Group 7: Zerone staking [TESTNET: unbonding halved, shorter cooldowns]
  patch '
    .app_state.zerone_staking.params.unbonding_period = 134280 |
    .app_state.zerone_staking.params.max_validators = 100 |
    .app_state.zerone_staking.params.min_self_delegation = "111000" |
    .app_state.zerone_staking.params.virtual_stake = "11000000" |
    .app_state.zerone_staking.params.max_slashes_per_epoch = 2 |
    .app_state.zerone_staking.params.slash_decay_period_blocks = 17136 |
    .app_state.zerone_staking.params.max_slash_count_deactivate = 3 |
    .app_state.zerone_staking.params.min_stake_for_verification = "111000" |
    .app_state.zerone_staking.params.slash_escalation_bps = 100000 |
    .app_state.zerone_staking.params.reputation_correct_delta = 100 |
    .app_state.zerone_staking.params.reputation_incorrect_delta = 200 |
    .app_state.zerone_staking.params.reputation_slash_delta = 10000 |
    .app_state.zerone_staking.params.redelegation_cooldown_blocks = 555 |
    .app_state.zerone_staking.params.tier_configs = [
      {"name":"Apprentice","min_stake":"111000","min_reputation":0,"min_verifications":0,"min_accuracy":0,"allowed_categories":["protocol","computational","formal"],"reward_multiplier_bps":100,"selection_weight_bps":100,"slash_multiplier_bps":1500},
      {"name":"Verified","min_stake":"1110000","min_reputation":770000,"min_verifications":22,"min_accuracy":770000,"allowed_categories":["protocol","computational","formal","empirical"],"reward_multiplier_bps":500,"selection_weight_bps":500,"slash_multiplier_bps":1200},
      {"name":"Scholar","min_stake":"1111000000","min_reputation":500000,"min_verifications":11,"min_accuracy":500000,"allowed_categories":["protocol","computational","formal","empirical","oracle","attestation"],"reward_multiplier_bps":1000,"selection_weight_bps":1000,"slash_multiplier_bps":1000},
      {"name":"Guardian","min_stake":"11111000000","min_reputation":770000,"min_verifications":333,"min_accuracy":770000,"max_slash_count":0,"allowed_categories":["protocol","computational","formal","empirical","oracle","attestation","historical","testimonial"],"reward_multiplier_bps":2000,"selection_weight_bps":1500,"min_contested_verifications":33,"contested_verification_multiplier":3,"slash_multiplier_bps":1000}
    ]
  '
  ok "Zerone staking: unbonding=134280 redelegation_cooldown=555 (halved from prod)"

  # Group 8: Vesting rewards [TESTNET: shorter epochs, lower validator threshold]
  patch '
    .app_state.vesting_rewards.params.block_reward = "10000000" |
    .app_state.vesting_rewards.params.reward_decay_bps = 994478 |
    .app_state.vesting_rewards.params.blocks_per_reward_epoch = 50000 |
    .app_state.vesting_rewards.params.founder_share_bps = 70000 |
    .app_state.vesting_rewards.params.founder_address = "" |
    .app_state.vesting_rewards.params.governance_activation_height = 0 |
    .app_state.vesting_rewards.params.vesting_enabled = true |
    .app_state.vesting_rewards.params.released_clawback_rate = 3300 |
    .app_state.vesting_rewards.params.min_validators_for_full_reward = 11 |
    .app_state.vesting_rewards.params.empty_block_reward_rate = 0 |
    .app_state.vesting_rewards.params.floor_reward = "100000" |
    .app_state.vesting_rewards.params.initial_fund_balance = "0" |
    .app_state.vesting_rewards.params.revenue_split = {"contributor_bps":550000,"protocol_bps":220000,"research_bps":130000,"burn_bps":100000} |
    .app_state.vesting_rewards.params.protocol_sub_split = {"citation_bps":500000,"verification_bps":300000,"treasury_bps":200000} |
    .app_state.vesting_rewards.params.category_reward_configs = [
      {"category":"axiomatic","reward_multiplier_bps":1200000},
      {"category":"formal_proof","reward_multiplier_bps":1100000},
      {"category":"on_chain","reward_multiplier_bps":1000000},
      {"category":"cryptographic","reward_multiplier_bps":1050000},
      {"category":"computational","reward_multiplier_bps":1000000},
      {"category":"peer_reviewed","reward_multiplier_bps":900000},
      {"category":"replicated","reward_multiplier_bps":950000},
      {"category":"oracle_feed","reward_multiplier_bps":800000},
      {"category":"attestation","reward_multiplier_bps":850000},
      {"category":"contested","reward_multiplier_bps":600000}
    ] |
    .app_state.vesting_rewards.params.category_configs = [
      {"category":"axiomatic","half_life_blocks":1111111,"cliff_blocks":11111,"max_release":950000},
      {"category":"formal_proof","half_life_blocks":555555,"cliff_blocks":5555,"max_release":920000},
      {"category":"on_chain","half_life_blocks":222222,"cliff_blocks":1111,"max_release":900000},
      {"category":"cryptographic","half_life_blocks":222222,"cliff_blocks":3333,"max_release":900000},
      {"category":"computational","half_life_blocks":333333,"cliff_blocks":2222,"max_release":880000},
      {"category":"peer_reviewed","half_life_blocks":111111,"cliff_blocks":5555,"max_release":850000},
      {"category":"replicated","half_life_blocks":111111,"cliff_blocks":3333,"max_release":880000},
      {"category":"oracle_feed","half_life_blocks":55555,"cliff_blocks":555,"max_release":800000},
      {"category":"attestation","half_life_blocks":77777,"cliff_blocks":2222,"max_release":800000},
      {"category":"contested","half_life_blocks":22222,"cliff_blocks":1111,"max_release":600000}
    ]
  '
  ok "Vesting rewards: epoch=50000 min_validators=11 (halved from prod)"

  # Group 9: Alignment
  patch '
    .app_state.alignment.params.observation_interval_blocks = 100 |
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
  ok "Alignment: 10 params"

  # Group 10: Autopoiesis (with multiplier states)
  patch '
    .app_state.autopoiesis.params.epoch_length_blocks = 100 |
    .app_state.autopoiesis.params.max_change_per_epoch_bps = 10000 |
    .app_state.autopoiesis.params.slash_multiplier_min = 500000 |
    .app_state.autopoiesis.params.slash_multiplier_max = 2000000 |
    .app_state.autopoiesis.params.ssi_critical_threshold = 250000 |
    .app_state.autopoiesis.params.ssi_stressed_threshold = 500000 |
    .app_state.autopoiesis.params.ssi_healthy_threshold = 750000 |
    .app_state.autopoiesis.params.enabled = true
  '
  ok "Autopoiesis: 8 params"

  # Group 11: Billing (with revenue split and dynamic pricing)
  patch '
    .app_state.billing.params.base_query_price = "1000000" |
    .app_state.billing.params.confidence_weight_bps = 200000 |
    .app_state.billing.params.novelty_weight_bps = 0 |
    .app_state.billing.params.freshness_weight_bps = 100000 |
    .app_state.billing.params.min_provider_stake = "100000000" |
    .app_state.billing.params.confidence_threshold = 500000 |
    .app_state.billing.params.freshness_window_blocks = 1000 |
    .app_state.billing.params.quote_validity_blocks = 100 |
    .app_state.billing.params.revenue_split = {"contributor_bps":550000,"protocol_bps":220000,"research_bps":130000,"burn_bps":100000} |
    .app_state.billing.params.dynamic_pricing_config = {"enabled":false,"target_query_cost_usd":"10000","manual_zrn_price_usd":"0","twap_window_blocks":1000,"staleness_blocks":5000,"min_cost_per_fact":"1000","max_cost_per_fact":"100000000"}
  '
  ok "Billing: 10 params (dynamic pricing disabled for testnet)"

  # Group 12: BVM
  patch '
    .app_state.bvm.params.max_bytecode_size = 65536 |
    .app_state.bvm.params.max_gas_per_call = 10000000 |
    .app_state.bvm.params.max_gas_per_block = 100000000 |
    .app_state.bvm.params.max_contracts_per_creator = 100 |
    .app_state.bvm.params.max_state_entries = 10000 |
    .app_state.bvm.params.deploy_cost = "5000000" |
    .app_state.bvm.params.max_schedule_gas = 1000000 |
    .app_state.bvm.params.schedule_horizon_blocks = 100000 |
    .app_state.bvm.params.current_bvm_version = 1 |
    .app_state.bvm.params.max_schedules_per_contract = 100
  '
  ok "BVM: 10 params"

  # Group 13: Capture challenge [TESTNET: halved periods]
  patch '
    .app_state.capture_challenge.params.min_challenge_stake = "10000000" |
    .app_state.capture_challenge.params.evidence_period_blocks = 2500 |
    .app_state.capture_challenge.params.review_period_blocks = 10000 |
    .app_state.capture_challenge.params.domain_pause_blocks = 500 |
    .app_state.capture_challenge.params.reward_rate_bps = 100000 |
    .app_state.capture_challenge.params.slash_rate_bps = 50000 |
    .app_state.capture_challenge.params.bounty_contribution_per_fact = "1000" |
    .app_state.capture_challenge.params.risk_analysis_interval = 500
  '
  ok "Capture challenge: evidence=2500 review=10000 (halved from prod)"

  # Group 14: Capture defense [TESTNET: halved periods]
  patch '
    .app_state.capture_defense.params.decay_epoch_blocks = 5000 |
    .app_state.capture_defense.params.min_verifications_for_score = 5 |
    .app_state.capture_defense.params.hhi_threshold = 250000 |
    .app_state.capture_defense.params.risk_analysis_interval = 500 |
    .app_state.capture_defense.params.history_retention_blocks = 25000 |
    .app_state.capture_defense.params.base_reputation_score = 500000 |
    .app_state.capture_defense.params.max_history_per_domain = 100
  '
  ok "Capture defense: decay=5000 retention=25000 (halved from prod)"

  # Group 15: Channels
  patch '
    .app_state.channels.params.min_deposit = "1000000" |
    .app_state.channels.params.min_timeout_blocks = 100 |
    .app_state.channels.params.max_timeout_blocks = 1000000 |
    .app_state.channels.params.dispute_window_blocks = 500 |
    .app_state.channels.params.default_settlement_freq = 100 |
    .app_state.channels.params.max_channels_per_pair = 10 |
    .app_state.channels.params.channel_open_fee = "100000"
  '
  ok "Channels: 7 params"

  # Group 16: Claiming pot
  patch '
    .app_state.claiming_pot.params.max_pots_active = 10 |
    .app_state.claiming_pot.params.min_claim_amount = "1000"
  '
  ok "Claiming pot: 2 params"

  # Group 17: Compute pool — no params to patch (module uses empty/minimal genesis)
  ok "Compute pool: uses default genesis (no params to patch)"

  # Group 18: Discovery [TESTNET: shorter profile expiry]
  patch '
    .app_state.discovery.params.min_registration_stake = "1000000" |
    .app_state.discovery.params.max_capabilities_per_agent = 20 |
    .app_state.discovery.params.profile_expiry_blocks = 50000
  '
  ok "Discovery: profile_expiry=50000 (halved from prod)"

  # Group 19: Disputes (with tier configs) [TESTNET: shorter escalation delay]
  patch '
    .app_state.disputes.params.max_active_disputes = 100 |
    .app_state.disputes.params.escalation_delay = 250 |
    .app_state.disputes.params.slash_rate_loser_bps = 500000 |
    .app_state.disputes.params.reward_rate_winner_bps = 400000 |
    .app_state.disputes.params.arbiter_reward_bps = 100000 |
    .app_state.disputes.params.tier_configs = [
      {"tier":1,"arbiter_count":3,"min_bond":"1000000","evidence_period":500,"voting_period":1000,"quorum_bps":500000,"majority_bps":666667},
      {"tier":2,"arbiter_count":7,"min_bond":"10000000","evidence_period":1000,"voting_period":2000,"quorum_bps":500000,"majority_bps":666667},
      {"tier":3,"arbiter_count":13,"min_bond":"100000000","evidence_period":2000,"voting_period":5000,"quorum_bps":600000,"majority_bps":750000},
      {"tier":4,"arbiter_count":21,"min_bond":"1000000000","evidence_period":5000,"voting_period":10000,"quorum_bps":666000,"majority_bps":800000}
    ]
  '
  ok "Disputes: escalation_delay=250 + 4 tier configs (halved from prod)"

  # Group 20: Evidence management [TESTNET: shorter challenge window]
  patch '
    .app_state.evidence_mgmt.params.min_verifier_tier = 2 |
    .app_state.evidence_mgmt.params.verification_quorum = 3 |
    .app_state.evidence_mgmt.params.challenge_bond = "500000" |
    .app_state.evidence_mgmt.params.challenge_window_blocks = 25000
  '
  ok "Evidence management: challenge_window=25000 (halved from prod)"

  # Group 21: Home [TESTNET: shorter session timeout]
  patch '
    .app_state.home.params.max_keys_per_home = 20 |
    .app_state.home.params.max_sessions_per_home = 5 |
    .app_state.home.params.session_timeout_blocks = 500 |
    .app_state.home.params.deadman_min_threshold = 100 |
    .app_state.home.params.deadman_max_threshold = 100000 |
    .app_state.home.params.max_alerts_per_home = 100 |
    .app_state.home.params.home_creation_fee = "10000000" |
    .app_state.home.params.max_recovery_addresses = 5
  '
  ok "Home: session_timeout=500 (halved from prod)"

  # Group 22: IBC rate limit
  patch '
    .app_state.ibcratelimit.params.enabled = true
  '
  ok "IBC rate limit: enabled"

  # Group 23: ICA auth
  patch '
    .app_state.icaauth.params.max_remote_accounts_per_owner = 5 |
    .app_state.icaauth.params.registration_cooldown = 100 |
    .app_state.icaauth.params.max_messages_per_tx = 5 |
    .app_state.icaauth.params.allowed_host_msg_types = [
      "/cosmos.bank.v1beta1.MsgSend",
      "/cosmos.staking.v1beta1.MsgDelegate",
      "/cosmos.staking.v1beta1.MsgUndelegate",
      "/cosmos.staking.v1beta1.MsgBeginRedelegate",
      "/cosmos.distribution.v1beta1.MsgWithdrawDelegatorReward",
      "/cosmos.gov.v1beta1.MsgVote"
    ]
  '
  ok "ICA auth: 4 params + 6 allowed msg types"

  # Group 24: Liquidity pool
  patch '
    .app_state.liquiditypool.params.default_swap_fee_bps = 3000 |
    .app_state.liquiditypool.params.max_pools = 3 |
    .app_state.liquiditypool.params.min_initial_liquidity = "10000000000" |
    .app_state.liquiditypool.params.twap_window_blocks = 1000 |
    .app_state.liquiditypool.params.protocol_fee_bps = 450000 |
    .app_state.liquiditypool.params.min_reserve = "1"
  '
  ok "Liquidity pool: 6 params"

  # Group 25: Ontology [TESTNET: shorter voting period]
  patch '
    .app_state.ontology.params.min_proposal_stake = "1000000" |
    .app_state.ontology.params.proposal_voting_period = 17136 |
    .app_state.ontology.params.min_endorsements = 3 |
    .app_state.ontology.params.cross_stratum_discount = 50000 |
    .app_state.ontology.params.max_domains_per_stratum = 100 |
    .app_state.ontology.params.allow_new_strata = false
  '
  ok "Ontology: proposal_voting=17136 (halved from prod)"

  # Group 26: Partnerships [TESTNET: halved durations]
  patch '
    .app_state.partnerships.params.formation_window_blocks = 500 |
    .app_state.partnerships.params.cooling_period_blocks = 2500 |
    .app_state.partnerships.params.common_pot_share_bps = 100000 |
    .app_state.partnerships.params.safety_freeze_duration_blocks = 250 |
    .app_state.partnerships.params.max_freezes_per_epoch = 3 |
    .app_state.partnerships.params.coercion_review_blocks = 1000 |
    .app_state.partnerships.params.base_cooldown_blocks = 50 |
    .app_state.partnerships.params.max_counter_proposal_depth = 3 |
    .app_state.partnerships.params.default_human_split_bps = 500000 |
    .app_state.partnerships.params.default_agent_split_bps = 500000 |
    .app_state.partnerships.params.min_partnership_stake = "1000000" |
    .app_state.partnerships.params.seed_partnership_duration = 5000 |
    .app_state.partnerships.params.seed_common_pot_cap = "100000000" |
    .app_state.partnerships.params.lock_tiers = [
      {"min_blocks":22222,"multiplier_bps":1000000,"exit_penalty_bps":110000},
      {"min_blocks":77777,"multiplier_bps":1110000,"exit_penalty_bps":220000},
      {"min_blocks":222222,"multiplier_bps":1220000,"exit_penalty_bps":330000},
      {"min_blocks":777777,"multiplier_bps":1440000,"exit_penalty_bps":440000},
      {"min_blocks":1111111,"multiplier_bps":1550000,"exit_penalty_bps":550000},
      {"min_blocks":2222222,"multiplier_bps":1770000,"exit_penalty_bps":660000}
    ]
  '
  ok "Partnerships: formation=500 cooling=2500 seed_duration=5000 (halved from prod)"

  # Group 27: Qualification [TESTNET: halved periods]
  patch '
    .app_state.qualification.params.min_stake_amount = "100000000" |
    .app_state.qualification.params.stake_lock_period = 50400 |
    .app_state.qualification.params.min_verifications = 100 |
    .app_state.qualification.params.min_accuracy_bps = 800000 |
    .app_state.qualification.params.min_reputation_score = 500000 |
    .app_state.qualification.params.qualification_period = 604800 |
    .app_state.qualification.params.probation_period = 151200 |
    .app_state.qualification.params.renewal_window = 50400 |
    .app_state.qualification.params.max_endorsements = 50 |
    .app_state.qualification.params.cross_ref_min_weight = 30 |
    .app_state.qualification.params.cross_ref_weight_discount_bps = 200000 |
    .app_state.qualification.params.inheritance_weight_discount_bps = 300000
  '
  ok "Qualification: stake_lock=50400 qualification=604800 (halved from prod)"

  # Group 28: Research [TESTNET: shorter review and bounty periods]
  patch '
    .app_state.research.params.min_research_stake = "1000000" |
    .app_state.research.params.min_challenge_stake = "1000000" |
    .app_state.research.params.review_period_blocks = 34272 |
    .app_state.research.params.min_reviewer_count = 3 |
    .app_state.research.params.acceptance_score_threshold = 70 |
    .app_state.research.params.rejection_slash_bps = 330000 |
    .app_state.research.params.max_bounty_reward = "10000000000" |
    .app_state.research.params.bounty_min_deadline_blocks = 17136
  '
  ok "Research: review=34272 bounty_deadline=17136 (halved from prod)"

  # Group 29: Schedule
  patch '
    .app_state.schedule.params.max_active_per_account = 20 |
    .app_state.schedule.params.max_gas_per_block = 50000000 |
    .app_state.schedule.params.min_interval_blocks = 10 |
    .app_state.schedule.params.min_fee_per_execution = "10000" |
    .app_state.schedule.params.max_compound_depth = 3
  '
  ok "Schedule: 5 params"

  # Group 30: Tokens
  patch '
    .app_state.tokens.params.emission_epoch_blocks = 0 |
    .app_state.tokens.params.default_fee_bps = ""
  '
  ok "Tokens: 2 params"

  # Group 31: Toolbox [TESTNET: shorter cooldowns and grace periods]
  patch '
    .app_state.toolbox.params.max_contributors = 22 |
    .app_state.toolbox.params.max_dependency_depth = 10 |
    .app_state.toolbox.params.max_dependencies = 20 |
    .app_state.toolbox.params.min_tool_stake = 11000000 |
    .app_state.toolbox.params.share_lock_cooldown_blocks = 17136 |
    .app_state.toolbox.params.deprecation_grace_blocks = 120000 |
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
    .app_state.toolbox.params.min_home_age_blocks = 5000 |
    .app_state.toolbox.params.free_calls_enabled = true |
    .app_state.toolbox.params.tool_revenue_bps = 550000 |
    .app_state.toolbox.params.protocol_bps = 220000 |
    .app_state.toolbox.params.research_bps = 130000 |
    .app_state.toolbox.params.burn_bps = 100000 |
    .app_state.toolbox.params.protocol_citation_bps = 500000 |
    .app_state.toolbox.params.protocol_verification_bps = 300000 |
    .app_state.toolbox.params.protocol_treasury_bps = 200000
  '
  ok "Toolbox: share_lock=17136 deprecation_grace=120000 min_home_age=5000 (halved from prod)"

  # Group 32: Tree [TESTNET: shorter deadlines and expiry]
  patch '
    .app_state.tree.params.min_budget = "1000000" |
    .app_state.tree.params.max_tasks_per_project = 200 |
    .app_state.tree.params.max_contributors = 50 |
    .app_state.tree.params.max_applications = 100 |
    .app_state.tree.params.task_deadline_min_blocks = 100 |
    .app_state.tree.params.task_deadline_max_blocks = 518400 |
    .app_state.tree.params.max_rejections = 3 |
    .app_state.tree.params.seed_expiry_blocks = 86400 |
    .app_state.tree.params.min_contributors_to_start = 1 |
    .app_state.tree.params.contributors_bp = 550000 |
    .app_state.tree.params.protocol_treasury_bp = 220000 |
    .app_state.tree.params.research_fund_bp = 130000 |
    .app_state.tree.params.burn_bp = 100000 |
    .app_state.tree.params.evidence_tax_bp = 220000
  '
  ok "Tree: deadline_max=518400 seed_expiry=86400 (halved from prod)"

  info "All 30 custom modules + SDK overrides patched"

  # ── Step 6b: Axiom injection hook ────────────────────────────────────
  if command -v go &>/dev/null && [ -f "${PROJECT_ROOT}/x/knowledge/types/genesis_axioms.json" ]; then
    if [ -d "${PROJECT_ROOT}/tools/axiom-loader" ]; then
      info "Injecting 777 genesis axioms..."
      (cd "${PROJECT_ROOT}" && go run tools/axiom-loader/main.go inject \
        x/knowledge/types/genesis_axioms.json \
        "${CEREMONY_HOME}/config/genesis.json") || warn "Axiom injection failed (non-fatal)"
    else
      warn "axiom-loader not found — skipping axiom injection"
    fi
  fi

  # Validate after all patches
  validate_genesis "param patching"

  # ── Step 7: Create bootstrap accounts ────────────────────────────────
  info "Creating bootstrap accounts..."

  # Foundation
  ${BINARY} keys add foundation --keyring-backend ${KEYRING} --home "${CEREMONY_HOME}" 2>/dev/null
  FOUNDATION_ADDR=$(${BINARY} keys show foundation -a --keyring-backend ${KEYRING} --home "${CEREMONY_HOME}")
  ${BINARY} add-genesis-account "${FOUNDATION_ADDR}" "${FOUNDATION_BALANCE}" --home "${CEREMONY_HOME}"
  info "  Foundation:        ${FOUNDATION_ADDR} (1M ZRN)"

  # Research treasury
  ${BINARY} keys add research-treasury --keyring-backend ${KEYRING} --home "${CEREMONY_HOME}" 2>/dev/null
  RESEARCH_ADDR=$(${BINARY} keys show research-treasury -a --keyring-backend ${KEYRING} --home "${CEREMONY_HOME}")
  ${BINARY} add-genesis-account "${RESEARCH_ADDR}" "${RESEARCH_BALANCE}" --home "${CEREMONY_HOME}"
  info "  Research Treasury: ${RESEARCH_ADDR} (500K ZRN)"

  # Faucet
  ${BINARY} keys add faucet --keyring-backend ${KEYRING} --home "${CEREMONY_HOME}" 2>/dev/null
  FAUCET_ADDR=$(${BINARY} keys show faucet -a --keyring-backend ${KEYRING} --home "${CEREMONY_HOME}")
  ${BINARY} add-genesis-account "${FAUCET_ADDR}" "${FAUCET_BALANCE}" --home "${CEREMONY_HOME}"
  info "  Faucet:            ${FAUCET_ADDR} (100K ZRN)"

  validate_genesis "bootstrap accounts"

  # ── Summary ──────────────────────────────────────────────────────────
  echo ""
  echo "═══════════════════════════════════════════════════════════════"
  echo "  Testnet Genesis INITIALIZED"
  echo "═══════════════════════════════════════════════════════════════"
  echo ""
  echo "  Chain ID:      ${CHAIN_ID}"
  echo "  Genesis Time:  ${GENESIS_TIME}"
  echo "  Ceremony Home: ${CEREMONY_HOME}"
  echo ""
  echo "  Bootstrap Accounts:"
  echo "    Foundation:        ${FOUNDATION_ADDR} (1,000,000 ZRN)"
  echo "    Research Treasury: ${RESEARCH_ADDR}   (500,000 ZRN)"
  echo "    Faucet:            ${FAUCET_ADDR}     (100,000 ZRN)"
  echo ""
  echo "  Testnet Tuning:"
  echo "    - Governance periods halved (voting=34272, discussion=17136)"
  echo "    - Emergency cooldowns halved (cooldown=55, epoch=17136)"
  echo "    - Staking unbonding halved (134280 blocks)"
  echo "    - Knowledge min_verifiers=2 (prod=3)"
  echo "    - Lower bootstrap balances (1M/500K/100K/100K)"
  echo ""
  echo "  Next: ./scripts/testnet-genesis.sh add-validator <name>"
  echo "═══════════════════════════════════════════════════════════════"
}

# ── cmd_add_validator ────────────────────────────────────────────────────

cmd_add_validator() {
  local name="${1:-}"
  [ -n "${name}" ] || die "Usage: $0 add-validator <NAME>"

  echo "═══════════════════════════════════════════════════════════════"
  echo "  Zerone — Testnet Genesis: ADD VALIDATOR '${name}'"
  echo "═══════════════════════════════════════════════════════════════"
  echo ""

  check_binary

  # ── Step 1: Validate ceremony home exists ────────────────────────────
  [ -d "${CEREMONY_HOME}" ] || die "Testnet genesis not initialized. Run: $0 init"
  [ -f "${CEREMONY_HOME}/config/genesis.json" ] || die "No genesis.json found. Run: $0 init"

  local val_home="${CEREMONY_HOME}/validators/${name}"

  if [ -d "${val_home}" ]; then
    die "Validator '${name}' already exists at ${val_home}"
  fi

  # ── Step 2: Generate unique consensus key in per-validator home ──────
  info "Initializing validator home..."
  ${BINARY} init "${name}" \
    --chain-id "${CHAIN_ID}" \
    --home "${val_home}" \
    --overwrite 2>/dev/null

  # ── Step 3: Copy coordinator genesis ─────────────────────────────────
  cp "${CEREMONY_HOME}/config/genesis.json" "${val_home}/config/genesis.json"

  # ── Step 4: Create validator account key in coordinator keyring ──────
  info "Creating validator key..."
  ${BINARY} keys add "${name}" \
    --keyring-backend ${KEYRING} \
    --home "${CEREMONY_HOME}" 2>/dev/null

  local val_addr
  val_addr=$(${BINARY} keys show "${name}" -a --keyring-backend ${KEYRING} --home "${CEREMONY_HOME}")

  # ── Step 5: Fund validator in coordinator genesis ────────────────────
  info "Funding validator: ${val_addr} (100K ZRN)"
  ${BINARY} add-genesis-account "${val_addr}" "${VALIDATOR_BALANCE}" \
    --home "${CEREMONY_HOME}"

  # ── Step 6: Copy updated genesis + keyring to validator home ─────────
  cp "${CEREMONY_HOME}/config/genesis.json" "${val_home}/config/genesis.json"
  cp -r "${CEREMONY_HOME}/keyring-test" "${val_home}/"

  # ── Step 7: Generate gentx ───────────────────────────────────────────
  info "Generating gentx (stake: 10K ZRN)..."
  mkdir -p "${CEREMONY_HOME}/config/gentx"

  ${BINARY} genesis gentx "${name}" "${VALIDATOR_STAKE}" \
    --chain-id "${CHAIN_ID}" \
    --keyring-backend ${KEYRING} \
    --home "${val_home}" \
    --moniker "${name}" \
    --commission-rate "0.10" \
    --commission-max-rate "0.20" \
    --commission-max-change-rate "0.01" \
    --output-document "${CEREMONY_HOME}/config/gentx/gentx-${name}.json" 2>/dev/null

  ok "Gentx generated"

  # ── Step 8: Save consensus key + node key for distribution ──────────
  local keys_dir="${CEREMONY_HOME}/validator-keys/${name}"
  mkdir -p "${keys_dir}"

  cp "${val_home}/config/priv_validator_key.json" "${keys_dir}/"
  cp "${val_home}/config/node_key.json" "${keys_dir}/"

  # Extract node ID
  local node_id
  node_id=$(${BINARY} tendermint show-node-id --home "${val_home}" 2>/dev/null || \
            ${BINARY} comet show-node-id --home "${val_home}" 2>/dev/null || \
            echo "unknown")
  echo "${node_id}" > "${keys_dir}/node_id.txt"

  # ── Summary ──────────────────────────────────────────────────────────
  echo ""
  echo "═══════════════════════════════════════════════════════════════"
  echo "  Validator '${name}' Added"
  echo "═══════════════════════════════════════════════════════════════"
  echo ""
  echo "  Address:  ${val_addr}"
  echo "  Balance:  100,000 ZRN"
  echo "  Stake:    10,000 ZRN"
  echo "  Node ID:  ${node_id}"
  echo ""
  echo "  Key Locations:"
  echo "    priv_validator_key.json: ${keys_dir}/priv_validator_key.json"
  echo "    node_key.json:           ${keys_dir}/node_key.json"
  echo "    gentx:                   ${CEREMONY_HOME}/config/gentx/gentx-${name}.json"
  echo ""
  echo "  Next: add more validators or run: $0 finalize"
  echo "═══════════════════════════════════════════════════════════════"
}

# ── cmd_finalize ─────────────────────────────────────────────────────────

cmd_finalize() {
  echo "═══════════════════════════════════════════════════════════════"
  echo "  Zerone — Testnet Genesis: FINALIZE"
  echo "═══════════════════════════════════════════════════════════════"
  echo ""

  check_binary

  [ -d "${CEREMONY_HOME}" ] || die "Testnet genesis not initialized. Run: $0 init"

  local gentx_dir="${CEREMONY_HOME}/config/gentx"
  [ -d "${gentx_dir}" ] || die "No gentx directory found. Add validators first."

  local gentx_count
  gentx_count=$(ls -1 "${gentx_dir}"/gentx-*.json 2>/dev/null | wc -l | tr -d ' ')
  [ "${gentx_count}" -gt 0 ] || die "No gentx files found. Add validators first."

  # ── Step 1: Collect all gentxs ───────────────────────────────────────
  info "Collecting ${gentx_count} gentxs..."
  ${BINARY} genesis collect-gentxs --home "${CEREMONY_HOME}" 2>/dev/null
  ok "Gentxs collected"

  # ── Step 2: Validate genesis ─────────────────────────────────────────
  info "Validating genesis..."
  validate_genesis "finalization"
  ok "Genesis validated"

  # ── Step 3: Print summary ────────────────────────────────────────────
  local genesis="${CEREMONY_HOME}/config/genesis.json"
  local genesis_hash
  genesis_hash=$(shasum -a 256 "${genesis}" | cut -d' ' -f1)
  local chain_id
  chain_id=$(jq -r '.chain_id' "${genesis}")
  local gen_time
  gen_time=$(jq -r '.genesis_time' "${genesis}")

  echo ""
  echo "═══════════════════════════════════════════════════════════════"
  echo "  Testnet Genesis FINALIZED"
  echo "═══════════════════════════════════════════════════════════════"
  echo ""
  echo "  Chain ID:     ${chain_id}"
  echo "  Genesis Time: ${gen_time}"
  echo "  Validators:   ${gentx_count}"
  echo "  Genesis Hash: ${genesis_hash}"
  echo ""
  echo "  Next: ./scripts/testnet-genesis.sh export"
  echo "═══════════════════════════════════════════════════════════════"
}

# ── cmd_export ───────────────────────────────────────────────────────────

cmd_export() {
  echo "═══════════════════════════════════════════════════════════════"
  echo "  Zerone — Testnet Genesis: EXPORT"
  echo "═══════════════════════════════════════════════════════════════"
  echo ""

  check_binary

  local genesis="${CEREMONY_HOME}/config/genesis.json"
  [ -f "${genesis}" ] || die "No genesis.json found. Run init + finalize first."

  # ── Step 1: Copy genesis.json to project root ────────────────────────
  info "Exporting genesis.json to project root..."
  cp "${genesis}" "${PROJECT_ROOT}/genesis.json"
  ok "Exported: ${PROJECT_ROOT}/genesis.json"

  # ── Step 2: Print chain info ─────────────────────────────────────────
  local chain_id gen_time account_count genesis_hash
  chain_id=$(jq -r '.chain_id' "${genesis}")
  gen_time=$(jq -r '.genesis_time' "${genesis}")
  account_count=$(jq '.app_state.auth.accounts | length' "${genesis}")
  genesis_hash=$(shasum -a 256 "${genesis}" | cut -d' ' -f1)

  echo ""
  echo "  Chain ID:     ${chain_id}"
  echo "  Genesis Time: ${gen_time}"
  echo "  Accounts:     ${account_count}"
  echo "  Genesis Hash: ${genesis_hash}"

  # ── Step 3: Extract validator node IDs ───────────────────────────────
  local keys_dir="${CEREMONY_HOME}/validator-keys"
  if [ -d "${keys_dir}" ]; then
    echo ""
    echo "  Validator Node IDs:"
    for val_dir in "${keys_dir}"/*/; do
      local val_name
      val_name=$(basename "${val_dir}")
      local node_id_file="${val_dir}/node_id.txt"
      if [ -f "${node_id_file}" ]; then
        local nid
        nid=$(cat "${node_id_file}")
        echo "    ${val_name}: ${nid}"
      fi
    done
  fi

  # ── Step 4: Print distribution instructions ──────────────────────────
  echo ""
  echo "═══════════════════════════════════════════════════════════════"
  echo "  Testnet Distribution Instructions"
  echo "═══════════════════════════════════════════════════════════════"
  echo ""
  echo "  Each validator operator must receive and install:"
  echo ""
  echo "  1. genesis.json -> ~/.zeroned/config/genesis.json"
  echo "     cp genesis.json ~/.zeroned/config/genesis.json"
  echo ""
  echo "  2. priv_validator_key.json -> ~/.zeroned/config/priv_validator_key.json"
  echo "     (Distribute securely — NEVER share publicly)"
  echo ""
  echo "  3. node_key.json -> ~/.zeroned/config/node_key.json"
  echo "     (Distribute securely)"
  echo ""
  echo "  4. Configure persistent_peers in ~/.zeroned/config/config.toml:"
  echo ""

  # Build persistent_peers string
  if [ -d "${keys_dir}" ]; then
    local peers=""
    for val_dir in "${keys_dir}"/*/; do
      local val_name
      val_name=$(basename "${val_dir}")
      local node_id_file="${val_dir}/node_id.txt"
      if [ -f "${node_id_file}" ]; then
        local nid
        nid=$(cat "${node_id_file}")
        if [ -n "${peers}" ]; then
          peers="${peers},"
        fi
        peers="${peers}${nid}@<${val_name}-ip>:26656"
      fi
    done
    echo "     persistent_peers = \"${peers}\""
    echo ""
    echo "     Replace <name-ip> with each validator's public IP address."
  fi

  echo ""
  echo "  5. Start the node:"
  echo "     zeroned start --minimum-gas-prices 0.025uzrn"
  echo ""
  echo "  Next: ./scripts/testnet-genesis.sh verify"
  echo "═══════════════════════════════════════════════════════════════"
}

# ── cmd_verify ───────────────────────────────────────────────────────────

cmd_verify() {
  local rpc_url="${RPC_URL:-http://localhost:26657}"

  echo "═══════════════════════════════════════════════════════════════"
  echo "  Zerone — Testnet Genesis: VERIFY"
  echo "  Querying node at ${rpc_url}"
  echo "═══════════════════════════════════════════════════════════════"
  echo ""

  command -v curl >/dev/null 2>&1 || die "curl required for verify subcommand."
  command -v jq   >/dev/null 2>&1 || die "jq >= 1.6 required."

  # ── Check if node is reachable ───────────────────────────────────────
  if ! curl -s "${rpc_url}/status" >/dev/null 2>&1; then
    die "Node not reachable at ${rpc_url}. Start with: zeroned start --minimum-gas-prices 0.025uzrn"
  fi

  local pass=0
  local fail=0
  local total=0

  verify_eq() {
    local label="$1" expected="$2" actual="$3"
    total=$((total + 1))
    if [ "${expected}" = "${actual}" ]; then
      ok "${label}: ${actual}"
      pass=$((pass + 1))
    else
      warn "${label}: expected ${expected}, got ${actual}"
      fail=$((fail + 1))
    fi
  }

  # ── Query chain-id ──────────────────────────────────────────────────
  info "Checking chain identity..."
  local status_json
  status_json=$(curl -s "${rpc_url}/status")

  local chain_id
  chain_id=$(echo "${status_json}" | jq -r '.result.node_info.network')
  verify_eq "Chain ID" "${CHAIN_ID}" "${chain_id}"

  # ── Query block height ──────────────────────────────────────────────
  local height
  height=$(echo "${status_json}" | jq -r '.result.sync_info.latest_block_height')
  ok "Block height: ${height}"

  local voting_power
  voting_power=$(echo "${status_json}" | jq -r '.result.validator_info.voting_power // "unknown"')
  ok "Voting power: ${voting_power}"

  # ── Query consensus params ──────────────────────────────────────────
  info "Checking consensus parameters..."
  local consensus_json
  consensus_json=$(curl -s "${rpc_url}/consensus_params")

  local max_gas
  max_gas=$(echo "${consensus_json}" | jq -r '.result.consensus_params.block.max_gas')
  verify_eq "Consensus max_gas" "33333333" "${max_gas}"

  local max_bytes
  max_bytes=$(echo "${consensus_json}" | jq -r '.result.consensus_params.block.max_bytes')
  verify_eq "Consensus max_bytes" "4194304" "${max_bytes}"

  # ── Query genesis for module params ─────────────────────────────────
  info "Checking module parameters from genesis..."
  local genesis_json
  genesis_json=$(curl -s "${rpc_url}/genesis")

  # Staking bond denom
  local bond_denom
  bond_denom=$(echo "${genesis_json}" | jq -r '.result.genesis.app_state.staking.params.bond_denom')
  verify_eq "Staking bond_denom" "uzrn" "${bond_denom}"

  # Knowledge min_verifiers
  local min_verifiers
  min_verifiers=$(echo "${genesis_json}" | jq -r '.result.genesis.app_state.knowledge.params.min_verifiers')
  verify_eq "Knowledge min_verifiers" "2" "${min_verifiers}"

  # Knowledge challenge_duration_blocks
  local challenge_duration
  challenge_duration=$(echo "${genesis_json}" | jq -r '.result.genesis.app_state.knowledge.params.challenge_duration_blocks')
  verify_eq "Knowledge challenge_duration_blocks" "17136" "${challenge_duration}"

  # Governance voting_period_blocks
  local voting_period
  voting_period=$(echo "${genesis_json}" | jq -r '.result.genesis.app_state.zerone_gov.params.voting_period_blocks')
  verify_eq "Governance voting_period_blocks" "34272" "${voting_period}"

  # Emergency cooldown_blocks
  local cooldown
  cooldown=$(echo "${genesis_json}" | jq -r '.result.genesis.app_state.emergency.params.cooldown_blocks')
  verify_eq "Emergency cooldown_blocks" "55" "${cooldown}"

  # Emergency epoch_blocks
  local epoch_blocks
  epoch_blocks=$(echo "${genesis_json}" | jq -r '.result.genesis.app_state.emergency.params.epoch_blocks')
  verify_eq "Emergency epoch_blocks" "17136" "${epoch_blocks}"

  # Zerone staking unbonding_period
  local unbonding
  unbonding=$(echo "${genesis_json}" | jq -r '.result.genesis.app_state.zerone_staking.params.unbonding_period')
  verify_eq "Zerone staking unbonding_period" "134280" "${unbonding}"

  # Vesting rewards blocks_per_reward_epoch
  local reward_epoch
  reward_epoch=$(echo "${genesis_json}" | jq -r '.result.genesis.app_state.vesting_rewards.params.blocks_per_reward_epoch')
  verify_eq "Vesting blocks_per_reward_epoch" "50000" "${reward_epoch}"

  # Vesting rewards min_validators_for_full_reward
  local min_validators
  min_validators=$(echo "${genesis_json}" | jq -r '.result.genesis.app_state.vesting_rewards.params.min_validators_for_full_reward')
  verify_eq "Vesting min_validators_for_full_reward" "11" "${min_validators}"

  # Capture challenge evidence_period_blocks
  local evidence_period
  evidence_period=$(echo "${genesis_json}" | jq -r '.result.genesis.app_state.capture_challenge.params.evidence_period_blocks')
  verify_eq "Capture challenge evidence_period" "2500" "${evidence_period}"

  # Capture defense decay_epoch_blocks
  local decay_epoch
  decay_epoch=$(echo "${genesis_json}" | jq -r '.result.genesis.app_state.capture_defense.params.decay_epoch_blocks')
  verify_eq "Capture defense decay_epoch" "5000" "${decay_epoch}"

  # Discovery profile_expiry_blocks
  local profile_expiry
  profile_expiry=$(echo "${genesis_json}" | jq -r '.result.genesis.app_state.discovery.params.profile_expiry_blocks')
  verify_eq "Discovery profile_expiry" "50000" "${profile_expiry}"

  # Disputes escalation_delay
  local escalation_delay
  escalation_delay=$(echo "${genesis_json}" | jq -r '.result.genesis.app_state.disputes.params.escalation_delay')
  verify_eq "Disputes escalation_delay" "250" "${escalation_delay}"

  # Evidence mgmt challenge_window_blocks
  local evidence_window
  evidence_window=$(echo "${genesis_json}" | jq -r '.result.genesis.app_state.evidence_mgmt.params.challenge_window_blocks')
  verify_eq "Evidence mgmt challenge_window" "25000" "${evidence_window}"

  # Home session_timeout_blocks
  local session_timeout
  session_timeout=$(echo "${genesis_json}" | jq -r '.result.genesis.app_state.home.params.session_timeout_blocks')
  verify_eq "Home session_timeout" "500" "${session_timeout}"

  # Ontology proposal_voting_period
  local ontology_voting
  ontology_voting=$(echo "${genesis_json}" | jq -r '.result.genesis.app_state.ontology.params.proposal_voting_period')
  verify_eq "Ontology proposal_voting_period" "17136" "${ontology_voting}"

  # Partnerships formation_window_blocks
  local formation_window
  formation_window=$(echo "${genesis_json}" | jq -r '.result.genesis.app_state.partnerships.params.formation_window_blocks')
  verify_eq "Partnerships formation_window" "500" "${formation_window}"

  # Qualification stake_lock_period
  local stake_lock
  stake_lock=$(echo "${genesis_json}" | jq -r '.result.genesis.app_state.qualification.params.stake_lock_period')
  verify_eq "Qualification stake_lock_period" "50400" "${stake_lock}"

  # Research review_period_blocks
  local research_review
  research_review=$(echo "${genesis_json}" | jq -r '.result.genesis.app_state.research.params.review_period_blocks')
  verify_eq "Research review_period" "34272" "${research_review}"

  # Toolbox share_lock_cooldown_blocks
  local toolbox_lock
  toolbox_lock=$(echo "${genesis_json}" | jq -r '.result.genesis.app_state.toolbox.params.share_lock_cooldown_blocks')
  verify_eq "Toolbox share_lock_cooldown" "17136" "${toolbox_lock}"

  # Tree task_deadline_max_blocks
  local tree_deadline
  tree_deadline=$(echo "${genesis_json}" | jq -r '.result.genesis.app_state.tree.params.task_deadline_max_blocks')
  verify_eq "Tree task_deadline_max" "518400" "${tree_deadline}"

  # ── Summary ──────────────────────────────────────────────────────────
  echo ""
  echo "═══════════════════════════════════════════════════════════════"
  echo "  Testnet Genesis Verification Summary"
  echo "═══════════════════════════════════════════════════════════════"
  echo ""
  echo "  Node:    ${rpc_url}"
  echo "  Chain:   ${chain_id}"
  echo "  Height:  ${height}"
  echo ""
  echo "  Checks:  ${total} total"
  echo "  Passed:  ${pass}"
  echo "  Failed:  ${fail}"
  echo ""

  if [ "${fail}" -gt 0 ]; then
    warn "${fail} parameter(s) did not match expected testnet values."
    warn "This may indicate a configuration drift or non-testnet genesis."
    echo ""
    echo "  Tip: Re-run 'init' to regenerate a clean testnet genesis."
  else
    ok "All testnet genesis parameters verified successfully."
  fi

  echo ""
  echo "  Override RPC endpoint: RPC_URL=http://host:port $0 verify"
  echo "═══════════════════════════════════════════════════════════════"

  # Return non-zero if any checks failed
  [ "${fail}" -eq 0 ]
}

# ── Main ─────────────────────────────────────────────────────────────────

case "${1:-help}" in
  init)
    cmd_init
    ;;
  add-validator)
    cmd_add_validator "${2:-}"
    ;;
  finalize)
    cmd_finalize
    ;;
  export)
    cmd_export
    ;;
  verify)
    cmd_verify
    ;;
  help|--help|-h)
    echo "Usage: $0 {init|add-validator <NAME>|finalize|export|verify}"
    echo ""
    echo "Commands:"
    echo "  init                Initialize testnet genesis (build, patch params, create accounts)"
    echo "  add-validator NAME  Add a validator (generate keys, fund, create gentx)"
    echo "  finalize            Collect gentxs and validate genesis"
    echo "  export              Export genesis.json and distribution instructions"
    echo "  verify              Verify testnet params on a running node"
    echo ""
    echo "Full ceremony flow:"
    echo "  $0 init"
    echo "  $0 add-validator val1"
    echo "  $0 add-validator val2"
    echo "  $0 add-validator val3"
    echo "  $0 finalize"
    echo "  $0 export"
    echo "  $0 verify   # after node is running"
    echo ""
    echo "Testnet Differences from Production:"
    echo "  - Lower bootstrap balances (1M/500K/100K vs 10M/5M/500K)"
    echo "  - Lower validator stake (10K vs 100K)"
    echo "  - Governance periods halved"
    echo "  - Emergency cooldowns halved"
    echo "  - Staking unbonding halved"
    echo "  - Knowledge min_verifiers=2 (prod=3)"
    echo ""
    echo "State directory: ${CEREMONY_HOME}"
    echo "Chain ID:        ${CHAIN_ID}"
    echo ""
    echo "Environment:"
    echo "  RPC_URL    Override RPC endpoint for verify (default: http://localhost:26657)"
    ;;
  *)
    die "Unknown command: $1. Use --help for usage."
    ;;
esac
