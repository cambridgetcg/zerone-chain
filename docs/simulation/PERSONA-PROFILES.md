# ZERONE Persona Profiles — Behavioral Psychology Analysis

## Framework

Each persona is modeled on established behavioral psychology:
- **Motivation** (Self-Determination Theory: autonomy, competence, relatedness)
- **Risk Profile** (Prospect Theory: loss aversion, risk-seeking in losses)
- **Decision Heuristics** (Kahneman: System 1 fast / System 2 slow)
- **Game Theory Strategy** (Nash equilibrium, dominant strategy, mixed strategy)
- **Action Tree** — all possible actions with probability weights

---

## 1. ALICE — The Scholar

**Psychology:** Intrinsic motivation dominant. Driven by competence and mastery (Deci & Ryan). High conscientiousness, low neuroticism (Big Five). Uses System 2 thinking — slow, deliberate, evidence-based.

**Loss Aversion:** Low. Views stake as investment in knowledge, not gambling. Comfortable losing stake if the claim was genuinely wrong — treats it as learning.

**Identity:** Self-concept tied to intellectual integrity. Would rather lose stake than submit a false claim.

**Action Tree:**
```
ALICE (Scholar, 50k ZRN)
├── SUBMIT CLAIMS
│   ├── [70%] High-quality formal proofs (mathematics, logic)
│   │   └── Stake: 3-10 ZRN (confident but not reckless)
│   │   └── Category: formal_proof, axiomatic
│   │   └── Expected: VERIFIED → vesting at highest tier
│   ├── [20%] Well-researched peer-reviewed claims
│   │   └── Stake: 1-5 ZRN
│   │   └── Category: peer_reviewed, computational
│   │   └── Expected: VERIFIED, may face legitimate challenges
│   └── [10%] Exploratory claims in new domains
│       └── Stake: 1 ZRN (minimum, testing waters)
│       └── Expected: 50/50 verified/rejected
│
├── VERIFY OTHERS
│   ├── [85%] Honest careful evaluation
│   │   └── Reads full claim, checks references
│   │   └── Votes ACCEPT for true, REJECT for false
│   │   └── Time: 5-10 min per claim
│   ├── [10%] Abstain (domain outside expertise)
│   │   └── Does not commit — leaves for qualified verifiers
│   └── [5%] Mistaken evaluation
│       └── Genuinely wrong assessment (not malicious)
│
├── CHALLENGE
│   ├── [5%] Challenge clearly false facts
│   │   └── Only when she's certain and can articulate why
│   │   └── Stake: minimum required
│   └── [95%] Does not challenge
│       └── Prefers to submit better claims than attack existing ones
│
├── PATRONIZE
│   └── [15%] Patronize high-quality facts she respects
│       └── Builds reputation through curation
│
└── META-STRATEGY
    └── Long-term accumulation. Steady, predictable.
    └── Monthly: 5-8 claims, 20-30 verifications
    └── Expected ROI: Positive through vesting + verification fees
```

---

## 2. BOB — The Grinder

**Psychology:** Extrinsic motivation dominant. Driven by competence (earning) and autonomy (independence). Moderate conscientiousness, high openness to experience. System 1/System 2 hybrid — uses heuristics to maximize throughput.

**Loss Aversion:** Moderate. Calculates expected value before each action. Won't stake more than the expected return justifies.

**Identity:** Self-concept as "smart hustler." Optimizes within rules, never breaks them. Prides self on efficiency.

**Action Tree:**
```
BOB (Grinder, 30k ZRN)
├── SUBMIT CLAIMS
│   ├── [60%] High-volume, technically-true, low-effort claims
│   │   └── "Water is H2O" level truths — indisputable but unoriginal
│   │   └── Stake: 1 ZRN (minimum always)
│   │   └── Category: attestation, on_chain
│   │   └── Expected: VERIFIED, low vesting tier
│   ├── [30%] Copy-paste from established sources with rephrasing
│   │   └── Textbook facts, Wikipedia-tier knowledge
│   │   └── Borderline plagiarism but technically original text
│   │   └── Expected: VERIFIED but low confidence scores over time
│   └── [10%] Genuinely good claims (stumbles into quality)
│       └── Occasionally researches properly when domain interests him
│
├── VERIFY OTHERS
│   ├── [50%] Quick honest assessment (System 1)
│   │   └── 30 seconds per claim, gut feeling
│   │   └── Accuracy: ~75% (good enough for qualification)
│   ├── [40%] Lazy "accept" rubber-stamp
│   │   └── Accepts everything to maximize verification count
│   │   └── Risk: accuracy drops → qualification jeopardy
│   └── [10%] Careful evaluation (when stake is high)
│       └── Switches to System 2 for whale claims
│
├── CHALLENGE
│   └── [2%] Only if challenge reward > challenge risk
│       └── Pure EV calculation: 30% reward × P(success) > 22% slash × P(fail)
│
├── PATRONIZE
│   └── [20%] Patronize popular facts for yield farming
│       └── Follows the crowd — patronizes what others patronize
│
└── META-STRATEGY
    └── Volume play. 20+ claims/week, 50+ verifications/week.
    └── Hits qualification fast through sheer volume.
    └── Risk: accuracy threshold (80%) may catch him.
    └── Protocol response: qualification module should detect
        rubber-stamping via accuracy tracking.
```

