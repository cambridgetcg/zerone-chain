package keeper

import (
	"context"
	"encoding/binary"
	"encoding/json"
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

	// contributionWrapper records privileged AnchorPin writes as
	// Substrate-class Contributions in x/contribution. Wired post-init
	// in app.go to break the import cycle (x/contribution imports
	// x/creed via the contribution adapter). Nil-safe: callers no-op
	// when the wrapper is not yet set. Layer 2 of the UW recursion
	// stack (runtime self-application).
	contributionWrapper types.ContributionWrapper
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

// SetContributionWrapper wires the x/contribution helper that records
// creed amendments as Substrate Contributions. Called from app.go
// after both keepers exist. Layer 2 of the recursion stack.
func (k *Keeper) SetContributionWrapper(w types.ContributionWrapper) {
	k.contributionWrapper = w
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
//
// Recursion layer 2: before writing the pin, wraps the action as a
// Substrate-class Contribution in x/contribution if the wrapper is
// set. UW: ZERONE is recursive — the chain records canonical creed
// amendments as Contributions reviewed by itself. Best-effort at
// Phase 1: failure to record the Contribution does not block the pin
// write because the recording is observational at this layer; Phase 6
// will tighten so wrap failures reject the write.
func (k Keeper) AnchorPin(ctx context.Context, p *types.PinnedCreed) error {
	if p == nil {
		return fmt.Errorf("nil pin")
	}
	cur := k.GetCurrentVersion(ctx)
	if p.Version != cur+1 {
		return types.ErrVersionNotMonotonic.Wrapf("expected version %d, got %d", cur+1, p.Version)
	}

	if k.contributionWrapper != nil {
		// Carry the canonical hash + version into the description so
		// off-chain indexers can correlate creed pin writes with the
		// resulting Substrate Contributions.
		if desc, mErr := k.cdc.Marshal(p); mErr == nil {
			_, _ = k.contributionWrapper.WrapAsSubstrateContribution(
				ctx,
				"doctrine",
				k.authority,
				desc,
				nil, // top-level pin; no parent Contribution to nest under
			)
		}
	}

	return k.SetPin(ctx, p)
}

// AnchorPinFromBytes is the cross-module entry point used by
// x/gov when a CategoryCreedAmendment LIP passes. The LIP carries
// the canonical hash and a JSON-encoded list of CommitmentEntry
// records; the keeper rebuilds the PinnedCreed locally and writes
// it under the next version with sourceLip recorded as the
// authorizer.
//
// docs/TRUTH_SEEKING.md commitment 19: this is the structural form
// of the post-launch creed-amendment path. direct_anchor_enabled
// can be false at that point — this entry is gov-mediated, so the
// gov authority's call counts whether or not the direct path is
// open.
func (k Keeper) AnchorPinFromBytes(ctx context.Context, sourceLip string, canonicalHash []byte, commitmentsJSON []byte) error {
	if len(canonicalHash) == 0 {
		return types.ErrEmptyHash
	}
	var commitments []*types.CommitmentEntry
	if len(commitmentsJSON) > 0 {
		if err := json.Unmarshal(commitmentsJSON, &commitments); err != nil {
			return fmt.Errorf("decode commitments_json: %w", err)
		}
	}
	cur := k.GetCurrentVersion(ctx)
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	pin := &types.PinnedCreed{
		Version:        cur + 1,
		CanonicalHash:  canonicalHash,
		PinnedAtHeight: uint64(sdkCtx.BlockHeight()),
		PinnedViaLip:   sourceLip,
		Commitments:    commitments,
	}
	return k.SetPin(ctx, pin)
}

// ── Council registry ───────────────────────────────────────────────

// councilKey returns the storage key for a council seat by address.
func councilKey(address string) []byte {
	out := make([]byte, 0, len(types.CouncilMemberPrefix)+len(address))
	out = append(out, types.CouncilMemberPrefix...)
	out = append(out, []byte(address)...)
	return out
}

// SetCouncilMember writes (or updates) a council seat. If the seat
// already exists, admitted_at_height is preserved unless the
// caller explicitly bumps it — this keeps the audit trail honest
// even through admin updates.
func (k Keeper) SetCouncilMember(ctx context.Context, m *types.CreedCouncilMember) error {
	if m == nil {
		return types.ErrInvalidCouncilMember.Wrap("nil member")
	}
	if m.Address == "" {
		return types.ErrInvalidCouncilMember.Wrap("address required")
	}
	bz, err := k.cdc.Marshal(m)
	if err != nil {
		return err
	}
	return k.storeService.OpenKVStore(ctx).Set(councilKey(m.Address), bz)
}

// GetCouncilMember returns the seat for an address, or false if
// no entry exists.
func (k Keeper) GetCouncilMember(ctx context.Context, address string) (*types.CreedCouncilMember, bool) {
	store := k.storeService.OpenKVStore(ctx)
	bz, err := store.Get(councilKey(address))
	if err != nil || bz == nil {
		return nil, false
	}
	var m types.CreedCouncilMember
	if err := k.cdc.Unmarshal(bz, &m); err != nil {
		return nil, false
	}
	return &m, true
}

// IsActiveCouncilMember reports whether the address holds an
// active council seat.
func (k Keeper) IsActiveCouncilMember(ctx context.Context, address string) bool {
	m, ok := k.GetCouncilMember(ctx, address)
	if !ok {
		return false
	}
	return m.Active
}

// IterateCouncilMembers walks all council seats in undefined
// order. cb receiving true stops iteration.
func (k Keeper) IterateCouncilMembers(ctx context.Context, cb func(*types.CreedCouncilMember) bool) {
	store := k.storeService.OpenKVStore(ctx)
	iter, err := store.Iterator(types.CouncilMemberPrefix, prefixEnd(types.CouncilMemberPrefix))
	if err != nil {
		return
	}
	defer iter.Close()
	for ; iter.Valid(); iter.Next() {
		var m types.CreedCouncilMember
		if err := k.cdc.Unmarshal(iter.Value(), &m); err != nil {
			continue
		}
		if cb(&m) {
			return
		}
	}
}

// CouncilTotalActiveWeight returns the sum of voting_weight_bps
// across all currently-active members. Used by off-chain composers
// computing quorum thresholds.
func (k Keeper) CouncilTotalActiveWeight(ctx context.Context) uint64 {
	var total uint64
	k.IterateCouncilMembers(ctx, func(m *types.CreedCouncilMember) bool {
		if m.Active {
			total += m.VotingWeightBps
		}
		return false
	})
	return total
}

// prefixEnd returns the smallest key strictly greater than the
// given prefix, suitable as the exclusive end of an iterator
// range.
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
