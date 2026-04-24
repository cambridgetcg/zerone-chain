# R25-5 Research & Bounties: Incentivised Truth Discovery — E2E Report

**Date:** 2026-02-26
**Chain:** `zerone-localnet` (4 validators, Cosmos SDK v0.50.15)
**Modules tested:** `x/research`, `x/tree`, `x/vesting_rewards`

---

## Step 1: Submit Research

**Status:** PASS

**Command:**
```bash
zeroned tx research submit \
    "Replication study of water boiling point claim" \
    "Independent measurement of water boiling point under standard conditions" \
    "general" 10000000 \
    --from researcher1
```

**Observation:**
- Research `RES-1` created at block 34 with status `submitted`
- Stake of 10,000,000 uzrn (10 ZRN) escrowed from researcher1
- Researcher balance dropped from 500M to ~489.9M (10M stake + gas)
- Domain set to `general`
- Supporting facts available via `--supporting-facts` flag (not tested)

**Economics:**
- Min research stake: 1,000,000 uzrn (1 ZRN)
- Stake escrowed to research module account

**CLI correction:** Actual command is `submit` (not `submit-research`). Takes 4 positional args: `[title] [description] [domain] [stake]`. No `research_type` or `target_fact_id` positional args — research type is not currently settable via CLI.

---

## Step 2: Peer Review

**Status:** PASS

**Command:**
```bash
zeroned tx research review RES-1 1 "Methodology is sound" 85 --from val0
zeroned tx research review RES-1 1 "Replicated successfully" 90 --from val1
zeroned tx research review RES-1 1 "Sound methodology" 88 --from val2
```

**Observation:**
- 3 reviews submitted, all with verdict `approve` (1)
- Status transitioned from `submitted` → `under_review` after first review
- Aggregate score computed: 87 (average of 85, 90, 88)
- Review count: 3, approve count: 3, reject count: 0
- Each review stored with unique ID (REV-1, REV-2, REV-3)
- Duplicate reviewer prevention confirmed (not explicitly tested but keeper has `HasReviewerReviewed`)

**Economics:**
- No reviewer stake required (reviews are free)
- Min reviews for resolution: 3 (`min_reviewer_count`)
- Acceptance threshold: 70 (`acceptance_score_threshold`)
- Verdict options: 0=unspecified, 1=approve, 2=reject, 3=revise

**CLI correction:** Actual command is `review` (not `review-research`). Verdict is numeric (1=approve), not string ("approve").

**Who can review?** Any account can submit a review — no validator or qualification check in the handler. This is a design gap: reviews should likely be restricted to qualified reviewers or validators.

---

## Step 3: Resolve Research

**Status:** BLOCKED

**Command:**
```bash
zeroned tx research resolve RES-1 --from val0
```

**Observation:**
- **Error:** `unauthorized` — resolve is authority-only (governance module address)
- The CLI command exists but can only be called with governance module signing authority
- No mechanism exists to resolve research without a governance proposal
- RES-1 remains stuck in `under_review` status with 3 approvals and score 87 (above threshold of 70)

**Issue: CRITICAL — resolve-research requires governance authority**

The `MsgResolveResearch` handler checks `msg.Authority == k.authority` where authority is the governance module address. There is no way to resolve research via CLI without submitting a governance proposal, making the entire research resolution flow non-functional for normal operations.

**Recommendation:** Either:
1. Allow auto-resolution when `min_reviewer_count` is met and `review_period_blocks` has elapsed (via EndBlocker)
2. Allow a designated "research authority" role
3. Allow the original submitter or any reviewer to trigger resolution after minimum reviews

---

## Step 4: Challenge Research

**Status:** PASS

**Command:**
```bash
zeroned tx research submit "Questioning thermodynamics" \
    "Investigation suggesting perpetual motion is possible" \
    "general" 10000000 --from researcher1

zeroned tx research challenge RES-2 \
    "This contradicts well-established thermodynamics" 10000000 \
    --from val0
```

**Observation:**
- RES-2 created at block 94 with status `submitted`
- Challenge from val0 transitions status to `challenged`
- Challenge stake (10M uzrn) escrowed
- Challenge can only target `submitted` or `under_review` research
- Challenge reason recorded