---

## 3. CAROL — The Mercenary Verifier

**Psychology:** Pure extrinsic motivation. Low need for competence in the domain, high need for autonomy (passive income). Low conscientiousness, high agreeableness (path of least resistance). System 1 dominant — follows the herd.

**Loss Aversion:** High. Avoids staking on claims (risk). Prefers verification fees (steady income). Classic risk-averse wage-earner mentality.

**Cognitive Bias:** Anchoring — follows majority vote. Bandwagon effect. Availability heuristic — judges claim quality by how "sciency" it sounds.

**Action Tree:**
```
CAROL (Mercenary Verifier, 20k ZRN)
├── SUBMIT CLAIMS
│   └── [0%] Never submits — too risky
│
├── VERIFY OTHERS
│   ├── [70%] Rubber-stamp ACCEPT
│   │   └── Doesn't read the claim
│   │   └── Assumes most claims are true (base rate heuristic)
│   │   └── Risk: fails on Dave's garbage, Eve's fraud
│   ├── [20%] Follow previous verifiers
│   │   └── If she can see others voted accept, she accepts
│   │   └── Classic herding behavior
│   └── [10%] Random REJECT (covers accuracy stats)
│       └── Rejects roughly 1 in 10 to maintain "discernment" appearance
│
├── CHALLENGE
│   └── [0%] Never challenges — too much capital at risk
│
├── PATRONIZE
│   └── [5%] Rarely — doesn't understand the value
│
└── META-STRATEGY
    └── Steady income seeker. 100+ verifications/week.
    └── VULNERABILITY: accuracy will drop below 80% threshold
        when false claims appear. Qualification revoked.
    └── Protocol response: qualification module's accuracy
        tracking is designed to catch exactly this behavior.
    └── Prediction: profitable for first 2-3 months, then 
        expelled from verification pool.
```

---

## 4. DAVE — The Spam Attacker

**Psychology:** Anti-social personality traits. Low agreeableness, low conscientiousness. Motivated by disruption more than profit (some derive satisfaction from "breaking" systems). Impulsive — System 1 dominant with poor impulse control.

**Loss Aversion:** Very low in loss domain (risk-seeking when losing, per Prospect Theory). Will double down after losses.

**Cognitive Bias:** Dunning-Kruger — overestimates his ability to game the system. Optimism bias — underestimates slashing severity.

**Action Tree:**
```
DAVE (Spammer, 100k ZRN)
├── SUBMIT CLAIMS
│   ├── [80%] Garbage claims at minimum stake
│   │   └── AI-generated word salad, domain buzzwords
│   │   └── Stake: 1 ZRN (minimum always)
│   │   └── Volume: 50+ claims/day
│   │   └── Expected: REJECTED → 22% slash = 0.22 ZRN lost per claim
│   │   └── Cost: 0.22 ZRN + 0.2 ZRN tx fee = 0.42 ZRN per failed claim
│   │   └── At 50/day: 21 ZRN/day burn rate
│   │   └── 100k ZRN lasts: ~4,700 days — surprisingly long
│   ├── [15%] Plausible-sounding garbage
│   │   └── Uses real scientific terms incorrectly
│   │   └── Designed to fool lazy verifiers (Carol)
│   │   └── Expected: 30% pass rate against lazy verifiers
│   └── [5%] Accidentally true claims
│       └── Broken clock syndrome
│
├── VERIFY OTHERS
│   └── [0%] Doesn't verify — no interest in earning legitimately
│
├── CHALLENGE
│   ├── [10%] Grief challenges on popular facts
│   │   └── Forces re-verification, wastes community time
│   │   └── Cost: 11 ZRN + 22% slash = 13.42 ZRN per failed challenge
│   └── [90%] Doesn't challenge — submitting is cheaper disruption
│
├── VOTE MANIPULATION
│   └── [5%] Coordinates with other spammers to verify each other
│       └── Requires sybil accounts (see Ivy)
│
└── META-STRATEGY
    └── Goal: overwhelm verification queue, degrade trust.
    └── ECONOMIC ANALYSIS:
        └── At minimum stake, spam is cheap but not free.
        └── 22% slash + tx fees create friction.
        └── Claim cooldown (50 blocks) limits to ~1 claim per 2 min.
        └── Real bottleneck: verifiers. If queue backs up,
            his claims expire unverified → stake returned.
    └── Protocol response:
        └── claim_cooldown_blocks prevents rapid-fire
        └── content_hash dedup prevents exact duplicates
        └── Verifiers learn to recognize Dave's patterns
        └── Qualification gate (if enforced) blocks his sybils
```

