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

	// contributionWrapper records privileged sub-creed pin writes as
	// Substrate-class Contributions in x/contribution. Wired post-init
	// (app.go) to break the import cycle (x/contribution depends on
	// x/work_creed via the contribution adapter path). Nil-safe:
	// callers no-op when the wrapper is not yet set. Phase 1 layer of
	// the UW recursion stack (Layer 2 — runtime self-application).
	contributionWrapper types.ContributionWrapper
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

// SetContributionWrapper wires the x/contribution-side helper that
// records pin writes as Substrate Contributions. Called from app.go
// after both keepers exist. Layer 2 of the recursion stack.
func (k *Keeper) SetContributionWrapper(w types.ContributionWrapper) {
	k.contributionWrapper = w
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
//
// Recursion layer 2: before writing the pin, wraps the action as a
// Substrate-class Contribution in x/contribution if the wrapper has
// been set (post-init in app.go). UW: ZERONE is recursive — the chain
// records its own doctrinal amendments as Contributions reviewed by
// itself.
//
// The wrap is best-effort at Phase 1: failure to record the
// Contribution does not block the pin write because the recording is
// observational (the action is the source of truth; the Contribution
// is the audit trail). Phase 6 will tighten this so wrap failures
// reject the write, paired with a real verifier + revert window.
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

	if k.contributionWrapper != nil {
		// Best-effort marshal of the pin into the description bytes.
		// The Contribution carries the canonical pin record; off-chain
		// indexers can replay sub-creed amendments by reading the
		// PIPELINE_IMPROVEMENT/SUBSTRATE class+phase combination.
		desc, mErr := k.cdc.Marshal(p)
		if mErr == nil {
			_, _ = k.contributionWrapper.WrapAsSubstrateContribution(
				ctx,
				"doctrine",
				k.authority,
				desc,
				nil, // top-level pin; no parent Contribution to nest under
			)
		}
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
