# R14-6 — Boot Verification: End-to-End Chain Launch

## Context

After R14-1 through R14-5, zerone should have:
- A working `zeroned` binary
- ≥ 2,800 tests passing
- Complete genesis pipeline (from R13)
- Full module wiring (from R1-R12)

This session runs the full verification: build → init → configure → start → produce blocks → smoke test.

## Prerequisites

- R14-1 merged (binary exists)
- R14-2/3/4/5 merged (test coverage)
- All R13 scripts available

## Task

### Step 1: Clean Build

```bash
make clean  # if exists, otherwise rm -rf build/
make build
./build/zeroned version
```

Verify: binary exists, version prints correctly.

### Step 2: Initialize Chain

```bash
export ZERONED_HOME=$(mktemp -d)
./build/zeroned init boot-verify --chain-id zerone-boot-1 --home $ZERONED_HOME
```

Verify: genesis.json created, node key generated.

### Step 3: Configure Genesis

Use the testnet genesis script to set all params:

```bash
bash scripts/testnet-genesis.sh $ZERONED_HOME
```

Or manually if the script requires the binary in PATH:

```bash
# Add validator
./build/zeroned keys add validator --keyring-backend test --home $ZERONED_HOME
ADDR=$(./build/zeroned keys show validator -a --keyring-backend test --home $ZERONED_HOME)

# Fund validator
./build/zeroned genesis add-genesis-account $ADDR 1000000000000uzrn --home $ZERONED_HOME

# Create gentx
./build/zeroned genesis gentx validator 100000000000uzrn --chain-id zerone-boot-1 --keyring-backend test --home $ZERONED_HOME

# Collect gentxs
./build/zeroned genesis collect-gentxs --home $ZERONED_HOME

# Validate genesis
./build/zeroned genesis validate-genesis --home $ZERONED_HOME
```

### Step 4: Inject Axioms (if axiom-loader is built)

```bash
cd tools/axiom-loader && go build -o ../../build/axiom-loader .
../../build/axiom-loader validate --input ../../seeds.txt
../../build/axiom-loader inject --input ../../seeds.txt --genesis $ZERONED_HOME/config/genesis.json
```

### Step 5: Start Chain

```bash
./build/zeroned start --home $ZERONED_HOME &
CHAIN_PID=$!
sleep 10  # wait for first blocks
```

Verify: blocks producing (check logs for "committed state" or "indexed block").

### Step 6: Smoke Tests

```bash
# Check node status
curl -s http://localhost:26657/status | jq '.result.sync_info.latest_block_height'

# Check it's producing blocks (height > 1)
HEIGHT=$(curl -s http://localhost:26657/status | jq -r '.result.sync_info.latest_block_height')
[ "$HEIGHT" -gt 1 ] && echo "PASS: blocks producing (height=$HEIGHT)" || echo "FAIL: no blocks"

# Check REST API
curl -s http://localhost:1317/cosmos/base/tendermint/v1beta1/node_info | jq '.default_node_info.network'

# Check module params queryable
curl -s http://localhost:1317/zerone/knowledge/v1/params | jq .

# Check balances
curl -s "http://localhost:1317/cosmos/bank/v1beta1/balances/$ADDR" | jq .
```

### Step 7: Transaction Test

```bash
# If possible, submit a basic tx
./build/zeroned tx bank send validator $ADDR 1000uzrn \
  --chain-id zerone-boot-1 --keyring-backend test --home $ZERONED_HOME \
  --fees 100uzrn -y
```

### Step 8: Graceful Shutdown

```bash
kill $CHAIN_PID
wait $CHAIN_PID
echo "Chain shutdown cleanly"
```

### Step 9: Full Test Suite

```bash
go test ./... -count=1
go vet ./...
make pr-check  # lint + test + build
```

### Step 10: Report

Generate a summary:

```markdown
## R14 Boot Verification Report

| Check | Status |
|-------|--------|
| `make build` | ✅/❌ |
| `zeroned version` | ✅/❌ |
| `zeroned init` | ✅/❌ |
| Genesis validation | ✅/❌ |
| Axiom injection | ✅/❌ |
| Chain start | ✅/❌ |
| Block production | ✅/❌ (height: N) |
| REST API | ✅/❌ |
| Module queries | ✅/❌ |
| Transaction | ✅/❌ |
| Graceful shutdown | ✅/❌ |
| `go test ./...` | ✅/❌ (N tests) |
| `make pr-check` | ✅/❌ |

### Test Coverage Summary
- Total tests: N
- Passing: N
- Failing: N
- Modules with 0 tests: [list]
```

## Cleanup

```bash
rm -rf $ZERONED_HOME
```

## Commit Convention

```
test(R14-6): boot verification — chain start + smoke test + report
```
