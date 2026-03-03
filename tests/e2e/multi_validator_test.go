package e2e_test

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	sdkmath "cosmossdk.io/math"
	"github.com/strangelove-ventures/interchaintest/v8"
	"github.com/strangelove-ventures/interchaintest/v8/chain/cosmos"
	"github.com/strangelove-ventures/interchaintest/v8/testreporter"
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
		// Get bonded validators
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
