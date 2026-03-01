# Staking Economics

Zerone uses a **4-tier graduated validator system** where validators earn the right to verify higher-confidence knowledge domains through demonstrated competence, not just capital.

## Validator Tiers

| Tier | Name | Min Stake | Min Reputation | Min Verifications | Min Accuracy | Knowledge Domains |
|------|------|-----------|---------------|-------------------|-------------|-------------------|
| 1 | **Apprentice** | 0.111 ZRN | 0% | 0 | 0% | protocol, computational, formal |
| 2 | **Verified** | 1.11 ZRN | 77% | 22 | 77% | + empirical |
| 3 | **Scholar** | 1,111 ZRN | 50% | 11 | 50% | + oracle, attestation |
| 4 | **Guardian** | 11,111 ZRN | 77% | 333 | 77% | All categories |

### Reward & Selection Multipliers

| Tier | Reward Multiplier | Selection Weight | Slash Multiplier |
|------|------------------|-----------------|-----------------|
| Apprentice | 0.1× | 0.1× | 1.5× |
| Verified | 0.5× | 0.5× | 1.2× |
| Scholar | 1.0× | 1.0× | 1.0× |
| Guardian | 2.0× | 1.5× | 1.0× |

Key design decisions:
- **Apprentices** earn very little (0.1×) but can still participate — this is the learning period
- **Guardians** earn 2× but have the highest entry bar (11,111 ZRN + 333 verifications + 77% accuracy)
- **Apprentice slash is 1.5×** — harsher on new validators to filter out bad actors quickly
- **Guardian selection weight is 1.5×** (not 2×) — prevents Guardians from dominating verification rounds

### Guardian Special Requirements

Guardians must have:
- At least 33 **contested verifications** (verifications that were themselves challenged)
- A contested verification multiplier of 3× (each contested verification counts as 3 normal ones)
- Zero active slashes

This ensures Guardians are battle-tested — they've been challenged and survived.

## Staking Parameters

| Parameter | Value | Description |
|-----------|-------|-------------|
| Unbonding Period | 268,560 blocks (~7 days) | Time to unstake |
| Max Validators | 100 | Active validator cap (Scholar+ tiers) |
| Min Self-Delegation | 0.111 ZRN | Minimum to register |
| Virtual Stake | 11 ZRN | VRF participation weight for T0/T1 |
| Redelegation Cooldown | 1,111 blocks (~46 min) | Between redelegations |

## Slashing

### Progressive Slashing

| Parameter | Value | Description |
|-----------|-------|-------------|
| Max Slashes/Epoch | 2 | Before deactivation |
| Max Total Slashes | 3 | Cumulative before permanent deactivation |
| Slash Escalation | 10% (100,000 BPS) | Each slash is 10% worse than the previous |
| Slash Decay Period | 34,272 blocks (~1 day) | Epoch for slash count reset |

### Knowledge-Specific Slashing

| Offense | Slash Rate | Description |
|---------|-----------|-------------|
| Wrong Verification | 5% | Voted incorrectly |
| Missed Reveal | 10% | Committed but didn't reveal |
| Equivocation | 20% | Conflicting votes in same round |
| Invalid Claim | 22% | Submitted a claim that fails validation |

### Reputation System

| Event | Reputation Change |
|-------|------------------|
| Correct verification | +0.01% |
| Incorrect verification | −0.02% |
| Slash event | −1% |

Reputation is a long grind upward and a fast slide downward. Getting from 0% to 77% (Verified tier) requires ~7,700 correct verifications with zero mistakes. One slash wipes 100 correct verifications.

## Economic Dynamics

### Delegation

Delegators stake to validators and share in their rewards. Commission rates are set per-validator (max 100%, capped at registration). There is no minimum delegation amount beyond the account balance.

Key dynamics:
- Delegators share in slashing risk (their stake gets slashed too)
- Delegators can redelegate between validators (with cooldown)
- Unbonding takes ~7 days (during which tokens earn no rewards)

### Validator Economics

A Guardian-tier validator with full participation:

```
Block reward to validators = 55% × 10 ZRN × 2.0× tier multiplier
                           = 11 ZRN per block (maximum)
```

At ~2.521s blocks (~1,428 blocks/hour), that's:
- ~4,364 ZRN/hour total (epoch 0)
- Rewards halve annually (~1-year half-life, 0.994478× per epoch)

Shared across all active validators weighted by selection + tier multipliers. With 22 validators of equal tier:
- ~198 ZRN/hour per validator (year 0)
- ~99 ZRN/hour per validator (year 1)
- ~50 ZRN/hour per validator (year 2)
- Floor: ~20 ZRN/hour per validator (year 6.6+)

The 1-year half-life balances early adopter incentive with accessibility: validators joining at year 2 earn half the genesis rate, not 1/100th.

### Qualification Module

Beyond the basic tier system, the `x/qualification` module adds domain-specific validation:
- Validators can earn qualifications in specific knowledge domains
- Qualifications require additional stake (100 ZRN), verifications (100), and accuracy (80%)
- Qualifications expire and must be renewed
- Cross-domain qualifications get a discount based on existing domain expertise

This creates specialisation: a validator might be highly qualified in "formal mathematics" but unqualified in "climate science," steering them toward domains where they're competent.

## Anti-Sybil Measures

| Mechanism | Value | Purpose |
|-----------|-------|---------|
| Max Apprentice Validators | 111 | Sybil cap on low-stake tier |
| Min Stake for Verification | 0.111 ZRN | Skin in the game |
| Reputation Requirements | Per-tier | Can't buy your way to high tiers |
| Contested Verification Req | 33 (Guardian) | Must survive adversarial testing |

The tiered system is inherently anti-Sybil: you can create 1,000 Apprentice validators, but they each earn 0.1× rewards with 0.1× selection weight. The economic returns from splitting stake across Sybil validators are worse than concentrating into a single high-tier validator.
