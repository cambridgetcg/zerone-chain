# R13-6 — End-to-End Verification: Ceremony → Axioms → Validate → Start → Blocks

## Context

R13-1 through R13-5 built:
1. Axiom loader tool (validate, inject, stats)
2. Production genesis ceremony script
3. Testnet genesis with all params explicitly configured
4. Node configuration with mode presets
5. Per-module genesis validation tests + block production

This session verifies the complete pipeline works end-to-end.

## Task

### 1. Build verification

```bash
cd /Users/yuai/Desktop/Zerone

# Clean build
go build ./...

# Binary works
./build/zeroned version
```

### 2. Axiom pipeline

```bash
# Validate the 777 axioms
go run tools/axiom-loader/main.go validate x/knowledge/types/genesis_axioms.json

# Print statistics
go run tools/axiom-loader/main.go stats x/knowledge/types/genesis_axioms.json

# Verify expected output:
#   - 777 axioms loaded
#   - 15 domains
#   - 0 cycles
#   - DAG validation passed
```

### 3. Genesis ceremony flow

```bash
# Run full ceremony (single validator for test)
./scripts/genesis-ceremony.sh init
./scripts/genesis-ceremony.sh add-validator test-val-1
./scripts/genesis-ceremony.sh finalize
./scripts/genesis-ceremony.sh export

# Verify genesis file
./build/zeroned validate-genesis 2>/dev/null || \
  ./build/zeroned validate --home $HOME/.zeroned/genesis-ceremony

# Check critical params in exported genesis.json:
jq '.consensus.params.abci.vote_extensions_enable_height' genesis.json
# Expected: "1"

jq '.app_state.knowledge.params.min_verifiers' genesis.json
# Expected: >= 1

jq '.app_state.bank.denom_metadata[0].symbol' genesis.json
# Expected: "ZRN"
```

### 4. Testnet genesis flow

```bash
# Run testnet genesis
./scripts/testnet-genesis.sh init
./scripts/testnet-genesis.sh add-validator testnet-val-1
./scripts/testnet-genesis.sh finalize

# Verify ALL modules have params set (not just defaults)
GENESIS=$HOME/.zeroned/testnet/config/genesis.json

# Spot-check critical modules:
echo "=== knowledge ==="
jq '.app_state.knowledge.params | keys | length' "$GENESIS"
# Expected: 27+ params

echo "=== zerone_staking ==="
jq '.app_state.zerone_staking.params.tier_configs | length' "$GENESIS"
# Expected: 4 tiers

echo "=== zerone_gov ==="
jq '.app_state.zerone_gov.params.voting_period_blocks' "$GENESIS"
# Expected: non-zero

echo "=== vesting_rewards ==="
jq '.app_state.vesting_rewards.category_configs | length' "$GENESIS"
# Expected: 10 categories

echo "=== disputes ==="
jq '.app_state.disputes.params.tier_configs | length' "$GENESIS"
# Expected: 3 tiers

echo "=== emergency ==="
jq '.app_state.emergency.params.halt_quorum' "$GENESIS"
# Expected: non-zero
```

### 5. Node configuration modes

```bash
# Test all 4 modes
TEMP_HOME=$(mktemp -d)
./build/zeroned init test-node --chain-id test-1 --home "$TEMP_HOME" 2>/dev/null

for mode in validator fullnode seed archive; do
    echo "=== Mode: $mode ==="
    ./scripts/configure-node.sh --home "$TEMP_HOME" --mode $mode --moniker "test-$mode"
    echo "  Pruning: $(grep '^pruning = ' "$TEMP_HOME/config/app.toml")"
    echo "  Seed mode: $(grep '^seed_mode = ' "$TEMP_HOME/config/config.toml")"
    echo "  Min gas: $(grep '^minimum-gas-prices = ' "$TEMP_HOME/config/app.toml")"
done

rm -rf "$TEMP_HOME"
```

### 6. Full test suite

```bash
# All tests
go test ./... -count=1 -timeout 300s

# Specifically genesis tests
go test ./tests/cross_stack/... -run "Genesis|Module|Block|Axiom" -v -count=1 -timeout 120s

# Axiom loader tests
go test ./tools/axiom-loader/... -v
```

### 7. Block production from ceremony genesis

```bash
# Start a node from the ceremony genesis
CEREMONY_HOME=$HOME/.zeroned/genesis-ceremony

# Ensure vote extensions are enabled and min_verifiers allows single validator
jq '.app_state.knowledge.params.min_verifiers = 1 | 
    .consensus.params.abci.vote_extensions_enable_height = "1"' \
    "$CEREMONY_HOME/config/genesis.json" > /tmp/genesis-patched.json
mv /tmp/genesis-patched.json "$CEREMONY_HOME/config/genesis.json"

# Start and check block production
./build/zeroned start --home "$CEREMONY_HOME" --minimum-gas-prices 0uzrn &
NODE_PID=$!
sleep 15

# Check block height
HEIGHT=$(curl -s localhost:26657/status 2>/dev/null | jq -r '.result.sync_info.latest_block_height' 2>/dev/null || echo "0")
echo "Block height: $HEIGHT"

if [ "$HEIGHT" -ge 3 ]; then
    echo "✅ Chain is producing blocks"
else
    echo "❌ Chain is NOT producing blocks after 15s"
fi

kill $NODE_PID 2>/dev/null
wait $NODE_PID 2>/dev/null
```

### 8. Config reference validation

```bash
# Verify the testnet-genesis-config.json is valid JSON
python3 -c "import json; d=json.load(open('scripts/testnet-genesis-config.json')); print(f'Modules configured: {len(d[\"modules\"])}')"
# Expected: 30+ modules
```

## Exit Criteria Checklist

- [ ] `go build ./...` — clean
- [ ] `go test ./...` — all pass
- [ ] `tools/axiom-loader validate` — 777 axioms, DAG valid
- [ ] `tools/axiom-loader inject` — axioms embedded in genesis
- [ ] `genesis-ceremony.sh` full flow — init→add-validator→finalize→export works
- [ ] `testnet-genesis.sh` — all 30+ modules have explicit params
- [ ] `testnet-genesis-config.json` — valid JSON, 30+ modules documented
- [ ] `configure-node.sh` — 4 modes work (validator/fullnode/seed/archive)
- [ ] Per-module DefaultGenesis+Validate — 30/30 pass
- [ ] Keeper-level InitGenesis→ExportGenesis — knowledge, staking, gov, vesting_rewards
- [ ] 100-block production test passes
- [ ] Chain boots from ceremony genesis and produces blocks
- [ ] No new regressions in existing tests

## Troubleshooting

**Binary panics on start:**
Check that vote_extensions_enable_height is set. Without this, PoT modules will fail.

**Genesis validation fails:**
Run module-by-module to isolate: `go test ./tests/cross_stack/... -run TestPerModuleGenesisValidation -v`

**Axiom loader can't find types:**
Ensure `tools/axiom-loader/main.go` imports `github.com/zerone-chain/zerone/x/knowledge/types`. Run `go mod tidy` if needed.

**jq patches fail:**
Verify module keys match: `zerone_auth` (not `auth`), `zerone_staking` (not `staking`), `zerone_gov` (not `gov`).
