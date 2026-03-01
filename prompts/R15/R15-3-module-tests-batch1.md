# R15-3 — Port Module Tests Batch 1: Partnerships, Research, Claiming Pot, Emergency

## Context

Four modules with significant test gaps:

| Module | Zerone | Prototype | Gap | Priority |
|--------|--------|-----------|-----|----------|
| research | 11 | 48 | 37 | High — treasury management |
| partnerships | 58 | 91 | 33 | High — collaboration logic |
| claiming_pot | 13 | 41 | 28 | Medium — eligibility |
| emergency | 22 | 45 | 23 | Medium — kill switch |
| **Total** | **104** | **225** | **121** | |

## Task

### Research (~37 new tests)

Port from `legible-money/x/research/keeper/keeper_test.go`:

**Submission lifecycle:**
- TestSubmitResearch, TestSubmitResearchInsufficientStake, TestSubmitResearchSequentialIds

**Review process:**
- TestReviewResearch, TestReviewResearchMultipleReviewers, TestReviewResearchDuplicateReviewer

**Resolution:**
- TestResolveResearchAccepted, TestResolveResearchRejected, TestResolveResearchInsufficientReviews

**Challenge system:**
- TestChallengeResearch, TestChallengeResearchNotFound, TestChallengeResearchAlreadyAccepted

**Treasury:**
- TestDisbursement, TestDisbursementInsufficientFunds, TestBountyCreation, TestBountyCompletion

**Governance:**
- TestUpdateParams, TestUpdateParamsUnauthorized

**Progress tracking:**
- TestProgressReport, TestProgressReportNotFound, TestResearchExpiry

Target: research ≥ 40 tests.

### Partnerships (~33 new tests)

Port from `legible-money/x/partnerships/keeper/`:

**Consortium tests** (most of the gap — prototype has `consortium_test.go`):
- TestConsortiumFormation, TestConsortiumMaxMembers, TestConsortiumMinMembers
- TestConsortiumSharesValidation, TestConsortiumRewardSplit
- TestConsortiumMemberExit, TestConsortiumDissolution
- TestGetActiveConsortiumForAddress
- TestConsortiumMemberAlreadyInPartnership, TestConsortiumMemberAlreadyInConsortium
- TestConsortiumFormationExpiry, TestSettleCoolingConsortia
- TestConsortiumCRUD, TestAcceptConsortiumAlreadyAccepted, TestAcceptConsortiumNotMember

**Anti-coercion:**
- TestAntiCoercionDelay, TestAntiCoercionCoolingPeriod

**Deliberation:**
- TestDeliberationWindow, TestDeliberationVoting

Target: partnerships ≥ 80 tests.

### Claiming Pot (~28 new tests)

Port from `legible-money/x/claiming_pot/keeper/`:

**Eligibility:**
- TestCheckEligibility, TestCheckEligibilityIneligible, TestCheckEligibilityExpired

**Claim lifecycle:**
- TestSubmitClaim, TestSubmitClaimDuplicate, TestClaimApproval, TestClaimRejection

**Pot management:**
- TestCreatePot, TestFundPot, TestPotExhausted, TestPotExpiry

**Distribution:**
- TestDistributeRewards, TestDistributeProRata, TestDistributeWithMinimum

**Security:**
- TestDoubleClaimPrevention, TestUnauthorizedClaimApproval

Target: claiming_pot ≥ 35 tests.

### Emergency (~23 new tests)

Port from `legible-money/x/emergency/keeper/`:

**Halt/Resume lifecycle:**
- TestHaltChain, TestResumeChain, TestHaltAlreadyHalted, TestResumeNotHalted

**Guardian council:**
- TestGuardianCouncilQuorum, TestGuardianVote, TestGuardianVoteDuplicate
- TestGuardianCouncilThreshold, TestAddGuardian, TestRemoveGuardian

**3-phase BFT kill switch:**
- TestPhaseTransition, TestPhaseTimeout, TestPhaseEscalation

**Cascade effects:**
- TestHaltBlocksTransactions, TestHaltAllowsEmergencyTx
- TestPartialHalt, TestModuleSpecificHalt

**Security:**
- TestUnauthorizedHalt, TestUnauthorizedResume, TestHaltWithInsufficientGuardians

Target: emergency ≥ 40 tests.

## Adaptation Rules

- Module path: `github.com/zerone-chain/zerone/x/{module}`
- Denom: `uzrn`
- BPS scale: 1,000,000
- Read existing zerone test patterns for each module first
- Match the existing test harness (setupKeeper, etc.)

## Verification

```bash
go test ./x/research/... -count=1 -v
go test ./x/partnerships/... -count=1 -v
go test ./x/claiming_pot/... -count=1 -v
go test ./x/emergency/... -count=1 -v
go vet ./x/research/... ./x/partnerships/... ./x/claiming_pot/... ./x/emergency/...
```

## Commit Convention

```
test(R15-3): port research submission + review + treasury tests
test(R15-3): port partnerships consortium + anti-coercion tests
test(R15-3): port claiming_pot eligibility + distribution tests
test(R15-3): port emergency halt/resume + guardian council tests
```
