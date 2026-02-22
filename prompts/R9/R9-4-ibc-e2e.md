# R9-4 — IBC End-to-End Tests

## Goal

Test IBC transfers between two Zerone chains, verify rate limiting works, and handle
timeout scenarios. This proves the chain can interoperate.

## Working Directory

`/Users/yuai/Desktop/Zerone`

## Reference

- `/Users/yuai/Desktop/legible_money/` — draft had IBC E2E in B22-6
- Cosmos SDK IBC testing patterns
- `x/ibcratelimit` — rate limiting module already ported

## Deliverables

### 1. Two-Chain Test Script
Create `scripts/ibc-test.sh` that:
- Starts two single-validator Zerone chains (chain-a, chain-b) on different ports
- Sets up an IBC relayer (use `hermes` or `rly` — check what's available)
- Creates an IBC connection and transfer channel
- Runs transfer tests
- Stops everything and cleans up

### 2. IBC Transfer Tests

#### Basic Transfers
1. **Send ZRN from chain-a to chain-b** — verify IBC denom on chain-b
2. **Send back from chain-b to chain-a** — verify original denom restored
3. **Multi-hop** — if applicable, verify denom trace

#### Rate Limiting
4. **Rate limit configuration** — add rate limit for ZRN on transfer channel
5. **Transfer within limit** — small transfer succeeds
6. **Transfer exceeding limit** — large transfer (> max_send) rejected
7. **Window reset** — after window expires, transfers succeed again
8. **Receive limit** — inbound transfer exceeding max_recv rejected

#### Timeouts
9. **Timeout on unreachable chain** — stop chain-b, send from chain-a, verify timeout
10. **Timeout refund** — after timeout, sender gets tokens back on chain-a

### 3. Go Integration Tests
Create `tests/ibc/` with ibctesting framework:
```go
// Use Cosmos SDK's ibctesting package for deterministic IBC tests
import ibctesting "github.com/cosmos/ibc-go/v8/testing"
```

This avoids needing an actual relayer — the ibctesting framework simulates IBC
at the application level:
```
tests/ibc/
├── setup_test.go         (test suite setup with 2 chains)
├── transfer_test.go      (basic transfer tests)
├── ratelimit_test.go     (rate limiting tests)
└── timeout_test.go       (timeout and refund tests)
```

### 4. IBC Configuration
Each chain needs:
- Different chain IDs: `zerone-ibc-a`, `zerone-ibc-b`
- Transfer module enabled
- IBC rate limiting configured in genesis for chain-a

## Implementation Notes

- Prefer the `ibctesting` Go framework over shell scripts for reliability
- The ibctesting framework creates mock chains in-process — no actual nodes needed
- Rate limit tests need: channel ID, denom, max_send, max_recv, window_blocks
- Timeout tests: set short timeout (e.g., 10 blocks) for fast test execution

## IBC Testing Framework Pattern
```go
func (suite *IBCTestSuite) SetupTest() {
    suite.coordinator = ibctesting.NewCoordinator(suite.T(), 2)
    suite.chainA = suite.coordinator.GetChain(ibctesting.GetChainID(1))
    suite.chainB = suite.coordinator.GetChain(ibctesting.GetChainID(2))
    // Create transfer path
    suite.path = ibctesting.NewPath(suite.chainA, suite.chainB)
    suite.coordinator.Setup(suite.path)
}
```

Note: The ibctesting framework may need custom `AppCreator` functions that return a Zerone app
instead of the default SimApp. Check how the app is created in ibctesting and provide a
Zerone-specific factory.

## Verification

```bash
# Go-based IBC tests
go test ./tests/ibc/...

# Full suite still green
go test ./...
```

## Constraints

- Must use Cosmos IBC-Go v8 testing patterns (match our IBC dependency version)
- Rate limiting tests must cover both send and receive directions
- Timeout must verify token refund (no lost tokens)
- Tests must complete in <60s (use short block times / timeouts)
- If ibctesting framework integration proves too complex, fall back to mock-based
  unit tests that verify IBC middleware hooks directly
