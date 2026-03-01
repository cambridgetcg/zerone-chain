# R23-2 — Knowledge Bridge: KQUERY/KVERIFY/KCITE from BVM

## Context

The BVM has three Zerone-specific opcodes that bridge into the knowledge module:

| Opcode | Byte | Stack In | Stack Out | Implementation |
|--------|------|----------|-----------|----------------|
| KQUERY | 0xE0 | 1 (factId) | 2 (exists, confidence) | **Working** — calls `KnowledgeKeeper.GetFactConfidence` |
| KVERIFY | 0xE1 | 2 (claimId, voteHash) | 1 (ok) | **Stub** — always returns false |
| KCITE | 0xE2 | 1 (factId) | 1 (ok) | **Stub** — always returns true |

This session tests KQUERY against real knowledge state and evaluates what KVERIFY and KCITE need to become real.

## Prerequisites

- Localnet running with knowledge facts loaded (axiom seeds or submitted claims)
- R23-1 complete (know how to deploy and call contracts)

## Task

### 1. Verify Knowledge State Has Facts

```bash
# List facts available on localnet
$BINARY query knowledge facts --limit 5 $Q_FLAGS

# Get a specific fact ID to use in tests
FACT_ID=$($BINARY query knowledge facts --limit 1 $Q_FLAGS | jq -r '.facts[0].id')
echo "Test fact ID: $FACT_ID"
echo "Confidence: $($BINARY query knowledge fact $FACT_ID $Q_FLAGS | jq '.fact.confidence')"
```

If no facts exist, submit one first:
```bash
$BINARY tx knowledge submit-claim \
    --domain "bvm-test" \
    --claim-type assertion \
    --subject "hydrogen" \
    --predicate "is the lightest element" \
    --from val0 $TX_FLAGS
# Wait for PoT round to complete
```

### 2. Deploy KQUERY Contract

A contract that takes a fact ID in calldata, executes KQUERY, and returns the confidence.

**Bytecode logic:**
```
# Load fact ID from calldata (first 32 bytes)
PUSH1 0x00
CALLDATALOAD     # stack: [factId]

# Execute KQUERY (0xE0)
# Input: factId (32 bytes from stack)
# Output: exists (0 or 1), confidence (uint64)
KQUERY           # stack: [exists, confidence]

# Store confidence in memory
PUSH1 0x00
MSTORE           # memory[0:32] = confidence

# Store exists in memory
PUSH1 0x20
MSTORE           # memory[32:64] = exists

# Return 64 bytes (confidence + exists)
PUSH1 0x40
PUSH1 0x00
RETURN
```

Assemble this to hex and deploy.

**Important:** The fact ID encoding matters. KQUERY takes `[]byte` from the stack. The interpreter code (`interpreter.go:772-787`) does:
```go
case KQUERY:
    factIdWord := interp.stack.Pop()
    factIdBytes := WordToBytes32(factIdWord)
    if host != nil {
        exists, confidence, _ := host.KQuery(factIdBytes[:])
```

This means the fact ID on the stack is treated as raw bytes (32 bytes from a Word). The `knowledgeBridgeHost.KQuery` then hex-encodes those bytes to get the fact ID string. So the calldata must contain the fact ID as a 32-byte hex-decoded value.

**Verify call with known fact:**
```bash
# Encode fact ID to 32-byte hex
FACT_ID_HEX=$(echo -n "$FACT_ID" | xxd -p | tr -d '\n')
# Pad to 64 hex chars (32 bytes)
FACT_ID_PADDED=$(printf "%-64s" "$FACT_ID_HEX" | tr ' ' '0')

$BINARY tx bvm call-contract \
    --contract-address <KQUERY_CONTRACT> \
    --input-data "$FACT_ID_PADDED" \
    --gas-limit 200000 \
    --from val0 $TX_FLAGS
```

**Verify:**
- [ ] `exists` = 1 (fact found)
- [ ] `confidence` > 0 (matches what `query knowledge fact` returns)
- [ ] Gas cost for KQUERY is reasonable (`GasKQuery` value)

**Verify with non-existent fact:**
```bash
$BINARY tx bvm call-contract \
    --contract-address <KQUERY_CONTRACT> \
    --input-data "0000000000000000000000000000000000000000000000000000000000000000" \
    --gas-limit 200000 \
    --from val0 $TX_FLAGS
```

**Verify:**
- [ ] `exists` = 0
- [ ] `confidence` = 0

### 3. Test KQUERY Fact ID Encoding

