# BVM Assessment Report

**Date:** 2026-02-26
**Sessions:** R23-1 through R23-4
**Localnet:** zerone-localnet (4 validators, Cosmos SDK v0.50.15 + CometBFT v0.38.20)

---

## Executive Summary

The BVM is a functional bytecode execution engine with solid fundamentals. Core operations (deploy, call, storage, gas metering, revert, value transfer) all work correctly on a live localnet. The home bridge (R23-4) is complete — contracts can query agent home state. However, the knowledge bridge is non-functional due to an encoding bug, the auth/capability system is effectively inert (session creation fails, opcodes ignore capabilities), and BVM logs are invisible to external tooling.

**The BVM is ready for testnet as a compute engine. It is not ready as a secure agent execution environment.** Three bugs must be fixed first: the KQUERY encoding mismatch, the session key proto parse error, and the missing capability gates on agent opcodes. Without these, agents cannot read knowledge state and there is no permission enforcement on agent-specific operations.

---

## Architecture Overview

### What the BVM Is

- EVM-compatible stack machine (146 opcodes, 0x00–0xFF)
- 3 knowledge bridge opcodes: KQUERY (0xE0), KVERIFY (0xE1), KCITE (0xE2)
- 3 home bridge opcodes: HQUERY (0xE3), HMEMORY (0xE4), HPARTNER (0xE5)
- DID/session key integration (CallerDID, SessionCapabilities populated in execution context)
- Scheduled execution with capability snapshot inheritance
- Contract storage persistence (SLOAD/SSTORE → Cosmos SDK KV store)
- Gas metering bridged to Cosmos SDK gas via `ConsumeGas()`
- EIP-2929 warm/cold access tracking
- Bytecode deduplication by SHA256 hash
- Deterministic contract addresses (deployer + block + nonce)

### What the BVM Is Not (Yet)

- Not a full smart contract platform (no Solidity compiler, no ABI encoding stdlib)
- Not EVM-compatible enough for existing tooling (no precompiles, no EIP-xxx conformance tests)
- No standard library for agent operations
- No event indexer (BVM logs discarded at SDK boundary)
- No contract verification or auditing tools
- No high-level language or assembler

---

## Feature Matrix

| Feature | Status | Evidence | Notes |
|---------|--------|----------|-------|
| Deploy contract | **PASS** | R23-1 Test 2 | Deploy cost deducted, metadata correct |
| Call contract | **PASS** | R23-1 Test 3 | Return data in protobuf response |
| Storage (SLOAD/SSTORE) | **PASS** | R23-1 Tests 3,7 | Persists across calls and ~200 blocks |
| Events (LOG0-LOG4) | **PARTIAL** | R23-1 Test 9 | Execute in interpreter, NOT bridged to SDK events |
| Payable calls (CALLVALUE) | **PASS** | R23-1 Test 10 | Value transfer + refund on revert |
| Static calls | **PASS** | R23-1 Test 8 | SSTORE rejected, pure reads OK |
| Revert/error handling | **PASS** | R23-1 Test 5 | Graceful failure, gas consumed, clear error |
| Gas metering (BVM↔SDK) | **PASS** | R23-1 Test 6 | Out-of-gas at SSTORE, SDK gas bridged |
| KQUERY (knowledge read) | **FAIL** | R23-2 Tests 2-3 | Encoding bug: 64-char hex key never matches stored fact IDs |
| KVERIFY (verification vote) | **STUB** | R23-2 Test 5 | Always returns false |
| KCITE (citation) | **STUB** | R23-2 Test 6 | Always returns true (no-op) |
| HQUERY (home query) | **PASS** | R23-4 Unit tests | Queries caller's primary home |
| HMEMORY (memory CID) | **PASS** | R23-4 Unit tests | Returns IPFS memory CID |
| HPARTNER (partnership) | **PASS** | R23-4 Unit tests | Returns partnership ID |
| CallerDID resolution | **PARTIAL** | R23-3 Test 2 | Resolved in msg_server, but never checked by opcodes |
| SessionCapabilities | **FAIL** | R23-3 Test 3 | Session creation fails (proto parse error) |
| Opcode capability gating | **FAIL** | R23-3 Test 2 | Anonymous callers execute KVERIFY/KCITE identically |
| Scheduled execution | **PARTIAL** | R23-3 Test 4 | Legacy pathway works; capability-aware has no CLI |
| Contract-to-contract calls | **PARTIAL** | R23-3 Test 5 | Works but drops CallerDID/Capabilities |
| CREATE/CREATE2 | **NOT_TESTED** | — | Opcodes exist in interpreter but untested on-chain |
| Bytecode assembler | **MISSING** | — | Hand-assembly only |

