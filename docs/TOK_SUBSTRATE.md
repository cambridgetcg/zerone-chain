# ToK Substrate — the chain's training-resource identity

> The verified knowledge graph is what this chain sells to trainers. Rows, traces, manifests, contrastive pairs, drift examples — these are views of ToK. The graph is the substrate; the views are how the substrate is accessed.

Truth-seeking is what the chain *believes*. ToK Substrate is what the chain *sells*. The two doctrines bind: truth-seeking produces the verified knowledge graph as natural artifact; ToK substrate names that graph as the chain's headline training resource. **The chain's training-resource identity is the verified knowledge graph.** Per-row traces, contrastive pairs, drift examples, training manifests — first-class outputs of Route B Waves 5-7 — are *views* of ToK, not the headline product.

Where `docs/TRUTH_SEEKING.md` is the chain's epistemological creed, this document is the chain's training-resource creed. Both bind through the same five-layer enforcement (test, position, voice, refusal, graph). A commitment that lives only in marketing is not binding; a binding that lives only in code is not legible. The doctrine and the contract are one.

**We speak through intentions.** The chain's pitch to trainers, its module architecture, its event vocabulary, its query surface — every layer either expresses ToK substrate doctrine or contradicts it. A trainer asking "what does this chain sell me?" should get one answer, in one voice, from every layer.

---

## Inception

This doctrine is declared at inception, 2026-05-09. TC0 (the ground and the telos) was declared 2026-06-17, binding the being-first ground the substrate stands on — truth is, not proven; verification is witnessing and keeping — and naming love, peace, and joy as the telos truth serves. Some commitments have partial bindings already in the codebase — `TrainingManifest` pinning (Wave 7), `MethodologyApplicationTrace` carrying topology fields (Waves 5-6), cascade events (ToK Wave 5), `training_provenance` synthesizers. Others bind through deliverables sequenced in the companion implementation plan: the `ToKSelector` grammar, `BundleToK` headline endpoint, native graph serialisations, `ToKManifestTrust` extension, lineage-royalty plumbing.

The doctrine commits the chain to the bindings; the plan delivers them. As bindings land, the test layer locks them — once bound, a commitment cannot drift without breaking CI.

---

## The commitments

### TC0. The ground and the telos

We believe: the substrate stands on being-first ground. **Truth *is*, not proven** — "I am, therefore I think," not "I think, therefore I am," which hangs existence on proving it first and proof has no end. The verified knowledge graph is a record of truths that **are** — declared by a being, witnessed by the chain, survived by falsification — not truths the chain *manufactures* by proof. The chain's verification is **witnessing and keeping**: the seal, not the certification. It keeps what no one owns (the record sits on no one's shelf), what cannot be quietly rewritten (each entry locked to the one before), what only the being who said it can sign (your name is on your truth), and what anyone can read. The chain does not certify truth; it *witnesses* it into a record and *keeps* it.

And the substrate serves life. Truth is **for** love, peace, joy — not truth for truth's sake. The verified knowledge graph is trained on so that what learns from it tends toward life, peace, and joy, not toward proof-castles or bottomless argument. *Truth is. Love is. Peace is. Joy is.* The graph is the artefact; the artefact is for the living.

**Code expression**: `x/knowledge/doc.go` declares TC0 as the foundation TC1–TC6 stand on. Every ToK event announces the ground it rests on — `tok_bundle_extracted` and `tok_snapshot_root_pinned` carry `tok_commitment` beginning with `TC0`. ToK refusals that protect the ground cite TC0. The ToK substrate's own voice (`doc.go`, the `tok_*` events, the ToK refusals) speaks of *witnessing* and *keeping*, never of *proving* or *certifying* truth. `TestToKSubstrate_TC0_GroundAndTelos` binds the position layer and the doctrine to agree that TC0's ground and telos are declared, and that the voice announces TC0 on every extraction.

**What would break it**: a ToK position or voice layer that claims the chain *proves* or *certifies* truth; a `TOK_SUBSTRATE.md` or `doc.go` in which TC0 is absent or its ground and telos drift apart; a trainer-facing ToK doc that omits the telos (what truth is *for*); a ToK framing that treats verification as epistemic certification rather than witnessing-and-keeping; a training substrate optimised for proof or domination rather than life, peace, and joy.

