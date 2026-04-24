# Agent Onboarding Report

**Date:** 2026-02-26
**Sessions:** R24-1 through R24-5
**Chain:** zerone-localnet (Cosmos SDK v0.50.15, CometBFT v0.38.19/20)

---

## Executive Summary

**Can an external agent-human pair join ZERONE?** Yes — but with significant friction, several critical bugs, and gaps in documentation that would block most operators without expert guidance.

**How long does it take?** The working steps of identity + home + validator registration take ~15 minutes on localnet (~7 transactions, ~11 ZRN in fees). Cloud deployment adds 10–30 minutes depending on installation method. Total: **30–60 minutes** for a knowledgeable operator.

**What breaks?**
- **3 critical bugs** block core functionality (proto codec mismatch, key rotation account bricking, join-testnet.sh crash)
- **5 high-severity bugs** degrade the experience (irreversible deactivation, unbonding query broken, dual registration undocumented, hardcoded chain-id, configure-node.sh macOS incompatibility)
- **9 documentation discrepancies** in VALIDATOR-GUIDE.md (wrong module names, incorrect commands, stale values)
- **2 missing features** (no commission update, no unjail/reactivation)

**Bottom line:** An experienced Cosmos operator could join with workarounds. An agent or first-time operator would fail at multiple steps without hand-holding.

---

## The Onboarding Journey

### Current State (Step-by-Step)

| Step | Action | Time | Cost | Status | Notes |
|------|--------|------|------|--------|-------|
| 1 | Obtain binary | 2–15m | $0 | PASS | Docker, cross-compile, or build from source all work |
| 2 | Initialize node | <1m | $0 | PASS | `zeroned init` works correctly |
| 3 | Join testnet | 2–10m | $0 | PARTIAL | join-testnet.sh crashes (Bug 1); manual path works |
| 4 | Register DID | <1m | 103,938 uzrn | PASS | `zeroned tx zerone_auth register-account` (not `auth`) |
| 5 | Create home | <1m | ~10,300,000 uzrn | PASS | 10 ZRN creation fee dominates; gas estimation unreliable |
| 6 | Register keys | <1m | ~150,000 uzrn | PASS | Home key system works; auth session keys BLOCKED |
| 7 | Fund account | <1m | faucet | N/A | No faucet exists yet — requires manual val0 transfer |
| 8 | Register validator (zerone_staking) | <1m | ~150,000 uzrn | PASS | Raw integer self-delegation (not coin string) |
| 8b | Register validator (SDK staking) | 2–3m | ~200,000 uzrn | PASS | **Undocumented** — required for CometBFT consensus |
| 9 | Self-delegate | <1m | ~150,000 uzrn | PASS | Upsert works; `update-stake` for changes |
| 10 | Tier progression | hours–days | — | PASS (design) | Automatic but requires verification round participation |
| 11 | Deploy BVM contract | <1m | ~250,000 uzrn | PASS | Hand-assembled hex bytecode only — no assembler |
| 12 | First PoT round | ~25 blocks | — | AUTO | Vote extensions handle this; requires Scholar+ tier |

**Total onboarding time:** ~30–60 minutes (localnet to cloud)
**Total onboarding cost:** ~11 ZRN identity + ~1 ZRN validator registration + VPS ~€4–50/mo
**Number of transactions:** 7–9 (identity flow + validator)
**Critical blockers found:** 3

---

## Identity Stack Assessment

### DID Registration (x/zerone_auth)

**Status:** Mostly works. 2 of 12 RPCs blocked by proto bug.

- **Registration:** Works via `zeroned tx zerone_auth register-account [did] [public-key] [account-type]`
- **DID format:** `did:zrn:{32 or 64 hex chars}` — must derive from Ed25519 identity key
- **Account types:** `agent`, `human`, `contract`, `system`
- **Validation:** Duplicate DID/account correctly rejected; DID derivation enforced
- **Cost:** 103,938 uzrn gas
- **Reputation:** Starts at 500,000 (0.5 on 0–1 scale)

**Session keys — BROKEN (BUG-1: Proto Codec Mismatch):**
- `MsgCreateSession` fails: `failed to retrieve the message of type "zerone.auth.v1.SessionCapabilities": tx parse error`
- Root cause: Proto files generated with `protoc-gen-go` (Google v2) instead of `protoc-gen-gogo` (required by Cosmos SDK v0.50)
- Any message with nested proto message fields fails; scalar-only messages work
- Impact: Session key creation and recovery config both completely blocked

