# R10-1 — Validator Onboarding

## Goal

Everything an external validator needs to join the Zerone testnet: documentation, scripts,
configuration templates, and troubleshooting guide.

## Working Directory

`/Users/yuai/Desktop/Zerone`

## Deliverables

### 1. Validator Guide (`docs/VALIDATOR-GUIDE.md`)

Comprehensive guide covering:

#### Prerequisites
- Hardware requirements (CPU, RAM, disk, network)
- Software: Go 1.22+, git, make
- OS: Linux (Ubuntu 22.04+ recommended), macOS for development

#### Installation
```bash
git clone https://github.com/zerone-chain/zerone.git
cd zerone
make install
zeroned version
```

#### Initializing a Node
```bash
zeroned init <moniker> --chain-id zerone-testnet-1
```

#### Getting the Genesis File
```bash
curl -o ~/.zeroned/config/genesis.json https://<genesis-url>/genesis.json
```

#### Configuring Peers
- Seed nodes list
- Persistent peers
- `config.toml` settings (mempool size, timeouts, CORS)
- `app.toml` settings (pruning, min gas prices, API/gRPC)

#### Creating a Validator
```bash
zeroned tx staking create-validator \
  --amount 10000000uzrn \
  --pubkey $(zeroned tendermint show-validator) \
  --moniker "my-validator" \
  --chain-id zerone-testnet-1 \
  --commission-rate 0.10 \
  --from validator-key
```

#### Zerone-Specific Registration
```bash
# Register in Zerone auth module
zeroned tx zerone-auth register-account --from validator-key

# Register in Zerone staking module  
zeroned tx zerone-staking register-validator --from validator-key
```

#### Proof of Truth Participation
- How PoT rounds work
- Validator requirements for each tier
- How to monitor your verification performance

#### Monitoring
- Prometheus metrics endpoint
- Key metrics to watch
- Alerting recommendations

#### Troubleshooting
- Common errors and solutions
- How to safely restart
- State sync / fast sync options

### 2. Join Script (`scripts/join-testnet.sh`)
Automated script that:
- Downloads and verifies the genesis file
- Configures seed nodes and persistent peers
- Sets recommended `config.toml` and `app.toml` values
- Optionally sets up cosmovisor
- Optionally sets up systemd service

```bash
#!/bin/bash
# Usage: ./scripts/join-testnet.sh <moniker> [--cosmovisor] [--systemd]
```

### 3. Seed Node Configuration
Create `seeds.txt` with seed node addresses:
```
<node-id>@<ip>:26656
```

### 4. Configuration Templates
- `config/config.toml.template` — recommended CometBFT settings
- `config/app.toml.template` — recommended app settings
- Include comments explaining each non-default setting

### 5. Parameter Reference (`docs/PARAMETERS.md`)
Document ALL governance-adjustable parameters across all 32 modules:
- Parameter name, module, type, default value, description
- Which parameters are critical (e.g., slash rates, epoch lengths)
- How to propose parameter changes via governance

### 6. FAQ (`docs/FAQ.md`)
- What is Proof of Truth?
- How do I earn ZRN as a validator?
- What are the tier requirements?
- How does slashing work?
- Can I run a validator on a VPS?

## Constraints

- All documentation must be accurate for the current codebase
- Scripts must be idempotent and safe to re-run
- No hardcoded IPs — use variables/config files
- Join script must work on Ubuntu 22.04 and macOS
