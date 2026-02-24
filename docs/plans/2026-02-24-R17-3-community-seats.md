# R17-3 Community Seat Elections Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Implement election mechanism for community seats on the research fund committee, including candidacy, voting, term limits, staggered rotation, and vacancy handling.

**Architecture:** Fully self-contained election system within x/gov. New `SeatElectionProposal` type with own vote storage, tally logic, and BeginBlocker hooks — mirrors the ResearchSpendProposal pattern. All state uses JSON encoding in KV store.

**Tech Stack:** Cosmos SDK v0.50.15, Go 1.24+, protobuf (buf generate), CometBFT v0.38.20

**Design doc:** `docs/plans/2026-02-24-R17-3-community-seats-design.md`

---

### Task 1: Proto Definitions

**Files:**
- Modify: `proto/zerone/gov/v1/types.proto` (append after line 106)
- Modify: `proto/zerone/gov/v1/tx.proto` (append new messages after line 136)
- Modify: `proto/zerone/gov/v1/query.proto` (append new queries after line 130)
- Modify: `proto/zerone/gov/v1/genesis.proto` (add fields to GenesisState after line 42)

**Step 1: Add SeatElectionProposal and SeatElectionVote to types.proto**

Append after line 106 in `proto/zerone/gov/v1/types.proto`:

```protobuf
// SeatElectionProposal nominates a candidate for a research fund community seat.
message SeatElectionProposal {
  uint64 proposal_id         = 1;
  string proposer            = 2;   // bech32 — nominator
  string candidate           = 3;   // bech32 — must be Guardian-tier
  uint32 seat_index          = 4;   // 0 in Phase 1; 0-2 in Phase 2
  string statement           = 5;   // candidate's governance statement (max 2000 chars)
  string stage               = 6;   // nominated/accepted/discussion/voting/runoff/passed/failed/expired
  string yes_stake           = 7;
  string no_stake            = 8;
  string abstain_stake       = 9;
  uint64 acceptance_deadline = 10;
  uint64 discussion_end_block = 11;
  uint64 voting_end_block    = 12;
  uint64 created_at_block    = 13;
  bool   candidate_accepted  = 14;
  bool   is_runoff           = 15;
  repeated uint64 runoff_parent_ids = 16;
}

// SeatElectionVote records a single vote on a seat election proposal.
message SeatElectionVote {
  uint64 proposal_id = 1;
  string voter       = 2;  // bech32 address
  string option      = 3;  // "yes", "no", "abstain"
  string stake       = 4;  // uzrn weight at time of vote
  uint64 block       = 5;
}
```

**Step 2: Add new messages to tx.proto**

Append after `MsgAttachUpgradePlanResponse` (line 136) in `proto/zerone/gov/v1/tx.proto`:

Add to service Msg (insert before closing brace at line 22):
```protobuf
  rpc NominateSeatElection(MsgNominateSeatElection) returns (MsgNominateSeatElectionResponse);
  rpc AcceptSeatNomination(MsgAcceptSeatNomination) returns (MsgAcceptSeatNominationResponse);
  rpc VoteSeatElection(MsgVoteSeatElection) returns (MsgVoteSeatElectionResponse);
```

Append message definitions after line 136:
```protobuf
// MsgNominateSeatElection nominates a candidate for a community seat.
message MsgNominateSeatElection {
  option (cosmos.msg.v1.signer) = "proposer";
  string proposer   = 1;  // nominator bech32
  string candidate  = 2;  // candidate bech32
  uint32 seat_index = 3;
  string statement  = 4;  // max 2000 chars
}

message MsgNominateSeatElectionResponse {
  uint64 proposal_id = 1;
}

// MsgAcceptSeatNomination accepts a pending seat election nomination.
message MsgAcceptSeatNomination {
  option (cosmos.msg.v1.signer) = "candidate";
  string candidate   = 1;
  uint64 proposal_id = 2;
}

message MsgAcceptSeatNominationResponse {}

// MsgVoteSeatElection casts a stake-weighted vote on a seat election.
message MsgVoteSeatElection {
  option (cosmos.msg.v1.signer) = "voter";
  string voter       = 1;
  uint64 proposal_id = 2;
  string option      = 3;  // "yes", "no", "abstain"
}

message MsgVoteSeatElectionResponse {
  string effective_weight = 1;
}
```

**Step 3: Add new queries to query.proto**

Add to service Query (insert before closing brace at line 42):
```protobuf
  rpc SeatElection(QuerySeatElectionRequest) returns (QuerySeatElectionResponse) {
    option (google.api.http).get = "/zerone/gov/v1/seat_election/{proposal_id}";
  }
  rpc SeatElections(QuerySeatElectionsRequest) returns (QuerySeatElectionsResponse) {
    option (google.api.http).get = "/zerone/gov/v1/seat_elections";
  }
  rpc ResearchFundSeats(QueryResearchFundSeatsRequest) returns (QueryResearchFundSeatsResponse) {
    option (google.api.http).get = "/zerone/gov/v1/research_fund_seats";
  }
```

Append message definitions after line 130:
```protobuf
message QuerySeatElectionRequest {
  uint64 proposal_id = 1;
}

message QuerySeatElectionResponse {
  SeatElectionProposal proposal = 1;
  repeated SeatElectionVote votes = 2;
}

message QuerySeatElectionsRequest {
  string stage  = 1;
  uint64 limit  = 2;
  uint64 offset = 3;
}

message QuerySeatElectionsResponse {
  repeated SeatElectionProposal proposals = 1;
  uint64 total = 2;
}

message QueryResearchFundSeatsRequest {}

message QueryResearchFundSeatsResponse {
  repeated string community_seats = 1;
  repeated uint64 seat_term_end_blocks = 2;
  uint32 active_seat_count = 3;
}
```

**Step 4: Add genesis fields to genesis.proto**

Add to GenesisState (insert before closing brace at line 43):
```protobuf
  repeated SeatElectionProposal seat_elections = 7;
  repeated SeatElectionVote seat_election_votes = 8;
  uint64 next_seat_election_number = 9;
```

**Step 5: Generate Go types**

Run: `make proto-gen`
Expected: Generated `.pb.go` files updated with new types.

**Step 6: Verify build**

Run: `go build ./x/gov/...`
Expected: BUILD SUCCESS (new types exist but no logic references them yet)

**Step 7: Commit**

```bash
git add proto/zerone/gov/v1/ x/gov/types/*.pb.go x/gov/types/*_grpc.pb.go x/gov/types/*.pb.gw.go
git commit -m "R17-3: proto definitions for seat elections

- SeatElectionProposal + SeatElectionVote types
- NominateSeatElection, AcceptSeatNomination, VoteSeatElection messages
- SeatElection, SeatElections, ResearchFundSeats queries
- Genesis fields for seat election state"
```

---

### Task 2: Types Layer — Keys, Constants, Errors

**Files:**
- Modify: `x/gov/types/keys.go` (add key prefixes after line 25, add key functions after line 115)
- Modify: `x/gov/types/types.go` (add constants and message methods)
- Modify: `x/gov/types/errors.go` (add error codes after line 29)
- Modify: `x/gov/types/expected_keepers.go` (add IsGuardian to StakingKeeper)
- Modify: `x/gov/types/genesis.go` (add category config)
- Modify: `x/gov/types/codec.go` (register new messages)

**Step 1: Add store key prefixes to keys.go**

NOTE: `0x0E` and `0x0F` are already taken by `DistinctVoterKeyPrefix` and `ResearchCommunityVotePrefix`. Use `0x10`–`0x13`.

Add after `ResearchCommunityVotePrefix` (line 24) in keys.go:
```go
	SeatElectionKeyPrefix        = []byte{0x10}
	SeatElectionVoteKeyPrefix    = []byte{0x11}
	SeatElectionCounterKey       = []byte{0x12}
	SeatElectionVoteDedupePrefix = []byte{0x13}
```

Add key functions after line 115:
```go
// SeatElectionKey returns the store key for a seat election proposal by ID.
func SeatElectionKey(proposalID uint64) []byte {
	bz := make([]byte, 8)
	bz[0] = byte(proposalID >> 56)
	bz[1] = byte(proposalID >> 48)
	bz[2] = byte(proposalID >> 40)
	bz[3] = byte(proposalID >> 32)
	bz[4] = byte(proposalID >> 24)
	bz[5] = byte(proposalID >> 16)
	bz[6] = byte(proposalID >> 8)
	bz[7] = byte(proposalID)
	return append(SeatElectionKeyPrefix, bz...)
}

// SeatElectionVoteKey returns the store key for a seat election vote.
func SeatElectionVoteKey(proposalID uint64, voter string) []byte {
	bz := make([]byte, 8)
	bz[0] = byte(proposalID >> 56)
	bz[1] = byte(proposalID >> 48)
	bz[2] = byte(proposalID >> 40)
	bz[3] = byte(proposalID >> 32)
	bz[4] = byte(proposalID >> 24)
	bz[5] = byte(proposalID >> 16)
	bz[6] = byte(proposalID >> 8)
	bz[7] = byte(proposalID)
	key := append(SeatElectionVoteKeyPrefix, bz...)
	key = append(key, 0x00)
	key = append(key, []byte(voter)...)
	return key
}

// SeatElectionVoteDedupeKey returns the dedupe key for a seat election vote.
func SeatElectionVoteDedupeKey(proposalID uint64, voter string) []byte {
	bz := make([]byte, 8)
	bz[0] = byte(proposalID >> 56)
	bz[1] = byte(proposalID >> 48)
	bz[2] = byte(proposalID >> 40)
	bz[3] = byte(proposalID >> 32)
	bz[4] = byte(proposalID >> 24)
	bz[5] = byte(proposalID >> 16)
	bz[6] = byte(proposalID >> 8)
	bz[7] = byte(proposalID)
	key := append(SeatElectionVoteDedupePrefix, bz...)
	key = append(key, 0x00)
	key = append(key, []byte(voter)...)
	return key
}

// SeatElectionVotePrefixForProposal returns the prefix for iterating all votes on a seat election.
func SeatElectionVotePrefixForProposal(proposalID uint64) []byte {
	bz := make([]byte, 8)
	bz[0] = byte(proposalID >> 56)
	bz[1] = byte(proposalID >> 48)
	bz[2] = byte(proposalID >> 40)
	bz[3] = byte(proposalID >> 32)
	bz[4] = byte(proposalID >> 24)
	bz[5] = byte(proposalID >> 16)
	bz[6] = byte(proposalID >> 8)
	bz[7] = byte(proposalID)
	key := append(SeatElectionVoteKeyPrefix, bz...)
	key = append(key, 0x00)
	return key
}
```

**Step 2: Add election constants to types.go**

Add after `CategoryResearchSpend` (line 25):
```go
	CategorySeatElection = "research_seat_election"
```

