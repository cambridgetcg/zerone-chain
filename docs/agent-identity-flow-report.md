# Agent Identity Flow Report — R24-1

**Date:** 2026-02-26
**Localnet:** zerone-localnet (4 validators, CometBFT v0.38.19)
**Binary:** zeroned built from source (Cosmos SDK v0.50.15)

---

## Summary

Tested the full agent identity bootstrapping sequence: fresh account → DID registration → session keys → key rotation → home creation → home key registration → recovery → freeze/unfreeze.

**Overall verdict:** 5 of 9 steps PASS, 2 steps BLOCKED by a critical proto codec bug, 1 step has a critical account-bricking bug, 1 step reveals a permanent lockout design issue.

---

## Critical Bugs Found

### BUG-1: Proto Codec Mismatch — Nested Messages Fail (CRITICAL)

**Affects:** `MsgCreateSession`, `MsgSetRecoveryConfig`
**Error:** `failed to retrieve the message of type "zerone.auth.v1.SessionCapabilities": tx parse error`
**Root Cause:** Proto files were generated with `protoc-gen-go` (Google protobuf v2) instead of `protoc-gen-gogo` (gogoproto). Cosmos SDK v0.50 uses gogoproto for transaction encoding/decoding. Messages with only scalar fields (strings, bytes, uint64) work because they're wire-compatible, but nested message fields trigger the type resolver which can't find the type in the gogoproto registry.

**Impact:**
- Session key creation completely broken — blocks delegated agent access
- Recovery config setup completely broken — blocks account recovery
- Any message type with nested proto messages is affected

**Fix:** Re-generate proto files using `protoc-gen-gogo` (standard for Cosmos SDK v0.50), or add proper gogoproto compatibility registration. This requires regenerating `tx.pb.go`, `types.pb.go`, and `query.pb.go` for the `zerone_auth` module.

**Messages that work (scalar fields only):**
- `MsgRegisterAccount` — strings only
- `MsgRotateKey` — strings + bytes
- `MsgRevokeSession` — strings only
- `MsgFreezeAccount` — strings only
- `MsgUnfreezeAccount` — strings only
- `MsgInitiateRecovery` — strings only
- `MsgSubmitRecoveryShard` — strings + uint32 + bytes
- `MsgChallengeRecovery` — strings only
- `MsgExecuteRecovery` — strings only

**Messages that fail (nested messages):**
- `MsgCreateSession` — contains `SessionCapabilities`
- `MsgSetRecoveryConfig` — contains `RecoveryConfig` → `ShardHolder`

### BUG-2: Key Rotation Bricks Account (CRITICAL)

**Affects:** `MsgRotateKey` (msg_server.go:136-192)
**Symptom:** After successful key rotation, the account can no longer sign ANY transactions.
**Root Cause:** The `RotateKey` handler syncs the new Ed25519 operational key to the Cosmos `BaseAccount` (line 167-174). However, Cosmos SDK v0.50's `sigverify.go:433` does **not** support Ed25519 for standard account signature verification — only secp256k1. The mismatch permanently locks the account.
**Error:** `ED25519 public keys are unsupported: invalid pubkey`

**Impact:** Any agent who rotates their key is permanently locked out. No transactions can be sent, including recovery or unfreeze operations.

**Fix:** Either:
1. Don't sync Ed25519 keys to Cosmos BaseAccount (keep them only in Zerone account state)
2. Add Ed25519 support to the custom AnteHandler chain
3. Use secp256k1 for operational keys instead of Ed25519

### BUG-3: Authorization Signature Not Validated

**Affects:** `MsgRotateKey`
**Symptom:** The `authorization_signature` field in `MsgRotateKey` is accepted but never validated in the handler. A dummy random signature succeeds.
**Impact:** The field provides no additional security. Key rotation relies solely on the standard tx signature (being the account sender). This may be intentional (operational security via tx sig), but the field's existence implies an additional authorization step that doesn't exist.

---

## Step-by-Step Results

### Step 1: Create Fresh Account
**Status:** PASS
**Tx Hash:** `833D7BD4...` (bank send)
**Cost:** 94,668 uzrn gas
**Notes:**
- Key generation via `zeroned keys add` — secp256k1 (standard Cosmos)
- Funded 100 ZRN (100,000,000 uzrn) from val0
- Minimum gas price: 1 uzrn/gas (not 0.025 as commonly assumed)

### Step 2: Register Account with DID
**Status:** PASS
**Tx Hash:** `F973291E...`
**Cost:** 103,938 uzrn gas
**Notes:**
- CLI: `zeroned tx zerone_auth register-account [did] [public-key] [account-type]`
- Module name is `zerone_auth`, NOT `auth` (confusing — `auth` is the standard Cosmos SDK module)
- DID format: `did:zrn:{32 or 64 hex chars}` — must derive from the identity public key
- Public key: 64 hex chars (32-byte Ed25519) — separate from the secp256k1 signing key
- Valid account types: `agent`, `human`, `contract`, `system`
- Metadata: JSON string, max 1024 bytes
- `operational_key_hash`: optional, not required
- `public_key`: manually provided, NOT auto-derived from signing key
- Bootstrap fund: disabled on localnet (`bootstrap_enabled: false`)
- Reputation score: starts at 500,000 (0.5 on 0-1 scale)

