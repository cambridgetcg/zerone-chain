# R13-4 — Node Configuration Script with Mode Presets

## Context

External validators joining the Zerone network need standardised configuration for their nodes. The prototype had `scripts/configure-node.sh` with 4 mode presets: `validator`, `fullnode`, `seed`, `archive`. Zerone has config templates (`config/config.toml.template`, `config/app.toml.template`) but no automated configuration script.

## Task

### Create `scripts/configure-node.sh`

A bash script that applies recommended production settings to `config.toml` and `app.toml` based on the node's intended role.

#### Usage

```bash
./scripts/configure-node.sh [options]

Options:
  --home <dir>           Node home directory (default: $HOME/.zeroned)
  --mode <mode>          Preset: validator|fullnode|seed|archive (default: validator)
  --gas-prices <prices>  Minimum gas prices (default: 0.025uzrn)
  --enable-api           Enable REST API on port 1317
  --enable-grpc          Enable gRPC on port 9090
  --prometheus           Enable Prometheus metrics on port 26660
  --external-address     External P2P address (ip:port)
  --moniker <name>       Set node moniker
```

#### Mode Presets

**`validator`** (default) — block production node
- Pruning: `default` (keeps recent 362,880 states)
- API/gRPC: **off** (reduce attack surface)
- Prometheus: **on** (monitoring)
- Seed mode: off
- Tx indexer: `kv` (default)
- Block time: `timeout_commit = "2521ms"`, `timeout_propose = "2000ms"`
- Min gas prices: `0.025uzrn`
- State sync: off (validators should have full history)
- Mempool: default

**`fullnode`** — query-serving node
- Pruning: `default`
- API: **on** (serve queries)
- gRPC: **on** (serve queries)
- Prometheus: **on**
- Seed mode: off
- CORS: enabled (for web frontends)
- Min gas prices: `0.025uzrn`

**`seed`** — peer exchange only
- Pruning: `everything` (keep almost nothing)
- API/gRPC: **off**
- Prometheus: on
- Seed mode: **on** (CometBFT seed mode)
- Tx indexer: `null` (don't index)
- Pex: on, addr book strict: off
- Crawl peers aggressively

**`archive`** — full historical state
- Pruning: `nothing` (keep everything)
- API: **on**
- gRPC: **on**
- Prometheus: on
- Snapshot interval: every 1000 blocks
- Tx indexer: `kv`
- State sync snapshots for other nodes to catch up from

#### Common Settings (all modes)

```toml
# config.toml
timeout_commit = "2521ms"
timeout_propose = "2000ms"
create_empty_blocks = true
create_empty_blocks_interval = "0s"

# app.toml
minimum-gas-prices = "0.025uzrn"
```

#### Platform Detection

Support both macOS and Linux `sed` syntax:

```bash
sedi() {
  if [[ "$(uname -s)" == "Darwin" ]]; then
    sed -i '' "$@"
  else
    sed -i "$@"
  fi
}
```

#### Safety

- Check that `$HOME/config/config.toml` and `app.toml` exist before modifying
- Create backups: `config.toml.backup`, `app.toml.backup`
- Print a summary of changes applied
- Warn if moniker is still "my-zerone-node" or default

#### Output

Print a summary:

```
═══ Zerone Node Configuration ═══

  Mode:      validator
  Home:      /home/user/.zeroned
  Moniker:   my-validator
  
  Changes Applied:
    ✓ Pruning:         default
    ✓ API:             disabled
    ✓ gRPC:            disabled
    ✓ Prometheus:      enabled
    ✓ Min gas prices:  0.025uzrn
    ✓ Block time:      2521ms commit, 2000ms propose
    
  Backups:
    config.toml → config.toml.backup
    app.toml    → app.toml.backup

  Start with:
    zeroned start --home /home/user/.zeroned
```

### Update `scripts/join-testnet.sh`

Add a call to `configure-node.sh` in the join flow:

```bash
# After init and genesis fetch, apply recommended config
if [ -f "${PROJECT_ROOT}/scripts/configure-node.sh" ]; then
    info "Applying recommended node configuration..."
    bash "${PROJECT_ROOT}/scripts/configure-node.sh" \
        --home "${ZERONED_HOME}" \
        --mode validator \
        --moniker "${MONIKER}"
fi
```

## Reference

- Prototype: `legible_money/scripts/configure-node.sh`
- Zerone config templates: `config/config.toml.template`, `config/app.toml.template`
- Zerone join script: `scripts/join-testnet.sh`
- Zerone localnet config: `scripts/localnet.sh` (networking patches reference)
- CometBFT config docs: https://docs.cometbft.com/v0.38/core/configuration

## Verification

```bash
# Initialize a test node
zeroned init test-node --chain-id zerone-testnet-1 --home /tmp/test-node

# Apply each mode and verify
for mode in validator fullnode seed archive; do
    echo "=== Testing mode: $mode ==="
    ./scripts/configure-node.sh --home /tmp/test-node --mode $mode
    grep "pruning" /tmp/test-node/config/app.toml
    grep "seed_mode" /tmp/test-node/config/config.toml
    echo ""
done
```
