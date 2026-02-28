# R27-5 — Testnet Faucet + Bootstrap Fund Distribution

## Context

New testnet participants need tokens to register accounts, stake for qualification, submit claims, and participate in the economy. Without a faucet, onboarding is blocked.

ZERONE also has a bootstrap fund mechanism (`x/claiming_pot` — the one place where account_type IS enforced). This session wires both distribution paths.

## Task

### 1. Faucet Design

**Simple approach — CLI-based faucet bot:**

A lightweight service that:
- Listens for requests (HTTP endpoint or Discord/Telegram bot)
- Sends testnet ZRN from a pre-funded faucet account
- Rate-limits per address (1 request per 24h)
- Caps per request (e.g., 100 ZRN = 100,000,000 uzrn)
- Logs all distributions

**Implementation options (pick one):**

**Option A: Shell script + cron** (simplest)
```bash
# faucet.sh — called by HTTP endpoint
ADDRESS=$1
# Check rate limit (last request timestamp in file)
# Send tokens
zeroned tx bank send faucet $ADDRESS 100000000uzrn --from faucet $TX_FLAGS
```

**Option B: Go service** (more robust)
- HTTP server with `/faucet?address=zrn1...` endpoint
- In-memory rate limiter with persistence
- Health check endpoint
- Can run on the seed node

**Option C: Discord/Telegram bot** (most accessible)
- Users type `!faucet zrn1...` in a channel
- Bot sends tokens and confirms
- Natural rate limiting via chat platform

**Recommend Option B** for testnet — it's self-contained, doesn't depend on chat platforms, and can run alongside the node. Keep it simple: single-file Go program, no framework.

### 2. Faucet Implementation

```go
// cmd/faucet/main.go
// - HTTP server on port 8080
// - POST /faucet {"address": "zrn1..."}
// - Rate limit: 1 per address per 24h (in-memory map + file persistence)
// - Amount: 100 ZRN per request
// - Uses keyring to sign and broadcast
// - Health: GET /health
// - Stats: GET /stats (total distributed, unique addresses, etc.)
```

### 3. Bootstrap Fund Distribution

The `x/claiming_pot` module already handles eligibility-gated token distribution. Verify:

```bash
# Check if bootstrap fund exists in genesis
$BINARY query claiming-pot pots $Q_FLAGS

# Check claiming flow
$BINARY tx claiming-pot claim <pot-id> --from alice $TX_FLAGS
```

The R25 assessment noted bootstrap funds are the ONE place account_type is enforced. Verify:
- Human accounts can claim bootstrap allocation
- Agent accounts can claim bootstrap allocation  
- Contract accounts CANNOT claim bootstrap allocation
- Each account can only claim once

### 4. Testnet Token Economics

Document the testnet token distribution:

| Allocation | Amount | Purpose |
|-----------|--------|---------|
| Faucet | 10,000,000 ZRN | Participant onboarding |
| Bootstrap fund | 1,000,000 ZRN | Per-account bootstrap claims |
| Validator stakes | 1,000,000 ZRN each | Initial validator set |
| Research fund | 0 (funded by block rewards) | Grows organically |

Total genesis supply should be enough for ~6 months of testnet activity without worrying about running out.

### 5. Airdrop Whitelist (Future Prep)

From MEMORY.md: "0.222 ZRN per whitelisted agent for bootstrap fund"

Don't implement the full airdrop now, but ensure the `x/claiming_pot` module can support:
- A whitelist of addresses
- Fixed amount per claim
- Deadline for claiming

Document whether the current module supports this or needs extension.

### 6. Test

- Faucet sends tokens successfully
- Rate limit blocks repeated requests
- Invalid addresses rejected
- Bootstrap fund claim works for human/agent accounts
- Bootstrap fund claim fails for contract accounts
- Faucet account doesn't run dry during test

## Files to Create

- `cmd/faucet/main.go` — Faucet HTTP service
- `cmd/faucet/README.md` — Setup and deployment instructions
- `docs/testnet-economics.md` — Token distribution documentation

## Success Criteria

- [ ] Faucet sends testnet ZRN on request
- [ ] Rate limiting works (1 per address per 24h)
- [ ] Bootstrap fund claim functional
- [ ] account_type enforcement verified on bootstrap
- [ ] Token economics documented
- [ ] Faucet deployable on seed node
