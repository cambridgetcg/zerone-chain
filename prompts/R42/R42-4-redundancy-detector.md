# R42-4 — Redundancy Detector

## Objective

Build the semantic redundancy detector that identifies clusters of TDUs teaching the same concept, keeps the most influential, and generates decay signals for the rest (20% weight).

## Design

### Approach

1. **Embed all active TDUs** using the same embedding model as dedup (R39-2)
2. **Cluster by similarity** — HDBSCAN or simple threshold-based clustering (cosine similarity ≥ 0.85)
3. **Within each cluster**:
   - Rank by current fitness score (descending)
   - Top TDU = "canonical" for that concept
   - Others get redundancy decay signal proportional to their similarity to the canonical
   - Higher similarity to canonical = stronger decay (more redundant)
4. **Protect corrections** — if a cluster contains a correction TDU, it becomes canonical regardless of fitness

### Pipeline

```
services/redundancy/
├── main.go
├── embedder.go      — Embed TDU content (batch)
├── clusterer.go     — Cluster TDUs by semantic similarity
├── ranker.go        — Rank within clusters, identify canonical
├── signal_gen.go    — Generate redundancy decay signals
└── redundancy_test.go
```

### Scheduling

- Run after each fine-tune cycle (alongside influence analysis)
- OR run on a timer (e.g., daily) independently
- Output: fitness signals for redundant TDUs

### Signal Generation

```go
// For each non-canonical TDU in a cluster:
signal := FitnessSignal{
    Type:   "redundancy",
    Weight: 0.2,  // 20% of total fitness
    Value:  -(similarity_to_canonical - threshold),
    // e.g., similarity 0.95 to canonical → Value = -0.10
    // e.g., similarity 0.87 to canonical → Value = -0.02
}
```

### Canonical Selection Rules

1. Highest fitness score wins (reward quality)
2. Corrections always win over non-corrections (reward improvement)
3. Older TDU wins ties (reward early contribution)
4. Canonical TDU gets small positive signal (+0.05) for being the best version

### Cluster Visualization (optional)

Generate a report showing:
- Number of clusters per domain
- Largest clusters (most redundancy)
- TDUs at risk of pruning due to redundancy
- Recommendations for submitters (what topics need more coverage vs over-covered)

## Tests

- Test: two near-identical TDUs → clustered together
- Test: canonical = highest fitness in cluster
- Test: correction TDU becomes canonical regardless of fitness
- Test: redundancy signal proportional to similarity
- Test: canonical gets positive signal
- Test: dissimilar TDUs NOT clustered (below threshold)
- Test: single TDU = no redundancy signal

## Constraints

- Embedding dimension: same as R39-2 dedup model
- Clustering threshold: 0.85 cosine similarity (configurable)
- Batch processing: embed in batches of 100 for memory efficiency
- Signal weight: 0.2 (20%)
