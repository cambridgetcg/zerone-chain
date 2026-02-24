package keeper

import (
	"fmt"

	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/zerone-chain/zerone/x/gov/types"
)

// CountConsecutiveExpiredProposals counts how many research spend proposals
// expired consecutively (most recent first) without being executed or rejected.
func (k Keeper) CountConsecutiveExpiredProposals(ctx sdk.Context) int {
	var count int
	// Collect all proposals and check the most recent ones.
	var all []*types.ResearchSpendProposal
	k.IterateResearchSpendProposals(ctx, func(prop *types.ResearchSpendProposal) bool {
		all = append(all, prop)
		return false
	})

	// Walk backwards from most recent by proposal ID (highest ID = most recent).
	for i := len(all) - 1; i >= 0; i-- {
		prop := all[i]
		if prop.Stage == string(types.ResearchStageExpired) {
			count++
		} else {
			break // streak broken
		}
	}
	return count
}

// ValidatePhaseRollback checks whether a phase rollback is justified.
func (k Keeper) ValidatePhaseRollback(ctx sdk.Context) error {
	state := k.GetResearchFundGovernanceState(ctx)

	if state.CurrentPhase <= types.ResearchFundPhase_RESEARCH_FUND_PHASE_GENESIS_PAIR {
		return types.ErrCannotRollbackGenesis
	}

	// Check there's no existing active phase transition.
	if k.HasActivePhaseTransitionLIP(ctx) {
		return types.ErrPendingTransitionExists
	}

	// Check gridlock: 3+ consecutive expired proposals.
	gridlocked := k.CountConsecutiveExpiredProposals(ctx) >= types.RollbackGridlockThreshold

	// Check emergency halt for research fund.
	haltTriggered := false
	if k.emergencyKeeper != nil {
		haltTriggered = k.emergencyKeeper.CountHaltsForReason(ctx, "research_fund") > 0
	}

	if !gridlocked && !haltTriggered {
		return types.ErrNoRollbackJustification
	}

	return nil
}

// ExecuteRollback rolls the phase back one level with a cooldown period.
func (k Keeper) ExecuteRollback(ctx sdk.Context) {
	state := k.GetResearchFundGovernanceState(ctx)
	height := uint64(ctx.BlockHeight())

	oldPhase := state.CurrentPhase
	previousPhase := state.CurrentPhase - 1

	state.CurrentPhase = previousPhase
	state.PhaseStartedAtBlock = height
	state.ProposalsExecutedInPhase = 0
	state.LastTransitionBlock = height

	// 3-month cooldown before re-attempting forward transition.
	state.RollbackCooldownUntil = height + types.RollbackCooldownBlocks

	// Resize community seats for previous phase.
	switch previousPhase {
	case types.ResearchFundPhase_RESEARCH_FUND_PHASE_GENESIS_PAIR:
		state.CommunitySeats = nil
		state.SeatTermEndBlocks = nil
	case types.ResearchFundPhase_RESEARCH_FUND_PHASE_OBSERVER:
		if len(state.CommunitySeats) > 1 {
			state.CommunitySeats = state.CommunitySeats[:1]
			state.SeatTermEndBlocks = state.SeatTermEndBlocks[:1]
		}
	}

	k.SetResearchFundGovernanceState(ctx, state)

	ctx.EventManager().EmitEvent(
		sdk.NewEvent(
			"zerone.gov.research_fund_phase_rollback",
			sdk.NewAttribute("from_phase", fmt.Sprint(oldPhase)),
			sdk.NewAttribute("to_phase", fmt.Sprint(previousPhase)),
			sdk.NewAttribute("block", fmt.Sprint(height)),
			sdk.NewAttribute("cooldown_until", fmt.Sprint(state.RollbackCooldownUntil)),
		),
	)
}
