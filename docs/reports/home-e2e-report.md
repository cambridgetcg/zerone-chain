# Home Module E2E Lifecycle Report

**Date:** 2026-02-26
**Chain:** zerone-testnet-1 (single validator, default port 26657)
**Block range:** 23903–24055
**Test account:** alice (`zrn16y70adqdqc9fkm343w0g6d8hjs5z5nm3srpyq2`)
**Binary:** `./build/zeroned`
**Balance before:** 9,998,700,000 uzrn | **After:** 9,964,900,000 uzrn (spent ~33.8 ZRN across all txs)

---

## Summary

| # | Scenario | Status | Notes |
|---|----------|--------|-------|
| 1 | Create Home | **PASS** | Home created, fee deducted, all defaults correct |
| 2 | Register Keys | **PASS** | 3 keys registered; duplicates rejected |
| 3 | Start/End Sessions | **PASS** | Permission intersection works, session cleanup correct |
| 4 | Update Memory CID | **FAIL** | CLI command does not exist |
| 5 | Configure Guardian | **FAIL** | CLI command does not exist |
| 6 | Trigger Deadman Switch | **FAIL** | Blocked — requires configure-guardian (Scenario 5) |
| 7 | Spending Limits | **PASS** | Stored correctly; enforcement unverified |
| 8 | Key Revocation | **PASS** | Key revoked, sessions terminated, alert created |
| 9 | Status Transitions | **PASS** | All valid transitions work; archived is terminal |
| 10 | Query Everything | **PASS** | All 7 queries return valid JSON |

**Result: 7/10 PASS, 3/10 FAIL (all failures due to missing CLI commands)**

---

## Scenario Details

### Scenario 1: Create Home
**Status:** PASS
**Tx Hash:** `40357F2CF82AFB4AF8177A55D3137FF8AA99C81AB194B2487EAF9D178DEBCE47`
**Block:** 23903

**Command:** `zeroned tx home create-home "AI-Alpha" --from alice ...`

**Observations:**
- Home created with `home_id: home-1` (human-readable, sequential — good)
- Status = "active", comfort_score = 50, treasury.reserved_balance = "0"
- Guardian defaults: defense_strategy = "moderate" (auto_defend field absent, not "false")
- Home creation fee: 10,000,000 uzrn (10 ZRN) deducted from balance
- Gas: estimated 80,422 used, but minimum required is 150,000 per message
- Owner index works: `homes-by-owner` correctly lists the home

**Issues:**
- `guardian.auto_defend` field not present in query output (omitempty on false? or unset?)
- No `memory_cid` field in home state — unclear if field exists at all
- CLI spec in R22-1 used `--name` flag, but actual CLI takes positional arg: `create-home [name]`

### Scenario 2: Register Keys
**Status:** PASS (with issues)
**Tx Hashes:**
- Primary key: `05A2B3389F836896CA9CA71CDA3833416F7E06E63F649EFF0B55F32CA7DC2FD9`
- Session key: `5F8FA239F91930337839D2FFE74517D8C40734E9634A26BD4E2E348F81957292`
- Guardian key: `FBDEA7951F6DC5D09409B8108CFD5D092E10D171693AE5B2B8593376C175C628`

**Command:** `zeroned tx home register-key [home-id] [key-hash] [key-type] [role] [permissions]`

**Observations:**
- 3 keys registered with correct roles and permissions
- CLI uses positional args, NOT flags (differs from R22-1 spec)
- Duplicate key registration properly rejected: "key already registered: sha256-primary-agent-key-001"
- `registered_at` populated on all keys
- `last_used_at` tracks session usage correctly

**Issues:**
- **No `--expires-at` flag** — session keys cannot be time-bound via CLI
- **Unknown permissions silently accepted** — registered a key with permissions "fly_to_moon,hack_nasa" and it was accepted (Code: 0). No validation on permission strings.
- `key_hash` accepts any string — no format validation (not necessarily wrong, but worth documenting)

### Scenario 3: Start and End Sessions
**Status:** PASS
**Tx Hashes:**
- Start: `86E9F28DD426704B87D1FD3E7F2200489DD931F07301AD1E21DD908F42AF12CB`
- End: `2D879747F051C81F1AEACFBEE138B2EA171F799F352D367834035A5726C41238`

**Command:** `zeroned tx home start-session [home-id] [key-hash] [permissions]`

**Observations:**
- Session created with `session_id: ses-sha256-p-23939` (format: `ses-{key_hash_prefix}-{block_height}`)
- Permissions correctly intersected: requested [transfer, submit_claim] ∩ key [transfer, stake, submit_claim, vote, memory_write] = [transfer, submit_claim]
- `started_at: 23939`, `expires_at: 24939` (= started_at + session_timeout_blocks of 1000)
- Home's `last_active_block` updated to session start block
- Session end removes session from state
- Ending non-existent session properly fails: "session not found: ses-nonexistent"

