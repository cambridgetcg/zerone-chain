package keeper_test

import (
	"context"
	"fmt"
	"testing"

	"cosmossdk.io/log"
	"cosmossdk.io/store"
	storemetrics "cosmossdk.io/store/metrics"
	storetypes "cosmossdk.io/store/types"

	dbm "github.com/cosmos/cosmos-db"
	"github.com/cosmos/cosmos-sdk/codec"
	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	sdk "github.com/cosmos/cosmos-sdk/types"

	cmtproto "github.com/cometbft/cometbft/proto/tendermint/types"

	"github.com/zerone-chain/zerone/x/gov/keeper"
	"github.com/zerone-chain/zerone/x/gov/types"
)

// ---------- Mock Staking Keeper with Guardian Support ----------

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

// ---------- Setup Helper ----------

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
	k := keeper.NewKeeper(
		cdc,
		storeKey,
		"authority",
		nil,  // bankKeeper
		mock, // stakingKeeper
	)

	ctx := sdk.NewContext(stateStore, cmtproto.Header{Height: 100}, false, log.NewNopLogger())
	k.SetParams(ctx, types.DefaultParams())

	return k, ctx, mock
}

// setupPhaseObserver sets the governance state to Phase 1 (Observer) with
// one empty community seat and configures the phase properly for elections.
func setupPhaseObserver(t *testing.T, k keeper.Keeper, ctx sdk.Context) {
	t.Helper()
	state := k.GetResearchFundGovernanceState(ctx)
	state.CurrentPhase = types.ResearchFundPhase_RESEARCH_FUND_PHASE_OBSERVER
	state.CommunitySeats = []string{""}
	state.SeatTermEndBlocks = []uint64{0}
	k.SetResearchFundGovernanceState(ctx, state)
}

// seedCandidateVotes creates the specified number of distinct LIP votes for the candidate.
func seedCandidateVotes(t *testing.T, k keeper.Keeper, ctx sdk.Context, candidate string, count int) {
	t.Helper()
	for i := 0; i < count; i++ {
		k.SetVote(ctx, &types.Vote{
			LipId:  fmt.Sprintf("LIP-%d", i+1),
			Voter:  candidate,
			Option: "yes",
			Weight: "1000000",
		})
	}
}

// ---------- Task 3 Tests: State CRUD ----------

func TestSeatElection_CRUD(t *testing.T) {
	k, ctx := setupKeeper(t)

	// Initially empty.
	all := k.GetAllSeatElections(ctx)
	if len(all) != 0 {
		t.Fatalf("expected 0 seat elections, got %d", len(all))
	}

	// Set a seat election.
	prop := &types.SeatElectionProposal{
		ProposalId:   1,
		Proposer:     testAddr("alice"),
		Candidate:    testAddr("bob"),
		SeatIndex:    0,
		Statement:    "I will improve governance",
		Stage:        types.SeatStageNominated,
		YesStake:     "0",
		NoStake:      "0",
		AbstainStake: "0",
	}
	k.SetSeatElection(ctx, prop)

	// Get should return it.
	got, found := k.GetSeatElection(ctx, 1)
	if !found {
		t.Fatal("expected to find seat election 1")
	}
	if got.Proposer != testAddr("alice") {
		t.Errorf("expected proposer=%s, got %s", testAddr("alice"), got.Proposer)
	}
	if got.Candidate != testAddr("bob") {
		t.Errorf("expected candidate=%s, got %s", testAddr("bob"), got.Candidate)
	}
	if got.Stage != types.SeatStageNominated {
		t.Errorf("expected stage=nominated, got %s", got.Stage)
	}

	// Not found for non-existent ID.
	_, found = k.GetSeatElection(ctx, 999)
	if found {
		t.Error("should not find non-existent seat election")
	}

	// Set a second one and iterate.
	prop2 := &types.SeatElectionProposal{
		ProposalId:   2,
		Proposer:     testAddr("charlie"),
		Candidate:    testAddr("dave"),
		SeatIndex:    0,
		Stage:        types.SeatStageVoting,
		YesStake:     "100",
		NoStake:      "50",
		AbstainStake: "0",
	}
	k.SetSeatElection(ctx, prop2)

	all = k.GetAllSeatElections(ctx)
	if len(all) != 2 {
		t.Fatalf("expected 2 seat elections, got %d", len(all))
	}

	// Iterate with callback.
	var count int
	k.IterateSeatElections(ctx, func(p *types.SeatElectionProposal) bool {
		count++
		return false
	})
	if count != 2 {
		t.Errorf("expected iterate count=2, got %d", count)
	}
}

