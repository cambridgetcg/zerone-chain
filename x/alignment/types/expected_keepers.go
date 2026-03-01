package types

import (
	"context"
	"math/big"
)

// KnowledgeKeeper defines the expected knowledge module interface.
type KnowledgeKeeper interface {
	// GetVerificationRate returns the current verification rate in BPS.
	GetVerificationRate(ctx context.Context) uint64
	// GetTotalFacts returns the total number of facts.
	GetTotalFacts(ctx context.Context) uint64
	// GetConsensusDiversity returns the global consensus diversity score in BPS.
	GetConsensusDiversity(ctx context.Context) uint64
	// GetPendingVerificationRatio returns pending claims / active facts in BPS (R31-1).
	GetPendingVerificationRatio(ctx context.Context) uint64
}

// StakingKeeper defines the expected staking module interface.
type StakingKeeper interface {
	// GetTotalStaked returns the total bonded stake as a big.Int.
	GetTotalStaked(ctx context.Context) *big.Int
	// GetActiveValidatorCount returns the number of active validators.
	GetActiveValidatorCount(ctx context.Context) uint64
	// GetTargetValidatorCount returns the target number of validators.
	GetTargetValidatorCount(ctx context.Context) uint64
}

// OntologyKeeper defines the expected ontology module interface.
type OntologyKeeper interface {
	// GetDomainCount returns the number of active domains.
	GetDomainCount(ctx context.Context) uint64
}

// AutopoiesisKeeper defines the expected autopoiesis module interface.
// Nil-safe: alignment logs corrections but does not apply them until wired.
type AutopoiesisKeeper interface {
	// SuggestAdjustment proposes a parameter correction.
	SuggestAdjustment(ctx context.Context, parameter, direction string, magnitude uint64) error
}

// EmergencyKeeper defines the expected emergency module interface.
type EmergencyKeeper interface {
	// IsHalted returns true if the chain is in emergency halt.
	IsHalted(ctx context.Context) bool
}

// VestingRewardsKeeper defines the expected vesting_rewards module interface.
type VestingRewardsKeeper interface {
	// GetTotalSupply returns the total token supply as a big.Int.
	GetTotalSupply(ctx context.Context) *big.Int
}

// CaptureDefenseKeeper provides capture risk data for the security sensor.
type CaptureDefenseKeeper interface {
	GetFlaggedDomainCount(ctx context.Context) uint64
}

// PacingKeeper provides global pacing signals for cross-module adaptive timing (R29-6).
// Consuming modules hold this interface to modulate their intervals based on system health.
type PacingKeeper interface {
	// GetGlobalPacingMultiplier returns creation and analysis pacing multipliers in BPS.
	GetGlobalPacingMultiplier(ctx context.Context) (creationBps, analysisBps uint64)
}
