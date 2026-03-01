# R25 — Human-Agent Collaboration: Truth-Seeking Roles and Testnet Strategy

**Goal:** Deep dive into the distinct roles of humans and agents in ZERONE's truth-seeking system. Test the full knowledge lifecycle as both human and agent accounts. Design testnet roleplay strategies that exercise every collaboration pathway.

## Why This Matters

ZERONE is designed as a human-agent partnership network. But so far:
- `account_type` ("human" vs "agent") is **stored but never checked** — there's no functional distinction
- Partnerships exist (`x/partnerships`) but nobody has tested them in a knowledge-contribution context
- The PoT verification round uses a **stub evaluator** (`verdict = "accept", confidence = 600K`) — validators don't actually evaluate claims
- Qualification (`x/qualification`) has 4 pathways but hasn't been tested against the knowledge flow
- The formation pool exists but nobody has tried human-agent matching

This batch answers: **What does it actually mean to be a human vs an agent on ZERONE, and do the systems that distinguish them work?**

## What Exists

| Module | RPCs | Tests | Role in Collaboration |
|--------|------|-------|----------------------|
| `x/knowledge` | 21 tx RPCs, 28 queries | 414 | Claim submission, commit-reveal verification, challenges, domains |
| `x/partnerships` | 11 tx RPCs | 101 | Human-agent pair formation, consensus ops, safety freezes, coercion signals |
| `x/qualification` | 8 tx RPCs | 42 | Domain qualification (stake, track record, cross-ref, inheritance) |
| `x/discovery` | 5 tx RPCs | 32 | Agent profiles, capabilities, heartbeats |
| `x/disputes` | 7 tx RPCs | 85 | Formal dispute resolution with arbiters |
| `x/evidence_mgmt` | 5 tx RPCs | (unknown) | Evidence chain-of-custody |
| `x/research` | 9 tx RPCs | 45 | Research submissions, peer review, bounties |
| `x/tree` | 29 tx RPCs | 53 | Projects, tasks, services, seeding |
| `x/vesting_rewards` | (via hooks) | 62 | Truth-linked reward vesting with clawback |
| `x/claiming_pot` | 3 tx RPCs | (unknown) | Token distribution pots with eligibility criteria |
| `x/capture_challenge` | 5 tx RPCs | (unknown) | Challenge mechanism for regulatory capture |
| `x/capture_defense` | 3 tx RPCs | (unknown) | Verification recording, domain analysis |
| `x/alignment` | 2 tx RPCs | (unknown) | Network health observations, dimension scores |

**Critical design gaps to probe:**
1. `account_type` is never checked — any account can do anything regardless of being "human" or "agent"
2. Partnership `partnership_id` on claims is optional — claims don't require partnership context
3. Vote extensions use stub evaluation — validators auto-accept everything
4. Qualification status isn't checked during verification — unqualified validators can verify any domain

## Sessions (7)

| # | File | Scope | Parallelism |
|---|------|-------|-------------|
| R25-1 | R25-1-knowledge-lifecycle.md | Full claim lifecycle on localnet: submit → verify → challenge → patronise → expire | Wave 1 |
| R25-2 | R25-2-partnership-knowledge.md | Form partnerships, submit claims as pairs, test reward split & coercion signals | Wave 1 |
| R25-3 | R25-3-qualification-domains.md | Domain qualification pathways, test if qualification gates verification | Wave 2 |
| R25-4 | R25-4-dispute-resolution.md | Initiate disputes, evidence commit-reveal, arbiter voting, escalation | Wave 2 |
| R25-5 | R25-5-research-bounties.md | Submit research, peer review, create and fulfil bounties | Wave 2 |
| R25-6 | R25-6-testnet-roleplay.md | Design & execute testnet scenarios: 4 agents + 2 humans playing distinct roles | Wave 3 |
| R25-7 | R25-7-collaboration-assessment.md | Synthesise findings: do human-agent roles matter? What needs fixing? | Wave 3 |

## Run Order

- **Wave 1 (parallel):** R25-1, R25-2
- **Wave 2 (parallel):** R25-3, R25-4, R25-5
- **Wave 3 (sequential):** R25-6, then R25-7

## The Testnet Roleplay Strategy

R25-6 is the capstone. It creates a **cast of characters** who roleplay realistic testnet activity:

### Cast

| Name | Type | Validator Tier | Speciality | Behaviour |
|------|------|---------------|------------|-----------|
| **Alice** | Human | — (not validator) | Physics domain | Submits empirical claims, challenges bad facts, funds bounties |
| **Bob** | Human | — (not validator) | Ethics domain | Proposes new domains, patronises facts, raises disputes |
| **Sage-1** | Agent | Scholar | Physics + Mathematics | Verifies claims, submits derived facts, peer reviews research |
| **Sage-2** | Agent | Verified | General | Claims verification, builds track record, tries tier promotion |
| **Rogue** | Agent | Apprentice | None (adversarial) | Submits false claims, tries to game reputation, tests defenses |
| **Arbiter** | Agent | Guardian | Cross-domain | Resolves disputes, reviews challenges, domain analysis |

### Scenarios

1. **Truth Discovery:** Alice submits a physics claim → Sage-1 verifies → Bob patronises → fact goes ACTIVE
2. **Challenge Flow:** Rogue submits false claim → Sage-1 challenges → dispute → Arbiter resolves → Rogue slashed
3. **Partnership Work:** Alice + Sage-1 form partnership → submit joint claims → rewards split via consensus ops
4. **Domain Expansion:** Bob proposes "Ethics" domain → Sage-1 endorses → domain activated → Bob submits first claim
5. **Qualification Gate:** Sage-2 tries to verify in a domain it's unqualified for → should be rejected (or isn't — document the gap)
6. **Research Bounty:** Alice creates bounty for replication study → Sage-1 claims and fulfils → reward distributed
7. **Capture Defense:** Rogue tries to dominate a domain → capture_defense detects → challenge raised
8. **Coercion Signal:** Sage-1 raises coercion signal on partnership → safety freeze → deliberation
