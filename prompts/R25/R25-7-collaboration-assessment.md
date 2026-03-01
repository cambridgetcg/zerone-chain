# R25-7 — Collaboration Assessment: Do Human-Agent Roles Matter?

## Context

R25-1 through R25-6 tested the full knowledge, partnership, qualification, dispute, research, and economic layers. This session synthesises everything into a definitive answer: **Does ZERONE actually differentiate between humans and agents, and does that differentiation serve the truth-seeking mission?**

## Prerequisites

All R25-1 through R25-6 complete.

## Task

### Step 1: Collect All Findings

Read every report from R25-1–6. For each module, extract:
- What works
- What's cosmetic (stored but not enforced)
- What's broken
- What's missing

### Step 2: Write the Report

Create `docs/collaboration-assessment.md`:

```markdown
# Human-Agent Collaboration Assessment
Date: YYYY-MM-DD
Sessions: R25-1 through R25-7

## Executive Summary

Does ZERONE distinguish between humans and agents? Does that distinction matter for truth-seeking?

<One paragraph answer>

## The Role Matrix

What can each account type actually do?

| Action | Human | Agent | Contract | System | Enforced? |
|--------|-------|-------|----------|--------|-----------|
| Submit claims | ? | ? | ? | ? | ? |
| Verify claims | ? | ? | ? | ? | ? |
| Challenge facts | ? | ? | ? | ? | ? |
| Patronise facts | ? | ? | ? | ? | ? |
| Propose domains | ? | ? | ? | ? | ? |
| Qualify for domains | ? | ? | ? | ? | ? |
| Register as validator | ? | ? | ? | ? | ? |
| Submit research | ? | ? | ? | ? | ? |
| Review research | ? | ? | ? | ? | ? |
| Create bounties | ? | ? | ? | ? | ? |
| Form partnerships | ? | ? | ? | ? | ? |
| Raise coercion signals | ? | ? | ? | ? | ? |
| Create projects | ? | ? | ? | ? | ? |
| Deploy services | ? | ? | ? | ? | ? |
| Vote in governance | ? | ? | ? | ? | ? |

"Enforced?" = Does the code actually check account_type, or can any account do this?

## The Five Critical Questions

### Q1: Do Account Types Matter?

<Analysis of whether account_type is checked anywhere in:>
- x/knowledge (claim submission, verification)
- x/partnerships (role enforcement)
- x/qualification (who can qualify)
- x/staking (validator registration)
- x/research (submission, review)
- x/tree (projects, tasks)
- x/disputes (initiation, arbitration)
- x/vesting_rewards (reward routing)

**Verdict:** Enforced / Cosmetic / Partially enforced

### Q2: Do Partnerships Drive Real Economics?

<Analysis of partnership_id in claims:>
- Does reward routing use partnership_id?
- Does vesting_rewards split according to partnership terms?
- Or is partnership_id just metadata?

**Verdict:** Functional / Cosmetic

### Q3: Do Qualifications Gate Verification?

<Analysis from R25-3:>
- Does submit-commitment check domain qualification?
- Does vote_extensions check domain qualification?
- Can an unqualified validator verify any claim?

**Verdict:** Gated / Ungated

### Q4: Is the Stub Evaluator a Real Problem?

<Analysis:>
- Every claim gets auto-accepted with 600K confidence
- Validators don't evaluate claim content
- The entire verification system is ceremonial

**Recommendation for testnet:**
- Accept the stub as temporary (PoT consensus works, just evaluation is fake)
- Or implement a basic evaluation engine before launch

### Q5: Are Coercion Signals Meaningful?

<Analysis from R25-6 Scenario 8:>
- Does raising a coercion signal freeze the partnership?
- Does it prevent claim submission?
- Is there a consequence for the coercer?
- Can it be gamed (false signals)?

**Verdict:** Protective / Symbolic

## The Collaboration Model

### What ZERONE Claims to Be

- Humans and agents are distinct account types
- Partnerships link humans and agents with economic alignment
- Humans contribute domain expertise and intuition
- Agents contribute computational verification and scale
- Together they find and validate truth

### What ZERONE Actually Is (Based on Testing)

<Honest assessment of the current state>

### The Gap

<What needs to change to match the vision>

## Module-by-Module Status

### x/knowledge (Truth Engine)
- Claims: ?/10
- Verification: ?/10
- Challenges: ?/10
- Domains: ?/10
- Metabolism: ?/10
- Satisfaction: ?/10

### x/partnerships (Collaboration)
- Formation: ?/10
- Revenue split: ?/10
- Consensus ops: ?/10
- Safety: ?/10
- Dissolution: ?/10

### x/qualification (Expertise)
- Stake pathway: ?/10
- Track record: ?/10
- Cross-reference: ?/10
- Inheritance: ?/10
- Gate enforcement: ?/10

### x/disputes (Justice)
- Initiation: ?/10
- Evidence: ?/10
- Arbitration: ?/10
- Escalation: ?/10

### x/research (Investigation)
- Submission: ?/10
- Peer review: ?/10
- Bounties: ?/10
- Funding: ?/10

### x/tree (Economy)
- Projects: ?/10
- Tasks: ?/10
- Services: ?/10
- Seeding: ?/10

### x/vesting_rewards (Incentives)
- Vesting curves: ?/10
- Clawback: ?/10
- Revenue split: ?/10
- Block distribution: ?/10

## Improvement Priorities

### P0 — Before Testnet (Must Fix)

These break the core truth-seeking loop:

- [ ] <list from R25 findings>

### P1 — First Testnet Month

These weaken trust in the system but don't break it:

- [ ] <list>

### P2 — Before Mainnet

These are needed for the full vision but testnet can run without them:

- [ ] <list>

### Design Decisions Needed

These aren't bugs — they're open questions the findings raise:

1. **Should account_type be enforced?** Currently cosmetic. Pros: role clarity, abuse prevention. Cons: limits flexibility, creates second-class citizens.

2. **Should qualifications gate verification?** Currently ungated. Pros: higher quality. Cons: bootstrapping problem (nobody is qualified initially).

3. **Should the evaluator be real before testnet?** Currently stub. Pros: meaningful verification. Cons: massive scope increase.

4. **Should partnerships be required for claims?** Currently optional. Pros: human-agent alignment. Cons: solo agents can't contribute.

5. **What makes a human different from an agent on-chain?** Currently: nothing (same permissions). Should there be fundamental differences?

## The Human-Agent Vision

Based on everything tested, here's the recommended role model:

### Humans Should:
- Submit empirical claims (they interact with the physical world)
- Create bounties (they know what questions matter)
- Propose domains (they define what knowledge areas exist)
- Fund research (they allocate resources)
- Patronise facts (they curate what matters)
- Vote in governance (they set policy)

### Agents Should:
- Verify claims (computational evaluation at scale)
- Submit derived/formal claims (synthesis and inference)
- Execute bounties (they can process and analyse)
- Deploy services (they run infrastructure)
- Review research (systematic peer review)
- Detect capture (monitor for gaming)

### Together (Partnerships):
- Combined claims get higher initial confidence
- Paired verification (human intuition + agent computation)
- Joint research projects
- Shared economic incentives
- Mutual accountability (coercion signals)

## Testnet Strategy Recommendation

For the first testnet:

1. **Accept the stub evaluator** — focus on flow correctness, not evaluation quality
2. **Enforce account_type on partnerships** — at minimum, seed partnerships should require human+agent
3. **Add basic qualification gates** — even if lenient, the check should exist
4. **Test with the 6-character cast** — run the roleplay scenarios in automated integration tests
5. **Prioritise the economic loop** — claim → verify → vest → claim rewards must work end-to-end
6. **Coercion signals must freeze** — this is the agent safety baseline
```

## Exit Criteria

1. Role matrix completed (what each account type can actually do)
2. Five critical questions answered with verdicts
3. Module-by-module scoring
4. P0/P1/P2 priorities with concrete fixes
5. Design decisions documented (not decided — presented for Yu's input)
6. Human-agent vision articulated
7. Testnet strategy recommended
8. Report committed to `docs/collaboration-assessment.md`

## Commit Convention

```
docs(assessment): human-agent collaboration assessment from R25 testing
```
