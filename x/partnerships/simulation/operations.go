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

	"github.com/zerone-chain/zerone/x/partnerships/keeper"
	"github.com/zerone-chain/zerone/x/partnerships/types"
)

// Operation weight constants used by AppParams.GetOrGenerate.
const (
	OpWeightMsgProposePartnership = "op_weight_msg_propose_partnership"
	OpWeightMsgAcceptPartnership  = "op_weight_msg_accept_partnership"
	OpWeightMsgJoinFormationPool  = "op_weight_msg_join_formation_pool"
	OpWeightMsgInitiateDissolution = "op_weight_msg_initiate_dissolution"

	DefaultWeightMsgProposePartnership = 50
	DefaultWeightMsgAcceptPartnership  = 40
	DefaultWeightMsgJoinFormationPool  = 30
	DefaultWeightMsgInitiateDissolution = 10
)

var domains = []string{
	"mathematics", "physics", "computer-science", "biology",
	"economics", "philosophy", "history", "chemistry",
}

// WeightedOperations returns all the partnerships module's simulation operations
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
		weightProposePartnership int
		weightAcceptPartnership  int
		weightJoinFormationPool  int
		weightInitiateDissolution int
	)

	appParams.GetOrGenerate(OpWeightMsgProposePartnership, &weightProposePartnership, nil, func(_ *rand.Rand) {
		weightProposePartnership = DefaultWeightMsgProposePartnership
	})
	appParams.GetOrGenerate(OpWeightMsgAcceptPartnership, &weightAcceptPartnership, nil, func(_ *rand.Rand) {
		weightAcceptPartnership = DefaultWeightMsgAcceptPartnership
	})
	appParams.GetOrGenerate(OpWeightMsgJoinFormationPool, &weightJoinFormationPool, nil, func(_ *rand.Rand) {
		weightJoinFormationPool = DefaultWeightMsgJoinFormationPool
	})
	appParams.GetOrGenerate(OpWeightMsgInitiateDissolution, &weightInitiateDissolution, nil, func(_ *rand.Rand) {
		weightInitiateDissolution = DefaultWeightMsgInitiateDissolution
	})

	return simulation.WeightedOperations{
		simulation.NewWeightedOperation(weightProposePartnership, SimulateMsgProposePartnership(txGen, ak, bk, k)),
		simulation.NewWeightedOperation(weightAcceptPartnership, SimulateMsgAcceptPartnership(txGen, ak, bk, k)),
		simulation.NewWeightedOperation(weightJoinFormationPool, SimulateMsgJoinFormationPool(txGen, ak, bk, k)),
		simulation.NewWeightedOperation(weightInitiateDissolution, SimulateMsgInitiateDissolution(txGen, ak, bk, k)),
	}
}

// SimulateMsgProposePartnership generates a random partnership proposal between two accounts.
func SimulateMsgProposePartnership(
	txGen client.TxConfig,
	ak simulation.AccountKeeper,
	bk simulation.BankKeeper,
	_ keeper.Keeper,
) simtypes.Operation {
	return func(
		r *rand.Rand, app *baseapp.BaseApp, ctx sdk.Context,
		accs []simtypes.Account, chainID string,
	) (simtypes.OperationMsg, []simtypes.FutureOperation, error) {
		proposer, _ := simtypes.RandomAcc(r, accs)

		deposit := int64(1_000_000 + r.Intn(5_000_000))
		spendable := bk.SpendableCoins(ctx, proposer.Address)
		if spendable.AmountOf("uzrn").LT(sdkmath.NewInt(deposit + 100_000)) {
			return simtypes.NoOpMsg(types.ModuleName, sdk.MsgTypeURL(&types.MsgProposePartnership{}), "insufficient funds"), nil, nil
		}

		// Pick a different account as partner.
		partner, _ := simtypes.RandomAcc(r, accs)
		if partner.Address.Equals(proposer.Address) {
			return simtypes.NoOpMsg(types.ModuleName, sdk.MsgTypeURL(&types.MsgProposePartnership{}), "same account"), nil, nil
		}

		msg := &types.MsgProposePartnership{
			Proposer:       proposer.Address.String(),
			Partner:        partner.Address.String(),
			InitialDeposit: sdkmath.NewInt(deposit).String(),
			ProposedTier:   uint32(1 + r.Intn(3)),
			Domain:         domains[r.Intn(len(domains))],
		}

		txCtx := simulation.OperationInput{
			R:               r,
			App:             app,
			TxGen:           txGen,
			Msg:             msg,
			Context:         ctx,
			SimAccount:      proposer,
			AccountKeeper:   ak,
			Bankkeeper:      bk,
			ModuleName:      types.ModuleName,
			CoinsSpentInMsg: sdk.NewCoins(sdk.NewInt64Coin("uzrn", deposit)),
		}
		return simulation.GenAndDeliverTxWithRandFees(txCtx)
	}
}

