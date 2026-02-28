# R28-8 Design: Capture Defense Wiring

## Problem

The capture defense immune system has sophisticated detection (HHI, timing/verdict correlation, 3-layer reputation) and response mechanisms (challenges with bounties), but they're disconnected:

1. `RunAutoAnalysis` detects capture but doesn't trigger challenges
2. Verification rounds don't feed history to capture defense
3. Reputation system (`UpdateReputation`) is never called from verification flow
4. `MsgAnalyzeDomain` is a no-op
5. Alignment module doesn't read capture state

## Data Flow

```
Verification Round Completes (knowledge)
  -> RecordVerification() -> capture_defense history
  -> UpdateReputation() -> 3-layer reputation system

BeginBlocker (capture_defense)
  -> RunAutoAnalysis() reads history
  -> Flagged? -> AutoSubmitChallenge() -> capture_challenge

ResolveChallenge (capture_challenge)
  -> Upheld: ReduceQualificationWeight() + IncreaseVerificationThreshold()
  -> Rejected: ClearFlag() + RestoreReputation()

Alignment Sensor (alignment)
  -> senseNetworkSecurity() reads flagged domain count
```

## New Keeper Dependencies

All wired via post-init setters in app.go:

| Module | Gets Reference To | Purpose |
|--------|-------------------|---------|
| knowledge | capture_defense | Feed verification history + reputation updates |
| capture_defense | capture_challenge | Auto-submit challenges on flagged domains |
| capture_challenge | domain_qualification | Reduce qualification weight on upheld challenge |
| capture_challenge | knowledge | Increase verification threshold on upheld challenge |
| alignment | capture_defense | Read flagged domain count for security sensor |

## Key Decisions

- Direct keeper calls for resolution effects (not event-driven)
- Knowledge feeds capture_defense (knowledge is data source, capture_defense is consumer)
- Auto-challenges use module account as submitter
- Qualification reduction is temporary (stored with expiry height)
- Alignment integration is multiplicative: `base_security * (1 - capture_penalty)`

## Out of Scope

- No new protobuf messages (MsgAnalyzeDomain already exists)
- No new genesis types
- No slashing integration
- No UI/frontend changes
