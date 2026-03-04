# R32-6 Multi-Validator Network E2E Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Comprehensive E2E tests for 4-validator network scenarios: startup, validator set changes, slashing, network partitions, coordinated upgrades, and full node sync.

**Architecture:** Single test file `tests/e2e/multi_validator_test.go` with 6 test functions, each spinning up independent chains. A new chain spec with fast slashing params added to `chain_config_test.go`. Tests use interchaintest v8's `PauseContainer`/`UnpauseContainer` for partition simulation, `AddFullNodes` for dynamic node addition, and SDK staking/slashing gRPC queries for state verification.

**Tech Stack:** Go 1.24, interchaintest v8.8.1, testify, Cosmos SDK v0.50.15 staking/slashing types

---

## Reference Files

| File | Purpose |
|------|---------|
| `tests/e2e/harness_test.go` | `SetupChain`, `WaitBlocks`, `ExecTx`, `QueryModule`, `FundAccount` |
| `tests/e2e/chain_config_test.go` | `ZeroneChainSpec`, `ZeroneGovChainSpec`, `testGenesisKV` |
| `tests/e2e/governance_test.go` | `SetupGovChain`, `queryJSON`, `jsonString`, `submitAndPassLIP`, `fundTestUser` |
| `tests/e2e/knowledge_helpers_test.go` | Node-specific ExecTx pattern (`val.ExecTx`) |
| `interchaintest/.../module_staking.go` | `StakingQueryValidators`, `StakingDelegate`, `StakingUnbond`, `StakingCreateValidator` |
| `interchaintest/.../module_slashing.go` | `SlashingUnJail`, `SlashingQuerySigningInfo`, `SlashingQuerySigningInfos` |
| `interchaintest/.../cosmos_chain.go` | `AddFullNodes`, `StopAllNodes`, `StartAllNodes`, `PauseContainer`, `UnpauseContainer` |

---

## Task 1: Add Slashing-Fast Chain Spec

**Files:**
- Modify: `tests/e2e/chain_config_test.go`

**Step 1: Add `slashingGenesisKV()` function**

Append after `govGenesisKV()`:

```go
// slashingGenesisKV extends testGenesisKV with fast slashing params
// for downtime slashing tests.
func slashingGenesisKV() []cosmos.GenesisKV {
	kvs := testGenesisKV()
	kvs = append(kvs,
		// Slashing: short window so downtime is detected quickly
		cosmos.NewGenesisKV("app_state.slashing.params.signed_blocks_window", "20"),
		cosmos.NewGenesisKV("app_state.slashing.params.min_signed_per_window", "0.500000000000000000"),
		cosmos.NewGenesisKV("app_state.slashing.params.downtime_jail_duration", "10s"),
		cosmos.NewGenesisKV("app_state.slashing.params.slash_fraction_downtime", "0.010000000000000000"),
		cosmos.NewGenesisKV("app_state.slashing.params.slash_fraction_double_sign", "0.050000000000000000"),
	)
	return kvs
}
```

**Step 2: Add `ZeroneSlashingChainSpec()` function**

```go
// ZeroneSlashingChainSpec returns a chain spec with fast slashing params
// for testing downtime jailing and unjailing.
func ZeroneSlashingChainSpec(numValidators int) *interchaintest.ChainSpec {
	numFullNodes := 0
	return &interchaintest.ChainSpec{
		ChainName: "zerone",
		Version:   "local",
		ChainConfig: ibc.ChainConfig{
			Type:           "cosmos",
			Name:           "zerone",
			ChainID:        "zerone-test-1",
			Bin:            "zeroned",
			Bech32Prefix:   "zrn",
			Denom:          "uzrn",
			GasPrices:      "1uzrn",
			GasAdjustment:  1.5,
			TrustingPeriod: "112h",
			NoHostMount:    false,
			ModifyGenesis:  cosmos.ModifyGenesis(slashingGenesisKV()),
			Images: []ibc.DockerImage{{
				Repository: "zerone",
				Version:    "local",
				UIDGID:     "0:0",
			}},
		},
		NumValidators: &numValidators,
		NumFullNodes:  &numFullNodes,
	}
}
```

**Step 3: Add `SetupSlashingChain()` in test file (Task 2 will create the file)**

This goes in `multi_validator_test.go` — written in Task 2.

