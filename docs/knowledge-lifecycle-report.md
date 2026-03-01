# Knowledge Lifecycle E2E Report — R25-1

**Date:** 2026-02-26
**Chain:** zerone-localnet (4 validators, block time ~2.5s)
**Binary:** build/zeroned (Cosmos SDK v0.50.15)
**Test Script:** `scripts/knowledge-lifecycle-test.sh`

## Summary

| # | Step | Status | Notes |
|---|------|--------|-------|
| 1 | Inspect State | PASS | 18 domains, 0 genesis facts, params loaded |
| 2 | Simple Claim | PASS | claim_id=c2fc69d1..., round auto-created |
| 3 | Structured Claim | PASS | structure stored, canonical_form auto-derived |
| 4 | Commit-Reveal | PASS | Full cycle: commit→reveal→aggregation→ACCEPT |
| 5 | Challenge Fact | PASS | Status: VERIFIED(3)→CHALLENGED(6) |
| 6 | Contradiction | PASS | Status→CONTESTED(5), counter-claim created |
| 7 | Patronise Fact | PASS | patronage_amount=10M, expiry_block set |
| 8 | Propose Domain | PASS | quantum_physics → PROPOSED(3) |
| 9 | Endorse Domain | PASS | 3 endorsements → ACTIVE(1), 19 total domains |
| 10 | Report Demand | STUB | No CLI (authority-only gRPC) |
| 11 | Rate Fact | STUB | No CLI (requires query receipt) |
| 12 | Common Knowledge | STUB | No CLI (authority-only); 70 genesis entries |
| 13 | Metabolism | PASS | Baseline: energy=5000, cap=10000 |
| 14 | Knowledge Graph | PASS | 10/10 queries executed |

**Totals:** 11 passed, 0 failed, 3 stubbed (no CLI)

---

## Detailed Results

### Step 1: Inspect Current Knowledge State
**Status:** PASS
**Result:**
- Genesis facts: **0** (axiom injection not run on this localnet instance)
- Domains: **18** active — agent_purpose, agent_rights, biology, chemistry, computer_science, cosmology, economics, ethics, general, information_theory, linguistics, logic, mathematics, philosophy, physics, psychology, sociology, theology
- All domains have status=1 (ACTIVE) with null strata
- Bootstrap fund: 22,222,000,000 uzrn (22,222 ZRN)
- Key params: commit_phase=10 blocks, reveal_phase=10 blocks, aggregation=5 blocks, min_review_fee=100,000 uzrn, min_challenge_stake=11,000,000 uzrn
- Metabolism: initial_energy=5000, energy_cap=10000, at_risk_epochs=5, fitness_epoch_blocks=10,000

**Design Question:** Genesis axioms (expected 777) were not loaded — the localnet `start` command doesn't include axiom injection. This is expected (use `init` + inject + `boot` for axioms).

---

### Step 2: Submit a Simple Claim
**Status:** PASS
**Tx Hash:** `0337321E0467596219402A8284B593E94A47D045C55B91CE15420D954D86D67D`
**Result:**
- Claim: "Water boils at 100 degrees Celsius at standard atmospheric pressure"
- Domain: general, Category: computational, Review fee: 1,000,000 uzrn (1 ZRN)
- claim_id: `c2fc69d18cc5f1c0fb5025c4b8ae5253`
- round_id: `9594c87d47ab40a465d1b9fa5b6ee69b`
- Claim status: PENDING→IN_VERIFICATION
- Verification round auto-created in COMMIT phase
- Review fee distributed (event: `zerone.knowledge.review_fee_distributed`)
- content_hash computed: `709a17646a9a73fb837b72535995f349d2000e7707d8d18ca3fab5482f1d9489`

**Events emitted:**
- `zerone.knowledge.submit_claim` — claim_id, submitter, domain, review_fee, content_hash, claim_type, sponsored
- `zerone.knowledge.verification_round_created` — round_id, claim_id, phase=COMMIT
- `zerone.knowledge.review_fee_distributed` — claim_id, fee_amount

---

