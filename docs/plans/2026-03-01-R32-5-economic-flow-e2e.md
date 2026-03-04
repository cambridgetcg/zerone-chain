# R32-5 Economic Flow E2E Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Docker-based E2E tests verifying the complete economic flow: block rewards, revenue split, research fund, fee distribution, staking rewards, and supply tracking on a real 2-validator chain.

**Architecture:** Single Go test file using the existing interchaintest framework. One shared chain setup (2 validators) with sequential subtests. Genesis overrides enable full rewards for empty blocks and set `min_validators_for_full_reward=2` so the 2-validator E2E chain earns full block rewards.

**Tech Stack:** Go 1.24, interchaintest/v8, testify, Docker (zerone:local image)

---

### Task 1: Add economic genesis overrides to chain config

**Files:**
- Modify: `tests/e2e/chain_config_test.go:41-68`

**Step 1: Add vesting_rewards genesis overrides**

Append these KVs to the existing `testGenesisKV()` return slice in `chain_config_test.go`:

```go
// ── Vesting rewards: enable full rewards for E2E testing ──
cosmos.NewGenesisKV("app_state.vesting_rewards.params.min_validators_for_full_reward", 2),
cosmos.NewGenesisKV("app_state.vesting_rewards.params.empty_block_reward_rate", 10000),
```

These ensure:
- 2 validators = full block reward (no scaling penalty)
- Empty blocks still earn rewards (avoids PoT empty-block zero-reward rule)

**Step 2: Verify compilation**

Run: `cd /Users/yournameisai/Desktop/zerone && go build ./tests/e2e/...`
Expected: No errors

**Step 3: Commit**

```bash
git add tests/e2e/chain_config_test.go
git commit -m "test(e2e): add vesting_rewards genesis overrides for economic tests"
```

---

### Task 2: Create economic flow test file with block reward distribution test

**Files:**
- Create: `tests/e2e/economic_flow_test.go`

**Step 1: Write the test file scaffold + block reward subtest**

Create `tests/e2e/economic_flow_test.go`:

