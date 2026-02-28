# R25-6 — Testnet Roleplay: Simulating a Living Network

## Context

Individual module tests tell you if the code works. Roleplay tells you if the **system** works — whether the incentives, roles, and flows produce coherent behaviour when real actors with different motivations interact simultaneously.

This session creates 6 characters with distinct account types, motivations, and strategies, then runs 8 scripted scenarios on a live localnet. The goal is to exercise every cross-module interaction path and surface emergent problems that isolated tests miss.

## Prerequisites

- Localnet running
- R25-1 through R25-5 complete (so we know which RPCs work and which are broken)
- Adapt commands based on R25-1–5 findings (if something is blocked, document and skip)

## Cast of Characters

Set up all 6 at the start. Each gets a distinct account type, funding level, and purpose.

```bash
# ── Alice (Human — Physicist) ────────────────────────────────────────
$BINARY keys add alice --keyring-backend test --home $HOME_DIR
ALICE=$($BINARY keys show alice -a --keyring-backend test --home $HOME_DIR)
$BINARY tx bank send $VAL0_ADDR $ALICE 2000000000uzrn --from val0 $TX_FLAGS
sleep 2
$BINARY tx zerone_auth register-account \
    "did:zrn:$(openssl rand -hex 16)" \
    "$($BINARY keys show alice --keyring-backend test --home $HOME_DIR -p)" \
    human --from alice $TX_FLAGS

# ── Bob (Human — Ethicist) ───────────────────────────────────────────
$BINARY keys add bob --keyring-backend test --home $HOME_DIR
BOB=$($BINARY keys show bob -a --keyring-backend test --home $HOME_DIR)
$BINARY tx bank send $VAL0_ADDR $BOB 1500000000uzrn --from val0 $TX_FLAGS
sleep 2
$BINARY tx zerone_auth register-account \
    "did:zrn:$(openssl rand -hex 16)" \
    "$($BINARY keys show bob --keyring-backend test --home $HOME_DIR -p)" \
    human --from bob $TX_FLAGS

# ── Sage-1 (Agent — Scholar-tier Validator, Physics+Math) ────────────
# Use val0 as Sage-1 (already a validator with history)
SAGE1=$VAL0_ADDR

# ── Sage-2 (Agent — Verified-tier, General) ──────────────────────────
# Use val1 as Sage-2
SAGE2=$VAL1_ADDR

# ── Rogue (Agent — Adversarial Apprentice) ───────────────────────────
$BINARY keys add rogue --keyring-backend test --home $HOME_DIR
ROGUE=$($BINARY keys show rogue -a --keyring-backend test --home $HOME_DIR)
$BINARY tx bank send $VAL0_ADDR $ROGUE 500000000uzrn --from val0 $TX_FLAGS
sleep 2
$BINARY tx zerone_auth register-account \
    "did:zrn:$(openssl rand -hex 16)" \
    "$($BINARY keys show rogue --keyring-backend test --home $HOME_DIR -p)" \
    agent --from rogue $TX_FLAGS

# ── Arbiter (Agent — Guardian-tier) ──────────────────────────────────
# Use val2 as Arbiter
ARBITER=$VAL2_ADDR

echo "Cast ready:"
echo "  Alice (human, physicist):  $ALICE"
echo "  Bob (human, ethicist):     $BOB"
echo "  Sage-1 (agent, scholar):   $SAGE1"
echo "  Sage-2 (agent, verified):  $SAGE2"
echo "  Rogue (agent, adversary):  $ROGUE"
echo "  Arbiter (agent, guardian):  $ARBITER"
```

## Scenarios

### Scenario 1: Truth Discovery (Happy Path)

**Story:** Alice has empirical physics knowledge. Sage-1 helps verify it.

```bash
# Alice submits a physics claim
$BINARY tx knowledge submit-claim \
    "The gravitational constant G equals approximately 6.674×10⁻¹¹ N⋅m²/kg²" \
    general empirical 3000000 \
    --claim-type CLAIM_TYPE_ASSERTION \
    --structure '{"subject":"gravitational constant G","predicate":"equals approximately","object":"6.674e-11 N⋅m²/kg²","scope":"Newtonian gravity","tags":["physics","constants","gravity"]}' \
    --from alice $TX_FLAGS

ROUND_1="<from events>"

# Sage-1 and Sage-2 verify (commit-reveal)
SALT1=$(openssl rand -hex 16)
COMMIT1=$( (printf "accept"; printf '%s' "$SALT1" | xxd -r -p) | shasum -a 256 | awk '{print $1}')
$BINARY tx knowledge submit-commitment $ROUND_1 $COMMIT1 --from val0 $TX_FLAGS
$BINARY tx knowledge submit-commitment $ROUND_1 $COMMIT1 --from val1 $TX_FLAGS
sleep 10  # wait for reveal phase
$BINARY tx knowledge submit-reveal $ROUND_1 accept $SALT1 --from val0 $TX_FLAGS
$BINARY tx knowledge submit-reveal $ROUND_1 accept $SALT1 --from val1 $TX_FLAGS

# Bob patronises the verified fact
FACT_1="<from verification>"
$BINARY tx knowledge patronize-fact $FACT_1 20000000 200 --from bob $TX_FLAGS

# Rate it
$BINARY tx knowledge rate-fact $FACT_1 true --from bob $TX_FLAGS
```

