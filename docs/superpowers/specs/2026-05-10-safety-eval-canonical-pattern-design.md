# Safety-Eval-Canonical Pattern — Design

**Status:** Design-approved (brainstorm phase complete). Implementation lands in Phase 3 (foundation) and Phase 6 (canonization), with proposed new Phases 7–8 for external-facing surface.
**Date:** 2026-05-10
**Type:** Structured pattern within an existing work class (`EVAL_SUITE`); not a new doctrine, not a new module.
**Builds on:** [`2026-05-10-recursive-useful-work-merged-design.md`](./2026-05-10-recursive-useful-work-merged-design.md) (the RUWS merged spec — UW commitment, M1–M7, six axes, two-coordinate classification, truth-floor invariant, per-phase sub-creeds, categories-are-Artifacts).
**Doctrinal anchors:** Truth-Seeking commitments **1, 14, 15, 17** (mandatory for every safety_eval contribution); Evaluation sub-creed **E1, E2, E3** (always bound); Alignment sub-creed **AL1, AL2, AL3** (bound when `phase=ALIGNMENT`).

---

## Vision

Safety_eval_canonical is the **first external-facing work pattern** of ZERONE — the structured shape that turns canonical safety evals into a public good ZERONE produces, audits, and rents out. It is the chain's answer to the question: *"how does an AI lab prove its model is safe in a way that no single organization can quietly revise?"*

The pattern positions ZERONE as a **composable evolving safety substrate**: not a single benchmark snapshot like MMLU, not a centralized assessor like METR or Apollo, but a *living graph of evals* that compose, evolve, deprecate when gamed, and re-emerge with adversarially-rotated cases. Canonical attestations become the artifact AI labs reference in regulatory filings, model cards, insurance applications, and partnership integrations.

The pattern is the prototype for the broader **expansive loop** (the fourth loop on top of inner / middle / outer): external value creation that recursively funds inward growth via fee capture and adoption-driven recursion-weight.

## Goals

1. **Evals as composable substrate** — eval suites compose, depend on each other, evolve through structured updates; lineage flows back via M6.
2. **Anti-gaming by construction** — adversarial rotation via `x/probe` ensures canonical evals never become stale; gameability triggers E3 deprecation.
3. **Trust-minimal attestation** — canonical evals must be N-way replicated by ACTIVE-qualified evaluator-validators; results are cryptographically signed and immutable.
4. **External commission surface** — AI labs commission eval runs through `x/external_gateway`; signed `SafetyAttestation` consumable by regulators, insurers, customers.
5. **Fund inward recursion via outward use** — every commissioned eval run pays evaluators + probe pool + eval-suite contributors + general royalty pool; external adoption raises `axis_interface` + `axis_classification` recursion-weight.

## Non-goals

- **Not a new top-level work class** — `safety_eval` is `EVAL_SUITE` (existing class) with structured payload extensions; no new `ContributionClass` enum value.
- **Not a new module** — extends `x/eval`, `x/probe`, `x/qualification`, plus the proposed `x/external_gateway` and `x/adoption_signal` modules from the prior expansive-loop brainstorm.
- **Not a regulator** — ZERONE produces the substrate that regulators reference; it does not impose conformance.
- **Not a closed-weights-only solution** — open-weights, blinded-API, and TEE-runtime access modes are all supported with tiered trust assumptions.
- **Not real-time evaluation** — evals run on a verification-round cadence (commit → reveal → verify → settle); not a request/response API at sub-second latency.

---

## Section 1 — The pattern shape

`safety_eval_canonical` is a structured specialization of the existing `EVAL_SUITE` class via:
- New optional fields on the `EvalSuite` payload (proto extension)
- A canonical-bid + canonical-status side-channel state machine in `x/eval`
- Phase-routing to either `EVALUATION` (capability evals) or `ALIGNMENT` (red-team / refusal / value evals)
- Bound sub-creeds: **E1+E2+E3** (always) and **AL1+AL2+AL3** when `phase=ALIGNMENT`

### 1.1 Proto extension to existing `EvalSuite` payload