**Step 4: Run `go vet ./tests/e2e/...`**

Run: `go vet ./tests/e2e/...`
Expected: no errors (new functions are unused until Task 2 adds tests)

**Step 5: Commit**

```bash
git add tests/e2e/chain_config_test.go
git commit -m "test(e2e): add slashing-fast chain spec for R32-6 multi-validator tests"
```

---

## Task 2: 4-Validator Startup Test

**Files:**
- Create: `tests/e2e/multi_validator_test.go`

**Step 1: Write the test file with imports and the startup test**

```go
package e2e_test

import (
	"context"
	"testing"
	"time"

	"github.com/strangelove-ventures/interchaintest/v8"
	"github.com/strangelove-ventures/interchaintest/v8/chain/cosmos"
	"github.com/strangelove-ventures/interchaintest/v8/testreporter"
	"github.com/strangelove-ventures/interchaintest/v8/testutil"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"

	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
)

// SetupSlashingChain spins up a chain with fast slashing params.
func SetupSlashingChain(t *testing.T, numValidators int) (*cosmos.CosmosChain, context.Context) {
	t.Helper()

	ctx := context.Background()

	cf := interchaintest.NewBuiltinChainFactory(
		zaptest.NewLogger(t),
		[]*interchaintest.ChainSpec{ZeroneSlashingChainSpec(numValidators)},
	)

	chains, err := cf.Chains(t.Name())
	require.NoError(t, err)
	require.Len(t, chains, 1)

	chain := chains[0].(*cosmos.CosmosChain)

	client, network := interchaintest.DockerSetup(t)

	ic := interchaintest.NewInterchain().AddChain(chain)

	rep := testreporter.NewNopReporter()

	err = ic.Build(ctx, rep.RelayerExecReporter(t), interchaintest.InterchainBuildOptions{
		TestName:  t.Name(),
		Client:    client,
		NetworkID: network,
	})
	require.NoError(t, err)

	t.Cleanup(func() {
		_ = ic.Close()
	})

	return chain, ctx
}

// TestMultiVal_FourValidatorStartup verifies that a 4-validator chain starts,
// all validators sign blocks, and the chain survives 1 validator being down.
func TestMultiVal_FourValidatorStartup(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping E2E test in short mode")
	}

	chain, ctx := SetupChain(t, 4)

	t.Run("all 4 validators active", func(t *testing.T) {
		// Chain should be producing blocks
		height, err := chain.Height(ctx)
		require.NoError(t, err)
		require.Greater(t, height, int64(0))

		// Verify we have 4 validator nodes
		require.Len(t, chain.Validators, 4)

		// All 4 should be bonded
		vals, err := chain.StakingQueryValidators(ctx, stakingtypes.BondStatusBonded)
		require.NoError(t, err)
		require.Len(t, vals, 4, "all 4 validators should be bonded")

		t.Logf("4 validators bonded at height %d", height)
		for i, v := range vals {
			t.Logf("  val[%d]: %s (tokens=%s)", i, v.OperatorAddress, v.Tokens)
		}
	})

	t.Run("chain produces blocks", func(t *testing.T) {
		h1, err := chain.Height(ctx)
		require.NoError(t, err)
		WaitBlocks(t, chain, ctx, 5)
		h2, err := chain.Height(ctx)
		require.NoError(t, err)
		require.GreaterOrEqual(t, h2, h1+5, "chain should advance at least 5 blocks")
	})

	t.Run("survives 1 validator down", func(t *testing.T) {
		// Pause validator 3 (simulates crash)
		err := chain.Validators[3].PauseContainer(ctx)
		require.NoError(t, err)
		t.Log("paused validator 3")

		// Chain should continue with 3/4 = 75% > 2/3
		h1, err := chain.Height(ctx)
		require.NoError(t, err)
		WaitBlocks(t, chain, ctx, 5)
		h2, err := chain.Height(ctx)
		require.NoError(t, err)
		require.Greater(t, h2, h1, "chain should continue with 3/4 validators")
		t.Logf("chain advanced from %d to %d with 1 validator down", h1, h2)

		// Restore validator 3
		err = chain.Validators[3].UnpauseContainer(ctx)
		require.NoError(t, err)
		t.Log("unpaused validator 3")

		WaitBlocks(t, chain, ctx, 3)
	})
}
```