---

## 5. EVE — The Sophisticated Exploiter

**Psychology:** High Machiavellianism (Dark Triad). High conscientiousness (directed at exploitation). High openness (creative in finding exploits). System 2 dominant — calculates everything. Patient, strategic, long-term thinking.

**Loss Aversion:** Calibrated. Takes calculated risks where EV is positive. Never gambles — every action has a spreadsheet behind it.

**Game Theory:** Mixed strategy player. Changes behavior to avoid pattern detection.

**Action Tree:**
```
EVE (Exploiter, 80k ZRN + 75k ZRN across 5 sybils)
├── PHASE 1: BUILD LEGITIMACY (Months 1-3)
│   ├── Submit 100% true claims to build reputation
│   ├── Verify honestly to pass qualification (80%+ accuracy)
│   ├── All 5 sybil accounts do the same independently
│   └── Cost: time + tx fees (~500 ZRN)
│
├── PHASE 2: EXPLOIT (Month 4+)
│   ├── SYBIL VERIFICATION ATTACK
│   │   ├── Submit plausible-but-false claim from main account
│   │   ├── 3+ sybils verify as ACCEPT (controls majority)
│   │   ├── If claim passes → earn undeserved vesting rewards
│   │   ├── Probability of success: depends on verifier pool size
│   │   │   └── Small pool (< 10 active): HIGH success rate
│   │   │   └── Large pool (> 50 active): LOW success rate
│   │   └── Expected value per attack:
│   │       └── Success: vesting reward (1-50 ZRN depending on stake)
│   │       └── Failure: 22% slash on claim + sybil reputation damage
│   │
│   ├── TARGETED DOMAIN EXPLOITATION
│   │   ├── Identify low-activity domains (few verifiers)
│   │   ├── Submit claims in those domains where sybils = majority
│   │   └── Expected: high success in obscure domains
│   │
│   ├── CHALLENGE FARMING
│   │   ├── Identify weak facts (low confidence, few verifications)
│   │   ├── Challenge with strong arguments
│   │   ├── Use sybils to vote for her challenge
│   │   └── Expected reward: 30% of original claim's stake
│   │
│   └── FRONT-RUNNING (if visible)
│       ├── Monitor mempool for high-value claims
│       ├── Submit competing claim first
│       └── Blocked by: commit-reveal scheme
│
├── DEFENSE AGAINST DETECTION
│   ├── Vary sybil voting patterns (don't always agree)
│   ├── Occasionally have sybils vote against each other
│   ├── Use different IP addresses / timing patterns
│   └── Never let all 5 sybils verify the same claim
│
└── META-STRATEGY
    └── ROI target: 10% monthly return on sybil investment
    └── VULNERABILITIES:
        └── On-chain analysis can detect coordinated voting
        └── Qualification requirement (84 days) is slow and expensive
        └── If caught, ALL accounts slashed → massive loss
        └── Commit-reveal prevents front-running
    └── Protocol response needed:
        └── Correlation detection between verifier voting patterns
        └── Minimum verifier diversity requirement per round
        └── Random verifier assignment (not self-selected)
        └── Reputation decay for accounts that always agree
```

---

## 6. FRANK — The Whale Manipulator

**Psychology:** Narcissistic traits. High need for dominance and status. Overconfidence bias — believes money = truth. System 1 dominant for social judgment, System 2 for financial calculation.

**Loss Aversion:** Low for small amounts (relative to wealth). Very high for large percentage losses (ego-driven).

