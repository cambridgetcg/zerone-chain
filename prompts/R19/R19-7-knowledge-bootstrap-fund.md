# R19-7 — Knowledge Bootstrap Fund

## Context

R19-6 introduces non-refundable review fees. This creates a cold-start problem: the knowledge base needs facts to be valuable, but submitters need to burn tokens to submit facts. Early participants — the most important ones, who seed the knowledge base from zero — bear the highest cost with the least proven return.

The Knowledge Bootstrap Fund solves this by providing a genesis-allocated pool that sponsors review fees for early claims. It's a time-limited subsidy that depletes naturally as the knowledge base grows, then the fee market takes over.

Real-world parallel: government research grants that fund initial investigations. Once the research produces results, it attracts private funding. The grant bridges the gap.

## Task

### 1. Module Account

In `x/knowledge/types/keys.go`, add:

```go
// BootstrapFundModuleName is the module account that holds the knowledge bootstrap fund.
BootstrapFundModuleName = "knowledge_bootstrap_fund"
```

Register as a module account in `app.go` (add to `maccPerms`):

```go
knowledge.BootstrapFundModuleName: {authtypes.Minter}, // needs Minter to receive genesis allocation
```

### 2. Proto: Add Bootstrap Fund Params

In `proto/zerone/knowledge/v1/genesis.proto`, add to `Params`:

```protobuf
// Bootstrap fund configuration
bool   bootstrap_fund_enabled         = <next>;  // Whether sponsored claims are accepted
string bootstrap_fund_max_per_address = <next>;  // Max sponsored claims per address (lifetime)
string bootstrap_fund_max_per_epoch   = <next>;  // Max sponsored claims per epoch (rate limit)
uint64 bootstrap_fund_epoch_blocks    = <next>;  // Epoch length in blocks for rate limiting
string bootstrap_fund_fee_cap         = <next>;  // Max fee the fund will cover per claim (uzrn)
```

### 3. Genesis Defaults

In `x/knowledge/types/genesis.go`:

```go
BootstrapFundEnabled:        true,
BootstrapFundMaxPerAddress:  "10",         // 10 sponsored claims per address lifetime
BootstrapFundMaxPerEpoch:    "100",        // 100 sponsored claims per epoch across all users
BootstrapFundEpochBlocks:    50000,        // ~1.5 days at 2.5s blocks
BootstrapFundFeeCap:         "5000000",    // Fund covers up to 5 ZRN per claim
```

### 4. Genesis Allocation

In `x/knowledge/types/genesis.go`, add to `GenesisState`:

```protobuf
string bootstrap_fund_allocation = <next>;  // Initial fund allocation (uzrn)
```

Default:

```go
BootstrapFundAllocation: "22222000000",  // 22,222 ZRN (0.01% of max supply)
```

In `x/knowledge/keeper/genesis.go`, `InitGenesis()`:

```go
// Fund the bootstrap fund from genesis allocation
if gs.BootstrapFundAllocation != "" && gs.BootstrapFundAllocation != "0" {
    alloc, ok := new(big.Int).SetString(gs.BootstrapFundAllocation, 10)
    if ok && alloc.Sign() > 0 {
        coins := sdk.NewCoins(sdk.NewCoin("uzrn", sdkmath.NewIntFromBigInt(alloc)))
        // Mint to module account
        if err := k.bankKeeper.MintCoins(ctx, types.BootstrapFundModuleName, coins); err != nil {
            panic(fmt.Errorf("failed to mint bootstrap fund: %w", err))
        }
    }
}
```

**Note:** This mint is part of genesis initialization, not ongoing inflation. It's a one-time allocation counted toward the 222,222,222 ZRN max supply.

### 5. Store: Sponsorship Tracking

In `x/knowledge/types/keys.go`:

```go
BootstrapClaimCountPrefix      = []byte{0x35}  // 0x35 | address → uint64 (lifetime count)
BootstrapEpochCountPrefix      = []byte{0x36}  // 0x36 | epoch_number → uint64 (epoch-wide count)
```

In `x/knowledge/keeper/state.go`:

```go
func (k Keeper) GetBootstrapClaimCount(ctx context.Context, address string) uint64
func (k Keeper) IncrementBootstrapClaimCount(ctx context.Context, address string) error
func (k Keeper) GetBootstrapEpochCount(ctx context.Context, epoch uint64) uint64
func (k Keeper) IncrementBootstrapEpochCount(ctx context.Context, epoch uint64) error
func (k Keeper) GetBootstrapFundBalance(ctx context.Context) sdk.Coin
func (k Keeper) CurrentEpoch(ctx context.Context) uint64  // block_height / epoch_blocks
```

