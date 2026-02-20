# R2-6 — Vesting Rewards Module: Full Port

## Goal

Port the reward distribution module — block rewards, decay curves,
revenue routing (4-way split), research fund management, and the
founder operational share.

## Dependencies

- R1-2 (core proto) must be complete. Independent of R2-1 through R2-5.

## Working Directory

`/Users/yuai/Desktop/Zerone`

## Reference

- `/Users/yuai/Desktop/legible_money/x/vesting_rewards/` — full module
- `/Users/yuai/Desktop/legible_money/proto/legible/vesting_rewards/` — proto defs
- `/Users/yuai/Desktop/legible_money/x/vesting_rewards/keeper/keeper_test.go` — 54 tests
- `/Users/yuai/Desktop/legible_money/x/vesting_rewards/keeper/rewards.go` — revenue routing
- `/Users/yuai/Desktop/legible_money/docs/PARAMETERS.md` — vesting_rewards params (16 + 30 category configs)

## Proto Files

### `proto/zerone/vesting_rewards/v1/types.proto`
```protobuf
message RewardRouting {
  string recipient_address = 1;
  string recipient_amount = 2;     // uzrn
  string protocol_share = 3;
  string research_share = 4;
  string burn_amount = 5;
  string citation_pool = 6;
  string verification_pool = 7;
  string treasury_share = 8;
  string founder_share = 9;        // 7% of research (when active)
}

message BlockRewardState {
  string current_reward = 1;       // uzrn per block
  uint64 last_decay_block = 2;
  uint64 epoch_number = 3;
  string total_distributed = 4;    // lifetime
  string total_burned = 5;
}
```

### `proto/zerone/vesting_rewards/v1/genesis.proto`
```protobuf
message Params {
  // Block rewards
  string block_reward = 1;                    // default: "10000000" (10 ZRN)
  uint64 reward_decay_bps = 2;               // default: 850,000 (0.85x per epoch)
  uint64 blocks_per_reward_epoch = 3;         // default: 100,000

  // Revenue split (governance-adjustable — B22-3 fix)
  zerone.common.v1.RevenueSplit revenue_split = 4;
  zerone.common.v1.ProtocolSubSplit protocol_sub_split = 5;

  // Founder share
  uint64 founder_share_bps = 6;              // default: 70,000 (7% of research)
  string founder_address = 7;                 // bech32, empty = disabled
  uint64 governance_activation_height = 8;    // block when founder share sunsets (0 = never)

  // Category reward multipliers (30 categories)
  repeated CategoryRewardConfig category_configs = 9;

  // Research fund
  string research_fund_module_account = 10;
}

message CategoryRewardConfig {
  string category = 1;
  uint64 multiplier_bps = 2;      // 1,000,000 = 1.0x
}
```

### `proto/zerone/vesting_rewards/v1/tx.proto`
- MsgDistributeBlockReward (called by BeginBlocker, not user-facing)
- MsgUpdateParams
- MsgDepositToResearchFund (internal, from other modules)

### `proto/zerone/vesting_rewards/v1/query.proto`
- QueryBlockRewardState
- QueryResearchFundBalance
- QueryRewardHistory (pagination)
- QueryParams
- QueryFounderShareStatus

## Key Implementation

### Revenue routing (core function)

```go
func (k Keeper) DistributeRevenue(ctx sdk.Context, source string, totalAmount sdk.Coins) (*types.RewardRouting, error) {
    params := k.GetParams(ctx)
    split := params.RevenueSplit

    // 4-way split using params (NOT constants)
    contributorAmount := totalAmount * split.ContributorBps / 1_000_000
    protocolAmount := totalAmount * split.ProtocolBps / 1_000_000
    researchAmount := totalAmount * split.ResearchBps / 1_000_000
    burnAmount := totalAmount - contributorAmount - protocolAmount - researchAmount

    // Protocol sub-split
    subSplit := params.ProtocolSubSplit
    citationPool := protocolAmount * subSplit.CitationBps / 1_000_000
    verificationPool := protocolAmount * subSplit.VerificationBps / 1_000_000
    treasuryAmount := protocolAmount - citationPool - verificationPool

    // Research fund deposit (with founder auto-split)
    k.DepositToResearchFund(ctx, researchAmount)

    // Burn
    k.BurnTokens(ctx, burnAmount)

    // Return routing for audit trail
}
```

### Research fund with founder split

```go
func (k Keeper) DepositToResearchFund(ctx sdk.Context, amount sdk.Coins) error {
    params := k.GetParams(ctx)

    if k.isFounderShareActive(ctx, params) {
        founderAmount := amount * params.FounderShareBps / 1_000_000
        remainingResearch := amount - founderAmount

        // Send founder share to founder address
        k.bankKeeper.SendCoins(ctx, moduleAccount, founderAddr, founderAmount)
        // Send remainder to research fund
        k.bankKeeper.SendCoinsFromModuleToModule(ctx, moduleName, "research_fund", remainingResearch)
    } else {
        // All to research fund
        k.bankKeeper.SendCoinsFromModuleToModule(ctx, moduleName, "research_fund", amount)
    }
}
```

### DisburseFromResearchFund

This method is called by governance (R3-5) after 2-of-2 vote:

```go
func (k Keeper) DisburseFromResearchFund(ctx sdk.Context, recipient sdk.AccAddress, amount sdk.Coins) error {
    return k.bankKeeper.SendCoinsFromModuleToAccount(ctx, "research_fund", recipient, amount)
}
```

### Block reward decay

```go
func (k Keeper) BeginBlocker(ctx sdk.Context) {
    // 1. Calculate current block reward (with decay)
    // 2. Mint reward tokens
    // 3. Distribute via DistributeRevenue
    // 4. Check epoch boundary → apply decay
}
```

### SendRestriction hook

Research fund can only receive via `DepositToResearchFund`, not direct bank sends:

```go
func (k Keeper) RegisterSendRestrictions(bankKeeper BankKeeperWithRestrictions) {
    bankKeeper.AppendSendRestriction(func(ctx sdk.Context, fromAddr, toAddr sdk.AccAddress, amt sdk.Coins) (sdk.AccAddress, error) {
        if toAddr == researchFundAddr && fromAddr != moduleAddr {
            return toAddr, fmt.Errorf("direct sends to research fund not allowed")
        }
        return toAddr, nil
    })
}
```

## Tests

Port all 54 tests. Add:

| Test | Validates |
|------|-----------|
| `TestRevenueSplit_FromParams` | Split uses params, not constants |
| `TestRevenueSplit_SumsTo100` | Params validated to sum to 1M |
| `TestFounderShare_ActiveWhenConfigured` | 7% goes to founder address |
| `TestFounderShare_DisabledWhenEmpty` | Empty address → all to research |
| `TestFounderShare_SunsetsAtHeight` | Governance activation → no more founder share |
| `TestResearchFund_SendRestriction` | Direct sends blocked |
| `TestResearchFund_Disbursement` | DisburseFromResearchFund works |
| `TestBlockReward_DecayPerEpoch` | Reward decreases correctly |
| `TestEconomicConservation` | minted = distributed + burned (no leaks) |

## Verification

```bash
make proto-gen
go build ./...
go test ./x/vesting_rewards/... -count=1 -v
```

## Commit

```
feat(vesting_rewards): block rewards, revenue routing, research fund, founder share
```

## Do NOT

- Use hardcoded revenue split constants (must be from params)
- Skip the SendRestriction on research fund
- Allow negative decay (reward must be non-negative)
- Skip economic conservation check in tests
- Forget the Migrator stub
