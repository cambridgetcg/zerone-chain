package types

import (
	"context"

	knowledgetypes "github.com/zerone-chain/zerone/x/knowledge/types"
	qualificationtypes "github.com/zerone-chain/zerone/x/qualification/types"
)

// KnowledgeKeeper exposes the calibration record this module needs.
type KnowledgeKeeper interface {
	GetAgentCalibration(ctx context.Context, addr string) (*knowledgetypes.AgentCalibration, bool)
}

// QualificationKeeper exposes the per-domain records and effective
// weight (penalty-adjusted, since GetQualificationWeight already
// consults the penalty store as of 63738ac).
type QualificationKeeper interface {
	IterateQualifications(ctx context.Context, cb func(*qualificationtypes.DomainQualification) bool)
	GetQualificationWeight(ctx context.Context, validator, domain string) uint32
	GetActiveQualificationPenalty(ctx context.Context, validator, domain string) (*qualificationtypes.QualificationPenalty, bool)
}

// CaptureChallengeKeeper exposes per-address strike counts via the
// adapter installed in app.go. Returns the number of UPHELD challenges
// in which `addr` appears among AccusedValidators.
type CaptureChallengeKeeper interface {
	CountUpheldStrikesAgainst(ctx context.Context, addr string) uint32
}
