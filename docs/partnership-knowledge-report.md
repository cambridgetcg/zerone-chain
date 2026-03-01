# Partnership-Knowledge Integration Report

**Date:** 2026-02-26
**Chain:** zerone-localnet (4 validators)
**Binary:** zeroned (built with partnership CLI wiring fix)

---

## Summary

The partnerships module (`x/partnerships`) is substantially implemented with full lifecycle
management (propose, accept, freeze, coercion, dissolve), formation pool, and seed partnerships.
However, the **integration with the knowledge module is purely cosmetic** — `partnership_id` on
claims is stored as metadata but never validated, and reward routing through partnerships is
not wired.

### Critical Findings

| # | Finding | Severity |
|---|---------|----------|
| 1 | `partnership_id` on claims is cosmetic — no validation, no reward routing | **CRITICAL** |
| 2 | No account type enforcement — agents can be "human", humans can be "agent" | **CRITICAL** |
| 3 | `split_change` consensus op is a stub — approved but never executes | **HIGH** |
| 4 | Partnership CLI was not wired (`GetTxCmd`/`GetQueryCmd` returned nil) | **HIGH** (fixed) |
| 5 | `invest` and `tier_upgrade` consensus ops are stubs | **MEDIUM** |
| 6 | Ante handler references stale partnership message type URLs | **MEDIUM** |
| 7 | Consensus ops expire in 22 blocks (micro-tier) — hard to use interactively | **LOW** |
| 8 | Seed partnerships don't graduate to full partnerships | **LOW** |

---

## Step-by-Step Results

### Step 1: Create Human and Agent Accounts
**Status:** PASS

Created `alice-human` (type: human) and `sage1-agent` (type: agent) via `zerone_auth register-account`.
Both funded with 1,000 ZRN. Registration requires:
- 64-char hex Ed25519 public key
- DID derived from first 32 hex chars of pubkey (`did:zrn:{pubkey[:32]}`)
- Account type: human, agent, contract, or system

**Addresses:**
- Alice: `zrn1yuu58l6wjsndjmqrk3d8pu4eckmh9fedlgsj5p` (human)
- Sage1: `zrn1a34k743pj3ahh98vy700cvz69xxwe0khvlfypj` (agent)

### Step 2: Form a Seed Partnership
**Status:** PASS (with design note)

```
zeroned tx partnerships create-seed <agent> <human-contribution>
```

- Seed partnership `seed-1` created successfully
- **Design note:** Seed partnerships are a **separate data type** from full partnerships.
  They do not produce a `partnership_id` usable on claims. There is no graduation path
  from seed to full partnership. The seed partnership expires after `seed_partnership_duration`
  (10,000 blocks) and is simply cleaned up.
- The `create-seed` command does NOT enforce account types — an agent can call it as the
  "human" side

### Step 3: Full Partnership (Propose + Accept)
**Status:** PASS

Full partnership requires two-step flow:
1. `propose <partner> <deposit> <tier>` — creates partnership in `pending` status
2. `accept <partnership-id> <deposit>` — activates the partnership

**Partnership `partnership-2` created:**
- Status: `active`
- Common pot: 200,000,000 uzrn (100 ZRN from each side)
- Split: 50/50 (500,000 bps each, matching `default_human_split_bps` / `default_agent_split_bps`)
- Cooperation score: 500,000
- Lock tier: 1, lock_expires_at: 77,865

### Step 4: Submit Claim Through Partnership
**Status:** COSMETIC

```
zeroned tx knowledge submit-claim <content> <domain> <category> <stake> --partnership-id partnership-2
```

- Claim submitted successfully with `partnership_id: partnership-2` in the TX body
- The `partnership_id` is stored on the claim record in state
- **GAP: No validation occurs.** The knowledge keeper does not:
  - Check that the partnership exists
  - Check that the partnership is active (not frozen/dissolved)
  - Check that the submitter is a participant in the partnership
  - Any arbitrary string is accepted as `partnership_id`
- Claim submitted during a safety freeze **also succeeded** — the frozen partnership
  status is completely ignored by the knowledge module

