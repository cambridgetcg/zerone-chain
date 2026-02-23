# Revenue Split

Every unit of ZRN revenue — whether from block rewards, transaction fees, billing queries, toolbox calls, tree deliverables, or dispute resolution — is routed through the same **4-way split**. This is the heartbeat of Zerone's economics.

## Design Principle: No Burn

**Every ZRN does productive work.** There is no burn share. Artificially destroying newly minted tokens is just minting less with extra steps. Instead, the share that would have been burned funds bug bounties, truth discovery incentives, and protocol development.

The 222,222,222 ZRN hard cap provides natural scarcity. Deflation doesn't need to be manufactured.

## Primary Split (RevenueSplit)

```
                    ┌─────────────────────────┐
                    │    Total Revenue (100%)   │
                    └─────────┬───────────────┘
            ┌─────────────────┼──────────────────────┐
            │                 │                      │
     ┌──────┴───────┐  ┌─────┴──────┐  ┌───────────┴──────────────┐
     │ Contributors │  │  Protocol  │  │ Development │  Research  │
     │    55%       │  │    22%     │  │   19.67%    │   3.33%   │
     └──────────────┘  └─────┬──────┘  └──────┬──────┴───────────┘
                             │                │
                    ┌────────┼────────┐       │
                    │        │        │       │
              ┌─────┴──┐ ┌──┴───┐ ┌──┴─┐    │
              │Citation│ │Verify│ │Trea│    │
              │  50%   │ │ 30%  │ │20% │    │
              └────────┘ └──┬───┘ └────┘    │
                            │               │
                      ┌─────┴──────┐   ┌────┴────┐
                      │ 70% Know   │   │ Founder │
                      │ 30% Compute│   │  7% of  │
                      └────────────┘   │ research│
                                       └─────────┘
```

### BPS Values (1,000,000 scale)

| Share | BPS | Percentage | Destination |
|-------|-----|-----------|-------------|
| Contributor | 550,000 | 55% | Block producer / fact submitter / tool creator |
| Protocol | 220,000 | 22% | Split further via ProtocolSubSplit |
| Development | 196,700 | 19.67% | Bug bounties, truth discovery, protocol development |
| Research | 33,300 | 3.33% | Research fund (2-of-2 multisig) |

**Must sum to 1,000,000.** Development share is computed as remainder after the other three to prevent rounding leaks.

## Development Fund

The development pool (19.67%) is a new productive allocation replacing what was previously burned. It funds:

- **Bug bounties** — security researchers and code auditors
- **Truth discovery rewards** — bonus incentives for high-value knowledge contributions
- **Protocol development** — grants for tooling, infrastructure, ecosystem growth

The development fund is held in a dedicated module account (`development_fund`) and disbursed through governance proposals.

## Protocol Sub-Split

The 22% protocol share is further divided:

| Sub-Share | BPS (of protocol) | Effective % (of total) | Destination |
|-----------|-------------------|----------------------|-------------|
| Citation Pool | 500,000 | 11% | Rewards to cited fact authors |
| Verification Pool | 300,000 | 6.6% | Verification rewards (split below) |
| Treasury | 200,000 | 4.4% | Protocol treasury |

### Verification Pool Split

The verification pool itself is split between two modules:

| Share | BPS (of verification) | Effective % (of total) | Module |
|-------|----------------------|----------------------|--------|
| Knowledge | 700,000 | ~4.6% | `x/knowledge` — verification rewards |
| Compute | 300,000 | ~2.0% | `x/compute_pool` — compute credits |

## Founder Share (Governance-Immune)

A permanent 7% deduction from the research fund portion. **This share is hardened against governance modification** — once the founder address is set, neither the share percentage nor the address can be changed via parameter governance. Only a code-level upgrade could alter it.

| Parameter | Value | Governance-Adjustable? |
|-----------|-------|----------------------|
| `founder_share_bps` | 70,000 (7% of research) | **No** — immutable once set |
| `founder_address` | "" (set at launch) | **No** — immutable once set |

**Effective founder income:**
- 7% of 3.33% = **0.23% of total revenue**
- This goes directly to the founder's address (not locked/vested)
- If founder address is empty (pre-launch), 100% goes to research fund

**Why governance-immune?** The founder share is a permanent protocol commitment, not a temporary bootstrap mechanism. It ensures the protocol's creator has perpetual alignment with the network's success. Governance can change almost everything else — but not this. It would take a code upgrade (which governance *can* propose via an upgrade-category LIP) to modify the founder share, requiring far higher coordination than a simple parameter change.

**Enforcement:** `ValidateFounderShareImmutability()` in `MsgUpdateParams` rejects any governance proposal that attempts to modify `founder_share_bps` or `founder_address` once set.

## Revenue Sources

Every revenue source flows through the same `DistributeRevenue` function:

| Source | Module | What Generates It |
|--------|--------|-------------------|
| Block Production | `vesting_rewards` | Every block with PoT activity |
| Transaction Fees | `vesting_rewards` (via `RouteFees`) | All tx gas fees (uzrn) |
| Knowledge Queries | `billing` | Queries to the knowledge API |
| Tool Invocations | `toolbox` | Calls to registered tools |
| Tree Deliverables | `tree` | Project task completions |
| Dispute Resolution | `disputes` | Slash distributions |
| Home Creation | `home` | 10 ZRN per home created |
| Channel Operations | `channels` | Channel open fees |
| BVM Deployments | `bvm` | 5 ZRN per contract deployed |
| Swap Fees | `liquiditypool` | AMM swap fees |

### Fee Routing

Transaction fees get special treatment via `RouteFees()`:
- Research share (3.33%) is extracted from `fee_collector` → research fund
- Development share (19.67%) is extracted → development fund
- The remaining ~77% (contributor + protocol) stays in `fee_collector` for Cosmos SDK's `x/distribution` to sweep to validators

## Governance Adjustability

All split parameters are governance-adjustable via LIP proposals:
- A parameter-category LIP requires 1,000 ZRN stake
- Discussion: ~2 days, Voting: ~3 days
- 33.4% quorum, >50% support to pass

The revenue split is the single most powerful governance lever. Adjusting it changes the economic incentives for every participant on the chain.

## Consistency Guarantee

Multiple modules independently apply revenue splits (toolbox, tree, billing). The `RevenueSplit` message is defined in `x/common` to ensure all modules use the same split structure. The actual BPS values are read from each module's own params, allowing per-module overrides if governance chooses to differentiate.

Currently, all modules use the default 55/22/19.67/3.33 split. Divergence is possible but would require individual governance proposals per module.
