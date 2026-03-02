package e2e_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// TestGenesis_AllModulesInitialize — R32-2 Task 1
//
// Verifies that every custom module initializes successfully on a real chain
// by querying each module's params endpoint and checking for valid JSON.
// ---------------------------------------------------------------------------

func TestGenesis_AllModulesInitialize(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping E2E test in short mode")
	}

	chain, ctx := SetupChain(t, 1)

	// Wait for 5 blocks to ensure all BeginBlock/EndBlock hooks run.
	WaitBlocks(t, chain, ctx, 5)

	// Every custom module with a params query.
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

			// Verify it's valid JSON.
			var parsed map[string]interface{}
			err := json.Unmarshal(out, &parsed)
			require.NoError(t, err, "module %s params not valid JSON: %s", mod, string(out))
		})
	}
}

// ---------------------------------------------------------------------------
// TestGenesis_ExportRoundTrip — R32-2 Task 2
//
// Exports genesis from a running chain, verifies all expected modules are
// present, key param overrides from testGenesisKV() survived, and the
// exported JSON can be re-marshaled (structural round-trip).
// ---------------------------------------------------------------------------

func TestGenesis_ExportRoundTrip(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping E2E test in short mode")
	}

	chain, ctx := SetupChain(t, 1)

	// Run for 20 blocks to accumulate state across all modules.
	WaitBlocks(t, chain, ctx, 20)

	height, err := chain.Height(ctx)
	require.NoError(t, err)

	// Export genesis via the real CLI path (zeroned export).
	exported, err := chain.ExportState(ctx, height)
	require.NoError(t, err)
	require.NotEmpty(t, exported)

	// Parse the exported genesis, preserving number precision.
	var genesis map[string]interface{}
	dec := json.NewDecoder(bytes.NewReader([]byte(exported)))
	dec.UseNumber()
	err = dec.Decode(&genesis)
	require.NoError(t, err, "exported genesis is not valid JSON")

	// Verify app_state exists.
	appState, ok := genesis["app_state"].(map[string]interface{})
	require.True(t, ok, "exported genesis missing app_state")

	// Verify app_state contains all expected custom modules (30 modules).
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

	// Verify key param values match testGenesisKV() overrides.
	verifyParamPath(t, appState, "knowledge", "params", "commit_phase_blocks", 10)
	verifyParamPath(t, appState, "alignment", "params", "observation_interval_blocks", 10)
	verifyParamPath(t, appState, "zerone_gov", "params", "voting_period_blocks", 10)
	verifyParamPath(t, appState, "zerone_staking", "params", "unbonding_period_blocks", 50)
	verifyParamPath(t, appState, "partnerships", "params", "formation_window_blocks", 20)

	// Re-marshal to JSON and re-parse (structural round-trip).
	remarshaled, err := json.Marshal(genesis)
	require.NoError(t, err, "failed to re-marshal exported genesis")

	var reparsed map[string]interface{}
	err = json.Unmarshal(remarshaled, &reparsed)
	require.NoError(t, err, "re-marshaled genesis is not valid JSON")
}

// verifyParamPath navigates nested maps in the exported app_state and asserts
// a field matches the expected value. Handles both string and numeric JSON
// values (Cosmos serializes some params as strings like "10" and some as
// numbers).
func verifyParamPath(t *testing.T, appState map[string]interface{}, module, section, field string, expected int) {
	t.Helper()

	modState, ok := appState[module].(map[string]interface{})
	require.True(t, ok, "module %s state is not a map", module)

	sec, ok := modState[section].(map[string]interface{})
	require.True(t, ok, "module %s missing section %s", module, section)

	actual, ok := sec[field]
	require.True(t, ok, "module %s.%s missing field %s", module, section, field)

	expectedStr := fmt.Sprintf("%d", expected)

	// Handle the various JSON number representations:
	//   - json.Number (when using UseNumber)
	//   - float64 (default json.Unmarshal)
	//   - string (proto3 uint64 encoding)
	switch v := actual.(type) {
	case json.Number:
		require.Equal(t, expectedStr, v.String(),
			"module %s.%s.%s: expected %d, got %s", module, section, field, expected, v)
	case float64:
		require.Equal(t, float64(expected), v,
			"module %s.%s.%s: expected %d, got %v", module, section, field, expected, v)
	case string:
		require.Equal(t, expectedStr, v,
			"module %s.%s.%s: expected %q, got %q", module, section, field, expectedStr, v)
	default:
		t.Fatalf("module %s.%s.%s: unexpected type %T for value %v",
			module, section, field, actual, actual)
	}
}

// ---------------------------------------------------------------------------
// TestGenesis_GenesisCheckTool — R32-2 Task 3
//
// Exports genesis from a running chain and runs tools/genesis-check against
// it, verifying the tool passes on a live-exported genesis.
// ---------------------------------------------------------------------------

func TestGenesis_GenesisCheckTool(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping E2E test in short mode")
	}

	chain, ctx := SetupChain(t, 1)
	WaitBlocks(t, chain, ctx, 10)

	height, err := chain.Height(ctx)
	require.NoError(t, err)

	// Export genesis.
	exported, err := chain.ExportState(ctx, height)
	require.NoError(t, err)
	require.NotEmpty(t, exported)

	// Write exported genesis to a temp file.
	tmpDir := t.TempDir()
	genesisPath := filepath.Join(tmpDir, "exported_genesis.json")
	err = os.WriteFile(genesisPath, []byte(exported), 0o644)
	require.NoError(t, err)

	// Find the project root by walking up from the test directory to find go.mod.
	projectRoot := findProjectRoot(t)

	// Run tools/genesis-check against the exported genesis.
	cmd := exec.Command("go", "run", "./tools/genesis-check/main.go",
		"--genesis", genesisPath,
		"--profile", "testnet",
	)
	cmd.Dir = projectRoot

	output, err := cmd.CombinedOutput()
	t.Logf("genesis-check output:\n%s", string(output))
	require.NoError(t, err, "genesis-check failed (exit code non-zero):\n%s", string(output))
}

// findProjectRoot walks up from the current working directory to locate the
// project root (the directory containing go.mod).
func findProjectRoot(t *testing.T) string {
	t.Helper()

	dir, err := os.Getwd()
	require.NoError(t, err)

	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("could not find project root (no go.mod found in any parent directory)")
		}
		dir = parent
	}
}
