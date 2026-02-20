# Batch R2 — Knowledge Layer (Proof of Truth Core)

## Goal

Port the Proof of Truth consensus mechanism. After this batch, claims can
be submitted, verified through commit/reveal rounds, and verdicts reached.
Verifiers earn rewards. Incorrect votes get slashed.

## Context

The knowledge module is the heart of Zerone — consensus through truth
verification rather than wasted computation. This is the largest single
module (~309 tests in the draft, 21 handlers, 12 query RPCs).

**Draft reference:** `/Users/yuai/Desktop/legible_money/x/knowledge/`

The draft knowledge module has:
- Fact lifecycle: submit → commit/reveal verification → verdict
- VRF-based verifier selection
- Confidence scoring (fundamentality, citations, cross-references)
- Domain management
- Extended params (72+ governance-adjustable parameters from R3-4)
- ABCI integration (ExtendVote, VerifyVoteExtension, PrepareProposal)

## Sessions (6)

| ID | Focus | Dependencies |
|----|-------|-------------|
| R2-1 | Knowledge proto + types: facts, claims, domains, rounds, VRF | None |
| R2-2 | Knowledge keeper: fact CRUD, round lifecycle, commit/reveal | R2-1 |
| R2-3 | Knowledge ABCI: ExtendVote, VerifyVoteExtension, PrepareProposal | R2-2 |
| R2-4 | Ontology proto + full module: domains, strata, relations | None |
| R2-5 | Knowledge tests: port all 309 tests + security fixes | R2-2, R2-3 |
| R2-6 | Vesting rewards proto + module: block rewards, decay, research fund, founder split | None |

## Run Order

- **Wave 1 (parallel):** R2-1, R2-4, R2-6 (independent)
- **Wave 2:** R2-2 (depends on R2-1)
- **Wave 3:** R2-3 (depends on R2-2)
- **Wave 4:** R2-5 (depends on R2-2, R2-3)

## Exit Criteria

Full PoT round works in tests: submit claim → VRF select verifiers →
commit votes → reveal votes → verdict (accept/reject) → reward/slash.
