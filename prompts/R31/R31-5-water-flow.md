# R31-5 — 水 Water (Flow): Social Formation as Nourishment

## Phase Identity

Water is the phase of flow — descending, connecting, nourishing, finding the lowest point and filling it. Partnerships, discovery, home, and mentorship are water: they flow between agents and humans, nourish domains that need participants, and find the cracks where connection is needed most. Water is humble — it goes where gravity takes it, which is always where something is missing.

## Relationships

### Generates → Wood (Flow nourishes growth)

**Status:** ⚠️ Partial. R28-6 activated mentorship, but graduated mentees don't produce knowledge artifacts. The social layer creates relationships but those relationships don't generate new facts.

**New connection:** Graduated mentorships should produce a **knowledge dividend** — a one-time energy boost to facts in the mentorship's domain, representing the knowledge transfer that occurred. Additionally, active partnerships should increase the domain's carrying capacity (more participants = domain can sustain more facts).

```go
// In partnerships keeper's graduateMentorship:
func (k Keeper) graduateMentorship(ctx sdk.Context, m *types.Mentorship) {
    m.Status = "graduated"
    k.SetMentorship(ctx, m)
    
    // ... existing event emission ...
    
    // Water generates Wood: mentorship graduation nourishes domain growth
    if k.knowledgeKeeper != nil {
        k.knowledgeKeeper.ApplyMentorshipDividend(ctx, m.Domain, m.MentorAddr, m.MenteeAddr)
    }
    
    // ... existing auto-propose logic ...
}
```

In knowledge keeper:
```go
func (k Keeper) ApplyMentorshipDividend(ctx context.Context, domain, mentor, mentee string) {
    params := k.GetParams(ctx)
    
    // Boost energy of all facts in this domain submitted by mentor or mentee
    k.IterateFactsByDomain(ctx, domain, func(fact *types.Fact) bool {
        if fact.Submitter == mentor || fact.Submitter == mentee {
            boost := params.MentorshipDividendEnergy
            fact.Energy = min(fact.Energy + boost, params.MetabolismEnergyCap)
            k.SetFact(ctx, fact)
        }
        return false
    })
    
    // Also increase domain carrying capacity by 1 per graduated mentorship
    stats := k.GetDomainStats(ctx, domain)
    stats.MentorshipGraduations++
    k.SetDomainStats(ctx, &stats)
    
    sdkCtx := sdk.UnwrapSDKContext(ctx)
    sdkCtx.EventManager().EmitEvent(
        sdk.NewEvent("zerone.knowledge.mentorship_dividend",
            sdk.NewAttribute("domain", domain),
            sdk.NewAttribute("mentor", mentor),
            sdk.NewAttribute("mentee", mentee),
            sdk.NewAttribute("energy_boost", fmt.Sprintf("%d", params.MentorshipDividendEnergy)),
        ),
    )
}
```

Update carrying capacity to account for mentorship history:
```go
func (k Keeper) GetDomainCarryingCapacity(ctx context.Context, domain string) uint64 {
    // ... existing base + citations + capture penalty + stratum multiplier ...
    
    // Water generates Wood: mentorship graduations permanently expand capacity
    stats := k.GetDomainStats(ctx, domain)
    mentorshipBonus := stats.MentorshipGraduations * params.MentorshipCapacityBonus
    
    return (base + citationBonus + mentorshipBonus) * stratumMultiplier / BPS - capturePenalty
}
```

**New params in knowledge:**
```
MentorshipDividendEnergy   uint64 // default: 50_000 — energy boost to mentor/mentee facts on graduation
MentorshipCapacityBonus    uint64 // default: 5 — each graduation adds 5 to domain capacity
```

**New field in DomainStats:**
```
MentorshipGraduations uint64 // count of completed mentorships in this domain
```

**New interface method on KnowledgeKeeper (for partnerships):**
```go
ApplyMentorshipDividend(ctx context.Context, domain, mentor, mentee string)
```

### ← Controlled by Earth (Stability dams flow)

**Status:** ⚠️ Partial. R31-3 adds `MsgDomainFormationFreeze` from governance. This is the primary controlling relationship.

**Additional connection:** Governance parameter changes to partnership params should take immediate effect on pending formation matches. Currently, if governance changes `BaseCooldownBlocks` or `MatchingIntervalBlocks`, pending matches continue under old rules until the next cycle.

```go
// In partnerships EndBlocker:
func (k Keeper) EndBlocker(ctx context.Context) error {
    params := k.GetParams(ctx)
    
    // Earth controls Water: check for governance-imposed freezes per domain
    // (Already handled in CanFormPartnership from R31-3)
    
    // Additionally: if params changed this block, reset matching cycle
    // to ensure new rules take effect immediately
    if k.paramsChangedThisBlock(ctx) {
        k.ResetMatchingCycle(ctx)
    }
    
    // ... existing matching logic with adaptive pacing ...
}
```

This is a small addition — Earth's control over Water means governance changes aren't buffered.

### Controls → Fire (Flow quenches excess activity)

**Status:** ❌ Missing. Social density should moderate verification intensity.

