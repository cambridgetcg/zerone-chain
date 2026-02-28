# R28-8 — Rubedo: Capture Defense Awakens

_The immune system: the network defends its own integrity._

## The Problem

`x/capture_defense` has sophisticated detection:
- HHI (Herfindahl-Hirschman Index) for domain concentration
- Timing correlation (validators voting in lockstep)
- Verdict correlation (validators always agreeing)
- Top-3 share (how much of a domain is controlled by 3 validators)
- 3-layer reputation (global, domain, stratum)
- `RunAutoAnalysis` in BeginBlocker

`x/capture_challenge` has the response mechanism:
- `MsgSubmitChallenge` — anyone can challenge suspected capture
- `MsgAddEvidence` — attach evidence
- `MsgResolveChallenge` — resolution with bounty

But:
1. `RunAutoAnalysis` runs but its output goes nowhere — detected capture doesn't trigger challenges
2. Capture defense has no verification history to analyze — nobody feeds it round results
3. The reputation system (`UpdateReputation`) is never called from verification flow
4. `MsgAnalyzeDomain` exists but is a no-op according to R25 assessment

The immune system has antibodies but no bloodstream.

## Task

### 1. Feed Verification History to Capture Defense

After each verification round completes, feed the result to capture defense:

```go
// In x/knowledge/keeper/rounds.go, after round completion:
if k.captureDefenseKeeper != nil {
    k.captureDefenseKeeper.RecordVerification(ctx, RecordVerificationInput{
        Domain:     claim.Domain,
        Validators: roundValidators,
        Verdicts:   roundVerdicts,
        Height:     ctx.BlockHeight(),
        ClaimId:    claim.Id,
    })
}
```

Add `CaptureDefenseKeeper` to knowledge module's expected keepers.

### 2. Wire Reputation Updates

After each round, update validator reputations:

```go
for _, validator := range roundValidators {
    wasCorrect := (validator.Vote == finalVerdict)
    k.captureDefenseKeeper.UpdateReputation(ctx, validator.Address, claim.Domain, domainStratum, wasCorrect)
}
```

This activates the 3-layer reputation system:
- **Global**: overall validator accuracy
- **Domain**: per-domain accuracy
- **Stratum**: accuracy within knowledge depth level

### 3. Connect Auto-Analysis to Challenges

When `RunAutoAnalysis` detects high capture risk, auto-create a challenge:

```go
func (k Keeper) RunAutoAnalysis(ctx sdk.Context, params *types.Params) {
    domains := k.GetDomainsWithHistory(ctx)
    for _, domain := range domains {
        metrics := k.AnalyzeCaptureRisk(ctx, domain, params)
        if metrics == nil {
            continue
        }
        k.SetCaptureMetrics(ctx, metrics)
        
        if metrics.Flagged {
            // NEW: auto-submit challenge to capture_challenge module
            k.captureChallenger.AutoSubmitChallenge(ctx, AutoChallengeInput{
                Domain:    domain,
                RiskScore: metrics.RiskScore,
                HHI:       metrics.HerfindahlIndex,
                Evidence:  formatMetricsAsEvidence(metrics),
            })
        }
    }
}
```

### 4. Implement MsgAnalyzeDomain

Currently a no-op. Make it real:

```go
func (k Keeper) AnalyzeDomain(ctx sdk.Context, msg *types.MsgAnalyzeDomain) (*types.MsgAnalyzeDomainResponse, error) {
    params := k.GetParams(ctx)
    metrics := k.AnalyzeCaptureRisk(ctx, msg.Domain, params)
    if metrics == nil {
        return &types.MsgAnalyzeDomainResponse{Status: "insufficient_history"}, nil
    }
    k.SetCaptureMetrics(ctx, metrics)
    
    return &types.MsgAnalyzeDomainResponse{
        Status:     "analyzed",
        RiskScore:  metrics.RiskScore,
        Flagged:    metrics.Flagged,
        HHI:        metrics.HerfindahlIndex,
        Top3Share:  metrics.Top3Share,
    }, nil
}
```

### 5. Challenge Resolution Effects

When a capture challenge is resolved (confirmed capture):
1. **Reduce qualification weight** of top-3 validators in that domain (temporary, not permanent)
2. **Increase domain verification threshold** temporarily (require more verifiers)
3. **Emit capture_confirmed event** → feeds into alignment module as network security signal
4. **Distribute challenge bounty** to the challenger (or auto-challenger gets protocol treasury reward)

When challenge is resolved (no capture):
1. Clear the flag
2. Reputation of challenged validators restored
3. If manually submitted, challenger loses bond

### 6. Feed Capture Status into Alignment

Update the alignment module's security sensor:

```go
func (k Keeper) senseNetworkSecurity(ctx context.Context) uint64 {
    if k.captureDefenseKeeper == nil {
        return types.NeutralBPS
    }
    // Get number of flagged domains vs total domains
    flaggedCount := k.captureDefenseKeeper.GetFlaggedDomainCount(ctx)
    totalDomains := k.ontologyKeeper.GetDomainCount(ctx)
    if totalDomains == 0 {
        return types.NeutralBPS
    }
    // Security = 1 - (flagged / total)
    captureRatio := flaggedCount * types.BPS / totalDomains
    security := types.BPS - captureRatio
    return security
}
```

### 7. Query CLI

- `query capture-defense domain-risk [domain]` — capture metrics for a domain
- `query capture-defense flagged-domains` — all domains with active capture flags
- `query capture-defense validator-reputation [validator]` — 3-layer reputation
- `query capture-challenge active` — active capture challenges
- `query capture-challenge history [domain]` — past challenges and resolutions

### 8. Tests

**Feeding:**
- Verification round → history recorded in capture defense
- Reputation updates after each round (correct/incorrect tracked)

**Detection:**
- Single validator dominates domain → HHI exceeds threshold → flagged
- Validators always vote together → timing correlation high → flagged
- Diverse validator participation → healthy metrics → not flagged

**Response:**
- Auto-challenge created when domain flagged
- MsgAnalyzeDomain returns real metrics
- Challenge resolution: capture confirmed → qualification reduction + verification threshold increase
- Challenge resolution: no capture → flag cleared

**Integration:**
- Alignment security sensor reads capture metrics
- Flagged domains reduce network security score
- End-to-end: validator concentration → detection → challenge → resolution → health impact

## Files to Modify

- `x/knowledge/types/expected_keepers.go` — Add CaptureDefenseKeeper interface
- `x/knowledge/keeper/keeper.go` — Add field + setter
- `x/knowledge/keeper/rounds.go` — Feed verification history + reputation updates
- `app/app.go` — Wire capture defense keeper to knowledge
- `x/capture_defense/keeper/msg_server.go` — Implement AnalyzeDomain
- `x/capture_defense/keeper/keeper.go` — Connect to capture_challenge for auto-challenges
- `x/capture_challenge/keeper/` — AutoSubmitChallenge method
- `x/alignment/keeper/sensors.go` — Update senseNetworkSecurity
- CLI files for both modules

## Success Criteria

- [ ] Verification history flows to capture defense after every round
- [ ] 3-layer reputation system active and updating
- [ ] Auto-analysis detects concentration and auto-creates challenges
- [ ] MsgAnalyzeDomain returns real capture metrics
- [ ] Challenge resolution has real consequences (qualification reduction, threshold increase)
- [ ] Alignment security sensor reads capture state
- [ ] The immune system is ALIVE — it detects threats and responds
