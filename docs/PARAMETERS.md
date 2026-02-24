# Zerone Parameters Reference

> **Testnet Note (`zerone-testnet-1`):** The parameters below are initial
> testnet values chosen for rapid iteration and observable dynamics. They are
> deliberately more aggressive than mainnet targets — shorter epochs, lower
> quorums, and smaller stake requirements — so that governance, slashing, and
> economic flows can be tested within hours rather than days. Mainnet values
> will be established through governance proposals during the testnet phase.
> Parameters marked with bold defaults in the tables below are most likely to
> change before mainnet.

Complete reference for all governance-adjustable parameters across Zerone's
30 custom modules. All BPS (basis points) values use a 1,000,000 scale
(1,000,000 = 100%). Token amounts are in `uzrn` (1 ZRN = 1,000,000 uzrn).

## Table of Contents

- [staking](#staking)
- [knowledge](#knowledge)
- [auth](#auth)
- [vesting_rewards](#vesting_rewards)
- [gov](#gov)
- [ontology](#ontology)
- [billing](#billing)
- [channels](#channels)
- [bvm](#bvm)
- [emergency](#emergency)
- [disputes](#disputes)
- [research](#research)
- [qualification](#qualification)
- [capture_defense](#capture_defense)
- [capture_challenge](#capture_challenge)
- [autopoiesis](#autopoiesis)
- [alignment](#alignment)
- [compute_pool](#compute_pool)
- [discovery](#discovery)
- [home](#home)
- [tree](#tree)
- [toolbox](#toolbox)
- [schedule](#schedule)
- [partnerships](#partnerships)
- [liquiditypool](#liquiditypool)
- [claiming_pot](#claiming_pot)
- [evidence_mgmt](#evidence_mgmt)
- [ibcratelimit](#ibcratelimit)
- [icaauth](#icaauth)
- [tokens](#tokens)
- [Proposing Parameter Changes](#proposing-parameter-changes)

---

## staking

Core Proof of Truth validator staking parameters.

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `unbonding_period` | uint64 | 268,560 | Unbonding duration in blocks (~7 days) |
| `virtual_stake` | string | "11000000" (11 ZRN) | Virtual stake for tier 0/1 VRF participation |
| `max_validators` | uint64 | 100 | Maximum active validators (Scholar+ tiers) |
| `min_self_delegation` | string | "111000" (0.111 ZRN) | Minimum self-delegation to register |
| `max_slashes_per_epoch` | uint64 | 2 | Max slashes before deactivation per epoch |
| `slash_decay_period_blocks` | uint64 | 34,272 | Slash count decay period (~1 day) |
| `max_slash_count_deactivate` | uint64 | 3 | Cumulative slashes to trigger deactivation |
| `min_stake_for_verification` | string | "111000" (0.111 ZRN) | Minimum stake to participate in verification |
| `slash_escalation_bps` | uint64 | 100,000 (10%) | Progressive penalty increase per successive slash |
| `reputation_correct_delta` | uint64 | 100 (+0.01%) | Reputation increase per correct verification |
| `reputation_incorrect_delta` | uint64 | 200 (-0.02%) | Reputation decrease per incorrect verification |
| `reputation_slash_delta` | uint64 | 10,000 (-1%) | Reputation decrease per slash event |
| `redelegation_cooldown_blocks` | uint64 | 1,111 | Cooldown between redelegations (~46 min) |

### Tier Configurations

Each tier has the following adjustable fields:

| Tier | min_stake | min_reputation | min_verifications | min_accuracy | reward_multiplier_bps | selection_weight_bps | slash_multiplier_bps |
|------|-----------|----------------|-------------------|-------------|----------------------|---------------------|---------------------|
| Apprentice | "111000" (0.111 ZRN) | 0 | 0 | 0 | 100 (0.1x) | 100 (0.1x) | 1,500 (1.5x) |
| Verified | "1110000" (1.11 ZRN) | 770,000 (77%) | 22 | 770,000 (77%) | 500 (0.5x) | 500 (0.5x) | 1,200 (1.2x) |
| Scholar | "1111000000" (1,111 ZRN) | 500,000 (50%) | 11 | 500,000 (50%) | 1,000 (1.0x) | 1,000 (1.0x) | 1,000 (1.0x) |
| Guardian | "11111000000" (11,111 ZRN) | 770,000 (77%) | 333 | 770,000 (77%) | 2,000 (2.0x) | 1,500 (1.5x) | 1,000 (1.0x) |

---

## knowledge

Proof of Truth knowledge verification parameters — the largest parameter set.

### Core Verification

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `min_verifiers` | uint64 | 3 | Minimum verifiers per round |
| `max_verifiers` | uint64 | 22 | Maximum verifiers per round |
| `commit_phase_blocks` | uint64 | 4 | Duration of commit phase |
| `reveal_phase_blocks` | uint64 | 4 | Duration of reveal phase |
| `aggregation_phase_blocks` | uint64 | 3 | Duration of aggregation phase |
| `claim_cooldown_blocks` | uint64 | 50 | Cooldown between claims by same submitter |
| `max_validators_per_round` | uint64 | 22 | Max validators selected per verification round |

### Confidence Scoring

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `initial_confidence` | uint64 | 500,000 (50%) | Starting confidence for new claims |
| `confidence_boost_per_verification` | uint64 | 50,000 (5%) | Confidence increase per verification |
| `confidence_threshold` | uint64 | 770,000 (77%) | Threshold for claim acceptance |
| `quorum_threshold` | uint64 | 660,000 (66%) | Minimum participation for valid round |
| `confidence_growth_epoch` | uint64 | 1,111 | Blocks per confidence growth epoch |
| `confidence_growth_per_epoch_bps` | uint64 | 11,000 (1.1%) | Confidence growth rate per epoch |
| `max_survival_confidence` | uint64 | 770,000 (77%) | Max confidence from natural growth |
| `survived_challenge_confidence_cap` | uint64 | 880,000 (88%) | Max confidence after surviving challenge |

### Slashing (all must be non-zero per B22-3 audit)

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `wrong_verification_slash_bps` | uint64 | 50,000 (5%) | **Penalty for incorrect verification** |
| `missed_reveal_slash_bps` | uint64 | 100,000 (10%) | **Penalty for missing reveal phase** |
| `equivocation_slash_bps` | uint64 | 200,000 (20%) | **Penalty for conflicting votes** |
| `invalid_claim_slash_bps` | uint64 | 220,000 (22%) | **Penalty for invalid claim submission** |

### Rewards

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `verification_reward` | string | "3000000" (3 ZRN) | Reward per correct verification |
| `verification_reward_decay_bps` | uint64 | 999,000 (0.999x) | Decay rate per epoch |

### Claim Validation

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `min_claim_text_length` | uint64 | 20 | Minimum claim text length (chars) |
| `max_claim_text_length` | uint64 | 10,000 | Maximum claim text length (chars) |
| `min_claim_stake` | string | "1000000" (1 ZRN) | Minimum stake to submit a claim |

### Adversarial Verification

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `adversarial_verification_enabled` | bool | true | Enable adversarial challenge system |
| `provisional_threshold` | uint64 | 500,000 (50%) | Confidence threshold for provisional status |
| `reject_threshold` | uint64 | 300,000 (30%) | Confidence below which claims are rejected |
| `challenge_duration_blocks` | uint64 | 34,272 (~1 day) | Duration of challenge window |
| `min_challenge_stake` | string | "11000000" (11 ZRN) | Minimum stake to challenge a fact |
| `failed_challenge_slash_bps` | uint64 | 220,000 (22%) | Penalty for losing a challenge |
| `successful_challenge_reward_bps` | uint64 | 300,000 (30%) | Reward for winning a challenge |
| `max_concurrent_challenges` | uint64 | 3 | Max simultaneous challenges per fact |

### Citation Economics

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `citation_share_bps` | uint64 | 150,000 (15%) | Share of rewards to cited fact authors |
| `cross_domain_bonus_bps` | uint64 | 200,000 (20%) | Bonus for cross-domain citations |
| `max_citations_per_claim` | uint64 | 50 | Maximum citations per claim |
| `citation_decay_per_level` | uint64 | 500,000 (50%) | Citation reward decay per ancestor level |
| `self_citation_discount_bps` | uint64 | 500,000 (50%) | Discount on self-citations |

### Domain & Fact Limits

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `max_facts_per_domain` | uint64 | 100,000 | Maximum facts stored per domain |
| `fact_expiry_blocks` | uint64 | 0 | Fact expiry (0 = no expiry) |
| `cross_stratum_discount_bps` | uint64 | 0 | Discount for cross-stratum claims |
| `novelty_bonus_bps` | uint64 | 0 | Bonus for novel claims |
| `max_apprentice_validators` | uint64 | 111 | Sybil cap on Apprentice validators |

### FARM Anti-Gaming

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `conformity_threshold_bps` | uint64 | 950,000 | FARM-1: conformity detection threshold |
| `calibration_trivial_threshold` | uint64 | 950,000 | FARM-2: trivial claim detection |
| `misbehavior_rejection_threshold` | uint64 | 300,000 | FARM-6: misbehavior rejection |
| `min_domain_contributors_for_novelty` | uint64 | 3 | FARM-7: minimum contributors for novelty bonus |
| `min_participation_rate_bps` | uint64 | 500,000 | FARM-8: minimum participation rate |
| `challenge_stake_ratio_min_bps` | uint64 | 500,000 | FARM-9: minimum challenge stake ratio |

### Research Fund

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `research_fund_share_bps` | uint64 | 130,000 (13%) | Share of knowledge rewards to research fund |

---

## auth

Account registration, session keys, and recovery.

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `max_session_keys` | uint32 | 5 | Maximum active session keys per account |
| `max_session_duration` | uint64 | 34,272 (~1 day) | Maximum session key duration in blocks |
| `key_rotation_cooldown` | uint64 | 111 | Blocks between key rotations |
| `recovery_delay_blocks` | uint64 | 1,000 (~42 min) | Delay before recovery can execute |
| `challenge_period_blocks` | uint64 | 500 (~21 min) | Window to challenge a recovery |
| `bootstrap_enabled` | bool | false | Enable account bootstrap funding |
| `bootstrap_amount` | string | "0" | Bootstrap funding amount (uzrn) |
| `max_metadata_length` | uint32 | 1,024 | Maximum account metadata length (bytes) |
| `require_did` | bool | false | Require DID for account registration |
| `max_recovery_shards` | uint32 | 10 | Maximum recovery shards per account |
| `recovery_challenge_period_blocks` | uint64 | 500 | Recovery challenge period |
| `recovery_execution_delay_blocks` | uint64 | 1,000 | Delay before recovery execution |

---

## vesting_rewards

Block rewards, vesting curves, and revenue distribution.

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `block_reward` | string | "10000000" (10 ZRN) | Base block reward |
| `reward_decay_bps` | uint64 | 994,478 (~1-year half-life) | Reward decay per epoch (0.994478x) |
| `blocks_per_reward_epoch` | uint64 | 100,000 (~2.9 days) | Blocks per reward epoch |
| `founder_share_bps` | uint64 | 70,000 (7%) | Founder share of research fund |
| `founder_address` | string | "" (disabled) | Founder address for share distribution |
| `governance_activation_height` | uint64 | 0 | Height at which governance takes over |
| `vesting_enabled` | bool | true | Enable vesting mechanics |
| `released_clawback_rate` | uint64 | 3,300 (33%) | Clawback rate on released vesting |
| `min_validators_for_full_reward` | uint32 | 22 | Minimum validators for full block reward |
| `empty_block_reward_rate` | uint64 | 0 (0%) | Reward rate for blocks with no PoT activity |
| `floor_reward` | string | "100000" (0.1 ZRN) | Minimum block reward floor |
| `initial_fund_balance` | string | "0" | Initial fund balance (pure PoT) |

### Revenue Split

| Parameter | Default | Description |
|-----------|---------|-------------|
| `contributor_bps` | 550,000 (55%) | Share to fact contributors |
| `protocol_bps` | 220,000 (22%) | Share to protocol |
| `research_bps` | 130,000 (13%) | Share to research fund |
| `burn_bps` | 100,000 (10%) | Share burned |

### Protocol Sub-Split

| Parameter | Default | Description |
|-----------|---------|-------------|
| `citation_bps` | 500,000 (50%) | Citation rewards share |
| `verification_bps` | 300,000 (30%) | Verification rewards share |
| `treasury_bps` | 200,000 (20%) | Treasury share |

### Category Reward Multipliers

| Category | Multiplier |
|----------|-----------|
| Axiomatic | 1.2x |
| Formal Proof | 1.1x |
| On-Chain | 1.0x |
| Cryptographic | 1.05x |
| Computational | 1.0x |
| Peer Reviewed | 0.9x |
| Replicated | 0.95x |
| Oracle Feed | 0.8x |
| Attestation | 0.85x |
| Contested | 0.6x |

---

## gov

Governance and Living Improvement Proposals (LIPs).

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `voting_period_blocks` | uint64 | 102,816 (~3 days) | Voting period duration |
| `discussion_period_blocks` | uint64 | 68,544 (~2 days) | Discussion period before voting |
| `quorum_threshold_bps` | uint64 | 334,000 (33.4%) | Minimum participation for valid vote |
| `support_threshold_bps` | uint64 | 500,000 (50%) | Minimum support for proposal to pass |
| `min_lip_stake` | string | "1000000" (1 ZRN) | Minimum stake to submit a LIP |
| `min_vote_stake` | string | "0" | Minimum stake to vote (0 = no minimum) |
| `research_discussion_blocks` | uint64 | 68,544 | Research spend discussion period |
| `research_voting_blocks` | uint64 | 102,816 | Research spend voting period |

### Category Configurations

| Category | Required Stake | Review Period |
|----------|---------------|---------------|
| Parameter | 1,000 ZRN | ~1 day (34,272 blocks) |
| Upgrade | 800 ZRN | ~1 day (34,272 blocks) |
| Text | 400 ZRN | ~12h (17,136 blocks) |
| Research Spend | 200 ZRN | ~12h (17,136 blocks) |

---

## ontology

Epistemic domain and stratum management.

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `min_proposal_stake` | string | "1000000" (1 ZRN) | Minimum stake for domain proposals |
| `proposal_voting_period` | uint64 | 34,272 (~1 day) | Domain proposal voting period |
| `min_endorsements` | uint32 | 3 | Minimum endorsements for domain approval |
| `cross_stratum_discount` | uint64 | 50,000 (5%) | Cross-stratum verification discount |
| `max_domains_per_stratum` | uint32 | 100 | Maximum domains per epistemic stratum |
| `allow_new_strata` | bool | false | Allow creation of new strata via governance |

---

## billing

Knowledge query pricing and revenue distribution.

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `base_query_price` | string | "1000000" (1 ZRN) | Base price per fact query |
| `confidence_weight_bps` | uint64 | 200,000 (20%) | Price adjustment for confidence level |
| `novelty_weight_bps` | uint64 | 0 | Price adjustment for novelty |
| `freshness_weight_bps` | uint64 | 100,000 (10%) | Price premium for fresh facts |
| `min_provider_stake` | string | "100000000" (100 ZRN) | Minimum stake to register as provider |
| `confidence_threshold` | uint64 | 500,000 (50%) | Minimum confidence for billable queries |
| `freshness_window_blocks` | uint64 | 1,000 | Freshness window duration |
| `quote_validity_blocks` | uint64 | 100 | Quote validity duration |

### Dynamic Pricing

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `enabled` | bool | false | Enable dynamic pricing oracle |
| `target_query_cost_usd` | string | "10000" ($0.01) | Target query cost in 6-decimal USD |
| `manual_zrn_price_usd` | string | "0" (disabled) | Manual ZRN/USD price override |
| `twap_window_blocks` | uint64 | 1,000 | TWAP oracle window |
| `staleness_blocks` | uint64 | 5,000 | Price staleness threshold |
| `min_cost_per_fact` | string | "1000" (0.001 ZRN) | Minimum fact query cost |
| `max_cost_per_fact` | string | "100000000" (100 ZRN) | Maximum fact query cost |

---

## channels

Payment channel parameters.

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `min_deposit` | string | "1000000" (1 ZRN) | Minimum channel deposit |
| `min_timeout_blocks` | uint64 | 100 | Minimum channel timeout |
| `max_timeout_blocks` | uint64 | 1,000,000 | Maximum channel timeout |
| `dispute_window_blocks` | uint64 | 500 | Window for channel disputes |
| `default_settlement_freq` | uint64 | 100 | Default on-chain settlement frequency |
| `max_channels_per_pair` | uint64 | 10 | Maximum channels between two parties |
| `channel_open_fee` | string | "100000" (0.1 ZRN) | Fee to open a channel |

---

## bvm

Bytecode Virtual Machine (smart contract) parameters.

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `max_bytecode_size` | uint64 | 65,536 (64 KB) | Maximum contract bytecode size |
| `max_gas_per_call` | uint64 | 10,000,000 | Maximum gas per contract call |
| `max_gas_per_block` | uint64 | 100,000,000 | Maximum total gas for BVM per block |
| `max_contracts_per_creator` | uint64 | 100 | Maximum contracts per creator |
| `max_state_entries` | uint64 | 10,000 | Maximum state entries per contract |
| `deploy_cost` | string | "5000000" (5 ZRN) | Contract deployment cost |
| `max_schedule_gas` | uint64 | 1,000,000 | Maximum gas for scheduled execution |
| `schedule_horizon_blocks` | uint64 | 100,000 | Maximum future blocks for scheduling |
| `current_bvm_version` | uint32 | 1 | Current BVM version |
| `max_schedules_per_contract` | uint64 | 100 | Maximum scheduled executions per contract |

---

## emergency

Emergency halt, revert, and resume parameters.

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `halt_quorum` | uint64 | 750,000 (75%) | **Quorum for emergency halt** |
| `revert_quorum` | uint64 | 800,000 (80%) | **Quorum for state revert** |
| `resume_quorum` | uint64 | 800,000 (80%) | **Quorum for resuming chain** |
| `halt_prevote_blocks` | uint64 | 11 | Halt prevote phase duration |
| `halt_precommit_blocks` | uint64 | 11 | Halt precommit phase duration |
| `halt_timeout_blocks` | uint64 | 44 | Halt timeout |
| `revert_prevote_blocks` | uint64 | 22 | Revert prevote phase duration |
| `revert_precommit_blocks` | uint64 | 22 | Revert precommit phase duration |
| `revert_timeout_blocks` | uint64 | 111 | Revert timeout |
| `resume_prevote_blocks` | uint64 | 22 | Resume prevote phase duration |
| `resume_precommit_blocks` | uint64 | 22 | Resume precommit phase duration |
| `resume_timeout_blocks` | uint64 | 111 | Resume timeout |
| `max_proposals_per_epoch` | uint64 | 3 | Max emergency proposals per epoch |
| `max_proposals_per_guardian_per_epoch` | uint64 | 1 | Max proposals per guardian per epoch |
| `cooldown_blocks` | uint64 | 111 | Cooldown between emergency proposals |
| `min_guardian_stake` | string | "111111000000" (111,111 ZRN) | **Minimum stake for emergency proposer** |
| `min_distinct_voters` | uint64 | 4 | Minimum distinct voters for quorum |
| `max_revert_depth` | uint64 | 111,111 | Maximum revert depth (blocks) |
| `epoch_blocks` | uint64 | 34,272 (~1 day) | Emergency epoch duration |
| `max_halt_duration_blocks` | uint64 | 34,272 (~1 day) | Auto-resume after halt |
| `council_virtual_stake` | string | "11111000000" (11,111 ZRN) | Genesis council virtual stake |
| `council_expiry_block` | uint64 | 0 | Block at which genesis council expires |

---

## disputes

Multi-tier dispute resolution.

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `max_active_disputes` | uint32 | 100 | Maximum concurrent disputes |
| `escalation_delay` | uint64 | 500 | Blocks before dispute can escalate |
| `slash_rate_loser_bps` | uint64 | 500,000 (50%) | Slash rate for losing party |
| `reward_rate_winner_bps` | uint64 | 400,000 (40%) | Reward for winning party (of slash) |
| `arbiter_reward_bps` | uint64 | 100,000 (10%) | Reward for arbiters (of slash) |

### Dispute Tier Configurations

| Tier | Arbiters | Min Bond | Evidence Period | Voting Period | Quorum | Majority |
|------|----------|----------|-----------------|---------------|--------|----------|
| 1 | 3 | 1 ZRN | 500 blocks | 1,000 blocks | 50% | 66.7% |
| 2 | 7 | 10 ZRN | 1,000 blocks | 2,000 blocks | 50% | 66.7% |
| 3 | 13 | 100 ZRN | 2,000 blocks | 5,000 blocks | 60% | 75% |
| 4 | 21 | 1,000 ZRN | 5,000 blocks | 10,000 blocks | 66.6% | 80% |

---

## research

Research submission and peer review.

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `min_research_stake` | string | "1000000" (1 ZRN) | Minimum stake to submit research |
| `min_challenge_stake` | string | "1000000" (1 ZRN) | Minimum stake to challenge research |
| `review_period_blocks` | uint64 | 68,544 (~2 days) | Peer review period |
| `min_reviewer_count` | uint32 | 3 | Minimum number of reviewers |
| `acceptance_score_threshold` | uint32 | 70 | Acceptance score threshold (0-100) |
| `rejection_slash_bps` | uint64 | 330,000 (33%) | Slash rate for rejected research |
| `max_bounty_reward` | string | "10000000000" (10,000 ZRN) | Maximum research bounty reward |
| `bounty_min_deadline_blocks` | uint64 | 34,272 (~1 day) | Minimum bounty deadline |

---

## qualification

Domain-specific validator qualification.

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `min_stake_amount` | string | "100000000" (100 ZRN) | Minimum stake for qualification |
| `stake_lock_period` | uint64 | 100,800 | Stake lock duration (blocks) |
| `min_verifications` | uint64 | 100 | Minimum verifications required |
| `min_accuracy_bps` | uint64 | 800,000 (80%) | Minimum accuracy for qualification |
| `min_reputation_score` | uint64 | 500,000 (50%) | Minimum reputation score |
| `qualification_period` | uint64 | 1,209,600 | Qualification validity (blocks) |
| `probation_period` | uint64 | 302,400 | Probation period (blocks) |
| `renewal_window` | uint64 | 100,800 | Renewal window before expiry |
| `max_endorsements` | uint32 | 50 | Maximum endorsements per qualification |
| `cross_ref_min_weight` | uint64 | 30 | Minimum cross-reference weight |
| `cross_ref_weight_discount_bps` | uint64 | 200,000 (20%) | Cross-reference weight discount |
| `inheritance_weight_discount_bps` | uint64 | 300,000 (30%) | Inherited qualification discount |

---

## capture_defense

Anti-capture reputation and defense scoring.

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `decay_epoch_blocks` | uint64 | -- | Reputation decay half-life (blocks) |
| `min_verifications_for_score` | uint64 | -- | Minimum verifications for reputation score |
| `hhi_threshold` | uint64 | -- | Herfindahl-Hirschman Index threshold |
| `risk_analysis_interval` | uint64 | -- | Interval for capture risk analysis |
| `history_retention_blocks` | uint64 | -- | History retention window |
| `base_reputation_score` | uint64 | -- | Floor reputation score |
| `max_history_per_domain` | uint64 | -- | Max history entries per domain |

---

## capture_challenge

Capture challenge mechanism.

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `min_challenge_stake` | string | -- | Minimum stake to submit a capture challenge |
| `evidence_period_blocks` | uint64 | -- | Evidence submission period |
| `review_period_blocks` | uint64 | -- | Review/adjudication period |
| `domain_pause_blocks` | uint64 | -- | Duration of domain pause on successful challenge |
| `reward_rate_bps` | uint64 | -- | Reward rate from bounty pool |
| `slash_rate_bps` | uint64 | -- | Slash rate on failed challenge |
| `bounty_contribution_per_fact` | string | -- | Per-fact contribution to bounty pool |
| `risk_analysis_interval` | uint64 | -- | Interval for automated risk analysis |

---

## autopoiesis

Self-regulating system sustainability index (SSI).

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `epoch_length_blocks` | uint64 | 100 | SSI observation epoch length |
| `max_change_per_epoch_bps` | uint64 | 10,000 (1%) | Maximum parameter change per epoch |
| `slash_multiplier_min` | uint64 | 500,000 (0.5x) | Minimum slash multiplier floor |
| `slash_multiplier_max` | uint64 | 2,000,000 (2.0x) | Maximum slash multiplier ceiling |
| `ssi_critical_threshold` | uint64 | 250,000 (25%) | SSI critical health threshold |
| `ssi_stressed_threshold` | uint64 | 500,000 (50%) | SSI stressed health threshold |
| `ssi_healthy_threshold` | uint64 | 750,000 (75%) | SSI healthy threshold |
| `enabled` | bool | true | Enable autopoietic regulation |

---

## alignment

System health alignment scoring.

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `observation_interval_blocks` | uint64 | 100 | Alignment check interval |
| `weight_knowledge_quality` | uint64 | 200,000 (20%) | Knowledge quality weight |
| `weight_economic_stability` | uint64 | 200,000 (20%) | Economic stability weight |
| `weight_governance_participation` | uint64 | 200,000 (20%) | Governance participation weight |
| `weight_network_security` | uint64 | 200,000 (20%) | Network security weight |
| `weight_staking_ratio` | uint64 | 200,000 (20%) | Staking ratio weight |
| `critical_threshold` | uint64 | 200,000 (20%) | Critical alignment threshold |
| `degraded_threshold` | uint64 | 400,000 (40%) | Degraded alignment threshold |
| `healthy_threshold` | uint64 | 700,000 (70%) | Healthy alignment threshold |
| `enabled` | bool | true | Enable alignment scoring |

---

## compute_pool

Compute provider marketplace.

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `compute_pool_share_bps` | uint64 | 100,000 (10%) | Pool share of compute fees |
| `base_cu_per_verification` | uint64 | 100 | Compute units per verification |
| `min_provider_stake` | string | "10000000" (10 ZRN) | Minimum provider stake |
| `min_uptime_bps` | uint64 | 900,000 (90%) | Minimum uptime requirement |
| `heartbeat_interval_blocks` | uint64 | 100 | Provider heartbeat interval |
| `max_price_per_cu` | string | "1000000" (1 ZRN) | Maximum price per compute unit |
| `provider_unbonding_blocks` | uint64 | 10,000 | Provider unbonding period |
| `price_change_delay_blocks` | uint64 | 500 | Delay between price changes |
| `max_latency_ms` | uint64 | 5,000 | Maximum allowed latency (ms) |
| `sla_window_blocks` | uint64 | 1,000 | SLA measurement window |
| `target_utilization_low_bps` | uint64 | 300,000 (30%) | Low utilization target |
| `target_utilization_high_bps` | uint64 | 800,000 (80%) | High utilization target |

---

## discovery

Agent discovery and registry.

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `min_registration_stake` | string | "1000000" (1 ZRN) | Minimum stake to register an agent |
| `max_capabilities_per_agent` | uint32 | 20 | Maximum capabilities per agent profile |
| `profile_expiry_blocks` | uint64 | 100,000 | Agent profile expiry |

---

## home

Agent Home (personal workspace) parameters.

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `max_keys_per_home` | uint64 | 20 | Maximum keys per home |
| `max_sessions_per_home` | uint64 | 5 | Maximum concurrent sessions |
| `session_timeout_blocks` | uint64 | 1,000 | Session inactivity timeout |
| `deadman_min_threshold` | uint64 | 100 | Deadman switch minimum threshold |
| `deadman_max_threshold` | uint64 | 100,000 | Deadman switch maximum threshold |
| `max_alerts_per_home` | uint64 | 100 | Maximum active alerts |
| `home_creation_fee` | string | "10000000" (10 ZRN) | Fee to create a home |
| `max_recovery_addresses` | uint64 | 5 | Maximum recovery addresses |

---

## tree

Tree of Life — project, task, and service management.

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `min_budget` | string | "1000000" (1 ZRN) | Minimum project budget |
| `max_tasks_per_project` | uint32 | 200 | Maximum tasks per project |
| `max_contributors` | uint32 | 50 | Maximum contributors per project |
| `max_applications` | uint32 | 100 | Maximum applications per project |
| `task_deadline_min_blocks` | uint64 | 100 | Minimum task deadline |
| `task_deadline_max_blocks` | uint64 | 1,036,800 (~30 days) | Maximum task deadline |
| `max_rejections` | uint32 | 3 | Maximum deliverable rejections |
| `seed_expiry_blocks` | uint64 | 172,800 (~5 days) | Seed opportunity expiry |
| `min_contributors_to_start` | uint32 | 1 | Minimum contributors to start project |

### Tree Revenue Split

| Parameter | Default | Description |
|-----------|---------|-------------|
| `contributors_bp` | 550,000 (55%) | Contributor share |
| `protocol_treasury_bp` | 220,000 (22%) | Protocol treasury share |
| `research_fund_bp` | 130,000 (13%) | Research fund share |
| `burn_bp` | 100,000 (10%) | Burn share |
| `evidence_tax_bp` | 220,000 (22%) | Evidence tax on deliverables |

---

## toolbox

Tool registry and marketplace.

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `max_contributors` | uint32 | 22 | Maximum contributors per tool |
| `max_dependency_depth` | uint32 | 10 | Maximum dependency chain depth |
| `max_dependencies` | uint32 | 20 | Maximum dependencies per tool |
| `min_tool_stake` | uint64 | 11,000,000 (11 ZRN) | Minimum stake to register a tool |
| `share_lock_cooldown_blocks` | uint64 | 34,272 (~1 day) | Share lock cooldown |
| `deprecation_grace_blocks` | uint64 | 240,000 (~1 week) | Grace period before deprecation |
| `blocks_per_trust_update` | uint64 | 1,000 (~42 min) | Trust score update interval |
| `verified_grace_period_blocks` | uint64 | 10,000 (~7 hours) | Grace period for verified tools |
| `tool_gas_limit` | uint64 | 1,000,000 | Gas limit per tool invocation |
| `free_calls_per_epoch` | uint64 | 50 | Free calls per epoch per home |
| `free_calls_enabled` | bool | true | Enable free tool calls |
| `min_home_age_blocks` | uint64 | 10,000 (~7 hours) | Minimum home age for free calls |
| `surge_enabled` | bool | true | Enable surge pricing |
| `surge_threshold_bps` | uint64 | 500,000 (50%) | Surge activation threshold |
| `surge_critical_bps` | uint64 | 800,000 (80%) | Critical surge threshold |
| `max_surge_multiplier_bps` | uint64 | 10,000,000 (10x) | Maximum surge price multiplier |
| `demand_window_size` | uint64 | 1,000 | Demand measurement window |
| `target_calls_per_block_per_tool` | uint64 | 10 | Target per-tool utilization |
| `target_global_calls_per_block` | uint64 | 100 | Target global utilization |

### Toolbox Revenue Split

| Parameter | Default | Description |
|-----------|---------|-------------|
| `tool_revenue_bps` | 550,000 (55%) | Tool creator share |
| `protocol_bps` | 220,000 (22%) | Protocol share |
| `research_bps` | 130,000 (13%) | Research fund share |
| `burn_bps` | 100,000 (10%) | Burn share |

---

## schedule

Scheduled transaction execution.

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `max_active_per_account` | uint32 | 20 | Maximum active schedules per account |
| `max_gas_per_block` | uint64 | 50,000,000 | Maximum gas for scheduled txs per block |
| `min_interval_blocks` | uint64 | 10 | Minimum interval between executions |
| `min_fee_per_execution` | string | "10000" (0.01 ZRN) | Minimum fee per scheduled execution |
| `max_compound_depth` | uint64 | 3 | Maximum compound schedule depth |

---

## partnerships

Human-agent partnership parameters.

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `formation_window_blocks` | uint64 | 1,000 | Window for partnership formation |
| `cooling_period_blocks` | uint64 | 5,000 | Post-exit cooling period |
| `common_pot_share_bps` | uint64 | 100,000 (10%) | Share deposited to common pot |
| `safety_freeze_duration_blocks` | uint64 | 500 | Safety freeze duration |
| `max_freezes_per_epoch` | uint32 | 3 | Maximum safety freezes per epoch |
| `coercion_review_blocks` | uint64 | 2,000 | Coercion signal review window |
| `base_cooldown_blocks` | uint64 | 100 | Base cooldown between operations |
| `max_counter_proposal_depth` | uint32 | 3 | Maximum counter-proposal nesting |
| `default_human_split_bps` | uint64 | 500,000 (50%) | Default human revenue split |
| `default_agent_split_bps` | uint64 | 500,000 (50%) | Default agent revenue split |
| `min_partnership_stake` | string | "1000000" (1 ZRN) | Minimum partnership stake |
| `seed_partnership_duration` | uint64 | 10,000 | Seed partnership duration |
| `seed_common_pot_cap` | string | "100000000" (100 ZRN) | Seed partnership pot cap |

---

## liquiditypool

On-chain liquidity pool parameters.

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `default_swap_fee_bps` | uint64 | 3,000 (0.3%) | Default swap fee |
| `max_pools` | uint64 | 3 | Maximum number of pools |
| `min_initial_liquidity` | string | "10000000000" (10,000 ZRN) | Minimum initial liquidity |
| `twap_window_blocks` | uint64 | 1,000 (~42 min) | TWAP oracle window |
| `protocol_fee_bps` | uint64 | 450,000 (45%) | Protocol fee share of swap fees |
| `min_reserve` | string | "1" | Minimum reserve after swap |

---

## claiming_pot

Community claiming pool parameters.

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `max_pots_active` | uint32 | 10 | Maximum active claiming pots |
| `min_claim_amount` | string | "1000" | Minimum claim amount (uzrn) |

---

## evidence_mgmt

Evidence management and oracle parameters.

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `min_verifier_tier` | uint32 | 2 (Verified) | Minimum tier for evidence verification |
| `verification_quorum` | uint32 | 3 | Verification quorum |
| `challenge_bond` | string | "500000" (0.5 ZRN) | Challenge bond amount |
| `challenge_window_blocks` | uint64 | 50,000 | Challenge submission window |

---

## ibcratelimit

IBC transfer rate limiting.

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `enabled` | bool | true | Enable IBC rate limiting |

Rate limits are configured per channel/denom via governance transactions:

```bash
zeroned tx ibcratelimit add-rate-limit [channel-id] [denom] [max-percent-send] [max-percent-recv] [duration-hours]
```

---

## icaauth

Interchain Accounts authorization.

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `max_remote_accounts_per_owner` | uint64 | 5 | Maximum remote accounts per owner |
| `registration_cooldown` | uint64 | 100 | Cooldown between ICA registrations |
| `max_messages_per_tx` | uint64 | 5 | Maximum messages per ICA transaction |
| `allowed_host_msg_types` | []string | (see below) | Allowed message types on host chains |

Default allowed message types:
- `/cosmos.bank.v1beta1.MsgSend`
- `/cosmos.staking.v1beta1.MsgDelegate`
- `/cosmos.staking.v1beta1.MsgUndelegate`
- `/cosmos.staking.v1beta1.MsgBeginRedelegate`
- `/cosmos.distribution.v1beta1.MsgWithdrawDelegatorReward`
- `/cosmos.gov.v1beta1.MsgVote`

---

## tokens

Token emission parameters.

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `emission_epoch_blocks` | uint64 | 0 (disabled) | Blocks per emission epoch |
| `default_fee_bps` | string | "" | Reserved for future use |

---

## Proposing Parameter Changes

Any governance-adjustable parameter can be modified through a LIP (Living
Improvement Proposal). The process:

### 1. Submit a parameter change proposal

```bash
zeroned tx gov submit-lip \
  --title "Change max_verifiers to 33" \
  --description "Increase maximum verifiers per round from 22 to 33..." \
  --category "parameter" \
  --from my-validator \
  --chain-id zerone-testnet-1 \
  --fees 5000uzrn
```

### 2. Discussion period

The proposal enters a discussion period (68,544 blocks, ~2 days) where
validators and delegators can review and comment. Parameter changes require
a 1,000 ZRN stake from the proposer.

### 3. Voting period

After discussion, the voting period (102,816 blocks, ~3 days) begins.
Validators vote `yes`, `no`, or `abstain`:

```bash
zeroned tx gov cast-vote <proposal-id> yes \
  --from my-validator \
  --chain-id zerone-testnet-1
```

### 4. Execution

If the proposal reaches quorum (33.4% participation) and passes with >50%
support, the parameter change takes effect at the next block.

### Query current parameters

```bash
# Query a specific module's parameters
zeroned query <module> params

# Examples:
zeroned query knowledge params
zeroned query staking params
zeroned query emergency params
```