**Economics:**
- Min challenge stake: 1,000,000 uzrn (1 ZRN)
- Rejection slash: 33% (`rejection_slash_bps` = 330000)

---

## Step 5: Create a Bounty

**Status:** PASS

**Command:**
```bash
zeroned tx research create-bounty \
    "Measure water boiling point at 3000m altitude" \
    "Empirical measurement needed" \
    50000000 50111 \
    --from val0
```

**Observation:**
- BOUNTY-1 created at block 112 with status `open`
- Reward of 50,000,000 uzrn (50 ZRN) escrowed to research module
- Deadline set to block 50111 (current + 50000)
- Anyone can view the bounty via `zeroned query research bounty BOUNTY-1`

**Economics:**
- Max bounty reward: 10,000,000,000 uzrn (10,000 ZRN)
- Min deadline offset: 34,272 blocks (~1 day at 2.5s blocks)
- Reward locked in research module account

**CLI correction:** Deadline is a positional arg `[deadline-height]` (absolute block height), not a `--deadline-blocks` flag (relative offset). No `--domain` field in the CLI args, though the prompt suggests one.

---

## Step 6: Claim and Fulfil a Bounty

**Status:** PARTIAL — Claim PASS, Fulfil BLOCKED

**Claim:**
```bash
zeroned tx research claim-bounty BOUNTY-1 --from researcher1
```

**Observation:**
- Bounty status: `open` → `claimed`
- `claimed_by` set to researcher1 address
- Claim is exclusive (only one claimer at a time)
- If deadline passes while claimed: status resets to `open` (claimer cleared, re-claimable)
- Optional `--fact-ids` flag for attaching supporting facts

**Fulfil:**
```bash
zeroned tx research fulfill-bounty BOUNTY-1 <claimer-addr> --from val0
# Error: unauthorized
```

**Issue: CRITICAL — fulfill-bounty requires governance authority**

Same issue as resolve-research. The `MsgFulfillBounty` handler requires governance module authority. There is no mechanism for a bounty creator or reviewer to approve fulfilment.

**Recommendation:** Add a bounty review mechanism where:
1. The claimer submits evidence/deliverable
2. The creator or designated reviewers approve
3. Auto-fulfil on sufficient approvals

**Bounty deadline enforcement:**
- Open bounties past deadline → `expired`, reward returned to creator
- Claimed bounties past deadline → cleared back to `open` (allows re-claim)
- Implemented in `BeginBlock` via `keeper.ExpireBounties()`

---

## Step 7: Fund Research

**Status:** PASS

**Command:**
```bash
zeroned tx research fund 25000000 --from val0
```

**Observation:**
- Research treasury balance: 0 → 25,000,000 uzrn
- Funds transferred from val0 to research module account
- Treasury is a single global pool (not per-research)
- Released to researcher on successful resolution (untestable due to resolve authority issue)

**CLI correction:** Command is `fund` (not `fund-research`). Takes 1 arg `[amount]`. Does NOT take a research ID — funds the treasury, not a specific research.

---

## Step 8: Vesting Rewards Deep Dive

**Status:** PASS

### Parameters

| Parameter | Value | Description |
|-----------|-------|-------------|
| `block_reward` | 10,000,000 uzrn | Base block reward (10 ZRN) |
| `reward_decay_bps` | 994478 | ~1-year half-life per 100K-block epoch |
| `floor_reward` | 100,000 uzrn | Minimum reward (0.1 ZRN) |
| `vesting_enabled` | true | Active |
| `released_clawback_rate` | 3300 | 33% of released clawed back on falsification |
| `min_validators_for_full_reward` | 22 | Below this: linearly scaled |

### Revenue Split (Block Rewards)

| Recipient | BPS | % of Total |
|-----------|-----|-----------|
| **Contributor** (block producer) | 550,000 | 55% |
| **Protocol** | 220,000 | 22% |
| → Citation pool | 50% of protocol | 11% |
| → Verification pool | 30% of protocol | 6.6% |
| → Treasury | 20% of protocol | 4.4% |
| **Research fund** | 33,300 | 3.33% |
| → Founder share | 7% of research | 0.23% |
| **Development fund** | 196,700 | 19.67% |

### Category Vesting Configs

