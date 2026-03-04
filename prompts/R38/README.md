# R38 — 市 (Ichi): The Market — Data Access & Revenue

**Goal:** Implement the data marketplace: dataset curation, sample access, revenue distribution, and consumer-facing query APIs. After R38, AI labs can discover, purchase, and download training data with automatic compensation to contributors.

市 (Ichi) means "market" — the place where supply meets demand.

## Sessions (4)

| # | File | Scope |
|---|------|-------|
| R38-1 | R38-1-dataset-curation.md | CreateDataset, dataset filters, dynamic sample counting, dataset updates |
| R38-2 | R38-2-access-payment.md | AccessSample, AccessDataset, micro-payments, bulk pricing, payment verification |
| R38-3 | R38-3-revenue-distribution.md | Revenue split engine: submitter share, validator share, protocol share, quality multipliers, consent multipliers |
| R38-4 | R38-4-consumer-api.md | Query optimizations for consumer access patterns: filtered search, pagination, bulk export format |

## Run Order

Sequential: R38-1 → R38-2 → R38-3 → R38-4

## Exit Criteria

1. Datasets can be created with filters and priced
2. Individual and bulk sample access triggers payment
3. Revenue correctly splits to submitters, validators, and protocol
4. Quality and consent multipliers correctly applied
5. Consumer queries are efficient for common access patterns
6. ≥ 40 tests for this batch
