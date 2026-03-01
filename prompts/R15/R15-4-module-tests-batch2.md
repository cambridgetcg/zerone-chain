# R15-4 — Port Module Tests Batch 2: Gov, Staking, Channels, Disputes, Discovery, IBC, App-Level

## Context

Remaining module gaps plus missing app-level tests:

| Module | Zerone | Prototype | Gap |
|--------|--------|-----------|-----|
| gov | 73 | 99 | 26 |
| channels | 75 | 95 | 20 |
| staking | 95 | 113 | 18 |
| disputes | 65 | 82 | 17 |
| discovery | 15 | 32 | 17 |
| icaauth | 18 | 33 | 15 |
| ibcratelimit | 50 | 65 | 15 |
| **Total module gap** | | | **128** |

**App-level gaps:**
- `app/vote_extensions_test.go` — missing entirely
- `app/research_fund_restriction_test.go` — missing entirely

## Task

### Gov (~26 new tests)

Port from `legible-money/x/gov/keeper/`:

**LIP lifecycle edge cases:**
- TestLIPExpiredBeforeVoting, TestLIPWithdrawal, TestLIPAmendment
- TestLIPCategoryValidation, TestLIPDuplicateTitle

**Param change execution:**
- TestParamChangeApplied, TestParamChangeInvalidModule, TestParamChangeRollbackOnError

**Upgrade proposals:**
- TestUpgradeLIPScheduled, TestUpgradeLIPHeightValidation

**Quadratic voting:**
- TestQuadraticVoteWeight, TestQuadraticVoteLargeStake

**Research spend:**
- TestResearchSpendProposal, TestResearchSpendExpiry, TestResearchSpendInsufficientFunds

Target: gov ≥ 90 tests.

### Channels (~20 new tests)

Port from `legible-money/x/channels/keeper/`:

**Payment channel lifecycle:**
- TestCooperativeClose, TestCooperativeCloseInvalidSig

**State updates:**
- TestStateUpdateSequenceNumber, TestStateUpdateExpired
- TestStateUpdateBalanceConservation

**Dispute resolution:**
- TestDisputeTimeout, TestDisputeCounterEvidence
- TestDisputeWithHigherSequence

**Edge cases:**
- TestChannelWithZeroBalance, TestChannelMaxLifetime
- TestMultipleChannelsBetweenPair

Target: channels ≥ 85 tests.

### Staking (~18 new tests)

Port from `legible-money/x/staking/keeper/`:

**Tier transitions:**
- TestTierPromotion_VerifiedToBonded, TestTierDemotion_BondedToVerified
- TestTierPromotion_RequiresMinStake, TestTierPromotion_RequiresReputation

**Slashing:**
- TestSlashValidator, TestSlashBelowMinStake_Demotion
- TestDoubleSignSlash, TestDowntimeSlash

**Reputation:**
- TestReputationAccumulation, TestReputationDecay
- TestReputationImpactsVRFWeight

**Delegation edge cases:**
- TestDelegateToInactiveValidator, TestUndelegateMoreThanStaked
- TestRedelegation

Target: staking ≥ 105 tests.

### Disputes (~17 new tests)

Port from `legible-money/x/disputes/keeper/`:

**Resolution logic:**
- TestDisputeResolutionByEvidence, TestDisputeResolutionByVoting
- TestDisputeResolutionTimeout

**Tier-specific configs:**
- TestDisputeConfigByClaimType, TestDisputeStakeRequirements

**Evidence submission:**
- TestSubmitMultipleEvidence, TestEvidenceDeadline
- TestEvidenceFromNonParty

**Edge cases:**
- TestDisputeAlreadyResolved, TestDisputeAgainstOwnClaim
- TestCascadingDisputes

Target: disputes ≥ 75 tests.

### Discovery (~17 new tests)

Port from `legible-money/x/discovery/keeper/`:

- TestDiscoveryRegistration, TestDiscoveryUpdate, TestDiscoveryDeregistration
- TestDiscoverySearch, TestDiscoverySearchByCategory, TestDiscoverySearchPagination
- TestDiscoveryRanking, TestDiscoveryRankingDecay
- TestDiscoveryMetadata, TestDiscoveryVerification
- TestDiscoveryExpiry, TestDiscoveryReactivation

Target: discovery ≥ 28 tests.

### ICA Auth + IBC Rate Limit (~30 new tests combined)

**icaauth** — port from `legible-money/x/icaauth/keeper/`:
- TestRegisterInterchainAccount, TestSubmitInterchainTx
- TestICATxTimeout, TestICATxUnauthorized
- TestICAAccountReuse, TestICAChannelOrdering

**ibcratelimit** — port remaining from `legible-money/x/ibcratelimit/keeper/`:
- TestRateLimitEpochReset, TestRateLimitPerDenom
- TestRateLimitQuotaOverflow, TestRateLimitWhitelist
- TestRateLimitInboundOutbound

Target: icaauth ≥ 28 tests, ibcratelimit ≥ 60 tests.

### App-Level Tests (~15 new tests)

**`app/vote_extensions_test.go`** (new file):
- TestExtendVote_ProducesVRF
- TestVerifyVoteExtension_Valid, TestVerifyVoteExtension_Invalid
- TestPrepareProposal_IncludesVoteExtensions
- TestProcessProposal_ValidatesExtensions

**`app/research_fund_restriction_test.go`** (new file):
- TestResearchFundRestriction_BlocksUnauthorizedSpend
- TestResearchFundRestriction_AllowsGovernanceSpend

Port from prototype's `app/research_fund_restriction_test.go` and create vote extension tests based on `app/vote_extensions.go`.

## Verification

```bash
go test ./x/gov/... ./x/channels/... ./x/staking/... ./x/disputes/... -count=1
go test ./x/discovery/... ./x/icaauth/... ./x/ibcratelimit/... -count=1
go test ./app/... -count=1
go vet ./...
```

## Commit Convention

```
test(R15-4): port gov LIP lifecycle + param change tests
test(R15-4): port staking tier + slashing + channels dispute tests
test(R15-4): port discovery + icaauth + ibcratelimit tests
test(R15-4): add vote_extensions + research_fund_restriction app tests
```