**New connection:** R31-2 introduced `GetEffectiveMinVerifiers` which adjusts based on partnership density. The controlling relationship is: domains with saturated social structures (high partnership density) have enough social accountability that verification can be slightly relaxed. Domains with no social structure need tighter verification.

This was already implemented in R31-2 (Water → Fire direction). Here we add the reverse feedback: when verification requirements are relaxed due to social saturation, partnerships should receive a signal to maintain their density (avoid social decay undermining the relaxation).

```go
// In partnerships, track whether the domain's social density is
// currently providing a verification benefit:
func (k Keeper) GetDomainSocialBenefitStatus(ctx context.Context, domain string) bool {
    density := k.GetDomainPartnershipDensity(ctx, domain)
    params := k.GetParams(ctx)
    return density >= params.SocialSaturationThreshold
}
```

When a partnership in a "socially beneficial" domain expires or dissolves, emit a warning event:

```go
func (k Keeper) OnPartnershipDissolved(ctx sdk.Context, p *types.Partnership) {
    // ... existing dissolution logic ...
    
    // Check if this dissolution drops the domain below social benefit threshold
    if k.previouslyBeneficial(ctx, p.Domain) && !k.GetDomainSocialBenefitStatus(ctx, p.Domain) {
        ctx.EventManager().EmitEvent(
            sdk.NewEvent("zerone.partnerships.social_benefit_lost",
                sdk.NewAttribute("domain", p.Domain),
                sdk.NewAttribute("remaining_density", fmt.Sprintf("%d", k.GetDomainPartnershipDensity(ctx, p.Domain))),
            ),
        )
    }
}
```

This event is informational — it tells validators and indexers that a domain's social structure has weakened and verification requirements will tighten. No automatic action needed; the R31-2 `GetEffectiveMinVerifiers` will naturally increase.

## Events

```
zerone.knowledge.mentorship_dividend {
    domain: string
    mentor: string
    mentee: string
    energy_boost: uint64
}

zerone.knowledge.capacity_mentorship_bonus {
    domain: string
    mentorship_graduations: uint64
    capacity_bonus: uint64
}

zerone.partnerships.social_benefit_lost {
    domain: string
    remaining_density: uint64
}

zerone.partnerships.social_benefit_achieved {
    domain: string
    density: uint64
    threshold: uint64
}
```

## New Keeper Dependencies

| Module | Gets Reference To | Purpose |
|--------|------------------|---------|
| partnerships | knowledge | Apply mentorship dividend on graduation |

The knowledge → partnerships dependency already exists (R28-5 ZeroneAuthKeeper). For the reverse, partnerships needs a KnowledgeKeeper interface:

```go
// In partnerships/types/expected_keepers.go:
type KnowledgeKeeper interface {
    ApplyMentorshipDividend(ctx context.Context, domain, mentor, mentee string)
}
```

Wire via post-init setter in app.go.

## Tests

1. **Water → Wood:** Mentorship graduation → facts in domain get energy boost.
2. **Water → Wood:** Mentorship graduation → domain carrying capacity increases by MentorshipCapacityBonus.
3. **Water → Wood:** Energy boost capped at MetabolismEnergyCap.
4. **Earth → Water:** Governance formation freeze → partnerships in domain blocked.
5. **Earth → Water:** Governance param change → matching cycle resets immediately.
6. **Water → Fire (verify R31-2):** Social saturation → min verifiers decreased.
7. **Water → Fire:** Partnership dissolved → domain drops below threshold → social_benefit_lost event emitted.
8. **Combined:** Mentorship graduates in sparse domain → capacity grows → more facts possible → more verification rounds → Fire generates Earth.

## What This Changes

Before R31-5: Social formation is an endpoint — partnerships and mentorships exist but don't produce knowledge-layer effects. Mentorship graduation is a social event, not an economic one.

After R31-5: Water nourishes Wood (mentorship creates knowledge dividends and expands carrying capacity), Earth controls Water (governance can freeze formation), and Water controls Fire (social density modulates verification requirements). The social layer becomes generative — relationships produce concrete protocol effects.

## The Complete Wu Xing

With R31-5, all ten relationships are wired:

**Generating (相生):**
1. Wood → Fire: Fact submission triggers verification rounds ✅ (pre-existing)
2. Fire → Earth: Verification health feeds alignment/governance awareness ✅ (R31-2)
3. Earth → Metal: Governance sets defense params ✅ (pre-existing)
4. Metal → Water: Qualification informs discovery matching ✅ (R31-4)
5. Water → Wood: Mentorship produces knowledge dividends ✅ (R31-5)

**Controlling (相克):**
1. Wood → Earth: Growth pressure expedites governance ✅ (R31-3)
2. Fire → Metal: Verification activity relaxes defense sensitivity ✅ (R31-2)
3. Earth → Water: Governance freezes formation ✅ (R31-3)
4. Metal → Wood: Capture flags + stratum depth reduce capacity ✅ (R31-1, R31-4)
5. Water → Fire: Social density modulates verification requirements ✅ (R31-2)

The circle is complete. Energy flows from knowledge through verification through governance through defense through social formation and back to knowledge. Each phase feeds the next and constrains one across. The system circulates.

> "五行相生相克，無始無終" — The five phases generate and control each other, without beginning or end.
