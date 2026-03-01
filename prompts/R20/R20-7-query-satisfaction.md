# R20-7 — Query Satisfaction: Relevance Feedback Loop

## Context

The knowledge ecology tracks **whether** agents query facts (`QueryCount`, `QueryCountEpoch`) and **whether** queries return results (`DemandSignal.FulfilledCount` vs `UnfulfilledCount`). But it has no signal for **quality of fulfillment** — a fact queried 100 times that disappoints every querier gets the same fitness boost as one that genuinely helps.

This creates a blind spot: popularity ≠ utility. A fact could be the only result in a niche, get queried often by necessity, and still be mediocre. The ecology can't distinguish between "I found what I needed" and "I found something but it was useless."

## The Mechanism

**Query Satisfaction** is a lightweight relevance feedback signal where queriers rate results after consuming them. The rating feeds directly into fitness calculation, creating selection pressure toward genuinely useful facts — not just frequently accessed ones.

### Biology Analogy

| Biology | Query Satisfaction |
|---------|-------------------|
| Prey nutritional value | Satisfaction score |
| Predator food preference | Agent rating behavior |
| Empty calories | High query count, low satisfaction |
| Nutrient-rich food | High query count, high satisfaction |

A fact that gets queried a lot but rated poorly is junk food — calories without nutrition. The ecology should route energy toward nutritious facts.

## Task

### 1. Proto: Add Satisfaction Fields to Fact

In `proto/zerone/knowledge/v1/types.proto`, add to the `Fact` message after `query_count_epoch` (field 33):

```proto
  // ─── Satisfaction feedback ──────────────────────────────────────────────
  uint64 satisfaction_up         = 60;  // Lifetime positive ratings
  uint64 satisfaction_down       = 61;  // Lifetime negative ratings
  uint64 satisfaction_up_epoch   = 62;  // Positive ratings this epoch (resets)
  uint64 satisfaction_down_epoch = 63;  // Negative ratings this epoch (resets)
```

Use field numbers in the 60s to avoid collision with existing fields. Check the highest existing field number first.

### 2. Proto: Add MsgRateFact Transaction

In `proto/zerone/knowledge/v1/tx.proto`, add a new message and register it with the `Msg` service:

```proto
// MsgRateFact allows a querier to provide relevance feedback on a fact.
// The querier must have previously queried this fact (enforced by the query receipt system).
message MsgRateFact {
  option (cosmos.msg.v1.signer) = "rater";

  string rater   = 1;   // Address of the rating agent
  string fact_id = 2;   // Fact being rated
  bool   useful  = 3;   // true = satisfied, false = dissatisfied
  string memo    = 4;   // Optional: brief reason (max 256 chars, for future analysis)
}

message MsgRateFactResponse {}
```

Register in the `Msg` service:

```proto
rpc RateFact(MsgRateFact) returns (MsgRateFactResponse) {
  option (cosmos.msg.v1.service) = true;
};
```

### 3. Query Receipt System

Prevent rating spam by requiring proof-of-query. When a fact is queried via gRPC (`GetFact` with `track_query = true`), record a **query receipt** — a lightweight store entry proving the querier accessed the fact.

In `x/knowledge/types/keys.go`, add:

```go
var QueryReceiptPrefix = []byte{0x30}

// QueryReceiptKey returns the key for a query receipt: 0x30 | rater | factID
func QueryReceiptKey(rater, factID string) []byte {
    key := append(QueryReceiptPrefix, []byte(rater)...)
    key = append(key, '/')
    key = append(key, []byte(factID)...)
    return key
}
```

In `x/knowledge/keeper/satisfaction.go` (new file):

```go
// RecordQueryReceipt stores proof that an address queried a specific fact.
// Receipts are ephemeral — cleared at epoch boundaries to bound storage.
func (k Keeper) RecordQueryReceipt(ctx context.Context, rater, factID string) error {
    store := k.storeService.OpenKVStore(ctx)
    sdkCtx := sdk.UnwrapSDKContext(ctx)
    // Store block height as receipt value (for optional staleness checks)
    bz := sdk.Uint64ToBigEndian(uint64(sdkCtx.BlockHeight()))
    return store.Set(types.QueryReceiptKey(rater, factID), bz)
}

// HasQueryReceipt checks if an address has a valid query receipt for a fact.
func (k Keeper) HasQueryReceipt(ctx context.Context, rater, factID string) bool {
    store := k.storeService.OpenKVStore(ctx)
    bz, err := store.Get(types.QueryReceiptKey(rater, factID))
    return err == nil && bz != nil
}

// ClearQueryReceipts deletes all query receipts. Called at epoch boundaries.
func (k Keeper) ClearQueryReceipts(ctx context.Context) {
    store := k.storeService.OpenKVStore(ctx)
    iter, err := store.Iterator(types.QueryReceiptPrefix, prefixEndBytes(types.QueryReceiptPrefix))
    if err != nil {
        return
    }
    defer iter.Close()
    var keys [][]byte
    for ; iter.Valid(); iter.Next() {
        keys = append(keys, append([]byte{}, iter.Key()...))
    }
    for _, key := range keys {
        _ = store.Delete(key)
    }
}
```

