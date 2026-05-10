package keeper

import (
	"context"
	"encoding/binary"
	"fmt"

	"cosmossdk.io/core/store"
	"cosmossdk.io/log"

	"github.com/cosmos/cosmos-sdk/codec"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"google.golang.org/protobuf/proto"

	"github.com/zerone-chain/zerone/x/sponsorship/types"
)

type Keeper struct {
	storeService    store.KVStoreService
	cdc             codec.BinaryCodec
	bankKeeper      types.BankKeeper
	knowledgeKeeper types.KnowledgeKeeper
}

func NewKeeper(
	storeService store.KVStoreService,
	cdc codec.BinaryCodec,
	bk types.BankKeeper,
	kk types.KnowledgeKeeper,
) Keeper {
	return Keeper{
		storeService:    storeService,
		cdc:             cdc,
		bankKeeper:      bk,
		knowledgeKeeper: kk,
	}
}

func (k Keeper) Logger(ctx context.Context) log.Logger {
	return sdk.UnwrapSDKContext(ctx).Logger().With("module", "x/"+types.ModuleName)
}

// ---------- Params ----------

func (k Keeper) SetParams(ctx context.Context, params *types.Params) {
	kv := k.storeService.OpenKVStore(ctx)
	bz, err := proto.Marshal(params)
	if err != nil {
		panic(fmt.Sprintf("marshal params: %v", err))
	}
	_ = kv.Set(types.ParamsKey, bz)
}

func (k Keeper) GetParams(ctx context.Context) *types.Params {
	kv := k.storeService.OpenKVStore(ctx)
	bz, err := kv.Get(types.ParamsKey)
	if err != nil || bz == nil {
		return types.DefaultParams()
	}
	var p types.Params
	if err := proto.Unmarshal(bz, &p); err != nil {
		return types.DefaultParams()
	}
	return &p
}

// ---------- Counter ----------

func (k Keeper) nextBountyID(ctx context.Context) uint64 {
	kv := k.storeService.OpenKVStore(ctx)
	bz, err := kv.Get(types.BountyCounterKey)
	if err != nil || bz == nil {
		bz = make([]byte, 8)
	}
	n := binary.BigEndian.Uint64(bz)
	n++
	newBz := make([]byte, 8)
	binary.BigEndian.PutUint64(newBz, n)
	_ = kv.Set(types.BountyCounterKey, newBz)
	return n
}

// ---------- BountyOrder CRUD ----------

func (k Keeper) SetBountyOrder(ctx context.Context, o *types.BountyOrder) {
	kv := k.storeService.OpenKVStore(ctx)
	bz, err := proto.Marshal(o)
	if err != nil {
		panic(fmt.Sprintf("marshal bounty: %v", err))
	}
	_ = kv.Set(types.BountyOrderKey(o.Id), bz)
}

func (k Keeper) GetBountyOrder(ctx context.Context, id string) (*types.BountyOrder, bool) {
	kv := k.storeService.OpenKVStore(ctx)
	bz, err := kv.Get(types.BountyOrderKey(id))
	if err != nil || bz == nil {
		return nil, false
	}
	var o types.BountyOrder
	if err := proto.Unmarshal(bz, &o); err != nil {
		return nil, false
	}
	return &o, true
}

func (k Keeper) IterateBountyOrders(ctx context.Context, cb func(*types.BountyOrder) bool) {
	kv := k.storeService.OpenKVStore(ctx)
	iter, err := kv.Iterator(types.BountyOrderKeyPrefix, prefixEndBytes(types.BountyOrderKeyPrefix))
	if err != nil {
		return
	}
	defer iter.Close()
	for ; iter.Valid(); iter.Next() {
		var o types.BountyOrder
		if err := proto.Unmarshal(iter.Value(), &o); err != nil {
			continue
		}
		if cb(&o) {
			break
		}
	}
}

func (k Keeper) GetAllBountyOrders(ctx context.Context) []*types.BountyOrder {
	var out []*types.BountyOrder
	k.IterateBountyOrders(ctx, func(o *types.BountyOrder) bool {
		out = append(out, o)
		return false
	})
	return out
}

func (k Keeper) CountActiveBountiesBySponsor(ctx context.Context, sponsor string) uint32 {
	var n uint32
	k.IterateBountyOrders(ctx, func(o *types.BountyOrder) bool {
		if o.Status == types.BountyStatus_BOUNTY_STATUS_ACTIVE && o.Sponsor == sponsor {
			n++
		}
		return false
	})
	return n
}

