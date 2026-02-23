# R13-1 — Port Axiom Loader Tool (Validate, Inject, Stats)

## Context

Zerone has 777 genesis axioms in `x/knowledge/types/genesis_axioms.json` and a basic `scripts/convert_axioms.go` parser that converts markdown → JSON. But there's no tool to:

- **Validate** the axiom DAG (detect cycles, missing dependencies, duplicate IDs)
- **Inject** axioms as pre-verified Facts into a genesis.json file
- **Analyze** the knowledge graph (statistics, edge export)

The prototype had `tools/axiom-loader` with subcommands: `validate`, `inject`, `stats`, `edges`.

## Task

### 1. Create `tools/axiom-loader/main.go`

Port from the prototype at `legible_money/tools/axiom-loader/main.go`, adapting for Zerone's module path and types.

Subcommands:

```
go run tools/axiom-loader/main.go validate <axioms.json>
go run tools/axiom-loader/main.go inject   <axioms.json> <genesis.json>
go run tools/axiom-loader/main.go stats    <axioms.json>
go run tools/axiom-loader/main.go edges    <axioms.json> [-o output.csv]
```

### 2. `validate` subcommand

Read axioms JSON and verify:

- **No duplicate IDs** — each `axiom_id` is unique
- **No missing dependencies** — every entry in `dependencies` refers to an existing `axiom_id`
- **No cycles** — topological sort succeeds (DAG property)
- **Schema validation** — required fields present: `axiom_id`, `statement`, `type`, `domain`, `confidence`
- **Confidence range** — 0.0 ≤ confidence ≤ 1.0
- **Known types** — type ∈ {`axiom`, `empirical_axiom`, `definition`, `regime_declaration`, `derived_claim`, `measurement_fact`, `meta`}
- **Domain consistency** — ID prefix matches domain (MATH → mathematics, PHYS → physics, etc.)

Output:
```
✓ 777 axioms loaded
✓ 0 duplicate IDs
✓ 0 missing dependencies
✓ 0 cycles detected
✓ DAG validation passed

Domains:  15
Types:    7 distinct
Roots:    N (no dependencies)
Leaves:   M (no dependents)
Max depth: D
```

Exit code 0 on success, 1 on any error.

### 3. `inject` subcommand

Read axioms JSON + genesis.json. For each axiom, create a `GenesisAxiom` (or `Fact`) entry in the knowledge module's genesis state and write the updated genesis.json back.

**Key:** The knowledge module's genesis state key in app_state is `"knowledge"`. Check the module's `DefaultGenesis()` and `GenesisState` proto to determine the exact field name for pre-verified facts/axioms.

Look at `x/knowledge/types/genesis.go` and `proto/zerone/knowledge/v1/genesis.proto` to find the correct field. It's likely `genesis_axioms` or `facts`.

The inject operation should:
1. Read existing genesis.json
2. Parse `app_state.knowledge`
3. Add/replace the axiom list
4. Write back genesis.json (preserving all other modules)
5. Print count: `✓ Injected 777 axioms into genesis.json`

### 4. `stats` subcommand

Print a summary table:

```
Domain          Count   Roots   Max Depth   Types
─────────────────────────────────────────────────────
mathematics     111     5       8           axiom, definition, derived_claim
physics         111     3       6           axiom, empirical_axiom, measurement_fact
...

Total:          777     N       D
Cross-domain deps: X
```

### 5. `edges` subcommand

Export the dependency graph as CSV for visualization:

```csv
source,target,source_domain,target_domain
MATH-002,MATH-001,mathematics,mathematics
PHYS-001,MATH-003,physics,mathematics
```

Optional `-o <file>` flag; default to stdout.

### 6. GenesisAxiom type

Use the existing type from `x/knowledge/types`. The axiom JSON schema:

```json
{
  "axiom_id": "MATH-001",
  "statement": "Zero is a natural number",
  "formal_expression": "0 in N",
  "type": "axiom",
  "domain": "mathematics",
  "epistemic_category": "analytic",
  "confidence": 1.0,
  "dependencies": [],
  "source_tradition": "Peano (1889)"
}
```

Import from `github.com/zerone-chain/zerone/x/knowledge/types` if the type exists there. If not, define a local struct matching the JSON schema and note that it should be unified with the module type later.

### 7. Tests

Create `tools/axiom-loader/main_test.go`:

- Test valid 777-axiom file passes validation
- Test duplicate ID detection
- Test missing dependency detection
- Test cycle detection (create a small cyclic test fixture)
- Test inject into a minimal genesis.json

## Reference

- Prototype: `legible_money/tools/axiom-loader/main.go`
- Existing axioms: `x/knowledge/types/genesis_axioms.json` (777 axioms)
- Existing parser: `scripts/convert_axioms.go`
- Knowledge genesis proto: `proto/zerone/knowledge/v1/genesis.proto`
- Knowledge genesis Go: `x/knowledge/types/genesis.go`

## Verification

```bash
# Build
go build ./tools/axiom-loader/...

# Validate existing axioms
go run tools/axiom-loader/main.go validate x/knowledge/types/genesis_axioms.json

# Stats
go run tools/axiom-loader/main.go stats x/knowledge/types/genesis_axioms.json

# Inject into a test genesis
zeroned init test-node --chain-id test-1 --home /tmp/test-genesis
go run tools/axiom-loader/main.go inject \
  x/knowledge/types/genesis_axioms.json \
  /tmp/test-genesis/config/genesis.json

# Tests
go test ./tools/axiom-loader/...
```
