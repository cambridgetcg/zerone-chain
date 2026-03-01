# R34-2 — Upgrade Path Testing

## Objective

Prove that ZERONE can upgrade its binary at a scheduled block height without losing state, using Cosmovisor. This is critical for mainnet — every chain needs to upgrade eventually.

## Tasks

### 1. Cosmovisor upgrade test

Using interchaintest (which has built-in upgrade support):

```go
func TestUpgrade_BinarySwap(t *testing.T) {
    // Start chain with v1 binary
    chain := SetupChainWithVersion(t, "v0.1.0", 4)
    
    // Submit upgrade proposal for height H+50
    SubmitUpgradeProposal(chain, ctx, "v0.2.0", currentHeight+50)
    VoteAndPass(chain, ctx)
    
    // Wait for upgrade height — chain halts
    WaitForUpgradeHalt(chain, ctx)
    
    // Swap binary to v2
    UpgradeBinary(chain, "v0.2.0")
    
    // Chain resumes with new binary
    WaitBlocks(chain, ctx, 10)
    
    // Verify state preserved
    VerifyStateAfterUpgrade(chain, ctx)
}
```

### 2. State migration test

- Create a migration handler in `app/upgrades/`:

```go
func CreateUpgradeHandler(mm *module.Manager, configurator module.Configurator) upgradetypes.UpgradeHandler {
    return func(ctx sdk.Context, plan upgradetypes.Plan, fromVM module.VersionMap) (module.VersionMap, error) {
        // Run module migrations
        return mm.RunMigrations(ctx, configurator, fromVM)
    }
}
```

- Test adding a new module param in v2 that didn't exist in v1
- Verify migration fills in default values
- Verify no data loss for existing state

### 3. Rollback test

- Start upgrade
- Simulate failure during migration
- Verify chain can restart with old binary
- Verify no state corruption

### 4. Multi-version compatibility

- Build v1 and v2 Docker images
- Start 4-validator network on v1
- Upgrade 2 validators to v2
- Verify chain continues until upgrade height
- At upgrade height, all validators switch
- Verify chain resumes

### 5. Genesis migration CLI

- Test `zeroned genesis migrate` from v1 genesis format to v2
- Verify all modules' genesis state is correctly migrated
- Verify migrated genesis starts a chain

## Acceptance Criteria

- [ ] Cosmovisor binary swap works at scheduled height
- [ ] State migration adds new params without data loss
- [ ] Failed migration doesn't corrupt state
- [ ] Multi-validator upgrade coordinates correctly
- [ ] Genesis migration CLI produces valid genesis
- [ ] `Dockerfile.validator` with Cosmovisor works correctly
