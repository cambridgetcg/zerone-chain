# R17-3 Community Seat Elections + Term Rotation — Design

## Overview

Implement election mechanics for community seats on the research fund committee. Elections are fully self-contained within x/gov, using a new `SeatElectionProposal` type with its own vote storage, tally logic, and BeginBlocker hooks.

## Data Model

### SeatElectionProposal

New protobuf message stored under key prefix `0x0E`:

```protobuf
message SeatElectionProposal {
  uint64 proposal_id         = 1;
  string proposer            = 2;   // bech32 — nominator
  string candidate           = 3;   // bech32 — must be Guardian-tier
  uint32 seat_index          = 4;   // 0 in Phase 1; 0-2 in Phase 2
  string statement           = 5;   // max 2000 chars
  string stage               = 6;   // nominated/accepted/discussion/voting/runoff/passed/failed/expired
  string yes_stake           = 7;
  string no_stake            = 8;
  string abstain_stake       = 9;
  uint64 acceptance_deadline = 10;
  uint64 discussion_end_block = 11;
  uint64 voting_end_block    = 12;
  uint64 created_at_block    = 13;
  bool   candidate_accepted  = 14;
  bool   is_runoff           = 15;  // runoff skips nomination/acceptance, enters voting directly
  repeated uint64 runoff_parent_ids = 16;  // parent proposal IDs that triggered this runoff
}
```

### SeatElectionVote

```protobuf
message SeatElectionVote {
  uint64 proposal_id = 1;
  string voter       = 2;
  string option      = 3;  // "yes", "no", "abstain"
  string stake       = 4;  // uzrn weight at time of vote
  uint64 block       = 5;
}
```

### Store Keys

```
0x0E  SeatElectionKeyPrefix
0x0F  SeatElectionVoteKeyPrefix
0x10  SeatElectionCounterKey
0x11  SeatElectionVoteDedupePrefix
```

### Category Config

Added to `DefaultParams().CategoryConfigs`:
```go
{Category: "research_seat_election", RequiredStakeBps: "500000000", ReviewBlocks: 34272}
// TODO: rename RequiredStakeBps → RequiredStakeUzrn (misnomer storing raw uzrn)
```

## Election Flow

### Standard Election

```
Nomination (500 ZRN stake)
    │
    ▼
Nominated ──[1 day / 34,272 blocks]──▶ Auto-fail (no acceptance)
    │
    ▼ (candidate tx)
Accepted ──[auto-advance]──▶ Discussion (1 day = 34,272 blocks)
    │
    ▼ (timer)
Voting (3 days = 102,816 blocks)
    │
    ▼ (timer)
Resolution: quorum 33.4% + majority 50%
    ├─ Uncontested: passed/failed
    └─ Contested (same seat_index):
         ├─ Highest yes_stake wins (gap > 5%)
         └─ Top 2 within 5% → runoff
```

### Runoff Election

When top 2 candidates' yes_stake are within 5%, a new `SeatElectionProposal` is created with:
- `is_runoff = true`
- `runoff_parent_ids = [parent1_id, parent2_id]`
- Enters `voting` stage directly (skips nomination/acceptance)
- 3-day voting period
- Winner = highest yes_stake (no further runoffs)

### Acceptance Validation

`MsgAcceptSeatNomination{ProposalId, Candidate}` validates:
1. Sender == candidate address
2. Guardian tier (staking keeper: `val.Tier == 4 && val.IsActive`)
3. Not already holding a community seat
4. Has voted on >= 5 distinct LIPs (standard LIP votes only, counted via VoteDedupePrefix)

## Term Management

### Phase 1 (OBSERVER): 1 seat
- Term length: 6,400,000 blocks (~6 months)

### Phase 2 (BALANCED): 3 seats, staggered
- Seat 0: `phase_start + 2,133,333` blocks (~2 months initial)
- Seat 1: `phase_start + 4,266,666` blocks (~4 months initial)
- Seat 2: `phase_start + 6,400,000` blocks (~6 months initial)
- After initial stagger: all terms = 6,400,000 blocks from election

### Term Expiry (BeginBlocker)

Check `state.SeatTermEndBlocks[i]` each block. On expiry:
- Clear seat address to ""
- Emit `zerone.gov.seat_expired` event
- Multisig threshold adjusts (vacant = absent):
  - Phase 1 vacant: 2-of-2 (founder pair)
  - Phase 2, 1 vacant: 3-of-4; 2 vacant: 2-of-3; 3 vacant: 2-of-2

### Vacancy Handling

- Vacant > 1,030,000 blocks (~30 days): emit `seat_vacancy_warning` per epoch
- Vacant > 3,090,000 blocks (~90 days): auto-submit governance notice LIP

## Emergency Removal

`MsgRemoveCommunitySeat{Authority, SeatIndex, Reason}`:
- Requires 75% quorum, 80% support
- Grounds: validator jailed or slashed 3+ times during term
- Clears the seat, emits `zerone.gov.seat_removed` event

## File Structure

### New files
- `x/gov/keeper/seat_election.go` — all election + term logic
- `x/gov/keeper/seat_election_test.go` — tests

### Modified files
- `proto/zerone/gov/v1/types.proto` — SeatElectionProposal, SeatElectionVote
- `proto/zerone/gov/v1/tx.proto` — new messages
- `proto/zerone/gov/v1/query.proto` — new queries
- `x/gov/types/keys.go` — new key prefixes
- `x/gov/types/types.go` — stage constants, category constant
- `x/gov/types/genesis.go` — category config, genesis export/import
- `x/gov/types/expected_keepers.go` — IsGuardian on StakingKeeper
- `x/gov/keeper/state.go` — CRUD for election proposals + votes
- `x/gov/keeper/abci.go` — BeginBlocker hooks for term expiry + election tally
- `x/gov/keeper/msg_server.go` — new message handlers
- `x/gov/keeper/grpc_query.go` — new query handlers
- `x/gov/client/cli/tx.go` — nominate, accept, vote commands
- `x/gov/client/cli/query.go` — seat queries

## Expected Keepers Extension

```go
type StakingKeeper interface {
    GetTotalBondedStake(ctx context.Context) (string, error)
    GetDelegatorTotalBonded(ctx context.Context, addr string) (string, error)
    IsGuardian(ctx context.Context, addr string) (bool, error)  // NEW
}
```

## CLI Commands

```bash
zeroned query gov research-fund-seats
zeroned query gov seat-election [id]
zeroned query gov seat-elections [--stage=voting]
zeroned tx gov nominate-research-seat --candidate=... --seat-index=0 --statement="..." --from=...
zeroned tx gov accept-research-nomination --proposal-id=42 --from=candidate
zeroned tx gov vote-seat-election --proposal-id=42 --option=yes --from=voter
```

## Testing Plan

1. Election lifecycle: nomination → acceptance → discussion → voting → installation
2. Contested elections: multiple candidates, highest yes_stake wins
3. Runoff: top 2 within 5%, triggers runoff entering voting directly
4. Auto-fail: nomination not accepted within deadline
5. Term expiry: seat clears on schedule, threshold adjusts
6. Vacancy: warnings at 30 days, notice at 90 days
7. Candidate validation: Guardian check, vote history >= 5, no double-seat
8. Emergency removal: 75%/80% supermajority thresholds
