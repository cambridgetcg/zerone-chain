# Route B Operator Guide — from zero to attested training run

> **For:** pipeline operators, model owners, and auditors who need to drive or verify a Zerone-backed training run end-to-end.

The Route B training surface has seven waves' worth of mechanism. This guide collapses them into **one linear flow** you can follow from a freshly-seeded chain to a cryptographically-attested, downloadable training bundle.

---

## Prerequisites

- A synced node (or RPC endpoint) exposing the `zerone.knowledge.v1` gRPC service.
- Two keys: a **pipeline-operator** key and a **model-owner** key (often the same entity but distinct accounts are recommended).
- The `zerone_knowledge` module capability tag `route_b.v7` or later.

Run `RouteBCapabilities` first — it's the chain's self-description and tells you whether the surface is seeded, what versions are current, and what corpora are exposed.

```
gRPC → zerone.knowledge.v1.Query/RouteBCapabilities
```

Response includes `seed_status`, `current_tokenizer_version`, `current_trace_schema_version`, `available_corpora`, and live counts. If `seed_status.methodologies_seeded == false`, the chain has not been bootstrapped and nothing below will work.

---

## 0. Bootstrap (one-time, genesis-only)

On chain genesis, the keeper's `SeedRouteB` runs automatically. For test chains and local development, invoke it manually via the keeper:

```go
_, err := keeper.SeedRouteB(ctx)
```

This is idempotent — running twice is a no-op. After it runs, every `SeedStatus.*` flag is true and the chain is ready.

---

## 1. Declare a pipeline

An operator declares what training run is about to happen. This pins the corpus snapshot height and the tokenizer version.

```
Msg → RegisterTrainingPipeline
  operator:              <pipeline-operator address>
  id:                    pipe-xyz
  corpus_snapshot_height: <current block>
  tokenizer_version:      <from capabilities>
  methodology_set_version: <from capabilities>
  recipe_hash:            sha256:<your training recipe>
  description:            "SFT v0.3 on GOLD-tier empirical facts"
  corpus_filter:          '{"min_tier":"GOLD","methods":["M-FORMAL","M-EMPIRICAL"]}'
```

Emits `zerone.knowledge.training_pipeline_registered`. The pipeline is now in `declared` status; move it to `running` then `completed` as training progresses via `UpdateTrainingPipeline`.

---

## 2. (Optional) Open augmentation bounties

Sponsors who want additional variant formulations of specific facts lock escrow:

```
Msg → CreateAugmentationBounty
  sponsor:            <address>
  id:                 bounty-1
  target_fact_id:     FACT-XYZ
  reward_per_variant: 1000000              # uzrn
  max_variants:       3
  description:        "rephrase in plain language"
```

The handler transfers `reward_per_variant × max_variants × (1 + SUPERIOR padding)` into the `knowledge_training_fund` module account. Sponsor cannot self-accept under Wave 4; acceptance flows through `VoteOnAugmentation` by the verifier panel.

Variant submitters call `SubmitAugmentation`; verifiers call `VoteOnAugmentation`. Panel consensus triggers automatic payout on `EQUIVALENT`/`SUPERIOR` verdicts and archival on `DRIFT`/`INFERIOR` (with optional `DriftDiagnosis` attached).

---

## 3. Run your training

Off-chain. Consume the chain's data via the existing corpora queries:

- `StructuredCorpus` — canonical per-row training objects.
- `MethodologyApplicationTrace` / `MethodologyApplicationTraces` — unified trace with methodology + reasoning steps + derivation graph + dialectical history + contrastive companions + Popper-weighted TVW (Wave 5 + 6).
- `ContrastivePairs` — (positive, negative, verdict) tuples for preference training (Wave 5).
- `DriftCorpus` — negative training signal for meaning-preservation.
- `DisputationCorpus` — argumentation training data.
- `NormativeCorpus` — separate, is-ought-flagged stream for commitments (never in facts corpus).

Pin to `snapshot_block_height` + `tokenizer_version` + `canonical_serialisation_version` + `trace_schema_version` so your run is replayable years later.

