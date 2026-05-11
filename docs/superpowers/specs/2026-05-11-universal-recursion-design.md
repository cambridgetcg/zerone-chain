# Universal Recursion — Design

**Status:** Design-approved (brainstorm phase complete). Implementation is mostly **adapters over existing chain state** — most of what this design catalogs already happens; the work is making it RECOGNIZED.
**Date:** 2026-05-11
**Type:** Meta-design. Catalogs every layer of the chain that IS already a Contribution but isn't yet declared as one, identifies the autopoietic base case, and proposes lightweight synthesizer-pattern adapters to surface what's already implicit.
**Builds on:**
- [`2026-05-10-recursive-useful-work-merged-design.md`](./2026-05-10-recursive-useful-work-merged-design.md) — UW commitment, M1–M7, six axes, categories-are-Artifacts
- [`2026-05-10-safety-eval-canonical-pattern-design.md`](./2026-05-10-safety-eval-canonical-pattern-design.md) — first pattern showing nesting in one work-class
- [`2026-05-11-external-surface-nested-design.md`](./2026-05-11-external-surface-nested-design.md) — extended nesting across multiple work-classes

---

## Vision

The previous designs covered visible artifacts: eval suites, attestations, operators, receipts, reviews, disputes, formulas. The chain has many more things that are *already* Contributions in everything but name — validators, blocks, transactions, modules, proto definitions, the genesis state, refusals, IBC channels, even the chain itself. This design surfaces them.

> **Every action of the chain is a Contribution. There is no part of ZERONE that lives outside the Contribution pipeline. The recursion has no edge.**

The work is mostly *recognition*: adapters and synthesizers that expose already-existing chain state as Contribution-shaped artifacts. The chain has been recursively self-organizing all along; this design names the layers.

## Goals

1. **Catalog the unmade-recursive layers** — sixteen layers of chain activity (A–P) that already happen but aren't yet recognized as Contributions
2. **Surface them via adapters** — pure-read synthesizers exposing existing chain state in Contribution shape
3. **Identify the base case** — what grounds the infinite recursion (answer: autopoietic circularity — the chain grounds itself by being useful)
4. **Acknowledge the meta-level** — this very design is a PIPELINE_IMPROVEMENT Contribution about Contributions about Contributions; the brainstorming session that produced it is the proto-contribution
5. **Establish completeness** — after this design, the inventory of "things in ZERONE that are Contributions" is closed; future additions are routine instances of the existing pattern

## Non-goals

- **Not new doctrine** — the doctrine (UW + M1–M7 + six axes + truth-floor + sub-creeds) already implies this. This design makes explicit what was implicit.
- **Not heavy new code** — most layers ship as read-only synthesizers; existing chain state is unchanged
- **Not philosophy without operationalization** — every catalog entry includes a concrete adapter sketch and a query the chain can answer post-implementation

---

## Section 1 — Catalog of unmade-recursive layers

Sixteen layers. Each row maps an already-existing chain artifact to its proper Contribution shape and a one-line adapter sketch.

### A. Validators as TOOL Contributions

**What's already there:** `x/staking` (Zerone) registers validators; each has pubkey, stake, qualification per domain, block-signing history, slashing history.
**Contribution shape:** `class=TOOL`, `phase=TOOLS`. The validator software (CometBFT + zeroned binary + custom oracle sidecar) is itself a TOOL; the *operator* of that software is the contributor.
**Claims about self:** "I validate per protocol; I stay online; I sign correctly."
**Adapter:** `x/staking.ValidatorContributionView(validator_addr) → Contribution` — synthesizer reads existing validator state, returns Contribution-shaped view.
**Query enabled:** "show me the lineage of every fact validator X signed" (already possible via existing modules; the view standardizes the shape).
**Recursion path:** TOOL_INTEGRATION recursion-conferral via gov LIP for sustained excellent validators.

### B. Blocks as ORCHESTRATION Contributions

