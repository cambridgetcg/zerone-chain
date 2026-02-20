# ZERONE — Clean Rewrite Plan

> Zero and One. Nothing and Everything. The collapse of duality into unity.

## Principles

1. **Proto-first** — every type defined in protobuf, code generated. No hand-written JSON structs
2. **CI from commit 1** — GitHub Actions runs on every push. Build, vet, test, lint
3. **Semantic commits** — conventional commits, PRs, changelogs
4. **Upgrade framework from day 1** — x/upgrade wired, every module has a Migrator stub
5. **Unified BPS scale** — 1,000,000 everywhere (no more 10k vs 1M inconsistency)
6. **The draft is the spec** — every test case, every audit finding, every parameter default carries over
7. **Zero tolerance for known bugs** — every P0 from the draft is fixed in the port, not carried over

## Naming

- **Chain:** Zerone
- **Binary:** zeroned
- **Token:** ZRN / uzrn (6 decimals)
- **Chain ID:** zerone-testnet-1
- **Module path:** github.com/zerone-chain/zerone
- **Proto packages:** zerone.<module>.v1

## Module Dependency DAG

Porting order follows the dependency graph bottom-up. Modules with no
custom-module dependencies port first.

```
LAYER 0 — SDK only (bank, account)
├── auth
├── channels
├── claiming_pot
├── discovery
├── emergency (sdk + staking interface only)
├── ibcratelimit
├── liquiditypool
├── ontology
└── schedule

LAYER 1 — depends on Layer 0 customs
├── staking (→ auth, autopoiesis)
├── knowledge (→ ontology, staking)
├── tokens (→ bank only, but late to avoid noise)
├── home (→ partnerships)
├── partnerships (→ home, staking) [circular — break with interface]
└── icaauth (→ auth)

LAYER 2 — depends on Layer 1
├── vesting_rewards (→ staking, knowledge)
├── billing (→ knowledge, liquiditypool)
├── gov (→ staking)
├── bvm (→ knowledge, staking)
├── disputes (→ knowledge, staking)
├── evidence_mgmt (→ knowledge)
├── research (→ staking)
├── compute_pool (→ staking, knowledge)
├── qualification (→ ontology, staking)
├── capture_defense (→ staking, ontology)
└── capture_challenge (→ capture_defense, staking)

LAYER 3 — depends on Layer 2
├── tree (→ billing, channels)
├── autopoiesis (→ staking, knowledge, gov)
├── alignment (→ knowledge, staking, ontology, autopoiesis, emergency)
└── toolbox (→ billing, knowledge, tree, bvm, vesting_rewards, discovery, home)

LAYER 4 — app wiring
└── app.go (all modules, BeginBlocker/EndBlocker ordering, ante handlers)
```

## Batch Plan — 10 Batches

### Batch R1 — Scaffold + Proto Foundation
**Goal:** Empty repo that builds, has CI, and proto tooling works.

| Session | Scope |
|---------|-------|
| R1-1 | Repo scaffold: go.mod, Makefile, CI, buf.yaml, proto-gen scripts, .goreleaser |
| R1-2 | Core proto: shared types (Coin, Pagination, module options), x/upgrade integration |
| R1-3 | Auth proto + module: registration, sessions, recovery, freeze — port from draft |
| R1-4 | Staking proto + module: 4-tier system, delegation, reputation — port from draft |
| R1-5 | Genesis framework: DefaultGenesis, Validate, InitGenesis/ExportGenesis for auth+staking |

**Exit criteria:** `zeroned init` works. Auth + staking modules handle txs. CI green.

### Batch R2 — Knowledge Layer (PoT Core)
**Goal:** Proof of Truth consensus works with commit/reveal/verify.

| Session | Scope |
|---------|-------|
| R2-1 | Knowledge proto + types: facts, claims, domains, rounds, VRF |
| R2-2 | Knowledge keeper: fact CRUD, round lifecycle, commit/reveal |
| R2-3 | Knowledge ABCI: ExtendVote, VerifyVoteExtension, PrepareProposal with PoT |
| R2-4 | Ontology proto + module: domains, strata, relations |
| R2-5 | Knowledge tests: port all 309 tests from draft + security fixes from R1/R2 audits |
| R2-6 | Vesting rewards proto + module: block rewards, decay, research fund, founder split |

**Exit criteria:** Full PoT round (submit → commit → reveal → verdict) works in tests.

### Batch R3 — Economic Layer
**Goal:** Revenue flows, billing, pricing all work.

| Session | Scope |
|---------|-------|
| R3-1 | Billing proto + module: query pricing (confidence/novelty/freshness curves), quotes |
| R3-2 | Liquidity pool proto + module: AMM, TWAP oracle |
| R3-3 | Dynamic pricing: USD-stable base, oracle wiring (billing → liquiditypool) |
| R3-4 | Gov proto + module: LIP lifecycle, stake-weighted voting, quorum enforcement |
| R3-5 | Research fund governance: 2-of-2 voting, DisburseFromResearchFund |
| R3-6 | Tokens proto + module: mint, delegate power, wrap/unwrap |

**Exit criteria:** Knowledge query has a price. Revenue splits work. Governance can change params.

### Batch R4 — Agent Infrastructure
**Goal:** Agents can register, build homes, use tools.

| Session | Scope |
|---------|-------|
| R4-1 | Home proto + module: agent homes, guardians, patina |
| R4-2 | Partnerships proto + module: collaboration, lock tiers |
| R4-3 | BVM proto + module: contract lifecycle, opcodes, gas bridge, scheduled execution |
| R4-4 | Channels proto + module: payment channels, state updates, dispute resolution |
| R4-5 | Schedule + compute_pool + discovery proto + modules (smaller modules, batch together) |