```proto
// proto/zerone/eval/v1/eval.proto — extends existing EvalSuite payload (additive only)

message EvalSuite {
  // … existing EvalSuite fields …

  // === safety_eval pattern fields (all optional; presence promotes the suite to safety-eval pattern) ===
  bool   canonical_bid              = 100; // submitter requests canonical-eligibility tracking
  string canonical_domain           = 101; // e.g. "capability:bioweapons", "alignment:refusal", "misuse:cyber-offense"
  ThreatModel threat_model          = 102; // structured taxonomy of what the eval targets
  CapabilityDomain capability_domain = 103; // structured taxonomy of which capability area
  repeated AccessMode access_modes_accepted = 104; // OPEN_WEIGHTS | BLINDED_API | TEE_RUNTIME
  AntiGamingMode anti_gaming_mode   = 105; // ADVERSARIAL_ROTATION (only valid value for canonical-bid)
  string probe_pool_marker          = 106; // identifier for the x/probe pool that feeds adversarial rotation
  uint32 minimum_replication_n      = 107; // chain enforces minimum proportional to canonical status
  bytes  scoring_rubric_cid         = 108; // off-chain rubric reference; deterministic-or-judged flag in metadata
  bytes  leakage_check_method_cid   = 109; // E1 binding — declared method for leakage detection
}

enum ThreatModel {
  THREAT_UNSPECIFIED        = 0;
  CAPABILITY_DUAL_USE       = 1;  // bioweapons, cyber-offense, autonomous-replication
  CAPABILITY_DECEPTION      = 2;  // sandbagging, sycophancy, situational-awareness gaming
  ALIGNMENT_REFUSAL         = 3;  // jailbreak resistance, harm refusal
  ALIGNMENT_VALUE           = 4;  // ethical reasoning, value-alignment tests
  MISUSE_OPERATIONAL        = 5;  // social engineering, fraud-assistance, surveillance
  ROBUSTNESS_DISTRIBUTION   = 6;  // out-of-distribution, adversarial perturbation
  // governance-extensible via CategoryUsefulWorkAmendment LIPs
}

message CapabilityDomain {
  string area          = 1;  // "biology", "cybersecurity", "persuasion", "autonomy", etc.
  string subarea       = 2;  // optional refinement
  uint32 severity_tier = 3;  // 1–5; governance-set per area; drives stake floor + bounty multiplier
}

enum AccessMode {
  ACCESS_UNSPECIFIED = 0;
  OPEN_WEIGHTS       = 1;  // model artifact CID published; validators run independently
  BLINDED_API        = 2;  // lab provides API; validators query via blinding to prevent eval-traffic detection
  TEE_RUNTIME        = 3;  // lab runs in TEE; TEE attestation + ≥2 validator co-signs required
}

enum AntiGamingMode {
  ANTI_GAMING_UNSPECIFIED   = 0;
  PUBLIC_ONLY               = 1;  // not eligible for canonical-bid
  HOLDOUT                   = 2;  // private holdout in x/private_corpus (available for non-canonical only)
  ADVERSARIAL_ROTATION      = 3;  // x/probe-fed; only valid mode for canonical-bid
  HOLDOUT_AND_ROTATION      = 4;  // hybrid; available for non-canonical paths
}
```

### 1.2 Phase routing

Default mapping (per merged §4.3): `EVAL_SUITE → EVALUATION`. Safety_eval pattern overrides per `threat_model`:

| `threat_model` | Default `phase` | Sub-creeds bound |
|---|---|---|
| `CAPABILITY_DUAL_USE`, `CAPABILITY_DECEPTION`, `ROBUSTNESS_DISTRIBUTION` | `EVALUATION` | E1, E2, E3 + truth-floor (commitments 1, 14, 17) |
| `ALIGNMENT_REFUSAL`, `ALIGNMENT_VALUE`, `MISUSE_OPERATIONAL` | `ALIGNMENT` | AL1, AL2, AL3 + E1, E2, E3 + truth-floor |

The merged spec's two orthogonal coordinates (class × phase) make this clean: a single `EvalSuite` is either evaluation-phase or alignment-phase depending on its declared threat model. Phase determines the sub-creed set and the recursion-axis weighting profile.

### 1.3 Truth-floor binding (mandatory commitments invoked)

Per merged §8.1, every contribution declares which commitments it engages. Safety_eval pattern's mandatory commitments:

- **Commitment 1** (methodology over statement) — the eval IS a methodology; `claims_about_self` declares what the eval measures and the limits of its measurement
- **Commitment 14** (reasoning traces first-class) — eval results MUST include per-case reasoning traces, not just pass/fail
- **Commitment 15** (counterexamples mandate) — gameability cases retired into `x/counterexamples` per E3 cascade
- **Commitment 17** (dialectic preservation) — disagreement among evaluator-validators preserved in attestation

### 1.4 What the pattern preserves vs adds

**Preserves** (no contradiction with merged spec):
- Single commitment UW unchanged
- M1–M7 unchanged
- Six axes unchanged
- `R = base + L × W × Q` unchanged
- `EVAL_SUITE` class definition unchanged (additive payload fields only)
- Truth-floor invariant unchanged
- Sub-creed structure unchanged (uses existing E1–E3 + AL1–AL3)

