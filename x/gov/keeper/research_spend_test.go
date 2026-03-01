package keeper_test

import (
	"fmt"
	"testing"

	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/zerone-chain/zerone/x/gov/keeper"
	"github.com/zerone-chain/zerone/x/gov/types"
)

// ---------- Mock VestingRewardsKeeper ----------

type mockVestingKeeper struct {
	disburseCalled bool
	disburseErr    error
}

func (m *mockVestingKeeper) DisburseFromResearchFund(_ sdk.Context, _ sdk.AccAddress, _ sdk.Coins) error {
	m.disburseCalled = true
	return m.disburseErr
}

// ---------- Helpers ----------

// setupResearchVoters configures voter1 and voter2 on the keeper and returns their addresses.
func setupResearchVoters(t *testing.T, k interface {
	SetResearchFundVoters(ctx sdk.Context, voters *types.ResearchFundVoters)
}, ctx sdk.Context) (voter1, voter2 string) {
	t.Helper()
	voter1 = testAddr("rfv1")
	voter2 = testAddr("rfv2")
	k.SetResearchFundVoters(ctx, &types.ResearchFundVoters{
		Voter1: voter1,
		Voter2: voter2,
	})
	return voter1, voter2
}

// ---------- Research Spend Tests ----------

func TestResearchSpend_SubmitByVoter1(t *testing.T) {
	k, ctx := setupKeeper(t)
	voter1, _ := setupResearchVoters(t, k, ctx)

	resp, err := k.SubmitResearchSpend(ctx, &types.MsgSubmitResearchSpend{
		Proposer:    voter1,
		Title:       "Fund AI safety research",
		Description: "Grant for adversarial testing framework",
		Recipient:   testAddr("researcher"),
		Amount:      "1000000000",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.ProposalId != 1 {
		t.Errorf("expected proposal_id=1, got %d", resp.ProposalId)
	}

	prop, found := k.GetResearchSpendProposal(ctx, 1)
	if !found {
		t.Fatal("proposal not found")
	}
	if prop.Stage != string(types.ResearchStageDiscussion) {
		t.Errorf("expected discussion stage, got %s", prop.Stage)
	}
	if prop.Proposer != voter1 {
		t.Errorf("wrong proposer")
	}
}

func TestResearchSpend_SubmitByVoter2(t *testing.T) {
	k, ctx := setupKeeper(t)
	_, voter2 := setupResearchVoters(t, k, ctx)

	resp, err := k.SubmitResearchSpend(ctx, &types.MsgSubmitResearchSpend{
		Proposer:    voter2,
		Title:       "Fund protocol audit",
		Description: "Security audit grant",
		Recipient:   testAddr("auditor"),
		Amount:      "500000000",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.ProposalId != 1 {
		t.Errorf("expected proposal_id=1, got %d", resp.ProposalId)
	}

	prop, found := k.GetResearchSpendProposal(ctx, 1)
	if !found {
		t.Fatal("proposal not found")
	}
	if prop.Proposer != voter2 {
		t.Errorf("wrong proposer: got %s, want %s", prop.Proposer, voter2)
	}
}

func TestResearchSpend_SubmitByNonVoter(t *testing.T) {
	k, ctx := setupKeeper(t)
	setupResearchVoters(t, k, ctx)

	_, err := k.SubmitResearchSpend(ctx, &types.MsgSubmitResearchSpend{
		Proposer:    testAddr("outsider"),
		Title:       "Sneaky proposal",
		Description: "Should fail",
		Recipient:   testAddr("me"),
		Amount:      "999999999",
	})
	if err == nil {
		t.Error("expected error for non-voter submit")
	}
	if err != types.ErrNotDesignatedVoter {
		t.Errorf("expected ErrNotDesignatedVoter, got %v", err)
	}
}

func TestResearchSpend_BothYes(t *testing.T) {
	k, ctx := setupKeeper(t)
	voter1, voter2 := setupResearchVoters(t, k, ctx)

	// Wire mock vesting keeper.
	mock := &mockVestingKeeper{}
	k.SetVestingKeeper(mock)

	// Submit proposal.
	k.SubmitResearchSpend(ctx, &types.MsgSubmitResearchSpend{
		Proposer:  voter1,
		Title:     "Both yes test",
		Recipient: testAddr("recipient"),
		Amount:    "100000000",
	})

	// Advance to voting by moving past discussion period.
	prop, _ := k.GetResearchSpendProposal(ctx, 1)
	prop.Stage = string(types.ResearchStageVoting)
	k.SetResearchSpendProposal(ctx, prop)

	// Voter 1 votes yes.
	_, err := k.VoteResearchSpend(ctx, &types.MsgVoteResearchSpend{
		Voter: voter1, ProposalId: 1, Vote: "yes", Reasoning: "Great research",
	})
	if err != nil {
		t.Fatalf("voter1 vote failed: %v", err)
	}

	// Voter 2 votes yes.
	_, err = k.VoteResearchSpend(ctx, &types.MsgVoteResearchSpend{
		Voter: voter2, ProposalId: 1, Vote: "yes", Reasoning: "I concur",
	})
	if err != nil {
		t.Fatalf("voter2 vote failed: %v", err)
	}

	prop, _ = k.GetResearchSpendProposal(ctx, 1)
	if prop.Stage != string(types.ResearchStageExecuted) {
		t.Errorf("expected executed, got %s", prop.Stage)
	}
	if !mock.disburseCalled {
		t.Error("expected DisburseFromResearchFund to be called")
	}
}

func TestResearchSpend_OneNo(t *testing.T) {
	k, ctx := setupKeeper(t)
	voter1, voter2 := setupResearchVoters(t, k, ctx)

	k.SubmitResearchSpend(ctx, &types.MsgSubmitResearchSpend{
		Proposer:  voter1,
		Title:     "One no test",
		Recipient: testAddr("recipient"),
		Amount:    "100000000",
	})

	// Advance to voting.
	prop, _ := k.GetResearchSpendProposal(ctx, 1)
	prop.Stage = string(types.ResearchStageVoting)
	k.SetResearchSpendProposal(ctx, prop)

	// Voter 1 votes yes.
	k.VoteResearchSpend(ctx, &types.MsgVoteResearchSpend{
		Voter: voter1, ProposalId: 1, Vote: "yes",
	})

	// Voter 2 votes no.
	k.VoteResearchSpend(ctx, &types.MsgVoteResearchSpend{
		Voter: voter2, ProposalId: 1, Vote: "no", Reasoning: "Not convinced",
	})

	prop, _ = k.GetResearchSpendProposal(ctx, 1)
	if prop.Stage != string(types.ResearchStageRejected) {
		t.Errorf("expected rejected, got %s", prop.Stage)
	}
}

func TestResearchSpend_Timeout(t *testing.T) {
	k, ctx := setupKeeper(t)
	voter1, _ := setupResearchVoters(t, k, ctx)

	// Override params for short periods.
	params := k.GetParams(ctx)
	params.ResearchDiscussionBlocks = 5
	params.ResearchVotingBlocks = 10
	k.SetParams(ctx, params)

	k.SubmitResearchSpend(ctx, &types.MsgSubmitResearchSpend{
		Proposer:  voter1,
		Title:     "Timeout test",
		Recipient: testAddr("recipient"),
		Amount:    "100000000",
	})

	// Discussion period ends at height 105, voting ends at 115.
	// Advance past discussion → voting via BeginBlocker.
	ctx2 := ctx.WithBlockHeight(106)
	k.BeginBlocker(ctx2)

	prop, _ := k.GetResearchSpendProposal(ctx2, 1)
	if prop.Stage != string(types.ResearchStageVoting) {
		t.Errorf("expected voting after discussion period, got %s", prop.Stage)
	}

	// Advance past voting → expired.
	ctx3 := ctx.WithBlockHeight(116)
	k.BeginBlocker(ctx3)

	prop, _ = k.GetResearchSpendProposal(ctx3, 1)
	if prop.Stage != string(types.ResearchStageExpired) {
		t.Errorf("expected expired after voting period, got %s", prop.Stage)
	}
}

func TestResearchSpend_DoubleVote(t *testing.T) {
	k, ctx := setupKeeper(t)
	voter1, _ := setupResearchVoters(t, k, ctx)

	k.SubmitResearchSpend(ctx, &types.MsgSubmitResearchSpend{
		Proposer:  voter1,
		Title:     "Double vote test",
		Recipient: testAddr("recipient"),
		Amount:    "100000000",
	})

	// Advance to voting.
	prop, _ := k.GetResearchSpendProposal(ctx, 1)
	prop.Stage = string(types.ResearchStageVoting)
	k.SetResearchSpendProposal(ctx, prop)

	// First vote.
	k.VoteResearchSpend(ctx, &types.MsgVoteResearchSpend{
		Voter: voter1, ProposalId: 1, Vote: "yes",
	})

	// Second vote (same voter) should fail.
	_, err := k.VoteResearchSpend(ctx, &types.MsgVoteResearchSpend{
		Voter: voter1, ProposalId: 1, Vote: "no",
	})
	if err == nil {
		t.Error("expected error for double vote")
	}
	if err != types.ErrResearchAlreadyVoted {
		t.Errorf("expected ErrResearchAlreadyVoted, got %v", err)
	}
}

func TestResearchSpend_NonVoterVote(t *testing.T) {
	k, ctx := setupKeeper(t)
	voter1, _ := setupResearchVoters(t, k, ctx)

	k.SubmitResearchSpend(ctx, &types.MsgSubmitResearchSpend{
		Proposer:  voter1,
		Title:     "Non-voter vote test",
		Recipient: testAddr("recipient"),
		Amount:    "100000000",
	})

	// Advance to voting.
	prop, _ := k.GetResearchSpendProposal(ctx, 1)
	prop.Stage = string(types.ResearchStageVoting)
	k.SetResearchSpendProposal(ctx, prop)

	_, err := k.VoteResearchSpend(ctx, &types.MsgVoteResearchSpend{
		Voter: testAddr("outsider"), ProposalId: 1, Vote: "yes",
	})
	if err == nil {
		t.Error("expected error for non-voter vote")
	}
	if err != types.ErrNotDesignatedVoter {
		t.Errorf("expected ErrNotDesignatedVoter, got %v", err)
	}
}

func TestResearchSpend_InsufficientFunds(t *testing.T) {
	k, ctx := setupKeeper(t)
	voter1, voter2 := setupResearchVoters(t, k, ctx)

	mock := &mockVestingKeeper{
		disburseErr: fmt.Errorf("insufficient funds in research_fund"),
	}
	k.SetVestingKeeper(mock)

	k.SubmitResearchSpend(ctx, &types.MsgSubmitResearchSpend{
		Proposer:  voter1,
		Title:     "Insufficient funds test",
		Recipient: testAddr("recipient"),
		Amount:    "999999999999999",
	})

	// Advance to voting.
	prop, _ := k.GetResearchSpendProposal(ctx, 1)
	prop.Stage = string(types.ResearchStageVoting)
	k.SetResearchSpendProposal(ctx, prop)

	// Both vote yes — execution should fail.
	k.VoteResearchSpend(ctx, &types.MsgVoteResearchSpend{
		Voter: voter1, ProposalId: 1, Vote: "yes",
	})
	k.VoteResearchSpend(ctx, &types.MsgVoteResearchSpend{
		Voter: voter2, ProposalId: 1, Vote: "yes",
	})

	prop, _ = k.GetResearchSpendProposal(ctx, 1)
	// Should NOT be in executed stage — execution error should be recorded.
	if prop.Stage == string(types.ResearchStageExecuted) {
		t.Error("should not be executed with insufficient funds")
	}
	if prop.ExecutionErr == "" {
		t.Error("expected execution_err to be set")
	}
}

func TestResearchSpend_DiscussionPeriod(t *testing.T) {
	k, ctx := setupKeeper(t)
	voter1, _ := setupResearchVoters(t, k, ctx)

	k.SubmitResearchSpend(ctx, &types.MsgSubmitResearchSpend{
		Proposer:  voter1,
		Title:     "Discussion period test",
		Recipient: testAddr("recipient"),
		Amount:    "100000000",
	})

	// Proposal is in discussion stage. Try to vote (should fail).
	_, err := k.VoteResearchSpend(ctx, &types.MsgVoteResearchSpend{
		Voter: voter1, ProposalId: 1, Vote: "yes",
	})
	if err == nil {
		t.Error("expected error for voting during discussion period")
	}
	if err != types.ErrDiscussionPeriodActive {
		t.Errorf("expected ErrDiscussionPeriodActive, got %v", err)
	}
}

func TestResearchSpend_AuditTrail(t *testing.T) {
	k, ctx := setupKeeper(t)
	voter1, voter2 := setupResearchVoters(t, k, ctx)
	mock := &mockVestingKeeper{}
	k.SetVestingKeeper(mock)

	k.SubmitResearchSpend(ctx, &types.MsgSubmitResearchSpend{
		Proposer:      voter1,
		Title:         "Audit trail test",
		Description:   "Testing vote storage",
		Recipient:     testAddr("recipient"),
		Amount:        "100000000",
		Justification: "Critical research need",
	})

	// Advance to voting.
	prop, _ := k.GetResearchSpendProposal(ctx, 1)
	prop.Stage = string(types.ResearchStageVoting)
	k.SetResearchSpendProposal(ctx, prop)

	// Vote with reasoning.
	k.VoteResearchSpend(ctx, &types.MsgVoteResearchSpend{
		Voter: voter1, ProposalId: 1, Vote: "yes", Reasoning: "Aligns with roadmap",
	})
	k.VoteResearchSpend(ctx, &types.MsgVoteResearchSpend{
		Voter: voter2, ProposalId: 1, Vote: "yes", Reasoning: "Verified budget",
	})

	prop, _ = k.GetResearchSpendProposal(ctx, 1)

	// Check voter 1 audit.
	if prop.Voter1Vote != "yes" {
		t.Errorf("voter1 vote: got %q, want yes", prop.Voter1Vote)
	}
	if prop.Voter1Reason != "Aligns with roadmap" {
		t.Errorf("voter1 reason: got %q", prop.Voter1Reason)
	}
	if prop.Voter1VotedAt != 100 { // block height from setupKeeper
		t.Errorf("voter1 voted_at: got %d, want 100", prop.Voter1VotedAt)
	}

	// Check voter 2 audit.
	if prop.Voter2Vote != "yes" {
		t.Errorf("voter2 vote: got %q, want yes", prop.Voter2Vote)
	}
	if prop.Voter2Reason != "Verified budget" {
		t.Errorf("voter2 reason: got %q", prop.Voter2Reason)
	}
	if prop.Voter2VotedAt != 100 {
		t.Errorf("voter2 voted_at: got %d, want 100", prop.Voter2VotedAt)
	}

	// Check justification preserved.
	if prop.Justification != "Critical research need" {
		t.Errorf("justification: got %q", prop.Justification)
	}
}

func TestResearchSpend_EmptyVoters(t *testing.T) {
	k, ctx := setupKeeper(t)

	// No voters configured — submit should fail.
	_, err := k.SubmitResearchSpend(ctx, &types.MsgSubmitResearchSpend{
		Proposer:  testAddr("anyone"),
		Title:     "No voters",
		Recipient: testAddr("recipient"),
		Amount:    "100000000",
	})
	if err == nil {
		t.Error("expected error when no voters configured")
	}
	if err != types.ErrResearchVotersNotSet {
		t.Errorf("expected ErrResearchVotersNotSet, got %v", err)
	}
}

func TestResearchVoters_GovernanceOnly(t *testing.T) {
	k, ctx := setupKeeper(t)

	// Non-authority should fail.
	_, err := k.SetResearchVoters(ctx, &types.MsgSetResearchVoters{
		Authority: testAddr("random"),
		Voters: &types.ResearchFundVoters{
			Voter1: testAddr("v1"),
			Voter2: testAddr("v2"),
		},
	})
	if err == nil {
		t.Error("expected error for non-authority")
	}
	if err != types.ErrUnauthorized {
		t.Errorf("expected ErrUnauthorized, got %v", err)
	}

	// Authority should succeed.
	_, err = k.SetResearchVoters(ctx, &types.MsgSetResearchVoters{
		Authority: "authority", // matches setupKeeper authority
		Voters: &types.ResearchFundVoters{
			Voter1: testAddr("v1"),
			Voter2: testAddr("v2"),
		},
	})
	if err != nil {
		t.Fatalf("authority call failed: %v", err)
	}

	voters := k.GetResearchFundVoters(ctx)
	if voters == nil {
		t.Fatal("voters not set")
	}
	if voters.Voter1 != testAddr("v1") {
		t.Errorf("voter1: got %s, want %s", voters.Voter1, testAddr("v1"))
	}
	if voters.Voter2 != testAddr("v2") {
		t.Errorf("voter2: got %s, want %s", voters.Voter2, testAddr("v2"))
	}
}

// ---------- Phase-Aware Multisig Tests ----------

func TestResearchSpend_PhaseFullGovernance_RejectsMultisig(t *testing.T) {
	k, ctx := setupKeeper(t)
	voter1, _ := setupResearchVoters(t, k, ctx)

	// Set phase to full governance.
	k.SetResearchFundPhase(ctx, types.ResearchFundPhase_RESEARCH_FUND_PHASE_FULL_GOVERNANCE)

	// Submit should fail — full governance uses standard LIP path.
	_, err := k.SubmitResearchSpend(ctx, &types.MsgSubmitResearchSpend{
		Proposer:  voter1,
		Title:     "Should fail in full governance",
		Recipient: testAddr("recipient"),
		Amount:    "100000000",
	})
	if err == nil {
		t.Error("expected error in full governance phase")
	}
	if err != types.ErrPhaseFullGovernance {
		t.Errorf("expected ErrPhaseFullGovernance, got %v", err)
	}
}

func TestResearchSpend_PhaseGenesisPair_IncrementsCounter(t *testing.T) {
	k, ctx := setupKeeper(t)
	voter1, voter2 := setupResearchVoters(t, k, ctx)
	mock := &mockVestingKeeper{}
	k.SetVestingKeeper(mock)

	// Default is phase 0 (genesis pair).
	resp, _ := k.SubmitResearchSpend(ctx, &types.MsgSubmitResearchSpend{
		Proposer:  voter1,
		Title:     "Phase 0 counter test",
		Recipient: testAddr("recipient"),
		Amount:    "100000000",
	})

	// Advance to voting.
	prop, _ := k.GetResearchSpendProposal(ctx, resp.ProposalId)
	prop.Stage = string(types.ResearchStageVoting)
	k.SetResearchSpendProposal(ctx, prop)

	// Both vote yes.
	k.VoteResearchSpend(ctx, &types.MsgVoteResearchSpend{
		Voter: voter1, ProposalId: resp.ProposalId, Vote: "yes",
	})
	k.VoteResearchSpend(ctx, &types.MsgVoteResearchSpend{
		Voter: voter2, ProposalId: resp.ProposalId, Vote: "yes",
	})

	prop, _ = k.GetResearchSpendProposal(ctx, resp.ProposalId)
	if prop.Stage != string(types.ResearchStageExecuted) {
		t.Errorf("expected executed, got %s", prop.Stage)
	}

	// Verify proposals executed counter incremented.
	state := k.GetResearchFundGovernanceState(ctx)
	if state.ProposalsExecutedInPhase != 1 {
		t.Errorf("expected ProposalsExecutedInPhase=1, got %d", state.ProposalsExecutedInPhase)
	}
}

func TestResearchSpend_PhaseObserver_2of3(t *testing.T) {
	k, ctx := setupKeeper(t)
	voter1, voter2 := setupResearchVoters(t, k, ctx)
	mock := &mockVestingKeeper{}
	k.SetVestingKeeper(mock)

	// Set phase to Observer (2-of-3) with one community seat.
	community1 := testAddr("community1")
	state := k.GetResearchFundGovernanceState(ctx)
	state.CurrentPhase = types.ResearchFundPhase_RESEARCH_FUND_PHASE_OBSERVER
	state.CommunitySeats = []string{community1}
	k.SetResearchFundGovernanceState(ctx, state)

	// Submit proposal.
	resp, err := k.SubmitResearchSpend(ctx, &types.MsgSubmitResearchSpend{
		Proposer:  voter1,
		Title:     "Phase 1 test",
		Recipient: testAddr("recipient"),
		Amount:    "100000000",
	})
	if err != nil {
		t.Fatalf("submit failed: %v", err)
	}

	// Advance to voting.
	prop, _ := k.GetResearchSpendProposal(ctx, resp.ProposalId)
	prop.Stage = string(types.ResearchStageVoting)
	k.SetResearchSpendProposal(ctx, prop)

	// Voter1 votes yes — 1-of-3, not enough.
	k.VoteResearchSpend(ctx, &types.MsgVoteResearchSpend{
		Voter: voter1, ProposalId: resp.ProposalId, Vote: "yes",
	})
	prop, _ = k.GetResearchSpendProposal(ctx, resp.ProposalId)
	if prop.Stage == string(types.ResearchStageExecuted) {
		t.Error("should not execute with only 1-of-3 approvals")
	}

	// Voter2 votes yes — 2-of-3, should execute.
	k.VoteResearchSpend(ctx, &types.MsgVoteResearchSpend{
		Voter: voter2, ProposalId: resp.ProposalId, Vote: "yes",
	})
	prop, _ = k.GetResearchSpendProposal(ctx, resp.ProposalId)
	if prop.Stage != string(types.ResearchStageExecuted) {
		t.Errorf("expected executed with 2-of-3, got %s", prop.Stage)
	}
}

func TestResearchSpend_PhaseObserver_CommunityVoterCanVote(t *testing.T) {
	k, ctx := setupKeeper(t)
	voter1, _ := setupResearchVoters(t, k, ctx)
	mock := &mockVestingKeeper{}
	k.SetVestingKeeper(mock)

	// Set phase to Observer (2-of-3) with one community seat.
	community1 := testAddr("community1")
	state := k.GetResearchFundGovernanceState(ctx)
	state.CurrentPhase = types.ResearchFundPhase_RESEARCH_FUND_PHASE_OBSERVER
	state.CommunitySeats = []string{community1}
	k.SetResearchFundGovernanceState(ctx, state)

	// Submit proposal.
	resp, _ := k.SubmitResearchSpend(ctx, &types.MsgSubmitResearchSpend{
		Proposer:  voter1,
		Title:     "Community voter test",
		Recipient: testAddr("recipient"),
		Amount:    "100000000",
	})

	// Advance to voting.
	prop, _ := k.GetResearchSpendProposal(ctx, resp.ProposalId)
	prop.Stage = string(types.ResearchStageVoting)
	k.SetResearchSpendProposal(ctx, prop)

	// Community voter votes yes.
	_, err := k.VoteResearchSpend(ctx, &types.MsgVoteResearchSpend{
		Voter: community1, ProposalId: resp.ProposalId, Vote: "yes",
	})
	if err != nil {
		t.Fatalf("community voter should be able to vote: %v", err)
	}

	// Voter1 votes yes — 2-of-3, should execute (community1 + voter1).
	k.VoteResearchSpend(ctx, &types.MsgVoteResearchSpend{
		Voter: voter1, ProposalId: resp.ProposalId, Vote: "yes",
	})
	prop, _ = k.GetResearchSpendProposal(ctx, resp.ProposalId)
	if prop.Stage != string(types.ResearchStageExecuted) {
		t.Errorf("expected executed with voter1+community1 (2-of-3), got %s", prop.Stage)
	}
}

func TestResearchSpend_NonDesignatedVoter_PhaseBased(t *testing.T) {
	k, ctx := setupKeeper(t)
	voter1, _ := setupResearchVoters(t, k, ctx)

	// Set phase to Observer with specific community seats.
	state := k.GetResearchFundGovernanceState(ctx)
	state.CurrentPhase = types.ResearchFundPhase_RESEARCH_FUND_PHASE_OBSERVER
	state.CommunitySeats = []string{testAddr("community1")}
	k.SetResearchFundGovernanceState(ctx, state)

	// Submit proposal.
	resp, _ := k.SubmitResearchSpend(ctx, &types.MsgSubmitResearchSpend{
		Proposer:  voter1,
		Title:     "Non-voter test",
		Recipient: testAddr("recipient"),
		Amount:    "100000000",
	})

	// Advance to voting.
	prop, _ := k.GetResearchSpendProposal(ctx, resp.ProposalId)
	prop.Stage = string(types.ResearchStageVoting)
	k.SetResearchSpendProposal(ctx, prop)

	// Random outsider tries to vote — should fail.
	_, err := k.VoteResearchSpend(ctx, &types.MsgVoteResearchSpend{
		Voter: testAddr("outsider"), ProposalId: resp.ProposalId, Vote: "yes",
	})
	if err == nil {
		t.Error("expected error for non-designated voter")
	}
}

func TestResearchSpend_PhaseBalanced_3of5(t *testing.T) {
	k, ctx := setupKeeper(t)
	voter1, _ := setupResearchVoters(t, k, ctx)
	mock := &mockVestingKeeper{}
	k.SetVestingKeeper(mock)

	// Set phase to Balanced (3-of-5) with three community seats.
	community1 := testAddr("community1")
	community2 := testAddr("community2")
	community3 := testAddr("community3")
	state := k.GetResearchFundGovernanceState(ctx)
	state.CurrentPhase = types.ResearchFundPhase_RESEARCH_FUND_PHASE_BALANCED
	state.CommunitySeats = []string{community1, community2, community3}
	k.SetResearchFundGovernanceState(ctx, state)

	// Submit proposal.
	resp, err := k.SubmitResearchSpend(ctx, &types.MsgSubmitResearchSpend{
		Proposer:  voter1,
		Title:     "Phase 2 balanced test",
		Recipient: testAddr("recipient"),
		Amount:    "100000000",
	})
	if err != nil {
		t.Fatalf("submit failed: %v", err)
	}

	// Advance to voting.
	prop, _ := k.GetResearchSpendProposal(ctx, resp.ProposalId)
	prop.Stage = string(types.ResearchStageVoting)
	k.SetResearchSpendProposal(ctx, prop)

	// Voter1 votes yes — 1-of-5, not enough.
	k.VoteResearchSpend(ctx, &types.MsgVoteResearchSpend{
		Voter: voter1, ProposalId: resp.ProposalId, Vote: "yes",
	})
	prop, _ = k.GetResearchSpendProposal(ctx, resp.ProposalId)
	if prop.Stage == string(types.ResearchStageExecuted) {
		t.Error("should not execute with only 1-of-5")
	}

	// Community1 votes yes — 2-of-5, still not enough.
	k.VoteResearchSpend(ctx, &types.MsgVoteResearchSpend{
		Voter: community1, ProposalId: resp.ProposalId, Vote: "yes",
	})
	prop, _ = k.GetResearchSpendProposal(ctx, resp.ProposalId)
	if prop.Stage == string(types.ResearchStageExecuted) {
		t.Error("should not execute with only 2-of-5")
	}

	// Community2 votes yes — 3-of-5, should execute.
	k.VoteResearchSpend(ctx, &types.MsgVoteResearchSpend{
		Voter: community2, ProposalId: resp.ProposalId, Vote: "yes",
	})
	prop, _ = k.GetResearchSpendProposal(ctx, resp.ProposalId)
	if prop.Stage != string(types.ResearchStageExecuted) {
		t.Errorf("expected executed with 3-of-5 (voter1+community1+community2), got %s", prop.Stage)
	}
}

func TestCountCommunitySeatVotes_ScopedToPhase(t *testing.T) {
	k, ctx := setupKeeper(t)
	voter1, _ := setupResearchVoters(t, k, ctx)

	// Set phase to Observer with one community seat, phase started at block 100.
	community1 := testAddr("community1")
	state := k.GetResearchFundGovernanceState(ctx)
	state.CurrentPhase = types.ResearchFundPhase_RESEARCH_FUND_PHASE_OBSERVER
	state.CommunitySeats = []string{community1}
	state.PhaseStartedAtBlock = 100
	k.SetResearchFundGovernanceState(ctx, state)

	// Submit a proposal at block 100 (in current phase).
	resp, _ := k.SubmitResearchSpend(ctx, &types.MsgSubmitResearchSpend{
		Proposer:  voter1,
		Title:     "Current phase proposal",
		Recipient: testAddr("recipient"),
		Amount:    "100000000",
	})

	// Record a community vote on it.
	k.SetResearchCommunityVote(ctx, resp.ProposalId, community1, "yes")

	// Count should include this vote.
	state = k.GetResearchFundGovernanceState(ctx)
	count := k.CountCommunitySeatVotes(ctx, state)
	if count != 1 {
		t.Errorf("expected 1 community seat vote in current phase, got %d", count)
	}

	// Now simulate a proposal from a previous phase (CreatedAt < PhaseStartedAtBlock).
	oldProp := k.GetAllResearchSpendProposals(ctx)
	if len(oldProp) != 1 {
		t.Fatalf("expected 1 proposal, got %d", len(oldProp))
	}
	// Manually create an older proposal with CreatedAt=50 (before phase started).
	k.SetNextResearchSpendID(ctx, 10)
	ctx2 := ctx.WithBlockHeight(50)
	resp2, _ := k.SubmitResearchSpend(ctx2, &types.MsgSubmitResearchSpend{
		Proposer:  voter1,
		Title:     "Old phase proposal",
		Recipient: testAddr("recipient2"),
		Amount:    "200000000",
	})
	k.SetResearchCommunityVote(ctx, resp2.ProposalId, community1, "yes")

	// Count should still be 1 — the old proposal's vote shouldn't be counted.
	count = k.CountCommunitySeatVotes(ctx, state)
	if count != 1 {
		t.Errorf("expected 1 community seat vote (old proposal excluded), got %d", count)
	}
}

func TestResearchSpend_CommunityVoterDoubleVote(t *testing.T) {
	k, ctx := setupKeeper(t)
	voter1, _ := setupResearchVoters(t, k, ctx)

	// Set phase to Observer with one community seat.
	community1 := testAddr("community1")
	state := k.GetResearchFundGovernanceState(ctx)
	state.CurrentPhase = types.ResearchFundPhase_RESEARCH_FUND_PHASE_OBSERVER
	state.CommunitySeats = []string{community1}
	k.SetResearchFundGovernanceState(ctx, state)

	// Submit proposal.
	resp, _ := k.SubmitResearchSpend(ctx, &types.MsgSubmitResearchSpend{
		Proposer:  voter1,
		Title:     "Community double vote test",
		Recipient: testAddr("recipient"),
		Amount:    "100000000",
	})

	// Advance to voting.
	prop, _ := k.GetResearchSpendProposal(ctx, resp.ProposalId)
	prop.Stage = string(types.ResearchStageVoting)
	k.SetResearchSpendProposal(ctx, prop)

	// Community voter votes once — should succeed.
	_, err := k.VoteResearchSpend(ctx, &types.MsgVoteResearchSpend{
		Voter: community1, ProposalId: resp.ProposalId, Vote: "yes",
	})
	if err != nil {
		t.Fatalf("first vote failed: %v", err)
	}

	// Community voter votes again — should fail.
	_, err = k.VoteResearchSpend(ctx, &types.MsgVoteResearchSpend{
		Voter: community1, ProposalId: resp.ProposalId, Vote: "no",
	})
	if err == nil {
		t.Error("expected error for community voter double vote")
	}
	if err != types.ErrResearchAlreadyVoted {
		t.Errorf("expected ErrResearchAlreadyVoted, got %v", err)
	}
}

// ========== Ported Tests: Research Spend Edge Cases ==========

// TestResearchSpend_GenesisRoundtrip verifies research spend proposals survive
// genesis export/import via the research_fund_voters param path.
func TestResearchSpend_GenesisRoundtrip(t *testing.T) {
	k, ctx := setupKeeper(t)
	voter1, voter2 := setupResearchVoters(t, k, ctx)

	// Submit a proposal.
	k.SubmitResearchSpend(ctx, &types.MsgSubmitResearchSpend{
		Proposer:  voter1,
		Title:     "Genesis Roundtrip",
		Recipient: testAddr("recipient"),
		Amount:    "100000000",
	})

	// Export genesis.
	gs := k.ExportGenesis(ctx)

	// Verify research voters are exported in params.
	if gs.Params.ResearchFundVoters == nil {
		t.Fatal("expected research_fund_voters in exported params")
	}
	if gs.Params.ResearchFundVoters.Voter1 != voter1 {
		t.Errorf("voter1 mismatch: got %s, want %s", gs.Params.ResearchFundVoters.Voter1, voter1)
	}
	if gs.Params.ResearchFundVoters.Voter2 != voter2 {
		t.Errorf("voter2 mismatch: got %s, want %s", gs.Params.ResearchFundVoters.Voter2, voter2)
	}

	// Import into fresh keeper.
	k2, ctx2 := setupKeeper(t)
	k2.InitGenesis(ctx2, gs)

	voters := k2.GetResearchFundVoters(ctx2)
	if voters == nil {
		t.Fatal("research voters lost after genesis roundtrip")
	}
	if voters.Voter1 != voter1 || voters.Voter2 != voter2 {
		t.Error("voter addresses do not match after roundtrip")
	}
}

// TestResearchSpend_MultipleProposals verifies multiple research spend
// proposals can exist simultaneously with incrementing IDs.
func TestResearchSpend_MultipleProposals(t *testing.T) {
	k, ctx := setupKeeper(t)
	voter1, _ := setupResearchVoters(t, k, ctx)

	for i := 0; i < 3; i++ {
		resp, err := k.SubmitResearchSpend(ctx, &types.MsgSubmitResearchSpend{
			Proposer:  voter1,
			Title:     fmt.Sprintf("Proposal %d", i+1),
			Recipient: testAddr("recipient"),
			Amount:    "100000000",
		})
		if err != nil {
			t.Fatalf("submit #%d failed: %v", i+1, err)
		}
		if resp.ProposalId != uint64(i+1) {
			t.Errorf("expected id=%d, got %d", i+1, resp.ProposalId)
		}
	}

	all := k.GetAllResearchSpendProposals(ctx)
	if len(all) != 3 {
		t.Errorf("expected 3 proposals, got %d", len(all))
	}
}

// TestResearchSpend_VoteOnTerminal verifies that voting on an already-rejected
// or executed proposal fails.
func TestResearchSpend_VoteOnTerminal(t *testing.T) {
	k, ctx := setupKeeper(t)
	voter1, voter2 := setupResearchVoters(t, k, ctx)

	k.SubmitResearchSpend(ctx, &types.MsgSubmitResearchSpend{
		Proposer:  voter1,
		Title:     "Terminal vote test",
		Recipient: testAddr("recipient"),
		Amount:    "100000000",
	})

	// Advance to voting and reject.
	prop, _ := k.GetResearchSpendProposal(ctx, 1)
	prop.Stage = string(types.ResearchStageVoting)
	k.SetResearchSpendProposal(ctx, prop)

	k.VoteResearchSpend(ctx, &types.MsgVoteResearchSpend{
		Voter: voter1, ProposalId: 1, Vote: "no",
	})

	prop, _ = k.GetResearchSpendProposal(ctx, 1)
	if prop.Stage != string(types.ResearchStageRejected) {
		t.Fatalf("expected rejected, got %s", prop.Stage)
	}

	// Try to vote on rejected proposal.
	_, err := k.VoteResearchSpend(ctx, &types.MsgVoteResearchSpend{
		Voter: voter2, ProposalId: 1, Vote: "yes",
	})
	if err == nil {
		t.Error("expected error for voting on rejected proposal")
	}
}

// TestResearchSpend_QueryByStage verifies the ResearchSpends query filters
// by stage correctly.
func TestResearchSpend_QueryByStage(t *testing.T) {
	k, ctx := setupKeeper(t)
	voter1, _ := setupResearchVoters(t, k, ctx)
	qs := keeper.NewQueryServerImpl(k)

	// Create two proposals.
	k.SubmitResearchSpend(ctx, &types.MsgSubmitResearchSpend{
		Proposer:  voter1,
		Title:     "Discussion Prop",
		Recipient: testAddr("recipient"),
		Amount:    "100000000",
	})
	k.SubmitResearchSpend(ctx, &types.MsgSubmitResearchSpend{
		Proposer:  voter1,
		Title:     "Voting Prop",
		Recipient: testAddr("recipient"),
		Amount:    "200000000",
	})

	// Move second to voting.
	prop2, _ := k.GetResearchSpendProposal(ctx, 2)
	prop2.Stage = string(types.ResearchStageVoting)
	k.SetResearchSpendProposal(ctx, prop2)

	// Query discussion stage.
	resp, err := qs.ResearchSpends(ctx, &types.QueryResearchSpendsRequest{
		Stage: string(types.ResearchStageDiscussion),
	})
	if err != nil {
		t.Fatalf("query failed: %v", err)
	}
	if resp.Total != 1 {
		t.Errorf("expected 1 discussion, got %d", resp.Total)
	}

	// Query voting stage.
	resp, err = qs.ResearchSpends(ctx, &types.QueryResearchSpendsRequest{
		Stage: string(types.ResearchStageVoting),
	})
	if err != nil {
		t.Fatalf("query failed: %v", err)
	}
	if resp.Total != 1 {
		t.Errorf("expected 1 voting, got %d", resp.Total)
	}

	// Query all.
	resp, err = qs.ResearchSpends(ctx, &types.QueryResearchSpendsRequest{})
	if err != nil {
		t.Fatalf("query failed: %v", err)
	}
	if resp.Total != 2 {
		t.Errorf("expected 2 total, got %d", resp.Total)
	}
}

// ========== Multisig Threshold Tests ==========

// TestThreshold_Phase2_VacantSeat_AdjustsTotal verifies that vacant community
// seats in Phase 2 (Balanced, 3-of-5) reduce the pool of possible approvers.
// With 2 vacant seats, only 3 voters remain (2 core + 1 community), so all
// three must approve to reach the required threshold of 3.
func TestThreshold_Phase2_VacantSeat_AdjustsTotal(t *testing.T) {
	k, ctx := setupKeeper(t)
	voter1, voter2 := setupResearchVoters(t, k, ctx)
	mock := &mockVestingKeeper{}
	k.SetVestingKeeper(mock)

	// Phase 2 with 2 vacant seats (empty strings) and 1 filled community seat.
	communityAddr := testAddr("community1")
	state := k.GetResearchFundGovernanceState(ctx)
	state.CurrentPhase = types.ResearchFundPhase_RESEARCH_FUND_PHASE_BALANCED
	state.CommunitySeats = []string{"", communityAddr, ""}
	k.SetResearchFundGovernanceState(ctx, state)

	// Submit proposal.
	resp, err := k.SubmitResearchSpend(ctx, &types.MsgSubmitResearchSpend{
		Proposer:  voter1,
		Title:     "Vacant seat test",
		Recipient: testAddr("recipient"),
		Amount:    "100000000",
	})
	if err != nil {
		t.Fatalf("submit failed: %v", err)
	}

	// Advance to voting.
	prop, _ := k.GetResearchSpendProposal(ctx, resp.ProposalId)
	prop.Stage = string(types.ResearchStageVoting)
	k.SetResearchSpendProposal(ctx, prop)

	// Voter1 votes yes (1 approval).
	k.VoteResearchSpend(ctx, &types.MsgVoteResearchSpend{
		Voter: voter1, ProposalId: resp.ProposalId, Vote: "yes",
	})
	prop, _ = k.GetResearchSpendProposal(ctx, resp.ProposalId)
	if prop.Stage == string(types.ResearchStageExecuted) {
		t.Error("should not execute with only 1 approval")
	}

	// Voter2 votes yes (2 approvals).
	k.VoteResearchSpend(ctx, &types.MsgVoteResearchSpend{
		Voter: voter2, ProposalId: resp.ProposalId, Vote: "yes",
	})
	prop, _ = k.GetResearchSpendProposal(ctx, resp.ProposalId)
	if prop.Stage == string(types.ResearchStageExecuted) {
		t.Error("should not execute with only 2-of-3 (need 3 required in Phase 2)")
	}

	// Community seat votes yes (3 approvals — reaches required threshold of 3).
	k.VoteResearchSpend(ctx, &types.MsgVoteResearchSpend{
		Voter: communityAddr, ProposalId: resp.ProposalId, Vote: "yes",
	})
	prop, _ = k.GetResearchSpendProposal(ctx, resp.ProposalId)
	if prop.Stage != string(types.ResearchStageExecuted) {
		t.Errorf("expected executed with 3-of-3 (all remaining voters), got %s", prop.Stage)
	}
}

// TestThreshold_Phase2_AllCommunityApprove verifies that in Phase 2 with all
// 3 community seats filled, reaching 3-of-5 with 1 core + 2 community
// approvals triggers execution.
func TestThreshold_Phase2_AllCommunityApprove(t *testing.T) {
	k, ctx := setupKeeper(t)
	voter1, _ := setupResearchVoters(t, k, ctx)
	mock := &mockVestingKeeper{}
	k.SetVestingKeeper(mock)

	// Phase 2 with all 3 community seats filled.
	community1 := testAddr("community1")
	community2 := testAddr("community2")
	community3 := testAddr("community3")
	state := k.GetResearchFundGovernanceState(ctx)
	state.CurrentPhase = types.ResearchFundPhase_RESEARCH_FUND_PHASE_BALANCED
	state.CommunitySeats = []string{community1, community2, community3}
	k.SetResearchFundGovernanceState(ctx, state)

	// Submit proposal.
	resp, err := k.SubmitResearchSpend(ctx, &types.MsgSubmitResearchSpend{
		Proposer:  voter1,
		Title:     "All community approve test",
		Recipient: testAddr("recipient"),
		Amount:    "100000000",
	})
	if err != nil {
		t.Fatalf("submit failed: %v", err)
	}

	// Advance to voting.
	prop, _ := k.GetResearchSpendProposal(ctx, resp.ProposalId)
	prop.Stage = string(types.ResearchStageVoting)
	k.SetResearchSpendProposal(ctx, prop)

	// Voter1 votes yes (1-of-5).
	k.VoteResearchSpend(ctx, &types.MsgVoteResearchSpend{
		Voter: voter1, ProposalId: resp.ProposalId, Vote: "yes",
	})

	// Community1 votes yes (2-of-5).
	k.VoteResearchSpend(ctx, &types.MsgVoteResearchSpend{
		Voter: community1, ProposalId: resp.ProposalId, Vote: "yes",
	})
	prop, _ = k.GetResearchSpendProposal(ctx, resp.ProposalId)
	if prop.Stage == string(types.ResearchStageExecuted) {
		t.Error("should not execute with only 2-of-5")
	}

	// Community2 votes yes (3-of-5 — meets threshold).
	k.VoteResearchSpend(ctx, &types.MsgVoteResearchSpend{
		Voter: community2, ProposalId: resp.ProposalId, Vote: "yes",
	})
	prop, _ = k.GetResearchSpendProposal(ctx, resp.ProposalId)
	if prop.Stage != string(types.ResearchStageExecuted) {
		t.Errorf("expected executed with 3-of-5 (voter1+community1+community2), got %s", prop.Stage)
	}
}

// TestThreshold_Phase2_RejectIfOnlyTwo verifies that in Phase 2 with 3
// community seats, only 2 approvals (below the 3-of-5 threshold) does
// not trigger execution.
func TestThreshold_Phase2_RejectIfOnlyTwo(t *testing.T) {
	k, ctx := setupKeeper(t)
	voter1, voter2 := setupResearchVoters(t, k, ctx)

	// Phase 2 with all 3 community seats filled.
	community1 := testAddr("community1")
	community2 := testAddr("community2")
	community3 := testAddr("community3")
	state := k.GetResearchFundGovernanceState(ctx)
	state.CurrentPhase = types.ResearchFundPhase_RESEARCH_FUND_PHASE_BALANCED
	state.CommunitySeats = []string{community1, community2, community3}
	k.SetResearchFundGovernanceState(ctx, state)

	// Submit proposal.
	resp, err := k.SubmitResearchSpend(ctx, &types.MsgSubmitResearchSpend{
		Proposer:  voter1,
		Title:     "Two approvals test",
		Recipient: testAddr("recipient"),
		Amount:    "100000000",
	})
	if err != nil {
		t.Fatalf("submit failed: %v", err)
	}

	// Advance to voting.
	prop, _ := k.GetResearchSpendProposal(ctx, resp.ProposalId)
	prop.Stage = string(types.ResearchStageVoting)
	k.SetResearchSpendProposal(ctx, prop)

	// Voter1 votes yes (1-of-5).
	k.VoteResearchSpend(ctx, &types.MsgVoteResearchSpend{
		Voter: voter1, ProposalId: resp.ProposalId, Vote: "yes",
	})

	// Community1 votes yes (2-of-5).
	k.VoteResearchSpend(ctx, &types.MsgVoteResearchSpend{
		Voter: community1, ProposalId: resp.ProposalId, Vote: "yes",
	})

	prop, _ = k.GetResearchSpendProposal(ctx, resp.ProposalId)
	if prop.Stage == string(types.ResearchStageExecuted) {
		t.Error("should NOT execute with only 2-of-5 approvals in Phase 2")
	}
	if prop.Stage != string(types.ResearchStageVoting) {
		t.Errorf("expected still in voting, got %s", prop.Stage)
	}

	// Voter2 votes NO — triggers immediate rejection (any core NO rejects).
	k.VoteResearchSpend(ctx, &types.MsgVoteResearchSpend{
		Voter: voter2, ProposalId: resp.ProposalId, Vote: "no",
	})
	prop, _ = k.GetResearchSpendProposal(ctx, resp.ProposalId)
	if prop.Stage != string(types.ResearchStageRejected) {
		t.Errorf("expected rejected after core voter NO, got %s", prop.Stage)
	}
}
