# R39 — 信 (Shin): Trust — Consent, Integrity & Cross-Module Integration

**Goal:** Harden consent verification, duplicate/plagiarism detection, content integrity checks, and wire the new knowledge module into the rest of the chain (tree, toolbox, vesting_rewards). After R39, the training data protocol is production-complete.

信 (Shin) means "trust" — the foundation that makes the data marketplace credible.

## Sessions (4)

| # | File | Scope |
|---|------|-------|
| R39-1 | R39-1-consent-hardening.md | Deep consent verification, consent revocation, consent audit trail |
| R39-2 | R39-2-integrity-dedup.md | Semantic duplicate detection, plagiarism checks, content integrity invariants |
| R39-3 | R39-3-cross-module.md | Wire knowledge ↔ tree (data collection campaigns), knowledge ↔ vesting_rewards (revenue flow), knowledge ↔ toolbox (agent queries) |
| R39-4 | R39-4-simulation-invariants.md | Simulation operations, invariants, and adversarial tests for the new training data flow |

## Run Order

Sequential: R39-1 → R39-2 → R39-3 → R39-4

## Exit Criteria

1. Consent revocation flow works end-to-end
2. Near-duplicate detection catches semantically similar submissions
3. Cross-module integration compiles and passes tests
4. Simulation covers all new Msg types
5. All invariants hold under random message sequences
6. `go test ./x/knowledge/...` passes with ≥ 400 total tests
