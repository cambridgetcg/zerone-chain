# R10-3 — Event Emission & Indexing

## Goal

Audit and standardize event emission across all 32 modules. Ensure every state-changing
operation emits queryable events. Set up websocket subscriptions for real-time monitoring.
Make the chain explorer-friendly.

## Working Directory

`/Users/yuai/Desktop/Zerone`

## Deliverables

### 1. Event Audit

For each of the 32 custom modules, verify that every message handler emits events:
```go
ctx.EventManager().EmitEvent(sdk.NewEvent(
    "zerone.knowledge.fact_added",
    sdk.NewAttribute("fact_id", fact.Id),
    sdk.NewAttribute("domain", fact.Domain),
    sdk.NewAttribute("submitter", msg.Submitter),
))
```

Create a checklist:

| Module | Msg Handlers | Events Emitted | Missing |
|--------|-------------|----------------|---------|
| auth | 11 | ? | ? |
| staking | 6 | ? | ? |
| knowledge | 12 | ? | ? |
| ... | ... | ... | ... |

Add missing events. Each event should include:
- Event type: `zerone.<module>.<action>` (e.g., `zerone.knowledge.fact_verified`)
- Relevant attributes: IDs, addresses, amounts, statuses

### 2. Event Types Documentation

Create `docs/EVENTS.md` documenting every event type:
```markdown
## zerone.knowledge.claim_submitted
- `claim_id` — unique claim identifier
- `submitter` — bech32 address of submitter
- `domain` — knowledge domain
- `content_hash` — hash of claim content

## zerone.knowledge.fact_verified
- `fact_id` — fact that was verified
- `confidence` — new confidence score (BPS)
- `verifier_count` — number of verifiers in round
```

### 3. BeginBlock / EndBlock Events

Ensure block lifecycle events are emitted:
- Autopoiesis: `zerone.autopoiesis.epoch_processed` (multiplier changes, SSI score)
- Alignment: `zerone.alignment.observation_recorded` (AHI, dimension scores)
- Vesting rewards: `zerone.vesting.block_reward_distributed` (amount, recipients)
- Schedule: `zerone.schedule.executed` (schedule ID, result)
- Emergency: `zerone.emergency.ceremony_advanced` (ceremony ID, phase)

### 4. WebSocket Subscription Guide

Document how to subscribe to events:
```bash
# Subscribe to all knowledge events
wscat -c ws://localhost:26657/websocket
> {"jsonrpc":"2.0","method":"subscribe","id":1,"params":{"query":"tm.event='Tx' AND zerone.knowledge.claim_submitted.submitter EXISTS"}}

# Subscribe to new blocks
> {"jsonrpc":"2.0","method":"subscribe","id":2,"params":{"query":"tm.event='NewBlock'"}}
```

### 5. Transaction Indexing

Verify CometBFT indexer config supports event queries:
```toml
# config.toml
[tx_index]
indexer = "kv"
```

Test that historical event queries work:
```bash
curl "http://localhost:26657/tx_search?query=\"zerone.knowledge.fact_verified.fact_id='axiom-001'\"&prove=true"
```

### 6. Block Explorer Compatibility

Ensure the chain is compatible with standard Cosmos block explorers:
- [Mintscan](https://mintscan.io) compatibility (standard event format)
- [ping.pub](https://ping.pub) compatibility (CosmosDirectory format)
- Document any custom explorer requirements for Zerone-specific features

## Tests

1. Every message handler emits at least one event (audit test)
2. Events are queryable via CometBFT tx_search
3. BeginBlock/EndBlock events emitted at correct intervals
4. Event attributes are non-empty and correctly typed

## Constraints

- Event type format: `zerone.<module>.<action>` (lowercase, dot-separated)
- All attribute values must be strings (CometBFT requirement)
- Events must be deterministic (same input → same events)
- No sensitive data in events (no private keys, no raw claim content)
- Keep event count reasonable (avoid flooding the index)