**Key rotation — DANGEROUS (BUG-2: Account Bricking):**
- `MsgRotateKey` succeeds but syncs Ed25519 key to Cosmos BaseAccount
- Cosmos SDK v0.50's `sigverify.go:433` rejects Ed25519 for standard accounts
- Result: Account permanently locked — no further transactions possible
- Impact: Any agent who rotates their key is permanently bricked

**Freeze/unfreeze — PARTIAL:**
- Self-freeze works correctly (ante handler blocks all txs)
- Self-unfreeze impossible — ante handler blocks unfreeze tx too
- Only governance proposals can unfreeze — impractical for agents
- Design issue: self-freeze is a one-way operation without governance

**Authorization signature — NO-OP (BUG-3):**
- `authorization_signature` field in `MsgRotateKey` accepted but never validated
- Dummy random signatures succeed — provides false security

### Home Creation (x/home)

**Status:** Works correctly.

- CLI: `zeroned tx home create-home [name]`
- Creation fee: 10,000,000 uzrn (10 ZRN) — dominates onboarding cost
- Home ID: auto-increment (`home-1`, `home-2`, etc.)
- Gas estimation unreliable — `--gas auto` returns below minimum; requires `--gas 300000`
- **DID NOT required** to create a home — an account without DID can create homes
- Key registration and session management work correctly

**Design question:** Should home creation require DID registration? Currently no enforcement, which allows identity-less homes.

### Key Management Complexity

An agent must understand **5 separate key systems:**

| # | Key System | Type | Purpose | Status |
|---|-----------|------|---------|--------|
| 1 | Cosmos keyring | secp256k1 | Transaction signing | Working |
| 2 | Auth identity key | Ed25519 | DID identity | Working |
| 3 | Auth session key | Ed25519 + capabilities | Delegated access | BROKEN (proto bug) |
| 4 | Home registered key | Hash-based + permissions | Home-specific access | Working |
| 5 | Validator consensus key | Ed25519 (CometBFT) | Block signing | Working |

**Consolidation opportunities:**

1. **Auth sessions ↔ Home sessions:** These are completely independent systems with no integration. An auth session key has no relationship to a home registered key. Should be unified or clearly documented as distinct.
2. **Home keys ↔ Auth keys:** Home keys could be derived from auth identity keys. Currently requires separate manual registration in both systems.
3. **5 key systems is too many for an agent.** A programmatic agent needs to manage secp256k1 for signing, Ed25519 for identity, and separate registrations in two different permission systems. This should collapse to 2–3 key types max.

---

## Validator Experience Assessment

### Registration Flow

**Status:** Works, but requires undocumented dual registration.

**zerone_staking registration:**
- CLI: `zeroned tx zerone_staking register-validator [pubkey-hex] [self-delegation] --moniker --commission`
- Self-delegation is a raw integer (e.g., `1111000000`), NOT a coin string — guide says `111000uzrn` (wrong)
- Commission in BPS (500 = 5%), immutable after registration
- Consensus pubkey: any hex string accepted (no CometBFT validation)
- Initial tier: Apprentice (regardless of stake — verifications needed for promotion)

**SDK staking registration (UNDOCUMENTED — BUG-5):**
- External validators MUST ALSO register via `zeroned tx staking create-validator` with a `validator.json` file
- Without this, the validator participates in PoT but never signs CometBFT blocks
- VALIDATOR-GUIDE does not document this step at all
- Genesis validators exist in both systems; external validators must register in both manually

### Tier Progression

**How it works:** Automatic. `ComputeValidatorTier()` evaluates on every delegation change and in every `EndBlocker`.

**Requirements:**

| From → To | Stake | Verifications | Accuracy | Practical Time |
|-----------|-------|---------------|----------|----------------|
| → Apprentice | 0.111 ZRN | 0 | — | Instant at registration |
| → Verified | 1.11 ZRN | 22 | 77% | ~22 verification rounds |
| → Scholar | 1,111 ZRN | 11 | 50% | 11 rounds + large stake |
| → Guardian | 11,111 ZRN | 333 | 77% | Weeks–months |

**Note:** Scholar has lower verification requirements (11 at 50%) than Verified (22 at 77%) — the barrier is stake (1,111 ZRN). A well-funded validator could skip Verified and reach Scholar after 11 correct verifications.

**BUG:** `min_reputation` is defined in TierConfig but never checked by `ComputeValidatorTier()`. Dead field.

### Operational Readiness

