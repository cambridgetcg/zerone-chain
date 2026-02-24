package keeper

import (
	"encoding/json"
	"fmt"

	storetypes "cosmossdk.io/store/types"

	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/zerone-chain/zerone/x/gov/types"
)

// ---------- Phase Transition Metadata CRUD ----------

// GetPhaseTransitionMeta retrieves phase transition metadata linked to a LIP.
func (k Keeper) GetPhaseTransitionMeta(ctx sdk.Context, lipID string) (*types.PhaseTransitionProposal, bool) {
	store := ctx.KVStore(k.storeKey)
	bz := store.Get(types.PhaseTransitionKey(lipID))
	if bz == nil {
		return nil, false
	}
	var prop types.PhaseTransitionProposal
	if err := json.Unmarshal(bz, &prop); err != nil {
		return nil, false
	}
	return &prop, true
}

// SetPhaseTransitionMeta stores phase transition metadata linked to a LIP.
func (k Keeper) SetPhaseTransitionMeta(ctx sdk.Context, prop *types.PhaseTransitionProposal) {
	store := ctx.KVStore(k.storeKey)
	bz, err := json.Marshal(prop)
	if err != nil {
		panic("failed to marshal phase transition metadata: " + err.Error())
	}
	store.Set(types.PhaseTransitionKey(prop.LipID), bz)
}

// IteratePhaseTransitionMeta iterates over all phase transition metadata entries.
func (k Keeper) IteratePhaseTransitionMeta(ctx sdk.Context, cb func(*types.PhaseTransitionProposal) bool) {
	store := ctx.KVStore(k.storeKey)
	iter := storetypes.KVStorePrefixIterator(store, types.PhaseTransitionKeyPrefix)
	defer iter.Close()

	for ; iter.Valid(); iter.Next() {
		var prop types.PhaseTransitionProposal
		if err := json.Unmarshal(iter.Value(), &prop); err != nil {
			continue
		}
		if cb(&prop) {
			break
		}
	}
}

// GetAllPhaseTransitionMeta returns all phase transition metadata entries.
func (k Keeper) GetAllPhaseTransitionMeta(ctx sdk.Context) []*types.PhaseTransitionProposal {
	var result []*types.PhaseTransitionProposal
	k.IteratePhaseTransitionMeta(ctx, func(prop *types.PhaseTransitionProposal) bool {
		result = append(result, prop)
		return false
	})
	return result
}

// GetPendingPhaseTransition returns the first pending_activation metadata, if any.
func (k Keeper) GetPendingPhaseTransition(ctx sdk.Context) *types.PhaseTransitionProposal {
	var pending *types.PhaseTransitionProposal
	k.IteratePhaseTransitionMeta(ctx, func(prop *types.PhaseTransitionProposal) bool {
		if prop.Stage == types.PhaseTransitionStagePending {
			pending = prop
			return true
		}
		return false
	})
	return pending
}

// HasActivePhaseTransitionLIP returns true if there's a non-terminal phase
// transition LIP in progress.
func (k Keeper) HasActivePhaseTransitionLIP(ctx sdk.Context) bool {
	found := false
	k.IteratePhaseTransitionMeta(ctx, func(prop *types.PhaseTransitionProposal) bool {
		if !types.IsTerminalPhaseTransitionStage(prop.Stage) {
			found = true
			return true
		}
		return false
	})
	return found
}

// ---------- Phase Transition Validation ----------

// ValidatePhaseTransition checks whether a forward phase transition proposal is valid.
func (k Keeper) ValidatePhaseTransition(ctx sdk.Context, targetPhase types.ResearchFundPhase) error {
	state := k.GetResearchFundGovernanceState(ctx)

	// Must target next phase (no skipping).
	if targetPhase != state.CurrentPhase+1 {
		return types.ErrInvalidPhaseTarget
	}

	// Cannot propose during rollback cooldown.
	if state.RollbackCooldownUntil > 0 && uint64(ctx.BlockHeight()) < state.RollbackCooldownUntil {
		return types.ErrRollbackCooldownActive
	}

	// Check there's no existing active phase transition.
	if k.HasActivePhaseTransitionLIP(ctx) {
		return types.ErrPendingTransitionExists
	}

	// Check all exit conditions are met.
	_, allMet := k.CheckPhaseExitConditions(ctx)
	if !allMet {
		return types.ErrExitConditionsNotMet
	}

	return nil
}

// ValidatePhaseTransitionLIP is called during LIP submission for phase
// transition categories. It validates the transition and creates metadata.
func (k Keeper) ValidatePhaseTransitionLIP(ctx sdk.Context, lip *types.LIP) error {
	if lip.Category == types.CategoryPhaseTransition {
		// Parse target phase from description (JSON: {"target_phase": N}).
		var meta types.PhaseTransitionMeta
		if err := json.Unmarshal([]byte(lip.Description), &meta); err != nil {
			return fmt.Errorf("phase transition description must be JSON with target_phase: %w", err)
		}

		targetPhase := types.ResearchFundPhase(meta.TargetPhase)
		if err := k.ValidatePhaseTransition(ctx, targetPhase); err != nil {
			return err
		}

		// Snapshot conditions at submission.
		conditions, _ := k.CheckPhaseExitConditions(ctx)

		k.SetPhaseTransitionMeta(ctx, &types.PhaseTransitionProposal{
			LipID:              lip.Id,
			TargetPhase:        targetPhase,
			ConditionsSnapshot: conditions,
			Stage:              types.PhaseTransitionStagePending,
		})
	}

	if lip.Category == types.CategoryPhaseRollback {
		if err := k.ValidatePhaseRollback(ctx); err != nil {
			return err
		}

		state := k.GetResearchFundGovernanceState(ctx)
		k.SetPhaseTransitionMeta(ctx, &types.PhaseTransitionProposal{
			LipID:       lip.Id,
			TargetPhase: state.CurrentPhase - 1,
			Stage:       types.PhaseTransitionStagePending,
			IsRollback:  true,
		})
	}

	return nil
}

