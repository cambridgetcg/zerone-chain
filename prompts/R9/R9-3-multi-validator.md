# R9-3 — 4-Validator Local Testnet

## Goal

Create a local testnet with 4 validators running PoT consensus. Verify tier progression,
delegation, slashing, and knowledge verification rounds work with multiple validators.

## Working Directory

`/Users/yuai/Desktop/Zerone`

## Reference

- `/Users/yuai/Desktop/legible_money/` — draft had multi-validator testnet in B22-5
- Cosmos SDK localnet patterns

## Deliverables

### 1. Local Testnet Script
Create `scripts/localnet.sh` that:
- Builds `zeroned`
- Creates 4 validator home directories (node0, node1, node2, node3)
- Initializes each with different monikers
- Generates keys for each validator
- Creates a shared genesis with all 4 validators' gentxs
- Configures persistent peers
- Starts all 4 nodes (background processes)
- Waits for block production
- Runs smoke tests
- Stops all nodes and cleans up

### 2. Multi-Validator Test Script
Create `scripts/localnet-test.sh` that runs against a running localnet:

#### Test Scenarios

1. **Block production** — all 4 validators sign blocks, verify with `zeroned query block`
2. **Validator registration** — each validator registers in x/staking with initial tier
3. **Delegation** — delegate from external accounts to validators, verify stake increases
4. **Tier progression** — after sufficient blocks/stake, validators progress to tier 2
5. **Knowledge round (PoT)** — submit claim → commit → reveal → verify verdict across validators
6. **Slashing** — stop one validator, verify it gets jailed after missing blocks
7. **Recovery** — restart jailed validator, unjail, verify it resumes signing
8. **Governance** — submit a parameter change proposal, all 4 vote, verify it passes

### 3. Docker Compose (Optional)
If time permits, create `docker-compose.yml` for reproducible multi-validator setup:
```yaml
services:
  node0:
    build: .
    command: zeroned start --home /data
    volumes: [./testnet/node0:/data]
    ports: ["26657:26657"]
  node1: ...
  node2: ...
  node3: ...
```

### 4. Configuration
Each node needs:
- Unique `config.toml` with persistent_peers pointing to other 3 nodes
- Shared genesis.json with all 4 gentxs
- Different P2P and RPC ports (26656/26657, 26666/26667, 26676/26677, 26686/26687)
- `minimum-gas-prices = "0.025uzrn"` in `app.toml`

### 5. Peer Configuration
```toml
# node0 config.toml
persistent_peers = "node1_id@127.0.0.1:26666,node2_id@127.0.0.1:26676,node3_id@127.0.0.1:26686"
```

## Implementation Notes

- Use `--home` flag to isolate each node's data directory
- Use different ports to avoid conflicts on single machine
- Genesis must have all 4 validators with sufficient stake
- Block time: ~2.5s (default CometBFT)
- Wait for at least 10 blocks before running tests
- PoT round test: need at least 3 validators to submit commitments for quorum

## Tests (Go)

Also create `tests/multivalidator/` with Go tests that connect to the running localnet
via gRPC and verify:

1. Validator set has 4 active validators
2. Blocks are signed by all 4
3. PoT round completes with multi-validator participation
4. Slashing reduces validator power

These can run as integration tests against a live testnet or be embedded in the script.

## Verification

```bash
# Start localnet
bash scripts/localnet.sh start

# Run tests
bash scripts/localnet-test.sh

# Stop
bash scripts/localnet.sh stop
```

## Constraints

- Must work on macOS (no Docker required for basic script)
- All 4 nodes must produce blocks within 30 seconds of start
- PoT round must complete (claim → commit → reveal → verdict)
- Script must be idempotent (clean start every time)
- Clean up all processes on exit (trap EXIT)