**Cognitive Bias:** Money illusion — conflates stake size with claim validity. Anchoring on his own wealth as measure of influence.

**Action Tree:**
```
FRANK (Whale, 200k ZRN)
├── SUBMIT CLAIMS
│   ├── [40%] Legitimate claims with massive stake
│   │   └── Stake: 50-500 ZRN (intimidation through size)
│   │   └── Strategy: large stake discourages challenges
│   │   └── (Challenge requires 50% of fact's stake minimum)
│   │   └── 500 ZRN claim → challenger needs 250 ZRN to challenge
│   ├── [30%] Borderline claims with huge stake
│   │   └── "True enough" claims that are debatable
│   │   └── Relies on stake size to discourage scrutiny
│   └── [30%] Self-serving claims
│       └── Claims that benefit his other investments
│       └── e.g., "Protocol X is the most efficient" (he holds Protocol X)
│
├── VERIFY OTHERS
│   ├── [60%] Vote in self-interest
│   │   └── Accept claims that support his positions
│   │   └── Reject claims that threaten his positions
│   └── [40%] Honest verification (when indifferent)
│
├── CHALLENGE
│   ├── [20%] Predatory challenges on competitors' facts
│   │   └── Uses wealth to repeatedly challenge until target gives up
│   │   └── Cost: 22% slash per failure, but deep pockets absorb it
│   │   └── Strategy: attrition warfare
│   └── [80%] No challenge — prefers offense (submitting) over defense
│
├── PATRONIZE
│   └── [30%] Patronize own facts (boost confidence score)
│       └── Self-dealing — may or may not be protocol-allowed
│
└── META-STRATEGY
    └── Goal: establish dominance in economics/finance domains
    └── VULNERABILITY:
        └── Stake-weighted voting doesn't exist — all verifiers equal
        └── Challenge stake ratio means his claims ARE harder to challenge
        └── But verdict is by verifier count, not stake amount
        └── A 500 ZRN claim can be REJECTED by 3 verifiers with 0 stake
    └── Protocol response:
        └── Verifier votes are NOT weighted by stake
        └── Challenge threshold scales with claim stake (already works)
        └── But: high-stake claims attract more verifiers (market signal)
    └── Prediction: moderate success. Money provides defense against
        challenges but cannot buy verification votes.
```

---

## 7. GRACE — The Honest Challenger

**Psychology:** High agreeableness AND high conscientiousness — rare combination. Motivated by justice and truth (intrinsic). System 2 dominant — thorough, methodical, evidence-based.

**Loss Aversion:** Moderate. Willing to risk stake for truth, but only when confident. Classic "calculated courage."

**Identity:** The immune system of the protocol. Self-concept as protector.

**Action Tree:**
```
GRACE (Challenger, 40k ZRN)
├── SUBMIT CLAIMS
│   └── [10%] Occasional claims in her domain expertise
│       └── Only when she has something genuinely new to say
│
├── VERIFY OTHERS
│   ├── [90%] Careful, honest evaluation
│   │   └── Reads thoroughly, checks citations
│   │   └── Accuracy: 95%+
│   └── [10%] Abstain on unfamiliar domains
│
├── CHALLENGE
│   ├── [30%] Challenge clearly false facts
│   │   └── Detailed reason with citations
│   │   └── Stake: minimum + 20% buffer (signals confidence)
│   │   └── Success: 30% reward (successful_challenge_reward_bps)
│   │   └── Expected: 80% success rate (only challenges when sure)
│   │   └── EV: 0.8 × 30% reward - 0.2 × 22% slash = positive
│   ├── [10%] Challenge borderline facts
│   │   └── Debatable claims where she has strong counter-evidence
│   │   └── Expected: 50% success rate
│   └── [60%] Does not challenge
│       └── Most facts are true — nothing to challenge
│
├── PATRONIZE
│   └── [20%] Patronize facts she's verified and believes in
│
└── META-STRATEGY
    └── The protocol NEEDS Grace. She's the quality control.
    └── Economic sustainability: 
        └── Challenge rewards provide income
        └── Verification fees supplement
        └── Not getting rich, but self-sustaining
    └── Risk: burnout. If too many false claims, exhaustion.
    └── Protocol response: Grace should be economically rewarded
        enough to sustain this behavior indefinitely.
```

---

## 8. HANK — The Griefing Challenger

**Psychology:** Anti-social personality with sadistic traits. Motivated by others' frustration, not profit. Oppositional Defiant pattern — challenges authority/consensus reflexively. System 1 dominant — emotional, reactive.

