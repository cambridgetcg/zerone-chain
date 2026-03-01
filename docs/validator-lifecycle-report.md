# Validator Lifecycle Report — R24-2

**Date:** 2026-02-26
**Chain:** zerone-localnet (4 validators, ~2.5s blocks)
**Module:** `zerone_staking` (custom, separate from Cosmos `x/staking`)

---

## Executive Summary

Full validator lifecycle tested: register → delegate → tier check → undelegate → redelegate → update-stake → deactivation. Discovered **5 issues**: 3 bugs and 2 missing features.

| Category | Count |
|----------|-------|
| Steps Passed | 7/10 |
| Bugs Found | 3 |
| Missing Features | 2 |
| Blocked Steps | 1 (slashing requires verification rounds) |

---

## Step 1: Staking Params & Tiers

**Status:** PASS

### Module Parameters

| Parameter | Value | Human-Readable |
|-----------|-------|----------------|
| unbonding_period | 268,560 blocks | ~7.75 days at 2.5s blocks |
| virtual_stake | 11,000,000 uzrn | 11 ZRN |
| max_validators | 100 | Block-producer cap (Scholar+) |
| min_self_delegation | 111,000 uzrn | 0.111 ZRN |
| max_slashes_per_epoch | 2 | Per-epoch rate limit |
| slash_decay_period_blocks | 34,272 | ~1 day per epoch |
| max_slash_count_deactivate | 3 | Deactivation threshold |
| slash_escalation_bps | 100,000 | +10% per prior slash |
| redelegation_cooldown_blocks | 1,111 | ~46 minutes |

### Tier Configurations

| Tier | Name | Min Stake | Min Verifications | Min Accuracy | Max Slash Count | Reward Mult | VRF Weight | Slash Mult | Categories |
|------|------|-----------|-------------------|--------------|-----------------|-------------|------------|------------|------------|
| 1 | Apprentice | 0.111 ZRN | 0 | 0% | unlimited | 0.1x | 0.1x | 1.5x | protocol, computational, formal |
| 2 | Verified | 1.11 ZRN | 22 | 77% | unlimited | 0.5x | 0.5x | 1.2x | + empirical |
| 3 | Scholar | 1,111 ZRN | 11 | 50% | unlimited | 1.0x | 1.0x | 1.0x | + oracle, attestation |
| 4 | Guardian | 11,111 ZRN | 333 | 77% | 0 (zero-tolerance, 10-epoch window) | 2.0x | 1.5x | 1.0x | + predictive, social, contested |

**Guardian special:** 33 contested verifications required, contested multiplier 3x.

**Observation:** Genesis validators (val0-val3) exist in Cosmos `x/staking` via gentxs but are NOT registered in `zerone_staking`. The two validator sets are completely independent.

---

## Step 2: Register a New Validator

**Status:** PASS

### Registration
```
Tx: register-validator [pubkey-hex] [self-delegation]
    --moniker --identity --commission --website --details
```