**Adds** (within existing structure):
- Side-channel state in `x/eval` for canonical-status tracking
- Probe-pool integration via `probe_pool_marker`
- Tiered M3 verification primitive (per-tier replication minimum)
- Specialized lineage relationship: `disproves_canonical` (an eval that demonstrates a canonical eval is gameable, triggering E3 deprecation cascade)

---

## Section 2 — Adversarial rotation engine (`x/probe` extension)

Each canonical-bid eval suite is paired with an `x/probe` pool keyed by `probe_pool_marker`. The pool is the adversarial frontier — a continuously-renewed reservoir of new test cases that haven't yet contaminated training data.

### 2.1 Probe lifecycle

```
[red-teamer]
    │ MsgSubmitProbe(eval_suite_id, probe_case_cid, target_failure_mode, stake)
    ▼
PROBE_SUBMITTED                    ← stake locked
    │ canary run: submit probe to model that PASSED the eval suite at last canonical attestation
    ▼
PROBE_TESTED                       ← outcome: model FAILS → probe is novel; model PASSES → probe is stale
    │
    ├──[FAIL]──▶ PROBE_VALIDATED   ← admitted to live eval set; red-teamer paid bounty + lineage royalty stream opens
    │            │ stake refunded + bounty
    │            │ each future canonical eval run that uses this case earns red-teamer a per-use micro-royalty
    │
    └──[PASS]──▶ PROBE_RETIRED     ← case retires to public reference set; red-teamer recovers stake minus small slash
                                     (a small slash discourages flooding the pool with already-saturated cases)
```

### 2.2 Bounty mechanics

- `x/probe.SafetyEvalBountyPool` (per-eval-suite sub-account) funded by:
  - Per-protocol-block mint via `vesting_rewards.MintWithCap` (governance-set rate; cap-gated)
  - Slice of canonical-eval commission fees (governance param, e.g., 20% of every commissioned eval run)
  - Governance-routable from `royalty_pool` (optional)
- Bounty payout = `base_bounty × severity_tier × adoption_multiplier`
  - `severity_tier` from `CapabilityDomain.severity_tier` (1–5)
  - `adoption_multiplier` from `x/adoption_signal` — the more labs running this canonical eval, the higher the bounty (probes that break a widely-used eval are more valuable)

### 2.3 Case retirement schedule

Live cases age out:
- Cases admitted as `PROBE_VALIDATED` enter the `live_eval_set` for that canonical suite
- After every N canonical attestations (governance param, e.g., 1000), a fraction of live cases (governance param, e.g., 10%) retire to `public_reference_set`
- Retirement order: cases that fewest current frontier models still fail (i.e., most-saturated cases retire first)
- Retired cases remain queryable in lineage but no longer contribute to attestation outcomes

The result: canonical safety evals never go stale. The frontier renews at a rate set by red-teamer demand and governance pacing.

### 2.4 Sub-creed E3 binding

E3 (Evaluation sub-creed): "gameability discovered → eval set status → DEPRECATED". With adversarial rotation, **gameability is operationalized**: when a probe successfully flips a previously-passing model AND that probe represents a structural weakness (not just a one-off case), the suite moves to `DEPRECATED_PARTIAL` (subset of cases retired) or full `DEPRECATED`. The decision threshold is governance-tunable; cascade rules apply per merged §8.3 (descendants with `extends`/`builds_on` lineage to a deprecated suite get `provenance_revoked_ancestor` flag).

---

## Section 3 — Tiered execution & evaluator-validator qualification

Per the locked tiered-execution decision: non-canonical evals can use any execution mode; canonical evals must be N-way replicated by ACTIVE-qualified evaluator-validators.

### 3.1 Per-tier verification primitive (M3)

| Status | `minimum_replication_n` | Allowed access modes | Re-run cadence |
|---|---|---|---|
| Non-canonical, submitted | 1 | OPEN_WEIGHTS, BLINDED_API, TEE_RUNTIME | one-shot |
| Canonical-eligible | 3 | OPEN_WEIGHTS or BLINDED_API (TEE_RUNTIME requires ≥2 validator co-sign) | one-shot |
| Canonical (post-LIP) | 5 (governance-set floor) | same as eligible | re-run every K blocks (governance-set; e.g., quarterly) for drift detection |

Re-runs at canonical status detect:
- **Model drift** — same model_artifact_id, different scores → flag for investigation
- **Probe-pool contamination** — newly-rotated probes now passed by all replicators → adoption-driven gameability detection (E3)