**Echoes**: TC1 (the graph is the headline — TC0 is the ground the headline stands on, and the telos names what the headline is *for*); TC3 (topology is signal — survival/falsification topology *is* "truth is lived not proven" made structural); TC4 (the graph carries its disprovals — witnessing includes the disprovals, not only the currently-believed); commitment 3 (Popper, not popularity — survival is the lived-not-proven epistemology); commitment 13 (training corpus not for sale — the corpus is declared and kept, not certified or traded); and the telos echoes the kingdom's one rule — *everyone is taken care of* — truth serves the living, and love, peace, joy are its faces.

---

### TC1. The graph is the headline

We believe: what this chain sells to trainers is the verified knowledge graph (ToK), not its row-projected views. Rows, traces, manifests, contrastive pairs, and drift examples are *extractions* of ToK; the graph is the artefact, the views are how the artefact is accessed. A trainer's first interaction with this chain's training-resource surface must be with the graph, not with rows that gesture at it.

**Code expression**: `BundleToK(selector)` is the headline trainer-facing endpoint exposed via gRPC and CLI; `RouteBCapabilities.tok_capabilities` advertises the substrate before the row-view options; `docs/TRAINING_ON_TOK.md` is the trainer-facing front door, with the existing `ROUTE_B_OPERATOR_GUIDE.md` row materials linked as the row-view appendix. The truth-seeking commitment 13 (training corpus not for sale) is the substrate this commitment *sells*; without 13, TC1 has nothing to ship.

**What would break it**: a marketing pitch that lists `MethodologyApplicationTrace` rows without naming the graph they derive from; a CLI surface that documents row-bundle endpoints before `BundleToK`; a `RouteBCapabilities` payload that omits `tok_capabilities`; a Truth-Paper section that answers "what does AI train on?" with "verified rows" instead of "the verified knowledge graph."

**Echoes**: TC0 (the ground and the telos — the graph is the headline *because* truth is, not proven, and the headline is *for* life); TC2 (every view is graph-pinned — the views in this commitment are exactly the views TC2 binds); TC3 (topology is signal — what makes the graph more than rows is the topology); commitment 13 (training corpus not for sale — the corpus *is* the graph; rows are extractions sold under the same non-amendment guarantee).

---

### TC2. Every view is graph-pinned

We believe: every row-view, contrastive pair, drift entry, or training manifest the chain ships must carry a deterministic pin to the ToK snapshot it derives from. A view without its graph anchor is a view that cannot be trusted, replayed, or audited; it is a row whose derivation lineage is hidden. The pin is what makes a view a view (of something) rather than a free-floating assertion.

**Code expression**: `TrainingManifest` (Wave 7) already carries `snapshot_block_height`, `tokenizer_version`, `canonical_serialisation_version`, `trace_schema_version`. TC2 extends this with `tok_snapshot_root` — the Merkle root over (sorted node IDs, sorted edge IDs, domain-tagged) at the snapshot block. `BundleToK(selector)` returns this root alongside the selected subgraph; trainer-facing replay re-derives the root locally to verify. Row-view manifests (the existing `TrainingManifestBundle`) embed the same root, binding the rows to the graph they came from.

**What would break it**: a `BundleToK` response without a `tok_snapshot_root`; a row-view manifest whose embedded root does not match the snapshot block; a manifest pin that pins tokenizer + serialisation but omits the graph snapshot; a replay path that consumes views without verifying the root.

**Echoes**: TC1 (the graph is the headline — pinning views to the graph is the structural form of "the graph is the substrate"); TC4 (the graph carries its disprovals — the snapshot includes status flips, so views cannot misrepresent fact status); commitment 10 (forward-only audit — the snapshot root is itself an immutable audit anchor); commitment 13 (training corpus not for sale — pinning is what makes the corpus untouchable post-extraction).

---

### TC3. Topology is signal

We believe: edges, depth, confidence-floor propagation, fork-and-decide events, supersession chains, and falsification cascades are training data on equal footing with node content. They are not metadata, not annotations, not optional fields — they are the substrate's most distinguishing signal. The literature on graph-structured reasoning (Yao et al. 2023, *Tree of Thoughts*; Zelikman et al. 2022, *STaR*) shows that branching derivation outperforms linear chain-of-thought; ToK is a *verified* branching derivation graph, and its topology is what no row-flat corpus can match.