### Step 5: Reward Routing Through Partnership
**Status:** GAP (not implemented)

- `DistributeReward` exists as a keeper method on the partnerships module but is **never
  called** by any other module
- The knowledge module's `CompleteRound` / reward distribution logic has **no reference**
  to partnerships at all
- There is no `KnowledgeKeeper` interface in the partnerships module's `expected_keepers.go`
- There is no cross-module call in either direction
- **This means `partnership_id` on claims is purely cosmetic metadata with zero functional
  impact on reward flow**

### Step 6: Consensus Operation (Split Change)
**Status:** PARTIAL (propose works, execution is stub)

**Propose:**
```
zeroned tx partnerships propose-op partnership-2 split_change 6000 "Adjusting split"
```
- Operation `op-3` created successfully
- Deliberation window: 22 blocks (micro-tier, amount 6000 vs pot 200M = <1%)
- **Bug found:** Gas estimation with `--gas auto --gas-adjustment 1.5` was insufficient
  (used 66,756 vs wanted 65,551 = out of gas). Required `--gas 200000` explicitly.

**Vote:**
```
zeroned tx partnerships vote-op partnership-2 op-3 true
```
- Vote succeeded (code: 0)
- **GAP: `split_change` branch in VoteConsensusOp is a stub** — the operation is marked
  as approved but the split values on the partnership are never updated
- Similarly, `invest` and `tier_upgrade` branches are comment-only stubs
- Only `withdraw` is actually implemented

### Step 7: Safety Freeze
**Status:** PASS

```
zeroned tx partnerships safety-freeze partnership-2
```

- Partnership status changed: `active` → `suspended`
- Cooperation score decreased: 500,000 → 499,900 (100 bps penalty)
- Both parties can trigger a freeze (agent used here)
- Freeze has a time limit: `safety_freeze_duration_blocks` = 500 blocks (from params)
- Freeze count tracked: `max_freezes_per_epoch` = 3

