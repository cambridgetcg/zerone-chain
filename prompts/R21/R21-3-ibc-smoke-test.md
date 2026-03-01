# R21-3 — IBC Smoke Test on Localnet

## Context

Zerone has `x/ibcratelimit` and `x/icaauth` modules with isolated unit tests. But no integration test verifies that IBC actually works on a running chain — creating a channel, sending tokens, and receiving them on the other end.

For testnet launch, IBC isn't day-1 critical (ZERONE launches as a sovereign chain), but it must work before any cross-chain integration. Better to find issues now than after launch.

## Prerequisites

- R21-1 complete (localnet boots, all 8 tests pass)

## Task

### Step 1: Two-Chain Localnet

Extend `scripts/localnet.sh` (or create `scripts/ibc-test.sh`) to boot **two** single-validator chains side by side:

```bash
# Chain A: zerone-ibc-a (port 26657, 1317, 9090)
# Chain B: zerone-ibc-b (port 26757, 1417, 9190)
```

Both use the same `zeroned` binary. Different chain IDs, different home directories, different ports.

### Step 2: Hermes Relayer Setup

Use [Hermes](https://hermes.informal.systems/) (the standard Cosmos IBC relayer):

```bash
# Install hermes if not present
# Create hermes config for both chains
# Create clients, connection, and channel
```

If Hermes is too heavy for a smoke test, use `rly` (Go relayer) instead — or even a minimal inline Go test that uses ibc-go's testing framework directly.

**Alternative: In-Process IBC Test**

If setting up an external relayer is too complex for a smoke test, use `ibctesting` from ibc-go:

```go
// In tests/integration/ibc_test.go
func TestIBCTransfer(t *testing.T) {
    // Use ibctesting.NewCoordinator to create 2 chains
    // Create client, connection, channel
    // Send ICS-20 transfer from chain A → chain B
    // Verify balance on chain B
    // Send back from chain B → chain A
    // Verify balance restored
}
```

This is the preferred approach — it runs in `go test` without external dependencies.

### Step 3: ICS-20 Transfer Test

Whether using Hermes or ibctesting, the test must:

1. **Send tokens from Chain A to Chain B**
   - 1,000,000 uzrn from validator on Chain A
   - Verify: balance decreases on Chain A
   - Verify: IBC voucher balance appears on Chain B

2. **Send tokens back from Chain B to Chain A**
   - Send the IBC vouchers back
   - Verify: balance restored on Chain A
   - Verify: IBC voucher balance is zero on Chain B

3. **Verify rate limiting** (x/ibcratelimit)
   - If rate limits are configured, send a transfer that exceeds the limit
   - Verify: transfer rejected with rate limit error
   - Verify: transfer below limit succeeds

### Step 4: ICA Test (Optional)

If `x/icaauth` is wired:

1. Register an interchain account from Chain A on Chain B
2. Send an ICA tx (e.g., bank send) from Chain A controlling the account on Chain B
3. Verify the tx executed on Chain B

This is lower priority than ICS-20 — mark as optional.

### Step 5: Add to localnet-test.sh

If using the script-based approach, add:

```bash
test_ibc_transfer() {
    info "Testing IBC transfer between two chains..."
    # Start chain B
    # Setup relayer
    # Create channel
    # Send transfer
    # Verify balances
    # Cleanup chain B
}
```

If using ibctesting, add to `go test ./tests/integration/...`.

## Implementation Notes

### ibctesting Approach (Recommended)

```go
package integration

import (
    "testing"
    
    ibctesting "github.com/cosmos/ibc-go/v8/testing"
    transfertypes "github.com/cosmos/ibc-go/v8/modules/apps/transfer/types"
    sdk "github.com/cosmos/cosmos-sdk/types"
)

// TestIBCTransferEndToEnd tests a full ICS-20 transfer round trip.
func TestIBCTransferEndToEnd(t *testing.T) {
    coord := ibctesting.NewCoordinator(t, 2)
    chainA := coord.GetChain(ibctesting.GetChainID(1))
    chainB := coord.GetChain(ibctesting.GetChainID(2))

    // Create transfer channel
    path := ibctesting.NewTransferPath(chainA, chainB)
    coord.Setup(path)

    // Send from A to B
    amount := sdk.NewInt(1_000_000)
    token := sdk.NewCoin("uzrn", amount)
    
    msg := transfertypes.NewMsgTransfer(
        path.EndpointA.ChannelConfig.PortID,
        path.EndpointA.ChannelID,
        token,
        chainA.SenderAccount.GetAddress().String(),
        chainB.SenderAccount.GetAddress().String(),
        clienttypes.NewHeight(1, 110),
        0, "",
    )
    
    // Execute and relay
    res, err := chainA.SendMsgs(msg)
    require.NoError(t, err)
    
    packet, err := ibctesting.ParsePacketFromEvents(res.Events)
    require.NoError(t, err)
    
    err = path.RelayPacket(packet)
    require.NoError(t, err)
    
    // Verify balance on B
    ibcDenom := transfertypes.GetPrefixedDenom(
        path.EndpointB.ChannelConfig.PortID,
        path.EndpointB.ChannelID,
        "uzrn",
    )
    denomTrace := transfertypes.ParseDenomTrace(ibcDenom)
    
    balanceB := chainB.GetSimApp().BankKeeper.GetBalance(
        chainB.GetContext(),
        chainB.SenderAccount.GetAddress(),
        denomTrace.IBCDenom(),
    )
    require.Equal(t, amount, balanceB.Amount)
}
```

Adapt the imports and app references to Zerone's actual app structure. The key is using `ibctesting` so it's fully in-process.

### Rate Limit Verification

```go
func TestIBCRateLimitEnforced(t *testing.T) {
    // Setup as above
    // Configure rate limit on channel (if auto-configured in genesis)
    // Send transfer exceeding rate limit
    // Assert: packet rejected or reverted
}
```

## Exit Criteria

1. ICS-20 transfer completes: Chain A → Chain B (balance verified both sides)
2. ICS-20 return transfer completes: Chain B → Chain A (balance restored)
3. Rate limit test: transfer exceeding limit is rejected (if rate limits configured)
4. All IBC tests in `go test` — no external relayer dependency for CI
5. Tests added to either `tests/integration/` or `x/ibcratelimit/keeper/`

## Commit Convention

```
test(ibc): end-to-end ICS-20 transfer round trip
test(ibc): rate limit enforcement on IBC channel
```
