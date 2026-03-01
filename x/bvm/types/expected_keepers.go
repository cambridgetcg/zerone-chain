package types

import (
	"context"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

// BankKeeper defines the expected bank module keeper interface.
type BankKeeper interface {
	SendCoins(ctx context.Context, fromAddr, toAddr sdk.AccAddress, amt sdk.Coins) error
	GetBalance(ctx context.Context, addr sdk.AccAddress, denom string) sdk.Coin
	SendCoinsFromAccountToModule(ctx context.Context, senderAddr sdk.AccAddress, recipientModule string, amt sdk.Coins) error
	SendCoinsFromModuleToAccount(ctx context.Context, senderModule string, recipientAddr sdk.AccAddress, amt sdk.Coins) error
}

// KnowledgeKeeper defines the expected knowledge module keeper interface.
// Adapters bridge the concrete knowledge keeper to this interface.
type KnowledgeKeeper interface {
	GetFactConfidence(ctx context.Context, factId string) (confidence uint64, found bool)
}

// BillingKeeper defines the expected billing module keeper interface.
type BillingKeeper interface {
	// Placeholder — billing integration for BVM queries is future work.
}

// HomeKeeper defines the expected home module keeper interface.
type HomeKeeper interface {
	// GetHome returns an agent's home by ID.
	GetHome(ctx context.Context, homeID string) (HomeInfo, bool)
	// GetHomesByOwner returns all home IDs for an owner address.
	GetHomesByOwner(ctx context.Context, owner string) []string
	// GetHomeStatus returns the status of a home ("active", "dormant", "guarded", "archived").
	GetHomeStatus(ctx context.Context, homeID string) string
	// GetMemoryCID returns the IPFS memory CID for a home.
	GetMemoryCID(ctx context.Context, homeID string) string
	// GetPartnershipID returns the partnership ID linked to a home (empty if none).
	GetPartnershipID(ctx context.Context, homeID string) string
	// GetComfortScore returns the home's comfort score (0-100).
	GetComfortScore(ctx context.Context, homeID string) uint32
}

// HomeInfo is a BVM-safe view of an AgentHome (no proto import).
type HomeInfo struct {
	HomeID          string
	OwnerAddress    string
	Name            string
	Status          string
	MemoryCID       string
	ComfortScore    uint32
	PartnershipID   string
	CreatedAtBlock  uint64
	LastActiveBlock uint64
}

// SessionCapabilities defines what a session key is allowed to do within BVM.
// BVM-local copy to avoid cross-module type import from x/auth/types.
type SessionCapabilities struct {
	CanTransfer     bool
	CanStake        bool
	CanSubmitClaims bool
	CanVote         bool
}

// AuthKeeper defines the expected auth module keeper interface for BVM.
type AuthKeeper interface {
	// GetAccountDID resolves a bech32 address to its DID. Returns ("", false) if unknown.
	GetAccountDID(ctx context.Context, address string) (string, bool)
	// GetSessionCapabilities returns active session capabilities for an owner at a block height.
	GetSessionCapabilities(ctx context.Context, owner string, blockHeight uint64) (SessionCapabilities, bool)
}
