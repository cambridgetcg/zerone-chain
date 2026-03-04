package keeper

import (
	"context"
	"fmt"

	"google.golang.org/protobuf/proto"

	"github.com/zerone-chain/zerone/x/knowledge/types"
)

var marshalOpts = proto.MarshalOptions{Deterministic: true}

// ─── Params ──────────────────────────────────────────────────────────────────

func (k Keeper) SetParams(ctx context.Context, params *types.Params) error {
	store := k.storeService.OpenKVStore(ctx)
	bz, err := marshalOpts.Marshal(params)
	if err != nil {
		return fmt.Errorf("failed to marshal params: %w", err)
	}
	return store.Set(types.ParamsKey, bz)
}

func (k Keeper) GetParams(ctx context.Context) (*types.Params, error) {
	store := k.storeService.OpenKVStore(ctx)
	bz, err := store.Get(types.ParamsKey)
	if err != nil {
		return nil, err
	}
	if bz == nil {
		p := types.DefaultParams()
		return &p, nil
	}
	var params types.Params
	if err := proto.Unmarshal(bz, &params); err != nil {
		p := types.DefaultParams()
		return &p, nil
	}
	return &params, nil
}

// ─── Domain CRUD ─────────────────────────────────────────────────────────────

func (k Keeper) SetDomain(ctx context.Context, domain *types.Domain) error {
	store := k.storeService.OpenKVStore(ctx)
	bz, err := marshalOpts.Marshal(domain)
	if err != nil {
		return fmt.Errorf("failed to marshal domain: %w", err)
	}
	return store.Set(types.DomainKey(domain.Name), bz)
}

func (k Keeper) GetDomain(ctx context.Context, name string) (*types.Domain, bool) {
	store := k.storeService.OpenKVStore(ctx)
	bz, err := store.Get(types.DomainKey(name))
	if err != nil || bz == nil {
		return nil, false
	}
	var domain types.Domain
	if err := proto.Unmarshal(bz, &domain); err != nil {
		return nil, false
	}
	return &domain, true
}

func (k Keeper) IterateDomains(ctx context.Context, cb func(domain *types.Domain) bool) {
	store := k.storeService.OpenKVStore(ctx)
	iter, err := store.Iterator(types.DomainKeyPrefix, prefixEndBytes(types.DomainKeyPrefix))
	if err != nil {
		return
	}
	defer iter.Close()
	for ; iter.Valid(); iter.Next() {
		var domain types.Domain
		if err := proto.Unmarshal(iter.Value(), &domain); err != nil {
			continue
		}
		if cb(&domain) {
			break
		}
	}
}

// ─── Store helpers ───────────────────────────────────────────────────────────

func prefixEndBytes(pfx []byte) []byte {
	if len(pfx) == 0 {
		return nil
	}
	end := make([]byte, len(pfx))
	copy(end, pfx)
	for i := len(end) - 1; i >= 0; i-- {
		end[i]++
		if end[i] != 0 {
			return end
		}
	}
	return nil
}