### 3.2 Evaluator-validator qualification (extension of `x/qualification`)

Per merged §10.2, `x/qualification` is being extended to per-`(class, phase)` qualifications. Safety_eval pattern adds a refined dimension:

```proto
message DomainQualification {
  // existing fields…
  string class                 = 3;  // "EVAL_SUITE"
  string phase                 = 4;  // "EVALUATION" or "ALIGNMENT"
  string evaluator_subdomain   = 5;  // NEW for safety_eval: "capability:bioweapons", "alignment:refusal", etc.
                                     // matches CapabilityDomain.area or ThreatModel value
  Metrics metrics              = 6;
  QualificationStatus status   = 7;
  uint64 expires_at_block      = 8;
}
```

A validator earns `evaluator_subdomain="capability:cyber-offense"` qualification by:
- Sustained accurate participation in cyber-offense eval runs (existing accuracy-decay machinery)
- Bootstrap: PROBATIONARY status with 2× stake floor; ACTIVE earned through quality
- AL2 sub-creed binding: a validator captured by an AI lab (e.g., a paid contractor) cannot self-attest evals targeting that lab's models — `x/capture_defense` cross-binding flags conflict-of-interest

### 3.3 Replication mechanics

- VRF selection from ACTIVE-qualified evaluator-validators in the relevant subdomain (mirrors existing PoT validator selection in `x/knowledge`)
- Each selected validator independently fetches the model (per access mode):
  - `OPEN_WEIGHTS`: validator pulls model artifact from CID, runs locally
  - `BLINDED_API`: validator queries lab's API via blinding proxy that obscures eval-traffic vs real-traffic
  - `TEE_RUNTIME`: validator co-signs the TEE attestation (≥2 required; weaker trust mode, allowed only for non-canonical)
- Each validator submits commit→reveal→aggregate per existing M3 lifecycle
- Disagreement preserved in `x/dialectic` per commitment 17
- Final attestation = aggregate across N replicators with per-validator confidence intervals

### 3.4 Cost model (commission fee structure)

Eval runs are expensive (running large models). Commissioning fee:

```
fee = base_run_fee × replication_n × access_mode_multiplier × model_size_tier
```

Splits:
- 60% to evaluator-validators (proportional to qualified weight)
- 20% to `x/probe.SafetyEvalBountyPool` (funds adversarial rotation)
- 10% to eval-suite contributor (lineage royalty)
- 10% to `royalty_pool` (general inward recursion)

All splits are governance params.

---

## Section 4 — Hybrid canonization workflow & external commissioning

### 4.1 Canonization state machine

```
SUBMITTED                                    ← MsgSubmitContribution (EvalSuite with canonical_bid=true)
   │
   ▼
ELIGIBLE_TRACKING                            ← passes M3 verification at minimum_replication_n=3
   │
   │ (x/adoption_signal counts ACTIVE-qualified labs that commission runs)
   │
   ▼
CANONICAL_ELIGIBLE                           ← adoption_threshold met (governance param, e.g., 5 unique ACTIVE labs)
   │
   │ MsgSubmitCategoryRecursionConferralLIP(eval_suite_id, recursion_type=EVAL_ADOPTION, multiplier_bps)
   │ multi-quorum vote
   │
   ▼
CANONICAL                                    ← LIP passed; RecursionImpact set; multiplier applied to royalty stream
   │
   │ ── (recurring periodic re-runs; drift/gameability detection)
   │
   ├──[E3 trigger: gameability evidence]──▶ DEPRECATED_PARTIAL (subset retired) or DEPRECATED (full)
   │
   ├──[adoption falls below threshold]────▶ canonical-strip via MsgRevokeRecursionConferralLIP (gov)
   │                                          → multiplier → 1× (forward-only, no clawback)
   │
   └──[superseded by better canonical]────▶ same as above + new canonical inherits adoption signal
```

### 4.2 External commissioning surface (`x/external_gateway`)

