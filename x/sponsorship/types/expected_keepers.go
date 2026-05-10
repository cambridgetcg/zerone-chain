package types

import (
	"context"

	sdk "github.com/cosmos/cosmos-sdk/types"

	knowledgetypes "github.com/zerone-chain/zerone/x/knowledge/types"
)

// KnowledgeKeeper is the read-only view this module needs of x/knowledge.
// Sponsorship never modifies facts; it only checks their status, domain,
// submission block, and submitter before paying out.
type KnowledgeKeeper interface {
	GetFact(ctx context.Context, factID string) (*knowledgetypes.Fact, bool)
}

// BankKeeper handles escrow movement: sponsor → module account on create,
// module account → worker on fulfill, module account → sponsor on cancel.
// No mint, no burn — every coin flowing through this module already
// existed when the sponsor escrowed it.
type BankKeeper interface {
	SendCoinsFromAccountToModule(ctx context.Context, senderAddr sdk.AccAddress, recipientModule string, amt sdk.Coins) error
	SendCoinsFromModuleToAccount(ctx context.Context, senderModule string, recipientAddr sdk.AccAddress, amt sdk.Coins) error
	SpendableCoins(ctx context.Context, addr sdk.AccAddress) sdk.Coins
}