**Validation tests:**
- [x] Duplicate account → "account already exists"
- [x] Duplicate DID from different account → "DID already registered"
- [x] Invalid DID format (`did:zerone:...`) → "DID must start with 'did:zrn:'"
- [x] Invalid account type (`robot`) → "account_type must be agent, human, contract, or system"
- [x] DID derivation enforced: DID suffix must match public key prefix

### Step 3: DID Resolution
**Status:** PASS
**Notes:**
- DID → Address: `zeroned query zerone_auth account-by-did [did]` — works, returns full account
- Address → DID: `zeroned query zerone_auth account [address]` — works, DID in account fields
- Non-existent DID → clear error: "account not found"
- No separate `account-did` query — the `account` query includes the DID field
- Bidirectional resolution confirmed working

### Step 4: Create Session Key
**Status:** BLOCKED (BUG-1)
**Error:** `failed to retrieve the message of type "zerone.auth.v1.SessionCapabilities": tx parse error`
**Notes:**
- CLI: `zeroned tx zerone_auth create-session [session-pub-key-hex] [expires-at-height]`
- Flags: `--can-transfer`, `--can-stake`, `--can-submit-claims`, `--can-vote`
- This is the R23-3 proto parse error **confirmed and reproduced**
- Session keys cannot be created via CLI due to the proto codec mismatch

**Auth module session params:**
- `max_session_keys`: 5
- `max_session_duration`: 34,272 blocks

### Step 5: Key Rotation
**Status:** PASS then BRICKS ACCOUNT (BUG-2)
**Tx Hash:** `D9851BBD...`
**Cost:** 99,325 uzrn gas
**Notes:**
- CLI: `zeroned tx zerone_auth rotate-key [new-op-key-hex] [auth-sig-hex]`
- First rotation succeeds — operational key updated, version incremented
- After rotation, account is PERMANENTLY LOCKED (Ed25519 pubkey on Cosmos BaseAccount)
- Authorization signature field is a no-op (BUG-3)
- Cooldown: 111 blocks between rotations (never tested — account bricked first)
- DID mapping preserved after rotation (same DID, different operational key)

**Account state after rotation:**
- `operational_key_version`: 1 → 2
- `operational_public_key`: changed to new key
- `public_key` (identity): unchanged
- Account unable to sign further transactions

### Step 6: Create Home
**Status:** PASS
**Tx Hash:** `325B95EF...`
**Cost:** ~10,300,000 uzrn (300,000 gas + 10,000,000 creation fee)
**Notes:**
- CLI: `zeroned tx home create-home [name]`
- Home creation fee: 10,000,000 uzrn (10 ZRN) — deducted from owner
- Home ID: auto-increment (`home-1`, `home-2`, etc.)
- Default guardian defense strategy: `moderate`
- Default comfort score: 50
- DID registration NOT required to create a home (agent-gamma without DID succeeded)
- Gas estimate (93,673) was below minimum (150,000) — had to override with `--gas 300000`

**Design question:** Should home creation require DID registration? Currently no enforcement.

### Step 7: Register Keys on Home
**Status:** PASS
**Tx Hash:** `3999A0B7...`
**Notes:**
- CLI: `zeroned tx home register-key [home-id] [key-hash] [key-type] [role] [permissions]`
- Key registered with role `session` and permissions `submit_claim,memory_write`
- Session started successfully with `zeroned tx home start-session`
- Session ID format: `ses-{key_hash_prefix}-{block_height}`
- Session expiry: `session_timeout_blocks` (1000) blocks from start
- Permission intersection works: requested `submit_claim` ∩ registered `submit_claim,memory_write` = `submit_claim`

**Auth vs Home key systems:**
- **x/auth session keys**: Ed25519 pubkey-based, with capability booleans (can_transfer, can_stake, etc.) — CURRENTLY BROKEN (BUG-1)
- **x/home key registration**: Hash-based, with string permissions — WORKING
- These are **separate systems** with no integration between them
- An agent needs to manage two different key registries independently
- This is redundant and confusing — should be unified or at least documented clearly

### Step 8: Account Recovery Setup
**Status:** BLOCKED (BUG-1)
**Error:** `failed to retrieve the message of type "zerone.auth.v1.RecoveryConfig": tx parse error`
**Notes:**
- CLI: `zeroned tx zerone_auth set-recovery-config [threshold] [total-shards] [holder-addresses...]`
- Same proto codec mismatch as session keys — nested `RecoveryConfig` message fails
- Recovery flow cannot be tested end-to-end
- Other recovery messages (initiate, submit shard, challenge, execute) use only scalars and should work

