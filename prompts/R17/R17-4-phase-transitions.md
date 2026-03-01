# R17-4 — Phase Transition LIPs + Rollback Mechanism

## Objective

Implement the on-chain mechanism for proposing, voting on, and executing research fund governance phase transitions, including the rollback safety valve.

## Prerequisites

R17-2 complete (phase tracking, condition checking).

## Changes Required

### 1. New LIP Category: `research_phase_transition`

```go
CategoryConfig{
    Category:         "research_phase_transition",
    RequiredStakeBps: 1_000_000_000, // 1,000 ZRN — very high bar
    ReviewBlocks:     1_030_000,     // ~30 days discussion (not standard ~2 days)
}
```

This is deliberately expensive and slow. Phase transitions reshape the power structure of the research fund. They should not be casual.

### 2. Phase Transition Proposal Type

```protobuf
message PhaseTransitionProposal {
    uint64 proposal_id = 1;
    string proposer = 2;
    ResearchFundPhase target_phase = 3;  // must be current_phase + 1
    string justification = 4;            // why the community is ready
    PhaseTransitionConditions conditions_snapshot = 5;  // recorded at submission
    string stage = 6;
    string yes_stake = 7;
    string no_stake = 8;
    string abstain_stake = 9;
    uint64 voting_end_block = 10;
    uint64 created_at_block = 11;
    uint64 activation_block = 12;        // voting_end + 240,000 (~7 day delay)
}
```

### 3. Submission Validation

When a `PhaseTransitionProposal` is submitted:

```go
func (k Keeper) ValidatePhaseTransition(ctx sdk.Context, proposal *PhaseTransitionProposal) error {
    state := k.GetResearchFundGovernanceState(ctx)
    
    // Must target next phase (no skipping)
    if proposal.TargetPhase != state.CurrentPhase + 1 {
        return ErrInvalidPhaseTarget
    }
    
    // Cannot propose during rollback cooldown
    if state.RollbackCooldownUntil > 0 && uint64(ctx.BlockHeight()) < state.RollbackCooldownUntil {
        return ErrRollbackCooldownActive
    }
    
    // Check all exit conditions are met
    conditions, allMet := k.CheckPhaseExitConditions(ctx)
    if !allMet {
        return ErrExitConditionsNotMet
    }
    
    // Snapshot conditions into proposal
    proposal.ConditionsSnapshot = conditions
    
    return nil
}
```

### 4. Voting — Supermajority Required

Phase transitions use a **66.7% support threshold** (not the standard 50%):

```go
func (k Keeper) ResolvePhaseTransitionVote(ctx sdk.Context, proposal *PhaseTransitionProposal) bool {
    totalVoted := proposal.YesStake + proposal.NoStake + proposal.AbstainStake
    quorum := k.GetQuorumThreshold(ctx) // standard 33.4%
    
    if totalVoted < quorum {
        return false
    }
    
    // Supermajority: 66.7% of non-abstain votes
    nonAbstain := proposal.YesStake + proposal.NoStake
    if nonAbstain == 0 {
        return false
    }
    
    // 667,000 BPS = 66.7% on 1M scale
    return (proposal.YesStake * 1_000_000 / nonAbstain) >= 667_000
}
```

### 5. Activation Delay

After supermajority vote passes:
- 7-day activation delay (`~240,000 blocks`)
- During this period, the transition can be challenged via an emergency halt proposal
- If no challenge: phase advances automatically at `activation_block`

```go
func (k Keeper) BeginBlockPhaseTransition(ctx sdk.Context) {
    // Check for pending transitions ready to activate
    pending := k.GetPendingPhaseTransition(ctx)
    if pending == nil {
        return
    }
    
    height := uint64(ctx.BlockHeight())
    if height < pending.ActivationBlock {
        return
    }
    
    // Re-verify conditions at activation time (they could have degraded)
    _, stillMet := k.CheckPhaseExitConditions(ctx)
    if !stillMet {
        k.CancelPhaseTransition(ctx, pending, "exit conditions no longer met")
        return
    }
    
    // Execute transition
    k.AdvancePhase(ctx, pending.TargetPhase)
}
```

### 6. Phase Advance Execution

```go
func (k Keeper) AdvancePhase(ctx sdk.Context, targetPhase types.ResearchFundPhase) {
    state := k.GetResearchFundGovernanceState(ctx)
    height := uint64(ctx.BlockHeight())
    
    oldPhase := state.CurrentPhase
    state.CurrentPhase = targetPhase
    state.PhaseStartedAtBlock = height
    state.ProposalsExecutedInPhase = 0  // reset counter for new phase
    state.LastTransitionBlock = height
    
    // Phase-specific initialization
    switch targetPhase {
    case types.RESEARCH_FUND_PHASE_OBSERVER:
        // Phase 1: 1 community seat available, needs election
        state.CommunitySeats = []string{""}  // 1 vacant seat
        state.SeatTermEndBlocks = []uint64{0}
        
    case types.RESEARCH_FUND_PHASE_BALANCED:
        // Phase 2: expand to 3 community seats with staggered terms
        // Keep existing Phase 1 seat holder (if any) in seat 0
        existing := ""
        if len(state.CommunitySeats) > 0 {
            existing = state.CommunitySeats[0]
        }
        state.CommunitySeats = []string{existing, "", ""}
        // Stagger initial terms: 2mo, 4mo, 6mo
        state.SeatTermEndBlocks = []uint64{
            height + 2_133_333,  // ~2 months
            height + 4_266_666,  // ~4 months
            height + 6_400_000,  // ~6 months
        }
        
    case types.RESEARCH_FUND_PHASE_FULL_GOVERNANCE:
        // Phase 3: clear community seats (no longer needed)
        state.CommunitySeats = nil
        state.SeatTermEndBlocks = nil
    }
    
    k.SetResearchFundGovernanceState(ctx, state)
    
    ctx.EventManager().EmitEvent(sdk.NewEvent(
        "zerone.gov.research_fund_phase_transition",
        sdk.NewAttribute("from_phase", fmt.Sprint(oldPhase)),
        sdk.NewAttribute("to_phase", fmt.Sprint(targetPhase)),
        sdk.NewAttribute("block", fmt.Sprint(height)),
    ))
}
```

