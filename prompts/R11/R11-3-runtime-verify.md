# R11-3 — Runtime Verification & Test Suite Green

## Context

After R11-1 and R11-2 fix the proto registration issues, this session verifies the full chain binary works end-to-end and all tests pass.

## Prerequisites

- R11-1 (gov proto fix) must be committed
- R11-2 (bvm proto fix) must be committed

## Task

### 1. Clean build

```bash
make clean
make build
```

### 2. Binary runtime checks

```bash
# Must not panic
./build/zeroned version

# Must succeed
./build/zeroned init test-node --chain-id zerone-testnet-1 --home /tmp/zerone-test

# Must start (even if it fails on missing genesis validators, it should not panic)
./build/zeroned start --home /tmp/zerone-test 2>&1 | head -50
```

### 3. Full test suite

```bash
go test ./... 2>&1
```

Fix any test failures. Common issues after proto migration:
- Field name changes (proto-generated CamelCase may differ from hand-written)
- Missing `Marshal`/`Unmarshal` methods expected by tests
- gRPC handler signature mismatches
- Codec registration order issues

### 4. Audit for remaining hand-written ProtoMessage stubs

```bash
# Should return only legitimate data types (not Msg types) and .pb.go files
grep -rn "ProtoMessage()" x/*/types/*.go | grep -v ".pb.go"
```

The only acceptable result is `x/ontology/types/types.go` with `IncompletenessAcknowledgment` (data type, not a transaction message).

If any Msg-type hand-written `ProtoMessage()` stubs remain, add them to the fix list.

### 5. Proto regeneration check

```bash
# Regenerate and verify no diff (proto matches generated code)
make proto-gen
git diff --stat
```

If there's a diff, the proto files and generated code are out of sync — fix.

### 6. Final commit

```bash
git add -A
git commit -m "R11: stabilise proto registration — runtime panic fixed, all tests green"
```

## Exit Criteria

- [ ] `zeroned version` prints version without panic
- [ ] `zeroned init` succeeds
- [ ] `go test ./...` — zero failures
- [ ] No hand-written Msg `ProtoMessage()` stubs outside .pb.go
- [ ] `make proto-gen` produces no diff
- [ ] Clean commit on main
