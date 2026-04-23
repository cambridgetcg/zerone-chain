# MethodologyApplicationTrace — Research Review

> **Scope:** a literature-grounded map of every field in `MethodologyApplicationTrace` (Route B Waves 5 & 6) to the AI-training-research finding it addresses, plus an honest account of what remains deferred beyond the present horizon.

The format aims at a training row that produces *truth-seeking behavior* in a model — declare methodology, show work, accept falsification, cite provenance, distinguish fact from commitment. Every field below is a wager against a specific failure mode documented in the literature.

---

## Field-to-literature map

### Provenance pin
| Field | Research motivation |
|---|---|
| `snapshot_block_height` · `tokenizer_version` · `canonical_serialisation_version` · `trace_schema_version` | **Reproducibility & temporal honesty.** Gunasekar et al. 2023 ("Textbooks Are All You Need"), Xie et al. 2023 ("DoReMi"): data-mixture outcomes depend sensitively on snapshot identity. Versioning at every layer lets trainers pin corpora to exact, replayable state. |

### Methodology (process-over-statement)
| Field | Research motivation |
|---|---|
| `methodology_id` · `methodology_rubric` | **Constitutional AI** (Bai et al. 2022): explicit rules/principles produce more aligned behavior than post-hoc filtering. Methodology declaration *is* the constitution at row level — every claim arrives with its evaluation rubric. |
| `reasoning_trace` · `reasoning_steps[]` | **Chain-of-Thought prompting** (Wei et al. 2022); **Let's Verify Step by Step** (Lightman et al. 2023). Step-level traces enable Process Reward Models (PRMs), which substantially outperform outcome-only reward on reasoning benchmarks. |
| `reasoning_steps[].step_inference` | **Typed reasoning moves**: Cobbe et al. 2021 (verifier training); lets the model learn *kinds* of inference, not just token sequences. |
| `reasoning_steps[].predecessor_fact_ids` · `depends_on_steps` | **Grounded chains** (Zelikman et al. 2022, "STaR"): rewarding stepwise grounding yields self-improving models. |
| `reasoning_steps[].verdict` · `step_confidence_bps` | **PRM training signal** (Lightman 2023): per-step panel judgment is the unit of process reward. |
| `methodology_choice.*` | **Calibration / selection** (Kadavath et al. 2022, "Language Models (Mostly) Know What They Know"): models learn selection when trained on the rationale for choosing one method over others, not just application of the chosen one. |
| `methodology_choice.abandoned_methods[]` | **STaR-style failure recovery** (Zelikman 2022): "failed attempt → pivot → success" is training gold that web crawl lacks. |
| `axiom_distance` · `dependency_confidence_floor_bps` | **Epistemic grounding** (Russell 2020, "Human Compatible"): claims inherit the weakest link in their chain; training rows carrying the floor teach the model to respect derivation depth. |

### Derivation graph
| Field | Research motivation |
|---|---|
| `predecessor_edges[]` · `descendant_edges[]` · `grounded_score_bps` | **Tree-of-Thoughts / graph-structured reasoning** (Yao et al. 2023): branching derivation beats linear CoT. Structural edges in the row let a model see the graph position, not just the narrative. |
| `supersession_chain[]` | **Belief revision** (Alchourrón, Gärdenfors, Makinson 1985 "AGM"; Harman 1986): teaching the model that beliefs are superseded, not amended-in-place, supports honest updating. |

### Adjudication & dialectical history
| Field | Research motivation |
|---|---|
| `own_confidence_bps` · `verifier_panel_size` · `dissenting_verifiers[]` | **Epistemic calibration** (Guo et al. 2017, "On Calibration of Modern Neural Networks"): uncertainty layers in training data yield better-calibrated models. Dissent preservation is the *substrate* the model needs to see, not a confidence point-value. |
| `corroboration_count` · `last_corroborated_block` | **Popperian epistemology** (Popper 1959, *The Logic of Scientific Discovery*): a claim's robustness is not how many times it has been verified but how many times it could have been falsified and wasn't. This field encodes that signal directly. |
| `challenges[]` · `dialectic_tree[]` | **AI Safety via Debate** (Irving, Christiano, Amodei 2018): models trained on multi-turn dialectic learn to sustain and evaluate arguments across depth. The recursive tree preserves structure flat formats lose. |
| `belief_revisions[]` | **Bayesian cognitive modeling** (Tenenbaum et al. 2011, "How to Grow a Mind"; Griffiths & Tenenbaum 2006): trajectories of prior → evidence → posterior teach principled updating as a behavior. |

