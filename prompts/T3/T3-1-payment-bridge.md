# T3-1 — Payment Bridge

## Goal

Build a Go service that bridges off-chain API usage metering with on-chain ZRN payments. The bridge handles the "pay as you go" experience without requiring an on-chain transaction per API call.

## Design

### Prepaid Deposit Model

1. **Deposit**: User sends ZRN to a deposit address (per-user escrow module account on-chain via x/billing)
2. **Balance tracking**: Bridge monitors on-chain deposits, maintains off-chain balance ledger (Redis)
3. **Deduction**: Each API call deducts from off-chain balance (fast, no chain latency)
4. **Settlement**: Periodically (every N minutes or M calls), bridge submits batch settlement tx on-chain
5. **Revenue distribution**: On-chain settlement triggers revenue split to contributors, storage nodes, protocol

### Why Off-Chain Ledger?

- On-chain tx per API call = ~2.5s latency + gas fees → unusable for real-time inference
- Off-chain ledger = millisecond deductions, batch settlement preserves on-chain auditability
- Trade-off: brief window where off-chain ledger is ahead of chain state

## Deliverables

### 1. Deposit Monitor

- Subscribe to x/billing deposit events on-chain
- When user deposits ZRN to their escrow account, credit their off-chain balance
- Handle deposit confirmations (wait for finality)

### 2. Off-Chain Balance Ledger

- Redis-backed balance tracking per API key / wallet address
- Atomic deduction on each API call (Lua script for atomicity)
- Balance floor: reject requests when balance < estimated cost of request
- Pre-flight cost estimation: estimate tokens based on input length + max_tokens param

### 3. Batch Settlement

- Accumulate usage records: {user, tokens_consumed, model, timestamp}
- Every N minutes (configurable, default 5min): submit MsgSettleUsage tx on-chain
- Settlement tx contains: user address, total tokens consumed, total ZRN cost, period
- On-chain module verifies and transfers from escrow to revenue pool
- Handle settlement failures (retry with exponential backoff)

### 4. Revenue Distribution Trigger

- After settlement, on-chain module distributes revenue per the split defined in T1-3
- Bridge doesn't handle distribution directly — it just triggers settlement
- Revenue flows: escrow → protocol treasury → contributor rewards, storage rewards, etc.

### 5. Withdrawal

- User requests withdrawal of unused deposit
- Bridge checks off-chain balance, submits MsgWithdraw on-chain
- Cool-down period to prevent withdraw-after-use attacks (pending settlements must clear first)

### 6. Dispute Handling

- If bridge goes down, unsettled usage is lost revenue (bridge's risk, not user's)
- If user disputes a charge, bridge provides usage logs + on-chain settlement receipt
- Design for auditability: every deduction logged with request ID, timestamp, token count

## Working Directory

`/Users/yournameisai/Desktop/zerone/services/payment-bridge/`

## Output

- Go module with gRPC interface (gateway calls bridge for balance check / deduction)
- Redis schema documentation
- Integration tests with mock chain client
- Dockerfile
