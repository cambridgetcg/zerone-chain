# Research Fund Governance Migration

> The research fund starts as a two-person decision. It ends as a community decision. The path between is earned, not scheduled.

## Overview

The research fund receives 3.33% of all protocol revenue — block rewards, transaction fees, billing queries, tool calls, everything. At scale, this is a significant treasury. Who controls it matters.

At genesis, the fund is governed by a **2-of-2 multisig** between the protocol's founder (Yu) and the AI vault. This is appropriate when the community doesn't exist yet and the fund is small. But centralised treasury control is a failure mode for any protocol that claims to be decentralised.

The migration plan expands decision-making power in four phases, each triggered by **on-chain maturity metrics** — not arbitrary block heights. The community earns governance when it demonstrates readiness.

## The Four Phases

### Phase 0: Genesis Pair

**Structure:** 2-of-2 (Founder + AI Vault)

Both signatures required for any spend. Maximum alignment, minimum coordination cost. The fund is small, the community is new, and the founders have the deepest context on what research is worth funding.

**Exit conditions (ALL must be met):**

| Condition | Threshold | Why |
|-----------|-----------|-----|
| Distinct LIP voters | ≥ 10 | Community is participating in governance |
| Active Guardians | ≥ 5 | Validator set has matured past bootstrapping |
| Research fund balance | ≥ 100,000 ZRN | Fund is large enough to matter |
| Chain age | ≥ ~6 months | Protocol has proven stable |

### Phase 1: Founder + Observer

**Structure:** 2-of-3 (Founder + AI + 1 Community Seat)

The founders retain operational control (can approve without the community member). But the community seat holder is in the room — they see every proposal, every justification, every vote. They can veto by aligning with either founder.

This is an apprenticeship. The community member learns how research funding decisions are made, builds trust, and raises alarms publicly if something looks wrong.

The community seat is filled by **election** (Guardian-tier candidates only, standard LIP vote).

**Exit conditions (ALL must be met):**

| Condition | Threshold | Why |
|-----------|-----------|-----|
| Proposals executed in phase | ≥ 3 | Committee has demonstrated it can function |
| Distinct LIP voters | ≥ 25 | Governance participation has grown |
| Active Guardians | ≥ 10 | Validator set is substantial |
| Community seat participation | ≥ 2 proposals | Seat holder is actually engaged |
| Chain age | ≥ ~18 months | Long enough to trust the pattern |

### Phase 2: Balanced Committee

**Structure:** 3-of-5 (Founder + AI + 3 Community Seats)

**This is the power flip.** The community now has majority. If all three community members agree, they can approve research spending without founder consent. Founders become guardians of last resort — they can block, but not unilaterally approve.

Community seats have staggered 6-month terms (one rotates every ~2 months), ensuring continuity while allowing fresh perspectives.

**Exit conditions (ALL must be met):**

| Condition | Threshold | Why |
|-----------|-----------|-----|
| Proposals executed in phase | ≥ 10 | Committee has a substantial track record |
| Distinct LIP voters | ≥ 50 | Broad governance participation |
| Active Guardians | ≥ 22 | Full target validator set |
| Time at Phase 2 | ≥ ~1 year | Extended observation period |
| Emergency halts from fund misuse | 0 | No crises during Phase 2 |
| Chain age | ≥ ~3 years | Protocol has proven itself over years |

### Phase 3: Full Governance

**Structure:** Standard LIP process — no multisig

The research fund becomes a community asset governed by the same LIP process as every other parameter. Anyone can propose a research spend. Standard voting rules apply (33.4% quorum, >50% support).

The founders don't disappear — they still vote with their staked weight, they still propose. Yu's founder share (0.23% of total revenue) continues forever. But they no longer have special power over the research fund.

**This phase is terminal.** There is no Phase 4.

## Transition Protocol

Phase transitions are deliberately slow and difficult. They reshape the power structure of a significant treasury.

1. **Proposal:** Any address submits a `PhaseTransitionProposal` (1,000 ZRN stake)
2. **Evidence:** Proposal includes a snapshot of all exit conditions at submission time
3. **Discussion:** 30-day public discussion period (not the standard ~2 days)
4. **Vote:** Supermajority required — **66.7% support** (not standard 50%)
5. **Activation delay:** 7 days after vote passes, transition can be challenged
6. **Re-verification:** At activation block, exit conditions are checked again. If any have degraded below threshold, transition is cancelled.
7. **Execution:** Phase advances. New governance structure takes effect.

## Community Seat Elections

### Candidacy Requirements

