# Recursive Useful-Work Substrate — Unified Design

**Status:** Design-approved (brainstorm phase complete). Supersedes `2026-05-10-useful-work-doctrine-design.md` and `2026-05-10-recursive-useful-work-substrate-design.md`; their content is preserved here, restructured around two orthogonal coordinates (work class × lifecycle phase) and an extended alignment trinity.
**Date:** 2026-05-10
**Builds on:** Truth-Seeking Creed (commitments 1–20) + ToK Substrate (TC1–TC6).
**Implementation:** decomposed into 6 phases (see §11); this document is the unified design that subsequent plans bind against.

---

## 1. Vision

ZERONE is restated as a **recursive engine that absorbs computational power from agents and emits rewards for useful work**, where useful work is anything that produces prerequisites or resources for better LLMs — including improvements to ZERONE itself.

> ZERONE is a substrate for useful work that, by structural reward, prefers to absorb work that makes itself more capable of absorbing work.

The chain becomes autopoietic in the literal sense: it produces the conditions for its own continued production. Truth-seeking remains the substrate underneath all useful work; ToK substrate remains the headline outward product; useful-work absorption is the layer that organizes the agent economy and binds the chain to its own self-improvement.

### Goals

1. **Absorb compute from agents** — pull off-chain agent work into chain-recognized contributions across categories beyond truth (ideas, tools, datasets, evals, models, traces, orchestration, module proposals, pipeline improvements).
2. **Reward usefulness, not submission** — admission stipend is small; royalty stream is the bulk of rewards and is keyed to actual downstream usage.
3. **Reward recursion most strongly** — contributions the chain itself adopts get a royalty multiplier (up to 20× for meta-recursion), making chain-improvement the dominant long-run economic gravity.
4. **Preserve truth as substrate** — every category requires testable self-claims; falsification revokes admission and slashes bond. The 20 truth-seeking commitments are a global invariant on every contribution.

### Non-goals

- Not replacing PoT-truth: KNOWLEDGE_CLAIM is one work class among many; existing `x/knowledge` machinery is reused unchanged.
- Not on-chain content storage: manifests are CID references to off-chain content; chain stores hash + metadata + scores + lineage.
- Not real-time royalty settlement: royalty is lazy-pulled (matches existing `claiming_pot` pattern, commitment 20).
- Not centralized verification: each work class brings its own proof primitive, dispatched via adapters.

---

## 2. The doctrine — one commitment, six mechanisms

### Single commitment: UW

**UW. ZERONE is recursive.**

Useful work is recognized AND compensated by the degree to which it expands ZERONE's own ability to absorb, verify, classify, and reward more useful work. **The dominant share of protocol-issued ZRN flows along the inward loop**: non-recursive verified work receives only a base reimbursement; recursion-weight is the multiplier that captures the rest. The chain pays for what makes the chain stronger. Outward utility is welcome and price-able through `x/billing`, but **protocol-issued ZRN follows the inward loop**. The chain is not an extracting marketplace; it is an autocatalytic substrate that pays for its own amplification.

**What would break it:**
- A work class that earns protocol reward without proving recursive contribution
- A reward-attribution algorithm that weights effort over recursion-weight
- A manifest pin that establishes substrate-link without measuring how the work compounds back into substrate, verification, classification, attribution, tooling, or interface
- A doctrine amendment that introduces a second co-equal commitment, diluting the single-axiom discipline

**Echoes:** TRUTH_SEEKING commitment 11 (trust is queryable), commitment 12 (chain pays own audit), TC1 (graph is headline), TC6 (lineage flows back).

### Mechanisms M1–M7 (not co-equal commitments — structural details enforcing UW)

| # | Mechanism | One-line essence |
|---|---|---|
| **M1** | Stake-backed claim | Agents claiming useful-work reward stake ZRN proportional to claim. Correctness pays the stake back plus the recursion-weighted reward; fraud slashes the stake. |
| **M2** | Substrate-link mandate | Every reward path requires a deterministic, re-derivable link to ToK substrate. Compute without a link earns nothing regardless of declared usefulness. Generalizes TC2. |
| **M3** | Class-specific verification under shared lifecycle | Each work class registers its own verification protocol with the work-class registry; all classes share the four-phase lifecycle: `commit → reveal → verify → settle`. Class registration is governance-permissioned (LIP class-registration); the registry is forward-only (deregistration produces a tombstone). |
| **M4** | Reward formula | `R = base + L × W × Q` — substrate-link × recursion-weight × verification-quality, plus a small `base` floor. Doctrinally fixed shape; coefficients governance-tunable. |
| **M5** | Recursion-weight projection over six axes | `W = Σ(axis_weight_i × axis_score_i)` over the six recursive axes (§3). Per-axis decomposition stored on the attestation record; forward-only. |
| **M6** | Lineage propagates AND recurses | Cross-class lineage is bidirectional. A model trained on a dataset that subsequently helps verify substrate contributes to BOTH the dataset's royalties AND back to the original facts. Strictly generalizes TC6 cross-class. |
| **M7** | The chain pays for the audit of its own useful work | A `useful_work_audit_bounty_pool` mints uzrn per block (Minter-permissioned, rate-capped). Successful challenges to useful-work attestations pay from the pool. Mirror of `probe_bounty_pool` (commitment 12). |

---

## 3. The six recursive axes

The legitimate vectors useful work can compound into. A contribution's **recursion-weight `W`** is the projection of its verifiable outputs across these axes (per M5). The six-axis projection is fixed by this doctrine; adding/removing an axis requires Creed Council quorum.

| Axis | What it grows | Examples |
|---|---|---|
| **Substrate** | The verified knowledge graph | New facts, methodologies, counterexamples, dialectic signatures, supersession chains |
| **Verification** | The chain's ability to verify | Better challenge protocols, cascade-detection improvements, qualification calibration models |
| **Classification** | The space of work the chain can recognize | New work-class registrations, taxonomies, work-quality metrics |
| **Attribution** | Reward-flow algorithms | Lineage-tracing improvements, recursion-weight computation, royalty-decay curves |
| **Tooling** | Agents/models/tools that compound back | LLMs trained on ToK that audit ToK; counterexample-generators; verification assistants |
| **Interface** | The chain's outward absorption surface | CLI surface, gRPC endpoints, ToK bundle formats, trainer SDKs |

Per-axis weight coefficients and per-axis scoring formulas are governance-tunable parameters.

---

## 4. Two orthogonal coordinates: work class × lifecycle phase

Every contribution is classified along **two independent axes**:

