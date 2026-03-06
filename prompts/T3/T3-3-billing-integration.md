# T3-3 — Billing Module Integration

## Goal

Wire the API gateway and payment bridge to the on-chain x/billing module. Ensure deposits, settlements, withdrawals, and revenue distribution work end-to-end from API call to on-chain finality.

## Deliverables

### 1. x/billing Module Adaptation

Review and extend the existing x/billing module to support:
- **Escrow deposits**: MsgDeposit → creates per-user escrow account
- **Batch settlement**: MsgSettleUsage → deducts from escrow, distributes revenue
- **Withdrawal**: MsgWithdraw → returns unused escrow balance (after cool-down)
- **Usage query**: QueryUsage → on-chain usage records for auditability

### 2. Revenue Distribution Wiring

After settlement, revenue flows to:
- x/knowledge contributor rewards (weighted by quality tier of samples in training set)
- Protocol treasury (via x/vesting_rewards)
- Storage node rewards (via compute_pool or custom)
- Reviewer rewards (via x/knowledge)

Implement the distribution logic in x/billing's EndBlocker or as a hook triggered by settlement.

### 3. Integration Tests

End-to-end test:
1. User deposits 1000 uzrn on-chain
2. Payment bridge detects deposit, credits off-chain balance
3. User makes 10 API calls (consuming ~500 uzrn)
4. Payment bridge settles batch on-chain
5. On-chain escrow shows ~500 uzrn remaining
6. Revenue distributed to contributors, protocol, etc.
7. User withdraws remaining ~500 uzrn

### 4. Edge Cases

- Settlement during chain downtime (queue and retry)
- Double-settlement prevention (idempotency keys)
- User deposits during pending settlement (don't double-count)
- Negative balance prevention (race between concurrent API calls)
- Chain fork handling (revert off-chain state to match)

## Working Directory

On-chain changes: `/Users/yournameisai/Desktop/zerone/x/billing/`
Off-chain: `/Users/yournameisai/Desktop/zerone/services/payment-bridge/`

## Output

- Updated x/billing module with new Msg types if needed
- Integration test suite covering the full deposit → use → settle → withdraw cycle
- Documentation of the settlement protocol
