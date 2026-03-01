# R30-1 — Proto-Go Consistency Audit & Enforcement

## Problem

Multiple R28 and R29 sessions added new params and message types to `.proto` files but either:
1. Didn't run `make proto-gen` afterward, leaving stale binary file descriptors (rawDesc) in `*.pb.go` files
2. Added fields to Go structs without adding them to the `.proto` file at all
3. Created hand-rolled gRPC services (`query_ext.go`) instead of adding RPCs to the proto Query service

The Cosmos SDK's `protojson` marshaler uses the embedded file descriptor — not Go struct tags — to serialize/deserialize. When the rawDesc is stale, new fields are silently dropped during JSON round-trips. This caused:
- Genesis validation failures (new params lost during `MarshalJSON` → `UnmarshalJSON`)
- App init panics (`cannot find method descriptor` for hand-rolled gRPC services)

These are silent, hard-to-debug failures that only surface in integration tests or on-chain.

## Objective

1. Audit every module for proto-Go mismatches
2. Fix all mismatches
3. Add a CI-enforceable check that prevents future drift

## Audit Procedure

For each module in `x/`:

### Step 1: Compare proto fields vs Go struct fields

```bash
# List all proto message fields for a module
grep -E "^\s+(uint64|string|bool|int64|repeated|bytes)" proto/zerone/<module>/v1/genesis.proto
grep -E "^\s+(uint64|string|bool|int64|repeated|bytes)" proto/zerone/<module>/v1/types.proto
grep -E "^\s+rpc " proto/zerone/<module>/v1/query.proto

# List all Go struct fields in the generated pb.go
grep -E "^\s+\w+\s+\w+\s+\`protobuf:" x/<module>/types/genesis.pb.go | wc -l
grep -E "^\s+\w+\s+\w+\s+\`protobuf:" x/<module>/types/types.pb.go | wc -l
```

If the Go struct has fields NOT in the proto (added manually to pb.go), those fields will be wiped on next `make proto-gen`.

If the proto has fields NOT reflected in the rawDesc (stale generation), `protojson` will drop them.

### Step 2: Check for hand-rolled gRPC services

```bash
# Find any query_ext.go or manually registered services
find x/ -name "query_ext.go" -path "*/types/*"
grep -rn "RegisterQueryExtServer\|QueryExtServer" x/*/module.go
```

These MUST be migrated to the proto Query service. The Cosmos SDK `GRPCQueryRouter` requires proto-registered service descriptors.

### Step 3: Check DefaultParams round-trip

For each module with params, verify the JSON round-trip preserves all fields:

```go
// Pseudo-test for each module:
cdc := codec.NewProtoCodec(ir)
gs := types.DefaultGenesis()
bz, _ := cdc.MarshalJSON(gs)
var gs2 types.GenesisState
cdc.UnmarshalJSON(bz, &gs2)
// Assert gs.Params == gs2.Params for ALL fields
```

### Step 4: Regenerate and diff

```bash
make proto-gen
git diff -- '*.pb.go' '*.pb.gw.go'
```

If there's any diff, the generated code was stale.

## Modules to Audit

Priority order (most likely to have drift based on recent changes):

| Module | R28/R29 Changes | Risk |
|--------|----------------|------|
| knowledge | R28-1 through R28-5, R29-1, R29-2, R29-3 | **High** — most new params |
| alignment | R28-7, R29-4, R29-6 | **High** — correction confidence params were missing from proto |
| capture_defense | R28-8, R29-5 | **Medium** — query_ext already fixed |
| capture_challenge | R28-8, R29-5 | **Medium** — query_ext already fixed |
| partnerships | R28-5, R28-6, R29-5 | **Medium** — structural immunity params |
| discovery | R29-6 | **Low** — only pacing interface added |
| ontology | R28-3 | **Low** |
| qualification | R28-3 | **Low** |
| autopoiesis | Unchanged | **Low** |

## Implementation

### Task 1: Full Audit Script

