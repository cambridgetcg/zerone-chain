package keeper

import (
	"context"
	"math/big"

	sdk "github.com/cosmos/cosmos-sdk/types"

	govtypes "github.com/zerone-chain/zerone/x/gov/types"
	"github.com/zerone-chain/zerone/x/staking/types"
)

// GovStakingKeeperAdapter wraps the staking Keeper to satisfy the
// governance module's StakingKeeper interface.
type GovStakingKeeperAdapter struct {
	k Keeper
}

// NewGovStakingKeeperAdapter returns an adapter for the governance module.
func NewGovStakingKeeperAdapter(k Keeper) *GovStakingKeeperAdapter {
	return &GovStakingKeeperAdapter{k: k}
}

// Compile-time interface check.
var _ govtypes.StakingKeeper = (*GovStakingKeeperAdapter)(nil)

// GetTotalBondedStake returns the total bonded stake as a decimal string.
func (a *GovStakingKeeperAdapter) GetTotalBondedStake(ctx context.Context) (string, error) {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	total := a.k.GetTotalBondedStake(sdkCtx)
	return total.String(), nil
}

// GetDelegatorTotalBonded returns the total bonded tokens for a delegator as a decimal string.
// It iterates all delegations and sums amounts where delegator_address matches.
func (a *GovStakingKeeperAdapter) GetDelegatorTotalBonded(ctx context.Context, addr string) (string, error) {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	total := new(big.Int)

	a.k.IterateDelegations(sdkCtx, func(del *types.Delegation) bool {
		if del.DelegatorAddress == addr {
			amt, ok := new(big.Int).SetString(del.Amount, 10)
			if ok {
				total.Add(total, amt)
			}
		}
		return false
	})

	return total.String(), nil
}

// CountActiveGuardians returns the number of active Guardian-tier validators.
func (a *GovStakingKeeperAdapter) CountActiveGuardians(ctx context.Context) (uint64, error) {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	var count uint64
	a.k.IterateValidators(sdkCtx, func(val *types.Validator) bool {
		if val.Tier == types.TierGuardian && val.IsActive {
			count++
		}
		return false
	})
	return count, nil
}
