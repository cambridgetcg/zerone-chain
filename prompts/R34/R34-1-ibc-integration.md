# R34-1 — IBC Integration Tests

## Objective

Test IBC functionality using interchaintest with a real relayer and real counterparty chain. Verify ZERONE can send/receive tokens, open channels, and participate in the interchain.

## Tasks

### 1. Basic IBC transfer

- Spin up ZERONE + Cosmos Hub (or simapp) via interchaintest
- Start Hermes or Go relayer between them
- Create IBC channel (transfer port)
- Send ZRN from ZERONE → counterparty
- Verify IBC denom on counterparty (`ibc/HASH`)
- Send back → verify original denom restored

### 2. IBC rate limiting

- Verify `ibcratelimit` module enforces transfer limits
- Send amount exceeding rate limit → expect rejection
- Wait for rate limit window reset
- Send again → expect success

### 3. IBC timeout and recovery

- Send IBC transfer with very short timeout
- Let it timeout
- Verify refund on source chain
- Verify no double-spend

### 4. Multi-hop IBC

- Spin up 3 chains: ZERONE → Hub → Osmosis (simapp)
- Transfer ZRN through both hops
- Verify correct denom traces
- Transfer back through same path

### 5. ICA (Interchain Accounts)

- Register interchain account from ZERONE on counterparty
- Execute a transaction on counterparty via ICA
- Verify execution result

## Acceptance Criteria

- [ ] IBC transfer works in both directions
- [ ] Rate limiting enforced correctly
- [ ] Timeout refund works without double-spend
- [ ] Multi-hop IBC produces correct denom traces
- [ ] ICA registration and execution work