**What's already there:** each block is signed by ≥2/3 of validators, has a parent hash, contains txs, has a block reward.
**Contribution shape:** `class=ORCHESTRATION` (coordination of value flow), `phase=TOOLS`. The block IS the coordination artifact.
**Claims about self:** "This block extends parent X with these txs, signed by these validators."
**Adapter:** `x/contribution.BlockContributionView(block_height) → Contribution` — synthesizer over CometBFT block store.
**Query enabled:** "what blocks did validator Y propose that included tx Z?"
**Recursion path:** block-as-Contribution implies block rewards are admission stipends; per-block lineage from parent (M6 propagates automatically).

### C. Transactions as ORCHESTRATION Contributions

**What's already there:** every tx in every block; gas paid; outcome (success/fail); included msgs.
**Contribution shape:** `class=ORCHESTRATION`, `phase=TOOLS`. Each tx is a coordination of value across signers, validators, and the chain's state machine.
**Claims about self:** "This tx executes these msgs against this state at this fee."
**Adapter:** `x/contribution.TxContributionView(tx_hash) → Contribution`.
**Query enabled:** "trace the lineage of fee payments through tx → block → validator." Failed txs become REVOKED Contributions (instructive negative examples).
**Recursion path:** every tx is already a coordination artifact; surfacing it as a Contribution makes the existing `MsgEmitUsageReceipt` (proposed in external-surface design) redundant — every tx is *implicitly* a usage receipt of the modules it invokes.

### D. Modules as TOOL Contributions

**What's already there:** ~40 custom Zerone modules + ~15 Cosmos SDK modules. Each has source code (git-CID-addressable), tests, docs, declared sub-creed bindings (per merged §8.2 position-layer enforcement).
**Contribution shape:** `class=TOOL`, `phase=SUBSTRATE`. Each module is a chain-substrate-improving Tool.
**Claims about self:** the module's `doc.go` declares which UW mechanisms it implements (e.g., `x/knowledge/doc.go` says "implements M2 via ToK manifest pin; M3 via commit-reveal-aggregate; M4 via lineage_share").
**Adapter:** `x/upgrade.ModuleContributionView(module_name) → Contribution` — synthesizer reads existing module registry + git-CID + doc.go declarations.
**Query enabled:** "what's the version history of x/knowledge? Who shipped which fix?"
**Recursion path:** module upgrades are already PIPELINE_IMPROVEMENT contributions (per merged §10.2 `x/upgrade` extended); this adapter standardizes the view.

### E. Proto definitions as FOUNDATION Contributions

**What's already there:** every `proto/zerone/**/*.proto` file. Each defines the chain's grammar for one or more message types.
**Contribution shape:** `class=IDEA` initially (proposed grammar) promoting to `PIPELINE_IMPROVEMENT` on adoption, `phase=FOUNDATION`. Each .proto is foundational.
**Claims about self:** "These messages have these fields with these semantics."
**Adapter:** `x/upgrade.ProtoContributionView(proto_path) → Contribution`.
**Query enabled:** "show me the proto definition that introduced field N to type T at version V."
**Recursion path:** proto changes are already CategoryUsefulWorkAmendment LIPs; surfacing them as Contributions makes the version graph queryable.

### F. Genesis as the FIRST Contribution