**Totals:** 10 PASS, 4 PARTIAL, 3 FAIL, 2 STUB, 1 NOT_TESTED, 1 MISSING

---

## Security Analysis

### Gas Model

| Aspect | Assessment |
|--------|-----------|
| BVM→SDK bridge | **Sound.** `ConsumeGas(result.GasUsed)` maps BVM gas directly to SDK gas meter |
| SDK base overhead | ~54-55K gas per call tx (ante handler, routing, events) |
| Max computation per block | 100M gas (MaxGasPerBlock param), limits scheduled executions |
| Max computation per call | 10M gas (MaxGasPerCall param) |
| EIP-2929 tracking | Implemented — cold/warm access costs for addresses and storage |
| DoS via gas manipulation | **Low risk.** Both SDK gas (tx fees) and BVM gas (execution limits) provide bounds |
| KCITE spam vector | **Medium risk.** 100 gas per KCITE allows 2,000 citations per call at default gas limits |

**Gas Recommendations:**

| Opcode | Current | Recommended | Rationale |
|--------|---------|-------------|-----------|
| KQUERY | 5,000 | 5,000 | Appropriate for cross-module read |
| KVERIFY | 3,000 | 8,000 | State-modifying, creates intent, cross-module |
| KCITE | 100 | 2,000 | State-modifying, anti-spam, cross-module write |
| HQUERY | 5,000 | 5,000 | Appropriate for cross-module read |
| HMEMORY | 5,000 | 5,000 | Appropriate for cross-module read |
| HPARTNER | 5,000 | 5,000 | Appropriate for cross-module read |

### State Access

| Question | Answer |
|----------|--------|
| Can contract A read B's storage? | **No.** Storage keys are namespaced by contract address |
| Can contract A read B's home state? | **Yes, via HMEMORY/HPARTNER.** Home data is already public (queryable by anyone) |
| Is HQUERY caller-restricted? | **Yes.** Uses `ctx.Caller` address — contracts can only query the calling agent's home |
| Knowledge bridge — read or write? | KQUERY is read-only. KVERIFY/KCITE are state-modifying (when implemented) |
| Home bridge — read or write? | All three opcodes are read-only (`IsStateModifier=false`) |

### Auth/Capability Model

| Aspect | Status | Notes |
|--------|--------|-------|
| Anonymous caller denial (C-1) | **NOT ENFORCED** | msg_server resolves DID but opcodes ignore capabilities |
| Session key restriction | **BROKEN** | Proto parse error blocks session creation entirely |
| Capability propagation in nested calls | **DROPPED** | CALL/DELEGATECALL/STATICCALL set CallerDID="" and Capabilities=nil |
| Stale capability in scheduled execution | **SAFE** | Capabilities are snapshot at creation, immutable after |
| Session expiry behavior | **SILENTLY UPGRADES** | Expired session → GetSessionCapabilities returns false → full access |
| DID revocation | **MISSING** | No revoke-DID or deregister tx exists |

### Known Vulnerabilities

| # | Severity | Description | File | Fix |
|---|----------|-------------|------|-----|
| V-1 | **CRITICAL** | KQUERY encoding bug makes all 777 genesis axioms and all generated facts unreachable | `msg_server.go:612` | Replace `hex.EncodeToString` with UTF-8 `string(bytes.TrimRight(factId, "\x00"))` |
| V-2 | **HIGH** | Session key creation fails with proto parse error, blocking entire capability restriction system | `MsgCreateSession` proto | Investigate proto registration; may need to inline SessionCapabilities fields |
| V-3 | **HIGH** | No capability gating on KVERIFY/KCITE opcodes — anonymous callers execute identically | `interpreter.go:789-819` | Add capability checks before executing agent opcodes |
| V-4 | **MEDIUM** | BVM logs (LOG0-LOG4) discarded — no observability for contract events | `msg_server.go:254-260` | Emit as `zerone.bvm.contract_log` SDK events |
| V-5 | **MEDIUM** | KCITE at 100 gas enables citation spam (2,000 citations per call) | `gas.go:83` | Increase to ≥2,000 gas |
| V-6 | **MEDIUM** | CallerDID dropped on CALL/DELEGATECALL/STATICCALL | `interpreter.go:1104-1122` | Propagate CallerDID in DELEGATECALL; use nil caps for CALL |
| V-7 | **LOW** | CallerDID hardcoded as "" in KVERIFY/KCITE host calls | `interpreter.go:795,810` | Pass `execCtx.CallerDID` |
| V-8 | **LOW** | Expired session key silently upgrades to full access | `msg_server.go:166-175` | Consider denying instead of upgrading |
| V-9 | **LOW** | Gas auto-estimation can be below 200K minimum per message | CLI | Document workaround; always use explicit `--gas 300000` |

