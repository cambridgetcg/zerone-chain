# Qualification & Domains Report — R25-3

**Date:** 2026-02-26
**Network:** zerone-localnet (4 validators, block ~150)
**Binary:** build/zeroned
**Tester:** Claude (automated e2e)

## Executive Summary

The qualification module (`x/qualification`) is architecturally well-designed with four pathways, endorsements, probation lifecycle, and time-based governance. However, **domain qualification is entirely cosmetic** — the knowledge module's verification system (`SubmitCommitment`, `SubmitReveal`, vote extensions) performs zero qualification checks. Any account with 100 ZRN can verify any claim in any domain. This is a critical gate enforcement gap.

---

## Step 1: Qualification Params

**Status:** PASS
**Observation:** Params returned correctly via gRPC.

| Parameter | Value | Interpretation |
|-----------|-------|----------------|
| `min_stake_amount` | 100,000,000 uzrn | 100 ZRN minimum stake |
| `stake_lock_period` | 100,800 blocks | ~7 days at 6s blocks |
| `min_verifications` | 100 | Track record pathway threshold |
| `min_accuracy_bps` | 800,000 | 80% accuracy threshold |
| `min_reputation_score` | 500,000 | 50% reputation threshold |
| `qualification_period` | 1,209,600 blocks | ~84 days |
| `probation_period` | 302,400 blocks | ~21 days |
| `renewal_window` | 100,800 blocks | ~7 days before expiry |
| `max_endorsements` | 50 | Per qualification |
| `cross_ref_min_weight` | 30 | Minimum source weight |
| `cross_ref_weight_discount_bps` | 200,000 | 20% weight reduction |
| `inheritance_weight_discount_bps` | 300,000 | 30% weight reduction |

---

## Step 2: Qualify by Stake

**Status:** PASS
**Pathway:** STAKE
**Observation:** qualifier1 (non-validator account) successfully qualified in "general" domain by staking 100 ZRN. Qualification created immediately with ACTIVE status.

```json
{
  "validator": "zrn1umul0cc7gegd3g0mjxndsruun4dqnf4yzey6px",
  "domain": "general",
  "pathway": 1,        // STAKE
  "status": 1,         // ACTIVE
  "weight": 50,
  "staked_amount": "100000000",
  "granted_at": "44",
  "expires_at": "1209644"
}
```

**Gate Enforced:** YES — MinStakeAmount validated, tokens transferred to module account.
**Issue:** No validator-status check. Any account with tokens can qualify.

---

## Step 3: Qualify by Track Record

**Status:** PASS (rejection behavior correct)
**Pathway:** TRACK_RECORD
**Observation:** val0 attempted track record qualification with zero verification history. Correctly rejected:

```
insufficient track record: need 100, got 0
```

**Gate Enforced:** YES — MinVerifications (100) and MinAccuracyBps (80%) enforced.
**Issue:** Cannot be tested end-to-end because `RecordVerificationOutcome` is never called from the knowledge module (see Step 7). The track record pathway is structurally unreachable in practice.

---

## Step 4a: Qualify by Cross-Reference

**Status:** PASS
**Pathway:** CROSS_REFERENCE
**Observation:** qualifier1 cross-referenced from "general" (weight 50) to "computational". New qualification created with 20% weight discount.

```
general (weight: 50) → computational (weight: 40, pathway: CROSS_REFERENCE)
```

**Gate Enforced:** YES — Source domain must be ACTIVE with weight >= CrossRefMinWeight (30).
**Issue:** None. Cross-reference discount correctly applied.

---

## Step 4b: Qualify by Inheritance

**Status:** PASS
**Pathway:** INHERITANCE
**Observation:** qualifier1 inherited from "general" to "quantum_physics". Weight reduced by 30%.

```
general (weight: 50) → quantum_physics (weight: 35, stratum: 1, pathway: INHERITANCE)
```

**Gate Enforced:** PARTIAL — Inheritance works but stratum hierarchy is hardcoded. `getTargetStratum()` returns 1 for all domains (`pathways.go:282-287`), so stratum-based inheritance ordering is a placeholder.
**Issue:** Stratum hierarchy is cosmetic. Any domain inherits from any other domain because stratum=0 (parent) < stratum=1 (target) always passes. No ontology integration yet.

---

## Step 5: Endorsement

**Status:** PASS
**Observation:** val0 endorsed qualifier1 in "general" with weight 80. Self-endorsement correctly rejected.

```json
{
  "endorsement": {
    "id": "1",
    "endorser": "zrn1lprnrxnaud3ksx9x6929u8te6a8k0pwg6mks4h",
    "reason": "Strong performance in general knowledge",
    "weight": 80,
    "expires_at": "1209683"
  }
}
```

