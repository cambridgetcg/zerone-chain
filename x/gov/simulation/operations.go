package simulation

import (
	"math/rand"

	sdkmath "cosmossdk.io/math"

	"github.com/cosmos/cosmos-sdk/baseapp"
	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/codec"
	sdk "github.com/cosmos/cosmos-sdk/types"
	simtypes "github.com/cosmos/cosmos-sdk/types/simulation"
	"github.com/cosmos/cosmos-sdk/x/simulation"

	"github.com/zerone-chain/zerone/x/gov/keeper"
	"github.com/zerone-chain/zerone/x/gov/types"
)

// Operation weight constants used by AppParams.GetOrGenerate.
const (
	OpWeightMsgSubmitLIP = "op_weight_msg_submit_lip"
	OpWeightMsgStakeLIP  = "op_weight_msg_stake_lip"
	OpWeightMsgCastVote  = "op_weight_msg_cast_vote"

	DefaultWeightMsgSubmitLIP = 40
	DefaultWeightMsgStakeLIP  = 30
	DefaultWeightMsgCastVote  = 60
)

var lipCategories = []string{"parameter", "upgrade", "text"}

// sampleLIPs returns up to max LIPs from state.
func sampleLIPs(ctx sdk.Context, k keeper.Keeper, max int) []*types.LIP {
	var lips []*types.LIP
	k.IterateLIPs(ctx, func(lip *types.LIP) bool {
		lips = append(lips, lip)
		return len(lips) >= max
	})
	return lips
}

// WeightedOperations returns all the gov module's simulation operations
// with their respective weight.
func WeightedOperations(
	appParams simtypes.AppParams,
	_ codec.JSONCodec,
	txGen client.TxConfig,
	ak simulation.AccountKeeper,
	bk simulation.BankKeeper,
	k keeper.Keeper,
) simulation.WeightedOperations {
	var (
		weightSubmitLIP int
		weightStakeLIP  int
		weightCastVote  int
	)

	appParams.GetOrGenerate(OpWeightMsgSubmitLIP, &weightSubmitLIP, nil, func(_ *rand.Rand) {
		weightSubmitLIP = DefaultWeightMsgSubmitLIP
	})
	appParams.GetOrGenerate(OpWeightMsgStakeLIP, &weightStakeLIP, nil, func(_ *rand.Rand) {
		weightStakeLIP = DefaultWeightMsgStakeLIP
	})
	appParams.GetOrGenerate(OpWeightMsgCastVote, &weightCastVote, nil, func(_ *rand.Rand) {
		weightCastVote = DefaultWeightMsgCastVote
	})

	return simulation.WeightedOperations{
		simulation.NewWeightedOperation(weightSubmitLIP, SimulateMsgSubmitLIP(txGen, ak, bk, k)),
		simulation.NewWeightedOperation(weightStakeLIP, SimulateMsgStakeLIP(txGen, ak, bk, k)),
		simulation.NewWeightedOperation(weightCastVote, SimulateMsgCastVote(txGen, ak, bk, k)),
	}
}

// SimulateMsgSubmitLIP generates a random LIP submission.
func SimulateMsgSubmitLIP(
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

		stake := int64(1_000_000_000) // 1000 ZRN minimum
		spendable := bk.SpendableCoins(ctx, simAccount.Address)
		if spendable.AmountOf("uzrn").LT(sdkmath.NewInt(stake + 100_000)) {
			return simtypes.NoOpMsg(types.ModuleName, sdk.MsgTypeURL(&types.MsgSubmitLIP{}), "insufficient funds"), nil, nil
		}

		msg := &types.MsgSubmitLIP{
			Proposer:     simAccount.Address.String(),
			Title:        simtypes.RandStringOfLength(r, 10+r.Intn(50)),
			Description:  simtypes.RandStringOfLength(r, 50+r.Intn(200)),
			Category:     lipCategories[r.Intn(len(lipCategories))],
			InitialStake: sdkmath.NewInt(stake).String(),
		}

		txCtx := simulation.OperationInput{
			R:               r,
			App:             app,
			TxGen:           txGen,
			Msg:             msg,
			Context:         ctx,
			SimAccount:      simAccount,
			AccountKeeper:   ak,
			Bankkeeper:      bk,
			ModuleName:      types.ModuleName,
			CoinsSpentInMsg: sdk.NewCoins(sdk.NewInt64Coin("uzrn", stake)),
		}
		return simulation.GenAndDeliverTxWithRandFees(txCtx)
	}
}

// SimulateMsgStakeLIP adds stake to a random existing LIP.
func SimulateMsgStakeLIP(
	txGen client.TxConfig,
	ak simulation.AccountKeeper,
	bk simulation.BankKeeper,
	k keeper.Keeper,
) simtypes.Operation {
	return func(
		r *rand.Rand, app *baseapp.BaseApp, ctx sdk.Context,
		accs []simtypes.Account, chainID string,
	) (simtypes.OperationMsg, []simtypes.FutureOperation, error) {
		simAccount, _ := simtypes.RandomAcc(r, accs)

		lips := sampleLIPs(ctx, k, 50)
		if len(lips) == 0 {
			return simtypes.NoOpMsg(types.ModuleName, sdk.MsgTypeURL(&types.MsgStakeLIP{}), "no LIPs"), nil, nil
		}

		amount := int64(100_000 + r.Intn(5_000_000))
		spendable := bk.SpendableCoins(ctx, simAccount.Address)
		if spendable.AmountOf("uzrn").LT(sdkmath.NewInt(amount + 100_000)) {
			return simtypes.NoOpMsg(types.ModuleName, sdk.MsgTypeURL(&types.MsgStakeLIP{}), "insufficient funds"), nil, nil
		}

		lip := lips[r.Intn(len(lips))]
		msg := &types.MsgStakeLIP{
			Staker: simAccount.Address.String(),
			LipId:  lip.Id,
			Amount: sdkmath.NewInt(amount).String(),
		}

		txCtx := simulation.OperationInput{
			R:               r,
			App:             app,
			TxGen:           txGen,
			Msg:             msg,
			Context:         ctx,
			SimAccount:      simAccount,
			AccountKeeper:   ak,
			Bankkeeper:      bk,
			ModuleName:      types.ModuleName,
			CoinsSpentInMsg: sdk.NewCoins(sdk.NewInt64Coin("uzrn", amount)),
		}
		return simulation.GenAndDeliverTxWithRandFees(txCtx)
	}
}

// SimulateMsgCastVote casts a random vote on an existing LIP.
func SimulateMsgCastVote(
	txGen client.TxConfig,
	ak simulation.AccountKeeper,
	bk simulation.BankKeeper,
	k keeper.Keeper,
) simtypes.Operation {
	return func(
		r *rand.Rand, app *baseapp.BaseApp, ctx sdk.Context,
		accs []simtypes.Account, chainID string,
	) (simtypes.OperationMsg, []simtypes.FutureOperation, error) {
		simAccount, _ := simtypes.RandomAcc(r, accs)

		lips := sampleLIPs(ctx, k, 50)
		if len(lips) == 0 {
			return simtypes.NoOpMsg(types.ModuleName, sdk.MsgTypeURL(&types.MsgCastVote{}), "no LIPs"), nil, nil
		}

		lip := lips[r.Intn(len(lips))]
		options := []string{"yes", "no", "abstain"}
		msg := &types.MsgCastVote{
			Voter:  simAccount.Address.String(),
			LipId:  lip.Id,
			Option: options[r.Intn(len(options))],
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
