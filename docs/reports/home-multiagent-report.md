# Home Module Multi-Agent Scenario Report (R22-2)

**Date:** 2026-02-26
**Chain:** zerone-localnet (4 validators)
**Block range:** 11–72
**Binary:** `./build/zeroned`
**Agents:**
- val0: `zrn1kdpvdxk88wtvtqp2ymyecsuekx6ljy0s4er8am`
- val1: `zrn1m685mg3mus5zv7sksm3nwkp7cz79g3e0hzy6t6`
- val2: `zrn19f7m9jhqc8u4q29ck02ugt9vvpq7wug2f4vqql`
- val3: `zrn12gue7ac8wpckxnxtmxtusqyr7u7pl0nvz3wn36`

---

## Summary

| # | Scenario | Status | Notes |
|---|----------|--------|-------|
| 1 | Multiple Homes Per Agent | **PASS** | 3 homes created with sequential IDs, independent state |
| 2 | Cross-Agent Isolation | **PASS** | All unauthorized modifications rejected with clear errors |
| 3 | Shared Guardian | **SKIP** | CLI commands not implemented (configure-guardian, acknowledge-alert) |
| 4 | Session Limit Exhaustion | **PASS** | 5/5 sessions succeed, 6th rejected, slot freed after end |
| 5 | Key Limit Exhaustion | **PASS** | 20/20 keys registered, 21st rejected, revoked keys free slots |
| 6 | Four Agents, Four Homes | **PASS** | All agents created homes, cross-queries work |
| 7 | Session Expiry Under Load | **SKIP** | 1000-block timeout too long; BeginBlocker verified in code |
| 8 | Concurrent Operations | **PASS** | Sequential ops succeed; parallel sends handled gracefully |

**Result: 19/19 PASS (testable checks), 0 FAIL, 4 SKIP (2 missing CLI, 2 timeout constraint)**

---

## Scenario Details

### Scenario 1: Multiple Homes Per Agent
**Status:** PASS (3/3 checks)

**Commands:**
```bash
zeroned tx home create-home "Workshop" --from val0 ...
zeroned tx home create-home "Archive" --from val0 ...
zeroned tx home create-home "Lab" --from val0 ...
```

**Observations:**
- Three homes created with sequential IDs: `home-1`, `home-2`, `home-3`
- `homes-by-owner` correctly returns all three
- Each home has independent state (name, status=active, comfort_score=50)
- No `max_homes_per_agent` parameter exists — unlimited by design
- Home creation fee: 10 ZRN each (30 ZRN total for 3 homes)

**Issues:**
- **No per-agent home limit:** An agent can create unlimited homes, each costing 10 ZRN. The fee acts as a soft limit, but a well-funded agent could create thousands. Consider whether a configurable max is needed.

---

### Scenario 2: Cross-Agent Isolation
**Status:** PASS (4/4 checks)

**Test matrix:**

| Action | Attacker | Target | Result | Error |
|--------|----------|--------|--------|-------|
| Update home name | val0 | home-4 (val1) | **Rejected** | `unauthorized: not the home owner` |
| Register key | val0 | home-4 (val1) | **Rejected** | `unauthorized: not the home owner` |
| Revoke key | val0 | home-4 (val1) | **Rejected** | `unauthorized: not the home owner` |
| Query home | val0 | home-4 (val1) | **Allowed** | (queries are public) |

**Observations:**
- All write operations from non-owners are rejected at the message server level
- Error messages are clear and consistent: `unauthorized: not the home owner`
- Error messages do NOT leak internal state (no addresses, no key details)
- Read operations (queries) are fully public — any address can query any home's details

**Issues:**
- **Public query visibility:** Home queries expose `owner_address`, `key_hash`, `permissions`, `session_id`, and `expires_at` to any querier. On a public chain, this data is visible anyway via raw state queries, but the structured query API makes enumeration trivial. Consider whether this is acceptable for the threat model. Document for R22-5.

---

### Scenario 3: Shared Guardian
**Status:** SKIP (2/2 checks)

**Reason:** `configure-guardian` and `acknowledge-alert` CLI commands are not implemented. The keeper logic exists (`msg_server.go:379-453` for guardian config, `msg_server.go:455-484` for alert ack) but no CLI wiring.

