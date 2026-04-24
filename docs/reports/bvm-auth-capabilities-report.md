# BVM Auth & Capabilities Report — R23-3

**Date:** 2026-02-26
**Chain:** zerone-localnet (4 validators)
**Block range:** 19–202
**Binary:** build/zeroned (Cosmos SDK v0.50.15 + CometBFT v0.38.20)

---

## Summary

Tested DID-gated BVM execution end-to-end on localnet. The auth/DID infrastructure is complete (registration, session keys, capability presets), but **capability enforcement at the BVM opcode level is absent**. Agent opcodes (KVERIFY, KCITE) execute identically for DID-registered and anonymous callers. Session key creation fails with a protobuf parse error.

---

## Test 1: Auth Module DID Support

**Status:** PASS (with caveats)

**Observation:**
- Custom auth CLI registered under `zeroned tx zerone_auth` (not `tx auth`)
- Commands available: `register-account`, `create-session`, `revoke-session`, `rotate-key`, `freeze-account`, etc.
- Queries: `account`, `account-by-did`, `session-keys`, `frozen-accounts`, `params`
- DID format: `did:zrn:{first 32 hex chars of Ed25519 pubkey}`
- Validators are NOT auto-registered with DIDs at genesis
- Successfully registered val0 with DID:
  - Address: `zrn1pjm3h6npdes4setwjjcqu9alh8cpwteej80vac`
  - DID: `did:zrn:ef060b134f6cbf3d0d62454648a7b6c4`
  - TX: `B0BF018FCAB04A421A7EC3A4709E3F976A9DE08FC012D55ADECD894A4C2F402C`

**Design Notes:**
- DID registration requires a 64-hex-char Ed25519 pubkey, while validators use secp256k1
- The system stores the Ed25519 pubkey as the "identity key" separately from the Cosmos account key
- If the Cosmos account already has a pubkey set (e.g., after signing txs), the Ed25519 sync is skipped (`msg_server.go:94`)
- Default account flags: `CanSubmitClaims=true`, `CanChallenge=true`, reputation=0.5

---

## Test 2: Authenticated vs Anonymous BVM Calls

**Status:** GAP — No capability gating on agent opcodes

### Setup
- Deployed KVERIFY test contract: `zrn1contract14cf6f0db5f4cd522b119d937b320d0e6a3fa7be`
  - Bytecode: `60026001e160005260206000f3` (pushes two values, calls KVERIFY, returns result)
  - Deploy TX: `B85940CEC6E2A6CE745EFCA082D4B19B262EF10BC6044F18155B7FF032B67D3E`
- Created anonymous account: `zrn1epuf97423uf09amdmzygqyngtujh4dux36r0py` (funded, no DID)

### Results

| Caller | DID | Capabilities | BVM Gas | Success | KVERIFY Result |
|--------|-----|-------------|---------|---------|----------------|
| val0 | `did:zrn:ef060b...` | Full access (`*SessionCapabilities` populated) | 3021 | true | false (stub) |
| anon-agent | (none) | nil (C-1 secure default) | 3021 | true | false (stub) |

**Both calls identical.** The anonymous caller executes KVERIFY with the same result as the DID-registered caller.

### Root Cause

**`interpreter.go:789-804`** — KVERIFY opcode does NOT check `ctx.CallerDID` or `ctx.Capabilities`:
```go
case KVERIFY:
    claimIdWord, _ := interp.stack.Pop()
    voteHashWord, _ := interp.stack.Pop()
    // ...
    ok := host.KVerify("", claimIdBytes[:], voteHashBytes[:])  // Hardcoded "" for callerDID
```

Same issue for KCITE (`interpreter.go:806-819`): hardcodes `""` as callerDID.

KQUERY (`interpreter.go:772-787`): no capability gate needed — it's read-only.

### Security Note
- The `Capabilities` field in `ExecutionContext` is correctly populated in `msg_server.go:153-178`
- Anonymous callers get `nil` capabilities (secure default at the msg_server level)
- But the interpreter never reads `Capabilities` — it's dead data during execution

**Recommendation:** Add capability gates before KVERIFY/KCITE execution:
```go
case KVERIFY:
    if interp.ctx.Capabilities == nil || !interp.ctx.Capabilities.CanVote {
        interp.stack.Push(new(big.Int)) // deny
        interp.pc++
        break
    }
    // ... existing stub logic
```

---

## Test 3: Session Key Capability Restrictions

**Status:** BUG — Session creation fails on-chain

### Observation
- CLI command `zeroned tx zerone_auth create-session` exists with correct flags:
  `--can-transfer`, `--can-stake`, `--can-submit-claims`, `--can-vote`
- Generated tx JSON is structurally valid
- **On-chain submission fails:**
  ```
  code: 2 / codespace: sdk
  raw_log: 'failed to retrieve the message of type "zerone.auth.v1.SessionCapabilities": tx parse error'
  ```
- Fails with both default and `--sign-mode direct`
- TX hash: `59B287A3F58364DBA890830C3E804C0F894A55B863C9F1E8EEA959F53202E4A2`

