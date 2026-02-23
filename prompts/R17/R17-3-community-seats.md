# R17-3 — Community Seat Elections + Term Rotation

## Objective

Implement the election mechanism for community seats on the research fund committee, including candidacy, voting, term limits, and staggered rotation.

## Prerequisites

R17-2 complete (phase tracking and governance state in place).

## Changes Required

### 1. New LIP Category: `research_seat_election`

Add to `x/gov` category configs:

```go
CategoryConfig{
    Category:        "research_seat_election",
    RequiredStakeBps: 500_000_000, // 500 ZRN — high bar for committee elections
    ReviewBlocks:     34_272,      // ~1 day
}
```

Election LIPs follow the standard LIP lifecycle (discussion → voting) but with additional validation:
- Candidate must be Guardian-tier at time of proposal submission
- Candidate must not already hold a community seat
- Candidate must have voted on ≥5 LIPs in the past (demonstrated governance participation)

### 2. Seat Election LIP Type

Add to `x/gov/types/`:

```protobuf
// SeatElectionProposal nominates a candidate for a research fund community seat.
message SeatElectionProposal {
    uint64 proposal_id = 1;
    string proposer = 2;              // bech32 — anyone can nominate
    string candidate = 3;             // bech32 — must be Guardian-tier
    uint32 seat_index = 4;            // which seat (0 in Phase 1; 0-2 in Phase 2)
    string statement = 5;             // candidate's governance statement (max 2000 chars)
    string stage = 6;                 // standard LIP stages
    string yes_stake = 7;
    string no_stake = 8;
    string abstain_stake = 9;
    uint64 voting_end_block = 10;
    uint64 created_at_block = 11;
}
```

### 3. Election Flow

```
Nomination → Candidate Acceptance → Discussion (1 day) → Voting (3 days) → Installation
```

1. **Nomination:** Any address submits a `SeatElectionProposal` with 500 ZRN stake
2. **Candidate Acceptance:** Candidate must submit an on-chain acceptance tx within 1 day
   - Validates Guardian tier status
   - Validates governance participation history (≥5 LIP votes)
   - Without acceptance, proposal auto-fails
3. **Discussion:** Standard 1-day review period
4. **Voting:** Standard 3-day voting period, standard quorum (33.4%) + majority (50%)
5. **Installation:** Winning candidate's address added to `ResearchFundGovernanceState.community_seats`

### 4. Contested Elections

If multiple candidates are nominated for the same seat simultaneously:
- All nominations proceed through voting independently
- The candidate with the **highest yes_stake** (absolute, not percentage) wins
- If top two candidates are within 5% of each other, a runoff is triggered (new 3-day vote between the top two)

### 5. Term Management

**Phase 1:** 1 community seat, term ≈ 6 months (~6,400,000 blocks)
**Phase 2:** 3 community seats, staggered terms:
- Seat 0: term expires at block `phase_start + 2,133,333` (~2 months)
- Seat 1: term expires at block `phase_start + 4,266,666` (~4 months)
- Seat 2: term expires at block `phase_start + 6,400,000` (~6 months)
- After initial stagger, all terms are 6 months from their election block

**Re-election:** Incumbents can run again. No term limits (the community decides).

### 6. Term Expiry Handler

In `x/gov` BeginBlocker (or EndBlocker), check for expired terms:

```go
func (k Keeper) CheckSeatTermExpiry(ctx sdk.Context) {
    state := k.GetResearchFundGovernanceState(ctx)
    height := uint64(ctx.BlockHeight())
    
    for i, endBlock := range state.SeatTermEndBlocks {
        if endBlock > 0 && height >= endBlock {
            // Seat expired — remove from active seats
            k.ExpireSeat(ctx, state, uint32(i))
            
            ctx.EventManager().EmitEvent(sdk.NewEvent(
                "zerone.gov.seat_expired",
                sdk.NewAttribute("seat_index", fmt.Sprint(i)),
                sdk.NewAttribute("former_holder", state.CommunitySeats[i]),
                sdk.NewAttribute("block", fmt.Sprint(height)),
            ))
        }
    }
}
```

When a seat expires:
- The seat becomes vacant (address set to "")
- A `seat_expired` event is emitted (signals that an election should be initiated)
- The multisig threshold calculation treats vacant seats as absent (threshold adjusts)
  - Phase 1 with vacant seat: effectively 2-of-2 (reverts to founder pair behavior)
  - Phase 2 with 1 vacant: 3-of-4; with 2 vacant: 2-of-3; with 3 vacant: 2-of-2

### 7. Vacancy Handling

If a community seat is vacant for more than 30 days (`~1,030,000 blocks`) and no election is active:
- Emit a `seat_vacancy_warning` event every epoch
- If vacant for more than 90 days: auto-submit a governance notice LIP alerting the community
- The research fund continues operating at reduced threshold (never stalls due to vacancy)

### 8. Seat Removal (Emergency)

A sitting community member can be removed before term expiry via:
- Emergency governance proposal (75% quorum, 80% support — same as emergency halt)
- Grounds: the seat holder has been jailed as a validator, or has been slashed 3+ times during their term

This is deliberately hard — removing an elected representative should require near-consensus.

### 9. Keeper Methods

```go
// InstallCommunitySeat adds a community member to the specified seat.
func (k Keeper) InstallCommunitySeat(ctx sdk.Context, seatIndex uint32, address string, termEndBlock uint64) error

// ExpireSeat clears a community seat that has reached term end.
func (k Keeper) ExpireSeat(ctx sdk.Context, state *types.ResearchFundGovernanceState, seatIndex uint32)

// GetActiveCommunitySeatCount returns how many community seats are currently filled.
func (k Keeper) GetActiveCommunitySeatCount(ctx sdk.Context) uint32

// ValidateSeatCandidate checks Guardian tier, governance history, and no existing seat.
func (k Keeper) ValidateSeatCandidate(ctx sdk.Context, candidate string) error

// CountCandidateGovernanceVotes returns how many LIPs the candidate has voted on.
func (k Keeper) CountCandidateGovernanceVotes(ctx sdk.Context, candidate string) uint64
```

### 10. CLI Commands

```bash
# View current community seats
zeroned query gov research-fund-seats

# Nominate a candidate
zeroned tx gov nominate-research-seat \
    --candidate zrn1abc... \
    --seat-index 0 \
    --statement "I will prioritize truth discovery bounties..." \
    --from proposer

# Accept nomination
zeroned tx gov accept-research-nomination \
    --proposal-id 42 \
    --from candidate
```

## Verification

```bash
go build ./x/gov/...

# Election lifecycle
go test ./x/gov/keeper/ -v -run "SeatElection"

# Term rotation
go test ./x/gov/keeper/ -v -run "TermExpiry\|TermRotation"

# Vacancy handling
go test ./x/gov/keeper/ -v -run "Vacancy"

# Candidate validation
go test ./x/gov/keeper/ -v -run "ValidateCandidate"
```

## Commit

```
R17-3: community seat elections + term rotation

- research_seat_election LIP category (500 ZRN stake, Guardian-only)
- SeatElectionProposal type with candidate acceptance flow
- Contested elections: highest yes_stake wins, runoff if within 5%
- Staggered 6-month terms (Phase 2: rotate 1 of 3 every 2 months)
- BeginBlocker term expiry check with vacancy handling
- Emergency removal via 75%/80% supermajority
- CLI: nominate-research-seat, accept-research-nomination, query seats
```