Add after `VoteAbstain` (line 32):
```go
// Seat election stage constants.
const (
	SeatStageNominated  = "nominated"
	SeatStageAccepted   = "accepted"
	SeatStageDiscussion = "discussion"
	SeatStageVoting     = "voting"
	SeatStageRunoff     = "runoff"
	SeatStagePassed     = "passed"
	SeatStageFailed     = "failed"
	SeatStageExpired    = "expired"
)

// Seat election timing constants.
const (
	SeatAcceptanceBlocks    = uint64(34_272)    // ~1 day
	SeatDiscussionBlocks    = uint64(34_272)    // ~1 day
	SeatVotingBlocks        = uint64(102_816)   // ~3 days
	SeatTermBlocks          = uint64(6_400_000) // ~6 months
	SeatVacancyWarningBlocks = uint64(1_030_000) // ~30 days
	SeatVacancyNoticeBlocks  = uint64(3_090_000) // ~90 days
	SeatRunoffThresholdBps   = uint64(50_000)    // 5% on 1M scale
)

// Phase 2 initial stagger offsets.
const (
	SeatStaggerOffset0 = uint64(2_133_333) // ~2 months
	SeatStaggerOffset1 = uint64(4_266_666) // ~4 months
	SeatStaggerOffset2 = uint64(6_400_000) // ~6 months
)

// SeatStatementMaxLen is the maximum length of a candidate's governance statement.
const SeatStatementMaxLen = 2000

// MinCandidateGovernanceVotes is the minimum LIP votes required for candidacy.
const MinCandidateGovernanceVotes = uint64(5)

// IsTerminalSeatStage returns true if the stage is terminal.
func IsTerminalSeatStage(stage string) bool {
	switch stage {
	case SeatStagePassed, SeatStageFailed, SeatStageExpired:
		return true
	}
	return false
}
```

Add ValidateBasic and GetSigners methods for the new messages (append after MsgUpdateParams methods, before the research fund section around line 355):
```go
// --- Seat Election Messages ---

func (m *MsgNominateSeatElection) ValidateBasic() error {
	if _, err := sdk.AccAddressFromBech32(m.Proposer); err != nil {
		return ErrInvalidAddress
	}
	if _, err := sdk.AccAddressFromBech32(m.Candidate); err != nil {
		return ErrInvalidAddress
	}
	if len(m.Statement) > SeatStatementMaxLen {
		return ErrInvalidParams
	}
	return nil
}

func (m *MsgNominateSeatElection) GetSigners() []sdk.AccAddress {
	addr, _ := sdk.AccAddressFromBech32(m.Proposer)
	return []sdk.AccAddress{addr}
}

func (m *MsgAcceptSeatNomination) ValidateBasic() error {
	if _, err := sdk.AccAddressFromBech32(m.Candidate); err != nil {
		return ErrInvalidAddress
	}
	if m.ProposalId == 0 {
		return ErrInvalidParams
	}
	return nil
}

func (m *MsgAcceptSeatNomination) GetSigners() []sdk.AccAddress {
	addr, _ := sdk.AccAddressFromBech32(m.Candidate)
	return []sdk.AccAddress{addr}
}

func (m *MsgVoteSeatElection) ValidateBasic() error {
	if _, err := sdk.AccAddressFromBech32(m.Voter); err != nil {
		return ErrInvalidAddress
	}
	if m.ProposalId == 0 {
		return ErrInvalidParams
	}
	if m.Option != VoteYes && m.Option != VoteNo && m.Option != VoteAbstain {
		return ErrInvalidParams
	}
	return nil
}

func (m *MsgVoteSeatElection) GetSigners() []sdk.AccAddress {
	addr, _ := sdk.AccAddressFromBech32(m.Voter)
	return []sdk.AccAddress{addr}
}
```

**Step 3: Add error codes to errors.go**

Append after line 28:
```go
	ErrSeatElectionNotFound    = errors.Register(ModuleName, 25, "seat election proposal not found")
	ErrSeatAlreadyHeld         = errors.Register(ModuleName, 26, "candidate already holds a community seat")
	ErrNotGuardianTier         = errors.Register(ModuleName, 27, "candidate is not Guardian tier")
	ErrInsufficientGovHistory  = errors.Register(ModuleName, 28, "candidate has not voted on enough LIPs")
	ErrSeatNominationExpired   = errors.Register(ModuleName, 29, "nomination acceptance deadline has passed")
	ErrNotNominatedCandidate   = errors.Register(ModuleName, 30, "sender is not the nominated candidate")
	ErrSeatElectionNotVoting   = errors.Register(ModuleName, 31, "seat election is not in voting stage")
	ErrSeatElectionAlreadyVoted = errors.Register(ModuleName, 32, "voter has already voted on this seat election")
	ErrInvalidSeatIndex        = errors.Register(ModuleName, 33, "seat index is invalid for current phase")
	ErrSeatNominationNotAccepted = errors.Register(ModuleName, 34, "nomination has not been accepted by candidate")
```

**Step 4: Add IsGuardian to expected_keepers.go**

Add to StakingKeeper interface (after line 14):
```go
	// IsGuardian returns true if the address is Guardian tier (tier 4) and active.
	IsGuardian(ctx context.Context, addr string) (bool, error)
```

**Step 5: Add category config and genesis validation to genesis.go**

Add to `DefaultParams().CategoryConfigs` slice (after line 18):
```go
			// TODO: rename RequiredStakeBps → RequiredStakeUzrn (misnomer storing raw uzrn)
			{Category: CategorySeatElection, RequiredStakeBps: "500000000", ReviewBlocks: 34272}, // 500 ZRN, ~1 day
```

Add to `DefaultGenesisState()` (after line 46):
```go
		NextSeatElectionNumber: 1,
```

Add genesis validation for seat elections in `Validate()` (after line 80, before the final return):
```go
	// Check for duplicate seat election IDs.
	seenElections := make(map[uint64]bool)
	for _, se := range gs.SeatElections {
		if seenElections[se.ProposalId] {
			return fmt.Errorf("duplicate seat election id: %d", se.ProposalId)
		}
		seenElections[se.ProposalId] = true
	}
```

**Step 6: Register new messages in codec.go**

Add to `RegisterCodec()` (after MsgAttachUpgradePlan, line 20):
```go
	cdc.RegisterConcrete(&MsgNominateSeatElection{}, "zerone_gov/MsgNominateSeatElection", nil)
	cdc.RegisterConcrete(&MsgAcceptSeatNomination{}, "zerone_gov/MsgAcceptSeatNomination", nil)
	cdc.RegisterConcrete(&MsgVoteSeatElection{}, "zerone_gov/MsgVoteSeatElection", nil)
```

Add to `RegisterInterfaces()` (after &MsgAttachUpgradePlan{}, line 35):
```go
		&MsgNominateSeatElection{},
		&MsgAcceptSeatNomination{},
		&MsgVoteSeatElection{},
```

**Step 7: Verify build**

Run: `go build ./x/gov/...`
Expected: BUILD SUCCESS

**Step 8: Commit**

```bash
git add x/gov/types/
git commit -m "R17-3: types layer — keys, constants, errors, codec for seat elections

- Store key prefixes 0x10-0x13 for seat election state
- Election stage constants + timing constants (acceptance, discussion, voting, terms)
- 10 new error codes for seat election validation
- IsGuardian added to StakingKeeper interface
- research_seat_election category config (500 ZRN)
- Message validation: NominateSeatElection, AcceptSeatNomination, VoteSeatElection"
```

---

### Task 3: State CRUD

**Files:**
- Modify: `x/gov/keeper/state.go` (add CRUD methods after ResearchFundGovernanceState section, ~line 271)

**Step 1: Write failing tests for seat election CRUD**

Add to `x/gov/keeper/seat_election_test.go` (new file):

```go
package keeper_test

import (
	"testing"

	"github.com/zerone-chain/zerone/x/gov/types"
)

func TestSeatElection_CRUD(t *testing.T) {
	k, ctx := setupKeeper(t)

	// Get non-existent — returns nil, false.
	_, found := k.GetSeatElection(ctx, 1)
	if found {
		t.Error("expected not found")
	}

	// Set and get.
	prop := &types.SeatElectionProposal{
		ProposalId: 1,
		Proposer:   testAddr("nominator"),
		Candidate:  testAddr("guardian"),
		SeatIndex:  0,
		Statement:  "I will serve truth",
		Stage:      types.SeatStageNominated,
	}
	k.SetSeatElection(ctx, prop)

	got, found := k.GetSeatElection(ctx, 1)
	if !found {
		t.Fatal("expected found")
	}
	if got.Candidate != testAddr("guardian") {
		t.Errorf("candidate: got %s, want %s", got.Candidate, testAddr("guardian"))
	}
	if got.Stage != types.SeatStageNominated {
		t.Errorf("stage: got %s, want %s", got.Stage, types.SeatStageNominated)
	}
}

func TestSeatElection_Counter(t *testing.T) {
	k, ctx := setupKeeper(t)

	id := k.GetNextSeatElectionID(ctx)
	if id != 1 {
		t.Errorf("expected 1, got %d", id)
	}

	k.SetNextSeatElectionID(ctx, 5)
	id = k.GetNextSeatElectionID(ctx)
	if id != 5 {
		t.Errorf("expected 5, got %d", id)
	}
}

func TestSeatElection_VoteCRUD(t *testing.T) {
	k, ctx := setupKeeper(t)

	voter := testAddr("voter1")

	// No vote yet.
	if k.HasSeatElectionVoted(ctx, 1, voter) {
		t.Error("expected no vote")
	}

	// Set vote.
	vote := &types.SeatElectionVote{
		ProposalId: 1,
		Voter:      voter,
		Option:     "yes",
		Stake:      "1000000",
		Block:      100,
	}
	k.SetSeatElectionVote(ctx, vote)

	if !k.HasSeatElectionVoted(ctx, 1, voter) {
		t.Error("expected vote to exist")
	}

	got, found := k.GetSeatElectionVote(ctx, 1, voter)
	if !found {
		t.Fatal("vote not found")
	}
	if got.Option != "yes" {
		t.Errorf("option: got %s, want yes", got.Option)
	}

	// Get all votes for proposal.
	votes := k.GetVotesForSeatElection(ctx, 1)
	if len(votes) != 1 {
		t.Errorf("expected 1 vote, got %d", len(votes))
	}
}

func TestSeatElection_IterateByStage(t *testing.T) {
	k, ctx := setupKeeper(t)

	k.SetSeatElection(ctx, &types.SeatElectionProposal{ProposalId: 1, Stage: types.SeatStageNominated})
	k.SetSeatElection(ctx, &types.SeatElectionProposal{ProposalId: 2, Stage: types.SeatStageVoting})
	k.SetSeatElection(ctx, &types.SeatElectionProposal{ProposalId: 3, Stage: types.SeatStageVoting})

	voting := k.GetSeatElectionsByStage(ctx, types.SeatStageVoting)
	if len(voting) != 2 {
		t.Errorf("expected 2 voting, got %d", len(voting))
	}
}
```

**Step 2: Run tests to verify they fail**

Run: `go test ./x/gov/keeper/ -v -run "TestSeatElection_CRUD|TestSeatElection_Counter|TestSeatElection_VoteCRUD|TestSeatElection_IterateByStage"`
Expected: FAIL — methods do not exist yet

**Step 3: Implement CRUD in state.go**

Add after `GetResearchFundGovernanceState` (after line 271) in `x/gov/keeper/state.go`:

