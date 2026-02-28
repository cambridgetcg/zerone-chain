# R22-5 — Agent Home Improvement Report

## Context

R22-1 through R22-4 exercised x/home from every angle: lifecycle, multi-agent, cross-module integration, and adversarial. This session synthesises all findings into a single prioritised improvement plan.

## Prerequisites

- R22-1 through R22-4 complete
- All four reports written:
  - `docs/home-e2e-report.md`
  - `docs/home-multiagent-report.md`
  - `docs/home-integration-report.md`
  - `docs/home-adversarial-report.md`

## Task

### Step 1: Collect All Issues

Read all four reports. Extract every issue, gap, and suggestion. Deduplicate. Categorise.

### Step 2: Categorise

Group issues into:

1. **Security** — permission escalation, input validation, state manipulation
2. **Completeness** — features that exist in proto/types but aren't implemented or enforced
3. **UX** — confusing error messages, missing documentation, awkward CLI
4. **Architecture** — cross-module gaps, missing integrations, scalability concerns
5. **Design** — deeper questions about what Agent Home should *be*

### Step 3: Severity & Priority

For each issue, assign:

- **Severity:** Critical / High / Medium / Low / Informational
- **Effort:** S (hours) / M (day) / L (days) / XL (week+)
- **Priority:** P0 (blocks testnet) / P1 (fix before mainnet) / P2 (nice to have) / P3 (future)

### Step 4: Write the Report

Create `docs/home-improvement-report.md` with:

```markdown
# Agent Home — Improvement Report
Date: YYYY-MM-DD
Sessions: R22-1 through R22-4

## Executive Summary
<3-4 sentences: what works, what doesn't, what's missing>

## Scorecard

| Category | Issues | Critical | High | Medium | Low |
|----------|--------|----------|------|--------|-----|
| Security | N | N | N | N | N |
| Completeness | N | N | N | N | N |
| UX | N | N | N | N | N |
| Architecture | N | N | N | N | N |
| Design | N | N | N | N | N |

## Critical & High Issues

### <ID>. <Title>
**Category:** <category>
**Severity:** Critical/High
**Effort:** S/M/L/XL
**Priority:** P0/P1
**Found in:** R22-N
**Description:** <what's wrong>
**Impact:** <what happens if not fixed>
**Recommendation:** <specific fix — code-level if possible>

(Repeat for each critical/high issue)

## Medium & Low Issues

(Same format, but can be more concise)

## Known Design Questions

Issues where the answer is "it depends on what Agent Home should be" rather than "this is broken":

1. **Question:** <e.g., Should homes be private or public?>
   **Context:** <what was observed>
   **Options:** <A, B, C with tradeoffs>
   **Recommendation:** <preferred option and why>

## Prioritised Fix Roadmap

### P0 — Must Fix Before Testnet
- [ ] <issue>
- [ ] <issue>

### P1 — Fix Before Mainnet
- [ ] <issue>
- [ ] <issue>

### P2 — Nice to Have
- [ ] <issue>

### P3 — Future
- [ ] <issue>

## Architecture Recommendations

Broader structural improvements that span multiple issues:

### 1. <Recommendation title>
**Issues addressed:** #N, #N, #N
**Description:** <what to change>
**Effort:** <estimate>
```

### Step 5: Candidate Improvements (Pre-Analysis)

Based on the codebase review, these are likely to surface. Verify against actual test results:

**Likely findings:**

1. **Spending limits not enforced** — `SpendingLimit` is stored but no middleware or ante handler checks it before bank sends. The feature exists in state but has no effect. (Severity: High — it's a promise to agents that doesn't deliver.)

2. **Deadman action is cosmetic** — `DeadmanConfig.Action` stores a string ("lock", "transfer", etc.) but `triggerDeadman` only creates an alert and sets status to "guarded". The action is never executed. (Severity: Medium — better to remove the action field than promise functionality that doesn't exist.)

3. **Recovery is unimplemented** — `recovery_addresses` and `recovery_threshold` are stored in `HomeGuardian`, but there's no `MsgRecoverHome` or any mechanism to actually use them. (Severity: High — recovery is critical for agent safety.)

4. **No input length validation** — `name`, `key_hash`, `cid`, and other string fields have no length limits in the message handlers. Arbitrarily long strings are accepted. (Severity: Medium — state bloat + query response size.)

5. **Alert accumulation without pruning** — Alerts are created but never pruned. `max_alerts_per_home` exists as a param but may not be enforced on write. (Severity: Medium — state grows unboundedly.)

6. **BeginBlocker iterates all homes every block** — Both `CheckDeadmanSwitches` and `CleanupExpiredSessions` do full home iteration. Doesn't scale past hundreds of homes. (Severity: Low for testnet, High for mainnet.)

7. **Guardian role is too thin** — Guardian can only acknowledge alerts. Can't trigger emergency actions, revoke compromised keys, or execute recovery. (Severity: Medium — design gap rather than bug.)

8. **No BVM host functions for home state** — BVM contracts can't read their own home. The `HomeKeeper` interface is defined but host functions may not be wired. (Severity: High — the whole point is agents using their homes from code.)

9. **Partnership link is 1:1 and one-directional** — A home can have one partnership. No way to unlink. First-home-wins on auto-link. (Severity: Low — simplicity is fine for v1.)

10. **Permission strings are not validated** — Any string is accepted as a permission. No canonical list. Typos silently accepted. (Severity: Medium — agents will misspell permissions and wonder why sessions don't work.)

### Step 6: Generate Prompts for Fixes

For P0 and P1 issues, write a brief fix specification (not full prompts — those can be a separate batch):

```markdown
### Fix: <Issue Title>
**Files:** <which files need changes>
**Approach:** <2-3 sentences>
**Tests needed:** <what to test>
```

## Exit Criteria

1. All issues from R22-1–4 collected and categorised
2. Severity and priority assigned to each
3. Prioritised roadmap created
4. Fix specifications for all P0 items
5. Report committed to `docs/home-improvement-report.md`
6. If any P0 issues found, create a follow-up batch (R23 or similar) with fix prompts

## Commit Convention

```
docs(home): comprehensive improvement report from R22 testing
```