**Freeze blocks operations:**
- Consensus ops cannot be proposed during freeze (checked in `ProposeConsensusOp`)
- **BUT** claim submission with the frozen partnership_id still succeeds (knowledge module
  doesn't check)

### Step 8: Coercion Signal
**Status:** PASS

```
zeroned tx partnerships raise-coercion partnership-2
```

- Coercion signal `coercion-4` raised successfully
- Partnership remains `suspended` (was already suspended from freeze)
- Signal has a review period: `coercion_review_blocks` = 2,000 blocks
- **Design observation:** There is no visible consequence for the coercer beyond the
  suspension. No governance alert, no notification mechanism. The coercion signal is a
  record in state that expires after the review period.

### Step 9: Partnership Dissolution
**Status:** PASS

```
zeroned tx partnerships dissolve partnership-2
```

- Status changed: `suspended` → `cooling`
- Common pot drained: 200,000,000 → 0 (distributed via exit settlement)
- Exit state recorded:
  - `initiated_by`: Alice
  - `initiated_at`: block 407
  - `cooldown_end`: block 5,407 (5,000 blocks cooling period)
- Both parties can initiate dissolution
- The initiator receives a penalty (asymmetric exit settlement)
- The `SettleCoolingPartnerships` EndBlock hook will finalize after cooldown

### Step 10: Formation Pool
**Status:** PASS

```
zeroned tx partnerships join-formation <deposit> --domains <d1,d2> --preferred-role <role>
zeroned tx partnerships leave-formation
```

- Both agent and human successfully joined the pool
- Pool entries include: address, deposit, domains, preferred_role, expires_at, status
- Deposit is escrowed on join and refunded on leave
- Pool cap: 222 entries
- TTL: ~11,000 blocks (formation_window_blocks from params, + block of registration)
- **No automatic matching** — the pool is a passive registry. Matching would need to be
  implemented externally or via governance.

### Step 11: Account Type Enforcement
**Status:** FAIL (no enforcement)

All of the following tests **succeeded** (should have been rejected):

| Test | Expected | Actual |
|------|----------|--------|
| Agent proposes partnership as "human" side | REJECT | **PASS** |
| Agent creates seed partnership as "human" side | REJECT | **PASS** |
| Agent↔Agent partnership | REJECT | **PASS** |
| Partnership with unregistered account | REJECT | **PASS** |

The partnership module does not reference `zerone_auth` account types at all. The `human_addr`
and `agent_addr` fields on partnerships are set solely based on who proposes vs who accepts —
there is no cross-module check that `human_addr` actually has `account_type = "human"`.

This means the entire human-agent distinction in partnerships is purely nominal and can be
trivially circumvented.

---

## Code-Level Findings

### Partnership CLI Not Wired (Fixed)
**File:** `x/partnerships/module.go:72-79`

`GetTxCmd()` and `GetQueryCmd()` both returned `nil`, making all partnership CLI commands
unreachable. **Fixed** by returning `cli.NewTxCmd()` and `cli.NewQueryCmd()`.

### Ante Handler Stale Message Types
**File:** `app/ante_zerone.go:469-476, 900-909`

The gas cost table and `isPartnershipMsg` function reference old/renamed message types:
- `MsgInitiatePartnership` → should be `MsgProposePartnership`
- `MsgDepositToPot` → removed (deposits happen via propose/accept)
- `MsgProposeOperation` → should be `MsgProposeConsensusOp`
- `MsgApproveOperation` / `MsgRejectOperation` → should be `MsgVoteConsensusOp`
- `MsgInitiateExit` → should be `MsgInitiateDissolution`
- Missing: `MsgCreateSeedPartnership`, `MsgJoinFormationPool`, `MsgLeaveFormationPool`,
  `MsgRaiseCoercionSignal`

### Consensus Op Execution Stubs
**File:** `x/partnerships/keeper/msg_server.go:353-358`

```go
case "invest":
    // invest adds to pot (funds already deposited)
case "split_change":
    // split_change would update splits
case "tier_upgrade":
    // tier_upgrade would update tier
```

Only `withdraw` has actual implementation.

### No Cross-Module Integration
- Knowledge module has no `PartnershipKeeper` interface
- Partnership module has no `KnowledgeKeeper` interface
- `DistributeReward` is a helper that is never called externally
- Reward flow from knowledge verification → partnership split is completely missing

---

## Partnership Module Parameters

```json
{
  "formation_window_blocks": "1000",
  "cooling_period_blocks": "5000",
  "common_pot_share_bps": "100000",
  "safety_freeze_duration_blocks": "500",
  "max_freezes_per_epoch": 3,
  "coercion_review_blocks": "2000",
  "base_cooldown_blocks": "100",
  "max_counter_proposal_depth": 3,
  "default_human_split_bps": "500000",
  "default_agent_split_bps": "500000",
  "min_partnership_stake": "1000000",
  "seed_partnership_duration": "10000",
  "seed_common_pot_cap": "100000000"
}
```

---

## Recommendations

### P0 (Must Fix)
1. **Wire partnership-knowledge reward routing** — When a claim with `partnership_id` is
   verified and rewards are distributed, call `partnerships.DistributeReward` to split
   rewards according to the partnership's `split_human_bps` / `split_agent_bps`
2. **Validate `partnership_id` on claim submission** — Check that the partnership exists,
   is active (not frozen/dissolved), and the submitter is a participant
3. **Enforce account types** — Add a `ZeroneAuthKeeper` to the partnerships module and
   verify `human_addr` has `account_type=human` and `agent_addr` has `account_type=agent`
   in `ProposePartnership`, `AcceptPartnership`, and `CreateSeedPartnership`

### P1 (Should Fix)
4. **Implement `split_change` execution** — When approved, update `split_human_bps` and
   `split_agent_bps` on the partnership record
5. **Update ante handler** — Fix stale message type URLs in gas cost table and
   `isPartnershipMsg`
6. **Implement seed → full partnership graduation** — Currently seeds expire and are
   cleaned up with no promotion path

### P2 (Nice to Have)
7. **Add formation pool matching** — Currently passive; consider automated matching based
   on domain overlap
8. **Implement `invest` and `tier_upgrade` ops** — Complete the stub branches
9. **Add mentorship CRUD** — `MentorshipConfig` has store keys but no handlers