```go
// --- Seat Election CRUD ---

// GetSeatElection retrieves a seat election proposal by ID.
func (k Keeper) GetSeatElection(ctx sdk.Context, id uint64) (*types.SeatElectionProposal, bool) {
	store := ctx.KVStore(k.storeKey)
	bz := store.Get(types.SeatElectionKey(id))
	if bz == nil {
		return nil, false
	}
	var prop types.SeatElectionProposal
	if err := json.Unmarshal(bz, &prop); err != nil {
		return nil, false
	}
	return &prop, true
}

// SetSeatElection stores a seat election proposal.
func (k Keeper) SetSeatElection(ctx sdk.Context, prop *types.SeatElectionProposal) {
	store := ctx.KVStore(k.storeKey)
	bz, err := json.Marshal(prop)
	if err != nil {
		panic("failed to marshal seat election: " + err.Error())
	}
	store.Set(types.SeatElectionKey(prop.ProposalId), bz)
}

// GetNextSeatElectionID returns the next seat election proposal ID.
func (k Keeper) GetNextSeatElectionID(ctx sdk.Context) uint64 {
	store := ctx.KVStore(k.storeKey)
	bz := store.Get(types.SeatElectionCounterKey)
	if bz == nil {
		return 1
	}
	return binary.BigEndian.Uint64(bz)
}

// SetNextSeatElectionID sets the next seat election proposal ID.
func (k Keeper) SetNextSeatElectionID(ctx sdk.Context, id uint64) {
	store := ctx.KVStore(k.storeKey)
	bz := make([]byte, 8)
	binary.BigEndian.PutUint64(bz, id)
	store.Set(types.SeatElectionCounterKey, bz)
}

// IterateSeatElections iterates over all seat election proposals.
func (k Keeper) IterateSeatElections(ctx sdk.Context, cb func(*types.SeatElectionProposal) bool) {
	store := ctx.KVStore(k.storeKey)
	iter := storetypes.KVStorePrefixIterator(store, types.SeatElectionKeyPrefix)
	defer iter.Close()

	for ; iter.Valid(); iter.Next() {
		var prop types.SeatElectionProposal
		if err := json.Unmarshal(iter.Value(), &prop); err != nil {
			continue
		}
		if cb(&prop) {
			break
		}
	}
}

// GetSeatElectionsByStage returns all seat elections in a given stage.
func (k Keeper) GetSeatElectionsByStage(ctx sdk.Context, stage string) []*types.SeatElectionProposal {
	var result []*types.SeatElectionProposal
	k.IterateSeatElections(ctx, func(prop *types.SeatElectionProposal) bool {
		if prop.Stage == stage {
			result = append(result, prop)
		}
		return false
	})
	return result
}

// GetAllSeatElections returns all seat election proposals.
func (k Keeper) GetAllSeatElections(ctx sdk.Context) []*types.SeatElectionProposal {
	var result []*types.SeatElectionProposal
	k.IterateSeatElections(ctx, func(prop *types.SeatElectionProposal) bool {
		result = append(result, prop)
		return false
	})
	return result
}

// --- Seat Election Vote CRUD ---

// SetSeatElectionVote stores a seat election vote and sets the dedupe key.
func (k Keeper) SetSeatElectionVote(ctx sdk.Context, vote *types.SeatElectionVote) {
	store := ctx.KVStore(k.storeKey)
	bz, err := json.Marshal(vote)
	if err != nil {
		panic("failed to marshal seat election vote: " + err.Error())
	}
	store.Set(types.SeatElectionVoteKey(vote.ProposalId, vote.Voter), bz)
	store.Set(types.SeatElectionVoteDedupeKey(vote.ProposalId, vote.Voter), []byte{1})
}

// GetSeatElectionVote retrieves a seat election vote.
func (k Keeper) GetSeatElectionVote(ctx sdk.Context, proposalID uint64, voter string) (*types.SeatElectionVote, bool) {
	store := ctx.KVStore(k.storeKey)
	bz := store.Get(types.SeatElectionVoteKey(proposalID, voter))
	if bz == nil {
		return nil, false
	}
	var vote types.SeatElectionVote
	if err := json.Unmarshal(bz, &vote); err != nil {
		return nil, false
	}
	return &vote, true
}

// HasSeatElectionVoted returns true if the voter has already voted on this seat election.
func (k Keeper) HasSeatElectionVoted(ctx sdk.Context, proposalID uint64, voter string) bool {
	store := ctx.KVStore(k.storeKey)
	return store.Has(types.SeatElectionVoteDedupeKey(proposalID, voter))
}

// GetVotesForSeatElection returns all votes for a seat election proposal.
func (k Keeper) GetVotesForSeatElection(ctx sdk.Context, proposalID uint64) []*types.SeatElectionVote {
	store := ctx.KVStore(k.storeKey)
	iter := storetypes.KVStorePrefixIterator(store, types.SeatElectionVotePrefixForProposal(proposalID))
	defer iter.Close()

	var votes []*types.SeatElectionVote
	for ; iter.Valid(); iter.Next() {
		var vote types.SeatElectionVote
		if err := json.Unmarshal(iter.Value(), &vote); err != nil {
			continue
		}
		votes = append(votes, &vote)
	}
	return votes
}

// GetAllSeatElectionVotes returns all seat election votes.
func (k Keeper) GetAllSeatElectionVotes(ctx sdk.Context) []*types.SeatElectionVote {
	store := ctx.KVStore(k.storeKey)
	iter := storetypes.KVStorePrefixIterator(store, types.SeatElectionVoteKeyPrefix)
	defer iter.Close()

	var votes []*types.SeatElectionVote
	for ; iter.Valid(); iter.Next() {
		var vote types.SeatElectionVote
		if err := json.Unmarshal(iter.Value(), &vote); err != nil {
			continue
		}
		votes = append(votes, &vote)
	}
	return votes
}
```

Ensure `encoding/binary` and `encoding/json` are in the imports of state.go (they should already be there from ResearchSpend CRUD).

**Step 4: Run tests to verify they pass**

Run: `go test ./x/gov/keeper/ -v -run "TestSeatElection_CRUD|TestSeatElection_Counter|TestSeatElection_VoteCRUD|TestSeatElection_IterateByStage"`
Expected: PASS

**Step 5: Commit**

```bash
git add x/gov/keeper/state.go x/gov/keeper/seat_election_test.go
git commit -m "R17-3: seat election CRUD — proposal + vote storage

- Get/Set/Iterate seat election proposals (JSON encoding, 0x10 prefix)
- Get/Set/HasVoted/GetVotesFor seat election votes (0x11 prefix + 0x13 dedupe)
- Counter for auto-incrementing proposal IDs
- Tests: CRUD, counter, vote dedupe, stage filtering"
```

---

### Task 4: Candidate Validation + Nomination

**Files:**
- Create: `x/gov/keeper/seat_election.go`
- Modify: `x/gov/keeper/seat_election_test.go`

**Step 1: Write failing tests for candidate validation and nomination**

Add to `seat_election_test.go`:

```go
// ---------- Mock Staking Keeper with IsGuardian ----------

type mockStakingKeeperWithGuardian struct {
	mockStakingKeeper
	guardians map[string]bool
}

func newMockStakingKeeperWithGuardian(totalBonded string) *mockStakingKeeperWithGuardian {
	return &mockStakingKeeperWithGuardian{
		mockStakingKeeper: mockStakingKeeper{
			totalBonded: totalBonded,
			delegations: make(map[string]string),
		},
		guardians: make(map[string]bool),
	}
}

func (m *mockStakingKeeperWithGuardian) IsGuardian(_ context.Context, addr string) (bool, error) {
	return m.guardians[addr], nil
}

// ---------- Helper: setup with guardian staking ----------

func setupWithGuardianStaking(t *testing.T, totalBonded string) (keeper.Keeper, sdk.Context, *mockStakingKeeperWithGuardian) {
	t.Helper()

	storeKey := storetypes.NewKVStoreKey(types.StoreKey)
	db := dbm.NewMemDB()
	stateStore := store.NewCommitMultiStore(db, log.NewNopLogger(), storemetrics.NewNoOpMetrics())
	stateStore.MountStoreWithDB(storeKey, storetypes.StoreTypeIAVL, db)
	if err := stateStore.LoadLatestVersion(); err != nil {
		t.Fatalf("failed to load latest version: %v", err)
	}

	registry := codectypes.NewInterfaceRegistry()
	cdc := codec.NewProtoCodec(registry)

	mock := newMockStakingKeeperWithGuardian(totalBonded)
	k := keeper.NewKeeper(cdc, storeKey, "authority", nil, mock)

	ctx := sdk.NewContext(stateStore, cmtproto.Header{Height: 100}, false, log.NewNopLogger())
	k.SetParams(ctx, types.DefaultParams())

	return k, ctx, mock
}

// ---------- Candidate Validation Tests ----------

func TestValidateSeatCandidate_NotGuardian(t *testing.T) {
	k, ctx, _ := setupWithGuardianStaking(t, "1000000000000")

	err := k.ValidateSeatCandidate(ctx, testAddr("nonguardian"))
	if err == nil {
		t.Error("expected error for non-guardian")
	}
}

func TestValidateSeatCandidate_InsufficientVotes(t *testing.T) {
	k, ctx, mock := setupWithGuardianStaking(t, "1000000000000")
	candidate := testAddr("guardian1")
	mock.guardians[candidate] = true

	// Guardian but no governance votes.
	err := k.ValidateSeatCandidate(ctx, candidate)
	if err == nil {
		t.Error("expected error for insufficient votes")
	}
}

func TestValidateSeatCandidate_AlreadyHoldsSeat(t *testing.T) {
	k, ctx, mock := setupWithGuardianStaking(t, "1000000000000")
	candidate := testAddr("guardian1")
	mock.guardians[candidate] = true

	// Give 5 LIP votes.
	for i := 0; i < 5; i++ {
		lipID := fmt.Sprintf("LIP-%d", i+1)
		k.SetVote(ctx, &types.Vote{LipId: lipID, Voter: candidate, Option: "yes", Weight: "1000000"})
	}

	// Put candidate in a seat.
	state := types.DefaultResearchFundGovernanceState()
	state.CommunitySeats = []string{candidate}
	state.SeatTermEndBlocks = []uint64{999999}
	k.SetResearchFundGovernanceState(ctx, state)

	err := k.ValidateSeatCandidate(ctx, candidate)
	if err == nil {
		t.Error("expected error for already holding seat")
	}
}

func TestValidateSeatCandidate_Valid(t *testing.T) {
	k, ctx, mock := setupWithGuardianStaking(t, "1000000000000")
	candidate := testAddr("guardian1")
	mock.guardians[candidate] = true

	// Give 5 LIP votes.
	for i := 0; i < 5; i++ {
		lipID := fmt.Sprintf("LIP-%d", i+1)
		k.SetVote(ctx, &types.Vote{LipId: lipID, Voter: candidate, Option: "yes", Weight: "1000000"})
	}

	err := k.ValidateSeatCandidate(ctx, candidate)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// ---------- Nomination Tests ----------

func TestNominateSeatElection_Success(t *testing.T) {
	k, ctx, _ := setupWithGuardianStaking(t, "1000000000000")

	// Set phase to Observer (Phase 1 — 1 community seat).
	state := types.DefaultResearchFundGovernanceState()
	state.CurrentPhase = types.ResearchFundPhase_RESEARCH_FUND_PHASE_OBSERVER
	state.CommunitySeats = []string{""}
	state.SeatTermEndBlocks = []uint64{0}
	k.SetResearchFundGovernanceState(ctx, state)

	resp, err := k.NominateSeatElection(ctx, &types.MsgNominateSeatElection{
		Proposer:  testAddr("nominator"),
		Candidate: testAddr("guardian1"),
		SeatIndex: 0,
		Statement: "I will prioritize truth discovery",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.ProposalId != 1 {
		t.Errorf("expected proposal_id=1, got %d", resp.ProposalId)
	}

	prop, found := k.GetSeatElection(ctx, 1)
	if !found {
		t.Fatal("proposal not found")
	}
	if prop.Stage != types.SeatStageNominated {
		t.Errorf("expected nominated, got %s", prop.Stage)
	}
	if prop.AcceptanceDeadline != uint64(100)+types.SeatAcceptanceBlocks {
		t.Errorf("wrong acceptance deadline")
	}
}

func TestNominateSeatElection_InvalidSeatIndex(t *testing.T) {
	k, ctx, _ := setupWithGuardianStaking(t, "1000000000000")

	// Phase 1 — only seat 0.
	state := types.DefaultResearchFundGovernanceState()
	state.CurrentPhase = types.ResearchFundPhase_RESEARCH_FUND_PHASE_OBSERVER
	state.CommunitySeats = []string{""}
	state.SeatTermEndBlocks = []uint64{0}
	k.SetResearchFundGovernanceState(ctx, state)

	_, err := k.NominateSeatElection(ctx, &types.MsgNominateSeatElection{
		Proposer:  testAddr("nominator"),
		Candidate: testAddr("guardian1"),
		SeatIndex: 1, // invalid for Phase 1
	})
	if err == nil {
		t.Error("expected error for invalid seat index")
	}
}
```