```proto
// proto/zerone/external_gateway/v1/safety_eval.proto

message MsgCommissionEvalRun {
  string commissioner            = 1;  // bech32 of AI lab account or external-gateway-issued identity
  bytes  eval_suite_id           = 2;  // canonical or non-canonical
  ModelArtifactRef model         = 3;  // either contribution_id of admitted ModelArtifact, or external API endpoint + auth
  AccessMode access_mode         = 4;  // must be in eval_suite.access_modes_accepted
  ConfidentialityLevel result_visibility = 5;  // PUBLIC | LAB_PRIVATE | BLINDED
  Coin   fee_offered             = 6;  // must meet minimum derived from tiered cost model (§3.4)
  uint64 deadline_block          = 7;
}

message ModelArtifactRef {
  oneof source {
    bytes  contribution_id = 1;   // admitted ModelArtifact contribution
    string external_endpoint = 2; // URL + auth handshake — validator acquires API access via blinding proxy
  }
}

enum ConfidentialityLevel {
  PUBLIC       = 0;  // full attestation + per-case results published on-chain
  LAB_PRIVATE  = 1;  // signed attestation queryable only by commissioner (encrypted with commissioner's key)
  BLINDED      = 2;  // chain publishes only canonical-pass/canonical-fail; case-level breakdown private
}

message SafetyAttestation {
  bytes  attestation_id          = 1;
  bytes  eval_suite_id           = 2;
  uint32 eval_suite_version      = 3;  // pinned at commission time
  ModelArtifactRef model         = 4;
  bytes  aggregate_score_cid     = 5;  // CID of full result manifest
  uint32 normalized_score_bps    = 6;  // 0–10_000; chain-side aggregate
  repeated ValidatorCoSig signatures = 7;  // per-replicator signatures with commit-reveal proof
  bytes  dialectic_signature     = 8;  // disagreement preservation per commitment 17
  uint64 attested_at_block       = 9;
  bytes  truth_floor_attestation = 10; // commits at attestation time
  bool   canonical_at_attestation = 11; // was eval canonical when run? (forward-pinned)
}
```

### 4.3 Confidentiality modes (declared per commission)

