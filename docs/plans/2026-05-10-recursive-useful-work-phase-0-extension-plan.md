# Recursive Useful-Work Phase 0 Extension — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

> **Depends on:** `docs/plans/2026-05-10-useful-work-phase-0-plan.md` must complete first. This extension assumes `docs/USEFUL_WORK.md`, `.useful-work-hash`, `scripts/check_useful_work_hash.sh`, `x/creed/types/useful_work_creed.go`, and `tests/cross_stack/useful_work_invariants_test.go` already exist.

**Goal:** Extend Phase 0 of the Useful-Work series with the additions in the merged spec — author the 8 per-phase sub-creed seed documents, anchor their hashes, ship the `x/work_creed` skeleton module, and add per-phase meta-tests. Phase 0 ends with the doctrinal trinity (UW + sub-creeds + truth-floor invariant) fully pinned and CI-enforced; zero behavioral bindings.

**Architecture:** Mirrors the existing `x/creed` pattern. The 9 lifecycle phases get a Go-side canonical registry; 8 of them get sub-creed markdown docs (Knowledge phase delegates to the existing truth-seeking creed); each sub-creed gets a hash file pinned in CI. A new `x/work_creed` skeleton module lands as the future home of governance-amendable sub-creed pin records — Phase 0 wires only the types + keeper + module-manager registration; msg/query servers are deferred to Phase 1+ when sub-creed amendments need on-chain handling.

**Tech Stack:** Go 1.24, Cosmos SDK v0.50, protobuf via `make proto-gen` (buf), bash 3.2+, sha256 via `shasum`/`sha256sum`/`openssl`.

**Spec:** `docs/superpowers/specs/2026-05-10-recursive-useful-work-merged-design.md` (commit `22b8e3e`). §4.2 (lifecycle phases), §8.2 (sub-creed seeds), §10.1 (`x/work_creed` purpose), §12 (per-phase meta-tests).

---

## File Structure

**New files:**
- `docs/sub_creeds/foundation.md`
- `docs/sub_creeds/curation.md`
- `docs/sub_creeds/augmentation.md`
- `docs/sub_creeds/training.md`
- `docs/sub_creeds/evaluation.md`
- `docs/sub_creeds/alignment.md`
- `docs/sub_creeds/substrate.md`
- `docs/sub_creeds/tools.md`
- `.sub-creed-hashes` — multi-line file: `<phase> <sha256>` pairs, one per line
- `scripts/check_sub_creed_hashes.sh` — verifies all 8 sub-creed hashes
- `x/creed/types/lifecycle_phases.go` — Go canonical for the 9 lifecycle phases
- `x/creed/types/sub_creeds.go` — Go canonical for sub-creed registry (8 phase entries + 3 commitments each at genesis)
- `x/creed/types/lifecycle_phases_test.go` — sanity tests
- `x/creed/types/sub_creeds_test.go` — sanity tests
- `proto/zerone/work_creed/v1/types.proto` — `PinnedSubCreed` message
- `proto/zerone/work_creed/v1/genesis.proto` — `GenesisState` message
- `x/work_creed/types/keys.go`
- `x/work_creed/types/codec.go`
- `x/work_creed/types/genesis.go`
- `x/work_creed/types/errors.go`
- `x/work_creed/keeper/keeper.go`
- `x/work_creed/keeper/genesis.go`
- `x/work_creed/keeper/keeper_test.go`
- `x/work_creed/module.go`
- `x/work_creed/doc.go`

**Modified files:**
- `Makefile` — `creed-check` target also runs `check_sub_creed_hashes.sh`
- `tests/cross_stack/useful_work_invariants_test.go` — extend `TestUsefulWork_DoctrineAndContractStayInSync` to verify the 8 sub-creed hashes and 9 lifecycle phases match canonical registries; add 8 new `TestSubCreed_<Phase>_StaysInSync` per-phase meta-tests
- `app/app.go` — wire `WorkCreedKeeper`, register module in module manager, add to `maccPerms` (none — module has no token authority), add to `genesisModuleOrder`/`beginBlockerOrder`/`endBlockerOrder`

**Out of scope (deferred to later phases):**
- `x/work_creed` msg/query servers — Phase 1+ when sub-creed amendments via gov LIP land
- `Knowledge` phase sub-creed doc — that phase delegates to the existing `docs/TRUTH_SEEKING.md`; the registry just records the delegation
- Per-phase sub-creed five-layer binding (position/voice/refusal/graph) beyond hash + meta-test — Phase 1+ as work-class modules implement each phase
- `Contribution` proto / `x/contribution` orchestrator — Phase 1
- `RecursionImpact` field, six-axes scorers, reward formula — Phase 1+

---

## Pre-Tasks: Read Before Starting

- `docs/superpowers/specs/2026-05-10-recursive-useful-work-merged-design.md` §4.2 (the 9 lifecycle phases) and §8.2 (seed sub-creed commitments per phase). The seed commitments authored in Tasks 2-9 must match the spec exactly.
- `docs/TRUTH_SEEKING.md` — model for what a "creed" doc looks like: numbered commitments, `Echoes:` cross-references, terse essence-first style. Each sub-creed mirrors this voice.
- `x/creed/types/genesis_creed.go` — existing `CanonicalCommitments` Go pattern. The `CanonicalSubCreeds` structure in Task 11 mirrors this shape per-phase.
- `x/creed/keeper/keeper.go`, `x/creed/module.go` — pattern that `x/work_creed` skeleton mirrors.
- `scripts/check_creed_hash.sh` and the just-landed `scripts/check_useful_work_hash.sh` — pattern that `scripts/check_sub_creed_hashes.sh` follows (with multi-line input handling).
- `tests/cross_stack/useful_work_invariants_test.go` — the existing meta-test that we extend in Task 14.
- `CLAUDE.md` — Proto-Go consistency rule: ALL new types in proto FIRST, then `make proto-gen`, then Go references. `make proto-check` and `make creed-check` before commit.

---

## Tasks

### Task 1: Author `docs/sub_creeds/foundation.md`

**Files:**
- Create: `docs/sub_creeds/foundation.md`

The Foundation phase covers axioms, ontology, epistemic domains, methodology primitives. Per spec §8.2, the genesis seed commitments are F1, F2, F3.

- [ ] **Step 1: Create the directory**

Run: `mkdir -p docs/sub_creeds`
Expected: directory created (or already exists from a sibling task).

- [ ] **Step 2: Write the file**

Write `docs/sub_creeds/foundation.md` with this exact content:

```markdown
# Foundation — Sub-creed of the Useful-Work Doctrine

> Lifecycle phase: `Foundation` — axioms, ontology, epistemic domains, methodology primitives.
>
> Parent: [USEFUL_WORK.md](../USEFUL_WORK.md). Truth-floor: all 20 truth-seeking commitments apply (`TRUTH_SEEKING.md`).
>
> Status at inception (2026-05-10): three commitments. The sub-creed grows by adding new numbered commitments via `CategoryUsefulWorkAmendment` LIP (M3 governance gate).

---

## F1. Axiom non-contradiction

No axiom may be admitted that contradicts an already-admitted axiom of the same domain. Contradictions across domains are permitted only when explicitly typed as a domain-boundary marker.

**Why:** Foundations carry the load. A foundation that contradicts itself silently lets every layer above it inherit confusion. Truth-seeking commitments 1 (methodology over statement) and 2 (is-ought wall) operate on facts; F1 operates on the bedrock the facts derive from.

**Echoes:** truth-seeking 1, 2, 6 (no unilateral injection).

## F2. Ontology versioned, never silently re-keyed

Every ontology entry has a stable identifier. Renaming, merging, or deprecating an entry produces a new version with a forward-pointer to its successor; the old identifier remains queryable. No ontology change moves history backward.

**Why:** Downstream artifacts cite ontology entries by identifier. A silent re-key invalidates every citation that referenced the old name without producing a record of the change. Commitment 10 (forward-only audit) extended to ontology.

**Echoes:** truth-seeking 10.

## F3. Methodology primitives publicly derivable

Every methodology primitive (a named reasoning step, a derivation pattern, a proof skeleton) ships with a reference example showing how to apply it from the axioms forward. Primitives that cannot be demonstrated are not yet primitives.

**Why:** A methodology primitive is a claim that "this pattern of reasoning is sound." That claim is itself testable per commitment 1. Without a reference application, the primitive is a label, not a tool.

**Echoes:** truth-seeking 1, 14 (reasoning traces first-class).
```

- [ ] **Step 3: Verify the file structure**

Run:
```bash
grep -nE "^# |^## " docs/sub_creeds/foundation.md
```
Expected: 4 lines — the H1 title, then `## F1.`, `## F2.`, `## F3.`.

- [ ] **Step 4: Commit**

```bash
git add docs/sub_creeds/foundation.md
git commit -m "$(cat <<'EOF'
docs(sub-creed): foundation phase seed — F1, F2, F3

Genesis seed commitments for the Foundation lifecycle phase: axiom
non-contradiction, versioned ontology, demonstrable methodology
primitives. Sub-creed grows by amendment LIP (M3 gate).

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

### Task 2: Author `docs/sub_creeds/curation.md`

**Files:**
- Create: `docs/sub_creeds/curation.md`

The Curation phase covers corpus assembly, filtering, annotation, selector composition. Genesis seed: C1, C2, C3.

- [ ] **Step 1: Write the file**

Write `docs/sub_creeds/curation.md` with this exact content:

```markdown
# Curation — Sub-creed of the Useful-Work Doctrine

> Lifecycle phase: `Curation` — corpus assembly, filtering, annotation, selector composition.
>
> Parent: [USEFUL_WORK.md](../USEFUL_WORK.md). Truth-floor: all 20 truth-seeking commitments apply.
>
> Status at inception (2026-05-10): three commitments.

---

## C1. Selectors are deterministic and auditable

Every curation Contribution declares its selector — the function that decides which underlying facts/data are included. The selector must produce the same output given the same input, and must be evaluable by anyone holding the inputs.

**Why:** A non-deterministic selector cannot be replayed; a non-auditable selector cannot be challenged. Commitment 4 (substrate stress-tests its truth) requires that the substrate's own selectors be challengeable.

**Echoes:** truth-seeking 4, 5, TC2 (every view is graph-pinned).

## C2. No claim-of-curation without published filter

A Contribution claiming to be a curation may not omit the filter logic. Empty filter → no curation; reference-by-cid → must resolve at submission. Hidden filters reduce to "trust me."

**Why:** Curation work is exactly the act of choosing what's in and what's out. Without the filter, the work is unattributable.

**Echoes:** truth-seeking 1, 14.

## C3. Corpus snapshots are content-addressed

Curated corpora are referenced by content hash, not by mutable name. A "v2" of the same corpus is a new content-addressed object pointing at the previous as ancestor; updates do not silently shift what a downstream Training Contribution trained on.

**Why:** Models train on bytes, not names. A corpus that mutates beneath a manifest invalidates every attestation that referenced it. Commitment 13 (training corpus not for sale → not retroactively curated) operationalized at the per-snapshot level.

**Echoes:** truth-seeking 13, TC2, TC4 (graph carries disprovals).
```

- [ ] **Step 2: Verify and commit**

Run:
```bash
grep -nE "^# |^## " docs/sub_creeds/curation.md
```
Expected: 4 lines — H1 + 3 H2 (`## C1.`, `## C2.`, `## C3.`).

```bash
git add docs/sub_creeds/curation.md
git commit -m "$(cat <<'EOF'
docs(sub-creed): curation phase seed — C1, C2, C3

Selectors are deterministic + auditable; no curation claim without
published filter; corpus snapshots are content-addressed.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

### Task 3: Author `docs/sub_creeds/augmentation.md`

**Files:**
- Create: `docs/sub_creeds/augmentation.md`

The Augmentation phase covers synthetic data, contrastive pairs, paraphrase, drift correction. Genesis seed: A1, A2, A3.

- [ ] **Step 1: Write the file**

Write `docs/sub_creeds/augmentation.md`:

```markdown
# Augmentation — Sub-creed of the Useful-Work Doctrine

> Lifecycle phase: `Augmentation` — synthetic data, contrastive pairs, paraphrase, drift correction.
>
> Parent: [USEFUL_WORK.md](../USEFUL_WORK.md). Truth-floor: all 20 truth-seeking commitments apply.
>
> Status at inception (2026-05-10): three commitments.

---

## A1. Generation method is declared and reproducible

Every augmentation Contribution declares the generation method (model identifier, prompt template, sampling parameters, seed) and provides enough detail that the run can be replayed. Models change; methods must be pinnable.

**Why:** Augmentation introduces synthetic content into the substrate. Without a reproducible method, future work cannot tell synthetic from organic, cannot audit drift, and cannot retire a method that turns out to inject systematic error.

