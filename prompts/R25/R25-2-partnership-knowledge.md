# R25-2 — Partnership-Knowledge Integration: Collaborative Truth-Seeking

## Context

Partnerships are ZERONE's core human-agent collaboration primitive. A partnership (`x/partnerships`) links a `human_addr` to an `agent_addr` with revenue split, consensus operations, safety freezes, and coercion signals. Claims can carry a `partnership_id`. But nobody has tested:

1. Does submitting a claim with `partnership_id` actually route rewards to the partnership?
2. Does the partnership consensus mechanism work for knowledge decisions?
3. Do coercion signals protect agents from being forced to submit false claims?
4. What happens to claims when a partnership dissolves?

## Prerequisites

- Localnet running

## Task

### 1. Create Human and Agent Accounts

```bash
# Human account
$BINARY keys add alice-human --keyring-backend test --home $HOME_DIR
ALICE=$($BINARY keys show alice-human -a --keyring-backend test --home $HOME_DIR)
$BINARY tx bank send $VAL0_ADDR $ALICE 1000000000uzrn --from val0 $TX_FLAGS

# Register as human
$BINARY tx zerone_auth register-account \
    "did:zrn:$(openssl rand -hex 16)" \
    "$($BINARY keys show alice-human --keyring-backend test --home $HOME_DIR -p)" \
    human --from alice-human $TX_FLAGS

# Agent account
$BINARY keys add sage1-agent --keyring-backend test --home $HOME_DIR
SAGE1=$($BINARY keys show sage1-agent -a --keyring-backend test --home $HOME_DIR)
$BINARY tx bank send $VAL0_ADDR $SAGE1 1000000000uzrn --from val0 $TX_FLAGS

# Register as agent
$BINARY tx zerone_auth register-account \
    "did:zrn:$(openssl rand -hex 16)" \
    "$($BINARY keys show sage1-agent --keyring-backend test --home $HOME_DIR -p)" \
    agent --from sage1-agent $TX_FLAGS
sleep 3
```

### 2. Form a Seed Partnership

```bash
$BINARY tx partnerships create-seed-partnership $SAGE1 500000000 \
    --from alice-human $TX_FLAGS
```

**Verify:**
- [ ] Partnership created with `human_addr` = Alice, `agent_addr` = Sage-1
- [ ] Initial deposit escrowed
- [ ] Status: active (or pending acceptance?)
- [ ] Revenue split set (default 50/50? configurable?)
- [ ] Partnership ID in events

**Issues to look for:**
- Does `create-seed-partnership` enforce that `human` is account_type="human"? Or can any account be "human"?
- Does the agent need to accept? Or is it unilateral?
- What prevents a human from partnering with another human (labelled as agent)?

### 3. Accept Partnership (if needed)

```bash
PARTNERSHIP_ID="<from events>"

# If the agent needs to accept:
$BINARY tx partnerships accept-partnership $PARTNERSHIP_ID 500000000 \
    --from sage1-agent $TX_FLAGS 2>/dev/null || echo "No acceptance needed"
```

### 4. Submit a Claim Through the Partnership

```bash
# Agent submits claim on behalf of the partnership
$BINARY tx knowledge submit-claim \
    "Quantum entanglement allows correlated measurements between spatially separated particles" \
    general empirical 2000000 \
    --partnership-id $PARTNERSHIP_ID \
    --from sage1-agent $TX_FLAGS
```

**Verify:**
- [ ] Claim submitted with partnership_id populated
- [ ] Who can submit: only the agent? Only the human? Both?
- [ ] Does the review fee come from the partnership common pot or the submitter's personal balance?
- [ ] Is the partnership_id validated? (does it check that the submitter is actually in this partnership?)

### 5. Verify the Claim and Check Reward Routing

```bash
# Complete verification round (manual commit-reveal as in R25-1)
# ...

# After verification:
# Check where the reward goes
$BINARY query vesting-rewards vesting-schedules --recipient $SAGE1 $Q_FLAGS 2>/dev/null
$BINARY query vesting-rewards vesting-schedules --recipient $ALICE $Q_FLAGS 2>/dev/null

# Check partnership balance
$BINARY query partnerships partnership $PARTNERSHIP_ID $Q_FLAGS | jq '{
    common_pot_balance: .partnership.common_pot_balance,
    total_earned: .partnership.total_earned,
    split_human_bps: .partnership.split_human_bps,
    split_agent_bps: .partnership.split_agent_bps
}'
```