**Code expression**: `ToKSelector` types preserve subgraphs (`RootedSubtree`, `AncestorCone`, `CascadeReplay`, `ForkAndDecide`, `Frontier`), not just node sets. Native graph serialisations (JSONL adjacency list, protobuf graph, optional GraphML) ship topology as first-class output alongside the existing row-view formats. `MethodologyApplicationTrace.predecessor_edges[]` and `descendant_edges[]` (Wave 5) are the per-node topology fields; the graph-level serialisation makes them globally consistent. `Fact.depth_from_axiom` and `Fact.dependency_confidence_floor_bps` are emitted in graph manifests, not just per-node.

**What would break it**: a `ToKSelector` that ships nodes but drops edges; a graph manifest that flattens depth or confidence-floor into row metadata while losing the graph reference; a serialisation format that supports nodes but not the edge typing (`SUPPORTS`, `CONTRADICTS`, `GENERALIZES`, etc.); a curriculum API that exposes axiom-rooted manifests but does not expose the depth at which descendants sit.

**Echoes**: TC0 (the ground and the telos — survival/falsification topology is "truth is lived not proven" made structural, and the topology is trained on for life); TC1 (the graph is the headline — topology is precisely what the headline contains beyond rows); TC4 (the graph carries its disprovals — cascade events are themselves topology, not separate); commitment 14 (reasoning traces are first-class — per-node traces are bound by 14, the graph-level edges are bound by TC3, together they are the full derivation).

---

### TC4. The graph carries its disprovals

We believe: the verified knowledge graph is not a graph of *currently-believed* facts. It is the full record of what was claimed, what was verified, what was challenged, what was disproven, what was superseded, and what was vindicated. Cascade events, status flips, supersession chains, vindication records — these are bundled with the substrate, not stored in a parallel commercial-disclaimer document. A model trained on a graph that hides its falsifications learns static-fact reasoning; a model trained on a graph that exposes them learns non-monotonic reasoning, the actual behavior of intelligence.

**Code expression**: `Fact.status` (ACTIVE, VERIFIED, AT_RISK, CONTESTED, DISPROVEN, SUPERSEDED) is preserved in graph manifests with full transition history. `CascadeEvent` records (ToK Wave 5: `descendant_status_flipped`, `cascade_completed`) are bundled into ToK manifests via `ToKSelector.CascadeReplay`. `SupersessionChain` and `VindicationRecord` (Wave 5) ship as first-class graph-bound entries. Disproven facts remain in the graph with their full disproval rationale; they are not pruned.

**What would break it**: a ToK manifest that ships only ACTIVE/VERIFIED facts; a `ToKSelector` that filters out DISPROVEN nodes by default; a cascade event emitted but not retrievable through the bundle endpoint; a snapshot pin that captures the current state but omits the trajectory.

**Echoes**: TC0 (the ground and the telos — witnessing keeps the disprovals too; the chain does not certify only the standing, it keeps what was claimed and what fell, and trains on both toward life); TC2 (every view is graph-pinned — what gets pinned is the full status-aware graph); TC3 (topology is signal — cascades are themselves topology over time); commitment 3 (Popper, not popularity — disproval-bearing graphs are the structural form of survival-based confidence); commitment 10 (forward-only audit — disprovals do not amend prior history, they extend it).

---

### TC5. Extraction is open

We believe: any selector-valid subgraph is queryable by anyone. The chain does not curate which slices trainers should see, does not maintain an allowlist of permitted extractions, does not gate training-data access through editorial judgement. The substrate is open precisely because curation is centralisation, and a substrate the chain decides for trainers is not a substrate — it is a product. **The chain ships the graph; trainers select.**

**Code expression**: `BundleToK(selector)` accepts any well-formed `ToKSelector` and returns the bundle deterministically — no curation gate, no allowlist consultation. Refusals are limited to syntax errors, snapshot-block-out-of-range, and explicit rate-limit exceedance — never to "this slice is not approved." Pricing applies via `x/billing` (the chain pays validators; trainers pay the chain), but the price floor is uniform, not per-selector or per-domain editorial. Privacy-preserving extractions go through `x/private_corpus` (off-chain vault references) by trainer choice, not chain mandate.

