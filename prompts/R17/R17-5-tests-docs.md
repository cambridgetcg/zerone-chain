# R17-5 — Tests + Tokenomics Documentation

## Objective

Comprehensive tests for the governance migration system and the formal documentation in the tokenomics directory.

## Prerequisites

R17-2, R17-3, R17-4 complete.

## Part 1: Tests

### Unit Tests — `x/gov/keeper/`

#### Phase State Tests
```
TestResearchFundPhase_DefaultIsGenesisPair
TestResearchFundPhase_GetSet_Roundtrip
TestResearchFundPhase_IncrementProposals
TestResearchFundPhase_ExportImportGenesis
```

#### Exit Condition Tests
```
TestExitConditions_Phase0_AllMet
TestExitConditions_Phase0_VotersMissing
TestExitConditions_Phase0_GuardiansMissing
TestExitConditions_Phase0_BalanceTooLow
TestExitConditions_Phase0_ChainTooYoung
TestExitConditions_Phase1_AllMet
TestExitConditions_Phase1_ProposalCountInsufficient
TestExitConditions_Phase1_CommunitySeatInactive
TestExitConditions_Phase2_AllMet
TestExitConditions_Phase2_EmergencyHaltBlocksTransition
```

#### Multisig Threshold Tests
```
TestThreshold_Phase0_RequiresBothFounders
TestThreshold_Phase1_TwoOfThree_FoundersApprove
TestThreshold_Phase1_TwoOfThree_OneFounderOneCommunity
TestThreshold_Phase1_TwoOfThree_RejectIfOnlyOne
TestThreshold_Phase2_ThreeOfFive_AllCommunityApprove
TestThreshold_Phase2_ThreeOfFive_TwoFoundersOneCommunity
TestThreshold_Phase2_ThreeOfFive_RejectIfOnlyTwo
TestThreshold_Phase2_VacantSeat_AdjustsTotal
TestThreshold_Phase3_StandardLIPPath
```

#### Phase Transition Tests
```
TestTransition_ValidateTarget_MustBeNextPhase
TestTransition_ValidateTarget_CannotSkipPhase
TestTransition_RollbackCooldown_RejectsTransition
TestTransition_Supermajority_66Percent_Passes
TestTransition_Supermajority_60Percent_Fails
TestTransition_ActivationDelay_ExecutesAtCorrectBlock
TestTransition_ActivationDelay_CancelsIfConditionsDegraded
TestTransition_Phase0to1_InitializesOneSeat
TestTransition_Phase1to2_ExpandsToThreeSeats_StaggeredTerms
TestTransition_Phase1to2_PreservesExistingSeatHolder
TestTransition_Phase2to3_ClearsSeats_EnablesLIPPath
```

#### Rollback Tests
```
TestRollback_RequiresGridlock_OrEmergencyHalt
TestRollback_GridlockThreshold_ThreeExpiredProposals
TestRollback_CannotRollbackBelowGenesis
TestRollback_SetsCooldown_ThreeMonths
TestRollback_Phase2to1_ResizesSeats
TestRollback_Phase1to0_ClearsAllSeats
TestRollback_CooldownPreventsImmediateReTransition
```

#### Election Tests
```
TestSeatElection_CandidateMustBeGuardian
TestSeatElection_CandidateMustHaveFiveVotes
TestSeatElection_CandidateMustAccept
TestSeatElection_AutoFailWithoutAcceptance
TestSeatElection_ContestedElection_HighestStakeWins
TestSeatElection_ContestedElection_RunoffIfWithin5Percent
TestSeatElection_InstallsWinnerToCorrectSeat
TestSeatElection_RejectIfSeatAlreadyOccupied
```

#### Term Rotation Tests
```
TestTermExpiry_ClearsSeatAtEndBlock
TestTermExpiry_EmitsEvent
TestTermExpiry_VacancyWarning_After30Days
TestTermExpiry_ReElection_AllowsIncumbent
TestTermStagger_Phase2_InitialOffsets
```

