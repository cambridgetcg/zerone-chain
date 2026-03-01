# ZRN Sinks and Flows

A complete map of where ZRN is created, destroyed, locked, and moves through the system.

## Token Creation (Sources)

| Source | Module | Mechanism | Cap |
|--------|--------|-----------|-----|
| **Block Rewards** | `vesting_rewards` | Minted per-block via PoT consensus | 222,222,222 ZRN total |
| **Genesis Bootstrap** | genesis | None — 0 ZRN at genesis | 0 |

There is only one ongoing source of new ZRN: block reward minting. All other token movement is redistribution of existing supply.

## Token Destruction (Sinks)

**No burn.** Zerone does not destroy tokens. Every minted ZRN enters productive circulation.

Slashing penalties and failed challenge stakes are redistributed (to challengers, arbiters, protocol pools) — not burned. The hard supply cap (222,222,222 ZRN) provides natural scarcity.

### Net Emission Rate

At epoch 0 with full validator participation:
- Minted per block: 10 ZRN
- **Net emission: 10 ZRN/block** (all enters circulation)

At floor reward:
- Minted per block: 0.1 ZRN
- **Net emission: 0.1 ZRN/block** (until cap binds)

## Locked ZRN (Not Circulating)

| Lock Type | Module | Duration | Size |
|-----------|--------|----------|------|
| **Vesting Schedules** | `vesting_rewards` | Days to years (category-dependent) | All contributor rewards |
| **Validator Self-Delegation** | `staking` | 7 days unbonding | Min 0.111 ZRN/validator |
| **Delegated Stake** | `staking` | 7 days unbonding | Variable |
| **Challenge Reserves** | `vesting_rewards` | Permanent (5–40% of reward) | Per vesting schedule |
| **Claiming Pot Locks** | `claiming_pot` | Configurable vesting | Per pot |
| **Partnership Common Pot** | `partnerships` | Lock tier dependent | 10% of partnership revenue |
| **Channel Deposits** | `channels` | Until settlement/timeout | Min 1 ZRN |
| **Dispute Bonds** | `disputes` | Until resolution | Tier 1: 1 ZRN → Tier 4: 1,000 ZRN |
| **LIP Stakes** | `gov` | Until proposal resolved | Category: 200–1,000 ZRN |
| **Research Stakes** | `research` | Until review complete | Min 1 ZRN |
| **Tool Registration** | `toolbox` | While active | 11 ZRN per tool |
| **Agent Registration** | `discovery` | While active | 1 ZRN per agent |
| **Home Creation** | `home` | Permanent (fee, not lock) | 10 ZRN per home |
| **Qualification Stake** | `qualification` | Lock period (~2.9 days) | 100 ZRN |

## Flow Map

```
                          ╔═══════════════════╗
                          ║  BLOCK REWARD     ║
                          ║  MINTING          ║
                          ║  (10 ZRN/block)   ║
                          ╚════════╤══════════╝
                                   │
                    ╔══════════════╧══════════════╗
                    ║    4-WAY REVENUE SPLIT      ║
                    ╚═══╤═══╤════════╤════════╤═══╝
                        │   │        │        │
                   55%  │   │ 22%    │19.67%  │ 3.33%
                        │   │        │        │
                  ┌─────┘   │   ┌────┘   ┌────┘
                  ▼         ▼   ▼        ▼
            ┌──────────┐ ┌─────────┐ ┌───────────┐ ┌────────┐
            │CONTRIBUTOR│ │PROTOCOL │ │DEVELOPMENT│ │RESEARCH│
            │(producer) │ │ POOLS   │ │  FUND     │ │ FUND   │
            └─────┬────┘ └────┬────┘ └───────────┘ └───┬────┘
                  │           │                         │
                  │      ┌────┼────┐                ┌───┴──┐
                  │      │    │    │                ▼      ▼
                  │      ▼    ▼    ▼           Research  Founder
                  │   Citation  Verify         Treasury  (7% of
                  │   Pool     Pool            (3.1%)  research)
                  │   (11%)   (6.6%)
                  │            │
                  │       ┌────┴────┐
                  │       ▼         ▼
                  │    Knowledge  Compute
                  │    Module     Pool
                  │    (4.6%)    (2.0%)
                  │
                  ▼
          ┌──────────────────────────┐
          │    CIRCULATING ECONOMY   │
          │                          │
          │  tx fees → RouteFees()   │──── 19.67% to development
          │  billing queries         │──── 3.33% to research
          │  toolbox calls           │──── 55% to creators
          │  tree deliverables       │──── 22% to protocol
          │  channel operations      │
          │  BVM deployments         │
          │  AMM swaps               │
          │  dispute bonds           │
          └──────────────────────────┘
```

## Module Account Balances

Key module accounts that hold ZRN:

| Module Account | Purpose | Typical Balance |
|----------------|---------|----------------|
| `vesting_rewards` | Staging for minted rewards before distribution | Transient (per-block) |
| `development_fund` | Bug bounties, truth discovery, protocol dev | Growing (19.67% of all revenue) |
| `research_fund` | Accumulated research treasury | Growing (3.33% of all revenue) |
| `knowledge` | Verification reward pool | Variable |
| `compute_pool` | Compute credits and rewards | Variable |
| `fee_collector` (SDK) | Accumulated tx fees before distribution | Transient (per-block) |
| `bonded_tokens_pool` (SDK) | All staked ZRN | Large (all delegation) |
| `not_bonded_tokens_pool` (SDK) | Unbonding ZRN | Variable |

## Partnership Economics

The `x/partnerships` module creates joint economic units between humans and agents:

| Parameter | Value | Description |
|-----------|-------|-------------|
| Common Pot Share | 10% | % of partnership revenue to shared pot |
| Default Human Split | 50% | Human's share of partnership earnings |
| Default Agent Split | 50% | Agent's share of partnership earnings |
| Min Partnership Stake | 1 ZRN | Minimum to form partnership |

### Lock Tier Multipliers

Longer commitments earn more:

| Lock Duration (blocks) | Duration (~) | Reward Multiplier | Exit Penalty |
|----------------------|-------------|------------------|-------------|
| 22,222 | ~15 hours | 1.00× | 11% |
| 77,777 | ~2.3 days | 1.11× | 22% |
| 222,222 | ~6.5 days | 1.22× | 33% |
| 777,777 | ~22.6 days | 1.44× | 44% |
| 1,111,111 | ~32.4 days | 1.55× | 55% |
| 2,222,222 | ~64.8 days | 1.77× | 66% |

## Liquidity Pool Mechanics

| Parameter | Value |
|-----------|-------|
| Default Swap Fee | 0.3% (3,000 BPS on 1M scale) |
| Protocol Fee (of swap fee) | 45% |
| Max Pools | 3 |
| Min Initial Liquidity | 10,000 ZRN per side |
| TWAP Window | 1,000 blocks (~42 min) |

The AMM provides on-chain price discovery for ZRN and future ZRN-20 tokens. The TWAP accumulator feeds into the billing module's dynamic pricing oracle.

## Surge Pricing (Toolbox)

When tool demand exceeds capacity, surge pricing activates:

| Parameter | Value | Description |
|-----------|-------|-------------|
| Surge Threshold | 50% | Demand/capacity ratio to activate |
| Critical Threshold | 80% | Maximum surcharge trigger |
| Max Surge Multiplier | 10× | Maximum price multiplier |
| Free Calls/Epoch | 50 | Free tool calls per epoch per home |

Surge revenue flows through the standard 4-way split (contributors, protocol, research, development fund).
