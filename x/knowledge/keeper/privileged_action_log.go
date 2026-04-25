package keeper

import (
	"context"
	"encoding/binary"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"google.golang.org/protobuf/proto"

	"github.com/zerone-chain/zerone/x/knowledge/types"
)

// ─── Wave 14: privileged-action audit log ───────────────────────────────
//
// Every authority-gated handler calls k.RecordPrivilegedAction at entry.
// The log is append-only, monotonic-seq-keyed, queryable by type and by
// block range. It is the chain's answer to "what has authority done
// recently?" — a simple query rather than a trawl through event history.
//
// Records are auto-numbered via a singleton counter (PrivilegedActionSeqKey).
// Storage is O(N) in total lifetime actions; for chains where log
// growth is a concern, governance can later add a TTL param and prune
// records older than N blocks in the BeginBlocker heartbeat.

// RecordPrivilegedAction appends an audit-log entry. Non-fatal on write
// failure — an error here would abort the privileged action itself, and
// we'd rather lose an audit entry than block a remediation. Failures are
// logged for post-hoc investigation.
func (k Keeper) RecordPrivilegedAction(
	ctx context.Context,
	actType types.PrivilegedActionType,
	invoker, target, incidentID, note string,
) uint64 {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	seq := k.nextPrivilegedActionSeq(ctx)

	rec := &types.PrivilegedAction{
		Seq:            seq,
		Type:           actType,
		Invoker:        invoker,
		InvokedAtBlock: uint64(sdkCtx.BlockHeight()),
		Target:         target,
		IncidentId:     incidentID,
		Note:           note,
	}
	bz, err := marshalOpts.Marshal(rec)
	if err != nil {
		k.Logger(ctx).Error("privileged action marshal failed",
			"seq", seq, "type", actType.String(), "error", err.Error())
		return seq
	}
	store := k.storeService.OpenKVStore(ctx)
	if err := store.Set(types.PrivilegedActionKey(seq), bz); err != nil {
		k.Logger(ctx).Error("privileged action write failed",
			"seq", seq, "type", actType.String(), "error", err.Error())
	}
	// Emit event even if store write failed — some audit trail is better
	// than none, and the event is a separate durable channel.
	// No unilateral injection (commitment 6) and forward-only audit
	// (commitment 10): every privileged action is announced as it
	// happens, with a monotonic seq that no future actor can rewrite.
	// See TRUTH_SEEKING.md.
	sdkCtx.EventManager().EmitEvent(sdk.NewEvent(
		"zerone.knowledge.privileged_action_recorded",
		sdk.NewAttribute("seq", u64String(seq)),
		sdk.NewAttribute("type", actType.String()),
		sdk.NewAttribute("invoker", invoker),
		sdk.NewAttribute("target", target),
		sdk.NewAttribute("incident_id", incidentID),
		sdk.NewAttribute("creed_commitment", "6,10"),
	))
	return seq
}

// nextPrivilegedActionSeq allocates a monotonic sequence number.
func (k Keeper) nextPrivilegedActionSeq(ctx context.Context) uint64 {
	store := k.storeService.OpenKVStore(ctx)
	bz, err := store.Get(types.PrivilegedActionSeqKey)
	var cur uint64
	if err == nil && bz != nil && len(bz) == 8 {
		cur = binary.BigEndian.Uint64(bz)
	}
	next := cur + 1
	buf := make([]byte, 8)
	binary.BigEndian.PutUint64(buf, next)
	_ = store.Set(types.PrivilegedActionSeqKey, buf)
	return next
}

// GetPrivilegedAction fetches a specific action by sequence number.
func (k Keeper) GetPrivilegedAction(ctx context.Context, seq uint64) (*types.PrivilegedAction, bool) {
	if seq == 0 {
		return nil, false
	}
	store := k.storeService.OpenKVStore(ctx)
	bz, err := store.Get(types.PrivilegedActionKey(seq))
	if err != nil || bz == nil {
		return nil, false
	}
	var rec types.PrivilegedAction
	if err := proto.Unmarshal(bz, &rec); err != nil {
		return nil, false
	}
	return &rec, true
}

// IteratePrivilegedActions yields actions in sequence-ascending order.
func (k Keeper) IteratePrivilegedActions(ctx context.Context, cb func(*types.PrivilegedAction) bool) {
	store := k.storeService.OpenKVStore(ctx)
	iter, err := store.Iterator(types.PrivilegedActionKeyPrefix, prefixEndBytes(types.PrivilegedActionKeyPrefix))
	if err != nil {
		return
	}
	defer iter.Close()
	for ; iter.Valid(); iter.Next() {
		var rec types.PrivilegedAction
		if err := proto.Unmarshal(iter.Value(), &rec); err != nil {
			continue
		}
		if cb(&rec) {
			return
		}
	}
}

// u64String is a tiny format helper to avoid the fmt-import dance.
func u64String(v uint64) string {
	// strconv.FormatUint is cheapest and deterministic.
	const digits = "0123456789"
	if v == 0 {
		return "0"
	}
	var buf [20]byte
	pos := len(buf)
	for v > 0 {
		pos--
		buf[pos] = digits[v%10]
		v /= 10
	}
	return string(buf[pos:])
}
