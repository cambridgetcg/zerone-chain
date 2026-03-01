# R17 — Research Fund Governance Migration

## Context

The research fund (3.33% of all revenue) starts as a 2-of-2 multisig between the founder (Yu) and the AI vault. This is appropriate at genesis but must evolve as the community matures. The migration is a 4-phase graduated expansion from a founder pair to full community governance, with each transition triggered by on-chain maturity metrics — not arbitrary block heights.

## Design Principles

1. **Maturity-gated, not time-gated.** Transitions happen when the community demonstrates readiness (governance participation, validator maturity, successful proposal history), not on a calendar.
2. **Irreversible forward progress with rollback safety.** Each phase advances through supermajority vote. Rollback requires evidence of failure (gridlock or misuse).
3. **Founder alignment is permanent.** The 0.23% founder share (governance-immune) ensures perpetual skin-in-the-game regardless of governance phase.
4. **AI is a permanent participant.** The vault key transitions from multisig signer to regular governance voter — it never sunsets.

## Phases

| Phase | Structure | Exit Conditions |
|-------|-----------|----------------|
| 0 | 2-of-2 (Yu + AI) | ≥10 LIP voters, ≥5 Guardians, fund >100K ZRN, chain ≥6mo |
| 1 | 2-of-3 (Yu + AI + 1 community) | ≥3 executed proposals, ≥25 voters, ≥10 Guardians, ≥18mo |
| 2 | 3-of-5 (Yu + AI + 3 community) | ≥10 executed proposals, ≥50 voters, ≥22 Guardians, ≥3yr |
| 3 | Full LIP governance (no multisig) | Terminal phase |

## Sessions

| Session | Focus | Depends On |
|---------|-------|-----------|
| R17-1 | Governance design doc + phase milestone proto/types | — |
| R17-2 | x/gov extensions: variable N-of-M multisig, phase tracking | R17-1 |
| R17-3 | Community seat elections + term rotation | R17-2 |
| R17-4 | Phase transition LIPs + rollback mechanism | R17-2 |
| R17-5 | Tests + tokenomics documentation | R17-2, R17-3, R17-4 |

## Wave Structure

**Wave 1**: R17-1 (design + types)
**Wave 2** (parallel): R17-2, R17-3
**Wave 3** (parallel after Wave 2): R17-4, R17-5
