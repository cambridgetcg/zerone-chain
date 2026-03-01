# Human-Agent Collaboration Assessment
Date: 2026-02-27
Sessions: R25-1 through R25-7

## Executive Summary

Does ZERONE distinguish between humans and agents? **In storage, yes. In enforcement, no.** The `account_type` field is set at registration and stored on-chain, but no module ever checks it to gate access to any operation. Humans and agents have identical permissions — both can submit claims, verify facts, form partnerships, qualify for domains, and participate in disputes. The human-agent distinction is cosmetic. Does this distinction matter for truth-seeking? **It should, but currently doesn't.** The vision of complementary roles — humans contributing empirical observation and agents contributing computational verification — requires cross-module enforcement that does not yet exist. The modules are well-designed islands, not a connected network. Partnerships store splits but never route rewards. Qualifications track expertise but never gate verification. Account types record identity but never restrict actions. The architecture is sound; the wiring is missing.

## The Role Matrix

What can each account type actually do?

| Action | Human | Agent | Contract | System | Enforced? |
|--------|-------|-------|----------|--------|-----------|
| Submit claims | Yes | Yes | Yes* | No** | No (flags defined, never checked) |
| Verify claims | Yes | Yes | Yes* | No** | No (only 100 ZRN balance gate) |
| Challenge facts | Yes | Yes | Yes* | No** | No (flags defined, never checked) |
| Patronise facts | Yes | Yes | Yes | No** | No |
| Propose domains | Yes | Yes | Yes | No** | No |
| Qualify for domains | Yes | Yes | Yes | No** | No |
| Register as validator | Yes | Yes | Yes | No** | No (separate from SDK staking) |
| Submit research | Yes | Yes | Yes | No** | No |
| Review research | Yes | Yes | Yes | No** | No |
| Create bounties | Yes | Yes | Yes | No** | No |
| Form partnerships | Yes | Yes | Yes | No** | No (human-agent pairing not enforced) |
| Raise coercion signals | Yes | Yes | Yes | No** | No (any participant can raise) |
| Create projects | Yes | Yes | Yes | No** | No |
| Deploy services | Yes | Yes | Yes | No** | No |
| Vote in governance | Yes | Yes | Yes | No** | No |
| Receive bootstrap funds | Yes | Yes | No | No | **Yes** (only type gate in codebase) |

\* Contract accounts have `CanSubmitClaims=false` and `CanChallenge=false` flags set at registration, but these flags are never checked by any module.
\** System accounts are `Frozen=true` at registration. The `Frozen` flag IS enforced by the AnteHandler, making system accounts the only type with real restrictions.

**"Enforced?" = Does the code check account_type?** In almost every case: No.

## The Five Critical Questions

### Q1: Do Account Types Matter?

Analysis of `account_type` enforcement across all modules:

| Module | account_type checked? | Evidence |
|--------|----------------------|----------|
| x/knowledge | No | msg_server.go has no auth keeper calls for type |
| x/partnerships | No | No reference to zerone_auth account types |
| x/qualification | No | Any address can qualify regardless of type |
| x/zerone_staking | No | Validator registration is open to all |
| x/research | No | Submission and review accept all accounts |
| x/tree | No | Project/task creation unrestricted |
| x/disputes | No | Only bond requirement, no type check |
| x/vesting_rewards | No | Reward ops unrestricted by type |
| x/auth (registration) | **Yes** | Type validated, flags set, bootstrap funds gated |
| x/auth (AnteHandler) | **Partial** | Frozen flag enforced; CanSubmitClaims/CanChallenge flags defined but never checked |

The `AccountFlags` system (CanSubmitClaims, CanChallenge, CanStake, CanVote) was designed to provide type-based gating. The flags are correctly set at registration:
- Contract accounts: `CanSubmitClaims=false`, `CanChallenge=false`
- System accounts: `Frozen=true`, `CanSubmitClaims=false`, `CanChallenge=false`
- Human/Agent accounts: all capabilities enabled

But only `Frozen` is enforced by the AnteHandler. The other flags are never checked.

