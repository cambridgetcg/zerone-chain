# R44-2 — Payment Flow End-to-End Tests

## Objective

Write comprehensive end-to-end tests that validate the entire payment flow: from API request → token metering → credit deduction → revenue distribution → stakeholder payouts.

## Test Scenarios

### Happy Path

1. **Full payment lifecycle**:
   - Create wallet, deposit 100 ZRN API credits
   - Create API key bound to wallet
   - Simulate 10 API requests with varying token counts
   - Verify: credits deducted correctly (input_tokens × price + output_tokens × price)
   - Verify: usage records accumulated
   - Trigger epoch revenue distribution
   - Verify: revenue split to training contributors, validators, submitters, protocol, research

2. **Credit management**:
   - Deposit 50 ZRN
   - Use 30 ZRN worth of API calls
   - Withdraw remaining 20 ZRN
   - Verify: wallet balance restored, API balance zero
   - Attempt API call → 402 error

3. **Model attribution revenue flow**:
   - Submit 5 TDUs, get them accepted
   - Train model using those TDUs (mock training record)
   - Serve API requests using that model
   - Trigger revenue distribution
   - Verify: training contributor share flows to the 5 TDU submitters
   - Verify: higher fitness TDUs get proportionally more revenue

### Access Payment (existing on-chain)

4. **Sample access pricing**:
   - Access a Gold-tier sample → quality multiplier applied (3×)
   - Access a Bronze-tier sample → 1× base price
   - Access from dataset (bulk) → bulk discount applied
   - Verify: correct payment amounts in each case

5. **Revenue distribution accuracy**:
   - Accumulate revenue from 20 sample accesses across 5 samples
   - Different consent types (self-authored gets 1.5×, fair-use gets 0.5×)
   - Trigger epoch distribution
   - Verify: submitter shares scaled by consent multiplier
   - Verify: validator share distributed proportionally
   - Verify: protocol share → research fund

6. **Sponsored sample access**:
   - Sample with patronage (pre-paid by sponsor)
   - Access within patronage period → free
   - Access after patronage expires → normal pricing
   - Verify: sponsor's deposit covers the access

### Edge Cases

7. **Insufficient credits**:
   - Deposit 1 ZRN
   - Attempt large API request (would cost 2 ZRN)
   - Verify: request rejected with clear error
   - Verify: no partial deduction

8. **Concurrent access**:
   - 10 simultaneous sample accesses on same sample
   - Verify: all payments recorded correctly
   - Verify: access_count incremented correctly
   - Verify: energy restored for each access

9. **Zero-revenue epoch**:
   - Run epoch distribution with no pending revenue
   - Verify: no errors, no empty transfers

10. **Revoked key usage**:
    - Create key, use it, revoke it
    - Attempt usage with revoked key
    - Verify: rejected at auth layer

11. **Quality tier transitions**:
    - Sample starts as Gold, gets contested and downgraded to Silver
    - Access before and after downgrade
    - Verify: pricing reflects current tier, not original

### Payment Bridge Integration

12. **Batch usage submission**:
    - Accumulate 100 usage records
    - Submit as single MsgRecordAPIUsage batch
    - Verify: all records processed, correct deductions
    - Verify: gas cost reasonable for batch size

13. **Duplicate batch rejection**:
    - Submit same usage batch twice
    - Verify: second submission rejected (idempotency)

14. **Bridge recovery**:
    - Simulate bridge crash mid-batch
    - Restart and resubmit
    - Verify: no double-charging

## Test Structure

Use table-driven tests. Each scenario self-contained with own keeper setup.
Group into files:
- `x/knowledge/keeper/payment_flow_test.go` — on-chain payment tests (scenarios 1-11)
- `services/payment-bridge/bridge_test.go` — bridge integration tests (scenarios 12-14)

## Key Files

- `x/knowledge/keeper/payment_flow_test.go` — NEW
- Use existing test helpers + mock bank keeper

## Exit Criteria

- ≥ 25 tests covering all 14 scenarios
- All tests pass
- `go test ./x/knowledge/keeper/ -v -run TestPayment -count=1` — clean pass