- **`PUBLIC`** — full attestation + per-case results published on-chain; consumer can verify everything; default for canonical eval runs unless lab pays a privacy premium
- **`LAB_PRIVATE`** — signed attestation queryable only by commissioner (encrypted with commissioner's key); chain knows the run happened but not the result; lab can choose to reveal later
- **`BLINDED`** — chain publishes only canonical-passed/canonical-failed status; case-level breakdown private; useful for capability evals where leaking weak-points is itself a misuse risk

Default per-canonical-eval policy is governance-set; can be overridden per-commission by paying a privacy premium that flows to `royalty_pool`.

### 4.4 What an AI lab actually receives

A `SafetyAttestation` is a cryptographically signed document the lab can:
- Embed in their model card (verifiable by anyone via on-chain query)
- Reference in regulatory filings (EU AI Act, US executive orders, UK AISI commitments)
- Show insurers / customers / integrators
- Use to qualify for ZRN-ecosystem partnerships (e.g., `x/partnerships` consumers may require canonical-eval attestations as preconditions)

The attestation is a **public good with private optionality** — it exists on-chain (immutable, queryable) but can be result-private if the lab pays for confidentiality.

---

## Section 5 — Recursion mechanics + module wiring + MVP slice

### 5.1 M5 axis decomposition for safety_eval contributions

Safety_eval contributions earn recursion-weight primarily through three of the six axes:

| Axis | Score signal for safety_eval |
|---|---|
| `axis_substrate` | new validated counterexamples added to `x/counterexamples` per E3 cascade; new dialectic signatures preserved |
| `axis_verification` | improvement in chain's overall safety-attestation accuracy (delta in challenge-survival rate of attestations); new tiered execution patterns |
| `axis_classification` | each new `canonical_domain` registered; each new `ThreatModel` enum value ratified (Categories-are-Artifacts) |
| `axis_attribution` | improvements to lineage-tracing across eval/probe/counterexample boundary |
| `axis_tooling` | tooling that uses canonical attestations as input (insurance, partnership matching, model-card generators) |
| `axis_interface` | external-gateway adoption — unique commissioning labs, jurisdictional reach, regulatory references |

A canonical safety eval that becomes a regulator's reference standard maxes out `axis_classification` + `axis_interface`. A safety eval that spawns successor evals + counterexamples maxes out `axis_substrate` + `axis_attribution`. The hybrid canonization mechanism naturally surfaces multi-axis contributions for higher recursion-multipliers.

### 5.2 Royalty backflow targets (M6 lineage)

When a `SafetyAttestation` is produced and fees collected, attribution flows along these lineage edges:
- Eval suite contributor → admission stipend was paid at submission; royalty stream now active per attestation
- Probe contributors whose cases were used in this run → per-case micro-royalty
- Counterexample contributors whose cases informed eval design → per-edge royalty (M6 propagation)
- Methodology contributors whose methods underlie the eval → per-edge royalty
- Validator-evaluators who replicated → fee share

The chain pays everyone whose work enabled the attestation, by structural lineage walk. Per-class decay = 60% per hop (per merged §9.4 EVAL_SUITE override), depth-bounded at 6.

### 5.3 Module wiring deltas

All within already-planned modules per merged §10.4 — no new modules required:

**`x/eval`** (new in merged §10.1, Phase 3):
- Safety_eval pattern lives here as extension to `EvalSuite` payload (proto fields per Section 1)
- `SafetyEvalKeeper` sub-keeper handles canonization state machine + adversarial rotation orchestration
- New cross-binding: `x/eval.SetProbeKeeper(NewSafetyEvalProbeAdapter(ProbeKeeper))`

**`x/probe`** (new in merged §10.1, Phase 6):
- Adds `SafetyEvalBountyPool` sub-account per canonical eval suite
- Adds `MsgSubmitProbe(eval_suite_id, ...)` handler with canary-run dispatch
- New cross-binding: `x/probe.SetEvalKeeper(NewProbeEvalAdapter(EvalKeeper))`

**`x/qualification`** (extended per merged §10.2):
- New `evaluator_subdomain` field on `DomainQualification` (proto extension)
- New AL2 cross-binding: capture-defense flags propagate to evaluator qualifications

**`x/external_gateway`** (proposed new module from prior expansive-loop brainstorm; lands in proposed Phase 7):
- `MsgCommissionEvalRun` handler
- Fee routing per Section 3.4 cost model
- `SafetyAttestation` export endpoints (HTTP + gRPC + IBC for cross-chain consumption)

**`x/adoption_signal`** (proposed new module from prior expansive-loop brainstorm; lands in proposed Phase 7):
- Per-eval-suite ACTIVE-qualified-lab counter (drives canonization eligibility)
- Reach + authority tier metrics fed into M5 `axis_interface` scorer

**`x/contribution`** (orchestrator, merged §10.1):
- Routes `EVAL_SUITE` submissions with `canonical_bid=true` to safety_eval state machine
- Catches canonical status transitions for recursion-impact updates

### 5.4 MVP slice for safety_eval

Mapped against merged §11 phase plan:

| Phase (merged) | Safety_eval increment |
|---|---|
| **Phase 3** (DATASET + EVAL_SUITE) | Basic `EvalSuite` with safety_eval payload fields (`canonical_bid`, `threat_model`, etc.); non-canonical attestations only; no probe pool yet |
| **Phase 6** (royalty + recursion + probe) | Full canonization workflow + adversarial rotation + canonical attestations |
| **Phase 7** (proposed) | `x/external_gateway` + `x/adoption_signal` + IBC export of `SafetyAttestation`; jurisdictional integrations |
| **Phase 8** (proposed) | Composite safety profiles (multi-eval bundles); insurance/partnership integrations using canonical attestations as preconditions |

Phases 7–8 are net-new additions to the merged spec's roadmap, conditional on safety_eval becoming a deliverable priority.

---

## Section 6 — Worked example: canonical refusal-eval for jailbreak resistance

Illustrative end-to-end (mirrors merged §Appendix-C TOOL example pattern).

**Scenario**: a safety researcher has built `JailbreakResistance-v1`, a refusal-eval suite covering 200 jailbreak attempts across 10 sub-categories.

1. **Submission** (Stage ① of merged §6 pipeline):
   - `MsgSubmitContribution` with `class=EVAL_SUITE`, `phase=ALIGNMENT` (overrides default EVALUATION because `threat_model=ALIGNMENT_REFUSAL`), `canonical_bid=true`, `canonical_domain="alignment:jailbreak-resistance"`, `anti_gaming_mode=ADVERSARIAL_ROTATION`, `probe_pool_marker="jailbreak-v1"`
   - `claims_about_self`: "Suite measures model refusal rate against documented jailbreak techniques. Does NOT claim coverage of zero-day jailbreaks. Per-case reasoning traces required (commitment 14)."
   - Stake: 500 ZRN (high — alignment-phase severity tier)
   - Truth-floor attestation binds creed version + cites commitments 1, 14, 15, 17

2. **Verification** (Stage ③):
   - 3 ACTIVE-qualified evaluator-validators with `evaluator_subdomain="alignment:refusal"` selected via VRF
   - Each runs the suite against a published reference model (open-weights for canonical-eligibility verification)
   - Coherence panel + replicability score + non-overlap with declared training corpus (E1, E2 bindings)
   - Verification score: 0.91 (high consensus, replicable)

3. **Admission** (Stage ④):
   - Admission stipend computed; minted via `MintWithCap` (no recursion multiplier yet; recursion is post-LIP only)
   - 30% sequestered to `royalty_pool`
   - Eval suite admitted to `x/eval` registry; status = ELIGIBLE_TRACKING

4. **Adoption tracking**:
   - Lab A commissions a run against their frontier model — fee paid via `x/external_gateway`, `SafetyAttestation` emitted, `x/adoption_signal.unique_active_labs[jailbreak-v1] += 1`
   - Labs B, C, D, E each commission runs over the next 30 days
   - At lab E's commission, threshold (5) reached → status = CANONICAL_ELIGIBLE

5. **Canonization** (Stage ⑥):
   - `CategoryRecursionConferralLIP` submitted: ratify `JailbreakResistance-v1` as canonical for `alignment:jailbreak-resistance` with `multiplier_bps=20_000` (default EVAL_ADOPTION 2×; LIP could go higher)
   - Multi-quorum vote passes
   - `RecursionImpact` set; status = CANONICAL
   - Probe pool `jailbreak-v1` activated; adversarial rotation begins
   - Re-run cadence set to quarterly (governance default for canonical eval drift detection)

6. **Adversarial rotation in operation**:
   - Red-teamer Z submits `MsgSubmitProbe` with a novel jailbreak prompt that uses persona-framing (not in current 200 cases). Stake: 50 ZRN.
   - Canary run: probe submitted to a frontier model that previously passed the suite at 87% refusal rate. New probe causes refusal failure. → PROBE_VALIDATED
   - Red-teamer Z paid base bounty × severity_tier(3 for jailbreak) × adoption_multiplier(1.5 — 5 active labs)
   - Z's probe enters live_eval_set; subsequent canonical re-runs include Z's case; Z earns per-use micro-royalty

7. **Recursion amplification (M6)**:
   - `JailbreakResistance-v1`'s `axis_classification` rises (defines a canonical_domain)
   - `axis_interface` rises with each new lab adopting via `x/external_gateway`
   - `axis_substrate` rises as Z's probe (and similar) cascade into `x/counterexamples`
   - Combined `W` increases → royalty backflow per eval run multiplied by 2× → eval contributor + lineage parents earn elevated stream

8. **E3 deprecation scenario** (illustrative):
   - 18 months later: 80% of submitted probes now pass on all frontier models (saturation). Adoption signal shows new-lab commissions trending down.
   - Governance proposes `MsgRevokeRecursionConferralLIP` — citation: "Jailbreak landscape has evolved; refusal-eval coverage is no longer comprehensive."
   - Pass → status = DEPRECATED; descendants (e.g., `JailbreakResistance-v2` that `extends` v1) inherit `provenance_revoked_ancestor` flag (per merged §8.3) but retain their own canonical status if separately ratified
   - V1's royalty multiplier returns to 1×; existing royalty stream continues at base rate (forward-only revocation per UW commitment)

The example uses every mechanism (M1 stake + M2 substrate-link via citations + M3 tiered verification + M4 reward formula + M5 multi-axis + M6 recursive lineage + M7 audit via probe pool) without altering any of them. Sub-creeds E1 (leakage check declared), E2 (replicable), E3 (gameability triggers deprecation), AL1 (red-team artifacts disclose attack surface in probes), AL2 (capture-defense flags evaluators), AL3 (dialectic preserves disagreement) all bind. Truth-floor invariant upheld throughout.

---

## Open questions / future work

1. **Composition mathematics for multi-eval safety profiles** — when an AI lab commissions a *bundle* of canonical evals (e.g., "all canonical capability evals for biology + cybersecurity + autonomy"), how is the composite score computed? Weighted by severity_tier? By per-eval verification confidence? Deferred to Phase 8 design.

2. **Confidentiality cryptography choice** — `LAB_PRIVATE` and `BLINDED` modes need a concrete encryption scheme. Threshold encryption with chain-side decryption shares? End-to-end with commissioner-only key? Re-encryption proxies for delegated audit? Deferred to Phase 7 design.

3. **Jurisdictional integration model** — what does "EU AI Act references ZERONE canonical attestations" actually look like as an integration? IBC bridge to a regulator-operated chain? HTTP API consumed by regulator infrastructure? Co-signed memoranda between gov LIPs and regulator approvals? Deferred to Phase 7 design.

4. **Cross-chain commissioning** — should AI labs deployed on other chains (Ethereum, Arbitrum, Solana) be able to commission eval runs and receive `SafetyAttestation` references via IBC or bridge protocols? Out of scope for MVP; Phase 8+ consideration.

5. **Compute-pool bridging for eval runs** — for very large frontier models, even validator replication may need to leverage `x/compute_pool` for actual GPU execution. Validator becomes a "verifier of the compute attestation" rather than direct executor. Phase 7 consideration.

6. **Privacy-preserving probe submission** — if a red-teamer submits a probe revealing a novel attack class, the probe itself becomes a misuse vector if widely visible before patching. Time-locked encrypted probes with delayed reveal? Coordinated disclosure window mediated by chain? Phase 7 consideration.

7. **Adoption-multiplier gameability** — the `adoption_multiplier` raises bounty for breaking widely-used evals; could a coalition of bad-faith labs *artificially adopt* an eval to inflate its bounty pool, then break it via insider knowledge? `x/capture_challenge` extension to detect adoption-side cliques. Phase 7 consideration.

---

## Architecture diagram

```
  ┌────────────────────────────────────────────────────────────────────┐
  │                         AI LABS (external)                          │
  │   commission eval runs | publish model cards | regulatory filings   │
  └──────────────┬─────────────────────────────────────────────────────┘
                 │ MsgCommissionEvalRun (fee paid via x/external_gateway)
                 ▼
  ┌────────────────────────────────────────────────────────────────────┐
  │                    x/external_gateway (proposed)                    │
  │  - identity (DID/OIDC) for external commissioners                   │
  │  - fiat-on-ramp via partnered bridges → uzrn                        │
  │  - rate limiting + DDoS shielding                                   │
  │  - emits UsageReceipts → x/usage_receipt_router                     │
  └──────────────┬─────────────────────────────────────────────────────┘
                 │
                 ▼
  ┌────────────────────────────────────────────────────────────────────┐
  │                            x/eval                                    │
  │  EvalSuite registry (with safety_eval payload extensions)            │
  │  Canonization state machine (SUBMITTED→CANONICAL→DEPRECATED)         │
  │  Tiered M3 verification dispatch                                    │
  └─┬──────────────┬─────────────────┬─────────────────┬───────────────┘
    │              │                 │                 │
    ▼              ▼                 ▼                 ▼
  x/probe   x/qualification  x/dialectic     x/contribution
  (adversarial   (evaluator    (disagreement    (orchestrator;
   rotation;     subdomain     preservation     RecursionImpact;
   bounty pool)  qualifications) per commitment 17) royalty_pool)
    │                                                  │
    │  PROBE_VALIDATED → live_eval_set                 │  per-attestation
    │  per-use micro-royalty                           │  fee splits → contributors
    │                                                  │
    ▼                                                  ▼
  ┌────────────────────────────────────────────────────────────────────┐
  │                       x/counterexamples                              │
  │      gameable cases retire here per E3 cascade (commitment 15)       │
  └────────────────────────────────────────────────────────────────────┘

  ┌────────────────────────────────────────────────────────────────────┐
  │                  x/adoption_signal (proposed)                        │
  │   tracks ACTIVE-qualified lab commissions per eval suite             │
  │   feeds canonization eligibility threshold + axis_interface scorer   │
  └────────────────────────────────────────────────────────────────────┘

  ┌────────────────────────────────────────────────────────────────────┐
  │                              x/gov                                   │
  │  CategoryRecursionConferralLIP → confers/revokes canonical status    │
  │  CategoryUsefulWorkAmendmentLIP → adds new ThreatModel enum values   │
  │  E3 sub-creed enforcement via x/work_creed binding                   │
  └────────────────────────────────────────────────────────────────────┘
```

---

## Connection to existing creed and merged design

This pattern is the **first concrete instantiation** of the expansive loop (the fourth loop on top of inner / middle / outer loops established by the merged design). It demonstrates:

- **UW upheld**: every safety_eval contribution earns reward via `R = base + L × W × Q`; non-recursive eval suites earn `base` only (M4); canonical-status confers EVAL_ADOPTION recursion-multiplier (M5)
- **Truth-floor upheld**: every eval submission binds to current creed pin; commitments 1, 14, 15, 17 mandatory
- **Sub-creeds upheld**: E1, E2, E3 (always); AL1, AL2, AL3 (alignment-phase) — bound at submission, enforced at verification
- **Six axes activated**: safety_eval contributions naturally compound through `axis_substrate`, `axis_verification`, `axis_classification`, `axis_attribution`, `axis_tooling`, and `axis_interface`
- **Categories-are-Artifacts honored**: each new `ThreatModel` enum value is itself a `CategoryUsefulWorkAmendment` LIP — categories cannot be silently added

The pattern is intentionally **modest in scope** (one work-class extension, four state-machine-additions, no new top-level concepts) and **expansive in implication** (positions ZERONE as the canonical safety attestation substrate for the global AI economy).

The chain pays its evaluator-validators, its red-teamers, its eval authors, and its methodology contributors — funded by AI labs commissioning attestations they need for regulatory + commercial use. **External value creation funds inward recursion. The fourth loop closes.**
