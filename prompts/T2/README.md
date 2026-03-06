# T2 — Dataset Pipeline: ToK Export → Training Format → Fine-Tuning

**Goal:** Build the pipeline that transforms on-chain approved Samples into training-ready datasets and runs fine-tuning jobs on open-source base models.

## Sessions (3)

| # | File | Scope |
|---|------|-------|
| T2-1 | T2-1-dataset-exporter.md | Go service that watches the chain for approved Samples, exports them into training-ready formats |
| T2-2 | T2-2-training-formats.md | Data transformation: SampleType → training format (instruction pairs, conversations, raw text). Quality filtering, domain slicing |
| T2-3 | T2-3-fine-tuning-pipeline.md | Python pipeline: LoRA/QLoRA fine-tuning on base models, model versioning, automated evaluation |

## Run Order

Sequential: T2-1 → T2-2 → T2-3

## Prerequisites

- T1 architecture document completed
- Access to ZERONE chain (testnet or local)

## Exit Criteria

1. Exporter connects to chain and pulls approved Samples
2. Samples correctly transformed to HuggingFace-compatible training formats
3. Fine-tuning pipeline runs end-to-end on a small test dataset
4. Model versioning tracks base model + dataset version + training config
5. Automated eval benchmarks the fine-tuned model against base
