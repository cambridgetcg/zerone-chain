# E2E Full Truth-Seeking Loop Report

**Date:** 2026-02-27
**Test script:** `scripts/e2e-full-loop.sh`
**Chain:** zerone-localnet (4 validators, Cosmos SDK v0.50.15 + CometBFT v0.38.20)
**Total runtime:** ~355s (~6 minutes)

## Checkpoint Summary

| # | Checkpoint | Result | Details |
|---|-----------|--------|---------|
| 1 | Account Registration | PASS | alice (human), sage1 (agent), rogue (agent), val0-val3 (agent) all registered |
| 2 | Block Rewards Flowing | **FAIL** | Staking keeper nil in VestingRewardsKeeper (see below) |
| 3 | Partnership Formation | PASS | alice + sage1 partnership active |
| 4 | Domain Qualification | PASS | val0/val1 qualified for "general", val2/val3 correctly rejected |
| 5 | Claim Verification | PASS | Claim ACCEPTED (status=6), verdict=ACCEPT, partnership common pot funded (2,100,000 uzrn) |
| 6 | Research Auto-Resolution | PASS | 2 reviews submitted, auto-resolved with aggregate score 82 |
| 7 | Negative Tests | PASS | Fake partnership rejected (code 80), coercion freeze blocks claims, auto-expiry restores partnership |

**Result: 6/7 passed, 1/7 failed**

## Failure Analysis

### Checkpoint 2: Block Rewards Not Flowing

**Root cause:** `VestingRewardsKeeper.stakingKeeper` is `nil`.

In `app/app.go:797-802`, the keeper is constructed with `nil` for the staking keeper:
```go
app.VestingRewardsKeeper = vestingrewardskeeper.NewKeeper(
    appCodec,
    sdkruntime.NewKVStoreService(keys[vestingrewardstypes.StoreKey]),
    app.BankKeeper,
    nil, // staking keeper set after x/staking wiring  <-- NEVER ACTUALLY SET
    authtypes.NewModuleAddress(govtypes.ModuleName).String(),
)
```

The comment says "set after x/staking wiring" but no subsequent `SetStakingKeeper` call exists. This causes:
1. `GetStakingKeeper()` returns nil
2. `activeValidatorCount` stays 0
3. `hasTransactions = GetBlockTxCount() > 0 && activeValidatorCount > 0` is always false
4. `DistributeBlockReward` treats every block as empty -> no rewards minted

**Severity:** HIGH - No block rewards are being minted at all. This affects the entire economic model.

**Fix:** Add a `SetStakingKeeper` method to `x/vesting_rewards/keeper/keeper.go` and wire it in `app/app.go` after the staking keeper is created (similar to how `SetAutopoiesisKeeper` is wired at line 1007).

**Note:** The `protocol_treasury` module account is also a placeholder that never receives funds by design (treasury share stays in the vesting_rewards module account). This is separate from the staking keeper bug.

## The Full Loop

The complete truth-seeking loop was successfully exercised:

```
Register (human + agent + validators)
  -> Form Partnership (alice + sage1)
    -> Qualify for Domain (val0, val1 qualified; val2 rejected)
      -> Submit Claim (through partnership, 1 ZRN stake)
        -> Qualified Verifiers (val0, val1 selected)
          -> Commit-Reveal Verification (both vote ACCEPT)
            -> Claim ACCEPTED, Rewards to Partnership
              -> Submit Research -> 2 Reviews -> Auto-Resolve (score 82)
```

Plus negative tests:
- Fake partnership ID -> rejected (code 80)
- Coercion signal -> partnership frozen -> claims blocked -> auto-expiry restores partnership
- Tree project creation -> consistent across validators

## Phase Timing

| Phase | Duration | Notes |
|-------|----------|-------|
| Phase 1: Chain Verify | 5s | |
| Phase 2: Registration | 64s | 7 accounts (3 test + 4 validators) |
| Phase 3: Block Rewards | 24s | 3 txs submitted, balances checked |
| Phase 4: Partnership | 13s | Propose + accept |
| Phase 5: Qualification | 14s | 2 validators qualified |
| Phase 6: Claim + Verification | 66s | Submit, commit, wait, reveal, wait, aggregate |
| Phase 7: Research | 86s | Submit, 2 reviews, wait 25 blocks for auto-resolution |
| Phase 8: Negative Tests | 83s | 3 tests + 20-block coercion expiry wait |

## Key Technical Findings

1. **Gas auto-estimation unreliable:** `--gas auto --gas-adjustment 1.5` consistently under-estimated gas for sequential operations (state changes between simulation and execution). Fixed by using `--gas 300000` (fixed).

2. **Validators need zerone_auth registration:** The `ZeroneCapabilityDecorator` in the AnteHandler blocks unregistered accounts from submitting knowledge commitments and research reviews (error code 30). Validators must be registered in zerone_auth before participating in verification.

3. **Proto enum values in JSON:** Query results return numeric proto enum values (e.g., ClaimStatus 6 = ACCEPTED, Verdict 1 = ACCEPT, Phase 4 = COMPLETE) rather than string names when using `--output json`.

4. **Tree module CLI panic:** `zeroned tx tree create-project` panics or errors with `couldn't make client config: mkdir /.zeroned` when the home directory isn't properly propagated. The tree functionality works (state is consistent across validators) but the CLI has a bug.

5. **Coercion auto-expiry works:** Partnership coercion signals correctly freeze partnerships and auto-expire after `coercion_review_blocks` (15 blocks in test config), returning the partnership to active status.

6. **Research auto-resolution works:** With `review_period_blocks=20` and `min_reviewer_count=2`, research is auto-resolved in the EndBlocker after the review period passes and minimum reviews are submitted. Aggregate score correctly computed (82 from scores 80 and 85).

## Testnet Readiness Verdict

### NOT YET - 1 Blocker

**Blocker:** Block rewards not flowing due to nil staking keeper in VestingRewardsKeeper. This must be fixed before testnet launch as it affects the entire PoT economic model (no ZRN is being minted).

**Fix required in `app/app.go`:**
1. Add `SetStakingKeeper` method to VestingRewardsKeeper
2. Wire it after staking keeper creation (around line 1007 where other post-init wiring happens)

**After this fix:** All other critical paths work correctly. The truth-seeking loop (register -> partner -> qualify -> claim -> verify -> reward -> research -> auto-resolve) is fully functional. Negative tests (capability enforcement, coercion freeze, tree determinism) all pass.

**Minor issues (non-blocking):**
- Tree module CLI home directory handling (state works, CLI doesn't)
- Gas estimation unreliable for E2E testing (use fixed gas)