### 6. Proto: Add Sponsored Flag to MsgSubmitClaim

In `proto/zerone/knowledge/v1/tx.proto`, add to `MsgSubmitClaim`:

```protobuf
bool sponsored = 12;  // Request bootstrap fund sponsorship for review fee
```

### 7. Submission Logic: Sponsored Claims

In `x/knowledge/keeper/msg_server.go`, `SubmitClaim()`, before fee collection:

```go
sponsored := msg.Sponsored
feeAmount := stakeAmt.Uint64()

if sponsored {
    params, _ := m.keeper.GetParams(ctx)

    // Check fund is enabled
    if !params.BootstrapFundEnabled {
        return nil, fmt.Errorf("bootstrap fund sponsorship is disabled")
    }

    // Check fee cap
    feeCap, _ := new(big.Int).SetString(params.BootstrapFundFeeCap, 10)
    if feeCap != nil && stakeAmt.Cmp(feeCap) > 0 {
        return nil, fmt.Errorf("review fee %s exceeds bootstrap fund cap %s", msg.Stake, params.BootstrapFundFeeCap)
    }

    // Check per-address lifetime limit
    addressCount := m.keeper.GetBootstrapClaimCount(ctx, msg.Submitter)
    maxPerAddr, _ := strconv.ParseUint(params.BootstrapFundMaxPerAddress, 10, 64)
    if addressCount >= maxPerAddr {
        return nil, fmt.Errorf("address has used all %d bootstrap fund claims", maxPerAddr)
    }

    // Check per-epoch rate limit
    epoch := m.keeper.CurrentEpoch(ctx)
    epochCount := m.keeper.GetBootstrapEpochCount(ctx, epoch)
    maxPerEpoch, _ := strconv.ParseUint(params.BootstrapFundMaxPerEpoch, 10, 64)
    if epochCount >= maxPerEpoch {
        return nil, fmt.Errorf("bootstrap fund epoch limit reached (%d/%d)", epochCount, maxPerEpoch)
    }

    // Check fund has sufficient balance
    fundBalance := m.keeper.GetBootstrapFundBalance(ctx)
    if fundBalance.Amount.LT(sdkmath.NewIntFromBigInt(stakeAmt)) {
        return nil, fmt.Errorf("bootstrap fund insufficient: has %s, need %s", fundBalance.Amount, stakeAmt)
    }

    // Pay fee from bootstrap fund instead of submitter
    feeCoins := sdk.NewCoins(sdk.NewCoin("uzrn", sdkmath.NewIntFromBigInt(stakeAmt)))
    if err := m.keeper.bankKeeper.SendCoinsFromModuleToModule(
        ctx, types.BootstrapFundModuleName, types.ModuleName, feeCoins,
    ); err != nil {
        return nil, fmt.Errorf("failed to draw from bootstrap fund: %w", err)
    }

    // Track usage
    m.keeper.IncrementBootstrapClaimCount(ctx, msg.Submitter)
    m.keeper.IncrementBootstrapEpochCount(ctx, epoch)

} else {
    // Normal path: submitter pays fee directly
    submitterAddr, _ := sdk.AccAddressFromBech32(msg.Submitter)
    feeCoins := sdk.NewCoins(sdk.NewCoin("uzrn", sdkmath.NewIntFromBigInt(stakeAmt)))
    if err := m.keeper.bankKeeper.SendCoinsFromAccountToModule(
        ctx, submitterAddr, types.ModuleName, feeCoins,
    ); err != nil {
        return nil, fmt.Errorf("failed to collect review fee: %w", err)
    }
}

// Distribute fee via revenue split (same path regardless of who paid)
if err := k.distributeReviewFee(ctx, feeAmount); err != nil {
    k.Logger(ctx).Error("failed to distribute review fee", "error", err)
}
```

### 8. Submitter Still Earns Vesting Rewards

Critical: sponsored claims still create vesting schedules for the **submitter** on acceptance. The fund paid for the review, but the submitter did the intellectual work. This is the incentive — you submit knowledge for free, and if it's valuable, you earn ongoing rewards.

No change needed in `createFactFromClaim()` — it already uses `claim.Submitter` for the vesting schedule.

### 9. Query: Fund Status

In `proto/zerone/knowledge/v1/query.proto`:

```protobuf
rpc BootstrapFundStatus(QueryBootstrapFundStatusRequest) returns (QueryBootstrapFundStatusResponse) {
    option (google.api.http).get = "/zerone/knowledge/v1/bootstrap_fund";
}

message QueryBootstrapFundStatusRequest {}

message QueryBootstrapFundStatusResponse {
    string balance              = 1;  // Current fund balance (uzrn)
    bool   enabled              = 2;
    string total_claims_funded  = 3;  // Lifetime count
    string total_amount_spent   = 4;  // Lifetime amount distributed
    string remaining_per_epoch  = 5;  // How many more claims this epoch
}
```

