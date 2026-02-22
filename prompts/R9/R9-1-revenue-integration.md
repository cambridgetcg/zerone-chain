# R9-1 — Revenue Flow Integration Tests

## Goal

Port the 10 revenue integration tests from the draft and add new cross-module revenue
flow tests for Zerone. Verify that every revenue path (fees, billing, toolbox, tree,
vesting rewards) routes tokens correctly with no leaks or double-taxation.

## Working Directory

`/Users/yuai/Desktop/Zerone`

## Reference

- `/Users/yuai/Desktop/legible_money/x/integration_test/revenue_test.go` — 10 tests, 1026 LOC
- Rename all `legible` → `zerone`, `ulgm` → `uzrn`, `LGM` → `ZRN`

## Tests to Port

From draft `revenue_test.go`:
1. **TestCompleteRevenueMap** — verify all revenue sources are accounted for
2. **TestFounderSplitAllSources** — founder share from every revenue stream
3. **TestNoDoubleTaxation** — revenue taxed exactly once per flow
4. **TestServiceRevenueBurn** — service revenue burn mechanics
5. **TestDeadAccountsRemoved** — zero-balance accounts cleaned up
6. **TestLedgerBalance** — total supply invariant holds after revenue operations
7. **TestDepositToResearchFund_NoFounder** — research fund deposits bypass founder split
8. **TestBillingResearchRoutedThroughDepositor** — billing revenue reaches research fund
9. **TestVerificationRewardDecay_PoolSolvency** — reward decay doesn't drain pool
10. **TestFullRevenueFlow_WithVerificationPool** — end-to-end revenue from query to validator

## New Tests for Zerone

11. **TestToolboxRevenueCascade** — tool call → revenue splits through dependency DAG → contributors
12. **TestTreeRevenueRouting** — service call → tree project → parent share → contributor shares
13. **TestAutopoiesisMultiplierAffectsRewards** — multiplier change → block reward change
14. **TestResearchFund2of2** — disbursement requires both founder + AI signatures
15. **TestFeeRouterSplit** — tx fee → 7% research fund + 93% validators

## Implementation

### File Structure
```
tests/integration/
├── revenue_test.go        (port from draft, 10 tests)
├── revenue_zerone_test.go (5 new Zerone-specific tests)
└── harness_test.go        (test harness — may reuse cross_stack or create new)
```

### Test Harness
These tests need a near-complete app with:
- Bank keeper for balance checks
- Vesting rewards for block rewards + fee routing
- Billing for query pricing
- Toolbox for tool revenue
- Tree for project revenue routing
- Autopoiesis for multiplier effects

Use the full app harness (like `app_test.go`) or extend `tests/cross_stack/harness_test.go`.

### Key Invariant
After every test: `total_supply_before == total_supply_after` (no tokens created or destroyed
except through explicit mint/burn).

## Constraints

- Every revenue path must be tested (no untested flows)
- Total supply invariant checked in every test
- BPS splits must sum to exactly 1M at every junction
- Use concrete token amounts (not mocks) for realistic balance verification
- All 10 draft tests must pass before adding new ones