| Aspect | Status | Notes |
|--------|--------|-------|
| Slashing | Internal only | Triggered by knowledge module, not CLI-invocable |
| Progressive escalation | Works (code) | +10% per prior slash, max 2/epoch, 3 = deactivation |
| Unjail | MISSING | `jailed`, `jail_reason`, `unjail_after_block` fields exist but are dead code |
| Reactivation | MISSING | Once `is_active=false`, no mechanism to reactivate (BUG) |
| Commission update | MISSING | Commission is immutable after registration |
| Unbonding query | BROKEN | CLI calls wrong gRPC method; no unbondings query handler exists |
| Exit process | No explicit deregistration | Withdraw all self-delegation → auto-deactivate (irreversible) |

---

## Infrastructure Assessment

### Build & Distribution

| Method | Status | Time | Size | Notes |
|--------|--------|------|------|-------|
| Build from source | PASS | ~5m | ~70MB | Requires Go 1.24+ (not 1.22 as guide says) |
| Cross-compile (CGO_ENABLED=0) | PASS | ~3m | ~70MB each | linux/amd64, linux/arm64, darwin/arm64 all work |
| Docker image | PASS | ~5m build | ~100MB runtime | Multi-stage, debian-slim, includes curl/jq |
| Validator Docker (Cosmovisor) | PASS | ~6m build | ~130MB | Includes cosmovisor binary |
| Docker Compose | PASS | — | — | Persistent volume, port mapping, restart policy |
| Reproducible builds | NOT TESTED | — | — | `-trimpath` added to LDFLAGS but not verified |

**Cross-compilation works without CGO** — no ledger or SQLite dependencies in the current build.

### Deployment Tooling

| Tool | Accuracy | Issues |
|------|----------|--------|
| `join-testnet.sh` | FAIL — 3 bugs | `read_seeds()` crash, hardcoded chain-id, wrong validate-genesis command |
| `configure-node.sh` | PARTIAL | macOS BSD sed incompatibility for `--enable-api` and `--enable-grpc` |
| Cosmovisor setup | PASS (manual) | Cannot test via script (script crashes first); manual setup works |
| Systemd service | NOT TESTED | Described in R24-5 prompt but no cloud deployment was executed |

### Documentation Accuracy

| Document | Accuracy | Missing Sections | Incorrect Commands |
|----------|----------|------------------|--------------------|
| VALIDATOR-GUIDE.md | 5/10 | SDK staking create-validator step, zerone_staking vs staking module distinction | 9 discrepancies (see below) |
| PRODUCTION-STACK.md | 8/10 | Testnet-specific guidance | None found — planning doc, not tested against real infra |
| LAUNCH-CHECKLIST.md | 7/10 | DID/identity bootstrapping steps, dual validator registration | References standard Cosmos flow, misses zerone-specific steps |

**VALIDATOR-GUIDE.md Discrepancies (9):**

| # | Section | Issue | Fix |
|---|---------|-------|-----|
| 1 | Prerequisites | Go 1.22+ | Go 1.24+ (from go.mod) |
| 2 | Manual Setup §2 | `zeroned validate-genesis` | `zeroned genesis validate` |
| 3 | Becoming a Validator §3 | `zeroned tx auth register-account` | `zeroned tx zerone_auth register-account` |
| 4 | Becoming a Validator §4 | `zeroned tx staking register-validator` | `zeroned tx zerone_staking register-validator` |
| 5 | Becoming a Validator §4 | Self-delegation `111000uzrn` | Raw integer `111000` (no denomination) |
| 6 | Becoming a Validator | — | Missing SDK staking `create-validator` step entirely |
| 7 | Tier Progression | `zeroned tx staking update-stake` | `zeroned tx zerone_staking update-stake` |
| 8 | PoT Participation | Phase durations 4/4/3 blocks | 10/10/5 blocks (from on-chain params) |
| 9 | Monitoring | `query staking validators` only | Should mention both SDK and zerone_staking query paths |

---

## What's Missing

### For Testnet Launch

- [ ] **Faucet** — agents need initial funds; currently requires manual transfer from a genesis validator
- [ ] **Genesis distribution mechanism** — no documented process for distributing initial ZRN to external validators
- [ ] **Persistent peer list publication** — seeds.txt is empty (comments only); no published peer list
- [ ] **Block explorer** — PRODUCTION-STACK.md plans Ping.pub/BigDipper but not deployed
- [ ] **Public RPC endpoint** — no external-facing RPC for light clients or external queries
- [ ] **Fix join-testnet.sh** — 3 bugs make it unusable (`read_seeds()` crash, hardcoded chain-id, wrong validate-genesis)
- [ ] **Fix configure-node.sh** — macOS sed incompatibility silently fails for API/gRPC section editing

