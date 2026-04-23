package keeper

import (
	"context"
	"fmt"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"google.golang.org/protobuf/proto"

	"github.com/zerone-chain/zerone/x/knowledge/types"
)

// ─── Wave 12: chain-wide module circuit breakers ────────────────────────
//
// The "localize damage" primitive of the resilience framework. Any module
// can be paused independently of the rest of the chain, stopping its
// write-path while a fix is deployed. Other modules continue unaffected.
//
// Integration pattern for any module's msg handler:
//
//   func (m *msgServer) DoThing(ctx, msg) (..., error) {
//       if m.keeper.IsModulePaused(ctx, types.ModuleName) {
//           return nil, fmt.Errorf("%s module paused; see resilience dashboard", types.ModuleName)
//       }
//       // normal logic
//   }
//
// The helper lives in the knowledge keeper (this file) because knowledge
// is already the resilience coordinator (incident records, migrations,
// upgrade registry). Other modules can accept a ResilienceChecker
// interface and have the knowledge keeper injected — this keeps zero new
// inter-module dependencies at the type level while sharing the primitive.

// ─── CRUD ────────────────────────────────────────────────────────────────

// SetModulePause records that a module is paused. Idempotent — calling
// this for an already-paused module replaces the record (useful if
// governance amends the reason or extends the auto-unpause block).
func (k Keeper) SetModulePause(ctx context.Context, p *types.ModulePause) error {
	if p == nil || p.ModuleName == "" {
		return fmt.Errorf("invalid pause record")
	}
	store := k.storeService.OpenKVStore(ctx)
	bz, err := marshalOpts.Marshal(p)
	if err != nil {
		return err
	}
	return store.Set(types.ModulePauseKey(p.ModuleName), bz)
}

// GetModulePause fetches the pause record for a module, or nil/false if
// the module is not paused.
func (k Keeper) GetModulePause(ctx context.Context, moduleName string) (*types.ModulePause, bool) {
	if moduleName == "" {
		return nil, false
	}
	store := k.storeService.OpenKVStore(ctx)
	bz, err := store.Get(types.ModulePauseKey(moduleName))
	if err != nil || bz == nil {
		return nil, false
	}
	var p types.ModulePause
	if err := proto.Unmarshal(bz, &p); err != nil {
		return nil, false
	}
	return &p, true
}

// ClearModulePause deletes the pause record, unpausing the module.
func (k Keeper) ClearModulePause(ctx context.Context, moduleName string) error {
	if moduleName == "" {
		return nil
	}
	store := k.storeService.OpenKVStore(ctx)
	return store.Delete(types.ModulePauseKey(moduleName))
}

// IteratePausedModules yields every paused module.
func (k Keeper) IteratePausedModules(ctx context.Context, cb func(*types.ModulePause) bool) {
	store := k.storeService.OpenKVStore(ctx)
	iter, err := store.Iterator(types.ModulePauseKeyPrefix, prefixEndBytes(types.ModulePauseKeyPrefix))
	if err != nil {
		return
	}
	defer iter.Close()
	for ; iter.Valid(); iter.Next() {
		var p types.ModulePause
		if err := proto.Unmarshal(iter.Value(), &p); err != nil {
			continue
		}
		if cb(&p) {
			return
		}
	}
}

// ─── The primitive ──────────────────────────────────────────────────────

// IsModulePaused returns true iff the named module is currently under a
// circuit-breaker pause. The helper other modules call from their
// handlers before touching state.
//
// Also honours auto-unpause-at-block: if the paused record's
// auto_unpause_at_block has passed, the pause is lazily cleared on read
// and the function returns false. This keeps maintenance-window pauses
// self-expiring without a separate heartbeat scan.
func (k Keeper) IsModulePaused(ctx context.Context, moduleName string) bool {
	p, ok := k.GetModulePause(ctx, moduleName)
	if !ok || p == nil {
		return false
	}
	if p.AutoUnpauseAtBlock > 0 {
		sdkCtx := sdk.UnwrapSDKContext(ctx)
		if uint64(sdkCtx.BlockHeight()) >= p.AutoUnpauseAtBlock {
			// Lazy-clear. Non-fatal if the delete fails; we still report
			// unpaused so the caller proceeds.
			_ = k.ClearModulePause(ctx, moduleName)
			return false
		}
	}
	return true
}

// RequireNotPaused returns an error when the module is currently paused.
// Sugar for handlers that prefer the error return form:
//
//   if err := k.RequireNotPaused(ctx, types.ModuleName); err != nil {
//       return nil, err
//   }
func (k Keeper) RequireNotPaused(ctx context.Context, moduleName string) error {
	if k.IsModulePaused(ctx, moduleName) {
		p, _ := k.GetModulePause(ctx, moduleName)
		reason := "circuit breaker active"
		if p != nil && p.Reason != "" {
			reason = p.Reason
		}
		return fmt.Errorf("module %s is paused: %s", moduleName, reason)
	}
	return nil
}

// ─── Msg handlers ────────────────────────────────────────────────────────

// PauseModule opens the circuit breaker for a named module. Authority-gated.
func (m *msgServer) PauseModule(ctx context.Context, msg *types.MsgPauseModule) (*types.MsgPauseModuleResponse, error) {
	if msg == nil || msg.ModuleName == "" {
		return nil, fmt.Errorf("module_name required")
	}
	if msg.Authority != m.keeper.GetAuthority() {
		return nil, fmt.Errorf("unauthorized: only governance authority may pause modules")
	}
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	height := uint64(sdkCtx.BlockHeight())

	p := &types.ModulePause{
		ModuleName:         msg.ModuleName,
		Reason:             msg.Reason,
		PausedAtBlock:      height,
		PausedBy:           msg.Authority,
		AutoUnpauseAtBlock: msg.AutoUnpauseAtBlock,
		IncidentId:         msg.IncidentId,
	}
	if err := m.keeper.SetModulePause(ctx, p); err != nil {
		return nil, err
	}
	sdkCtx.EventManager().EmitEvent(sdk.NewEvent(
		"zerone.knowledge.module_paused",
		sdk.NewAttribute("module_name", p.ModuleName),
		sdk.NewAttribute("reason", p.Reason),
		sdk.NewAttribute("incident_id", p.IncidentId),
		sdk.NewAttribute("auto_unpause_at_block", fmt.Sprintf("%d", p.AutoUnpauseAtBlock)),
	))
	return &types.MsgPauseModuleResponse{PausedAtBlock: height}, nil
}

// UnpauseModule closes the circuit breaker; writes resume.
func (m *msgServer) UnpauseModule(ctx context.Context, msg *types.MsgUnpauseModule) (*types.MsgUnpauseModuleResponse, error) {
	if msg == nil || msg.ModuleName == "" {
		return nil, fmt.Errorf("module_name required")
	}
	if msg.Authority != m.keeper.GetAuthority() {
		return nil, fmt.Errorf("unauthorized")
	}
	if _, ok := m.keeper.GetModulePause(ctx, msg.ModuleName); !ok {
		return nil, fmt.Errorf("module %s is not paused", msg.ModuleName)
	}
	if err := m.keeper.ClearModulePause(ctx, msg.ModuleName); err != nil {
		return nil, err
	}
	sdk.UnwrapSDKContext(ctx).EventManager().EmitEvent(sdk.NewEvent(
		"zerone.knowledge.module_unpaused",
		sdk.NewAttribute("module_name", msg.ModuleName),
		sdk.NewAttribute("note", msg.Note),
	))
	return &types.MsgUnpauseModuleResponse{}, nil
}