**Issues:**
- Session ID format includes partial key hash — could leak info about key identity
- Gas still consumed on failed end-session (49,624 gas used) — standard SDK behavior but worth noting

### Scenario 4: Update Memory CID
**Status:** FAIL — CLI not implemented
**Observation:** No `update-memory-cid` tx command exists. The `tx home` subcommands are: create-home, end-session, register-key, revoke-key, set-spending-limit, start-session, update-home. The `update-home` command only has `--name` and `--status` flags.

**Issue:** The home proto may have a `memory_cid` field (it appears in the spec), but there is no CLI command to set it. The `query home home` output also has no `memory_cid` field, suggesting it may not be wired into the state at all.

### Scenario 5: Configure Guardian
**Status:** FAIL — CLI not implemented
**Observation:** No `configure-guardian` tx command exists. The guardian config in home state only shows `defense_strategy: "moderate"` from defaults. There is no way to configure:
- `auto_defend`
- Deadman switch (enabled, threshold, action, beneficiary)
- Recovery addresses / threshold
- Guardian address

**Issue:** Guardian configuration is a core feature of the home module but has no CLI exposure. The proto likely defines `MsgConfigureGuardian` but the CLI tx command was never registered.

### Scenario 6: Trigger Deadman Switch
**Status:** FAIL — blocked by Scenario 5
**Observation:** Cannot configure deadman switch (requires configure-guardian CLI). Default guardian has no deadman settings. The BeginBlocker may have deadman logic, but without being able to enable it, the feature cannot be tested.

**Issue:** Even if `configure-guardian` existed, the spec notes that the deadman switch "only creates an alert" rather than executing the configured action. The `action` field ("lock") appears cosmetic. This should be clarified in documentation.

### Scenario 7: Spending Limits
**Status:** PASS (with concerns)
**Tx Hash:** `B52ACE19B549F87010BEA97D2DFB13F40E3C129C5A7B07C5C8C82BD742EFFEDF`

**Command:** `zeroned tx home set-spending-limit [home-id] [key-type] [max-amount] [period-blocks]`

**Observations:**
- Spending limit stored: key_type=ed25519, max_amount=1000000, period_blocks=100
- `spent_in_period: "0"` and `period_start: "23969"` correctly initialized
- Query returns limits correctly

**Issues:**
- Spending limits are stored but likely **not enforced** anywhere in the tx execution path. Without reading the keeper code (blocked by FS permissions), this cannot be confirmed. If limits are decorative only, this is a UX promise without teeth.
- Period reset logic (does `spent_in_period` reset after `period_blocks`?) cannot be verified without extensive waiting or code review.

### Scenario 8: Key Revocation
**Status:** PASS
**Tx Hash:** `DAB20F56FEDDCF86A8E6986419E852E436AA20B18C648C4AE17DF90528C66CE9`

**Command:** `zeroned tx home revoke-key [home-id] [key-hash]`

**Observations:**
- Key marked as `revoked: true` with `revoked_at: "23978"`
- Active session using that key (ses-sha256-s-23975) immediately terminated
- Alert created: `alert_id: "key-revoked-sha256-s-23978"`, type: `key_revoked`, priority: `medium`, message: "Key sha256-session-key-001 has been revoked"
- Starting new session with revoked key fails: "key has been revoked: sha256-session-key-001"

**Issues:** None — this feature works correctly end-to-end.

### Scenario 9: Status Transitions
**Status:** PASS

**Transitions tested:**

| From | To | Result | Tx Hash |
|------|----|--------|---------|
| active | dormant | PASS | `1928D4...` |
| dormant | active | PASS | `AF7CCB...` |
| active | archived | PASS | `16C4A9...` |
| archived | active | FAIL (expected) | `BBA467...` — "invalid status transition: archived -> active" |
| active | guarded | PASS | `64CA79...` |
| guarded | active | PASS | `AF575A...` |
| active | invalid_garbage | FAIL (expected) | `3E3743...` — "invalid status transition: active -> invalid_garbage" |

**Observations:**
- All valid transitions work
- Archived is terminal — cannot transition out
- Invalid status names are rejected with clear error message
- `guarded` status is directly settable by owner (not just by deadman switch)
- `last_active_block` updates on every status change

### Scenario 10: Query Everything
**Status:** PASS (with issues)

| Query | Works | Notes |
|-------|-------|-------|
| `query home params` | Yes | All 8 params returned |
| `query home home <id>` | Yes | Full home state |
| `query home homes-by-owner <addr>` | Yes | Lists all homes (including archived) |
| `query home keys <id>` | Yes | Lists all keys with metadata |
| `query home sessions <id>` | Yes | Lists active sessions |
| `query home alerts <id>` | Yes | Lists alerts |
| `query home spending-limits <id>` | Yes | Lists spending limits |

**Issues:**
- Empty results return `{}` instead of `{"keys": []}`, `{"sessions": []}`, etc. This creates inconsistent response shapes — clients must handle both missing key and empty array. When populated, the response uses `{"keys": [...]}`.

---

## Additional Edge Case Findings

