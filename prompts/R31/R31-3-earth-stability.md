# R31-3 — 土 Earth (Stability): Governance as the Center

## Phase Identity

Earth is the center — the phase of stability, grounding, and accumulation. Governance is earth: proposals accumulate support, parameters ground the system's behavior, and the LIP process provides the stable bedrock that all other modules stand on. Earth doesn't move — it holds.

## Relationships

### Generates → Metal (Stability creates structure)

**Status:** ✅ Already exists. Governance parameter updates (`MsgUpdateParams`) directly configure capture_defense thresholds, qualification requirements, and ontology structures. This is the primary generating relationship.

**Verify only:** Write a test confirming that a governance proposal to change `HhiThreshold` in capture_defense correctly takes effect after proposal passes. This should already work via the standard Cosmos SDK governance flow.

### ← Controlled by Wood (Growth disrupts stability)

**Status:** ⚠️ Partial. R31-1 adds a growth pressure signal to alignment, but governance itself doesn't experience pressure from knowledge growth.

**New connection:** When knowledge growth rate is unsustainably high (pending verification ratio > threshold), governance should automatically enter a **stability mode** — expedited voting on parameter adjustment proposals related to knowledge/verification.

This is NOT auto-governance. This is a signal that makes certain proposal types eligible for expedited processing:

```go
// In governance keeper's proposal handling:
func (k Keeper) GetEffectiveVotingPeriod(ctx context.Context, proposal *types.Proposal) uint64 {
    params := k.GetParams(ctx)
    basePeriod := params.VotingPeriodBlocks
    
    // Check if this is a parameter adjustment proposal for knowledge/verification
    if !isKnowledgeParamProposal(proposal) {
        return basePeriod
    }
    
    // Wood controls Earth: excessive growth pressure → expedited governance
    if k.alignmentKeeper != nil {
        health := k.alignmentKeeper.GetHealthCategory(ctx)
        if health == "degraded" || health == "critical" {
            // 50% voting period for knowledge-related proposals during system stress
            return basePeriod / 2
        }
    }
    
    return basePeriod
}

func isKnowledgeParamProposal(proposal *types.Proposal) bool {
    // Check if the proposal targets knowledge, verification, or alignment params
    // Based on the proposal's module target
    return proposal.TargetModule == "knowledge" || 
           proposal.TargetModule == "alignment" ||
           proposal.TargetModule == "capture_defense"
}
```

**New keeper dependency in governance:**
```go
type AlignmentKeeper interface {
    GetHealthCategory(ctx context.Context) string
}
```

This is a light touch — governance doesn't auto-propose, it just processes faster when the system is stressed. The validators still vote. Earth's stability is respected, but Wood's pressure is acknowledged.

### Controls → Water (Stability constrains flow)

**Status:** ⚠️ Partial. Emergency halt stops everything, but there's no graduated governance control over social formation. Governance can change partnership params, but there's no dynamic governance pressure on social activity.

**New connection:** Governance proposals that pass for a domain can impose a **formation cooldown** — a period where new partnerships in that domain are paused. This is the governance equivalent of "we need to think about this domain before more people pile in."

```go
// New governance message type:
// MsgDomainFormationFreeze { authority, domain, duration_blocks, reason }
// Only executable via governance proposal

func (k msgServer) DomainFormationFreeze(ctx context.Context, msg *types.MsgDomainFormationFreeze) (*types.MsgDomainFormationFreezeResponse, error) {
    if msg.Authority != k.authority {
        return nil, types.ErrUnauthorized
    }
    
    sdkCtx := sdk.UnwrapSDKContext(ctx)
    expiryHeight := uint64(sdkCtx.BlockHeight()) + msg.DurationBlocks
    
    // Signal to partnerships module
    if k.partnershipsKeeper != nil {
        k.partnershipsKeeper.SetDomainFormationFreeze(ctx, msg.Domain, expiryHeight, msg.Reason)
    }
    
    sdkCtx.EventManager().EmitEvent(
        sdk.NewEvent("zerone.gov.domain_formation_freeze",
            sdk.NewAttribute("domain", msg.Domain),
            sdk.NewAttribute("duration_blocks", fmt.Sprintf("%d", msg.DurationBlocks)),
            sdk.NewAttribute("expiry_height", fmt.Sprintf("%d", expiryHeight)),
            sdk.NewAttribute("reason", msg.Reason),
        ),
    )
    
    return &types.MsgDomainFormationFreezeResponse{}, nil
}
```

In partnerships, check the freeze:
```go
func (k Keeper) CanFormPartnership(ctx context.Context, domain string) error {
    freeze := k.GetDomainFormationFreeze(ctx, domain)
    if freeze != nil {
        height := uint64(sdk.UnwrapSDKContext(ctx).BlockHeight())
        if height < freeze.ExpiryHeight {
            return fmt.Errorf("domain %s is under formation freeze until block %d: %s", 
                domain, freeze.ExpiryHeight, freeze.Reason)
        }
        // Freeze expired — clear it
        k.DeleteDomainFormationFreeze(ctx, domain)
    }
    return nil
}
```

**New store in partnerships:**
```
DomainFormationFreeze {
    Domain       string
    ExpiryHeight uint64
    Reason       string
}
```
Key: `formation_freeze/{domain}`

**New keeper dependency:**
```go
// In governance's expected_keepers.go:
type PartnershipsKeeper interface {
    SetDomainFormationFreeze(ctx context.Context, domain string, expiryHeight uint64, reason string)
}
```

**New proto message:**
Add `MsgDomainFormationFreeze` and response to governance tx.proto. Register in msg_server.go.

## Events

```
zerone.gov.expedited_voting {
    proposal_id: uint64
    target_module: string
    health_category: string
    base_voting_period: uint64
    effective_voting_period: uint64
}

zerone.gov.domain_formation_freeze {
    domain: string
    duration_blocks: uint64
    expiry_height: uint64
    reason: string
}

zerone.partnerships.formation_blocked {
    domain: string
    freeze_expiry: uint64
    freeze_reason: string
    requester: string
}
```

## Tests

1. **Earth → Metal (verify existing):** Governance proposal to change capture_defense HhiThreshold → new threshold takes effect.
2. **Wood → Earth:** Alignment health = degraded → knowledge-related proposals get 50% voting period.
3. **Wood → Earth:** Alignment health = healthy → normal voting period (no expediting).
4. **Wood → Earth:** Non-knowledge proposals → normal voting period even during degradation.
5. **Earth → Water:** DomainFormationFreeze governance proposal → partnerships in that domain blocked.
6. **Earth → Water:** Freeze expires → partnerships resume normally.
7. **Earth → Water:** Freeze on domain A doesn't affect domain B.

## What This Changes

Before R31-3: Governance is a static parameter-setting mechanism. It doesn't respond to system health. It can't target social formation in specific domains.

After R31-3: Earth responds to Wood's pressure (expedited voting during system stress) and controls Water (domain-specific formation freezes). Governance becomes adaptive — it processes urgent matters faster when the system needs it, and it can surgically constrain social formation where needed. Earth holds the center but breathes with the system.