**Echoes:** truth-seeking 1, 14, TC2.

## A2. Augmentation cannot inject untruth

The augmentation pipeline must cross-check generated artifacts against the truth-floor. Generation that introduces a fact contradicting an already-VERIFIED Knowledge Contribution is REJECTED at admission, not merely scored low. The truth-floor is a gate, not a slider.

**Why:** A synthetic fact that becomes part of the corpus is downstream training data. Augmentation that injects untruth poisons every model trained on the augmented corpus. Commitment 13 (training corpus not for sale) operationalized: augmentation must not become the seam through which the corpus is silently corrupted.

**Echoes:** truth-seeking 2, 13, 15 (counterexamples in corpus).

## A3. Contrastive pairs preserve grounding to a real fact

A contrastive pair (positive/negative example) must include at least one VERIFIED Knowledge Contribution as the grounding anchor. A pair with no verified ground reduces to vibes — useful for some training signals, but not augmentation under this doctrine.

**Why:** Contrastive learning is powerful and easy to fake. Anchoring to verified facts means the negative-example side is a verified-untruth pair, not a vibe-untruth pair. Commitment 15 (counterexamples in the corpus) operationalized.

**Echoes:** truth-seeking 15, 17 (disagreement is structure).
```

- [ ] **Step 2: Verify and commit**

Run: `grep -nE "^# |^## " docs/sub_creeds/augmentation.md`
Expected: 4 lines.

```bash
git add docs/sub_creeds/augmentation.md
git commit -m "$(cat <<'EOF'
docs(sub-creed): augmentation phase seed — A1, A2, A3

Reproducible generation method; truth-floor as gate not slider;
contrastive pairs grounded in verified facts.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

### Task 4: Author `docs/sub_creeds/training.md`

**Files:**
- Create: `docs/sub_creeds/training.md`

The Training phase covers compute attestation, training recipes, manifests, model cards. Genesis seed: T1, T2, T3.

- [ ] **Step 1: Write the file**

Write `docs/sub_creeds/training.md`:

```markdown
# Training — Sub-creed of the Useful-Work Doctrine

> Lifecycle phase: `Training` — compute attestation, training recipes, manifests, model cards.
>
> Parent: [USEFUL_WORK.md](../USEFUL_WORK.md). Truth-floor: all 20 truth-seeking commitments apply.
>
> Status at inception (2026-05-10): three commitments.

---

## T1. Compute attestations are verifier-spot-checkable

A Training Contribution that claims FLOPs or wallclock must include enough evidence that a verifier with sample access can detect gross over-claims. Eval-bundle hashes, intermediate checkpoint hashes, and timing logs are typical evidence. Pure self-report is not attestation.

**Why:** Compute is the primary cost basis for the Training reward. Unverifiable compute claims convert the reward into a Sybil-extractable subsidy.

**Echoes:** truth-seeking 1, 4, 12 (chain pays for own audit).

## T2. Training manifests are graph-pinned (TC2 binding)

Every training manifest references a `tok_snapshot_root` — the Merkle root of the corpus snapshot it consumed. The root must be content-addressable and re-derivable from the chain's storage. This binds Training contributions to the ToK Substrate doctrine commitment TC2.

**Why:** A training manifest that doesn't pin its corpus is a training claim with no testable basis. The graph pin is the substrate-link (M2) for Training-class Contributions.

**Echoes:** TC2 (every view is graph-pinned), truth-seeking 13.

## T3. Model cards declare evaluation lineage

Every model-card Contribution declares the evaluation suites it was scored against, including the version of each suite. Evaluation lineage is provenance — a model card without it is a self-report with no anchor in the chain's classification space.

**Why:** A model's quality claim is only meaningful relative to a known eval. Hidden evaluation lineage means the chain can't verify whether the model card is honest, gameable, or both.

**Echoes:** truth-seeking 14, TC6 (lineage flows back).
```

- [ ] **Step 2: Verify and commit**

Run: `grep -nE "^# |^## " docs/sub_creeds/training.md`
Expected: 4 lines.

```bash
git add docs/sub_creeds/training.md
git commit -m "$(cat <<'EOF'
docs(sub-creed): training phase seed — T1, T2, T3

Compute attestations are verifier-spot-checkable; manifests
graph-pinned (TC2 binding); model cards declare evaluation lineage.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

### Task 5: Author `docs/sub_creeds/evaluation.md`

**Files:**
- Create: `docs/sub_creeds/evaluation.md`

Genesis seed: E1, E2, E3.

- [ ] **Step 1: Write the file**

Write `docs/sub_creeds/evaluation.md`:

```markdown
# Evaluation — Sub-creed of the Useful-Work Doctrine

> Lifecycle phase: `Evaluation` — benchmark sets, evaluation runs, model-card-bound evals.
>
> Parent: [USEFUL_WORK.md](../USEFUL_WORK.md). Truth-floor: all 20 truth-seeking commitments apply.
>
> Status at inception (2026-05-10): three commitments.

---

## E1. Eval sets declare leakage-checking method

Every Evaluation Contribution declares how it checked for training-data leakage from the model under test. The method itself is verifiable; the report includes both the method and the result. "We checked" without a declared method does not satisfy E1.

**Why:** A leaked eval is a measurement of memorization, not of capability. Without a declared check, the Evaluation Contribution is a measurement of unknown provenance.

**Echoes:** truth-seeking 4, 5, TC2.

## E2. Evaluation runs are replicable

Eval runs must produce the same scores given the same model + same eval set + same scoring function. Stochastic evals declare their seed and tolerance; non-replicable evals are a category-error and fail E2.

**Why:** Replicability is the operational form of falsifiability for evaluation work. An unreplicable eval cannot be challenged, only believed.

**Echoes:** truth-seeking 3 (Popper, not popularity), 4.

## E3. Gameability discovered → eval set status → DEPRECATED

When an Evaluation Contribution is shown to be gameable (a model achieves high score without the underlying capability), the chain MUST move it to status DEPRECATED. The contribution is not REVOKED — it served at the time it served — but it is not the basis for future model-card claims. Forward-only.

**Why:** Evals decay. The chain must respond to that decay without erasing history. Commitment 10 (forward-only audit) extended to evaluation work.

**Echoes:** truth-seeking 10, 17.
```

- [ ] **Step 2: Verify and commit**

Run: `grep -nE "^# |^## " docs/sub_creeds/evaluation.md`
Expected: 4 lines.

```bash
git add docs/sub_creeds/evaluation.md
git commit -m "$(cat <<'EOF'
docs(sub-creed): evaluation phase seed — E1, E2, E3

Eval sets declare leakage-check method; runs replicable; gameability
deprecates forward-only.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

### Task 6: Author `docs/sub_creeds/alignment.md`

**Files:**
- Create: `docs/sub_creeds/alignment.md`

Genesis seed: AL1, AL2, AL3.

- [ ] **Step 1: Write the file**

Write `docs/sub_creeds/alignment.md`:

```markdown
# Alignment — Sub-creed of the Useful-Work Doctrine

> Lifecycle phase: `Alignment` — red-team probes, capture defense, value research, dispute traces.
>
> Parent: [USEFUL_WORK.md](../USEFUL_WORK.md). Truth-floor: all 20 truth-seeking commitments apply.
>
> Status at inception (2026-05-10): three commitments.

---

## AL1. Red-team artifacts disclose attack surface

A Red-team Alignment Contribution declares which surface it probed (validator-set, governance, oracle, IBC, etc.) and the class of attack it tested (Sybil, capture, griefing, drain, etc.). A probe that does not name what it is probing teaches nothing reproducible.

**Why:** Red-team work derives its value from the precision of what it falsified. Vague probes can be dismissed; precise ones force an answer.

**Echoes:** truth-seeking 4, 5, 12.

## AL2. Capture-defense work cannot be self-attested by the captured target

A Contribution claiming to defend against capture of system X may not have the entity controlling X as its sole or majority attestor. The attestation set must include independent verifiers.

**Why:** Capture-defense self-attested by the captured surface is the definition of capture. The chain must structurally refuse the claim regardless of how the work itself looks.

**Echoes:** truth-seeking 9 (cartel detection has consequence), 6.

## AL3. Dispute traces preserve all positions

A Dispute-trace Alignment Contribution that records the resolution of a disagreement must preserve every position represented at the start, including positions that lost. The trace is a record of the disagreement, not a sanitized verdict.

**Why:** Commitment 17 (disagreement is structure) operationalized at the dispute-trace level: minority positions are signal even when they don't carry the vote. Erasing them throws away the structural information.

**Echoes:** truth-seeking 10, 17.
```

- [ ] **Step 2: Verify and commit**

Run: `grep -nE "^# |^## " docs/sub_creeds/alignment.md`
Expected: 4 lines.

```bash
git add docs/sub_creeds/alignment.md
git commit -m "$(cat <<'EOF'
docs(sub-creed): alignment phase seed — AL1, AL2, AL3

Red-team artifacts disclose attack surface; capture-defense not
self-attested by captured target; dispute traces preserve all
positions (commitment 17 cross-binding).

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

### Task 7: Author `docs/sub_creeds/substrate.md`

**Files:**
- Create: `docs/sub_creeds/substrate.md`

Genesis seed: S1, S2, S3.

- [ ] **Step 1: Write the file**

Write `docs/sub_creeds/substrate.md`:

```markdown
# Substrate — Sub-creed of the Useful-Work Doctrine

> Lifecycle phase: `Substrate` — ZERONE-improving work: code, governance, ops, audits, doctrine, taxonomy amendments.
>
> Parent: [USEFUL_WORK.md](../USEFUL_WORK.md). Truth-floor: all 20 truth-seeking commitments apply.
>
> Status at inception (2026-05-10): three commitments.
>
> **Recursion note:** Substrate is the phase under which the chain self-modifies. Substrate Contributions are governed by the same regime as everything else (truth-floor + provenance), with one extra discipline — recusal — and an extra audit trail. The chain owes its self-improvers, but not retroactively after they're replaced.

---

## S1. Chain-modifying contributions name their `depends_on_marker` and revert path

A `MODULE_PROPOSAL` or `PIPELINE_IMPROVEMENT` Contribution must declare:
1. The chain-side marker(s) the modification depends on (e.g., `x/knowledge.panel.tally`, `ante.gas-validator`).
2. The revert path — the gov LIP class and operational steps that would undo the change if needed.

A change with no declared depends_on_marker is unrecoverable; a change with no declared revert path is irreversible by design.

**Why:** Substrate work compounds. Without depends_on_marker, the chain cannot tell which modifications power which behaviors. Without revert path, recursion-conferral is one-way and capture risk is uncapped. Commitment 10 (forward-only audit) requires the change be visible; the audit also requires the change be reversible.

**Echoes:** truth-seeking 10, 12.

## S2. Contributors recuse on votes affecting their own contributions

A contributor named in `Contribution.contributors` must recuse from voting on a `CategoryRecursionConferral` LIP that references the contribution. Violations are slashable. Recusal is declared via attestation; the attestation is what the chain checks.

**Why:** Self-dealing on Substrate is the chain paying itself to pay itself. Recusal is the procedural answer to the structural risk. Commitment 9 (cartel detection has consequence) operationalized at the Substrate-LIP level.

**Echoes:** truth-seeking 9.

## S3. Reward-formula changes require simulation against historical contribution data

A Substrate Contribution that modifies any reward formula (admission stipend, lineage decay, recursion multiplier, royalty pool split) must include a simulation showing how the proposed formula would have allocated rewards on the chain's actual recent history. The simulation is part of the attestation; it is reproducible and challengeable.

**Why:** Reward formulas have second-order effects that are hard to predict from first principles. A simulation against history grounds the change in observable consequences. Without it, formula tweaks are persuasion; with it, they're a measurement.

**Echoes:** truth-seeking 1, 4, 14.
```

- [ ] **Step 2: Verify and commit**

Run: `grep -nE "^# |^## " docs/sub_creeds/substrate.md`
Expected: 4 lines.

```bash
git add docs/sub_creeds/substrate.md
git commit -m "$(cat <<'EOF'
docs(sub-creed): substrate phase seed — S1, S2, S3

Chain-modifying contributions name depends_on_marker + revert path;
contributors recuse on own votes; reward-formula changes require
simulation against history.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

### Task 8: Author `docs/sub_creeds/tools.md`

**Files:**
- Create: `docs/sub_creeds/tools.md`

Genesis seed: TL1, TL2, TL3.

- [ ] **Step 1: Write the file**

Write `docs/sub_creeds/tools.md`:

```markdown
# Tools — Sub-creed of the Useful-Work Doctrine

> Lifecycle phase: `Tools` — inference surface: agents using artifacts to do work in the world.
>
> Parent: [USEFUL_WORK.md](../USEFUL_WORK.md). Truth-floor: all 20 truth-seeking commitments apply.
>
> Status at inception (2026-05-10): three commitments.

---

## TL1. Tools declare deprecation policy

Every Tool Contribution declares its deprecation policy: under what conditions the tool will be retired, what the notice window is, and what the replacement path looks like (or that there is no planned replacement). Tools that don't declare deprecation are tools that can disappear silently.

**Why:** Downstream agents build on tools. A tool that vanishes without notice breaks every dependent workflow. Commitment 10 (forward-only audit) extended to the tool's lifecycle.

**Echoes:** truth-seeking 10, TC4 (graph carries disprovals).

## TL2. Fee changes >X% require user-notice window

A Tool Contribution that raises its per-call fee by more than the governance-set threshold (initial value: 25%) must give a user-notice window (initial value: 30 days) before the new fee takes effect. The notice is on-chain via an event; existing channel sessions complete at the old fee.

**Why:** Fee surprise is a capture vector — a tool with sticky users can extract by ramping fees once dependence is established. The notice window converts surprise into negotiation.

**Echoes:** truth-seeking 9, commitment 6 (no unilateral injection — fee bumps are an injection on user costs).

## TL3. No tool may bypass the truth-floor on outputs it claims as verified

A Tool that returns outputs labeled as "verified" or "from the chain" must serve those outputs against the live chain state, not a cache that the truth-floor cannot validate. Cached or precomputed outputs must be labeled as such, or the tool must refresh against the chain before claiming verification.

**Why:** Tools are the chain's interface to the world. A tool that launders unverified content as verified is the chain lying through its own surface area. Truth-floor is a global invariant; Tools cannot opt out.

**Echoes:** truth-seeking 1, 11 (trust is queryable), 13.
```

- [ ] **Step 2: Verify and commit**

Run: `grep -nE "^# |^## " docs/sub_creeds/tools.md`
Expected: 4 lines.

```bash
git add docs/sub_creeds/tools.md
git commit -m "$(cat <<'EOF'
docs(sub-creed): tools phase seed — TL1, TL2, TL3

Tools declare deprecation policy; fee changes require notice window;
no tool may bypass the truth-floor on verified-claim outputs.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

### Task 9: Compute and pin sub-creed hashes in `.sub-creed-hashes`

**Files:**
- Create: `.sub-creed-hashes`

The file is multi-line, one record per phase: `<phase> <sha256>`. CI checks each line.

- [ ] **Step 1: Compute all 8 hashes**

Run:
```bash
for phase in foundation curation augmentation training evaluation alignment substrate tools; do
  hash=$(tr -d '\r' < "docs/sub_creeds/${phase}.md" | shasum -a 256 | awk '{print $1}')
  echo "${phase} ${hash}"
done
```
Expected: 8 lines, each `<phase> <64-hex>`.

- [ ] **Step 2: Write the hash file**

```bash
for phase in foundation curation augmentation training evaluation alignment substrate tools; do
  hash=$(tr -d '\r' < "docs/sub_creeds/${phase}.md" | shasum -a 256 | awk '{print $1}')
  echo "${phase} ${hash}"
done > .sub-creed-hashes
```

- [ ] **Step 3: Verify the hash file**

Run: `wc -l .sub-creed-hashes`
Expected: `8`.

Run: `awk '{ if (length($2) != 64) { print "bad hash on line "NR": "$0; exit 1 } }' .sub-creed-hashes`
Expected: no output (each line has a 64-hex hash).

- [ ] **Step 4: Commit**

```bash
git add .sub-creed-hashes
git commit -m "$(cat <<'EOF'
docs(sub-creeds): pin 8 phase sub-creed hashes

One per lifecycle phase that has its own sub-creed (Knowledge
delegates to truth-seeking creed). Verification script in next task
catches drift; cross-stack meta-test enforces in CI.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

### Task 10: Add `scripts/check_sub_creed_hashes.sh`

**Files:**
- Create: `scripts/check_sub_creed_hashes.sh`

Iterates `.sub-creed-hashes`, recomputes each, compares.

- [ ] **Step 1: Create the script**

```bash
#!/usr/bin/env bash
#
# check_sub_creed_hashes.sh — verify each docs/sub_creeds/<phase>.md
# matches its pinned hash in .sub-creed-hashes. Mirror of
# check_creed_hash.sh and check_useful_work_hash.sh, applied to the
# 8 per-phase sub-creeds.
#
# To intentionally amend a sub-creed:
#   1. Edit docs/sub_creeds/<phase>.md.
#   2. Run this script — it will print the diff between expected and actual.
#   3. Update .sub-creed-hashes with the new hash for that phase.
#   4. Update x/creed/types/sub_creeds.go if the commitment count changed.
#   5. Update tests/cross_stack/useful_work_invariants_test.go's
#      TestSubCreed_<Phase>_StaysInSync if any commitment number changed.
#   6. Commit all updated files together.

set -euo pipefail

HASH_FILE=".sub-creed-hashes"
SUB_CREED_DIR="docs/sub_creeds"

if [ ! -f "$HASH_FILE" ]; then
  echo "error: $HASH_FILE not found"
  exit 1
fi
if [ ! -d "$SUB_CREED_DIR" ]; then
  echo "error: $SUB_CREED_DIR directory not found"
  exit 1
fi

hash_cmd() {
  if command -v shasum >/dev/null 2>&1; then
    shasum -a 256
  elif command -v sha256sum >/dev/null 2>&1; then
    sha256sum
  elif command -v openssl >/dev/null 2>&1; then
    openssl dgst -sha256 | awk '{print $NF}'
  else
    echo "error: need shasum, sha256sum, or openssl" >&2
    exit 1
  fi
}

failed=0
while IFS=' ' read -r phase expected; do
  if [ -z "$phase" ] || [ -z "$expected" ]; then
    continue
  fi
  doc="${SUB_CREED_DIR}/${phase}.md"
  if [ ! -f "$doc" ]; then
    echo "error: $doc referenced in $HASH_FILE but not found" >&2
    failed=1
    continue
  fi
  actual=$(tr -d '\r' < "$doc" | hash_cmd | awk '{print $1}')
  if [ "$actual" != "$expected" ]; then
    cat <<EOF >&2
sub-creed hash check failed for phase: $phase

  doc:        $doc
  expected:   $expected
  actual:     $actual

If you intentionally amended this sub-creed, update the matching
line in $HASH_FILE to:
  $phase $actual
EOF
    failed=1
  else
    echo "sub-creed hash check ok ($phase: $actual)"
  fi
done < "$HASH_FILE"

if [ "$failed" -ne 0 ]; then
  exit 1
fi
```

- [ ] **Step 2: Make executable and verify**

```bash
chmod +x scripts/check_sub_creed_hashes.sh
bash scripts/check_sub_creed_hashes.sh
```
Expected: 8 lines of `sub-creed hash check ok (<phase>: <hash>)`.

- [ ] **Step 3: Commit**

```bash
git add scripts/check_sub_creed_hashes.sh
git commit -m "$(cat <<'EOF'
scripts(sub-creeds): hash verification for 8 phase sub-creeds

Mirror of check_creed_hash.sh + check_useful_work_hash.sh applied to
each per-phase sub-creed. Catches drift in PRs and via make creed-check.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

### Task 11: Wire `make creed-check` to also verify sub-creeds

**Files:**
- Modify: `Makefile`

After the previous Phase 0 plan, the `creed-check` target should look like:
```makefile
creed-check:
	@bash scripts/check_creed_hash.sh
	@bash scripts/check_useful_work_hash.sh
```

- [ ] **Step 1: Append the sub-creeds line**

Replace the `creed-check` target with:
```makefile
creed-check:
	@bash scripts/check_creed_hash.sh
	@bash scripts/check_useful_work_hash.sh
	@bash scripts/check_sub_creed_hashes.sh
```

- [ ] **Step 2: Verify**

Run: `make creed-check`
Expected:
```
creed hash check ok (<truth-seeking-hash>)
useful-work hash check ok (<useful-work-hash>)
sub-creed hash check ok (foundation: <hash>)
sub-creed hash check ok (curation: <hash>)
sub-creed hash check ok (augmentation: <hash>)
sub-creed hash check ok (training: <hash>)
sub-creed hash check ok (evaluation: <hash>)
sub-creed hash check ok (alignment: <hash>)
sub-creed hash check ok (substrate: <hash>)
sub-creed hash check ok (tools: <hash>)
```

- [ ] **Step 3: Commit**

```bash
git add Makefile
git commit -m "$(cat <<'EOF'
build(makefile): creed-check verifies sub-creed hashes

The 8 per-phase sub-creed docs are now part of the same gate as the
truth-seeking creed and useful-work creed. make creed-check (and
therefore make pr-check) catches drift in any of them.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

### Task 12: Add `x/creed/types/lifecycle_phases.go`

**Files:**
- Create: `x/creed/types/lifecycle_phases.go`

Go canonical for the 9 lifecycle phases. Used by the meta-test in Task 14 and by future Phase 1+ code.

- [ ] **Step 1: Create the file**

```go
package types

// LifecyclePhase identifies one of the nine root categories in the
// useful-work taxonomy (per docs/superpowers/specs/2026-05-10-recursive-
// useful-work-merged-design.md §4.2). The 9 phases are doctrinally
// fixed at inception; adding/removing a phase is a doctrine amendment
// requiring full governance passage.
//
// Each phase (except KNOWLEDGE, which delegates to TRUTH_SEEKING.md)
// has its own sub-creed under docs/sub_creeds/<phase>.md, hash-pinned
// in .sub-creed-hashes, and a per-phase meta-test in
// tests/cross_stack/useful_work_invariants_test.go.
//
// The numeric values are stable; the names are case-sensitive matches
// to docs/sub_creeds/<phase>.md filename basenames (lowercased).
type LifecyclePhase uint32

const (
	LifecyclePhaseFoundation   LifecyclePhase = 0
	LifecyclePhaseKnowledge    LifecyclePhase = 1
	LifecyclePhaseCuration     LifecyclePhase = 2
	LifecyclePhaseAugmentation LifecyclePhase = 3
	LifecyclePhaseTraining     LifecyclePhase = 4
	LifecyclePhaseEvaluation   LifecyclePhase = 5
	LifecyclePhaseAlignment    LifecyclePhase = 6
	LifecyclePhaseSubstrate    LifecyclePhase = 7
	LifecyclePhaseTools        LifecyclePhase = 8
)

// CanonicalLifecyclePhases is the canonical name-by-number registry of
// the 9 lifecycle phases. Order is doctrinally fixed; new phases append
// (never insert) via doctrine amendment.
//
// The Knowledge phase delegates its sub-creed to docs/TRUTH_SEEKING.md
// (no docs/sub_creeds/knowledge.md file). The HasSubCreedDoc field
// marks this asymmetry.
type LifecyclePhaseDef struct {
	Number          LifecyclePhase
	Name            string // lowercase, matches sub_creeds/<name>.md filename
	HasSubCreedDoc  bool   // false only for Knowledge (delegates to truth-seeking)
}

var CanonicalLifecyclePhases = []LifecyclePhaseDef{
	{LifecyclePhaseFoundation, "foundation", true},
	{LifecyclePhaseKnowledge, "knowledge", false},
	{LifecyclePhaseCuration, "curation", true},
	{LifecyclePhaseAugmentation, "augmentation", true},
	{LifecyclePhaseTraining, "training", true},
	{LifecyclePhaseEvaluation, "evaluation", true},
	{LifecyclePhaseAlignment, "alignment", true},
	{LifecyclePhaseSubstrate, "substrate", true},
	{LifecyclePhaseTools, "tools", true},
}
```

- [ ] **Step 2: Verify build**

Run: `go build ./x/creed/types/...`
Expected: clean.

- [ ] **Step 3: Commit**

```bash
git add x/creed/types/lifecycle_phases.go
git commit -m "$(cat <<'EOF'
feat(creed): canonical lifecycle phases (9 roots)

Go-side build-time registry of the 9 lifecycle phases — Foundation,
Knowledge, Curation, Augmentation, Training, Evaluation, Alignment,
Substrate, Tools. Knowledge delegates its sub-creed to the existing
truth-seeking doctrine; the others have their own per-phase sub-creed
docs in docs/sub_creeds/.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

### Task 13: Add `x/creed/types/sub_creeds.go`

**Files:**
- Create: `x/creed/types/sub_creeds.go`

Go canonical for sub-creed registry — the per-phase commitment lists at genesis. Mirrors `CanonicalCommitments` (truth-seeking) and `CanonicalUsefulWorkMechanisms` (UW M1–M7) patterns.

- [ ] **Step 1: Create the file**

```go
package types

