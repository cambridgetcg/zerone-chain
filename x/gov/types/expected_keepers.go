package types

import (
	"context"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

// StakingKeeper defines the staking module interface required by governance.
type StakingKeeper interface {
	// GetTotalBondedStake returns the total bonded stake as a decimal string.
	GetTotalBondedStake(ctx context.Context) (string, error)
	// GetDelegatorTotalBonded returns the total bonded tokens for a delegator as a decimal string.
	GetDelegatorTotalBonded(ctx context.Context, addr string) (string, error)
	// CountActiveGuardians returns the number of active Guardian-tier validators.
	CountActiveGuardians(ctx context.Context) (uint64, error)
	// IsGuardian returns true if the address is Guardian tier (tier 4) and active.
	IsGuardian(ctx context.Context, addr string) (bool, error)
}

// BankKeeper defines the bank module interface required by governance.
type BankKeeper interface {
	SendCoinsFromAccountToModule(ctx context.Context, senderAddr sdk.AccAddress, recipientModule string, amt sdk.Coins) error
	SendCoinsFromModuleToAccount(ctx context.Context, senderModule string, recipientAddr sdk.AccAddress, amt sdk.Coins) error
	GetAllBalances(ctx context.Context, addr sdk.AccAddress) sdk.Coins
}

// VestingRewardsKeeper defines the vesting rewards module interface for research fund disbursement.
type VestingRewardsKeeper interface {
	DisburseFromResearchFund(ctx sdk.Context, recipient sdk.AccAddress, amount sdk.Coins) error
}

// UpgradeKeeper defines the upgrade module interface for scheduling software upgrades.
type UpgradeKeeper interface {
	ScheduleUpgrade(ctx context.Context, plan *UpgradePlan) error
}

// ParamRouter dispatches parameter changes from passed LIPs to the target module keepers.
type ParamRouter interface {
	ApplyParamChange(ctx context.Context, module, key, value string) error
}

// FundingRecorder is the interface used by the SybilFundingDecorator.
type FundingRecorder interface {
	RecordFunding(ctx sdk.Context, sender, recipient, amount string, blockHeight uint64)
}

// EmergencyKeeper defines the emergency module interface for governance condition checking.
type EmergencyKeeper interface {
	CountHaltsForReason(ctx context.Context, reason string) uint64
}
