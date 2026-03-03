package e2e_test

import (
	"context"
	"testing"

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
