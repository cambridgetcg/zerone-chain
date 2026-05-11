package types

import (
	"context"

	sdk "github.com/cosmos/cosmos-sdk/types"

	knowledgetypes "github.com/zerone-chain/zerone/x/knowledge/types"
	qualificationtypes "github.com/zerone-chain/zerone/x/qualification/types"
)

// KnowledgeKeeper is the subset of x/knowledge.Keeper used by
// substrate_bridge. PendingClaim auto-submission and CitedFact existence
// checks go through here. Implementations: x/knowledge/keeper.Keeper.
type KnowledgeKeeper interface {
	GetFact(ctx context.Context, factID string) (*knowledgetypes.Fact, bool)
	GetClaim(ctx context.Context, claimID string) (*knowledgetypes.Claim, bool)
	SetClaim(ctx context.Context, claim *knowledgetypes.Claim) error
}

// QualificationKeeper is the subset of x/qualification.Keeper used
// for submitter qualification checks.
type QualificationKeeper interface {
	GetDomainQualification(ctx context.Context, address, domain string) (qualificationtypes.DomainQualification, bool)
}

// BankKeeper escrows submitter bonds and disburses rewards.
type BankKeeper interface {
	SendCoinsFromAccountToModule(ctx context.Context, sender sdk.AccAddress, recipientModule string, coins sdk.Coins) error
	SendCoinsFromModuleToAccount(ctx context.Context, senderModule string, recipient sdk.AccAddress, coins sdk.Coins) error
	BurnCoins(ctx context.Context, moduleName string, coins sdk.Coins) error
}

// AccountKeeper materializes the module account for bond escrow.
type AccountKeeper interface {
	GetModuleAddress(name string) sdk.AccAddress
}
