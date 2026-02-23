# R16 — Revenue Split Refactor & Founder Immutability Audit

## Context

Three design decisions made 2026-02-23 require a thorough codebase sweep:

1. **Burn → Development Fund**: `burn_bps` renamed to `development_bps`. Burns replaced with deposits to a new `development_fund` module account. No tokens are ever destroyed.
2. **Research 13% → 3.33%**: Research fund share reduced; freed allocation absorbed by development fund.
3. **Founder Share Governance-Immune**: `FounderShareBps` and `FounderAddress` cannot be modified via `MsgUpdateParams` once set. `GovernanceActivationHeight` sunset removed.

New revenue split (must sum to 1,000,000 BPS):
- Contributors: 550,000 (55%)
- Protocol: 220,000 (22%)
- Development: 196,700 (19.67%)
- Research: 33,300 (3.33%)
- Burn: 0 (removed)

## Sessions

| Session | Focus | Depends On |
|---------|-------|-----------|
| R16-1 | Proto + generated code + common types | — |
| R16-2 | vesting_rewards module (core revenue engine) | R16-1 |
| R16-3 | Module-level revenue splits (billing, toolbox, tree, etc.) | R16-1 |
| R16-4 | Tests — unit + integration + simulation | R16-2, R16-3 |
| R16-5 | Genesis configs, parameter docs, CLI | R16-2 |
| R16-6 | Audit pass — grep-verify zero remaining old references | R16-1–5 |

## Wave Structure

**Wave 1** (parallel): R16-1
**Wave 2** (parallel after R16-1): R16-2, R16-3
**Wave 3** (parallel after Wave 2): R16-4, R16-5
**Wave 4** (sequential, final): R16-6

## Gate: R16-6 Must Pass

R16-6 is a mechanical audit. It greps the entire codebase for:
- `burn_bps` / `BurnBps` / `BurnAmount` / `BurnCoins` / `BurnTokens` in revenue contexts
- Old default values (130000 for research, 100000 for burn)
- `GovernanceActivationHeight` in founder share logic
- Any `MsgUpdateParams` handler that doesn't call `ValidateFounderShareImmutability`

ALL hits must be resolved or explicitly documented as false positives.