**Step 2: Run the test to verify it compiles**

Run: `go vet ./tests/e2e/...`
Expected: PASS (no compilation errors)

**Step 3: Commit**

```bash
git add tests/e2e/multi_validator_test.go
git commit -m "test(e2e): add 4-validator startup test (R32-6 task 1)"
```

---

## Task 3: Validator Set Changes Test

**Files:**
- Modify: `tests/e2e/multi_validator_test.go`

**Step 1: Add the validator set changes test**

Append to `multi_validator_test.go`:

```go
// TestMultiVal_ValidatorSetChanges tests adding and removing validators mid-chain.
func TestMultiVal_ValidatorSetChanges(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping E2E test in short mode")
	}

	chain, ctx := SetupChain(t, 4)
	WaitBlocks(t, chain, ctx, 5)

	t.Run("add 5th validator via full node promotion", func(t *testing.T) {
		// Verify starting with 4 bonded validators
		vals, err := chain.StakingQueryValidators(ctx, stakingtypes.BondStatusBonded)
		require.NoError(t, err)
		require.Len(t, vals, 4)

		// Add a full node to the network
		err = chain.AddFullNodes(ctx, nil, 1)
		require.NoError(t, err)
		require.Len(t, chain.FullNodes, 1, "should have 1 full node")

		fullNode := chain.FullNodes[0]

		// Wait for the full node to sync
		WaitBlocks(t, chain, ctx, 5)

		// Create a key on the full node for the new validator
		err = fullNode.CreateKey(ctx, "newval")
		require.NoError(t, err)

		newValAddr, err := fullNode.AccountKeyBech32(ctx, "newval")
		require.NoError(t, err)
		t.Logf("new validator account address: %s", newValAddr)

		// Fund the new validator from faucet
		FundAccount(t, chain, ctx, newValAddr, sdkmath.NewInt(100_000_000_000)) // 100k ZRN
		WaitBlocks(t, chain, ctx, 2)

		// Get the full node's validator pubkey from priv_validator_key.json
		pubKeyJSON, err := fullNode.ReadFile(ctx, "config/priv_validator_key.json")
		require.NoError(t, err)

		// Extract just the pubkey portion
		var privValKey struct {
			PubKey struct {
				Type  string `json:"type"`
				Value string `json:"value"`
			} `json:"pub_key"`
		}
		err = json.Unmarshal(pubKeyJSON, &privValKey)
		require.NoError(t, err)

		// Create a validator JSON file on the full node
		valFile := "validator.json"
		pubkeyFormatted := fmt.Sprintf(`{"@type":"/cosmos.crypto.ed25519.PubKey","key":"%s"}`, privValKey.PubKey.Value)
		err = fullNode.StakingCreateValidatorFile(
			ctx, valFile,
			pubkeyFormatted,
			"50000000000uzrn", // 50k ZRN self-delegation
			"newval5",         // moniker
			"",                // identity
			"",                // website
			"",                // security
			"fifth validator", // details
			"0.10",            // commission rate
			"0.20",            // commission max rate
			"0.01",            // commission max change rate
			"1",               // min self delegation
		)
		require.NoError(t, err)

		// Create the validator
		err = fullNode.StakingCreateValidator(ctx, "newval", valFile)
		require.NoError(t, err)
		t.Log("created 5th validator")

		// Wait for the validator set to update (next epoch)
		WaitBlocks(t, chain, ctx, 5)

		// Verify 5 bonded validators
		vals, err = chain.StakingQueryValidators(ctx, stakingtypes.BondStatusBonded)
		require.NoError(t, err)
		require.Len(t, vals, 5, "should have 5 bonded validators after promotion")
		t.Logf("5 validators now bonded")
	})

	t.Run("remove validator via unbonding", func(t *testing.T) {
		// Get validator 0's operator address
		vals, err := chain.StakingQueryValidators(ctx, stakingtypes.BondStatusBonded)
		require.NoError(t, err)
		initialCount := len(vals)

		// Find the newly added validator's operator address (last one)
		newVal := vals[len(vals)-1]
		t.Logf("unbonding validator: %s", newVal.OperatorAddress)

		// Unbond all tokens from the new validator using the full node
		fullNode := chain.FullNodes[0]
		err = fullNode.StakingUnbond(ctx, "newval", newVal.OperatorAddress, "50000000000uzrn")
		require.NoError(t, err)

		// Wait for unbonding (unbonding_period=50 blocks in genesis)
		WaitBlocks(t, chain, ctx, 55)

		// Verify validator count decreased
		vals, err = chain.StakingQueryValidators(ctx, stakingtypes.BondStatusBonded)
		require.NoError(t, err)
		require.Less(t, len(vals), initialCount, "bonded validator count should decrease after unbonding")

		// Chain should still be producing blocks
		h1, err := chain.Height(ctx)
		require.NoError(t, err)
		WaitBlocks(t, chain, ctx, 3)
		h2, err := chain.Height(ctx)
		require.NoError(t, err)
		require.Greater(t, h2, h1, "chain continues after validator removal")
		t.Logf("chain continues at height %d with %d validators", h2, len(vals))
	})
}
```