### Root Cause
The `SessionCapabilities` protobuf message (nested within `MsgCreateSession`) cannot be resolved by the SDK's tx decoder. The type is registered in the proto file descriptor (`types.pb.go` init), but the Cosmos SDK v0.50 tx decoder appears unable to resolve it during transaction parsing.

**Security Note:** This blocks the entire session key capability restriction system. Without session keys, all DID-registered accounts default to full access.

**Recommendation:** Investigate proto registration. Possible fixes:
1. Register `SessionCapabilities` as a standalone interface implementation
2. Inline the capability fields directly in `MsgCreateSession` (avoid nested message)
3. Check if `cosmos.msg.v1.signer` annotation causes nested message resolution issues

---

## Test 4: Scheduled Execution Capability Inheritance

**Status:** PARTIAL — Legacy pathway works, capability-aware pathway has no CLI

### Legacy Pathway (MsgScheduleExecution via CLI)
- Successfully scheduled: `schedule-execution <contract> <input-data>`
- Schedule ID: `sched-197-0`
- Executed at block 198 in BeginBlock
- Success=true, gas_used=3021

| Event | Value |
|-------|-------|
| `zerone.bvm.execution_scheduled` | schedule_id=sched-197-0, execute_at_block=198 |
| `zerone.bvm.schedule_executed` | success=true, gas_used=3021, mode=BeginBlock |

### Capability-Aware Pathway (MsgScheduleContract)
- **No CLI command wired** — only accessible via SDK/gRPC
- Code at `msg_server.go:376-395` correctly snapshots capabilities at creation time
- Code at `msg_server.go:513-537` correctly uses stored snapshot at execution time
- TOCTOU analysis: **No vulnerability** — capabilities are immutable after snapshot

### Security Note: TOCTOU Resolution
| Aspect | Behavior |
|--------|----------|
| Capabilities resolved at | Schedule creation time (block X) |
| Storage | Persistent via `SetScheduleCapabilities()` |
| Used at execution time | Stored snapshot (immutable) |
| DID revoked between schedule and execution | DID checked at execution for resolution, but stored caps used regardless |
| Session key expired | Doesn't matter — snapshot is independent |

**Recommendation:** Wire `MsgScheduleContract` to CLI (add `--execute-at-block` and `--gas-limit` flags to `schedule-execution`, or create new subcommand).

---

## Test 5: Cross-Call Capability Propagation

**Status:** GAP — Capabilities dropped on internal calls

### Code Analysis (`interpreter.go:1104-1122`)

The `ExecuteCall` function constructs `subCtx` for internal CALL/DELEGATECALL/STATICCALL but **omits CallerDID and Capabilities**:

```go
subCtx := &ExecutionContext{
    Caller:             caller,
    Origin:             callerCtx.Origin,
    // ... all standard EVM fields propagated ...
    // CallerDID:       NOT SET → ""
    // Capabilities:    NOT SET → nil
}
```

| Opcode | Lines | CallerDID Propagated | Capabilities Propagated |
|--------|-------|---------------------|------------------------|
| CALL | 866-904 | No → "" | No → nil |
| STATICCALL | 906-940 | No → "" | No → nil |
| DELEGATECALL | 942-977 | No → "" | No → nil |
| CALLCODE | 0xF2 | Not implemented | N/A |

### Security Note
This is a **secure default** (nil capabilities = deny all). However:
- DELEGATECALL should preserve caller context (EVM convention: msg.sender = original caller)
- Currently drops CallerDID, creating asymmetry with EVM semantics
- No mechanism for explicit capability delegation exists

**Recommendation:**
1. DELEGATECALL: propagate CallerDID and Capabilities from callerCtx (matches EVM semantics)
2. CALL: propagate CallerDID but use nil Capabilities (least-privilege for cross-contract calls)
3. STATICCALL: keep nil Capabilities (read-only, no agent ops needed)

---

## Test 6: Capability Denial Events & Error Handling

**Status:** GAP — No capability denial events in BVM

### Findings

| Layer | Capability Check | Event Emitted | Error Type |
|-------|-----------------|---------------|------------|
| AnteHandler (`ante_zerone.go:734-864`) | Session key → tx-level msg type check | No event (pre-block rejection) | `ErrSessionCapabilityDenied` (code 16) |
| BVM CallContract (`msg_server.go:153-178`) | DID/caps resolution | No event | N/A (always proceeds) |
| Interpreter opcodes (`interpreter.go:772-819`) | None | None | N/A |

### Anonymous Caller Behavior
1. `msg_server.go:154-178`: CallerDID="" → caps stays nil
2. **Execution proceeds anyway** — anonymous calls are not rejected
3. Gas consumed normally (`msg_server.go:233`)
4. `contract_called` event emitted with success=true
5. No `capability_denied` or `anonymous_caller` event

### Gas Consumption
| Scenario | Gas Behavior |
|----------|-------------|
| Session key denied at AnteHandler | Gas=0 (pre-execution rejection) |
| Anonymous caller (no DID) | Full gas consumed |
| Opcode-level denial (future) | Would consume gas up to denial point |