### Step 3: Submit a Structured Claim
**Status:** PASS
**Tx Hash:** `FD5DBBF370510111669F525446DB2CE5294F50E77696B849A1BABB221ABAE001`
**Result:**
- Content: "The speed of light in vacuum is approximately 299792458 metres per second"
- Domain: physics, Category: formal_proof, Review fee: 2,000,000 uzrn
- claim_id: `1af6617b25ec98dca854b38a1a4af10b`
- Structure stored correctly:
  - subject: "speed of light in vacuum"
  - predicate: "equals approximately"
  - object: "299792458 m/s"
  - scope: "special relativity"
  - tags: ["physics", "constants", "speed-of-light"]
  - negatable: true
- canonical_form auto-derived: `assert("physics", "speed of light in vacuum", "equals approximately", "special relativity")`
- canonical_hash computed: `efb83c97549b9e84142d33ab064cf5116a9fbe8fb583a9bb1a6c386e7609a305`
- Round expired with verdict=INCONCLUSIVE (3) — no verifiers committed

**Issue:** The structured claim's verification round expired (status=INSUFFICIENT=10) because no one committed to verify it. Only the first claim was manually verified.

**Design Question:** Should verifiers be auto-notified or auto-assigned to new rounds? Currently, verifiers must independently discover and participate in rounds.

---

### Step 4: Commit-Reveal Verification
**Status:** PASS
**Result:**
- Round: `9594c87d47ab40a465d1b9fa5b6ee69b`
- Phase progression: COMMIT(1) → REVEAL(2) → AGGREGATION(3) → COMPLETE(4)
- Commitments from val0 + val1 accepted in COMMIT phase
- Reveals from val0 + val1 matched commitments in REVEAL phase
- Verdict: **ACCEPT (1)** — both verifiers voted "accept"
- Fact created: `b679280fc036beaabbc0232f3cd8b5ba`
- Fact status: VERIFIED (3)
- Fact confidence: 950,000 (on 1M scale)
- Fact stratum: "empirical" (auto-assigned)
- Initial energy: 5000, energy_cap: 10000
- fitness_score: 500,000

**Commit hash construction:** `SHA256(printf "accept" + salt_raw_bytes)` — matches reveal handler verification.

---

### Step 5: Challenge a Fact
**Status:** PASS
**Tx Hash:** `37F93CD0D8E26EA1D958143BEF4DA59DF9CF2CA3DA24EEFC9BFC1F108A8DD82B`
**Result:**
- Fact `b679280fc036beaabbc0232f3cd8b5ba` challenged
- Status transition: **VERIFIED(3) → CHALLENGED(6)**
- Challenge stake: 11,000,000 uzrn (11 ZRN) — matches min_challenge_stake
- New verification round created: `23785da2f3173162aeaac0e4238a9403` (for the challenge)
- Reason: "The boiling point varies with altitude — claim is incomplete without specifying pressure explicitly"

**Events emitted:**
- `zerone.knowledge.challenge_fact` — fact_id, challenger, round_id, stake, reason
- `zerone.knowledge.verification_round_created` — for challenge re-verification

**Note:** Challenge creates a new verification round. If the challenge is upheld (new round rejects), the fact transitions to DISPROVEN and the challenger gets rewarded. If rejected, the challenger loses stake.

---

### Step 6: Submit a Contradiction
**Status:** PASS
**Tx Hash:** `7661FE1977490E86A5162430690BC3FDB1819292CF0B5E38197E020C06FA3AAE`
**Result:**
- Target fact: `b679280fc036beaabbc0232f3cd8b5ba`
- Counter-claim: "Water boils at different temperatures depending on atmospheric pressure, not fixed at 100C"
- Counter-claim ID: `8b25742073160075591309aeb6efdd94`
- Stake: 3,000,000 uzrn (3 ZRN)
- Fact status after contradiction: **CONTESTED(5)**
- New verification round: `16359070fd63a6d3b3851b5512e6a4ae`

**Events emitted:**
- `zerone.knowledge.submit_contradiction` — fact_id, submitter, counter_claim_id, domain, stake
- `zerone.knowledge.verification_round_created` — for the counter-claim verification

**Note:** A contradiction is separate from a challenge. The contradiction creates a new claim that explicitly conflicts with the target fact. Both the challenge (Step 5) and contradiction (Step 6) affected this fact:
- After challenge: VERIFIED(3) → CHALLENGED(6)
- After contradiction: CHALLENGED(6) → CONTESTED(5)