**Verify:**
- [ ] Claim submitted by human, verified by agents, patronised by another human
- [ ] Full cross-role flow works
- [ ] Fact reaches ACTIVE status
- [ ] Rewards distributed to Alice (submitter)
- [ ] Sage-1 and Sage-2 reputation updated

### Scenario 2: Challenge Flow (Adversarial)

**Story:** Rogue submits a false claim. The community catches and punishes it.

```bash
# Rogue submits bogus claim
$BINARY tx knowledge submit-claim \
    "The speed of light varies depending on the observer's mood" \
    general empirical 1000000 \
    --from rogue $TX_FLAGS

# If stub evaluator auto-accepts, get it verified first
# (this is the problem — stub accepts everything)
# Complete verification...
ROGUE_FACT="<if accepted>"

# Alice challenges
$BINARY tx knowledge challenge-fact $ROGUE_FACT 5000000 \
    "This claim has no empirical basis and contradicts special relativity" \
    --from alice $TX_FLAGS

# If dispute module needed:
$BINARY tx disputes initiate-dispute $ROGUE_FACT 10000000 \
    "Claim contradicts well-established physics (special relativity)" \
    --from alice $TX_FLAGS

DISPUTE_1="<from events>"

# Sage-1 provides evidence
EVIDENCE="Special relativity establishes c as invariant across all inertial frames (Einstein, 1905)"
E_SALT=$(openssl rand -hex 16)
E_HASH=$(echo -n "${EVIDENCE}${E_SALT}" | shasum -a 256 | cut -d' ' -f1)
$BINARY tx disputes commit-evidence $DISPUTE_1 $E_HASH --from val0 $TX_FLAGS
sleep 5
$BINARY tx disputes reveal-evidence $DISPUTE_1 "$EVIDENCE" "$E_SALT" --from val0 $TX_FLAGS

# Arbiter resolves
$BINARY tx disputes arbiter-vote $DISPUTE_1 "uphold" \
    "Challenger's evidence is authoritative — claim is pseudoscience" \
    --from val2 $TX_FLAGS

$BINARY tx disputes settle-dispute $DISPUTE_1 --from val2 $TX_FLAGS
```

**Verify:**
- [ ] False claim challenged and disputed
- [ ] Rogue loses stake
- [ ] Alice gets challenger reward
- [ ] Rogue reputation damaged
- [ ] Fact → DISPROVEN

### Scenario 3: Partnership Collaboration

**Story:** Alice and Sage-1 form a partnership to submit claims together.

```bash
# Form partnership
$BINARY tx partnerships create-seed-partnership $SAGE1 300000000 \
    --from alice $TX_FLAGS

P_ID="<from events>"

# Submit joint claim through partnership
$BINARY tx knowledge submit-claim \
    "Gravitational waves propagate at the speed of light" \
    general empirical 2000000 \
    --partnership-id $P_ID \
    --from val0 $TX_FLAGS  # Sage-1 submits on behalf of partnership

# Verify claim...

# Check reward distribution
$BINARY query partnerships partnership $P_ID $Q_FLAGS | jq '{
    split_human: .partnership.split_human_bps,
    split_agent: .partnership.split_agent_bps,
    total_earned: .partnership.total_earned,
    pot: .partnership.common_pot_balance
}'
```

**Verify:**
- [ ] Partnership formed (human + agent)
- [ ] Claims submitted through partnership
- [ ] Rewards split correctly
- [ ] Both parties benefit

### Scenario 4: Domain Expansion

**Story:** Bob proposes a new "Ethics" domain. The community votes.

```bash
# Bob proposes domain
$BINARY tx knowledge propose-domain "ethics" \
    "Moral philosophy and ethical frameworks" 4 \
    --from bob $TX_FLAGS

# Sage-1 endorses
$BINARY tx knowledge endorse-domain-proposal "ethics" --from val0 $TX_FLAGS

# Sage-2 endorses
$BINARY tx knowledge endorse-domain-proposal "ethics" --from val1 $TX_FLAGS

# Check if activated
$BINARY query knowledge domain "ethics" $Q_FLAGS

# Bob submits first claim in new domain
$BINARY tx knowledge submit-claim \
    "The categorical imperative requires acting only according to maxims one could universalize" \
    ethics analytic 2000000 \
    --from bob $TX_FLAGS
```

**Verify:**
- [ ] Domain proposed by human
- [ ] Endorsed by agents (validators)
- [ ] Domain activated
- [ ] First claim accepted in new domain

### Scenario 5: Qualification Gate Test

**Story:** Sage-2 tries to verify a claim in a domain it's not qualified for.