```go
package e2e_test

import (
	"encoding/json"
	"fmt"
	"math/big"
	"testing"

	sdkmath "cosmossdk.io/math"
	"github.com/strangelove-ventures/interchaintest/v8"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// blockRewardDist mirrors the JSON output of `query vesting_rewards block-reward`.
type blockRewardDist struct {
	BlockHeight     string `json:"block_height"`
	ProducerReward  string `json:"producer_reward"`
	ResearchShare   string `json:"research_share"`
	TotalMinted     string `json:"total_minted"`
	ValidatorCount  string `json:"validator_count"`
	FundBalanceAfter string `json:"fund_balance_after"`
	FounderShare    string `json:"founder_share"`
	DevelopmentAmount string `json:"development_amount"`
	ProtocolShare   string `json:"protocol_share"`
}

// researchFundResp mirrors the JSON output of `query vesting_rewards research-fund-balance`.
type researchFundResp struct {
	Balance string `json:"balance"`
	Denom   string `json:"denom"`
}

// paramsResp mirrors the JSON output of `query vesting_rewards params`.
type paramsResp struct {
	Params struct {
		BlockReward                string `json:"block_reward"`
		MinValidatorsForFullReward int    `json:"min_validators_for_full_reward"`
		EmptyBlockRewardRate       int    `json:"empty_block_reward_rate"`
	} `json:"params"`
}

// supplyResp mirrors the JSON output of `query bank total`.
type supplyResp struct {
	Supply []struct {
		Denom  string `json:"denom"`
		Amount string `json:"amount"`
	} `json:"supply"`
}

// TestEconomicFlow runs all economic flow E2E tests on a shared 2-validator chain.
// Subtests run sequentially — state carries over, simulating a real chain lifecycle.
func TestEconomicFlow(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping E2E test in short mode")
	}

	chain, ctx := SetupChain(t, 2)

	// ── Phase 1: Wait for reward accumulation ──
	WaitBlocks(t, chain, ctx, 10)

	t.Run("BlockRewardDistribution", func(t *testing.T) {
		height, err := chain.Height(ctx)
		require.NoError(t, err)
		require.Greater(t, height, int64(5), "chain should have produced blocks")

		// Query a recent block's reward distribution
		// Try a few recent blocks — some may be empty at startup
		var dist blockRewardDist
		var found bool
		for h := height - 1; h >= height-5 && h > 2; h-- {
			out := QueryModule(t, chain, ctx, "vesting_rewards", "block-reward", fmt.Sprintf("%d", h))

			var resp struct {
				Distribution blockRewardDist `json:"distribution"`
				Found        bool            `json:"found"`
			}
			require.NoError(t, json.Unmarshal(out, &resp))

			if resp.Found {
				dist = resp.Distribution
				found = true
				break
			}
		}
		require.True(t, found, "should find at least one block with reward distribution")

		totalMinted := mustBigInt(t, dist.TotalMinted)
		require.True(t, totalMinted.Sign() > 0, "total_minted should be positive")

		// Block reward should be 10 ZRN = 10,000,000 uzrn (epoch 0, no decay)
		expectedReward := big.NewInt(10_000_000)
		assert.Equal(t, expectedReward.String(), totalMinted.String(),
			"block reward at epoch 0 should be 10 ZRN (10000000 uzrn)")
	})

	t.Run("RevenueSplit", func(t *testing.T) {
		height, err := chain.Height(ctx)
		require.NoError(t, err)

		// Find a block with a distribution
		var dist blockRewardDist
		for h := height - 1; h >= height-5 && h > 2; h-- {
			out := QueryModule(t, chain, ctx, "vesting_rewards", "block-reward", fmt.Sprintf("%d", h))
			var resp struct {
				Distribution blockRewardDist `json:"distribution"`
				Found        bool            `json:"found"`
			}
			require.NoError(t, json.Unmarshal(out, &resp))
			if resp.Found && mustBigInt(t, resp.Distribution.TotalMinted).Sign() > 0 {
				dist = resp.Distribution
				break
			}
		}

		totalMinted := mustBigInt(t, dist.TotalMinted)
		require.True(t, totalMinted.Sign() > 0, "need a non-zero reward block for revenue split test")

		bps := big.NewInt(1_000_000)

		// Contributor: 55% (550,000 bps)
		expectedContributor := new(big.Int).Mul(totalMinted, big.NewInt(550_000))
		expectedContributor.Div(expectedContributor, bps)
		assert.Equal(t, expectedContributor.String(), mustBigInt(t, dist.ProducerReward).String(),
			"contributor share should be 55%%")

		// Protocol: 22% (220,000 bps)
		expectedProtocol := new(big.Int).Mul(totalMinted, big.NewInt(220_000))
		expectedProtocol.Div(expectedProtocol, bps)
		assert.Equal(t, expectedProtocol.String(), mustBigInt(t, dist.ProtocolShare).String(),
			"protocol share should be 22%%")

		// Research: 3.33% (33,300 bps)
		// Note: founder share is deducted from research if active.
		// With empty founder address, all research goes to research_fund.
		expectedResearch := new(big.Int).Mul(totalMinted, big.NewInt(33_300))
		expectedResearch.Div(expectedResearch, bps)
		actualResearch := mustBigInt(t, dist.ResearchShare)
		founderShare := mustBigInt(t, dist.FounderShare)
		grossResearch := new(big.Int).Add(actualResearch, founderShare)
		assert.Equal(t, expectedResearch.String(), grossResearch.String(),
			"gross research (research + founder) should be 3.33%%")

		// Development: remainder (19.67% = 196,700 bps)
		expectedDev := new(big.Int).Set(totalMinted)
		expectedDev.Sub(expectedDev, expectedContributor)
		expectedDev.Sub(expectedDev, expectedProtocol)
		expectedDev.Sub(expectedDev, expectedResearch)
		assert.Equal(t, expectedDev.String(), mustBigInt(t, dist.DevelopmentAmount).String(),
			"development amount should be the remainder (~19.67%%)")

		// Cross-check: all shares sum to total
		sum := new(big.Int)
		sum.Add(sum, mustBigInt(t, dist.ProducerReward))
		sum.Add(sum, mustBigInt(t, dist.ProtocolShare))
		sum.Add(sum, grossResearch)
		sum.Add(sum, mustBigInt(t, dist.DevelopmentAmount))
		assert.Equal(t, totalMinted.String(), sum.String(),
			"all revenue shares must sum to total minted")
	})
}

// mustBigInt parses a string as big.Int, defaulting to 0 on empty/failure.
func mustBigInt(t *testing.T, s string) *big.Int {
	t.Helper()
	v := new(big.Int)
	if s == "" {
		return v
	}
	if _, ok := v.SetString(s, 10); !ok {
		t.Fatalf("failed to parse big.Int from %q", s)
	}
	return v
}
```

