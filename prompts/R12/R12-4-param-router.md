# R12-4 — Wire ParamRouter for Parameter-Category LIP Execution

## Context

When a parameter-category LIP passes in Zerone, `tallyAndResolve` calls `executeParamChanges` — but that function is a stub:

```go
// x/gov/keeper/abci.go
func (k Keeper) executeParamChanges(ctx sdk.Context, lip *types.LIP) {
	logger := k.Logger(ctx)
	for _, pc := range lip.ParamChanges {
		logger.Info("executing param change", "module", pc.Module, "key", pc.Key, "value", pc.Value)
		// TODO: Wire ParamRouter in app.go post-keeper-init for actual param changes.
	}
}
```

The `ParamChange` proto is already defined (module, key, value). We need a router that dispatches each change to the correct module's `MsgUpdateParams` handler.

## Design

A `ParamRouter` is an interface set on the gov keeper (post-init, like `UpgradeKeeper`). When a parameter LIP passes, the gov module calls `ParamRouter.ApplyParamChange(ctx, module, key, value)` for each change. The router in `app/` knows about all module keepers and calls the appropriate `SetParams`.

### Why not call MsgUpdateParams directly?

`MsgUpdateParams` takes a full `Params` struct, but LIP param changes are granular (single key-value). The router reads current params, patches the single field, validates, and writes back.

## Task

### 1. Define ParamRouter interface in `x/gov/types/expected_keepers.go`

```go
// ParamRouter routes parameter changes from governance LIPs to module keepers.
// Each module that supports governance-adjustable params registers itself.
type ParamRouter interface {
	// ApplyParamChange patches a single parameter on the target module.
	// Returns an error if the module is unknown, the key is invalid, or
	// the value fails validation.
	ApplyParamChange(ctx context.Context, module, key, value string) error

	// HasModule returns true if the module is registered in the router.
	HasModule(module string) bool
}
```

### 2. Add ParamRouter to gov Keeper

In `x/gov/keeper/keeper.go`:

```go
type Keeper struct {
	// ... existing fields ...
	paramRouter   types.ParamRouter // set post-init
}

func (k *Keeper) SetParamRouter(pr types.ParamRouter) {
	k.paramRouter = pr
}
```

### 3. Implement executeParamChanges properly

In `x/gov/keeper/abci.go`, replace the stub:

```go
func (k Keeper) executeParamChanges(ctx sdk.Context, lip *types.LIP) {
	if k.paramRouter == nil {
		k.Logger(ctx).Error("param router not wired, skipping param changes", "lip_id", lip.Id)
		return
	}

	logger := k.Logger(ctx)
	for _, pc := range lip.ParamChanges {
		if err := k.paramRouter.ApplyParamChange(ctx, pc.Module, pc.Key, pc.Value); err != nil {
			logger.Error("failed to apply param change",
				"lip_id", lip.Id,
				"module", pc.Module,
				"key", pc.Key,
				"error", err,
			)
			// Emit failure event but don't halt — partial execution is better than none
			ctx.EventManager().EmitEvent(
				sdk.NewEvent("zerone.gov.param_change_failed",
					sdk.NewAttribute("lip_id", lip.Id),
					sdk.NewAttribute("module", pc.Module),
					sdk.NewAttribute("key", pc.Key),
					sdk.NewAttribute("error", err.Error()),
				),
			)
			continue
		}

		logger.Info("applied param change",
			"lip_id", lip.Id,
			"module", pc.Module,
			"key", pc.Key,
		)
		ctx.EventManager().EmitEvent(
			sdk.NewEvent("zerone.gov.param_change_applied",
				sdk.NewAttribute("lip_id", lip.Id),
				sdk.NewAttribute("module", pc.Module),
				sdk.NewAttribute("key", pc.Key),
				sdk.NewAttribute("value", pc.Value),
			),
		)
	}
}
```

### 4. Create `app/param_router.go`

This is the concrete implementation that knows about all module keepers:

