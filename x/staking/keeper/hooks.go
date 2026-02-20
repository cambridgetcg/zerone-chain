package keeper

import (
	sdk "github.com/cosmos/cosmos-sdk/types"
)

// VerificationHooks is the interface that the knowledge module calls after a verification round.
type VerificationHooks interface {
	AfterVerificationCompleted(ctx sdk.Context, validatorAddr string, claimID string, correct bool, contested bool) error
}

// StakingVerificationHooks implements VerificationHooks by delegating to the staking keeper.
type StakingVerificationHooks struct {
	k Keeper
}

// NewStakingVerificationHooks creates a new hooks instance.
func NewStakingVerificationHooks(k Keeper) StakingVerificationHooks {
	return StakingVerificationHooks{k: k}
}

// AfterVerificationCompleted records the verification result for the validator (P0-2 fix: passes contested).
func (h StakingVerificationHooks) AfterVerificationCompleted(ctx sdk.Context, validatorAddr string, _ string, correct bool, contested bool) error {
	h.k.RecordVerification(ctx, validatorAddr, correct, contested)
	return nil
}