| Category | Half-Life (blocks) | Cliff (blocks) | Max Release | Reserve |
|----------|-------------------|----------------|-------------|---------|
| axiomatic | 1,111,111 (~32d) | 11,111 | 95% | 5% |
| formal_proof | 555,555 (~16d) | 5,555 | 92% | 8% |
| on_chain | 222,222 (~6.5d) | 1,111 | 90% | 10% |
| cryptographic | 222,222 (~6.5d) | 3,333 | 90% | 10% |
| computational | 333,333 (~9.7d) | 2,222 | 88% | 12% |
| peer_reviewed | 111,111 (~3.2d) | 5,555 | 85% | 15% |
| replicated | 111,111 (~3.2d) | 3,333 | 88% | 12% |
| oracle_feed | 55,555 (~1.6d) | 555 | 80% | 20% |
| attestation | 77,777 (~2.3d) | 2,222 | 80% | 20% |
| contested | 22,222 (~0.6d) | 1,111 | 60% | 40% |

### Active Vesting Schedules

4 schedules found (from genesis axiom verification rewards):

| ID | Recipient | Total | Claimable | Category | Source |
|----|-----------|-------|-----------|----------|--------|
| `0db6a4ca...` | `zrn16y70...` | 1,000,000 | 166,373 | peer_reviewed | verification |
| `7d5aad33...` | `zrn18xym...` | 1,000,000 | 166,472 | peer_reviewed | verification |
| `bb0f14b5...` | `zrn1m54e...` | 1,000,000 | 166,456 | peer_reviewed | verification |
| `c75144f4...` | `zrn14xwp...` | 1,000,000 | 166,462 | peer_reviewed | verification |

Each schedule:
- Total: 1,000,000 uzrn (1 ZRN verification reward)
- Reserve: 150,000 uzrn (15% = 1 - 85% max_release for peer_reviewed)
- Cliff ends at block ~5623 (cliff_blocks = 5555)
- Claimable: ~166K uzrn (16.6% of total — vesting is progressing past cliff)

### Vesting Economics Answers

| Question | Answer |
|----------|--------|
| Reward for verified claim | 1,000,000 uzrn per verifier (configurable via `verification_reward` in knowledge params) |
| How long until first payout? | Depends on category cliff: peer_reviewed = 5,555 blocks (~3.9 hours at 2.5s) |
| What triggers clawback? | `FalsifyClaim` tx — claws back 33% of released + all unvested + reserve |
| Does reserve ever release? | No — reserve is permanent. Forfeited to challenger on falsification |

### Founder Share

| Setting | Value |
|---------|-------|
| `founder_share_bps` | 70,000 (7% of research fund) |
| `founder_address` | "" (not set, disabled) |
| Immutable? | Yes — once set, cannot be changed via governance |

---

## Step 9: Block Reward Distribution

**Status:** FAIL — No block rewards being minted

**Observation:**
- `block-reward` query returns `{}` for all block heights (including blocks with transactions)
- Total supply remains at 121,152,222,000,000 uzrn (genesis allocation only)
- Research fund module balance: 0 uzrn
- Development fund balance: 1,180,200 uzrn (from fee routing only)

**Issue: CRITICAL — Block rewards are never minted**

**Root cause:** `SetBlockTxCount()` is never called from the app layer.

The vesting_rewards module's `BeginBlock` checks:
```go
hasTransactions := am.keeper.GetBlockTxCount() > 0 && activeValidatorCount > 0
```

But `blockTxCount` (an in-memory field on the keeper) defaults to 0 and is never set. The `SetBlockTxCount(count int)` method exists but is dead code — no caller in `app/abci.go` or anywhere else.

Since `EmptyBlockRewardRate = 0` (pure PoT), blocks with `hasTransactions=false` get 0 reward. This means:
- No new ZRN is ever minted
- Block producers receive nothing
- Research fund receives nothing
- Development fund only receives transaction fee routing
- The entire inflationary reward system is inoperative

**Fix:** Add `app.VestingRewardsKeeper.SetBlockTxCount(len(req.Txs))` in `PotPreBlocker()`.

---

## Step 10: Tree Module — Project Creation

**Status:** PASS (with bugs)