The bridge does `hex.EncodeToString(factIdBytes)` which means if your fact ID is "abc123", the stack must contain the raw bytes `0xabc123` not the ASCII bytes of "abc123". This encoding gap is likely a bug or at least confusing. Document the exact encoding expected.

**Test:**
- [ ] What format are fact IDs in the knowledge module? (hex string? UUID? hash?)
- [ ] How must they be encoded on the BVM stack?
- [ ] Is there a mismatch between how facts are identified in queries vs how KQUERY decodes them?

### 4. KVERIFY Stub Analysis

```go
func (h *knowledgeBridgeHost) KVerify(_ string, _ []byte, _ []byte) bool {
    return false // Stub: verification voting requires full round integration
}
```

**Document what's needed:**
- [ ] What does KVERIFY semantically mean? (agent casts a verification vote from BVM)
- [ ] Why is it stubbed? (PoT rounds are complex — commit/reveal can't happen in a single BVM call)
- [ ] What would a real implementation look like?
  - Option A: KVERIFY immediately adds a vote (breaks commit/reveal)
  - Option B: KVERIFY queues a vote intent, resolved by the PoT round
  - Option C: KVERIFY is removed — voting should happen via `MsgSubmitVote`, not from BVM
- [ ] Recommendation: which option is architecturally correct?

### 5. KCITE Stub Analysis

```go
func (h *knowledgeBridgeHost) KCite(_ string, _ []byte) bool {
    return true // Citation recording is fire-and-forget
}
```

**Document what's needed:**
- [ ] KCITE should record that a contract/agent cited a fact (boosts the fact's `IncomingCitationCount`)
- [ ] Current implementation is a no-op that returns true
- [ ] What state change should KCITE make? Call `knowledge.AddCitation(callerDID, factId)`?
- [ ] Should KCITE require CallerDID? (currently `callerDID` param is passed but ignored)
- [ ] What prevents citation spam? (gas cost only? or also require proof-of-query?)

### 6. Deploy a Conditional Contract

A contract that queries a fact's confidence and branches on it:

```
# If fact confidence >= 80, return 1 (trust). Otherwise return 0 (distrust).
PUSH1 0x00
CALLDATALOAD     # factId
KQUERY           # exists, confidence

# Check: confidence >= 80
PUSH1 0x50       # 80 decimal
LT               # confidence < 80?
ISZERO           # NOT(confidence < 80) = confidence >= 80

# Branch
PUSH1 <trust_label>
JUMPI

# Distrust path: return 0
PUSH1 0x00
PUSH1 0x00
MSTORE
PUSH1 0x20
PUSH1 0x00
RETURN

# Trust path: return 1
JUMPDEST
PUSH1 0x01
PUSH1 0x00
MSTORE
PUSH1 0x20
PUSH1 0x00
RETURN
```

**Verify:**
- [ ] Contract returns 1 for high-confidence facts
- [ ] Contract returns 0 for low-confidence or non-existent facts
- [ ] This demonstrates an agent making an on-chain decision based on knowledge state

### 7. Gas Analysis

Document gas costs for knowledge bridge opcodes:

```bash
# From gas.go or opcodes.go
grep "GasKQuery\|GasKVerify\|GasKCite" ~/Desktop/zerone/x/bvm/vm/gas.go
```

**Verify:**
- [ ] KQUERY gas cost — is it proportional to the knowledge store read cost?
- [ ] Are the knowledge opcodes more expensive than arithmetic (as they should be)?
- [ ] Does heavy KQUERY usage in a contract hit the gas limit quickly?

## Report Template

```markdown
### Test N: <name>
**Status:** PASS / FAIL / STUB / DOCUMENTED
**Observation:** <what happened>
**Encoding Notes:** <fact ID encoding, calldata format>
**Issue:** <if any>
**Recommendation:** <for stubs — what's needed to make it real>
```

## Exit Criteria

1. KQUERY tested against real knowledge state (existing fact + non-existent)
2. Fact ID encoding documented (stack format → hex → knowledge query)
3. KVERIFY stub analysed — recommendation for real implementation or removal
4. KCITE stub analysed — recommendation for real implementation
5. Conditional contract demonstrates knowledge-gated BVM logic
6. Gas costs documented
7. Report written to `docs/bvm-knowledge-bridge-report.md`

## Commit Convention

```
test(bvm): knowledge bridge e2e — KQUERY against live facts
docs(bvm): knowledge bridge report — KQUERY working, KVERIFY/KCITE stubs analysed
fix(bvm): <any encoding or bridge fixes>
```
