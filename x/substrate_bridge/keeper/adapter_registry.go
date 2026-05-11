package keeper

import (
	"context"

	storetypes "cosmossdk.io/store/types"
	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/zerone-chain/zerone/x/substrate_bridge/types"
)

// WriteAdapter persists an AdapterRegistration to the store.
// Returns ErrAdapterTombstoned if an adapter with the same ID is already
// tombstoned (commitment 10: forward-only tombstone). Maintains the
// (status, id) reverse index at 0x89, deleting old-status entries on
// status transition.
func (k Keeper) WriteAdapter(ctx context.Context, a *types.AdapterRegistration) error {
	if a == nil || a.AdapterId == "" {
		return types.ErrAdapterNotFound
	}
	existing, found := k.GetAdapter(ctx, a.AdapterId)
	if found && existing.Status == types.AdapterStatus_ADAPTER_STATUS_TOMBSTONED {
		return types.ErrAdapterTombstoned
	}
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	kvStore := sdkCtx.KVStore(k.storeKey)
	// Delete old status-index entry when status changes.
	if found && existing.Status != a.Status {
		kvStore.Delete(types.AdapterByStatusKey(uint8(existing.Status), a.AdapterId))
	}
	kvStore.Set(types.AdapterKey(a.AdapterId), k.cdc.MustMarshal(a))
	kvStore.Set(types.AdapterByStatusKey(uint8(a.Status), a.AdapterId), []byte{0x01})
	return nil
}

// GetAdapter retrieves an AdapterRegistration by adapter ID.
// Returns (nil, false) when not found.
func (k Keeper) GetAdapter(ctx context.Context, adapterID string) (*types.AdapterRegistration, bool) {
	kvStore := sdk.UnwrapSDKContext(ctx).KVStore(k.storeKey)
	bz := kvStore.Get(types.AdapterKey(adapterID))
	if bz == nil {
		return nil, false
	}
	var a types.AdapterRegistration
	if err := k.cdc.Unmarshal(bz, &a); err != nil {
		return nil, false
	}
	return &a, true
}

// IterateAdapters walks every AdapterRegistration in insertion order.
// Returning true from cb stops iteration early.
func (k Keeper) IterateAdapters(ctx context.Context, cb func(*types.AdapterRegistration) bool) {
	kvStore := sdk.UnwrapSDKContext(ctx).KVStore(k.storeKey)
	iter := storetypes.KVStorePrefixIterator(kvStore, types.AdapterRegistrationPrefix)
	defer iter.Close()
	for ; iter.Valid(); iter.Next() {
		var a types.AdapterRegistration
		if err := k.cdc.Unmarshal(iter.Value(), &a); err != nil {
			continue
		}
		if cb(&a) {
			return
		}
	}
}

// IterateAdaptersByStatus walks every AdapterRegistration with the given status.
// Uses the 0x89 reverse index for O(|matching|) scans instead of O(|all|).
// Returning true from cb stops iteration early.
func (k Keeper) IterateAdaptersByStatus(ctx context.Context, status types.AdapterStatus, cb func(*types.AdapterRegistration) bool) {
	kvStore := sdk.UnwrapSDKContext(ctx).KVStore(k.storeKey)
	prefix := append([]byte{}, types.AdapterByStatusPrefix...)
	prefix = append(prefix, uint8(status))
	iter := storetypes.KVStorePrefixIterator(kvStore, prefix)
	defer iter.Close()
	for ; iter.Valid(); iter.Next() {
		// Key layout: 0x89 | status_byte | adapterID
		key := iter.Key()
		// Skip the prefix (1 byte status); remainder is adapterID.
		adapterID := string(key[1:])
		a, found := k.GetAdapter(ctx, adapterID)
		if !found {
			continue
		}
		if cb(a) {
			return
		}
	}
}

// SuspendAdapter transitions an ACTIVE adapter to SUSPENDED.
// Returns ErrAdapterNotFound or ErrAdapterTombstoned for terminal states.
func (k Keeper) SuspendAdapter(ctx context.Context, adapterID, reason string) error {
	a, found := k.GetAdapter(ctx, adapterID)
	if !found {
		return types.ErrAdapterNotFound
	}
	if a.Status == types.AdapterStatus_ADAPTER_STATUS_TOMBSTONED {
		return types.ErrAdapterTombstoned
	}
	a.Status = types.AdapterStatus_ADAPTER_STATUS_SUSPENDED
	return k.WriteAdapter(ctx, a)
}

// WriteAdapterFromGov is the governance-dispatch entry point for the
// CategoryAdapterRegistration LIP class. It deserialises the proto-encoded
// AdapterRegistration bytes that were attached to the LIP and delegates to
// WriteAdapter. lipID is stored on the adapter record for on-chain audit
// (it overrides any LipId already present in the payload).
//
// Phase-0 note: full payload-attachment wiring is deferred to Phase-1.
// The method is present so that x/gov.SubstrateBridgeKeeper is satisfied
// and the gov keeper can hold a typed reference now.
func (k Keeper) WriteAdapterFromGov(ctx context.Context, lipID string, adapterProtoBytes []byte) error {
	var a types.AdapterRegistration
	if err := k.cdc.Unmarshal(adapterProtoBytes, &a); err != nil {
		return err
	}
	// Stamp the authorising LIP ID on the record for auditability.
	a.RegisteredViaLipId = lipID
	return k.WriteAdapter(ctx, &a)
}

// TombstoneAdapter permanently retires an adapter (commitment 10: forward-only).
// Sets TombstonedAtBlock = current block height. After tombstoning, WriteAdapter
// will refuse any re-registration with the same adapter ID.
func (k Keeper) TombstoneAdapter(ctx context.Context, adapterID string) error {
	a, found := k.GetAdapter(ctx, adapterID)
	if !found {
		return types.ErrAdapterNotFound
	}
	if a.Status == types.AdapterStatus_ADAPTER_STATUS_TOMBSTONED {
		return types.ErrAdapterTombstoned
	}
	a.Status = types.AdapterStatus_ADAPTER_STATUS_TOMBSTONED
	a.TombstonedAtBlock = uint64(sdk.UnwrapSDKContext(ctx).BlockHeight())
	return k.WriteAdapter(ctx, a)
}
