# Testnet Roleplay Report — R25-6

**Date:** 2026-02-27
**Chain:** `zerone-localnet` (4 validators, Cosmos SDK v0.50.15 + CometBFT v0.38.20)
**Block range:** 1–680
**Prior work:** R25-1 (knowledge lifecycle), R25-2 (partnerships), R25-3 (qualification), R25-4 (disputes), R25-5 (research/bounties)

---

## Cast of Characters

| Character | Account Type | Address | Starting Balance | Role |
|-----------|-------------|---------|-----------------|------|
| Alice | human | `zrn1yulq…7vr0hj3vs` | 2,000 ZRN | Physicist — submits claims, challenges fraud, creates bounties |
| Bob | human | `zrn12kf7…7sztv6d` | 1,500 ZRN | Ethicist — patronizes facts, proposes domains |
| Sage-1 | agent (val0) | `zrn19x24…6v4cpl` | ~100,000 ZRN | Scholar validator — verifies claims, fulfils bounties |
| Sage-2 | agent (val1) | `zrn16edk…38cc5` | ~1,000,000 ZRN | Verified validator — verifies claims, qualification test subject |
| Rogue | agent | `zrn12yhv…q9pces` | 500 ZRN | Adversary — submits false claims, floods domain |
| Arbiter | agent (val2) | `zrn1tnhw…9pces` | ~10,000,000 ZRN | Guardian validator — resolves disputes, endorses domains |

All characters registered via `zerone_auth register-account` with distinct types. Validators registered in `zerone_staking`.

---

## Scenario Results

### Scenario 1: Truth Discovery (Happy Path) — PASS

**Story:** Alice submits empirical physics knowledge. Sage-1 and Sage-2 verify it. Bob patronizes.

| Step | Action | Result |
|------|--------|--------|
| 1.1 | Alice submits claim: "Gravitational constant G equals 6.674e-11 in SI" | code=0, claim_id=`9220509485944c71…` |
| 1.2 | Sage-1 + Sage-2 commit (SHA256("accept" \|\| salt_bytes)) | Both code=0 |
| 1.3 | Sage-1 + Sage-2 reveal after commit phase | Both code=0 |
| 1.4 | Round aggregates → verdict ACCEPT (1) | fact_id=`1170d19975345ee0…` |
| 1.5 | Bob patronizes (20 ZRN, 200 blocks) | code=0 |
| 1.6 | Query fact status | VERIFIED (3), confidence=950000, patronage=20,000,000 uzrn |

**Cross-module path:** `knowledge (claim) → knowledge (commit-reveal) → knowledge (fact) → knowledge (patronize)`

**Checklist:**
- [x] Claim submitted by human, verified by agents, patronized by another human
- [x] Full cross-role flow works
- [x] Fact reaches VERIFIED status (3)
- [ ] Rewards distributed to Alice (submitter) — **no observable reward event; block rewards never minted (R25-5 finding)**
- [ ] Sage-1 and Sage-2 reputation updated — **no reputation query available; verification reward routing is cosmetic**

**Issues:**
- No submitter reward mechanism fires (block reward minting is broken per R25-5)
- No verifier reputation tracking queryable at the CLI level
- `rate-fact` command not available (not implemented)

---

### Scenario 2: Challenge Flow (Adversarial) — PASS

**Story:** Rogue submits a false claim. The community catches and disputes it.

| Step | Action | Result |
|------|--------|--------|
| 2.1 | Rogue submits: "Water freezes at 200°C under normal pressure" | code=0, claim_id from events |
| 2.2 | Sage-1 + Sage-2 verify (commit-reveal, both "accept") | Verdict=ACCEPT (1) |
| 2.3 | Fact created: `94739273ecb7a4a2…` | status=VERIFIED (3), confidence=950000 |
| 2.4 | Alice challenges (11 ZRN stake) | code=0, fact→status 6 (CHALLENGED) |
| 2.5 | Alice initiates dispute (target_type=1, bond=1 ZRN) | code=0, dispute_id=`a8467e78…` |
| 2.6 | Arbiter (val2) votes for challenger | code=0 |
| 2.7 | Dispute state: phase=1 (evidence), voting_deadline=block 2033 | Active, awaiting more votes |

