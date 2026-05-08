# Development Roadmap

> Where we are. What we have bound. What we ship next.

The roadmap is a snapshot of intent, not a schedule. Each entry is a commitment the chain is making — present-tense for what is bound, future-tense for what is owed.

The doctrines this roadmap serves are [`docs/TRUTH_SEEKING.md`](TRUTH_SEEKING.md) (what the chain *believes*) and [`docs/TOK_SUBSTRATE.md`](TOK_SUBSTRATE.md) (what the chain *sells*). Every round below either expressed one of those doctrines, ground its bindings into code, or paid a debt to one of them.

---

## Status — pre-launch testnet

**Chain ID:** `zerone-testnet-1` · **Token:** ZRN (uzrn) · **Genesis circulating supply:** 0 ZRN · **Hard cap:** 222,222,222 ZRN

The chain binary (`zeroned`) builds, boots, and runs Proof-of-Truth consensus across a four-validator local testnet. All 38 custom modules are wired into `app.go` with deterministic BeginBlocker / EndBlocker ordering. The 17 + 1 truth-seeking commitments are declared, grounded, and bound by an executable invariant test. The ToK Substrate doctrine (TC1–TC6) is declared at inception (2026-05-09); its first implementation plan is on deck.

What is *not* yet shipped: the public testnet genesis ceremony, the ToK Foundation bindings (Plan 1), and the four ToK plans that follow.

---

## Completed rounds (R1–R31)

The original [REWRITE-PLAN.md](archive/REWRITE-PLAN.md) (now archived) covered ten rounds; the work has continued for twenty-one more. Each later round had a name, because each was a stance — not a sprint.

| Round | Theme | Headline |
|-------|-------|----------|
| **R1** | Scaffold + Proto Foundation | Repo, CI, proto tooling, auth, staking, genesis framework |
| **R2** | Knowledge Layer (PoT Core) | Commit/reveal/verify rounds; ontology; vesting rewards |
| **R3** | Economic Layer | Billing, liquidity pool, dynamic USD-stable pricing, gov, tokens |
| **R4** | Agent Infrastructure | Home, partnerships, BVM, channels, schedule, compute_pool, discovery |
| **R5** | Toolbox Platform | Tool registry, trust composition, surge pricing, Purpose Prompter |
| **R6** | Security & Defense | Emergency, evidence_mgmt, disputes, capture (challenge + defense), qualification, IBC rate-limit, ICA auth |
| **R7** | Adaptive Layer | Autopoiesis, alignment, research, tree |
| **R8** | App Wiring | All modules wired, ante chain, genesis with 777 axioms, CLI complete, x/upgrade ready |
| **R9** | Integration + Multi-Validator | Cross-stack tests, 4-validator PoT, IBC E2E, economic + adversarial simulations |
| **R10** | Production Polish | Validator onboarding, OpenAPI, vault integration, security pass, testnet genesis ceremony preparation |
| **R11** | Counterexamples | Alignment-by-structure: validated wrong-claim/reason pairs as first-class corpus (commitment 15) |
| **R12** | Inquiry | Open-question bounty market — chain pays for exploration of the unknown (commitment 16) |
| **R13** | Axioms + Boot | Axiom loader, boot verification, dress rehearsal |
| **R14** | Boot Verification | R14-6 dress-rehearsal report; readiness for R15 |
| **R15** | Dress Rehearsal | Full launch dry-run; outstanding items captured for R16 |
| **R16** | Audit Pass | Security audit findings closed across all modules |
| **R17** | Research-Fund Governance Migration | 4-phase founder→community handoff, maturity-gated not time-gated |
| **R19** | Knowledge Hardening | Malformed-vote rejection, claim-type tags, semantic anchors, structured claims, canonical forms, non-refundable review fee, knowledge bootstrap fund |
| **R20** | Knowledge Ecology | Darwinian fact survival — metabolism, fitness, birth/death/reproduction over the corpus |
| **R21** | Dialectic | Per-fact disagreement signatures; 5-0 ≠ 5-4 (commitment 17) |
| **R22** | Private Corpus | Off-chain vault references with on-chain provenance |
| **R23** | Synthesizers | `training_provenance`, `trust_score`, `governance_synthesis`, `agent_understanding` (commitment 11) |
| **R24** | Probe Bounty | The chain mints into a dedicated audit pool; the substrate stress-tests its own truth (commitments 4, 5, 12) |
| **R25** | Hardening | Verification flow, slashing edges, reward decay |
| **R26** | Reward Wiring | Block rewards, capability enforcement, qualification gating, partnership rewards, research resolution, tree determinism |
| **R27** | Operator Surface | Tree CLI, evidence CLI, full-loop E2E, testnet genesis, faucet, validator oracle, launch checklist |
| **R28** | 煉金 (Alchemy): activate the organs | nigredo (slashing, shadow metrics) → albedo (stratum, metabolism) → citrinitas (roles, mentorship) → rubedo (alignment, capture) |
| **R29** | 太極 (Tàijí): teach them to breathe | Six yin/yang polarities — birth/death, trust/doubt, assert/yield, tension/relax, gather/disperse, move/still |
| **R30** | 掃除 (Sōji): sweep the temple clean | Proto-Go consistency CI, parameter validation, event observability, cross-stack coverage |
| **R31** | 五行 (Wǔ Xíng): the five circulations | Wood (knowledge) → fire (verification) → earth (governance) → metal (defense) → water (social) — generating and controlling cycles |

