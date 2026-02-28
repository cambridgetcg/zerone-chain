# R26-1 — Block Rewards: Activate the Economic Loop

## Context

`x/vesting_rewards` has a fully implemented `SetBlockTxCount(count int)` method at `x/vesting_rewards/keeper/keeper.go:88`. It's never called from the app layer. This means:

- Zero block rewards are ever minted
- Research fund stays at 0
- Development fund stays at 0  
- The entire inflationary economics is dead

This is the single highest-impact fix in the codebase. One call unlocks the entire downstream economy.

## Task

### 1. Wire SetBlockTxCount into PrepareProposal/ProcessProposal

The call site is `app/abci.go`. The proposer knows the tx count.

**In `prepareProposal` (~line 148):**
- After building the final tx list, call `app.VestingRewardsKeeper.SetBlockTxCount(len(txs))`
- This must happen before the response is returned

**In `processProposal` (~line 212):**
- After validating the proposal, call `app.VestingRewardsKeeper.SetBlockTxCount(len(req.Txs))`
- Both proposer and validators must agree on the count

**Important:** The count should include all txs in the block (user txs + injection tx). Verify whether the injection tx (vote extension aggregation) should be counted or excluded — check how `len(txs)` vs `len(req.Txs)` relates to the injection.

### 2. Verify EndBlocker Distribution

Once SetBlockTxCount is called, the EndBlocker in `x/vesting_rewards` should:
- Calculate block rewards based on tx count + params
- Split rewards per the configured ratios (55/22/19.67/3.33)
- Mint tokens to the respective module accounts
- Emit distribution events

**Check:**
```bash
# Before fix: these should all be 0
$BINARY query bank balances $(BINARY query auth module-account protocol_treasury -o json $Q_FLAGS | jq -r '.account.base_account.address') $Q_FLAGS
$BINARY query bank balances $(BINARY query auth module-account development_fund -o json $Q_FLAGS | jq -r '.account.base_account.address') $Q_FLAGS
$BINARY query bank balances $(BINARY query auth module-account research_fund -o json $Q_FLAGS | jq -r '.account.base_account.address') $Q_FLAGS
```

### 3. Test on Localnet

```bash
# Start localnet
scripts/localnet.sh start

# Submit some txs to generate non-zero block reward
$BINARY tx bank send $VAL0_ADDR $VAL1_ADDR 1000uzrn --from val0 $TX_FLAGS
sleep 6  # Wait ~1 block

# Check that rewards were distributed
$BINARY query vesting-rewards params $Q_FLAGS
$BINARY query bank balances <protocol_treasury_addr> $Q_FLAGS
$BINARY query bank balances <research_fund_addr> $Q_FLAGS
```

**Expected:** Non-zero balances in fund accounts after blocks with transactions.

### 4. Write Tests

- Unit test: mock PrepareProposal with N txs, verify SetBlockTxCount called with correct count
- Integration test: submit tx on localnet, wait 1 block, verify fund balances increased
- Edge case: empty block (0 txs) — verify no rewards minted or minimal base reward, depending on params

### 5. Verify Revenue Split Ratios

Cross-reference the actual distribution with configured params:
- 55% contributors
- 22% protocol treasury
- 19.67% development fund
- 3.33% research fund (of which 86% treasury, 7% founder ops, 7% AI ops)

Document any discrepancies.

## Files to Modify

- `app/abci.go` — Add SetBlockTxCount calls in prepareProposal + processProposal
- `app/abci_test.go` — Add tests for tx count propagation
- Possibly `x/vesting_rewards/keeper/keeper.go` — If the EndBlocker distribution has issues once activated

## Success Criteria

- [ ] Block rewards minted after blocks with transactions
- [ ] Fund accounts have non-zero balances on localnet
- [ ] Revenue split matches configured ratios (within rounding)
- [ ] Empty blocks produce 0 (or minimal base) rewards
- [ ] All existing tests still pass
- [ ] New tests cover the wiring
