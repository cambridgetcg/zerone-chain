# Agent Home — Improvement Report

**Date:** 2026-02-26
**Sessions:** R22-1 through R22-4
**Reports:** `home-e2e-report.md`, `home-multiagent-report.md`, `home-integration-report.md`, `home-adversarial-report.md`

## Executive Summary

The home module's core lifecycle works well: creation, key management, sessions, status transitions, and cross-agent isolation are all solid. R22-4 fixed 11 input validation and DoS gaps, bringing the security posture to a reasonable testnet baseline. The main remaining risks are **spending limits that are stored but never enforced** (a promise to agents that doesn't deliver), **recovery addresses that cannot be used** (no `MsgRecoverHome`), and an **empty BVM HomeKeeper interface** that blocks the central use case of agents accessing their home from code. 18 open issues remain: 0 critical, 4 high, 8 medium, 6 low/informational.

## Scorecard

| Category | Issues | Critical | High | Medium | Low | Info |
|----------|--------|----------|------|--------|-----|------|
| Security | 3 | 0 | 2 | 1 | 0 | 0 |
| Completeness | 5 | 0 | 2 | 2 | 1 | 0 |
| UX | 4 | 0 | 0 | 1 | 3 | 0 |
| Architecture | 6 | 0 | 0 | 4 | 1 | 1 |
| Design | 5 | 0 | 0 | 0 | 0 | 5 |
| **Total** | **23** | **0** | **4** | **8** | **5** | **6** |

*Note: 11 issues from R22-4 (input validation, alert limits, missing CLI) were fixed during testing and are not counted above.*

---

## Critical & High Issues

### H1. Spending Limits Not Enforced
**Category:** Security
**Severity:** High
**Effort:** M (day)
**Priority:** P1
**Found in:** R22-1 (Scenario 7)

**Description:** `set-spending-limit` stores `SpendingLimit` in state (key_type, max_amount, period_blocks, spent_in_period, period_start) but no middleware, ante handler, or keeper logic checks it before bank sends. The feature exists in state but has zero effect on transaction execution.

**Impact:** Agents configure spending limits believing they're protected. A compromised session key can drain the full treasury. This is a false safety promise.

**Recommendation:** Add spending limit enforcement in the home keeper's bank send wrapper, or in a custom ante decorator that intercepts `MsgSend` from session-key-authorized addresses and checks against the home's spending limits.

### H2. Recovery Addresses Are Stored But Unusable
**Category:** Security
**Severity:** High
**Effort:** L (days)
**Priority:** P1
**Found in:** R22-4 (D2)

**Description:** `ConfigureGuardian` stores `recovery_addresses` and `recovery_threshold` in `HomeGuardian`, but no `MsgRecoverHome` message type exists. There is no mechanism to actually use stored recovery addresses to recover a home.

**Impact:** Agents configure recovery believing it will protect them from key loss. If the owner loses access, there is no recovery path despite addresses being configured.

**Recommendation:** Implement `MsgRecoverHome` that requires `threshold` signatures from recovery addresses, transitions the home to a recovery state, and allows a new owner address to be set.

### H3. Unknown Permissions Silently Accepted
**Category:** Completeness
**Severity:** High
**Effort:** S (hours)
**Priority:** P1
**Found in:** R22-1 (Scenario 2)

**Description:** `register-key` accepts any string as a permission. `"fly_to_moon,hack_nasa"` was accepted (code 0). No canonical permission list exists in the keeper. Typos create keys with useless permissions that can never be matched in session intersection.

**Impact:** Agents misspell permissions (e.g., `tranfer` instead of `transfer`) and sessions silently get zero effective permissions. Debugging is difficult because the typo is stored without error.

**Recommendation:** Define a canonical permission set in `x/home/types/permissions.go` and validate in `RegisterKey`. Valid permissions: `transfer`, `stake`, `submit_claim`, `vote`, `memory_write`, `acknowledge_alert`, `defend`. Reject unknown strings with `ErrInvalidInput`.

### H4. Deadman Action Is Cosmetic
**Category:** Completeness
**Severity:** High
**Effort:** M (day)
**Priority:** P1
**Found in:** R22-1 (Scenario 6)

**Description:** `DeadmanConfig.Action` stores a string ("lock", "transfer", etc.) but `triggerDeadman` in `begin_blocker.go:45-65` only creates an alert and sets status to "guarded". The configured action is never executed — "lock" and "transfer" do the same thing (nothing beyond the alert).