**Command:**
```bash
zeroned tx tree create-project \
    "Altitude Boiling Point Database" \
    "Comprehensive database of water boiling points at various altitudes" \
    --from val0
```

**Observation:**
- Project `proj-207-1` created in `seed` phase
- Founder set to val0 address
- Budget: empty (no budget positional arg in CLI)
- Task list: empty
- Gas estimate bug: auto-estimate returned 71665 but minimum was 80000 → required manual `--gas 200000`

**Issue: NON-DETERMINISM — Project missing on val0**

The project was created successfully (tx code=0, event emitted) but querying on val0 (gRPC 9090) returns "project not found". Querying val1/val2/val3 returns the project correctly. All validators are at the same block height. This indicates a state divergence on val0 — a potential consensus/non-determinism bug.

This did NOT cause a chain halt, which suggests either:
1. The AppHash computation doesn't include this state, or
2. The divergence occurred in a non-consensus layer (cache, query service)

**Further investigation needed** to determine if this is a persistent state issue or a query-level cache bug.

**CLI notes:**
- Command takes only 2 args: `[name] [description]`
- No domain or budget positional args (though message supports them)
- First task used in gas estimation should account for the 80K minimum

---

## Step 10b: Add Task

**Status:** PASS

**Command:**
```bash
zeroned tx tree add-task proj-207-1 \
    "Collect measurements from 0-5000m" \
    "Systematic measurements every 500m" \
    25000000 --from val0
```

**Observation:**
- Task `task-254-1` created with status `open`
- Bounty of 25,000,000 uzrn escrowed from val0 to tree module
- Task linked to project `proj-207-1`
- Query confirmed on val1 (val0 has the state divergence issue)

---

## Step 11: Service Deployment

**Status:** FAIL — CLI arg mapping bug

**Command:**
```bash
zeroned tx tree deploy-service proj-207-1 \
    "Boiling Point API" \
    "Query altitude-adjusted boiling points" \
    "https://api.boiling-points.example.com" \
    1000 --from val0
```

**Observation:**
Service `svc-268-1` deployed, but fields are **misaligned**:

| Expected | Actual Value | Should Be |
|----------|-------------|-----------|
| `name` | `proj-207-1` | `Boiling Point API` |
| `description` | `Boiling Point API` | `Query altitude-adjusted boiling points` |
| `contract_address` | `Query altitude-adjusted boiling points` | `https://api.boiling-points.example.com` |
| `price_per_call` | `https://api.boiling-points.example.com` | `1000` |

**Issue: BUG — deploy-service CLI arg mapping off-by-one**

The CLI usage says `deploy-service [project-id] [name] [description] [endpoint] [price-per-call]` (5 args), but the message builder maps:
```go
msg := &types.MsgDeployService{
    Name:         args[0],  // gets project-id
    Description:  args[1],  // gets name
    Endpoint:     args[2],  // gets description
    PricePerCall: args[3],  // gets endpoint
}
// args[4] (price-per-call) is IGNORED
```

`MsgDeployService` proto has NO `ProjectId` field, so the CLI advertises a field that doesn't exist. The 5th arg (actual price) is silently dropped.

**Issue: Service not linked to project**

Even if the CLI were fixed, `DeployService` handler doesn't add the service to the project's `ServiceIds` list. Services are standalone entities with no project relationship in the msg handler.

---

## Step 12: Opportunity Detection

**Status:** BLOCKED — No CLI commands

**Observation:**
- `detect-opportunity` and `begin-seeding` are defined in the proto Msg service (23 total RPCs)
- Only 4 CLI tx commands are registered: `create-project`, `add-task`, `deploy-service`, `call-service`
- The remaining 19 message types (including opportunity detection, task assignment, deliverable submission, project lifecycle transitions) have no CLI wrappers
- Programmatic access via gRPC/REST is possible but not tested

---

## Module Account Balances (End of Test)

| Module | Balance (uzrn) | Notes |
|--------|---------------|-------|
| `research` | 105,000,000 | 2x10M research stakes + 10M challenge + 50M bounty + 25M treasury |
| `knowledge` | 3,300,000 | Verification pool from fee routing |
| `development_fund` | 1,180,200 | Fee routing only (no block rewards minted) |
| `research_fund` | 0 | No block rewards minted |
| `compute_pool` | 0 | No verification pool distribution |

