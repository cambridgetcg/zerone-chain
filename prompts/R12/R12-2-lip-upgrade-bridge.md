# R12-2 — LIP→x/upgrade Bridge: MsgAttachUpgradePlan + ScheduleUpgrade

## Context

The prototype (Legible Money) had a complete governance→upgrade pipeline:

1. `MsgAttachUpgradePlan` — proposer attaches an upgrade plan (name, height, info) to an upgrade-category LIP
2. `UpgradeKeeper` interface + `LGMGovUpgradeAdapter` — bridges gov module to `cosmossdk.io/x/upgrade`
3. On LIP acceptance, `tallyAndResolve` calls `ScheduleUpgrade` → chain halts at target height → cosmovisor swaps binary

Zerone has `CategoryUpgrade` in the LIP category enum and cosmovisor configured, but the actual mechanism to attach plans, store them, and trigger `ScheduleUpgrade` is completely absent.

## Task

### 1. Add proto definitions to `proto/zerone/gov/v1/tx.proto`

Add to the `service Msg` block:
```protobuf
rpc AttachUpgradePlan(MsgAttachUpgradePlan) returns (MsgAttachUpgradePlanResponse);
```

Add message definitions:
```protobuf
message MsgAttachUpgradePlan {
  option (cosmos.msg.v1.signer) = "proposer";

  string proposer     = 1; // must be the LIP proposer
  string lip_id       = 2; // target LIP (must be upgrade category, non-terminal)
  string upgrade_name = 3; // must match a registered upgrade handler name
  int64  height       = 4; // block height to halt at
  string info         = 5; // upgrade info (release notes URL, binary hash, etc.)
}

message MsgAttachUpgradePlanResponse {}
```

Add to `proto/zerone/gov/v1/types.proto`:
```protobuf
message UpgradePlan {
  string name   = 1;
  int64  height = 2;
  string info   = 3;
}
```

Regenerate:
```bash
cd proto && buf generate
```

### 2. Add UpgradeKeeper interface to `x/gov/types/expected_keepers.go`

```go
// UpgradeKeeper defines the upgrade module interface for scheduling software upgrades.
// When an upgrade-category LIP passes with an attached plan, ScheduleUpgrade halts
// the chain at the specified height so validators can swap binaries.
type UpgradeKeeper interface {
	ScheduleUpgrade(ctx context.Context, plan UpgradePlan) error
}
```

Note: `UpgradePlan` here is the proto-generated `zerone.gov.v1.UpgradePlan`, NOT the Cosmos SDK plan type. The adapter (in app/) will convert between them.

### 3. Add UpgradeKeeper to gov Keeper

In `x/gov/keeper/keeper.go`:

```go
type Keeper struct {
	cdc           codec.Codec
	storeKey      *storetypes.KVStoreKey
	authority     string
	bankKeeper    types.BankKeeper
	stakingKeeper types.StakingKeeper
	vestingKeeper types.VestingRewardsKeeper
	upgradeKeeper types.UpgradeKeeper // ADD THIS
}
```

Add setter (post-init wiring to avoid circular deps):
```go
func (k *Keeper) SetUpgradeKeeper(uk types.UpgradeKeeper) {
	k.upgradeKeeper = uk
}

func (k Keeper) GetUpgradeKeeper() types.UpgradeKeeper {
	return k.upgradeKeeper
}
```

### 4. Add KV store key prefix for upgrade plans

In `x/gov/types/keys.go`, add:
```go
UpgradePlanKeyPrefix = []byte{0x0C} // next available after 0x0B (SybilParamsKey)
```

Add key helper:
```go
func UpgradePlanKey(lipID string) []byte {
	return append(UpgradePlanKeyPrefix, []byte(lipID)...)
}
```

### 5. Add state methods to `x/gov/keeper/state.go`

Port from prototype (but use proto marshal instead of JSON):

