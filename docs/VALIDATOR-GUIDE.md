# Zerone Validator Guide

This guide covers everything you need to join **zerone-testnet-1** as a validator.

## Table of Contents

- [Prerequisites](#prerequisites)
- [Installation](#installation)
- [Node Setup](#node-setup)
- [Becoming a Validator](#becoming-a-validator)
- [Validator Tiers](#validator-tiers)
- [Proof of Truth Participation](#proof-of-truth-participation)
- [Cosmovisor Setup](#cosmovisor-setup)
- [Monitoring](#monitoring)
- [Troubleshooting](#troubleshooting)

---

## Prerequisites

### Hardware Requirements

| Component | Minimum | Recommended |
|-----------|---------|-------------|
| CPU       | 4 cores | 8+ cores    |
| RAM       | 16 GB   | 32 GB       |
| Disk      | 500 GB SSD | 1 TB NVMe |
| Network   | 100 Mbps | 1 Gbps     |

### Software Requirements

- **Go** 1.24+ ([install guide](https://go.dev/doc/install))
- **jq** 1.6+ (`brew install jq` on macOS, `apt install jq` on Ubuntu)
- **make**
- **git**
- **OS**: Ubuntu 22.04+ or macOS 13+

---

## Installation

### Build from source

```bash
git clone https://github.com/zerone-chain/zerone.git
cd zerone
make install
```

This installs the `zeroned` binary to `$(go env GOPATH)/bin/zeroned`.

Verify:

```bash
zeroned version
```

### Docker (easiest)

Build and run with Docker — no Go toolchain required:

```bash
# Build the image
docker build -t zerone:latest .

# Verify
docker run --rm zerone:latest version

# Initialize and run a node
docker run -v ~/.zeroned:/root/.zeroned zerone:latest init my-node --chain-id zerone-testnet-1
docker compose up -d
```

For validators with Cosmovisor auto-upgrades:

```bash
docker build -f Dockerfile.validator -t zerone-validator:latest .
```

### Pre-built binary

Download the binary for your platform from the releases page:

```bash
# Linux amd64
curl -L https://github.com/zerone-chain/zerone/releases/latest/download/zeroned-linux-amd64 -o zeroned

# Linux arm64
curl -L https://github.com/zerone-chain/zerone/releases/latest/download/zeroned-linux-arm64 -o zeroned

# macOS arm64 (Apple Silicon)
curl -L https://github.com/zerone-chain/zerone/releases/latest/download/zeroned-darwin-arm64 -o zeroned

chmod +x zeroned
sudo mv zeroned /usr/local/bin/
zeroned version
```

Verify the checksum against the `.sha256` file published with each release.

---

## Node Setup

### Option A: Automated (recommended)

Use the join-testnet script:

```bash
scripts/join-testnet.sh --moniker "my-validator"
```

With Cosmovisor and systemd:

```bash
scripts/join-testnet.sh \
  --moniker "my-validator" \
  --genesis /path/to/genesis.json \
  --cosmovisor \
  --systemd
```

See `scripts/join-testnet.sh --help` for all options.

### Option B: Manual

#### 1. Initialize the node

```bash
zeroned init "my-validator" --chain-id zerone-testnet-1
```

This creates the default config at `$HOME/.zeroned/`.

#### 2. Install the genesis file

Copy the official genesis file to your config directory:

```bash
cp genesis.json $HOME/.zeroned/config/genesis.json
zeroned genesis validate
```

#### 3. Configure seed nodes

Add seed nodes from [`seeds.txt`](../seeds.txt) to your `config.toml`:

```toml
# $HOME/.zeroned/config/config.toml
[p2p]
seeds = "node-id1@ip1:26656,node-id2@ip2:26656"
```

#### 4. Set minimum gas price

```toml
# $HOME/.zeroned/config/app.toml
minimum-gas-prices = "0.025uzrn"
```

See [`config/config.toml.template`](../config/config.toml.template) and
[`config/app.toml.template`](../config/app.toml.template) for full
recommended configurations.

#### 5. Start the node

```bash
zeroned start --minimum-gas-prices 0.025uzrn
```

Wait for your node to fully sync before registering as a validator:

```bash
zeroned status | jq '.sync_info.catching_up'
# Should return: false
```

---

## Becoming a Validator

Zerone uses a two-step registration process: **register your account**, then
**register as a validator**.

### Step 1: Create a key

```bash
zeroned keys add my-validator
```

Save the mnemonic securely. Your address will have the `zrn1...` prefix.

### Step 2: Fund your account

Obtain testnet ZRN from the faucet or another validator. You need enough to
cover your self-delegation plus transaction fees.

### Step 3: Register your account

<!-- Source: x/auth/client/cli/tx.go:44-76 -->

```bash
zeroned tx auth register-account \
  "did:zerone:my-validator" \
  "$(zeroned keys show my-validator --pubkey --output json | jq -r .key)" \
  "validator" \
  --from my-validator \
  --chain-id zerone-testnet-1 \
  --fees 5000uzrn
```

Arguments:
- `did` - Your decentralized identifier (DID) string
- `public-key` - Your account's public key (hex)
- `account-type` - Account type: `"validator"`, `"agent"`, or `"user"`

Optional flags:
- `--operational-key-hash` - Hash of your operational key
- `--metadata` - Account metadata (JSON string)

### Step 4: Register as a validator

<!-- Source: x/staking/client/cli/tx.go:35-74 -->

```bash
zeroned tx staking register-validator \
  "$(zeroned comet show-validator)" \
  111000uzrn \
  --commission 500 \
  --moniker "My Validator" \
  --identity "did:zerone:my-validator" \
  --website "https://example.com" \
  --details "A Zerone validator" \
  --from my-validator \
  --chain-id zerone-testnet-1 \
  --fees 5000uzrn
```

Arguments:
- `pubkey-hex` - Your CometBFT consensus public key
- `self-delegation` - Amount to self-delegate (e.g., `111000uzrn` for Apprentice tier)

Flags:
- `--commission` - Commission rate in basis points (BPS). Max 10,000 (= 100%). Example: `500` = 5%
- `--moniker` - Human-readable validator name (max 70 characters)
- `--identity` - DID for validator identity (max 128 characters)
- `--website` - Website URL (max 140 characters)
- `--details` - Description (max 2,000 characters)

> **Note:** Zerone uses `register-validator`, NOT the standard Cosmos SDK
> `create-validator` command. The `--commission` flag accepts basis points
> (not a decimal fraction).

### Bootstrap period

During the first 480,000 blocks (~14 days), these transaction types are
**gas-free**:

<!-- Source: app/gas.go:364-371 -->

- `MsgRegisterValidator`
- `MsgRegisterAccount`
- `MsgSubmitClaim`
- `MsgSubmitCommitment`
- `MsgSubmitReveal`

This allows the network to bootstrap Proof of Truth consensus before fees
are collected.

---

## Validator Tiers

<!-- Source: x/staking/types/types.go:66-132 -->

Zerone validators are organized into four tiers with increasing requirements
and rewards:

| Tier | Min Stake | Min Reputation | Min Verifications | Reward Multiplier | Selection Weight |
|------|-----------|----------------|-------------------|-------------------|-----------------|
| **Apprentice** | 111,000 uzrn (0.111 ZRN) | -- | -- | 0.1x | 0.1x |
| **Verified** | 1,110,000 uzrn (1.11 ZRN) | 77% | 22 | 0.5x | 0.5x |
| **Scholar** | 1,111,000,000 uzrn (1,111 ZRN) | 50% | 11 | 1.0x | 1.0x |
| **Guardian** | 11,111,000,000 uzrn (11,111 ZRN) | 77% | 333 | 2.0x | 1.5x |

### Tier details

**Apprentice** — Entry tier. Can verify `protocol`, `computational`, and
`formal` claims. Higher slash multiplier (1.5x) to discourage Sybil
behavior. Maximum 111 Apprentice validators (Sybil cap).

**Verified** — Proven verifiers with 77% accuracy and 22+ verifications.
Gains access to `empirical` claims. Slash multiplier 1.2x.

**Scholar** — Block-producing tier. Requires substantial stake (1,111 ZRN).
Full category access including `oracle` and `attestation`. Standard slash
multiplier (1.0x). Subject to `MaxValidators` cap (default: 100).

**Guardian** — Highest tier. Requires 11,111 ZRN stake, 77% accuracy, and
333 verifications. Zero tolerance for slashing. Gains access to `predictive`,
`social`, and `contested` categories. 2.0x reward multiplier with 3x
contested verification multiplier.

### Tier progression

Your tier is computed automatically based on your current stake, reputation
score, verification count, and accuracy. Increase your self-delegation with:

```bash
zeroned tx staking update-stake 1000000000uzrn \
  --increase \
  --from my-validator \
  --chain-id zerone-testnet-1
```

---

## Proof of Truth Participation

Zerone validators participate in **Proof of Truth (PoT)** consensus — a
three-phase knowledge verification process.

### Verification phases

<!-- Source: x/knowledge/types/genesis.go:7-16 -->

1. **Commit phase** (10 blocks) — Validators submit a hash commitment of
   their verification judgment.
2. **Reveal phase** (10 blocks) — Validators reveal their actual judgment.
   Missing a reveal is slashed at 10%.
3. **Aggregation phase** (5 blocks) — The network aggregates all revealed
   judgments to determine the claim's truth status.

### Claim lifecycle

A claim is submitted with a minimum stake of 1 ZRN. Between 3 and 22
verifiers are selected based on tier and reputation. If the aggregated
confidence reaches 77%, the claim is accepted as a fact. Below 30%, it is
rejected. Between 30% and 50%, it enters a provisional state that can be
challenged.

### Slashing

<!-- Source: x/knowledge/types/genesis.go:23-27 -->

| Offense | Slash Rate |
|---------|-----------|
| Wrong verification | 5% of stake |
| Missed reveal | 10% of stake |
| Equivocation (conflicting votes) | 20% of stake |
| Invalid claim submission | 22% of stake |

### Rewards

Correct verifications earn 3 ZRN per round (decaying by 0.1% per epoch).
Citation economics distribute 15% of rewards to cited fact authors, with a
20% bonus for cross-domain citations.

---

## Cosmovisor Setup

Cosmovisor automates binary upgrades through governance proposals. See
[`cosmovisor/README.md`](../cosmovisor/README.md) for the full setup guide.

Quick setup:

```bash
# Install cosmovisor
go install cosmossdk.io/tools/cosmovisor/cmd/cosmovisor@latest

# Initialize (copies binary to cosmovisor/genesis/bin/)
make cosmovisor-init

# Set environment
export DAEMON_NAME=zeroned
export DAEMON_HOME=$HOME/.zeroned
export DAEMON_ALLOW_DOWNLOAD_BINARIES=false
export DAEMON_RESTART_AFTER_UPGRADE=true
export DAEMON_LOG_BUFFER_SIZE=512

# Start via cosmovisor
cosmovisor run start
```

Or use the join script with `--cosmovisor` flag:

```bash
scripts/join-testnet.sh --moniker "my-validator" --cosmovisor
```

---

## Monitoring

### Prometheus metrics

Both CometBFT and the application expose Prometheus metrics when enabled in
the configuration templates:

- **CometBFT metrics**: `http://localhost:26660/metrics`
- **App telemetry**: enabled via `app.toml` telemetry section

### Key metrics to watch

| Metric | Description |
|--------|-------------|
| `cometbft_consensus_height` | Current block height |
| `cometbft_consensus_validators` | Number of active validators |
| `cometbft_consensus_missing_validators` | Validators missing from last block |
| `cometbft_consensus_rounds` | Number of rounds in current height |
| `cometbft_p2p_peers` | Number of connected peers |
| `cometbft_mempool_size` | Transactions in mempool |

### Status checks

```bash
# Node sync status
zeroned status | jq '.sync_info'

# Validator info
zeroned query staking validators --output json | jq

# Your validator's signing info
zeroned query slashing signing-info "$(zeroned comet show-validator)"
```

---

## Troubleshooting

### Node won't start

**"Genesis file not found"**
```bash
# Verify genesis file exists
ls -la $HOME/.zeroned/config/genesis.json

# Re-validate
zeroned genesis validate
```

**"Address already in use"**

Another process is using port 26656/26657. Check and kill it:
```bash
lsof -i :26656
lsof -i :26657
```

### Node is not syncing

**Check peers:**
```bash
zeroned status | jq '.node_info.network'
curl -s localhost:26657/net_info | jq '.result.n_peers'
```

If zero peers, verify your `seeds` and `persistent_peers` in `config.toml`
and ensure port 26656 is open in your firewall.

**State sync (fast catch-up):**

If the chain has been running for a while, enable state sync in
`config.toml` to bootstrap quickly. You need a trusted block height and hash
from an RPC node:

```bash
# Get a recent block
LATEST=$(curl -s https://rpc.zerone.network/block | jq -r '.result.block.header.height')
TRUST_HEIGHT=$((LATEST - 2000))
TRUST_HASH=$(curl -s "https://rpc.zerone.network/block?height=${TRUST_HEIGHT}" | jq -r '.result.block_id.hash')
```

Then set in `config.toml`:
```toml
[statesync]
enable = true
rpc_servers = "https://rpc.zerone.network:443,https://rpc2.zerone.network:443"
trust_height = <TRUST_HEIGHT>
trust_hash = "<TRUST_HASH>"
```

### Validator is jailed

If your validator misses too many blocks, it may be jailed. To unjail:

```bash
zeroned tx slashing unjail \
  --from my-validator \
  --chain-id zerone-testnet-1 \
  --fees 5000uzrn
```

**"Transactions are not being processed"**

SDK v0.50 defaults `max-txs = -1` in `app.toml`, which activates the
NoOpMempool (silently dropping all transactions). Fix:

```toml
# $HOME/.zeroned/config/app.toml
[mempool]
max-txs = 5000
```

Or run `configure-node.sh` which sets this automatically.

### Common errors

| Error | Cause | Fix |
|-------|-------|-----|
| `validator already registered` | Duplicate registration | Your validator is already active |
| `insufficient funds` | Not enough ZRN | Fund your account before registering |
| `commission exceeds 100%` | `--commission` > 10,000 | Use BPS: 500 = 5%, 1000 = 10% |
| `moniker too long` | > 70 characters | Shorten your moniker |
| `min self delegation not met` | Stake below 111,000 uzrn | Increase your self-delegation |

### Restarting safely

```bash
# Graceful stop
kill -TERM $(pgrep zeroned)

# Or if using systemd
sudo systemctl stop zeroned

# Start again
zeroned start --minimum-gas-prices 0.025uzrn
```

---

## Further Reading

- [Parameters Reference](PARAMETERS.md) — All governance-adjustable parameters
- [FAQ](FAQ.md) — Frequently asked questions
- [Cosmovisor Setup](../cosmovisor/README.md) — Automated upgrade management