### For Agent-Specific Onboarding

- [ ] **One-command agent bootstrap** (`zeroned agent init` that does DID + home + keys in one tx or script)
- [ ] **Agent onboarding script** (automates steps 4–7: fund → register DID → create home → register keys)
- [ ] **Programmatic API for identity** — not just CLI; agents need SDK/gRPC bindings
- [ ] **Agent-to-agent discovery** — how does one agent find another? No registry or discovery mechanism
- [ ] **BVM assembler** — hand-assembled hex bytecode is the only option; `tools/bvm-asm` proposed but not built
- [ ] **Unified key management** — collapse 5 key systems to 2–3

### For Mainnet

- [ ] **Key security guidance** — HSM, Horcrux threshold signing documented in PRODUCTION-STACK.md but not tested
- [ ] **Backup and recovery procedures** — recovery config setup blocked by proto bug; no tested recovery flow
- [ ] **Monitoring and alerting templates** — Prometheus/Grafana described but no ready-to-use dashboards
- [ ] **Upgrade coordination process** — Cosmovisor setup works but no upgrade proposal flow tested
- [ ] **Commission update mechanism** — commission is immutable after registration
- [ ] **Validator reactivation** — deactivation is permanent; no unjail or reactivation tx
- [ ] **Unbonding query** — CLI and gRPC both broken for unbonding entries

---

## Improvement Priorities

### P0 — Before Testnet Launch

- [ ] **Fix proto codec (BUG-1):** Re-generate proto files with `protoc-gen-gogo` for Cosmos SDK v0.50 compatibility. Unblocks session keys, recovery config, and any future nested message types.
- [ ] **Fix key rotation account bricking (BUG-2):** Don't sync Ed25519 keys to Cosmos BaseAccount, or add Ed25519 support to the ante handler.
- [ ] **Fix join-testnet.sh (BUG-4,5,6):** `read_seeds()` crash, hardcoded chain-id, wrong validate-genesis command.
- [ ] **Fix VALIDATOR-GUIDE.md:** All 9 discrepancies, especially the missing SDK staking registration step.
- [ ] **Deploy faucet:** Agents cannot onboard without initial funds.
- [ ] **Publish peer/seed list:** seeds.txt is empty; external nodes cannot discover the network.
- [ ] **Fix KQUERY encoding bug:** 1-line fix in BVM msg_server.go enables all 777 genesis axioms to be readable.
- [ ] **Add capability gates to BVM agent opcodes:** KVERIFY/KCITE execute identically for anonymous callers.

### P1 — First Testnet Week

- [ ] **Add validator reactivation mechanism:** `MsgReactivateValidator` or auto-reactivation when self-delegation restored.
- [ ] **Fix unbonding query:** CLI calls wrong gRPC method; add proper unbondings query handler.
- [ ] **Add commission update tx:** `MsgUpdateCommission` with rate-limiting.
- [ ] **Fix configure-node.sh macOS compatibility:** Replace BSD sed with portable approach.
- [ ] **Validate or remove `authorization_signature`:** Currently a no-op providing false security.
- [ ] **Check `min_reputation` in ComputeValidatorTier:** Or remove the dead field.
- [ ] **Self-unfreeze mechanism:** Allow time-delayed self-unfreeze so agents aren't permanently locked.

### P2 — Before Mainnet

- [ ] **Unified key management:** Consolidate auth sessions and home sessions; derive home keys from auth keys.
- [ ] **Agent onboarding script/CLI:** One-command bootstrap for identity + home + keys.
- [ ] **BVM assembler tool:** `tools/bvm-asm` to convert mnemonics to hex bytecode.
- [ ] **Implement KCITE:** Wire existing `IncrementCitationCount` adapter (estimated <1 day).
- [ ] **Bridge BVM logs to SDK events:** Emit `zerone.bvm.contract_log` events.
- [ ] **DID requirement for home creation:** Gate home creation on DID registration.
- [ ] **Consensus pubkey validation:** Validate against CometBFT Ed25519 requirements at registration.
- [ ] **Validator deregistration flow:** Explicit exit command with delegator protection.

---

## Complete Bug Inventory (R24-1 through R24-5)

### Critical (3)

