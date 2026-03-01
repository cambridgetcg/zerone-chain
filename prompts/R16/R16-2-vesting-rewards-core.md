# R16-2 — vesting_rewards Module: Core Revenue Engine

## Objective

Refactor the vesting_rewards module — the central revenue router — to replace burn with development fund deposits and enforce founder share immutability.

## Prerequisites

R16-1 complete (proto regenerated, `DevelopmentBps` available in generated types).

## Changes Required

### 1. `x/vesting_rewards/types/keys.go`

Add new module account constant:
```go
// DevelopmentFundModuleName is the module account for bug bounties,
// truth discovery, and protocol development.
DevelopmentFundModuleName = "development_fund"
```

### 2. `x/vesting_rewards/types/genesis.go`

Update `DefaultRevenueSplit()`:
```go
&commontypes.RevenueSplit{
    ContributorBps:  550000,
    ProtocolBps:     220000,
    ResearchBps:     33300,
    DevelopmentBps:  196700,
}
```

Update `validateRevenueSplit()`:
- Change: `total := split.ContributorBps + split.ProtocolBps + split.ResearchBps + split.DevelopmentBps`
- Remove any reference to `BurnBps`

**Already done** (verify): `ValidateFounderShareImmutability()` function exists.

### 3. `x/vesting_rewards/keeper/rewards.go` — `DistributeRevenue()`

The 4-way split computation:
- Rename `burnAmount` variable → `developmentAmount`
- Change: development = remainder after contributor + protocol + research (preserves rounding safety)
- In `RewardRouting` output: use `DevelopmentAmount` instead of `BurnAmount`

### 4. `x/vesting_rewards/keeper/rewards.go` — `RouteFees()`

Currently burns the burn share of transaction fees. Change to:
- Instead of `bankKeeper.BurnCoins()`, send to `DevelopmentFundModuleName`
- Route: `fee_collector` → `vesting_rewards` (escrow) → `development_fund`

Remove all calls to `bankKeeper.BurnCoins()` in this function.

### 5. `x/vesting_rewards/keeper/rewards.go` — `DistributeBlockReward()`

Currently burns the burn share of block rewards. Change to:
- Instead of `bankKeeper.BurnCoins()`, send to development fund:
  ```go
  devCoins := sdk.NewCoins(sdk.NewCoin("uzrn", sdkmath.NewIntFromBigInt(developmentBig)))
  k.bankKeeper.SendCoinsFromModuleToModule(ctx, types.ModuleName, types.DevelopmentFundModuleName, devCoins)
  ```
- Update `BlockRewardDistribution` to use `DevelopmentAmount` field

### 6. `x/vesting_rewards/keeper/rewards.go` — `BurnTokens()`

Either:
- Remove the function entirely (nothing should burn)
- Or rename to `DepositToDevelopmentFund()` with send-to-module semantics

### 7. `x/vesting_rewards/keeper/keeper.go` — `isFounderShareActive()`

**Already done** (verify): governance activation height check removed.

### 8. `x/vesting_rewards/keeper/msg_server.go` — `UpdateParams()`

**Already done** (verify): calls `ValidateFounderShareImmutability()`.

### 9. `app/app.go` — Module Account Registration

Register `development_fund` as a module account with minting permissions:
```go
maccPerms[vesting_rewards_types.DevelopmentFundModuleName] = nil
```

It needs no special permissions (just receives coins via `SendCoinsFromModuleToModule`).

### 10. Add `DisburseFromDevelopmentFund()`

Mirror `DisburseFromResearchFund()`:
```go
func (k Keeper) DisburseFromDevelopmentFund(ctx sdk.Context, recipient sdk.AccAddress, amount sdk.Coins) error {
    return k.bankKeeper.SendCoinsFromModuleToAccount(ctx, types.DevelopmentFundModuleName, recipient, amount)
}
```

This will be called by governance proposals for bug bounties and development grants.

## Verification

```bash
# No BurnCoins calls in vesting_rewards (except possible error-path cleanup)
grep -rn "BurnCoins" x/vesting_rewards/ --include="*.go" | grep -v _test.go | grep -v pb.go
# Should be 0

# No BurnBps references
grep -rn "BurnBps\|burn_bps" x/vesting_rewards/ --include="*.go" | grep -v pb.go
# Should be 0

# Build
go build ./...
```

## Commit

```
R16-2: vesting_rewards — burn → development fund, founder immutability

- DistributeRevenue: development share sent to development_fund module
- RouteFees: fee burn replaced with development_fund deposit
- DistributeBlockReward: block reward burn replaced with development_fund deposit
- BurnTokens removed; DisburseFromDevelopmentFund added
- Default revenue split: 55/22/19.67/3.33 (no burn)
- development_fund module account registered in app.go
- Founder share immune: ValidateFounderShareImmutability in UpdateParams
- GovernanceActivationHeight sunset logic removed
```
