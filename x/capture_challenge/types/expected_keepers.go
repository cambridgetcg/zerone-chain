package types

import (
	"context"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

// BankKeeper defines the expected bank module interface.
type BankKeeper interface {
	SendCoins(ctx context.Context, fromAddr sdk.AccAddress, toAddr sdk.AccAddress, amt sdk.Coins) error
	SendCoinsFromAccountToModule(ctx context.Context, senderAddr sdk.AccAddress, recipientModule string, amt sdk.Coins) error
	SendCoinsFromModuleToAccount(ctx context.Context, senderModule string, recipientAddr sdk.AccAddress, amt sdk.Coins) error
	SendCoinsFromModuleToModule(ctx context.Context, senderModule string, recipientModule string, amt sdk.Coins) error
}

// CaptureDefenseKeeper provides capture risk analysis.
type CaptureDefenseKeeper interface {
	GetCaptureMetrics(ctx context.Context, domain string) (*CaptureMetricsData, bool)
	ClearCaptureFlag(ctx context.Context, domain string)
}

// DomainQualificationKeeper allows reducing qualification weight on confirmed capture.
type DomainQualificationKeeper interface {
	ReduceQualificationWeight(ctx context.Context, validator string, domain string, reductionBps uint64, expiryHeight uint64) error
}

// KnowledgeKeeper allows adjusting verification thresholds and recording role impact on confirmed capture.
type KnowledgeKeeper interface {
	IncreaseVerificationThreshold(ctx context.Context, domain string, additionalVerifiers uint32, expiryHeight uint64) error
	RecordChallengeRoleImpact(ctx context.Context, factId, domain string, upheld bool) error
}

// CaptureMetricsData is a plain struct to avoid circular imports with capture_defense/types.
type CaptureMetricsData struct {
	Domain              string
	HerfindahlIndex     uint64
	TimingCorrelation   uint64
	VerdictCorrelation  uint64
	Top3Share           uint64
	RiskScore           uint64
	TotalParticipations uint64
	AnalyzedAtBlock     uint64
	Flagged             bool
}