Self-endorsement error: `"cannot endorse own qualification"`

**Gate Enforced:** YES — Self-endorsement blocked, weight range [1-100] validated, max endorsements (50) enforced.
**Issue:** Endorsements do NOT affect qualification weight. Endorsement count is tracked (incremented to 1) but has no mechanical effect. The weight field is recorded but never consumed.

---

## Step 6: Renewal

**Status:** PASS (rejection behavior correct)
**Observation:** Renewal attempt at block ~130 for qualification expiring at block 1,209,644 correctly rejected:

```
renewal window not yet open: renewal opens at block 1108844
```

**Gate Enforced:** YES — RenewalWindow (100,800 blocks) enforced. Renewal only available in the last ~7 days before expiry.
**Issue:** Cannot test successful renewal on localnet without advancing 1.1M+ blocks.

---

## Step 7: THE KEY TEST — Does Verification Check Qualification?

**Status:** NOT_ENFORCED
**Gate Enforced:** NO

### Test Design
1. Submitted claim in "general" domain (where qualifier1 IS qualified)
2. qualifier1 successfully committed (expected: pass)
3. Submitted claim in "philosophy" domain (where qualifier1 is NOT qualified)
4. qualifier1 successfully committed (expected: reject, actual: **ACCEPTED**)

### Evidence

**Qualified commit (general):** code 0 (success) — correct
**Unqualified commit (philosophy):** code 0 (success) — **INCORRECT: should reject**

### Code Analysis

**`SubmitCommitment` (`x/knowledge/keeper/msg_server.go:239-296`):**
```go
// Verifier minimum balance gate (stopgap until full qualification module)
if m.keeper.bankKeeper != nil {
    bal := m.keeper.bankKeeper.GetBalance(ctx, verifierAddr, "uzrn")
    minBalance := sdkmath.NewInt(100_000_000) // 100 ZRN
    if bal.Amount.LT(minBalance) {
        return nil, fmt.Errorf("verifier does not meet minimum balance requirement")
    }
}
```
- Comment acknowledges this is a "stopgap until full qualification module"
- Only checks balance >= 100 ZRN, not domain qualification
- No call to `IsQualified()` or `GetQualificationWeight()`
- No call to `RecordVerificationOutcome()` after rounds complete

**`vote_extensions.go`:** Zero references to qualification module. Verifier selection uses VRF + stake weight only.

**`validator_selection.go`:** `GetEligibleValidators()` has a `domainQualificationKeeper` field but the filtering code is a comment: `// Future: filter by domain qualification`. Returns all active validators regardless.

### Impact

**Domain qualification is entirely cosmetic.** The qualification module is a standalone system that creates, tracks, and expires qualifications, but the knowledge module — which actually runs verification rounds — ignores it completely. Anyone with 100 ZRN can verify any claim in any domain.

This means:
- Stake in domain qualification is locked but provides no gating benefit
- Cross-reference and inheritance pathways create qualifications that nothing reads
- Endorsements are recorded but have no effect on verification access
- Track record metrics are never populated (RecordVerificationOutcome never called)
- The probation/suspension lifecycle is structurally unreachable

---

## Step 8: Withdrawal

**Status:** PASS
**Observation:**

1. **Inheritance withdrawal (no stake):** Successfully withdrawn. quantum_physics qualification removed immediately.
2. **Stake-locked withdrawal:** Correctly rejected:

```
stake is still locked: unlocks at block 100844
```

**Gate Enforced:** YES — StakeLockPeriod (100,800 blocks) properly enforced.

---

## Step 9: Probation

**Status:** NOT_TESTABLE
**Observation:** No code path transitions a qualification from ACTIVE to PROBATIONARY. The `RecordVerificationOutcome()` method updates metrics but never changes status. The `BeginBlocker` processes existing PROBATIONARY qualifications (promoting or suspending them based on metrics), but nothing creates them.

**Issue:** Probation is dead code. The PROBATIONARY status exists in proto, the BeginBlocker handles it, but no transition to PROBATIONARY exists in any message handler or keeper method.

---

## Step 10: Human vs Agent Qualification

**Status:** PASS (no restriction)
**Observation:** A non-validator human-style account ("humanuser") successfully qualified by stake in "general" domain.

```json
{
  "validator": "zrn1xg52tmz8tycxm9w0jzxkgjy7dem2gp6hj5ksap",
  "domain": "general",
  "pathway": 1,
  "status": 1,
  "weight": 50
}
```

**Gate Enforced:** NO — No validator-status check in any pathway.

### Findings