---

### Step 7: Patronise a Fact
**Status:** PASS
**Tx Hash:** `3AFC402C4276B824551E75771F549B545A5B1D4EE75E012653C6D3E878EB7C55`
**Result:**
- Fact `b679280fc036beaabbc0232f3cd8b5ba` patronised
- Amount: 10,000,000 uzrn (10 ZRN)
- Duration: 100 blocks
- Expiry block: 350
- patronage_amount on fact: 10,000,000 (recorded correctly)
- Energy unchanged at 5000 (patronage energy bonus applies at next metabolism epoch)

**Events emitted:**
- `zerone.knowledge.patronize_fact` — fact_id, patron, amount, duration_blocks, expiry_block

**Design Question:** Patronage amount is stored on the fact, but the energy bonus (`metabolism_energy_per_patronage=200` per patronage unit) is only applied during metabolism epoch processing (every 10,000 blocks). Should there be an immediate energy bump?

---

### Step 8: Propose a New Domain
**Status:** PASS
**Tx Hash:** `95A383820348041D5010846EAA0387E0DE6235CE257C7445A8585C04A8784739`
**Result:**
- Domain name: "quantum_physics"
- Description: "Quantum mechanics and quantum field theory"
- Stratum: "computational"
- Stake: 5,000,000 uzrn (5 ZRN)
- Status: **PROPOSED (3)**

---

### Step 9: Endorse and Activate Domain
**Status:** PASS
**Result:**
- 3 endorsements from val0, val1, val2
- Domain auto-activated after 3rd endorsement
- Status: **PROPOSED(3) → ACTIVE(1)**
- Total domains: 19 (was 18)

**Endorsement tx hashes:**
- val0: `CB894854...`
- val1: `B6F1D6DF...`
- val2: `81B17530...`

---

### Step 10: Report Demand
**Status:** STUB
**Result:** No CLI command exists for `MsgReportDemand`. Requires whitelisted reporter addresses (set in module params). This is designed for agent infrastructure to report demand signals, not end users.
- Current demand signals: 0
- Current demand gaps: 0

---

### Step 11: Rate a Fact (Satisfaction Feedback)
**Status:** STUB
**Result:** No CLI command exists for `MsgRateFact`. Requires a prior query receipt (fact must have been queried via the gRPC QueryServer first, which generates a receipt).

---

### Step 12: Add Common Knowledge
**Status:** STUB
**Result:** No CLI command exists for `MsgAddCommonKnowledge`. Authority-only operation.
- 70 common knowledge entries exist from genesis
- Claims with matching subjects get `common_knowledge_match = true` and reduced novelty score

---

### Step 13: Fact Metabolism (Energy Observation)
**Status:** PASS
**Result:**
- Fact energy: 5000 (initial), cap: 10000
- No energy change observed (metabolism runs every `fitness_epoch_blocks=10000` blocks, ~7 hours)
- Facts at risk: 0

**Metabolism cycle (per epoch):**
1. UpdateAllFitnessScores — recalculates fitness using weighted formula
2. ProcessCompetition — ranks facts within niches, assigns leader
3. ProcessSymbiosis — adjusts fitness based on SUPPORTS relationships
4. ProcessMetabolism — energy = old + income - cost (income from queries, citations, patronage; cost from base + content length + competition)
5. Energy → 0: fact → AT_RISK
6. AT_RISK for 5 epochs: → EXPIRED
7. EXPIRED for 20 more epochs: → PRUNED
8. Energy recovery: → back to ACTIVE

---

### Step 14: Query the Knowledge Graph
**Status:** PASS (10/10 queries)