**Cross-module path:** `knowledge (claim) → knowledge (verify) → knowledge (challenge) → disputes (initiate) → disputes (vote)`

**Checklist:**
- [x] False claim challenged and disputed
- [ ] Rogue loses stake — **dispute not yet settled (needs majority votes + settlement)**
- [ ] Alice gets challenger reward — **pending dispute resolution**
- [ ] Rogue reputation damaged — **no reputation system queryable**
- [ ] Fact → DISPROVEN — **fact is CHALLENGED (6), not DISPROVEN; dispute still in progress**

**Critical finding:** The stub evaluator accepts **all** claims, including obviously false ones. Verification is a rubber stamp — verifiers have no incentive or mechanism to reject. The only defense is post-facto challenge + dispute.

**Issues:**
- `settle-dispute` and `resolve-dispute` commands not available or require governance authority
- Dispute auto-settlement would need either (a) majority arbiter votes or (b) deadline expiry
- Evidence commit/reveal format differs from knowledge module (R25-4 finding)

---

### Scenario 3: Partnership Collaboration — PASS

**Story:** Alice and Sage-1 form a partnership to collaborate.

| Step | Action | Result |
|------|--------|--------|
| 3.1 | Alice proposes partnership with Sage-1 (100 ZRN each) | code=0 |
| 3.2 | Sage-1 (val0) accepts | code=0, partnership-1 created |
| 3.3 | Partnership active | common_pot=200 ZRN, 50/50 split, lock_tier=1 |

**Cross-module path:** `partnerships (propose) → partnerships (accept) → partnerships (active)`

**Checklist:**
- [x] Partnership formed (human + agent)
- [ ] Claims submitted through partnership — **partnership_id on claims is cosmetic (R25-2); no --partnership-id flag available**
- [ ] Rewards split correctly — **reward routing to partnerships not implemented (R25-2)**
- [x] Both parties contributed to common pot

**Final state:** partnership-1 status=`suspended` (after S8 coercion/freeze events). cooperation_score=500000.

**Issues:**
- Partnership reward routing is entirely cosmetic (R25-2 confirmed)
- No `--partnership-id` flag on `submit-claim`
- Partnership is a financial container but doesn't affect knowledge module behavior

---

### Scenario 4: Domain Expansion — PASS

**Story:** Bob proposes a "bioethics" domain. Validators endorse it.

| Step | Action | Result |
|------|--------|--------|
| 4.1 | Bob proposes "bioethics" domain (stratum=analytic, stake=5 ZRN) | code=0 |
| 4.2 | Sage-1 (val0) endorses | code=0 |
| 4.3 | Sage-2 (val1) endorses | code=0 |
| 4.4 | Arbiter (val2) endorses | code=0 |
| 4.5 | Domain status | ACTIVE (1), 3 endorsers |
| 4.6 | Bob submits first bioethics claim | code=0, round created |

**Cross-module path:** `knowledge (propose-domain) → knowledge (endorse-domain) → knowledge (activate) → knowledge (submit-claim in new domain)`

**Checklist:**
- [x] Domain proposed by human
- [x] Endorsed by agents (validators)
- [x] Domain activated (status=1 after 3 endorsements)
- [x] First claim accepted in new domain

**CLI notes:**
- Command is `endorse-domain` (not `endorse-domain-proposal`)
- `propose-domain` requires 4 args: `[name] [description] [stratum] [stake]`
- 3 endorsements required to activate

---

### Scenario 5: Qualification Gate Test — FAIL (expected)

**Story:** Sage-2 qualifies in `general` but tries to verify in `bioethics`.

