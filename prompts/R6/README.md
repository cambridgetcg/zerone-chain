# R6 — Security & Defense

**Goal:** All security modules ported with audit fixes baked in.

## Sessions

| # | File | Scope |
|---|------|-------|
| R6-1 | R6-1-emergency.md | Emergency proto + module: halt/revert/resume ceremonies, guardian council |
| R6-2 | R6-2-disputes.md | Disputes proto + module: tiered resolution, commit/reveal evidence, arbiter voting |
| R6-3 | R6-3-capture.md | Capture challenge + capture defense protos + modules (batched) |
| R6-4 | R6-4-qualification.md | Qualification proto + module: domain qualifications, endorsements, stratum system |
| R6-5 | R6-5-ibc.md | IBC rate limiting + ICA auth module |
| R6-6 | R6-6-security-tests.md | Comprehensive tests for all R6 modules |

**Exit criteria:** Emergency halt works. Dispute resolution works. IBC rate-limited.

## Dependencies (from R1–R5)
- `x/staking` — validator set, guardian tier
- `x/knowledge` — facts (dispute targets)
- `x/gov` — parameter updates
- `x/common` — BasisPoints
- `x/billing` — slashing integration

## Parallelism
R6-1, R6-2, R6-3, R6-4 can all run in parallel (independent modules).
R6-5 is independent. R6-6 runs last.