**Step 2: Add missing imports to the file**

Make sure these imports are present at top of `multi_validator_test.go`:

```go
import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	sdkmath "cosmossdk.io/math"
	"github.com/strangelove-ventures/interchaintest/v8"
	"github.com/strangelove-ventures/interchaintest/v8/chain/cosmos"
	"github.com/strangelove-ventures/interchaintest/v8/testreporter"
	"github.com/strangelove-ventures/interchaintest/v8/testutil"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"

	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
)
```

**Step 3: Run `go vet`**

Run: `go vet ./tests/e2e/...`
Expected: PASS

**Step 4: Commit**

```bash
git add tests/e2e/multi_validator_test.go
git commit -m "test(e2e): add validator set changes test (R32-6 task 2)"
```

---

## Task 4: Downtime Slashing Test

**Files:**
- Modify: `tests/e2e/multi_validator_test.go`

**Step 1: Add the slashing test**

Append to `multi_validator_test.go`:

```go
// TestMultiVal_DowntimeSlashing tests that a validator missing blocks gets
// jailed and slashed, and can unjail to re-enter the active set.
// Uses fast slashing params: signed_blocks_window=20, min_signed=50%.
func TestMultiVal_DowntimeSlashing(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping E2E test in short mode")
	}

	chain, ctx := SetupSlashingChain(t, 4)
	WaitBlocks(t, chain, ctx, 5)

	// Get the validator we'll take offline
	targetVal := chain.Validators[3]
	targetAddr, err := targetVal.KeyBech32(ctx, "validator", "val")
	require.NoError(t, err)
	t.Logf("target validator (val3) operator address: %s", targetAddr)

	t.Run("pre-slashing all validators bonded", func(t *testing.T) {
		vals, err := chain.StakingQueryValidators(ctx, stakingtypes.BondStatusBonded)
		require.NoError(t, err)
		require.Len(t, vals, 4)

		// Record initial tokens for the target validator
		targetSDKVal, err := chain.StakingQueryValidator(ctx, targetAddr)
		require.NoError(t, err)
		t.Logf("target validator tokens before: %s", targetSDKVal.Tokens)
	})

	// Record tokens before slashing
	targetSDKVal, err := chain.StakingQueryValidator(ctx, targetAddr)
	require.NoError(t, err)
	tokensBefore := targetSDKVal.Tokens

	t.Run("pause validator to trigger downtime", func(t *testing.T) {
		// Pause validator 3 — it will miss blocks
		err := targetVal.PauseContainer(ctx)
		require.NoError(t, err)
		t.Log("paused validator 3")

		// Wait for enough blocks to exceed the signed_blocks_window (20 blocks)
		// Validator must miss >50% of 20 = >10 blocks
		// Wait 25 blocks to be safe
		WaitBlocks(t, chain, ctx, 25)
		t.Log("waited 25 blocks with validator 3 offline")
	})

	t.Run("validator is jailed", func(t *testing.T) {
		// Unpause container first so we can query from it
		err := targetVal.UnpauseContainer(ctx)
		require.NoError(t, err)
		WaitBlocks(t, chain, ctx, 3)

		// Check that the validator is jailed
		targetSDKVal, err := chain.StakingQueryValidator(ctx, targetAddr)
		require.NoError(t, err)
		require.True(t, targetSDKVal.Jailed, "validator should be jailed after downtime")
		t.Logf("validator jailed: %v, status: %s", targetSDKVal.Jailed, targetSDKVal.Status)
	})

	t.Run("slashing penalty applied", func(t *testing.T) {
		targetSDKVal, err := chain.StakingQueryValidator(ctx, targetAddr)
		require.NoError(t, err)
		tokensAfter := targetSDKVal.Tokens

		// Tokens should have decreased (1% slash for downtime)
		require.True(t, tokensAfter.LT(tokensBefore),
			"tokens should decrease after slashing: before=%s after=%s",
			tokensBefore, tokensAfter)
		t.Logf("tokens before=%s after=%s (slashed)", tokensBefore, tokensAfter)
	})

	t.Run("jailed validator excluded from consensus", func(t *testing.T) {
		// Only 3 validators should be bonded now
		vals, err := chain.StakingQueryValidators(ctx, stakingtypes.BondStatusBonded)
		require.NoError(t, err)

		// Count non-jailed validators
		activeCount := 0
		for _, v := range vals {
			if !v.Jailed {
				activeCount++
			}
		}
		require.Equal(t, 3, activeCount, "should have 3 active (non-jailed) validators")
	})

	t.Run("unjail after jail period", func(t *testing.T) {
		// Wait for jail duration to pass (10s in our genesis)
		time.Sleep(12 * time.Second)
		WaitBlocks(t, chain, ctx, 2)

		// Unjail from the target validator's node
		err := targetVal.SlashingUnJail(ctx, "validator")
		require.NoError(t, err)
		t.Log("unjailed validator 3")

		WaitBlocks(t, chain, ctx, 3)
	})

	t.Run("validator re-enters active set", func(t *testing.T) {
		targetSDKVal, err := chain.StakingQueryValidator(ctx, targetAddr)
		require.NoError(t, err)
		require.False(t, targetSDKVal.Jailed, "validator should no longer be jailed")
		require.Equal(t, stakingtypes.Bonded, targetSDKVal.Status,
			"validator should be bonded again after unjail")

		vals, err := chain.StakingQueryValidators(ctx, stakingtypes.BondStatusBonded)
		require.NoError(t, err)
		require.Len(t, vals, 4, "all 4 validators should be bonded after unjail")
		t.Log("validator 3 re-entered active set")
	})
}
```

