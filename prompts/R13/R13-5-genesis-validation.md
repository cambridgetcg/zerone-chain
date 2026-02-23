# R13-5 — Per-Module Genesis Validation Tests + Block Production

## Context

The prototype had comprehensive Go-level genesis validation:
- `TestFullGenesisValidation`: calls `DefaultGenesis() + Validate()` for every module **individually**
- `TestFullGenesisInitExport`: InitGenesis → ExportGenesis round-trip at keeper level
- `TestBlockProductionTo100`: boots a full app and produces 100 blocks from default genesis

Zerone has `tests/cross_stack/genesis_test.go` with `TestDefaultGenesis_AllModules` (bulk validation) and `TestGenesisRoundTrip` (JSON round-trip), plus `genesis_roundtrip_test.go` with an app boot smoke test. But it lacks:

1. **Per-module individual testing** — if one module's genesis breaks, you want to know which
2. **Keeper-level InitGenesis→ExportGenesis** — with actual state, not just JSON marshaling
3. **Block production test** — prove the chain can actually produce blocks from genesis

## Task

### 1. Enhance `tests/cross_stack/genesis_test.go`

Add `TestPerModuleGenesisValidation` — one sub-test per custom module:

```go
func TestPerModuleGenesisValidation(t *testing.T) {
    tests := []struct {
        name     string
        validate func() error
    }{
        {"alignment", func() error { return alignmenttypes.DefaultGenesis().Validate() }},
        {"zerone_auth", func() error { return authtypes.DefaultGenesis().Validate() }},
        {"autopoiesis", func() error { return autopoiesistypes.DefaultGenesis().Validate() }},
        {"billing", func() error { return billingtypes.DefaultGenesis().Validate() }},
        // ... all 30 modules
    }

    for _, tc := range tests {
        t.Run(tc.name, func(t *testing.T) {
            err := tc.validate()
            require.NoError(t, err, "DefaultGenesis().Validate() failed for %s", tc.name)
        })
    }
}
```

All 30 custom modules:
```
alignment, zerone_auth, autopoiesis, billing, bvm,
capture_challenge, capture_defense, channels, claiming_pot, compute_pool,
discovery, disputes, emergency, evidence_mgmt, zerone_gov,
home, ibcratelimit, icaauth, knowledge, liquiditypool,
ontology, partnerships, qualification, research, schedule,
zerone_staking, tokens, toolbox, tree, vesting_rewards
```

For each module, import `x/<module>/types` and call `DefaultGenesis().Validate()`.

