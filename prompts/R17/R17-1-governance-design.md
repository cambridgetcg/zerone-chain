# R17-1 — Governance Design Doc + Phase Milestone Types

## Objective

Create the formal governance design document for the research fund migration and define the proto/Go types that track phase state on-chain.

## Deliverables

### 1. Design Document: `docs/tokenomics/GOVERNANCE-MIGRATION.md`

Write a complete governance migration spec covering:

#### Phase 0: Genesis Pair (Launch → Milestone 1)

**Structure:** 2-of-2 multisig (Yu + AI Vault)
- Existing `ResearchSpendProposal` in `x/gov` handles this
- Smallest decision team, maximum alignment
- At 3.33% of ~90M year-1 emission ≈ 3M ZRN to research year 1

**Exit conditions (ALL must be true):**
- `distinct_lip_voters >= 10` — at least 10 unique addresses have voted on LIPs
- `active_guardians >= 5` — at least 5 Guardian-tier validators active
- `research_fund_balance >= 100_000_000_000 uzrn` (100,000 ZRN)
- `chain_age_blocks >= 2_200_000` (~6 months at 2.521s blocks)

#### Phase 1: Founder + Observer (Milestone 1 → Milestone 2)

**Structure:** 2-of-3 (Yu + AI + 1 Community Seat)
- Community seat elected via dedicated LIP category (`research_seat_election`)
- Candidates must be Guardian-tier validators
- Term: ~6 months (~6.4M blocks), renewable by election
- Founders retain control (can approve 2-of-3 without community member)
- Community member has visibility + veto power when aligned with either founder

**Exit conditions (ALL must be true):**
- `executed_proposals_in_phase >= 3` — at least 3 successful research spends
- `distinct_lip_voters >= 25`
- `active_guardians >= 10`
- `community_seat_participation >= 2` — community seat voted on ≥2 proposals
- `chain_age_blocks >= 5_700_000` (~18 months)

#### Phase 2: Balanced Committee (Milestone 2 → Milestone 3)

**Structure:** 3-of-5 (Yu + AI + 3 Community Seats)
- Community has majority if all three align
- Founders can block but not unilaterally approve
- Staggered terms: seats rotate every ~2 months (1 of 3 community seats per election)
- Proposals require 7-day (`~240,000 blocks`) discussion before committee vote

**Exit conditions (ALL must be true):**
- `executed_proposals_in_phase >= 10`
- `distinct_lip_voters >= 50`
- `active_guardians >= 22` (full target validator set)
- `phase_duration_blocks >= 9_500_000` (~1 year at Phase 2)
- `emergency_halts_from_fund_misuse == 0`
- `chain_age_blocks >= 12_600_000` (~3 years)

#### Phase 3: Full Governance (Terminal)

**Structure:** Standard LIP process — no multisig
- Research spending proposals use `research_spend` LIP category
- 200 ZRN proposer stake, ~12h review, ~3-day voting
- 33.4% quorum, >50% support
- Multisig dissolved; funds in module account, disbursed by governance
- Yu and AI participate as regular voters with their staked weight

#### Safety Mechanisms

**Transition Protocol:**
1. Announcement LIP submitted (public, with evidence that exit conditions are met)
2. Extended discussion: 30 days (`~1,030,000 blocks`)
3. Supermajority vote: 66.7% support (not standard 50%)
4. Activation delay: 7 days (`~240,000 blocks`) after vote passes
5. Phase advances on-chain; old multisig config replaced

**Rollback Clause:**
If expanded committee experiences gridlock (3 consecutive proposals expire without action) OR an emergency halt is triggered citing research fund misuse:
- Any Guardian can submit a rollback LIP
- Same supermajority (66.7%) required
- Rolls back to previous phase
- Cooldown: cannot attempt forward transition for 3 months after rollback

**Founder Anchor:**
- Founder share (0.23%) is governance-immune at ALL phases (per R16 decision)
- AI vault key persists — transitions from signer to regular voter in Phase 3

