package keeper

import (
	"context"
	"encoding/binary"
	"fmt"

	"cosmossdk.io/core/store"
	"cosmossdk.io/log"
	"github.com/cosmos/cosmos-sdk/codec"
	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/zerone-chain/zerone/x/creed/types"
)

// Keeper anchors the chain's canonical creed on chain. It owns
// the PinnedCreed history (forward-only, monotonically versioned)
// and the per-commitment registry. Other modules consume it
// read-only via gRPC; the only writers are MsgAnchorPin (authority-
// gated, eventually flowing through the CategoryCreedAmendment LIP
// in x/gov) and MsgUpdateParams.
//
// docs/TRUTH_SEEKING.md commitments 6 and 10:
//   - 6 (no unilateral injection): the authority gate prevents any
//     single key from silently amending the creed. Once direct-
//     anchor is disabled and the LIP class ships, the only
//     legitimate authority is the gov module account.
//   - 10 (forward-only audit): pins are append-only; CurrentVersion
//     monotonically increases; archived commitments are marked, not
//     deleted.
type Keeper struct {
	cdc          codec.BinaryCodec
	storeService store.KVStoreService
	authority    string
}

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

func (k Keeper) Logger(ctx context.Context) log.Logger {
	return sdk.UnwrapSDKContext(ctx).Logger().With("module", "x/"+types.ModuleName)
}

func (k Keeper) GetAuthority() string { return k.authority }

// ── Params ──────────────────────────────────────────────────────────

func (k Keeper) GetParams(ctx context.Context) types.Params {
	store := k.storeService.OpenKVStore(ctx)
	bz, err := store.Get(types.ParamsKey)
	if err != nil || bz == nil {
		return *types.DefaultParams()
	}
	var p types.Params
	if err := k.cdc.Unmarshal(bz, &p); err != nil {
		return *types.DefaultParams()
	}
	return p
}

func (k Keeper) SetParams(ctx context.Context, p types.Params) error {
	bz, err := k.cdc.Marshal(&p)
	if err != nil {
		return err
	}
	return k.storeService.OpenKVStore(ctx).Set(types.ParamsKey, bz)
}

// ── Pin storage ─────────────────────────────────────────────────────

// pinKey returns the storage key for a specific pin version.
func pinKey(version uint32) []byte {
	out := make([]byte, len(types.PinPrefix)+4)
	copy(out, types.PinPrefix)
	binary.BigEndian.PutUint32(out[len(types.PinPrefix):], version)
	return out
}

// SetPin writes a PinnedCreed at its version. Caller is
// responsible for invariant checks (handled in the msg server).
func (k Keeper) SetPin(ctx context.Context, p *types.PinnedCreed) error {
	bz, err := k.cdc.Marshal(p)
	if err != nil {
		return err
	}
	store := k.storeService.OpenKVStore(ctx)
	if err := store.Set(pinKey(p.Version), bz); err != nil {
		return err
	}
	// Update CurrentVersion if this pin is the new highest.
	cur := k.GetCurrentVersion(ctx)
	if p.Version > cur {
		buf := make([]byte, 4)
		binary.BigEndian.PutUint32(buf, p.Version)
		if err := store.Set(types.CurrentVersionKey, buf); err != nil {
			return err
		}
	}
	return nil
}

// GetPin returns the pin at a specific version, or false if no
// pin exists at that version.
func (k Keeper) GetPin(ctx context.Context, version uint32) (*types.PinnedCreed, bool) {
	store := k.storeService.OpenKVStore(ctx)
	bz, err := store.Get(pinKey(version))
	if err != nil || bz == nil {
		return nil, false
	}
	var p types.PinnedCreed
	if err := k.cdc.Unmarshal(bz, &p); err != nil {
		return nil, false
	}
	return &p, true
}

// GetCurrentVersion returns the highest pinned version, or 0 if
// no pin has been recorded yet.
func (k Keeper) GetCurrentVersion(ctx context.Context) uint32 {
	store := k.storeService.OpenKVStore(ctx)
	bz, err := store.Get(types.CurrentVersionKey)
	if err != nil || bz == nil || len(bz) != 4 {
		return 0
	}
	return binary.BigEndian.Uint32(bz)
}

// GetCurrentPin returns the currently-canonical pin, or false if
// the chain is in a pre-anchor state (genesis didn't seed a pin
// and no AnchorPin has run yet).
func (k Keeper) GetCurrentPin(ctx context.Context) (*types.PinnedCreed, bool) {
	v := k.GetCurrentVersion(ctx)
	if v == 0 {
		return nil, false
	}
	return k.GetPin(ctx, v)
}

// IteratePinsDescending walks pinned versions from newest to
// oldest, calling cb for each. Stops if cb returns true.
func (k Keeper) IteratePinsDescending(ctx context.Context, cb func(*types.PinnedCreed) bool) {
	cur := k.GetCurrentVersion(ctx)
	for v := cur; v > 0; v-- {
		p, ok := k.GetPin(ctx, v)
		if !ok {
			continue
		}
		if cb(p) {
			return
		}
	}
}

// CurrentCommitment returns the current registry entry for a
// commitment number, or false if not declared (or archived).
func (k Keeper) CurrentCommitment(ctx context.Context, number uint32) (*types.CommitmentEntry, bool) {
	pin, ok := k.GetCurrentPin(ctx)
	if !ok {
		return nil, false
	}
	for _, c := range pin.Commitments {
		if c.Number == number {
			if c.Archived {
				return c, false
			}
			return c, true
		}
	}
	return nil, false
}

// AnchorPin records a new pin at version+1. The handler in
// msg_server.go validates the pin's structural invariants before
// calling this.
func (k Keeper) AnchorPin(ctx context.Context, p *types.PinnedCreed) error {
	if p == nil {
		return fmt.Errorf("nil pin")
	}
	cur := k.GetCurrentVersion(ctx)
	if p.Version != cur+1 {
		return types.ErrVersionNotMonotonic.Wrapf("expected version %d, got %d", cur+1, p.Version)
	}
	return k.SetPin(ctx, p)
}