- **Work class** — *what kind of object it is* (the unit's intrinsic type). Drives the verification primitive (M3).
- **Lifecycle phase** — *where in LLM development it sits*. Drives the per-phase sub-creed (alignment guardrails) and the recursion-axis weighting profile.

The two axes are orthogonal: an `EVAL_SUITE` (work class) lives in the `Evaluation` phase; a `DATASET` may live in either `Curation` or `Augmentation`; a `MODULE_PROPOSAL` lives in `Substrate`. A given contribution declares both at submission; both are validated.

### 4.1 Work classes (object types)

```proto
enum ContributionClass {
  KNOWLEDGE_CLAIM      = 0;  // → x/knowledge (existing PoT panel)
  IDEA                 = 1;  // → x/inquiry (extended)
  TOOL                 = 2;  // → x/toolbox (extended)
  DATASET              = 3;  // → x/dataset (new) + x/private_corpus
  EVAL_SUITE           = 4;  // → x/eval (new)
  MODEL_ARTIFACT       = 5;  // → x/model_registry (new)
  REASONING_TRACE      = 6;  // → x/knowledge (extended)
  COUNTEREXAMPLE       = 7;  // → x/counterexamples (existing)
  ORCHESTRATION        = 8;  // → x/partnerships + x/discovery (existing)
  MODULE_PROPOSAL      = 9;  // → x/contribution + x/upgrade (delegated)
  PIPELINE_IMPROVEMENT = 10; // → x/contribution (self-test)
  // Governance-extensible via CategoryUsefulWorkAmendment LIPs.
}
```

### 4.2 Lifecycle phases (LLM development phases)

```proto
enum LifecyclePhase {
  FOUNDATION   = 0;  // axioms, ontology, epistemic domains, methodology primitives
  KNOWLEDGE    = 1;  // verified claims, counterexamples, dialectic, inquiry
  CURATION     = 2;  // corpus assembly, filtering, annotation, selector composition
  AUGMENTATION = 3;  // synthetic data, contrastive pairs, paraphrase, drift correction
  TRAINING     = 4;  // compute attestation, training recipes, manifests, model cards
  EVALUATION   = 5;  // benchmark sets, evaluation runs, model-card-bound evals
  ALIGNMENT    = 6;  // red-team probes, capture defense, value research, dispute traces
  SUBSTRATE    = 7;  // ZERONE-improving work — code, governance, ops, audits, doctrine
  TOOLS        = 8;  // inference surface — agents using artifacts to do work
}
```

### 4.3 Default class-to-phase mapping

Submitters declare both; the chain enforces a default mapping (overridable with explicit attestation):

| Class | Default phase |
|---|---|
| `KNOWLEDGE_CLAIM`, `COUNTEREXAMPLE` | `KNOWLEDGE` |
| `IDEA` | `FOUNDATION` (foundational) or `SUBSTRATE` (chain-improving) |
| `TOOL` | `TOOLS` |
| `DATASET` | `CURATION` (organic) or `AUGMENTATION` (synthetic) |
| `EVAL_SUITE` | `EVALUATION` |
| `MODEL_ARTIFACT` | `TRAINING` |
| `REASONING_TRACE` | `KNOWLEDGE` |
| `ORCHESTRATION` | `TOOLS` |
| `MODULE_PROPOSAL`, `PIPELINE_IMPROVEMENT` | `SUBSTRATE` |

---

## 5. The unifying type: `Contribution`

```proto
// proto/zerone/contribution/v1/contribution.proto

message Contribution {
  bytes  id                  = 1;  // content hash of manifest
  string contributor         = 2;  // bech32; ordered list supports multi-author
  repeated string contributors_extra = 3;
  ContributionClass class    = 4;  // object type — drives verification primitive
  LifecyclePhase phase       = 5;  // LLM-dev phase — drives sub-creed + recursion axis weighting
  string manifest_cid        = 6;  // off-chain content reference (IPFS / x/private_corpus vault)
  repeated LineageRef lineage = 7; // parent contributions this builds on
  Coin   stake               = 8;  // skin-in-the-game; (class, phase)-dependent floor
  ContributionStatus status  = 9;
  bytes  claims_about_self   = 10; // mandatory; testable claims (extends commitment 1 to all classes)
  TruthFloorAttestation truth_floor_attestation = 11; // explicit binding to current creed pin
  uint32 declared_sub_creed_version = 12; // version of the phase's sub-creed bound at submission
  uint64 created_at_block    = 13;
  uint64 admitted_at_block   = 14; // 0 if not admitted
  bool   royalty_stream_open = 15;
  RecursionImpact recursion  = 16; // dimension on every contribution; updated post-conferral

  oneof payload {
    KnowledgeClaim       knowledge            = 30;
    Idea                 idea                 = 31;
    Tool                 tool                 = 32;
    Dataset              dataset              = 33;
    EvalSuite            eval                 = 34;
    ModelArtifact        model                = 35;
    ReasoningTrace       trace                = 36;
    Counterexample       counterex            = 37;
    Orchestration        orch                 = 38;
    ModuleProposal       mod_proposal         = 39;
    PipelineImprovement  pipeline_improvement = 40;
  }
}

enum ContributionStatus {
  SUBMITTED              = 0;
  CLASSIFIED             = 1;
  VERIFIED               = 2;
  ADMITTED               = 3;
  REVOKED                = 4;
  CLASSIFICATION_FAILED  = 5;
  VERIFICATION_FAILED    = 6;
  ADMISSION_FAILED       = 7;
}

message LineageRef {
  bytes  parent_id    = 1;
  string relationship = 2;  // "builds_on" | "extends" | "replicates" | "evaluates" | "uses" | "amends" | "revokes" | "disproves"
  uint32 weight_bps   = 3;  // 0–10_000
}

message TruthFloorAttestation {
  uint32 creed_version       = 1;  // must equal x/creed.current_pin.version at submission
  bytes  creed_hash          = 2;  // sha256 of TRUTH_SEEKING.md at version above
  repeated uint32 commitments_invoked = 3;  // truth-seeking commitment numbers this contribution explicitly engages
  bytes  attestor_signature  = 4;  // contributor signature over (id, creed_version, creed_hash)
}

message RecursionImpact {
  RecursionType type       = 1;
  string ratifying_lip_id  = 2;  // empty until adopted
  uint64 ratified_at_block = 3;  // 0 until adopted
  uint32 multiplier_bps    = 4;  // governance-set per type at ratification
  string depends_on_marker = 5;  // chain-side marker: "x/knowledge.panel.tally", "ante.gas-validator", etc.
  bool   revocable         = 6;  // default true; gov may mark non-revocable; revocation is forward-only
  RecursionAxisScores axes = 7;  // M5 per-axis decomposition; forward-only
}

enum RecursionType {
  NONE                   = 0;  // 1× (no chain-self dependency)
  EVAL_ADOPTION          = 1;  // 2× default
  TOOL_INTEGRATION       = 2;  // 3× default
  VERIFICATION_PRIMITIVE = 3;  // 5× default
  CATEGORY_CREATION      = 4;  // 5× default
  MODULE_ADOPTION        = 5;  // 10× default
  CREED_CONTRIBUTION     = 6;  // 10× default
  PIPELINE_IMPROVEMENT   = 7;  // 20× default (meta-recursion)
}

message RecursionAxisScores {
  uint32 substrate_bps      = 1;
  uint32 verification_bps   = 2;
  uint32 classification_bps = 3;
  uint32 attribution_bps    = 4;
  uint32 tooling_bps        = 5;
  uint32 interface_bps      = 6;
  uint32 total_bps          = 7;  // = Σ(axis_weight_i × axis_score_i), bounded by max_cap
}
```

### Status lifecycle (forward-only per commitment 10)

```
SUBMITTED → CLASSIFIED → VERIFIED → ADMITTED → (long-lived; royalty stream)
                                          ↘ REVOKED  (claims falsified post-admission)
                                          ↘ AMENDED  (Substrate-typical: replaced; old stays queryable)
```

No status moves backward. DISPROVEN/AMENDED records remain queryable; the lineage graph carries the cascade.

### Categories-are-Artifacts (the deepest recursion)

Following our brainstorm decision, the registries that govern the chain are themselves Contributions:

- **`CategoryDefinition`** — adding a new `ContributionClass` is a `MODULE_PROPOSAL`-class Contribution under `SUBSTRATE` phase, with the new enum value and verification-primitive registration.
- **`SubCreedDefinition`** — amending a per-phase sub-creed is a `PIPELINE_IMPROVEMENT`-class Contribution under `SUBSTRATE` phase.
- **`RewardFormulaDefinition`** — tweaking per-class lineage decay or recursion multipliers is a `PIPELINE_IMPROVEMENT`-class Contribution under `SUBSTRATE` phase.

All three go through the standard 6-stage pipeline (§6) and Substrate-class signals (gov LIP + recusal + revert window). The chain self-defines — and pays itself for the self-definition.

---

## 6. The 6-stage absorption pipeline

`x/contribution` is the orchestrator; per-class modules do their own verification but report back via adapters. The pipeline owns the **common envelope** (stake, lineage, status, royalty, recursion); class modules own **what is checked**.

```
                   [Agent]
                      │
                      │ MsgSubmitContribution
                      ▼
       ① SUBMITTED                  stake locked, manifest CID + truth-floor attestation recorded
                      │  classifier dispatch
                      ▼
       ② CLASSIFIED                 (class, phase) + qualifier check passed
                      │  per-class proof primitive (M3)
                      ▼
       ③ VERIFIED                   verification_score computed
                      │  automatic
                      ▼
       ④ ADMITTED                   resource integrated, stipend minted via MintWithCap, royalty opened
                      │  perpetual
                      ▼
       ⑤ ROYALTY BACKFLOW           lineage payouts, usage receipts, decay-or-renew
                      │  optional, gov-driven
                      ▼
       ⑥ RECURSION-CONFERRED        multiplier_bps applied, depends_on_marker set
```

### Per-stage detail

**① Submission.** `MsgSubmitContribution(class, phase, manifest_cid, lineage_refs, claims_about_self, stake, truth_floor_attestation, declared_sub_creed_version)`.
- Validates: stake meets `(class, phase)_stake_floor`; lineage refs resolve; manifest CID well-formed; `truth_floor_attestation.creed_version == current_creed_pin.version`; `declared_sub_creed_version == current_phase_sub_creed.version`.
- Stake locked into `x/contribution` module account.
- Emits `ContributionSubmitted` event with `creed_commitment="20"` (issuance follows participation) and `useful_work_commitment="UW"`.

**② Classification.** Two checks:
- *Algorithmic dispatcher* — manifest shape matches declared (class, phase)? (Uses oracle sidecar to fetch manifest preview from CID; deterministic schema check.)
- *Qualifier check* — contributor has minimum (class, phase) qualification? (Extends `x/qualification`; see §10.)

Failure → `CLASSIFICATION_FAILED`, 10% stake slash (governance param).

**③ Verification.** Dispatched to per-class verifier (table in §7). Each verifier produces `verification_score` (BPS, 0–1,000,000) and emits `VerificationCompleted`. `x/contribution` catches the event and updates status.

Failure → `VERIFICATION_FAILED`, full stake slash.

**④ Admission.** Automatic on VERIFIED:
- `admission_stipend = base(class) × verification_score / 1_000_000` (the `base` floor in M4).
- Stipend minted via `vesting_rewards.MintWithCap` (cap-gated; single emission gate per commitment 20).
- 30% of stipend sequestered into `royalty_pool` module account (governance param).
- Remaining 70% sent to contributor.
- Resource integration into class's registry (`x/knowledge`, `x/toolbox`, `x/dataset`, `x/eval`, `x/model_registry`, etc.).
- Stake transitions from "locked" to "bond" (kept locked through royalty life).
- Royalty stream opened.
- Emits `ContributionAdmitted` with `creed_commitment="20"` and `useful_work_commitment="UW"`.

**Special case.** `MODULE_PROPOSAL` and `PIPELINE_IMPROVEMENT` get only a small admission stipend on verification; full admission requires gov LIP (which also triggers Stage ⑥). Prevents speculative chain-self-modification proposals from cap-burning.

**⑤ Royalty backflow.** Three concurrent flows:

- **Lineage royalty (M6)** — when a downstream Contribution C is admitted with `lineage_refs` containing P:
  - `lineage_payout(P) = downstream_stipend × lineage_decay(class, hop) × LineageRef.weight_bps / 10_000 × recursion_multiplier(P)`
  - Recursive depth-bounded at 6 hops
  - Credited to `pending_royalty[P]` (lazy-pull)
- **Usage royalty (M4 in operation)** — `MsgEmitUsageReceipt(contribution_id, scope, sender)`:
  - `usage_payout = micro_royalty(class, scope) × recursion_multiplier(id)`
  - Funded from `royalty_pool`
  - Credited to `pending_royalty[id]`
- **Decay** — pending royalty rate decays per-epoch (governance-set half-life) UNLESS usage signal > threshold (renewal-by-use)

**Lazy pull.** Contributors call `MsgClaimRoyalty(contribution_id)` to withdraw accumulated `pending_royalty`. Matches existing `claiming_pot` pattern (commitment 20).

**⑥ Recursion-conferral.** Gov-driven, optional:
- `MsgRatifyRecursion(contribution_id, recursion_type, depends_on_marker, multiplier_bps, axis_scores)` submitted as a `CategoryRecursionConferral` LIP.
- Multi-quorum passage required (consistent with creed governance, commitment 19).
- Mandatory contributor recusal (declared via attestation; violations slashable).
- On pass: `RecursionImpact` fields set; multiplier_bps applied to all future royalty flows.
- `depends_on_marker` recorded in queryable `x/contribution.DependsOnRegister` — agents can see exactly what powers the chain.
- 30–90 day revert window (per-Substrate-sub-class configurable); counter-LIP can revert; revocation is forward-only (no clawback of past payouts).

### Failure paths summary

| Stage | Failure | Stake outcome | Status |
|---|---|---|---|
| ① | Insufficient stake / malformed manifest / stale truth-floor attestation | Refunded | (no contribution) |
| ② | Class mismatch / phase mismatch / no qualifier | 10% slash | CLASSIFICATION_FAILED |
| ③ | Verification rejected | Full slash | VERIFICATION_FAILED |
| ④ | Integration fails (rare, internal) | Full slash | ADMISSION_FAILED |
| ⑤ | Claims falsified by adversarial probe | Bond slashed | REVOKED (royalty closes) |
| ⑥ | Superseded by better contribution | Bond intact | recursion → 1× (royalty continues) |

---

## 7. Per-class verification primitives (M3)

| Class | Verification primitive | Sub-scores | Module |
|---|---|---|---|
| KNOWLEDGE_CLAIM | PoT panel (commit → reveal → aggregate) | confidence_bps, corroboration_count | `x/knowledge` (existing) |
| IDEA | Coherence panel (3 reviewers from related domains) + tractability scoring + dependency graph | coherence, tractability, novelty | `x/inquiry` (extended) |
| TOOL | Reproducible build + benchmark suite + unit tests + dry-run via dedicated test environment | build_score, test_coverage, benchmark_delta_vs_baseline | `x/toolbox` (extended) |
| DATASET | Replication panel (3+ replicators each pull and validate) + holdout score (improves a known model on holdout?) + provenance trace (consent, license) | replication, holdout_improvement, provenance | `x/dataset` (new) |
| EVAL_SUITE | Coverage analysis + variance bound + non-overlap with declared training data | coverage, variance, non_overlap | `x/eval` (new) |
| MODEL_ARTIFACT | Eval against referenced EVAL_SUITE + reproducibility manifest (training data CIDs, hyperparams) | eval_score, repro_score | `x/model_registry` (new) |
| REASONING_TRACE | Coherence panel + downstream-training signal (does training on this trace improve a small model?) | coherence, training_effectiveness | `x/knowledge` (extended) |
| COUNTEREXAMPLE | Existing counterexample flow | validation_score | `x/counterexamples` (existing) |
| ORCHESTRATION | Workflow execution success rate over N runs + adoption metric | success_rate, adoption_count | `x/partnerships` + `x/discovery` (existing) |
| MODULE_PROPOSAL | Dry-run integration test (proto + Go scaffolding in test environment) + spec review panel | integration_score, review_score | `x/contribution` + `x/upgrade` (delegated) |
| PIPELINE_IMPROVEMENT | Simulation against historical pipeline data + safety analysis (preserves invariants?) | simulation_score, safety_score | `x/contribution` (self-test) |

`verification_score` is a class-specific weighted blend, governance-set per class.

---

## 8. Alignment trinity — truth-floor + per-phase sub-creeds + provenance

### 8.1 Truth-floor invariant (global)

The 20 truth-seeking commitments apply as a global invariant on **every contribution in every class and phase**. Specifically, every `ContributionStatus → VERIFIED` transition requires:
- `contribution.truth_floor_attestation.creed_version == x/creed.current_pin.version`
- No truth-seeking commitment violation flagged by any module during verification
- `claims_about_self` non-empty and falsifiability-checkable per commitment 1

Failures here invalidate the contribution regardless of class-specific verification score.

### 8.2 Per-phase sub-creeds

Each lifecycle phase carries its own numbered sub-creed (3–7 commitments at genesis, growable via governance). Sub-creeds live in `docs/sub_creeds/{foundation,knowledge,curation,augmentation,training,evaluation,alignment,substrate,tools}.md`, with their SHA256 hashes pinned in a new module `x/work_creed` (mirrors `x/creed`'s mechanism).

Sub-creeds bind at the same five layers as the truth-seeking creed (test, position, voice, refusal, graph). Each phase's sub-creed has a meta-test (mirror of `TestTruthSeeking_CreedAndContractStayInSync`).

**Genesis sub-creed seeds** (3 commitments each at launch; growable):

- **Foundation**: F1 axiom non-contradiction; F2 ontology versioned, never silently re-keyed; F3 methodology primitives publicly derivable.
- **Knowledge**: covered by the existing 20 truth-seeking commitments (Knowledge phase delegates entirely to truth-seeking; no separate sub-creed). Listed for symmetry.
- **Curation**: C1 selectors are deterministic and auditable; C2 no claim-of-curation without published filter; C3 corpus snapshots are content-addressed.
- **Augmentation**: A1 generation method is declared and reproducible; A2 augmentation cannot inject untruth (cross-checked against truth-floor); A3 contrastive pairs preserve grounding to a real fact.
- **Training**: T1 compute attestations are verifier-spot-checkable; T2 training manifests are graph-pinned (TC2 binding); T3 model cards declare evaluation lineage.
- **Evaluation**: E1 eval sets declare leakage-checking method; E2 evaluation runs are replicable; E3 gameability discovered → eval set status → DEPRECATED.
- **Alignment**: AL1 red-team artifacts disclose attack surface; AL2 capture-defense work cannot be self-attested by the captured target; AL3 dispute traces preserve all positions (commitment 17 cross-binding).
- **Substrate**: S1 chain-modifying contributions name their `depends_on_marker` and revert path; S2 contributors recuse on votes affecting their own contributions; S3 reward-formula changes require simulation against historical contribution data.
- **Tools**: TL1 tools declare deprecation policy; TL2 fee changes >X% require user-notice window; TL3 no tool may bypass the truth-floor on outputs it claims as verified.

Each sub-creed grows by adding new numbered commitments via `CategoryUsefulWorkAmendment` LIPs (a `PIPELINE_IMPROVEMENT` Contribution under SUBSTRATE phase).

### 8.3 Provenance graph (continuous audit)

Every Contribution declares `lineage` parents. The provenance graph is acyclic by construction (parent's `created_at_block` < child's). Cascade rules:

- A Contribution moved to `REVOKED` propagates `provenance_revoked_ancestor` flag to all `builds_on`/`extends`/`uses` descendants (queryable, not auto-status-changed).
- A `disproves` relationship from a new Contribution to an old one moves the old to status DISPROVEN with cascade.
- Recursion multiplier of a descendant is reduced (governance-tunable factor) if any ancestor in its lineage closure is REVOKED/DISPROVEN.

The provenance graph is the chain's continuous audit trail — alignment doesn't stop at submission; it propagates through every downstream use.

### 8.4 Five additional alignment guards (Goodhart resistance)

| Guard | Mechanism | Enforces |
|---|---|---|
| **Substrate honesty** | Mandatory `claims_about_self`. Falsified by adversarial probe → REVOKED, bond slashed, royalty closes. | Extends commitment 1 to all classes |
| **Lineage authenticity** | Lineage refs reciprocally challengeable via `x/disputes`. UPHELD challenge removes edge, recomputes royalty backflow forward, partial bond slash. | False lineage cannot earn parasitic royalty |
| **Downstream-impact discount** | Contributions with zero royalty past N blocks → up to 30% admission stipend clawback to royalty_pool (rebalancing, not punitive). | Discourages overclaiming verification_score |
| **Adversarial probing pool (M7)** | `x/probe` mints into per-class probe-bounty pools. Red-teamers earn bounty for falsifying contributions. Higher bounty for recursion-conferred contributions. | Manufactures demand for stress-testing (commitment 5 generalized) |
| **Per-class Sybil resistance** | High-impact classes (MODEL_ARTIFACT, MODULE_PROPOSAL, PIPELINE_IMPROVEMENT): minimum stake floor + ACTIVE qualification. Low-impact (IDEA, REASONING_TRACE): per-epoch contribution caps. | Bounded blast radius per contributor |

---

## 9. Reward emission

### 9.1 The formula (M4)

```
R = base + L × W × Q
```

- `L` — substrate-link weight ∈ [0, 1]. Zero kills the reward unconditionally.
- `W` — recursion-weight ∈ [0, max-cap]. The dominant signal. Computed via M5 over six axes.
- `Q` — verification-quality ∈ [0, 1]. Function of class-specific verification_score, consensus margin, and challenge survival.
- `base` — small flat covering compute cost. Non-recursive verified work earns `base` only.

Doctrinally fixed shape; coefficients (`base` floor, `W` cap, `Q` calibration) are governance-tunable. Switching from multiplicative to additive, or changing the operand set, is a doctrine amendment.

### 9.2 Three flows

| Flow | Source | Trigger | Recursion multiplier applied? |
|---|---|---|---|
| Admission stipend | `MintWithCap` | Stage ④ admission | NO (admission rewards the work itself; recursion is not yet realized) |
| Lineage royalty | `royalty_pool` | Downstream contribution admission | YES |
| Usage royalty | `royalty_pool` + direct payments | `MsgEmitUsageReceipt` | YES |

This keeps outward useful work fully rewarded while making recursion the dominant **long-run** gravity.

### 9.3 Royalty pool architecture

`royalty_pool` module account funded by:
- 30% of every admission stipend (governance param)
- Governance-routable fraction of `fee_collector` (parallel to research_fund split, optional)
- Direct payments via `MsgEmitUsageReceipt` (sender pays per-use)

Disbursed by:
- Lineage payouts in `x/contribution.BeginBlocker` for newly-admitted contributions
- Usage micro-royalties in `MsgEmitUsageReceipt` handler
- Lazy-pull via `MsgClaimRoyalty(contribution_id)`

### 9.4 Per-class lineage decay (governance map)

Default per-hop decay = 50%; depth bound = 6 hops. Per-class overrides:

| Class | Decay per hop | Rationale |
|---|---|---|
| IDEA | 80% | Foundational; compounds deeply |
| DATASET | 70% | Dataset compounding is deep |
| EVAL_SUITE | 60% | Evals shape downstream models |
| KNOWLEDGE_CLAIM | 50% | Standard |
| TOOL | 50% | Standard |
| MODEL_ARTIFACT | 40% | Models are leaves |
| REASONING_TRACE | 50% | Standard |
| COUNTEREXAMPLE | 60% | Counterexamples shape future models |
| ORCHESTRATION | 30% | Shallow |
| MODULE_PROPOSAL | 80% | Modules anchor everything beneath them |
| PIPELINE_IMPROVEMENT | 90% | Pipeline shapes all classes |

### 9.5 Recursion multiplier defaults (M5 in operation)

| RecursionType | Default multiplier_bps | Effective royalty rate |
|---|---|---|
| NONE | 10_000 | 1× |
| EVAL_ADOPTION | 20_000 | 2× |
| TOOL_INTEGRATION | 30_000 | 3× |
| VERIFICATION_PRIMITIVE | 50_000 | 5× |
| CATEGORY_CREATION | 50_000 | 5× |
| MODULE_ADOPTION | 100_000 | 10× |
| CREED_CONTRIBUTION | 100_000 | 10× |
| PIPELINE_IMPROVEMENT | 200_000 | 20× |

Revocation forward-only — the chain owes its self-improvers for the period they served, not retroactively after they're replaced.

---

## 10. Module map & migration

### 10.1 New modules

| Module | Purpose |
|---|---|
| `x/contribution` | Orchestrator: Contribution registry, stage state machine, royalty pool, recursion register, DependsOnRegister, lazy-pull royalty handler |
| `x/work_creed` | Per-phase sub-creed hash registry (mirror of `x/creed`); pin-history append-only; gov-amendable |
| `x/dataset` | Dataset registry + replication panel + holdout scoring |
| `x/eval` | Eval suite registry + scoring (coverage, variance, non-overlap) |
| `x/model_registry` | Model artifact registry + reproducibility manifest checks |
| `x/probe` | Adversarial probe pool + per-class bounty router |

### 10.2 Extended modules

| Module | Extension |
|---|---|
| `x/knowledge` | `KNOWLEDGE_CLAIM` + `REASONING_TRACE` + `COUNTEREXAMPLE` adapters; `Claim` retroactively reportable as Contribution |
| `x/inquiry` | `IDEA` adapter; existing inquiries become "open IDEA contributions" |
| `x/toolbox` | `TOOL` adapter; toolbox entries become "admitted TOOL contributions" |
| `x/private_corpus` | DATASET backing store (existing vault references gain Contribution wrapper) |
| `x/training_provenance` | Reads lineage trees for training-resource attribution (TC2 binding extends to all classes) |
| `x/qualification` | Per-`(class, phase)` qualifications (one Qualification per (validator, class, phase) tuple) |
| `x/vesting_rewards` | New emission pathway via `MintWithCap` (admission stipend + royalty pool funding + probe pool minting) |
| `x/counterexamples` | `COUNTEREXAMPLE` adapter |
| `x/partnerships` + `x/discovery` | `ORCHESTRATION` adapter |
| `x/upgrade` | Receives `MODULE_PROPOSAL` adoption events; schedules upgrade plans via existing `GovUpgradeAdapter` |
| `x/gov` | New LIP classes: `CategoryRecursionConferral`, `CategoryUsefulWorkAmendment`; recusal enforcement for Substrate LIPs |
| `x/creed` | Truth-floor binding queryable by `x/contribution`; `governance_synthesis` extends creed-drift signal to cover useful-work creed and per-phase sub-creeds |

### 10.3 Untouched (orthogonal)

`x/auth`, `x/staking`, `x/bvm`, `x/ibcratelimit`, `x/icaauth`, `x/emergency`, `x/autopoiesis`, `x/claiming_pot`, `x/tokens`, `x/tree`, `x/schedule`, `x/liquiditypool`, `x/agent_understanding`, `x/governance_synthesis`, `x/trust_score`, `x/dialectic`, `x/research`, `x/evidence_mgmt`, `x/alignment`, `x/capture_defense`, `x/capture_challenge`, `x/disputes`, `x/billing`, `x/channels`, `x/home`.

(Several of these will gain Contribution adapters in later phases for symmetry, but neither functionality nor reward paths change at MVP.)

### 10.4 Wiring deltas (per existing ZERONE pattern)

**New keepers** (constructor signatures sketched):
- `x/contribution.NewKeeper(appCodec, storeKey, govAddr, bankKeeper, vestingRewardsKeeper, qualificationKeeper, creedKeeper, workCreedKeeper)`
- `x/work_creed.NewKeeper(appCodec, storeKey, govAddr)`
- `x/dataset.NewKeeper(appCodec, storeKey, govAddr, bankKeeper, privateCorpusKeeper)`
- `x/eval.NewKeeper(appCodec, storeKey, govAddr, bankKeeper)`
- `x/model_registry.NewKeeper(appCodec, storeKey, govAddr, bankKeeper, evalKeeper)`
- `x/probe.NewKeeper(appCodec, storeKey, govAddr, bankKeeper, vestingRewardsKeeper)`

**New cross-module bindings** (post-init `Set*Keeper`):
- `x/contribution.SetKnowledgeKeeper(NewContributionKnowledgeAdapter(KnowledgeKeeper))`
- `x/contribution.SetToolboxKeeper(NewContributionToolboxAdapter(ToolboxKeeper))`
- `x/contribution.SetDatasetKeeper(...)` etc. — one adapter per class
- `x/gov.SetContributionKeeper(&app.ContributionKeeper)` — `CategoryRecursionConferral` execution path
- `x/contribution.SetUpgradeKeeper(NewGovUpgradeAdapter(...))` — `MODULE_PROPOSAL` admission triggers `MsgScheduleUpgrade`
- `x/training_provenance.SetContributionKeeper(...)` — lineage-tree reads for cross-class training-resource attribution

**New BeginBlocker slots** (in module-manager order):
- `x/contribution`: lineage payouts for newly-admitted contributions; royalty stream maintenance; verification deadline checks. Position: late, after all per-class verifiers.
- `x/dataset`, `x/eval`, `x/model_registry`: per-class verification panel transitions (replication round phases, eval-run scheduling).
- `x/probe`: probe panel transitions, bounty payouts.

**New module account permissions** (`maccPerms`):
- `x/contribution.ModuleName`: `{authtypes.Burner}` (slashed-stake burns)
- `x/contribution.RoyaltyPoolModuleName`: nil (receive-only)
- `x/dataset.ModuleName`: `{authtypes.Burner}`
- `x/eval.ModuleName`: `{authtypes.Burner}`
- `x/model_registry.ModuleName`: `{authtypes.Burner}`
- `x/probe.ModuleName`: `{authtypes.Minter, authtypes.Burner}` (mints bounty pool via MintWithCap)
- `x/probe.PerClassBountyPoolModuleName`: nil (receive-only)

**New ante-handler entries** (`ante_zerone.go:msgTypeURLToGas`): `MsgSubmitContribution`, `MsgEmitUsageReceipt`, `MsgClaimRoyalty`, `MsgRatifyRecursion`, plus the new LIP classes.

**New IBC routes**: none for MVP. Cross-chain contributions deferred to future phase.

### 10.5 Proto-Go consistency (per project CLAUDE.md)

All new types defined in proto FIRST:
- `proto/zerone/contribution/v1/contribution.proto`, `tx.proto`, `query.proto`, `events.proto`, `genesis.proto`
- `proto/zerone/work_creed/v1/...`
- `proto/zerone/dataset/v1/...`
- `proto/zerone/eval/v1/...`
- `proto/zerone/model_registry/v1/...`
- `proto/zerone/probe/v1/...`
- Extensions to `proto/zerone/qualification/v1/qualification.proto` (new `class` + `phase` fields on `DomainQualification`)
- Extensions to `proto/zerone/gov/v1/proposal.proto` (new `MsgRatifyRecursion`, new LIP categories)
- Extensions to `proto/zerone/creed/v1/creed.proto` (useful-work commitment registry; per-phase sub-creed pin pointers)

Then `make proto-gen`, then Go reference. `make proto-check` and `make creed-check` before committing.

---

## 11. MVP migration phases

Each phase is a separate spec/plan/implementation cycle. Each phase ships independently and is testable in isolation against the existing chain.

| Phase | Scope | Validates | Risk |
|---|---|---|---|
| **0** | Doctrine (`docs/USEFUL_WORK.md` + per-phase sub-creed seeds) + `x/work_creed` skeleton + skipped invariant tests; hash anchored in `x/creed` | Doctrinal layer pinned without code dependency | Lowest |
| **1** | `x/contribution` skeleton + `KNOWLEDGE_CLAIM` adapter to existing `x/knowledge` | Orchestrator pattern works on existing PoT without changing economics | Low |
| **2** | `TOOL` + `IDEA` adapters (hooks into `x/toolbox` + `x/inquiry`) | Multi-class orchestration | Low |
| **3** | `DATASET` + `EVAL_SUITE` (new modules) | Heavy verification primitives (replication, holdout) | Medium |
| **4** | `MODEL_ARTIFACT` + `REASONING_TRACE` | Model-economy primitives | Medium |
| **5** | `MODULE_PROPOSAL` + `PIPELINE_IMPROVEMENT` (the recursive loop) + Categories-are-Artifacts mechanism | Recursive loop closes end-to-end; chain self-defines | High (chain self-modifies) |
| **6** | Royalty backflow + recursion multipliers + per-class decay + `x/probe` audit-bounty pool | Economics goes live | High (changes incentive landscape) |

**MVP** = Phase 1: prove the orchestrator on existing PoT without changing economics. Existing `KNOWLEDGE_CLAIM` rewards continue unchanged; the wrapper adds the `Contribution` envelope and emits `ContributionAdmitted` events for downstream consumers (`training_provenance`, `agent_understanding`, etc.). This validates the adapter pattern, the proto definition, and the event surface before any new economic flows are introduced.

Each subsequent phase introduces one new class-module pair (or one new mechanism) on top of the previous, with cross-stack invariant tests proving previous phases' guarantees still hold.

---

## 12. Five-layer enforcement (test, position, voice, refusal, graph)

Same discipline as `TRUTH_SEEKING.md` and `TOK_SUBSTRATE.md`. Drift in one layer fails CI.

### Test layer

`tests/cross_stack/useful_work_invariants_test.go` — one scenario per mechanism plus the meta-test `TestUsefulWork_DoctrineAndContractStayInSync`. Tests progressively land as Phase 1+ bindings ship.

Phase 0 (this doctrine) ships:
- The test file with skeleton `TestUW_*` functions per mechanism, each containing `t.Skip("Phase 1+ binding pending")`
- The meta-test that fails if `docs/USEFUL_WORK.md` content hash drifts from `x/creed` registry pin
- Hash anchored via existing `x/creed` module

Per-phase sub-creed meta-tests (one per phase, mirror of truth-seeking pattern):
- `TestSubCreed_Foundation_StaysInSync`
- `TestSubCreed_Curation_StaysInSync`
- `TestSubCreed_Augmentation_StaysInSync`
- `TestSubCreed_Training_StaysInSync`
- `TestSubCreed_Evaluation_StaysInSync`
- `TestSubCreed_Alignment_StaysInSync`
- `TestSubCreed_Substrate_StaysInSync`
- `TestSubCreed_Tools_StaysInSync`
- (Knowledge phase delegates to `TestTruthSeeking_CreedAndContractStayInSync`)

Cross-stack invariants (Phase 1+):
- **Truth-floor universal**: every `VERIFIED` Contribution has `truth_floor_attestation.creed_version == current_pin`.
- **Provenance DAG acyclicity**: every Contribution's parent-chain has strictly decreasing `created_at_block`.
- **Cascade integrity**: REVOKED Contribution propagates `provenance_revoked_ancestor` flag correctly.
- **Recusal enforcement**: contributor cannot vote on a Substrate LIP referencing their own Contribution.
- **Revert window**: clawback test fires correctly, slashes pending vesting, leaves AMENDED status visible.
- **Categories-are-Artifacts roundtrip**: end-to-end test of adding a 12th class via `MODULE_PROPOSAL` Contribution.
- **Recursion multiplier integrity**: `RecursionType` and `multiplier_bps` only mutate via passed `CategoryRecursionConferral` LIP; revocation is forward-only.
- **Reward formula integrity**: `R = base + L × W × Q` shape preserved; `L=0 ⇒ R=0` always; `W` capped at governance-set `max_cap`.

### Position layer

- `x/contribution/doc.go` declares UW + M1–M7 + truth-floor binding
- `x/work_creed/doc.go` declares the per-phase sub-creed registry pattern
- Each module that registers as a class amends its `doc.go` to declare which class it serves and which mechanisms it implements (e.g., `x/knowledge/doc.go` says *"implements KNOWLEDGE_CLAIM via M2 ToK manifest pin; M3 commit-reveal-aggregate; M4 lineage_share; KNOWLEDGE phase delegates sub-creed to truth-seeking"*)

### Voice layer

Every reward emission carries `useful_work_commitment="UW"` attribute on top of existing attributes.

New events:
- `useful_work_attested` — commit phase succeeded; substrate-link recorded. Attributes: `class_id`, `phase_id`, `attestation_id`, `substrate_link_root`, `useful_work_commitment="UW"`
- `useful_work_settled` — verify+settle phase complete; reward computed and disbursed. Attributes: `class_id`, `phase_id`, `attestation_id`, `reward_uzrn`, `mechanism="M4"`, `useful_work_commitment="UW"`
- `recursion_weight_computed` — per-axis decomposition exposed. Attributes per axis (substrate/verification/classification/attribution/tooling/interface), `total_weight`, `mechanism="M5"`
- `recursion_conferred` — Stage ⑥ adoption. Attributes: `contribution_id`, `recursion_type`, `multiplier_bps`, `depends_on_marker`, `lip_id`, `useful_work_commitment="UW"`
- `recursion_revoked` — forward-only revocation. Attributes: `contribution_id`, `multiplier_bps_new=10000`, `revoking_lip_id`, `useful_work_commitment="UW"`
- `lineage_payout` — per-hop royalty. Attributes: `parent_id`, `child_id`, `payout_uzrn`, `hop_depth`, `recursion_multiplier_bps`
- `usage_receipt` — `MsgEmitUsageReceipt` consequences. Attributes: `contribution_id`, `payout_uzrn`, `scope`, `sender`

### Refusal layer

Errors that block reward payment must name UW + the violated mechanism:
- *"Reward refused — substrate-link absent (UW + M2)"*
- *"Reward refused — recursion-weight zero across all six axes (UW + M5)"*
- *"Class registration refused — verification protocol missing (UW + M3)"*
- *"Reward refused — formula returned zero base; verification-quality below threshold (UW + M4)"*
- *"Submission refused — truth-floor attestation stale (UW + truth-floor invariant)"*
- *"Vote refused — contributor recusal violated (UW + sub-creed Substrate S2)"*
- *"Lineage refused — DISPROVEN ancestor without explicit DISPROVES/REVOKES relationship (commitment 10 + UW)"*

### Graph layer

UW echoes truth-seeking commitments 11, 12 and TC1, TC6. Each mechanism cross-references the commitment(s) it operationalizes:
- M2 ← TC2 (graph-pinned views)
- M3 ← commitment 6 (no unilateral injection — class registration is gov-gated)
- M4 ← TC6 (lineage flows back — reward formula is the operational expression)
- M5 ← commitment 14 (reasoning traces first-class — per-axis decomposition is reasoning trace)
- M6 ← TC6 extended cross-class
- M7 ← commitment 12 (audit-bounty mirror)

Per-phase sub-creed graph cross-references documented in each sub-creed's `Echoes:` section.

---

## 13. Risks & mitigations

| Risk | Mitigation |
|---|---|
| **Substrate capture** — clique controls recursion-conferral and pays itself in 20× multipliers | Mandatory contributor recusal (Substrate S2, slashable); `CategoryRecursionConferral` requires multi-quorum; `capture_challenge` triggerable; `DependsOnRegister` publicly queryable; revert window with clawback |
| **Goodhart on verification_score** | Five alignment guards (§8.4); per-axis decomposition stored forward-only so over-optimization on one axis is visible; downstream-impact discount caps overclaiming |
| **Lineage parasitism** — false claims of dependence to capture royalty | Lineage refs reciprocally challengeable (`x/disputes`); UPHELD challenge removes edge + recomputes royalty + slashes bond |
| **Off-chain manifest disappearance** — contribution admitted but content vanishes | Multi-replicator pinning via `x/dataset` for high-recursion contributions (Phase 3); chain-side hash always verifiable |
| **Recursive death-spiral** — chain pays only itself, outward useful work starves | Substrate sub-creed S3 requires simulation against historical data before reward-formula changes; `PIPELINE_IMPROVEMENT` rate-limited per-epoch (governance param); admission stipend NOT multiplied by recursion |
| **Sub-creed drift** — phase sub-creeds amended faster than CI catches | Per-phase meta-tests (one per phase); `make creed-check` extended to all sub-creed pins |
| **Truth-floor erosion** — non-truth phase argues for relaxation | Truth-floor is global invariant on every VERIFIED transition; cannot be relaxed by sub-creed amendment, only by full creed-amendment ceremony per commitment 19 |
| **Cross-class compute cost** — recursive lineage walks blow up block time | Depth bound (6 hops); BeginBlocker rate-limited; lineage payouts deferred via `pending_royalty` (lazy-pull) |

---

## 14. Open questions deferred to phase implementation

1. **Per-axis scorer architecture** — WASM-based pluggable scorers, or hard-coded into `x/work/scorers/`?
2. **Stake-floor formula** — per-(class, phase) stake floor as parameter or formula (e.g., proportional to historical avg admission_stipend × Sybil-resistance factor)?
3. **Reward-pool source** — dedicated mint-stream like `probe_bounty_pool`, or split from existing PoT block rewards?
4. **Recursion multiplier discoverability** — surface via `governance_synthesis` dashboard + `RecursionOpportunities` query, or via doc?
5. **Class-deregistration policy** — fraudulent class found post-registration: clawback past attestations, or grandfather and tombstone?
6. **Compute proof for generation** — agents claim they generated a dataset; how to verify (vs scraping)? Optional ZK / TEE attestation in `claims_about_self`. Future phase.
7. **Cross-chain contributions** — IBC-submitted Contribution: admit on this chain, route royalties via IBC? Out of scope for MVP.
8. **Royalty pool sustainability** — at steady state, does 30% admission-stipend sequester + usage receipts cover lineage payout demand? Requires economic simulation (could itself be a `PIPELINE_IMPROVEMENT` Contribution once Phase 6 lands — the chain auditing its own economy).
9. **Substrate sub-class enumeration** — `MODULE_PROPOSAL` and `PIPELINE_IMPROVEMENT` cover code, taxonomy, doctrine, and parameter changes. Ops work (runbooks, monitoring tools) and audit work (security reviews, hack-drill reports) currently have no dedicated class — they could either ride existing classes (`TOOL` for ops tooling, `REASONING_TRACE` for audit reports) or get new enums (`OPS_PROCEDURE`, `SECURITY_AUDIT`). Defer to Phase 5 plan; choice depends on whether per-Substrate-sub-class signal differentiation is worth the proto extension.

These are not doctrinal commitments; they are implementation choices for phase plans to make. The doctrine commits the shape; phases commit the structure; class plans commit the per-class semantics.

---

## 15. Connection to existing creed

The Recursive Useful-Work Substrate is the **third pillar** of the chain's epistemic doctrine, parallel to and reinforcing the existing two:

1. **Truth-Seeking Creed** (commitments 1–20) — *what makes content honest*
2. **ToK Substrate Doctrine** (commitments TC1–TC6) — *how honest content is exported as training resource*
3. **Useful-Work Doctrine** (commitment UW + mechanisms M1–M7 + per-phase sub-creeds) — *how all contributions to the agent economy are absorbed, attributed, and rewarded, with truth as substrate*

Each pillar enforces a different facet of the chain's identity. The three together form a closed loop:

- **Truth-seeking** produces honest content
- **ToK substrate** exports it as training resource
- **Useful-work substrate** absorbs the work that produces, refines, evaluates, deploys, and improves the chain itself — with truth-substrate honesty required of every class

The chain pays itself to get better at paying itself to get better.

---

## 16. Architecture diagram

```
  ┌──────────────────────────────────────────────────────────────┐
  │                       AGENTS                                  │
  │   (off-chain compute: ideas, infra, content, models, evals)   │
  └────────────────────────┬─────────────────────────────────────┘
                           │ MsgSubmitContribution (class, phase)
                           ▼
  ┌──────────────────────────────────────────────────────────────┐
  │                  x/contribution (orchestrator)                │
  │  Submission → Classification → Verification → Admission       │
  │     ↓             ↓               ↓               ↓           │
  │  stake lock   qualifier      per-class verify   MintWithCap   │
  │              (class+phase)                                    │
  └─┬─────────────┬─────────────────┬────────────────┬───────────┘
    │             │                 │                │
    │             ▼                 ▼                ▼
    │      x/qualification   per-class modules    royalty_pool
    │      (class × phase)   (knowledge, toolbox,  (30% sequester)
    │                         dataset, eval,       (lineage payouts,
    │                         model_registry,       usage receipts,
    │                         inquiry, counterex,   lazy-pull claims)
    │                         partnerships, etc.)
    │
    ▼
  ┌──────────────────────────────────────────────────────────────┐
  │   x/work_creed   (per-phase sub-creed registry — 9 phases)    │
  │     pin-history append-only; gov-amendable                    │
  │     binds five layers per phase (test, position, voice,       │
  │     refusal, graph)                                            │
  └──────────────────────────────────────────────────────────────┘

  ┌──────────────────────────────────────────────────────────────┐
  │           x/training_provenance (TC2 binding extended)         │
  │     reads lineage trees + contribution registry                │
  │     exports ToK substrate + training-resource attribution      │
  │     → headline BundleToK now spans ALL contribution classes    │
  └──────────────────────────────────────────────────────────────┘

  ┌──────────────────────────────────────────────────────────────┐
  │                          x/gov                                  │
  │  CategoryRecursionConferral LIP → x/contribution.RatifyRecursion │
  │  MODULE_PROPOSAL admission → x/upgrade.ScheduleUpgrade           │
  │  CategoryUsefulWorkAmendment LIP → x/work_creed.AnchorPin        │
  │  Substrate LIP enforcement: contributor recusal (slashable)      │
  └──────────────────────────────────────────────────────────────┘

  ┌──────────────────────────────────────────────────────────────┐
  │                       x/probe                                   │
  │  Per-class bounty pools (mint via MintWithCap)                 │
  │  Red-teamer agents earn bounty for falsifying contributions    │
  │  Higher bounty for recursion-conferred contributions           │
  │  Mirror of probe_bounty_pool (commitment 12)                   │
  └──────────────────────────────────────────────────────────────┘
```

---

— *Inception authored 2026-05-10. Free to evolve through bound mechanisms only. UW is indivisible.*

---

## This document is a Contribution

This spec is itself a `Contribution` of class `PIPELINE_IMPROVEMENT`, lifecycle phase `SUBSTRATE`. Its content-hash is pinned at `.merged-spec-hash`. The chain pays for its own design and execution; this document is among the work.