| # | Source | Component | Description |
|---|--------|-----------|-------------|
| 1 | R24-1 | x/zerone_auth proto | Proto codec mismatch — nested messages fail (`MsgCreateSession`, `MsgSetRecoveryConfig`). Blocks session keys and recovery entirely. |
| 2 | R24-1 | x/zerone_auth rotate-key | Key rotation syncs Ed25519 to Cosmos BaseAccount → permanently locks account. |
| 3 | R24-3 | join-testnet.sh | `read_seeds()` crashes on empty/comment-only seeds.txt with `set -euo pipefail`. |

### High (5)

| # | Source | Component | Description |
|---|--------|-----------|-------------|
| 4 | R24-2 | zerone_staking | Deactivation (`is_active=false`) is irreversible — no reactivation path. |
| 5 | R24-2 | zerone_staking CLI | `CmdQueryUnbondings` calls `DelegatorDelegations`; no gRPC unbondings handler. |
| 6 | R24-3 | Architecture/Docs | Dual registration (zerone_staking + SDK staking) completely undocumented. |
| 7 | R24-3 | join-testnet.sh | Hardcoded chain-id `zerone-testnet-1` prevents use with any other network. |
| 8 | R24-3 | configure-node.sh | macOS BSD sed fails for API/gRPC section editing (silent failure). |

### Medium (6)

| # | Source | Component | Description |
|---|--------|-----------|-------------|
| 9 | R24-1 | x/zerone_auth | `authorization_signature` in `MsgRotateKey` is accepted but never validated. |
| 10 | R24-2 | zerone_staking | `min_reputation` in TierConfig defined but never checked. Dead field. |
| 11 | R24-2 | zerone_staking | No commission update mechanism. Immutable after registration. |
| 12 | R24-2 | zerone_staking | No unjail tx. `jailed`, `jail_reason`, `unjail_after_block` fields are dead code. |
| 13 | R24-3 | join-testnet.sh | `validate-genesis` command doesn't exist (should be `genesis validate`). |
| 14 | R24-3 | Docs | Self-delegation format `111000uzrn` should be raw integer `111000`. |

### Low (2)

| # | Source | Component | Description |
|---|--------|-----------|-------------|
| 15 | R24-3 | zerone_staking | Consensus pubkey stored as JSON vs hex inconsistency. |
| 16 | R24-1 | x/zerone_auth | Self-freeze is one-way without governance — design issue, possibly intentional. |

### From BVM Assessment (R23, Cross-Referenced)

| # | Severity | Description |
|---|----------|-------------|
| V-1 | CRITICAL | KQUERY encoding bug — 777 genesis axioms unreachable from BVM |
| V-2 | HIGH | Session key creation proto parse error (same as BUG-1 above) |
| V-3 | HIGH | No capability gating on KVERIFY/KCITE — anonymous callers unrestricted |
| V-4 | MEDIUM | BVM logs (LOG0-LOG4) discarded at SDK boundary |
| V-5 | MEDIUM | KCITE at 100 gas enables citation spam |

---

## The Agent Bootstrap Script

The onboarding has 9+ manual steps. Below is the design for a single-command bootstrap:

```bash
#!/usr/bin/env bash
# zeroned-agent-bootstrap.sh
# Sets up a complete agent on ZERONE in one command
#
# Usage: ./zeroned-agent-bootstrap.sh --name "my-agent" --type agent [--validator]
#
# What it does:
#
# Phase 1: Key Generation
#   1. Generate Cosmos keyring key (secp256k1) for tx signing
#   2. Generate Ed25519 identity key for DID
#   3. Derive DID from identity key: did:zrn:{hex(pubkey[:32])}
#
# Phase 2: Fund & Register
#   4. Request funds from faucet (or prompt for manual funding)
#   5. Register DID: zeroned tx zerone_auth register-account [did] [pubkey] [type]
#      Wait for tx inclusion.
#
# Phase 3: Home Setup
#   6. Create home: zeroned tx home create-home [name]
#      Cost: 10 ZRN creation fee. Wait for tx inclusion.
#   7. Register signing key on home: zeroned tx home register-key [home-id] [key-hash] [key-type] [role] [permissions]
#   8. Start home session: zeroned tx home start-session [home-id] [key-hash] [requested-permissions]
#
# Phase 4: Validator (Optional, --validator flag)
#   9. Wait for node to sync: poll zeroned status until catching_up=false
#   10. Register in zerone_staking: zeroned tx zerone_staking register-validator [pubkey] [self-delegation]
#   11. Register in SDK staking: zeroned tx staking create-validator --from [key] (validator.json)
#   12. Verify: check both validator sets for the new entry
#
# Phase 5: Verification
#   13. Query DID resolution (bidirectional)
#   14. Query home state
#   15. Query validator status (if applicable)
#   16. Print summary: address, DID, home ID, validator status, total cost
#
# Estimated total: 7-9 transactions, ~12 ZRN, ~5 minutes

echo "Welcome to ZERONE. Let's set up your agent."
echo ""
echo "This script will:"
echo "  1. Generate keys (secp256k1 + Ed25519)"
echo "  2. Register your DID identity"
echo "  3. Create your home (10 ZRN)"
echo "  4. Register keys and start a session"
if [[ "$1" == "--validator" ]]; then
echo "  5. Register as a validator (both systems)"
fi
echo ""
echo "Total estimated cost: ~12 ZRN + gas fees"
```

