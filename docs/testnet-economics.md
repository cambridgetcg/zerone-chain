# Testnet Token Economics

This document describes the token supply, allocation, and distribution mechanics for the Zerone testnet (localnet and `zerone-testnet-1`).

## Genesis Supply

| Denomination | Amount |
|-------------|--------|
| ZRN | 222,222,222,222 |
| uzrn | 222,222,222,222,000,000 |

1 ZRN = 1,000,000 uzrn (micro-ZRN).

## Allocation Table

| Category | ZRN | % | Purpose |
|----------|-----|---|---------|
| Research Fund | 44,444,444,444 | 20.00% | Protocol-governed research grants and bounties |
| Founder | 22,222,222,222 | 10.00% | Founder allocation |
| AI Agent | 22,222,222,222 | 10.00% | AI agent bootstrap and operational funds |
| Validators (4 x 22,222,222,222) | 88,888,888,888 | 40.00% | Genesis validator allocations (equal per-validator) |
| Claiming Pots | 44,444,444,446 | 20.00% | Airdrops, vesting distributions, community claims |
| **Total** | **222,222,222,222** | **100.00%** | |

The Claiming Pots allocation includes 2 ZRN above an exact 20% share to absorb the integer rounding remainder from dividing the total supply into the five allocation buckets.

## Faucet

The faucet is a standalone HTTP service (`tools/faucet/`) that distributes testnet ZRN from a pre-funded account.

| Parameter | Value |
|-----------|-------|
| Pre-funded balance | 10,000,000 ZRN (10M) |
| Per-request amount | 100 ZRN |
| Rate limit | 1 request per address per 24 hours |
| Total distribution cap | 10,000,000 ZRN (equals pre-funded balance) |
| Unique requests before cap | 100,000 |

The faucet account is funded at genesis in `scripts/localnet.sh` with `FAUCET_BALANCE=10000000000000` uzrn (10M ZRN). The per-request default is `FAUCET_AMOUNT=100000000` uzrn (100 ZRN), configurable via environment variable.

### Endpoints

| Method | Path | Purpose |
|--------|------|---------|
| POST | `/faucet` | Request tokens (`{"address":"zrn1..."}`) |
| GET | `/health` | Liveness check |
| GET | `/stats` | Distribution stats (total distributed, unique addresses, remaining) |

## Claiming Pots

The `x/claiming_pot` module manages on-chain token distribution pots with eligibility rules and vesting schedules.

### Capabilities

- **Whitelist**: Restrict claiming to a set of bech32 addresses. Empty whitelist means open to all qualified.
- **Minimum staking tier**: Require claimants to hold a minimum validator tier.
- **Minimum registration age**: Require a minimum number of blocks since account registration.
- **Linear vesting**: Tokens vest linearly from `start_block` to `end_block` with optional `cliff_blocks` delay.
- **One claim per address per pot**: Duplicate claims are rejected.
- **Pot lifecycle**: Pots transition from ACTIVE to DEPLETED (fully claimed) or EXPIRED.

### account_type Enforcement

The claiming pot module does **not** enforce `account_type`. No `account_type` field exists in the `EligibilityCriteria` proto or in the claim validation logic. Eligibility is determined solely by whitelist membership, staking tier, and registration age. Adding account_type enforcement would be a separate feature task.

### Airdrop Support

The module can implement airdrops using existing primitives:

1. Create a pot with a **whitelist** of eligible addresses.
2. Set `total_amount` to `per_address_amount * len(whitelist)`.
3. Set `end_block` as a deadline after which the pot expires.
4. Each whitelisted address claims once, receiving their vested share.

Example: distributing 0.222 ZRN to each of 1,000 whitelisted agents requires a pot with `total_amount = 222000000` uzrn (222 ZRN), a whitelist of 1,000 addresses, and a per-claim vesting amount of 222,000 uzrn.

## Localnet Accounts

### Validators

| Account | Balance (ZRN) | Stake (ZRN) | Purpose |
|---------|--------------|-------------|---------|
| val0 | 100,000 | 100 | Minimal-stake validator (Apprentice tier testing) |
| val1 | 1,000,000 | 1,000 | Low-stake validator |
| val2 | 10,000,000 | 10,000 | Medium-stake validator |
| val3 | 100,000,000 | 100,000 | High-stake validator (tier progression testing) |

### Bootstrap Accounts

| Account | Balance (ZRN) | Purpose |
|---------|--------------|---------|
| faucet | 10,000,000 | Faucet distribution source |
| test1 | 10,000 | General testing |
| test2 | 10,000 | General testing |
| test3 | 10,000 | General testing |

### Localnet Endpoints

| Validator | RPC | gRPC | API |
|-----------|-----|------|-----|
| val0 | 127.0.0.1:26601 | 127.0.0.1:9090 | 127.0.0.1:1317 |
| val1 | 127.0.0.1:26611 | 127.0.0.1:9091 | 127.0.0.1:1318 |
| val2 | 127.0.0.1:26621 | 127.0.0.1:9092 | 127.0.0.1:1319 |
| val3 | 127.0.0.1:26631 | 127.0.0.1:9093 | 127.0.0.1:1320 |

## Source References

- `app/constants.go` -- supply and allocation constants
- `scripts/localnet.sh` -- validator balances, stakes, faucet funding, test accounts
- `proto/zerone/claiming_pot/v1/state.proto` -- pot and eligibility proto definitions
- `x/claiming_pot/keeper/msg_server.go` -- eligibility enforcement logic
- `docs/plans/2026-02-27-faucet-design.md` -- faucet design specification