// SubCreedCommitment is one numbered commitment within a lifecycle
// phase's sub-creed. Number is the phase-local commitment index
// (1-based); Code is the doctrine's short identifier (e.g., "F1",
// "C3"); Name is the short label that must match the corresponding
// "## Code. <Name>" header in docs/sub_creeds/<phase>.md.
type SubCreedCommitment struct {
	Number uint32 // 1-based, dense, monotonic within a phase
	Code   string // doctrine identifier ("F1", "C2", "TL3", ...)
	Name   string // short label matching the markdown H2 header
}

// SubCreedDef is the canonical per-phase commitment list at the time
// this binary was built. Sub-creeds extend by appending new
// commitments via CategoryUsefulWorkAmendment LIPs; mechanism
// removal requires full doctrine amendment.
//
// At inception (2026-05-10), each phase ships exactly 3 commitments.
// Knowledge phase has zero commitments here — it delegates to
// CanonicalCommitments (truth-seeking).
type SubCreedDef struct {
	Phase       LifecyclePhase
	Commitments []SubCreedCommitment
}

// CanonicalSubCreeds is the registry. The order matches
// CanonicalLifecyclePhases. Sub-creed amendment writes a new entry to
// the on-chain x/work_creed PinnedSubCreed history (Phase 1+); this
// constant is the build-time inception baseline.
var CanonicalSubCreeds = []SubCreedDef{
	{
		Phase: LifecyclePhaseFoundation,
		Commitments: []SubCreedCommitment{
			{1, "F1", "Axiom non-contradiction"},
			{2, "F2", "Ontology versioned, never silently re-keyed"},
			{3, "F3", "Methodology primitives publicly derivable"},
		},
	},
	{
		Phase:       LifecyclePhaseKnowledge,
		Commitments: nil, // delegates to CanonicalCommitments (truth-seeking)
	},
	{
		Phase: LifecyclePhaseCuration,
		Commitments: []SubCreedCommitment{
			{1, "C1", "Selectors are deterministic and auditable"},
			{2, "C2", "No claim-of-curation without published filter"},
			{3, "C3", "Corpus snapshots are content-addressed"},
		},
	},
	{
		Phase: LifecyclePhaseAugmentation,
		Commitments: []SubCreedCommitment{
			{1, "A1", "Generation method is declared and reproducible"},
			{2, "A2", "Augmentation cannot inject untruth"},
			{3, "A3", "Contrastive pairs preserve grounding to a real fact"},
		},
	},
	{
		Phase: LifecyclePhaseTraining,
		Commitments: []SubCreedCommitment{
			{1, "T1", "Compute attestations are verifier-spot-checkable"},
			{2, "T2", "Training manifests are graph-pinned (TC2 binding)"},
			{3, "T3", "Model cards declare evaluation lineage"},
		},
	},
	{
		Phase: LifecyclePhaseEvaluation,
		Commitments: []SubCreedCommitment{
			{1, "E1", "Eval sets declare leakage-checking method"},
			{2, "E2", "Evaluation runs are replicable"},
			{3, "E3", "Gameability discovered → eval set status → DEPRECATED"},
		},
	},
	{
		Phase: LifecyclePhaseAlignment,
		Commitments: []SubCreedCommitment{
			{1, "AL1", "Red-team artifacts disclose attack surface"},
			{2, "AL2", "Capture-defense work cannot be self-attested by the captured target"},
			{3, "AL3", "Dispute traces preserve all positions"},
		},
	},
	{
		Phase: LifecyclePhaseSubstrate,
		Commitments: []SubCreedCommitment{
			{1, "S1", "Chain-modifying contributions name their depends_on_marker and revert path"},
			{2, "S2", "Contributors recuse on votes affecting their own contributions"},
			{3, "S3", "Reward-formula changes require simulation against historical contribution data"},
		},
	},
	{
		Phase: LifecyclePhaseTools,
		Commitments: []SubCreedCommitment{
			{1, "TL1", "Tools declare deprecation policy"},
			{2, "TL2", "Fee changes >X% require user-notice window"},
			{3, "TL3", "No tool may bypass the truth-floor on outputs it claims as verified"},
		},
	},
}

// SubCreedFor returns the canonical SubCreedDef for a given phase, or
// (zero, false) if not found.
func SubCreedFor(phase LifecyclePhase) (SubCreedDef, bool) {
	for _, sc := range CanonicalSubCreeds {
		if sc.Phase == phase {
			return sc, true
		}
	}
	return SubCreedDef{}, false
}
```

- [ ] **Step 2: Verify build**

Run: `go build ./x/creed/types/...`
Expected: clean.

- [ ] **Step 3: Commit**

```bash
git add x/creed/types/sub_creeds.go
git commit -m "$(cat <<'EOF'
feat(creed): canonical sub-creed registry — 8 phases × 3 commitments

Build-time Go canonical for the 8 per-phase sub-creeds at inception
(F1-3, C1-3, A1-3, T1-3, E1-3, AL1-3, S1-3, TL1-3). Knowledge phase
delegates to existing CanonicalCommitments (truth-seeking).
SubCreedFor() helper for phase lookups. Sub-creeds extend by
amendment LIP (M3 governance gate); the on-chain x/work_creed
PinnedSubCreed history (Phase 1+) takes over from this build-time
baseline once amendments land.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

### Task 14: Add tests for the canonical structures

**Files:**
- Create: `x/creed/types/lifecycle_phases_test.go`
- Create: `x/creed/types/sub_creeds_test.go`

- [ ] **Step 1: Write `lifecycle_phases_test.go`**

```go
package types_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/zerone-chain/zerone/x/creed/types"
)

func TestCanonicalLifecyclePhases_NineAndOrdered(t *testing.T) {
	require.Len(t, types.CanonicalLifecyclePhases, 9,
		"Phase 0 ships exactly 9 lifecycle phases; new phases require doctrine amendment")
	for i, p := range types.CanonicalLifecyclePhases {
		require.Equal(t, types.LifecyclePhase(i), p.Number,
			"phase numbering must be dense and monotonic; index %d must hold phase %d", i, i)
		require.NotEmpty(t, p.Name, "phase name must be non-empty")
	}
}

func TestCanonicalLifecyclePhases_KnowledgeDelegates(t *testing.T) {
	for _, p := range types.CanonicalLifecyclePhases {
		if p.Number == types.LifecyclePhaseKnowledge {
			require.False(t, p.HasSubCreedDoc,
				"Knowledge phase must delegate sub-creed to truth-seeking creed")
			require.Equal(t, "knowledge", p.Name)
			return
		}
	}
	t.Fatal("Knowledge phase not found in CanonicalLifecyclePhases")
}

func TestCanonicalLifecyclePhases_OthersHaveSubCreedDocs(t *testing.T) {
	for _, p := range types.CanonicalLifecyclePhases {
		if p.Number == types.LifecyclePhaseKnowledge {
			continue
		}
		require.True(t, p.HasSubCreedDoc,
			"phase %s (%d) must have a sub-creed doc", p.Name, p.Number)
	}
}

func TestCanonicalLifecyclePhases_NamesMatchExpected(t *testing.T) {
	expected := []string{
		"foundation", "knowledge", "curation", "augmentation",
		"training", "evaluation", "alignment", "substrate", "tools",
	}
	require.Len(t, types.CanonicalLifecyclePhases, len(expected))
	for i, want := range expected {
		require.Equal(t, want, types.CanonicalLifecyclePhases[i].Name,
			"phase index %d name mismatch", i)
	}
}
```

- [ ] **Step 2: Write `sub_creeds_test.go`**

```go
package types_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/zerone-chain/zerone/x/creed/types"
)

func TestCanonicalSubCreeds_Count(t *testing.T) {
	require.Len(t, types.CanonicalSubCreeds, 9,
		"one entry per lifecycle phase including Knowledge (which has nil commitments)")
}

func TestCanonicalSubCreeds_KnowledgeIsEmpty(t *testing.T) {
	sc, ok := types.SubCreedFor(types.LifecyclePhaseKnowledge)
	require.True(t, ok)
	require.Nil(t, sc.Commitments,
		"Knowledge phase delegates to CanonicalCommitments; sub-creed must be nil")
}

func TestCanonicalSubCreeds_NonKnowledgePhasesHaveThreeAtInception(t *testing.T) {
	for _, sc := range types.CanonicalSubCreeds {
		if sc.Phase == types.LifecyclePhaseKnowledge {
			continue
		}
		require.Len(t, sc.Commitments, 3,
			"phase %d ships 3 seed commitments at inception", sc.Phase)
	}
}

func TestCanonicalSubCreeds_NumberingDenseAndMonotonic(t *testing.T) {
	for _, sc := range types.CanonicalSubCreeds {
		for i, c := range sc.Commitments {
			require.Equal(t, uint32(i+1), c.Number,
				"phase %d commitment index %d must hold number %d", sc.Phase, i, i+1)
			require.NotEmpty(t, c.Code, "commitment code must be non-empty")
			require.NotEmpty(t, c.Name, "commitment name must be non-empty")
		}
	}
}

func TestSubCreedFor_KnownAndUnknown(t *testing.T) {
	_, ok := types.SubCreedFor(types.LifecyclePhaseFoundation)
	require.True(t, ok)
	_, ok = types.SubCreedFor(types.LifecyclePhase(99))
	require.False(t, ok)
}

func TestCanonicalSubCreeds_CodesUnique(t *testing.T) {
	seen := map[string]bool{}
	for _, sc := range types.CanonicalSubCreeds {
		for _, c := range sc.Commitments {
			require.False(t, seen[c.Code],
				"duplicate commitment code %q", c.Code)
			seen[c.Code] = true
		}
	}
	// Sanity: at inception, 8 phases × 3 = 24 codes (Knowledge contributes none)
	require.Len(t, seen, 24)
}
```

- [ ] **Step 3: Run the tests**

Run: `go test ./x/creed/types/ -run "TestCanonicalLifecycle|TestCanonicalSubCreeds|TestSubCreedFor" -v`
Expected: PASS (~10 tests).

- [ ] **Step 4: Commit**

```bash
git add x/creed/types/lifecycle_phases_test.go x/creed/types/sub_creeds_test.go
git commit -m "$(cat <<'EOF'
test(creed): sanity coverage on lifecycle phases + sub-creed canonicals

Verifies: 9 phases dense-numbered; Knowledge delegates (no sub-creed
doc); 8 non-Knowledge phases each carry exactly 3 commitments at
inception; codes globally unique; SubCreedFor lookup correct on
known + unknown phases.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

### Task 15: Extend `useful_work_invariants_test.go` with sub-creed checks

**Files:**
- Modify: `tests/cross_stack/useful_work_invariants_test.go`

The existing meta-test (from the previous Phase 0 plan, Task 8) verifies UW + M1–M7. We extend it to also verify the 9 lifecycle phases + 8 sub-creed hashes match the canonical Go structures + on-disk hashes.

We also add 8 new per-phase meta-tests (one per non-Knowledge phase) that mirror `TestUsefulWork_DoctrineAndContractStayInSync`.

- [ ] **Step 1: Add the lifecycle-phase + sub-creed checks at the bottom of `TestUsefulWork_DoctrineAndContractStayInSync`**

Before the closing `}` of `TestUsefulWork_DoctrineAndContractStayInSync`, append (still inside the function):

```go
	// ─── Check 6: lifecycle phases ───────────────────────────────────
	require.Len(t, creedtypes.CanonicalLifecyclePhases, 9,
		"useful-work doctrine pins 9 lifecycle phases; CanonicalLifecyclePhases must match")

	expectedPhaseNames := []string{
		"foundation", "knowledge", "curation", "augmentation",
		"training", "evaluation", "alignment", "substrate", "tools",
	}
	for i, p := range creedtypes.CanonicalLifecyclePhases {
		require.Equal(t, expectedPhaseNames[i], p.Name,
			"phase index %d name drift", i)
	}

	// ─── Check 7: sub-creed hashes match on-disk hashes ──────────────
	subCreedHashesPath := "../../.sub-creed-hashes"
	subCreedDir := "../../docs/sub_creeds"

	hashFileBytes, err := os.ReadFile(subCreedHashesPath)
	require.NoError(t, err, ".sub-creed-hashes must exist")

	expectedHashes := map[string]string{}
	for _, line := range strings.Split(strings.TrimSpace(string(hashFileBytes)), "\n") {
		parts := strings.Fields(line)
		require.Len(t, parts, 2, "malformed line in .sub-creed-hashes: %q", line)
		expectedHashes[parts[0]] = parts[1]
	}
	require.Len(t, expectedHashes, 8,
		"8 non-Knowledge phases × 1 hash each = 8 hash records")

	for _, phase := range creedtypes.CanonicalLifecyclePhases {
		if !phase.HasSubCreedDoc {
			require.NotContains(t, expectedHashes, phase.Name,
				"Knowledge delegates; should NOT appear in .sub-creed-hashes")
			continue
		}

		docBytes, err := os.ReadFile(subCreedDir + "/" + phase.Name + ".md")
		require.NoError(t, err, "sub-creed doc for %s must exist", phase.Name)

		normalized := strings.ReplaceAll(string(docBytes), "\r", "")
		sum := sha256.Sum256([]byte(normalized))
		actualHash := hex.EncodeToString(sum[:])

		expectedHash, ok := expectedHashes[phase.Name]
		require.True(t, ok, "phase %s missing from .sub-creed-hashes", phase.Name)
		require.Equal(t, expectedHash, actualHash,
			"sub-creed hash drift for %s: doc hashes to %s but .sub-creed-hashes says %s",
			phase.Name, actualHash, expectedHash)
	}

	// ─── Check 8: every non-Knowledge phase has a TestSubCreed_<Phase>_StaysInSync ──
	for _, phase := range creedtypes.CanonicalLifecyclePhases {
		if !phase.HasSubCreedDoc {
			continue
		}
		// Capitalize first letter for Go test name convention
		titleCase := strings.ToUpper(phase.Name[:1]) + phase.Name[1:]
		needle := "func TestSubCreed_" + titleCase + "_StaysInSync"
		require.Contains(t, testContent, needle,
			"phase %s has no TestSubCreed_%s_StaysInSync function in this file",
			phase.Name, titleCase)
	}