### 7. Rollback Mechanism

New LIP category: `research_phase_rollback`

```go
CategoryConfig{
    Category:         "research_phase_rollback",
    RequiredStakeBps: 500_000_000,  // 500 ZRN
    ReviewBlocks:     240_000,      // ~7 days (faster than forward transition)
}
```

**Trigger conditions** (at least one must be true):
- ≥3 consecutive research spend proposals expired without committee action (gridlock)
- Emergency halt triggered with `reason` containing "research_fund"

```go
func (k Keeper) ValidatePhaseRollback(ctx sdk.Context) error {
    state := k.GetResearchFundGovernanceState(ctx)
    
    if state.CurrentPhase <= types.RESEARCH_FUND_PHASE_GENESIS_PAIR {
        return ErrCannotRollbackGenesis
    }
    
    // Check gridlock
    gridlocked := k.CountConsecutiveExpiredProposals(ctx) >= 3
    
    // Check emergency halt for research fund
    haltTriggered := k.emergencyKeeper.HasHaltForReason(ctx, "research_fund")
    
    if !gridlocked && !haltTriggered {
        return ErrNoRollbackJustification
    }
    
    return nil
}
```

**Rollback voting:** Same supermajority (66.7%) required.

**Rollback execution:**
```go
func (k Keeper) ExecuteRollback(ctx sdk.Context) {
    state := k.GetResearchFundGovernanceState(ctx)
    height := uint64(ctx.BlockHeight())
    
    previousPhase := state.CurrentPhase - 1
    
    // Clear community seats back to previous phase level
    state.CurrentPhase = previousPhase
    state.PhaseStartedAtBlock = height
    state.ProposalsExecutedInPhase = 0
    state.LastTransitionBlock = height
    
    // 3-month cooldown before re-attempting forward transition
    state.RollbackCooldownUntil = height + 3_100_000  // ~3 months
    
    // Resize community seats for previous phase
    switch previousPhase {
    case types.RESEARCH_FUND_PHASE_GENESIS_PAIR:
        state.CommunitySeats = nil
        state.SeatTermEndBlocks = nil
    case types.RESEARCH_FUND_PHASE_OBSERVER:
        if len(state.CommunitySeats) > 1 {
            state.CommunitySeats = state.CommunitySeats[:1]
            state.SeatTermEndBlocks = state.SeatTermEndBlocks[:1]
        }
    }
    
    k.SetResearchFundGovernanceState(ctx, state)
}
```

### 8. Error Types

Add to `x/gov/types/errors.go`:

```go
ErrInvalidPhaseTarget      = errors.Register(ModuleName, N, "target phase must be current + 1")
ErrRollbackCooldownActive  = errors.Register(ModuleName, N, "rollback cooldown active, cannot propose transition")
ErrExitConditionsNotMet    = errors.Register(ModuleName, N, "not all exit conditions are met")
ErrCannotRollbackGenesis   = errors.Register(ModuleName, N, "cannot rollback below genesis phase")
ErrNoRollbackJustification = errors.Register(ModuleName, N, "rollback requires gridlock or emergency halt evidence")
```

### 9. CLI Commands

```bash
# Propose phase transition
zeroned tx gov propose-phase-transition \
    --target-phase 1 \
    --justification "Community has matured: 12 voters, 6 guardians, 150K ZRN in fund..." \
    --from proposer \
    --amount 1000000000uzrn

# Query pending transition
zeroned query gov pending-phase-transition

# Propose rollback
zeroned tx gov propose-phase-rollback \
    --justification "Committee gridlocked: 3 proposals expired without action" \
    --from guardian
```

## Verification

```bash
go build ./x/gov/...

# Phase transition lifecycle
go test ./x/gov/keeper/ -v -run "PhaseTransition"

# Rollback
go test ./x/gov/keeper/ -v -run "Rollback"

# Activation delay
go test ./x/gov/keeper/ -v -run "ActivationDelay"

# Condition re-verification
go test ./x/gov/keeper/ -v -run "ConditionRecheck"
```

## Commit

```
R17-4: phase transition LIPs + rollback mechanism

- research_phase_transition LIP category (1000 ZRN, 30-day discussion)
- PhaseTransitionProposal with condition snapshot + supermajority (66.7%)
- 7-day activation delay with condition re-verification
- BeginBlocker auto-advances phase at activation block
- research_phase_rollback LIP category (500 ZRN, 7-day review)
- Rollback requires gridlock (3 expired proposals) or emergency halt
- 3-month cooldown after rollback before re-attempting forward transition
- CLI: propose-phase-transition, propose-phase-rollback
```
