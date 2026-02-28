# R26-6 — Investigate and Fix Tree Module Non-Determinism

## Context

From R25-5 testing (docs/research-bounties-report.md):

> The project was created successfully (tx code=0, event emitted) but querying on val0 (gRPC 9090) returns "project not found". Querying val1/val2/val3 returns the project correctly. All validators are at the same block height. This indicates a state divergence on val0 — a potential consensus/non-determinism bug.

This is a **consensus safety issue**. If validators have different state, the chain will eventually halt or fork. Must be investigated before testnet.

## Task

### 1. Reproduce the Bug

```bash
# Start localnet
scripts/localnet.sh start

# Create a project
$BINARY tx tree create-project "Test Project" "Description" --from val1 $TX_FLAGS
sleep 6

# Query on all validators
for port in 9090 9091 9092 9093; do
    echo "=== Port $port ==="
    $BINARY query tree project <project_id> --node tcp://localhost:$((26657 + (port - 9090) * 100)) --grpc-addr localhost:$port
done
```

Document whether the bug is reproducible and which validator(s) diverge.

### 2. Investigate Root Cause

Common causes of non-determinism in Cosmos SDK modules:

**a) Map iteration** — Go maps have non-deterministic iteration order. If the module iterates a map to build state, different validators may process entries in different orders.
```bash
grep -rn "range.*map\[" x/tree/ --include="*.go"
grep -rn "for.*range" x/tree/keeper/ --include="*.go" | head -20
```

**b) Floating point** — Any use of float32/float64 in state computation.
```bash
grep -rn "float32\|float64" x/tree/ --include="*.go"
```

**c) Time-dependent logic** — Using `time.Now()` instead of `ctx.BlockTime()`.
```bash
grep -rn "time\.Now" x/tree/ --include="*.go"
```

**d) Goroutines / concurrency** — Any goroutine usage in the keeper.
```bash
grep -rn "go func\|go " x/tree/keeper/ --include="*.go"
```

**e) Store key ordering** — If store keys are constructed differently across validators.
```bash
# Check key construction in x/tree/types/keys.go
cat x/tree/types/keys.go
```

**f) External dependencies** — Calls to external services, random number generation without deterministic seed.
```bash
grep -rn "rand\.\|crypto/rand\|os\." x/tree/keeper/ --include="*.go"
```

### 3. Check if It's a Query Bug (Not State Bug)

The issue might be a **query routing problem**, not actual state divergence:
- val0 might have a stale gRPC cache
- The query might hit a different store version
- The gRPC endpoint might be misconfigured

**To distinguish:**
```bash
# Check state via abci_query (bypasses gRPC layer)
# Compare app_hash across validators at the same height
for port in 26657 26757 26857 26957; do
    echo "=== Port $port ==="
    curl -s http://localhost:$port/status | jq '.result.sync_info.latest_block_height, .result.sync_info.latest_app_hash'
done
```

If `latest_app_hash` matches across all validators → it's a query bug, not state divergence.
If `latest_app_hash` differs → it's real non-determinism.

### 4. Fix

Depends on root cause:
- Map iteration → sort keys before iterating
- Float → replace with `math/big` or `sdk.Dec`
- time.Now → replace with `ctx.BlockTime()`
- Store key → fix key construction

### 5. Test

- Determinism test: run same tx sequence on two fresh chains, compare state hashes
- Regression test for the specific project creation scenario
- If map iteration was the cause, add a test that creates many items and verifies ordering

## Files to Investigate

- `x/tree/keeper/` — All keeper files, especially msg_server.go and any EndBlocker logic
- `x/tree/types/keys.go` — Store key construction
- `x/tree/module.go` — BeginBlocker/EndBlocker

## Success Criteria

- [ ] Root cause identified and documented
- [ ] Fix implemented (or confirmed as query bug, not state divergence)
- [ ] Bug no longer reproducible on localnet
- [ ] app_hash matches across all validators after fix
- [ ] Determinism test added
- [ ] All existing tests pass
