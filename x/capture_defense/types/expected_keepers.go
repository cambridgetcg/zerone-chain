package types

import (
	"context"
)

// KnowledgeKeeper provides access to knowledge module state for verification data.
type KnowledgeKeeper interface {
	GetFactDomain(ctx context.Context, factId string) (string, bool)
	GetFactSubmitter(ctx context.Context, factId string) (string, bool)
	GetDomainVerificationActivity(ctx context.Context, domain string) uint64 // R31-4
}

// StakingKeeper provides access to staking module state for validator info.
type StakingKeeper interface {
	IsActiveValidator(ctx context.Context, valAddr string) (bool, error)
	GetValidatorStake(ctx context.Context, valAddr string) (string, error)
}

// OntologyKeeper provides access to ontology module state for domain depth lookups.
type OntologyKeeper interface {
	GetDepthForDomain(ctx context.Context, domainName string) (uint32, error)
}

// CaptureChallengeKeeper allows capture_defense to auto-submit challenges.
type CaptureChallengeKeeper interface {
	AutoSubmitChallenge(ctx context.Context, domain string, riskScore uint64, hhi uint64, evidence string) error
}

// PartnershipsKeeper provides access to partnerships module for structural immunity (R29-5).
type PartnershipsKeeper interface {
	GetDomainPartnershipDensity(ctx context.Context, domain string) uint64
	SetDomainFormationBonus(ctx context.Context, domain string, bonusBps uint64, reason string, expiryHeight uint64)
	GetPartnershipCountByParticipant(ctx context.Context, addr string, domain string) uint64
}

// PacingKeeper provides global pacing signals for adaptive analysis timing (R29-6).
type PacingKeeper interface {
	GetGlobalPacingMultiplier(ctx context.Context) (creationBps, analysisBps uint64)
}
