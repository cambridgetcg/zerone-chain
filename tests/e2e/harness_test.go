package e2e_test

import (
	"context"
	"fmt"
	"testing"

	sdkmath "cosmossdk.io/math"
	"github.com/strangelove-ventures/interchaintest/v8"
	"github.com/strangelove-ventures/interchaintest/v8/chain/cosmos"
	"github.com/strangelove-ventures/interchaintest/v8/ibc"
	"github.com/strangelove-ventures/interchaintest/v8/testreporter"
	"github.com/strangelove-ventures/interchaintest/v8/testutil"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

// SetupChain spins up a zerone chain with the given number of validators
// and returns the chain handle and a background context.
func SetupChain(t *testing.T, numValidators int) (*cosmos.CosmosChain, context.Context) {
	t.Helper()

	ctx := context.Background()

	cf := interchaintest.NewBuiltinChainFactory(
		zaptest.NewLogger(t),
		[]*interchaintest.ChainSpec{ZeroneChainSpec(numValidators)},
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

// FundAccount sends tokens from the faucet (validator key) to the given address.
func FundAccount(t *testing.T, chain *cosmos.CosmosChain, ctx context.Context, addr string, amount sdkmath.Int) {
	t.Helper()

	err := chain.SendFunds(ctx, interchaintest.FaucetAccountKeyName, ibc.WalletAmount{
		Address: addr,
		Denom:   "uzrn",
		Amount:  amount,
	})
	require.NoError(t, err)
}

// WaitBlocks waits for n blocks to be produced on the chain.
func WaitBlocks(t *testing.T, chain *cosmos.CosmosChain, ctx context.Context, n int) {
	t.Helper()

	err := testutil.WaitForBlocks(ctx, n, chain)
	require.NoError(t, err)
}

// QueryModule executes a gRPC query against a custom module via the chain's CLI.
// module is the module name (e.g. "knowledge"), query is the query command
// (e.g. "params"), and args are additional CLI arguments.
// ExecQuery already prepends "query" and appends "--output json".
func QueryModule(t *testing.T, chain *cosmos.CosmosChain, ctx context.Context, module, query string, args ...string) []byte {
	t.Helper()

	cmd := append([]string{module, query}, args...)
	stdout, _, err := chain.GetNode().ExecQuery(ctx, cmd...)
	require.NoError(t, err)

	return stdout
}

// ExecTx broadcasts a transaction using the given key and waits for inclusion.
// cmd should be the full tx subcommand, e.g. "knowledge", "submit-claim", "content", ...
func ExecTx(t *testing.T, chain *cosmos.CosmosChain, ctx context.Context, keyName string, cmd ...string) string {
	t.Helper()

	txHash, err := chain.GetNode().ExecTx(ctx, keyName, cmd...)
	require.NoError(t, err, fmt.Sprintf("ExecTx failed for cmd: %v", cmd))

	return txHash
}
