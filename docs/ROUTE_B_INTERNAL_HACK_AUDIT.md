# Route B Internal-Hack Audit — Wave 14

> **Recursive stress-test of the chain's response pipeline against *insider* abuse — the attacker is the governance authority itself (compromised, coerced, or acting maliciously). External drill (Wave 13) asked "can the chain recover from an outside break-in?" Internal drill asks the harder question: can the chain limit damage even when the highest-privileged role turns?**

**Test file:** [`tests/cross_stack/internal_hack_drill_test.go`](../tests/cross_stack/internal_hack_drill_test.go)

---

## Summary

| Iteration | Attack | New code added | Outcome |
|---|---|---|---|
| **Iter 1** | Authority sets `auto_unpause_at_block=0` → indefinite pause / DoS | — (baseline) | **Critical gap**: no cap on pause duration; compromised authority = permanent censorship. |
| **Iter 2** | Same attack, with `MaxPauseDurationBlocks` cap | +1 param + handler-level rewrite | Indefinite pauses structurally impossible. Overlong windows truncated. Honest short windows unchanged. |
| **Iter 3** | Authority amends `TraceSchema` maliciously | +1 query + 1 keeper log + wiring into 7 handlers | Detection now programmatic via `PrivilegedActions`; no more event-log trawling. Two amendments (attack + fix) both auditable. |
| **Iter 4** | Authority spams 30 fake incidents to bury a real P0 | **None** | **Convergence**: severity filter on `OpenIncidents` + `PrivilegedActions` burst detection absorb the attack. No new primitives required. |

**Gap delta across iterations: 1 → 1 → 0 → 0.** Convergence achieved.

---

## Attack-by-attack

### Iter 1 — indefinite pause DoS

The simplest internal attack on the circuit breaker: call `MsgPauseModule` with `auto_unpause_at_block=0`, wait forever. The Wave 12 breaker was designed for honest use; it accepted `0` as "no auto-unpause", assuming the caller was trustworthy.

**Finding:** compromised authority → permanent censorship of the paused module. Recovery requires key rotation (out of scope).

**Severity:** CRITICAL. A single compromised authority key could DoS the knowledge module indefinitely.

### Iter 2 — `MaxPauseDurationBlocks` cap

**Fix:** one new governance param (`MaxPauseDurationBlocks`, default 28,800 ≈ 40h at 5s blocks). Handler silently rewrites `auto_unpause_at_block` to `min(requested, paused_at + cap)` — compromised authority cannot set an indefinite or overlong window.

**Semantics:**
- `auto_unpause_at_block = 0` → rewritten to `paused_at + cap`.
- `auto_unpause_at_block > paused_at + cap` → truncated to cap.
- `auto_unpause_at_block` within cap → respected unchanged (honest short windows unaffected).

**Verified by three tests:** capped indefinite, truncated overlong, respected honest.

**Residual gap:** detection of abuse relies on event stream trawling. No unified "what has authority done recently?" surface.

### Iter 3 — `PrivilegedActionLog`

**Fix:** a new proto record (`PrivilegedAction`), one new query (`QueryPrivilegedActions`), and wiring into every authority-gated handler. Every call to `MsgPauseModule`, `MsgUnpauseModule`, `MsgCorrectManifestMerkleRoot`, `MsgOpenIncident`, `MsgResolveIncident`, `MsgCloseIncident`, `MsgAmendTokenizerSpec`, `MsgAmendTraceSchema` emits a structured record with monotonic sequence, invoker, target, incident binding.