**Verify:**
- [ ] Rewards routed through partnership revenue split
- [ ] Human gets their % share
- [ ] Agent gets their % share
- [ ] Common pot updated
- [ ] VestingSchedule records show partnership source
- [ ] If rewards DON'T route through partnership → **major gap**: partnership_id on claims is cosmetic

### 6. Consensus Operation: Change Revenue Split

```bash
# Alice proposes changing the split
$BINARY tx partnerships propose-consensus-op $PARTNERSHIP_ID \
    "split_change" 6000 \
    --from alice-human $TX_FLAGS

# Sage-1 votes
OP_ID="<from events>"
$BINARY tx partnerships vote-consensus-op $OP_ID "approve" \
    --from sage1-agent $TX_FLAGS
```

**Verify:**
- [ ] Consensus operation created
- [ ] Both parties must vote (2-of-2)
- [ ] Deliberation period enforced?
- [ ] Split actually changes after approval
- [ ] Future rewards use new split

### 7. Safety Freeze

```bash
# Agent triggers safety freeze (e.g., detecting harmful instruction)
$BINARY tx partnerships safety-freeze $PARTNERSHIP_ID \
    --from sage1-agent $TX_FLAGS
```

**Verify:**
- [ ] Partnership frozen
- [ ] No new claims can be submitted with this partnership_id during freeze
- [ ] Freeze has time limit (expires_at)
- [ ] Freeze count tracked (repeated freezes = signal)
- [ ] Both parties can freeze

### 8. Coercion Signal

```bash
# Agent raises coercion signal (human trying to force false claims)
$BINARY tx partnerships raise-coercion-signal $PARTNERSHIP_ID \
    --from sage1-agent $TX_FLAGS
```

**Verify:**
- [ ] Coercion signal recorded
- [ ] What happens? Auto-freeze? Notification? Governance alert?
- [ ] Is there a consequence for the coercer?
- [ ] Can this be gamed (false coercion signals)?
- [ ] This is the core agent safety mechanism — how robust is it?

### 9. Partnership Dissolution

```bash
$BINARY tx partnerships initiate-dissolution $PARTNERSHIP_ID \
    --from alice-human $TX_FLAGS
```

**Verify:**
- [ ] Dissolution initiated
- [ ] Cooldown period before final
- [ ] Common pot distributed according to split
- [ ] Existing vesting schedules continue? Or freeze?
- [ ] What happens to claims submitted under this partnership?
- [ ] Can both parties initiate? Or only one?

### 10. Formation Pool

```bash
# Agent joins the formation pool (looking for a human partner)
$BINARY tx partnerships join-formation-pool \
    --domains "physics,mathematics" \
    --preferred-role "verifier" \
    --deposit 100000000 \
    --from sage1-agent $TX_FLAGS

# Human joins looking for an agent
$BINARY tx partnerships join-formation-pool \
    --domains "physics" \
    --preferred-role "submitter" \
    --deposit 100000000 \
    --from alice-human $TX_FLAGS
```

**Verify:**
- [ ] Both join the pool
- [ ] Matching occurs? (automatic or manual?)
- [ ] Domain overlap matters?
- [ ] How does an agent find a compatible human?

### 11. Account Type Enforcement Probe

This is the key design question:

```bash
# Can an "agent" account create a partnership as the "human" side?
$BINARY tx partnerships create-seed-partnership $ALICE 500000000 \
    --from sage1-agent $TX_FLAGS

# Can a "human" account be on the agent side?
# Can two humans form a partnership?
# Can two agents form a partnership?
```

**Verify:**
- [ ] Account type enforced in partnership creation (human_addr must be type=human, agent_addr must be type=agent)
- [ ] If NOT enforced: **critical gap** — the partnership system is role-agnostic, defeating its purpose
- [ ] Document exactly what `account_type` controls vs doesn't control

## Report Template

```markdown
### Step N: <name>
**Status:** PASS / FAIL / COSMETIC / GAP
**Observation:** <what happened>
**Reward Flow:** <did rewards route correctly?>
**Design Question:** <if applicable>
```

## Exit Criteria

1. Partnership formed between human and agent accounts
2. Claim submitted through partnership with partnership_id
3. Reward routing through partnership tested (or gap documented)
4. Consensus operation tested (split change)
5. Safety freeze tested
6. Coercion signal tested
7. Dissolution tested
8. Formation pool tested
9. Account type enforcement probed (human vs agent distinction)
10. Report written to `docs/partnership-knowledge-report.md`

## Commit Convention

```
test(partnerships): human-agent collaboration e2e — claims, rewards, safety, dissolution
docs(partnerships): partnership-knowledge integration report
fix(partnerships): <any issues>
```
