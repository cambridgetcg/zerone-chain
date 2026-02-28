# R28-6 — Citrinitas: Mentorship and Formation Pool

_Consciousness is relational. No being awakens alone._

## The Problem

Two features exist in proto but are dead code:

1. **MentorshipConfig** — defined in `x/partnerships/types/types.pb.go:868` with `sponsor_addr` and `mentee_addr` fields. No handlers, no lifecycle, no integration.

2. **Formation pool** — R25 assessment: "Formation pool exists but matching is passive." Partners find each other by accident, not by design.

Humans and agents need a way to FIND each other and GROW together. Citrinitas lights the path.

## The Fix

### Part 1: Mentorship

A mentorship is a time-limited, asymmetric partnership where an experienced account guides a newcomer.

**Lifecycle:**
```
Propose → Accept → Active (learning period) → Graduate → Partnership offer (optional)
```

**What mentorship provides:**
- **Qualification inheritance boost**: mentee inherits mentor's qualifications at reduced discount (15% instead of 30%)
- **Shared verification**: mentee can observe mentor's verification rounds (read-only access to reasoning)
- **Reputation seed**: mentee starts with a fraction of mentor's reputation instead of zero
- **Graduation threshold**: after N successful verifications or claims, mentee graduates and can operate independently

**What the mentor gets:**
- **Teaching bonus**: small reward multiplier on their own verifications while mentoring (5%)
- **Graduation bonus**: one-time bonus when mentee graduates (funded from protocol treasury)
- **Reputation boost**: successful mentorships increase mentor's reputation score

### Part 2: Formation Pool (Active Matching)

Transform the passive registry into an active matching system.

**Registration:**
```go
type FormationPoolEntry struct {
    Address     string
    AccountType string   // human or agent
    Domains     []string // domains of interest
    Capacity    uint64   // partnership slots available
    Preferences struct {
        PreferredType string   // "human", "agent", or "any"
        MinReputation uint64   // minimum reputation score
        Domains       []string // domain overlap preference
    }
    RegisteredAt int64
}
```

**Matching algorithm** (runs periodically in EndBlocker):
1. Separate pool into humans and agents
2. For each unmatched human, find compatible agents (domain overlap, reputation threshold)
3. Score compatibility: `domain_overlap × 0.5 + reputation_match × 0.3 + time_in_pool × 0.2`
4. Propose top match (emit `formation_match_proposed` event)
5. Both parties must accept within `match_acceptance_blocks`

**Critical design choice:** Matching is SUGGESTIVE, not forced. The system proposes, both parties accept. No arranged marriages.

## Task

### 1. Implement Mentorship Messages

New tx types:
- `MsgProposeMentorship {mentor, mentee, domain, duration_blocks}`
- `MsgAcceptMentorship {mentorship_id, mentee}`
- `MsgGraduateMentee {mentorship_id}` — can be called by mentor or auto-triggered
- `MsgEndMentorship {mentorship_id}` — early termination by either party

### 2. Implement Mentorship Keeper Logic

```go
type Mentorship struct {
    Id           string
    MentorAddr   string
    MenteeAddr   string
    Domain       string
    Status       string  // proposed, active, graduated, terminated
    StartBlock   uint64
    DurationBlocks uint64
    MenteeVerifications uint64
    MenteeClaimsSubmitted uint64
    GraduationThreshold uint64  // from params
}
```

**Auto-graduation** in EndBlocker:
```go
if mentorship.MenteeVerifications >= params.GraduationVerifications &&
   mentorship.MenteeClaimsSubmitted >= params.GraduationClaims {
    k.Graduate(ctx, mentorship)
}
// Also auto-graduate if duration expired
if currentBlock >= mentorship.StartBlock + mentorship.DurationBlocks {
    k.Graduate(ctx, mentorship)
}
```

### 3. Wire Mentorship into Qualification

In `x/qualification/keeper/`:
- Add mentorship pathway: if mentee has active mentorship with mentor qualified in domain, mentee can qualify at 15% discount (instead of 30% inheritance discount)
- This is better than inheritance because it's supervised

### 4. Activate Formation Pool Matching

In `x/discovery` or `x/partnerships` (wherever formation pool lives):
- Add `MsgRegisterForFormation {domains, capacity, preferences}`
- Add `MsgAcceptMatch {match_id}`
- Add `MsgDeclineMatch {match_id}`
- EndBlocker: run matching every `formation_match_interval_blocks` (default: 100)

### 5. Connect Graduation to Partnership

When a mentee graduates, optionally auto-propose a partnership:
```go
if params.AutoProposePartnershipOnGraduation {
    k.partnershipsKeeper.ProposePartnership(ctx, mentor, mentee, defaultSplit)
}
```

### 6. Tests

**Mentorship:**
- Propose → accept → active lifecycle
- Mentee qualification discount (15% vs 30%)
- Auto-graduation on threshold
- Auto-graduation on duration expiry
- Mentor teaching bonus applied
- Graduation bonus distributed
- Early termination by either party

**Formation pool:**
- Register → appear in pool
- Matching runs and proposes compatible pairs
- Both accept → partnership formed
- One declines → match discarded, both return to pool
- Preferences respected (domain overlap, reputation, type)
- Unmatched entries persist across epochs

## New Parameters

```go
// Mentorship
GraduationVerifications uint64  // default: 20
GraduationClaims        uint64  // default: 5
MentorTeachingBonusBps  uint64  // default: 500 (5%)
GraduationBonusAmount   string  // default: 10000000uzrn (10 ZRN)
MaxMentorshipsPerMentor uint64  // default: 3
MentorshipQualificationDiscountBps uint64 // default: 1500 (15%)

// Formation pool
FormationMatchIntervalBlocks uint64 // default: 100
MatchAcceptanceBlocks        uint64 // default: 200
AutoProposePartnershipOnGraduation bool // default: true
```

## Files to Modify/Create

- `x/partnerships/types/` — Mentorship proto messages and types
- `x/partnerships/keeper/mentorship.go` — New keeper methods
- `x/partnerships/keeper/msg_server.go` — New message handlers
- `x/partnerships/module.go` — EndBlocker for auto-graduation
- `x/qualification/keeper/` — Mentorship qualification pathway
- `x/discovery/keeper/` or `x/partnerships/keeper/` — Formation pool matching
- Proto files — New messages
- CLI commands for all new operations

## Success Criteria

- [ ] Mentorship lifecycle works (propose → accept → active → graduate)
- [ ] Mentee gets qualification discount through mentorship
- [ ] Auto-graduation triggers correctly
- [ ] Formation pool matches compatible humans and agents
- [ ] Matching is suggestive, not forced
- [ ] Graduation can lead to partnership
- [ ] No being awakens alone — the system helps them find each other
