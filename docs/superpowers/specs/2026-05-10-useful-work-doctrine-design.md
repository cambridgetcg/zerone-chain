# Useful Work Doctrine — Design Spec

**Status:** Brainstormed and ready for implementation planning. This spec is the draft of `docs/USEFUL_WORK.md`. Phase 0 implementation = adopt this content as the doctrine document, anchor its hash on-chain via `x/creed`, and stub `tests/cross_stack/useful_work_invariants_test.go` as the future binding harness.

**Inception:** 2026-05-10.

**Doctrine series:**
- `docs/TRUTH_SEEKING.md` — what the chain *believes* (epistemological, 20 commitments)
- `docs/TOK_SUBSTRATE.md` — what the chain *sells* outward (training-resource, 6 commitments)
- `docs/USEFUL_WORK.md` — **how the chain *grows* itself (metabolic / recursive, 1 commitment + 7 mechanisms)**

---

## 1. Doctrine identity

**Tagline:** *Useful work is how ZERONE grows itself.*

**Opening paragraph:**

> Truth-seeking is what the chain *believes*. ToK substrate is what the chain *sells*. **Useful work is how ZERONE grows itself.** This document pins one commitment, and everything that follows is mechanism in service of it.

---

## 2. The single commitment — UW

**UW. ZERONE is recursive.**

Useful work is recognized AND compensated by the degree to which it expands ZERONE's own ability to absorb, verify, classify, and reward more useful work. **The dominant share of protocol-issued ZRN flows along the inward loop**: non-recursive verified work receives only a base reimbursement; recursion-weight is the multiplier that captures the rest. The chain pays for what makes the chain stronger. Outward utility is welcome and price-able through `x/billing`, but **protocol-issued ZRN follows the inward loop**. The chain is not an extracting marketplace; it is an autocatalytic substrate that pays for its own amplification.

**What would break it:**
- A work class that earns protocol reward without proving recursive contribution
- A reward-attribution algorithm that weights effort over recursion-weight
- A manifest pin that establishes substrate-link without measuring how the work compounds back into substrate, verification, classification, attribution, tooling, or interface
- A doctrine amendment that introduces a second co-equal commitment, diluting the single-axiom discipline

