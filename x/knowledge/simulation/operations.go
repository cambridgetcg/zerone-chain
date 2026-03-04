package simulation

import (
	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/codec"
	simtypes "github.com/cosmos/cosmos-sdk/types/simulation"
	"github.com/cosmos/cosmos-sdk/x/simulation"

	"github.com/zerone-chain/zerone/x/knowledge/keeper"
)

// WeightedOperations returns knowledge module simulation operations.
// Stubbed: the training data protocol handlers are not yet implemented.
func WeightedOperations(
	_ simtypes.AppParams,
	_ codec.JSONCodec,
	_ client.TxConfig,
	_ simulation.AccountKeeper,
	_ simulation.BankKeeper,
	_ keeper.Keeper,
) simulation.WeightedOperations {
	return nil
}
