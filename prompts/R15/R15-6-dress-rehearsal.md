# R15-6 — Testnet Dress Rehearsal

## Context

This is the go/no-go gate. Every component has been built, tested, and verified individually. This session runs the full pipeline as a single unbroken sequence — exactly as it would happen on launch day.

## Prerequisites

ALL R15-1 through R15-5 merged. All tests passing.

## Task

### Phase 1: Clean State

```bash
cd ~/Desktop/zerone
git checkout main && git pull
make clean
make build
./build/zeroned version
```

Verify: clean build from latest main.

### Phase 2: Genesis Ceremony

Run the full ceremony script:

```bash
export REHEARSAL_HOME=$(mktemp -d)/zerone-rehearsal

# Step 1: Initialize
bash scripts/genesis-ceremony.sh init \
  --chain-id zerone-dress-1 \
  --home $REHEARSAL_HOME

# Step 2: Add 4 validators
for i in 1 2 3 4; do
  bash scripts/genesis-ceremony.sh add-validator "validator-$i" \
    --home $REHEARSAL_HOME
done

# Step 3: Configure testnet genesis params
bash scripts/testnet-genesis.sh $REHEARSAL_HOME

# Step 4: Inject axioms
cd tools/axiom-loader && go build -o ../../build/axiom-loader .
../../build/axiom-loader validate --input ../../seeds.txt
../../build/axiom-loader inject \
  --input ../../seeds.txt \
  --genesis $REHEARSAL_HOME/config/genesis.json
cd ../..

# Step 5: Validate genesis
./build/zeroned genesis validate-genesis --home $REHEARSAL_HOME

# Step 6: Finalize
bash scripts/genesis-ceremony.sh finalize --home $REHEARSAL_HOME
```

If `genesis-ceremony.sh` subcommands differ, adapt accordingly. The point is to use the actual scripts, not manual `jq` patches.

### Phase 3: Configure Nodes

```bash
# Use configure-node.sh for each validator
for i in 1 2 3 4; do
  bash scripts/configure-node.sh \
    --mode validator \
    --home $REHEARSAL_HOME/validator-$i
done
```

### Phase 4: Start Chain

```bash
# Start all 4 validators
for i in 1 2 3 4; do
  ./build/zeroned start \
    --home $REHEARSAL_HOME/validator-$i \
    --log_level info &
done

# Wait for blocks
sleep 15

# Verify blocks producing
HEIGHT=$(curl -s http://localhost:26657/status | jq -r '.result.sync_info.latest_block_height')
echo "Current height: $HEIGHT"
```

### Phase 5: Produce 100 Blocks

```bash
# Wait for 100 blocks (~252 seconds at 2521ms block time, ~4 minutes)
# Or check periodically
while true; do
  H=$(curl -s http://localhost:26657/status | jq -r '.result.sync_info.latest_block_height')
  echo "Height: $H"
  [ "$H" -ge 100 ] && break
  sleep 10
done
echo "100 blocks produced ✅"
```

### Phase 6: Smoke Tests

```bash
# Node info
curl -s http://localhost:1317/cosmos/base/tendermint/v1beta1/node_info | jq '.default_node_info.network'

# Balances
ADDR=$(./build/zeroned keys show validator-1 -a --keyring-backend test --home $REHEARSAL_HOME/validator-1)
curl -s "http://localhost:1317/cosmos/bank/v1beta1/balances/$ADDR" | jq .

# Module params
curl -s http://localhost:1317/zerone/knowledge/v1/params | jq .
curl -s http://localhost:1317/zerone/gov/v1/params | jq .
curl -s http://localhost:1317/zerone/staking/v1/params | jq .

# Validator set
curl -s http://localhost:26657/validators | jq '.result.total'

# Axioms loaded
./build/zeroned query knowledge facts --limit 5 --home $REHEARSAL_HOME/validator-1
```

### Phase 7: Governance Transaction

Submit a parameter change LIP and vote it through:

```bash
# Submit LIP
./build/zeroned tx zerone-gov submit-lip \
  --title "Test Parameter Change" \
  --description "Dress rehearsal governance test" \
  --category parameter \
  --param-changes '[{"module":"knowledge","key":"min_verifiers","value":"5"}]' \
  --initial-stake 1000000uzrn \
  --from validator-1 --keyring-backend test \
  --home $REHEARSAL_HOME/validator-1 \
  --chain-id zerone-dress-1 --fees 100uzrn -y

# Wait for review period to advance (or use short test params)
# Vote from all 4 validators
for i in 1 2 3 4; do
  ./build/zeroned tx zerone-gov vote-lip \
    --lip-id 1 --vote yes \
    --from validator-$i --keyring-backend test \
    --home $REHEARSAL_HOME/validator-$i \
    --chain-id zerone-dress-1 --fees 100uzrn -y
done

# Wait for voting period to end + tally
# Verify param changed
curl -s http://localhost:1317/zerone/knowledge/v1/params | jq '.params.min_verifiers'
```

If exact CLI commands differ, adapt — the goal is to prove the governance pipeline works end-to-end.

### Phase 8: Bank Transfer

```bash
ADDR2=$(./build/zeroned keys show validator-2 -a --keyring-backend test --home $REHEARSAL_HOME/validator-2)

./build/zeroned tx bank send $ADDR $ADDR2 1000000uzrn \
  --chain-id zerone-dress-1 --keyring-backend test \
  --home $REHEARSAL_HOME/validator-1 --fees 100uzrn -y

# Verify
curl -s "http://localhost:1317/cosmos/bank/v1beta1/balances/$ADDR2" | jq .
```

### Phase 9: Graceful Shutdown

```bash
# Kill all validators
pkill -f "zeroned start" || true
sleep 5
echo "All nodes stopped"
```

### Phase 10: Full Test Suite

```bash
go test ./... -count=1
go vet ./...
make pr-check
```

### Phase 11: Final Report

```markdown
## Testnet Dress Rehearsal Report

### Pipeline
| Step | Status | Notes |
|------|--------|-------|
| Clean build | ✅/❌ | |
| Genesis ceremony | ✅/❌ | |
| Axiom injection (777) | ✅/❌ | |
| Genesis validation | ✅/❌ | |
| Node configuration (4x) | ✅/❌ | |
| Chain start (4 validators) | ✅/❌ | |
| Block production | ✅/❌ | Height reached: N |
| 100 blocks | ✅/❌ | Time: Ns |
| REST API | ✅/❌ | |
| Module params queryable | ✅/❌ | |
| Validator set correct | ✅/❌ | Count: N |
| Axioms in knowledge | ✅/❌ | |
| LIP submitted | ✅/❌ | |
| LIP voted | ✅/❌ | |
| LIP passed | ✅/❌ | |
| Param change applied | ✅/❌ | |
| Bank transfer | ✅/❌ | |
| Graceful shutdown | ✅/❌ | |
| `go test ./...` | ✅/❌ | N tests |
| `make pr-check` | ✅/❌ | |

### Test Coverage
- Total test functions: N
- Passing: N
- Failing: N

### Verdict
**READY FOR TESTNET** / **BLOCKED** (reason)
```

## Cleanup

```bash
rm -rf $REHEARSAL_HOME
```

## Commit Convention

```
test(R15-6): testnet dress rehearsal — full pipeline verification
docs(R15-6): dress rehearsal report
```
