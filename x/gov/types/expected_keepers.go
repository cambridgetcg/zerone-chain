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
}

// BankKeeper defines the bank module interface required by governance.
type BankKeeper interface {
	SendCoinsFromAccountToModule(ctx context.Context, senderAddr sdk.AccAddress, recipientModule string, amt sdk.Coins) error
	SendCoinsFromModuleToAccount(ctx context.Context, senderModule string, recipientAddr sdk.AccAddress, amt sdk.Coins) error
}

// VestingRewardsKeeper defines the vesting rewards module interface for research fund disbursement.
type VestingRewardsKeeper interface {
	DisburseFromResearchFund(ctx sdk.Context, recipient sdk.AccAddress, amount sdk.Coins) error
}

// UpgradeKeeper defines the upgrade module interface for scheduling software upgrades.
// When an upgrade-category LIP passes with an attached plan, ScheduleUpgrade halts
// the chain at the specified height so validators can swap binaries.
type UpgradeKeeper interface {
	ScheduleUpgrade(ctx context.Context, plan UpgradePlan) error
}

// FundingRecorder is the interface used by the SybilFundingDecorator to record
// sender->recipient funding relationships for sybil vote-weight decay.
type FundingRecorder interface {
	RecordFunding(ctx sdk.Context, sender, recipient, amount string, blockHeight uint64)
}