**Note:** Some modules may not have a `Validate()` method on their `GenesisState`. In those cases, validate via `Params.Validate()` and document the gap (following the prototype's pattern of `I1`, `I2`, etc. known issues).

### 2. Add keeper-level InitGenesis → ExportGenesis round-trip

Create `TestKeeperGenesisRoundTrip` for modules with complex genesis state. Priority modules:

```go
func TestKeeperGenesisRoundTrip_Knowledge(t *testing.T) {
    // 1. Create keeper with test store
    // 2. Set up default genesis with 777 axioms
    // 3. InitGenesis(ctx, genesis)
    // 4. exported := ExportGenesis(ctx)
    // 5. Assert exported matches original:
    //    - Same number of axioms
    //    - Same params
    //    - Axiom IDs preserved
}

func TestKeeperGenesisRoundTrip_Staking(t *testing.T) {
    // 1. DefaultGenesis with tier configs
    // 2. InitGenesis → ExportGenesis
    // 3. Assert all 4 tier configs preserved
    // 4. Assert params match
}

func TestKeeperGenesisRoundTrip_Gov(t *testing.T) {
    // 1. DefaultGenesis with category configs
    // 2. InitGenesis → ExportGenesis
    // 3. Assert category configs preserved
}

func TestKeeperGenesisRoundTrip_VestingRewards(t *testing.T) {
    // 1. DefaultGenesis with 10 category configs
    // 2. InitGenesis → ExportGenesis
    // 3. Assert all category configs + params preserved
}
```

Use the existing test harness infrastructure from `tests/cross_stack/` (e.g., `NewTestHarness`).

### 3. Add block production test

```go
func TestBlockProduction_100Blocks(t *testing.T) {
    if testing.Short() {
        t.Skip("block production test requires full app boot")
    }

    // 1. Create test harness (boots full app with InitChain + Commit)
    h := NewTestHarness(t)

    // 2. Produce 100 blocks
    h.AdvanceBlocks(100)

    // 3. Verify height
    ctx := h.Ctx()
    require.Equal(t, int64(101), ctx.BlockHeight(),
        "expected block height 101 after 100 advances")

    // 4. Verify module invariants hold after 100 blocks:
    //    - Bank supply conserved
    //    - Staking total bonded consistent
    //    - No panics (implicit — test wouldn't reach here)
}
```

### 4. Add genesis axiom validation test

```go
func TestGenesisAxioms_LoadAndValidate(t *testing.T) {
    // 1. Load genesis_axioms.json
    // 2. Verify count == 777
    // 3. Verify no duplicate IDs
    // 4. Verify all dependencies reference existing axioms
    // 5. Verify DAG property (topological sort succeeds)
    // 6. Verify each axiom has required fields
}
```

This provides Go-level coverage complementing the `axiom-loader validate` tool.

### 5. Add genesis with explicit params test

```go
func TestExplicitGenesisParams_Smoke(t *testing.T) {
    // 1. Boot app with default genesis
    // 2. Query each module's params
    // 3. Verify critical params are non-zero / non-default where expected:
    //    - knowledge.min_verifiers > 0
    //    - zerone_staking has 4 tier configs
    //    - vesting_rewards has 10 category configs
    //    - emergency.halt_quorum > 0
    //    - disputes has 3 tier configs
}
```

## Import Reference

Import paths for all 30 modules:
```go
alignmenttypes "github.com/zerone-chain/zerone/x/alignment/types"
authtypes "github.com/zerone-chain/zerone/x/auth/types"
autopoiesistypes "github.com/zerone-chain/zerone/x/autopoiesis/types"
billingtypes "github.com/zerone-chain/zerone/x/billing/types"
bvmtypes "github.com/zerone-chain/zerone/x/bvm/types"
capturechallengetypes "github.com/zerone-chain/zerone/x/capture_challenge/types"
capturedefensetypes "github.com/zerone-chain/zerone/x/capture_defense/types"
channelstypes "github.com/zerone-chain/zerone/x/channels/types"
claimingpottypes "github.com/zerone-chain/zerone/x/claiming_pot/types"
computepooltypes "github.com/zerone-chain/zerone/x/compute_pool/types"
discoverytypes "github.com/zerone-chain/zerone/x/discovery/types"
disputestypes "github.com/zerone-chain/zerone/x/disputes/types"
emergencytypes "github.com/zerone-chain/zerone/x/emergency/types"
evidencemgmttypes "github.com/zerone-chain/zerone/x/evidence_mgmt/types"
govtypes "github.com/zerone-chain/zerone/x/gov/types"
hometypes "github.com/zerone-chain/zerone/x/home/types"
ibcratelimittypes "github.com/zerone-chain/zerone/x/ibcratelimit/types"
icaauthtypes "github.com/zerone-chain/zerone/x/icaauth/types"
knowledgetypes "github.com/zerone-chain/zerone/x/knowledge/types"
liquiditypooltypes "github.com/zerone-chain/zerone/x/liquiditypool/types"
ontologytypes "github.com/zerone-chain/zerone/x/ontology/types"
partnershipstypes "github.com/zerone-chain/zerone/x/partnerships/types"
qualificationtypes "github.com/zerone-chain/zerone/x/qualification/types"
researchtypes "github.com/zerone-chain/zerone/x/research/types"
scheduletypes "github.com/zerone-chain/zerone/x/schedule/types"
stakingtypes "github.com/zerone-chain/zerone/x/staking/types"
tokenstypes "github.com/zerone-chain/zerone/x/tokens/types"
toolboxtypes "github.com/zerone-chain/zerone/x/toolbox/types"
treetypes "github.com/zerone-chain/zerone/x/tree/types"
vestingrewardstypes "github.com/zerone-chain/zerone/x/vesting_rewards/types"
```

## Reference

- Prototype: `legible_money/tests/cross_stack/genesis_validation_test.go`
- Existing Zerone tests: `tests/cross_stack/genesis_test.go`, `genesis_roundtrip_test.go`
- Existing harness: `tests/cross_stack/harness_test.go`
- Axioms: `x/knowledge/types/genesis_axioms.json`

## Verification

```bash
# Run all genesis tests
go test ./tests/cross_stack/... -run "Genesis|Module|Block" -v -count=1 -timeout 120s

# Run per-module validation (fast)
go test ./tests/cross_stack/... -run TestPerModuleGenesisValidation -v

# Run block production (slower)
go test ./tests/cross_stack/... -run TestBlockProduction_100Blocks -v -timeout 60s
```