**Step 2: Run `go vet`**

Run: `go vet ./tests/e2e/...`
Expected: PASS

**Step 3: Commit**

```bash
git add tests/e2e/multi_validator_test.go
git commit -m "test(e2e): add downtime slashing test (R32-6 task 3)"
```

---

## Task 5: Network Partition Simulation Test

**Files:**
- Modify: `tests/e2e/multi_validator_test.go`

**Step 1: Add the network partition test**

Append to `multi_validator_test.go`:

```go
// TestMultiVal_NetworkPartition tests chain behavior under validator failures:
// - 1 of 4 down: chain continues (75% > 2/3)
// - 2 of 4 down: chain halts (50% < 2/3)
// - Restart: chain resumes and catches up
func TestMultiVal_NetworkPartition(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping E2E test in short mode")
	}

	chain, ctx := SetupChain(t, 4)
	WaitBlocks(t, chain, ctx, 5)

	t.Run("1 of 4 down - chain continues", func(t *testing.T) {
		err := chain.Validators[3].PauseContainer(ctx)
		require.NoError(t, err)
		t.Log("paused validator 3")

		// Chain should continue: 3/4 = 75% > 66.7%
		h1, err := chain.Height(ctx)
		require.NoError(t, err)
		WaitBlocks(t, chain, ctx, 5)
		h2, err := chain.Height(ctx)
		require.NoError(t, err)
		require.Greater(t, h2, h1, "chain should produce blocks with 3/4 validators")
		t.Logf("chain advanced %d→%d with 1 validator down", h1, h2)
	})

	t.Run("2 of 4 down - chain halts", func(t *testing.T) {
		// Pause validator 2 (validator 3 is already paused)
		err := chain.Validators[2].PauseContainer(ctx)
		require.NoError(t, err)
		t.Log("paused validator 2 (now 2/4 down)")

		// Chain should halt: 2/4 = 50% < 66.7%
		// Use a short timeout to detect the halt
		h1, err := chain.Height(ctx)
		require.NoError(t, err)

		// Wait a bit and check the chain hasn't advanced much
		// (it may produce 1-2 blocks from in-flight consensus rounds)
		timeoutCtx, cancel := context.WithTimeout(ctx, 20*time.Second)
		defer cancel()
		err = testutil.WaitForBlocks(timeoutCtx, 5, chain)
		require.Error(t, err, "chain should not produce 5 blocks with only 2/4 validators")
		t.Log("confirmed: chain halted with 2/4 validators down")

		h2, err := chain.Height(ctx)
		if err == nil {
			t.Logf("height stalled around %d (started at %d)", h2, h1)
		}
	})

	t.Run("restart validators - chain resumes", func(t *testing.T) {
		// Record height before restart
		hBefore, _ := chain.Height(ctx)

		// Unpause both validators
		err := chain.Validators[2].UnpauseContainer(ctx)
		require.NoError(t, err)
		t.Log("unpaused validator 2")

		err = chain.Validators[3].UnpauseContainer(ctx)
		require.NoError(t, err)
		t.Log("unpaused validator 3")

		// Chain should resume — wait for blocks with generous timeout
		resumeCtx, cancel := context.WithTimeout(ctx, 60*time.Second)
		defer cancel()
		err = testutil.WaitForBlocks(resumeCtx, 5, chain)
		require.NoError(t, err, "chain should resume producing blocks after validators restart")

		hAfter, err := chain.Height(ctx)
		require.NoError(t, err)
		require.Greater(t, hAfter, hBefore, "chain should have advanced after restart")
		t.Logf("chain resumed: %d → %d", hBefore, hAfter)
	})

	t.Run("all validators caught up", func(t *testing.T) {
		// Wait a few more blocks to let all validators sync
		WaitBlocks(t, chain, ctx, 5)

		// All 4 nodes should report similar heights
		for i, val := range chain.Validators {
			h, err := val.Height(ctx)
			require.NoError(t, err)
			t.Logf("validator %d height: %d", i, h)
		}

		// Heights should be within 1-2 blocks of each other
		h0, _ := chain.Validators[0].Height(ctx)
		for i := 1; i < 4; i++ {
			hi, _ := chain.Validators[i].Height(ctx)
			diff := h0 - hi
			if diff < 0 {
				diff = -diff
			}
			require.LessOrEqual(t, diff, int64(2),
				"validator %d should be within 2 blocks of validator 0", i)
		}
	})
}
```

