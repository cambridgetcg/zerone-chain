package cross_stack_test

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	zeroneapp "github.com/zerone-chain/zerone/app"
	zeroneauthtypes "github.com/zerone-chain/zerone/x/auth/types"
	zeronestakingtypes "github.com/zerone-chain/zerone/x/staking/types"
)

// TestDefaultGenesis_AllModules verifies that every registered module
// produces a valid DefaultGenesis that passes ValidateGenesis.
func TestDefaultGenesis_AllModules(t *testing.T) {
	app := newTestApp(t, testChainID)

	genState := app.DefaultGenesis()
	require.NotEmpty(t, genState)

	// Validate that all modules have valid default genesis.
	err := zeroneapp.ModuleBasics.ValidateGenesis(
		app.AppCodec(),
		app.TxConfig(),
		genState,
	)
	require.NoError(t, err, "DefaultGenesis must pass ValidateGenesis for all modules")

	// Verify critical modules are present.
	for _, mod := range []string{
		"auth", "bank", "staking", "distribution", "gov",
		"slashing", "evidence", "upgrade", "ibc",
		zeroneauthtypes.ModuleName,
		zeronestakingtypes.ModuleName,
	} {
		_, ok := genState[mod]
		require.True(t, ok, "expected module %q in default genesis", mod)
	}
}

// TestGenesisRoundTrip verifies that default genesis JSON can be marshaled,
// unmarshaled, and re-validated without data loss. This catches serialization
// and deserialization bugs across all module genesis states.
func TestGenesisRoundTrip(t *testing.T) {
	app := newTestApp(t, testChainID)

	genState := app.DefaultGenesis()
	require.NotEmpty(t, genState)

	// Step 1: Marshal genesis to JSON.
	stateBytes, err := json.Marshal(genState)
	require.NoError(t, err)
	require.NotEmpty(t, stateBytes)

	// Step 2: Unmarshal back to GenesisState.
	var restored zeroneapp.GenesisState
	require.NoError(t, json.Unmarshal(stateBytes, &restored))

	// Step 3: Same number of modules present.
	require.Equal(t, len(genState), len(restored),
		"round-trip must preserve module count")

	// Step 4: Each module's genesis is preserved.
	for mod, original := range genState {
		restoredRaw, ok := restored[mod]
		require.True(t, ok, "module %q missing after round-trip", mod)
		require.JSONEq(t, string(original), string(restoredRaw),
			"module %q genesis differs after round-trip", mod)
	}

	// Step 5: Validate the restored genesis.
	err = zeroneapp.ModuleBasics.ValidateGenesis(
		app.AppCodec(),
		app.TxConfig(),
		restored,
	)
	require.NoError(t, err, "restored genesis must pass ValidateGenesis")

	// Step 6: Verify custom module genesis states specifically.
	// Zerone auth genesis.
	authRaw, ok := restored[zeroneauthtypes.ModuleName]
	require.True(t, ok)
	var authGen zeroneauthtypes.GenesisState
	require.NoError(t, app.AppCodec().UnmarshalJSON(authRaw, &authGen))
	require.NotNil(t, authGen.Params)
	require.Greater(t, authGen.Params.MaxSessionKeys, uint32(0))

	// Zerone staking genesis.
	stakingRaw, ok := restored[zeronestakingtypes.ModuleName]
	require.True(t, ok)
	var stakingGen zeronestakingtypes.GenesisState
	require.NoError(t, json.Unmarshal(stakingRaw, &stakingGen))
	require.NotNil(t, stakingGen.Params)
}

// TestGenesisJSON_Deterministic verifies that serializing genesis twice
// from the same state produces byte-identical JSON. This ensures
// deterministic serialization across all modules.
func TestGenesisJSON_Deterministic(t *testing.T) {
	app := newTestApp(t, testChainID)

	genState := app.DefaultGenesis()

	// Marshal genesis twice.
	bytes1, err := json.Marshal(genState)
	require.NoError(t, err)

	bytes2, err := json.Marshal(genState)
	require.NoError(t, err)

	// Both exports must be byte-identical.
	require.Equal(t, bytes1, bytes2,
		"two serializations of the same genesis must produce identical JSON")

	// Also verify that DefaultGenesis is deterministic across calls.
	genState2 := app.DefaultGenesis()
	bytes3, err := json.Marshal(genState2)
	require.NoError(t, err)
	require.Equal(t, bytes1, bytes3,
		"two calls to DefaultGenesis must produce identical JSON")
}