### Empty Name Accepted
**Tx Hash:** `F4BFEFE684E48B3A754685107415E43593B95CC03D20157F7E6F66129AE71247`
A home was successfully created with an empty string name. The `name` field is omitted from the JSON output (omitempty). This is a validation gap — a minimum name length should be enforced.

### Authorization Works
**Tx Hash:** `7E68706CA2FB6F354002D588D1F5FCC1311286276E8A49270D01C82892CE09CE`
Bob attempted to rename Alice's home-2. Properly rejected: "unauthorized: not the home owner" (Code: 1).

### Gas Minimum Per Message
The chain enforces a minimum of 150,000 gas per message. The `--gas auto` estimate (93,573 for create-home) is below this minimum, causing the tx to fail with "tx gas limit 93573 below minimum required 150000." Users must explicitly set `--gas 200000` or higher.

### Gas Price Floor
Minimum gas price is 1 uzrn (node config `--minimum-gas-prices 0.001uzrn` but the AnteHandler enforces 1 uzrn per gas). Using `--gas-prices 0.025uzrn` results in "fee ... below minimum" errors.

---

## CLI Command Inventory

### Transaction Commands (7 registered)

| Command | Arg Style | Works |
|---------|-----------|-------|
| `create-home [name]` | Positional | Yes |
| `register-key [home-id] [key-hash] [key-type] [role] [permissions]` | Positional | Yes |
| `start-session [home-id] [key-hash] [permissions]` | Positional | Yes |
| `end-session [home-id] [session-id]` | Positional | Yes |
| `update-home [home-id] --name --status` | Mixed | Yes |
| `set-spending-limit [home-id] [key-type] [max-amount] [period-blocks]` | Positional | Yes |
| `revoke-key [home-id] [key-hash]` | Positional | Yes |

### Missing TX Commands (from spec)
- `configure-guardian` — guardian/deadman/recovery configuration
- `update-memory-cid` — IPFS CID for agent memory

### Query Commands (7 registered, all working)
`params`, `home`, `homes-by-owner`, `keys`, `sessions`, `alerts`, `spending-limits`

---

## Module Parameters

| Parameter | Value | Notes |
|-----------|-------|-------|
| max_keys_per_home | 20 | Not tested at limit |
| max_sessions_per_home | 5 | Not tested at limit |
| session_timeout_blocks | 1000 | Verified in session creation |
| deadman_min_threshold | 100 | Cannot test (no configure-guardian) |
| deadman_max_threshold | 100000 | Cannot test |
| max_alerts_per_home | 100 | Not tested at limit |
| home_creation_fee | 10,000,000 uzrn | Verified (10 ZRN per home) |
| max_recovery_addresses | 5 | Cannot test (no configure-guardian) |

---

## Improvement Suggestions

### 1. Implement `configure-guardian` CLI command (Critical)
The guardian system (deadman switch, recovery addresses, defense strategy changes, auto-defend) has no CLI exposure. This is the biggest gap — 3 of 10 scenarios fail because of it. The proto message likely exists but the CLI tx command was never registered in `tx.go`.

### 2. Implement `update-memory-cid` CLI command (High)
The memory CID feature appears in the spec but has no CLI command and the field may not even be in the home state proto. If memory CID is a core concept, it needs both proto field and CLI support.

### 3. Validate permission strings on key registration (High)
Unknown permissions like "fly_to_moon" are silently accepted. The keeper should maintain an enum of valid permissions and reject unknowns. At minimum: `transfer`, `stake`, `submit_claim`, `vote`, `memory_write`, `acknowledge_alert`, `defend`. This prevents typos from creating keys with useless permissions.

### 4. Validate home name on creation (Medium)
Empty names are accepted. Enforce a minimum length (e.g., 1 character) and optionally a maximum length. Consider rejecting names that are only whitespace.

### 5. Add `--expires-at` flag to `register-key` CLI (Medium)
Session keys should be time-boundable. The proto may support `expires_at_block` but the CLI doesn't expose it. This is listed in the R22-1 spec as a key feature of session keys.

### 6. Return consistent empty collections in queries (Low)
Empty results return `{}` instead of `{"keys": []}`. This forces clients to handle two different shapes. All collection queries should return the named wrapper key with an empty array.

### 7. Fix `--gas auto` interaction with minimum gas (Low)
Auto-estimation returns values below the 150,000 minimum, causing confusing failures. Either the auto-estimator should respect the minimum, or the error message should suggest using `--gas 200000`.

### 8. Document gas price floor (Low)
The node enforces a minimum gas price of 1 uzrn, but the CLI help and typical Cosmos examples suggest lower values (0.025). This should be documented in localnet setup or CLI help text.

---

## Homes Created During Testing

| Home ID | Name | Status | Created Block |
|---------|------|--------|---------------|
| home-1 | AI-Alpha | archived | 23903 |
| home-2 | Fortress-Prime (renamed) | active | 24015 |
| home-3 | (empty) | active | 24029 |
