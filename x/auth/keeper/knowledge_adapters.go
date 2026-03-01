package keeper

import (
	"context"

	sdk "github.com/cosmos/cosmos-sdk/types"

	knowledgetypes "github.com/zerone-chain/zerone/x/knowledge/types"
	partnershipstypes "github.com/zerone-chain/zerone/x/partnerships/types"
)

// KnowledgeAuthAdapter wraps the zerone auth Keeper to satisfy
// knowledgetypes.ZeroneAuthKeeper and partnershipstypes.ZeroneAuthKeeper.
type KnowledgeAuthAdapter struct {
	k Keeper
}

// NewKnowledgeAuthAdapter returns an adapter for the knowledge and partnerships modules.
func NewKnowledgeAuthAdapter(k Keeper) *KnowledgeAuthAdapter {
	return &KnowledgeAuthAdapter{k: k}
}

var _ knowledgetypes.ZeroneAuthKeeper = (*KnowledgeAuthAdapter)(nil)
var _ partnershipstypes.ZeroneAuthKeeper = (*KnowledgeAuthAdapter)(nil)

// GetAccountType returns the account type for a bech32 address.
func (a *KnowledgeAuthAdapter) GetAccountType(goCtx context.Context, address string) (string, bool) {
	ctx := sdk.UnwrapSDKContext(goCtx)
	account, found := a.k.GetAccount(ctx, address)
	if !found {
		return "", false
	}
	return account.AccountType, true
}