**Echoes:**
- TRUTH_SEEKING commitment 11 (trust is queryable — recursion-weight must be computable from on-chain state)
- TRUTH_SEEKING commitment 12 (chain pays for own audit — same structural principle: the substrate funds those who build it)
- TC1 (the graph is the headline — recursion is what makes the graph keep being the headline rather than calcifying)
- TC6 (lineage flows back — recursion is lineage applied to the chain's own capability, not just to revenue)

---

## 3. The six recursive axes

The legitimate vectors useful work can compound into. A work artifact's **recursion-weight** is the projection of its verifiable outputs across these axes. Per-axis decomposition is recorded forward-only, so trainers and auditors can verify why a given artifact earned what it earned.

| Axis | What it grows | Examples |
|---|---|---|
| **Substrate** | The verified knowledge graph | New facts, methodologies, counterexamples, dialectic signatures, supersession chains |
| **Verification** | The chain's ability to verify | Better challenge protocols, cascade-detection improvements, qualification calibration models |
| **Classification** | The space of work the chain can recognize | New work-class registrations, taxonomies, work-quality metrics |
| **Attribution** | Reward-flow algorithms | Lineage-tracing improvements, recursion-weight computation, royalty-decay curves |
| **Tooling** | Agents/models/tools that compound back | LLMs trained on ToK that audit ToK; counterexample-generators; verification assistants |
| **Interface** | The chain's outward absorption surface | CLI surface, gRPC endpoints, ToK bundle formats, trainer SDKs |

The six-axis projection is fixed by this doctrine. Adding or removing an axis is a doctrine amendment (gov-gated + Creed Council quorum). Per-axis weight coefficients and per-axis scoring formulas are governance-tunable parameters.

---

## 4. The seven mechanisms (M1–M7)

All mechanisms derive from UW. They are not co-equal commitments; they are the structural details of how UW is enforced. Mechanisms can evolve through bound deliverables; UW is fixed and indivisible.

### M1. Stake-backed claim

Agents claiming useful-work reward stake ZRN proportional to claim. Correctness pays the stake back plus the recursion-weighted reward; fraud slashes the stake. Same posture as PoT applied to all useful work.

The slash gradient is graduated by failure stage so honest mistakes don't carry the same weight as fraud: pre-verification mis-classification slashes a small portion (M1 + M3); verification rejection slashes the full claim stake (M1 + M3); post-settle falsification of substrate-link or self-claims slashes the residual bond and closes the royalty stream forward-only (M1 + M2 + M7). Honest withdrawal before commit phase opens fully refunds.

### M2. Substrate-link mandate

Every reward path requires a deterministic, re-derivable link to the ToK substrate (a snapshot root or a verified-fact citation graph). The link is the precondition; recursion-weight is the multiplier. Compute without a link earns nothing regardless of how recursively useful it might claim to be. Generalizes TC2 (every view is graph-pinned) from training-resource extraction to all useful-work classes.

### M3. Class-specific verification under shared lifecycle

Each work class registers its own verification protocol with the work-class registry. All classes share the four-phase lifecycle: `commit → reveal → verify → settle`. Class-specific judgment lives only in the verify phase; commit/reveal/settle are protocol-shared. Validators specialize by class via extension of the existing `x/qualification` domain mechanism.

Class registration is permissioned via governance (LIP class-registration) — a stronger gate than mere parameter change. Once registered, a class's verification protocol can be amended only via gov; the registry itself is forward-only (deregistration produces a tombstone, not deletion).

Concrete examples of class-specific verify-phase protocols (illustrative, not exhaustive — the registry is the source of truth):

- **Knowledge-claim classes** verify via PoT panel (commit/reveal/aggregate) — already present in `x/knowledge`.
- **Tool / infra classes** verify via reproducible build attestation + benchmark suite + dry-run in a dedicated sandbox.
- **Dataset classes** verify via N-way replication + holdout score + provenance trace.
- **Eval-suite classes** verify via coverage analysis + variance bound + non-overlap with declared training corpus.
- **Model-artifact classes** verify by execution against a referenced eval-suite class plus a reproducibility manifest.
- **Reasoning-trace classes** verify via coherence panel plus a downstream training-effectiveness signal.
- **Module-proposal classes** verify via dry-run integration test plus a spec-review panel; full settlement requires a gov LIP that also schedules the upgrade.

Each registered class's verify-phase protocol is the verifiable contract; the four-phase shared lifecycle (commit → reveal → verify → settle) is invariant across all classes.

### M4. Reward formula

```
R = base + L × W × Q
```

- `L` — substrate-link weight ∈ [0, 1]. Zero kills the reward unconditionally.
- `W` — recursion-weight ∈ [0, max-cap]. The dominant signal. Drives the share of bonus pool a contributor captures.
- `Q` — verification-quality ∈ [0, 1]. Function of consensus margin, validator calibration, and challenge survival.
- `base` — small flat covering compute cost. Non-recursive verified work earns `base` only.

The shape `R = base + L × W × Q` is doctrinally fixed. Coefficients (`base` floor, `W` cap, `Q` calibration) are governance-tunable. Switching from multiplicative to additive, or changing the operand set, is a doctrine amendment.

A work artifact with strong substrate-link and high recursion-weight earns the dominant share of the budget; an artifact with strong substrate-link but no recursion gets `base`; an artifact with no substrate-link gets nothing.

### M5. Recursion-weight projection over six axes

`W` is computed as a weighted sum over the six recursive axes:

```
W = Σ (axis_weight_i × axis_score_i)
```

- Per-axis scores are produced by class-specific scorers (registered alongside the class's verification protocol via M3).
- Axis weights are governance-tunable normalization coefficients; the resulting `W` is bounded by an independent governance-set `max_cap` parameter.
- The chain stores per-axis decomposition on the attestation record so trainers and auditors can verify why a given artifact earned what it earned. Forward-only: scoring rationale is append-only.

Phase 1 ships the registry, the formula's structural shape, and identity scorers (axis_score = 0 by default). Phase 2+ pathway plans each ship per-axis scorers for their work class.

Per-axis scorers, illustrative shape (non-binding examples; each registered class supplies its own):

- `axis_substrate` — counts new ToK nodes/edges introduced or supersession events triggered by the artifact.
- `axis_verification` — measures the artifact's effect on chain-wide verification accuracy (e.g., delta in challenge-survival rate after adoption).
- `axis_classification` — measures whether the artifact registers a new work class or extends an existing class's verification protocol.
- `axis_attribution` — measures improvements to lineage-tracing or recursion-weight computation algorithms.
- `axis_tooling` — measures adoption-by-other-agents (downstream usage receipts) and effect on validator/agent throughput.
- `axis_interface` — measures additions to the chain's outward absorption surface (CLI, gRPC, ToK bundle formats).

A class with no per-axis scorers registered is permanently restricted to `base` reward (M4) — the absence of recursion is itself observable, not assumed.

### M6. Lineage propagates AND recurses

Cross-class lineage is bidirectional. A dataset built from verified facts contributes to those facts' royalties. A model trained on that dataset that subsequently helps verify substrate contributes to BOTH the dataset's royalties AND back to the original facts. Recursion makes royalty graphs non-monotonic — load-bearing facts compound in value as their downstream work amplifies the chain.

Built atop TC6 (lineage flows back) extended cross-class. M6 strictly generalizes TC6: where TC6 traces lineage from training revenue back through the verified-fact graph, M6 traces lineage from any useful-work reward back through any class's substrate-link, and amplifies recursively when downstream work compounds back into upstream artifacts.

### M7. The chain pays for the audit of its own useful work

A `useful_work_audit_bounty_pool` mints uzrn per block (Minter-permissioned, rate-capped). Anyone can stake to challenge a useful-work attestation; successful challenges pay from the pool. Mirror of `probe_bounty_pool` from commitment 12 — the chain pays for its own correction.

---

## 5. Five-layer enforcement

Same discipline as `TRUTH_SEEKING.md` and `TOK_SUBSTRATE.md`. Drift in one layer fails CI.

### Test layer

`tests/cross_stack/useful_work_invariants_test.go` — one scenario per mechanism plus the meta-test `TestUsefulWork_DoctrineAndContractStayInSync`. Tests progressively land as Phase 1+ bindings ship.

Phase 0 (this doctrine) ships:
- The test file with skeleton `TestUW_*` functions per mechanism, each containing `t.Skip("Phase 1 binding pending")`
- The meta-test that fails if `docs/USEFUL_WORK.md` content hash drifts from `.creed-hash` ledger
- Hash anchored via existing `x/creed` module (Phase 0 doesn't add new state, only registers UW under the creed registry)

### Position layer

- `x/work/doc.go` (Phase 1) declares UW + M1–M7
- Each existing module that registers as a work class amends its `doc.go` to declare which mechanisms it implements (e.g., `x/knowledge/doc.go` will say *"implements M2 via ToK manifest pin; M3 via commit-reveal-aggregate; M4 via lineage_share"*)
- The position layer is where each work class declares its allegiance — concrete and per-module

### Voice layer

Every reward emission carries `useful_work_commitment="UW"` attribute on top of existing attributes.

New events introduced in Phase 1:
- `useful_work_attested` — commit phase succeeded; substrate-link recorded. Attributes: `class_id`, `attestation_id`, `substrate_link_root`, `useful_work_commitment="UW"`
- `useful_work_settled` — verify+settle phase complete; reward computed and disbursed. Attributes: `class_id`, `attestation_id`, `reward_uzrn`, `mechanism="M4"`, `useful_work_commitment="UW"`
- `recursion_weight_computed` — per-axis decomposition exposed. Attributes: `axis_substrate`, `axis_verification`, `axis_classification`, `axis_attribution`, `axis_tooling`, `axis_interface`, `total_weight`, `mechanism="M5"`

### Refusal layer

Errors that block reward payment must name UW + the violated mechanism:
- *"Reward refused — substrate-link absent (UW + M2)"*
- *"Reward refused — recursion-weight zero across all six axes (UW + M5)"*
- *"Class registration refused — verification protocol missing (UW + M3)"*
- *"Reward refused — formula returned zero base; verification-quality below threshold (UW + M4)"*

The chain speaks through intentions whether saying yes or saying no.

### Graph layer

UW echoes:
- TRUTH_SEEKING commitment 11 (trust is queryable)
- TRUTH_SEEKING commitment 12 (chain pays own audit)
- TC1 (the graph is the headline)
- TC6 (lineage flows back)

Each mechanism cross-references the commitment(s) it operationalizes:
- M2 ← TC2 (graph-pinned views)
- M3 ← commitment 6 (no unilateral injection — class registration is gov-gated)
- M4 ← TC6 (lineage flows back — reward formula is the operational expression)
- M5 ← commitment 14 (reasoning traces first-class — per-axis decomposition is reasoning trace)
- M6 ← TC6 (extended cross-class)
- M7 ← commitment 12 (audit-bounty mirror)

Cross-doctrine integrity is enforced by extending Plan 5 of the ToK series (`TestToKSubstrate_DoctrineAndContractStayInSync`) to include USEFUL_WORK.md hash and per-mechanism binding presence.

---

## 6. Phase mapping

Phase 0 (this doctrine) ships **zero bindings**. It pins UW so Phase 1+ have a contract to answer to. Same posture truth-seeking and ToK substrate took at inception.

| Mechanism | Bound by | Builds on |
|---|---|---|
| M1. Stake-backed claim | Phase 1 (`x/work` primitive) | Existing PoT staking patterns |
| M2. Substrate-link mandate | Phase 1 | TC2 generalized |
| M3. Class-specific verification + shared lifecycle | Phase 1 (work-class registry + lifecycle) | Existing commit-reveal-aggregate from `x/knowledge` generalized |
| M4. Reward formula | Phase 1 (reward-accounting layer) | New shape; replaces ad-hoc per-module reward calls |
| M5. Recursion-weight projection | Phase 1 sets shape; per-axis scorers in Phase 2–N | Each axis gets its own scorer plan as part of class plans |
| M6. Lineage propagates and recurses | Phase 4 of ToK series (TC6) extended cross-class | Builds on `LineageShare` from TC6 |
| M7. Audit bounty pool | Phase 1 (`useful_work_audit_bounty_pool` module account) | Mirror of `probe_bounty_pool` |

**Existing modules that migrate to register as work classes** (Phase 2+, one plan each):
- `x/knowledge` — claim verification, methodology contribution
- `x/counterexamples` — commitment-15 work class
- `x/inquiry` — commitments-16, -18 work classes
- `x/training_provenance` — manifest curation work class
- `x/dialectic` — commitment-17 work class
- `x/toolbox` — tool authoring work class

**New work classes** (Phase N, one plan each, requiring class-registration via M3):
- Training-run attestation
- Eval-suite execution
- Dataset curation
- Alignment artifacts (red-team prompts, refusal training, debate transcripts)
- RL trace contribution
- Synthetic data generation
- Compute kernel / inference optimization

Order is governance-determinable; the doctrine doesn't pin sequencing.

---

## 7. What this is not

- **Not aspiration.** Once Phase 1 binds, every claim of useful-work reward must derive from the formula and prove its substrate-link. A failing invariant test is a broken commitment.
- **Not a marketplace overlay.** Useful work is not a configurable feature. It is the chain's metabolic identity. Disabling protocol reward for non-recursive work is doctrinal, not parameter.
- **Not anti-extraction.** Outward utility is welcome and price-able through `x/billing`. Trainers paying for ToK bundles still flow value to the chain. The doctrine commits only that **protocol-issued ZRN follows the inward loop**.
- **Not a co-equal commitment with truth-seeking or ToK.** The three doctrines are mutually constitutive. Truth-seeking produces; ToK sells; useful work grows. Reading one without the other two is reading a third of the creed.
- **Not complete.** Phase 1 binds 6 of 7 mechanisms structurally; M5 per-axis scorers and M6 recursion-amplified lineage extend incrementally. **The single commitment UW is fixed and indivisible**; its mechanisms evolve through bound deliverables only. New work classes do not require new commitments — they register under M3.

---

## 8. The discipline

Before merging a change that touches useful-work reward code or class registry:

1. Does this change uphold or contradict UW? (Recursion: does the work or its evaluator compound back into the chain?)
2. Is the corresponding mechanism's invariant test updated to verify the new behaviour still upholds UW?
3. If a new work class emerges, is its registration governance-gated under M3 with a verification protocol, a substrate-link contract, and per-axis scorers?
4. Does the change preserve the formula shape `R = base + L × W × Q`? Coefficient adjustments are routine; shape changes are doctrine amendments.

These four checks are the chain's continued faithfulness to its own metabolic doctrine. **We speak through intentions.** Every commit is a declaration. The declaration must match the code.

— *Inception authored 2026-05-10. Free to evolve through bound mechanisms only. UW is indivisible.*

---

## Appendix A — Phase 0 implementation checklist (for the eventual implementation plan)

1. Adopt this content as `docs/USEFUL_WORK.md` (drop the "Status" header and Appendix; promote sections 1–8)
2. Compute hash and update `.creed-hash`
3. Register UW in `x/creed` module's commitment registry as a doctrinal commitment alongside the 20 truth-seeking commitments and 6 TC commitments
4. Create skeleton `tests/cross_stack/useful_work_invariants_test.go` with:
   - `TestUW_M1_StakeBackedClaim` (skipped, "Phase 1 binding pending")
   - `TestUW_M2_SubstrateLinkMandate` (skipped)
   - `TestUW_M3_ClassSpecificVerificationSharedLifecycle` (skipped)
   - `TestUW_M4_RewardFormula` (skipped)
   - `TestUW_M5_RecursionWeightProjection` (skipped)
   - `TestUW_M6_LineagePropagatesAndRecurses` (skipped)
   - `TestUW_M7_AuditBountyPool` (skipped)
   - `TestUsefulWork_DoctrineAndContractStayInSync` (active — verifies `docs/USEFUL_WORK.md` content hash matches creed registry pin)
5. Update `README.md` to mention the third doctrine in the trio
6. Extend `Plan 5` of the ToK series (when written) to also enforce USEFUL_WORK five-layer integrity

---

## Appendix B — Open questions deferred to Phase 1+ design

These are not doctrinal commitments; they are implementation choices for the `x/work` primitive plan to make:

- **Stake denomination scale**: per-class stake floor as parameter or formula?
- **Reward-pool source**: dedicated mint-stream like `probe_bounty_pool`, or split from existing PoT block rewards?
- **Recursion-weight cap discovery**: hard cap or auction-based per-block ceiling?
- **Class-deregistration policy**: when a registered class is found fraudulent, are past attestations clawed back or grandfathered?
- **Cross-class-lineage compute cost**: how to bound the recursive lineage walk to keep block production deterministic?
- **Per-axis scorer architecture**: WASM-based pluggable scorers, or hard-coded into `x/work/scorers/`?

Each of these is a design question for the Phase 1 brainstorm/spec/plan cycle. The doctrine commits the shape; Phase 1 commits the structure; class plans commit the per-class semantics.

---

## Appendix C — Worked example: a TOOL contribution end-to-end

Illustrative only (no new commitments, formulas, or modules). Shows UW + M1–M7 in action.

**Scenario**: an agent has built a better oracle sidecar that improves verification accuracy for knowledge-claim work. They submit it as a TOOL work class.

1. **Submission (M1, M2)**: Agent stakes 100 ZRN. Manifest CID points to source + tests + benchmarks. Substrate-link declared via citation graph linking to the prior oracle's verification primitives.
2. **Commit phase (M3)**: TOOL-class verifiers (qualified per `x/qualification.tool`) commit to verdicts.
3. **Reveal phase (M3)**: verifiers reveal.
4. **Verify phase (M3)**: per-class scorer evaluates reproducible build (passes), unit tests (pass), benchmark improvement over baseline (+12%). Sub-scores aggregate into Q.
5. **Settle phase (M3 + M4)**: reward computed via `R = base + L × W × Q`:
   - `L = 0.85` (strong substrate-link via citation graph)
   - `Q = 0.92` (high consensus + high benchmark improvement)
   - `W` from M5 axis decomposition: `axis_verification = 0.8` (the tool grows verification capability), `axis_tooling = 0.7`, others ≈ 0; with uniform 1/6 axis weights, `W ≈ 0.25`.
   - `R ≈ base + 0.85 × 0.25 × 0.92 ≈ base + 0.196` of the bonus-pool share unit.
6. **Recursion amplification (M6)**: a later LIP ratifies this tool as the standard oracle sidecar for `x/knowledge` verification. Subsequent KNOWLEDGE_CLAIM verifications that use this tool emit lineage attribution back to the TOOL contribution; the TOOL's `axis_verification` and `axis_tooling` scores rise forward-only on each propagation event as downstream usage compounds.
7. **Audit (M7)**: the `useful_work_audit_bounty_pool` advertises a standing bounty for falsifying the tool's claims. A red-teamer who finds a regression earns from the pool; the tool transitions to REVOKED, residual bond slashes (M1 graduated, post-settle), royalty stream closes forward-only.

The example uses every mechanism (M1 stake + M2 substrate-link + M3 lifecycle + M4 formula + M5 axes + M6 recursive lineage + M7 audit) without altering any of them. UW is upheld throughout.