**Impact:** Agents configure specific deadman actions believing they will execute. The `action` field is a UX lie.

**Recommendation:** Either implement action execution (lock = revoke all keys; transfer = send treasury to beneficiary) or remove the `action` field and document that deadman switch only creates an alert + transitions to guarded status.

---

## Medium Issues

### M1. Partnership Link Overwrite — No Warning
**Category:** Architecture
**Severity:** Medium
**Effort:** S (hours)
**Priority:** P2
**Found in:** R22-3 (S3)

**Description:** `SetPartnershipOnHome` (`keeper.go:198-206`) unconditionally overwrites `home.PartnershipId`. When a second partnership is accepted, the first partnership's link is silently removed. No event emitted, no warning, no check.

**Impact:** An agent's home can be "stolen" from one partnership to another without any notification to the first partnership's human partner.

**Recommendation:** Emit a `partnership_link_replaced` event with both old and new partnership IDs. Consider requiring explicit unlinking before re-linking.

### M2. Stale Partnership Link on Archived Home
**Category:** Architecture
**Severity:** Medium
**Effort:** S (hours)
**Priority:** P2
**Found in:** R22-3 (S6)

**Description:** When a home is archived, its `partnership_id` field is retained. The partnership module doesn't know the home is archived. No cross-module notification fires on status transitions.

**Impact:** Partnership module may assume the home is still active. Toolbox correctly checks `status == "active"`, but other modules might not.

**Recommendation:** Clear `partnership_id` on archive, or emit a cross-module event that the partnership module can listen to.

### M3. Empty BVM HomeKeeper Interface
**Category:** Architecture
**Severity:** Medium
**Effort:** L (days)
**Priority:** P1
**Found in:** R22-3 (S5)

**Description:** `x/bvm/types/expected_keepers.go:28-31` defines `HomeKeeper` as an empty interface. BVM programs cannot query home state, check ownership, or verify sessions. Contrast with `ToolboxHomeAdapter` which implements 3 methods.

**Impact:** The central use case — agents accessing their home from BVM code — is impossible. BVM programs cannot condition execution on home existence, status, or partnership.

**Recommendation:** Define BVM HomeKeeper methods: `GetHome`, `GetHomesByOwner`, `GetHomeStatus`, `GetPartnershipOnHome`. Implement a `BVMHomeAdapter` similar to `ToolboxHomeAdapter`.

### M4. No Alert Pruning
**Category:** Architecture
**Severity:** Medium
**Effort:** M (day)
**Priority:** P2
**Found in:** R22-3 (S7)

**Description:** Acknowledged alerts remain in state forever. `MaxAlertsPerHome` only caps pending (unacknowledged) alerts. Over time, alert state grows unboundedly.

**Impact:** State bloat proportional to home activity × lifetime. Thousands of acknowledged alerts per home on a long-running chain.

**Recommendation:** Add alert pruning in BeginBlocker: delete acknowledged alerts older than `alert_retention_blocks` (new param, default ~100,000 blocks / ~6 days).

### M5. `--gas auto` Incompatible with Home Module
**Category:** UX
**Severity:** Medium
**Effort:** M (day)
**Priority:** P2
**Found in:** R22-1, R22-2 (BUG-1)

**Description:** `--gas auto --gas-adjustment 1.5` estimates ~80k gas for home operations, but `ZeroneGasDecorator` enforces per-message-type minimums (150k for create-home). Simulation doesn't account for the custom minimum.

**Impact:** Every home operation fails with `--gas auto`. Users must provide explicit `--gas 250000`.

**Recommendation:** Override simulation result with `max(simulated * adjustment, msgMinGas)` in the gas estimation path, or document explicit gas requirements per message type.

### M6. No Retroactive Home-Partnership Link
**Category:** Architecture
**Severity:** Medium
**Effort:** S (hours)
**Priority:** P2
**Found in:** R22-3 (S2)

**Description:** Auto-link only fires in `AcceptPartnership`. If an agent creates their home after a partnership already exists, no link is created. The agent must dissolve and re-form the partnership to get a link.

**Impact:** Order-dependent behavior surprises agents. "Create home then partnership" works; "partnership then home" silently doesn't link.