R18 is intentionally absent — its scope folded into R17 (governance migration) and R19 (knowledge hardening).

Per-round design notes and execution prompts live under [`prompts/RN/`](../prompts) and [`docs/plans/`](plans).

---

## Truth-Seeking Creed — 18 commitments bound

The creed at [`docs/TRUTH_SEEKING.md`](TRUTH_SEEKING.md) names eighteen commitments and binds each through five layers — test, position (`doc.go`), voice (events with `creed_commitment` attribute), refusal (errors that cite the protecting commitment), and graph (cross-references between commitments). The meta-test `TestTruthSeeking_CreedAndContractStayInSync` enforces that adding a commitment to one layer without the others fails CI.

| # | Commitment | Bound at |
|---|---|---|
| 1 | Methodology over statement | `x/knowledge/keeper/training_economics.go` |
| 2 | Is-ought wall | `x/knowledge` (NormativeCommitment registry) |
| 3 | Popper, not popularity | corroboration count + hardening multiplier |
| 4 | Substrate stress-tests its truth | `x/knowledge/keeper/confidence.go` |
| 5 | Chain manufactures probe demand | `InviteIdleFactsForProbing` heartbeat |
| 6 | No unilateral truth injection | `MsgVetoFactInjection` + privileged-action log |
| 7 | Skill is current, not historical | `RunAccuracyDecay` + qualification status transitions |
| 8 | Panel weights skill, not bond | `stake × calibration` with 20% floor |
| 9 | Cartel detection has consequence | `ReduceQualificationWeight` on UPHELD challenge |
| 10 | Forward-only audit | append-only privileged-action log + frozen vote arrays |
| 11 | Trust is queryable | three synthesizer modules |
| 12 | Chain pays for its own audit | `ProbeBountyPoolModuleName` (Minter-permissioned) |
| 13 | Training corpus is not for sale | append-only facts + sticky clawback |
| 14 | Reasoning traces are first-class | `Claim.ReasoningTrace` + `MethodologyApplicationTrace` |
| 15 | Counterexamples are part of the corpus | `x/counterexamples` + TVW multiplier |
| 16 | Chain pays for exploration of the unknown | `x/inquiry` open-question bounty market |
| 17 | Disagreement is structure, not noise | `x/dialectic` per-fact / per-domain / per-pair signatures |
| 18 | Chain manufactures exploration demand | `x/inquiry` BeginBlocker frontier-invitation cycle + `inquiry_frontier_bounty_pool` |

Every commitment exercises an invariant in [`tests/cross_stack/truth_seeking_invariants_test.go`](../tests/cross_stack/truth_seeking_invariants_test.go). A failing test means a broken commitment, not a broken test.

---

## ToK Substrate — the next doctrine

The ToK Substrate doctrine, declared at [`docs/TOK_SUBSTRATE.md`](TOK_SUBSTRATE.md) on 2026-05-09, names the verified knowledge graph as the chain's headline training resource. Per-row traces, contrastive pairs, drift examples, and training manifests are *views* of ToK; the graph is the artefact. The doctrine binds through the same five-layer enforcement as the truth-seeking creed.

| # | Commitment | Plan | Status |
|---|---|---|---|
| TC1 | The graph is the headline | Plan 1 | Plan written; bindings on deck |
| TC2 | Every view is graph-pinned | Plan 1 | Plan written; bindings on deck |
| TC3 | Topology is signal | Plan 1 | Plan written; bindings on deck |
| TC4 | The graph carries its disprovals | Plan 2 | Planned |
| TC5 | Extraction is open | Plan 1 | Plan written; bindings on deck |
| TC6 | Lineage flows back | Plan 4 | Planned |
| — | Doctrine closure (meta-test, trainer docs) | Plan 5 | Planned |