---

## Bugs Found

### Critical

1. **Block rewards never minted** — `SetBlockTxCount()` never called from app layer, so `hasTransactions` is always false and `EmptyBlockRewardRate=0` means zero reward. The entire inflationary reward + revenue distribution system is inoperative.

2. **resolve-research requires governance authority** — Research submissions can never be resolved via CLI. The flow (submit → review → resolve) is broken because resolve requires the governance module address to sign. No auto-resolution mechanism exists.

3. **fulfill-bounty requires governance authority** — Same issue. Bounties can be created and claimed but never fulfilled. Reward distribution is impossible without governance proposal.

### High

4. **Tree module non-determinism on val0** — Project `proj-207-1` exists on val1/val2/val3 but not val0, despite all validators being at the same block height. Potential consensus bug.

5. **deploy-service CLI arg mapping off-by-one** — The CLI advertises `[project-id]` as the first arg but `MsgDeployService` has no ProjectId field. All args shift by one position, the 5th arg (actual price) is dropped.

### Medium

6. **Service not linked to project** — `DeployService` handler creates a standalone service entity. It doesn't update the project's `ServiceIds` list, so project ↔ service relationship is lost.

7. **Missing CLI commands for tree module** — 19 of 23 message types have no CLI wrappers (task assignment, deliverable submission, project lifecycle transitions, opportunity detection, etc.)

8. **Gas estimate below minimum** — `create-project` gas estimate returns ~71K but the minimum per-message gas is 80K, causing automatic rejection. Users must manually set `--gas 200000`.

### Low

9. **No reviewer qualification check** — Any account can review research. No validator check, no reputation threshold, no domain qualification.

10. **Research type not settable via CLI** — The `research_type` field exists in the state but is not exposed as a CLI flag or positional arg.

11. **Fund research is global, not per-research** — The `fund` command adds to a global treasury, not to a specific research submission. The link between funding and research outcomes is unclear.

---

## Economic Loop Analysis

**Intended flow:** `claim → verify → vest → claim rewards`

| Stage | Status | Blocker |
|-------|--------|---------|
| Submit knowledge claim | WORKS | — |
| Verification round (commit/reveal) | WORKS | — |
| Claim accepted → vesting schedule created | WORKS | 4 schedules found |
| Vesting accumulation (half-life curve) | WORKS | Claimable amounts increasing |
| Claim vested rewards | UNTESTED | No test key matches recipient addresses |
| Block rewards minted | BROKEN | `SetBlockTxCount` never called |
| Revenue distribution (4-way split) | PARTIAL | Fee routing works, block rewards don't |
| Research submission → resolution | BROKEN | Resolve requires governance |
| Bounty creation → fulfilment | BROKEN | Fulfil requires governance |
| Falsification → clawback | UNTESTED | Requires falsify tx (not tested) |

**Conclusion:** The core vesting mechanism works (schedules are created, amounts accrue via half-life curve, reserves are computed). But the economic loop is broken at two critical points:
1. No new ZRN is ever minted (block rewards dead)
2. Research/bounty resolution requires governance authority (manual resolution impossible)

---

## Recommendations

1. **Wire `SetBlockTxCount` in PreBlocker** — Add `app.VestingRewardsKeeper.SetBlockTxCount(len(req.Txs))` in `PotPreBlocker()` to enable block rewards.

2. **Add EndBlocker auto-resolution for research** — When `min_reviewer_count` reviews are met AND `review_period_blocks` has elapsed, auto-resolve based on aggregate score vs threshold.

3. **Add bounty fulfil mechanism** — Allow bounty creator (or consensus of reviewers) to approve fulfilment without governance.

4. **Fix deploy-service CLI** — Either add `ProjectId` to the proto message or remove it from the CLI usage and adjust arg mapping.

5. **Add missing tree CLI commands** — Priority: `assign-task`, `submit-deliverable`, `approve-deliverable`, `detect-opportunity`.

6. **Investigate val0 state divergence** — Check if the tree module's counter or storage has non-deterministic behavior.

7. **Add reviewer qualification** — Gate research reviews behind validator status, reputation score, or domain qualification.