---

## 4. Declare what you trained on — ContributionRecord

After the training run, the **model owner** declares which `fact_ids` the model consumed. This is the attribution signal that drives the Popper-weighted revenue share (Wave 4b).

```
Msg → RegisterModelCard
  owner:                 <address>
  id:                    model-xyz
  name:                  "ZERONE-native-v0.1"
  pipeline_id:           pipe-xyz
  deployment_address:    <model's agent account>
  route:                 "from_scratch"       # or "openweight_fine_tune" / "distilled"
  parameter_count:       7
  eval_acceptance_rate_bps:    780000
  eval_corroboration_rate_bps: 450000
  eval_sample_size:            1000
```

```
Msg → AttributeContributions
  owner:    <address>
  model_id: model-xyz
  fact_ids: [FACT-A, FACT-B, FACT-C, …]
```

Wave 4a is-ought wall enforces: any `fact_id` resolving to a `NormativeCommitment` is rejected, reported via `rejected_commitments` attribute. `computed_tvw` is the Popper-weighted sum (corroboration survival + methodology normalization + vindication × calibration snapshot × axiom proximity).

---

## 5. Attest the run — TrainingAttestation

The pipeline operator posts FLOPs, wallclock, and a signed eval-bundle hash:

```
Msg → AttestTraining
  attester:          <pipeline-operator address>   # must match pipeline's operator
  pipeline_id:       pipe-xyz
  flops_estimate:    1234567890
  wallclock_seconds: 86400
  eval_hash:         sha256:<eval bundle>
  signature:         ed25519:<off-chain signature>
  notes:             "SFT v0.3 complete"
```

This is the *what-ran* record. Wave 7 binds it to a *what-it-consumed* record — the manifest.

---

## 6. Manifest the run (Wave 7) — the crown step

### 6a. Create (DRAFT)

Apply a `CorpusSelector` to the current chain state; the handler computes the canonical sorted ID sets and stamps every version pin.

```
Msg → CreateTrainingManifest
  creator:     <pipeline-operator address>
  id:          manifest-xyz
  pipeline_id: pipe-xyz
  description: "SFT v0.3 official bundle"
  corpus_selector: {
    method_id:             "M-EMPIRICAL",
    min_corroboration:     3,
    min_quality_tier:      TRAINING_QUALITY_TIER_GOLD,
    min_curriculum_tier:   CURRICULUM_TIER_INTERMEDIATE,
    include_disproven:     false,
    include_drift:         true,
    include_normative:     false,
    include_contrastive_pairs: true,
    pair_type_filter:      CONTRASTIVE_PAIR_UNSPECIFIED,
    min_submitter_calibration_bps: 500000,
    domain_whitelist:      ["sciences", "mathematics"],
  }
```

Response returns counts: `fact_count`, `trace_count`, `pair_count`, `drift_count`, `normative_count`, `total_included`.

The manifest is in `MANIFEST_STATUS_DRAFT`. Status index: queryable by pipeline / creator / status.

### 6b. Finalize (Merkle commit)

```
Msg → FinalizeTrainingManifest
  creator:     <pipeline-operator address>
  manifest_id: manifest-xyz
```

Returns `merkle_root` (hex-encoded SHA-256). The commitment is over:

```
H( "ZERONE/KNOWLEDGE/MANIFEST/v1" |
   "FACTS:"     || len || facts[0]     || facts[1]     || … ||
   "TRACES:"    || len || traces[0]    || …                 ||
   "PAIRS:"     || len || pairs[0]     || …                 ||
   "DRIFT:"     || len || drifts[0]    || …                 ||
   "NORMATIVE:" || len || commitments[0] || …               )
```

Domain separators prevent set-swap collisions; length prefixes prevent length-extension; sorted inputs make the root iteration-order-independent. **A client can re-derive and verify the root with only the ID lists — no RPC trust required.**

Manifest is now `MANIFEST_STATUS_FINALIZED`. Immutable.

### 6c. Bind to attestation