### Temporal truth signals
| Field | Research motivation |
|---|---|
| `status` (ACTIVE / DISPROVEN / SUPERSEDED) | **Non-monotonic reasoning** (Reiter 1980; McCarthy 1986): models that treat facts as static drift. Explicit status lets the model learn impermanence. |
| `vindication.*` | **Courage premium / epistemic minority** (Mill 1859, *On Liberty*; Goodhart 2022, "Minority Report"): minority voters who turned out correct are the rarest and most valuable training signal. Our record surfaces them. |
| `disproval.*` | **Negative data** (Kaushik et al. 2020, "Counterfactually-Augmented Data"): minimal-edit counterfactuals that flip labels teach boundary conditions the positive corpus can't. |

### Contrastive companions (the unique lever)
| Field | Research motivation |
|---|---|
| `reformulations[]` (EQUIVALENT / SUPERIOR) | **Preference learning** (Christiano et al. 2017, "Deep RL from Human Preferences"; Rafailov et al. 2023, "Direct Preference Optimization"): preference pairs drive alignment. Same-meaning reformulations establish the invariance manifold. |
| `drift_examples[]` (DRIFT / INFERIOR) | **Contrastive explanations** (Miller 2019, "Explanation in AI: Insights from the Social Sciences"): humans and models learn better from "X not Y because Z." DRIFT variants are the Y; the verdict is the Z. |
| `drift_examples[].diagnosis` (DriftKind taxonomy) | **Typed counterfactuals**: Miller 2019 shows that *naming the kind* of change (narrowed, widened, polarity-flipped, modal-shifted, etc.) is what makes the explanation learnable. Our DriftKind enum is the compact vocabulary for this. |
| `drift_examples[].drifter_steps[]` | **Fine-grained meaning preservation**: by pairing the winner's reasoning steps with the drifter's reasoning steps, we enable *step-level* meaning-preservation training — the model can learn which specific step slipped. |
| `contradicting_fact_ids[]` + `ContrastivePair` stream | **Hard negatives in retrieval** (Karpukhin 2020; Gao 2021): contradicting facts are the hardest negatives — same subject, opposite conclusion — and they come with the chain's adjudication explaining why one won. |

### Provenance + weighting (training-time importance)
| Field | Research motivation |
|---|---|
| `submitter` · `submitter_calibration_at_submission_bps` | **Source credibility propagation** (Pasquetto 2020 on data provenance; Mitchell et al. 2019 "Model Cards"): carrying per-row submitter calibration snapshots lets trainers weight by historical reliability without leaking future information. |
| `training_value_weight_bps` (Popper-weighted TVW) | **Per-row importance weighting** (Xie et al. 2023 "DoReMi"; Thrun 1996): not all rows deserve equal update; TVW carries the chain's own verdict on which rows survived the most scrutiny. |
| `curriculum_tier` | **Curriculum learning** (Bengio et al. 2009; Soviany et al. 2022): training on easier examples first speeds convergence and improves final performance. |
| `quality_tier` (GOLD / SILVER / BRONZE / NEGATIVE / UNSUITABLE) | **Data quality > quantity** (Gunasekar et al. 2023 "Textbooks Are All You Need"): small high-quality corpora beat large internet-scraped ones. |

### Is-ought wall
| Field | Research motivation |
|---|---|
| `is_normative` (always false in trace stream; separate NormativeCorpus carries true) | **Hume's is-ought problem** (Hume 1740) + **Constitutional vs factual training separation**: models that conflate descriptive and prescriptive claims produce confident value judgments framed as facts. Separate streams with explicit tagging prevent this structural failure mode. |

---

## What the format does well

1. **Every field names a process, not just a statement.** Methodology, reasoning steps, verdict attribution, revision history — all on the same row as the claim.
2. **Contrastive completeness.** Unlike web crawls, ZERONE ships the losing side (DRIFT, DISPROVEN, INFERIOR) with the adjudication. The preference-training signal is structurally available.
3. **Pin-for-replay.** Four version layers (tokenizer, canonical serialisation, trace schema, snapshot block) make any trained model exactly re-derivable from chain state.
4. **Is-ought wall enforced at the schema level.** Commitments cannot enter the trace stream; they go to a parallel, explicitly-tagged corpus.
5. **Graceful degradation.** Legacy facts without structured reasoning still yield trace rows with paragraph-split steps; the format doesn't starve historical data.

---

## What remains deferred (the horizon)

