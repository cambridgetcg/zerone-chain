package simulation

import (
	"math/rand"

	"github.com/cosmos/cosmos-sdk/baseapp"
	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/codec"
	sdk "github.com/cosmos/cosmos-sdk/types"
	simtypes "github.com/cosmos/cosmos-sdk/types/simulation"
	"github.com/cosmos/cosmos-sdk/x/simulation"

	"github.com/zerone-chain/zerone/x/vesting_rewards/keeper"
	"github.com/zerone-chain/zerone/x/vesting_rewards/types"
)

// Operation weight constants used by AppParams.GetOrGenerate.
const (
	OpWeightMsgClaimVesting = "op_weight_msg_claim_vesting"

	DefaultWeightMsgClaimVesting = 30
)

// sampleVestingIDs returns up to max vesting schedule IDs from state.
func sampleVestingIDs(ctx sdk.Context, k keeper.Keeper, max int) []string {
	schedules := k.GetAllActiveVestingSchedules(ctx)
	if len(schedules) > max {
		schedules = schedules[:max]
	}
	ids := make([]string, len(schedules))
	for i, s := range schedules {
		ids[i] = s.Id
	}
	return ids
}

// WeightedOperations returns the vesting_rewards module's simulation operations.
func WeightedOperations(
	appParams simtypes.AppParams,
	_ codec.JSONCodec,
	txGen client.TxConfig,
	ak simulation.AccountKeeper,
	bk simulation.BankKeeper,
	k keeper.Keeper,
) simulation.WeightedOperations {
	var weightClaimVesting int

	appParams.GetOrGenerate(OpWeightMsgClaimVesting, &weightClaimVesting, nil, func(_ *rand.Rand) {
		weightClaimVesting = DefaultWeightMsgClaimVesting
	})

	return simulation.WeightedOperations{
		simulation.NewWeightedOperation(weightClaimVesting, SimulateMsgClaimVesting(txGen, ak, bk, k)),
	}
}

// SimulateMsgClaimVesting attempts to claim a random active vesting schedule.
func SimulateMsgClaimVesting(
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

		ids := sampleVestingIDs(ctx, k, 50)
		if len(ids) == 0 {
			return simtypes.NoOpMsg(types.ModuleName, sdk.MsgTypeURL(&types.MsgClaimVesting{}), "no active vestings"), nil, nil
		}

		// Pick 1-3 random vesting IDs to claim.
		numClaims := 1 + r.Intn(min(3, len(ids)))
		r.Shuffle(len(ids), func(i, j int) { ids[i], ids[j] = ids[j], ids[i] })
		chosen := ids[:numClaims]

		msg := &types.MsgClaimVesting{
			Claimer:    simAccount.Address.String(),
			VestingIds: chosen,
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

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
