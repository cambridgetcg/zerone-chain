package app_test

import (
	"encoding/json"
	"fmt"
	"os"
	"testing"

	dbm "github.com/cosmos/cosmos-db"
	"github.com/stretchr/testify/require"

	"cosmossdk.io/log"

	"github.com/cosmos/cosmos-sdk/baseapp"
	simtestutil "github.com/cosmos/cosmos-sdk/testutil/sims"
	"github.com/cosmos/cosmos-sdk/types/module"
	simtypes "github.com/cosmos/cosmos-sdk/types/simulation"
	"github.com/cosmos/cosmos-sdk/x/simulation"
	simcli "github.com/cosmos/cosmos-sdk/x/simulation/client/cli"

	zeroneapp "github.com/zerone-chain/zerone/app"
)

func init() {
	simcli.GetSimulatorFlags()
}

// simConfig returns a simulation Config suitable for CI / local runs.
func simConfig() simtypes.Config {
	return simtypes.Config{
		Seed:               42,
		NumBlocks:          100,
		BlockSize:          50,
		ChainID:            "sim-zerone-1",
		Commit:             true,
		Lean:               true,
		InitialBlockHeight: 1,
	}
}

// BenchmarkFullAppSimulation runs a deterministic multi-block simulation using
// the SDK simulation framework with all registered weighted operations.
func BenchmarkFullAppSimulation(b *testing.B) {
	config := simcli.NewConfigFromFlags()
	if config.NumBlocks == 0 {
		config = simConfig()
	}

	db := dbm.NewMemDB()
	app := zeroneapp.NewZeroneApp(
		log.NewNopLogger(),
		db,
		nil,
		true,
		simtestutil.NewAppOptionsWithFlagHome(b.TempDir()),
		baseapp.SetChainID(config.ChainID),
	)
	require.NotNil(b, app)

	sm := app.SimulationManager()
	require.NotNil(b, sm)

	appStateFn := zeroneapp.AppStateFn(app.AppCodec())

	_, simParams, simErr := simulation.SimulateFromSeed(
		b,
		os.Stdout,
		app.BaseApp,
		appStateFn,
		simtypes.RandomAccounts,
		sm.WeightedOperations(
			module.SimulationState{
				AppParams: make(simtypes.AppParams),
				Cdc:       app.AppCodec(),
				TxConfig:  app.TxConfig(),
			},
		),
		zeroneapp.BlockedModuleAccountAddrs(),
		config,
		app.AppCodec(),
	)
	require.NoError(b, simErr)

	if config.ExportParamsPath != "" {
		bz, err := json.MarshalIndent(simParams, "", " ")
		require.NoError(b, err)
		require.NoError(b, os.WriteFile(config.ExportParamsPath, bz, 0o600))
	}
}

// TestFullAppSimulation_Deterministic runs a short simulation twice with the
// same seed and asserts both produce identical app hashes.
func TestFullAppSimulation_Deterministic(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping simulation test in short mode")
	}

	config := simConfig()
	config.NumBlocks = 20
	config.BlockSize = 25

	run := func() []byte {
		db := dbm.NewMemDB()
		app := zeroneapp.NewZeroneApp(
			log.NewNopLogger(),
			db,
			nil,
			true,
			simtestutil.NewAppOptionsWithFlagHome(t.TempDir()),
			baseapp.SetChainID(config.ChainID),
		)

		sm := app.SimulationManager()
		require.NotNil(t, sm)

		appStateFn := zeroneapp.AppStateFn(app.AppCodec())

		_, _, simErr := simulation.SimulateFromSeed(
			t,
			os.Stdout,
			app.BaseApp,
			appStateFn,
			simtypes.RandomAccounts,
			sm.WeightedOperations(
				module.SimulationState{
					AppParams: make(simtypes.AppParams),
					Cdc:       app.AppCodec(),
					TxConfig:  app.TxConfig(),
				},
			),
			zeroneapp.BlockedModuleAccountAddrs(),
			config,
			app.AppCodec(),
		)
		require.NoError(t, simErr)

		// Export the final app hash via genesis export.
		exported, err := app.ExportAppStateAndValidators(false, nil, nil)
		require.NoError(t, err)
		return exported.AppState
	}

	state1 := run()
	state2 := run()

	require.Equal(t, state1, state2,
		fmt.Sprintf("deterministic simulation produced different app states"))
}
