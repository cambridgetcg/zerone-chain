# R32-1 — Interchaintest Harness Scaffold

## Objective

Add `interchaintest` as a Go test dependency and build the base E2E harness that all subsequent R32 sessions use.

## Prerequisites

- Docker must be available (tests run real containers)
- `Dockerfile` already exists and builds `zeroned` — interchaintest will use it

## Tasks

### 1. Add interchaintest dependency

```bash
go get github.com/strangelove-ventures/interchaintest/v8@latest
```

### 2. Create chain configuration

Create `tests/e2e/chain_config.go`:

```go
package e2e_test

import (
    "github.com/strangelove-ventures/interchaintest/v8/chain/cosmos"
    "github.com/strangelove-ventures/interchaintest/v8/ibc"
)

func ZeroneChainSpec() *interchain.ChainSpec {
    return &interchain.ChainSpec{
        Name:    "zerone",
        Version: "local",  // uses locally-built Docker image
        ChainConfig: ibc.ChainConfig{
            Type:           "cosmos",
            Name:           "zerone",
            ChainID:        "zerone-test-1",
            Bin:            "zeroned",
            Bech32Prefix:   "lgm",  // verify current prefix
            Denom:          "uzrn",
            GasPrices:      "0.025uzrn",
            GasAdjustment:  1.5,
            TrustingPeriod: "112h",
            NoHostMount:    false,
            ModifyGenesis:  modifyGenesis,  // inject custom module defaults
            Images: []ibc.DockerImage{{
                Repository: "zerone",
                Version:    "local",
                UidGid:     "1025:1025",
            }},
        },
        NumValidators: 1,
        NumFullNodes:  0,
    }
}
```

### 3. Build Docker image helper

Add a Makefile target:

```makefile
docker-build-local:
	docker build -t zerone:local .
```

### 4. Create base harness

Create `tests/e2e/harness.go` with:

- `SetupChain(t, numValidators)` → spins up chain, returns `*cosmos.CosmosChain`
- `FundAccount(chain, ctx, addr, amount)` → sends tokens from faucet
- `WaitBlocks(chain, ctx, n)` → waits for n blocks to be produced
- `QueryModule(chain, ctx, module, query)` → generic gRPC query helper
- `ExecTx(chain, ctx, keyName, cmd...)` → broadcast tx and wait for inclusion

### 5. Create `modifyGenesis` function

This is critical — must inject default params for all 32 custom modules into genesis. Use the existing `tools/genesis-check` logic as reference for what a valid genesis looks like.

Key modules that need explicit genesis config:
- `knowledge` (DomainBaseCapacity, verification params)
- `vesting_rewards` (revenue split percentages)
- `alignment` (sensor weights, thresholds)
- `capture_defense` (detection thresholds)
- `partnerships` (formation params)
- `governance` (voting periods — use short periods for tests: 30s)
- `pacing` (multipliers)

### 6. Smoke test

Create `tests/e2e/smoke_test.go`:

```go
func TestSmoke_ChainStarts(t *testing.T) {
    if testing.Short() {
        t.Skip("skipping E2E test in short mode")
    }
    
    chain := SetupChain(t, 1)
    ctx := context.Background()
    
    // Chain produces blocks
    height, err := chain.Height(ctx)
    require.NoError(t, err)
    require.Greater(t, height, int64(0))
    
    // Can query bank balance
    balance, err := chain.GetBalance(ctx, ..., "uzrn")
    require.NoError(t, err)
    require.True(t, balance.GT(sdkmath.ZeroInt()))
}
```

### 7. CI integration

Update `.github/workflows/ci.yml` to add an E2E job:

```yaml
  e2e:
    name: "E2E Tests"
    runs-on: ubuntu-latest
    timeout-minutes: 20
    needs: [build]
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version: ${{ env.GO_VERSION }}
          cache: true
      - name: Build Docker image
        run: make docker-build-local
      - name: Run E2E tests
        run: go test -v -timeout 20m ./tests/e2e/...
```

## Acceptance Criteria

- [ ] `go test -short ./tests/e2e/...` skips all E2E tests
- [ ] `go test -v -timeout 10m -run TestSmoke ./tests/e2e/...` starts a real chain, produces blocks, queries balance
- [ ] Chain starts with all 32 modules loaded (no init panics)
- [ ] Genesis export after 10 blocks matches import (round-trip)
- [ ] CI job added but can be skipped with label (e2e tests are slow)

## Notes

- Use `testing.Short()` guard on ALL E2E tests — they require Docker
- interchaintest v8 `main` branch targets SDK v0.50 — matches our dependency
- The `UidGid` in DockerImage may need adjustment based on Dockerfile USER
- If `Dockerfile` doesn't set a non-root user, use `"0:0"`
