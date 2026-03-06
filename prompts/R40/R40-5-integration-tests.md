# R40-5 — Full Integration Tests

## Objective

Write end-to-end integration tests that exercise the complete submission → review → resolution → fitness → sharding → reputation cycle, proving all four ToK features work together as a system.

## Context

After R40-1 through R40-4, the four ToK features are wired in. This session validates they compose correctly under both happy-path and adversarial conditions.

## Test Scenarios

### Happy Path

1. **Full lifecycle test**:
   - Agent submits TDU with stake → quality round opens
   - 3 reviewers commit scores (each escrowing reviewer stake)
   - Reviewers reveal → aggregation runs
   - ACCEPT outcome: submitter gets stake + accept bonus, majority reviewers get stake back + show-up
   - FitnessRecord created at 0.5
   - Submitter and majority reviewers gain reputation
   - Next snapshot interval: TDU assigned to R validators via sharding

2. **Fitness evolution over time**:
   - Submit and accept 5 TDUs
   - Simulate 10 epochs: access some TDUs frequently, others never
   - Verify: frequently accessed TDUs → Core (≥0.7), untouched → decay toward Dormant
   - Verify: Core TDUs earn longevity rewards, Dormant earn nothing
   - Verify: Pruned TDUs excluded from shard assignments

3. **Reputation-weighted voting**:
   - Agent A has high reputation (submitted 10 accepted TDUs)
   - Agent B is new (0 reputation)
   - Both review same submission — A says accept, B says reject
   - Verify: A's vote has more weight, submission accepts
   - Verify: B loses reputation for being minority

### Adversarial

4. **Rubber-stamp attack**:
   - 3 reviewers always accept without reading
   - Garbage submission gets through initial round
   - Contest mechanism triggered → re-review
   - Verify: rubber-stampers lose reviewer stakes + reputation
   - Verify: after enough bad reviews, their vote weight is negligible

5. **Lazy-reject attack**:
   - Reviewer always rejects to collect challenge bonus
   - Verify: on good submissions that pass, lazy-rejecter is minority → loses stake + reputation
   - Over time: reputation floor hit (25% of peak), low vote weight

6. **Sybil reviewer ring**:
   - 3 colluding reviewers from same entity
   - All vote the same way on every submission
   - Verify: consistent agreement pattern doesn't break the system
   - Verify: if they're wrong, all lose stakes simultaneously

7. **Deep contested scenario**:
   - 5 reviewers, split 3-2 but no supermajority
   - Verify: deep contested → all stakes returned
   - Same content resubmitted → second round
   - Third strike → permanent reject

8. **Stake exhaustion**:
   - Reviewer commits but doesn't have enough funds for stake
   - Verify: commitment rejected, round continues without them
   - Verify: round still resolves with remaining reviewers

### State Consistency

9. **Genesis round-trip**:
   - Run full lifecycle, accumulate state (fitness records, shard assignments, reputation, stakes)
   - Export genesis → import genesis
   - Verify: all state preserved exactly
   - Continue operations after import → no errors

10. **Epoch boundary**:
    - Verify: fitness decay, reputation decay, and shard reshuffle can all trigger in same BeginBlocker
    - Verify: no ordering dependencies or race conditions between the three

## Test Structure

Use table-driven tests where possible. Each scenario should be self-contained with its own test keeper setup.

## Key Files

- `x/knowledge/keeper/integration_test.go` — NEW file for these tests
- Use existing test helpers from `*_test.go` files

## Exit Criteria

- ≥ 20 integration tests covering all 10 scenarios
- All tests pass
- No panics or unexpected errors under adversarial conditions
- `go test ./x/knowledge/keeper/ -v -count=1` — clean pass
