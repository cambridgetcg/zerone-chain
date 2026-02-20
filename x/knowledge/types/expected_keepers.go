package types

import (
	"context"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

// AccountKeeper defines the expected account keeper interface.
type AccountKeeper interface {
	GetAccount(ctx context.Context, addr sdk.AccAddress) sdk.AccountI
}

// BankKeeper defines the expected bank keeper interface.
type BankKeeper interface {
	SendCoins(ctx context.Context, fromAddr, toAddr sdk.AccAddress, amt sdk.Coins) error
	SendCoinsFromModuleToAccount(ctx context.Context, senderModule string, recipientAddr sdk.AccAddress, amt sdk.Coins) error
	SendCoinsFromAccountToModule(ctx context.Context, senderAddr sdk.AccAddress, recipientModule string, amt sdk.Coins) error
	SendCoinsFromModuleToModule(ctx context.Context, senderModule, recipientModule string, amt sdk.Coins) error
	BurnCoins(ctx context.Context, moduleName string, amt sdk.Coins) error
	GetBalance(ctx context.Context, addr sdk.AccAddress, denom string) sdk.Coin
}

// StakingKeeper defines the expected staking keeper interface.
type StakingKeeper interface {
	// GetActiveValidatorInfos returns all validators eligible for selection.
	GetActiveValidatorInfos(ctx context.Context) ([]ValidatorInfo, error)
	// GetValidatorInfo returns info for a specific validator.
	GetValidatorInfo(ctx context.Context, addr string) (*ValidatorInfo, error)
	// GetEffectiveStake returns the effective stake for a validator (including virtual stake).
	GetEffectiveStake(ctx context.Context, addr string) (uint64, error)
	// GetTotalStake returns the total staked amount across all validators.
	GetTotalStake(ctx context.Context) (uint64, error)
	// SlashValidator slashes a validator by the given BPS amount.
	SlashValidator(ctx context.Context, addr string, slashBps uint64) error
}

// OntologyKeeper defines the expected ontology keeper interface.
// Provides confidence ceilings, logic zones, and stratum definitions.
type OntologyKeeper interface {
	// GetConfidenceCeiling returns the max confidence for a stratum (0-1,000,000).
	GetConfidenceCeiling(ctx context.Context, stratum string) (uint64, error)
	// IsValidLogicZone checks if a domain is within a valid logic zone.
	IsValidLogicZone(ctx context.Context, domain string) (bool, error)
	// AcknowledgesIncompleteness checks if a domain acknowledges Gödelian limits.
	AcknowledgesIncompleteness(ctx context.Context, domain string) (bool, error)
	// GetStratumForDomain returns the stratum associated with a domain.
	GetStratumForDomain(ctx context.Context, domain string) (string, error)
}

// DomainQualificationKeeper defines the expected domain qualification keeper interface.
type DomainQualificationKeeper interface {
	// IsQualified checks if a validator is qualified for a domain.
	IsQualified(ctx context.Context, validatorAddr, domain string) (bool, error)
	// GetQualificationWeight returns the qualification weight for a validator in a domain.
	GetQualificationWeight(ctx context.Context, validatorAddr, domain string) (uint64, error)
	// GetQualifiedValidators returns all validators qualified for a domain.
	GetQualifiedValidators(ctx context.Context, domain string) ([]string, error)
	// RecordVerificationOutcome records the outcome of a verification for reputation.
	RecordVerificationOutcome(ctx context.Context, validatorAddr, domain string, accepted bool) error
}

// VestingRewardsKeeper defines the expected vesting rewards keeper interface.
type VestingRewardsKeeper interface {
	// CreateVestingScheduleFromKnowledge creates a vesting schedule for a knowledge reward.
	CreateVestingScheduleFromKnowledge(ctx context.Context, recipient string, amount sdk.Coin, claimID string) error
	// DistributeFalsificationReward distributes a falsification reward.
	DistributeFalsificationReward(ctx context.Context, recipient string, amount sdk.Coin) error
	// GetEpochBlockRewardPool returns the current epoch's block reward pool.
	GetEpochBlockRewardPool(ctx context.Context) (sdk.Coin, error)
	// PauseVestingByClaimId pauses vesting schedules associated with a claim.
	PauseVestingByClaimId(ctx context.Context, claimID string) error
	// PauseAllVestingByRecipient pauses all vesting schedules for a recipient.
	PauseAllVestingByRecipient(ctx context.Context, recipient string) error
	// GetResearchShareBps returns the current research fund share in BPS.
	GetResearchShareBps(ctx context.Context) (uint64, error)
	// DepositToResearchFund deposits an amount into the research fund.
	DepositToResearchFund(ctx context.Context, amount sdk.Coin) error
}

// AutopoiesisKeeper defines the expected autopoiesis keeper interface.
type AutopoiesisKeeper interface {
	// GetMultiplier returns the autopoiesis reward multiplier for a path (BPS, 1,000,000 = 1.0×).
	GetMultiplier(ctx context.Context, path string) (uint64, error)
}