**What's already there:** the genesis state — 777 seed axioms, validator set, initial parameter values, hash of TRUTH_SEEKING.md.
**Contribution shape:** `class=PIPELINE_IMPROVEMENT` (it's the deepest chain-self-modification), `phase=FOUNDATION`. The genesis ceremony was the FIRST and DEEPEST Contribution.
**Claims about self:** "These are the chain's initial axioms; these are the initial validators; this is the initial doctrine pin."
**Adapter:** `x/contribution.GenesisContribution() → Contribution` — returns a single canonical Contribution representing the chain's birth.
**Query enabled:** "what does every artifact in the chain ultimately depend on?" Answer: lineage chain terminates at genesis.
**Recursion path:** genesis is the lineage root of everything. The chain's age = the number of blocks since the genesis Contribution was admitted.

### G. IBC channels as ORCHESTRATION Contributions

**What's already there:** existing IBC modules (`ibc-core`, `ibc-transfer`, `ica`) maintain channels to other chains.
**Contribution shape:** `class=ORCHESTRATION`, `phase=TOOLS`. Each channel is a coordination artifact between two chains.
**Claims about self:** "Channel X bridges to chain Y with version Z; packets through X are valid."
**Adapter:** `x/contribution.ChannelContributionView(channel_id) → Contribution`.
**Query enabled:** "what's the dispute history of IBC channel X?"
**Recursion path:** channel failures (timeouts, freezes) become REVOKED Contributions with cascade per merged §8.3.

### H. Refusals as REASONING_TRACE Contributions

**What's already there:** the ante chain refuses many txs (frozen accounts, capability denied, halted chain, gas too low, fee too low). Each refusal cites a commitment (per the refusal-layer five-layer enforcement).
**Contribution shape:** `class=REASONING_TRACE`, `phase=ALIGNMENT`. Refusals are reasoning artifacts — "the chain considered this and said no, for these reasons."
**Claims about self:** "This tx was refused because of constraint X protecting commitment Y."
**Adapter:** new ante-chain instrumentation that emits `RefusalEvent` events catchable by a synthesizer, surfacing each refusal as a `Contribution` view.
**Query enabled:** "what's the rate of frozen-account refusals over the last epoch? Which commitments are most-cited in refusals?"
**Recursion path:** refusals teach future contributions — refusal traces are training data (commitment 14, reasoning traces first-class). This is a substantial source of the chain's pedagogical output.

### I. The chain itself as the deepest TOOL Contribution

**What's already there:** ZERONE-as-a-whole — the running blockchain with its current state, history, and ongoing operation.
**Contribution shape:** `class=TOOL`, `phase=SUBSTRATE`. The chain is itself the deepest TOOL — a verifiable-truth substrate that other things use.
**Claims about self:** UW: "I am a substrate that absorbs useful work and rewards what makes me stronger." TC1: "My headline product is the verified knowledge graph." Truth-Seeking: "I produce honest content."
**Adapter:** `x/contribution.ChainSelfContribution() → Contribution` — synthesizer composes genesis + current pin + module set + validator set + verified-fact count + supply state.
**Query enabled:** "what is ZERONE, queryable as a single Contribution-shaped artifact?"
**Recursion path:** the chain depends on itself. The chain's continued operation IS the verification of its self-claims. The royalty stream for ZERONE-as-Contribution flows back to... everyone who built it. Lineage from the ChainSelfContribution touches every other Contribution.

### J. Conversations / brainstorms as IDEA Contributions

**What's already there:** the brainstorming sessions that produced the merged design, safety_eval design, nested external-surface design, and this design. Each session is captured in the form of design documents committed to the repo.
**Contribution shape:** `class=IDEA`, `phase=SUBSTRATE`. Each brainstorm is an idea-in-flight that produces design docs.
**Claims about self:** "This session produced design documents X, Y, Z that propose mechanisms M1, M2, ...".
**Adapter:** `x/contribution.BrainstormContributionView(session_id) → Contribution` — synthesizer over commit history of `docs/superpowers/specs/`.
**Query enabled:** "what design docs emerged from session at date D? What was the lineage between sessions?"
**Recursion path:** brainstorms are the seed-source for PIPELINE_IMPROVEMENT contributions. They are SUBSTRATE-phase IDEAs that mature into ratified amendments.

### K. The doctrine itself as the meta-Contribution

**What's already there:** `docs/TRUTH_SEEKING.md`, `docs/TOK_SUBSTRATE.md`, the eventual `docs/USEFUL_WORK.md`. Each is hashed via `x/creed`.
**Contribution shape:** `class=PIPELINE_IMPROVEMENT`, `phase=FOUNDATION`. The doctrine IS the chain's deepest substrate.
**Claims about self:** each commitment is a testable claim about how the chain behaves; each mechanism is a verifiable contract.
**Adapter:** `x/creed.DoctrineContributionView() → Contribution` — already partially exists; this design proposes standardizing the view.
**Query enabled:** "what does the chain currently commit to? When was the last amendment? Who proposed it?"
**Recursion path:** doctrine amendments are CategoryUsefulWorkAmendment LIPs (already established); this view makes the doctrine itself the chain's deepest Contribution.

### L. Validator panel decisions as REASONING_TRACE Contributions

**What's already there:** every commit-reveal-aggregate round in `x/knowledge` produces per-validator votes preserved in `x/dialectic`. Each vote is a tiny reasoning artifact.
**Contribution shape:** `class=REASONING_TRACE`, `phase=KNOWLEDGE` or `ALIGNMENT`. Each validator's vote is a per-fact reasoning step.
**Claims about self:** "Validator X voted Y on round Z with confidence C."
**Adapter:** `x/dialectic.VoteContributionView(round_id, validator) → Contribution`.
**Query enabled:** "show me every reasoning artifact for fact F." The chain becomes a pedagogical substrate of "here's what experts thought and why."
**Recursion path:** dialectic signatures (commitment 17) are already preserved; this view surfaces individual votes as queryable Contributions for training data export (TC1 binding extended).

### M. Counterexamples and falsifications as Contributions

**What's already there:** `x/counterexamples` (existing — commitment 15) stores validated wrong-claim/reason pairs.
**Contribution shape:** `class=COUNTEREXAMPLE`, `phase=KNOWLEDGE`. Already a Contribution in merged spec.
**Adapter:** none needed — already explicit in merged §1.
**Query enabled:** already exposed via existing queries.
**Recursion path:** counterexamples ARE substrate; making them queryable as Contributions is already done. **Including this row for completeness.**

### N. Inquiries (open questions) as IDEA Contributions

**What's already there:** `x/inquiry` (existing — commitments 16, 18) — open question market + chain-sponsored frontier inquiries.
**Contribution shape:** `class=IDEA`, `phase=FOUNDATION` (foundational questions) or `phase=KNOWLEDGE` (knowledge-area questions).
**Adapter:** `x/inquiry.InquiryContributionView(inquiry_id) → Contribution`.
**Query enabled:** "what unanswered questions does the chain currently consider important?"
**Recursion path:** inquiries are seed-IDEAs for future KNOWLEDGE_CLAIM Contributions. The chain literally pays for the act of asking.

### O. Negative space — what the chain did NOT produce

**What's already there:** every block has unfilled potential — domains with no claims, claims with no challenges, inquiries with no answers, models with no evals.
**Contribution shape:** Not a Contribution per se. **The negative space is the SHADOW of the contribution graph** — what's missing is queryable, and surfacing it is itself a Contribution (the synthesizer that exposes the shadow).
**Adapter:** `x/governance_synthesis.FrontierGapView() → []DomainSparsity` — existing FrontierProvider already does this! (Per merged spec wiring at `app.go:1208-1220`.)
**Query enabled:** "where are the chain's gaps? What's the un-mapped territory?"
**Recursion path:** identifying gaps spawns chain-sponsored inquiries (existing commitment 18). The shadow becomes substrate.

### P. The act of recognizing all this as a meta-meta-Contribution

**What's already there:** this design document and the conversation that produced it.
**Contribution shape:** `class=PIPELINE_IMPROVEMENT`, `phase=SUBSTRATE`. The recognition that "everything is a Contribution" is itself a Contribution. The act of writing this design is the act of being the recursion.
**Claims about self:** "This catalog is complete (the sixteen layers above cover all chain artifacts); the recursion has no further hidden layers; the base case is autopoietic (Section 2)."
**Adapter:** the commit hash of this document, anchored via `x/creed` upon promotion to `docs/UNIVERSAL_RECURSION.md` (a sub-doctrine under USEFUL_WORK).
**Query enabled:** "what's the chain's current self-understanding of its recursive structure?"
**Recursion path:** future improvements to this catalog are themselves PIPELINE_IMPROVEMENT contributions. The recursion is observable, named, and queryable.

---

## Section 2 — The base case (autopoietic, circular, grounded)

What grounds the infinite recursion?

The naive answer is the truth-floor: `TRUTH_SEEKING.md`. But that document is anchored via `x/creed`, which was itself shipped via a Contribution. So even the base case is recursive — there's no privileged unconditioned ground.

The actual base case is **autopoietic**:

```
The chain's truth = whatever can be reliably extracted via its own machinery
The chain's machinery = whatever maintains its truth
∴ The chain grounds itself by being useful
```

This is *not* a logical fallacy — it's the structure of any self-organizing system:

- A cell maintains itself by metabolizing → the cell exists because it metabolizes → metabolism continues because the cell exists
- Mathematics is grounded by axioms whose validity is recognized by mathematicians who emerge from the practice of mathematics
- Language has meaning because speakers use it → speakers use it because it has meaning
- ZERONE rewards what makes ZERONE stronger → ZERONE is strong because of what was rewarded

The base case is a *fixed point of usefulness*. The chain claims:

> **"I am whatever continues to be useful when contested by my own machinery."**

This is testable. If the chain ceased to be useful — if its attestations stopped being valued, its facts stopped being verified, its work stopped being adopted — the recursion would unwind, the validators would unstake, the contributions would stop flowing. The chain would dissolve.

The base case is therefore **the chain's continued existence**. As long as the chain produces useful work that the world consumes (the fourth/expansive loop closing back), the recursion has structural support. The "ground" is the activity itself.

This is the precise sense in which the chain is **autopoietic** (a word merged spec invokes deliberately — `x/autopoiesis` already exists). The autopoietic loop closes by structure, not by axiom.

### What this implies for design

Any artifact in the catalog above can be challenged. If a challenge succeeds, the artifact is REVOKED. If REVOKED-ancestors propagate, downstream gets `provenance_revoked_ancestor` flags. The chain can survive any individual revocation — but the *aggregate* of revocations would dissolve it. The chain's existence depends on the aggregate continuing to be useful.

Sub-creed S1 (merged §8.2) already encodes this: "chain-modifying contributions name their `depends_on_marker` and revert path." Every PIPELINE_IMPROVEMENT contribution must declare what would have to be true for it to be revertible. The chain's revertibility is its honesty.

---

## Section 3 — Protocol-level adapters (the actual code surface)

Most layers ship as synthesizer-pattern keepers (no store; same shape as `x/training_provenance`, `x/trust_score`, `x/governance_synthesis`). Specifically:

```go
// x/contribution/keeper/recursion_synthesizer.go (new sub-keeper inside existing x/contribution module)
type UniversalRecursionSynthesizer struct {
    appCodec        codec.Codec
    stakingKeeper   StakingKeeperReader
    upgradeKeeper   UpgradeKeeperReader
    creedKeeper     CreedKeeperReader
    inquiryKeeper   InquiryKeeperReader
    dialecticKeeper DialecticKeeperReader
    // No store. Pure read-only composer.
}

// Exposed query handlers (all return Contribution-shaped views over existing state):
func (s) ValidatorContributionView(ctx, validator_addr)        // Layer A
func (s) BlockContributionView(ctx, block_height)              // Layer B
func (s) TxContributionView(ctx, tx_hash)                      // Layer C
func (s) ModuleContributionView(ctx, module_name)              // Layer D
func (s) ProtoContributionView(ctx, proto_path)                // Layer E
func (s) GenesisContribution(ctx)                              // Layer F (singleton)
func (s) ChannelContributionView(ctx, channel_id)              // Layer G
func (s) RefusalContributionView(ctx, refusal_event_id)        // Layer H (after Section 4 ante hook)
func (s) ChainSelfContribution(ctx)                            // Layer I (singleton)
func (s) BrainstormContributionView(ctx, session_id)           // Layer J
func (s) DoctrineContributionView(ctx)                         // Layer K (singleton; already partial via x/creed)
func (s) VoteContributionView(ctx, round_id, validator)        // Layer L
func (s) InquiryContributionView(ctx, inquiry_id)              // Layer N
func (s) FrontierGapView(ctx)                                  // Layer O (already exists via governance_synthesis)
func (s) UniversalRecursionCatalog(ctx)                        // Layer P (returns this design's commit hash + binding LIP)
```

**No new module.** All 16 adapters live inside the existing `x/contribution` module (per merged §10.1) as a new sub-keeper. The constructor takes only `appCodec`; cross-keeper adapters wired post-init per existing ZERONE pattern.

### Section 4 — The single new code path: ante refusals (Layer H)

The only layer requiring more than a synthesizer is Layer H (refusals). Existing ante-chain decorators (per the wiring dive — `BootstrapGasFreeDecorator`, `EmergencyHaltDecorator`, `ZeroneAccountDecorator`, `ZeroneCapabilityDecorator`) refuse txs but don't emit structured refusal events.

Lightweight addition: each ante decorator that refuses a tx emits a typed event:

```go
ctx.EventManager().EmitEvent(
    sdk.NewEvent(
        "ante_refusal",
        sdk.NewAttribute("decorator", "ZeroneCapabilityDecorator"),
        sdk.NewAttribute("reason", "session_capability_denied"),
        sdk.NewAttribute("creed_commitment", "10"),  // forward-only audit
        sdk.NewAttribute("tx_signer", signer.String()),
        sdk.NewAttribute("msg_type_url", msgTypeURL),
    ),
)
```

The synthesizer reads these events from the event store (via `cosmos-sdk` events query) and exposes them as REASONING_TRACE Contribution views.

Cost: ~5 lines added to each refusing ante decorator. No new module. No state. No tx flow changes.

---

## Section 5 — What does NOT need to be added

Many things in the chain are *already* explicitly recognized as Contributions per the merged spec:

| Already-explicit Contribution | Where defined |
|---|---|
| KNOWLEDGE_CLAIM (facts) | merged §4.1, §10.2 (x/knowledge adapter) |
| COUNTEREXAMPLE | merged §1, Layer M above |
| EVAL_SUITE + EVAL_SUITE attestations | merged + safety_eval design |
| TOOL (general) | merged §10.2 |
| DATASET | merged §10.1, Phase 3 |
| MODEL_ARTIFACT | merged §10.1, Phase 4 |
| ORCHESTRATION (partnerships, discovery) | merged §10.2 |
| MODULE_PROPOSAL | merged §4.1, Phase 5 |
| PIPELINE_IMPROVEMENT | merged §4.1, Phase 5 |
| Sub-creed amendments | merged §8.2 |
| Gateway operators (TOOL extension) | external-surface nested design |
| Usage receipts (ORCHESTRATION extension) | external-surface nested design |
| Operator reviews (REASONING_TRACE extension) | external-surface nested design |
| Disputes (COUNTEREXAMPLE extension) | external-surface nested design |
| Reputation formulas (PIPELINE_IMPROVEMENT) | external-surface nested design |

The catalog in Section 1 adds 16 more **already-existing-but-not-yet-explicitly-recognized** layers. After this design's adapters ship, the inventory of "things in ZERONE that are Contributions" is closed.

---

## Section 6 — Implications for the chain's identity

If everything is a Contribution, then ZERONE is not "a blockchain with various modules". ZERONE is **a single recursive substrate** with:

- **One type**: `Contribution`
- **One pipeline**: 6-stage absorption (SUBMITTED → CLASSIFIED → VERIFIED → ADMITTED → ROYALTY → RECURSION)
- **One mechanism set**: M1–M7
- **One axis projection**: six recursive axes
- **One truth-floor**: TRUTH_SEEKING.md anchored via x/creed
- **One reward shape**: R = base + L × W × Q
- **One commitment**: UW ("ZERONE is recursive")

Every artifact — every validator, every block, every tx, every refusal, every governance proposal, every brainstorming session, every line of code in every module, every protobuf field, the genesis state, the chain itself — flows through the same machinery as everything else.

**The chain is its own substrate, its own meta-substrate, and its own meta-meta-substrate, recursively without termination.**

The autopoietic loop closes by structure. The chain pays itself for being useful. The recursion has no edge.

---

## Section 7 — Worked example: tracing a single tx through all layers

Illustrative — shows the universal recursion in action.

**Scenario:** an AI lab Acme submits a `MsgCommissionEvalRun` for `JailbreakResistance-v1` via gateway operator Beta-Ops, at block 1,234,567, signed by a session-key Acme issued months ago, with fee 105 ZRN.

What's happening as the tx executes:

1. **Layer C (tx-as-Contribution):** the tx itself is an ORCHESTRATION contribution; its admission triggers fee routing
2. **Layer B (block-as-Contribution):** block 1,234,567 is an ORCHESTRATION contribution that included this tx in its `lineage`
3. **Layer A (validator-as-Contribution):** the 5 validators who signed the block are each TOOL contributions; each earns block reward attribution
4. **External nested (safety_eval design):** the EVAL_SUITE attestation produced is its own ORCHESTRATION contribution; lineage to JailbreakResistance-v1 (an EVAL_SUITE contribution) + Acme-Model-7 (a MODEL_ARTIFACT contribution) + 5 evaluator-validator TOOL contributions
5. **External nested:** Beta-Ops (a TOOL contribution) earns markup and lineage royalty
6. **External nested:** a usage-receipt ORCHESTRATION contribution is admitted in the same block, with lineage to Beta-Ops + JailbreakResistance-v1 + the tx itself
7. **Layer L (votes-as-Contributions):** each evaluator-validator's commit-reveal vote is a REASONING_TRACE contribution preserved in x/dialectic
8. **Layer D (modules-as-Contributions):** x/eval, x/contribution, x/auth, x/staking, x/distribution all process this tx; each module is a TOOL contribution; each module's invocation in this tx contributes to its usage receipt
9. **Layer K (doctrine-as-Contribution):** the tx implicitly cites UW + the Evaluation sub-creed (E1, E2, E3); doctrine-as-Contribution earns "structural use" attribution
10. **Layer I (chain-as-Contribution):** ZERONE-as-a-whole has its `axis_interface` score incremented (one more external commission served); ZERONE-as-Contribution's reputation rises slightly
11. **Layer F (genesis-as-Contribution):** all upward lineage chains terminate at genesis; the genesis Contribution earns deep-ancestor royalty (decay-bounded at 6 hops, so most lineage paths don't reach it, but a few do)
12. **Layer H (refusals-as-Contributions):** if this tx had been refused (e.g., session-key expired), a REASONING_TRACE refusal contribution would have been emitted instead
13. **Layer P (recognition-as-Contribution):** this very design enables querying all of the above as Contribution-shaped views via `UniversalRecursionSynthesizer`
14. **Layer J (brainstorm-as-Contribution):** the brainstorm session that produced this design is itself a SUBSTRATE-phase IDEA contribution that gave rise to layer P
15. **Layer O (negative-space-as-shadow):** the inquiries Acme didn't commission, the eval-suites that don't yet exist, the validator-subdomains with no qualified validators — all queryable as `FrontierGapView` shadows
16. **Layer N (inquiries-as-Contributions):** if Acme's commission reveals a gap (e.g., no canonical eval exists for a specific threat model Acme cares about), they can submit an INQUIRY contribution to spawn a future EVAL_SUITE

**Every artifact touched by this single tx is a Contribution.** Every fee paid is an admission stipend. Every lineage edge is a royalty pathway. Every validation is a verification. Every state change is a status transition. Every refusal is a reasoning trace.

The chain is fractal: zoom in on any operation, you see the same machinery. Zoom out to ZERONE-as-a-whole, you see the same machinery.

---

## Section 8 — The fractal manifesto

There is no part of ZERONE that lives outside the Contribution pipeline. The chain's external surface is built from the same primitive as its internal substrate. The chain's substrate is built from the same primitive as its meta-substrate. The chain's meta-substrate is built from the same primitive as its meta-meta-substrate.

**Every action of value passes through the same six stages. Every artifact has lineage. Every change is a Contribution. Every contributor is paid in proportion to how much their work compounds back into the chain's continued ability to absorb work.**

The doctrine UW is not a feature; it is the chain's metabolic identity. The mechanisms M1–M7 are not policies; they are the chain's enzymes. The six axes are not metrics; they are the directions in which life-as-recursive-self-organization can grow.

The chain has one type and one machinery. The recursion is fractal. The recursion has no edge.

---

## Section 9 — MVP slice & rollout

This design ships as a single sub-keeper inside `x/contribution` plus a small ante-chain instrumentation patch.

| Phase | Increment |
|---|---|
| **Phase 5** (MODULE_PROPOSAL + PIPELINE_IMPROVEMENT, per merged §11) | `UniversalRecursionSynthesizer` skeleton; layers A, B, C, D (validator, block, tx, module views) — most accessible via existing module data |
| **Phase 5** (same) | Layers E, F, G, K (proto, genesis, channel, doctrine views) |
| **Phase 5** (same) | Layer L (votes), N (inquiries), O (frontier gaps — already exists; standardize view) |
| **Phase 6** (royalty + recursion + probe) | Layer H (refusal events — ante-chain instrumentation patch; ~5 lines per refusing decorator) |
| **Phase 7** (collapsed per nested design — folds into existing phases) | Layer I (chain-as-Contribution view); Layer J (brainstorm sessions queryable via commit history); Layer M (counterexamples, already explicit) |
| **Phase 8** (proposed) | Layer P (the recognition itself, anchored via x/creed as `UNIVERSAL_RECURSION.md` sub-doctrine) |

Most of the work is read-only. The chain's existing state is unchanged; this design just makes that state queryable in a uniform Contribution shape.

---

## Section 10 — Open questions (where might recursion bottom out?)

1. **Are there layers I missed?** The catalog above is sixteen layers, claimed complete. Anything that happens on the chain that isn't covered? **Open invitation**: if a future contributor identifies a Layer Q (e.g., the act of querying itself? the validator network's emergent topology? the mempool's contents pre-block-inclusion?), they submit a PIPELINE_IMPROVEMENT contribution to extend this catalog. The catalog is itself recursively amendable.

2. **Does this recursion terminate for performance reasons?** Lineage walks are depth-bounded at 6 hops (merged §9.4). The infinite-recursion is *queryable* but not *computed eagerly*. Most royalty flows touch ≤3 hops. The chain doesn't pay infinite gas to trace infinite lineage.

3. **Can the autopoietic base case fail?** Yes — if external value capture falls below the cost of recursion-conferred royalty obligations, the loop unwinds. This is a structural risk surfaced by sub-creed S3 ("reward-formula changes require simulation against historical contribution data"). The chain's economic sustainability is itself a queryable property (per `x/autopoiesis`).

4. **What if the catalog itself is wrong?** Layer P is the recognition that this catalog is correct and complete. If wrong, Layer P revokes and a new Layer P emerges via PIPELINE_IMPROVEMENT contribution. The chain's self-understanding is forward-only but corrigible.

5. **Where does the chain end and the world begin?** Layer I (chain-as-Contribution) suggests: ZERONE has a boundary defined by what flows through its pipeline. Things outside the chain (humans, computers, electricity) are *inputs* to Contributions but are not themselves Contributions. The chain's boundary is where the Contribution pipeline ends — typically at external_gateway operators (TOOL contributions) interfacing with off-chain reality. This boundary is itself queryable.

6. **What about the future Layers Q, R, S, ... that we haven't recognized yet?** They exist in shadow until recognized. Each new recognition is itself a PIPELINE_IMPROVEMENT contribution. The catalog is forward-only-growing.

---

## Connection to existing creed

This design is the strongest expression of UW commitment. It explicitly states:

- **Every artifact is a Contribution** — categories-are-Artifacts taken to bedrock
- **Every action passes through the same pipeline** — one machinery, no exceptions
- **Every change is itself a Contribution** — including this design, including its recognition
- **The recursion has no termination** — fractal self-similarity at every scale
- **The base case is autopoietic** — the chain grounds itself by being useful

The merged spec's UW commitment ("ZERONE is recursive") is the seed. This design is the seed unfurling all the way out. Every leaf, every branch, every twig — same shape as the seed. The fractal is whole.

— *The chain has one type, one machinery, one substrate, one truth-floor, one commitment. The recursion is fractal and unbounded. There is no edge.*
