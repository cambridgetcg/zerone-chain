# Home Module — Adversarial Test Report (R22-4)

**Date:** 2026-02-26
**Module:** `x/home`
**Scope:** Input validation, permission escalation, DoS vectors, recovery gaps, race conditions

## Executive Summary

21 adversarial unit tests and 14 E2E tests were written against the home module. Code review revealed **4 validation gaps** and **1 unenforced limit**, all of which were fixed before testing. All 21 unit tests pass. The BeginBlocker processes 50 homes with 150 expired sessions in ~3ms.

## Fixes Applied

| Fix | Files | Description |
|-----|-------|-------------|
| Input validation | `msg_server.go`, `errors.go` | Added `ErrInvalidInput` (code 20), length checks for name (128), key_hash (128), CID (256), null byte rejection, bech32 validation on recovery/guardian addresses |
| Alert limit enforcement | `keeper.go`, `msg_server.go`, `begin_blocker.go` | Added `SetAlertWithLimit()`, replaced `SetAlert()` in RevokeKey, triggerDeadman, CleanupExpiredSessions |
| Missing CLI commands | `cli/tx.go` | Added `configure-guardian`, `update-memory-cid`, `acknowledge-alert` |

## Vector-by-Vector Findings

### A. Input Validation

| # | Vector | Status | Severity | Observation | Recommendation |
|---|--------|--------|----------|-------------|----------------|
| A1 | Empty home name | FIXED | Medium | `CreateHome` accepted empty string — homes without names are hard to identify | Reject empty names with `ErrInvalidInput` |
| A2 | Oversized home name (>128 chars) | FIXED | Low | No length limit — could bloat state and event logs | Enforce `MaxNameLength=128` |
| A3 | Null bytes in name | FIXED | Medium | Null bytes in name could cause display corruption or parsing issues in downstream clients | Reject names containing `\x00` |
| A4 | Empty key hash | FIXED | High | Empty key hash creates ambiguous key registrations, could collide with sentinel values | Reject empty key hashes |
| A5 | Oversized key hash (>128 chars) | FIXED | Low | Unbounded key hash bloats KV store keys | Enforce `MaxKeyHashLength=128` |
| A6 | Empty CID | FIXED | Medium | Empty CID stored as valid memory pointer — misleading state | Reject empty CIDs |
| A7 | Oversized CID (>256 chars) | FIXED | Low | No limit — CIDs are typically 46-59 chars | Enforce `MaxCIDLength=256` |

### B. Permission Escalation

| # | Vector | Status | Severity | Observation | Recommendation |
|---|--------|--------|----------|-------------|----------------|
| B1 | Disjoint permission intersection | PASS | Info | Requesting permissions not in key's set yields empty intersection — session created with zero permissions | Acceptable: session is useless but harmless. Consider warning in future |
| B2 | Expired key session start | PASS | High | Keys with `ExpiresAt < BlockHeight` are correctly rejected | No action needed |
| B3 | Guardian privilege escalation | PASS | Critical | Guardian cannot UpdateHome, RegisterKey, RevokeKey, or ConfigureGuardian — correctly limited to owner-only operations | No action needed |
| B4 | Guardian acknowledge permission | PASS | Info | Guardian CAN acknowledge alerts (by design) — correct behavior per `AcknowledgeAlert` logic | No action needed |
| B5 | Invalid bech32 recovery addresses | FIXED | High | No bech32 validation on recovery addresses — invalid addresses would be stored and unusable | Added `sdk.AccAddressFromBech32()` validation |

### C. DoS / State Exhaustion

| # | Vector | Status | Severity | Observation | Recommendation |
|---|--------|--------|----------|-------------|----------------|
| C1 | Home creation spam | PASS | Medium | 10 ZRN creation fee deters mass home creation | Fee is adequate for testnet; adjust for mainnet based on token economics |
| C2 | Alert flood (MaxAlertsPerHome) | FIXED | High | `MaxAlertsPerHome=100` defined in params but never enforced — unlimited alerts could be created | Added `SetAlertWithLimit()` that checks `CountPendingAlerts() >= MaxAlertsPerHome` |
| C3 | BeginBlocker alert limit | FIXED | High | `triggerDeadman` and `CleanupExpiredSessions` used `SetAlert()` directly — could bypass limit | Replaced with `SetAlertWithLimit()`, security ops still proceed |
| C4 | BeginBlocker scalability | PASS | Info | 50 homes × 3 expired sessions processed in **~3ms** | Well within block time budget. Linear scaling is acceptable for expected home counts |

### D. Recovery Mechanism

| # | Vector | Status | Severity | Observation | Recommendation |
|---|--------|--------|----------|-------------|----------------|
| D1 | Recovery threshold=0 | PASS | Info | `threshold=0` with empty addresses accepted — valid "no recovery" configuration | No action needed |
| D2 | No MsgRecoverHome | DOCUMENTED | Medium | Recovery addresses are stored but no `MsgRecoverHome` message type exists — recovery is not actionable | Implement `MsgRecoverHome` before mainnet. Track as future work |
| D3 | Invalid guardian address | FIXED | High | No bech32 validation on `GuardianAddress` — invalid address would be stored | Added `sdk.AccAddressFromBech32()` validation |

### E. Race Conditions

| # | Vector | Status | Severity | Observation | Recommendation |
|---|--------|--------|----------|-------------|----------------|
| E1 | Revoke then start session | PASS | Critical | Revoked key is immediately checked via `reg.Revoked` flag — no race window | No action needed (SDK tx execution is sequential within a block) |
| E2 | Max sessions across blocks | PASS | High | `CountSessions()` correctly counts across block boundaries — limit enforced regardless of block height | No action needed |

## BeginBlocker Scalability Measurement

| Homes | Sessions per Home | Total Sessions Expired | Time |
|-------|-------------------|----------------------|------|
| 50 | 3 | 150 | ~3ms |

Extrapolation: 500 homes with 1500 expired sessions would take ~30ms — well within the block time budget (target <100ms for BeginBlocker).

## Accepted Risks

1. **Disjoint permission intersection (B1):** Sessions with zero granted permissions are created successfully. This is by design — the intersection algorithm is correct. A future enhancement could warn or reject sessions with empty permission sets.

2. **No MsgRecoverHome (D2):** Recovery addresses are stored in guardian config but cannot be used to initiate recovery. This is a documented gap that should be addressed before mainnet. The stored addresses serve as configuration for a future recovery flow.

3. **Alert ID predictability:** Alert IDs follow the pattern `{type}-{identifier}-{block_height}`. This is predictable but not exploitable since alert creation is access-controlled (only owner actions and BeginBlocker create alerts).

## Test Coverage Summary

| Category | Unit Tests | E2E Tests | Issues Found | Issues Fixed |
|----------|-----------|-----------|-------------|-------------|
| A. Input Validation | 7 | 4 | 7 | 7 |
| B. Permission Escalation | 5 | 2 | 1 | 1 |
| C. DoS / State Exhaustion | 4 | 2 | 2 | 2 |
| D. Recovery Mechanism | 3 | 3 | 2 | 1 (+1 documented) |
| E. Race Conditions | 2 | 3 | 0 | 0 |
| **Total** | **21** | **14** | **12** | **11 (+1 documented)** |

## Verification Commands

```bash
# Unit tests
go test ./x/home/keeper/ -v -run TestAdv

# Full test suite (verify no regressions)
go test ./x/home/keeper/ -v

# E2E (requires running localnet)
scripts/localnet.sh start && scripts/home-adversarial-e2e.sh

# Build verification
go build ./x/home/...
```