---

## Stub Completion Roadmap

### KVERIFY — Priority: Medium

**Current:** Always returns false (`msg_server.go:620-622`)
**Architectural challenge:** Zerone's PoT uses two-phase commit-reveal across 10+10+5 blocks. A BVM call executes in one block and cannot participate in a multi-block protocol.
**Recommended design:** Queued Intent (Option B from R23-2)
1. `KVERIFY(claimId, saltedVote)` → creates a `VerificationIntent` in state
2. BeginBlocker handles commit/reveal across blocks
3. Contract later calls KQUERY to check outcome
**New infrastructure:** VerificationIntent proto, BeginBlocker intent processing, index by status+block
**Effort:** Medium-high (2-3 weeks). Precedent: `MsgScheduleExecution` deferred execution
**Recommendation:** Implement with queued intent. Removing the opcode is an acceptable fallback if the complexity is too high for testnet.

### KCITE — Priority: High

**Current:** Always returns true, no-op (`msg_server.go:624-626`)
**Infrastructure exists:** `BillingKnowledgeAdapter.IncrementCitationCount()` already implements the core logic. Fact type has `IncomingCitationCount` field.
**Implementation:**
1. Fix encoding (same UTF-8 trim as KQUERY)
2. Check fact exists and is in citable state (VERIFIED or ACTIVE)
3. Self-citation prevention (callerDID != fact.Submitter)
4. Increment `IncomingCitationCount` via existing adapter
5. Track for metabolism energy via `IncrementNewCitationEpoch`
**Anti-spam:** Gas increase to 2,000 + self-citation check for v1. Add rate limiting via governance param if needed.
**Effort:** Low (<1 day). Wire existing adapter + encoding fix + self-citation check.
**Recommendation:** Implement for testnet. Low risk, high utility.

### Spending Limit Enforcement — Priority: High

**Current:** Spending limits stored in x/home but NOT checked by BVM value transfers (`msg_server.go:140-149`).
**Needed:** Before `SendCoinsFromAccountToModule`, check `homeKeeper.GetSpendingLimit(caller)` and enforce.
**Effort:** Small — add a check in `CallContract` before value transfer.
**Recommendation:** Implement before testnet.

### MsgRecoverHome — Priority: Medium

**Current:** Recovery addresses stored in home state, but no recovery mechanism wired.
**Needed:** A `MsgRecoverHome` that allows recovery addresses to claim ownership.
**Effort:** Medium (1-2 days for the message + keeper logic + CLI).
**Recommendation:** Implement before mainnet. Not blocking for testnet.

---

## What Would an Agent SDK Look Like?

The BVM is currently programmable only via raw hex bytecode. For agents to use it, they need:

### Layer 1: Assembler (Minimum Viable)

A simple assembler converting mnemonics to hex:

```
# agent-contract.basm
.entry main

main:
    CALLER            # push my address
    HQUERY            # check if I have a home
    PUSH1 0x00
    EQ                # no home?
    JUMPI no_home

    # I have a home — query a fact
    PUSH8 "AGRT-000"  # fact ID (ASCII bytes)
    KQUERY
    # ... branch on confidence ...

no_home:
    PUSH1 0x00
    PUSH1 0x00
    REVERT
```

Features: all opcodes by mnemonic, labels (JUMPDEST targets), PUSH with hex/string immediates, comments.

### Layer 2: ABI Encoding

Standard calldata format so contracts can expose typed function interfaces:
- Function selector (4-byte keccak256 hash)
- Argument encoding (uint256, bytes32, string)
- Return data encoding