Create `scripts/proto-audit.sh`:

```bash
#!/bin/bash
# Compares proto definitions against generated Go code
# Exit 1 if any mismatch found

set -e

echo "=== Step 1: Regenerating proto ==="
make proto-gen

echo "=== Step 2: Checking for drift ==="
if ! git diff --quiet -- '*.pb.go' '*.pb.gw.go'; then
    echo "ERROR: Proto-generated files are stale. Diff:"
    git diff --stat -- '*.pb.go' '*.pb.gw.go'
    exit 1
fi

echo "=== Step 3: Checking for hand-rolled gRPC services ==="
EXT_FILES=$(find x/ -name "query_ext.go" -path "*/types/*" 2>/dev/null)
if [ -n "$EXT_FILES" ]; then
    echo "ERROR: Hand-rolled gRPC services found (must migrate to proto):"
    echo "$EXT_FILES"
    exit 1
fi

echo "=== Step 4: Checking for RegisterQueryExtServer ==="
EXT_REGS=$(grep -rn "RegisterQueryExtServer" x/*/module.go 2>/dev/null || true)
if [ -n "$EXT_REGS" ]; then
    echo "ERROR: QueryExt registrations found:"
    echo "$EXT_REGS"
    exit 1
fi

echo "✅ All proto files consistent"
```

### Task 2: Genesis Round-Trip Test

Create `tests/cross_stack/proto_consistency_test.go`:

```go
func TestProtoJSONRoundTrip_AllModuleParams(t *testing.T) {
    app := newTestApp(t, testChainID)
    cdc := app.AppCodec()

    genState := app.DefaultGenesis()

    for moduleName, rawGenesis := range genState {
        // Marshal → Unmarshal → Marshal again
        // The two marshal outputs must be identical
        var intermediate json.RawMessage
        err := json.Unmarshal(rawGenesis, &intermediate)
        require.NoError(t, err, "module %s: first unmarshal", moduleName)

        bz2, err := json.Marshal(intermediate)
        require.NoError(t, err, "module %s: re-marshal", moduleName)

        // Re-validate after round-trip
        // This catches the exact bug: fields dropped by protojson
    }

    // Full validation after round-trip
    err := zeroneapp.ModuleBasics.ValidateGenesis(cdc, app.TxConfig(), genState)
    require.NoError(t, err, "genesis must validate after round-trip")
}
```

### Task 3: Fix Any Remaining Mismatches

For each module that fails audit:

1. Add missing fields to the `.proto` file
2. Run `make proto-gen`
3. Verify `go build ./...`
4. Verify `go test ./...`
5. Commit proto + generated code together

### Task 4: Add to Makefile

```makefile
proto-check:
	@bash scripts/proto-audit.sh

pr-check: lint test proto-check
```

### Task 5: Document the Rule

Add to the project's contributing guide or AGENTS.md equivalent:

> **Proto-Go Consistency Rule:**
> When adding new fields to any module's Params, state types, or query messages:
> 1. Add the field to the `.proto` file FIRST
> 2. Run `make proto-gen`
> 3. Reference the generated type in your Go code
> 4. NEVER add fields directly to `*.pb.go` — they will be overwritten
> 5. NEVER create `query_ext.go` for new query RPCs — add them to `query.proto`
> 6. Run `scripts/proto-audit.sh` before committing

## Tests

1. **proto-audit.sh passes** on current main with zero drift
2. **Genesis round-trip test** passes for all 32+ modules
3. **No query_ext.go files** remain in the codebase
4. **No RegisterQueryExtServer calls** remain in any module.go
5. **Intentional drift test:** Add a field to a proto, DON'T regen, verify proto-audit.sh catches it

## Success Criteria

After this session:
- `make proto-check` passes
- `make pr-check` includes proto consistency
- Every module's DefaultGenesis survives a `protojson` round-trip with all params intact
- Future sessions that touch proto files will be caught by CI if they forget to regen
