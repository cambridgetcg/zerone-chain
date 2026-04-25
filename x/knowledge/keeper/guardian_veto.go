package keeper

import (
	"context"
	"encoding/binary"
	"fmt"

	"github.com/cosmos/gogoproto/proto"
	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/zerone-chain/zerone/x/knowledge/types"
)

// ─── Pending-injection store ops ──────────────────────────────────────────

// SetPendingFactInjection writes a queued fact-injection to state and
// indexes it by execute_at_block so BeginBlocker can scan in time order.
func (k Keeper) SetPendingFactInjection(ctx context.Context, p *types.PendingFactInjection) error {
	if p == nil || p.Id == "" {
		return fmt.Errorf("pending fact injection requires non-empty id")
	}
	bz, err := proto.Marshal(p)
	if err != nil {
		return err
	}
	store := k.storeService.OpenKVStore(ctx)
	if err := store.Set(types.PendingFactInjectionKey(p.Id), bz); err != nil {
		return err
	}
	// Time-ordered index: scan by execute_at_block ascending.
	return store.Set(types.PendingFactInjectionByExecuteKey(p.ExecuteAtBlock, p.Id), []byte{1})
}

// GetPendingFactInjection retrieves a queued injection by id.
func (k Keeper) GetPendingFactInjection(ctx context.Context, id string) (*types.PendingFactInjection, bool) {
	store := k.storeService.OpenKVStore(ctx)
	bz, err := store.Get(types.PendingFactInjectionKey(id))
	if err != nil || bz == nil {
		return nil, false
	}
	var p types.PendingFactInjection
	if err := proto.Unmarshal(bz, &p); err != nil {
		return nil, false
	}
	return &p, true
}

// DeletePendingFactInjection removes the entry and its time index.
func (k Keeper) DeletePendingFactInjection(ctx context.Context, id string) error {
	p, ok := k.GetPendingFactInjection(ctx, id)
	if !ok {
		return nil
	}
	store := k.storeService.OpenKVStore(ctx)
	if err := store.Delete(types.PendingFactInjectionKey(id)); err != nil {
		return err
	}
	return store.Delete(types.PendingFactInjectionByExecuteKey(p.ExecuteAtBlock, id))
}

// IteratePendingFactInjectionsDue yields every pending injection whose
// execute_at_block is ≤ height, in ascending order (oldest first).
// BeginBlocker uses this to materialize matured proposals.
func (k Keeper) IteratePendingFactInjectionsDue(ctx context.Context, height uint64, cb func(*types.PendingFactInjection) bool) {
	store := k.storeService.OpenKVStore(ctx)
	end := types.PendingFactInjectionByExecuteKeyPrefix
	endKey := append(append([]byte{}, end...), make([]byte, 8)...)
	binary.BigEndian.PutUint64(endKey[len(end):], height+1) // exclusive upper bound at height+1
	iter, err := store.Iterator(types.PendingFactInjectionByExecuteKeyPrefix, endKey)
	if err != nil {
		return
	}
	defer iter.Close()
	for ; iter.Valid(); iter.Next() {
		// Key format: prefix(1) | be64(execute_at) | id(string)
		full := iter.Key()
		if len(full) < 1+8 {
			continue
		}
		id := string(full[1+8:])
		p, ok := k.GetPendingFactInjection(ctx, id)
		if !ok {
			continue
		}
		if cb(p) {
			return
		}
	}
}

// ─── Helpers ──────────────────────────────────────────────────────────────

// IsGuardian reports whether addr appears in Params.GuardianAddresses.
func (k Keeper) IsGuardian(ctx context.Context, addr string) bool {
	params, err := k.GetParams(ctx)
	if err != nil {
		return false
	}
	for _, g := range params.GuardianAddresses {
		if g == addr {
			return true
		}
	}
	return false
}

// AddFactVetoEnabled is true iff the guardian-veto window for fact
// injection is currently active (params populated, guardian set
// non-empty). When false, MsgAddFact executes immediately as before.
func (k Keeper) AddFactVetoEnabled(ctx context.Context) bool {
	params, err := k.GetParams(ctx)
	if err != nil {
		return false
	}
	return params.AddFactVetoWindowBlocks > 0 && len(params.GuardianAddresses) > 0
}

// ─── BeginBlocker materialization ─────────────────────────────────────────

// MaterializeMaturedFactInjections is the BeginBlocker hook for the
// guardian-veto queue. Any pending fact-injection whose execute window
// has expired without veto becomes a real Fact at this point.
func (k Keeper) MaterializeMaturedFactInjections(ctx context.Context, height uint64) {
	var matured []*types.PendingFactInjection
	k.IteratePendingFactInjectionsDue(ctx, height, func(p *types.PendingFactInjection) bool {
		matured = append(matured, p)
		return false
	})
	if len(matured) == 0 {
		return
	}
	params, err := k.GetParams(ctx)
	if err != nil {
		return
	}
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	for _, p := range matured {
		// Reconstruct the Fact the way MsgAddFact would have, then
		// commit. Confidence is clamped against domain caps the same
		// way MsgAddFact does at submit time.
		fact := &types.Fact{
			Id:                p.Id,
			Content:           p.Content,
			Domain:            p.Domain,
			Category:          p.Category,
			Confidence:        k.ClampConfidence(ctx, p.Confidence, p.Domain),
			Submitter:         p.Proposer,
			SubmittedAtBlock:  p.ProposedAtBlock,
			VerifiedAtBlock:   height,
			LastVerifiedBlock: height,
			References:        p.References,
			Status:            types.FactStatus_FACT_STATUS_VERIFIED,
			Energy:            params.MetabolismInitialEnergy,
			EnergyCap:         params.MetabolismEnergyCap,
			EnergyLastUpdated: height,
		}
		fact.Energy = k.ApplyBirthPressure(ctx, p.Domain, fact.Energy)
		if err := k.SetFact(ctx, fact); err != nil {
			k.Logger(ctx).Error("matured pending injection SetFact failed",
				"pending", p.Id, "err", err)
			continue
		}
		k.IncrementDomainFactCount(ctx, fact.Domain, true, fact.Energy)

		// Privileged-action log already recorded this at proposal time;
		// emit a separate "materialized" event so external indexers can
		// distinguish proposal from final state transition.
		sdkCtx.EventManager().EmitEvent(sdk.NewEvent(
			"zerone.knowledge.pending_fact_materialized",
			sdk.NewAttribute("fact_id", p.Id),
			sdk.NewAttribute("proposer", p.Proposer),
			sdk.NewAttribute("domain", p.Domain),
			sdk.NewAttribute("category", p.Category),
			sdk.NewAttribute("proposed_at_block", fmt.Sprintf("%d", p.ProposedAtBlock)),
			sdk.NewAttribute("materialized_at_block", fmt.Sprintf("%d", height)),
		))

		_ = k.DeletePendingFactInjection(ctx, p.Id)
	}
}
