# R40-4 — Reputation Wiring

## Objective

Wire `reputation.go` into the quality round flow and BeginBlocker so reputation accumulates from honest curation and decays during inactivity, and reputation weights influence vote aggregation.

## Context

- `reputation.go` has: `UpdateReputation()`, `ResetInactivityTimer()`, `ApplyReputationDecay()`, `GetAgentDomainReputation()`
- Reputation is per-agent, per-domain (e.g., agent X has different rep in "code" vs "math")
- 5% decay per month of inactivity, floor at 25% of peak
- Any successful submission or review resets the inactivity timer

## Tasks

1. **On successful submission** (`AggregateQualityRound()` when ACCEPT):
   - Call `UpdateReputation(ctx, submitter, domainID, +reputationGain)` — gain proportional to difficulty
   - Call `ResetInactivityTimer(ctx, submitter, domainID, currentHeight)`

2. **On successful review** (majority-side reviewer in aggregation):
   - Call `UpdateReputation(ctx, reviewer, domainID, +smallGain)` — reviewers gain less than submitters
   - Call `ResetInactivityTimer(ctx, reviewer, domainID, currentHeight)`

3. **On bad review** (minority-side reviewer):
   - Call `UpdateReputation(ctx, reviewer, domainID, -reputationPenalty)` — lose reputation for wrong calls
   - Do NOT reset timer (bad reviews don't count as "active")

4. **Vote weight in quality rounds**:
   - In `AggregateQualityRound()`, when computing majority/minority, weight each reviewer's vote by their domain reputation
   - `vote_weight = base_weight + reputation_score * reputation_multiplier`
   - Default: `reputation_multiplier = 2.0` (high-rep reviewer's vote counts up to 3× a new reviewer's)
   - Cold start: new reviewers with 0 reputation still get `base_weight = 1.0`

5. **In BeginBlocker** (every `DecayIntervalBlocks` ≈ monthly):
   - Call `ApplyReputationDecay(ctx, currentHeight)` — iterate all records, apply 5% decay to inactive agents
   - Floor enforcement: never below 25% of `PeakScore`

6. **Domain resolution**:
   - Determine domain from submission metadata (category/domain field)
   - If no domain specified, use "general" as default domain

## Tests

- Test: accepted submission increases submitter reputation
- Test: majority reviewer gains small reputation
- Test: minority reviewer loses reputation
- Test: high-rep reviewer's vote has more weight in aggregation
- Test: inactivity for DecayInterval → 5% decay applied
- Test: reputation never drops below 25% of peak
- Test: successful review resets inactivity timer
- Test: bad review does NOT reset timer

## Key Files

- `x/knowledge/keeper/reputation.go` — call into
- `x/knowledge/keeper/quality_round.go` — add rep updates + weighted voting
- `x/knowledge/keeper/ecology.go` — add decay to BeginBlocker
- `x/knowledge/types/reputation.go` — types

## Constraints

- Do NOT modify proto files
- DecayIntervalBlocks default: 432,000 blocks (~30 days at 6s/block)
- Reputation gains/penalties should be configurable via params
