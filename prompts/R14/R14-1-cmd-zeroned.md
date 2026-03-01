# R14-1 — Create `cmd/zeroned/` Chain Binary

## Context

Zerone has 32 wired modules, 1992 passing tests, full app wiring in `app/app.go`, but no binary entry point. `make build` fails because `cmd/zeroned/` doesn't exist. This is the single blocker preventing the chain from running.

The prototype has `cmd/legbled/` with `main.go`, `cmd/root.go`, `cmd/config.go`, `cmd/genesis.go`, `cmd/amounts.go`, `cmd/quick_tx.go`, `cmd/explore.go`, `cmd/dashboard.go`. We port the essential files and add zerone-specific helpers.

## Task

### 1. Create `cmd/zeroned/main.go`

Minimal entry point:

```go
package main

import (
    "os"

    "cosmossdk.io/log"
    svrcmd "github.com/cosmos/cosmos-sdk/server/cmd"

    "github.com/zerone-chain/zerone/app"
    "github.com/zerone-chain/zerone/cmd/zeroned/cmd"
)

func main() {
    rootCmd := cmd.NewRootCmd()
    if err := svrcmd.Execute(rootCmd, "", app.DefaultNodeHome); err != nil {
        log.NewNopLogger().Error("failure when running app", "err", err)
        os.Exit(1)
    }
}
```

### 2. Create `cmd/zeroned/cmd/root.go`

Standard Cosmos SDK v0.50 root command wiring:

- `initRootCmd()` — registers `genutilcli.InitCmd`, `server.AddCommands`, `keys.Commands`, debug, config, prune, rollback, comet, snapshots, status
- `newApp()` — creates `app.NewZeroneApp()` from server context
- `appExport()` — for genesis export
- `initClientCtx()` — standard client context with zerone address prefixes
- Account address prefix: `zrn` (from `app.AccountAddressPrefix`)
- `DefaultNodeHome`: `~/.zeroned`

Reference: prototype's `cmd/legbled/cmd/root.go` (272 lines), adapted for zerone naming.

### 3. Create `cmd/zeroned/cmd/config.go`

Set zerone-specific defaults:

```go
func initAppConfig() (string, interface{}) {
    srvCfg := serverconfig.DefaultConfig()
    srvCfg.MinGasPrices = "0.025uzrn"
    srvCfg.API.Enable = true
    srvCfg.API.Swagger = true
    // Block time 2521ms
    return serverconfig.DefaultConfigTemplate, srvCfg
}
```

### 4. Create `cmd/zeroned/cmd/genesis.go`

Genesis helpers:
- `AddGenesisAccountCmd` — add accounts to genesis
- `GenTxCmd` — generate genesis transactions
- Adapted from `genutilcli` with zerone defaults

### 5. Ensure `app.DefaultNodeHome` exists

Verify `app/constants.go` or `app/app.go` exports:

```go
const (
    AccountAddressPrefix = "zrn"
    DefaultNodeHome      = "~/.zeroned"  // or use os.UserHomeDir
)
```

Check that `app.go` already has these. If not, add them.

### 6. Verify build + basic operation

```bash
make build
./build/zeroned version
./build/zeroned init test-node --chain-id zerone-testnet-1
./build/zeroned start  # should start and produce blocks (single validator)
```

## What NOT to port

- `cmd/quick_tx.go` — convenience wrapper, not needed for testnet
- `cmd/explore.go` — explorer helper, nice-to-have later
- `cmd/dashboard.go` — TUI dashboard, nice-to-have later
- `cmd/amounts.go` — denomination helper, port only if time permits

## Files to Create

```
cmd/zeroned/
├── main.go          (~30 lines)
└── cmd/
    ├── root.go      (~250 lines)
    ├── config.go    (~50 lines)
    └── genesis.go   (~120 lines)
```

## Verification

```bash
# Must all succeed:
make build
./build/zeroned version
./build/zeroned init smoke-test --chain-id zerone-smoke-1 --home /tmp/zeroned-smoke
./build/zeroned keys add validator --keyring-backend test --home /tmp/zeroned-smoke
go test ./cmd/... -count=1  # if any tests added
go vet ./cmd/...
```

## Commit Convention

```
feat(R14-1): create cmd/zeroned binary entry point
```
