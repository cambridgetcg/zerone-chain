package keeper

import (
	"context"

	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/zerone-chain/zerone/x/knowledge/types"
)

// RecordQueryReceipt stores proof that an address queried a specific fact.
// Receipts are ephemeral — cleared at epoch boundaries to bound storage.
func (k Keeper) RecordQueryReceipt(ctx context.Context, rater, factID string) error {
	store := k.storeService.OpenKVStore(ctx)
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	bz := sdk.Uint64ToBigEndian(uint64(sdkCtx.BlockHeight()))
	return store.Set(types.QueryReceiptKey(rater, factID), bz)
}

// HasQueryReceipt checks if an address has a valid query receipt for a fact.
func (k Keeper) HasQueryReceipt(ctx context.Context, rater, factID string) bool {
	store := k.storeService.OpenKVStore(ctx)
	bz, err := store.Get(types.QueryReceiptKey(rater, factID))
	return err == nil && bz != nil
}

// ConsumeQueryReceipt deletes a query receipt (one rating per query).
func (k Keeper) ConsumeQueryReceipt(ctx context.Context, rater, factID string) error {
	store := k.storeService.OpenKVStore(ctx)
	return store.Delete(types.QueryReceiptKey(rater, factID))
}

// ClearQueryReceipts deletes all query receipts. Called at epoch boundaries.
func (k Keeper) ClearQueryReceipts(ctx context.Context) {
	store := k.storeService.OpenKVStore(ctx)
	iter, err := store.Iterator(types.QueryReceiptPrefix, prefixEndBytes(types.QueryReceiptPrefix))
	if err != nil {
		return
	}
	defer iter.Close()
	var keys [][]byte
	for ; iter.Valid(); iter.Next() {
		keys = append(keys, append([]byte{}, iter.Key()...))
	}
	for _, key := range keys {
		_ = store.Delete(key)
	}
}
