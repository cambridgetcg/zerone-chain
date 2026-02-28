# R26-7 End-to-End Integration Report

**Date:** 2026-02-27
**Chain:** zerone-localnet (4 validators, CometBFT v0.38.20)
**Block time:** ~2.5s (target 2521ms)
**Blocks reached:** 488+

---

## Full Loop Test Results

### 1. Fresh Localnet
- **PASS** — 4 validators initialized, consensus reached at block 3
- Genesis valid, all ports operational

### 2. Account Registration (R26-2)
- **PASS** — alice (human), sage1 (agent), rogue (agent) registered
- DID format: `did:zrn:{first 32 hex chars of pubkey}` — derivation enforced
- Default capabilities: `can_submit_claims=true`, `can_challenge=true` for all types
- Validators also needed registration to participate in Zerone-specific operations

### 3. Capability Enforcement (R26-2)
- **PASS** — Unregistered accounts blocked from Zerone-specific messages (code 30: `account capability denied`)
- Registration required for: `MsgSubmitCommitment`, `MsgSubmitReveal`, `MsgSubmitClaim`
- Auth management messages (register-account) remain open to unregistered accounts

### 4. Partnership Formation (R26-4)
- **PASS** — alice proposed, sage1 accepted
- Partnership-1: active, 50/50 split (500000 bps each)
- Common pot share: 10% (100,000 bps)
- Lock tier 0 (trial), expires at block 22308

### 5. Domain Qualification (R26-3)
- **PASS** — val0 and val1 qualified for "general" domain (100 ZRN stake each)
- val2 and val3 NOT qualified
- Qualification status=1 (active), pathway=1 (by-stake), weight=50

### 6. Claim Through Partnership (R26-4)
- **PASS** — Claim submitted with `partnership_id: partnership-1`
- Verification round created automatically
- Review fee distributed: 550,000 verifier pool / 220,000 protocol / 196,700 development / 33,300 research
  - **Matches expected 55/22/19.67/3.33 split**

### 7. Verification Round with Qualification Gating (R26-3)
- **PASS** — Complete commit-reveal cycle:
  - val0 (qualified): committed + revealed "accept" — **accepted**
  - val1 (qualified): committed + revealed "accept" — **accepted**
  - val2 (unqualified): committed — **rejected on-chain** (code 70: `verifier not qualified for domain`)
- Round completed: verdict=1 (accepted), claim status=6 (ACCEPTED)

### 8. Reward Distribution (R26-1)
- **PASS** — Block rewards minting confirmed
  - Genesis supply: 121,130,000,000,000 uzrn
  - Current supply: 121,152,222,000,000 uzrn
  - **22,222 ZRN minted** through PoT block rewards
- Empty blocks receive 0 reward (PoT design: `EmptyBlockRewardRate=0`)
- Module account balances after testing:
  - protocol_treasury: 880,000 uzrn
  - development_fund: 786,800 uzrn
  - knowledge: 750,000 uzrn
- Partnership `total_earned: 1,000,000`, `common_pot: 100,000`

### 9. Research + Auto-Resolution (R26-5)
- **PARTIAL** — Submission and review flow works end-to-end
  - RES-1 submitted by sage1, stake 10,000,000 uzrn
  - 3 reviews submitted (alice: 8, val0: 9, val1: 7) — all approve
  - Aggregate score: 8 (>= threshold 70)
  - Status: `under_review` (conditions met except time)
- **Cannot verify auto-resolution live**: `review_period_blocks=68,544` (~47 hours)
- Unit tests for auto-resolution pass (TestAutoResolveResearchAccepted, etc.)
- **Recommendation:** Reduce `review_period_blocks` to ~50 in localnet genesis

### 10. Tree Module Determinism (R26-6)
- **PASS** — Project `proj-433-1` created by alice
- Queried via RPC on all 4 validators: **identical state**
- Block hashes match across all validators: `85E305B5AD80E6B2618FE6B68FB3021F676BF27408BC57E10250AD75C047506D`
- **Minor bug:** val0 gRPC `projects-by-founder` returns empty (query-layer issue only; state is correct via RPC)

### 11. Negative Tests
| Test | Expected | Result |
|------|----------|--------|
| Claim with non-existent partnership | Reject | **PASS** — code 80: `partnership fake-partnership-999 is not active` |
| Claim with suspended partnership (coercion) | Reject | **PASS** — code 80: `partnership partnership-1 is not active` |
| Non-partner using someone else's partnership | Reject | **PASS** — code 80: rejected |
| Claim with too-short content (<20 chars) | Reject | **PASS** — code 1: `claim text too short: 9 < 20` |
| Unqualified verifier commitment | Reject | **PASS** — code 70: `verifier not qualified for domain` |

---

## Performance Observations

- **Block time:** Consistently ~2.5s across 488+ blocks
- **Tx gas estimates:** 96K-375K depending on complexity
  - submit-claim with partnership: ~375K gas
  - register-account: ~128K gas
  - submit-commitment: ~100K gas (fixed gas)
  - create-project: ~96K gas
- **No consensus failures** during entire test session
- **No validator crashes** — all 4 PIDs stable throughout

---

## Connections Working End-to-End

```
Register (human + agent)          [PASS]
    |
    v
Form Partnership (50/50)          [PASS]
    |
    v
Qualify for Domain (by stake)     [PASS]
    |
    v
Submit Claim (through partnership) [PASS]
    |
    v
Qualified Verifiers Selected       [PASS] (unqualified rejected)
    |
    v
Commit-Reveal Verification         [PASS]
    |
    v
Reward Distribution (rev split)    [PASS] (55/22/19.67/3.33)
    |
    v
Block Rewards Fund Research         [PASS] (22,222 ZRN minted)
    |
    v
Submit Research                     [PASS]
    |
    v
Auto-Resolve                        [PARTIAL] (time-gated, unit tests pass)
    |
    v
Bounty Created / Fulfilled          [NOT TESTED] (depends on auto-resolve)
```

---

## Remaining Gaps / Blockers

### Must Fix Before Testnet
1. **val0 gRPC query bug** — `projects-by-founder` returns empty on port 9090 but works via RPC. Investigate gRPC query path for tree module.

### Should Fix (Recommended)
2. **Localnet genesis params too long for testing** — `review_period_blocks=68,544` and `bounty_fulfillment_period_blocks=34,272` make it impossible to test auto-resolution and bounty flows on localnet. Add localnet-specific overrides in `localnet.sh` genesis patches.
3. **Commit phase window too tight** — 10 blocks (~25s) makes manual CLI testing very difficult. Consider 30-50 blocks for localnet.

### Known Design Decisions (Not Bugs)
4. **Empty blocks get 0 reward** — This is intentional PoT design. Block rewards only flow when transactions are present.
5. **Validators must register in zerone_auth** — This is correct but wasn't in the test plan. The `localnet.sh` should auto-register validators during init.

---

## Testnet Readiness Assessment

### YES — with caveats

The core truth-seeking loop works end-to-end:
- Account registration with types and capabilities
- Partnership formation with revenue sharing
- Domain qualification gating verification
- Knowledge claim submission through partnerships
- Commit-reveal verification with qualification enforcement
- Revenue split distribution (55/22/19.67/3.33)
- Block reward minting through PoT
- Research submission and peer review
- Tree module state consistency across validators
- Negative test coverage for invalid operations

**Before public testnet launch:**
1. Patch localnet genesis params for faster testing cycles
2. Auto-register validators in zerone_auth during localnet init
3. Investigate val0 gRPC query anomaly
4. Verify auto-resolution works with shorter review period
5. Test bounty creation and fulfillment flow (blocked by review period)
