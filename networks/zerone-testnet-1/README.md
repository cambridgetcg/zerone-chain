# zerone-testnet-1

Public testnet for the Zerone network.

## Chain Info

| Field | Value |
|-------|-------|
| Chain ID | `zerone-testnet-1` |
| Denomination | `uzrn` (1 ZRN = 1,000,000 uzrn) |
| Target block time | 6 seconds |
| Max gas per block | 50,000,000 |
| Max validators | 20 |
| Genesis time | TBD (set at ceremony) |

## Seed Node

```
# Zerone VPS seed node
persistent_peers = "<node-id>@80.78.19.135:26656"
```

Node ID will be published after the genesis ceremony is run.

## Genesis

The genesis file will be generated using the testnet genesis pipeline:

```bash
# Generate genesis (coordinator)
scripts/testnet-genesis.sh init
scripts/testnet-genesis.sh add-validator val1
scripts/testnet-genesis.sh add-validator val2
scripts/testnet-genesis.sh finalize
scripts/testnet-genesis.sh export
```

Once finalized, `genesis.json` will be placed in this directory.

Verify your genesis:

```bash
shasum -a 256 genesis.json
```

## Bootstrap Accounts

| Account | Balance | Purpose |
|---------|---------|---------|
| Foundation | 1,000,000 ZRN | Network operations |
| Research Treasury | 500,000 ZRN | Research fund |
| Faucet | 100,000 ZRN | Token distribution |

Validators receive 100,000 ZRN balance with 10,000 ZRN staked at genesis.

## Faucet

Testnet faucet details will be published here once available.

## Joining

See the [Testnet Validator Guide](../../docs/testnet-validator-guide.md) for full instructions.

Quick start:

```bash
# 1. Build
git clone https://github.com/nickkpope/zerone.git && cd zerone
go build -o build/zeroned ./cmd/zeroned
sudo cp build/zeroned /usr/local/bin/

# 2. Init
zeroned init <moniker> --chain-id zerone-testnet-1

# 3. Genesis
curl -o ~/.zeroned/config/genesis.json \
  https://raw.githubusercontent.com/nickkpope/zerone/main/networks/zerone-testnet-1/genesis.json

# 4. Peers (edit ~/.zeroned/config/config.toml)
# persistent_peers = "<node-id>@80.78.19.135:26656"

# 5. Start
zeroned start --minimum-gas-prices 1uzrn
```

## Key Parameters (Testnet-Tuned)

| Module | Parameter | Value | Notes |
|--------|-----------|-------|-------|
| Knowledge | commit_phase_blocks | 50 | ~5 min |
| Knowledge | reveal_phase_blocks | 50 | ~5 min |
| Knowledge | min_verifiers | 2 | |
| Knowledge | fitness_epoch_blocks | 1,000 | ~100 min |
| Research | review_period_blocks | 500 | ~50 min |
| Research | min_reviewer_count | 2 | |
| Research | bounty_fulfillment | 1,000 | ~100 min |
| Qualification | min_stake | 100 ZRN | |
| Qualification | min_verifications | 50 | mainnet: 100 |
| Qualification | min_accuracy | 75% | mainnet: 80% |
| Partnerships | coercion_review | 100 | ~10 min |
| SDK Gov | voting_period | 1 day | mainnet: 14 days |
| SDK Gov | min_deposit | 10 ZRN | |
| SDK Gov | quorum | 25% | |
| SDK Staking | unbonding_time | 1 day | mainnet: ~7 days |
| SDK Staking | max_validators | 20 | |
| Vesting | revenue_split | 55/22/19.67/3.33 | same as mainnet |

## Endpoints

Will be populated after launch:

| Validator | RPC | gRPC | API |
|-----------|-----|------|-----|
| seed (VPS) | `http://80.78.19.135:26657` | `80.78.19.135:9090` | `http://80.78.19.135:1317` |

## Verifying Parameters

After the chain is running, verify testnet parameters:

```bash
scripts/testnet-genesis.sh verify
# or with custom RPC:
RPC_URL=http://80.78.19.135:26657 scripts/testnet-genesis.sh verify
```
