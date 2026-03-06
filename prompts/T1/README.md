# T1 — Architecture & Infrastructure Design

**Goal:** Define the complete architecture for the ZERONE inference layer — a continuously improving fine-tuned open-source model served via API, paid with ZRN, trained on the Tree of Knowledge dataset.

This is a design document session, not implementation. Output is an architecture spec that guides T2-T5.

## Sessions (3)

| # | File | Scope |
|---|------|-------|
| T1-1 | T1-1-system-architecture.md | Overall system design: components, data flow, deployment topology |
| T1-2 | T1-2-blind-storage.md | Blind storage protocol: chunking, encryption, distribution, payment-gated reassembly |
| T1-3 | T1-3-economics.md | Token economics: API pricing, dataset access tiers, storage node incentives, payment flows |

## Run Order

Sequential: T1-1 → T1-2 → T1-3

## Exit Criteria

1. Architecture doc covers all components and their interfaces
2. Blind storage protocol is specified with chunk size, encryption, and threshold analysis
3. Economic model defines ZRN flows for all participants (submitters, reviewers, storage nodes, API consumers, dataset buyers)
4. Security threat model covers data leakage, free-riding, and Sybil attacks