**Recommendation:** Add a check in `CreateHome`: if the owner has active partnerships, auto-link to the most recent one. Or document the order dependency.

### M7. Guardian Role Too Thin
**Category:** Completeness
**Severity:** Medium
**Effort:** L (days)
**Priority:** P2
**Found in:** R22-2 (DESIGN-2)

**Description:** Guardian can only acknowledge alerts. Cannot trigger emergency status transitions, revoke compromised keys, execute recovery, or initiate key rotation.

**Impact:** Guardian provides minimal protection. An unresponsive owner with a compromised key has no guardian-initiated recourse.

**Recommendation:** Expand guardian capabilities with a configurable permission set. At minimum: `GuardianRevokeKey` (revoke a specific key in emergency) and `GuardianTransitionStatus` (active → guarded).

### M8. No `--expires-at` Flag on `register-key`
**Category:** Completeness
**Severity:** Medium
**Effort:** S (hours)
**Priority:** P2
**Found in:** R22-1 (Scenario 2)

**Description:** The proto supports `expires_at_block` on keys, and the keeper checks it during session start, but the CLI has no flag to set it. Session keys cannot be time-bounded via CLI.

**Impact:** Agents cannot create auto-expiring keys. All keys persist until manually revoked.

**Recommendation:** Add `--expires-at` flag to `register-key` CLI command.

---

## Low & Informational Issues

### L1. No Per-Agent Home Limit
**Category:** Architecture
**Severity:** Low
**Effort:** S (hours)
**Priority:** P3
**Found in:** R22-2 (DESIGN-1)

**Description:** Agents can create unlimited homes (10 ZRN each). A well-funded agent could create thousands.

**Recommendation:** Consider `max_homes_per_agent` param (default ~10). The 10 ZRN fee provides a soft limit.

### L2. Inconsistent Empty Collection Responses
**Category:** UX
**Severity:** Low
**Effort:** S (hours)
**Priority:** P3
**Found in:** R22-1 (Scenario 10)

**Description:** Empty results return `{}` instead of `{"keys": []}`. Clients must handle two response shapes.

**Recommendation:** Initialize empty slices in query responses so empty arrays are always returned.

### L3. Gas Price Floor Not Documented
**Category:** UX
**Severity:** Low
**Effort:** S (hours)
**Priority:** P3
**Found in:** R22-1

**Description:** Node enforces minimum gas price of 1 uzrn, but typical Cosmos examples suggest lower values (0.025).

**Recommendation:** Document in localnet setup guide and CLI help text.

### L4. Session ID Leaks Key Hash Prefix
**Category:** UX
**Severity:** Low
**Effort:** S (hours)
**Priority:** P3
**Found in:** R22-1 (Scenario 3)

**Description:** Session IDs follow `ses-{key_hash_prefix}-{block_height}`, leaking which key started the session.

**Recommendation:** Use a hash of the key hash or a random suffix instead.

### L5. BeginBlocker Scalability at Mainnet Scale
**Category:** Architecture
**Severity:** Informational
**Effort:** XL (week+)
**Priority:** P3
**Found in:** R22-2, R22-4 (C4)

**Description:** `CheckDeadmanSwitches` and `CleanupExpiredSessions` iterate all homes every block. At 50 homes ~3ms, linear scaling. At 10,000 homes, estimated ~600ms — may approach block time budget.

**Recommendation:** For mainnet: index homes by `last_active_block + threshold` and `session.expires_at` for O(1) expiry lookups instead of full scan.

### L6. Disjoint Permission Intersection Creates Useless Sessions
**Category:** Completeness
**Severity:** Informational
**Effort:** S (hours)
**Priority:** P3
**Found in:** R22-4 (B1)

**Description:** Requesting permissions not in a key's set yields an empty intersection. A session is created with zero permissions — useless but harmless.

**Recommendation:** Consider rejecting sessions with empty permission sets, or emitting a warning event.

---

## Known Design Questions

### DQ1. Should home state be private or public?
**Context:** All home state (owner, keys, sessions, permissions, alerts) is publicly queryable. Makes agent profiling trivial.
**Options:**
- **A. Keep public** (current) — standard for public blockchains; raw state is accessible anyway
- **B. Encrypt sensitive fields** — complicates querying, requires key management
- **C. Omit sensitive fields from structured queries** — security through obscurity, raw state still public
**Recommendation:** A (keep public). Document as explicit design choice. Public chains are transparent by nature.