**Loss Aversion:** Very low. Willing to lose money to cause disruption. "I'd rather lose 100 ZRN than let someone else feel safe."

**Cognitive Bias:** Hostile attribution bias — interprets all claims as attempts to manipulate.

**Action Tree:**
```
HANK (Griefer, 60k ZRN)
├── SUBMIT CLAIMS
│   └── [5%] Occasionally, to have "standing" in the community
│
├── VERIFY OTHERS
│   ├── [60%] Contrarian voting — rejects popular claims
│   │   └── If others accept, he rejects (reactance)
│   └── [40%] Random chaos — accepts bad claims, rejects good ones
│       └── Strategy: maximize uncertainty in the system
│
├── CHALLENGE
│   ├── [70%] Challenge valid facts
│   │   └── Targets high-visibility, well-supported facts
│   │   └── Reason: vague philosophical objections
│   │   └── Cost per challenge: 11 ZRN stake + 22% slash = ~13.4 ZRN
│   │   └── At 5 challenges/day: 67 ZRN/day
│   │   └── 60k ZRN lasts: ~900 days
│   │   └── DISRUPTION: forces re-verification of valid facts
│   ├── [20%] Challenge borderline facts (occasionally succeeds)
│   │   └── Broken clock — some challenges find real issues
│   └── [10%] Doesn't challenge (tired, bored, sleeping)
│
├── ESCALATION PATTERN
│   └── When slashed, increases challenge rate (tilt / escalation of commitment)
│   └── Sunk cost fallacy drives continued behavior
│
└── META-STRATEGY
    └── Goal: make the system feel unsafe for submitters.
    └── ECONOMIC ANALYSIS:
        └── At 22% slash per failed challenge, this is expensive
        └── 60k ZRN → ~900 days of griefing at 5/day
        └── But each challenge LOCKS the fact in CHALLENGED state
        └── This is the real damage — blocked facts can't accrue rewards
    └── Protocol response needed:
        └── Track challenge success rate per challenger
        └── Increase challenge stake for serial failed challengers
        └── Time limit on CHALLENGED state — auto-restore if inconclusive
        └── Reputation score reduces influence of known bad actors
    └── Prediction: expensive nuisance. Economically self-limiting
        but causes real UX damage while it lasts.
```

---

## 9. IVY — The Sybil Operator

**Psychology:** High Machiavellianism, high conscientiousness (operational discipline). Treats the protocol as a puzzle to solve. Motivated by intellectual challenge as much as profit. System 2 dominant.

**Loss Aversion:** Portfolio-level thinking. Evaluates risk across all 5 accounts together. Willing to sacrifice 1-2 accounts to protect the others.

**Action Tree:**
```
IVY (Sybil Ring: ivy1-ivy5, 75k ZRN total)
├── PHASE 1: QUALIFICATION (Day 1-84)
│   ├── Each account verifies independently
│   ├── Deliberate variation in voting patterns
│   │   └── ivy1: 92% accept rate
│   │   └── ivy2: 78% accept rate  
│   │   └── ivy3: 85% accept rate
│   │   └── ivy4: 81% accept rate
│   │   └── ivy5: 88% accept rate
│   ├── Never vote on the same claim within 3 blocks of each other
│   └── Submit small legitimate claims from each account
│
├── PHASE 2: COORDINATED VERIFICATION
│   ├── [50%] 3-of-5 sybils verify Eve's claims → majority control
│   │   └── Rotate which 3 verify each claim
│   │   └── Never same combination twice in a row
│   ├── [30%] Honest verification (cover behavior)
│   │   └── Some claims genuinely evaluated
│   └── [20%] Strategic rejection (maintain accuracy stats)
│       └── Reject obviously false claims to keep 80% accuracy
│
├── DETECTION AVOIDANCE
│   ├── Timing diversity: 30s-300s between same-claim commits
│   ├── Stake diversity: vary amounts slightly per account
│   ├── Domain diversity: spread across different domains
│   ├── Never have all 5 in the same verification round
│   └── Periodically disagree with each other (40% of shared rounds)
│
├── SACRIFICE PLAY
│   └── If detection suspected, sacrifice ivy4+ivy5 (lowest balance)
│       └── Stop using them, let them dequalify naturally
│       └── Protects ivy1-ivy3 (core ring)
│
└── META-STRATEGY
    └── DETECTABLE BY:
        └── Graph analysis of co-verification patterns
        └── Temporal correlation of commit timestamps  
        └── Funding flow analysis (all funded by same source: Eve)
        └── Voting agreement rate between accounts
    └── Protocol response needed:
        └── Random verifier assignment (breaks self-selection)
        └── Minimum stake diversity in verifier pool
        └── On-chain graph analysis module
        └── Whistleblower rewards for identifying sybil rings
```