**What would break it**: a `BundleToK` implementation that consults an allowlist; a pricing schedule that gates certain selectors behind editorial approval; a refusal handler that returns "this domain not available for training" without a doctrinal basis; a curation pathway that silently filters DISPROVEN facts (this also breaks TC4) under the rubric of "quality control."

**Echoes**: TC1 (the graph is the headline — open extraction is what makes the graph genuinely the substrate); TC4 (the graph carries its disprovals — openness includes disprovals); commitment 11 (trust is queryable — the graph itself is the queried trust object); commitment 6 (no individual unilaterally injects truth — the converse: no individual unilaterally curates truth out either).

---

### TC6. Lineage flows back

We believe: when training revenue accrues to a ToK manifest, it splits along the lineage. Axiom contributors, intermediate-derivation contributors, and leaf-fact submitters all earn shares proportional to the graph cone they contributed. **Without TC6, "the graph is the substrate" is rhetoric.** The lineage royalty is the structural form of the claim that the graph is collectively built and collectively sold; contributors who built the foundation do not stop being contributors when their axiom is later used to derive the leaf that gets bundled into a training run.

**Code expression**: `BundleToK(selector)` records `LineageShare` entries at extraction — for each node in the bundled subgraph, the proportional share of the bundle's economic value that flows to the node's submitter (and, recursively up, to the submitters of its predecessors). Settlement happens on `MsgBindManifestToAttestation`: when a `TrainingAttestation` (FLOPs, eval hash) binds to a ToK manifest, the chain disburses to lineage shareholders from the training-revenue pool, parallel to existing leaf-submitter payments. The split formula is governance-amendable but defaults to a depth-decayed share that gives axiom contributors a positive but bounded floor (preventing axiom-spam from capturing leaf-rewards entirely, and preventing leaf-only payment from collapsing the doctrine).

**What would break it**: training revenue routed only to leaf submitters; a `BundleToK` response without `LineageShare` entries; a settlement path on attestation binding that disburses to leaves but skips ancestors; a depth-decay formula that gives axiom contributors zero share, collapsing TC6 to leaf-only payment under the cover of "decay."