```go
// SetUpgradePlan stores an upgrade plan associated with a LIP.
func (k Keeper) SetUpgradePlan(ctx sdk.Context, lipID string, plan *types.UpgradePlan) {
	store := ctx.KVStore(k.storeKey)
	bz, err := k.cdc.Marshal(plan)
	if err != nil {
		panic("failed to marshal upgrade plan: " + err.Error())
	}
	store.Set(types.UpgradePlanKey(lipID), bz)
}

// GetUpgradePlan retrieves the upgrade plan for a LIP.
func (k Keeper) GetUpgradePlan(ctx sdk.Context, lipID string) (*types.UpgradePlan, bool) {
	store := ctx.KVStore(k.storeKey)
	bz := store.Get(types.UpgradePlanKey(lipID))
	if bz == nil {
		return nil, false
	}
	var plan types.UpgradePlan
	if err := k.cdc.Unmarshal(bz, &plan); err != nil {
		return nil, false
	}
	return &plan, true
}

// DeleteUpgradePlan removes the upgrade plan for a LIP.
func (k Keeper) DeleteUpgradePlan(ctx sdk.Context, lipID string) {
	store := ctx.KVStore(k.storeKey)
	store.Delete(types.UpgradePlanKey(lipID))
}
```

### 6. Implement MsgAttachUpgradePlan handler in `x/gov/keeper/msg_server.go`

```go
func (ms msgServer) AttachUpgradePlan(goCtx context.Context, msg *types.MsgAttachUpgradePlan) (*types.MsgAttachUpgradePlanResponse, error) {
	ctx := sdk.UnwrapSDKContext(goCtx)

	lip, found := ms.GetLIP(ctx, msg.LipId)
	if !found {
		return nil, types.ErrLIPNotFound
	}

	// Only the proposer can attach an upgrade plan
	if lip.Proposer != msg.Proposer {
		return nil, fmt.Errorf("only the LIP proposer can attach an upgrade plan")
	}

	// Only upgrade-category LIPs can carry upgrade plans
	if lip.Category != types.CategoryUpgrade {
		return nil, fmt.Errorf("only upgrade-category LIPs can carry upgrade plans")
	}

	// Cannot attach to terminal LIPs
	if types.IsTerminalStage(lip.Stage) {
		return nil, fmt.Errorf("cannot attach upgrade plan to terminal LIP")
	}

	// Check if an upgrade plan already exists for this LIP
	if _, exists := ms.GetUpgradePlan(ctx, msg.LipId); exists {
		return nil, fmt.Errorf("upgrade plan already attached to LIP %s", msg.LipId)
	}

	// Validate upgrade plan fields
	if msg.Height <= 0 {
		return nil, fmt.Errorf("upgrade height must be positive")
	}

	plan := &types.UpgradePlan{
		Name:   msg.UpgradeName,
		Height: msg.Height,
		Info:   msg.Info,
	}
	ms.SetUpgradePlan(ctx, msg.LipId, plan)

	ctx.EventManager().EmitEvent(
		sdk.NewEvent("zerone.gov.upgrade_plan_attached",
			sdk.NewAttribute("lip_id", msg.LipId),
			sdk.NewAttribute("upgrade_name", msg.UpgradeName),
			sdk.NewAttribute("height", fmt.Sprintf("%d", msg.Height)),
		),
	)

	return &types.MsgAttachUpgradePlanResponse{}, nil
}
```

### 7. Wire ScheduleUpgrade into `tallyAndResolve` in `x/gov/keeper/abci.go`

After setting `lip.Stage = types.StatusPassed`, add:

```go
// If this is an upgrade-category LIP with an attached plan, schedule the upgrade
if lip.Category == types.CategoryUpgrade {
	if plan, found := k.GetUpgradePlan(ctx, lip.Id); found {
		if uk := k.GetUpgradeKeeper(); uk != nil {
			if err := uk.ScheduleUpgrade(ctx, *plan); err != nil {
				k.Logger(ctx).Error("failed to schedule upgrade from LIP",
					"lip_id", lip.Id,
					"upgrade_name", plan.Name,
					"error", err,
				)
			} else {
				ctx.EventManager().EmitEvent(
					sdk.NewEvent("zerone.gov.upgrade_scheduled",
						sdk.NewAttribute("lip_id", lip.Id),
						sdk.NewAttribute("upgrade_name", plan.Name),
						sdk.NewAttribute("height", fmt.Sprintf("%d", plan.Height)),
					),
				)
				k.Logger(ctx).Info("software upgrade scheduled via LIP governance",
					"lip_id", lip.Id, "upgrade_name", plan.Name, "height", plan.Height,
				)
			}
		}
	}
}
```