**Step 2: Verify compilation**

Run: `cd /Users/yournameisai/Desktop/zerone && go build ./tests/e2e/...`
Expected: No errors

**Step 3: Commit**

```bash
git add tests/e2e/economic_flow_test.go
git commit -m "test(e2e): add block reward distribution and revenue split tests (R32-5)"
```

---

### Task 3: Add research fund accumulation subtest

**Files:**
- Modify: `tests/e2e/economic_flow_test.go`

**Step 1: Add ResearchFundAccumulation subtest**

Insert before the closing `}` of `TestEconomicFlow`:

```go
	t.Run("ResearchFundAccumulation", func(t *testing.T) {
		// Query research fund balance after ~10 blocks of rewards
		out := QueryModule(t, chain, ctx, "vesting_rewards", "research-fund-balance")
		var resp researchFundResp
		require.NoError(t, json.Unmarshal(out, &resp))

		balance := mustBigInt(t, resp.Balance)
		assert.Equal(t, "uzrn", resp.Denom, "research fund denom should be uzrn")
		assert.True(t, balance.Sign() > 0,
			"research fund should have accumulated balance after blocks")

		// With 10 ZRN block reward and 3.33% research share:
		// Per block research = 10,000,000 * 33,300 / 1,000,000 = 333 uzrn
		// After 10+ blocks, expect >= 333 * 5 = 1,665 uzrn (conservatively)
		minExpected := big.NewInt(1_665)
		assert.True(t, balance.Cmp(minExpected) >= 0,
			"research fund balance %s should be >= %s after several blocks", balance, minExpected)
	})
```

**Step 2: Verify compilation**

Run: `cd /Users/yournameisai/Desktop/zerone && go build ./tests/e2e/...`
Expected: No errors

**Step 3: Commit**

```bash
git add tests/e2e/economic_flow_test.go
git commit -m "test(e2e): add research fund accumulation test (R32-5)"
```

---

### Task 4: Add supply growth tracking subtest

**Files:**
- Modify: `tests/e2e/economic_flow_test.go`

**Step 1: Add SupplyGrowth subtest**

Insert before the closing `}` of `TestEconomicFlow`:

```go
	t.Run("SupplyGrowth", func(t *testing.T) {
		// Record supply before waiting
		supplyBefore := queryTotalSupply(t, chain, ctx)
		heightBefore, err := chain.Height(ctx)
		require.NoError(t, err)

		// Wait 20 more blocks
		WaitBlocks(t, chain, ctx, 20)

		supplyAfter := queryTotalSupply(t, chain, ctx)
		heightAfter, err := chain.Height(ctx)
		require.NoError(t, err)

		blocksElapsed := heightAfter - heightBefore
		require.Greater(t, blocksElapsed, int64(15), "should have advanced at least 15 blocks")

		// Supply should have increased
		growth := new(big.Int).Sub(supplyAfter, supplyBefore)
		assert.True(t, growth.Sign() > 0, "total supply should increase over time")

		// Expected growth: ~10 ZRN per block (10,000,000 uzrn)
		// With 2 validators and min=2, full reward per block.
		// Allow 50% tolerance for timing variance.
		expectedGrowth := new(big.Int).Mul(big.NewInt(10_000_000), big.NewInt(blocksElapsed))
		halfExpected := new(big.Int).Div(expectedGrowth, big.NewInt(2))
		assert.True(t, growth.Cmp(halfExpected) >= 0,
			"supply growth %s should be >= 50%% of expected %s over %d blocks",
			growth, expectedGrowth, blocksElapsed)

		t.Logf("Supply grew by %s uzrn over %d blocks (expected ~%s)",
			growth, blocksElapsed, expectedGrowth)
	})
```