```go
package app

import (
	"context"
	"encoding/json"
	"fmt"

	sdk "github.com/cosmos/cosmos-sdk/types"

	govtypes "github.com/zerone-chain/zerone/x/gov/types"
)

// ParamUpdater is a function that reads current params, patches one key, validates, and writes.
type ParamUpdater func(ctx sdk.Context, key, value string) error

// ZeroneParamRouter dispatches parameter changes from governance LIPs to module keepers.
type ZeroneParamRouter struct {
	routes map[string]ParamUpdater
}

func NewZeroneParamRouter() *ZeroneParamRouter {
	return &ZeroneParamRouter{
		routes: make(map[string]ParamUpdater),
	}
}

func (r *ZeroneParamRouter) Register(module string, updater ParamUpdater) {
	r.routes[module] = updater
}

func (r *ZeroneParamRouter) ApplyParamChange(goCtx context.Context, module, key, value string) error {
	updater, ok := r.routes[module]
	if !ok {
		return fmt.Errorf("unknown module %q in param router", module)
	}
	ctx := sdk.UnwrapSDKContext(goCtx)
	return updater(ctx, key, value)
}

func (r *ZeroneParamRouter) HasModule(module string) bool {
	_, ok := r.routes[module]
	return ok
}

var _ govtypes.ParamRouter = (*ZeroneParamRouter)(nil)
```

### 5. Wire in `app/app.go`

After all keepers are created, register each module's param updater:

```go
// Wire ParamRouter for governance parameter LIPs
paramRouter := NewZeroneParamRouter()

// Register each module that has governance-adjustable params.
// Each updater reads current params, patches the key, validates, and writes.
// Example for billing:
paramRouter.Register("billing", func(ctx sdk.Context, key, value string) error {
	params := app.BillingKeeper.GetParams(ctx)
	if err := patchParam(params, key, value); err != nil {
		return err
	}
	if err := params.Validate(); err != nil {
		return err
	}
	app.BillingKeeper.SetParams(ctx, params)
	return nil
})

// ... repeat for all modules with adjustable params ...

app.LgmGovKeeper.SetParamRouter(paramRouter)
```

The `patchParam` helper uses reflection or a switch to set a single field:

```go
// patchParam sets a single field on a params struct by JSON key name.
func patchParam(params interface{}, key, value string) error {
	// Marshal to map, patch key, unmarshal back
	bz, err := json.Marshal(params)
	if err != nil {
		return err
	}
	var m map[string]json.RawMessage
	if err := json.Unmarshal(bz, &m); err != nil {
		return err
	}
	m[key] = json.RawMessage(value)
	patched, err := json.Marshal(m)
	if err != nil {
		return err
	}
	return json.Unmarshal(patched, params)
}
```

**Alternative (recommended for type safety):** Each module registers a per-key switch instead of using reflection. This is more explicit and catches unknown keys at registration time.

### 6. Validate LIP param changes at submission time

In `MsgSubmitLIP` handler (or `ValidateBasic`), if category is `"parameter"`, validate that:
- Each `ParamChange.Module` is a known module in the router
- Each `ParamChange.Key` is a valid parameter key for that module
- Each `ParamChange.Value` is valid JSON

This prevents invalid param changes from wasting governance cycles.

## Verification

```bash
go build ./...
go test ./x/gov/...
go test ./app/...
```

Write a test that:
1. Registers a mock module in the ParamRouter
2. Submits a parameter-category LIP with `param_changes: [{module: "billing", key: "base_query_fee", value: "\"5000\""}]`
3. Stakes + votes to pass it
4. Runs `BeginBlocker` past the voting deadline
5. Asserts the param change was applied (mock's `ApplyParamChange` called)
6. Asserts the event `zerone.gov.param_change_applied` was emitted

## Reference

- Zerone stub: `x/gov/keeper/abci.go:88-93` (executeParamChanges)
- ParamChange proto: `proto/zerone/gov/v1/types.proto:27-31`
- LIP proto `param_changes` field: `proto/zerone/gov/v1/types.proto:23`
