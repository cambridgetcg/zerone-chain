# R38-1 — Dataset Curation

## Objective

Implement dataset creation, management, and dynamic sample enumeration. Datasets are curated views over the sample store — defined by filters, not by copying data.

## Tasks

### 1. CreateDataset

```go
func (k Keeper) CreateDataset(ctx context.Context, msg *types.MsgCreateDataset) (*types.MsgCreateDatasetResponse, error) {
    dataset := &types.Dataset{
        Id:             k.nextDatasetSeq(ctx),
        Name:           msg.Name,
        Description:    msg.Description,
        Domain:         msg.Domain,
        License:        msg.License,
        FilterType:     msg.FilterType,
        FilterLanguage: msg.FilterLanguage,
        FilterTags:     msg.FilterTags,
        MinQuality:     msg.MinQuality,
        PricePerSample: msg.PricePerSample,
        BulkPrice:      msg.BulkPrice,
        Curator:        msg.Curator,
        CreatedAtBlock: currentBlock,
    }
    // Compute initial sample_count and total_tokens by running filter
    dataset.SampleCount = k.countMatchingSamples(ctx, dataset)
    k.SetDataset(ctx, dataset)
    return &types.MsgCreateDatasetResponse{DatasetId: dataset.Id}, nil
}
```

### 2. Filter Engine

Datasets are defined by filters (not explicit sample lists). This keeps datasets automatically up-to-date.

```go
func (k Keeper) getMatchingSamples(ctx context.Context, dataset *types.Dataset, page *query.PageRequest) ([]*types.Sample, *query.PageResponse, error) {
    // Apply filters:
    // 1. Domain match (if set)
    // 2. SampleType match (if set)
    // 3. Language match (if set)
    // 4. Tag intersection (if set)
    // 5. MinQuality threshold
    // 6. Status must be active (gold/silver/bronze)
    // Use domain+type index for efficient first filter, then refine
}
```

### 3. Dataset Stats Refresh

Periodically (or on query) refresh dataset stats:
- `sample_count`: current matching samples
- `total_tokens`: estimated token count (content bytes / 4 rough estimate)

### 4. Curator Revenue

Dataset curators earn a small commission (e.g. 5% of access fees) for creating valuable dataset definitions. This incentivizes good curation.

### 5. Tests

- Create dataset with various filters
- Dataset sample count reflects filter
- New sample matching filter increments count
- Sample pruning decrements count
- Dataset with no matches → count 0
- Multiple datasets can overlap

Target: ≥ 15 tests.
