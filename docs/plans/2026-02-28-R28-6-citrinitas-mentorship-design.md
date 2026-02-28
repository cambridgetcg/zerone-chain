# R28-6 Citrinitas: Mentorship & Formation Pool — Design

## Overview

Add mentorship lifecycle and active formation pool matching to the partnerships module. Mentorship is a separate entity (not a partnership variant) that can optionally lead to a partnership after graduation. Formation pool matching proposes compatible pairs; both must accept.

## Decisions Made

- **Mentorship is a separate entity** — own storage, own lifecycle, independent of Partnership. On graduation, optionally proposes a new Partnership.
- **Matching lives in x/partnerships** — pool entries already there; query discovery for reputation/domain data later.
- **Qualification wiring deferred** — the qualification module doesn't have a discount mechanism today; we track domain but don't wire discount logic.
- **Rewards tracked, payouts deferred** — teaching bonus and graduation bonus fields exist on the struct but no token transfers in this iteration.
- **Addresses are role-agnostic** — don't assume mentor=human. Auto-propose partnership uses addresses directly.

## Data Model

### Mentorship (new proto type in types.proto)

```proto
message Mentorship {
  string id                      = 1;
  string mentor_addr             = 2;
  string mentee_addr             = 3;
  string domain                  = 4;
  string status                  = 5;  // proposed, active, graduated, terminated
  uint64 start_block             = 6;
  uint64 duration_blocks         = 7;
  uint64 mentee_verifications    = 8;
  uint64 mentee_claims_submitted = 9;
  uint64 graduation_threshold    = 10;
  uint64 graduation_claims_req   = 11;
}
```

### FormationMatch (new proto type in types.proto)

```proto
message FormationMatch {
  string id             = 1;
  string addr1          = 2;
  string addr2          = 3;
  uint64 score          = 4;
  uint64 proposed_at    = 5;
  uint64 expires_at     = 6;
  string status         = 7;  // proposed, accepted, declined, expired
  bool   addr1_accepted = 8;
  bool   addr2_accepted = 9;
}
```

### Storage Keys (keys.go)

- `MentorshipKeyPrefix` — already exists at `0x13`
- `ByMentorIndexPrefix = 0x15`
- `ByMenteeIndexPrefix = 0x16`
- `FormationMatchKeyPrefix = 0x17`

### Genesis Extension

Add `repeated Mentorship mentorships` and `repeated FormationMatch formation_matches` to GenesisState.

### New Params

```proto
uint64 graduation_verifications       = 14; // default: 20
uint64 graduation_claims              = 15; // default: 5
uint64 max_mentorships_per_mentor     = 16; // default: 3
uint64 formation_match_interval_blocks = 17; // default: 100
uint64 match_acceptance_blocks        = 18; // default: 200
bool   auto_propose_partnership_on_graduation = 19; // default: true
```

## Mentorship Lifecycle

### Messages

| Message | Signer | Fields | Validation |
|---------|--------|--------|------------|
| MsgProposeMentorship | mentor | mentor, mentee, domain, duration_blocks | No self-mentorship, mentor < max active, mentee not already mentored |
| MsgAcceptMentorship | mentee | mentee, mentorship_id | Status must be "proposed", mentee must match |
| MsgGraduateMentee | mentor | mentor, mentorship_id | Status must be "active", mentor must match |
| MsgEndMentorship | sender | sender, mentorship_id | Status must be "proposed" or "active", sender must be mentor or mentee |

### State Machine

```
proposed → active (on accept)
active → graduated (manual by mentor, or auto by EndBlocker)
proposed/active → terminated (end by either party)
```

### Keeper Methods (mentorship.go)

- `SetMentorship()`, `GetMentorship()`, `DeleteMentorship()` — CRUD
- `GetMentorshipsByMentor()`, `GetMentorshipsByMentee()` — index queries
- `CountActiveMentorshipsForMentor()` — enforce max
- `GetActiveMentorshipForMentee()` — at most 1 active per mentee
- `GraduateMentorship()` — status → "graduated", emit event, optionally propose partnership

### EndBlocker: AutoGraduateMentorships

Iterates active mentorships. If `currentBlock >= startBlock + durationBlocks` → graduate.

### Auto-Propose Partnership

When `AutoProposePartnershipOnGraduation` is true and a mentorship graduates, create a pending Partnership between mentor and mentee addresses. Uses addresses directly without assuming roles.

## Formation Pool Matching

### Matching Engine (EndBlocker, every FormationMatchIntervalBlocks)

1. Gather active, unmatched pool entries
2. Sort deterministically by address
3. Cap at 200 entries for scoring (prevents EndBlocker gas spikes)
4. Score all pairs, greedily assign top matches
5. Create FormationMatch with acceptance window

### Scoring (basis points, max 10000)

- Domain overlap: `(shared_domains / max(len1, len2)) * 5000`
- Preferred role compatibility: 3000 (complementary), 1500 (any), 0 (same-seeking-same)
- Time in pool: `min(time_in_pool / 1000, 2000)`

### New Messages

| Message | Signer | Fields | Effect |
|---------|--------|--------|--------|
| MsgAcceptMatch | accepter | accepter, match_id | Marks acceptance. Both accepted → auto-propose partnership, remove from pool |
| MsgDeclineMatch | decliner | decliner, match_id | Status → "declined", both return to unmatched in pool |

### Match Expiry

EndBlocker: `ExpireFormationMatches()` — matches past `MatchAcceptanceBlocks` → status "expired", entries return to pool.

## Tests

### Mentorship Tests
- ProposeAndAccept — lifecycle happy path
- SelfMentorshipBlocked — can't mentor yourself
- MaxMentorshipsEnforced — cap at 3
- AutoGraduation — duration expiry triggers graduation
- ManualGraduation — mentor calls graduate
- EarlyTermination — either party can end
- AutoProposePartnership — graduation proposes partnership when param enabled

### Formation Matching Tests
- MatchingRunsAtInterval — only runs every N blocks
- CompatiblePairsMatched — domain overlap scored correctly
- BothAcceptFormsPartnership — dual-accept flow
- DeclineReturnsToPool — decline flow
- MatchExpiry — expired matches cleaned up
- CappedAt200 — scoring capped for gas safety

## CLI Commands

### Tx
- `propose-mentorship [mentee] [domain] [duration]`
- `accept-mentorship [mentorship-id]`
- `graduate-mentee [mentorship-id]`
- `end-mentorship [mentorship-id]`
- `accept-match [match-id]`
- `decline-match [match-id]`

### Query
- `mentorship [id]`
- `mentorships-by-address [addr]`
- `formation-matches`

## Files to Create/Modify

- `proto/zerone/partnerships/v1/types.proto` — Mentorship, FormationMatch
- `proto/zerone/partnerships/v1/tx.proto` — 6 new messages
- `proto/zerone/partnerships/v1/genesis.proto` — add mentorships, matches, params
- `proto/zerone/partnerships/v1/query.proto` — new queries
- Regenerate `types.pb.go` etc.
- `x/partnerships/types/keys.go` — new prefixes
- `x/partnerships/keeper/mentorship.go` — new CRUD + lifecycle
- `x/partnerships/keeper/formation_matching.go` — matching engine
- `x/partnerships/keeper/msg_server.go` — 6 new handlers
- `x/partnerships/module.go` — EndBlocker additions
- `x/partnerships/client/cli/tx.go` — 6 new commands
- `x/partnerships/client/cli/query.go` — 3 new commands
- `x/partnerships/keeper/keeper_test.go` — 13 new tests
- `x/partnerships/keeper/genesis.go` — mentorship + match import/export
