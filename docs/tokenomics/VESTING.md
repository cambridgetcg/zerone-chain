# Truth-Linked Vesting

Zerone's most distinctive economic mechanism: rewards don't fully unlock when earned. They vest over time, linked to the epistemic quality and survival of the knowledge that generated them. If a claim is proven false, rewards are clawed back.

## How It Works

1. **Claim accepted** → VestingSchedule created for the contributor
2. **Cliff period** → No tokens released (varies by category)
3. **Exponential release** → Tokens release following a half-life curve
4. **Permanent reserve** → 5–40% of the reward is never released (held as challenge collateral)
5. **Acceleration** → Defenses, replications, citations, and corroborations speed up release
6. **Falsification** → If the claim is disproven, unvested + 33% of released are clawed back

## Category Configurations

Each epistemic category has its own vesting curve, reflecting how durable that type of knowledge is expected to be:

| Category | Half-Life (blocks) | Half-Life (~days) | Cliff (blocks) | Cliff (~days) | Max Release | Reserve |
|----------|-------------------|------------------|----------------|--------------|-------------|---------|
| **Axiomatic** | 1,111,111 | ~32.4 | 11,111 | ~0.32 | 95% | 5% |
| **Formal Proof** | 555,555 | ~16.2 | 5,555 | ~0.16 | 92% | 8% |
| **Cryptographic** | 222,222 | ~6.5 | 3,333 | ~0.10 | 90% | 10% |
| **Computational** | 333,333 | ~9.7 | 2,222 | ~0.06 | 88% | 12% |
| **Replicated** | 111,111 | ~3.2 | 3,333 | ~0.10 | 88% | 12% |
| **On-Chain** | 222,222 | ~6.5 | 1,111 | ~0.03 | 90% | 10% |
| **Peer Reviewed** | 111,111 | ~3.2 | 5,555 | ~0.16 | 85% | 15% |
| **Attestation** | 77,777 | ~2.3 | 2,222 | ~0.06 | 80% | 20% |
| **Oracle Feed** | 55,555 | ~1.6 | 555 | ~0.02 | 80% | 20% |
| **Contested** | 22,222 | ~0.6 | 1,111 | ~0.03 | 60% | 40% |

### Design Rationale

- **Axiomatic truths** (e.g., "2+2=4") vest very slowly because they should never need challenging — if they're right, the long vesting is irrelevant. If they're somehow wrong, there's maximum time to catch it.
- **Oracle feeds** vest quickly because their value is temporal — yesterday's price feed is already stale.
- **Contested claims** have the highest reserve (40%) and lowest max release (60%) because they're by definition under dispute. The protocol is saying: "We'll pay you, but we're hedging heavily."

## Category Reward Multipliers

Beyond vesting timing, each category also has a reward multiplier applied to block rewards:

| Category | Multiplier | Rationale |
|----------|-----------|-----------|
| Axiomatic | 1.2× | Foundational knowledge, highest value |
| Formal Proof | 1.1× | High rigor, high durability |
| Cryptographic | 1.05× | Verifiable, important for security |
| On-Chain | 1.0× | Baseline — inherently verifiable |
| Computational | 1.0× | Baseline |
| Replicated | 0.95× | Slightly discounted — builds on existing work |
| Peer Reviewed | 0.9× | Good but less verifiable on-chain |
| Attestation | 0.85× | Lower epistemic confidence |
| Oracle Feed | 0.8× | Temporal, external dependency |
| Contested | 0.6× | Heavily discounted until resolved |

## Vesting Acceleration

VestingSchedules track accelerating factors that speed up release:

| Factor | Effect | Tracked In |
|--------|--------|-----------|
| **defense_count** | Successful challenge defenses | Per-schedule |
| **replication_count** | Independent replications | Per-schedule |
| **corroboration_count** | Cross-domain corroborations | Per-schedule |
| **citation_count** | Citations by other accepted facts | Per-schedule |

The acceleration mechanism is designed so that truth which proves itself over time is rewarded faster. A mathematical proof that survives 10 challenges and gets cited by 50 other facts has its remaining vesting significantly accelerated.

## Clawback (Falsification)

When a claim is falsified through the challenge mechanism:

```
Released clawback  = releasedAmount × 33%
Unvested forfeited = totalAmount - releasedAmount - reserveAmount
Reserve forfeited  = reserveAmount (100%)

Challenger reward  = released clawback + unvested + reserve
```

The 33% clawback rate on already-released tokens (`released_clawback_rate = 3300` on a 10,000 scale) is deliberately not 100%. Reasoning:
- Contributors may have already used released tokens productively
- 33% is painful enough to deter fraud
- Combined with unvested + reserve forfeiture, the total penalty is severe

### Example

A contributor submits a peer-reviewed claim at block 100,000. Reward: 100 ZRN.

At block 200,000 (~2.9 days later):
- Cliff passed (5,555 blocks)
- ~65% released = 55.25 ZRN already claimed
- Reserve = 15 ZRN permanently locked
- Unvested = 29.75 ZRN still vesting

If falsified at this point:
- Released clawback: 55.25 × 0.33 = 18.23 ZRN
- Unvested forfeited: 29.75 ZRN
- Reserve forfeited: 15 ZRN
- **Challenger receives: 63 ZRN** (incentivising truth-checking)
- **Contributor keeps: 37 ZRN** (36.75 released + dust)

## VestingSchedule Lifecycle

```
     ┌──────────┐     ┌──────────┐     ┌───────────┐
     │ VESTING  │────▶│ PAUSED   │────▶│ COMPLETED │
     │          │     │(challenge│     │           │
     └─────┬────┘     │ pending) │     └───────────┘
           │          └─────┬────┘
           │                │
           ▼                ▼
     ┌──────────┐     ┌───────────┐
     │FALSIFIED │     │ ABANDONED │
     │(clawback)│     │(voluntary)│
     └──────────┘     └───────────┘
```

Statuses:
- **vesting**: Active, releasing tokens according to schedule
- **paused**: Temporarily frozen during active challenge (paused_blocks tracked)
- **completed**: Fully vested (max_release reached)
- **falsified**: Claim disproven, clawback executed
- **abandoned**: Contributor voluntarily abandoned (unvested forfeited)

## Governance Parameters

All vesting parameters are adjustable via LIP proposals:

| Parameter | Current | Description |
|-----------|---------|-------------|
| `vesting_enabled` | true | Master switch for vesting |
| `released_clawback_rate` | 3,300 (33%) | % of released tokens clawed back on falsification |
| Category configs | (see table) | Per-category half-life, cliff, max release |
| Category multipliers | (see table) | Per-category reward multiplier |