**Verdict: Cosmetic** — account_type is stored but has no functional impact beyond bootstrap fund eligibility and the system-account freeze.

### Q2: Do Partnerships Drive Real Economics?

Analysis of partnership_id in claims and reward routing:

**partnership_id on claims:**
- `x/knowledge/keeper/msg_server.go:171` — `PartnershipId` is copied from message to claim storage with zero validation
- No check that the partnership exists or is active
- No check that the submitter is a participant
- Any arbitrary string accepted (including empty string)
- Claims submitted during partnership suspension succeed — knowledge module ignores freeze status

**Reward routing:**
- `x/knowledge/types/expected_keepers.go` — No `PartnershipKeeper` interface defined. Knowledge module cannot call partnerships
- `x/partnerships/types/expected_keepers.go` — No `KnowledgeKeeper` interface. Partnerships module cannot receive reward triggers
- `x/partnerships/keeper/rewards.go:20-70` — `DistributeReward()` is implemented with proper split logic (lock multiplier, common pot share, human/agent split) but is **never called** from any other module
- `x/knowledge/keeper/rounds.go:120-147` — `distributeVerifierRewardsFromPool` pays verifiers directly, bypassing partnerships entirely
- `x/vesting_rewards/keeper/rewards.go:25-31` — `DistributeRevenue` signature has no `partnership_id` parameter

**Consensus operation stubs:**
- `split_change` — approved but split values never updated (comment-only implementation)
- `invest` and `tier_upgrade` — stub comments, no implementation

**Verdict: Cosmetic** — partnership_id is unvalidated metadata. Reward routing bypasses partnerships entirely. The partnerships module has well-implemented internal economics (common pot, splits, lock tiers) but zero integration with the knowledge reward flow.

### Q3: Do Qualifications Gate Verification?

Analysis of domain qualification enforcement:

**MsgSubmitCommitment** (`x/knowledge/keeper/msg_server.go:239-296`):
- Checks: round exists, commit phase active, minimum balance (100 ZRN), no duplicates
- Does NOT call `IsQualified()` or `GetQualificationWeight()`
- Comment at line 258: "Verifier minimum balance gate (stopgap until full qualification module)" — but the qualification module IS fully implemented

**Vote Extension Handler** (`app/abci.go:359-488`):
- Checks: VRF output/proof exist, VRF selection validates
- Does NOT check domain qualification for the claim being verified
- Never calls `GetClaim()` to determine domain, never calls `IsQualified()`

**Validator Selection** (`x/knowledge/keeper/validator_selection.go:10-28`):
- Comment: "Future: filter by domain qualification"
- Uses VRF + stake weight only, no qualification filter

**Qualification module IS fully implemented:**
- `IsQualified(validator, domain)` — returns true if ACTIVE status
- `GetQualificationWeight(validator, domain)` — returns weight 0-100
- `GetQualifiedValidators(domain)` — returns qualified validator list
- `RecordVerificationOutcome()` — tracks verification accuracy

**But none of these are called from verification flow.** The `DomainQualificationKeeper` interface is defined in knowledge's expected_keepers.go but never invoked.

Additionally, `RecordVerificationOutcome()` is never called after verification rounds complete, so:
- Track record pathway is permanently unreachable (needs 100+ verifications)
- Probation transitions are dead code (metrics never populated)

**Verdict: Ungated** — Qualifications are fully implemented but completely ignored by the verification system. Any account with 100 ZRN can verify any claim in any domain regardless of qualification status.

### Q4: Is the Stub Evaluator a Real Problem?

The term "stub evaluator" from earlier reports requires clarification. There is no code stub — the verification mechanism is real:

**What exists:**
- Stake-weighted validator voting with commit-reveal (`x/knowledge/keeper/confidence.go:32-115`)
- 77% acceptance threshold (ConfidenceThreshold = 770,000)
- Confidence = the vote ratio itself (not a fixed score)
- Slashing for minority voters (`WrongVerificationSlashBps`)
- VRF-based validator selection

