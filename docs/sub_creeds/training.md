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
