package types

import (
	"context"
)

// KnowledgeKeeper provides access to knowledge module state for verification data.
type KnowledgeKeeper interface {
	GetFactDomain(ctx context.Context, factId string) (string, bool)
	GetFactSubmitter(ctx context.Context, factId string) (string, bool)
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
