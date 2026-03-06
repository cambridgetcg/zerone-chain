# R45-1: Model Registry & Training Lineage

## Context

You're working on the Zerone blockchain (`~/Desktop/zerone`), a Cosmos SDK v0.50 chain. The `x/knowledge` module implements a Tree of Knowledge (ToK) — a decentralised AI training data curation platform.

The TEE training enclave (`services/training-enclave/`) trains models from curated data, producing cryptographic attestations. But there's no on-chain registry for the output. Trained models exist in a void — no discovery, no versioning, no quality tracking, no lineage.

## Task

Build a **Model Registry** in `x/knowledge/keeper/` that tracks trained models as first-class on-chain entities with full training lineage.

### Files to Create

1. **`x/knowledge/keeper/model_registry.go`** — Core registry logic
2. **`x/knowledge/keeper/model_registry_test.go`** — Comprehensive tests
3. **`x/knowledge/types/model.go`** — Model types and validation

### Types (`x/knowledge/types/model.go`)

```go
// ModelRecord represents a trained model registered on-chain
type ModelRecord struct {
    ModelID          string          // unique ID (hash of attestation)
    Name             string          // human-readable name
    Domain           string          // training domain (e.g., "code/go", "code/rust")
    Version          uint64          // monotonically increasing per lineage
    ParentModelID    string          // previous version (empty for v1)
    TrainingRecordID string          // link to TEE training attestation
    
    // Training Lineage
    TDUIDs           []string        // TDU IDs that contributed to training
    DatasetIDs       []string        // dataset IDs used
    TDUCount         uint64          // total TDU count
    
    // Quality Metrics
    BenchmarkScore   sdkmath.LegacyDec // aggregate benchmark score (0.0-1.0)
    BenchmarkDetails []BenchmarkResult // per-benchmark results
    FitnessWeighted  sdkmath.LegacyDec // avg fitness of training data (weighted)
    
    // Lifecycle
    Status           ModelStatus     // ACTIVE, DEPRECATED, SUPERSEDED, FAILED
    Publisher        string          // address that published (usually validator)
    PublishedAt      int64           // block height
    DeprecatedAt     int64           // block height (0 if active)
    DeprecationReason string         // why deprecated
    
    // Inference
    ServingEndpoints []string        // registered inference endpoints
    InferenceCount   uint64          // total API calls served
    
    // Attestation
    TEEAttestation   []byte          // cryptographic proof from training enclave
    ModelHash        string          // SHA-256 of model weights
}

type ModelStatus int32
const (
    ModelStatusActive     ModelStatus = 0
    ModelStatusDeprecated ModelStatus = 1
    ModelStatusSuperseded ModelStatus = 2
    ModelStatusFailed     ModelStatus = 3
)

type BenchmarkResult struct {
    BenchmarkID string
    Score       sdkmath.LegacyDec
    Category    string // "code", "reasoning", "instruction"
    PassRate    sdkmath.LegacyDec
}

// ModelLineage tracks the full ancestry of a model
type ModelLineage struct {
    ModelID    string
    Ancestors  []string // ordered: parent, grandparent, ...
    Generation uint64   // how many training rounds deep
}
```

### Keeper Methods (`x/knowledge/keeper/model_registry.go`)

Implement these methods on `Keeper`:

```go
// Registration
func (k Keeper) PublishModel(ctx sdk.Context, record ModelRecord) error
func (k Keeper) DeprecateModel(ctx sdk.Context, modelID string, reason string) error
func (k Keeper) SupersedeModel(ctx sdk.Context, oldModelID, newModelID string) error

// Queries
func (k Keeper) GetModel(ctx sdk.Context, modelID string) (ModelRecord, bool)
func (k Keeper) GetModelsByDomain(ctx sdk.Context, domain string) []ModelRecord
func (k Keeper) GetActiveModels(ctx sdk.Context) []ModelRecord
func (k Keeper) GetModelLineage(ctx sdk.Context, modelID string) ModelLineage
func (k Keeper) GetLatestModel(ctx sdk.Context, domain string) (ModelRecord, bool)

// Lineage & Attribution
func (k Keeper) GetContributingTDUs(ctx sdk.Context, modelID string) []string
func (k Keeper) GetModelsByTDU(ctx sdk.Context, tduID string) []string  // which models used this TDU
func (k Keeper) CalculateContributorRewards(ctx sdk.Context, modelID string) map[string]sdkmath.Int

// Quality Gating
func (k Keeper) ValidateModelQuality(ctx sdk.Context, record ModelRecord) error
func (k Keeper) UpdateBenchmarkScores(ctx sdk.Context, modelID string, results []BenchmarkResult) error

// Inference Tracking
func (k Keeper) RegisterEndpoint(ctx sdk.Context, modelID string, endpoint string) error
func (k Keeper) RemoveEndpoint(ctx sdk.Context, modelID string, endpoint string) error
func (k Keeper) RecordInference(ctx sdk.Context, modelID string) error
```

### Storage Keys

Use the existing KVStore pattern. Prefix keys:
- `ModelRecordPrefix = []byte{0x60}` — model records
- `ModelDomainIndexPrefix = []byte{0x61}` — domain → model IDs
- `ModelTDUIndexPrefix = []byte{0x62}` — TDU → model IDs (reverse index)
- `ModelLineagePrefix = []byte{0x63}` — model lineage chains
- `ModelEndpointPrefix = []byte{0x64}` — inference endpoints

### Quality Gating Rules

1. `BenchmarkScore >= 0.3` — minimum quality threshold to publish
2. `FitnessWeighted >= 0.4` — training data must be reasonably fit
3. `TDUCount >= 10` — minimum training data size
4. TEE attestation must be valid (check against registered enclaves)
5. Model hash must be non-empty

### Contributor Reward Calculation

When a model earns revenue through API inference, contributors (TDU submitters) receive proportional rewards based on:
- TDU fitness score at time of training (higher fitness = larger share)
- TDU's training influence (from `services/influence/`)
- Normalized across all contributing TDUs

Formula: `reward_i = total_reward × (fitness_i × influence_i) / Σ(fitness_j × influence_j)`

### Tests (`x/knowledge/keeper/model_registry_test.go`)

Cover at minimum:
1. Publish model — happy path with valid attestation
2. Publish model — reject below quality threshold
3. Publish model — reject without TEE attestation
4. Deprecate model — status transition
5. Supersede model — old becomes SUPERSEDED, new links to old
6. Model lineage — 3 generations deep, verify ancestry
7. Domain query — multiple models, filter by domain
8. TDU reverse index — find all models that used a specific TDU
9. Contributor rewards — proportional to fitness × influence
10. Endpoint registration — add, remove, inference counting
11. Latest model — returns highest version for domain
12. Quality gate edge cases — exactly at threshold

### Important Notes

- Use `sdkmath.Int` for all monetary amounts (uzrn denomination)
- Use `sdkmath.LegacyDec` for scores and ratios
- Import existing types from `x/knowledge/types/`
- Check `x/knowledge/keeper/keeper.go` for the Keeper struct and store access patterns
- Check `x/knowledge/types/keys.go` for existing key prefixes (don't collide)
- Check `x/knowledge/keeper/training.go` for TrainingRecord integration
- Check `x/knowledge/keeper/tee.go` for TEE attestation verification patterns
- All amounts in uzrn (1 ZRN = 1,000,000 uzrn)
- Commit with message: `feat(knowledge): R45-1 model registry — on-chain model lifecycle with training lineage`