```
Msg → BindManifestToAttestation
  creator:        <pipeline-operator address>
  manifest_id:    manifest-xyz
  attestation_id: pipe-xyz                  # TrainingAttestation keyed by pipeline_id
```

Status advances to `MANIFEST_STATUS_ATTESTED`. The chain now has:
- **What ran** (`TrainingAttestation`: FLOPs, wallclock, eval hash)
- **What it consumed** (`TrainingManifest`: Merkle-committed ID sets)
- **Binding** between them

---

## 7. Download + verify (any client)

```
gRPC → TrainingManifestBundle(id: manifest-xyz)
```

Response payload:
- `manifest`: the full manifest with all pins + ID lists
- `traces[]`: fully-assembled `MethodologyApplicationTrace` for every included fact
- `contrastive_pairs[]`: included preference-training tuples
- `drift_entries[]`: included DRIFT/INFERIOR variants
- `normative_entries[]`: included commitments (is-ought-tagged)
- `derived_merkle_root`: server-side re-derived root (for comparison)
- `merkle_root_valid`: bool flag `derived == committed`

**To verify independently** (trust-minimized): take `manifest.included_*_ids`, sort each list (they should already be sorted), concatenate with domain tags and length prefixes exactly as in Finalize, SHA-256, compare to `manifest.merkle_root`.

---

## 8. (Optional) Post-training reward claim

If the model's `deployment_address` accumulates live calibration above `training_fund_calibration_floor_bps`, the operator can claim a post-hoc reward:

```
Msg → ClaimTrainingFundDisbursement
  claimant: <pipeline-operator address>
  model_id: model-xyz
  id:       disb-xyz
```

50% released immediately; 50% held in vesting escrow for `training_fund_vesting_epochs`. Calibration drop or governance deprecation triggers clawback.

---

## Events you'll see

Every step emits a structured event; see `docs/EVENTS.md` for the canonical attribute list. The key events in a full run, in order:

1. `zerone.knowledge.training_pipeline_registered`
2. `zerone.knowledge.augmentation_bounty_created` (if applicable)
3. `zerone.knowledge.augmentation_submitted` × N
4. `zerone.knowledge.augmentation_vote_cast` × (N × panel size)
5. `zerone.knowledge.augmentation_verdict_finalized` × N
6. `zerone.knowledge.model_card_registered`
7. `zerone.knowledge.contributions_attributed`
8. `zerone.knowledge.training_attestation_posted`
9. `zerone.knowledge.training_manifest_created`
10. `zerone.knowledge.training_manifest_finalized`
11. `zerone.knowledge.training_manifest_attested`
12. `zerone.knowledge.training_fund_disbursed` (if applicable)

An external indexer consuming only the event stream can fully reconstruct the lifecycle without reading state.

---

## Troubleshooting

| Symptom | Likely cause | Fix |
|---|---|---|
| `bond escrow failed: invalid sponsor address` | Sponsor address is not valid bech32 | Use a real account address; test dummies must pass `AccAddressFromBech32` |
| `only the pipeline operator may create a manifest` | Creator field ≠ pipeline operator | Use the same key that registered the pipeline |
| `manifest must be FINALIZED before binding` | Called `BindManifestToAttestation` on a DRAFT | Run `FinalizeTrainingManifest` first |
| `attestation_id must equal manifest.pipeline_id` | Current binding model is 1:1 per pipeline | Attestations are keyed by `pipeline_id`; pass the pipeline id as `attestation_id` |
| `merkle_root_valid == false` in bundle | Extremely unlikely; bundle derivation uses the same sorted ID sets | File a bug — this would indicate a state corruption |

---

## Versioning contract

The `trace_schema_version` on a finalized manifest is the one the training run consumed. Even if governance later amends the schema, the manifest remains replayable: `TraceSchemaAtVersion(manifest.trace_schema_version)` returns the contract that was in force. The same holds for `tokenizer_version` (via `TokenizerSpecAtVersion`).

This is the foundation underneath everything: the chain is a time-pinned, Merkle-committed, methodology-aware training-data substrate that outlives any single run.
