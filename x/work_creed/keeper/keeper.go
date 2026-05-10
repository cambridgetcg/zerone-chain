package keeper

import (
	"context"
	"encoding/binary"
	"fmt"

	"cosmossdk.io/core/store"
	"cosmossdk.io/errors"
	"cosmossdk.io/log"

	"github.com/cosmos/cosmos-sdk/codec"
	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/zerone-chain/zerone/x/work_creed/types"
)

// Keeper is the work_creed module keeper. Phase 0 exposes:
//   - GetLatestSubCreedPin: read the latest pin for a phase
//   - SetSubCreedPin: write a pin (used by InitGenesis; Phase 1+ also
//     used by AnchorSubCreedPin msg handler)
//   - IterateSubCreedPins: iterate latest pins
//
// The module has no token authority and no msg/query servers at Phase 0.
type Keeper struct {
	cdc          codec.BinaryCodec
	storeService store.KVStoreService

	// authority is the gov module address. Used by Phase 1+ msg
	// handlers; Phase 0 stores it but doesn't enforce it.
	authority string
}

// NewKeeper constructs the Keeper.
func NewKeeper(
	storeService store.KVStoreService,
	cdc codec.BinaryCodec,
	authority string,
) Keeper {
	return Keeper{
		cdc:          cdc,
		storeService: storeService,
		authority:    authority,
	}
}

// Authority returns the gov authority address (used by Phase 1+ msg
// handlers).
func (k Keeper) Authority() string { return k.authority }

// Logger returns a sub-logger.
func (k Keeper) Logger(ctx context.Context) log.Logger {
	return sdk.UnwrapSDKContext(ctx).Logger().With("module", "x/"+types.ModuleName)
}

// latestPinKey returns the store key for the latest pin of a phase.
func latestPinKey(phase uint32) []byte {
	buf := make([]byte, 4)
	binary.BigEndian.PutUint32(buf, phase)
	return append(types.LatestSubCreedPinKey, buf...)
}

// prefixEnd returns the smallest key strictly greater than the given
// prefix, suitable as the exclusive end of an iterator range.
func prefixEnd(prefix []byte) []byte {
	end := make([]byte, len(prefix))
	copy(end, prefix)
	for i := len(end) - 1; i >= 0; i-- {
		if end[i] < 0xFF {
			end[i]++
			return end[:i+1]
		}
	}
	return nil
}

// GetLatestSubCreedPin returns the latest pin for a phase, or
// (nil, false) if none.
func (k Keeper) GetLatestSubCreedPin(ctx context.Context, phase uint32) (*types.PinnedSubCreed, bool) {
	kvStore := k.storeService.OpenKVStore(ctx)
	bz, err := kvStore.Get(latestPinKey(phase))
	if err != nil || bz == nil {
		return nil, false
	}
	var p types.PinnedSubCreed
	k.cdc.MustUnmarshal(bz, &p)
	return &p, true
}

// SetSubCreedPin writes a pin as the latest for its phase. Caller is
// responsible for monotonicity of (phase, version) — InitGenesis writes
// version=1; Phase 1+ msg handler will check current+1 before calling.
func (k Keeper) SetSubCreedPin(ctx context.Context, p *types.PinnedSubCreed) error {
	if p.Phase > 8 {
		return errors.Wrapf(types.ErrUnknownPhase, "phase %d out of range", p.Phase)
	}
	if p.Phase == 1 {
		return fmt.Errorf("Knowledge phase delegates to x/creed; cannot pin here")
	}
	if len(p.CanonicalHash) != 32 {
		return fmt.Errorf("canonical_hash must be 32 bytes, got %d", len(p.CanonicalHash))
	}
	kvStore := k.storeService.OpenKVStore(ctx)
	return kvStore.Set(latestPinKey(p.Phase), k.cdc.MustMarshal(p))
}

// IterateSubCreedPins calls fn for the latest pin of every phase that
// has one. Iteration order is by phase number ascending.
func (k Keeper) IterateSubCreedPins(ctx context.Context, fn func(p *types.PinnedSubCreed) (stop bool)) {
	kvStore := k.storeService.OpenKVStore(ctx)
	iter, err := kvStore.Iterator(types.LatestSubCreedPinKey, prefixEnd(types.LatestSubCreedPinKey))
	if err != nil {
		return
	}
	defer iter.Close()
	for ; iter.Valid(); iter.Next() {
		var p types.PinnedSubCreed
		k.cdc.MustUnmarshal(iter.Value(), &p)
		if fn(&p) {
			return
		}
	}
}
