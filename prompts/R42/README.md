# R42 — 鏡 (Kagami): The Mirror — Benchmark Suite & Fitness Feedback Loop

**Goal:** Build the benchmark suite that evaluates fine-tuned models and feeds fitness signals back to TDUs, closing the recursive improvement loop. After R42, the dataset learns what it needs to know.

鏡 (Kagami) means "mirror" — the model reflects the quality of its training data back to the data itself.

## Context

The fitness decay system (R40-2) accepts `FitnessSignal` updates but has no source for them beyond quality round outcomes. The ToK spec (section 8.2) describes three signal sources:

1. **Training influence (50%)** — gradient-based attribution: which TDUs most influenced correct outputs
2. **Usage correlation (30%)** — API user feedback traced back to training data
3. **Redundancy detection (20%)** — multiple TDUs teaching the same concept

This round builds the benchmark runner and signal generator that powers all three.

## Sessions (4)

| # | File | Scope |
|---|------|-------|
| R42-1 | R42-1-benchmark-suite.md | Benchmark test suite: code generation, reasoning, instruction following. Automated evaluation harness. |
| R42-2 | R42-2-influence-analysis.md | Training influence attribution: which TDUs contributed to correct benchmark answers. Gradient-based or simpler proxy methods. |
| R42-3 | R42-3-usage-signals.md | API usage feedback pipeline: trace user ratings/retries back to training data, generate usage fitness signals. |
| R42-4 | R42-4-redundancy-detector.md | Semantic redundancy detection: identify TDU clusters teaching the same concept, keep most influential, decay rest. |

## Run Order

Sequential: R42-1 → R42-2 → R42-3 → R42-4

## Exit Criteria

1. Benchmark suite evaluates a model on ≥ 50 test cases across code/reasoning/instruction domains
2. Influence analysis produces per-TDU fitness signals after each fine-tune cycle
3. Usage feedback pipeline generates signals from API interaction data
4. Redundancy detector identifies and scores overlapping TDUs
5. All three signal types feed into `UpdateFitnessScore()` with correct weights
6. End-to-end test: train → benchmark → score → update fitness → verify TDU lifecycle changes