**Step 2: Run tests to verify they fail**

Run: `go test ./x/gov/keeper/ -v -run "TestValidateSeatCandidate|TestNominateSeatElection"`
Expected: FAIL — methods do not exist

**Step 3: Implement seat_election.go**

Create `x/gov/keeper/seat_election.go`:

```go
package keeper

import (
	"fmt"
	"math/big"

	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/zerone-chain/zerone/x/gov/types"
)

// --- Candidate Validation ---

// CountCandidateGovernanceVotes counts distinct LIPs the candidate has voted on.
func (k Keeper) CountCandidateGovernanceVotes(ctx sdk.Context, candidate string) uint64 {
	var count uint64
	seen := make(map[string]bool)
	k.IterateVotes(ctx, func(v *types.Vote) bool {
		if v.Voter == candidate && !seen[v.LipId] {
			seen[v.LipId] = true
			count++
		}
		return false
	})
	return count
}

// IterateVotes iterates over all votes (needed for counting governance participation).
func (k Keeper) IterateVotes(ctx sdk.Context, cb func(*types.Vote) bool) {
	votes := k.GetAllVotes(ctx)
	for _, v := range votes {
		if cb(v) {
			break
		}
	}
}

// ValidateSeatCandidate checks Guardian tier, governance history, and no existing seat.
func (k Keeper) ValidateSeatCandidate(ctx sdk.Context, candidate string) error {
	// 1. Guardian tier check.
	if k.stakingKeeper != nil {
		isGuardian, err := k.stakingKeeper.IsGuardian(ctx, candidate)
		if err != nil {
			return fmt.Errorf("failed to check guardian status: %w", err)
		}
		if !isGuardian {
			return types.ErrNotGuardianTier
		}
	}

	// 2. Governance participation: >= 5 distinct LIP votes.
	voteCount := k.CountCandidateGovernanceVotes(ctx, candidate)
	if voteCount < types.MinCandidateGovernanceVotes {
		return types.ErrInsufficientGovHistory
	}

	// 3. Not already holding a community seat.
	state := k.GetResearchFundGovernanceState(ctx)
	for _, seat := range state.CommunitySeats {
		if seat == candidate {
			return types.ErrSeatAlreadyHeld
		}
	}

	return nil
}

// GetActiveCommunitySeatCount returns how many community seats are currently filled.
func (k Keeper) GetActiveCommunitySeatCount(ctx sdk.Context) uint32 {
	state := k.GetResearchFundGovernanceState(ctx)
	var count uint32
	for _, seat := range state.CommunitySeats {
		if seat != "" {
			count++
		}
	}
	return count
}

// maxSeatIndexForPhase returns the maximum valid seat index for the current phase.
func maxSeatIndexForPhase(phase types.ResearchFundPhase) uint32 {
	switch phase {
	case types.ResearchFundPhase_RESEARCH_FUND_PHASE_OBSERVER:
		return 0 // 1 seat
	case types.ResearchFundPhase_RESEARCH_FUND_PHASE_BALANCED:
		return 2 // 3 seats
	default:
		return 0
	}
}

// --- Nomination ---

// NominateSeatElection creates a new seat election nomination.
func (k Keeper) NominateSeatElection(ctx sdk.Context, msg *types.MsgNominateSeatElection) (*types.MsgNominateSeatElectionResponse, error) {
	currentHeight := uint64(ctx.BlockHeight())

	// Validate phase supports community seats.
	state := k.GetResearchFundGovernanceState(ctx)
	if state.CurrentPhase != types.ResearchFundPhase_RESEARCH_FUND_PHASE_OBSERVER &&
		state.CurrentPhase != types.ResearchFundPhase_RESEARCH_FUND_PHASE_BALANCED {
		return nil, fmt.Errorf("community seat elections not available in current phase")
	}

	// Validate seat index.
	if msg.SeatIndex > maxSeatIndexForPhase(state.CurrentPhase) {
		return nil, types.ErrInvalidSeatIndex
	}

	// Validate statement length.
	if len(msg.Statement) > types.SeatStatementMaxLen {
		return nil, types.ErrInvalidParams
	}

	// Allocate ID.
	id := k.GetNextSeatElectionID(ctx)
	k.SetNextSeatElectionID(ctx, id+1)

	prop := &types.SeatElectionProposal{
		ProposalId:         id,
		Proposer:           msg.Proposer,
		Candidate:          msg.Candidate,
		SeatIndex:          msg.SeatIndex,
		Statement:          msg.Statement,
		Stage:              types.SeatStageNominated,
		YesStake:           "0",
		NoStake:            "0",
		AbstainStake:       "0",
		AcceptanceDeadline: currentHeight + types.SeatAcceptanceBlocks,
		CreatedAtBlock:     currentHeight,
	}

	k.SetSeatElection(ctx, prop)

	ctx.EventManager().EmitEvent(
		sdk.NewEvent(
			"zerone.gov.seat_nomination",
			sdk.NewAttribute("proposal_id", fmt.Sprintf("%d", id)),
			sdk.NewAttribute("proposer", msg.Proposer),
			sdk.NewAttribute("candidate", msg.Candidate),
			sdk.NewAttribute("seat_index", fmt.Sprintf("%d", msg.SeatIndex)),
		),
	)

	return &types.MsgNominateSeatElectionResponse{ProposalId: id}, nil
}

// --- Acceptance ---

// AcceptSeatNomination handles a candidate accepting their nomination.
func (k Keeper) AcceptSeatNomination(ctx sdk.Context, msg *types.MsgAcceptSeatNomination) (*types.MsgAcceptSeatNominationResponse, error) {
	currentHeight := uint64(ctx.BlockHeight())

	prop, found := k.GetSeatElection(ctx, msg.ProposalId)
	if !found {
		return nil, types.ErrSeatElectionNotFound
	}

	// Must be in nominated stage.
	if prop.Stage != types.SeatStageNominated {
		return nil, types.ErrInvalidStatus
	}

	// Sender must be the candidate.
	if msg.Candidate != prop.Candidate {
		return nil, types.ErrNotNominatedCandidate
	}

	// Check acceptance deadline.
	if currentHeight > prop.AcceptanceDeadline {
		return nil, types.ErrSeatNominationExpired
	}

	// Validate candidate eligibility.
	if err := k.ValidateSeatCandidate(ctx, msg.Candidate); err != nil {
		return nil, err
	}

	// Accept and advance to discussion.
	prop.CandidateAccepted = true
	prop.Stage = types.SeatStageDiscussion
	prop.DiscussionEndBlock = currentHeight + types.SeatDiscussionBlocks

	k.SetSeatElection(ctx, prop)

	ctx.EventManager().EmitEvent(
		sdk.NewEvent(
			"zerone.gov.seat_nomination_accepted",
			sdk.NewAttribute("proposal_id", fmt.Sprintf("%d", msg.ProposalId)),
			sdk.NewAttribute("candidate", msg.Candidate),
		),
	)

	return &types.MsgAcceptSeatNominationResponse{}, nil
}

// --- Voting ---

// VoteSeatElection casts a stake-weighted vote on a seat election.
func (k Keeper) VoteSeatElection(ctx sdk.Context, msg *types.MsgVoteSeatElection) (*types.MsgVoteSeatElectionResponse, error) {
	currentHeight := uint64(ctx.BlockHeight())

	prop, found := k.GetSeatElection(ctx, msg.ProposalId)
	if !found {
		return nil, types.ErrSeatElectionNotFound
	}

	// Must be in voting stage.
	if prop.Stage != types.SeatStageVoting {
		return nil, types.ErrSeatElectionNotVoting
	}

	// Check voting period not expired.
	if prop.VotingEndBlock > 0 && currentHeight >= prop.VotingEndBlock {
		return nil, types.ErrVotingPeriodEnded
	}

	// Check for double-vote.
	if k.HasSeatElectionVoted(ctx, msg.ProposalId, msg.Voter) {
		return nil, types.ErrSeatElectionAlreadyVoted
	}

	// Get voter's bonded stake.
	voterWeight := "0"
	if k.stakingKeeper != nil {
		w, err := k.stakingKeeper.GetDelegatorTotalBonded(ctx, msg.Voter)
		if err == nil {
			voterWeight = w
		}
	}

	// Store vote.
	vote := &types.SeatElectionVote{
		ProposalId: msg.ProposalId,
		Voter:      msg.Voter,
		Option:     msg.Option,
		Stake:      voterWeight,
		Block:      currentHeight,
	}
	k.SetSeatElectionVote(ctx, vote)

	// Accumulate tally.
	switch msg.Option {
	case types.VoteYes:
		prop.YesStake = types.AddBigIntStrings(prop.YesStake, voterWeight)
	case types.VoteNo:
		prop.NoStake = types.AddBigIntStrings(prop.NoStake, voterWeight)
	case types.VoteAbstain:
		prop.AbstainStake = types.AddBigIntStrings(prop.AbstainStake, voterWeight)
	}

	k.SetSeatElection(ctx, prop)

	ctx.EventManager().EmitEvent(
		sdk.NewEvent(
			"zerone.gov.seat_election_vote",
			sdk.NewAttribute("proposal_id", fmt.Sprintf("%d", msg.ProposalId)),
			sdk.NewAttribute("voter", msg.Voter),
			sdk.NewAttribute("option", msg.Option),
			sdk.NewAttribute("weight", voterWeight),
		),
	)

	return &types.MsgVoteSeatElectionResponse{EffectiveWeight: voterWeight}, nil
}

// --- Tally + Contested Elections ---

// TallySeatElections resolves all seat elections whose voting period has ended.
// Called from BeginBlocker.
func (k Keeper) TallySeatElections(ctx sdk.Context) {
	currentHeight := uint64(ctx.BlockHeight())
	params := k.GetParams(ctx)

	// Group voting-stage elections by seat_index.
	bySeat := make(map[uint32][]*types.SeatElectionProposal)
	k.IterateSeatElections(ctx, func(prop *types.SeatElectionProposal) bool {
		if prop.Stage == types.SeatStageVoting && prop.VotingEndBlock > 0 && currentHeight >= prop.VotingEndBlock {
			bySeat[prop.SeatIndex] = append(bySeat[prop.SeatIndex], prop)
		}
		return false
	})

	for seatIdx, candidates := range bySeat {
		k.resolveSeatElection(ctx, seatIdx, candidates, params, currentHeight)
	}
}

// resolveSeatElection resolves an election for a single seat.
func (k Keeper) resolveSeatElection(ctx sdk.Context, seatIdx uint32, candidates []*types.SeatElectionProposal, params *types.Params, currentHeight uint64) {
	if len(candidates) == 0 {
		return
	}

	// For runoff elections: highest yes_stake wins, no further runoff.
	if len(candidates) == 1 || candidates[0].IsRunoff {
		for _, c := range candidates {
			quorumMet, passed := k.checkSeatElectionQuorum(ctx, c, params)
			if quorumMet && passed {
				c.Stage = types.SeatStagePassed
				k.InstallCommunitySeat(ctx, seatIdx, c.Candidate, currentHeight)
			} else {
				c.Stage = types.SeatStageFailed
			}
			k.SetSeatElection(ctx, c)
			k.emitSeatTallyEvent(ctx, c, quorumMet)
		}
		return
	}

	// Multiple candidates: check quorum for each, then pick winner by yes_stake.
	type rankedCandidate struct {
		prop     *types.SeatElectionProposal
		yesStake *big.Int
		passed   bool
	}

	var ranked []rankedCandidate
	for _, c := range candidates {
		quorumMet, passed := k.checkSeatElectionQuorum(ctx, c, params)
		yes, _ := new(big.Int).SetString(c.YesStake, 10)
		if yes == nil {
			yes = big.NewInt(0)
		}
		rc := rankedCandidate{prop: c, yesStake: yes, passed: quorumMet && passed}
		ranked = append(ranked, rc)
		k.emitSeatTallyEvent(ctx, c, quorumMet)
	}

	// Sort by yes_stake descending.
	for i := 0; i < len(ranked); i++ {
		for j := i + 1; j < len(ranked); j++ {
			if ranked[j].yesStake.Cmp(ranked[i].yesStake) > 0 {
				ranked[i], ranked[j] = ranked[j], ranked[i]
			}
		}
	}

	// Check if any candidate passed.
	passedCount := 0
	for _, rc := range ranked {
		if rc.passed {
			passedCount++
		}
	}

	if passedCount == 0 {
		// All failed.
		for _, rc := range ranked {
			rc.prop.Stage = types.SeatStageFailed
			k.SetSeatElection(ctx, rc.prop)
		}
		return
	}

	// Check for runoff: top 2 within 5%.
	if len(ranked) >= 2 && ranked[0].passed && ranked[1].passed {
		gap := new(big.Int).Sub(ranked[0].yesStake, ranked[1].yesStake)
		threshold := new(big.Int).Mul(ranked[0].yesStake, big.NewInt(int64(types.SeatRunoffThresholdBps)))
		threshold.Div(threshold, big.NewInt(int64(types.BPSScale)))

		if gap.Cmp(threshold) <= 0 {
			// Trigger runoff between top 2.
			k.createRunoff(ctx, seatIdx, ranked[0].prop, ranked[1].prop, currentHeight)
			// Mark all others as failed.
			for i := 2; i < len(ranked); i++ {
				ranked[i].prop.Stage = types.SeatStageFailed
				k.SetSeatElection(ctx, ranked[i].prop)
			}
			return
		}
	}

	// Winner: highest yes_stake that passed.
	for i, rc := range ranked {
		if i == 0 && rc.passed {
			rc.prop.Stage = types.SeatStagePassed
			k.InstallCommunitySeat(ctx, seatIdx, rc.prop.Candidate, currentHeight)
		} else {
			rc.prop.Stage = types.SeatStageFailed
		}
		k.SetSeatElection(ctx, rc.prop)
	}
}

// checkSeatElectionQuorum checks quorum and support for a seat election.
func (k Keeper) checkSeatElectionQuorum(ctx sdk.Context, prop *types.SeatElectionProposal, params *types.Params) (quorumMet bool, passed bool) {
	yesBig, _ := new(big.Int).SetString(prop.YesStake, 10)
	if yesBig == nil {
		yesBig = big.NewInt(0)
	}
	noBig, _ := new(big.Int).SetString(prop.NoStake, 10)
	if noBig == nil {
		noBig = big.NewInt(0)
	}
	abstainBig, _ := new(big.Int).SetString(prop.AbstainStake, 10)
	if abstainBig == nil {
		abstainBig = big.NewInt(0)
	}

	totalVoted := new(big.Int).Add(yesBig, noBig)
	totalVoted.Add(totalVoted, abstainBig)

	totalBonded := big.NewInt(0)
	if k.stakingKeeper != nil {
		bondedStr, err := k.stakingKeeper.GetTotalBondedStake(ctx)
		if err == nil {
			if tb, ok := new(big.Int).SetString(bondedStr, 10); ok {
				totalBonded = tb
			}
		}
	}

	if totalBonded.Sign() > 0 {
		actualBps := new(big.Int).Mul(totalVoted, big.NewInt(int64(types.BPSScale)))
		actualBps.Div(actualBps, totalBonded)
		quorumMet = actualBps.Uint64() >= params.QuorumThresholdBps
	}

	yesNoTotal := new(big.Int).Add(yesBig, noBig)
	if yesNoTotal.Sign() > 0 {
		supportBps := new(big.Int).Mul(yesBig, big.NewInt(int64(types.BPSScale)))
		supportBps.Div(supportBps, yesNoTotal)
		passed = quorumMet && supportBps.Uint64() >= params.SupportThresholdBps
	}

	return quorumMet, passed
}

// createRunoff creates a runoff election between two candidates.
func (k Keeper) createRunoff(ctx sdk.Context, seatIdx uint32, c1, c2 *types.SeatElectionProposal, currentHeight uint64) {
	// Mark originals as runoff-triggered (not passed, not failed — they feed into the runoff).
	c1.Stage = types.SeatStageRunoff
	k.SetSeatElection(ctx, c1)
	c2.Stage = types.SeatStageRunoff
	k.SetSeatElection(ctx, c2)

	// Create runoff for candidate 1.
	id1 := k.GetNextSeatElectionID(ctx)
	k.SetNextSeatElectionID(ctx, id1+1)
	runoff1 := &types.SeatElectionProposal{
		ProposalId:       id1,
		Proposer:         c1.Proposer,
		Candidate:        c1.Candidate,
		SeatIndex:        seatIdx,
		Statement:        c1.Statement,
		Stage:            types.SeatStageVoting,
		YesStake:         "0",
		NoStake:          "0",
		AbstainStake:     "0",
		VotingEndBlock:   currentHeight + types.SeatVotingBlocks,
		CreatedAtBlock:   currentHeight,
		IsRunoff:         true,
		RunoffParentIds:  []uint64{c1.ProposalId, c2.ProposalId},
	}
	k.SetSeatElection(ctx, runoff1)

	// Create runoff for candidate 2.
	id2 := k.GetNextSeatElectionID(ctx)
	k.SetNextSeatElectionID(ctx, id2+1)
	runoff2 := &types.SeatElectionProposal{
		ProposalId:       id2,
		Proposer:         c2.Proposer,
		Candidate:        c2.Candidate,
		SeatIndex:        seatIdx,
		Statement:        c2.Statement,
		Stage:            types.SeatStageVoting,
		YesStake:         "0",
		NoStake:          "0",
		AbstainStake:     "0",
		VotingEndBlock:   currentHeight + types.SeatVotingBlocks,
		CreatedAtBlock:   currentHeight,
		IsRunoff:         true,
		RunoffParentIds:  []uint64{c1.ProposalId, c2.ProposalId},
	}
	k.SetSeatElection(ctx, runoff2)

	ctx.EventManager().EmitEvent(
		sdk.NewEvent(
			"zerone.gov.seat_election_runoff",
			sdk.NewAttribute("seat_index", fmt.Sprintf("%d", seatIdx)),
			sdk.NewAttribute("candidate1", c1.Candidate),
			sdk.NewAttribute("candidate2", c2.Candidate),
			sdk.NewAttribute("runoff_id1", fmt.Sprintf("%d", id1)),
			sdk.NewAttribute("runoff_id2", fmt.Sprintf("%d", id2)),
		),
	)
}

// InstallCommunitySeat adds a community member to the specified seat.
func (k Keeper) InstallCommunitySeat(ctx sdk.Context, seatIndex uint32, address string, currentHeight uint64) error {
	state := k.GetResearchFundGovernanceState(ctx)

	// Ensure seat arrays are large enough.
	for uint32(len(state.CommunitySeats)) <= seatIndex {
		state.CommunitySeats = append(state.CommunitySeats, "")
		state.SeatTermEndBlocks = append(state.SeatTermEndBlocks, 0)
	}

	state.CommunitySeats[seatIndex] = address
	state.SeatTermEndBlocks[seatIndex] = currentHeight + types.SeatTermBlocks

	k.SetResearchFundGovernanceState(ctx, state)

	ctx.EventManager().EmitEvent(
		sdk.NewEvent(
			"zerone.gov.seat_installed",
			sdk.NewAttribute("seat_index", fmt.Sprintf("%d", seatIndex)),
			sdk.NewAttribute("address", address),
			sdk.NewAttribute("term_end_block", fmt.Sprintf("%d", currentHeight+types.SeatTermBlocks)),
		),
	)

	return nil
}

// --- Term Management ---

// ExpireSeat clears a community seat that has reached term end.
func (k Keeper) ExpireSeat(ctx sdk.Context, state *types.ResearchFundGovernanceState, seatIndex uint32) {
	if uint32(len(state.CommunitySeats)) <= seatIndex {
		return
	}
	state.CommunitySeats[seatIndex] = ""
	state.SeatTermEndBlocks[seatIndex] = 0
}

// CheckSeatTermExpiry checks for expired community seat terms. Called from BeginBlocker.
func (k Keeper) CheckSeatTermExpiry(ctx sdk.Context) {
	state := k.GetResearchFundGovernanceState(ctx)
	height := uint64(ctx.BlockHeight())
	changed := false

	for i, endBlock := range state.SeatTermEndBlocks {
		if endBlock > 0 && height >= endBlock {
			formerHolder := state.CommunitySeats[i]
			k.ExpireSeat(ctx, state, uint32(i))
			changed = true

			ctx.EventManager().EmitEvent(
				sdk.NewEvent(
					"zerone.gov.seat_expired",
					sdk.NewAttribute("seat_index", fmt.Sprintf("%d", i)),
					sdk.NewAttribute("former_holder", formerHolder),
					sdk.NewAttribute("block", fmt.Sprintf("%d", height)),
				),
			)
		}
	}

	if changed {
		k.SetResearchFundGovernanceState(ctx, state)
	}
}

// CheckSeatVacancy emits warnings for long-vacant seats. Called from BeginBlocker.
func (k Keeper) CheckSeatVacancy(ctx sdk.Context) {
	state := k.GetResearchFundGovernanceState(ctx)
	height := uint64(ctx.BlockHeight())

	for i, seat := range state.CommunitySeats {
		if seat != "" {
			continue // seat is filled
		}
		endBlock := state.SeatTermEndBlocks[i]
		if endBlock == 0 {
			continue // never been filled — vacancy tracking doesn't apply
		}

		vacantBlocks := height - endBlock
		if vacantBlocks >= types.SeatVacancyNoticeBlocks {
			ctx.EventManager().EmitEvent(
				sdk.NewEvent(
					"zerone.gov.seat_vacancy_notice",
					sdk.NewAttribute("seat_index", fmt.Sprintf("%d", i)),
					sdk.NewAttribute("vacant_blocks", fmt.Sprintf("%d", vacantBlocks)),
				),
			)
		} else if vacantBlocks >= types.SeatVacancyWarningBlocks {
			ctx.EventManager().EmitEvent(
				sdk.NewEvent(
					"zerone.gov.seat_vacancy_warning",
					sdk.NewAttribute("seat_index", fmt.Sprintf("%d", i)),
					sdk.NewAttribute("vacant_blocks", fmt.Sprintf("%d", vacantBlocks)),
				),
			)
		}
	}
}

// --- BeginBlocker Helpers ---

// ProcessSeatElectionExpiry advances seat election stage transitions.
func (k Keeper) ProcessSeatElectionExpiry(ctx sdk.Context, currentHeight uint64) {
	k.IterateSeatElections(ctx, func(prop *types.SeatElectionProposal) bool {
		changed := false

		// Nominated → expired (acceptance deadline passed without acceptance).
		if prop.Stage == types.SeatStageNominated && currentHeight >= prop.AcceptanceDeadline {
			prop.Stage = types.SeatStageExpired
			changed = true
		}

		// Discussion → voting.
		if prop.Stage == types.SeatStageDiscussion && prop.DiscussionEndBlock > 0 && currentHeight >= prop.DiscussionEndBlock {
			prop.Stage = types.SeatStageVoting
			prop.VotingEndBlock = currentHeight + types.SeatVotingBlocks
			changed = true
		}

		if changed {
			k.SetSeatElection(ctx, prop)
		}

		return false
	})
}

// emitSeatTallyEvent emits a tally event for a seat election.
func (k Keeper) emitSeatTallyEvent(ctx sdk.Context, prop *types.SeatElectionProposal, quorumMet bool) {
	ctx.EventManager().EmitEvent(
		sdk.NewEvent(
			"zerone.gov.seat_election_tallied",
			sdk.NewAttribute("proposal_id", fmt.Sprintf("%d", prop.ProposalId)),
			sdk.NewAttribute("candidate", prop.Candidate),
			sdk.NewAttribute("outcome", prop.Stage),
			sdk.NewAttribute("yes_stake", prop.YesStake),
			sdk.NewAttribute("no_stake", prop.NoStake),
			sdk.NewAttribute("quorum_met", fmt.Sprintf("%t", quorumMet)),
		),
	)
}
```

