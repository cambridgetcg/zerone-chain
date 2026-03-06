# T5 — Blind Storage Network & Dataset Marketplace

**Goal:** Implement the blind storage protocol for distributing encrypted training data chunks, and the dataset marketplace where buyers purchase access with ZRN.

## Sessions (3)

| # | File | Scope |
|---|------|-------|
| T5-1 | T5-1-chunk-engine.md | Chunking, erasure coding, encryption of training datasets |
| T5-2 | T5-2-storage-network.md | P2P storage node protocol: registration, chunk distribution, proof-of-storage |
| T5-3 | T5-3-marketplace.md | Dataset marketplace: purchase flow, key release, chunk retrieval, reconstruction |

## Run Order

Sequential: T5-1 → T5-2 → T5-3

## Prerequisites

- T1 architecture (blind storage design)
- T2 dataset pipeline (data to distribute)
- T3 payment integration (ZRN payments)

## Exit Criteria

1. Dataset chunked and encrypted with erasure coding
2. Chunks distributed across storage nodes
3. Storage nodes pass proof-of-storage challenges
4. ZRN payment unlocks chunk access proportional to payment
5. Sufficient payment enables full dataset reconstruction
6. Below threshold, data is computationally useless