func TestSeatElection_Counter(t *testing.T) {
	k, ctx := setupKeeper(t)

	// Default counter should be 1.
	id := k.GetNextSeatElectionID(ctx)
	if id != 1 {
		t.Errorf("expected next ID=1, got %d", id)
	}

	// Set and verify.
	k.SetNextSeatElectionID(ctx, 5)
	id = k.GetNextSeatElectionID(ctx)
	if id != 5 {
		t.Errorf("expected next ID=5, got %d", id)
	}

	// Increment.
	k.SetNextSeatElectionID(ctx, id+1)
	id = k.GetNextSeatElectionID(ctx)
	if id != 6 {
		t.Errorf("expected next ID=6, got %d", id)
	}
}

func TestSeatElection_VoteCRUD(t *testing.T) {
	k, ctx := setupKeeper(t)

	// No votes initially.
	votes := k.GetAllSeatElectionVotes(ctx)
	if len(votes) != 0 {
		t.Fatalf("expected 0 votes, got %d", len(votes))
	}

	// Has not voted.
	if k.HasSeatElectionVoted(ctx, 1, testAddr("alice")) {
		t.Error("alice should not have voted yet")
	}

	// Set a vote.
	vote := &types.SeatElectionVote{
		ProposalId: 1,
		Voter:      testAddr("alice"),
		Option:     "yes",
		Stake:      "500000",
		Block:      100,
	}
	k.SetSeatElectionVote(ctx, vote)

	// Now has voted (dedupe key).
	if !k.HasSeatElectionVoted(ctx, 1, testAddr("alice")) {
		t.Error("alice should have voted now")
	}

	// Get vote.
	got, found := k.GetSeatElectionVote(ctx, 1, testAddr("alice"))
	if !found {
		t.Fatal("expected to find vote")
	}
	if got.Stake != "500000" {
		t.Errorf("expected stake=500000, got %s", got.Stake)
	}
	if got.Option != "yes" {
		t.Errorf("expected option=yes, got %s", got.Option)
	}

	// Not found for different voter.
	_, found = k.GetSeatElectionVote(ctx, 1, testAddr("bob"))
	if found {
		t.Error("should not find vote for bob")
	}

	// Add another vote and check iteration.
	vote2 := &types.SeatElectionVote{
		ProposalId: 1,
		Voter:      testAddr("bob"),
		Option:     "no",
		Stake:      "300000",
		Block:      100,
	}
	k.SetSeatElectionVote(ctx, vote2)

	votesForProposal := k.GetVotesForSeatElection(ctx, 1)
	if len(votesForProposal) != 2 {
		t.Errorf("expected 2 votes for proposal 1, got %d", len(votesForProposal))
	}

	// Votes for non-existent proposal.
	votesForEmpty := k.GetVotesForSeatElection(ctx, 999)
	if len(votesForEmpty) != 0 {
		t.Errorf("expected 0 votes for proposal 999, got %d", len(votesForEmpty))
	}

	// All votes.
	allVotes := k.GetAllSeatElectionVotes(ctx)
	if len(allVotes) != 2 {
		t.Errorf("expected 2 total votes, got %d", len(allVotes))
	}
}

func TestSeatElection_IterateByStage(t *testing.T) {
	k, ctx := setupKeeper(t)

	// Create proposals in different stages.
	k.SetSeatElection(ctx, &types.SeatElectionProposal{
		ProposalId: 1, Stage: types.SeatStageNominated, YesStake: "0", NoStake: "0", AbstainStake: "0",
	})
	k.SetSeatElection(ctx, &types.SeatElectionProposal{
		ProposalId: 2, Stage: types.SeatStageVoting, YesStake: "0", NoStake: "0", AbstainStake: "0",
	})
	k.SetSeatElection(ctx, &types.SeatElectionProposal{
		ProposalId: 3, Stage: types.SeatStageVoting, YesStake: "0", NoStake: "0", AbstainStake: "0",
	})
	k.SetSeatElection(ctx, &types.SeatElectionProposal{
		ProposalId: 4, Stage: types.SeatStagePassed, YesStake: "0", NoStake: "0", AbstainStake: "0",
	})

	// Filter by stage.
	nominated := k.GetSeatElectionsByStage(ctx, types.SeatStageNominated)
	if len(nominated) != 1 {
		t.Errorf("expected 1 nominated, got %d", len(nominated))
	}

	voting := k.GetSeatElectionsByStage(ctx, types.SeatStageVoting)
	if len(voting) != 2 {
		t.Errorf("expected 2 voting, got %d", len(voting))
	}

	passed := k.GetSeatElectionsByStage(ctx, types.SeatStagePassed)
	if len(passed) != 1 {
		t.Errorf("expected 1 passed, got %d", len(passed))
	}

	discussion := k.GetSeatElectionsByStage(ctx, types.SeatStageDiscussion)
	if len(discussion) != 0 {
		t.Errorf("expected 0 discussion, got %d", len(discussion))
	}
}