// ---------- Fulfillment CRUD ----------

func (k Keeper) SetFulfillment(ctx context.Context, f *types.BountyFulfillment) {
	kv := k.storeService.OpenKVStore(ctx)
	bz, err := proto.Marshal(f)
	if err != nil {
		panic(fmt.Sprintf("marshal fulfillment: %v", err))
	}
	_ = kv.Set(types.FulfillmentKey(f.BountyId, f.FactId), bz)
}

func (k Keeper) GetFulfillment(ctx context.Context, bountyID, factID string) (*types.BountyFulfillment, bool) {
	kv := k.storeService.OpenKVStore(ctx)
	bz, err := kv.Get(types.FulfillmentKey(bountyID, factID))
	if err != nil || bz == nil {
		return nil, false
	}
	var f types.BountyFulfillment
	if err := proto.Unmarshal(bz, &f); err != nil {
		return nil, false
	}
	return &f, true
}

func (k Keeper) GetAllFulfillments(ctx context.Context) []*types.BountyFulfillment {
	kv := k.storeService.OpenKVStore(ctx)
	iter, err := kv.Iterator(types.FulfillmentKeyPrefix, prefixEndBytes(types.FulfillmentKeyPrefix))
	if err != nil {
		return nil
	}
	defer iter.Close()
	var out []*types.BountyFulfillment
	for ; iter.Valid(); iter.Next() {
		var f types.BountyFulfillment
		if err := proto.Unmarshal(iter.Value(), &f); err != nil {
			continue
		}
		out = append(out, &f)
	}
	return out
}

// ---------- BeginBlocker — Expiry Sweep ----------

// ProcessBountyExpiry flips ACTIVE bounties whose end_block has elapsed
// to EXPIRED. Unlike claiming_pot's bootstrap-pot rule, sponsorship
// bounties DO expire — the sponsor's deadline is a methodological
// commitment (commitment 1) and the chain honors it. Funds remain in
// escrow on EXPIRED bounties until the sponsor calls CancelBountyOrder
// to reclaim them.
func (k Keeper) ProcessBountyExpiry(ctx context.Context, currentBlock uint64) {
	k.IterateBountyOrders(ctx, func(o *types.BountyOrder) bool {
		if o.Status == types.BountyStatus_BOUNTY_STATUS_ACTIVE && currentBlock >= o.EndBlock {
			o.Status = types.BountyStatus_BOUNTY_STATUS_EXPIRED
			k.SetBountyOrder(ctx, o)
		}
		return false
	})
}

// ---------- Genesis ----------

func (k Keeper) InitGenesis(ctx context.Context, gs *types.GenesisState) {
	if gs.Params != nil {
		k.SetParams(ctx, gs.Params)
	}
	for _, o := range gs.Orders {
		k.SetBountyOrder(ctx, o)
	}
	for _, f := range gs.Fulfillments {
		k.SetFulfillment(ctx, f)
	}
	if gs.NextBountyId > 0 {
		kv := k.storeService.OpenKVStore(ctx)
		buf := make([]byte, 8)
		binary.BigEndian.PutUint64(buf, gs.NextBountyId-1) // counter increments before use
		_ = kv.Set(types.BountyCounterKey, buf)
	}
}

func (k Keeper) ExportGenesis(ctx context.Context) *types.GenesisState {
	return &types.GenesisState{
		Params:       k.GetParams(ctx),
		Orders:       k.GetAllBountyOrders(ctx),
		Fulfillments: k.GetAllFulfillments(ctx),
		NextBountyId: k.peekNextBountyID(ctx),
	}
}

func (k Keeper) peekNextBountyID(ctx context.Context) uint64 {
	kv := k.storeService.OpenKVStore(ctx)
	bz, err := kv.Get(types.BountyCounterKey)
	if err != nil || bz == nil {
		return 1
	}
	return binary.BigEndian.Uint64(bz) + 1
}

// ---------- helpers ----------

func prefixEndBytes(prefix []byte) []byte {
	if len(prefix) == 0 {
		return nil
	}
	end := make([]byte, len(prefix))
	copy(end, prefix)
	for i := len(end) - 1; i >= 0; i-- {
		end[i]++
		if end[i] != 0 {
			return end
		}
	}
	return nil
}