**Recommendation:**
1. Emit `zerone.bvm.capability_denied` event when anonymous callers invoke agent opcodes
2. Include caller address, missing capability, and opcode in event attributes
3. Consider: should anonymous callers be denied BVM execution entirely, or only gated at opcode level?

---

## Test 7: Edge Cases — "DID + No Session Key = Full Access"

**Status:** DOCUMENTED — Intentional design, security implications noted

### Behavior (`msg_server.go:166-175`)
```go
if sc, found := m.authKeeper.GetSessionCapabilities(ctx, msg.Caller, blockHeight); found {
    caps = &vm.SessionCapabilities{...restricted...}
} else {
    // Identity/operational key with no session key → full access
    caps = &vm.SessionCapabilities{
        CanTransfer: true, CanStake: true,
        CanSubmitClaims: true, CanVote: true,
    }
}
```

| Scenario | CallerDID | Capabilities |
|----------|-----------|-------------|
| No DID registered | "" | nil (deny all) |
| DID + no session key | "did:zrn:..." | Full access (all true) |
| DID + active session key | "did:zrn:..." | Session-restricted |
| DID + expired session key | "did:zrn:..." | Full access (session not found) |
| DID revoked after registration | Not testable (no revoke-DID tx) | Stale DID in state |

### Security Implications
1. **DID + no session key = all capabilities**: This is intentional (identity key = full authority). Documented in comments. Reasonable design.
2. **Session key expiry silently upgrades to full access**: When a session key expires, `GetSessionCapabilities` returns false, and the caller silently gets full access. This could surprise users who expect expired sessions to be denied.
3. **No DID revocation mechanism**: There is no `revoke-did` or `deregister-account` tx. Once a DID is registered, it persists indefinitely. Account freezing (`freeze-account`) exists but only affects tx validation, not BVM capability resolution.

---

## AnteHandler Capability Enforcement (Working Layer)

While BVM opcodes lack capability gates, the **AnteHandler correctly enforces** session key capabilities at the transaction level:

**File:** `app/ante_zerone.go:734-864`

The `ZeroneCapabilityDecorator` validates:
- Session key pubkey matches a registered session
- All messages in the tx are permitted by the session's capabilities
- Default-deny: unrecognized message types rejected for session keys

**Capability presets** (`ante_zerone.go:938-983`):
```
knowledge-worker:      CanSubmitClaims, CanVote
partnership-operator:  CanPartnership
researcher:            CanResearch
autonomous-agent:      CanSubmitClaims, CanVote, CanPartnership, CanResearch, CanDispute, CanTransfer
dispute-arbiter:       CanDispute, CanVote
```

**Note:** This layer is blocked by the session creation bug (Test 3).

---

## Findings Summary

| # | Test | Status | Severity |
|---|------|--------|----------|
| 1 | Auth module DID support | **PASS** | — |
| 2 | Authenticated vs anonymous BVM calls | **GAP** | HIGH — no opcode-level capability gating |
| 3 | Session key creation | **BUG** | HIGH — proto parse error blocks session keys |
| 4 | Scheduled execution capabilities | **PARTIAL** | MEDIUM — capability-aware pathway has no CLI |
| 5 | Cross-call capability propagation | **GAP** | MEDIUM — capabilities dropped on internal calls |
| 6 | Capability denial events | **GAP** | LOW — no audit trail for anonymous/denied calls |
| 7 | "DID + no session key = full access" | **DOCUMENTED** | LOW — intentional, but expired session upgrades silently |

---

## Recommendations (Priority Order)

### P0 — Security Critical
1. **Fix session key creation proto parse error** — blocks the entire capability restriction system
2. **Add capability gates to KVERIFY/KCITE opcodes** — currently any account (anonymous or otherwise) can execute agent operations

### P1 — Functional Gaps
3. **Propagate CallerDID in DELEGATECALL** — maintains EVM semantic consistency
4. **Wire MsgScheduleContract to CLI** — capability-snapshotting is code-complete but CLI-unreachable
5. **Emit capability denial events** — needed for security audit trail

### P2 — Design Improvements
6. **Session key expiry behavior** — consider denying (not upgrading) when session expires
7. **DID revocation mechanism** — add deregister-account or revoke-DID tx
8. **Pass CallerDID to KVerify/KCite host functions** — currently hardcoded as "" in interpreter

---

## Appendix: Transaction Log

| TX Hash | Description | Code |
|---------|-------------|------|
| `B0BF01...` | Register val0 DID | 0 |
| `B85940...` | Deploy KVERIFY test contract | 0 |
| `EE2066...` | Fund anon-agent | 0 |
| `E53E1F...` | Call contract from val0 (DID) | 0 |
| `4C4CA8...` | Call contract from anon-agent (no DID) | 0 |
| `59B287...` | Create session key (FAILED) | 2 |
| `866C38...` | Schedule execution | 0 |
