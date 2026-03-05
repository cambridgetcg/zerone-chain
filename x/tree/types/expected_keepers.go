package types

import (
	"context"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

// BankKeeper defines the expected bank module interface.
type BankKeeper interface {
	SendCoins(ctx context.Context, fromAddr, toAddr sdk.AccAddress, amt sdk.Coins) error
	SendCoinsFromAccountToModule(ctx context.Context, senderAddr sdk.AccAddress, recipientModule string, amt sdk.Coins) error
	SendCoinsFromModuleToAccount(ctx context.Context, senderModule string, recipientAddr sdk.AccAddress, amt sdk.Coins) error
	SendCoinsFromModuleToModule(ctx context.Context, senderModule, recipientModule string, amt sdk.Coins) error
}

// ResearchFundDepositor routes deposits to the research fund with founder auto-split.
// Satisfied by vesting_rewards.Keeper.
type ResearchFundDepositor interface {
	DepositToResearchFund(ctx context.Context, sourceModule string, amount sdk.Coins) error
}

// ChannelsKeeper defines the expected channels module interface for channel-gated service calls.
// Nil-safe — set post-init when x/channels is wired.
type ChannelsKeeper interface {
	GetChannelInfo(ctx context.Context, channelId string) (payer, provider, available, status string, found bool)
	SpendFromChannel(ctx context.Context, channelId string, amount string, recipientModule string) error
}

// KnowledgeKeeper defines the expected knowledge module interface for data collection campaigns.
// Nil-safe — set post-init when x/knowledge is wired.
type KnowledgeKeeper interface {
	// CreateProjectBounty creates a data bounty linked to a tree project.
	CreateProjectBounty(ctx context.Context, domain string, targetCount uint64, minQuality uint64, budget sdk.Coins, projectID string) error
	// GetBountyProgress returns current submission count for a project bounty.
	GetBountyProgress(ctx context.Context, projectID string) (current uint64, target uint64, found bool)
}