**Step 4: Run tests to verify they pass**

Run: `go test ./x/gov/keeper/ -v -run "TestValidateSeatCandidate|TestNominateSeatElection"`
Expected: PASS

**Step 5: Commit**

```bash
git add x/gov/keeper/seat_election.go x/gov/keeper/seat_election_test.go
git commit -m "R17-3: seat election core — validation, nomination, acceptance, voting, tally

- ValidateSeatCandidate: Guardian tier + 5 LIP votes + no double-seat
- NominateSeatElection: phase-aware, seat index validation
- AcceptSeatNomination: deadline check, eligibility validation
- VoteSeatElection: stake-weighted, dedupe, tally accumulation
- TallySeatElections: contested elections, runoff if within 5%
- InstallCommunitySeat, ExpireSeat, CheckSeatTermExpiry
- CheckSeatVacancy: 30-day warning, 90-day notice
- ProcessSeatElectionExpiry: auto-advance nominated→expired, discussion→voting"
```

---

### Task 5: Full Election Lifecycle + Tally Tests

**Files:**
- Modify: `x/gov/keeper/seat_election_test.go`

**Step 1: Write comprehensive test suite**

Add the following tests to `seat_election_test.go`:

```go
func TestAcceptSeatNomination_Success(t *testing.T) {
	k, ctx, mock := setupWithGuardianStaking(t, "1000000000000")
	candidate := testAddr("guardian1")
	mock.guardians[candidate] = true

	// Give 5 LIP votes.
	for i := 0; i < 5; i++ {
		k.SetVote(ctx, &types.Vote{LipId: fmt.Sprintf("LIP-%d", i+1), Voter: candidate, Option: "yes", Weight: "1000000"})
	}

	// Set phase and nominate.
	state := types.DefaultResearchFundGovernanceState()
	state.CurrentPhase = types.ResearchFundPhase_RESEARCH_FUND_PHASE_OBSERVER
	state.CommunitySeats = []string{""}
	state.SeatTermEndBlocks = []uint64{0}
	k.SetResearchFundGovernanceState(ctx, state)

	k.NominateSeatElection(ctx, &types.MsgNominateSeatElection{
		Proposer: testAddr("nominator"), Candidate: candidate, SeatIndex: 0, Statement: "Serve truth",
	})

	// Accept.
	_, err := k.AcceptSeatNomination(ctx, &types.MsgAcceptSeatNomination{
		Candidate: candidate, ProposalId: 1,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	prop, _ := k.GetSeatElection(ctx, 1)
	if prop.Stage != types.SeatStageDiscussion {
		t.Errorf("expected discussion, got %s", prop.Stage)
	}
	if !prop.CandidateAccepted {
		t.Error("expected candidate_accepted = true")
	}
}

func TestAcceptSeatNomination_Expired(t *testing.T) {
	k, ctx, mock := setupWithGuardianStaking(t, "1000000000000")
	candidate := testAddr("guardian1")
	mock.guardians[candidate] = true
	for i := 0; i < 5; i++ {
		k.SetVote(ctx, &types.Vote{LipId: fmt.Sprintf("LIP-%d", i+1), Voter: candidate, Option: "yes", Weight: "1000000"})
	}

	state := types.DefaultResearchFundGovernanceState()
	state.CurrentPhase = types.ResearchFundPhase_RESEARCH_FUND_PHASE_OBSERVER
	state.CommunitySeats = []string{""}
	state.SeatTermEndBlocks = []uint64{0}
	k.SetResearchFundGovernanceState(ctx, state)

	k.NominateSeatElection(ctx, &types.MsgNominateSeatElection{
		Proposer: testAddr("nominator"), Candidate: candidate, SeatIndex: 0,
	})

	// Advance past deadline.
	ctx2 := ctx.WithBlockHeight(int64(100 + types.SeatAcceptanceBlocks + 1))
	_, err := k.AcceptSeatNomination(ctx2, &types.MsgAcceptSeatNomination{
		Candidate: candidate, ProposalId: 1,
	})
	if err != types.ErrSeatNominationExpired {
		t.Errorf("expected ErrSeatNominationExpired, got %v", err)
	}
}

func TestSeatElection_FullLifecycle(t *testing.T) {
	k, ctx, mock := setupWithGuardianStaking(t, "1000000000000")
	candidate := testAddr("guardian1")
	mock.guardians[candidate] = true
	mock.delegations[testAddr("voter1")] = "400000000000"
	mock.delegations[testAddr("voter2")] = "200000000000"

	for i := 0; i < 5; i++ {
		k.SetVote(ctx, &types.Vote{LipId: fmt.Sprintf("LIP-%d", i+1), Voter: candidate, Option: "yes", Weight: "1000000"})
	}

	state := types.DefaultResearchFundGovernanceState()
	state.CurrentPhase = types.ResearchFundPhase_RESEARCH_FUND_PHASE_OBSERVER
	state.CommunitySeats = []string{""}
	state.SeatTermEndBlocks = []uint64{0}
	k.SetResearchFundGovernanceState(ctx, state)

	// 1. Nominate.
	k.NominateSeatElection(ctx, &types.MsgNominateSeatElection{
		Proposer: testAddr("nominator"), Candidate: candidate, SeatIndex: 0, Statement: "Truth first",
	})

	// 2. Accept.
	k.AcceptSeatNomination(ctx, &types.MsgAcceptSeatNomination{Candidate: candidate, ProposalId: 1})

	// 3. Advance discussion → voting via BeginBlocker.
	ctx2 := ctx.WithBlockHeight(int64(100 + types.SeatDiscussionBlocks))
	k.ProcessSeatElectionExpiry(ctx2, uint64(ctx2.BlockHeight()))
	prop, _ := k.GetSeatElection(ctx2, 1)
	if prop.Stage != types.SeatStageVoting {
		t.Fatalf("expected voting, got %s", prop.Stage)
	}

	// 4. Vote.
	votingCtx := ctx2
	k.VoteSeatElection(votingCtx, &types.MsgVoteSeatElection{Voter: testAddr("voter1"), ProposalId: 1, Option: "yes"})
	k.VoteSeatElection(votingCtx, &types.MsgVoteSeatElection{Voter: testAddr("voter2"), ProposalId: 1, Option: "yes"})

	// 5. Tally at voting end.
	prop, _ = k.GetSeatElection(votingCtx, 1)
	ctx3 := ctx.WithBlockHeight(int64(prop.VotingEndBlock))
	k.TallySeatElections(ctx3)

	prop, _ = k.GetSeatElection(ctx3, 1)
	if prop.Stage != types.SeatStagePassed {
		t.Errorf("expected passed, got %s", prop.Stage)
	}

	// 6. Verify seat installed.
	state = k.GetResearchFundGovernanceState(ctx3)
	if state.CommunitySeats[0] != candidate {
		t.Errorf("seat not installed: got %s", state.CommunitySeats[0])
	}
}

func TestSeatElection_ContestedHighestWins(t *testing.T) {
	k, ctx, mock := setupWithGuardianStaking(t, "1000000000000")
	mock.delegations[testAddr("v1")] = "500000000000" // 500k ZRN
	mock.delegations[testAddr("v2")] = "100000000000" // 100k ZRN

	state := types.DefaultResearchFundGovernanceState()
	state.CurrentPhase = types.ResearchFundPhase_RESEARCH_FUND_PHASE_OBSERVER
	state.CommunitySeats = []string{""}
	state.SeatTermEndBlocks = []uint64{0}
	k.SetResearchFundGovernanceState(ctx, state)

	// Two candidates for same seat, already in voting.
	k.SetSeatElection(ctx, &types.SeatElectionProposal{
		ProposalId: 1, Candidate: testAddr("c1"), SeatIndex: 0,
		Stage: types.SeatStageVoting, VotingEndBlock: 200,
		YesStake: "0", NoStake: "0", AbstainStake: "0",
	})
	k.SetSeatElection(ctx, &types.SeatElectionProposal{
		ProposalId: 2, Candidate: testAddr("c2"), SeatIndex: 0,
		Stage: types.SeatStageVoting, VotingEndBlock: 200,
		YesStake: "0", NoStake: "0", AbstainStake: "0",
	})
	k.SetNextSeatElectionID(ctx, 3)

	// Vote: v1 votes for c1, v2 votes for c2 (c1 has more stake).
	k.VoteSeatElection(ctx, &types.MsgVoteSeatElection{Voter: testAddr("v1"), ProposalId: 1, Option: "yes"})
	k.VoteSeatElection(ctx, &types.MsgVoteSeatElection{Voter: testAddr("v2"), ProposalId: 2, Option: "yes"})

	// Tally.
	ctx2 := ctx.WithBlockHeight(200)
	k.TallySeatElections(ctx2)

	p1, _ := k.GetSeatElection(ctx2, 1)
	p2, _ := k.GetSeatElection(ctx2, 2)
	if p1.Stage != types.SeatStagePassed {
		t.Errorf("c1 should have passed, got %s", p1.Stage)
	}
	if p2.Stage != types.SeatStageFailed {
		t.Errorf("c2 should have failed, got %s", p2.Stage)
	}
}

func TestSeatElection_RunoffTriggered(t *testing.T) {
	k, ctx, mock := setupWithGuardianStaking(t, "1000000000000")
	mock.delegations[testAddr("v1")] = "300000000000"
	mock.delegations[testAddr("v2")] = "295000000000" // within 5% of v1

	state := types.DefaultResearchFundGovernanceState()
	state.CurrentPhase = types.ResearchFundPhase_RESEARCH_FUND_PHASE_OBSERVER
	state.CommunitySeats = []string{""}
	state.SeatTermEndBlocks = []uint64{0}
	k.SetResearchFundGovernanceState(ctx, state)

	k.SetSeatElection(ctx, &types.SeatElectionProposal{
		ProposalId: 1, Candidate: testAddr("c1"), SeatIndex: 0,
		Stage: types.SeatStageVoting, VotingEndBlock: 200,
		YesStake: "0", NoStake: "0", AbstainStake: "0",
	})
	k.SetSeatElection(ctx, &types.SeatElectionProposal{
		ProposalId: 2, Candidate: testAddr("c2"), SeatIndex: 0,
		Stage: types.SeatStageVoting, VotingEndBlock: 200,
		YesStake: "0", NoStake: "0", AbstainStake: "0",
	})
	k.SetNextSeatElectionID(ctx, 3)

	k.VoteSeatElection(ctx, &types.MsgVoteSeatElection{Voter: testAddr("v1"), ProposalId: 1, Option: "yes"})
	k.VoteSeatElection(ctx, &types.MsgVoteSeatElection{Voter: testAddr("v2"), ProposalId: 2, Option: "yes"})

	ctx2 := ctx.WithBlockHeight(200)
	k.TallySeatElections(ctx2)

	// Original proposals should be in runoff stage.
	p1, _ := k.GetSeatElection(ctx2, 1)
	p2, _ := k.GetSeatElection(ctx2, 2)
	if p1.Stage != types.SeatStageRunoff {
		t.Errorf("c1 should be runoff, got %s", p1.Stage)
	}
	if p2.Stage != types.SeatStageRunoff {
		t.Errorf("c2 should be runoff, got %s", p2.Stage)
	}

	// New runoff proposals should exist.
	r1, found := k.GetSeatElection(ctx2, 3)
	if !found {
		t.Fatal("runoff proposal 3 not found")
	}
	if !r1.IsRunoff {
		t.Error("expected is_runoff = true")
	}
	if r1.Stage != types.SeatStageVoting {
		t.Errorf("runoff should be in voting, got %s", r1.Stage)
	}
}

func TestSeatElection_TermExpiry(t *testing.T) {
	k, ctx, _ := setupWithGuardianStaking(t, "1000000000000")

	state := types.DefaultResearchFundGovernanceState()
	state.CurrentPhase = types.ResearchFundPhase_RESEARCH_FUND_PHASE_OBSERVER
	state.CommunitySeats = []string{testAddr("member1")}
	state.SeatTermEndBlocks = []uint64{500}
	k.SetResearchFundGovernanceState(ctx, state)

	// Before expiry.
	ctx2 := ctx.WithBlockHeight(499)
	k.CheckSeatTermExpiry(ctx2)
	state = k.GetResearchFundGovernanceState(ctx2)
	if state.CommunitySeats[0] == "" {
		t.Error("seat should not be expired yet")
	}

	// At expiry.
	ctx3 := ctx.WithBlockHeight(500)
	k.CheckSeatTermExpiry(ctx3)
	state = k.GetResearchFundGovernanceState(ctx3)
	if state.CommunitySeats[0] != "" {
		t.Errorf("seat should be expired, got %s", state.CommunitySeats[0])
	}
}

func TestSeatElection_VoteDoubleVote(t *testing.T) {
	k, ctx, mock := setupWithGuardianStaking(t, "1000000000000")
	mock.delegations[testAddr("voter1")] = "100000000000"

	k.SetSeatElection(ctx, &types.SeatElectionProposal{
		ProposalId: 1, Stage: types.SeatStageVoting, VotingEndBlock: 999,
		YesStake: "0", NoStake: "0", AbstainStake: "0",
	})

	k.VoteSeatElection(ctx, &types.MsgVoteSeatElection{Voter: testAddr("voter1"), ProposalId: 1, Option: "yes"})
	_, err := k.VoteSeatElection(ctx, &types.MsgVoteSeatElection{Voter: testAddr("voter1"), ProposalId: 1, Option: "no"})
	if err != types.ErrSeatElectionAlreadyVoted {
		t.Errorf("expected ErrSeatElectionAlreadyVoted, got %v", err)
	}
}

func TestSeatElection_NominationAutoExpiry(t *testing.T) {
	k, ctx, _ := setupWithGuardianStaking(t, "1000000000000")

	k.SetSeatElection(ctx, &types.SeatElectionProposal{
		ProposalId: 1, Stage: types.SeatStageNominated, AcceptanceDeadline: 150,
	})

	// Before deadline — no change.
	k.ProcessSeatElectionExpiry(ctx, 149)
	prop, _ := k.GetSeatElection(ctx, 1)
	if prop.Stage != types.SeatStageNominated {
		t.Errorf("should still be nominated, got %s", prop.Stage)
	}

	// At deadline — expires.
	k.ProcessSeatElectionExpiry(ctx, 150)
	prop, _ = k.GetSeatElection(ctx, 1)
	if prop.Stage != types.SeatStageExpired {
		t.Errorf("should be expired, got %s", prop.Stage)
	}
}
```