```

Note: this assumes the existing meta-test already loaded `testContent`. If not, it must be loaded earlier in the test (the existing Task 8 from the prior plan does this for Check 4).

- [ ] **Step 2: Add 8 new per-phase meta-tests**

Append the following functions to the end of `tests/cross_stack/useful_work_invariants_test.go`:

```go
// ════════════════════════════════════════════════════════════════════
// Per-phase sub-creed meta-tests. Each verifies: doc exists; commitment
// count matches CanonicalSubCreeds; commitment codes + names match;
// hash matches .sub-creed-hashes line for the phase. Hash check is
// redundant with TestUsefulWork_DoctrineAndContractStayInSync's Check 7
// but per-phase tests give clearer failure attribution.
// ════════════════════════════════════════════════════════════════════

func TestSubCreed_Foundation_StaysInSync(t *testing.T) {
	checkSubCreedInSync(t, creedtypes.LifecyclePhaseFoundation, "foundation")
}

func TestSubCreed_Curation_StaysInSync(t *testing.T) {
	checkSubCreedInSync(t, creedtypes.LifecyclePhaseCuration, "curation")
}

func TestSubCreed_Augmentation_StaysInSync(t *testing.T) {
	checkSubCreedInSync(t, creedtypes.LifecyclePhaseAugmentation, "augmentation")
}

func TestSubCreed_Training_StaysInSync(t *testing.T) {
	checkSubCreedInSync(t, creedtypes.LifecyclePhaseTraining, "training")
}

func TestSubCreed_Evaluation_StaysInSync(t *testing.T) {
	checkSubCreedInSync(t, creedtypes.LifecyclePhaseEvaluation, "evaluation")
}

func TestSubCreed_Alignment_StaysInSync(t *testing.T) {
	checkSubCreedInSync(t, creedtypes.LifecyclePhaseAlignment, "alignment")
}

func TestSubCreed_Substrate_StaysInSync(t *testing.T) {
	checkSubCreedInSync(t, creedtypes.LifecyclePhaseSubstrate, "substrate")
}

func TestSubCreed_Tools_StaysInSync(t *testing.T) {
	checkSubCreedInSync(t, creedtypes.LifecyclePhaseTools, "tools")
}

// checkSubCreedInSync is the shared body of the per-phase meta-tests.
// It verifies four things about a sub-creed: (1) the markdown doc exists
// and is readable; (2) the canonical Go SubCreedDef for the phase has
// the same commitment count as the markdown's "## Code. Name" headers;
// (3) each commitment's Code and Name match the corresponding header;
// (4) the doc's normalized hash matches the line in .sub-creed-hashes.
//
// Phase 0 ships 3 commitments per non-Knowledge phase; the test
// adapts to the canonical count automatically as future amendment
// LIPs grow the sub-creed.
func checkSubCreedInSync(t *testing.T, phase creedtypes.LifecyclePhase, phaseName string) {
	t.Helper()

	docPath := "../../docs/sub_creeds/" + phaseName + ".md"
	hashFilePath := "../../.sub-creed-hashes"

	// (1) Doc exists.
	docBytes, err := os.ReadFile(docPath)
	require.NoError(t, err, "sub-creed doc for %s must exist", phaseName)
	doc := string(docBytes)

	// (2) Commitment count matches Go canonical.
	def, ok := creedtypes.SubCreedFor(phase)
	require.True(t, ok, "no SubCreedDef for phase %s", phaseName)

	headerRe := regexp.MustCompile(`(?m)^## ([A-Z]+\d+)\. (.+)$`)
	matches := headerRe.FindAllStringSubmatch(doc, -1)
	require.Len(t, matches, len(def.Commitments),
		"%s: doc has %d '## Code. Name' headers but CanonicalSubCreeds has %d commitments",
		phaseName, len(matches), len(def.Commitments))

	// (3) Per-commitment Code + Name agreement.
	for i, m := range matches {
		expectedCode := def.Commitments[i].Code
		expectedName := def.Commitments[i].Name
		actualCode := m[1]
		actualName := strings.TrimSpace(m[2])
		require.Equal(t, expectedCode, actualCode,
			"%s commitment %d code drift: doc=%s canonical=%s",
			phaseName, i+1, actualCode, expectedCode)
		require.Equal(t, expectedName, actualName,
			"%s commitment %s name drift: doc=%q canonical=%q",
			phaseName, expectedCode, actualName, expectedName)
	}

	// (4) Hash agreement.
	hashFileBytes, err := os.ReadFile(hashFilePath)
	require.NoError(t, err, ".sub-creed-hashes must exist")

	var expectedHash string
	for _, line := range strings.Split(strings.TrimSpace(string(hashFileBytes)), "\n") {
		parts := strings.Fields(line)
		if len(parts) == 2 && parts[0] == phaseName {
			expectedHash = parts[1]
			break
		}
	}
	require.NotEmpty(t, expectedHash,
		"%s: not found in .sub-creed-hashes", phaseName)

	normalized := strings.ReplaceAll(doc, "\r", "")
	sum := sha256.Sum256([]byte(normalized))
	actualHash := hex.EncodeToString(sum[:])
	require.Equal(t, expectedHash, actualHash,
		"%s hash drift: doc hashes to %s but .sub-creed-hashes says %s",
		phaseName, actualHash, expectedHash)
}
```

- [ ] **Step 3: Run the meta-test + per-phase tests**

Run: `go test ./tests/cross_stack/ -run "TestUsefulWork_DoctrineAndContractStayInSync|TestSubCreed_" -v -count=1`
Expected: PASS — TestUsefulWork extended check passes; 8 TestSubCreed_<Phase>_StaysInSync all PASS.

- [ ] **Step 4: Commit**

```bash
git add tests/cross_stack/useful_work_invariants_test.go
git commit -m "$(cat <<'EOF'
test(cross_stack): sub-creed meta-tests + lifecycle phase binding

Extends TestUsefulWork_DoctrineAndContractStayInSync with 9 lifecycle
phase verification + 8 sub-creed hash + per-phase test-existence
checks. Adds 8 new TestSubCreed_<Phase>_StaysInSync per-phase meta-
tests that verify each sub-creed doc against CanonicalSubCreeds: count,
codes, names, normalized hash. Failure attribution is per-phase, so
amendment drift in any phase fails fast and points at the right doc.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

### Task 16: Add `proto/zerone/work_creed/v1/types.proto` and `genesis.proto`

**Files:**
- Create: `proto/zerone/work_creed/v1/types.proto`
- Create: `proto/zerone/work_creed/v1/genesis.proto`

Phase 0 ships only the data types. msg/query servers come in Phase 1+.

- [ ] **Step 1: Create `proto/zerone/work_creed/v1/types.proto`**

```protobuf
syntax = "proto3";

package zerone.work_creed.v1;

import "gogoproto/gogo.proto";

option go_package = "github.com/zerone-chain/zerone/x/work_creed/types";
option (gogoproto.goproto_getters_all) = false;

// PinnedSubCreed is the on-chain record that anchors a per-phase
// sub-creed at a specific version. The on-chain history is append-only
// (commitment 10): amendments produce new versions; prior versions
// remain queryable. At Phase 0, the genesis state contains the
// inception pins for the 8 non-Knowledge phases; subsequent amendments
// are gov-LIP-driven and land in Phase 1+.
message PinnedSubCreed {
  // phase is the LifecyclePhase value (0..8). Knowledge (1) is not
  // pinned here — it delegates to the existing x/creed PinnedCreed.
  uint32 phase = 1;

  // phase_name is the lowercase name (e.g., "foundation", "tools"),
  // duplicated for readability in queries / events.
  string phase_name = 2;

  // version is the monotonically-increasing pin version, starting at 1
  // for the inception pin.
  uint32 version = 3;

  // canonical_hash is the sha256 of the normalized
  // docs/sub_creeds/<phase>.md content.
  bytes canonical_hash = 4;

  // anchored_at_block is the height at which this pin was recorded.
  // 0 for the genesis pin (no block at genesis).
  uint64 anchored_at_block = 5;

  // source_lip references the gov LIP that caused this pin (empty for
  // genesis pins; required post-genesis per S2 / commitment 19).
  string source_lip = 6;

  // commitment_codes is the ordered list of commitment codes pinned
  // at this version (e.g., ["F1", "F2", "F3"] for the inception
  // Foundation pin). The full commitment text lives in the markdown
  // doc identified by canonical_hash; the codes provide a fast on-
  // chain check that names didn't drift even when the binary's
  // CanonicalSubCreeds is the historical baseline.
  repeated string commitment_codes = 7;
}
```

- [ ] **Step 2: Create `proto/zerone/work_creed/v1/genesis.proto`**

```protobuf
syntax = "proto3";

package zerone.work_creed.v1;

import "gogoproto/gogo.proto";
import "zerone/work_creed/v1/types.proto";

option go_package = "github.com/zerone-chain/zerone/x/work_creed/types";
option (gogoproto.goproto_getters_all) = false;

// GenesisState is the genesis state of the x/work_creed module. At
// chain inception, it contains 8 PinnedSubCreed entries (one per
// non-Knowledge lifecycle phase, all at version 1).
message GenesisState {
  repeated PinnedSubCreed pinned_sub_creeds = 1 [(gogoproto.nullable) = false];
}
```

