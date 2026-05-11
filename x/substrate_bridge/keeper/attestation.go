package keeper

import (
	"context"
	"encoding/binary"
	"fmt"

	storetypes "cosmossdk.io/store/types"
	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/zerone-chain/zerone/x/substrate_bridge/types"
)

func (k Keeper) WriteAttestation(ctx context.Context, att *types.ExternalAttestation) error {
	if att == nil || att.AttestationId == "" {
		return types.ErrAttestationNotFound
	}
	store := sdk.UnwrapSDKContext(ctx).KVStore(k.storeKey)
	if existing, found := k.GetAttestation(ctx, att.AttestationId); found && existing.Status != att.Status {
		store.Delete(types.AttestationByStatusKey(uint8(existing.Status), att.AttestationId))
	}
	store.Set(types.AttestationKey(att.AttestationId), k.cdc.MustMarshal(att))
	store.Set(types.AttestationByStatusKey(uint8(att.Status), att.AttestationId), []byte{0x01})
	return nil
}

func (k Keeper) GetAttestation(ctx context.Context, attestationID string) (*types.ExternalAttestation, bool) {
	store := sdk.UnwrapSDKContext(ctx).KVStore(k.storeKey)
	bz := store.Get(types.AttestationKey(attestationID))
	if bz == nil {
		return nil, false
	}
	var att types.ExternalAttestation
	if err := k.cdc.Unmarshal(bz, &att); err != nil {
		return nil, false
	}
	return &att, true
}

func (k Keeper) IterateAttestationsByStatus(
	ctx context.Context,
	status types.AttestationStatus,
	cb func(attestationID string) bool,
) {
	store := sdk.UnwrapSDKContext(ctx).KVStore(k.storeKey)
	prefix := append(append([]byte{}, types.AttestationByStatusPrefix...), uint8(status))
	iter := storetypes.KVStorePrefixIterator(store, prefix)
	defer iter.Close()
	prefixLen := len(prefix)
	for ; iter.Valid(); iter.Next() {
		if cb(string(iter.Key()[prefixLen:])) {
			return
		}
	}
}

func (k Keeper) NextAttestationID(ctx context.Context) string {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	store := sdkCtx.KVStore(k.storeKey)
	var next uint64
	if buf := store.Get(types.AttestationIDCounterKey); buf != nil {
		next, _ = binary.Uvarint(buf)
	}
	next++
	buf := make([]byte, binary.MaxVarintLen64)
	n := binary.PutUvarint(buf, next)
	store.Set(types.AttestationIDCounterKey, buf[:n])
	return fmt.Sprintf("att-%d-%d", sdkCtx.BlockHeight(), next)
}