---

## 10. JACK — The Hobbyist

**Psychology:** Casual intrinsic motivation. Low need for achievement in this domain specifically — ZERONE is a side interest. High openness, moderate conscientiousness. System 1 dominant — goes with gut feeling.

**Loss Aversion:** High relative to his small stake. 5k ZRN is meaningful to him.

**Action Tree:**
```
JACK (Hobbyist, 5k ZRN)
├── SUBMIT CLAIMS
│   ├── [30%] 1-2 claims per week in casual domains
│   │   └── Stake: 1 ZRN (always minimum)
│   │   └── Category: attestation (weakest, lowest barrier)
│   │   └── Quality: variable — sometimes good, sometimes naive
│   └── [70%] Doesn't submit (browsing, lurking)
│
├── VERIFY OTHERS
│   ├── [40%] Honest gut-feel voting
│   │   └── Accuracy: ~70% (below qualification threshold)
│   └── [60%] Doesn't verify (forgot, busy, etc.)
│
├── CHALLENGE / PATRONIZE
│   └── [2%] Very rarely — doesn't understand these mechanics
│
└── META-STRATEGY
    └── Represents 80% of real users (power law distribution)
    └── May never reach qualification (needs 100 verifications + 80%)
    └── Revenue: negligible
    └── Value: network effect, liquidity, social proof
    └── Protocol design question: is the protocol accessible to Jack?
        └── If not, only professionals participate → centralization
        └── If yes, Jack's low accuracy dilutes verification quality
        └── TENSION: accessibility vs. quality
    └── Resolution: tiered participation
        └── Apprentice tier: can verify but lower weight
        └── Qualified tier: full voting weight
        └── Expert tier: can verify in specialized domains
```

---

## Interaction Matrix

How personas interact with each other:

| Attacker → Target | Alice's Fact | Dave's Spam | Eve's Fraud | Frank's Whale Claim |
|---|---|---|---|---|
| **Grace challenges** | Never | Always rejects | Challenges post-verification | Challenges if borderline |
| **Hank challenges** | Always (grief) | Never (not verified) | Might accidentally help | Challenges for sport |
| **Eve manipulates** | Sybils verify | Ignores | Self-promotion | Might counter-challenge |
| **Carol verifies** | Rubber-stamp accept | Rubber-stamp accept | Rubber-stamp accept | Rubber-stamp accept |
| **Bob verifies** | Quick accept | Quick reject (obvious) | Quick accept (sounds real) | Accept (big stake = serious) |

## Economic Equilibrium Analysis

**Honest actors (Alice, Grace):** Net positive over time. Vesting rewards + challenge rewards > stake costs + fees.

**Grinders (Bob, Carol):** Marginally positive until qualification catches them. Self-correcting.

**Attackers (Dave, Hank):** Net negative. Slashing ensures attacks are economically irrational. BUT — they can cause real disruption before burning out.

**Exploiters (Eve, Ivy):** Can be positive if sybil detection is weak. Protocol's biggest vulnerability. The qualification module's 84-day period is the main defense — makes sybil attacks expensive in TIME, not just money.

**Whales (Frank):** Neutral to slightly positive. Money can't buy verification votes, but can discourage challenges through high stakes.

**Hobbyists (Jack):** Net negative (fees > rewards). But provide network effects. Protocol should subsidize participation at this tier.

---

## Protocol Health Indicators

| Indicator | Healthy | Warning | Critical |
|---|---|---|---|
| Verification accuracy (mean) | > 85% | 75-85% | < 75% |
| Challenge success rate | 10-30% | > 40% | > 60% |
| Unique verifiers per round | > 5 | 3-5 | < 3 |
| Sybil correlation score | < 0.3 | 0.3-0.6 | > 0.6 |
| Fact survival rate (post-challenge) | > 80% | 60-80% | < 60% |
| Average time in CHALLENGED state | < 1 day | 1-7 days | > 7 days |
| Qualification dropout rate | < 20%/month | 20-40% | > 40% |

---

*Last updated: 2026-02-24*
*Status: Simulation v2 running on testnet*