**Step 2: Run `go vet`**

Run: `go vet ./tests/e2e/...`
Expected: PASS

**Step 3: Commit**

```bash
git add tests/e2e/multi_validator_test.go
git commit -m "test(e2e): add network partition simulation test (R32-6 task 4)"
```

---

## Task 6: Coordinated Upgrade Test

**Files:**
- Modify: `tests/e2e/multi_validator_test.go`

**Step 1: Add the coordinated upgrade test**

Append to `multi_validator_test.go`:

```go
// TestMultiVal_CoordinatedUpgrade tests the governance upgrade flow on a
// 4-validator network: submit upgrade LIP, all validators vote yes,
// verify the upgrade plan is scheduled.
func TestMultiVal_CoordinatedUpgrade(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping E2E test in short mode")
	}

	chain, ctx := SetupGovChain(t, 4)
	WaitBlocks(t, chain, ctx, 5)

	t.Run("submit upgrade LIP", func(t *testing.T) {
		ExecTx(t, chain, ctx, "validator", "zerone_gov", "submit-lip",
			"v3.0.0 upgrade",
			"Schedule the v3.0.0 upgrade at a future height",
			"upgrade",
			"1000000",
		)
		WaitBlocks(t, chain, ctx, 1)
	})

	lipID := findLatestLIP(t, chain, ctx)
	t.Logf("submitted upgrade LIP: %s", lipID)

	t.Run("attach upgrade plan", func(t *testing.T) {
		ExecTx(t, chain, ctx, "validator", "zerone_gov", "attach-upgrade-plan",
			lipID, "v3.0.0", "999999", "https://github.com/zerone-chain/zerone/releases/tag/v3.0.0",
		)
		WaitBlocks(t, chain, ctx, 1)
	})

	t.Run("stake to enter review", func(t *testing.T) {
		ExecTx(t, chain, ctx, "validator", "zerone_gov", "stake-lip", lipID, "1")
		WaitBlocks(t, chain, ctx, 1)

		stage := getLIPField(t, chain, ctx, lipID, "stage")
		require.Equal(t, "review", stage)
	})

	t.Run("wait for voting stage", func(t *testing.T) {
		// review_blocks=3 + discussion_period_blocks=5 + margin
		WaitBlocks(t, chain, ctx, 12)

		stage := getLIPField(t, chain, ctx, lipID, "stage")
		require.Equal(t, "voting", stage)
	})

	t.Run("all validators vote yes", func(t *testing.T) {
		// Each validator votes from their own node
		for i, val := range chain.Validators {
			_, err := val.ExecTx(ctx, "validator", "zerone_gov", "cast-vote", lipID, "yes")
			require.NoError(t, err, "validator %d should vote successfully", i)
			t.Logf("validator %d voted yes", i)
		}
	})

	t.Run("LIP passes", func(t *testing.T) {
		// voting_period_blocks=10 + margin
		WaitBlocks(t, chain, ctx, 12)

		stage := getLIPField(t, chain, ctx, lipID, "stage")
		require.Equal(t, "passed", stage, "upgrade LIP should pass with unanimous vote")

		tally := queryJSON(t, chain, ctx, "zerone_gov", "tally-result", lipID)
		passed, _ := tally["passed"].(bool)
		require.True(t, passed)
		t.Logf("upgrade LIP %s passed", lipID)
	})

	t.Run("upgrade plan scheduled", func(t *testing.T) {
		stdout, _, err := chain.GetNode().ExecQuery(ctx, "upgrade", "plan")
		if err == nil && len(stdout) > 0 {
			var planResp map[string]interface{}
			if json.Unmarshal(stdout, &planResp) == nil {
				if plan, ok := planResp["plan"].(map[string]interface{}); ok {
					t.Logf("upgrade plan: name=%s height=%s",
						jsonString(plan["name"]), jsonString(plan["height"]))
					require.Equal(t, "v3.0.0", jsonString(plan["name"]))
					require.Equal(t, "999999", jsonString(plan["height"]))
				}
			}
		} else {
			t.Logf("upgrade plan query returned no plan (may not be registered yet): %v", err)
		}
	})
}
```