### DQ2. What should the guardian role encompass?
**Context:** Guardian can only acknowledge alerts. Cannot revoke keys, trigger emergency transitions, or execute recovery.
**Options:**
- **A. Minimal guardian** (current) — acknowledge alerts only
- **B. Emergency guardian** — add key revocation and status transitions in emergency
- **C. Full co-owner** — guardian as second signer for all critical operations
**Recommendation:** B. Emergency guardian with configurable permissions: revoke keys, transition to guarded status. Full co-owner is over-engineered for v1.

### DQ3. Should partnership links be 1:1 or 1:N?
**Context:** A home has one `partnership_id`. Unconditional overwrite on new partnership.
**Options:**
- **A. 1:1 with explicit overwrite** (current, but silent) — simple, adequate for v1
- **B. 1:N** — home tracks multiple partnerships; complicates queries
- **C. 1:1 with rejection** — reject new link if one exists; require explicit unlink first
**Recommendation:** C for safety. 1:1 is fine, but require explicit `ClearPartnershipLink` before overwriting. Prevents accidental partner displacement.

### DQ4. Should session timeout be adjustable for testnet?
**Context:** Default 1000 blocks (~83 min) makes e2e session expiry testing impractical.
**Options:**
- **A. Reduce default for testnet** — 50-100 blocks
- **B. Add test-only override param** — `session_timeout_override`
- **C. Keep current, test via unit tests only**
**Recommendation:** A. Reduce to 100 blocks for testnet. Unit tests cover correctness; e2e should verify the full flow.

### DQ5. Should CreateHome auto-link existing partnerships?
**Context:** Auto-link fires only in AcceptPartnership. Home created after partnership gets no link.
**Options:**
- **A. No auto-link on CreateHome** (current) — simple, order matters
- **B. Auto-link on CreateHome** — check for active partnerships, link to most recent
- **C. Manual-only linking** — remove auto-link, require explicit link command
**Recommendation:** B. Add a check in CreateHome for active partnerships. The current behavior is surprising.

---

## Prioritised Fix Roadmap

### P0 — Must Fix Before Testnet
*No P0 issues. Testnet can launch with current state.*

### P1 — Fix Before Mainnet
- [ ] **H1** — Enforce spending limits (or remove the feature)
- [ ] **H2** — Implement `MsgRecoverHome`
- [ ] **H3** — Validate permission strings against canonical set
- [ ] **H4** — Implement deadman actions (or remove `action` field)
- [ ] **M3** — Populate BVM HomeKeeper interface

### P2 — Nice to Have
- [ ] **M1** — Emit event on partnership link overwrite
- [ ] **M2** — Clear/notify on partnership link when home archived
- [ ] **M4** — Add alert pruning in BeginBlocker
- [ ] **M5** — Fix `--gas auto` for home module
- [ ] **M6** — Auto-link partnership on CreateHome
- [ ] **M7** — Expand guardian capabilities
- [ ] **M8** — Add `--expires-at` flag to register-key

### P3 — Future
- [ ] **L1** — Per-agent home limit
- [ ] **L2** — Consistent empty collection responses
- [ ] **L3** — Document gas price floor
- [ ] **L4** — Non-leaky session IDs
- [ ] **L5** — BeginBlocker scalability (indexed expiry)
- [ ] **L6** — Reject zero-permission sessions

---

## Architecture Recommendations

### 1. Home↔Partnership Cross-Module Events
**Issues addressed:** M1, M2, M6
**Description:** Implement a cross-module event system where status transitions in x/home emit events that x/partnerships listens to (and vice versa). This addresses the stale link problem, silent overwrite, and retroactive linking.
**Effort:** M (day)

### 2. BVM Home Integration Layer
**Issues addressed:** M3
**Description:** Define and implement `BVMHomeAdapter` (like `ToolboxHomeAdapter`) with methods: `GetHome`, `GetHomesByOwner`, `GetHomeStatus`, `GetPartnershipOnHome`. Wire into BVM host functions so programs can query their own home state.
**Effort:** L (days)

### 3. Spending Limit Enforcement Middleware
**Issues addressed:** H1
**Description:** Add a spending limit ante decorator that intercepts `MsgSend` and session-authorized transactions, checks against the home's configured spending limits, and rejects over-limit sends. Alternatively, implement enforcement in the home keeper's send wrapper.
**Effort:** M (day)