- Registered `Test-Validator-5` with 1,111 ZRN self-delegation, 5% commission
- Initial tier: **Apprentice** (even with 1,111 ZRN meeting Scholar's stake minimum — insufficient verifications)
- Reputation starts at 500,000 (50%)
- `is_active = true`
- `joined_at_block = 85`

### Edge Cases Tested

| Test | Result | Error |
|------|--------|-------|
| Duplicate address | FAIL (correct) | `validator already registered` |
| Duplicate DID | FAIL (correct) | `DID already registered to another validator` |
| Below min self-delegation (100 uzrn) | FAIL (correct) | `minimum 111000 uzrn, got 100: insufficient self-delegation` |
| Wrong pubkey format | PASS — any hex string accepted | No CometBFT validation |

### Observations
- Consensus pubkey is stored as a raw hex string — **no format validation against CometBFT ed25519 requirements**
- DID is optional (empty string accepted)
- Registration does NOT require a running node — purely on-chain state
- Min self-delegation is 0.111 ZRN (extremely low)

---

## Step 3: Delegation

**Status:** PASS

Delegated 50,000 ZRN from faucet to new validator.

| Metric | Before | After |
|--------|--------|-------|
| Validator delegated_stake | 0 | 50,000,000,000 |
| Validator total_stake | 1,111,000,000 | 51,111,000,000 |
| Faucet balance | 9,899,998 ZRN | 9,849,998 ZRN |

- Delegation record created with `created_at_block`
- Self-delegation and external delegation both visible in `delegations-for-validator`
- Upsert logic works (additional delegations add to existing)

---

## Step 4: Tier Progression

**Status:** PASS (documented behaviour)

### Key Finding: Tier Promotion is Automatic but Requires Verifications

Tier is computed by `ComputeValidatorTier()` which checks:
1. **Stake** (total_stake vs tier minimum)
2. **Verification count** (total_verifications)
3. **Accuracy** (correct_verifications / total_verifications)
4. For Guardian: windowed slash count, contested verifications

Tier transitions are checked:
- On every delegation/undelegation/redelegation/update-stake tx
- In `EndBlocker()` for all validators every block

### Issue: `min_reputation` Never Enforced

**BUG (Severity: Medium):** `TierConfig.min_reputation` is defined in the proto and returned in params, but `ComputeValidatorTier()` in `tiers.go:13-70` never checks reputation. The field is dead weight. Verified and Scholar tiers specify 77% and 50% min_reputation respectively, but these are never evaluated.

### Tier Progression on Localnet

Not achievable via staking alone. Requires participation in knowledge verification rounds:
- Scholar needs 11 verifications at 50% accuracy
- Verified needs 22 verifications at 77% accuracy

No `promote-tier` tx exists — promotion is automatic when requirements are met.

---

## Step 5: Undelegation

**Status:** PASS

Undelegated 10,000 ZRN from new-validator.

| Metric | Before | After |
|--------|--------|-------|
| Validator delegated_stake | 50,000 ZRN | 40,000 ZRN |
| Faucet delegation | 50,000 ZRN | 40,000 ZRN |

- Unbonding entry created at block 125
- `completes_at = 268,685` (current + 268,560 unbonding period)
- Validator's stake reduced **immediately** (not after unbonding)
- Funds locked until completion

### Issue: Unbonding Query Broken

**BUG (Severity: High):** `CmdQueryUnbondings` in `client/cli/query.go:157-177` calls `DelegatorDelegations` instead of querying unbonding entries. Returns delegations, not unbondings. Additionally, `grpc_query.go` has **no Unbondings query handler** at all despite the keeper having `GetUnbonding()`, `IterateUnbondings()`, and `GetMatureUnbondings()` methods.

**Impact:** No way to query unbonding entries via CLI or gRPC. Only visible through tx events (`zerone.staking.delegation_unbonding` with `completes_at` attribute).

---

## Step 6: Redelegation

**Status:** PASS

Redelegated 5,000 ZRN from new-validator → val1.

| Metric | Source (new-validator) | Dest (val1) |
|--------|----------------------|-------------|
| delegated_stake before | 40,000 ZRN | 0 |
| delegated_stake after | 35,000 ZRN | 5,000 ZRN |

- Tokens moved **instantly** — no unbonding period for redelegation
- Delegation records updated correctly on both sides
- **Cooldown enforced:** immediate re-redelegate returns `redelegation cooldown active` (1,111 blocks / ~46 min)
- Self-redelegation (src=dst) blocked at CLI level: `source and destination validator must differ`

---

## Step 7: Slashing & Unjail

**Status:** BLOCKED (cannot simulate from CLI)

### Slashing Mechanism (from code analysis)

Slashing is **internal only** — triggered by the knowledge module when verifications are incorrect. No CLI tx to slash.

**SlashValidator() Logic:**
1. Per-epoch rate limit: max 2 slashes per epoch
2. Progressive escalation: `adjusted = amount × (1M + slashCount × 100K) / 1M`
   - 1st slash: 1.0x, 2nd: 1.1x, 3rd: 1.2x, etc.
3. Autopoiesis SSI multiplier applied
4. Self-delegation slashed first, overflow to delegated
5. Slashed tokens routed to `development_fund` module
6. Reputation decreased by 10,000 BPS (1%) per slash
7. After 3 slashes: `is_active = false` (deactivated)

**Slash Decay:**
- `BeginBlocker` at epoch boundaries (every 34,272 blocks / ~1 day)
- Decays `slash_count` by 1 if no slashes in that epoch
- Resets `slashes_this_epoch` counter

### Issue: No Unjail Mechanism

**BUG / MISSING FEATURE (Severity: High):**
- `jailed`, `jail_reason`, `unjail_after_block` fields exist in the Validator proto
- `SlashValidator()` **never sets** `jailed = true` — it only sets `is_active = false`
- **No MsgUnjail** in zerone_staking module
- Standard Cosmos `x/slashing unjail` only works for Cosmos `x/staking` validators
- The jailing fields are completely unused dead code

---

## Step 8: Commission Update & Self-Delegation Update

**Status:** PARTIAL PASS

### Update-Stake

| Operation | Self-Delegation Before | After |
|-----------|----------------------|-------|
| Increase +5,000 ZRN | 1,111 ZRN | 6,111 ZRN |
| Decrease -1,000 ZRN | 6,111 ZRN | 5,111 ZRN |

- Increase: tokens transferred from account to staking module
- Decrease: creates unbonding entry (same unbonding period as undelegate)
- Self-delegation record upserted/updated correctly

### Issue: No Commission Update

**MISSING FEATURE (Severity: Medium):** No `update-commission` or `update-validator` tx exists. Commission (`commission_bps`) is set at registration and **cannot be changed**. Only 5 tx types exist:
1. `register-validator`
2. `update-stake` (self-delegation only, no commission flag)
3. `delegate`
4. `undelegate`
5. `redelegate`

---

## Step 9: Validator Deregistration

**Status:** PASS (documented behaviour)

### No Explicit Deregistration

No `deregister`, `leave`, or `exit` command exists.

### Implicit Exit Path
1. `update-stake --increase=false` with full self-delegation amount
2. Triggers auto-deactivation if: tier=Apprentice AND self_stake=0 AND total_verifications<22
3. Validator becomes `is_active = false`
4. Delegators must undelegate independently

### Issue: Deactivation is Irreversible

**BUG (Severity: High):** Once `is_active` is set to `false` (either by full self-delegation withdrawal or by 3+ slashes), there is **no mechanism to reactivate**. The `UpdateValidatorStake` (increase) handler doesn't restore `is_active`. Tested: added back self-delegation to a deactivated validator — remained `is_active = false`.

**Impact:** Validators who accidentally withdraw all self-delegation or get deactivated by slashing are permanently dead. They can never rejoin the active set. Delegators' tokens remain locked to a permanently inactive validator.

---

## Step 10: Summary of Issues

### Bugs

| # | Severity | Location | Description |
|---|----------|----------|-------------|
| 1 | **High** | `client/cli/query.go:157-177` | `CmdQueryUnbondings` calls `DelegatorDelegations` — returns delegations, not unbondings. No gRPC unbondings query handler exists. |
| 2 | **High** | `keeper/msg_server.go:506` | Deactivation (`is_active = false`) is irreversible — no reactivation path in `UpdateValidatorStake`, `Delegate`, or any other handler. |
| 3 | **Medium** | `keeper/tiers.go:13-70` | `min_reputation` in TierConfig is defined but never checked by `ComputeValidatorTier()`. Dead field in params. |

### Missing Features

| # | Severity | Description |
|---|----------|-------------|
| 4 | **Medium** | No commission update mechanism. Commission is immutable after registration. |
| 5 | **Medium** | No unjail / reactivation tx. `jailed`, `jail_reason`, `unjail_after_block` fields are dead code. |

### Additional Observations

| Observation | Impact |
|-------------|--------|
| Consensus pubkey accepts any hex string (no CometBFT ed25519 validation) | Validators can register with invalid pubkeys |
| Genesis validators (x/staking) and zerone_staking validators are completely separate | Dual staking systems with no bridge |
| Unbonding period = 268,560 blocks (~7.75 days) — not testable on localnet without fast-forward | Cannot verify unbonding completion |
| Slash escalation: +10% per prior slash, capped at 2/epoch, 3 = deactivation | Aggressive progressive punishment |
| Slashed tokens route to `development_fund` module | Ecosystem recycling, not burn |

---

## Tier Progression Requirements (Practical Guide)

For a new validator to progress through tiers:

| From → To | What's Needed | Localnet Feasibility |
|-----------|--------------|---------------------|
| → Apprentice | 0.111 ZRN self-delegation | Instant at registration |
| Apprentice → Verified | 1.11 ZRN stake + 22 verifications at 77% accuracy | Requires ~22 verification rounds |
| Verified → Scholar | 1,111 ZRN stake + 11 verifications at 50% accuracy | Requires 11 verification rounds + large delegation |
| Scholar → Guardian | 11,111 ZRN stake + 333 verifications at 77% accuracy + 33 contested correct + 0 windowed slashes | Not achievable on short localnet session |

**Note:** Scholar has lower verification requirements (11 at 50%) than Verified (22 at 77%) — the barrier is the stake requirement (1,111 ZRN vs 1.11 ZRN). A validator with enough stake could jump from Apprentice directly to Scholar after just 11 correct verifications.

---

## Recommended Fixes (Priority Order)

1. **Add gRPC unbondings query and fix CLI** — Users cannot see their pending unbondings
2. **Add reactivation mechanism** — Either `MsgReactivateValidator` or auto-reactivation when self-delegation restored
3. **Check `min_reputation` in ComputeValidatorTier** — Or remove the field from TierConfig
4. **Add commission update tx** — `MsgUpdateCommission` with rate-limiting
5. **Add unjail tx** — Populate jailed/jail_reason fields in SlashValidator, add MsgUnjail with cooldown