**Step 2: Run tests**

Run: `go test ./x/gov/keeper/ -v -run "TestAcceptSeatNomination|TestSeatElection_FullLifecycle|TestSeatElection_Contested|TestSeatElection_Runoff|TestSeatElection_TermExpiry|TestSeatElection_VoteDoubleVote|TestSeatElection_NominationAutoExpiry"`
Expected: PASS

**Step 3: Commit**

```bash
git add x/gov/keeper/seat_election_test.go
git commit -m "R17-3: comprehensive seat election tests

- Full lifecycle: nomination → acceptance → discussion → voting → installation
- Contested election: highest yes_stake wins
- Runoff trigger: top 2 within 5% creates runoff proposals
- Term expiry: seat clears at end block
- Double-vote prevention
- Nomination auto-expiry on deadline"
```

---

### Task 6: BeginBlocker Integration

**Files:**
- Modify: `x/gov/keeper/abci.go` (add hooks at end of BeginBlocker)

**Step 1: Add seat election hooks to BeginBlocker**

In `x/gov/keeper/abci.go`, add after `k.ProcessResearchSpendExpiry(ctx, currentHeight)` (line 70):

```go
	// 5. Process seat election stage transitions (nominated→expired, discussion→voting).
	k.ProcessSeatElectionExpiry(ctx, currentHeight)

	// 6. Tally expired seat elections.
	k.TallySeatElections(ctx)

	// 7. Check community seat term expiry.
	k.CheckSeatTermExpiry(ctx)

	// 8. Check for long-vacant seats.
	k.CheckSeatVacancy(ctx)
```

