package keeper

import (
	"context"

	"google.golang.org/protobuf/proto"

	"github.com/zerone-chain/zerone/x/knowledge/types"
)

// SetMethodology stores (or updates) a methodology in the registry.
// Intended for genesis seeding and governance amendments only.
func (k Keeper) SetMethodology(ctx context.Context, m *types.Methodology) error {
	if m == nil || m.Id == "" {
		return nil
	}
	store := k.storeService.OpenKVStore(ctx)
	bz, err := marshalOpts.Marshal(m)
	if err != nil {
		return err
	}
	return store.Set(types.MethodologyKey(m.Id), bz)
}

// GetMethodology fetches a methodology by id. Returns (nil, false) if not found.
func (k Keeper) GetMethodology(ctx context.Context, id string) (*types.Methodology, bool) {
	if id == "" {
		return nil, false
	}
	store := k.storeService.OpenKVStore(ctx)
	bz, err := store.Get(types.MethodologyKey(id))
	if err != nil || bz == nil {
		return nil, false
	}
	var m types.Methodology
	if err := proto.Unmarshal(bz, &m); err != nil {
		return nil, false
	}
	return &m, true
}

// IterateMethodologies yields every registered methodology. Return true from
// cb to stop iteration.
func (k Keeper) IterateMethodologies(ctx context.Context, cb func(*types.Methodology) bool) {
	store := k.storeService.OpenKVStore(ctx)
	iter, err := store.Iterator(types.MethodologyKeyPrefix, prefixEndBytes(types.MethodologyKeyPrefix))
	if err != nil {
		return
	}
	defer iter.Close()
	for ; iter.Valid(); iter.Next() {
		var m types.Methodology
		if err := proto.Unmarshal(iter.Value(), &m); err != nil {
			continue
		}
		if cb(&m) {
			return
		}
	}
}

// GetAllMethodologies returns every registered methodology.
func (k Keeper) GetAllMethodologies(ctx context.Context) []*types.Methodology {
	var out []*types.Methodology
	k.IterateMethodologies(ctx, func(m *types.Methodology) bool {
		out = append(out, m)
		return false
	})
	return out
}

// IsMethodologyRegistered reports whether the given id names a methodology
// currently in the registry (governance-amendable).
func (k Keeper) IsMethodologyRegistered(ctx context.Context, id string) bool {
	_, found := k.GetMethodology(ctx, id)
	return found
}

// ResolveMethodId returns the methodology id to use for a claim that did not
// explicitly declare one. Under the transitional regime this maps to
// M-LEGACY. Once the legacy sunset triggers, callers should enforce non-empty
// method_id at submission instead of relying on this default.
func ResolveMethodId(declared string) string {
	if declared == "" {
		return types.MethodologyLegacy
	}
	return declared
}

// SeedDefaultMethodologies writes the seven bootstrap methodologies to the
// registry. Called from InitGenesis; exposed so tests and migrations can
// trigger the same seeding without going through full InitChain.
func (k Keeper) SeedDefaultMethodologies(ctx context.Context) error {
	for _, m := range types.DefaultMethodologies() {
		if m == nil {
			continue
		}
		if err := k.SetMethodology(ctx, m); err != nil {
			return err
		}
	}
	return nil
}