### 8. Create app-level adapter: `app/gov_upgrade_adapter.go`

This bridges the gov module's `UpgradeKeeper` interface to the real Cosmos SDK upgrade keeper:

```go
package app

import (
	"context"

	upgradekeeper "cosmossdk.io/x/upgrade/keeper"
	upgradetypes "cosmossdk.io/x/upgrade/types"

	govtypes "github.com/zerone-chain/zerone/x/gov/types"
)

// GovUpgradeAdapter bridges the Zerone governance module's UpgradeKeeper
// interface to the real Cosmos SDK x/upgrade keeper.
type GovUpgradeAdapter struct {
	keeper *upgradekeeper.Keeper
}

func NewGovUpgradeAdapter(k *upgradekeeper.Keeper) *GovUpgradeAdapter {
	return &GovUpgradeAdapter{keeper: k}
}

func (a *GovUpgradeAdapter) ScheduleUpgrade(ctx context.Context, plan govtypes.UpgradePlan) error {
	return a.keeper.ScheduleUpgrade(ctx, upgradetypes.Plan{
		Name:   plan.Name,
		Height: plan.Height,
		Info:   plan.Info,
	})
}

var _ govtypes.UpgradeKeeper = (*GovUpgradeAdapter)(nil)
```

### 9. Wire in `app/app.go`

After the gov keeper is created and upgrade keeper exists, add post-init wiring:

```go
app.LgmGovKeeper.SetUpgradeKeeper(NewGovUpgradeAdapter(&app.UpgradeKeeper))
```

Find the section where `SetVestingKeeper` is called — add the upgrade keeper wiring nearby.

### 10. Register codec for new message type

In `x/gov/types/codec.go`, register `MsgAttachUpgradePlan`:

```go
registry.RegisterImplementations((*sdk.Msg)(nil),
	// ... existing registrations ...
	&MsgAttachUpgradePlan{},
)
```

### 11. Add `IsTerminalStage` helper if not present

In `x/gov/types/types.go`:
```go
func IsTerminalStage(stage string) bool {
	return stage == StatusPassed || stage == StatusFailed || stage == StatusWithdrawn
}
```

### 12. Genesis export/import for upgrade plans

In `x/gov/keeper/keeper.go` (or state.go), add iteration:
```go
func (k Keeper) IterateUpgradePlans(ctx sdk.Context, fn func(lipID string, plan *types.UpgradePlan) bool) {
	// prefix scan over UpgradePlanKeyPrefix
}
```

Include upgrade plans in `ExportGenesis` / `InitGenesis`. Add `UpgradePlans` field to the genesis proto if needed.

## Verification

```bash
go build ./...
go test ./x/gov/...
```

Write a test in `x/gov/keeper/` that:
1. Submits an upgrade-category LIP
2. Attaches an upgrade plan
3. Stakes + votes to pass it
4. Runs `BeginBlocker` past the voting deadline
5. Asserts `ScheduleUpgrade` was called on a mock upgrade keeper

## Reference

- Prototype implementation: `legible_money/x/gov/keeper/msg_server.go:533-578` (AttachUpgradePlan)
- Prototype state methods: `legible_money/x/gov/keeper/state.go:302-330`
- Prototype adapter: `legible_money/app/gov_upgrade_adapter.go`
- Prototype tally bridge: `legible_money/x/gov/module.go:195-220` (tallyAndResolveLIP → ScheduleUpgrade)
- Zerone tally: `x/gov/keeper/abci.go:74` (tallyAndResolve)
- Existing proto pattern: `proto/zerone/gov/v1/tx.proto`