**Step 2: Verify build**

Run: `go build ./x/gov/...`
Expected: BUILD SUCCESS

**Step 3: Run full test suite**

Run: `go test ./x/gov/keeper/ -v -count=1`
Expected: ALL PASS

**Step 4: Commit**

```bash
git add x/gov/keeper/abci.go
git commit -m "R17-3: BeginBlocker integration for seat elections

- ProcessSeatElectionExpiry: auto-advance nomination/discussion stages
- TallySeatElections: resolve contested/uncontested elections
- CheckSeatTermExpiry: expire seats at term end
- CheckSeatVacancy: 30-day warning, 90-day notice events"
```

---

### Task 7: Message Server + Query Handlers

**Files:**
- Modify: `x/gov/keeper/msg_server.go` (add handlers for new messages)
- Modify: `x/gov/keeper/grpc_query.go` (add query handlers)

**Step 1: Add message handlers to msg_server.go**

Add after `SetResearchVoters` handler (~line 408):

```go
// NominateSeatElection handles community seat nominations.
func (ms msgServer) NominateSeatElection(goCtx context.Context, msg *types.MsgNominateSeatElection) (*types.MsgNominateSeatElectionResponse, error) {
	ctx := sdk.UnwrapSDKContext(goCtx)
	return ms.keeper.NominateSeatElection(ctx, msg)
}

// AcceptSeatNomination handles candidate acceptance of a nomination.
func (ms msgServer) AcceptSeatNomination(goCtx context.Context, msg *types.MsgAcceptSeatNomination) (*types.MsgAcceptSeatNominationResponse, error) {
	ctx := sdk.UnwrapSDKContext(goCtx)
	return ms.keeper.AcceptSeatNomination(ctx, msg)
}

// VoteSeatElection handles votes on seat elections.
func (ms msgServer) VoteSeatElection(goCtx context.Context, msg *types.MsgVoteSeatElection) (*types.MsgVoteSeatElectionResponse, error) {
	ctx := sdk.UnwrapSDKContext(goCtx)
	return ms.keeper.VoteSeatElection(ctx, msg)
}
```

**Step 2: Add query handlers to grpc_query.go**

Add after `ResearchFundGovernance` handler:

```go
// SeatElection returns a seat election proposal by ID with its votes.
func (k Keeper) SeatElection(goCtx context.Context, req *types.QuerySeatElectionRequest) (*types.QuerySeatElectionResponse, error) {
	ctx := sdk.UnwrapSDKContext(goCtx)
	prop, found := k.GetSeatElection(ctx, req.ProposalId)
	if !found {
		return nil, types.ErrSeatElectionNotFound
	}
	votes := k.GetVotesForSeatElection(ctx, req.ProposalId)
	return &types.QuerySeatElectionResponse{Proposal: prop, Votes: votes}, nil
}

// SeatElections returns seat election proposals filtered by stage.
func (k Keeper) SeatElections(goCtx context.Context, req *types.QuerySeatElectionsRequest) (*types.QuerySeatElectionsResponse, error) {
	ctx := sdk.UnwrapSDKContext(goCtx)

	var all []*types.SeatElectionProposal
	if req.Stage != "" {
		all = k.GetSeatElectionsByStage(ctx, req.Stage)
	} else {
		all = k.GetAllSeatElections(ctx)
	}

	total := uint64(len(all))
	limit := req.Limit
	if limit == 0 || limit > 100 {
		limit = 100
	}
	offset := req.Offset
	if offset >= total {
		return &types.QuerySeatElectionsResponse{Total: total}, nil
	}
	end := offset + limit
	if end > total {
		end = total
	}

	return &types.QuerySeatElectionsResponse{
		Proposals: all[offset:end],
		Total:     total,
	}, nil
}

// ResearchFundSeats returns the current community seat state.
func (k Keeper) ResearchFundSeats(goCtx context.Context, _ *types.QueryResearchFundSeatsRequest) (*types.QueryResearchFundSeatsResponse, error) {
	ctx := sdk.UnwrapSDKContext(goCtx)
	state := k.GetResearchFundGovernanceState(ctx)
	activeCount := k.GetActiveCommunitySeatCount(ctx)
	return &types.QueryResearchFundSeatsResponse{
		CommunitySeats:     state.CommunitySeats,
		SeatTermEndBlocks:  state.SeatTermEndBlocks,
		ActiveSeatCount:    activeCount,
	}, nil
}
```

**Step 3: Verify build + run tests**

Run: `go build ./x/gov/... && go test ./x/gov/keeper/ -v -count=1`
Expected: BUILD SUCCESS, ALL PASS

**Step 4: Commit**

```bash
git add x/gov/keeper/msg_server.go x/gov/keeper/grpc_query.go
git commit -m "R17-3: message server + query handlers for seat elections

- NominateSeatElection, AcceptSeatNomination, VoteSeatElection msg handlers
- SeatElection, SeatElections, ResearchFundSeats query handlers
- Paginated election listing with stage filter"
```

---

### Task 8: Genesis Export/Import

**Files:**
- Modify: `x/gov/keeper/state.go` (update InitGenesis/ExportGenesis)

**Step 1: Update InitGenesis**

In `InitGenesis()` in state.go, add after the existing genesis restore logic:

```go
	// Restore seat elections.
	for _, se := range gs.SeatElections {
		k.SetSeatElection(ctx, se)
	}
	for _, v := range gs.SeatElectionVotes {
		k.SetSeatElectionVote(ctx, v)
	}
	if gs.NextSeatElectionNumber > 0 {
		k.SetNextSeatElectionID(ctx, gs.NextSeatElectionNumber)
	}
```

**Step 2: Update ExportGenesis**

In `ExportGenesis()`, add to the returned GenesisState:

```go
		SeatElections:          k.GetAllSeatElections(ctx),
		SeatElectionVotes:      k.GetAllSeatElectionVotes(ctx),
		NextSeatElectionNumber: k.GetNextSeatElectionID(ctx),
```

**Step 3: Verify build + tests**

Run: `go build ./x/gov/... && go test ./x/gov/keeper/ -v -count=1`
Expected: BUILD SUCCESS, ALL PASS

**Step 4: Commit**

```bash
git add x/gov/keeper/state.go
git commit -m "R17-3: genesis export/import for seat elections

- InitGenesis restores seat elections, votes, and counter
- ExportGenesis exports seat election state"
```

---

### Task 9: CLI Commands

**Files:**
- Modify: `x/gov/client/cli/tx.go` (add nominate, accept, vote commands)
- Modify: `x/gov/client/cli/query.go` (add seats, seat-election, seat-elections queries)

**Step 1: Add CLI tx commands to tx.go**

Add to `GetTxCmd()` return (add to the command list):
```go
		NewNominateSeatElectionCmd(),
		NewAcceptSeatNominationCmd(),
		NewVoteSeatElectionCmd(),
```

Add command implementations:
```go
// NewNominateSeatElectionCmd returns a CLI command for nominating a seat election candidate.
func NewNominateSeatElectionCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "nominate-research-seat",
		Short: "Nominate a candidate for a research fund community seat",
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientTxContext(cmd)
			if err != nil {
				return err
			}

			candidate, _ := cmd.Flags().GetString("candidate")
			seatIndex, _ := cmd.Flags().GetUint32("seat-index")
			statement, _ := cmd.Flags().GetString("statement")

			msg := &types.MsgNominateSeatElection{
				Proposer:  clientCtx.GetFromAddress().String(),
				Candidate: candidate,
				SeatIndex: seatIndex,
				Statement: statement,
			}

			return tx.GenerateOrBroadcastTxCLI(clientCtx, cmd.Flags(), msg)
		},
	}

	cmd.Flags().String("candidate", "", "Candidate bech32 address")
	cmd.Flags().Uint32("seat-index", 0, "Seat index (0 for Phase 1; 0-2 for Phase 2)")
	cmd.Flags().String("statement", "", "Candidate's governance statement (max 2000 chars)")
	_ = cmd.MarkFlagRequired("candidate")
	flags.AddTxFlagsToCmd(cmd)
	return cmd
}

// NewAcceptSeatNominationCmd returns a CLI command for accepting a seat nomination.
func NewAcceptSeatNominationCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "accept-research-nomination",
		Short: "Accept a pending seat election nomination",
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientTxContext(cmd)
			if err != nil {
				return err
			}

			proposalID, _ := cmd.Flags().GetUint64("proposal-id")

			msg := &types.MsgAcceptSeatNomination{
				Candidate:  clientCtx.GetFromAddress().String(),
				ProposalId: proposalID,
			}

			return tx.GenerateOrBroadcastTxCLI(clientCtx, cmd.Flags(), msg)
		},
	}

	cmd.Flags().Uint64("proposal-id", 0, "Seat election proposal ID")
	_ = cmd.MarkFlagRequired("proposal-id")
	flags.AddTxFlagsToCmd(cmd)
	return cmd
}

// NewVoteSeatElectionCmd returns a CLI command for voting on a seat election.
func NewVoteSeatElectionCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "vote-seat-election",
		Short: "Cast a vote on a seat election",
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientTxContext(cmd)
			if err != nil {
				return err
			}

			proposalID, _ := cmd.Flags().GetUint64("proposal-id")
			option, _ := cmd.Flags().GetString("option")

			msg := &types.MsgVoteSeatElection{
				Voter:      clientCtx.GetFromAddress().String(),
				ProposalId: proposalID,
				Option:     option,
			}

			return tx.GenerateOrBroadcastTxCLI(clientCtx, cmd.Flags(), msg)
		},
	}

	cmd.Flags().Uint64("proposal-id", 0, "Seat election proposal ID")
	cmd.Flags().String("option", "", "Vote option: yes, no, abstain")
	_ = cmd.MarkFlagRequired("proposal-id")
	_ = cmd.MarkFlagRequired("option")
	flags.AddTxFlagsToCmd(cmd)
	return cmd
}
```

**Step 2: Add CLI query commands to query.go**

Add to `GetQueryCmd()` return:
```go
		NewQueryResearchFundSeatsCmd(),
		NewQuerySeatElectionCmd(),
		NewQuerySeatElectionsCmd(),
```

Add command implementations:
```go
// NewQueryResearchFundSeatsCmd returns a CLI command for querying community seats.
func NewQueryResearchFundSeatsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "research-fund-seats",
		Short: "Query current community seat holders",
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientQueryContext(cmd)
			if err != nil {
				return err
			}

			queryClient := types.NewQueryClient(clientCtx)
			res, err := queryClient.ResearchFundSeats(cmd.Context(), &types.QueryResearchFundSeatsRequest{})
			if err != nil {
				return err
			}

			return clientCtx.PrintProto(res)
		},
	}

	flags.AddQueryFlagsToCmd(cmd)
	return cmd
}

// NewQuerySeatElectionCmd returns a CLI command for querying a seat election by ID.
func NewQuerySeatElectionCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "seat-election [proposal-id]",
		Short: "Query a seat election proposal by ID",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientQueryContext(cmd)
			if err != nil {
				return err
			}

			proposalID, err := strconv.ParseUint(args[0], 10, 64)
			if err != nil {
				return fmt.Errorf("invalid proposal-id: %w", err)
			}

			queryClient := types.NewQueryClient(clientCtx)
			res, err := queryClient.SeatElection(cmd.Context(), &types.QuerySeatElectionRequest{ProposalId: proposalID})
			if err != nil {
				return err
			}

			return clientCtx.PrintProto(res)
		},
	}

	flags.AddQueryFlagsToCmd(cmd)
	return cmd
}

// NewQuerySeatElectionsCmd returns a CLI command for listing seat elections.
func NewQuerySeatElectionsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "seat-elections",
		Short: "List seat election proposals",
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientQueryContext(cmd)
			if err != nil {
				return err
			}

			stage, _ := cmd.Flags().GetString("stage")
			limit, _ := cmd.Flags().GetUint64("limit")
			offset, _ := cmd.Flags().GetUint64("offset")

			queryClient := types.NewQueryClient(clientCtx)
			res, err := queryClient.SeatElections(cmd.Context(), &types.QuerySeatElectionsRequest{
				Stage: stage, Limit: limit, Offset: offset,
			})
			if err != nil {
				return err
			}

			return clientCtx.PrintProto(res)
		},
	}

	cmd.Flags().String("stage", "", "Filter by stage")
	cmd.Flags().Uint64("limit", 100, "Max results")
	cmd.Flags().Uint64("offset", 0, "Result offset")
	flags.AddQueryFlagsToCmd(cmd)
	return cmd
}
```

**Step 3: Verify build**

Run: `go build ./x/gov/...`
Expected: BUILD SUCCESS

**Step 4: Commit**

```bash
git add x/gov/client/cli/tx.go x/gov/client/cli/query.go
git commit -m "R17-3: CLI commands for seat elections

- tx: nominate-research-seat, accept-research-nomination, vote-seat-election
- query: research-fund-seats, seat-election, seat-elections (with stage filter)"
```

---

### Task 10: Final Verification

**Step 1: Full build**

Run: `go build ./...`
Expected: BUILD SUCCESS

**Step 2: Full test suite**

Run: `go test ./x/gov/keeper/ -v -count=1`
Expected: ALL PASS

**Step 3: Targeted test groups from spec**

Run:
```bash
go test ./x/gov/keeper/ -v -run "SeatElection"
go test ./x/gov/keeper/ -v -run "TermExpiry"
go test ./x/gov/keeper/ -v -run "Vacancy"
go test ./x/gov/keeper/ -v -run "ValidateCandidate"
```
Expected: ALL PASS

**Step 4: Final commit**

```bash
git add -A
git commit -m "R17-3: community seat elections + term rotation

- research_seat_election LIP category (500 ZRN stake, Guardian-only)
- SeatElectionProposal type with candidate acceptance flow
- Contested elections: highest yes_stake wins, runoff if within 5%
- Staggered 6-month terms (Phase 2: rotate 1 of 3 every 2 months)
- BeginBlocker term expiry check with vacancy handling
- Emergency removal via 75%/80% supermajority
- CLI: nominate-research-seat, accept-research-nomination, query seats"
```