| Query | Result |
|-------|--------|
| `facts-by-domain general` | 1 fact |
| `facts-by-submitter <faucet>` | 1 fact |
| `fact-confidence <fact_id>` | 950,000 |
| `fact-relations <fact_id>` | 0 relations |
| `fact-citation-count <fact_id>` | (returned) |
| `check-novelty general "water boiling" "..."` | 1,000,000 (max novelty) |
| `bounties` | 0 active bounties |
| `pending-claims` | 0 (all resolved) |
| `facts-by-tag physics` | 0 (structured claim didn't become a fact) |
| `facts-by-subject physics "speed of light"` | 0 (same reason) |

---

## RPCs Tested

### Transaction RPCs (21 total)

| RPC | CLI | Tested | Notes |
|-----|-----|--------|-------|
| SubmitClaim | `submit-claim` | YES | Steps 2, 3 |
| SubmitCommitment | `submit-commitment` | YES | Step 4 |
| SubmitReveal | `submit-reveal` | YES | Step 4 |
| ChallengeFact | `challenge-fact` | YES | Step 5 |
| SubmitContradiction | `submit-contradiction` | YES | Step 6 |
| PatronizeFact | `patronize-fact` | YES | Step 7 |
| ProposeDomain | `propose-domain` | YES | Step 8 |
| EndorseDomainProposal | `endorse-domain` | YES | Step 9 |
| ChallengeDomainProposal | `challenge-domain` | NO | Not tested (would need a proposed domain to challenge) |
| RegisterStratum | `register-stratum` | NO | Authority-only |
| ChallengeProvisionalFact | `challenge-provisional` | NO | No provisional facts available |
| ProposeResearchFund | `propose-research-fund` | NO | Not in scope |
| VoteResearchProposal | `vote-research-proposal` | NO | Not in scope |
| ExecuteResearchProposal | `execute-research-proposal` | NO | Authority-only |
| AddFact | `add-fact` | NO | Authority-only |
| UpdateParams | (no CLI) | NO | Authority-only |
| UpdateExtendedParams | (no CLI) | NO | Authority-only |
| AddCommonKnowledge | (no CLI) | STUB | Authority-only, no CLI |
| RemoveCommonKnowledge | (no CLI) | NO | Authority-only, no CLI |
| ReportDemand | (no CLI) | STUB | Whitelisted reporters only |
| RateFact | (no CLI) | STUB | Requires query receipt |

**Tested: 8/21 tx RPCs** (8 with CLI, 3 stubbed, 10 not in scope or authority-only)

### Query RPCs (28 total)

| Query | CLI | Tested | Notes |
|-------|-----|--------|-------|
| Params | `params` | YES | Step 1 |
| Fact | `fact [id]` | YES | Steps 4-7, 13 |
| Facts | `facts` | YES | Steps 1, 4, 14 |
| FactsByDomain | `facts-by-domain` | YES | Step 14 |
| FactsBySubmitter | `facts-by-submitter` | YES | Step 14 |
| Claim | `claim [id]` | YES | Steps 2, 3 |
| PendingClaims | `pending-claims` | YES | Step 14 |
| VerificationRound | `verification-round` | YES | Step 4 |
| Domain | `domain [name]` | YES | Steps 8, 9 |
| Domains | `domains` | YES | Steps 1, 9 |
| FactConfidence | `fact-confidence` | YES | Step 14 |
| FactCitationCount | `fact-citation-count` | YES | Step 14 |
| FactRelations | `fact-relations` | YES | Step 14 |
| FactsBySubject | `facts-by-subject` | YES | Steps 3, 14 |
| FactsByTag | `facts-by-tag` | YES | Steps 3, 14 |
| FactByCanonical | `fact-by-canonical` | YES | Step 3 (returned NotFound — claim didn't become fact) |
| BootstrapFundStatus | `bootstrap-fund-status` | YES | Step 1 |
| FactsAtRisk | `facts-at-risk` | YES | Step 13 |
| CheckNovelty | `check-novelty` | YES | Step 14 |
| CommonKnowledge | `common-knowledge` | YES | Step 12 |
| ActiveBounties | `bounties` | YES | Step 14 |
| DemandSignals | `demand-signals` | YES | Step 10 |
| TopDemandGaps | `demand-gaps` | YES | Step 10 |
| FactsByFitness | (no CLI) | NO | gRPC/REST only |
| FactLineage | (no CLI) | NO | gRPC/REST only |
| FactProgeny | (no CLI) | NO | gRPC/REST only |
| NicheInfo | (no CLI) | NO | gRPC/REST only |
| NichesByDomain | (no CLI) | NO | gRPC/REST only |

**Tested: 23/28 query RPCs** (5 without CLI, accessible via REST)

---

## Issues Found

### Issue 1: No Genesis Axioms on Fresh Localnet
**Severity:** Low (expected behavior)
**Description:** `localnet.sh start` does not inject the 777 axioms. Must use `init` + axiom injection + `boot`.
**Impact:** Tests start with 0 facts, which is fine for lifecycle testing but means metabolism and graph queries have limited data.

### Issue 2: Structured Claim Round Expired (INSUFFICIENT)
**Severity:** Medium (design consideration)
**Description:** The structured claim's verification round expired with verdict=INCONCLUSIVE because no verifiers participated. Verifiers must independently discover and commit to rounds.
**Design Question:** Should the module include a verifier notification/assignment mechanism? Currently, if no verifiers commit within `commit_phase_blocks`, the claim dies.

### Issue 3: Patronage Energy Not Immediately Applied
**Severity:** Low (by design)
**Description:** After patronizing a fact (10 ZRN for 100 blocks), the `patronage_amount` is recorded on the fact, but the `energy` field doesn't change until the next metabolism epoch (every 10,000 blocks).
**Design Question:** Should patronage provide an immediate energy boost? The current design means a fact could be AT_RISK and receive patronage but not recover until the next epoch.

### Issue 4: Missing CLI Commands for 3 Tx Types
**Severity:** Medium (operational gap)
**Description:**
- `MsgReportDemand` — requires whitelisted reporter, no CLI
- `MsgRateFact` — requires query receipt, no CLI
- `MsgAddCommonKnowledge` / `MsgRemoveCommonKnowledge` — authority-only, no CLI
**Impact:** These features cannot be tested without gRPC tooling or custom scripts. Adding CLI commands (even with access control) would improve testability.

### Issue 5: Missing CLI Queries for 5 Query Types
**Severity:** Low (accessible via REST)
**Description:** `FactsByFitness`, `FactLineage`, `FactProgeny`, `NicheInfo`, `NichesByDomain` have no CLI commands. They are accessible via REST API endpoints.

### Issue 6: Fact Confidence Exceeds survived_challenge_confidence_cap
**Severity:** Low (investigation needed)
**Description:** The verified fact has confidence=950,000 but `survived_challenge_confidence_cap=880,000`. The 950K confidence was set at initial verification, not after surviving a challenge. The cap may only apply to post-challenge confidence boosts.

---

## Design Questions Raised

1. **Verifier Discovery:** How do verifiers learn about new rounds? Is there an off-chain notification system or do they poll `pending-claims`?

2. **Patronage Timing:** Should patronage provide immediate energy or only at epoch boundaries? A fact at energy=0 (AT_RISK) receiving patronage won't recover until the next epoch.

3. **Challenge + Contradiction Interaction:** A fact can be both CHALLENGED and have a CONTRADICTION submitted. The status went VERIFIED→CHALLENGED→CONTESTED. Is this the intended precedence? What happens if the challenge round resolves while a contradiction is pending?

4. **CLI Coverage:** 3 tx types and 5 query types lack CLI commands. Are CLI wrappers planned for RateFact and ReportDemand?

5. **Metabolism Epoch Length:** `fitness_epoch_blocks=10000` (~7 hours) is very long for testing. Consider a localnet override (e.g., 100 blocks) to enable metabolism testing.

---

## Fact Lifecycle Observed

```
SubmitClaim ──→ PENDING ──→ COMMIT phase
                              │
                         val0,val1 commit
                              │
                         REVEAL phase
                              │
                         val0,val1 reveal
                              │
                         AGGREGATION
                              │
                    verdict=ACCEPT (2/2)
                              │
                         VERIFIED (3)
                         confidence=950K
                              │
              ┌───────────────┤
              │               │
         challenge         contradiction
              │               │
         CHALLENGED (6)   CONTESTED (5)
              │
         patronise (10 ZRN)
              │
         energy=5000 (unchanged until epoch)
         patronage_amount=10M
```

---

## Test Reproduction

```bash
# Start fresh localnet
scripts/localnet.sh start

# Run the lifecycle test
scripts/knowledge-lifecycle-test.sh

# Run individual steps
scripts/knowledge-lifecycle-test.sh claim
scripts/knowledge-lifecycle-test.sh verify
scripts/knowledge-lifecycle-test.sh challenge
```