// ---------- Phase Transition Post-Vote Processing ----------

// HandlePhaseTransitionPass is called when a phase transition LIP passes
// the supermajority vote. It sets the activation block with a delay.
func (k Keeper) HandlePhaseTransitionPass(ctx sdk.Context, lipID string) {
	meta, found := k.GetPhaseTransitionMeta(ctx, lipID)
	if !found {
		return
	}

	currentHeight := uint64(ctx.BlockHeight())
	meta.ActivationBlock = currentHeight + types.TransitionActivationDelay
	// Stage remains pending_activation; BeginBlocker will execute it.
	k.SetPhaseTransitionMeta(ctx, meta)

	ctx.EventManager().EmitEvent(
		sdk.NewEvent(
			"zerone.gov.phase_transition_passed",
			sdk.NewAttribute("lip_id", lipID),
			sdk.NewAttribute("target_phase", fmt.Sprint(meta.TargetPhase)),
			sdk.NewAttribute("activation_block", fmt.Sprintf("%d", meta.ActivationBlock)),
			sdk.NewAttribute("is_rollback", fmt.Sprintf("%t", meta.IsRollback)),
		),
	)
}

// HandlePhaseTransitionFail is called when a phase transition LIP fails.
func (k Keeper) HandlePhaseTransitionFail(ctx sdk.Context, lipID string) {
	meta, found := k.GetPhaseTransitionMeta(ctx, lipID)
	if !found {
		return
	}

	meta.Stage = types.PhaseTransitionStageFailed
	k.SetPhaseTransitionMeta(ctx, meta)
}

// ---------- BeginBlocker Phase Transition Processing ----------

// BeginBlockPhaseTransition checks for pending phase transitions ready to activate.
func (k Keeper) BeginBlockPhaseTransition(ctx sdk.Context) {
	currentHeight := uint64(ctx.BlockHeight())

	k.IteratePhaseTransitionMeta(ctx, func(meta *types.PhaseTransitionProposal) bool {
		if meta.Stage != types.PhaseTransitionStagePending {
			return false
		}

		if meta.ActivationBlock == 0 || currentHeight < meta.ActivationBlock {
			return false
		}

		// Ready to activate.
		if meta.IsRollback {
			k.ExecuteRollback(ctx)
			meta.Stage = types.PhaseTransitionStageActivated
			k.SetPhaseTransitionMeta(ctx, meta)
		} else {
			// Re-verify conditions at activation time.
			_, stillMet := k.CheckPhaseExitConditions(ctx)
			if !stillMet {
				k.CancelPhaseTransition(ctx, meta, "exit conditions no longer met")
				return false
			}
			k.AdvancePhase(ctx, meta.TargetPhase)
			meta.Stage = types.PhaseTransitionStageActivated
			k.SetPhaseTransitionMeta(ctx, meta)
		}

		return false
	})
}

// ---------- Phase Advance / Cancel ----------

// AdvancePhase executes the phase transition with phase-specific initialization.
func (k Keeper) AdvancePhase(ctx sdk.Context, targetPhase types.ResearchFundPhase) {
	state := k.GetResearchFundGovernanceState(ctx)
	height := uint64(ctx.BlockHeight())

	oldPhase := state.CurrentPhase
	state.CurrentPhase = targetPhase
	state.PhaseStartedAtBlock = height
	state.ProposalsExecutedInPhase = 0
	state.LastTransitionBlock = height

	// Phase-specific initialization.
	switch targetPhase {
	case types.ResearchFundPhase_RESEARCH_FUND_PHASE_OBSERVER:
		// Phase 1: 1 community seat available, needs election.
		state.CommunitySeats = []string{""}
		state.SeatTermEndBlocks = []uint64{0}

	case types.ResearchFundPhase_RESEARCH_FUND_PHASE_BALANCED:
		// Phase 2: expand to 3 community seats with staggered terms.
		existing := ""
		if len(state.CommunitySeats) > 0 {
			existing = state.CommunitySeats[0]
		}
		state.CommunitySeats = []string{existing, "", ""}
		state.SeatTermEndBlocks = []uint64{
			height + types.SeatStaggerOffset0,
			height + types.SeatStaggerOffset1,
			height + types.SeatStaggerOffset2,
		}

	case types.ResearchFundPhase_RESEARCH_FUND_PHASE_FULL_GOVERNANCE:
		// Phase 3: clear community seats (no longer needed).
		state.CommunitySeats = nil
		state.SeatTermEndBlocks = nil
	}

	k.SetResearchFundGovernanceState(ctx, state)

	ctx.EventManager().EmitEvent(
		sdk.NewEvent(
			"zerone.gov.research_fund_phase_transition",
			sdk.NewAttribute("from_phase", fmt.Sprint(oldPhase)),
			sdk.NewAttribute("to_phase", fmt.Sprint(targetPhase)),
			sdk.NewAttribute("block", fmt.Sprint(height)),
		),
	)
}

// CancelPhaseTransition cancels a pending transition and records the reason.
func (k Keeper) CancelPhaseTransition(ctx sdk.Context, meta *types.PhaseTransitionProposal, reason string) {
	meta.Stage = types.PhaseTransitionStageCancelled
	meta.CancelReason = reason
	k.SetPhaseTransitionMeta(ctx, meta)

	ctx.EventManager().EmitEvent(
		sdk.NewEvent(
			"zerone.gov.phase_transition_cancelled",
			sdk.NewAttribute("lip_id", meta.LipID),
			sdk.NewAttribute("reason", reason),
		),
	)
}