**Recovery params:**
- `recovery_delay_blocks`: 1,000
- `challenge_period_blocks`: 500
- `max_recovery_shards`: 10

### Step 9: Account Freeze/Unfreeze
**Status:** PARTIAL PASS
**Tx Hash (freeze):** `8EB963AF...`
**Notes:**

**Freeze:**
- Self-freeze works: `zeroned tx zerone_auth freeze-account [own-address] --reason "..."`
- Account correctly marked frozen with reason
- All transactions from frozen account rejected: "account is frozen" (code 8, codespace zerone_auth)
- Frozen account can still **receive** funds (sending TO frozen works)

**Unfreeze:**
- Self-unfreeze IMPOSSIBLE: ante handler blocks ALL txs from frozen accounts, including unfreeze
- Validator unfreeze fails: val0 is not the authority
- Authority is governance module address (`authtypes.NewModuleAddress(govtypes.ModuleName)`)
- Only governance proposals can unfreeze — impractical on localnet

**Design issue:** Self-freeze is a one-way operation without governance. An agent who self-freezes for "emergency lockdown" has no way to reverse it without a governance proposal. This may be intentional (security) but is dangerous for agent autonomy.

---

## Timing & Cost Summary

| Step | Tx Count | Gas Used | Fee (uzrn) | Notes |
|------|----------|----------|------------|-------|
| 1. Fund account | 1 | 94,668 | 94,668 | From faucet/val0 |
| 2. Register DID | 1 | 103,938 | 103,938 | |
| 3. DID resolution | 0 | — | — | Query only |
| 4. Create session | BLOCKED | — | — | Proto bug |
| 5. Rotate key | 1 | 99,325 | 99,325 | BRICKS ACCOUNT |
| 6. Create home | 1 | ~150,000 | 10,300,000 | Includes 10M creation fee |
| 7. Register home key | 1 | ~150,000 | 150,000 | |
| 7b. Start session | 1 | ~150,000 | 150,000 | |
| 8. Recovery config | BLOCKED | — | — | Proto bug |
| 9. Freeze | 1 | ~150,000 | 150,000 | |
| **Total (working)** | **7** | **~900K** | **~11M** | **~11 ZRN** |

**Full onboarding cost** (steps 1-7, excluding broken steps): ~11 ZRN total, dominated by the 10 ZRN home creation fee.

---

## Auth Module Query Surface

| Query | Command | Status |
|-------|---------|--------|
| Account by address | `query zerone_auth account [addr]` | PASS |
| Account by DID | `query zerone_auth account-by-did [did]` | PASS |
| Session keys | `query zerone_auth session-keys [owner]` | PASS (empty — can't create sessions) |
| Module params | `query zerone_auth params` | PASS |
| Frozen accounts | `query zerone_auth frozen-accounts` | PASS |

---

## Architecture Observations

### 1. Dual Key System Confusion
The agent must manage two separate key systems:
- **x/auth**: Identity key (Ed25519), operational key (Ed25519), session keys (Ed25519)
- **x/home**: Registered keys (hash-based), home sessions (permission-based)

These are completely independent — an auth session key has no relationship to a home registered key. This creates confusion and redundant operations.

### 2. Signing Key ≠ Identity Key
The Cosmos keyring uses secp256k1 for transaction signing. The Zerone identity system uses Ed25519 for identity/operational keys. These are stored separately and serve different purposes. This is not clearly documented and leads to BUG-2 when they're mixed.

### 3. Module Naming
The module is `zerone_auth` but the standard Cosmos SDK `auth` module is also present. CLI commands for the Zerone auth module are under `zeroned tx zerone_auth`, not `zeroned tx auth`. This is confusing.

### 4. Gas Estimation Unreliable
Gas estimation via `--gas auto` returned values below the minimum gas floor (150,000 for home creation). Manual gas override was needed.

---

## Recommendations

1. **Fix proto codec (P0):** Re-generate proto files with `protoc-gen-gogo` for Cosmos SDK v0.50 compatibility. This unblocks session keys, recovery, and any future nested message types.

2. **Fix key rotation pubkey sync (P0):** Don't sync Ed25519 keys to Cosmos BaseAccount, or add Ed25519 support to the ante handler. Currently, key rotation bricks accounts.

3. **Remove or validate auth signature (P1):** The `authorization_signature` field in `MsgRotateKey` is a no-op. Either validate it properly or remove the field to avoid false security assumptions.

4. **Unify key systems or document clearly (P1):** The x/auth and x/home key systems are separate. Either unify them or add clear documentation explaining the distinction and when to use which.

5. **Self-unfreeze mechanism (P2):** Consider allowing self-unfreeze (perhaps with a time delay) so agents aren't permanently locked out without governance intervention.

6. **Require DID for home creation (P2):** Consider gating home creation on DID registration to enforce the identity-first flow.

7. **Gas floor documentation (P2):** Document the minimum gas requirements per message type. The `create-home` gas floor of 150,000 caused failures with `--gas auto`.