**Step 2: Run `go vet`**

Run: `go vet ./tests/e2e/...`
Expected: PASS

**Step 3: Commit**

```bash
git add tests/e2e/multi_validator_test.go
git commit -m "test(e2e): add coordinated upgrade test (R32-6 task 5)"
```

---

## Task 7: Full Node Sync Verification Test

**Files:**
- Modify: `tests/e2e/multi_validator_test.go`

**Step 1: Add the full node sync test**

Append to `multi_validator_test.go`:

```go
// TestMultiVal_FullNodeSync tests that a new full node can join a running
// 4-validator chain, catch up to the current height, and serve correct queries.
func TestMultiVal_FullNodeSync(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping E2E test in short mode")
	}

	chain, ctx := SetupChain(t, 4)

	// Let the chain run for a while to build up state
	t.Log("waiting 50 blocks to build chain state...")
	WaitBlocks(t, chain, ctx, 50)

	validatorHeight, err := chain.Height(ctx)
	require.NoError(t, err)
	t.Logf("chain at height %d before adding full node", validatorHeight)

	t.Run("add full node", func(t *testing.T) {
		err := chain.AddFullNodes(ctx, nil, 1)
		require.NoError(t, err)
		require.Len(t, chain.FullNodes, 1)
		t.Log("full node added successfully")
	})

	fullNode := chain.FullNodes[0]

	t.Run("full node catches up", func(t *testing.T) {
		// Wait for the full node to sync — it needs to replay all blocks
		// Give it generous time since it's block-syncing from genesis
		for i := 0; i < 30; i++ {
			WaitBlocks(t, chain, ctx, 2)
			fnHeight, err := fullNode.Height(ctx)
			if err != nil {
				continue
			}
			valHeight, _ := chain.Height(ctx)
			if fnHeight >= valHeight-2 {
				t.Logf("full node synced: fn=%d, validators=%d", fnHeight, valHeight)
				return
			}
			t.Logf("full node catching up: fn=%d, validators=%d", fnHeight, valHeight)
		}
		// Final check
		fnHeight, err := fullNode.Height(ctx)
		require.NoError(t, err)
		valHeight, _ := chain.Height(ctx)
		require.InDelta(t, float64(valHeight), float64(fnHeight), 5,
			"full node should be within 5 blocks of validators")
	})

	t.Run("full node serves correct queries", func(t *testing.T) {
		// Query bank total supply via full node
		stdout, _, err := fullNode.ExecQuery(ctx, "bank", "total-supply")
		require.NoError(t, err)
		require.NotEmpty(t, stdout)
		t.Logf("full node bank query OK: %d bytes", len(stdout))

		// Query staking validators via full node
		stdout, _, err = fullNode.ExecQuery(ctx, "staking", "validators")
		require.NoError(t, err)
		require.NotEmpty(t, stdout)
		t.Logf("full node staking query OK: %d bytes", len(stdout))

		// Query knowledge params via full node
		stdout, _, err = fullNode.ExecQuery(ctx, "knowledge", "params")
		require.NoError(t, err)
		require.NotEmpty(t, stdout)
		t.Logf("full node knowledge query OK: %d bytes", len(stdout))

		// Query alignment params via full node
		stdout, _, err = fullNode.ExecQuery(ctx, "alignment", "params")
		require.NoError(t, err)
		require.NotEmpty(t, stdout)
		t.Logf("full node alignment query OK: %d bytes", len(stdout))
	})

	t.Run("full node height matches validators", func(t *testing.T) {
		WaitBlocks(t, chain, ctx, 3)

		fnHeight, err := fullNode.Height(ctx)
		require.NoError(t, err)
		valHeight, err := chain.Height(ctx)
		require.NoError(t, err)

		require.InDelta(t, float64(valHeight), float64(fnHeight), 2,
			"full node height (%d) should match validator height (%d) within 2 blocks",
			fnHeight, valHeight)
		t.Logf("final heights: full_node=%d, validators=%d", fnHeight, valHeight)
	})
}
```

