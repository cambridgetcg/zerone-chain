# T2-2 — Training Format Transformation

## Goal

Build the transformation layer that converts raw Samples from the staging database into HuggingFace-compatible training formats suitable for fine-tuning different model architectures.

## Context

Samples have a `sample_type` field that indicates the discourse format:
- DISCUSSION — multi-party conversation
- DEBATE — opposing viewpoints
- EXPLANATION — concept explanation
- TROUBLESHOOT — problem → diagnosis → solution
- REVIEW — evaluation/critique
- TUTORIAL — step-by-step teaching
- OPINION — reasoned opinion
- NARRATIVE — storytelling
- Q_AND_A — question and answer
- CREATIVE — creative writing
- ANNOTATION — labeling/annotation
- CORRECTION — correcting misinformation

Each type maps naturally to different training formats.

## Deliverables

### 1. Format Mappings

Define how each SampleType maps to training formats:

**Chat/Instruction format** (for chat fine-tuning):
```json
{"messages": [{"role": "system", "content": "..."}, {"role": "user", "content": "..."}, {"role": "assistant", "content": "..."}]}
```
- Best for: Q_AND_A, EXPLANATION, TUTORIAL, TROUBLESHOOT

**Multi-turn conversation format**:
```json
{"messages": [{"role": "user", "content": "..."}, {"role": "assistant", "content": "..."}, {"role": "user", "content": "..."}, {"role": "assistant", "content": "..."}]}
```
- Best for: DISCUSSION, DEBATE (can model both sides)

**Completion format** (for continued pretraining):
```json
{"text": "..."}
```
- Best for: NARRATIVE, CREATIVE, OPINION

**Preference/DPO format** (for alignment):
```json
{"prompt": "...", "chosen": "...", "rejected": "..."}
```
- Best for: CORRECTION (original = rejected, correction = chosen), REVIEW

### 2. Transformation Pipeline

Python module: `services/training-pipeline/transforms/`

- Read from staging DB (PostgreSQL)
- Apply format mapping based on sample_type
- Handle thread context (reconstruct conversations from thread_id + parent_sample_id + thread_position)
- Quality-weighted sampling: gold samples appear more in training set
- Domain tagging: prepend domain context to system prompts where appropriate
- Language filtering: separate datasets per language or multilingual mix
- Output: JSONL files compatible with HuggingFace datasets library

### 3. Data Quality Filters

Apply additional filtering beyond on-chain quality tiers:
- Minimum content length (skip trivially short samples)
- Deduplication (near-duplicate detection via MinHash/SimHash)
- Language detection verification (confirm language tag matches content)
- Toxicity re-check (optional secondary filter)
- Token count estimation (for batch sizing)

### 4. Dataset Splits

- Train / validation / test splits (90/5/5 or configurable)
- Stratified by domain and quality tier
- Held-out test set never used in training (for evaluation)

### 5. CLI & Integration

```bash
transform run --snapshot v1.0.0 --format chat --output /data/training/v1.0.0/
transform run --snapshot v1.0.0 --format dpo --domain technical --output /data/dpo/v1.0.0/
transform stats --snapshot v1.0.0   # Show sample counts by type, domain, quality
```

## Output

- Python package at `services/training-pipeline/transforms/`
- Unit tests for each SampleType → format mapping
- Integration test: staging DB with test samples → transformed JSONL output
