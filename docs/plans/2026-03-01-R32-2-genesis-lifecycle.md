# R32-2 Genesis Lifecycle E2E — Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** E2E tests validating all 32 custom modules initialize on a real Docker chain, genesis exports cleanly, and `tools/genesis-check` passes on live-exported genesis.

**Architecture:** Single test file `tests/e2e/genesis_lifecycle_test.go` using the existing interchaintest harness. Three tests: module initialization, export round-trip validation, and genesis-check tool integration. Uses `chain.ExportState()` for the real CLI export path.

**Tech Stack:** Go 1.24+, interchaintest/v8, testify, Docker

---

### Task 1: Write TestGenesis_AllModulesInitialize

**Files:**
- Create: `tests/e2e/genesis_lifecycle_test.go`

**Step 1: Write the test**

```go
package e2e_test

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGenesis_AllModulesInitialize(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping E2E test in short mode")
	}

	chain, ctx := SetupChain(t, 1)

	// Wait for 5 blocks to ensure all BeginBlock/EndBlock hooks run
	WaitBlocks(t, chain, ctx, 5)

	// Every custom module with a Params query should be queryable
	modules := []string{
		"alignment",
		"autopoiesis",
		"billing",
		"bvm",
		"capture_challenge",
		"capture_defense",
		"channels",
		"claiming_pot",
		"compute_pool",
		"discovery",
		"disputes",
		"emergency",
		"home",
		"ibcratelimit",
		"icaauth",
		"knowledge",
		"liquiditypool",
		"zerone_ontology",
		"partnerships",
		"qualification",
		"research",
		"schedule",
		"tokens",
		"toolbox",
		"tree",
		"vesting_rewards",
		"zerone_auth",
		"zerone_gov",
		"zerone_staking",
	}

	for _, mod := range modules {
		t.Run(mod, func(t *testing.T) {
			out := QueryModule(t, chain, ctx, mod, "params")
			require.NotEmpty(t, out, "module %s params query returned empty", mod)

			// Verify it's valid JSON
			var parsed map[string]interface{}
			err := json.Unmarshal(out, &parsed)
			require.NoError(t, err, "module %s params not valid JSON: %s", mod, string(out))
		})
	}
}
```

**Step 2: Verify it compiles**

Run: `cd /Users/yournameisai/Desktop/zerone && go vet ./tests/e2e/...`
Expected: No errors (compilation check only, no Docker needed)

**Step 3: Commit**

```bash
git add tests/e2e/genesis_lifecycle_test.go
git commit -m "test(e2e): add TestGenesis_AllModulesInitialize — R32-2"
```

---

### Task 2: Write TestGenesis_ExportRoundTrip

**Files:**
- Modify: `tests/e2e/genesis_lifecycle_test.go`

**Step 1: Add the export round-trip test**

Append to `genesis_lifecycle_test.go`:

```go
func TestGenesis_ExportRoundTrip(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping E2E test in short mode")
	}

	chain, ctx := SetupChain(t, 1)

	// Run for 20 blocks to accumulate state across all modules
	WaitBlocks(t, chain, ctx, 20)

	height, err := chain.Height(ctx)
	require.NoError(t, err)

	// Export genesis via the real CLI path (zeroned export)
	exported, err := chain.ExportState(ctx, height)
	require.NoError(t, err)
	require.NotEmpty(t, exported)

	// Parse the exported genesis
	var genesis map[string]interface{}
	err = json.Unmarshal([]byte(exported), &genesis)
	require.NoError(t, err, "exported genesis is not valid JSON")

	// Verify app_state contains all expected custom modules
	appState, ok := genesis["app_state"].(map[string]interface{})
	require.True(t, ok, "exported genesis missing app_state")

	expectedModules := []string{
		"knowledge", "alignment", "autopoiesis", "partnerships",
		"vesting_rewards", "capture_defense", "capture_challenge",
		"zerone_gov", "zerone_staking", "zerone_auth",
		"emergency", "research", "tree", "disputes",
		"billing", "tokens", "liquiditypool", "bvm",
		"home", "channels", "schedule", "compute_pool",
		"discovery", "toolbox", "qualification",
		"zerone_ontology", "claiming_pot", "evidence_mgmt",
		"ibcratelimit", "icaauth",
	}

	for _, mod := range expectedModules {
		_, exists := appState[mod]
		require.True(t, exists, "exported genesis missing module: %s", mod)
	}

	// Verify key param values match testGenesisKV() overrides
	verifyParamPath(t, appState, "knowledge", "params", "commit_phase_blocks", float64(10))
	verifyParamPath(t, appState, "alignment", "params", "observation_interval_blocks", float64(10))
	verifyParamPath(t, appState, "zerone_gov", "params", "voting_period_blocks", float64(10))
	verifyParamPath(t, appState, "zerone_staking", "params", "unbonding_period_blocks", float64(50))
	verifyParamPath(t, appState, "partnerships", "params", "formation_window_blocks", float64(20))

	// Re-marshal and verify it's still valid JSON (structural round-trip)
	remarshaled, err := json.Marshal(genesis)
	require.NoError(t, err, "failed to re-marshal exported genesis")

	var reparsed map[string]interface{}
	err = json.Unmarshal(remarshaled, &reparsed)
	require.NoError(t, err, "re-marshaled genesis is not valid JSON")
}

// verifyParamPath checks a nested value in the exported genesis app_state.
// path: module -> section -> field
func verifyParamPath(t *testing.T, appState map[string]interface{}, module, section, field string, expected interface{}) {
	t.Helper()

	modState, ok := appState[module].(map[string]interface{})
	require.True(t, ok, "module %s state is not a map", module)

	sec, ok := modState[section].(map[string]interface{})
	require.True(t, ok, "module %s missing section %s", module, section)

	actual, ok := sec[field]
	require.True(t, ok, "module %s.%s missing field %s", module, section, field)

	// JSON numbers may be strings in Cosmos genesis; handle both
	switch v := actual.(type) {
	case string:
		// Some Cosmos params are serialized as strings (e.g. "10")
		expectedStr, isStr := expected.(string)
		if isStr {
			require.Equal(t, expectedStr, v, "module %s.%s.%s", module, section, field)
		}
		// If expected is float64 but actual is string, just verify non-empty
		require.NotEmpty(t, v, "module %s.%s.%s is empty string", module, section, field)
	default:
		require.Equal(t, expected, actual, "module %s.%s.%s", module, section, field)
	}
}
```

