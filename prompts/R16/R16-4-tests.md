# R16-4 — Test Updates: Unit + Integration + Simulation

## Objective

Update all tests to reflect the new revenue split, development fund, founder immutability, and removal of burn.

## Prerequisites

R16-2 and R16-3 complete (all keeper logic updated).

## Test Categories

### 1. `x/vesting_rewards/keeper/keeper_test.go` (~1,774 lines)

High-priority — this is the most comprehensive test file for revenue logic.

**Rename/update tests:**
- `TestDistributeBlockReward_BurnsTokens` → `TestDistributeBlockReward_DevelopmentFund`
  - Assert: development_fund module account receives 19.67% (not burned)
  - Assert: no BurnCoins called on bankKeeper mock
- `TestDistributeRevenue_4WaySplit` → verify new split percentages
  - contributor: 550,000 BPS
  - protocol: 220,000 BPS
  - research: 33,300 BPS
  - development: 196,700 BPS (remainder)
- `TestDistributeRevenue_SplitSumsToTotal` → verify development gets remainder
- `TestDistributeRevenue_ProtocolSubSplit` → unchanged (sub-split unaffected)
- `TestDistributeBlockReward_4WayAccounting` → update assertions
- `TestMintWithCap_BurnRecycling` → **REMOVE or REWRITE**
  - Burn recycling no longer exists (no burns = no recycled headroom)
  - Supply monotonically increases to cap

**New tests to add:**
- `TestFounderShareImmutability_RejectBpsChange` — UpdateParams with changed FounderShareBps should fail
- `TestFounderShareImmutability_RejectAddressChange` — UpdateParams with changed FounderAddress should fail
- `TestFounderShareImmutability_AllowInitialSet` — Setting from zero/empty should succeed
- `TestFounderShareImmutability_AllowIdenticalValues` — Setting same values should succeed
- `TestDevelopmentFundDeposit_BlockReward` — 19.67% of block reward goes to development_fund
- `TestDevelopmentFundDeposit_RouteFees` — 19.67% of fees goes to development_fund
- `TestNoGovernanceSunset` — isFounderShareActive returns true regardless of block height when address is set

**Mock updates:**
- `mockBankKeeper`: remove or keep `BurnCoins` mock but verify it's never called in revenue paths
- Add `SendCoinsFromModuleToModule` mock tracking for development_fund deposits

### 2. `tests/integration/revenue_test.go`

Full-stack integration tests for revenue flow:
- Update expected split ratios in all assertions
- Verify development_fund module account balance grows
- Verify no tokens are burned (total supply = total minted)

### 3. `tests/integration/revenue_zerone_test.go`

Zerone-specific revenue integration:
- Same updates as revenue_test.go
- Verify 4-way split with Zerone-specific modules

### 4. `tests/simulation/invariants.go`

**Critical:** The simulation invariant that checks revenue split sum:
```go
total := split.ContributorBps + split.ProtocolBps + split.ResearchBps + split.BurnBps
```
Change to:
```go
total := split.ContributorBps + split.ProtocolBps + split.ResearchBps + split.DevelopmentBps
```

**Add new invariant:**
- `FounderShareImmutableInvariant` — verify that founder share params match genesis values
- `NoBurnInvariant` — verify that total bank supply == total minted (no burns occurred)

### 5. `tests/simulation/economic_sim_test.go`

Update economic simulation:
- Remove burn-related assertions
- Add development fund accumulation assertions
- Update expected equilibrium calculations

### 6. Module-specific test files

For each module updated in R16-3, update its `keeper_test.go`:

| Module | Test File | Key Changes |
|--------|-----------|-------------|
| toolbox | `x/toolbox/keeper/keeper_test.go` | Split defaults, no burn |
| toolbox | `x/toolbox/keeper/purpose_prompter_test.go` | If it references split |
| billing | `x/billing/keeper/keeper_test.go` | Split defaults, no burn |
| tree | `x/tree/keeper/keeper_test.go` | Split defaults, no burn |
| knowledge | `x/knowledge/keeper/abci_test.go` | Slash routing |
| knowledge | `x/knowledge/keeper/helpers_test.go` | Slash routing |
| disputes | `x/disputes/keeper/keeper_test.go` | Slash routing |
| staking | `x/staking/keeper/keeper_test.go` | Slash routing |
| liquiditypool | `x/liquiditypool/keeper/keeper_test.go` | Fee routing |
| capture_challenge | `x/capture_challenge/keeper/keeper_test.go` | Stake routing |
| partnerships | `x/partnerships/keeper/keeper_test.go` | Exit penalty routing |
| research | `x/research/keeper/keeper_test.go` | Rejection slash routing |

### 7. `tests/integration/harness_test.go`

Update test harness:
- Register `development_fund` module account in test app setup
- Update default revenue split in test fixtures

## Verification

```bash
# All tests pass
go test ./... -count=1

# Specific revenue tests
go test ./x/vesting_rewards/... -v -run "Revenue\|Split\|Burn\|Development\|Founder"
go test ./tests/integration/... -v -run "Revenue"
go test ./tests/simulation/... -v -run "Invariant\|Economic"

# Grep for stale burn assertions
grep -rn "BurnCoins\|BurnAmount\|burn_amount" --include="*_test.go" | grep -v "tokens/"
# Should be 0 (or explicitly testing that burn doesn't happen)
```

## Commit

```
R16-4: update all tests for revenue split refactor

- vesting_rewards: 7 new tests for founder immutability + development fund
- BurnRecycling test removed (no burns in new model)
- All revenue split assertions updated to 55/22/19.67/3.33
- Simulation invariants: added FounderShareImmutable + NoBurn
- Integration tests: verify development_fund receives deposits
- 12 module test files updated for new routing
- Test harness: development_fund module account registered
```