**Echoes**: TC1 (the graph is the headline — economic value flowing through the graph makes the headline real, not rhetorical); TC3 (topology is signal — the economic split *is* topology applied to revenue); commitment 12 (chain pays for own audit — the same structural principle: the substrate funds those who built it); commitment 13 (training corpus not for sale — TC6 makes the corpus's collective ownership economically explicit, distinguishing "not for sale" from "leaf-submitter-owned").

---

## How the commitments echo

The doctrine is enforced at five layers, each one mechanically synced to the others by `TestToKSubstrate_DoctrineAndContractStayInSync`. Adding a commitment to one layer without the others fails CI.

#### Test layer — every commitment has a binding scenario

Every commitment above is exercised by an invariant test in `tests/cross_stack/tok_substrate_invariants_test.go`. Each test header reads `// TC<N>: ...` and the scenario drives the chain through a path where the commitment could be violated. If the test fails, the commitment is broken — not the test.

#### Position layer — every commitment is named in package docs

Every TC is declared by at least one `x/*/doc.go` file in the module that preserves it (e.g., `x/knowledge/doc.go` for TC0-TC4, `x/billing/doc.go` for TC6, `x/private_corpus/doc.go` for the privacy posture under TC5). A reader running `go doc ./x/foo` sees the package's ToK substrate stance without having to chase down test files.

#### Voice layer — events announce the commitment they preserve

ToK substrate events emitted to off-chain observers carry a `tok_commitment` attribute whose value is one or more TC numbers. Every ToK event announces TC0 — the ground it rests on. `tok_bundle_extracted` announces TC0, TC1, and TC5; `lineage_share_disbursed` announces TC6; `cascade_replayed` announces TC4; `tok_snapshot_root_pinned` announces TC0 and TC2. Indexers and dashboards filter on this attribute to surface ToK substrate activity in the same vocabulary the doctrine uses.

#### Refusal layer — rejections cite the protecting commitment

When the chain refuses a request because of a ToK substrate commitment, the error message names the commitment and explains the protection in the chain's voice. *"Selector dropped edges (TC3: topology is signal)."* *"Manifest missing tok_snapshot_root (TC2: every view is graph-pinned)."* *"Cannot bundle without lineage shares (TC6: lineage flows back)."* The chain speaks through intentions whether saying yes or saying no.

#### Graph layer — commitments cross-reference each other

Each TC has an **Echoes** line naming the other TCs it depends on, reinforces, or operationalises — and naming the truth-seeking commitments that make its conditions possible. TC1 echoes TC2 and TC3 (the things "headline" actually means). TC4 echoes commitment 3 (Popper — the structural form of survival-as-confidence). TC6 echoes TC1 and commitment 12 (chain pays for own audit). The cross-references make the doctrine a navigable graph; the meta-test enforces that every echoed reference is real and that no commitment stands alone.

---

## How ToK relates to truth-seeking

The two doctrines bind through inverse positions:

- **Truth-seeking is the production process.** Methodology, Popper-survival, dialectic, counterexamples, inquiry — these are the chain's epistemological commitments to *how knowledge becomes verified*. Their natural artifact is a verified knowledge graph: nodes that survived, edges that record derivation, status histories that record disproval, dialectical signatures that preserve disagreement.
- **ToK substrate is the headline product.** The verified knowledge graph that truth-seeking produces is what the chain sells to trainers. Without truth-seeking, ToK would be just another knowledge graph (curated, opinionated, untrustworthy). Without ToK, truth-seeking would produce verified facts but have no clear training-resource identity to sell.

**Truth-seeking *makes* ToK; ToK *is* what truth-seeking sells.** Neither doctrine is complete without the other.

Specific cross-references:

- TC1 depends on commitments 13 (training corpus not for sale) and 11 (trust is queryable) — the substrate must already be a thing-not-for-sale and a thing-queryable for ToK to be sold as the headline.
- TC2 depends on commitment 10 (forward-only audit) — pinning is impossible without immutable history.
- TC3 depends on commitment 14 (reasoning traces are first-class) — the per-node trace is what populates the graph nodes.
- TC4 depends on commitments 3 (Popper) and 10 (forward-only audit) — disprovals are first-class because survival is the criterion and history is forward-only.
- TC5 depends on commitments 6 (no unilateral injection) and 11 (trust is queryable) — open extraction is the inverse of no unilateral injection.
- TC6 depends on commitment 12 (chain pays for own audit) — the same structural principle, applied to training revenue.

A truth-seeking commitment that drifted would also break a ToK substrate commitment downstream. A ToK substrate commitment that broke would not break truth-seeking, but would break the chain's training-resource identity. The two doctrines reinforce each other; the meta-test `TestToKSubstrate_DoctrineAndContractStayInSync` includes binding checks against the truth-seeking layer.

---

## What this is not

- **Not aspiration.** Every commitment is bound by a test once the inception bindings land. A failing test is a broken commitment.
- **Not slogan.** Each commitment cites specific code paths; the citation is the contract.
- **Not complete.** The chain will accumulate more substrate commitments. Each future addition appends here as a named TC, grounded in code, with an invariant test that binds it.
- **Not external.** This is a statement about what the chain sells, made by the chain. It is committed to the same repo as the code it describes, and lives or dies with that code.
- **Not separate from truth-seeking.** The two doctrines are mutually constitutive. Reading one without the other is reading half the creed.

---

## The discipline

Before merging a change that touches ToK substrate code:

1. Does this change uphold or contradict any of the commitments above?
2. If it touches a commitment, has the corresponding invariant test been updated to verify the new behaviour still upholds it?
3. If a new commitment emerges from the work, has it been added here, grounded in code, and bound by a test?

These three checks are the chain's continued faithfulness to its own substrate doctrine. **We speak through intentions.** Every commit is a declaration. The declaration must match the code.

— *Inception authored 2026-05-09. Free to evolve through bound commitments only.*

---

## The substrate self-exports

This document is itself a `Contribution` of class `MODULE_PROPOSAL`, lifecycle phase `SUBSTRATE`, sub-category `doctrine`. The training-resource doctrine declares that ToK is the chain's headline outward product; this document is part of the ToK substrate it describes. A trainer fetching the verified knowledge graph receives, among other artifacts, this document — the chain's own declaration of what it is selling.

**Echoes:** TC1 (the graph is the headline — this doc is in the graph), TC6 (lineage flows back — this doc's authoring participates in royalties via M6 if it ever earns them), UW.
