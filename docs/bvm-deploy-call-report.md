# BVM Deploy & Call E2E Report — R23-1

**Date:** 2026-02-26
**Chain:** `zerone-localnet` (4 validators)
**Block range:** 491–709
**Deployer:** val0 (`zrn100mxrvv5chhhrj0yd9y4q8354z4edm42mukf5r`)

---

## Summary

| # | Test | Status | Notes |
|---|------|--------|-------|
| 1 | BVM Params | PASS | All defaults correct |
| 2 | Deploy Minimal Contract | PASS | 16 bytes, deploy cost deducted |
| 3 | Call & Return Data | PASS | 0x42 returned, storage written |
| 4 | Arithmetic Contract | PASS | Addition, zero, overflow wrap |
| 5 | Revert Handling | PASS | Graceful failure, error message |
| 6 | Gas Exhaustion | PASS | Out-of-gas at SSTORE |
| 7 | Storage Persistence | PASS | Across calls and ~200 blocks |
| 8 | Static Call | PASS | SSTORE rejected, pure reads OK |
| 9 | LOG1 Event | PASS* | Executes, but NOT bridged to SDK events |
| 10 | Value Transfer (Payable) | PASS | CALLVALUE correct, refund on revert |

**Exit criteria met:** All 6 requirements satisfied.

---

## Test 1: BVM Params

```json
{
  "max_bytecode_size": "65536",
  "max_gas_per_call": "10000000",
  "max_gas_per_block": "100000000",
  "max_contracts_per_creator": "100",
  "max_state_entries": "10000",
  "deploy_cost": "5000000",
  "max_schedule_gas": "1000000",
  "schedule_horizon_blocks": "100000",
  "current_bvm_version": 1,
  "max_schedules_per_contract": "100"
}
```

**Observation:** All params present and reasonable. `deploy_cost` = 5 ZRN, `max_gas_per_call` = 10M.

---

## Test 2: Deploy Minimal Contract

**Bytecode:** `604260005560005460005260206000f3` (PUSH1 0x42, PUSH1 0x00, SSTORE, PUSH1 0x00, SLOAD, PUSH1 0x00, MSTORE, PUSH1 0x20, PUSH1 0x00, RETURN)

**Contract Address:** `zrn1contractc587e87be2fcc3778155e55b76d90a7ee55df67a`
**Tx Hash:** `95BFD30D6007E72960F0164245AE2E94F721957B652F9C9B68B4AE3FA7BC4761`
**Block:** 522
**SDK Gas Used:** 106,022
**Deploy Cost:** 5,000,000 uzrn (deducted from deployer)

**Contract Metadata:**
```json
{
  "code_hash": "3c94113bda277d97c7974d682aa3e6ab19e644c7219d230c81c89d02cb0a982b",
  "creator": "zrn100mxrvv5chhhrj0yd9y4q8354z4edm42mukf5r",
  "deployed_at_block": "522",
  "bytecode_size": "16",
  "bvm_version": 1
}
```

**Balance verification:**
- Before: 99,885,500,000 uzrn
- After: 99,880,200,000 uzrn
- Diff: 5,300,000 = deploy_cost (5,000,000) + gas fee (300,000)

---

## Test 3: Call Minimal Contract

**Tx Hash:** `475D2B1E51EF7BC47B29AC42A1ED2F571C703F8038648BAA872089449CF37053`
**Block:** 534
**BVM Gas Used:** 22,324
**SDK Gas Used:** 84,139

**Return Data:** `0000000000000000000000000000000000000000000000000000000000000042`
- Decimal: 66 (0x42) — correct

**Storage Slot 0:** `0000000000000000000000000000000000000000000000000000000000000042` — matches return data

**Observation:** Return data is embedded in the protobuf-encoded `MsgCallContractResponse` inside the tx `data` field. Requires proto decoding — not directly visible in events.

---

## Test 4: Arithmetic Contract

**Bytecode:** `6000356020350160005260206000f3` (CALLDATALOAD(0) + CALLDATALOAD(32) → MSTORE → RETURN)

**Contract Address:** `zrn1contract26fd10d9dc363ed0ab1aa4d04b9f3b56c7c63be2`
**Block:** 560
**BVM Gas per call:** 30

### 4a: Normal Addition (3 + 5)

**Tx Hash:** `B659778BB025DF0547D0A686A9ED5DA8BBD9E0102454164F1B55414C861AB592`
**Return Data:** 8 — correct

### 4b: Zero Inputs (0 + 0)

**Tx Hash:** `78B4FDC6560DB5A9422BE4FD184E7CCC6B4D3DC1C3E306D2E136E355C69649D2`
**Return Data:** 0 — correct

### 4c: Overflow ((2^256 - 1) + 1)

**Tx Hash:** `34F26E4923BB81C250F92F729552320C110F7511B0753870F905AB83BB565714`
**Return Data:** 0 — correct EVM wrap-around