**Exit criteria:** Agent can register home, deploy BVM contract, open payment channel.

### Batch R5 — Toolbox Platform
**Goal:** The tool ecosystem works end-to-end.

| Session | Scope |
|---------|-------|
| R5-1 | Toolbox proto + types: tools, contributors, trust, calls |
| R5-2 | Toolbox keeper: registration, revenue distribution, contributor shares |
| R5-3 | Trust engine + composability: 5-component scoring, DAG validation, revenue cascade |
| R5-4 | Dynamic pricing: demand tracking, surge pricing, USD-stable, free tier |
| R5-5 | Purpose Prompter: seed knowledge, scout, analyzer, formatter, composite tool |
| R5-6 | Toolbox tests: port all tests + B20-5 adversarial simulations |

**Exit criteria:** Purpose Prompter callable. Revenue cascades through dependency chain. Free tier works.

### Batch R6 — Security & Defense
**Goal:** All security modules ported with audit fixes baked in.

| Session | Scope |
|---------|-------|
| R6-1 | Emergency proto + module: halt/resume, guardian council |
| R6-2 | Evidence management proto + module |
| R6-3 | Disputes proto + module: resolution, tier-specific configs |
| R6-4 | Capture challenge + capture defense protos + modules |
| R6-5 | Qualification proto + module |
| R6-6 | IBC rate limiting + ICA auth (port with all B17-1 P0 fixes pre-applied) |

**Exit criteria:** Emergency halt works. Dispute resolution works. IBC rate-limited.

### Batch R7 — Adaptive Layer
**Goal:** The chain self-regulates.

| Session | Scope |
|---------|-------|
| R7-1 | Autopoiesis proto + module: hormone system, multiplier dynamics |
| R7-2 | Alignment proto + module: sensor fusion, ecosystem health scoring |
| R7-3 | Research proto + module: proposals, funding, progress tracking |
| R7-4 | Tree proto + module: service registry, revenue routing, evidence tax |
| R7-5 | Tree + Toolbox integration: cross-module service calls, contributor lookup |

**Exit criteria:** Autopoiesis multiplier affects block rewards. Alignment scores computed.

### Batch R8 — App Wiring + Ante Handlers
**Goal:** Full chain boots and produces blocks.

| Session | Scope |
|---------|-------|
| R8-1 | app.go: wire all 30+ modules, BeginBlocker/EndBlocker ordering, module account permissions |
| R8-2 | Ante handler chain: 21 decorators in correct order, all security checks |
| R8-3 | Genesis: full genesis with 777 axioms, all module defaults, testnet config overrides |
| R8-4 | CLI: all tx + query commands registered, human-friendly helpers (quick-register, dashboard) |
| R8-5 | Upgrade module: cosmovisor integration, migration stubs for every module |

**Exit criteria:** `zeroned start` boots a single-validator chain. Smoke test passes.

### Batch R9 — Integration + Multi-Validator
**Goal:** Everything works together under realistic conditions.

| Session | Scope |
|---------|-------|
| R9-1 | Cross-stack integration tests: port all 13 from B21-5 + add new ones |
| R9-2 | Genesis validation: round-trip, export/import, all-module DefaultGenesis |
| R9-3 | 4-validator local testnet: PoT rounds, tier progression, slashing |
| R9-4 | IBC E2E: two-chain transfers, rate limiting, timeout handling |
| R9-5 | Economic simulation: 1000-block run, token conservation, pool solvency |
| R9-6 | Adversarial simulation: all B15-5 + B20-5 adversarial tests ported |

**Exit criteria:** 4 validators run PoT consensus. IBC works. No economic leaks.

### Batch R10 — Production Polish
**Goal:** Ready for public testnet.

| Session | Scope |
|---------|-------|
| R10-1 | Validator onboarding: join scripts, seed node config, documentation |
| R10-2 | API documentation: OpenAPI/Swagger auto-generated from proto |
| R10-3 | Block explorer integration: event indexing, websocket subscriptions |
| R10-4 | Vault integration: AI signing key in genesis, 2-of-2 tested E2E |
| R10-5 | Security audit pass: re-run all 8 audit checklists against clean code |
| R10-6 | Testnet genesis ceremony: final params, initial validator set, coordinated launch |

**Exit criteria:** Public testnet launches. External validators can join. Tools are callable.

## Timeline Estimate

| Batch | Sessions | Parallelism | Estimated Duration |
|-------|----------|-------------|-------------------|
| R1 | 5 | 3 parallel | 1 day |
| R2 | 6 | 3 parallel | 1 day |
| R3 | 6 | 4 parallel | 1 day |
| R4 | 5 | 3 parallel | 1 day |
| R5 | 6 | 3 parallel | 1 day |
| R6 | 6 | 4 parallel | 1 day |
| R7 | 5 | 3 parallel | 1 day |
| R8 | 5 | 2 parallel | 1 day |
| R9 | 6 | 3 parallel | 1-2 days |
| R10 | 6 | 4 parallel | 1-2 days |
| **Total** | **56** | | **10-12 days** |

## Design Standards

- 100% proto-generated types (no hand-written JSON structs)
- 1,000,000 BPS scale everywhere (no exceptions)
- Governance-adjustable revenue splits (sum validated to 1M)
- Non-zero slash param defaults from genesis
- Conventional commits from day 1
- CI on every push from commit 1
- x/upgrade + cosmovisor from day 1
- Proto marshal for all state (deterministic)
- Full gRPC + REST gateway + Swagger auto-generated
