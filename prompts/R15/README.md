# R15 — Testnet Readiness: Auth Bridge, Test Parity, E2E Hardening

**Goal:** Zerone is testnet-ready. Auth/DID integration into BVM complete, test coverage at parity with prototype (~3,000+), multi-validator PoT rounds work, and a full dress rehearsal passes end-to-end.

## The Problem

R14 delivered the binary and closed the biggest test gaps. But:

1. **Auth/DID → BVM bridge missing** — BVM contracts can't resolve caller identity or enforce session capabilities. The prototype has `CallerDID`, `SessionCapabilities`, capability inheritance for scheduled execution. Zerone's BVM has none of this. Without it, BVM is identity-blind.

2. **~264 test gap across 11 modules** — partnerships (33), research (37), claiming_pot (28), gov (26), emergency (23), channels (20), staking (18), disputes (17), discovery (17), icaauth (15), ibcratelimit (15). Plus missing app-level tests (vote_extensions, research_fund_restriction).

3. **No multi-validator PoT verification** — localnet scripts exist but haven't been run with the new binary. PoT rounds completing across 4 validators is unverified.

4. **No dress rehearsal** — genesis ceremony → axiom injection → configure → start → governance tx hasn't been run as a single pipeline.

## Sessions (6)

| # | File | Scope | Parallelism |
|---|------|-------|-------------|
| R15-1 | R15-1-auth-bvm-bridge.md | Wire authKeeper into BVM, CallerDID resolution, SessionCapabilities in execution context | Wave 1 |
| R15-2 | R15-2-auth-bvm-tests.md | Port ~22 auth-dependent BVM tests skipped in R14 + schedule capability inheritance | Wave 2 (after R15-1) |
| R15-3 | R15-3-module-tests-batch1.md | Port partnerships + research + claiming_pot + emergency tests (~121 gap) | Wave 1 |
| R15-4 | R15-4-module-tests-batch2.md | Port gov + staking + channels + disputes + discovery + IBC + app-level tests (~143 gap) | Wave 1 |
| R15-5 | R15-5-multivalidator-pot.md | 4-validator localnet with PoT rounds, tier progression, slashing | Wave 2 (after R15-1) |
| R15-6 | R15-6-dress-rehearsal.md | Full testnet dress rehearsal: ceremony → axioms → configure → start → 100 blocks → governance | Wave 3 (after all) |

## Run Order

- **Wave 1 (parallel):** R15-1, R15-3, R15-4
- **Wave 2 (after R15-1):** R15-2, R15-5
- **Wave 3 (after all):** R15-6

## Exit Criteria

1. BVM `authKeeper` wired — CallerDID resolves, SessionCapabilities enforced
2. Auth-dependent BVM tests pass (all 22 ported)
3. Test count ≥ 3,000
4. All 11 module gaps closed to ≤ 5 tests of prototype
5. 4-validator localnet produces PoT rounds (commit/reveal/verdict cycle completes)
6. Dress rehearsal passes: genesis ceremony → 100 blocks → governance param change verified
7. `make pr-check` clean
8. `go test ./...` — zero failures

## After R15

You're launching. The remaining work is operational:
- Public testnet announcement
- External validator onboarding
- Vault integration E2E (AI signing key in genesis)
- Documentation polish