---

## Test 5: Revert Handling

**Bytecode:** `60006000fd` (PUSH1 0x00, PUSH1 0x00, REVERT)

**Contract Address:** `zrn1contract40e30457fe35176d4edc7231b764b8af785e96eb`
**Tx Hash:** `7E9ABED6A067C73C8A86578D9CAACFE4DB34414673A8A952FBFA6EE11E97826A`
**Tx Code:** 1 (failure)
**Error:** `"execution reverted: gas_used=6"`
**SDK Gas Used:** 54,328 (still consumed)
**BVM Gas Used:** 6 (3 opcodes before revert)

**Observation:** Revert fails the entire tx (code=1). No state changes persisted. Error message clearly indicates BVM gas consumed at revert point. This is correct Cosmos SDK behavior — a failed message fails the whole tx.

---

## Test 6: Gas Exhaustion

**Tx Hash:** `A998E84E3AEDED4F1DFB32964B3BCF1F0214D29E3E12E677B42A2092A100E69A`
**Tx Code:** 1 (failure)
**Error:** `"out of gas at SSTORE: gas_used=10"`
**SDK Gas Used:** 55,467

**Observation:** With gas_limit=10, execution reached SSTORE (cost 20,000) and correctly ran out of gas. The error message identifies the exact opcode where gas was exhausted. No state changes persisted.

---

## Test 7: Storage Persistence

**Contract Address:** `zrn1contract33397f1c7b1790352521e7bf3c1afbe74e5ea35d`
**Bytecode:** `602035600035556000355460005260206000f3` (reads slot + value from calldata, SSTOREs, SLOADs back, returns)

### Write Slot 0 = 0xAA
**Tx Hash:** `BA7192C87E78DFB76877DFCE12D9ACA3C0CF181DEC0DBEA4A777A2CF6F9259D4`

### Write Slot 1 = 0xBB
**Tx Hash:** `F45036D3EF37171B53680A6146FC91189C04F495FE411C0C0FFD98E35CFBDA90`

### Verification
| Slot | Expected | Actual | Match |
|------|----------|--------|-------|
| 0 | 0xAA | 0x00...AA | Yes |
| 1 | 0xBB | 0x00...BB | Yes |
| 2 (untouched) | empty | `{}` | Yes |

### Cross-Block Persistence
- Contract 1 (deployed block 522) storage slot 0 still holds 0x42 at block 709 (~187 blocks later)
- Multiple storage slots work independently

---

## Test 8: Static Call

### 8a: SSTORE in Static Call (should reject)

**Tx Hash:** `35C4764332480FE62F65ABF4DC3BE2F14E0904BD8C06E917E4D075C3206F7159`
**Tx Code:** 1 (failure)
**Error:** `"state modification in static call: SSTORE: gas_used=0"`

### 8b: Pure Computation in Static Call (should succeed)

**Tx Hash:** `46DD86ABF84D7E1B8B952FA88908D22E4BC51D505E51F48AC3032153896CBA88`
**Tx Code:** 0 (success)
**Return Data:** 10 (7+3) — correct

**Observation:** Static call correctly enforces read-only semantics. State modifications are rejected at the opcode level with clear error messages. Pure computations work fine.

---

## Test 9: LOG1 Event Emission

**Contract Address:** `zrn1contract9553023965cdbf1e0188496b30bf319711f433f4`
**Bytecode:** `604260005260ef60206000a100` (MSTORE 0x42, LOG1 with topic 0xEF, STOP)

**Tx Hash:** `4C3C0E40997DC252EA32BD36FF696F8F83C67910797F1B5D345932F5DCE3EE5B`
**Tx Code:** 0 (success)
**BVM Gas Used:** 1,027

**Issue:** BVM logs (LOG0-LOG4) are NOT emitted as Cosmos SDK events. The `zerone.bvm.contract_called` event only contains `contract`, `caller`, `gas_used`, and `success`. The BVM execution logs are collected in the interpreter but discarded in `msg_server.go` — they are not included in `MsgCallContractResponse` either.

**Impact:** No way to observe contract events from external tooling (block explorers, indexers, event subscriptions). This needs to be addressed for any contract ecosystem to function.

---

## Test 10: Value Transfer (Payable)

**Contract Address:** `zrn1contract82ade9967efe46864e72db0fb970533d56080847`
**Bytecode:** `3460005260206000f3` (CALLVALUE, MSTORE, RETURN)

### 10a: Successful Value Transfer

**Tx Hash:** `4A7DE1FA60A45E8C435D844F9A45D4A3B12391B3AA3A40839CCE18F61B3EE26D`
**Value Sent:** 1,000,000 uzrn
**CALLVALUE Return:** 1,000,000 — correct
**Transfer Events:**
- 300,000 uzrn → fee collector
- 1,000,000 uzrn → BVM module account (`zrn1qp7g4mas5wpv6gdgwjrvxl8xysa8jty993seas`)

