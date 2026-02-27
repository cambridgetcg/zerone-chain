package keeper

import (
	"context"
	"fmt"
	"math/big"

	sdkmath "cosmossdk.io/math"

	sdk "github.com/cosmos/cosmos-sdk/types"

	knowledgetypes "github.com/zerone-chain/zerone/x/knowledge/types"
	"github.com/zerone-chain/zerone/x/staking/types"
)

// StakingKeeperAdapter wraps the staking Keeper to satisfy the
// knowledge module's StakingKeeper interface.
type StakingKeeperAdapter struct {
	k Keeper
}

// NewStakingKeeperAdapter returns an adapter that bridges the staking keeper
// to the knowledge module's expected interface.
func NewStakingKeeperAdapter(k Keeper) *StakingKeeperAdapter {
	return &StakingKeeperAdapter{k: k}
}

// Ensure compile-time interface compliance.
var _ knowledgetypes.StakingKeeper = (*StakingKeeperAdapter)(nil)

// GetActiveValidatorInfos returns all active validators as ValidatorInfo structs.
func (a *StakingKeeperAdapter) GetActiveValidatorInfos(ctx context.Context) ([]knowledgetypes.ValidatorInfo, error) {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	validators := a.k.GetActiveValidatorSet(sdkCtx)

	infos := make([]knowledgetypes.ValidatorInfo, 0, len(validators))
	for _, val := range validators {
		infos = append(infos, validatorToInfo(sdkCtx, a.k, val))
	}
	return infos, nil
}

// GetValidatorInfo returns info for a specific validator by operator address.
func (a *StakingKeeperAdapter) GetValidatorInfo(ctx context.Context, addr string) (*knowledgetypes.ValidatorInfo, error) {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	val, found := a.k.GetValidator(sdkCtx, addr)
	if !found {
		return nil, fmt.Errorf("validator %s not found", addr)
	}
	info := validatorToInfo(sdkCtx, a.k, val)
	return &info, nil
}

// GetEffectiveStake returns the effective selection stake for a validator as uint64.
func (a *StakingKeeperAdapter) GetEffectiveStake(ctx context.Context, addr string) (uint64, error) {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	val, found := a.k.GetValidator(sdkCtx, addr)
	if !found {
		return 0, fmt.Errorf("validator %s not found", addr)
	}
	effective := a.k.GetEffectiveSelectionStake(sdkCtx, val)
	if !effective.IsUint64() {
		return ^uint64(0), nil // cap at max uint64
	}
	return effective.Uint64(), nil
}

// GetTotalStake returns the total bonded stake across all active validators as uint64.
func (a *StakingKeeperAdapter) GetTotalStake(ctx context.Context) (uint64, error) {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	total := a.k.GetTotalBondedStake(sdkCtx)
	if !total.IsUint64() {
		return ^uint64(0), nil
	}
	return total.Uint64(), nil
}

// SlashValidator slashes a validator by the given BPS amount.
// Converts slashBps to an absolute amount: amount = effectiveStake * slashBps / 1_000_000.
func (a *StakingKeeperAdapter) SlashValidator(ctx context.Context, addr string, slashBps uint64) error {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	val, found := a.k.GetValidator(sdkCtx, addr)
	if !found {
		return fmt.Errorf("validator %s not found", addr)
	}

	totalStake, _ := new(big.Int).SetString(val.TotalStake, 10)
	if totalStake == nil || totalStake.Sign() <= 0 {
		return nil // nothing to slash
	}

	amount := new(big.Int).Mul(totalStake, new(big.Int).SetUint64(slashBps))
	amount.Div(amount, new(big.Int).SetUint64(1_000_000))

	if amount.Sign() <= 0 {
		return nil
	}

	a.k.SlashValidator(sdkCtx, addr, amount, "knowledge_verification")
	return nil
}

// SlashValidatorToModule slashes a validator and routes slashed tokens to a specified module account.
// Returns the actual slashed amount (after escalation and SSI adjustments).
func (a *StakingKeeperAdapter) SlashValidatorToModule(ctx context.Context, addr string, slashBps uint64, destModule string) (sdkmath.Int, error) {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	val, found := a.k.GetValidator(sdkCtx, addr)
	if !found {
		return sdkmath.ZeroInt(), fmt.Errorf("validator %s not found", addr)
	}

	totalStake, _ := new(big.Int).SetString(val.TotalStake, 10)
	if totalStake == nil || totalStake.Sign() <= 0 {
		return sdkmath.ZeroInt(), nil
	}

	amount := new(big.Int).Mul(totalStake, new(big.Int).SetUint64(slashBps))
	amount.Div(amount, new(big.Int).SetUint64(1_000_000))

	if amount.Sign() <= 0 {
		return sdkmath.ZeroInt(), nil
	}

	slashed := a.k.SlashValidatorToModule(sdkCtx, addr, amount, destModule, "knowledge_verification")
	return sdkmath.NewIntFromBigInt(slashed), nil
}

// validatorToInfo converts a staking Validator to a knowledge ValidatorInfo.
func validatorToInfo(ctx sdk.Context, k Keeper, val *types.Validator) knowledgetypes.ValidatorInfo {
	effective := k.GetEffectiveSelectionStake(ctx, val)
	var stakeUint64 uint64
	if effective.IsUint64() {
		stakeUint64 = effective.Uint64()
	} else {
		stakeUint64 = ^uint64(0)
	}

	var accuracyBps uint64
	if val.TotalVerifications > 0 {
		accuracyBps = val.CorrectVerifications * 1_000_000 / val.TotalVerifications
	}

	return knowledgetypes.ValidatorInfo{
		Address:           val.OperatorAddress,
		Stake:             stakeUint64,
		Tier:              types.ValidatorTierString(val.Tier),
		VerificationCount: val.TotalVerifications,
		AccuracyBps:       accuracyBps,
	}
}