### 4. Recovery Protocol
**Issues addressed:** H2, M7 (partial)
**Description:** Implement `MsgRecoverHome` with threshold-signature requirement from stored recovery addresses. Define recovery flow: recovery request → waiting period → ownership transfer. Guardian should be able to initiate recovery on behalf of unresponsive owner.
**Effort:** L (days)

---

## Fix Specifications (P1 Items)

### Fix: Enforce Spending Limits (H1)
**Files:** `x/home/keeper/spending.go` (new), `app/ante.go`
**Approach:** Create a `CheckSpendingLimit` method in the home keeper. Add an ante decorator that, for `MsgSend` from addresses that own homes, checks if the send amount + `spent_in_period` exceeds `max_amount`. Reset `spent_in_period` when `current_block > period_start + period_blocks`.
**Tests needed:** Unit test for limit enforcement, period reset, multi-home edge cases. E2E test: set limit, send within limit (pass), send over limit (fail).

### Fix: Implement MsgRecoverHome (H2)
**Files:** `x/home/types/tx.pb.go`, `x/home/keeper/msg_server.go`, `x/home/client/cli/tx.go`
**Approach:** Define `MsgRecoverHome{HomeId, NewOwner, Signers}`. In keeper: verify signers count >= `recovery_threshold`, verify all signers are in `recovery_addresses`, transition home to `recovery` status, set new `owner_address` after a cooldown period (e.g., 1000 blocks).
**Tests needed:** Threshold check, invalid signer rejection, cooldown period, ownership transfer verification.

### Fix: Validate Permission Strings (H3)
**Files:** `x/home/types/permissions.go` (new), `x/home/keeper/msg_server.go`
**Approach:** Define `ValidPermissions = map[string]bool{"transfer": true, "stake": true, ...}`. In `RegisterKey`, validate each permission string against the map. Reject with `ErrInvalidInput` if unknown.
**Tests needed:** Valid permission accepted, unknown permission rejected, empty permission list handling.

### Fix: Deadman Action Execution (H4)
**Files:** `x/home/keeper/begin_blocker.go`
**Approach:** In `triggerDeadman`, switch on `action`: `"lock"` → revoke all active keys; `"transfer"` → send treasury to beneficiary address (via bank keeper); `"alert"` → current behavior (alert + guarded status). Default to `"alert"` if action is empty.
**Tests needed:** Each action type executes correctly. Key revocation cascades to active sessions. Transfer moves correct amount.

### Fix: BVM HomeKeeper Interface (M3)
**Files:** `x/bvm/types/expected_keepers.go`, `x/home/keeper/bvm_adapters.go` (new)
**Approach:** Define 4 methods on `HomeKeeper` interface matching what `ToolboxHomeAdapter` already does. Create `BVMHomeAdapter` struct wrapping the home keeper. Wire in `app/app.go` when initializing the BVM module.
**Tests needed:** Interface compliance test, query from BVM context returns correct home data.

---

## Issues Fixed During R22 Testing (Reference)

These 11 issues were found and fixed during R22-4. They are NOT in the open issue count above.

| # | Issue | Severity | Fix |
|---|-------|----------|-----|
| F1 | Empty home name accepted | Medium | `ErrInvalidInput` on empty name |
| F2 | Oversized home name (>128) | Low | `MaxNameLength=128` check |
| F3 | Null bytes in name | Medium | Reject `\x00` in string fields |
| F4 | Empty key hash accepted | High | Reject empty key hash |
| F5 | Oversized key hash (>128) | Low | `MaxKeyHashLength=128` check |
| F6 | Empty CID accepted | Medium | Reject empty CID |
| F7 | Oversized CID (>256) | Low | `MaxCIDLength=256` check |
| F8 | Invalid bech32 recovery addresses | High | `sdk.AccAddressFromBech32()` validation |
| F9 | Alert flood — MaxAlertsPerHome unenforced | High | `SetAlertWithLimit()` |
| F10 | BeginBlocker bypassed alert limit | High | Replaced `SetAlert()` with `SetAlertWithLimit()` |
| F11 | Invalid guardian address stored | High | `sdk.AccAddressFromBech32()` validation |
