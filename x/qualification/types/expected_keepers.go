package types

import (
	"context"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

// BankKeeper defines the expected bank module interface.
type BankKeeper interface {
	SendCoinsFromAccountToModule(ctx context.Context, senderAddr sdk.AccAddress, recipientModule string, amt sdk.Coins) error
	SendCoinsFromModuleToAccount(ctx context.Context, senderModule string, recipientAddr sdk.AccAddress, amt sdk.Coins) error
}

// StakingKeeper defines the expected staking interface for validator checks.
type StakingKeeper interface {
	IsValidator(ctx context.Context, addr string) bool
}

// DomainReputation is the reputation data returned by CaptureDefenseKeeper.
type DomainReputation struct {
	Score      uint64
	TotalStake string
}

// CaptureDefenseKeeper defines the expected capture defense module interface.
// This module does not exist yet — the keeper field is nil-safe.
type CaptureDefenseKeeper interface {
	GetDomainReputation(ctx context.Context, validator string, domain string) (*DomainReputation, bool)
}

// OntologyKeeper defines the expected ontology module interface for domain depth lookups.
type OntologyKeeper interface {
	GetDepthForDomain(ctx context.Context, domainName string) (uint32, error)
}