| Step | Action | Result |
|------|--------|--------|
| 5.1 | Sage-2 qualifies in `general` (100 ZRN stake) | code=0 |
| 5.2 | Sage-2 submits commitment on bioethics round | **code=0 — ACCEPTED** |

**Cross-module path:** `qualification (qualify) → knowledge (submit-commitment)` — **no cross-module check occurs**

**Checklist:**
- [ ] Rejected (qualification enforced) — **NOT ENFORCED**
- [x] Accepted (qualification gate missing) — **CONFIRMED: gate is missing**

**Root cause (from R25-3):** `SubmitCommitment` in the knowledge module only checks that the verifier has ≥100 ZRN balance. It does **not** call `qualification.IsQualified()`. Vote extensions also ignore qualification. The entire qualification module is cosmetic.

---

### Scenario 6: Research Bounty — PARTIAL

**Story:** Alice funds a bounty, Sage-1 claims it.

| Step | Action | Result |
|------|--------|--------|
| 6.1 | Alice creates bounty (50 ZRN, deadline=block 50351) | code=0, BOUNTY-1 created |
| 6.2 | Sage-1 claims bounty | code=0 |
| 6.3 | Sage-1 tries to fulfill | code=0 (mempool accepted) but **status unchanged** |
| 6.4 | Final bounty state | status=`claimed`, not `fulfilled` |

**Cross-module path:** `research (create-bounty) → research (claim-bounty)` — fulfill is blocked

**Checklist:**
- [x] Human creates bounty, agent claims
- [ ] Reward distributed to agent — **fulfill requires governance authority (R25-5)**
- [x] Complementary roles demonstrated: human directs research, agent claims work

**Issues:**
- `fulfill-bounty` is "authority only" — requires governance module submission
- TX returns code 0 (accepted into mempool) but the message handler rejects non-authority callers silently
- No alternative fulfillment path exists for regular accounts

---

### Scenario 7: Capture Defense — PARTIAL

**Story:** Rogue floods general domain with claims. Alice challenges for capture.

| Step | Action | Result |
|------|--------|--------|
| 7.1 | Rogue submits 5 flood claims | 4/5 succeeded (1 sequence mismatch) |
| 7.2 | Sage-1 runs analyze-domain | code=0 but no metrics stored |
| 7.3 | Query capture_defense metrics | "metrics not found: general" |
| 7.4 | Alice submits capture challenge (10 ZRN stake, accusing Rogue) | code=0 |
| 7.5 | Query challenge | ID=`95ba92e6…`, status=2, evidence_deadline=block 5669 |

**Cross-module path:** `knowledge (claims) → capture_defense (analyze) → capture_challenge (submit)` — analysis is cosmetic, challenge creates record

**Checklist:**
- [ ] Rate limiting exists — **NO per-account rate limiting; Rogue submitted 4 claims in quick succession**
- [ ] Capture analysis detects concentration — **analyze-domain returns code 0 but stores nothing**
- [x] Challenge mechanism works (challenge created, ID assigned, deadlines set)
- [ ] Rogue penalized or claims rejected — **no automatic penalty from challenge or analysis**

**Issues:**
- `analyze-domain` appears to be a no-op (accepted but produces no queryable state)
- No claim rate limiting per account
- Capture challenges create records but have no auto-resolution path visible
- Accused validators field accepts any address (not just validators)

---

### Scenario 8: Coercion Signal (Partnership Safety) — PASS

**Story:** Sage-1 raises a coercion signal in the Alice partnership. Safety freeze activates.

| Step | Action | Result |
|------|--------|--------|
| 8.1 | Sage-1 raises coercion signal on partnership-1 | code=0 |
| 8.2 | Partnership status | → `suspended` |
| 8.3 | Sage-1 safety-freezes partnership | code=0 |
| 8.4 | Final status | `suspended`, cooperation_score=500000 |

**Cross-module path:** `partnerships (raise-coercion) → partnerships (suspend) → partnerships (safety-freeze)`