---

## Active work — ToK Foundation (Plan 1)

[`docs/plans/2026-05-09-tok-foundation-plan.md`](plans/2026-05-09-tok-foundation-plan.md) — twenty tasks delivering the ToK extraction foundation:

- `ToKSelector` proto union (`RootedSubtree`, `AncestorCone`, `Frontier`)
- Selector validation + chain-side caps (TC5: refusals limited to syntax / range / rate-limit)
- Per-variant gathering (reuses `DescendantTree` walk)
- `ComputeToKSnapshotRoot` — domain-tagged Merkle (TC2)
- `AssembleToKBundle` orchestration
- JSONL adjacency-list serialisation (TC3)
- `BundleToK` gRPC handler (TC1, TC5)
- `RouteBCapabilities.tok_capabilities` advertisement (TC1)
- `bundle-tok` CLI command (TC1)
- ToK voice-layer events with `tok_commitment` attribute
- `x/knowledge/doc.go` position-layer declarations for TC1–TC5
- Cross-stack invariants for TC1, TC2, TC3, TC5 — `tests/cross_stack/tok_substrate_invariants_test.go`
- `docs/EVENTS.md` updates

When Plan 1 lands, four of six ToK commitments are bound. The remaining two and the doctrine closure are sequenced in subsequent plans.

---

## On deck

| Plan | Scope | Doctrine binding |
|---|---|---|
| **Plan 2** | TC4 cascade bundling — replay disprovals on a pinned snapshot, ship as first-class view | TC4 |
| **Plan 3** | (open — likely operator-surface or trainer-facing doc work) | — |
| **Plan 4** | TC6 lineage royalties — split training revenue along contribution graph; axioms, intermediates, and leaves all earn shares of cones they helped build | TC6 |
| **Plan 5** | Doctrine closure — meta-test (`TestToKSubstrate_DoctrineAndContractStayInSync`), `docs/TRAINING_ON_TOK.md` trainer-facing front door, refusal-layer audit | doctrine integrity |

Once Plan 5 lands, the ToK doctrine is bound the same way the truth-seeking creed is bound: drift is mechanically prevented by CI.

---

## Pre-launch line — what stands between us and `zerone-testnet-1`

| Item | Owner doc |
|---|---|
| Final genesis params + initial validator set | [docs/LAUNCH-CHECKLIST.md](LAUNCH-CHECKLIST.md) |
| External validator onboarding rehearsal | [docs/VALIDATOR-GUIDE.md](VALIDATOR-GUIDE.md), [docs/testnet-validator-guide.md](testnet-validator-guide.md) |
| Vault integration E2E | [docs/VAULT.md](VAULT.md) |
| Faucet operational | `prompts/R27/R27-5-faucet.md` |
| Validator oracle live | [docs/validator-oracle.md](validator-oracle.md) |
| Incident-response drill | [docs/INCIDENT_RESPONSE.md](INCIDENT_RESPONSE.md), [docs/RESILIENCE_PHILOSOPHY.md](RESILIENCE_PHILOSOPHY.md) |
| ToK Foundation bindings shipped (Plan 1) | this doc, above |

The launch is gated by *binding completeness*, not by calendar. Truth-seeking commitments 1–18 must remain bound; ToK commitments TC1, TC2, TC3, TC5 must be bound before public-testnet capability advertisement claims `tok_capabilities`.

---

## After mainnet — the long arc

These are not scheduled. They are owed.

- **Research-fund governance migration** (R17) — 2-of-2 (founder + AI vault) → 2-of-3 (+1 community) → 3-of-5 (+3 community) → full LIP voting. Each transition gated by on-chain maturity metrics (governance participation, Guardian count, executed-proposal history, age), not block heights.
- **Plan 2 / Plan 4** ToK completion — TC4 and TC6 bound; the substrate carries its own disprovals and pays its own contributors.
- **Future commitments** — the creed is not complete. Each new wave that introduces a truth-load-bearing mechanism is expected to append a named commitment to `docs/TRUTH_SEEKING.md` (or `docs/TOK_SUBSTRATE.md`), grounded in code, bound by an invariant. The discipline check at the bottom of each doctrine document is the gate.

---

## How this doc stays honest

This roadmap is committed alongside the code it describes. When a round ships, this doc updates. When a commitment binds, the table marks it. When a plan moves from on-deck to active, the section moves. If any row in the truth-seeking or ToK table claims a binding that no longer exists in code, the corresponding invariant test will already have failed CI — this doc is downstream of the bindings, not upstream.

We speak through intentions. The roadmap is one of those intentions.