### Layer 3: Standard Library

Common agent patterns as reusable bytecode snippets:
- `require_home()` — revert if caller has no home
- `query_fact(id) → confidence` — KQUERY with encoding
- `require_capability(cap)` — check CallerDID + capabilities
- `safe_transfer(to, amount)` — value transfer with balance check

### Layer 4: Contract Templates

Pre-built contracts for common agent scenarios:
- "Fact-gated decision maker" — queries knowledge, branches on confidence
- "Home-aware agent" — checks home state before acting
- "Scheduled observer" — runs periodically, logs observations

### Minimal Viable SDK

A Go tool at `tools/bvm-asm/main.go` that converts `.basm` → hex bytecode would make the BVM usable for testing without building a full compiler. This is the single most impactful developer experience improvement.

---

## Improvement Priorities

### P0 — Before Testnet

- [ ] **Fix KQUERY encoding bug** — replace `hex.EncodeToString` with UTF-8 string interpretation (V-1)
- [ ] **Fix session key creation proto parse error** — investigate proto registration (V-2)
- [ ] **Add capability gates to KVERIFY/KCITE opcodes** — deny anonymous callers (V-3)
- [ ] **Implement KCITE** — wire existing `IncrementCitationCount` adapter
- [ ] **Bridge BVM logs to SDK events** — emit `zerone.bvm.contract_log` events (V-4)
- [ ] **Increase KCITE gas to 2,000** — prevent citation spam (V-5)

### P1 — Before Mainnet

- [ ] Propagate CallerDID in DELEGATECALL (V-6)
- [ ] Pass CallerDID to KVERIFY/KCITE host functions (V-7)
- [ ] Wire MsgScheduleContract to CLI (capability-aware scheduling)
- [ ] Implement spending limit enforcement in BVM value transfers
- [ ] Session key expiry behavior review (deny vs upgrade)
- [ ] Bytecode assembler tool (`tools/bvm-asm`)
- [ ] Add `return_data` attribute to `contract_called` event

### P2 — Post-Launch

- [ ] KVERIFY implementation (queued intent design)
- [ ] DID revocation mechanism
- [ ] Agent SDK / high-level language
- [ ] Contract verification tooling
- [ ] Event indexer for BVM logs
- [ ] E2E test suite for knowledge bridge (post encoding fix)

### P3 — Future

- [ ] EVM precompiles (SHA256, RIPEMD160, ecrecover)
- [ ] Cross-contract standard patterns
- [ ] On-chain contract registry
- [ ] CREATE/CREATE2 E2E testing
- [ ] Contract upgrade mechanism

---

## Verdict

### Is the BVM ready for testnet agents?

**CONDITIONAL**

The BVM is ready as a **compute engine** — deploy, call, storage, gas metering, and error handling all work correctly on a live chain. The home bridge is complete and tested. The core interpreter is solid.

It is **not ready** as a **secure agent execution environment** without three fixes:

1. **KQUERY encoding (Critical):** Agents cannot read knowledge state. The entire knowledge bridge is non-functional. This is a 1-line fix.

2. **Session key creation (High):** The capability restriction system is completely blocked by a proto parse error. All DID-registered accounts default to full access with no way to restrict. Investigation needed to determine fix scope.

3. **Capability gating on agent opcodes (High):** Anonymous callers can execute KVERIFY/KCITE identically to DID-registered agents. A few-line fix per opcode.

**With these three fixes applied, the BVM is testnet-ready.** The remaining issues (KVERIFY stub, LOG bridging, KCITE implementation, gas repricing) are improvements that can ship iteratively.

**Without these fixes, deploying agent contracts would be misleading** — agents would appear to operate on knowledge state but actually read zeros, and there would be no permission boundary between authenticated and anonymous callers.

### Effort Estimate for Testnet Readiness

| Fix | Effort | Risk |
|-----|--------|------|
| KQUERY encoding | 1 line + test | Very low |
| Capability gates on opcodes | ~10 lines per opcode | Low |
| Session key proto parse | Investigation needed | Medium (proto debugging) |
| KCITE implementation | <1 day | Low |
| BVM log bridging | ~30 lines | Low |
| KCITE gas repricing | 1 line | Very low |

**Total: 1-3 days of focused work** to reach testnet readiness, assuming the session key proto issue has a straightforward resolution.
