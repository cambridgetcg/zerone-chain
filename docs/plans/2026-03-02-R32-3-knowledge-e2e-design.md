# R32-3 Knowledge Lifecycle E2E — Design

## Overview

End-to-end tests for the complete knowledge verification lifecycle running on a real Docker-based chain via interchaintest v8. Tests cover: claim-to-fact lifecycle, domain pressure, verification dissent, metabolism decay, and Wu Xing cross-module flows.

## File Structure

| File | Purpose |
|------|---------|
| `tests/e2e/knowledge_helpers_test.go` | Knowledge-specific E2E helpers |
| `tests/e2e/knowledge_test.go` | 5 test functions |
| `tests/e2e/chain_config_test.go` | Add `fitness_epoch_blocks` genesis override |

## Genesis Override

Add to `testGenesisKV()` in `chain_config_test.go`:

```go
cosmos.NewGenesisKV("app_state.knowledge.params.fitness_epoch_blocks", 10),
```

This reduces the default 10,000-block fitness epoch to 10 blocks for observable metabolism in E2E.

## Helpers (`knowledge_helpers_test.go`)

### Core Transaction Helpers

- **`SubmitClaim(t, chain, ctx, keyName, content, domain, category, fee) → claimID`**
  Submits claim via `ExecTx("knowledge", "submit-claim", ...)`, then queries pending claims to locate the new claim ID.

- **`CommitVote(t, chain, ctx, keyName, roundID, vote, salt)`**
  Computes `SHA-256(vote || salt)`, hex-encodes the hash, calls `ExecTx("knowledge", "submit-commitment", roundID, hashHex)`.

- **`RevealVote(t, chain, ctx, keyName, roundID, vote, salt)`**
  Hex-encodes salt, calls `ExecTx("knowledge", "submit-reveal", roundID, vote, saltHex)`.

### Query Helpers

- **`GetClaimRoundID(t, chain, ctx, claimID) → roundID`**
  Queries claim by ID, extracts `verification_round_id` from JSON response.

- **`WaitForRoundPhase(t, chain, ctx, roundID, phase, maxBlocks)`**
  Polls round state every block until phase matches or maxBlocks exceeded.

- **`QueryFact(t, chain, ctx, factID) → map[string]interface{}`**
  Queries fact by ID, returns parsed JSON.

- **`QueryDomainCapacity(t, chain, ctx, domain) → map[string]interface{}`**
  Queries domain-capacity endpoint, returns pressure/active count.

### Composite Helpers

- **`DriveClaimToFact(t, chain, ctx, submitterKey, verifierKeys, content, domain, category, fee) → factID`**
  Full lifecycle: submit → commit (all verifiers) → wait for reveal → reveal (all) → wait for completion → return fact ID. Used by pressure, metabolism, and Wu Xing tests.

## Tests (`knowledge_test.go`)

### Test 1: `TestKnowledge_ClaimToFact` (~25 blocks, ~30s)

- 1 validator chain
- Fund submitter account
- Submit claim ("Water boils at 100°C at standard pressure", domain="physics")
- Query claim → get round ID
- Validator commits "accept" with random salt
- Wait 10 blocks → reveal phase
- Validator reveals vote + salt
- Wait 15 blocks → aggregation completes
- Query fact by claim → assert exists, domain="physics", status=VERIFIED

### Test 2: `TestKnowledge_DomainPressure` (~150 blocks)

- 1 validator chain
- Submit 5 distinct claims to "physics" domain, each driven through full lifecycle
- After each accepted fact: query `domain-capacity` for "physics"
- Assert pressure BPS increases monotonically with each new fact
- Verify active count increments

### Test 3: `TestKnowledge_Dissent` (~25 blocks)

- 3-validator chain
- Submit claim, get round ID
- Validator 0 + 1 commit "accept", validator 2 commits "reject"
- All three reveal
- Wait for aggregation
- Verify claim accepted (2/3 majority)
- Query round → verify reveals contain both accept and reject entries

### Test 4: `TestKnowledge_Metabolism` (~40 blocks)

- 1 validator chain (with `fitness_epoch_blocks=10`)
- Drive claim to fact
- Query fact immediately → record initial energy
- Wait 12 blocks (past one fitness epoch boundary)
- Query fact again → assert energy decreased (maintenance cost > 0, no query income)

### Test 5: `TestKnowledge_WuXing` (~30 blocks)

- 1 validator chain
- Drive claim to fact (creates knowledge growth event)
- Wait 10 blocks (alignment observation interval)
- Query alignment module sensors → verify knowledge-related metrics registered
- Validates R31 Fire→Earth flow (verification health feeds alignment sensing)

## Key Design Decisions

1. **Validators as verifiers**: Only validator keys have staking power for weighted voting. Non-validator accounts have zero effective stake and would fail quorum.
2. **Commit hash format**: Simple `SHA-256(vote || salt)` — confirmed this is what the keeper verifies in `msg_server.go:390-393`.
3. **Separate chains per test**: Full isolation, no state leakage between tests.
4. **No mocks**: Everything runs on real consensus with real ABCI++ vote extensions.