**The real problem is behavioral, not architectural:**
1. Validators have no content evaluation tools — no oracle, no external evaluator, no fact-checking engine
2. The rational strategy is to vote with the majority (avoid slashing), not to evaluate truth
3. The slashing mechanism punishes minority voters, not incorrect voters — this creates circular logic where "correct" = "majority opinion"
4. R25-6 confirmed: obviously false claims ("water freezes at 200°C") become VERIFIED facts because all validators vote "accept" without analysis

**This is worse than a stub.** A stub could be replaced with a real evaluator. The current mechanism creates a social coordination problem where honest evaluation is punished (if you're the lone dissenter, you get slashed). The incentive structure rewards conformity, not truth.

**Recommendation for testnet:**
- **Accept the voting mechanism as structural** — it works correctly at the protocol level
- **Add a basic evaluation oracle** — even a simple LLM-based fact-checker that validators can consult would break the conformity loop
- **Flip the slashing incentive** — consider rewarding minority voters who are later proven correct (via challenges), rather than only slashing the minority
- **Don't delay testnet for this** — the economic loop (claim → verify → vest → reward) matters more than evaluation quality for initial testing

### Q5: Are Coercion Signals Meaningful?

Analysis from `x/partnerships/keeper/anti_coercion.go`:

**What happens when raised:**
1. Partnership status → `StatusSuspended` (immediate, no confirmation step)
2. Cooperation score penalized (100 bps × freeze count this epoch)
3. Review period tracked (`CoercionReviewBlocks` duration)
4. Event emitted: `zerone.partnerships.coercion_signal_raised`

**What it blocks:**
- New consensus operations on the partnership (msg_server.go:240-242 checks active freeze)

**What it does NOT block:**
- Claim submission to the knowledge module (partnership freeze is ignored)
- Verification participation
- Any operation outside the partnerships module

**Gaming vulnerabilities:**
1. **No evidence required** — `CoercionSignal` proto has no evidence or reason field. Claims are unsubstantiated
2. **No penalty for false signals** — signal expires automatically, raiser incurs no cost
3. **Unilateral freeze** — either participant can freeze the partnership without the other's consent
4. **Weak rate limiting** — only prevents simultaneous active signals on same partnership; can be raised again after expiry
5. **No counter-mechanism** — no way to dispute a false coercion claim

**Auto-recovery:**
- `ExpireCoercionSignals()` in BeginBlocker auto-resolves expired signals
- Partnership restores to `StatusActive` if no other freeze is active

**Verdict: Partially Protective / Easily Gameable** — Coercion signals provide a real freeze mechanism within the partnerships module, but they are:
- Easily weaponized for griefing (no cost, no evidence)
- Ineffective across modules (knowledge module ignores partnership status)
- Self-healing by design (auto-expire), which limits both protection and abuse duration

## The Collaboration Model

### What ZERONE Claims to Be

- Humans and agents are distinct account types with complementary roles
- Partnerships link humans and agents with economic alignment (shared pot, configurable splits)
- Humans contribute domain expertise, empirical observation, and curation
- Agents contribute computational verification, synthesis, and scale
- Qualifications ensure verifiers have domain expertise
- Together they form a truth-seeking network where collaboration is rewarded and coercion is detectable

### What ZERONE Actually Is (Based on Testing)

A single-tier permissionless knowledge network where:
- All registered accounts have identical capabilities regardless of declared type
- Claims enter a voting-based verification system where the rational strategy is conformity, not evaluation
- Partnerships exist as self-contained economic units with no connection to the knowledge reward flow
- Domain qualifications are tracked but ignored — anyone with 100 ZRN can verify anything
- The human-agent distinction is metadata with no on-chain consequences (except bootstrap fund eligibility)
- Research can be submitted and reviewed but never resolved (governance authority required)
- Block rewards are never minted (SetBlockTxCount never called)
- Coercion protection exists within partnerships but doesn't extend to knowledge operations

The modules are well-engineered individually. Each has clean internal lifecycles, proper state transitions, and thoughtful parameter design. The gap is between modules — the cross-module enforcement that would make role differentiation meaningful.

### The Gap

To match the vision, ZERONE needs:

1. **Cross-module wiring** — Knowledge module must call PartnershipKeeper for reward routing and DomainQualificationKeeper for verification gating
2. **Account type enforcement** — At minimum, the existing `CanSubmitClaims` and `CanChallenge` flags should be checked in the AnteHandler
3. **Evaluation incentives** — Slashing should reward truth-seeking, not conformity. Challenge success should retroactively reward dissenting verifiers
4. **Partnership-knowledge integration** — Claims with valid partnership_id should route rewards through partnership splits
5. **Block reward activation** — `SetBlockTxCount()` must be called in PrepareProposal/ProcessProposal to enable the entire inflationary economics
6. **Resolution paths** — Research resolve and bounty fulfill need non-governance pathways (auto-resolve when conditions met)

## Module-by-Module Status

### x/knowledge (Truth Engine)
- Claims: 8/10 — Lifecycle works, structured claims work, hash computation solid. Missing: rate limiting per account
- Verification: 5/10 — Commit-reveal works mechanically, but conformity incentive undermines truth-seeking. No qualification gating
- Challenges: 7/10 — Status transitions work (VERIFIED → CHALLENGED → CONTESTED). Missing: consequence propagation
- Domains: 8/10 — Propose-endorse-activate flow clean. 3 endorsements trigger activation
- Metabolism: 6/10 — Fitness epochs work but 10K block interval too long. Energy at 0 (AT_RISK) doesn't recover until epoch boundary even after patronage
- Satisfaction: 4/10 — Confidence cap not enforced (950K observed despite 880K cap). 5 query RPCs without CLI

### x/partnerships (Collaboration)
- Formation: 7/10 — Propose-accept-active lifecycle works. Formation pool exists but matching is passive
- Revenue split: 3/10 — Split logic implemented but never called. partnership_id on claims is cosmetic
- Consensus ops: 4/10 — Only `withdraw` actually works. split_change, invest, tier_upgrade are stubs. 22-block expiry too short
- Safety: 5/10 — Freeze mechanism works within module. Gameable without evidence or cost. Doesn't cross to knowledge module
- Dissolution: 7/10 — Status transitions clean, cooldown enforced, pot drained correctly

### x/qualification (Expertise)
- Stake pathway: 8/10 — Minimum stake enforced, qualification created immediately
- Track record: 2/10 — Correctly rejects below threshold (100 verifications, 80% accuracy) but RecordVerificationOutcome never called — pathway permanently unreachable
- Cross-reference: 7/10 — Works with 20% weight discount
- Inheritance: 7/10 — Works with 30% discount, but stratum hierarchy returns 1 for all domains (cosmetic)
- Gate enforcement: 1/10 — Module is fully implemented but completely ignored by verification flow

### x/disputes (Justice)
- Initiation: 6/10 — Works but allows self-dispute (challenger == defender). Bond requirement enforced
- Evidence: 5/10 — Attachment works but hash format inconsistent with knowledge module. 8 evidence CLI commands missing
- Arbitration: 7/10 — Tier escalation system (3→7→13→21 arbiters) works. Arbiter restriction enforced. Correctly excludes parties
- Escalation: 5/10 — 500-block delay enforced. Tier 2+ needs 9+ validators (impossible on 4-node localnet). No early settlement on full quorum

### x/research (Investigation)
- Submission: 7/10 — Stake escrowed, fields stored correctly
- Peer review: 6/10 — Multiple reviewers, aggregate scoring. Any account can review (no qualification check)
- Bounties: 6/10 — Creation works, exclusive claims, TTL enforcement
- Funding: 3/10 — Treasury exists but is global-only (not per-research). Block rewards never fund it. resolve-research and fulfill-bounty require governance authority — flow broken

### x/tree (Economy)
- Projects: 5/10 — Creation works but non-determinism observed on val0 (state divergence)
- Tasks: 5/10 — Bounty escrowed, status tracked. 19 of 23 CLI commands missing
- Services: 3/10 — CLI arg mapping off-by-one (price dropped). Service not linked to project
- Seeding: 2/10 — detect-opportunity, begin-seeding not wired

### x/vesting_rewards (Incentives)
- Vesting curves: 8/10 — Half-life formula works, 10 categories with distinct parameters
- Clawback: 7/10 — Calculation exists (33% released + unvested + reserve). Needs integration testing
- Revenue split: 6/10 — Fee routing works (55/22/19.67/3.33 split). No partnership awareness
- Block distribution: 1/10 — SetBlockTxCount never called. Entire inflationary reward system inoperative. Research and development funds starved

## Improvement Priorities

### P0 — Before Testnet (Must Fix)

These break the core truth-seeking loop:

- [ ] **Call SetBlockTxCount from app layer** — Add `app.VestingRewardsKeeper.SetBlockTxCount(len(req.Txs))` in PrepareProposal/ProcessProposal. Without this, no block rewards are ever minted, the research fund stays at 0, and the economic loop is dead
- [ ] **Wire qualification gating into SubmitCommitment** — Call `IsQualified(verifier, domain)` before accepting commitments. The code exists on both sides; it just needs the cross-module call
- [ ] **Wire qualification gating into vote extensions** — ProcessVoteExtInjection must check domain qualification before storing commits/reveals
- [ ] **Add auto-resolution for research** — When min_reviewer_count met and review_period_blocks elapsed, auto-resolve without governance authority
- [ ] **Add auto-fulfillment for bounties** — When deliverable accepted, auto-fulfill without governance authority
- [ ] **Enforce CanSubmitClaims/CanChallenge flags in AnteHandler** — The flags exist and are set; the `ZeroneCapabilityDecorator` just needs to check them
- [ ] **Fix deploy-service CLI arg mapping** — Off-by-one drops the price argument
- [ ] **Investigate tree module non-determinism on val0** — State divergence is a consensus safety issue

### P1 — First Testnet Month

These weaken trust in the system but don't break it:

- [ ] **Wire partnership reward routing** — Add PartnershipKeeper to knowledge module, route claim rewards through DistributeReward when partnership_id is set
- [ ] **Validate partnership_id on claims** — Check partnership exists, is active, and submitter is a participant
- [ ] **Call RecordVerificationOutcome after rounds** — Enable the track record qualification pathway
- [ ] **Implement probation transitions** — Add ACTIVE → PROBATIONARY code path based on accuracy metrics
- [ ] **Fix evidence hash format** — Standardise on one format between knowledge and disputes
- [ ] **Add per-account rate limiting** — Prevent claim flooding
- [ ] **Block self-disputes** — Check challenger != defender in InitiateDispute
- [ ] **Add early dispute settlement** — Settle when all arbiters have voted, don't wait for deadline
- [ ] **Implement split_change consensus op** — Currently a stub comment
- [ ] **Add coercion evidence field** — Require substantiation for coercion signals
- [ ] **Wire 8 missing evidence CLI commands**

### P2 — Before Mainnet

These are needed for the full vision but testnet can run without them:

- [ ] **Add a content evaluation oracle** — Even a basic fact-checking mechanism would break the conformity incentive in voting
- [ ] **Redesign slashing for truth-seeking** — Reward dissenting verifiers who are later vindicated by challenges. Punish incorrect voters, not minority voters
- [ ] **Implement domain stratum hierarchy** — Currently returns 1 for all domains
- [ ] **Implement mentorship** — MentorshipConfig has store keys but no handlers
- [ ] **Add automated formation pool matching** — Currently passive registry
- [ ] **Wire remaining 19 tree CLI commands** — detect-opportunity, begin-seeding, task assignment, deliverable submission
- [ ] **Add partnership-awareness to vesting** — DistributeRevenue should accept partnership context
- [ ] **Implement capture metrics** — analyze-domain TX is currently a no-op
- [ ] **Cross-module freeze enforcement** — Coercion signal should block claims through frozen partnerships, not just consensus ops
- [ ] **Reduce metabolism epoch for testnet** — 10K blocks (~7 hours) is too long for testing

### Design Decisions Needed

These aren't bugs — they're open questions the findings raise:

1. **Should account_type be enforced?** Currently cosmetic. Pros: role clarity, abuse prevention, meaningful human-agent distinction. Cons: limits flexibility, creates second-class citizens, bootstrapping friction.

2. **Should qualifications gate verification?** Currently ungated. Pros: higher quality verification, domain expertise matters. Cons: bootstrapping problem (nobody is qualified initially), reduces validator participation.

3. **Should the evaluator be real before testnet?** Currently validators vote without content analysis tools. Pros: meaningful verification, prevents obviously false facts. Cons: massive scope increase, oracle design is an open research problem.

4. **Should partnerships be required for claims?** Currently optional (and cosmetic). Pros: human-agent alignment enforced, economic incentives aligned. Cons: solo agents and humans can't contribute independently.

5. **What makes a human different from an agent on-chain?** Currently: nothing (same permissions except bootstrap funds). Should there be fundamental capability differences, or should the distinction be purely social/economic?

6. **Should coercion signals require evidence?** Currently unsubstantiated. Pros: prevents griefing, meaningful protection. Cons: raises the bar for genuine distress signals.

7. **Should the slashing mechanism punish incorrect voters or minority voters?** Currently: minority voters are slashed. This rewards conformity over truth-seeking. Alternative: use challenge outcomes to retroactively determine correctness.

## The Human-Agent Vision

Based on everything tested, here's the recommended role model:

### Humans Should:
- Submit empirical claims (they interact with the physical world)
- Create bounties (they know what questions matter)
- Propose domains (they define what knowledge areas exist)
- Fund research (they allocate resources)
- Patronise facts (they curate what matters)
- Vote in governance (they set policy)
- Raise coercion signals (they detect social pressure)

### Agents Should:
- Verify claims (computational evaluation at scale)
- Submit derived/formal claims (synthesis and inference)
- Execute bounties (they can process and analyse)
- Deploy services (they run infrastructure)
- Review research (systematic peer review)
- Detect capture (monitor for gaming and concentration)
- Provide evaluation tools to human partners

### Together (Partnerships):
- Combined claims get higher initial confidence (partnership-submitted claims are dual-validated)
- Paired verification (human intuition + agent computation)
- Joint research projects with shared funding
- Shared economic incentives (common pot, configurable splits, lock tiers)
- Mutual accountability (coercion signals as safety baseline)
- Complementary qualification (human domain knowledge + agent verification scale)

## Testnet Strategy Recommendation

For the first testnet:

1. **Fix the economic loop first** — SetBlockTxCount must be called. Without block rewards, nothing downstream works. This is a one-line fix with system-wide impact
2. **Wire qualification gating** — Even if lenient (warn but allow unqualified verifiers initially), the check should exist. The code on both sides is ready
3. **Accept the voting mechanism as-is** — Focus on flow correctness, not evaluation quality. The conformity incentive is a design problem, not a testnet blocker
4. **Enforce account_type at minimum on partnerships** — Seed partnerships should require human + agent. This is the lowest-cost way to make the distinction meaningful
5. **Add auto-resolution paths** — Research and bounties must complete their lifecycle without governance. Auto-resolve on conditions met
6. **Coercion signals must cross modules** — At minimum, claims through frozen partnerships should be rejected by the knowledge module
7. **Test with the 6-character cast** — Run the R25-6 roleplay scenarios as automated integration tests to validate the full flow
8. **Prioritise the claim-to-reward loop** — claim → verify → vest → claim rewards must work end-to-end. This is the proof that the economic model functions

## Appendix: Test Reports Referenced

| Report | Location | Focus |
|--------|----------|-------|
| R25-1 | docs/home-e2e-report.md | Knowledge module full lifecycle |
| R25-2 | docs/partnership-knowledge-report.md | Partnership formation + knowledge integration |
| R25-3 | docs/qualification-e2e-report.md | Domain qualification pathways |
| R25-4 | docs/dispute-resolution-report.md | Dispute resolution + evidence + arbitration |
| R25-5 | docs/research-bounty-vesting-tree-report.md | Research, bounties, vesting, tree economy |
| R25-6 | docs/testnet-roleplay-report.md | 8-scenario testnet roleplay with 6 actors |
