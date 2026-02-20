package app

import (
	"encoding/json"

	sdk "github.com/cosmos/cosmos-sdk/types"
	servertypes "github.com/cosmos/cosmos-sdk/server/types"
	"github.com/cosmos/cosmos-sdk/x/staking"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
)

// ExportAppStateAndValidators exports the application state and validators for a genesis file.
// Called by the `zeroned export` command (Cosmovisor-compatible).
func (app *ZeroneApp) ExportAppStateAndValidators(
	forZeroHeight bool,
	jailAllowedAddrs []string,
	modulesToExport []string,
) (servertypes.ExportedApp, error) {
	ctx := app.NewContext(true)

	if forZeroHeight {
		app.prepForZeroHeightGenesis(ctx, jailAllowedAddrs)
	}

	genState, err := app.ModuleManager.ExportGenesisForModules(ctx, app.appCodec, modulesToExport)
	if err != nil {
		return servertypes.ExportedApp{}, err
	}

	appState, err := json.Marshal(genState)
	if err != nil {
		return servertypes.ExportedApp{}, err
	}

	validators, err := staking.WriteValidators(ctx, app.StakingKeeper)
	if err != nil {
		return servertypes.ExportedApp{}, err
	}

	return servertypes.ExportedApp{
		AppState:        appState,
		Validators:      validators,
		Height:          app.LastBlockHeight(),
		ConsensusParams: app.GetConsensusParams(ctx),
	}, nil
}

// prepForZeroHeightGenesis prepares app state for a zero-height genesis export.
// Withdraws all outstanding rewards and clears slash event history so the
// exported genesis is clean for a fresh chain start.
func (app *ZeroneApp) prepForZeroHeightGenesis(ctx sdk.Context, jailAllowedAddrs []string) {
	allowedAddrsMap := make(map[string]bool)
	for _, addr := range jailAllowedAddrs {
		allowedAddrsMap[addr] = true
	}

	// Withdraw all validator commissions.
	app.StakingKeeper.IterateValidators(ctx, func(_ int64, val stakingtypes.ValidatorI) (stop bool) {
		valBz, err := app.StakingKeeper.ValidatorAddressCodec().StringToBytes(val.GetOperator())
		if err != nil {
			return false
		}
		_, _ = app.DistrKeeper.WithdrawValidatorCommission(ctx, valBz)
		return false
	})

	// Withdraw all delegator rewards.
	dels, err := app.StakingKeeper.GetAllDelegations(ctx)
	if err != nil {
		ctx.Logger().Error("prepForZeroHeightGenesis: failed to get delegations", "err", err)
		return
	}
	for _, del := range dels {
		valBz, err := app.StakingKeeper.ValidatorAddressCodec().StringToBytes(del.GetValidatorAddr())
		if err != nil {
			continue
		}
		delAddr, err := app.AccountKeeper.AddressCodec().StringToBytes(del.GetDelegatorAddr())
		if err != nil {
			continue
		}
		_, _ = app.DistrKeeper.WithdrawDelegationRewards(ctx, delAddr, valBz)
	}

	// Clear validator slash events and historical rewards.
	app.DistrKeeper.DeleteAllValidatorSlashEvents(ctx)
	app.DistrKeeper.DeleteAllValidatorHistoricalRewards(ctx)
}
