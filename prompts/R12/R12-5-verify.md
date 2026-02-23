# R12-5 — Verification: Build, Test, Upgrade Simulation

## Context

R12-1 through R12-4 added:
1. Migration wiring for all 30 modules
2. LIP→x/upgrade bridge (MsgAttachUpgradePlan → ScheduleUpgrade)
3. MsgUpdateParams for 5 previously missing modules
4. ParamRouter for parameter-category LIP execution

This session verifies everything works end-to-end.

## Task

### 1. Build verification

```bash
cd /Users/yuai/Desktop/Zerone

# Clean build
go build ./...

# Vet
go vet ./...

# Binary works
./build/zeroned version
```

### 2. Full test suite

```bash
go test ./... -count=1
```

Fix any failures. Common issues to watch for:
- Import cycle from the new adapter in `app/`
- Missing codec registrations for `MsgAttachUpgradePlan`
- Mock keepers in tests not implementing the new `UpgradeKeeper` interface
- Proto-generated field name mismatches

### 3. Migration wiring audit

Verify all 30 custom modules are wired:

```bash
# Should output exactly 30 lines (one per module)
grep -rn "RegisterMigration" x/*/module.go | wc -l

# List them to visually confirm
grep -rn "RegisterMigration" x/*/module.go | sed 's|x/||;s|/module.go.*||' | sort
```

Expected list (30 modules):
```
alignment, auth, autopoiesis, billing, bvm,
capture_challenge, capture_defense, channels, claiming_pot, compute_pool,
discovery, disputes, emergency, evidence_mgmt, gov,
home, ibcratelimit, icaauth, knowledge, liquiditypool,
ontology, partnerships, qualification, research, schedule,
staking, tokens, toolbox, tree, vesting_rewards
```

### 4. MsgUpdateParams audit

Verify all 30 modules have UpdateParams:

```bash
# Should find UpdateParams in all 30 module msg_servers
grep -l "UpdateParams" x/*/keeper/msg_server.go | wc -l
```

### 5. Upgrade bridge integration test

Write `x/gov/keeper/upgrade_bridge_test.go`:

```go
func TestUpgradeBridge_FullLifecycle(t *testing.T) {
	// Setup: keeper with mock staking + mock upgrade keeper
	// 1. Submit upgrade-category LIP
	// 2. Attach upgrade plan (name: "v2.0.0", height: 1000)
	// 3. Stake to meet quorum
	// 4. Vote yes to pass support threshold
	// 5. Advance block height past voting deadline
	// 6. Run BeginBlocker
	// 7. Assert LIP status is "passed"
	// 8. Assert mock upgrade keeper received ScheduleUpgrade call
	//    with name="v2.0.0", height=1000
	// 9. Assert "zerone.gov.upgrade_scheduled" event emitted
}

func TestUpgradeBridge_NonUpgradeLIP_NoSchedule(t *testing.T) {
	// Submit a parameter-category LIP
	// Attach upgrade plan → should fail (wrong category)
}

func TestUpgradeBridge_FailedLIP_NoSchedule(t *testing.T) {
	// Submit upgrade LIP with plan
	// Vote to reject it
	// Run BeginBlocker
	// Assert ScheduleUpgrade was NOT called
}
```

### 6. ParamRouter integration test

Write `x/gov/keeper/param_router_test.go`:

```go
func TestParamRouter_FullLifecycle(t *testing.T) {
	// Setup: keeper with mock param router
	// 1. Submit parameter-category LIP with param_changes
	// 2. Pass it through governance
	// 3. Run BeginBlocker
	// 4. Assert ApplyParamChange was called for each change
	// 5. Assert events emitted
}

func TestParamRouter_UnknownModule_EmitsError(t *testing.T) {
	// LIP with param_change targeting unknown module
	// Should emit "param_change_failed" event, not panic
}
```

### 7. Genesis round-trip test

Verify that upgrade plans survive export/import:

```go
func TestGenesis_UpgradePlanRoundTrip(t *testing.T) {
	// 1. Create LIP + attach upgrade plan
	// 2. ExportGenesis
	// 3. InitGenesis on fresh keeper
	// 4. Assert upgrade plan is retrievable
}
```

### 8. Upgrade simulation (if time permits)

```bash
# Init a test chain
./build/zeroned init test-node --chain-id zerone-testnet-1

# Verify it starts
./build/zeroned start --minimum-gas-prices 0uzrn &
PID=$!
sleep 5
kill $PID

# Verify cosmovisor integration
ls cosmovisor/genesis/bin/
```

## Exit Criteria Checklist

- [ ] `go build ./...` — clean, no errors
- [ ] `go vet ./...` — clean
- [ ] `go test ./...` — all pass
- [ ] 30/30 modules have `RegisterMigration` in `RegisterServices`
- [ ] 30/30 modules have `keeper/migrator.go`
- [ ] 30/30 modules have `MsgUpdateParams`
- [ ] `MsgAttachUpgradePlan` proto-defined, codec-registered, handler tested
- [ ] `tallyAndResolve` calls `ScheduleUpgrade` for passed upgrade LIPs
- [ ] `GovUpgradeAdapter` in `app/` bridges to real `x/upgrade` keeper
- [ ] `ParamRouter` wired in `app/app.go`, dispatches to module keepers
- [ ] `executeParamChanges` calls `ParamRouter.ApplyParamChange` (not just logging)
- [ ] Integration tests pass for upgrade bridge + param router + genesis round-trip
- [ ] No import cycles
- [ ] No hand-written `ProtoMessage()` stubs for new message types

## Troubleshooting

**Import cycle with app/ adapter:**
The adapter should only import `x/gov/types` (for the interface) and `cosmossdk.io/x/upgrade/keeper` (for the concrete keeper). If there's a cycle, the interface definition is in the wrong place.

**Mock upgrade keeper in tests:**
Tests in `x/gov/keeper/` can't import `app/`. Create a mock in the test file:
```go
type mockUpgradeKeeper struct {
	scheduledPlan *types.UpgradePlan
}
func (m *mockUpgradeKeeper) ScheduleUpgrade(_ context.Context, plan types.UpgradePlan) error {
	m.scheduledPlan = &plan
	return nil
}
```

**Proto regeneration issues:**
If `buf generate` fails, check that `UpgradePlan` in `types.proto` doesn't conflict with the Cosmos SDK's own `Plan` type. They're in different packages so should be fine, but verify the import paths.