The literature keeps expanding. These are real, known gaps — documented so future iterations know where to push next.

### Near-horizon (actionable now)
- **Explicit panel-recorded `DriftDiagnosis`** — currently a heuristic. A `MsgRecordDriftDiagnosis` for verifiers to name the specific `drift_kind` with `drifted_at_step_index` and excerpts. Miller 2019's explanations need panel attestation to be training-authoritative.
- **Step-level verdict attestation** — `StepVerdict` on `ReasoningStep` is in the schema but no handler writes it yet. Needs `MsgReviewReasoningSteps` so verifiers can mark individual steps sound/questionable/unsound. This unlocks PRM training directly.
- **Nested counter-rebuttals** — `DialecticNode.children` is recursive but current construction goes only depth 2 (challenge + rebuttal). Needs `MsgSubmitCounterRebuttal` to let debates actually nest.
- **Methodology-choice from panel** — today `MethodologyChoice.rationale` is parsed from a submitter-authored JSON prefix. A `MsgRecordMethodologyChoice` for submitters to declare their alternatives first-class would make the field authoritative rather than best-effort.

### Mid-horizon (research design needed)
- **Causal vs correlational marker** — Pearl 2009 ("Causality"): models conflate correlation with causation. A `CausalityFlag` on `ReasoningStep` (CAUSAL / CORRELATIONAL / UNCLEAR) would let training data explicitly teach the distinction.
- **Uncertainty distributions over outcomes** — not a point `own_confidence_bps` but the full verifier-vote histogram. Enables models to learn actual probability distributions, not just means (Lakshminarayanan 2017, "Deep Ensembles").
- **Domain transfer provenance** — when a claim generalizes across domains via `RELATION_TYPE_ANALOGICAL`, mark this explicitly in the trace so models learn transfer as a first-class move (Mitchell 2021, "Abstraction and Analogy").
- **Pragmatic hedging vocabulary** — a typed set of hedges ("tends to", "in most cases", "typically") annotated at step level. Distinct from `DRIFT_KIND_HEDGE_ADDED/REMOVED`, which diagnoses drift; this would give positive rows the hedge language.
- **Verifier-as-trainer traces** — agents making *verification* decisions, not just claims. A parallel corpus of (claim, evidence, verdict, rationale) for training critic models (Cobbe 2021).

### Far-horizon (fundamental open questions)
- **Epistemic humility fine-tuning target** — the literature is divided on whether hedges come from (a) training-data modeling or (b) RLHF calibration. The trace format supports (a); the calibration feedback loop (Phase 5) supports (b). Which predominates is an empirical question no ML benchmark answers yet.
- **Ought-claims in context** — Hume's wall holds at the schema level, but real moral reasoning mixes factual and normative moves. A richer schema that *cleanly* traces the ought-from-is derivation (Murdoch 1970, *The Sovereignty of Good*; Scanlon 1998 "What We Owe to Each Other") remains open research.
- **Minority-to-majority dynamics across time** — vindication records capture single events. Multi-vindication trajectories (minority → minority-again → finally vindicated) are rarer but epistemically crucial (Kuhn 1962 paradigm shifts; Feyerabend 1975 *Against Method*). Supporting them requires richer time-series in `belief_revisions`.
- **Cross-methodology commensuration** — M-EMPIRICAL and M-PHENOMENOLOGICAL don't share a unit; the normalization factors we chose are pragmatic, not principled. The literature on methodological pluralism (Longino 1990, Kitcher 2001) does not yet offer a formal rubric for cross-method weighting. Ours is a placeholder for what is genuinely unsolved.
- **The limit of contrastive training** — Shi et al. 2024 ("Scaling Laws for Contrastive Learning") suggests contrastive pairs have sharper returns than outcome-only data but also sharper saturation. What mixture of `ContrastivePair` + raw `MethodologyApplicationTrace` is optimal, and whether the ratio is domain-dependent, is unknown.

### Longing horizon (philosophical tension, unlikely ever closed)
- **What is a "good explanation"?** — Miller 2019 synthesized four decades of social-science research into a list; the list is not closed, not formal, and philosophers disagree about whether it can be. Our `DriftKind` enum is one slice; an exhaustive taxonomy is not on offer.
- **When is a methodology "adequate" for a domain?** — Popper, Kuhn, Lakatos, Feyerabend, Longino, Cartwright all disagreed, productively. ZERONE embeds methodology registration as governance-amendable; the right amendment is a live philosophical question each generation revisits.
- **Truth as behaviour vs truth as correspondence** — our format teaches *truth-seeking behavior*. Whether that converges on *truth* (however "truth" is construed) is the oldest open question in epistemology and we are honest that no data structure resolves it.