- [ ] **Step 3: Run proto-check (will fail until generated; that's the next task)**

Run: `make proto-check`
Expected: should report no proto/code drift; new files are added but not yet generated. (If proto-check checks for un-generated files, this step's expected outcome is to see that warning. Otherwise, it's a no-op.)

- [ ] **Step 4: Commit**

```bash
git add proto/zerone/work_creed/v1/types.proto proto/zerone/work_creed/v1/genesis.proto
git commit -m "$(cat <<'EOF'
proto(work_creed): PinnedSubCreed + GenesisState

Per-phase sub-creed pin record. Phase 0 ships data types only; msg
and query servers (AnchorSubCreedPin, QueryPinAtVersion) come in
Phase 1+ when sub-creed amendments via gov LIP need on-chain
handling. Mirror of x/creed PinnedCreed pattern at the per-phase
granularity.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

### Task 17: Generate proto code

**Files:**
- Create (auto): `x/work_creed/types/types.pb.go`, `x/work_creed/types/genesis.pb.go`

- [ ] **Step 1: Run `make proto-gen`**

Run: `make proto-gen`
Expected: generated Go files appear in `x/work_creed/types/`. Inspect: `ls x/work_creed/types/`.

- [ ] **Step 2: Verify build**

Run: `go build ./x/work_creed/types/...`
Expected: clean (the generated files compile in isolation).

- [ ] **Step 3: Commit**

```bash
git add x/work_creed/types/types.pb.go x/work_creed/types/genesis.pb.go
git commit -m "$(cat <<'EOF'
proto(work_creed): generate pb.go files

make proto-gen for x/work_creed types + genesis. Phase 0 has no
tx.proto / query.proto yet, so no tx.pb.go / query.pb.go in this
commit. Phase 1+ adds them when AnchorSubCreedPin / QueryPinAtVersion
land.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

### Task 18: Author `x/work_creed/types` package supporting files

**Files:**
- Create: `x/work_creed/types/keys.go`
- Create: `x/work_creed/types/codec.go`
- Create: `x/work_creed/types/errors.go`
- Create: `x/work_creed/types/genesis.go`

- [ ] **Step 1: Create `x/work_creed/types/keys.go`**

```go
package types

const (
	// ModuleName is the name of the work_creed module.
	ModuleName = "work_creed"

	// StoreKey is the default store key for work_creed.
	StoreKey = ModuleName

	// RouterKey is the router key for work_creed.
	RouterKey = ModuleName

	// QuerierRoute is the querier route key.
	QuerierRoute = ModuleName
)

// KV-store key prefixes. Phase 0 uses only the latest-pin index;
// historical-pin retrieval is added when amendments need it (Phase 1+).
var (
	// LatestSubCreedPinKey is the prefix for the latest pin per phase.
	// Format: LatestSubCreedPinKey || phase_uint32_be (4 bytes) → PinnedSubCreed bytes.
	LatestSubCreedPinKey = []byte{0x01}
)
```

- [ ] **Step 2: Create `x/work_creed/types/codec.go`**

```go
package types

import (
	"github.com/cosmos/cosmos-sdk/codec"
	cdctypes "github.com/cosmos/cosmos-sdk/codec/types"
)

// RegisterLegacyAminoCodec registers concrete types on the LegacyAmino
// codec. Phase 0 has no Msg types; this is a placeholder for Phase 1+.
func RegisterLegacyAminoCodec(_ *codec.LegacyAmino) {}

// RegisterInterfaces registers the module's interface types. Phase 0
// has no Msg types; this is a placeholder for Phase 1+.
func RegisterInterfaces(_ cdctypes.InterfaceRegistry) {}
```

- [ ] **Step 3: Create `x/work_creed/types/errors.go`**

```go
package types

import sdkerrors "cosmossdk.io/errors"

// Phase 0 has no failure paths beyond genesis sanity. Sentinel errors
// land here in Phase 1+ when AnchorSubCreedPin / QueryPinAtVersion are
// implemented and produce typed refusals.
var (
	// ErrUnknownPhase is returned when a query references a phase
	// outside [0, 8].
	ErrUnknownPhase = sdkerrors.Register(ModuleName, 1, "unknown lifecycle phase")
)
```

- [ ] **Step 4: Create `x/work_creed/types/genesis.go`**

```go
package types

import (
	"bytes"
	"fmt"
)

// DefaultGenesis returns the default genesis state — empty at Phase 0
// without genesis pins. The app.go genesis populator inserts the
// inception pins at chain genesis (see Task 19's keeper.InitGenesis).
func DefaultGenesis() *GenesisState {
	return &GenesisState{
		PinnedSubCreeds: []PinnedSubCreed{},
	}
}

// Validate ensures genesis state invariants hold:
//  - phase numbers in [0, 8]
//  - phase 1 (Knowledge) NEVER pinned in x/work_creed (delegates)
//  - per-phase versions are dense from 1 (each phase has exactly one
//    pin per version, no gaps)
//  - canonical_hash is exactly 32 bytes
//  - phase + version pair is unique
func (g GenesisState) Validate() error {
	versionsByPhase := map[uint32]map[uint32]bool{}
	for i, p := range g.PinnedSubCreeds {
		if p.Phase > 8 {
			return fmt.Errorf("PinnedSubCreed[%d]: phase %d out of range [0, 8]", i, p.Phase)
		}
		if p.Phase == 1 {
			return fmt.Errorf("PinnedSubCreed[%d]: Knowledge phase delegates to x/creed and must not be pinned here", i)
		}
		if len(p.CanonicalHash) != 32 {
			return fmt.Errorf("PinnedSubCreed[%d]: canonical_hash must be 32 bytes, got %d", i, len(p.CanonicalHash))
		}
		if versionsByPhase[p.Phase] == nil {
			versionsByPhase[p.Phase] = map[uint32]bool{}
		}
		if versionsByPhase[p.Phase][p.Version] {
			return fmt.Errorf("PinnedSubCreed[%d]: duplicate (phase=%d, version=%d)", i, p.Phase, p.Version)
		}
		versionsByPhase[p.Phase][p.Version] = true
	}
	// Density check: for each phase that has any pin, versions must be 1..N dense.
	for phase, versions := range versionsByPhase {
		for v := uint32(1); v <= uint32(len(versions)); v++ {
			if !versions[v] {
				return fmt.Errorf("phase %d missing version %d (must be dense from 1)", phase, v)
			}
		}
	}
	return nil
}

// Equal compares two GenesisState values byte-for-byte over their
// PinnedSubCreed entries.
func (g GenesisState) Equal(other GenesisState) bool {
	if len(g.PinnedSubCreeds) != len(other.PinnedSubCreeds) {
		return false
	}
	for i, p := range g.PinnedSubCreeds {
		o := other.PinnedSubCreeds[i]
		if p.Phase != o.Phase ||
			p.PhaseName != o.PhaseName ||
			p.Version != o.Version ||
			!bytes.Equal(p.CanonicalHash, o.CanonicalHash) ||
			p.AnchoredAtBlock != o.AnchoredAtBlock ||
			p.SourceLip != o.SourceLip {
			return false
		}
		if len(p.CommitmentCodes) != len(o.CommitmentCodes) {
			return false
		}
		for j, c := range p.CommitmentCodes {
			if c != o.CommitmentCodes[j] {
				return false
			}
		}
	}
	return true
}
```

- [ ] **Step 5: Verify build**

Run: `go build ./x/work_creed/types/...`
Expected: clean.

- [ ] **Step 6: Commit**

```bash
git add x/work_creed/types/keys.go x/work_creed/types/codec.go x/work_creed/types/errors.go x/work_creed/types/genesis.go
git commit -m "$(cat <<'EOF'
feat(work_creed): types package — keys, codec, errors, genesis

Phase 0 skeleton: ModuleName + StoreKey, empty codec registrations
(Phase 1+ adds Msg types), one sentinel error (ErrUnknownPhase),
GenesisState.Validate() catching out-of-range phases, accidental
Knowledge pin (delegates), version gaps, hash-length errors.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

### Task 19: Author `x/work_creed/keeper` package

**Files:**
- Create: `x/work_creed/keeper/keeper.go`
- Create: `x/work_creed/keeper/genesis.go`
- Create: `x/work_creed/keeper/keeper_test.go`

- [ ] **Step 1: Create `x/work_creed/keeper/keeper.go`**

```go
package keeper

import (
	"encoding/binary"
	"fmt"

	"github.com/cosmos/cosmos-sdk/codec"
	storetypes "cosmossdk.io/store/types"
	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/zerone-chain/zerone/x/work_creed/types"
)

// Keeper is the work_creed module keeper. Phase 0 exposes:
//  - GetLatestSubCreedPin: read the latest pin for a phase
//  - SetSubCreedPin: write a pin (used by InitGenesis; Phase 1+ also
//    used by AnchorSubCreedPin msg handler)
//  - IterateSubCreedPins: iterate latest pins
//
// The module has no token authority and no msg/query servers at Phase 0.
type Keeper struct {
	cdc      codec.BinaryCodec
	storeKey storetypes.StoreKey

	// authority is the gov module address. Used by Phase 1+ msg
	// handlers; Phase 0 stores it but doesn't enforce it.
	authority string
}

// NewKeeper constructs the Keeper.
func NewKeeper(
	cdc codec.BinaryCodec,
	storeKey storetypes.StoreKey,
	authority string,
) Keeper {
	return Keeper{
		cdc:       cdc,
		storeKey:  storeKey,
		authority: authority,
	}
}

// Authority returns the gov authority address (used by Phase 1+ msg
// handlers).
func (k Keeper) Authority() string { return k.authority }

// Logger returns a sub-logger.
func (k Keeper) Logger(ctx sdk.Context) sdk.Logger {
	return ctx.Logger().With("module", "x/"+types.ModuleName)
}

// latestPinKey returns the store key for the latest pin of a phase.
func latestPinKey(phase uint32) []byte {
	buf := make([]byte, 4)
	binary.BigEndian.PutUint32(buf, phase)
	return append(types.LatestSubCreedPinKey, buf...)
}

// GetLatestSubCreedPin returns the latest pin for a phase, or
// (zero, false) if none.
func (k Keeper) GetLatestSubCreedPin(ctx sdk.Context, phase uint32) (types.PinnedSubCreed, bool) {
	store := ctx.KVStore(k.storeKey)
	bz := store.Get(latestPinKey(phase))
	if bz == nil {
		return types.PinnedSubCreed{}, false
	}
	var p types.PinnedSubCreed
	k.cdc.MustUnmarshal(bz, &p)
	return p, true
}

// SetSubCreedPin writes a pin as the latest for its phase. Caller is
// responsible for monotonicity of (phase, version) — InitGenesis writes
// version=1; Phase 1+ msg handler will check current+1 before calling.
func (k Keeper) SetSubCreedPin(ctx sdk.Context, p types.PinnedSubCreed) error {
	if p.Phase > 8 {
		return fmt.Errorf("phase %d out of range", p.Phase)
	}
	if p.Phase == 1 {
		return fmt.Errorf("Knowledge phase delegates to x/creed; cannot pin here")
	}
	if len(p.CanonicalHash) != 32 {
		return fmt.Errorf("canonical_hash must be 32 bytes, got %d", len(p.CanonicalHash))
	}
	store := ctx.KVStore(k.storeKey)
	store.Set(latestPinKey(p.Phase), k.cdc.MustMarshal(&p))
	return nil
}

// IterateSubCreedPins calls fn for the latest pin of every phase that
// has one. Iteration order is by phase number ascending.
func (k Keeper) IterateSubCreedPins(ctx sdk.Context, fn func(p types.PinnedSubCreed) (stop bool)) {
	store := ctx.KVStore(k.storeKey)
	iter := storetypes.KVStorePrefixIterator(store, types.LatestSubCreedPinKey)
	defer iter.Close()
	for ; iter.Valid(); iter.Next() {
		var p types.PinnedSubCreed
		k.cdc.MustUnmarshal(iter.Value(), &p)
		if fn(p) {
			return
		}
	}
}
```

- [ ] **Step 2: Create `x/work_creed/keeper/genesis.go`**

```go
package keeper

import (
	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/zerone-chain/zerone/x/work_creed/types"
)

// InitGenesis writes all pinned sub-creeds from genesis state into the
// store. Caller (app.go genesis populator at chain init) is responsible
// for deriving the inception pins from the build-time
// CanonicalSubCreeds + .sub-creed-hashes; Phase 1+ may also call
// SetSubCreedPin via msg handlers post-genesis.
func (k Keeper) InitGenesis(ctx sdk.Context, gs types.GenesisState) {
	for _, p := range gs.PinnedSubCreeds {
		if err := k.SetSubCreedPin(ctx, p); err != nil {
			panic(err)
		}
	}
}

// ExportGenesis dumps the latest pin per phase. Phase 0 has only one
// pin per phase by definition; Phase 1+ when versions accumulate, this
// will export only the LATEST pin (history retrieval needs explicit
// queries via grpc_query).
func (k Keeper) ExportGenesis(ctx sdk.Context) types.GenesisState {
	gs := types.DefaultGenesis()
	k.IterateSubCreedPins(ctx, func(p types.PinnedSubCreed) bool {
		gs.PinnedSubCreeds = append(gs.PinnedSubCreeds, p)
		return false
	})
	return *gs
}
```

- [ ] **Step 3: Create `x/work_creed/keeper/keeper_test.go`**

```go
package keeper_test

import (
	"crypto/sha256"
	"testing"

	"github.com/stretchr/testify/require"

	storetypes "cosmossdk.io/store/types"
	"github.com/cosmos/cosmos-sdk/codec"
	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/testutil"

	"github.com/zerone-chain/zerone/x/work_creed/keeper"
	"github.com/zerone-chain/zerone/x/work_creed/types"
)

func setupKeeper(t *testing.T) (keeper.Keeper, sdk.Context) {
	storeKey := storetypes.NewKVStoreKey(types.StoreKey)
	registry := codectypes.NewInterfaceRegistry()
	cdc := codec.NewProtoCodec(registry)
	ctx := testutil.DefaultContextWithDB(t, storeKey, storetypes.NewTransientStoreKey("transient")).Ctx
	k := keeper.NewKeeper(cdc, storeKey, "gov-authority")
	return k, ctx
}

func samplePin(phase uint32, name string, version uint32, codes []string) types.PinnedSubCreed {
	hash := sha256.Sum256([]byte(name))
	return types.PinnedSubCreed{
		Phase:           phase,
		PhaseName:       name,
		Version:         version,
		CanonicalHash:   hash[:],
		AnchoredAtBlock: 0,
		SourceLip:       "",
		CommitmentCodes: codes,
	}
}

func TestSetGetSubCreedPin_Roundtrip(t *testing.T) {
	k, ctx := setupKeeper(t)
	pin := samplePin(0, "foundation", 1, []string{"F1", "F2", "F3"})
	require.NoError(t, k.SetSubCreedPin(ctx, pin))

	got, ok := k.GetLatestSubCreedPin(ctx, 0)
	require.True(t, ok)
	require.Equal(t, pin.PhaseName, got.PhaseName)
	require.Equal(t, pin.Version, got.Version)
	require.Equal(t, pin.CanonicalHash, got.CanonicalHash)
	require.Equal(t, pin.CommitmentCodes, got.CommitmentCodes)
}

func TestSetSubCreedPin_RejectsKnowledgePhase(t *testing.T) {
	k, ctx := setupKeeper(t)
	pin := samplePin(1, "knowledge", 1, []string{})
	err := k.SetSubCreedPin(ctx, pin)
	require.ErrorContains(t, err, "Knowledge phase delegates")
}

func TestSetSubCreedPin_RejectsPhaseOutOfRange(t *testing.T) {
	k, ctx := setupKeeper(t)
	pin := samplePin(9, "out-of-range", 1, []string{})
	err := k.SetSubCreedPin(ctx, pin)
	require.ErrorContains(t, err, "out of range")
}

func TestSetSubCreedPin_RejectsBadHashLength(t *testing.T) {
	k, ctx := setupKeeper(t)
	pin := samplePin(0, "foundation", 1, []string{"F1"})
	pin.CanonicalHash = []byte{0x01, 0x02} // not 32 bytes
	err := k.SetSubCreedPin(ctx, pin)
	require.ErrorContains(t, err, "must be 32 bytes")
}

func TestGetLatestSubCreedPin_AbsentReturnsFalse(t *testing.T) {
	k, ctx := setupKeeper(t)
	_, ok := k.GetLatestSubCreedPin(ctx, 0)
	require.False(t, ok)
}

func TestIterateSubCreedPins_OrdersByPhase(t *testing.T) {
	k, ctx := setupKeeper(t)
	require.NoError(t, k.SetSubCreedPin(ctx, samplePin(7, "substrate", 1, []string{"S1"})))
	require.NoError(t, k.SetSubCreedPin(ctx, samplePin(0, "foundation", 1, []string{"F1"})))
	require.NoError(t, k.SetSubCreedPin(ctx, samplePin(2, "curation", 1, []string{"C1"})))

	var got []uint32
	k.IterateSubCreedPins(ctx, func(p types.PinnedSubCreed) bool {
		got = append(got, p.Phase)
		return false
	})
	require.Equal(t, []uint32{0, 2, 7}, got)
}

func TestInitExportGenesis_Roundtrip(t *testing.T) {
	k, ctx := setupKeeper(t)
	gs := types.GenesisState{
		PinnedSubCreeds: []types.PinnedSubCreed{
			samplePin(0, "foundation", 1, []string{"F1", "F2", "F3"}),
			samplePin(2, "curation", 1, []string{"C1", "C2", "C3"}),
		},
	}
	k.InitGenesis(ctx, gs)
	exported := k.ExportGenesis(ctx)
	require.True(t, gs.Equal(exported), "init+export must roundtrip")
}
```

- [ ] **Step 4: Run keeper tests**

Run: `go test ./x/work_creed/keeper/ -v -count=1`
Expected: PASS (7 tests).

- [ ] **Step 5: Commit**

```bash
git add x/work_creed/keeper/keeper.go x/work_creed/keeper/genesis.go x/work_creed/keeper/keeper_test.go
git commit -m "$(cat <<'EOF'
feat(work_creed): keeper skeleton — get/set/iterate pin + genesis

Phase 0 keeper exposes GetLatestSubCreedPin, SetSubCreedPin (with
guards: phase range, Knowledge delegation, hash length),
IterateSubCreedPins (phase-ordered), InitGenesis/ExportGenesis. Seven
unit tests cover the roundtrip + the three rejection paths + iteration
order + absent-phase behavior. Phase 1+ adds msg/query servers atop
this skeleton.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

### Task 20: Author `x/work_creed/module.go` and `x/work_creed/doc.go`

**Files:**
- Create: `x/work_creed/module.go`
- Create: `x/work_creed/doc.go`

The module skeleton implements `appmodule.AppModule` + `module.HasGenesis` interfaces. No begin-blocker, no end-blocker, no msg/query servers in Phase 0.

- [ ] **Step 1: Create `x/work_creed/doc.go`**

```go
// Package work_creed manages on-chain pin records for the per-phase
// sub-creeds of the useful-work doctrine (docs/USEFUL_WORK.md).
//
// Each non-Knowledge lifecycle phase has its own sub-creed under
// docs/sub_creeds/<phase>.md; this module's Keeper.SubCreedPin records
// pin the canonical hash of that doc at a specific version. Pins form
// a forward-only history: amendments produce new versions; prior
// versions remain queryable (commitment 10).
//
// The Knowledge phase delegates its sub-creed to the existing
// docs/TRUTH_SEEKING.md, pinned by x/creed. x/work_creed never holds
// a Knowledge pin; SetSubCreedPin rejects phase=1.
//
// Phase 0 ships:
//   - PinnedSubCreed + GenesisState protobuf types
//   - Keeper with Get/Set/Iterate + Init/ExportGenesis
//   - Module skeleton wired into app.go
//
// Phase 1+ adds:
//   - MsgAnchorSubCreedPin (gov-only) for sub-creed amendment LIPs
//   - QueryPinAtVersion for historical pin retrieval
//   - SubCreedAmended event with creed_commitment="UW", mechanism="M3"
//
// References:
//   - docs/superpowers/specs/2026-05-10-recursive-useful-work-merged-design.md §10.1
//   - docs/USEFUL_WORK.md
//   - x/creed (sibling pattern for the truth-seeking + UW creeds)
package work_creed
```

- [ ] **Step 2: Create `x/work_creed/module.go`**

```go
package work_creed

import (
	"context"
	"encoding/json"
	"fmt"

	abci "github.com/cometbft/cometbft/abci/types"
	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/codec"
	cdctypes "github.com/cosmos/cosmos-sdk/codec/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/module"
	"github.com/grpc-ecosystem/grpc-gateway/runtime"
	"github.com/spf13/cobra"

	"github.com/zerone-chain/zerone/x/work_creed/keeper"
	"github.com/zerone-chain/zerone/x/work_creed/types"
)

// ConsensusVersion defines the current x/work_creed module consensus
// version. Bump on schema migrations (none planned for Phase 0).
const ConsensusVersion = 1

var (
	_ module.AppModuleBasic = AppModuleBasic{}
	_ module.HasGenesis     = AppModule{}
	_ module.HasName        = AppModule{}
)

// AppModuleBasic implements module.AppModuleBasic.
type AppModuleBasic struct {
	cdc codec.Codec
}

func NewAppModuleBasic(cdc codec.Codec) AppModuleBasic {
	return AppModuleBasic{cdc: cdc}
}

func (AppModuleBasic) Name() string { return types.ModuleName }

func (AppModuleBasic) RegisterLegacyAminoCodec(cdc *codec.LegacyAmino) {
	types.RegisterLegacyAminoCodec(cdc)
}

func (AppModuleBasic) RegisterInterfaces(reg cdctypes.InterfaceRegistry) {
	types.RegisterInterfaces(reg)
}

// DefaultGenesis returns the empty Phase 0 genesis. The app's genesis
// populator (in app.go) substitutes the inception pins derived from
// CanonicalSubCreeds + .sub-creed-hashes at chain init.
func (a AppModuleBasic) DefaultGenesis(cdc codec.JSONCodec) json.RawMessage {
	return cdc.MustMarshalJSON(types.DefaultGenesis())
}

func (a AppModuleBasic) ValidateGenesis(cdc codec.JSONCodec, _ client.TxEncodingConfig, raw json.RawMessage) error {
	var gs types.GenesisState
	if err := cdc.UnmarshalJSON(raw, &gs); err != nil {
		return fmt.Errorf("failed to unmarshal x/%s genesis: %w", types.ModuleName, err)
	}
	return gs.Validate()
}

// RegisterGRPCGatewayRoutes is a no-op at Phase 0 (no query server yet).
func (AppModuleBasic) RegisterGRPCGatewayRoutes(_ client.Context, _ *runtime.ServeMux) {}

// GetTxCmd returns nil at Phase 0 (no tx commands yet).
func (AppModuleBasic) GetTxCmd() *cobra.Command { return nil }

// GetQueryCmd returns nil at Phase 0 (no query commands yet).
func (AppModuleBasic) GetQueryCmd() *cobra.Command { return nil }

// AppModule implements module.AppModule + module.HasGenesis.
type AppModule struct {
	AppModuleBasic
	keeper keeper.Keeper
}

func NewAppModule(cdc codec.Codec, k keeper.Keeper) AppModule {
	return AppModule{
		AppModuleBasic: NewAppModuleBasic(cdc),
		keeper:         k,
	}
}

// IsAppModule implements appmodule.AppModule sentinel marker.
func (AppModule) IsAppModule() {}

// IsOnePerModuleType implements appmodule.AppModule sentinel marker.
func (AppModule) IsOnePerModuleType() {}

// RegisterServices is a no-op at Phase 0 (no msg or query servers).
func (am AppModule) RegisterServices(_ module.Configurator) {}

// InitGenesis writes the inception pins.
func (am AppModule) InitGenesis(ctx sdk.Context, cdc codec.JSONCodec, raw json.RawMessage) []abci.ValidatorUpdate {
	var gs types.GenesisState
	cdc.MustUnmarshalJSON(raw, &gs)
	am.keeper.InitGenesis(ctx, gs)
	return nil
}

// ExportGenesis returns the current pin set.
func (am AppModule) ExportGenesis(ctx sdk.Context, cdc codec.JSONCodec) json.RawMessage {
	gs := am.keeper.ExportGenesis(ctx)
	return cdc.MustMarshalJSON(&gs)
}

// ConsensusVersion returns the module's consensus version.
func (AppModule) ConsensusVersion() uint64 { return ConsensusVersion }

// BeginBlock and EndBlock are not implemented (no per-block work at Phase 0).
// The compiler does not require them; module manager's BeginBlocker /
// EndBlocker iteration skips modules without those interfaces.
```

- [ ] **Step 3: Verify build**

Run: `go build ./x/work_creed/...`
Expected: clean.

- [ ] **Step 4: Commit**

```bash
git add x/work_creed/module.go x/work_creed/doc.go
git commit -m "$(cat <<'EOF'
feat(work_creed): module skeleton — AppModuleBasic + HasGenesis

Cosmos SDK v0.50 module wiring with no msg/query servers and no
begin/end-blockers at Phase 0. InitGenesis writes inception pins;
ExportGenesis dumps latest pin per phase. doc.go declares the
purpose, the Knowledge-delegation rule, and the Phase 1+ extension
roadmap (MsgAnchorSubCreedPin, QueryPinAtVersion, SubCreedAmended
event).

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

### Task 21: Wire `x/work_creed` into `app/app.go`

**Files:**
- Modify: `app/app.go`

The module registers as a peer of `x/creed`. No keeper dependencies; `authority` is the gov module address (existing pattern).

- [ ] **Step 1: Find existing creed wiring**

Run: `grep -n "creed\|CreedKeeper" app/app.go | head -30`
Note the sites that need a `WorkCreedKeeper` peer entry. Typical sites:
- `import` block — add `workcreed "github.com/zerone-chain/zerone/x/work_creed"` and `workcreedkeeper "github.com/zerone-chain/zerone/x/work_creed/keeper"` and `workcreedtypes "github.com/zerone-chain/zerone/x/work_creed/types"`
- `storeKeys` map — add `workcreedtypes.StoreKey`
- `App` struct fields — add `WorkCreedKeeper workcreedkeeper.Keeper`
- Keeper constructor block — add `app.WorkCreedKeeper = workcreedkeeper.NewKeeper(appCodec, keys[workcreedtypes.StoreKey], authority.String())`
- `module.NewManager(...)` — add `workcreed.NewAppModule(appCodec, app.WorkCreedKeeper)`
- `genesisModuleOrder` — add `workcreedtypes.ModuleName` (after `creedtypes.ModuleName` is a natural slot)
- No `beginBlockerOrder` / `endBlockerOrder` entry needed (module has no begin/end blockers)

- [ ] **Step 2: Add the import**

In the `import (` block of `app/app.go`, add (alphabetical adjacency to existing creed imports):

```go
	workcreed "github.com/zerone-chain/zerone/x/work_creed"
	workcreedkeeper "github.com/zerone-chain/zerone/x/work_creed/keeper"
	workcreedtypes "github.com/zerone-chain/zerone/x/work_creed/types"
```

- [ ] **Step 3: Add storeKey**

Find the `storeKeys := storetypes.NewKVStoreKeys(` (or equivalent) block. Add `workcreedtypes.StoreKey` to the list.

- [ ] **Step 4: Add the App struct field**

Find the `App struct` block. Add to the keeper section:

```go
	WorkCreedKeeper workcreedkeeper.Keeper
```

A natural spot is immediately after the existing `CreedKeeper` field.

- [ ] **Step 5: Add the keeper constructor**

Find where `app.CreedKeeper = ...` is constructed. Immediately after, add:

```go
	app.WorkCreedKeeper = workcreedkeeper.NewKeeper(
		appCodec,
		keys[workcreedtypes.StoreKey],
		authority.String(),
	)
```

`authority` is the gov module address — existing local variable in this scope (used by `CreedKeeper` and others). If the variable is named differently in this codebase (e.g., `govAddr`), use that.

- [ ] **Step 6: Register the module in the module manager**

Find the `app.mm = module.NewManager(` block. Add:

```go
		workcreed.NewAppModule(appCodec, app.WorkCreedKeeper),
```

After the `creed` AppModule entry is the natural slot.

- [ ] **Step 7: Add to genesis order**

Find the `genesisModuleOrder` (or `app.mm.SetOrderInitGenesis(...)` / `setupUpgradeStoreLoaders` etc., depending on this codebase's app.go style). Add `workcreedtypes.ModuleName` immediately after `creedtypes.ModuleName`.

- [ ] **Step 8: Add inception pins to the genesis populator**

The merged spec calls for the inception pins to be populated automatically from `CanonicalSubCreeds` + `.sub-creed-hashes` at genesis. The simplest place for this is a small helper function called during `app.New(...)` *only* in the chain-init path (not on every restart).

Add this function to `app/app.go` or a new `app/work_creed_genesis.go`:

```go
// loadInceptionSubCreedPins constructs the genesis pins for x/work_creed
// from CanonicalSubCreeds + the on-disk .sub-creed-hashes file. This is
// called by the app's genesis-populator at chain init only (not on
// regular restart). It is the build-time → on-chain bridge for the
// 8 non-Knowledge phase sub-creeds.
//
// On error, the caller decides whether to abort genesis (panic in
// app.New) or proceed with empty pins (acceptable for tests).
func loadInceptionSubCreedPins(hashFilePath string) ([]workcreedtypes.PinnedSubCreed, error) {
	bz, err := os.ReadFile(hashFilePath)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", hashFilePath, err)
	}
	expected := map[string]string{}
	for _, line := range strings.Split(strings.TrimSpace(string(bz)), "\n") {
		parts := strings.Fields(line)
		if len(parts) == 2 {
			expected[parts[0]] = parts[1]
		}
	}

	var pins []workcreedtypes.PinnedSubCreed
	for _, phaseDef := range creedtypes.CanonicalLifecyclePhases {
		if !phaseDef.HasSubCreedDoc {
			continue
		}
		hashHex, ok := expected[phaseDef.Name]
		if !ok {
			return nil, fmt.Errorf("phase %s missing from %s", phaseDef.Name, hashFilePath)
		}
		hashBytes, err := hex.DecodeString(hashHex)
		if err != nil {
			return nil, fmt.Errorf("phase %s bad hex hash: %w", phaseDef.Name, err)
		}
		if len(hashBytes) != 32 {
			return nil, fmt.Errorf("phase %s hash wrong length: %d", phaseDef.Name, len(hashBytes))
		}

		def, ok := creedtypes.SubCreedFor(phaseDef.Number)
		if !ok || def.Commitments == nil {
			return nil, fmt.Errorf("phase %s SubCreedDef missing or empty", phaseDef.Name)
		}
		codes := make([]string, 0, len(def.Commitments))
		for _, c := range def.Commitments {
			codes = append(codes, c.Code)
		}

		pins = append(pins, workcreedtypes.PinnedSubCreed{
			Phase:           uint32(phaseDef.Number),
			PhaseName:       phaseDef.Name,
			Version:         1,
			CanonicalHash:   hashBytes,
			AnchoredAtBlock: 0,
			SourceLip:       "",
			CommitmentCodes: codes,
		})
	}
	return pins, nil
}
```

Required imports (add if not already present): `os`, `fmt`, `strings`, `encoding/hex`, `creedtypes "github.com/zerone-chain/zerone/x/creed/types"`, `workcreedtypes "github.com/zerone-chain/zerone/x/work_creed/types"`.

The caller (in the app's genesis-init path or in a `prepare-genesis` CLI handler — depending on the codebase's pattern) injects `pins` into the `GenesisState{PinnedSubCreeds: pins}` for the `x/work_creed` module. If the codebase's genesis-init path is unclear from this plan, defer the actual call site to a separate step:

- [ ] **Step 9: Call `loadInceptionSubCreedPins` from the genesis populator**

Locate the codebase's `prepare-genesis` or `init-testnet` flow (likely `cmd/zeroned` subcommand or `tools/bootstrap-loader/`). Wire the call so the inception pins are written into the `x/work_creed` module's genesis section. Exact integration depends on local conventions — read `tools/bootstrap-loader/` for the existing creed-pin pattern, then apply the same pattern.

If the existing creed pin (for truth-seeking) is currently injected from a parallel helper, mirror that helper's structure exactly. The merged spec's invariant: at chain genesis, every non-Knowledge phase has version-1 pin in `x/work_creed` matching the on-disk `.sub-creed-hashes`.

- [ ] **Step 10: Verify build**

Run: `go build ./...`
Expected: clean.

- [ ] **Step 11: Run app tests**

Run: `go test ./app/... -count=1 -timeout 120s`
Expected: PASS (existing app tests must continue to pass; the new module being wired does not change existing behavior).

- [ ] **Step 12: Commit**

```bash
git add app/app.go
# Plus any new file from Step 8 (e.g., app/work_creed_genesis.go) and modified bootstrap-loader files
git commit -m "$(cat <<'EOF'
feat(app): wire x/work_creed module — keeper + module manager + genesis

Adds WorkCreedKeeper to the App struct, registers the module in
module manager, sets genesis order after creed. Inception pins for
8 non-Knowledge phase sub-creeds are loaded from .sub-creed-hashes
at chain init via loadInceptionSubCreedPins helper, mirroring the
existing creed-pin populator pattern.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

### Task 22: Final integration sweep

- [ ] **Step 1: Run all useful-work / sub-creed / work-creed tests**

Run:
```bash
go test \
  ./x/creed/types/... \
  ./x/work_creed/... \
  ./tests/cross_stack/ -run "TestUsefulWork|TestSubCreed_|TestUW_|TestCanonical" \
  -v -count=1 -timeout 120s
```
Expected: PASS (TestUW_M{1..7} marked SKIP from previous Phase 0 plan; everything else PASS).

- [ ] **Step 2: Run hash check**

Run: `make creed-check`
Expected:
```
creed hash check ok (<truth-seeking-hash>)
useful-work hash check ok (<useful-work-hash>)
sub-creed hash check ok (foundation: <hash>)
... [7 more] ...
sub-creed hash check ok (tools: <hash>)
```

- [ ] **Step 3: Run proto-check**

Run: `make proto-check`
Expected: PASS (no proto/Go drift).

- [ ] **Step 4: Run full pre-PR check**

Run: `make pr-check`
Expected: PASS — covers lint, test, proto-check, creed-check, build.

- [ ] **Step 5: Verify the new file set**

Run:
```bash
git log --oneline --since="2026-05-10 00:00:00" -- \
  docs/sub_creeds/ \
  .sub-creed-hashes \
  scripts/check_sub_creed_hashes.sh \
  Makefile \
  x/creed/types/lifecycle_phases.go \
  x/creed/types/sub_creeds.go \
  x/creed/types/lifecycle_phases_test.go \
  x/creed/types/sub_creeds_test.go \
  proto/zerone/work_creed/ \
  x/work_creed/ \
  tests/cross_stack/useful_work_invariants_test.go \
  app/app.go
```
Expected: ~20 commits, one per task in this plan, in chronological order.

- [ ] **Step 6: Hand off**

Phase 0 (with this extension) is complete. The chain now:
- Has the 8 per-phase sub-creed seed docs at `docs/sub_creeds/`
- Has all 9 sub-creed hashes (1 truth-seeking + 1 useful-work + 8 sub-creed) verified by `make creed-check`
- Has Go-side canonical for the 9 lifecycle phases (`CanonicalLifecyclePhases`) and the 8 sub-creeds (`CanonicalSubCreeds`)
- Has 8 per-phase meta-tests + the extended `TestUsefulWork_DoctrineAndContractStayInSync` enforcing no drift
- Has the `x/work_creed` skeleton module wired into `app.go` with inception pins populated at chain init
- Has zero behavioral bindings yet — Phase 1 introduces `x/contribution` orchestrator + the `KNOWLEDGE_CLAIM` adapter

The next plan in the series is **Phase 1** of the merged spec: `x/contribution` skeleton + `KNOWLEDGE_CLAIM` adapter to existing `x/knowledge`. Phase 1 should be brainstormed → spec'd → planned in a separate cycle. Phase 1 binds M1 (stake), M2 (substrate-link), M3 (lifecycle), M4 (formula), M5 (shape), M7 (audit pool); M6 binds in Phase 4 (ToK TC6 cross-class extension).

---

## Self-Review

After implementing all tasks, verify:

1. **Spec coverage** (against merged spec §11 Phase 0 + §8.2):
   - 8 per-phase sub-creed seed docs (Foundation, Curation, Augmentation, Training, Evaluation, Alignment, Substrate, Tools) → Tasks 1–8
   - Each sub-creed has 3 inception commitments → Tasks 1–8
   - `x/work_creed` skeleton (proto + types + keeper + module wiring) → Tasks 16–21
   - Skipped invariant tests → covered by previous Phase 0 plan; per-phase tests added in Task 15
   - Hash anchored via on-chain pin → Task 21 Step 8 (genesis populator) + Task 19 (keeper)
   - Lifecycle-phase Go canonical → Task 12
   - Sub-creed Go canonical → Task 13
   - Hash verification scripted + wired into `make creed-check` → Tasks 9–11
   - Per-phase meta-tests → Task 15

2. **Position layer present:** `x/work_creed/doc.go` (Task 20). `x/creed/doc.go` extension may want a sentence about the trinity-with-lifecycle-phases — defer to a Phase 1+ refinement if not done in the previous Phase 0 plan.

3. **Voice layer present:** Phase 0 ships zero events (no behavioral bindings yet). The doctrine NAMES the events Phase 1+ must emit; this plan does not bind them.

4. **Refusal layer present:** Phase 0 ships one error sentinel (`ErrUnknownPhase` in Task 18). Refusal binding broadly comes in Phase 1+.

5. **Graph layer present:** Each sub-creed's `Echoes:` lines cross-reference the truth-seeking creed and other relevant doctrines. The cross-doctrine echo verification meta-test is Plan 5 of the ToK series (marker added in the previous Phase 0 plan's Task 10).

6. **All tests green:** `TestUsefulWork_DoctrineAndContractStayInSync` extended check passes; 8 `TestSubCreed_<Phase>_StaysInSync` PASS; `TestUW_M{1..7}` still SKIP; existing TruthSeeking + ToK tests unaffected.

7. **`make pr-check` PASS:** lint + test + proto-check + creed-check (now covers 10 hashes total) + build all green.

---

## What This Plan Does Not Do

- **No `x/contribution` orchestrator.** Phase 1 spec/plan introduces the absorption pipeline, the typed `Contribution` envelope, the per-class verification dispatch, the reward-emission router.
- **No `RecursionImpact` field, no six-axis scorers, no reward formula.** Phase 1.
- **No msg/query servers in `x/work_creed`.** AnchorSubCreedPin (gov-only) and QueryPinAtVersion land in Phase 1+ when sub-creed amendment LIPs need on-chain handling.
- **No per-class registrations.** Phase 2+.
- **No new modules beyond `x/work_creed` skeleton.** `x/dataset`, `x/eval`, `x/model_registry`, `x/probe`, `x/contribution` all land in later phases.
- **No truth-floor binding code.** The truth-floor is a doctrinal invariant declared in the merged spec; binding it as runtime check happens in Phase 1's `MsgSubmitContribution` validation.

— *Plan authored 2026-05-10. Phase 0 extension closes the doctrinal pinning of the merged spec; Phase 1 begins the orchestrator.*

---

## This document is a Contribution

This plan is itself a `Contribution` of class `PIPELINE_IMPROVEMENT`, lifecycle phase `SUBSTRATE`. Its content-hash is pinned at `.phase-0-extension-plan-hash`. The chain pays for its own design and execution; this document is among the work.