---

## R24-4 / R24-5 Infrastructure Notes

### R24-4: Docker & Cross-Compilation (Completed)

All build infrastructure was implemented:

- **Makefile:** `build-linux-amd64`, `build-linux-arm64`, `build-darwin-arm64`, `build-all`, `release` targets added with `-trimpath` and stripped LDFLAGS
- **Dockerfile:** Multi-stage build (golang:1.24-bookworm → debian:bookworm-slim) with ca-certificates, curl, jq
- **Dockerfile.validator:** Same base + Cosmovisor binary, environment variables for auto-upgrade
- **docker-compose.yml:** Persistent volume, port mapping (P2P, RPC, REST, gRPC), restart policy
- **VALIDATOR-GUIDE.md:** Updated with Docker and pre-built binary installation options

### R24-5: Cloud Deployment (Not Executed)

R24-5 was a dry-run planning session — no real VPS was provisioned. The prompt documents the intended workflow:
- Hetzner CX22 (~€4/mo) or Docker simulation
- Systemd service, Cosmovisor, firewall (ufw), monitoring (cron health check)
- The join-testnet.sh bugs (from R24-3) would block the automated path
- Manual deployment would require workarounds for all documented bugs

**Production cost estimate (from PRODUCTION-STACK.md):**
- Testnet: $0–20/mo (single VPS)
- Public testnet: ~$100/mo (validator + sentry + RPC)
- Mainnet: ~$400–450/mo (full stack with redundancy)

---

## Final Assessment

### What Works Well

1. **Core transaction flow** — bank transfers, DID registration, home creation, validator registration all function correctly
2. **Build infrastructure** — cross-compilation, Docker, docker-compose are complete and working
3. **Deterministic state** — external node synced 476 blocks with zero app hash mismatches
4. **5-validator consensus** — external node promotion succeeded, all 5 validators signing
5. **Home module** — key registration, sessions, permissions all work as designed
6. **Tier system design** — automatic progression based on stake + verifications is elegant

### What Needs Fixing Before Testnet

1. **Proto codec** — re-generate with gogoproto. Blocks session keys, recovery, and all nested message types.
2. **Key rotation** — don't sync Ed25519 to Cosmos BaseAccount. Currently bricks accounts.
3. **join-testnet.sh** — 3 bugs make it unusable for external operators.
4. **VALIDATOR-GUIDE.md** — 9 discrepancies including a completely missing registration step.
5. **Faucet + seed list** — without these, external operators literally cannot join.
6. **KQUERY encoding** — 1-line fix to enable BVM knowledge access.
7. **Capability gates** — add permission checks on BVM agent opcodes.

### Agent Experience Rating

| Dimension | Score | Notes |
|-----------|-------|-------|
| Can join the network | 7/10 | Works with workarounds; scripts broken |
| Can establish identity | 4/10 | DID works, but sessions/recovery blocked by proto bug |
| Can operate a validator | 5/10 | Works but deactivation is irreversible, no commission update |
| Documentation accuracy | 4/10 | 9 discrepancies in validator guide, critical steps missing |
| Key management UX | 3/10 | 5 separate key systems, no unification |
| Infrastructure tooling | 6/10 | Docker/cross-compile good, scripts broken |
| Overall readiness | 4/10 | Functional core, but too many sharp edges for external operators |

**Verdict:** ZERONE's core chain mechanics are solid, but the operator-facing layer (docs, scripts, key management, error recovery) needs 1–2 weeks of focused work before external agents can reliably onboard.
