package e2e_test

import (
	"testing"

	sdkmath "cosmossdk.io/math"
	"github.com/strangelove-ventures/interchaintest/v8"
	"github.com/stretchr/testify/require"
)

func TestSmoke_ChainStarts(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping E2E test in short mode")
	}

	chain, ctx := SetupChain(t, 1)

	// Chain produces blocks
	height, err := chain.Height(ctx)
	require.NoError(t, err)
	require.Greater(t, height, int64(0), "chain should produce blocks")

	// Wait a few more blocks to confirm liveness
	WaitBlocks(t, chain, ctx, 3)

	height2, err := chain.Height(ctx)
	require.NoError(t, err)
	require.Greater(t, height2, height, "chain should continue producing blocks")

	// Fund a test user and verify balance
	users := interchaintest.GetAndFundTestUsers(t, ctx, t.Name(), sdkmath.NewInt(10_000_000), chain)
	require.Len(t, users, 1)

	balance, err := chain.GetBalance(ctx, users[0].FormattedAddress(), "uzrn")
	require.NoError(t, err)
	require.True(t, balance.GT(sdkmath.ZeroInt()), "funded user should have positive balance")
}
