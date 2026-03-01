# Zerone Testnet Faucet

Lightweight HTTP faucet that distributes testnet ZRN tokens with per-address rate limiting and total supply cap enforcement.

## Build

```bash
go build -o faucet ./tools/faucet/
```

## Run

```bash
FAUCET_HOME=~/.zeroned/localnet/coordinator \
FAUCET_CHAIN_ID=zerone-localnet \
./faucet
```

The `zeroned` binary must be in `$PATH`.

## Configuration

All settings are via environment variables.

| Variable | Required | Default | Description |
|---|---|---|---|
| `FAUCET_HOME` | yes | - | zeroned home directory with keyring |
| `FAUCET_CHAIN_ID` | yes | - | Chain ID |
| `FAUCET_AMOUNT` | no | `100000000` | uzrn per request (100 ZRN) |
| `FAUCET_COOLDOWN` | no | `24` | Hours between requests per address |
| `FAUCET_PORT` | no | `8080` | Listen port |
| `FAUCET_KEYRING_BACKEND` | no | `test` | Keyring backend |
| `FAUCET_FROM` | no | `faucet` | Signing key name |
| `FAUCET_NODE` | no | `tcp://localhost:26657` | Node RPC URL |
| `FAUCET_STATE_FILE` | no | `faucet-state.json` | Rate-limit persistence file |
| `FAUCET_MAX_TOTAL` | no | `10000000000000` | Total cap in uzrn (10M ZRN) |

## Endpoints

### POST /faucet

Request tokens for an address.

```bash
curl -s -X POST http://localhost:8080/faucet \
  -H "Content-Type: application/json" \
  -d '{"address":"zrn1abc123..."}'
```

**Success (200):**
```json
{
  "status": "ok",
  "tx_hash": "A1B2C3D4E5F6...",
  "amount": "100000000uzrn"
}
```

**Rate limited (429):**
```json
{
  "status": "error",
  "error": "rate limited",
  "retry_after": "2026-02-28T12:00:00Z"
}
```

**Invalid address (400):**
```json
{
  "status": "error",
  "error": "invalid address: must be zrn1... bech32"
}
```

### GET /health

```bash
curl -s http://localhost:8080/health
```

```json
{"status":"ok"}
```

### GET /stats

```bash
curl -s http://localhost:8080/stats
```

```json
{
  "total_distributed_uzrn": 500000000,
  "unique_addresses": 5,
  "remaining_uzrn": 9999500000000,
  "amount_per_request_uzrn": 100000000,
  "cooldown_hours": 24
}
```

## Rate Limits

- 1 request per address per 24 hours (configurable via `FAUCET_COOLDOWN`).
- Returns **429 Too Many Requests** with a `retry_after` timestamp when on cooldown.
- Returns **503 Service Unavailable** when the total distribution cap (`FAUCET_MAX_TOTAL`) is reached.

State is persisted to `FAUCET_STATE_FILE` after every successful send so rate limits survive restarts.
