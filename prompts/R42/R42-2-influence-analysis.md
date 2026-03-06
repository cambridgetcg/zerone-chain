# R42-2 — Training Influence Analysis

## Objective

Build the influence analysis pipeline that traces model quality improvements back to specific TDUs, producing the training influence fitness signal (50% weight).

## Design

### Approach

Full gradient-based influence functions (TracIn, TRAK) are expensive. Start with a practical proxy:

**Leave-one-out approximation:**
1. After each fine-tune cycle, run the benchmark suite → baseline score
2. For each TDU batch (not individual — too expensive):
   - Fine-tune without that batch
   - Run benchmark → compare score
   - Score drop = that batch's positive influence
   - Score improvement = that batch's negative influence (it was hurting)
3. Attribute batch influence equally to member TDUs (refinement later)

**Simpler first pass — loss-based attribution:**
1. After training, compute per-TDU training loss
2. TDUs with low loss = model learned them well (may be redundant OR foundational)
3. TDUs with high loss = model struggled (may be novel OR noisy)
4. Cross-reference with benchmark: low-loss + benchmark-helpful = Core candidate

### Pipeline

```
services/influence/
├── main.go
├── analyzer.go      — Core influence analysis logic
├── loss_tracker.go   — Per-TDU loss computation during training
├── signal_emitter.go — Converts analysis to FitnessSignals, submits on-chain
└── influence_test.go
```

### Signal Generation

```go
type InfluenceResult struct {
    TDUID     string
    Score     float64  // -1.0 to 1.0 (negative = harmful)
    Method    string   // "leave-one-out" | "loss-based"
    Benchmark string   // which benchmark domain
}

// Convert to FitnessSignal
signal := FitnessSignal{
    Type:   "training_influence",
    Weight: 0.5, // 50% of total fitness
    Value:  result.Score,
}
```

### Integration with Training Pipeline (T2)

- After `services/training-pipeline` completes a fine-tune run:
  1. Export per-TDU loss values
  2. Run benchmark suite (R42-1) on new model
  3. Run influence analysis
  4. Submit fitness signals on-chain via agent SDK (R41-4)

### On-Chain Submission

Use a dedicated "fitness-oracle" account to submit batch fitness updates:
```
zeroned tx knowledge update-fitness-batch \
  --signals-file ./fitness-signals.json \
  --from fitness-oracle
```

Add `MsgUpdateFitnessBatch` to msg_server if not present.

## Tests

- Test: loss tracker computes per-TDU loss from training logs
- Test: influence analyzer produces correct signal direction (helpful vs harmful)
- Test: signal emitter generates valid FitnessSignal structs
- Test: batch submission submits all signals in one tx

## Constraints

- Influence analysis runs AFTER training, not during (no training pipeline changes)
- Start with loss-based attribution (simpler), graduate to leave-one-out later
- Fitness oracle account needs authority — use module governance or designated address
