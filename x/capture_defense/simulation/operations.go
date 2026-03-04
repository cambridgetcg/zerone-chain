package simulation

import (
	"math/rand"

	"github.com/cosmos/cosmos-sdk/baseapp"
	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/codec"
	sdk "github.com/cosmos/cosmos-sdk/types"
	simtypes "github.com/cosmos/cosmos-sdk/types/simulation"
	"github.com/cosmos/cosmos-sdk/x/simulation"

	"github.com/zerone-chain/zerone/x/capture_defense/keeper"
	"github.com/zerone-chain/zerone/x/capture_defense/types"
)

// Operation weight constants used by AppParams.GetOrGenerate.
const (
	OpWeightMsgAnalyzeDomain = "op_weight_msg_analyze_domain"

	DefaultWeightMsgAnalyzeDomain = 25
)

var domains = []string{
	"mathematics", "physics", "computer-science", "biology",
	"economics", "philosophy", "history", "chemistry",
}

// WeightedOperations returns the capture_defense module's simulation operations.
func WeightedOperations(
	appParams simtypes.AppParams,
	_ codec.JSONCodec,
	txGen client.TxConfig,
	ak simulation.AccountKeeper,
	bk simulation.BankKeeper,
	k keeper.Keeper,
) simulation.WeightedOperations {
	var weightAnalyzeDomain int

	appParams.GetOrGenerate(OpWeightMsgAnalyzeDomain, &weightAnalyzeDomain, nil, func(_ *rand.Rand) {
		weightAnalyzeDomain = DefaultWeightMsgAnalyzeDomain
	})

	return simulation.WeightedOperations{
		simulation.NewWeightedOperation(weightAnalyzeDomain, SimulateMsgAnalyzeDomain(txGen, ak, bk, k)),
	}
}

// SimulateMsgAnalyzeDomain triggers a domain capture analysis for a random domain.
func SimulateMsgAnalyzeDomain(
	txGen client.TxConfig,
	ak simulation.AccountKeeper,
	bk simulation.BankKeeper,
	_ keeper.Keeper,
) simtypes.Operation {
	return func(
		r *rand.Rand, app *baseapp.BaseApp, ctx sdk.Context,
		accs []simtypes.Account, chainID string,
	) (simtypes.OperationMsg, []simtypes.FutureOperation, error) {
		simAccount, _ := simtypes.RandomAcc(r, accs)

		msg := &types.MsgAnalyzeDomain{
			Sender: simAccount.Address.String(),
			Domain: domains[r.Intn(len(domains))],
		}

		txCtx := simulation.OperationInput{
			R:             r,
			App:           app,
			TxGen:         txGen,
			Msg:           msg,
			Context:       ctx,
			SimAccount:    simAccount,
			AccountKeeper: ak,
			Bankkeeper:    bk,
			ModuleName:    types.ModuleName,
		}
		return simulation.GenAndDeliverTxWithRandFees(txCtx)
	}
}