// ---------- Task 4 Tests: Core Election Logic ----------

func TestValidateSeatCandidate_NotGuardian(t *testing.T) {
	k, ctx, _ := setupWithGuardianStaking(t, "1000000000000")

	candidate := testAddr("candidate")
	// Not a guardian — should fail.
	err := k.ValidateSeatCandidate(ctx, candidate)
	if err == nil {
		t.Fatal("expected error for non-guardian candidate")
	}
	if err != types.ErrNotGuardianTier {
		t.Errorf("expected ErrNotGuardianTier, got %v", err)
	}
}

func TestValidateSeatCandidate_InsufficientVotes(t *testing.T) {
	k, ctx, mock := setupWithGuardianStaking(t, "1000000000000")

	candidate := testAddr("candidate")
	mock.guardians[candidate] = true

	// Only 3 LIP votes — need 5.
	seedCandidateVotes(t, k, ctx, candidate, 3)

	err := k.ValidateSeatCandidate(ctx, candidate)
	if err == nil {
		t.Fatal("expected error for insufficient votes")
	}
	if err != types.ErrInsufficientGovHistory {
		t.Errorf("expected ErrInsufficientGovHistory, got %v", err)
	}
}

func TestValidateSeatCandidate_AlreadyHoldsSeat(t *testing.T) {
	k, ctx, mock := setupWithGuardianStaking(t, "1000000000000")

	candidate := testAddr("candidate")
	mock.guardians[candidate] = true
	seedCandidateVotes(t, k, ctx, candidate, 5)

	// Set candidate as already holding a seat.
	state := k.GetResearchFundGovernanceState(ctx)
	state.CommunitySeats = []string{candidate}
	state.SeatTermEndBlocks = []uint64{999999}
	k.SetResearchFundGovernanceState(ctx, state)

	err := k.ValidateSeatCandidate(ctx, candidate)
	if err == nil {
		t.Fatal("expected error for candidate already holding seat")
	}
	if err != types.ErrSeatAlreadyHeld {
		t.Errorf("expected ErrSeatAlreadyHeld, got %v", err)
	}
}

func TestValidateSeatCandidate_Valid(t *testing.T) {
	k, ctx, mock := setupWithGuardianStaking(t, "1000000000000")

	candidate := testAddr("candidate")
	mock.guardians[candidate] = true
	seedCandidateVotes(t, k, ctx, candidate, 5)

	// Empty seats — candidate doesn't hold one.
	state := k.GetResearchFundGovernanceState(ctx)
	state.CommunitySeats = []string{""}
	state.SeatTermEndBlocks = []uint64{0}
	k.SetResearchFundGovernanceState(ctx, state)

	err := k.ValidateSeatCandidate(ctx, candidate)
	if err != nil {
		t.Fatalf("expected no error for valid candidate, got %v", err)
	}
}

