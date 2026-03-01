package app

import (
	"context"
	"fmt"

	sdk "github.com/cosmos/cosmos-sdk/types"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"

	vestingrewardstypes "github.com/zerone-chain/zerone/x/vesting_rewards/types"
)

var researchFundModuleAddr = authtypes.NewModuleAddress(ResearchFundName)

var allowedResearchFundDepositors = map[string]bool{
	authtypes.NewModuleAddress(vestingrewardstypes.ModuleName).String(): true,
}

// ResearchFundRestriction is a bank SendRestriction that rejects any send TO the
// research_fund module account unless it originates from vesting_rewards.
// This enforces the invariant that all research fund deposits flow through
// DepositToResearchFund (which handles the founder 7% split).
// Genesis (block height 0) is exempt to allow initial fund seeding.
func ResearchFundRestriction(ctx context.Context, fromAddr, toAddr sdk.AccAddress, amt sdk.Coins) (sdk.AccAddress, error) {
	if toAddr.Equals(researchFundModuleAddr) {
		sdkCtx := sdk.UnwrapSDKContext(ctx)
		if sdkCtx.BlockHeight() == 0 {
			return toAddr, nil // genesis bypass
		}
		if !allowedResearchFundDepositors[fromAddr.String()] {
			return toAddr, fmt.Errorf(
				"unauthorized deposit to research_fund from %s: must route through DepositToResearchFund",
				fromAddr.String(),
			)
		}
	}
	return toAddr, nil
}
