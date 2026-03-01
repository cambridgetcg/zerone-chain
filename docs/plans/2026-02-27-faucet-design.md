# R27-5 Faucet Design

## Overview

Standalone Go HTTP server that distributes testnet ZRN tokens to new participants. Runs alongside a validator node, signs transactions by shelling out to `zeroned tx bank send`.

## Architecture

Single-file Go binary at `tools/faucet/main.go`. No framework, no SDK client wiring — just `net/http` + subprocess calls.

### Endpoints

| Method | Path | Purpose |
|--------|------|---------|
| POST | /faucet | Request tokens: `{"address":"zrn1..."}` |
| GET | /health | Liveness check |
| GET | /stats | Distribution stats (total distributed, unique addresses, remaining budget) |

### Configuration (env vars)

| Var | Default | Description |
|-----|---------|-------------|
| FAUCET_AMOUNT | 100000000 (100 ZRN) | uzrn per request |
| FAUCET_COOLDOWN | 24 | Hours between requests per address |
| FAUCET_PORT | 8080 | Listen port |
| FAUCET_KEYRING_BACKEND | test | Keyring backend |
| FAUCET_FROM | faucet | Signing key name |
| FAUCET_NODE | tcp://localhost:26657 | Node RPC URL |
| FAUCET_HOME | (required) | zeroned home directory |
| FAUCET_CHAIN_ID | (required) | Chain ID |
| FAUCET_STATE_FILE | faucet-state.json | Rate-limit persistence file |
| FAUCET_MAX_TOTAL | 10000000000000 (10M ZRN) | Total distribution cap in uzrn |

### Request Flow

```
POST /faucet {"address":"zrn1..."}
  → Validate address (bech32, zrn1 prefix)
  → Check total cap (503 if exhausted — soft check, outside mutex)
  → Acquire mutex
  → Check rate limit (429 if cooldown active)
  → zeroned tx bank send --broadcast-mode block
  → Parse tx hash from JSON output
  → Update state (total_distributed, per-address timestamp)
  → Persist state (atomic: write .tmp, os.Rename)
  → Release mutex
  → Return {"status":"ok","tx_hash":"...","amount":"100000000uzrn"}
```

Rate limit check is inside the mutex to prevent TOCTOU races (two concurrent requests from the same address both passing the check).

### Error Responses

| Code | When | Body |
|------|------|------|
| 400 | Invalid/missing address | `{"error":"invalid address: must be bech32 zrn1..."}` |
| 429 | Rate limited | `{"error":"rate limited","retry_after":"..."}` |
| 503 | Total cap exhausted | `{"error":"faucet depleted"}` |
| 500 | Tx broadcast failure | `{"error":"send failed: ..."}` |

### State Persistence

```json
{
  "total_distributed": 500000000,
  "requests": {
    "zrn1abc...": "2026-02-27T10:00:00Z"
  }
}
```

Atomic writes: write to `.tmp` file, then `os.Rename`. Loaded on startup.

### Concurrency

`sync.Mutex` around the send+rate-check+persist critical section. Single-threaded sends prevent sequence number conflicts.

### CORS

`Access-Control-Allow-Origin: *` on all responses + OPTIONS preflight handler.

## Claiming Pot Analysis

The `x/claiming_pot` module does NOT enforce `account_type`. Its `EligibilityCriteria` supports:
- Whitelist (list of addresses)
- MinStakingTier (validator tier check)
- MinRegistrationAge (registration block age — currently stubbed, returns 0 for all)

No account_type field exists in the proto or enforcement logic. Adding it would be a separate feature task.

For airdrop/whitelist support: the module already supports per-pot whitelists with fixed amounts and deadlines (via `end_block`). The "0.222 ZRN per whitelisted agent" use case is achievable with the current module by creating a pot with a whitelist and `total_amount = 0.222 * len(whitelist)`.

## Deliverables

1. `tools/faucet/main.go` — HTTP faucet server
2. `tools/faucet/README.md` — setup and deployment instructions
3. `docs/testnet-economics.md` — token distribution documentation
