# R38-1 Dataset Curation — Design

## Summary

Datasets are filter-based views over the sample store. No data copying — a `Dataset` stores filter criteria and cached stats. The filter engine uses the existing domain index as primary filter, then applies type/language/tags/quality filters in-memory.

## Components

### 1. State Layer (state.go additions)
- `NextDatasetID(ctx)` — hex-encoded uint64 sequence (follows NextBountyID pattern)
- `SetDataset` / `GetDataset` / `DeleteDataset` — CRUD with `DatasetKeyPrefix` (0x05)
- `IterateDatasets` — full iteration for listing
- `SetDatasetDomainIndex` / `GetDatasetsByDomain` — domain index (new prefix 0xA9)

### 2. Filter Engine (dataset.go)
- `getMatchingSamples(ctx, dataset, pageReq)` — paginated samples matching all filters
- `countMatchingSamples(ctx, dataset)` — returns count + estimated total tokens
- Filter chain: domain → sample_type → language → tags (intersection) → min_quality → active status (GOLD/SILVER/BRONZE)
- Uses `GetSamplesByDomain` when domain is set, `IterateSamples` when not

### 3. Transaction Handlers (dataset.go)
- `CreateDataset` — validate, NextDatasetID, compute initial stats, persist + index, emit event
- `AccessDataset` — verify payment, transfer funds (95% curator / 5% module fee), return sample count

### 4. Query Handlers (grpc_query.go)
- `Dataset(id)` — single lookup, refresh stats on query
- `Datasets(domain, pagination)` — paginated list by domain

### 5. Stats Refresh
- `sample_count` and `total_tokens` refreshed on query via `countMatchingSamples`

## Curator Revenue
- 95% of AccessDataset bulk_price goes to curator
- 5% retained by module
- Uses bankKeeper.SendCoinsFromAccountToModule + SendCoinsFromModuleToAccount

## Active Sample Status
A sample is "active" if its status is GOLD (3), SILVER (4), or BRONZE (5).
