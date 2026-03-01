# R17-2 — x/gov Extensions: Variable N-of-M Multisig + Phase Tracking

## Objective

Extend the governance module to support the research fund's variable multisig that evolves across phases, and implement phase state tracking with automatic condition checking.

## Prerequisites

R17-1 complete (proto types generated, design doc written).

## Changes Required

### 1. Phase State Keeper Methods

Add to `x/gov/keeper/`:

```go
// GetResearchFundPhase returns the current research fund governance phase.
func (k Keeper) GetResearchFundPhase(ctx sdk.Context) types.ResearchFundPhase

// SetResearchFundPhase stores the current phase and emits a transition event.
func (k Keeper) SetResearchFundPhase(ctx sdk.Context, phase types.ResearchFundPhase)

// GetResearchFundGovernanceState returns the full governance state.
func (k Keeper) GetResearchFundGovernanceState(ctx sdk.Context) *types.ResearchFundGovernanceState

// SetResearchFundGovernanceState stores the full governance state.
func (k Keeper) SetResearchFundGovernanceState(ctx sdk.Context, state *types.ResearchFundGovernanceState)

// IncrementProposalsExecuted increments the executed proposal counter for the current phase.
// Called by ResearchSpendProposal execution handler.
func (k Keeper) IncrementProposalsExecuted(ctx sdk.Context)
```

### 2. Multisig Validation — Phase-Aware

Modify `ResearchSpendProposal` execution to be phase-aware:

**Phase 0 (2-of-2):** Existing logic — require voter1 + voter2 from `ResearchFundVoters`.

**Phase 1 (2-of-3):** Extend `ResearchFundVoters` to support N voters:
```go
// Current (Phase 0):
type ResearchFundVoters struct {
    Voter1 string
    Voter2 string
}

// Extended (Phase 1+):
// Add to ResearchFundGovernanceState.community_seats
// Threshold determined by phase:
//   Phase 0: 2-of-2 (voter1 + voter2)
//   Phase 1: 2-of-3 (voter1 + voter2 + community_seats[0])
//   Phase 2: 3-of-5 (voter1 + voter2 + community_seats[0..2])
//   Phase 3: N/A (standard LIP)
```

Implement `GetResearchFundThreshold(phase) (required, total)`:
```go
func GetResearchFundThreshold(phase ResearchFundPhase) (uint32, uint32) {
    switch phase {
    case RESEARCH_FUND_PHASE_GENESIS_PAIR:
        return 2, 2
    case RESEARCH_FUND_PHASE_OBSERVER:
        return 2, 3
    case RESEARCH_FUND_PHASE_BALANCED:
        return 3, 5
    case RESEARCH_FUND_PHASE_FULL_GOVERNANCE:
        return 0, 0 // not used — standard LIP
    }
}
```

### 3. ResearchSpendProposal — Phase Routing

Modify the `ResearchSpendProposal` execution path:

```go
func (k Keeper) ExecuteResearchSpend(ctx sdk.Context, proposal *ResearchSpendProposal) error {
    state := k.GetResearchFundGovernanceState(ctx)
    
    switch state.CurrentPhase {
    case RESEARCH_FUND_PHASE_FULL_GOVERNANCE:
        // Standard LIP path — already passed vote
        return k.disburseResearchFund(ctx, proposal)
    default:
        // Multisig path — check threshold
        threshold, total := GetResearchFundThreshold(state.CurrentPhase)
        approvals := k.countResearchSpendApprovals(ctx, proposal, state)
        if approvals < threshold {
            return ErrInsufficientApprovals
        }
        err := k.disburseResearchFund(ctx, proposal)
        if err == nil {
            k.IncrementProposalsExecuted(ctx)
        }
        return err
    }
}
```

### 4. Phase Condition Checker

Implement `CheckPhaseExitConditions()` — called on-demand (by transition LIP or query):