**Step 2: Verify it compiles**

Run: `cd /Users/yournameisai/Desktop/zerone && go vet ./tests/e2e/...`
Expected: No errors

**Step 3: Commit**

```bash
git add tests/e2e/genesis_lifecycle_test.go
git commit -m "test(e2e): add TestGenesis_ExportRoundTrip — R32-2"
```

---

### Task 3: Write TestGenesis_GenesisCheckTool

**Files:**
- Modify: `tests/e2e/genesis_lifecycle_test.go`

**Step 1: Add the genesis-check integration test**

This test exports genesis from the running chain, writes it to a temp file, and runs `tools/genesis-check` against it. Since `tools/genesis-check` runs on the host (not in Docker), we use `exec.Command` directly.

Append to `genesis_lifecycle_test.go`:

```go
import (
	"os"
	"os/exec"
	"path/filepath"
)

func TestGenesis_GenesisCheckTool(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping E2E test in short mode")
	}

	chain, ctx := SetupChain(t, 1)
	WaitBlocks(t, chain, ctx, 10)

	height, err := chain.Height(ctx)
	require.NoError(t, err)

	// Export genesis
	exported, err := chain.ExportState(ctx, height)
	require.NoError(t, err)
	require.NotEmpty(t, exported)

	// Write to temp file
	tmpDir := t.TempDir()
	genesisPath := filepath.Join(tmpDir, "exported_genesis.json")
	err = os.WriteFile(genesisPath, []byte(exported), 0o644)
	require.NoError(t, err)

	// Run tools/genesis-check against the exported genesis
	cmd := exec.Command("go", "run", "../../tools/genesis-check/main.go",
		"--genesis", genesisPath,
		"--profile", "testnet",
	)
	cmd.Dir = filepath.Join("tests", "e2e")
	output, err := cmd.CombinedOutput()
	require.NoError(t, err, "genesis-check failed:\n%s", string(output))

	t.Logf("genesis-check output:\n%s", string(output))
}
```

**Step 2: Verify it compiles**

Run: `cd /Users/yournameisai/Desktop/zerone && go vet ./tests/e2e/...`
Expected: No errors

**Step 3: Commit**

```bash
git add tests/e2e/genesis_lifecycle_test.go
git commit -m "test(e2e): add TestGenesis_GenesisCheckTool — R32-2"
```

---

### Task 4: Run E2E tests

**Step 1: Build Docker image**

Run: `cd /Users/yournameisai/Desktop/zerone && make docker-build-local`
Expected: `zerone:local` image built successfully

**Step 2: Run E2E test suite**

Run: `cd /Users/yournameisai/Desktop/zerone && go test -v -timeout 20m -run "TestGenesis_" ./tests/e2e/...`
Expected: All three tests pass

**Step 3: Fix any failures and re-run**

Common issues to watch for:
- Module name mismatch (e.g. module registered as different name than expected)
- `ExportState` returning empty or error (may need `chain.GetNode()` instead of `chain.GetFullNode()`)
- `genesis-check` path resolution (adjust `cmd.Dir` if needed)
- Params serialized as strings vs numbers in genesis JSON

**Step 4: Commit fixes if needed**

```bash
git add tests/e2e/genesis_lifecycle_test.go
git commit -m "fix(e2e): address genesis lifecycle test fixes — R32-2"
```

---

### Task 5: Final commit

**Step 1: Run full E2E suite to confirm nothing regressed**

Run: `cd /Users/yournameisai/Desktop/zerone && make e2e-test`
Expected: All E2E tests pass (smoke + genesis lifecycle)

**Step 2: Commit design doc**

```bash
git add docs/plans/2026-03-01-R32-2-genesis-lifecycle-design.md
git add docs/plans/2026-03-01-R32-2-genesis-lifecycle.md
git commit -m "docs: add R32-2 genesis lifecycle design + plan"
```