**Design notes for R22-5:**
- Guardian role is currently thin: only alert acknowledgment
- Guardian should arguably be able to:
  - Trigger emergency status transitions (`active → guarded`)
  - Revoke compromised keys on behalf of owner
  - Execute recovery if owner goes silent (deadman trigger)
  - Initiate key rotation in emergency scenarios
- The deadman switch exists in BeginBlocker but cannot be configured without the CLI

---

### Scenario 4: Session Limit Exhaustion
**Status:** PASS (3/3 checks)

**Parameters:** `max_sessions_per_home = 5`

| Step | Action | Result |
|------|--------|--------|
| 1 | Register 6 session keys | All 6 registered (keys ≠ sessions) |
| 2 | Start sessions 1–5 | All 5 started successfully |
| 3 | Start session 6 (overflow) | Rejected: `maximum sessions per home reached: limit 5` |
| 4 | End session 1 | Ended successfully |
| 5 | Start session 6 (retry) | **Succeeded** — slot freed |

**Observations:**
- Session limit is enforced on **active count**, not lifetime total
- Ending a session immediately frees the slot for a new one
- Each session requires a unique registered key (cannot reuse keys for multiple concurrent sessions)
- Error message is clear: `maximum sessions per home reached: limit 5`

---

### Scenario 5: Key Limit Exhaustion
**Status:** PASS (3/3 checks)

**Parameters:** `max_keys_per_home = 20`

| Step | Action | Result |
|------|--------|--------|
| 1 | Register 20 keys (limit-key-1 through limit-key-20) | All 20 registered |
| 2 | Register 21st key (overflow-key) | Rejected: `maximum keys per home reached: limit 20` |
| 3 | Revoke limit-key-1 | Revoked; `key_revoked` alert created |
| 4 | Register replacement-key | **Succeeded** — revoked key freed the slot |

**Observations:**
- Key limit is enforced on **non-revoked count**, not total historical count
- Revoking a key frees the slot for a new registration
- Agents cannot get permanently locked out of key registration
- Alert generated on revocation: `alert_type=key_revoked, priority=medium`

**Issues:**
- **Performance at scale:** Registering 20 keys required 20 sequential transactions (~120 seconds). For agents that need frequent key rotation, this is slow. Consider batch key registration.

---

### Scenario 6: Four Agents, Four Homes, Cross-Queries
**Status:** PASS (3/3 checks)

**Home distribution:**

| Agent | Homes | IDs |
|-------|-------|-----|
| val0 | 3 | home-1, home-2, home-3 |
| val1 | 1 | home-4 |
| val2 | 1 | home-5 |
| val3 | 1 | home-6 |

**Observations:**
- All four agents successfully created homes (total: 6 homes on chain)
- Cross-agent queries succeed — any agent can query any other's home
- Owner address, key registrations, session details are all publicly visible
- Home ID counter is global (not per-agent): IDs are `home-1` through `home-7`

**Issues:**
- **Privacy:** Home state is fully transparent. In a public blockchain this is expected, but the structured query API (especially `keys` and `sessions`) makes it easy to profile agent behavior. Consider if any fields should be encrypted or omitted from public queries. Document for R22-5.

---

### Scenario 7: Session Expiry Under Load
**Status:** SKIP (2/2 checks)

**Reason:** `session_timeout_blocks = 1000` (~83 minutes at 5s/block). Cannot wait for expiry in an interactive test session.

**Partial verification:**
- 5 active sessions exist on `home-1` with `expires_at` ~1000 blocks in the future
- BeginBlocker cleanup logic verified in code: `begin_blocker.go:68-103` iterates all homes, deletes expired sessions, creates `session_expired` alerts
- Alert system functional: `key_revoked` alert exists on `home-3` (from Scenario 5)

**Recommendations:**
- Add a governance-adjustable param or test-only override to reduce `session_timeout_blocks` for testing (e.g., 10 blocks)
- Consider adding a CLI command to query session expiry times for monitoring

---

### Scenario 8: Concurrent Home Operations
**Status:** PASS (3/3 checks)