### 10. Events

Emit sponsorship event:

```go
sdkCtx.EventManager().EmitEvent(sdk.NewEvent(
    "zerone.knowledge.bootstrap_sponsored",
    sdk.NewAttribute("claim_id", claimID),
    sdk.NewAttribute("submitter", msg.Submitter),
    sdk.NewAttribute("fee_amount", msg.Stake),
    sdk.NewAttribute("fund_balance_after", fundBalanceAfter.String()),
    sdk.NewAttribute("address_claims_used", fmt.Sprintf("%d", addressCount+1)),
))
```

### 11. CLI

Add `--sponsored` flag to `submit-claim`:

```
--sponsored    Request bootstrap fund sponsorship (fund pays review fee)
```

Add query command:

```
zeroned query knowledge bootstrap-fund-status
```

### 12. Governance: Fund Management

The bootstrap fund can be topped up via governance proposal (standard `MsgSend` from community pool) or disabled by setting `bootstrap_fund_enabled = false` via `UpdateParams`.

When the fund runs out, it simply stops accepting sponsored claims — submitters pay normally. No hard failure.

### 13. Tests

1. **TestSponsoredClaim_FundPays** — sponsored claim draws from fund, not submitter
2. **TestSponsoredClaim_SubmitterGetsVesting** — accepted sponsored claim creates vesting for submitter
3. **TestSponsoredClaim_FeeDistributed** — fee from fund still goes through revenue split
4. **TestSponsoredClaim_PerAddressLimit** — 11th claim from same address rejected
5. **TestSponsoredClaim_PerEpochLimit** — 101st claim in epoch rejected
6. **TestSponsoredClaim_FundExhausted** — returns clear error when fund is empty
7. **TestSponsoredClaim_FeeCap** — claim above fee cap rejected
8. **TestSponsoredClaim_Disabled** — sponsored=true rejected when fund disabled
9. **TestBootstrapFundStatus_Query** — REST endpoint returns correct balance/counts
10. **TestGenesisInit_FundAllocated** — genesis allocation mints correct amount to fund

## Design Notes

- **22,222 ZRN allocation** — enough for ~22,222 claims at 1 ZRN minimum, or ~4,444 claims at 5 ZRN. At 100 claims/epoch (~1.5 days), the fund lasts ~33-166 epochs (~50-250 days). This is intentionally finite — the subsidy should end.
- **Lifetime per-address limit (10)** — prevents a single actor from draining the fund. 10 claims is enough to bootstrap a contributor's reputation. After that, their accepted facts should generate enough vesting rewards to self-fund.
- **Fee cap (5 ZRN)** — the fund covers standard claims, not premium ones. Submitters who want to pay 100 ZRN for priority review can do so out of pocket.
- **Submitter still pays gas** — the fund covers the review fee only, not the transaction fee. This prevents zero-cost spam (submitter still needs ~0.2 ZRN per tx for gas).
- **One-time genesis mint** — this is NOT ongoing inflation. It's part of the initial supply, counted toward the 222,222,222 ZRN cap. When it's gone, it's gone unless governance tops it up from the community pool.
- **Partnership integration** — partnerships can also sponsor claims. A human-agent pair with a common pot could pay the review fee from their pot. This is already supported since the fee comes from whatever account submits the claim. The bootstrap fund is specifically for unpartnered newcomers.

## Dependencies

- R19-6 (non-refundable review fee) — this builds directly on the fee model

## Files Modified

- `proto/zerone/knowledge/v1/genesis.proto` — bootstrap fund params
- `proto/zerone/knowledge/v1/tx.proto` — sponsored flag on MsgSubmitClaim
- `proto/zerone/knowledge/v1/query.proto` — BootstrapFundStatus query
- `x/knowledge/types/*.pb.go` — regenerated
- `x/knowledge/types/keys.go` — BootstrapFundModuleName, count prefixes
- `x/knowledge/types/genesis.go` — defaults + validation
- `x/knowledge/keeper/genesis.go` — mint fund allocation
- `x/knowledge/keeper/state.go` — count tracking + fund balance queries
- `x/knowledge/keeper/msg_server.go` — sponsored claim logic
- `x/knowledge/keeper/grpc_query.go` — BootstrapFundStatus handler
- `app/app.go` — register module account
- CLI: --sponsored flag + bootstrap-fund-status query
- Tests: 10 new tests

## Commit

Single commit: `feat(knowledge): add knowledge bootstrap fund for sponsored claim review`