```go
func (k Keeper) CheckPhaseExitConditions(ctx sdk.Context) (*types.PhaseTransitionConditions, bool) {
    state := k.GetResearchFundGovernanceState(ctx)
    conditions := k.GetPhaseExitConditions(state.CurrentPhase)
    
    actual := &types.PhaseTransitionConditions{
        DistinctLipVoters:          k.CountDistinctVoters(ctx),
        ActiveGuardians:            k.stakingKeeper.CountActiveGuardians(ctx),
        ResearchFundBalance:        k.GetResearchFundBalance(ctx).String(),
        ChainAgeBlocks:             uint64(ctx.BlockHeight()),
        ProposalsExecutedInPhase:   state.ProposalsExecutedInPhase,
        CommunitySeatParticipation: k.CountCommunitySeatVotes(ctx, state),
        EmergencyHaltsFromMisuse:   k.emergencyKeeper.CountHaltsForReason(ctx, "research_fund"),
    }
    
    met := checkAllConditionsMet(actual, conditions)
    return actual, met
}
```

This requires keeper interfaces:
- `stakingKeeper.CountActiveGuardians(ctx) uint64` — count validators at Guardian tier
- `emergencyKeeper.CountHaltsForReason(ctx, reason) uint64` — count emergency halts with specific reason tag

Add these to `x/gov/types/expected_keepers.go`.

### 5. Distinct Voter Tracking

Track unique governance participants. Options:

**Option A: Epoch-based counter (recommended)**
- Maintain a KV store of `voter_address → first_vote_block`
- `CountDistinctVoters()` iterates the prefix and counts entries
- Append-only — once you've voted, you're counted forever

**Option B: Derived from vote records**
- Scan all `Vote` records and count unique voter addresses
- More accurate but O(n) on total votes — may be slow at scale

Recommend Option A for performance. Add to vote casting handler:
```go
func (k Keeper) recordDistinctVoter(ctx sdk.Context, voter string) {
    key := DistinctVoterKey(voter)
    store := k.storeService.OpenKVStore(ctx)
    if exists, _ := store.Has(key); !exists {
        store.Set(key, sdk.Uint64ToBigEndian(uint64(ctx.BlockHeight())))
    }
}
```

### 6. Research Fund Balance Query

Add to keeper (may already exist via bank keeper):
```go
func (k Keeper) GetResearchFundBalance(ctx sdk.Context) sdk.Coins {
    return k.bankKeeper.GetAllBalances(ctx, 
        authtypes.NewModuleAddress(vesting_rewards_types.ResearchFundModuleName))
}
```

### 7. GRPC Query Implementation

Implement `ResearchFundGovernance` query:

```go
func (q queryServer) ResearchFundGovernance(ctx context.Context, req *QueryResearchFundGovernanceRequest) (*QueryResearchFundGovernanceResponse, error) {
    sdkCtx := sdk.UnwrapSDKContext(ctx)
    state := q.Keeper.GetResearchFundGovernanceState(sdkCtx)
    conditions, allMet := q.Keeper.CheckPhaseExitConditions(sdkCtx)
    
    return &QueryResearchFundGovernanceResponse{
        State:              state,
        ExitConditions:     conditions,
        AllConditionsMet:   allMet,
        NextPhase:          state.CurrentPhase + 1,
    }, nil
}
```

### 8. CLI Command

```bash
zeroned query gov research-fund-governance
```

Output:
```
Current Phase: GENESIS_PAIR (0)
Phase Started: Block 0
Proposals Executed: 1 / 3 required
Time in Phase: 1,234,567 blocks (~3.6 months)

Exit Conditions:
  ✅ Distinct LIP voters: 12 / 10 required
  ❌ Active Guardians: 3 / 5 required
  ✅ Research fund balance: 234,567 ZRN / 100,000 required
  ❌ Chain age: 1,234,567 / 2,200,000 blocks required

Next transition: OBSERVER (Phase 1) — 2 conditions remaining
```

## Verification

```bash
# Build
go build ./x/gov/...

# Phase state roundtrip
go test ./x/gov/keeper/ -v -run "ResearchFundPhase"

# Condition checking
go test ./x/gov/keeper/ -v -run "PhaseExitConditions"

# Multisig threshold
go test ./x/gov/keeper/ -v -run "ResearchFundThreshold"
```

## Commit

```
R17-2: x/gov — variable N-of-M multisig + phase tracking

- ResearchFundGovernanceState stored on-chain (phase, proposals, seats)
- Phase-aware ResearchSpendProposal execution (2-of-2 → 2-of-3 → 3-of-5 → LIP)
- GetResearchFundThreshold() returns required/total per phase
- CheckPhaseExitConditions() validates on-chain maturity metrics
- Distinct voter tracking (append-only KV store)
- GRPC query: current phase + exit condition status
- CLI: zeroned query gov research-fund-governance
```