### 10b: Value Refund on Revert

**Tx Hash:** `498CDB5D4F40382E4917F72BCD233D16C1FC44E59A3F740BD402051FAF70F798`
**Value Sent:** 500,000 uzrn
**Error:** `"execution reverted: gas_used=6"`
**Balance Diff:** 300,000 (gas fee only — value refunded)

**Observation:** Value transfer mechanics are correct. On success, value goes to the BVM module account. On revert, value is refunded to caller. Only gas fees are consumed.

---

## CLI Interface Reference

### Transaction Commands

```bash
# Deploy
zeroned tx bvm deploy [bytecode-hex] [initial-deposit] \
  --constructor-args [hex]   # optional
  --from [key] --gas [gas] --gas-prices [price] --chain-id [id] --node [url] \
  --keyring-backend test --yes --broadcast-mode sync --output json

# Call
zeroned tx bvm call [contract-address] [input-data-hex] \
  --gas-limit [bvm-gas]      # BVM gas limit (0 = use param default)
  --value [amount]           # uzrn to send
  --static                   # read-only mode
  --from [key] --gas [sdk-gas] --gas-prices [price] ...
```

### Query Commands

```bash
# Module params
zeroned query bvm params --node [url] --output json

# Contract metadata
zeroned query bvm contract [address] --node [url] --output json

# Contract storage
zeroned query bvm contract-state [address] [key-hex-64chars] --node [url] --output json

# All contracts by creator
zeroned query bvm contracts-by-creator [creator-address] --node [url] --output json
```

### Key Notes

1. **Bytecode format:** Hex string, no `0x` prefix
2. **Input data format:** Hex string, no `0x` prefix. Use `""` for empty
3. **Storage key format:** 64-character hex (32-byte padded)
4. **Minimum SDK gas:** 200,000 per message (enforced by ante handler)
5. **BVM gas vs SDK gas:** Separate meters. `--gas` controls SDK gas, `--gas-limit` controls BVM gas
6. **Return data:** In protobuf-encoded tx `data` field, requires decoding

---

## Deployed Contracts

| Address | Purpose | Block | Size |
|---------|---------|-------|------|
| `zrn1contractc587e87...` | Store 0x42 + return | 522 | 16B |
| `zrn1contract26fd10d...` | Arithmetic (ADD) | 560 | 15B |
| `zrn1contract40e30457...` | Always revert | 586 | 5B |
| `zrn1contract33397f1c...` | Multi-slot storage | 611 | 19B |
| `zrn1contract95530239...` | LOG1 emit | 661 | 13B |
| `zrn1contract82ade996...` | CALLVALUE return | 683 | 9B |

---

## Issues Found

### Issue 1: BVM Logs Not Bridged to SDK Events (Medium)

**Location:** `x/bvm/keeper/msg_server.go:254-260`
**Description:** The interpreter collects LOG0-LOG4 events during execution, but `msg_server.go` only emits a summary `zerone.bvm.contract_called` event. The BVM logs are completely discarded.
**Impact:** External tooling cannot observe contract events. This is a fundamental requirement for any contract ecosystem (indexers, block explorers, dApp frontends).
**Suggestion:** Emit BVM logs as `zerone.bvm.contract_log` SDK events with topics and data attributes. Also include them in `MsgCallContractResponse.events`.

### Issue 2: Gas Auto-Estimation Below Minimum (Low)

**Description:** `--gas auto --gas-adjustment 1.5` can estimate gas below the 200,000 minimum per message, causing tx rejection. Example: deploy estimated 131,950 but minimum is 200,000.
**Workaround:** Always use explicit `--gas 300000` (or higher) instead of auto.

### Issue 3: Return Data Not in Events (Low)

**Description:** Contract return data is only available in the protobuf-encoded tx `data` field, requiring proto decoding. Not present in human-readable events.
**Suggestion:** Add `return_data` attribute to the `zerone.bvm.contract_called` event for easier debugging and tooling.

---

## Gas Metering Summary

| Operation | BVM Gas | SDK Gas | Notes |
|-----------|---------|---------|-------|
| Deploy (16B) | — | 106,022 | Plus 5M deploy cost |
| Call + SSTORE + SLOAD + RETURN | 22,324 | 84,139 | SSTORE dominates |
| Pure arithmetic (ADD) | 30 | 55,052 | SDK overhead ~55K base |
| LOG1 emit | 1,027 | 55,373 | LOG gas: 375 + 375 topic + 8*32 data |
| CALLVALUE + RETURN | 17 | 72,295 | Higher SDK gas from value transfer |
| Revert (3 opcodes) | 6 | 54,328 | Gas consumed up to revert point |
| Out-of-gas | 10 | 55,467 | Stops at SSTORE |

**Observation:** SDK base overhead is ~54-55K gas per call tx (ante handler, message routing, event emission). BVM gas maps directly to SDK gas via `ConsumeGas()`.