**Step 2: Add `queryTotalSupply` helper at end of file**

```go
// queryTotalSupply returns the total ZRN supply as big.Int (in uzrn).
func queryTotalSupply(t *testing.T, chain *cosmos.CosmosChain, ctx context.Context) *big.Int {
	t.Helper()

	stdout, _, err := chain.GetNode().ExecQuery(ctx, "bank", "total-supply", "--output", "json")
	require.NoError(t, err)

	var resp supplyResp
	require.NoError(t, json.Unmarshal(stdout, &resp))

	for _, coin := range resp.Supply {
		if coin.Denom == "uzrn" {
			return mustBigInt(t, coin.Amount)
		}
	}

	t.Fatal("uzrn not found in total supply")
	return nil
}
```

**Step 3: Add missing import for `cosmos` and `context`**

Ensure these imports are present at the top of the file:

```go
import (
	"context"
	"encoding/json"
	"fmt"
	"math/big"
	"testing"

	sdkmath "cosmossdk.io/math"
	"github.com/strangelove-ventures/interchaintest/v8"
	"github.com/strangelove-ventures/interchaintest/v8/chain/cosmos"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)
```

**Step 4: Verify compilation**

Run: `cd /Users/yournameisai/Desktop/zerone && go build ./tests/e2e/...`
Expected: No errors

**Step 5: Commit**

```bash
git add tests/e2e/economic_flow_test.go
git commit -m "test(e2e): add supply growth tracking test (R32-5)"
```

---

### Task 5: Add fee distribution subtest

**Files:**
- Modify: `tests/e2e/economic_flow_test.go`

**Step 1: Add FeeDistribution subtest**

Insert before the closing `}` of `TestEconomicFlow`:

```go
	t.Run("FeeDistribution", func(t *testing.T) {
		// Create and fund a test user to generate fee-bearing transactions
		users := interchaintest.GetAndFundTestUsers(t, ctx, "fee-test", sdkmath.NewInt(100_000_000), chain)
		require.Len(t, users, 1)
		sender := users[0]

		// Record research fund balance before fees
		outBefore := QueryModule(t, chain, ctx, "vesting_rewards", "research-fund-balance")
		var respBefore researchFundResp
		require.NoError(t, json.Unmarshal(outBefore, &respBefore))
		researchBefore := mustBigInt(t, respBefore.Balance)

		// Submit several fee-bearing bank transfers
		recipient := "zrn1qqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqnrql8a" // burn address
		for i := 0; i < 3; i++ {
			ExecTx(t, chain, ctx, sender.KeyName(),
				"bank", "send", sender.FormattedAddress(), recipient,
				"1000uzrn",
				"--fees", "50000uzrn",
			)
		}

		// Wait for fee routing to process
		WaitBlocks(t, chain, ctx, 3)

		// Research fund should have grown (3.33% of fees routed there)
		outAfter := QueryModule(t, chain, ctx, "vesting_rewards", "research-fund-balance")
		var respAfter researchFundResp
		require.NoError(t, json.Unmarshal(outAfter, &respAfter))
		researchAfter := mustBigInt(t, respAfter.Balance)

		assert.True(t, researchAfter.Cmp(researchBefore) > 0,
			"research fund should grow from fee distribution: before=%s after=%s",
			researchBefore, researchAfter)
	})
```

**Step 2: Verify compilation**

Run: `cd /Users/yournameisai/Desktop/zerone && go build ./tests/e2e/...`
Expected: No errors

**Step 3: Commit**

```bash
git add tests/e2e/economic_flow_test.go
git commit -m "test(e2e): add fee distribution test (R32-5)"
```

---

### Task 6: Add staking reward flow subtest

**Files:**
- Modify: `tests/e2e/economic_flow_test.go`

**Step 1: Add StakingRewardFlow subtest**

Insert before the closing `}` of `TestEconomicFlow`:

```go
	t.Run("StakingRewardFlow", func(t *testing.T) {
		// Fund a delegator
		users := interchaintest.GetAndFundTestUsers(t, ctx, "staking-test", sdkmath.NewInt(500_000_000), chain)
		require.Len(t, users, 1)
		delegator := users[0]

		// Get validator address for delegation
		vals, err := chain.StakingQueryValidators(ctx, "BOND_STATUS_BONDED")
		require.NoError(t, err)
		require.NotEmpty(t, vals, "should have bonded validators")
		valAddr := vals[0].OperatorAddress

		balanceBefore, err := chain.GetBalance(ctx, delegator.FormattedAddress(), "uzrn")
		require.NoError(t, err)

		// Delegate tokens
		ExecTx(t, chain, ctx, delegator.KeyName(),
			"staking", "delegate", valAddr, "100000000uzrn",
			"--fees", "5000uzrn",
		)

		// Wait for reward accumulation (10+ blocks)
		WaitBlocks(t, chain, ctx, 15)

		// Query delegation rewards
		rewardsOut, _, err := chain.GetNode().ExecQuery(ctx,
			"distribution", "rewards", delegator.FormattedAddress(), valAddr,
			"--output", "json",
		)
		require.NoError(t, err)

		var rewardsResp struct {
			Rewards []struct {
				Denom  string `json:"denom"`
				Amount string `json:"amount"`
			} `json:"rewards"`
		}
		require.NoError(t, json.Unmarshal(rewardsOut, &rewardsResp))

		// There should be some rewards accumulated
		hasRewards := false
		for _, r := range rewardsResp.Rewards {
			if r.Denom == "uzrn" {
				amt := mustBigInt(t, r.Amount)
				if amt.Sign() > 0 {
					hasRewards = true
					t.Logf("Delegation rewards accumulated: %s uzrn", amt)
				}
			}
		}
		assert.True(t, hasRewards, "should have accumulated staking rewards after 15 blocks")

		// Withdraw rewards
		ExecTx(t, chain, ctx, delegator.KeyName(),
			"distribution", "withdraw-rewards", valAddr,
			"--fees", "5000uzrn",
		)

		// Balance after withdrawal should reflect claimed rewards
		// (minus delegation amount and fees, plus rewards)
		balanceAfter, err := chain.GetBalance(ctx, delegator.FormattedAddress(), "uzrn")
		require.NoError(t, err)
		t.Logf("Delegator balance: before=%s, after_withdraw=%s", balanceBefore, balanceAfter)

		// The balance won't be higher than before (we delegated 100M uzrn + fees),
		// but it should be higher than (initial - delegation - fees) due to rewards.
		expectedMinBalance := balanceBefore.Sub(sdkmath.NewInt(100_010_000)) // delegation + fees
		assert.True(t, balanceAfter.GT(expectedMinBalance),
			"balance after withdrawal should exceed (initial - delegation - fees)")
	})
```

**Step 2: Verify compilation**

Run: `cd /Users/yournameisai/Desktop/zerone && go build ./tests/e2e/...`
Expected: No errors

**Step 3: Commit**

```bash
git add tests/e2e/economic_flow_test.go
git commit -m "test(e2e): add staking reward flow test (R32-5)"
```

---

### Task 7: Add zero-supply genesis verification subtest

**Files:**
- Modify: `tests/e2e/economic_flow_test.go`

**Step 1: Add ZeroSupplyGenesis subtest**

This subtest verifies that the chain started from zero ZRN supply (pure PoT) and minted tokens. Insert it as the **first** subtest in `TestEconomicFlow` (move above BlockRewardDistribution), since it validates the foundational economic invariant:

```go
	t.Run("ZeroSupplyGenesis_PoTMinting", func(t *testing.T) {
		// Verify params confirm pure PoT (initial_fund_balance = 0)
		out := QueryModule(t, chain, ctx, "vesting_rewards", "params")
		var pResp paramsResp
		require.NoError(t, json.Unmarshal(out, &pResp))
		assert.Equal(t, "10000000", pResp.Params.BlockReward,
			"block reward should be 10 ZRN (10000000 uzrn)")

		// Total supply should be positive (PoT has minted tokens)
		supply := queryTotalSupply(t, chain, ctx)
		assert.True(t, supply.Sign() > 0,
			"total supply should be positive — PoT should have minted tokens")

		height, err := chain.Height(ctx)
		require.NoError(t, err)

		// Supply should be roughly proportional to blocks produced
		// 10 ZRN per block, but first 1-2 blocks may not mint (genesis skip)
		maxExpectedSupply := new(big.Int).Mul(big.NewInt(10_000_000), big.NewInt(height))
		assert.True(t, supply.Cmp(maxExpectedSupply) <= 0,
			"supply %s should not exceed max possible %s at height %d",
			supply, maxExpectedSupply, height)

		t.Logf("At height %d: total supply = %s uzrn (max possible = %s)",
			height, supply, maxExpectedSupply)
	})
```

**Step 2: Verify compilation**

Run: `cd /Users/yournameisai/Desktop/zerone && go build ./tests/e2e/...`
Expected: No errors

**Step 3: Commit**

```bash
git add tests/e2e/economic_flow_test.go
git commit -m "test(e2e): add zero-supply genesis PoT minting verification (R32-5)"
```

---

### Task 8: Run full E2E test and fix any issues

**Step 1: Build Docker image**

Run: `cd /Users/yournameisai/Desktop/zerone && make docker-build-local`
Expected: Successful build of `zerone:local` Docker image

**Step 2: Run economic flow E2E test**

Run: `cd /Users/yournameisai/Desktop/zerone && go test -v -timeout 20m -run TestEconomicFlow ./tests/e2e/...`
Expected: All subtests PASS

**Step 3: Fix any failures**

If any subtest fails:
1. Read the failure output carefully
2. Identify root cause (JSON field name mismatch, balance calculation, timing issue)
3. Fix the specific assertion or query
4. Re-run only the failing subtest to verify

**Step 4: Run full E2E suite to verify no regressions**

Run: `cd /Users/yournameisai/Desktop/zerone && go test -v -timeout 20m ./tests/e2e/...`
Expected: `TestSmoke_ChainStarts` and `TestEconomicFlow` both PASS

**Step 5: Commit any fixes**

```bash
git add tests/e2e/economic_flow_test.go
git commit -m "test(e2e): fix economic flow E2E test issues (R32-5)"
```

---

### Task 9: Final commit with dress-rehearsal log update

**Step 1: Update dress-rehearsal.log**

Append a line to `dress-rehearsal.log`:
```
R32-5  economic-e2e   DONE   2026-03-01   E2E: block rewards, revenue split, research fund, fees, staking, supply
```

**Step 2: Final commit**

```bash
git add dress-rehearsal.log
git commit -m "prompts: mark R32-5 economic flow E2E complete"
```

---

## Notes for Implementer

### Key Economic Constants (for assertion reference)
| Parameter | Value | Notes |
|-----------|-------|-------|
| Block reward | 10,000,000 uzrn (10 ZRN) | Epoch 0, no decay |
| Contributor share | 55% (550,000 bps) | Block producer |
| Protocol share | 22% (220,000 bps) | Citation + verification + treasury |
| Research share | 3.33% (33,300 bps) | Research fund + founder |
| Development share | 19.67% (remainder) | Development fund |
| Founder share | 7% of research (70,000 bps) | Only active if founder address set |
| Max supply | 222,222,222 ZRN | Hard cap |
| Floor reward | 100,000 uzrn (0.1 ZRN) | Minimum per block |

### JSON Field Names
Proto uses snake_case (`total_minted`, `producer_reward`). The Go JSON marshaler for proto3 messages uses camelCase by default, but `ExecQuery` returns `--output json` which uses snake_case. Verify actual field names against chain output if tests fail on unmarshal.

### Timing Sensitivity
E2E tests are sensitive to block timing. The interchaintest framework handles Docker orchestration, but block production rate varies. Use generous tolerances (50%+) for supply/balance assertions. Use `WaitBlocks` rather than `time.Sleep`.

### Unused Import Cleanup
The `sdkmath` and `interchaintest` imports are used in staking/fee subtests. If building only the initial tasks, temporary compilation may require `_` imports. Clean up before final commit.
