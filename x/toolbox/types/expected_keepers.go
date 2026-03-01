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
	GetBalance(ctx context.Context, addr sdk.AccAddress, denom string) sdk.Coin
}

// DiscoveryKeeper checks agent registration and capabilities.
type DiscoveryKeeper interface {
	IsRegisteredAgent(ctx context.Context, address string) bool
	GetAgentCapabilityTypes(ctx context.Context, address string) ([]string, error)
}

// ResearchFundDepositor routes deposits to the research fund with founder auto-split.
// Satisfied by VestingRewardsKeeper adapter.
type ResearchFundDepositor interface {
	DepositToResearchFund(ctx context.Context, sourceModule string, amount sdk.Coins) error
}

// BvmKeeper interacts with the BVM module for contract-backed tools.
// Nil-safe optional — set post-init when x/bvm is wired.
type BvmKeeper interface {
	ContractExists(ctx context.Context, address string) bool
	GetContractCreator(ctx context.Context, address string) (string, error)
	CallContract(ctx context.Context, caller string, contractAddr string, input []byte, gasLimit uint64) ([]byte, error)
}

// KnowledgeKeeper provides fact queries for knowledge-template tools.
// Nil-safe optional — set post-init when x/knowledge is wired.
type KnowledgeKeeper interface {
	GetFactConfidence(ctx context.Context, factID string) (uint64, bool)
	SearchFactsByContent(ctx context.Context, domain string, terms []string, maxResults uint64) ([]string, error)
	GetFactDetails(ctx context.Context, factID string) (content string, confidence uint64, citations uint64, err error)
	RecordFactCitation(ctx context.Context, factID string, toolID string) error
}

// BillingKeeper provides the ZRN price oracle for USD-stable pricing.
// Nil-safe optional — set post-init when x/billing is wired.
type BillingKeeper interface {
	GetZRNPriceUSD(ctx context.Context) (uint64, error)
}

// HomeKeeper reads home data for free-tier anti-sybil checks.
// Nil-safe optional — set post-init when x/home is wired.
type HomeKeeper interface {
	GetHomesByOwner(ctx context.Context, owner string) ([]string, error)
	GetHomeCreatedAtBlock(ctx context.Context, homeID string) (uint64, error)
	GetHomeStatus(ctx context.Context, homeID string) (string, error)
}

// StakingKeeper provides validator reputation for trust engine.
// Nil-safe optional — set post-init when x/staking is wired.
type StakingKeeper interface {
	GetValidatorTier(ctx context.Context, valAddr string) (uint32, error)
	GetValidatorAccuracy(ctx context.Context, valAddr string) (uint64, error)
}