### 2. Proto Types: `proto/zerone/gov/v1/types.proto`

Add to existing gov types proto:

```protobuf
// ResearchFundPhase tracks the current governance phase of the research fund.
enum ResearchFundPhase {
  RESEARCH_FUND_PHASE_UNSPECIFIED = 0;
  RESEARCH_FUND_PHASE_GENESIS_PAIR = 1;   // 2-of-2: founder + AI
  RESEARCH_FUND_PHASE_OBSERVER = 2;        // 2-of-3: founder + AI + 1 community
  RESEARCH_FUND_PHASE_BALANCED = 3;        // 3-of-5: founder + AI + 3 community
  RESEARCH_FUND_PHASE_FULL_GOVERNANCE = 4; // standard LIP process
}

// ResearchFundGovernanceState tracks the research fund governance lifecycle.
message ResearchFundGovernanceState {
  ResearchFundPhase current_phase = 1;
  uint64 phase_started_at_block = 2;
  uint64 proposals_executed_in_phase = 3;
  uint64 last_transition_block = 4;
  repeated string community_seats = 5;        // bech32 addresses of current community seat holders
  repeated uint64 seat_term_end_blocks = 6;   // term expiry per community seat
  uint64 rollback_cooldown_until = 7;          // block height; 0 = no cooldown
}

// PhaseTransitionConditions records the metrics at time of transition proposal.
message PhaseTransitionConditions {
  uint64 distinct_lip_voters = 1;
  uint64 active_guardians = 2;
  string research_fund_balance = 3;            // uzrn bigint string
  uint64 chain_age_blocks = 4;
  uint64 proposals_executed_in_phase = 5;
  uint64 community_seat_participation = 6;
  uint64 emergency_halts_from_misuse = 7;
}
```

### 3. Go Types: `x/gov/types/`

Add to `x/gov/types/` (hand-written, not just proto-generated):

```go
// Phase exit condition thresholds (governance-adjustable via standard LIP)
type PhaseExitConditions struct {
    MinDistinctVoters         uint64
    MinActiveGuardians        uint64
    MinResearchFundBalance    string // uzrn
    MinChainAgeBlocks         uint64
    MinProposalsExecuted      uint64
    MinCommunitySeatVotes     uint64
    MaxEmergencyHalts         uint64
}
```

Default conditions for each phase transition (0→1, 1→2, 2→3) as specified above.

### 4. Genesis State Extension

Add `ResearchFundGovernanceState` to `x/gov` GenesisState:

```protobuf
message GenesisState {
  // ... existing fields ...
  ResearchFundGovernanceState research_fund_governance = N; // new field
}
```

Default genesis: Phase 0 (GENESIS_PAIR), started at block 0, 0 proposals executed.

### 5. Query Endpoint

Add to `proto/zerone/gov/v1/query.proto`:

```protobuf
rpc ResearchFundGovernance(QueryResearchFundGovernanceRequest) 
    returns (QueryResearchFundGovernanceResponse);
```

Response includes: current phase, exit conditions status (which are met, which aren't), community seats, time in phase, next transition eligibility.

## Verification

```bash
# Proto compiles
make proto-gen

# Types compile  
go build ./x/gov/...

# Design doc exists and is linked from tokenomics README
cat docs/tokenomics/GOVERNANCE-MIGRATION.md | head -5
grep -l "GOVERNANCE-MIGRATION" docs/tokenomics/README.md
```

## Commit

```
R17-1: research fund governance migration design + proto types

Design doc: docs/tokenomics/GOVERNANCE-MIGRATION.md
- 4-phase graduated expansion from 2-of-2 to full LIP governance
- Maturity-gated transitions (not time-gated)
- Rollback clause, transition protocol, founder anchor

Proto: ResearchFundPhase enum, ResearchFundGovernanceState,
PhaseTransitionConditions added to x/gov types.
Genesis state extended. Query endpoint added.
```