### 4. Wire Query Receipt into gRPC Query

Modify the `GetFact` gRPC handler in `x/knowledge/keeper/grpc_query.go`:

The existing `QueryFactRequest` proto needs a new field:

```proto
message QueryFactRequest {
  string id          = 1;
  bool   track_query = 2;  // If true, increment query counter + record receipt
  string querier     = 3;  // Address of the querier (required if track_query=true)
}
```

In the handler:

```go
func (q *queryServer) Fact(ctx context.Context, req *types.QueryFactRequest) (*types.QueryFactResponse, error) {
    // ... existing validation ...

    fact, found := q.keeper.GetFact(ctx, req.Id)
    if !found {
        return nil, status.Error(codes.NotFound, "fact not found")
    }

    if req.TrackQuery {
        q.keeper.IncrementFactQueryCount(ctx, req.Id)
        if req.Querier != "" {
            _ = q.keeper.RecordQueryReceipt(ctx, req.Querier, req.Id)
        }
    }

    return &types.QueryFactResponse{Fact: fact}, nil
}
```

### 5. MsgRateFact Handler

In `x/knowledge/keeper/msg_server.go`, add:

```go
func (m *msgServer) RateFact(ctx context.Context, msg *types.MsgRateFact) (*types.MsgRateFactResponse, error) {
    // Validate memo length
    if len(msg.Memo) > 256 {
        return nil, fmt.Errorf("memo exceeds 256 characters")
    }

    // Verify fact exists
    fact, found := m.keeper.GetFact(ctx, msg.FactId)
    if !found {
        return nil, fmt.Errorf("fact not found: %s", msg.FactId)
    }

    // Verify query receipt (proof-of-query)
    if !m.keeper.HasQueryReceipt(ctx, msg.Rater, msg.FactId) {
        return nil, fmt.Errorf("no query receipt: you must query a fact before rating it")
    }

    // Prevent double-rating: consume the receipt
    if err := m.keeper.ConsumeQueryReceipt(ctx, msg.Rater, msg.FactId); err != nil {
        return nil, fmt.Errorf("failed to consume receipt: %w", err)
    }

    // Apply rating
    if msg.Useful {
        fact.SatisfactionUp++
        fact.SatisfactionUpEpoch++
    } else {
        fact.SatisfactionDown++
        fact.SatisfactionDownEpoch++
    }

    if err := m.keeper.SetFact(ctx, fact); err != nil {
        return nil, fmt.Errorf("failed to update fact: %w", err)
    }

    sdkCtx := sdk.UnwrapSDKContext(ctx)
    sdkCtx.EventManager().EmitEvent(sdk.NewEvent(
        "zerone.knowledge.fact_rated",
        sdk.NewAttribute("fact_id", msg.FactId),
        sdk.NewAttribute("rater", msg.Rater),
        sdk.NewAttribute("useful", fmt.Sprintf("%t", msg.Useful)),
    ))

    return &types.MsgRateFactResponse{}, nil
}
```

Add `ConsumeQueryReceipt` to `satisfaction.go`:

```go
// ConsumeQueryReceipt deletes a query receipt (one rating per query).
func (k Keeper) ConsumeQueryReceipt(ctx context.Context, rater, factID string) error {
    store := k.storeService.OpenKVStore(ctx)
    return store.Delete(types.QueryReceiptKey(rater, factID))
}
```

### 6. Integrate into Fitness Calculation

Add a **satisfaction component** to `CalculateFitness` in `x/knowledge/keeper/fitness.go`:

```go
// ─── Satisfaction component ────────────────────────────
// Satisfaction ratio: up / (up + down), scaled to 0-1M BPS
// Default to neutral (500k) if no ratings yet — don't penalize unrated facts
satisfactionScore := uint64(500_000) // neutral default
totalRatings := fact.SatisfactionUpEpoch + fact.SatisfactionDownEpoch
if totalRatings >= params.SatisfactionMinRatings { // require minimum sample size
    satisfactionScore = safeMulDiv(fact.SatisfactionUpEpoch, 1_000_000, totalRatings)
}
```

