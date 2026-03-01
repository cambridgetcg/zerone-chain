# R31-5: Water Flow — Social Formation as Nourishment Design

## Overview

Wire three Wu Xing relationships through the social formation layer:
1. **Water -> Wood**: Mentorship graduation produces knowledge dividends and expands domain carrying capacity
2. **Earth -> Water**: Governance param changes take immediate effect on formation matching
3. **Water -> Fire**: Social density tracking with informational events when benefit threshold is crossed

## Approach

Direct state injection and cross-module keeper wiring. New `KnowledgeKeeper` interface in partnerships, `ApplyMentorshipDividend` in knowledge, param-change height tracking for matching reset, social benefit status tracking with events.

## Changes

### Knowledge module
- Add `MentorshipGraduations` field to `DomainStats` struct
- Add `MentorshipDividendEnergy` (default 50,000) and `MentorshipCapacityBonus` (default 5) params at proto fields 129-130
- New `ApplyMentorshipDividend(ctx, domain, mentor, mentee)` method
- Modify `GetDomainCarryingCapacity` to include mentorship bonus

### Partnerships module
- New `KnowledgeKeeper` interface in `expected_keepers.go` with `ApplyMentorshipDividend`
- Add `knowledgeKeeper` field + `SetKnowledgeKeeper` setter
- Call dividend in `graduateMentorship()`
- Add `SocialSaturationThreshold` param (default 4) at proto field 21
- Add `LastParamUpdateHeight` KV store tracking
- Modify `SetParams` to record update height
- Modify `RunFormationMatching` to reset cycle on param change
- New `GetDomainSocialBenefitStatus(ctx, domain) bool`
- Emit `social_benefit_lost`/`social_benefit_achieved` events in `SettleCoolingPartnerships`

### App wiring
- Wire partnerships -> knowledge keeper adapter in app.go