| Question | Answer | Evidence |
|----------|--------|----------|
| Is qualification restricted to validators? | **NO** | qualifier1 (non-validator) qualified successfully |
| Can non-validators verify claims? | **YES** | qualifier1 committed to verification round |
| Can humans verify? | **YES** | humanuser qualified successfully |
| Does verification check validator status? | **NO** | SubmitCommitment only checks balance, not validator set |
| Does verification check qualification? | **NO** | Unqualified commit accepted in unqualified domain |

---

## Summary Table

| Step | Test | Status | Gate Enforced |
|------|------|--------|---------------|
| 1 | Params query | PASS | N/A |
| 2 | Qualify by stake | PASS | YES (min stake) |
| 3 | Qualify by track record | PASS (correct rejection) | YES (min verifications) |
| 4a | Qualify by cross-reference | PASS | YES (source weight) |
| 4b | Qualify by inheritance | PASS | PARTIAL (stratum hardcoded) |
| 5 | Endorsement | PASS | YES (self-endorse blocked) |
| 6 | Renewal | PASS (correct rejection) | YES (renewal window) |
| 7 | **Verification checks qualification?** | **NOT_ENFORCED** | **NO — CRITICAL** |
| 8 | Withdrawal | PASS | YES (stake lock) |
| 9 | Probation | NOT_TESTABLE | NO (dead code) |
| 10 | Human/non-validator access | PASS (no restriction) | NO (no validator check) |

---

## Critical Issues

### 1. Domain Qualification is Cosmetic (CRITICAL)
- **Location:** `x/knowledge/keeper/msg_server.go:258-269`
- **Problem:** `SubmitCommitment` uses 100 ZRN balance check, not `IsQualified()`
- **Impact:** The entire qualification module is inert. Domain expertise gating does not exist.
- **Fix:** Replace balance check with `domainQualificationKeeper.IsQualified(ctx, verifier, claim.Domain)` call in SubmitCommitment.

### 2. RecordVerificationOutcome Never Called (HIGH)
- **Location:** `x/knowledge/keeper/msg_server.go` (missing from aggregation/reveal handling)
- **Problem:** Verification outcomes are never recorded to qualification metrics
- **Impact:** Track record pathway permanently unreachable; metrics always zero
- **Fix:** Call `RecordVerificationOutcome()` after verification round aggregation

### 3. Probation Transition Missing (MEDIUM)
- **Location:** `x/qualification/keeper/` (no code sets PROBATIONARY status)
- **Problem:** No code path transitions ACTIVE → PROBATIONARY
- **Impact:** BeginBlocker probation handling is dead code
- **Fix:** Add accuracy-based probation trigger in RecordVerificationOutcome or BeginBlocker

### 4. Stratum Hierarchy Hardcoded (MEDIUM)
- **Location:** `x/qualification/keeper/pathways.go:282-287`
- **Problem:** `getTargetStratum()` returns 1 for all domains
- **Impact:** Inheritance works between any domains (no real hierarchy)
- **Fix:** Integrate with ontology module or add domain→stratum configuration

### 5. No Validator-Status Check (LOW-MEDIUM)
- **Location:** `x/qualification/keeper/pathways.go` (all pathways)
- **Problem:** "validator" field is a misnomer — any address can qualify
- **Impact:** Non-validators and human accounts can qualify (may be intentional)
- **Decision needed:** Is qualification for validators only, or for all participants?

### 6. Endorsements Have No Mechanical Effect (LOW)
- **Problem:** Endorsement count tracked but never used in weight calculation or access decisions
- **Impact:** Endorsement is social signaling only
- **Fix:** Factor endorsement count/weight into qualification weight or verification priority

### 7. Vote Extensions Ignore Qualification (LOW)
- **Location:** `app/vote_extensions.go`
- **Problem:** Verifier selection uses VRF + stake weight with no qualification filter
- **Impact:** Even with SubmitCommitment gating, vote extension-based selection ignores domains
- **Fix:** Filter by `GetQualifiedValidators(domain)` before VRF selection

---

## Recommendations

1. **Immediate:** Wire `IsQualified()` check into `SubmitCommitment` to enforce domain gating
2. **Immediate:** Call `RecordVerificationOutcome()` after verification round aggregation
3. **Short-term:** Add ACTIVE → PROBATIONARY transition trigger for poor accuracy
4. **Short-term:** Integrate domain stratum with ontology module or config
5. **Design decision:** Clarify whether qualification is for validators only or all participants
6. **Design decision:** Define endorsement's mechanical role (weight boost? priority?)

---

## Commit Convention

```
test(qualification): domain qualification pathways e2e on localnet
docs(qualification): qualification-domains report — gate enforcement analysis
```