// SimulateMsgAcceptPartnership accepts a random pending partnership.
func SimulateMsgAcceptPartnership(
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

		partnerships := k.GetAllPartnerships(ctx)
		if len(partnerships) == 0 {
			return simtypes.NoOpMsg(types.ModuleName, sdk.MsgTypeURL(&types.MsgAcceptPartnership{}), "no partnerships"), nil, nil
		}

		p := partnerships[r.Intn(len(partnerships))]
		deposit := int64(1_000_000)

		spendable := bk.SpendableCoins(ctx, simAccount.Address)
		if spendable.AmountOf("uzrn").LT(sdkmath.NewInt(deposit + 100_000)) {
			return simtypes.NoOpMsg(types.ModuleName, sdk.MsgTypeURL(&types.MsgAcceptPartnership{}), "insufficient funds"), nil, nil
		}

		msg := &types.MsgAcceptPartnership{
			Accepter:      simAccount.Address.String(),
			PartnershipId: p.Id,
			Deposit:       sdkmath.NewInt(deposit).String(),
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
			CoinsSpentInMsg: sdk.NewCoins(sdk.NewInt64Coin("uzrn", deposit)),
		}
		return simulation.GenAndDeliverTxWithRandFees(txCtx)
	}
}

// SimulateMsgJoinFormationPool joins the formation pool with random domains.
func SimulateMsgJoinFormationPool(
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

		deposit := int64(500_000 + r.Intn(2_000_000))
		spendable := bk.SpendableCoins(ctx, simAccount.Address)
		if spendable.AmountOf("uzrn").LT(sdkmath.NewInt(deposit + 100_000)) {
			return simtypes.NoOpMsg(types.ModuleName, sdk.MsgTypeURL(&types.MsgJoinFormationPool{}), "insufficient funds"), nil, nil
		}

		// Pick 1-3 random domains.
		numDomains := 1 + r.Intn(3)
		chosen := make([]string, 0, numDomains)
		for i := 0; i < numDomains; i++ {
			chosen = append(chosen, domains[r.Intn(len(domains))])
		}

		roles := []string{"contributor", "reviewer", "mentor"}
		msg := &types.MsgJoinFormationPool{
			Joiner:        simAccount.Address.String(),
			Domains:       chosen,
			PreferredRole: roles[r.Intn(len(roles))],
			Deposit:       sdkmath.NewInt(deposit).String(),
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
			CoinsSpentInMsg: sdk.NewCoins(sdk.NewInt64Coin("uzrn", deposit)),
		}
		return simulation.GenAndDeliverTxWithRandFees(txCtx)
	}
}

// SimulateMsgInitiateDissolution attempts to dissolve a random existing partnership.
func SimulateMsgInitiateDissolution(
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

		partnerships := k.GetAllPartnerships(ctx)
		if len(partnerships) == 0 {
			return simtypes.NoOpMsg(types.ModuleName, sdk.MsgTypeURL(&types.MsgInitiateDissolution{}), "no partnerships"), nil, nil
		}

		p := partnerships[r.Intn(len(partnerships))]

		msg := &types.MsgInitiateDissolution{
			Initiator:     simAccount.Address.String(),
			PartnershipId: p.Id,
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