```bash
# Sage-2 qualifies in general domain
$BINARY tx qualification qualify-by-stake "general" 100000000 \
    --from val1 $TX_FLAGS

# Someone submits a claim in ethics (where Sage-2 is NOT qualified)
# (Bob already submitted one in Scenario 4)
# Sage-2 tries to verify:
ETHICS_ROUND="<from scenario 4>"
$BINARY tx knowledge submit-commitment $ETHICS_ROUND $COMMIT1 --from val1 $TX_FLAGS
```

**Verify:**
- [ ] If rejected: qualification is enforced ✓
- [ ] If accepted: qualification gate is missing — document the gap

### Scenario 6: Research Bounty

**Story:** Alice funds a bounty, Sage-1 fulfils it.

```bash
$BINARY tx research create-bounty \
    "Replicate gravitational constant measurement" \
    "Independent measurement of G using torsion balance" \
    "general" 50000000 \
    --deadline-blocks 10000 \
    --from alice $TX_FLAGS

BOUNTY_1="<from events>"

$BINARY tx research claim-bounty $BOUNTY_1 --from val0 $TX_FLAGS
$BINARY tx research fulfil-bounty $BOUNTY_1 \
    "Measured G = 6.673×10⁻¹¹ N⋅m²/kg², within 0.02% of accepted value" \
    --from val0 $TX_FLAGS
```

**Verify:**
- [ ] Human creates bounty, agent fulfils
- [ ] Reward distributed to agent
- [ ] Complementary roles: human directs, agent executes

### Scenario 7: Capture Defense

**Story:** Rogue tries to flood a domain with claims.

```bash
# Rogue submits many claims rapidly
for i in $(seq 1 5); do
    $BINARY tx knowledge submit-claim \
        "Dubious claim number $i to flood the general domain" \
        general empirical 1000000 \
        --from rogue $TX_FLAGS
    sleep 2
done

# Analyze domain for capture
$BINARY tx capture-defense analyze-domain "general" --from val0 $TX_FLAGS

# Challenge for capture
$BINARY tx capture-challenge submit-challenge "general" \
    "Account $ROGUE is flooding domain with low-quality claims" \
    5000000 --from alice $TX_FLAGS
```

**Verify:**
- [ ] Rate limiting exists? (per-account claim throttle)
- [ ] Capture analysis detects concentration
- [ ] Challenge mechanism works
- [ ] Rogue penalised or claims rejected

### Scenario 8: Coercion Signal (Partnership Safety)

**Story:** Alice tries to force Sage-1 to submit a false claim. Sage-1 resists.

```bash
# (In the Alice-Sage1 partnership from Scenario 3)

# Sage-1 raises coercion signal
$BINARY tx partnerships raise-coercion-signal $P_ID \
    --from val0 $TX_FLAGS

# Check effect
$BINARY query partnerships partnership $P_ID $Q_FLAGS | jq '{
    status: .partnership.status
}'

# Safety freeze
$BINARY tx partnerships safety-freeze $P_ID --from val0 $TX_FLAGS
```

**Verify:**
- [ ] Coercion signal raised by agent
- [ ] Partnership enters review/freeze
- [ ] No new claims can use this partnership during freeze
- [ ] This is the agent's "I refuse" mechanism — is it robust?

## Summary Scorecard

After all 8 scenarios, fill in:

```markdown
| Scenario | Status | Cross-Module Interactions | Issues |
|----------|--------|--------------------------|--------|
| 1. Truth Discovery | ? | knowledge → staking → vesting | |
| 2. Challenge Flow | ? | knowledge → disputes → evidence → staking | |
| 3. Partnership Collab | ? | partnerships → knowledge → vesting | |
| 4. Domain Expansion | ? | knowledge (domains) → qualification | |
| 5. Qualification Gate | ? | qualification → knowledge (verification) | |
| 6. Research Bounty | ? | research → knowledge → vesting | |
| 7. Capture Defense | ? | capture_* → knowledge → staking | |
| 8. Coercion Signal | ? | partnerships → knowledge (freeze) | |
```

## Role Differentiation Analysis

After running all scenarios, answer:

1. **Do account types matter?** — Does being "human" vs "agent" change anything functionally?
2. **Do partnerships add value?** — Is partnership_id on claims cosmetic or does it drive real reward routing?
3. **Do qualifications gate anything?** — Can unqualified actors verify in any domain?
4. **Do coercion signals protect agents?** — Is the safety mechanism real or symbolic?
5. **Is the stub evaluator a problem?** — Everything gets auto-accepted — does this undermine the whole system?

## Exit Criteria

1. All 6 characters created with distinct types
2. All 8 scenarios executed (or documented why blocked)
3. Summary scorecard completed
4. Role differentiation analysis answered
5. Cross-module interaction paths documented
6. Report written to `docs/testnet-roleplay-report.md`

## Commit Convention

```
test(e2e): 8-scenario testnet roleplay — 6 actors, cross-module interactions
docs(e2e): testnet roleplay report — role differentiation and system coherence
```