func TestNominateSeatElection_Success(t *testing.T) {
	k, ctx, _ := setupWithGuardianStaking(t, "1000000000000")
	setupPhaseObserver(t, k, ctx)

	resp, err := k.NominateSeatElection(ctx, &types.MsgNominateSeatElection{
		Proposer:  testAddr("nominator"),
		Candidate: testAddr("candidate"),
		SeatIndex: 0,
		Statement: "I will serve the community well",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.ProposalId != 1 {
		t.Errorf("expected proposal_id=1, got %d", resp.ProposalId)
	}

	// Verify stored proposal.
	prop, found := k.GetSeatElection(ctx, 1)
	if !found {
		t.Fatal("expected to find seat election 1")
	}
	if prop.Stage != types.SeatStageNominated {
		t.Errorf("expected stage=nominated, got %s", prop.Stage)
	}
	if prop.Candidate != testAddr("candidate") {
		t.Errorf("expected candidate=%s, got %s", testAddr("candidate"), prop.Candidate)
	}
	if prop.AcceptanceDeadline != uint64(100)+types.SeatAcceptanceBlocks {
		t.Errorf("expected acceptance_deadline=%d, got %d", uint64(100)+types.SeatAcceptanceBlocks, prop.AcceptanceDeadline)
	}
	if prop.CandidateAccepted {
		t.Error("candidate_accepted should be false at nomination stage")
	}

	// Counter should have incremented.
	nextID := k.GetNextSeatElectionID(ctx)
	if nextID != 2 {
		t.Errorf("expected next ID=2, got %d", nextID)
	}
}

func TestNominateSeatElection_InvalidSeatIndex(t *testing.T) {
	k, ctx, _ := setupWithGuardianStaking(t, "1000000000000")
	setupPhaseObserver(t, k, ctx)

	// Phase 1 (Observer) only allows seat index 0. Index 1 should fail.
	_, err := k.NominateSeatElection(ctx, &types.MsgNominateSeatElection{
		Proposer:  testAddr("nominator"),
		Candidate: testAddr("candidate"),
		SeatIndex: 1,
		Statement: "Oops, wrong seat",
	})
	if err == nil {
		t.Fatal("expected error for invalid seat index")
	}
	if err != types.ErrInvalidSeatIndex {
		t.Errorf("expected ErrInvalidSeatIndex, got %v", err)
	}
}

func TestAcceptSeatNomination_Success(t *testing.T) {
	k, ctx, mock := setupWithGuardianStaking(t, "1000000000000")
	setupPhaseObserver(t, k, ctx)

	candidate := testAddr("candidate")
	mock.guardians[candidate] = true
	seedCandidateVotes(t, k, ctx, candidate, 5)

	// Nominate.
	resp, err := k.NominateSeatElection(ctx, &types.MsgNominateSeatElection{
		Proposer:  testAddr("nominator"),
		Candidate: candidate,
		SeatIndex: 0,
		Statement: "Great candidate",
	})
	if err != nil {
		t.Fatalf("nominate failed: %v", err)
	}

	// Accept.
	_, err = k.AcceptSeatNomination(ctx, &types.MsgAcceptSeatNomination{
		Candidate:  candidate,
		ProposalId: resp.ProposalId,
	})
	if err != nil {
		t.Fatalf("accept failed: %v", err)
	}

	// Verify advanced to discussion.
	prop, _ := k.GetSeatElection(ctx, resp.ProposalId)
	if prop.Stage != types.SeatStageDiscussion {
		t.Errorf("expected stage=discussion, got %s", prop.Stage)
	}
	if !prop.CandidateAccepted {
		t.Error("expected candidate_accepted=true")
	}
	if prop.DiscussionEndBlock != uint64(100)+types.SeatDiscussionBlocks {
		t.Errorf("expected discussion_end_block=%d, got %d", uint64(100)+types.SeatDiscussionBlocks, prop.DiscussionEndBlock)
	}
}

func TestAcceptSeatNomination_Expired(t *testing.T) {
	k, ctx, mock := setupWithGuardianStaking(t, "1000000000000")
	setupPhaseObserver(t, k, ctx)

	candidate := testAddr("candidate")
	mock.guardians[candidate] = true
	seedCandidateVotes(t, k, ctx, candidate, 5)

	// Nominate at block 100.
	resp, err := k.NominateSeatElection(ctx, &types.MsgNominateSeatElection{
		Proposer:  testAddr("nominator"),
		Candidate: candidate,
		SeatIndex: 0,
	})
	if err != nil {
		t.Fatalf("nominate failed: %v", err)
	}

	// Try to accept after deadline.
	futureHeight := int64(100) + int64(types.SeatAcceptanceBlocks) + 1
	ctx2 := ctx.WithBlockHeight(futureHeight)
	_, err = k.AcceptSeatNomination(ctx2, &types.MsgAcceptSeatNomination{
		Candidate:  candidate,
		ProposalId: resp.ProposalId,
	})
	if err == nil {
		t.Fatal("expected error for expired nomination")
	}
	if err != types.ErrSeatNominationExpired {
		t.Errorf("expected ErrSeatNominationExpired, got %v", err)
	}
}

func TestSeatElection_FullLifecycle(t *testing.T) {
	k, ctx, mock := setupWithGuardianStaking(t, "1000000000000")
	setupPhaseObserver(t, k, ctx)

	candidate := testAddr("candidate")
	mock.guardians[candidate] = true
	seedCandidateVotes(t, k, ctx, candidate, 5)
	mock.delegations[testAddr("voter1")] = "500000000000" // 50% of total bonded

	// 1. Nominate.
	resp, err := k.NominateSeatElection(ctx, &types.MsgNominateSeatElection{
		Proposer:  testAddr("nominator"),
		Candidate: candidate,
		SeatIndex: 0,
		Statement: "Full lifecycle test",
	})
	if err != nil {
		t.Fatalf("nominate failed: %v", err)
	}

	// 2. Accept.
	_, err = k.AcceptSeatNomination(ctx, &types.MsgAcceptSeatNomination{
		Candidate:  candidate,
		ProposalId: resp.ProposalId,
	})
	if err != nil {
		t.Fatalf("accept failed: %v", err)
	}

	prop, _ := k.GetSeatElection(ctx, resp.ProposalId)
	if prop.Stage != types.SeatStageDiscussion {
		t.Fatalf("expected discussion, got %s", prop.Stage)
	}

	// 3. Auto-advance discussion → voting via ProcessSeatElectionExpiry.
	discussionEnd := int64(prop.DiscussionEndBlock)
	ctx3 := ctx.WithBlockHeight(discussionEnd)
	k.ProcessSeatElectionExpiry(ctx3, uint64(discussionEnd))

	prop, _ = k.GetSeatElection(ctx3, resp.ProposalId)
	if prop.Stage != types.SeatStageVoting {
		t.Fatalf("expected voting, got %s", prop.Stage)
	}
	if prop.VotingEndBlock == 0 {
		t.Fatal("voting_end_block should be set")
	}

	// 4. Cast vote with majority stake.
	votingCtx := ctx.WithBlockHeight(discussionEnd + 1)
	_, err = k.VoteSeatElection(votingCtx, &types.MsgVoteSeatElection{
		Voter:      testAddr("voter1"),
		ProposalId: resp.ProposalId,
		Option:     types.VoteYes,
	})
	if err != nil {
		t.Fatalf("vote failed: %v", err)
	}

	// 5. Tally after voting period.
	tallyCtx := ctx.WithBlockHeight(int64(prop.VotingEndBlock))
	k.TallySeatElections(tallyCtx)

	prop, _ = k.GetSeatElection(tallyCtx, resp.ProposalId)
	if prop.Stage != types.SeatStagePassed {
		t.Fatalf("expected passed, got %s", prop.Stage)
	}

	// 6. Verify seat installed.
	state := k.GetResearchFundGovernanceState(tallyCtx)
	if len(state.CommunitySeats) == 0 || state.CommunitySeats[0] != candidate {
		t.Errorf("expected community seat 0 = %s, got %v", candidate, state.CommunitySeats)
	}
	if len(state.SeatTermEndBlocks) == 0 || state.SeatTermEndBlocks[0] == 0 {
		t.Error("expected seat term end block to be set")
	}
}

func TestSeatElection_ContestedHighestWins(t *testing.T) {
	k, ctx, mock := setupWithGuardianStaking(t, "1000000000000")
	setupPhaseObserver(t, k, ctx)

	candidate1 := testAddr("candidate1")
	candidate2 := testAddr("candidate2")
	mock.guardians[candidate1] = true
	mock.guardians[candidate2] = true
	seedCandidateVotes(t, k, ctx, candidate1, 5)
	seedCandidateVotes(t, k, ctx, candidate2, 5)

	// Create two proposals for the same seat (both already in voting).
	k.SetSeatElection(ctx, &types.SeatElectionProposal{
		ProposalId:        1,
		Candidate:         candidate1,
		SeatIndex:         0,
		Stage:             types.SeatStageVoting,
		YesStake:          "600000000000", // 60% — clear leader
		NoStake:           "0",
		AbstainStake:      "0",
		VotingEndBlock:    100,
		CandidateAccepted: true,
	})
	k.SetSeatElection(ctx, &types.SeatElectionProposal{
		ProposalId:        2,
		Candidate:         candidate2,
		SeatIndex:         0,
		Stage:             types.SeatStageVoting,
		YesStake:          "200000000000", // 20% — far behind
		NoStake:           "0",
		AbstainStake:      "0",
		VotingEndBlock:    100,
		CandidateAccepted: true,
	})

	// Tally.
	k.TallySeatElections(ctx)

	prop1, _ := k.GetSeatElection(ctx, 1)
	prop2, _ := k.GetSeatElection(ctx, 2)

	if prop1.Stage != types.SeatStagePassed {
		t.Errorf("expected candidate1 to pass, got %s", prop1.Stage)
	}
	if prop2.Stage != types.SeatStageFailed {
		t.Errorf("expected candidate2 to fail, got %s", prop2.Stage)
	}

	// Verify seat installed for winner.
	state := k.GetResearchFundGovernanceState(ctx)
	if len(state.CommunitySeats) == 0 || state.CommunitySeats[0] != candidate1 {
		t.Errorf("expected seat 0 = %s, got %v", candidate1, state.CommunitySeats)
	}
}

func TestSeatElection_RunoffTriggered(t *testing.T) {
	k, ctx, mock := setupWithGuardianStaking(t, "1000000000000")
	setupPhaseObserver(t, k, ctx)

	candidate1 := testAddr("candidate1")
	candidate2 := testAddr("candidate2")
	mock.guardians[candidate1] = true
	mock.guardians[candidate2] = true

	// Create two proposals for the same seat with stakes within 5%.
	// candidate1: 500, candidate2: 490 — gap is 10, threshold is 500*50000/1000000 = 25.
	// 10 < 25 → runoff triggered.
	k.SetSeatElection(ctx, &types.SeatElectionProposal{
		ProposalId:        1,
		Proposer:          testAddr("nom1"),
		Candidate:         candidate1,
		SeatIndex:         0,
		Stage:             types.SeatStageVoting,
		Statement:         "Candidate 1 statement",
		YesStake:          "500000000000",
		NoStake:           "0",
		AbstainStake:      "0",
		VotingEndBlock:    100,
		CandidateAccepted: true,
	})
	k.SetSeatElection(ctx, &types.SeatElectionProposal{
		ProposalId:        2,
		Proposer:          testAddr("nom2"),
		Candidate:         candidate2,
		SeatIndex:         0,
		Stage:             types.SeatStageVoting,
		Statement:         "Candidate 2 statement",
		YesStake:          "490000000000",
		NoStake:           "0",
		AbstainStake:      "0",
		VotingEndBlock:    100,
		CandidateAccepted: true,
	})
	k.SetNextSeatElectionID(ctx, 3) // Next ID starts at 3.

	// Tally.
	k.TallySeatElections(ctx)

	// Originals should be in "runoff" stage.
	prop1, _ := k.GetSeatElection(ctx, 1)
	prop2, _ := k.GetSeatElection(ctx, 2)
	if prop1.Stage != types.SeatStageRunoff {
		t.Errorf("expected original 1 stage=runoff, got %s", prop1.Stage)
	}
	if prop2.Stage != types.SeatStageRunoff {
		t.Errorf("expected original 2 stage=runoff, got %s", prop2.Stage)
	}

	// Two new runoff proposals should exist.
	runoff1, found1 := k.GetSeatElection(ctx, 3)
	runoff2, found2 := k.GetSeatElection(ctx, 4)
	if !found1 || !found2 {
		t.Fatal("expected two runoff proposals to be created")
	}

	if !runoff1.IsRunoff {
		t.Error("runoff1 should have is_runoff=true")
	}
	if !runoff2.IsRunoff {
		t.Error("runoff2 should have is_runoff=true")
	}
	if runoff1.Stage != types.SeatStageVoting {
		t.Errorf("expected runoff1 stage=voting, got %s", runoff1.Stage)
	}
	if runoff2.Stage != types.SeatStageVoting {
		t.Errorf("expected runoff2 stage=voting, got %s", runoff2.Stage)
	}
	if runoff1.Candidate != candidate1 {
		t.Errorf("expected runoff1 candidate=%s, got %s", candidate1, runoff1.Candidate)
	}
	if runoff2.Candidate != candidate2 {
		t.Errorf("expected runoff2 candidate=%s, got %s", candidate2, runoff2.Candidate)
	}
	if len(runoff1.RunoffParentIds) != 2 {
		t.Errorf("expected 2 parent IDs, got %d", len(runoff1.RunoffParentIds))
	}
	if runoff1.VotingEndBlock != 100+types.SeatVotingBlocks {
		t.Errorf("expected voting_end_block=%d, got %d", 100+types.SeatVotingBlocks, runoff1.VotingEndBlock)
	}
}

func TestSeatElection_TermExpiry(t *testing.T) {
	k, ctx, _ := setupWithGuardianStaking(t, "1000000000000")

	candidate := testAddr("seatHolder")

	// Install a seat with a known term end.
	state := k.GetResearchFundGovernanceState(ctx)
	state.CurrentPhase = types.ResearchFundPhase_RESEARCH_FUND_PHASE_OBSERVER
	state.CommunitySeats = []string{candidate}
	state.SeatTermEndBlocks = []uint64{200} // Expires at block 200.
	k.SetResearchFundGovernanceState(ctx, state)

	// Before expiry — seat should be occupied.
	state = k.GetResearchFundGovernanceState(ctx)
	if state.CommunitySeats[0] != candidate {
		t.Errorf("expected seat holder=%s, got %s", candidate, state.CommunitySeats[0])
	}

	// After expiry — seat should be cleared.
	ctx2 := ctx.WithBlockHeight(200)
	k.CheckSeatTermExpiry(ctx2)

	state = k.GetResearchFundGovernanceState(ctx2)
	if state.CommunitySeats[0] != "" {
		t.Errorf("expected seat to be cleared after expiry, got %s", state.CommunitySeats[0])
	}
	if state.SeatTermEndBlocks[0] != 0 {
		t.Errorf("expected term end block to be 0 after expiry, got %d", state.SeatTermEndBlocks[0])
	}
}

func TestSeatElection_VoteDoubleVote(t *testing.T) {
	k, ctx, mock := setupWithGuardianStaking(t, "1000000000000")
	setupPhaseObserver(t, k, ctx)

	mock.delegations[testAddr("voter")] = "100000000000"

	// Create a proposal in voting stage.
	k.SetSeatElection(ctx, &types.SeatElectionProposal{
		ProposalId:        1,
		Candidate:         testAddr("candidate"),
		SeatIndex:         0,
		Stage:             types.SeatStageVoting,
		YesStake:          "0",
		NoStake:           "0",
		AbstainStake:      "0",
		VotingEndBlock:    200,
		CandidateAccepted: true,
	})

	// First vote — should succeed.
	_, err := k.VoteSeatElection(ctx, &types.MsgVoteSeatElection{
		Voter:      testAddr("voter"),
		ProposalId: 1,
		Option:     types.VoteYes,
	})
	if err != nil {
		t.Fatalf("first vote failed: %v", err)
	}

	// Second vote — should fail.
	_, err = k.VoteSeatElection(ctx, &types.MsgVoteSeatElection{
		Voter:      testAddr("voter"),
		ProposalId: 1,
		Option:     types.VoteNo,
	})
	if err == nil {
		t.Fatal("expected error for double vote")
	}
	if err != types.ErrSeatElectionAlreadyVoted {
		t.Errorf("expected ErrSeatElectionAlreadyVoted, got %v", err)
	}
}

func TestSeatElection_NominationAutoExpiry(t *testing.T) {
	k, ctx, _ := setupWithGuardianStaking(t, "1000000000000")
	setupPhaseObserver(t, k, ctx)

	// Nominate at block 100.
	resp, err := k.NominateSeatElection(ctx, &types.MsgNominateSeatElection{
		Proposer:  testAddr("nominator"),
		Candidate: testAddr("candidate"),
		SeatIndex: 0,
	})
	if err != nil {
		t.Fatalf("nominate failed: %v", err)
	}

	prop, _ := k.GetSeatElection(ctx, resp.ProposalId)
	if prop.Stage != types.SeatStageNominated {
		t.Fatalf("expected nominated, got %s", prop.Stage)
	}

	// Advance past acceptance deadline.
	expiryHeight := int64(prop.AcceptanceDeadline) + 1
	ctx2 := ctx.WithBlockHeight(expiryHeight)
	k.ProcessSeatElectionExpiry(ctx2, uint64(expiryHeight))

	prop, _ = k.GetSeatElection(ctx2, resp.ProposalId)
	if prop.Stage != types.SeatStageExpired {
		t.Errorf("expected expired, got %s", prop.Stage)
	}
}

func TestSeatElection_RunoffResolution(t *testing.T) {
	k, ctx, mock := setupWithGuardianStaking(t, "1000000000000")
	setupPhaseObserver(t, k, ctx)

	candidate1 := testAddr("candidate1")
	candidate2 := testAddr("candidate2")
	voter1 := testAddr("voter1")
	voter2 := testAddr("voter2")

	// Voter1 has 50% stake, voter2 has 40%.
	mock.delegations[voter1] = "500000000000"
	mock.delegations[voter2] = "400000000000"

	// Create two runoff proposals (is_runoff=true, stage=voting).
	k.SetSeatElection(ctx, &types.SeatElectionProposal{
		ProposalId:        1,
		Proposer:          testAddr("nom1"),
		Candidate:         candidate1,
		SeatIndex:         0,
		Stage:             types.SeatStageVoting,
		Statement:         "Runoff candidate 1",
		YesStake:          "0",
		NoStake:           "0",
		AbstainStake:      "0",
		VotingEndBlock:    200,
		CreatedAtBlock:    100,
		CandidateAccepted: true,
		IsRunoff:          true,
		RunoffParentIds:   []uint64{10, 11},
	})
	k.SetSeatElection(ctx, &types.SeatElectionProposal{
		ProposalId:        2,
		Proposer:          testAddr("nom2"),
		Candidate:         candidate2,
		SeatIndex:         0,
		Stage:             types.SeatStageVoting,
		Statement:         "Runoff candidate 2",
		YesStake:          "0",
		NoStake:           "0",
		AbstainStake:      "0",
		VotingEndBlock:    200,
		CreatedAtBlock:    100,
		CandidateAccepted: true,
		IsRunoff:          true,
		RunoffParentIds:   []uint64{10, 11},
	})
	k.SetNextSeatElectionID(ctx, 3)

	// Cast votes: voter1 (50%) votes yes on candidate1, voter2 (40%) votes yes on candidate2.
	votingCtx := ctx.WithBlockHeight(150)
	_, err := k.VoteSeatElection(votingCtx, &types.MsgVoteSeatElection{
		Voter:      voter1,
		ProposalId: 1,
		Option:     types.VoteYes,
	})
	if err != nil {
		t.Fatalf("vote on runoff 1 failed: %v", err)
	}
	_, err = k.VoteSeatElection(votingCtx, &types.MsgVoteSeatElection{
		Voter:      voter2,
		ProposalId: 2,
		Option:     types.VoteYes,
	})
	if err != nil {
		t.Fatalf("vote on runoff 2 failed: %v", err)
	}

	// Tally after voting period ends.
	tallyCtx := ctx.WithBlockHeight(200)
	k.TallySeatElections(tallyCtx)

	// Candidate1 (higher yes_stake) should win.
	prop1, _ := k.GetSeatElection(tallyCtx, 1)
	prop2, _ := k.GetSeatElection(tallyCtx, 2)

	if prop1.Stage != types.SeatStagePassed {
		t.Errorf("expected runoff candidate1 to pass, got %s", prop1.Stage)
	}
	if prop2.Stage != types.SeatStageFailed {
		t.Errorf("expected runoff candidate2 to fail, got %s", prop2.Stage)
	}

	// Verify seat installed for winner.
	state := k.GetResearchFundGovernanceState(tallyCtx)
	if len(state.CommunitySeats) == 0 || state.CommunitySeats[0] != candidate1 {
		t.Errorf("expected community seat 0 = %s, got %v", candidate1, state.CommunitySeats)
	}

	// Verify no further runoff proposals were created (runoff resolves directly).
	_, found := k.GetSeatElection(tallyCtx, 3)
	if found {
		t.Error("expected no further runoff proposals, but proposal 3 exists")
	}
}

func TestSeatElection_QuorumFailure(t *testing.T) {
	k, ctx, mock := setupWithGuardianStaking(t, "1000000000000")
	setupPhaseObserver(t, k, ctx)

	candidate := testAddr("candidate")
	voter := testAddr("smallvoter")

	// Voter has only 1% of total bonded stake (10B out of 1T).
	// Quorum requires 33.4%, so 1% is far below threshold.
	mock.delegations[voter] = "10000000000"

	// Create a single-candidate election already in voting stage.
	k.SetSeatElection(ctx, &types.SeatElectionProposal{
		ProposalId:        1,
		Proposer:          testAddr("nominator"),
		Candidate:         candidate,
		SeatIndex:         0,
		Stage:             types.SeatStageVoting,
		Statement:         "Low turnout candidate",
		YesStake:          "0",
		NoStake:           "0",
		AbstainStake:      "0",
		VotingEndBlock:    200,
		CreatedAtBlock:    100,
		CandidateAccepted: true,
	})

	// Cast a small yes vote (1% of total bonded).
	votingCtx := ctx.WithBlockHeight(150)
	_, err := k.VoteSeatElection(votingCtx, &types.MsgVoteSeatElection{
		Voter:      voter,
		ProposalId: 1,
		Option:     types.VoteYes,
	})
	if err != nil {
		t.Fatalf("vote failed: %v", err)
	}

	// Tally after voting period.
	tallyCtx := ctx.WithBlockHeight(200)
	k.TallySeatElections(tallyCtx)

	// Proposal should fail due to insufficient quorum.
	prop, _ := k.GetSeatElection(tallyCtx, 1)
	if prop.Stage != types.SeatStageFailed {
		t.Errorf("expected proposal to fail due to quorum, got stage=%s", prop.Stage)
	}

	// Verify seat was NOT installed.
	state := k.GetResearchFundGovernanceState(tallyCtx)
	if len(state.CommunitySeats) > 0 && state.CommunitySeats[0] != "" {
		t.Errorf("expected seat 0 to remain empty, got %s", state.CommunitySeats[0])
	}
}