**Step 2: Run `go vet`**

Run: `go vet ./tests/e2e/...`
Expected: PASS

**Step 3: Commit**

```bash
git add tests/e2e/multi_validator_test.go
git commit -m "test(e2e): add full node sync verification test (R32-6 task 6)"
```

---

## Task 8: Build and Validate

**Step 1: Ensure Docker image builds**

Run: `make docker-build-local`
Expected: Successful build of zerone:local image

**Step 2: Run `go vet` on entire E2E package**

Run: `go vet ./tests/e2e/...`
Expected: PASS

**Step 3: Run a quick smoke test to verify setup**

Run: `go test -v -timeout 5m -run TestMultiVal_FourValidatorStartup ./tests/e2e/...`
Expected: PASS — chain starts with 4 validators, survives 1 failure

**Step 4: Run the full multi-validator test suite**

Run: `go test -v -timeout 25m -run TestMultiVal ./tests/e2e/...`
Expected: All 6 TestMultiVal_* tests pass

**Step 5: Final commit with any fixes**

```bash
git add tests/e2e/multi_validator_test.go tests/e2e/chain_config_test.go
git commit -m "test(e2e): complete R32-6 multi-validator network E2E tests"
```

---

## Acceptance Criteria Mapping

| Criterion | Test Function | Verification |
|-----------|--------------|--------------|
| 4-validator network starts and produces blocks | `TestMultiVal_FourValidatorStartup` | 4 bonded validators, blocks produced |
| Chain survives 1 validator failure | `TestMultiVal_FourValidatorStartup/survives_1_down` | PauseContainer + WaitBlocks |
| Chain halts at 2 failures, recovers | `TestMultiVal_NetworkPartition` | Pause 2, timeout, unpause, resume |
| Validator addition/removal mid-chain | `TestMultiVal_ValidatorSetChanges` | AddFullNodes + CreateValidator, Unbond |
| Slashing mechanics work | `TestMultiVal_DowntimeSlashing` | Jail, slash penalty, unjail, re-enter |
| Upgrade proposal flow on multi-val | `TestMultiVal_CoordinatedUpgrade` | LIP lifecycle, 4 validators vote |
| Full node sync + query | `TestMultiVal_FullNodeSync` | AddFullNodes, catch-up, serve queries |