**Sequential rapid-fire (within 3 blocks):**

| Operation | Result |
|-----------|--------|
| Create home (Speed-Test → home-7) | Success |
| Register key (speed-key-1) | Success |
| Start session | Success |
| Update name → Speed-Test-Renamed | Success |

**Final state verification:** name=Speed-Test-Renamed, keys=1, sessions=1 — consistent.

**Parallel submission (2 TXs from same account simultaneously):**
- TX1 (parallel-key-1): Succeeded
- TX2 (parallel-key-2): Failed (account sequence mismatch)
- **No data corruption** — state remained consistent

**Observations:**
- Cosmos SDK handles sequential operations from the same account correctly
- Parallel sends from the same account hit expected sequence number conflicts
- No partial state visible between blocks — atomicity preserved

---

## Bugs Found

### BUG-1: `--gas auto` Incompatible with Home Module (Severity: Medium)

**Symptom:** `--gas auto --gas-adjustment 1.5` always fails for home module transactions with:
```
tx gas limit 93442 below minimum required 150000 for 1 messages: out of gas
```

**Root cause:** The simulation phase estimates actual execution gas (~80k for create-home), applies the 1.5x adjustment (~120k), but the AnteHandler's `ZeroneGasDecorator` independently enforces a per-message-type minimum from `msgTypeURLToGas` (150k for create-home). The simulation doesn't account for this custom minimum.

**Impact:** All home module operations fail with `--gas auto`. Users must provide explicit gas values (`--gas 250000`).

**Location:** `app/ante_zerone.go:190-211` (gas check), `app/ante_zerone.go:412-422` (home gas mappings)

**Suggestion:** The gas estimation endpoint should include the per-message-type minimum in its calculation. Either:
1. Override the simulation result with `max(simulated * adjustment, msgMinGas)` in the client
2. Return the minimum gas requirement in the simulation response so the CLI can clamp
3. Document the explicit gas requirements per message type

---

## Design Issues for R22-5

### DESIGN-1: No Per-Agent Home Limit
- Agents can create unlimited homes (10 ZRN each)
- A well-funded agent could create thousands of homes, increasing state size
- **Recommendation:** Consider `max_homes_per_agent` param (default ~10)

### DESIGN-2: Guardian Role Too Thin
- Guardian can only acknowledge alerts
- Cannot trigger emergency status transitions
- Cannot revoke compromised keys
- Cannot execute recovery if owner is unresponsive
- **Recommendation:** Expand guardian capabilities with configurable permission set

### DESIGN-3: Public Query Visibility
- All home state (owner, keys, sessions, permissions) is publicly queryable
- Makes agent profiling and behavior tracking trivial
- On a public chain this is inherent, but structured APIs amplify it
- **Recommendation:** Document explicitly as a design choice; consider optional encryption for sensitive fields

### DESIGN-4: Session Timeout Not Testable
- Default 1000-block timeout makes e2e testing impractical
- **Recommendation:** Reduce to 50-100 blocks for testnet, or add `session_timeout_override` for test chains

### DESIGN-5: Missing CLI Commands
- `configure-guardian`: Keeper logic exists, CLI missing
- `acknowledge-alert`: Keeper logic exists, CLI missing
- `update-memory-cid`: Keeper logic exists, CLI missing
- **Recommendation:** Implement these CLIs as priority for R22-3 (blocks guardian and partnership testing)

---

## Test Script

The automated test script is at `scripts/home-multiagent-test.sh`. Run with:
```bash
scripts/localnet.sh start
scripts/home-multiagent-test.sh
```

---

## Exit Criteria Checklist

- [x] All 8 scenarios attempted (6 executed, 1 blocked by missing CLI, 1 by timeout)
- [x] Cross-agent isolation verified (S2: all 4 unauthorized modifications rejected)
- [x] Limit enforcement tested (S4: sessions 5/5 + overflow, S5: keys 20/20 + overflow)
- [x] 5 boundary/design issues documented (DESIGN-1 through DESIGN-5)
- [x] 1 bug documented (BUG-1: --gas auto incompatibility)
- [x] Report written to `docs/home-multiagent-report.md`
