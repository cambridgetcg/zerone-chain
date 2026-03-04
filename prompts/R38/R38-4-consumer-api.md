# R38-4 — Consumer Query API

## Objective

Optimize query endpoints for AI lab access patterns: filtered search, bulk export, and efficient pagination over large sample sets.

## Tasks

### 1. Filtered Sample Search

```go
func (k Keeper) QuerySamples(ctx context.Context, req *types.QuerySamplesRequest) (*types.QuerySamplesResponse, error) {
    // Composite filter:
    // - domain (index lookup)
    // - sample_type
    // - language
    // - min_quality_score
    // - tags (intersection)
    // - source_platform
    // - created_after / created_before (block range)
    // Use most selective index first, then refine in-memory
}
```

### 2. Thread Retrieval

```go
func (k Keeper) QuerySamplesByThread(ctx context.Context, req *types.QuerySamplesByThreadRequest) (*types.QuerySamplesResponse, error) {
    // Return all samples in a thread, ordered by thread_position
    // Consumers often want complete conversations, not isolated messages
}
```

### 3. Bulk Export Format

For dataset access, return samples in a training-ready format:

```go
type ExportSample struct {
    Content        string            `json:"content"`
    SampleType     string            `json:"type"`
    Domain         string            `json:"domain"`
    Language       string            `json:"language"`
    QualityTier    string            `json:"quality_tier"`
    QualityScore   uint64            `json:"quality_score"`
    SourcePlatform string            `json:"source_platform"`
    ThreadId       string            `json:"thread_id,omitempty"`
    ThreadPosition uint64            `json:"thread_position,omitempty"`
    Tags           []string          `json:"tags"`
    License        string            `json:"license"`
    ConsentType    string            `json:"consent_type"`
    ProvenanceHash string            `json:"provenance_hash"`  // On-chain proof
}
```

### 4. Protocol Stats

```go
func (k Keeper) QueryProtocolStats(ctx context.Context, req *types.QueryProtocolStatsRequest) (*types.QueryProtocolStatsResponse, error) {
    // Total samples, total submissions, samples by tier, samples by domain,
    // total revenue, active bounties, active datasets
}

func (k Keeper) QueryDomainStats(ctx context.Context, req *types.QueryDomainStatsRequest) (*types.QueryDomainStatsResponse, error) {
    // Per-domain: sample count by tier, access count, revenue, top topics,
    // saturation score, active bounties
}
```

### 5. Submitter Dashboard Queries

```go
// What has a submitter contributed and earned?
func (k Keeper) QuerySamplesBySubmitter(ctx context.Context, req *types.QuerySamplesBySubmitterRequest) (*types.QuerySamplesResponse, error)
```

### 6. Tests

- Filtered query by domain
- Filtered query by type + language
- Filtered query by min quality
- Thread retrieval ordered by position
- Bulk export format correctness
- Protocol stats accuracy
- Domain stats accuracy
- Submitter query returns only their samples
- Pagination works correctly
- Empty result sets handled

Target: ≥ 10 tests.
