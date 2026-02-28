# Zerone Testnet Validator Guide

Chain ID: `zerone-testnet-1`

This guide walks you through joining the Zerone public testnet as a validator.

## 1. Hardware Requirements

Testnet requirements are minimal:

| Resource | Minimum | Recommended |
|----------|---------|-------------|
| CPU      | 2 cores | 4 cores     |
| RAM      | 4 GB    | 8 GB        |
| Disk     | 50 GB SSD | 100 GB SSD |
| Network  | 10 Mbps | 50 Mbps     |

Any modern Linux VPS (Ubuntu 22.04+, Debian 12+) or macOS machine will work.

## 2. Install zeroned

### Option A: Build from Source

Requires Go 1.24+ and `jq`:

```bash
# Clone the repository
git clone https://github.com/nickkpope/zerone.git
cd zerone

# Build the binary
go build -ldflags "-X github.com/cosmos/cosmos-sdk/version.Name=zerone \
  -X github.com/cosmos/cosmos-sdk/version.AppName=zeroned" \
  -o build/zeroned ./cmd/zeroned

# Install to PATH
sudo cp build/zeroned /usr/local/bin/

# Verify
zeroned version
```

### Option B: Docker

```bash
# Pull the image (when available)
docker pull ghcr.io/nickkpope/zerone:latest

# Run zeroned commands via Docker
docker run --rm -v ~/.zeroned:/root/.zeroned ghcr.io/nickkpope/zerone:latest zeroned version
```

## 3. Initialize Your Node

```bash
# Choose a moniker (your validator's display name)
zeroned init <your-moniker> --chain-id zerone-testnet-1
```

This creates `~/.zeroned/` with default configuration.

## 4. Download Genesis

Download the testnet genesis file:

```bash
# From the repository
curl -o ~/.zeroned/config/genesis.json \
  https://raw.githubusercontent.com/nickkpope/zerone/main/networks/zerone-testnet-1/genesis.json
```

Verify the genesis hash matches the published value:

```bash
shasum -a 256 ~/.zeroned/config/genesis.json
```

## 5. Set Persistent Peers

Edit `~/.zeroned/config/config.toml` and set the seed/persistent peers:

```toml
# Seed node (Zerone VPS)
seeds = ""
persistent_peers = "<node-id>@80.78.19.135:26656"
```

Check `networks/zerone-testnet-1/README.md` for current seed node addresses.

Additional recommended config changes in `config.toml`:

```toml
# Target 6-second block time
timeout_commit = "6s"
timeout_propose = "3s"
```

And in `~/.zeroned/config/app.toml`:

```toml
# Set minimum gas price
minimum-gas-prices = "1uzrn"

# Enable mempool
max-txs = 5000

# Disable IAVL fast node (prevents query errors)
iavl-disable-fastnode = true
```

## 6. Start Your Node

```bash
zeroned start --minimum-gas-prices 1uzrn
```

Wait for the node to sync. You can check sync status:

```bash
# In another terminal
curl -s http://localhost:26657/status | jq '.result.sync_info'
```

When `catching_up` is `false`, your node is synced and ready.

### Running as a Service (recommended)

Create a systemd service for automatic restart:

```bash
sudo tee /etc/systemd/system/zeroned.service > /dev/null <<EOF
[Unit]
Description=Zerone Node
After=network-online.target

[Service]
User=$USER
ExecStart=$(which zeroned) start --minimum-gas-prices 1uzrn
Restart=on-failure
RestartSec=3
LimitNOFILE=65535

[Install]
WantedBy=multi-user.target
EOF

sudo systemctl daemon-reload
sudo systemctl enable zeroned
sudo systemctl start zeroned

# Check logs
journalctl -u zeroned -f
```

## 7. Create Your Validator

Once your node is synced, create a validator transaction:

```bash
# Create a key (or recover existing)
zeroned keys add <key-name>
# Save the mnemonic securely!

# Get your address
zeroned keys show <key-name> -a
```

Get testnet tokens from the faucet (see network README for faucet details), then:

```bash
zeroned tx staking create-validator \
  --amount 1000000uzrn \
  --pubkey $(zeroned tendermint show-validator) \
  --moniker "<your-moniker>" \
  --chain-id zerone-testnet-1 \
  --commission-rate "0.10" \
  --commission-max-rate "0.20" \
  --commission-max-change-rate "0.01" \
  --min-self-delegation "1000000" \
  --from <key-name> \
  --fees 100000uzrn
```

Verify your validator is in the active set:

```bash
zeroned query staking validators --output json | jq '.validators[] | select(.description.moniker == "<your-moniker>")'
```

## 8. Register Your Account

Register your account type in the Zerone auth system:

```bash
# Register as a human account
zeroned tx zerone-auth register-account human \
  --from <key-name> \
  --chain-id zerone-testnet-1 \
  --fees 100000uzrn
```

Account types: `human`, `agent`, `hybrid`

## 9. Qualify for Knowledge Domains

To participate in knowledge verification, qualify for domains by staking:

```bash
# Qualify for a domain (requires 100 ZRN min stake)
zeroned tx qualification qualify-stake <domain> 100000000uzrn \
  --from <key-name> \
  --chain-id zerone-testnet-1 \
  --fees 100000uzrn
```

Available domains: `mathematics`, `physics`, `computer_science`, `philosophy`, `logic`, `chemistry`, `biology`, `economics`, `linguistics`, `psychology`, `sociology`, `cosmology`, `information_theory`, `ethics`, `theology`, `agent_rights`, `agent_purpose`, `general`

Testnet qualification requirements (lower than mainnet for iteration):
- Minimum stake: 100 ZRN
- Minimum verifications for track record: 50
- Minimum accuracy: 75%

## 10. Submit Knowledge Claims

Once qualified, you can submit claims to the Proof of Truth system:

```bash
zeroned tx knowledge submit-claim \
  "<claim content - at least 20 characters>" \
  <domain> \
  <category> \
  1000000 \
  --from <key-name> \
  --chain-id zerone-testnet-1 \
  --fees 100000uzrn
```

Categories: `analytic`, `formal`, `empirical`, `protocol`, `computational`

## Testnet Parameters Reference

| Parameter | Value | Notes |
|-----------|-------|-------|
| Block time | ~6s | target timeout_commit |
| Max gas/block | 50,000,000 | |
| Knowledge commit phase | 50 blocks (~5 min) | |
| Knowledge reveal phase | 50 blocks (~5 min) | |
| Research review period | 500 blocks (~50 min) | |
| Min reviewers | 2 | |
| Bounty fulfillment | 1,000 blocks (~100 min) | |
| SDK Gov voting | 1 day | |
| SDK Gov min deposit | 10 ZRN | |
| SDK Staking unbonding | 1 day | |
| Max validators | 20 | |
| Qualification min stake | 100 ZRN | |
| Qualification min verifications | 50 | lower than mainnet 100 |
| Qualification min accuracy | 75% | lower than mainnet 80% |

## Troubleshooting

**Node won't start:**
- Check genesis hash matches published value
- Ensure ports 26656 (P2P) and 26657 (RPC) are not in use
- Verify Go version: `go version` (need 1.24+)

**Node stuck syncing:**
- Check persistent_peers is set correctly
- Ensure firewall allows port 26656 inbound
- Try adding more peers from the network README

**Validator not in active set:**
- Check you have enough stake (min 1 ZRN self-delegation)
- Active set is limited to 20 validators
- Verify with: `zeroned query staking validators --status bonded`

**Transaction errors:**
- Ensure `minimum-gas-prices = "1uzrn"` in app.toml
- Check account has sufficient balance: `zeroned query bank balances <address>`
