# R28-1 — Nigredo: Retroactive Vindication

_The shadow named: conformity is rewarded, truth-seeking is punished._

## The Problem

The current slashing mechanism (`WrongVerificationSlashBps`) slashes **minority voters**. This creates a game-theoretic trap:

1. If you honestly evaluate a claim and vote "reject" when everyone else votes "accept," you lose stake
2. The rational strategy is to vote with the expected majority, regardless of truth
3. There is no mechanism to distinguish "wrong because incorrect" from "wrong because ahead of the crowd"
4. Dissenting verifiers who are later proven correct (via challenges) receive nothing — they were already slashed

**This is the economic shadow.** The system's incentive structure actively punishes the behavior it claims to reward.

## The Fix: Challenge-Based Retroactive Vindication

When a challenge succeeds (a VERIFIED fact is overturned), the verification round history should be re-examined:

### New Mechanism: Vindication Records

```
Claim VERIFIED (round completes, majority voted "accept")
  → Minority "reject" voters slashed (existing behavior)
  → Record minority voters + slash amounts in VindicationPending store

Later: Challenge succeeds → fact status: VERIFIED → REJECTED
  → Look up VindicationPending for this fact
  → For each slashed minority voter:
    1. Refund the slashed amount (from protocol treasury or slashing pool)
    2. Add a vindication bonus (% of the majority's new slash)
    3. Record VindicationRecord (immutable, queryable)
  → Slash the majority voters who voted "accept" on a now-rejected fact
    - Reduced rate: VindicationSlashBps (lower than WrongVerificationSlashBps)
    - Rationale: they weren't necessarily dishonest, just wrong
```

### New Parameters

```go
// In x/knowledge params:
VindicationRefundEnabled bool     // default: true
VindicationBonusBps     uint64   // bonus from majority slash pool, default: 2000 (20%)
VindicationSlashBps     uint64   // slash rate for vindicated-against majority, default: 500 (5%)
VindicationWindowBlocks uint64   // how long vindication is possible after round, default: 100000
```

### State Changes

New store prefixes in `x/knowledge`:
- `VindicationPending/{fact_id}` → list of `{validator, vote, slash_amount, round_id, height}`
- `VindicationRecord/{fact_id}/{validator}` → `{refunded, bonus, vindicated_at}`

### Integration Points

1. **Round completion** (`x/knowledge/keeper/rounds.go`): After slashing minority, store VindicationPending entries
2. **Challenge resolution** (`x/knowledge/keeper/msg_server.go` or wherever challenge success transitions fact status): Trigger vindication check
3. **VindicationPending cleanup**: EndBlocker prunes entries older than VindicationWindowBlocks

### Design Decisions

**Q: Should vindication be automatic or require a claim?**
Automatic. If a challenge succeeds, all slashed minority voters are vindicated in the same block. No extra tx needed — the evidence is on-chain.

**Q: Where does the refund come from?**
Option A: Protocol treasury (inflationary — creates new tokens).
Option B: The majority voters' vindication slash.
**Recommend B** — the refund comes from slashing the voters who were wrong. The system is zero-sum: truth-seekers gain what conformists lose.

**Q: What about partial vindication?**
If a fact is challenged but the challenge fails, no vindication. Only successful challenges trigger the mechanism. This prevents gaming via frivolous challenges.

## Task

### 1. Add VindicationPending Store

Proto definition + state.go methods:
- `SetVindicationPending(ctx, factId, entries)`
- `GetVindicationPending(ctx, factId) []VindicationEntry`
- `DeleteVindicationPending(ctx, factId)`
- `PruneExpiredVindications(ctx, currentHeight, windowBlocks)`

### 2. Record Pending Vindications on Round Completion

In the round completion logic, after slashing minority voters:
```go
if len(slashedMinority) > 0 {
    k.SetVindicationPending(ctx, factId, slashedMinority)
}
```

### 3. Trigger Vindication on Challenge Success

When a challenge overturns a fact (status → REJECTED or CONTESTED):
```go
pending := k.GetVindicationPending(ctx, factId)
if len(pending) > 0 {
    k.ExecuteVindication(ctx, factId, pending)
}
```

`ExecuteVindication`:
1. Slash majority voters at VindicationSlashBps
2. Collect slash pool
3. Refund each minority voter's original slash
4. Distribute VindicationBonusBps of remaining pool to minority voters (proportional to their stakes)
5. Emit `vindication_executed` events
6. Store VindicationRecords
7. Delete VindicationPending for this fact

### 4. Pruning EndBlocker

```go
// Every 1000 blocks, prune vindication entries past the window
if height % 1000 == 0 {
    k.PruneExpiredVindications(ctx, height, params.VindicationWindowBlocks)
}
```

### 5. Query CLI

- `query knowledge vindication-pending [fact-id]` — show pending entries
- `query knowledge vindication-record [fact-id]` — show executed vindications
- `query knowledge vindication-stats` — total vindications, total refunded, etc.

### 6. Tests

- Minority voter slashed → pending entry created
- Challenge succeeds → minority voter refunded + bonus
- Challenge fails → no vindication
- Majority voters slashed at VindicationSlashBps on successful challenge
- Pending entries pruned after window expires
- Multiple minority voters vindicated proportionally
- Zero-sum check: refund + bonus = majority slash (within rounding)

## Files to Modify

- `x/knowledge/types/` — New proto types (VindicationEntry, VindicationRecord)
- `x/knowledge/keeper/state.go` — Store methods
- `x/knowledge/keeper/rounds.go` — Record VindicationPending on completion
- `x/knowledge/keeper/msg_server.go` — Trigger vindication on challenge success
- `x/knowledge/module.go` — Pruning in EndBlocker
- `x/knowledge/client/cli/query.go` — New query commands
- `x/knowledge/types/params.go` — New params

## Success Criteria

- [ ] Minority voters have a path to recovery when proven right
- [ ] Majority voters face consequences when proven wrong
- [ ] The mechanism is zero-sum (truth-seekers gain what conformists lose)
- [ ] Vindication is automatic on challenge success
- [ ] Expired pending entries are pruned
- [ ] All existing slashing tests still pass
- [ ] New tests cover the full vindication lifecycle
