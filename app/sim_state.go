package app

import (
	"encoding/json"
	"math/rand"
	"time"

	"github.com/cosmos/cosmos-sdk/codec"
	simtypes "github.com/cosmos/cosmos-sdk/types/simulation"
)

// AppStateFn returns the initial application state using the default genesis
// and random simulation accounts. This is the entry point for the SDK
// simulation framework's AppStateFn parameter.
func AppStateFn(cdc codec.JSONCodec) simtypes.AppStateFn {
	return func(
		r *rand.Rand,
		accs []simtypes.Account,
		config simtypes.Config,
	) (json.RawMessage, []simtypes.Account, string, time.Time) {
		genesis := ModuleBasics.DefaultGenesis(cdc)

		genesisTimestamp := time.Unix(r.Int63n(1e9), 0).UTC()
		chainID := config.ChainID
		if chainID == "" {
			chainID = "sim-zerone-" + simtypes.RandStringOfLength(r, 6)
		}

		appState, err := json.Marshal(genesis)
		if err != nil {
			panic("failed to marshal default genesis: " + err.Error())
		}

		return appState, accs, chainID, genesisTimestamp
	}
}