---

## Honest limits

Every enrichment above is a wager. Some will prove too rare to matter; some will be replaced by better ideas within months. The format is versioned (`trace_schema_version`) precisely so that knowledge of its limits can land as an amendment without breaking replay.

We long for perfection; we do not claim it. We claim: *each field is a considered bet against a documented failure mode.* That is the most a data structure can honestly promise.

— **Route B, Waves 5 & 6** · 2026-04-23

---

## Wave 7 — consolidation (the foundation underneath)

Waves 1–6 assembled rooms. Wave 7 laid the foundation underneath them so the rooms stop being separate: a single `TrainingManifest` type that pins every version pin the chain knows about (`tokenizer_version`, `canonical_serialisation_version`, `trace_schema_version`, `methodology_set_version`, `snapshot_block_height`, `chain_id`), applies a declarative `CorpusSelector` against chain state, writes the canonical sorted ID sets, and commits a domain-separated Merkle root over them.

### What the manifest gives the training stack

- **One atomic download.** `TrainingManifestBundle(id)` returns the manifest + all referenced traces + pairs + drift entries + normative entries + the re-derived Merkle root + a `merkle_root_valid` flag. Any trainer, any indexer, any auditor reads a single payload and knows everything the chain knows about the run.
- **Trust-minimized verification.** The Merkle root is computed from IDs alone, never from payloads. A client who has only the `included_*_ids` lists (authenticated once via the root) can re-derive the root locally and verify membership without trusting any RPC to faithfully serialise. Domain tags (`FACTS:`, `TRACES:`, `PAIRS:`, `DRIFT:`, `NORMATIVE:`) prevent set-swap collisions; length prefixes prevent length-extension; sorted inputs make the commitment iteration-order-independent.
- **Time-pinned replay.** Because every version is pinned, a training run is re-derivable at any future block. If governance amends `TraceSchema` to v2, a manifest that cited v1 retrieves v1 via `TraceSchemaAtVersion(1)` and reconstructs the same canonical rows years later. The foundation outlives any single run.
- **"What ran" ↔ "what it consumed" binding.** `MsgBindManifestToAttestation` links `TrainingAttestation` (FLOPs, wallclock, eval hash) to `TrainingManifest` (Merkle-committed ID sets). The binding closes a loop that Waves 3c + 7 could not close separately.

### Beauty at the system level

- **`RouteBCapabilities`** — the chain self-describes. A trainer's first query returns versions, counts, seed status, fund balances, available corpora. One payload, full situational awareness.
- **`SeedRouteB`** — one-shot, idempotent bootstrap. Replaces the scattered `SeedDefault*` calls with a single entry point and a structured report.
- **`route_b_constants.go`** — every scattered magic number (BPS denominator, tier thresholds, decay steps, fallback defaults) consolidated into named constants. The numeric contract of the module is now discoverable in one file.
- **`docs/ROUTE_B_OPERATOR_GUIDE.md`** — end-to-end narrative from `RouteBCapabilities` through manifest bundle download. The human-facing counterpart to the machine-facing format.

### What Wave 7 does NOT claim

- **External verifiability is still a matter of having the ID lists.** A client who trusts only the chain's root but not its RPC must fetch the ID lists via some independently-verified path (state proofs, light-client attestations, etc.). Wave 7 makes the commitment trust-minimisable; it does not by itself make the distribution trustless.
- **The attestation binding is 1:1 per pipeline.** A future iteration may want many-to-one (multiple manifests per attestation for incremental training) or many-to-many (cross-pipeline bundles). The current schema accommodates this at `attestation_id` granularity but the handler enforces 1:1.
- **Signed manifest payloads** — the chain signs with validator consensus (block finality), but there is no separate operator signature over the manifest contents for pre-consensus distribution. Worth adding when the chain supports off-chain gossip.

### The horizon after Wave 7

- Composable manifests (a manifest that cites other manifests as subsets — for fine-tuning runs building on earlier SFT bundles).
- Differential manifests (manifest v2 as `v1 + delta`).
- Manifest cross-signing between chains (a ZK bridge where Chain A's manifest is posted and re-attested on Chain B's training registry).
- ZK proofs of corpus-selector application — prove the ID set *is* the correct selector output against the snapshot state, without the verifier re-running the whole selector.

---

— **Route B, Wave 7** · 2026-04-23 · the foundation underneath