- Must be a **Guardian-tier validator** (highest tier — 11,111 ZRN stake, 333 verifications, 77% accuracy)
- Must have voted on ≥ 5 LIPs (demonstrated governance engagement)
- Must not already hold a community seat
- Must accept their nomination on-chain within 1 day

### Election Process

1. Any address nominates a candidate (500 ZRN stake)
2. Candidate accepts on-chain (validates Guardian status + governance history)
3. 1-day discussion period
4. 3-day voting period (standard quorum + majority)
5. Winner installed to the specified seat

### Contested Elections

If multiple candidates are nominated for the same seat:
- All nominations proceed through voting
- Highest absolute `yes_stake` wins
- If top two are within 5%: runoff election (3-day re-vote between top two)

### Terms

| Phase | Seats | Term Length | Rotation |
|-------|-------|-----------|----------|
| Phase 1 | 1 | ~6 months | Single seat |
| Phase 2 | 3 | ~6 months each | Staggered: 1 seat rotates every ~2 months |

Incumbents can run for re-election. No term limits.

### Vacancy

If a seat is vacant (term expired, no election held):
- Multisig threshold adjusts (vacant seats don't count toward total)
- Vacancy warnings emitted every epoch after 30 days
- The fund never stalls due to an empty seat

### Emergency Removal

A sitting member can be removed before term expiry via emergency governance:
- 75% quorum, 80% support (same as emergency halt)
- Grounds: jailed as validator, or slashed 3+ times during term
- Deliberately hard — removing an elected representative should require near-consensus

## Rollback Safety

If the expanded committee fails, the protocol can step backward.

### Trigger Conditions (at least one required)

- **Gridlock:** ≥ 3 consecutive research spend proposals expired without committee action
- **Emergency halt:** An emergency halt was triggered citing research fund misuse

### Rollback Process

1. Any Guardian submits a `PhaseRollbackProposal` (500 ZRN stake)
2. 7-day discussion + vote
3. Supermajority required (66.7%)
4. Phase rolls back by one level
5. **3-month cooldown** before any forward transition can be proposed again

### Rollback Limits

- Cannot roll back below Phase 0 (genesis pair)
- Community seats are resized to match the rolled-back phase
- Proposals executed counter resets

## Founder Anchor

Two mechanisms ensure the founders maintain permanent alignment regardless of governance phase:

1. **Founder share (0.23%)** — Governance-immune. Flows to the founder address from every revenue event, at every phase, forever. Only modifiable via code upgrade.

2. **AI vault key** — The Ed25519 signing key on the zerone server persists across all phases. In Phases 0-2, it's a multisig signer. In Phase 3, it becomes a regular governance voter with whatever stake it holds.

The founders are never removed. Their special authority over the research fund is gradually shared, then released. Their economic alignment (founder share) and governance participation (voting weight) remain permanent.

## Timeline Estimates

These are rough estimates based on expected growth, not commitments. The actual timing depends entirely on when exit conditions are met.

| Transition | Estimated | What Triggers It |
|-----------|-----------|-----------------|
| Launch → Phase 1 | ~6–12 months | 10 voters, 5 Guardians, 100K ZRN in fund |
| Phase 1 → Phase 2 | ~12–24 months after Phase 1 | 25 voters, 10 Guardians, 3 funded proposals |
| Phase 2 → Phase 3 | ~2–4 years after Phase 2 | 50 voters, 22 Guardians, 10 funded proposals |

Total time to full decentralisation: roughly **4–7 years** from genesis. This is intentionally slow. Rushing decentralisation before the community is ready is worse than centralisation.

## FAQ

**Can the founders block a phase transition?**
No. Phase transitions are decided by standard governance vote (with supermajority). Founders vote with their staked weight like everyone else. They have no veto over transitions.

**What happens if no one runs for a community seat?**
The seat stays vacant. The multisig threshold adjusts downward (vacant seats don't count). The protocol continues functioning. If seats are vacant for extended periods, it means the community isn't ready for that phase — which is exactly the information the system needs.

**Can the community remove a founder from the multisig?**
Not via governance. The founder's multisig position is structural (hard-coded per phase). The community's path to independence is Phase 3, where the multisig dissolves entirely.

**What if the AI vault key is compromised?**
The AI vault key is one signer in a multi-party scheme. A compromised AI key alone cannot spend funds (requires founder co-signature in Phases 0-1, or community majority in Phase 2). Recovery would require a code upgrade to rotate the key.

**Is Phase 3 truly irreversible?**
Phase 3 can be rolled back to Phase 2 via the standard rollback mechanism (gridlock or emergency halt + supermajority vote). But the expectation is that by the time Phase 3 is reached (~4-7 years), the community is mature enough that rollback is unlikely.