**Attack scenario:** compromised authority amends `TraceSchema` maliciously. The handler accepts (this IS the attack surface — there's no way to prevent a legitimate-looking amendment), but the action lands in the log. External monitors polling `PrivilegedActions(type=SCHEMA_AMEND_TRACE)` detect the amendment; community inspects the new schema; if malicious, governance amends BACK to a sanitised version.

**Critical property:** both the attack amendment AND the corrective amendment appear in the log. The full audit trail is preserved even though the chain state has moved forward.

**Residual gap:** none observed. A compromised-authority amendment is structurally unpreventable (they have the power), but it is *always visible*, and the response primitives (pause, amend, incident) cover the recovery. Detection moved from "operator discretion" to "machine-parseable log query."

### Iter 4 — convergence check: incident spam

**Attack:** compromised authority spams 30 fake P3 incidents to bury a real P0 in the `OpenIncidents` dashboard.

**Response using existing primitives only:**
1. `OpenIncidents(severity=P0)` — the severity filter (Wave 11) isolates the real incident from the spam.
2. `PrivilegedActions(type=INCIDENT_OPEN)` — surfaces the burst; an indexer running anomaly detection flags the 30+1 INCIDENT_OPENs from the same invoker in the same block.
3. Real incident handled through the normal flow.

**No new msg types. No new query surfaces. No new proto types.** Convergence signal: the existing response surface absorbs the novel attack.

---

## What the drill proved

1. **Handler-level caps neutralise time-based abuse.** A single param + a `min()` call in the pause handler made indefinite-pause structurally impossible.
2. **A chain-wide audit log is load-bearing for internal resilience.** External drills could rely on "detect via integrity check" (merkle_root_valid). Internal drills can't — the attacker IS a legitimate caller. The log is the substitute for integrity: you can't prevent the action, but you can ensure it's observable, attributable, and timestamped.
3. **Severity filters matter.** The incident spam attack was absorbed by the pre-existing severity filter on `OpenIncidents`. The community dashboard wasn't designed as anti-spam infrastructure; it happens to be one because the filter exists.
4. **Authority-compromise is bounded but not eliminated.** Every fix in Wave 14 limits damage; none eliminates it. A compromised authority can still cause 40 hours of DoS, can still push a malicious amendment, can still spam incidents. What they cannot do: these actions silently, permanently, or without leaving a queryable trail.
5. **Convergence is tighter for internal attacks than external.** Wave 13 took 2 increments to converge; Wave 14 took 2 as well. For both, the same pattern held: 1 critical fix, 1 observability fix, then novel attacks absorbed by existing primitives.

---

## What Wave 14 does NOT solve

- **Persistent authority compromise.** If the authority key stays compromised for longer than the governance amendment cycle, the attacker can re-apply malicious changes after each correction. The answer is multi-sig / threshold-signature authority or a separate "guardian" role with veto power — future wave.
- **Collusion between authority and validators.** If validators AND authority are compromised, the compromised authority can pass governance proposals changing the resilience params themselves (including `MaxPauseDurationBlocks`). The only defence is decentralised stake + diverse validator set — out of scope for the knowledge module.
- **Silent attack surface outside the msg-server path.** Any code path that mutates state without passing through an authority-gated `msgServer` handler bypasses the log. We've audited the paths that exist; future waves must keep doing so when they add new handlers.
- **Log DoS via log-flooding.** A compromised authority can spam the log itself with null-effect calls (e.g., `OpenIncident` then immediate `CloseIncident` × 1000). Log is append-only; storage grows. Future wave might add a TTL param and BeginBlocker prune for ancient entries.

---

## The convergence equation (internal)

Same as Wave 13 but with new values:

```
G_1 = 0 (baseline)
G_2 = 1 (MaxPauseDurationBlocks param + handler cap)
G_3 = 1 (PrivilegedActionLog: 1 proto message, 1 keeper, 1 query, ~7 wire-ins)
G_4 = 0 (novel attack absorbed)
```

Converged at iter 4. For novel internal attacks in the same class (any combination of abuse of existing authority-gated handlers), `G_n = 0` should continue to hold — detection via log + bounded damage via caps + structured response primitives.

---

## Running the drill

```
go test -run 'TestInternalHackDrill_' -v -count=1 -timeout 120s ./tests/cross_stack/...
```

Run before every release. A new internal-attack class requires its own iteration. If the iteration needs new code, that code plus a new audit entry here belong in the release notes.

---

## The honest reckoning

Internal resilience is harder than external resilience. The external attacker has to break in; once in, the chain's integrity checks catch most outcomes. The internal attacker is already in, by design — they hold the key. What we can do is:

- **Cap their reach.** Handler-level maxima on durations, counts, depths.
- **Log their actions.** Every privileged call is a record that outlives the caller.
- **Preserve evidence.** Forward-only state transitions. Incident logs. Event streams.
- **Enable community override.** Not yet — Wave 15+ needs multi-sig authority and guardian-veto. This is the next frontier.

What we cannot do today is stop a single-signer authority from causing *bounded* damage. Bounded is better than unbounded. The box is smaller than the room, and we know its edges.

— **Route B, Wave 14 · Internal-Hack Audit** · 2026-04-24
