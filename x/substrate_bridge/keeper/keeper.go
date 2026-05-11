package keeper

import (
	"context"

	"cosmossdk.io/log"
	storetypes "cosmossdk.io/store/types"
	"github.com/cosmos/cosmos-sdk/codec"

	"github.com/zerone-chain/zerone/x/substrate_bridge/types"
)

type Keeper struct {
	cdc      codec.BinaryCodec
	storeKey storetypes.StoreKey
	authority string

	knowledgeKeeper     types.KnowledgeKeeper
	qualificationKeeper types.QualificationKeeper
	bankKeeper          types.BankKeeper
	accountKeeper       types.AccountKeeper
}

func NewKeeper(
	cdc codec.BinaryCodec,
	storeKey storetypes.StoreKey,
	authority string,
	kk types.KnowledgeKeeper,
	qk types.QualificationKeeper,
	bk types.BankKeeper,
	ak types.AccountKeeper,
) Keeper {
	return Keeper{cdc: cdc, storeKey: storeKey, authority: authority,
		knowledgeKeeper: kk, qualificationKeeper: qk, bankKeeper: bk, accountKeeper: ak}
}

func (k Keeper) Authority() string { return k.authority }
func (k Keeper) Logger(ctx interface{ Logger() log.Logger }) log.Logger {
	return ctx.Logger().With("module", "x/"+types.ModuleName)
}

// TODO: STUB methods replaced in SB-12 (adapter_registry.go).
// These exist only so keeper/genesis.go compiles before SB-12 lands.

// WriteAdapter persists an AdapterRegistration. STUB: replaced in SB-12.
func (k Keeper) WriteAdapter(ctx context.Context, a *types.AdapterRegistration) error {
	return nil
}

// IterateAdapters walks every AdapterRegistration in the store. STUB: replaced in SB-12.
func (k Keeper) IterateAdapters(ctx context.Context, cb func(*types.AdapterRegistration) bool) {
}