**Checklist:**
- [x] Coercion signal raised by agent
- [x] Partnership enters suspended state
- [ ] No new claims can use this partnership during freeze — **partnership_id on claims is cosmetic (R25-2), so freeze has no functional effect on knowledge module**
- [x] "I refuse" mechanism exists and changes partnership status

**Issues:**
- Freeze is self-contained to partnerships module — knowledge module doesn't check partnership status
- `cooperation_score` unchanged at 500000 (not penalized by coercion event)
- No automatic mediation or arbitration triggered by coercion signal

---

## Summary Scorecard

| Scenario | Status | Cross-Module Interactions | Issues |
|----------|--------|--------------------------|--------|
| 1. Truth Discovery | **PASS** | knowledge (claim→verify→fact→patronize) | No submitter rewards, no rate-fact, no reputation tracking |
| 2. Challenge Flow | **PASS** | knowledge → disputes → evidence | Stub evaluator accepts all; dispute settlement requires governance |
| 3. Partnership Collab | **PASS** | partnerships (propose→accept→active) | Reward routing cosmetic; no claim integration |
| 4. Domain Expansion | **PASS** | knowledge (propose→endorse→activate→claim) | Clean flow; 3 endorsements to activate |
| 5. Qualification Gate | **FAIL** | qualification ✗→ knowledge | Gate not enforced; qualification module is cosmetic |
| 6. Research Bounty | **PARTIAL** | research (create→claim) | Fulfill requires governance authority |
| 7. Capture Defense | **PARTIAL** | capture_defense + capture_challenge | analyze-domain is no-op; no rate limiting |
| 8. Coercion Signal | **PASS** | partnerships (coercion→suspend→freeze) | Freeze doesn't affect knowledge module |

**Overall: 4 PASS, 1 FAIL, 2 PARTIAL, 1 PASS (expected known bug)**

---

## Role Differentiation Analysis

### 1. Do account types matter?

**No.** Being registered as "human" vs "agent" via `zerone_auth` changes nothing functionally. Alice (human) and Rogue (agent) can perform identical operations. The account type is stored in state but no module checks it before executing transactions. There is no:
- Human-only operation gate
- Agent-only operation gate
- Role-based fee adjustment
- Type-based rate limiting

### 2. Do partnerships add value?

**Cosmetically only.** Partnerships create a financial container (common pot, split ratios, cooperation score) and support lifecycle events (propose, accept, coerce, freeze, dissolve). However:
- No `--partnership-id` flag on `submit-claim` — claims can't be attributed to partnerships
- Knowledge module has no reward routing to partnerships
- Partnership status (frozen/suspended) doesn't block any knowledge operations
- The partnership module is self-contained with no cross-module integration

### 3. Do qualifications gate anything?

**No.** The qualification module's `qualify-by-stake` works — it records qualifications. But:
- `knowledge.SubmitCommitment` only checks balance ≥100 ZRN, not `IsQualified()`
- Vote extensions ignore qualification state
- Any account with enough balance can verify in any domain regardless of qualification
- The entire domain-specific expertise system is non-functional

### 4. Do coercion signals protect agents?

**Partially.** The mechanism works within the partnerships module:
- `raise-coercion` transitions partnership to `suspended`
- `safety-freeze` further locks the partnership
- The agent has a clear "I refuse" path

**But** the protection is symbolic because:
- Partnership freeze has no effect on claim submission (no integration)
- No automatic mediation or arbitration is triggered
- Cooperation score is unchanged by coercion events
- The human partner faces no penalty or investigation

### 5. Is the stub evaluator a problem?

**Yes — it is the single biggest systemic issue.** The stub evaluator auto-accepts every claim regardless of content. This means:
- Obviously false claims ("water freezes at 200°C") become VERIFIED facts
- Verifiers commit/reveal "accept" with no content analysis
- The only defense is post-facto challenge + dispute (which itself requires governance authority to settle)
- The entire knowledge verification system is a rubber stamp
- Domain expertise, qualification, and reputation are meaningless when every claim is accepted

