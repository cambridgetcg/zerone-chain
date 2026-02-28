# R23-1 — Deploy & Call: BVM End-to-End on Localnet

## Context

252 unit tests pass. The interpreter executes bytecode, the keeper deploys contracts, calls return results. But no one has done it on a live chain via CLI. This session does that.

## Prerequisites

- Localnet running (`scripts/localnet.sh start`)
- BVM module params configured (check defaults: `zeroned query bvm params`)

## Task

### 1. Inspect BVM Params

```bash
$BINARY query bvm params $Q_FLAGS
```

**Document:**
- [ ] `max_bytecode_size` — what's the limit?
- [ ] `max_gas_per_call` — is it reasonable?
- [ ] `deploy_cost` — fee to deploy
- [ ] `current_bvm_version` — version gate
- [ ] `max_schedules_per_contract` — scheduling limits
- [ ] Any params missing or defaulting to zero?

### 2. Deploy a Minimal Contract

Write the simplest possible contract: push a value, store it, return it.

**Bytecode (hand-assembled):**
```
PUSH1 0x42    # push value 66
PUSH1 0x00    # storage slot 0
SSTORE        # store 66 at slot 0

PUSH1 0x00    # storage slot 0
SLOAD         # load from slot 0
PUSH1 0x00    # memory offset 0
MSTORE        # store in memory

PUSH1 0x20    # return 32 bytes
PUSH1 0x00    # from offset 0
RETURN
```

Hex: `60420060005560005460005260206000f3`

```bash
$BINARY tx bvm deploy-contract \
    --bytecode "60420060005560005460005260206000f3" \
    --from val0 $TX_FLAGS
```

**Verify:**
- [ ] Tx succeeds, returns contract address
- [ ] Query contract: `$BINARY query bvm contract <ADDRESS> $Q_FLAGS`
- [ ] CodeHash, Creator, BytecodeSize, BvmVersion all correct
- [ ] Deploy fee deducted from deployer

**Issues to look for:**
- Is bytecode passed as hex string? Base64? Check CLI --help
- Does the contract address format look right? (`zrn1contract...`)
- Is there a way to query deployed bytecode back?

### 3. Call the Contract

```bash
$BINARY tx bvm call-contract \
    --contract-address <ADDRESS> \
    --input-data "" \
    --gas-limit 100000 \
    --from val0 $TX_FLAGS
```

**Verify:**
- [ ] Call succeeds
- [ ] Return data in tx events/response = 0x42 (66 in hex, padded to 32 bytes)
- [ ] Gas used is reasonable
- [ ] Storage slot 0 has value 0x42

**Issues to look for:**
- How is return data exposed? In events? In the tx response proto?
- Can you query contract storage directly? `$BINARY query bvm contract-state <ADDRESS>`
- Gas metering: does BVM gas translate correctly to SDK gas?

### 4. Arithmetic Contract

Deploy a contract that takes two calldata inputs and returns their sum:

```
# Pseudocode: return calldata[0:32] + calldata[32:64]
PUSH1 0x00    # offset 0
CALLDATALOAD  # load first 32 bytes
PUSH1 0x20    # offset 32
CALLDATALOAD  # load second 32 bytes
ADD           # sum them
PUSH1 0x00    # memory offset
MSTORE        # store result
PUSH1 0x20    # 32 bytes
PUSH1 0x00    # from offset 0
RETURN
```

Hex: `600035602035016000526020600f3`

Call with input data encoding two uint256 values.

**Verify:**
- [ ] Return data = sum of inputs
- [ ] Overflow: what happens when sum exceeds 2^256? (should wrap per EVM semantics)
- [ ] Zero inputs: returns zero

### 5. Event/Log Contract

Deploy a contract that emits a LOG1 event:

```
# Push topic
PUSH32 <topic_hash>
# Push data
PUSH1 0x20    # data size
PUSH1 0x00    # data offset
LOG1
STOP
```

**Verify:**
- [ ] Log emitted and visible in tx events
- [ ] Topic and data correct
- [ ] Log events bridge to Cosmos SDK events (`zerone.bvm.*`)

### 6. Payable Contract (Value Transfer)

```bash
$BINARY tx bvm call-contract \
    --contract-address <ADDRESS> \
    --value 1000000 \
    --gas-limit 100000 \
    --from val0 $TX_FLAGS
```

**Verify:**
- [ ] Value transferred from caller to BVM module account
- [ ] Contract can read CALLVALUE
- [ ] On success: value stays in module
- [ ] On revert: value refunded to caller

### 7. Static Call (Read-Only)

```bash
$BINARY tx bvm call-contract \
    --contract-address <ADDRESS> \
    --static-call \
    --gas-limit 100000 \
    --from val0 $TX_FLAGS
```

**Verify:**
- [ ] SSTORE in static call → revert
- [ ] LOG in static call → revert
- [ ] Pure reads (SLOAD, CALLDATALOAD) work fine

### 8. Revert & Error Handling

Deploy a contract that always reverts:

```
PUSH1 0x00
PUSH1 0x00
REVERT
```

**Verify:**
- [ ] Call fails gracefully (no panic)
- [ ] Gas consumed up to revert point
- [ ] No state changes persisted
- [ ] Error message in response

### 9. Gas Exhaustion

Call a contract with deliberately low gas:

```bash
$BINARY tx bvm call-contract \
    --contract-address <ADDRESS> \
    --gas-limit 10 \
    --from val0 $TX_FLAGS
```

**Verify:**
- [ ] Out-of-gas error returned
- [ ] No state changes
- [ ] Gas meter correctly bridged to SDK (tx gas consumed matches BVM gas)

### 10. Contract Storage Persistence

```bash
# Call contract that writes to storage
# Query storage
$BINARY query bvm contract-state <ADDRESS> $Q_FLAGS

# Call again — storage should retain previous values
# Verify: SLOAD returns what previous call SSTORED
```

**Verify:**
- [ ] Storage persists across calls
- [ ] Storage persists across blocks
- [ ] Multiple storage slots work independently

## Bytecode Assembly Note

Hand-assembling BVM bytecode is tedious but necessary for this session. If a simple assembler exists or can be trivially written, use it. Otherwise, use this pattern:

```bash
# Helper: hex encode bytecode
echo -n "PUSH1 0x42 PUSH1 0x00 SSTORE" | python3 -c "
import sys
# Map mnemonics to opcodes...
"
```

If the bytecode format is wrong (base64 vs hex, with/without 0x prefix), figure out the expected format from the CLI help and proto definition first.

## Report Template

```markdown
### Test N: <name>
**Status:** PASS / FAIL / BLOCKED
**Contract Address:** <addr>
**Tx Hash:** <hash>
**Gas Used:** <gas>
**Return Data:** <hex>
**Observation:** <what happened>
**Issue:** <if any>
```

## Exit Criteria

1. At least one contract deployed and called on live localnet
2. Storage persistence verified across calls
3. Gas metering verified (BVM ↔ SDK bridge)
4. Revert and error handling tested
5. CLI interface documented (what flags work, what doesn't)
6. Report written to `docs/bvm-deploy-call-report.md`

## Commit Convention

```
test(bvm): e2e deploy and call testing on localnet
docs(bvm): deploy/call test report
fix(bvm): <any fixes found during testing>
```
