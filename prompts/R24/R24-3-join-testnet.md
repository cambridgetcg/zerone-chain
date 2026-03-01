# R24-3 — Join Testnet: External Node Joining a Running Network

## Context

`scripts/join-testnet.sh` (352 lines) configures a node to join a testnet with Cosmovisor and systemd support. `docs/VALIDATOR-GUIDE.md` (481 lines) describes the full process. Neither has been tested against a running network.

This session runs the join script against the localnet as if it were a public testnet — simulating an external operator following the docs.

## Prerequisites

- Localnet running (`scripts/localnet.sh start`)
- R24-1 complete (know the identity flow)

## Task

### 1. Simulate External Node Environment

Create a completely separate environment (different directory, no access to localnet coordinator state):

```bash
export EXT_HOME=$(mktemp -d)/zeroned-external
mkdir -p "$EXT_HOME"
echo "External node home: $EXT_HOME"
```

### 2. Try the Automated Path First

```bash
# Get the genesis from the running localnet (simulating downloading it)
cp ~/.zeroned/localnet/coordinator/config/genesis.json /tmp/localnet-genesis.json

# Get peer addresses (simulating what a testnet page would list)
# val0's node ID and address
NODE_ID=$($BINARY tendermint show-node-id --home ~/.zeroned/localnet/val0)
PEER="${NODE_ID}@127.0.0.1:26600"
echo "Seed peer: $PEER"

# Run join-testnet with the localnet as target
scripts/join-testnet.sh \
    --moniker "external-node" \
    --home "$EXT_HOME" \
    --genesis /tmp/localnet-genesis.json \
    --reset 2>&1 | tee /tmp/join-testnet.log
```

**Verify:**
- [ ] Script runs without errors
- [ ] Node initialised with correct chain-id
- [ ] Genesis file installed
- [ ] Config applied (gas prices, pruning, etc.)

**Issues to look for:**
- Does the script expect a URL for genesis or accept a local path?
- Does it configure seeds/persistent peers? From where?
- Does it handle the case where the network is already running?

### 3. Configure Peers Manually (if script doesn't)

```bash
# Edit config.toml to add the localnet as peers
PEERS=""
for i in 0 1 2 3; do
    NID=$($BINARY tendermint show-node-id --home ~/.zeroned/localnet/val$i)
    PORT=$((26600 + i))
    PEERS="${PEERS}${NID}@127.0.0.1:${PORT},"
done
PEERS=${PEERS%,}

# Use configure-node.sh
scripts/configure-node.sh \
    --home "$EXT_HOME" \
    --mode fullnode \
    --enable-api \
    --moniker "external-node"

# Manually set persistent_peers
sed -i '' "s/^persistent_peers = .*/persistent_peers = \"${PEERS}\"/" "$EXT_HOME/config/config.toml"
```

### 4. Start External Node

```bash
$BINARY start --home "$EXT_HOME" --log_level info 2>&1 | tee /tmp/external-node.log &
EXT_PID=$!
sleep 15
```

**Verify:**
- [ ] Node starts without panics
- [ ] Connects to localnet peers
- [ ] Begins syncing blocks (height increasing)
- [ ] Catches up to the current localnet height
- [ ] State hash matches (no app hash mismatch)

**Issues to look for:**
- Port conflicts with localnet (P2P, RPC, gRPC, REST all need different ports)
- Does the external node need different ports configured?
- How long does initial sync take? (localnet is small, should be fast)
- Any genesis-related issues (missing module states, param mismatches)?

### 5. Query from External Node

```bash
EXT_RPC="--node http://127.0.0.1:26657"  # adjust port
$BINARY query bank balances $VAL0_ADDR $EXT_RPC --output json
$BINARY query zerone-staking validators $EXT_RPC --output json | jq '.validators | length'
$BINARY query knowledge facts --limit 5 $EXT_RPC --output json
```

**Verify:**
- [ ] External node serves queries
- [ ] Data matches what localnet returns
- [ ] REST API accessible (if enabled)

### 6. Promote External Node to Validator

```bash
# Register the external node as a validator
$BINARY keys add ext-validator --keyring-backend test --home "$EXT_HOME"
EXT_VAL=$($BINARY keys show ext-validator -a --keyring-backend test --home "$EXT_HOME")

# Fund from localnet
$BINARY tx bank send $VAL0_ADDR $EXT_VAL 500000000000uzrn \
    --from val0 $TX_FLAGS
sleep 3

# Get consensus pubkey from the external node
EXT_PUBKEY=$($BINARY tendermint show-validator --home "$EXT_HOME")

# Register as validator
$BINARY tx zerone-staking register-validator \
    --moniker "external-validator" \
    --consensus-pubkey "$EXT_PUBKEY" \
    --self-delegation 100000000000uzrn \
    --commission-bps 500 \
    --from ext-validator \
    --keyring-backend test --home "$EXT_HOME" \
    --node http://127.0.0.1:26601 \
    --chain-id zerone-localnet \
    --gas auto --gas-adjustment 1.5 --gas-prices 0.025uzrn --yes
```

**Verify:**
- [ ] Validator registered from external node
- [ ] External node begins participating in consensus (signs blocks)
- [ ] PoT rounds include the new validator
- [ ] 5-validator set works (was 4)

**Issues to look for:**
- Consensus pubkey format from `tendermint show-validator` vs what register-validator expects
- Does the external validator need to wait before participating?
- Any CometBFT consensus issues with 5 validators vs the genesis 4?

### 7. Test Cosmovisor Setup (Optional)

```bash
scripts/join-testnet.sh \
    --moniker "cosmovisor-node" \
    --home "$EXT_HOME-cv" \
    --genesis /tmp/localnet-genesis.json \
    --cosmovisor \
    --reset
```

**Verify:**
- [ ] Cosmovisor directory structure created
- [ ] Binary placed in `cosmovisor/genesis/bin/`
- [ ] Cosmovisor can start and manage the node

### 8. Validate the VALIDATOR-GUIDE

Read through `docs/VALIDATOR-GUIDE.md` step by step. For each instruction:
- Does the command work as written?
- Are the flags correct?
- Are there missing steps?
- Is the ordering correct?

**Document every discrepancy.**

## Report Template

```markdown
### Step N: <name>
**Status:** PASS / FAIL / BLOCKED
**Time:** <how long it took>
**Doc Match:** <does VALIDATOR-GUIDE describe this correctly?>
**Issue:** <if any>
**Fix:** <suggested fix to docs or script>
```

## Exit Criteria

1. External node joins running localnet and syncs
2. External node serves queries matching localnet state
3. External node promoted to validator and signs blocks
4. join-testnet.sh tested (or fixes documented)
5. VALIDATOR-GUIDE accuracy validated
6. Every documentation discrepancy recorded
7. Report written to `docs/join-testnet-report.md`

## Cleanup

```bash
kill $EXT_PID 2>/dev/null
rm -rf "$EXT_HOME" "$EXT_HOME-cv"
```

## Commit Convention

```
test(infra): external node join + validator promotion e2e
docs(validator): fix VALIDATOR-GUIDE discrepancies found in testing
fix(scripts): join-testnet.sh fixes from testing
```