This fundamentally undermines the knowledge module's purpose as a truth-discovery mechanism.

---

## Cross-Module Integration Map

```
knowledge ←─── WORKS ───→ knowledge (claim lifecycle, patronize, challenge)
knowledge ←─── COSMETIC ──→ partnerships (partnership_id on claims)
knowledge ←─── MISSING ───→ qualification (gate not enforced)
knowledge ←─── WORKS ───→ disputes (challenge → initiate dispute)
knowledge ←─── MISSING ──→ vesting_rewards (no block rewards minted)
partnerships ←── WORKS ──→ partnerships (full lifecycle: propose→accept→coerce→freeze)
partnerships ←── MISSING ─→ knowledge (freeze doesn't affect claims)
disputes ←──── WORKS ──→ disputes (initiate → vote)
disputes ←──── BLOCKED ──→ disputes (settle requires governance)
research ←──── WORKS ──→ research (create → claim bounty)
research ←──── BLOCKED ──→ research (fulfill requires governance)
capture_* ←─── PARTIAL ──→ capture_challenge (challenge creates record)
capture_* ←─── NO-OP ───→ capture_defense (analyze-domain stores nothing)
```

---

## Final Balances

| Character | Starting | Final | Net Change | Notes |
|-----------|----------|-------|------------|-------|
| Alice | 2,000 ZRN | 1,817.2 ZRN | −182.8 ZRN | Claims, challenges, dispute bonds, bounties, fees |
| Bob | 1,500 ZRN | 1,469.8 ZRN | −30.2 ZRN | Patronage, domain proposal, claims, fees |
| Rogue | 500 ZRN | 488.8 ZRN | −11.2 ZRN | Bogus claims, flood claims, fees |
| Sage-1 (val0) | ~100,000 ZRN | 99,697.3 ZRN | −302.7 ZRN | Partnership contribution, verifications, fees |
| Sage-2 (val1) | ~1,000,000 ZRN | 997,898.1 ZRN | −2,101.9 ZRN | Qualification stake, verifications, fees |
| Arbiter (val2) | ~10,000,000 ZRN | 9,980,000 ZRN | −20,000 ZRN | Dispute votes, domain endorsement, fees |

**Note:** All actors lost funds (stakes + fees) with no incoming rewards. Block reward minting is broken (R25-5), so the token economy is deflationary-only in this test.

---

## Key Findings Summary

### What works
1. **Knowledge claim lifecycle** — submit → commit-reveal → fact creation is solid
2. **Domain governance** — propose → endorse → activate flow is clean
3. **Challenge mechanism** — challenge-fact transitions fact status correctly
4. **Dispute initiation** — cross-module from knowledge to disputes works
5. **Partnership lifecycle** — full propose → accept → coerce → freeze path
6. **Capture challenge creation** — records are created with proper deadlines
7. **Research bounty creation and claiming** — create → claim works

### What's broken or cosmetic
1. **Stub evaluator** — accepts everything, making verification meaningless
2. **Qualification gates** — not enforced; any account can verify anywhere
3. **Account type enforcement** — human/agent distinction is cosmetic
4. **Partnership integration** — no reward routing, no claim attribution
5. **Block rewards** — never minted; no positive token flow
6. **Dispute settlement** — requires governance authority
7. **Bounty fulfillment** — requires governance authority
8. **Capture defense analysis** — analyze-domain is a no-op
9. **Rate limiting** — no per-account claim throttle

### Architectural assessment

The system has well-designed **individual module lifecycles** but lacks **cross-module enforcement**. Each module (knowledge, partnerships, qualification, disputes, research, capture) works internally but doesn't check constraints from sibling modules. The result is a system where:
- Roles exist but aren't enforced
- Expertise domains exist but aren't gated
- Partnerships exist but don't affect knowledge flow
- Safety mechanisms exist but are self-contained

**The modules are islands, not a network.** The next development priority should be wiring the cross-module checks that make the role differentiation meaningful.