#### Emergency Removal Tests
```
TestEmergencyRemoval_RequiresSupermajority
TestEmergencyRemoval_JailedValidator
TestEmergencyRemoval_SlashedThreeTimes
TestEmergencyRemoval_RejectWithoutGrounds
```

### Integration Tests — `tests/integration/`

```
TestResearchFundGovernance_FullLifecycle_Phase0Through3
TestResearchFundGovernance_RollbackAndRecovery
TestResearchFundGovernance_ElectionCycle
TestResearchFundGovernance_MultisigSpend_AllPhases
```

The full lifecycle test should:
1. Start at Phase 0
2. Simulate enough activity to meet Phase 0 exit conditions
3. Submit and pass a transition proposal
4. Verify Phase 1 state
5. Run an election for the community seat
6. Execute a research spend proposal with 2-of-3
7. Continue through Phase 2 and Phase 3
8. Verify standard LIP path works in Phase 3

### Simulation — `tests/simulation/`

Add invariant:
```
PhaseConsistencyInvariant — verifies that the current phase matches
the multisig threshold being enforced, community seat count matches
phase requirements, and no seat terms exceed the maximum.
```

## Part 2: Documentation

### `docs/tokenomics/GOVERNANCE-MIGRATION.md`

Full specification document (derived from R17-1 design doc but polished for external audience):

Structure:
1. **Overview** — why migration matters, core principle (maturity-gated)
2. **Phase Descriptions** — each phase with structure, powers, exit conditions
3. **Transition Protocol** — how a transition happens (proposal → discussion → supermajority → delay → activation)
4. **Community Seat Elections** — candidacy, voting, terms, rotation
5. **Rollback Safety** — when and how rollback works
6. **Founder Anchor** — permanent 0.23% share, AI vault continuity
7. **Timeline Estimates** — rough estimates of when each phase might be reached (not commitments)
8. **FAQ**:
   - "Can the founders block a phase transition?"
   - "What happens if no one runs for a community seat?"
   - "Can the community remove a founder from the multisig?"
   - "What if the AI vault key is compromised?"
   - "Is Phase 3 truly irreversible?"

### Update `docs/tokenomics/README.md`

Add GOVERNANCE-MIGRATION.md to the table of contents.

### Update `docs/tokenomics/GENESIS.md`

Add section on Phase 0 governance structure — link to GOVERNANCE-MIGRATION.md for full spec.

### Update `docs/tokenomics/REVIEW.md`

Update the "Research Fund Centralisation Risk" item:
- Note that the 4-phase migration plan addresses the concern
- Phase 0 is still centralized (by design) — the risk now has a mitigation timeline
- Add open question: "What if exit conditions are never met?"

### Update `docs/PARAMETERS.md`

Add governance migration parameters:
- Phase exit condition thresholds (per phase)
- Election parameters (stake, review period)
- Transition proposal parameters (stake, discussion period, supermajority threshold)
- Rollback cooldown duration
- Term length, stagger offsets

## Verification

```bash
# All tests pass
go test ./x/gov/... -v -count=1
go test ./tests/integration/... -v -run "ResearchFundGovernance"

# Test count increase
go test ./x/gov/... -v 2>&1 | grep -c "PASS:"
# Should be ≥ 40 new tests

# Documentation exists
ls docs/tokenomics/GOVERNANCE-MIGRATION.md
grep "GOVERNANCE-MIGRATION" docs/tokenomics/README.md
```

## Commit

```
R17-5: tests + tokenomics documentation for governance migration

Tests: 40+ new tests across unit, integration, simulation
- Phase state, exit conditions, multisig thresholds (all phases)
- Phase transitions with supermajority + activation delay
- Rollback lifecycle with cooldown
- Community seat elections, term rotation, emergency removal
- Full lifecycle integration test (Phase 0 → Phase 3)
- PhaseConsistency simulation invariant

Docs: GOVERNANCE-MIGRATION.md in tokenomics directory
- Full spec: 4 phases, election mechanics, rollback, FAQ
- README, GENESIS, REVIEW, PARAMETERS updated with cross-refs
```
