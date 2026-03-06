# R42-3 — Usage Feedback Signals

## Objective

Build the pipeline that traces API usage patterns back to training data, generating usage correlation fitness signals (30% weight).

## Design

### Signal Sources

From the API gateway (T3):
1. **User ratings** — explicit thumbs up/down on responses
2. **Retries** — user re-asks same question (negative signal)
3. **Session length** — longer productive sessions = positive signal
4. **Follow-up patterns** — "that's wrong" / "try again" = negative; "thanks" / continued use = positive

### Attribution Method

Map API responses back to training data:
1. During inference, log which LoRA adapter was active + domain
2. For each response, compute semantic similarity to top-K training TDUs (embedding-based)
3. Positive user signal → positive fitness boost to similar TDUs
4. Negative user signal → negative fitness signal to similar TDUs
5. Decay attribution with similarity distance (closest TDU gets most signal)

### Pipeline

```
services/usage-feedback/
├── main.go
├── collector.go     — Collect usage events from API gateway logs
├── attributor.go    — Map usage events to TDU IDs via embedding similarity
├── aggregator.go    — Batch and aggregate signals over time window
├── emitter.go       — Submit aggregated fitness signals on-chain
└── feedback_test.go
```

### Data Flow

```
API Gateway → usage events (ratings, retries) → collector
collector → attributor (embed query, find similar TDUs)
attributor → aggregator (batch signals per TDU over time window)
aggregator → emitter (submit MsgUpdateFitnessBatch on-chain)
```

### Aggregation Window

- Collect signals for 1 hour (configurable)
- Average per-TDU signals within window
- Submit batch update once per window
- Minimum signal count threshold: 3 signals per TDU to emit (avoid noise)

## Tests

- Test: collector parses API gateway log format
- Test: attributor finds correct similar TDUs from query embedding
- Test: positive rating → positive signal to attributed TDUs
- Test: retry pattern → negative signal
- Test: aggregator batches correctly within time window
- Test: below minimum threshold → no signal emitted

## Constraints

- Embedding model for similarity: use same dedup model from R39-2
- Privacy: strip user identity before attribution — only aggregate patterns
- Signal weight in fitness: 0.3 (30%)
