# R16-6 — Audit Pass: Verify Zero Remaining Old References

## Objective

Mechanical grep-based audit to verify the revenue split refactor is complete. Every hit must be resolved or documented as a false positive.

## Prerequisites

R16-1 through R16-5 complete.

## Audit Checklist

### 1. Burn References (Proto/Go)

```bash
# BurnBps / burn_bps in any Go file (excluding pb.go, which should also be clean)
grep -rn "BurnBps\|burn_bps" --include="*.go" --include="*.proto" | grep -v ".worktrees/"
```

**Expected: 0 hits** (or only in x/tokens for user-initiated ZRN-20 burns)

### 2. BurnCoins in Revenue Paths

```bash
# BurnCoins calls — should ONLY exist in x/tokens (user burns) and SDK wrappers
grep -rn "BurnCoins" --include="*.go" | grep -v pb.go | grep -v _test.go | grep -v ".worktrees/"
```

**Expected:** Only in:
- `x/tokens/keeper/msg_server.go` (user-initiated secondary token burns)
- Any SDK-level wrapper that's not in our module code

**NOT expected in:**
- `x/vesting_rewards/` (was removed in R16-2)
- `x/toolbox/` (was removed in R16-3)
- `x/billing/` through `x/research/` (was removed in R16-3)

### 3. BurnTokens Function

```bash
grep -rn "func.*BurnTokens\|\.BurnTokens(" --include="*.go" | grep -v pb.go | grep -v ".worktrees/"
```

**Expected: 0 hits** (function removed in R16-2, replaced by DisburseFromDevelopmentFund)

### 4. Old Default Values

```bash
# Old research default (130000)
grep -rn "130000\|130_000" --include="*.go" --include="*.json" --include="*.sh" | grep -iv "block\|gas\|byte\|size\|max_facts\|challenge_duration" | grep -v ".worktrees/"
```

**Review each hit:** Is it a research BPS default? Should be 33300 now.

```bash
# Old burn default (100000) — careful, 100000 is used elsewhere (e.g., slash rates)
grep -rn "100000\|100_000" --include="*.go" | grep -i "burn\|revenue\|split" | grep -v ".worktrees/"
```

**Expected: 0 burn-related hits**

### 5. GovernanceActivationHeight in Active Logic

```bash
grep -rn "GovernanceActivationHeight\|governance_activation_height" --include="*.go" | grep -v pb.go | grep -v "DEPRECATED\|deprecated\|removed\|NOTE" | grep -v ".worktrees/"
```

**Expected: 0 active logic hits** (ok in comments marked DEPRECATED)

### 6. Founder Share Governance Protection

```bash
# Every MsgUpdateParams handler should call ValidateFounderShareImmutability
grep -rn "func.*UpdateParams" x/vesting_rewards/keeper/msg_server.go
```

Verify the function body contains `ValidateFounderShareImmutability`.

```bash
# No other module should be able to change founder params either
grep -rn "FounderShareBps\|FounderAddress\|founder_share_bps\|founder_address" --include="*.go" | grep -v pb.go | grep -v _test.go | grep -v ".worktrees/" | grep -v "vesting_rewards"
```

**Expected:** Only query/read paths, never write paths outside vesting_rewards.

### 7. Revenue Split Sum Validation

```bash
# Every validation that sums revenue split fields
grep -rn "ContributorBps.*ProtocolBps.*ResearchBps" --include="*.go" | grep -v pb.go | grep -v ".worktrees/"
```

**Each hit must include `DevelopmentBps` (not `BurnBps`) in the sum.**

### 8. Development Fund Module Account

```bash
# Verify development_fund is registered
grep -rn "development_fund\|DevelopmentFund" --include="*.go" | grep -v pb.go | grep -v _test.go | grep -v ".worktrees/"
```

**Expected:** Registration in app.go + keys.go + keeper usage

### 9. Documentation Consistency

```bash
# No burn references in tokenomics docs
grep -rn "burn\|burned\|burning" docs/tokenomics/ | grep -iv "no burn\|no.*burn\|without burn\|removed\|replaced"
```

**Expected: 0 hits** that imply active burning

```bash
# Revenue split percentages in docs
grep -rn "13%\|130,000\|13 percent" docs/ | grep -i "research"
```

**Expected: 0 hits** (should all be 3.33% / 33,300 now)

### 10. Genesis Config Consistency

```bash
# No burn_bps in any config or script
grep -rn "burn_bps\|burn_bp" scripts/ docs/
```

**Expected: 0 hits**

```bash
# Verify development_bps exists in genesis config
grep -rn "development_bps\|development_bp" scripts/testnet-genesis-config.json
```

**Expected: present in all revenue split sections**

## Worktree Check

The `.worktrees/` directory may contain stale code. Verify:
```bash
# Count stale references in worktrees (informational, not blocking)
grep -rn "BurnBps\|burn_bps" .worktrees/ --include="*.go" | wc -l
```

If worktrees are stale, either rebase them or document that they're pre-R16.

## Report Format

Create `docs/plans/2026-XX-XX-R16-6-audit-report.md`:

```markdown
# R16-6 Audit Report

## Date: YYYY-MM-DD
## Auditor: [session ID]

### Results

| Check | Hits | Status | Notes |
|-------|------|--------|-------|
| 1. burn_bps/BurnBps | 0 | ✅ PASS | |
| 2. BurnCoins in revenue | 0 | ✅ PASS | Only in x/tokens (user burns) |
| ... | | | |

### False Positives Documented
(list any grep hits that are intentional/correct)

### Remaining Issues
(list any unresolved findings)
```

## Commit

```
R16-6: audit pass — verify revenue split refactor completeness

Full grep-based audit across codebase:
- 0 burn_bps/BurnBps references in active code
- 0 protocol-level BurnCoins calls (only user burns in x/tokens)
- All revenue split sums use DevelopmentBps
- Founder share immutability enforced in UpdateParams
- development_fund module account registered
- Documentation consistent with new split
- Audit report at docs/plans/YYYY-MM-DD-R16-6-audit-report.md
```