Add to the weighted sum:

```go
fitness += safeMulDiv(satisfactionScore, params.FitnessWeightSatisfactionBps, 1_000_000)
```

### 7. New Params

Add to `knowledge` module params:

```proto
uint64 fitness_weight_satisfaction_bps = 60;  // Weight of satisfaction in fitness (default: 150_000 = 15%)
uint64 satisfaction_min_ratings        = 61;  // Minimum ratings before satisfaction affects fitness (default: 3)
```

Default values:
- `FitnessWeightSatisfactionBps`: 150,000 (15% of fitness)
- `SatisfactionMinRatings`: 3

When adding the satisfaction weight, reduce `FitnessWeightQueryBps` by 150,000 so total weights still sum to 1,000,000. This rebalances: raw query volume becomes less important now that we have a quality signal.

### 8. Epoch Reset

In `UpdateAllFitnessScores`, add resets for the satisfaction epoch counters alongside the existing `QueryCountEpoch` reset:

```go
fact.SatisfactionUpEpoch = 0
fact.SatisfactionDownEpoch = 0
```

In `BeginBlock` (or wherever epoch processing runs), call `ClearQueryReceipts(ctx)` at epoch boundaries to bound receipt storage.

### 9. Fee for Rating (Anti-Spam)

Rating is a state-writing transaction but should be cheap — we *want* feedback. Apply a minimal fee:

In `x/knowledge/keeper/fees.go`, add:

```go
const RateFactBaseFee = 1000 // 0.001 ZRN — trivial but nonzero
```

Wire this into the `MsgRateFact` handler's fee deduction (same pattern as other knowledge messages).

### 10. Tests

Create `x/knowledge/keeper/satisfaction_test.go`:

1. **TestRecordQueryReceipt** — recording and checking receipt existence
2. **TestConsumeQueryReceipt** — receipt consumed after rating, double-rating fails
3. **TestClearQueryReceipts** — all receipts cleared at epoch
4. **TestRateFactPositive** — query → rate useful → satisfaction_up increments
5. **TestRateFactNegative** — query → rate not useful → satisfaction_down increments
6. **TestRateFactNoReceipt** — rating without prior query fails
7. **TestRateFactDoubleRate** — second rating for same fact fails (receipt consumed)
8. **TestSatisfactionFitnessImpact** — high-satisfaction fact scores better than low-satisfaction
9. **TestSatisfactionMinRatings** — below min threshold, satisfaction score stays neutral
10. **TestSatisfactionEpochReset** — epoch resets clear epoch counters but not lifetime
11. **TestMemoTooLong** — memo > 256 chars rejected

## Design Rationale

**Why binary (useful/not useful) instead of a scale?**
Simpler, faster, harder to game. A 1-5 star system adds complexity and agents will cluster at extremes anyway. Binary maps cleanly to the biological analogy: the prey was either nutritious or not.

**Why require proof-of-query?**
Without it, any agent can carpet-bomb ratings on facts they never read. The receipt system makes ratings expensive to forge — you have to actually query the fact first, which costs gas and increments the query counter.

**Why consume the receipt (one rating per query)?**
Prevents a single query from generating unlimited ratings. Want to rate again? Query again. This also creates a natural correlation: query volume and rating volume stay proportional.

**Why epoch-scoped receipts?**
Receipts are ephemeral — storing them forever would bloat state. Clearing at epoch boundaries keeps storage bounded while giving agents a full epoch window to provide feedback.

**Why neutral default (500k) for unrated facts?**
New facts shouldn't be penalized for lacking ratings. Neutral is fair — they rise or fall on other fitness components until enough agents have rated them.

**Why 15% weight?**
Significant enough to matter but not dominant. Query volume (now reduced) still drives discovery. Satisfaction ensures quality wins over time. Can be adjusted via governance.

## Dependencies

- R20-1 (fitness score) — satisfaction is a new fitness component
- R20-6 (agent demand signal) — satisfaction extends the demand feedback model

## Exit Criteria

1. `MsgRateFact` transaction works end-to-end: query → receipt → rate → fact updated
2. Double-rating fails (receipt consumed)
3. Rating without receipt fails
4. Fitness calculation includes satisfaction component
5. Satisfaction weight defaults to 15%, query weight reduced by 15%
6. Epoch resets clear satisfaction epoch counters and receipts
7. All 11 tests pass
8. `make pr-check` clean
