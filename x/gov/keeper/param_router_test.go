package keeper_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/zerone-chain/zerone/x/gov/keeper"
	"github.com/zerone-chain/zerone/x/gov/types"
)

// ---------- Mock ParamRouter ----------

type mockParamRouter struct {
	applied []paramChangeRecord
}

type paramChangeRecord struct {
	module string
	key    string
	value  string
}

func (m *mockParamRouter) ApplyParamChange(_ context.Context, module, key, value string) error {
	if module == "unknown_module" {
		return fmt.Errorf("no param handler registered for module %q", module)
	}
	m.applied = append(m.applied, paramChangeRecord{module: module, key: key, value: value})
	return nil
}

// ---------- ParamRouter Integration Tests ----------

func TestParamRouter_FullLifecycle(t *testing.T) {
	// Setup: keeper with mock staking + mock param router
	k, ctx, mock := setupWithStaking(t, "1000000")
	ms := keeper.NewMsgServerImpl(k)

	mockPR := &mockParamRouter{}
	k.SetParamRouter(mockPR)

	// 1. Submit parameter-category LIP with param_changes
	resp, err := ms.SubmitLIP(ctx, &types.MsgSubmitLIP{
		Proposer:     testAddr("alice"),
		Title:        "Update Voting Period",
		Description:  "Change voting period blocks",
		Category:     types.CategoryParameter,
		InitialStake: "1000000",
		ParamChanges: []*types.ParamChange{
			{Module: "zerone_gov", Key: "voting_period_blocks", Value: "200000"},
			{Module: "zerone_staking", Key: "max_validators", Value: "150"},
		},
	})
	if err != nil {
		t.Fatalf("submit failed: %v", err)
	}
	lipID := resp.LipId

	// 2. Pass it through governance — put in voting and vote yes
	lip, _ := k.GetLIP(ctx, lipID)
	lip.Stage = types.StatusVoting
	lip.VotingEndBlock = 100
	k.SetLIP(ctx, lip)

	mock.delegations[testAddr("voter1")] = "500000" // 50% of total
	ms.CastVote(ctx, &types.MsgCastVote{
		Voter: testAddr("voter1"), LipId: lipID, Option: types.VoteYes,
	})

	// 3. Run BeginBlocker
	k.BeginBlocker(ctx)

	// 4. Assert ApplyParamChange was called for each change
	lip, _ = k.GetLIP(ctx, lipID)
	if lip.Stage != types.StatusPassed {
		t.Fatalf("expected passed, got %s", lip.Stage)
	}

	if len(mockPR.applied) != 2 {
		t.Fatalf("expected 2 param changes applied, got %d", len(mockPR.applied))
	}

	if mockPR.applied[0].module != "zerone_gov" || mockPR.applied[0].key != "voting_period_blocks" || mockPR.applied[0].value != "200000" {
		t.Errorf("first param change mismatch: %+v", mockPR.applied[0])
	}
	if mockPR.applied[1].module != "zerone_staking" || mockPR.applied[1].key != "max_validators" || mockPR.applied[1].value != "150" {
		t.Errorf("second param change mismatch: %+v", mockPR.applied[1])
	}

	// 5. Assert events emitted
	events := ctx.EventManager().Events()
	appliedCount := 0
	for _, e := range events {
		if e.Type == "zerone.gov.param_change_applied" {
			appliedCount++
		}
	}
	if appliedCount != 2 {
		t.Errorf("expected 2 param_change_applied events, got %d", appliedCount)
	}
}

func TestParamRouter_UnknownModule_EmitsError(t *testing.T) {
	// Setup: keeper with mock staking + mock param router
	k, ctx, mock := setupWithStaking(t, "1000000")
	ms := keeper.NewMsgServerImpl(k)

	mockPR := &mockParamRouter{}
	k.SetParamRouter(mockPR)

	// LIP with param_change targeting unknown module
	resp, err := ms.SubmitLIP(ctx, &types.MsgSubmitLIP{
		Proposer:     testAddr("alice"),
		Title:        "Bad Param Change",
		Description:  "Targets unknown module",
		Category:     types.CategoryParameter,
		InitialStake: "1000000",
		ParamChanges: []*types.ParamChange{
			{Module: "unknown_module", Key: "some_key", Value: "42"},
		},
	})
	if err != nil {
		t.Fatalf("submit failed: %v", err)
	}
	lipID := resp.LipId

	// Pass through governance
	lip, _ := k.GetLIP(ctx, lipID)
	lip.Stage = types.StatusVoting
	lip.VotingEndBlock = 100
	k.SetLIP(ctx, lip)

	mock.delegations[testAddr("voter1")] = "500000"
	ms.CastVote(ctx, &types.MsgCastVote{
		Voter: testAddr("voter1"), LipId: lipID, Option: types.VoteYes,
	})

	// Run BeginBlocker — should not panic
	k.BeginBlocker(ctx)

	// LIP still passes (param change failure doesn't block LIP resolution)
	lip, _ = k.GetLIP(ctx, lipID)
	if lip.Stage != types.StatusPassed {
		t.Errorf("expected passed, got %s", lip.Stage)
	}

	// No changes should have been applied successfully
	if len(mockPR.applied) != 0 {
		t.Errorf("expected 0 applied changes, got %d", len(mockPR.applied))
	}

	// Should emit "param_change_failed" event, not panic
	events := ctx.EventManager().Events()
	failedCount := 0
	for _, e := range events {
		if e.Type == "zerone.gov.param_change_failed" {
			failedCount++
			for _, attr := range e.Attributes {
				if attr.Key == "module" && attr.Value != "unknown_module" {
					t.Errorf("expected module=unknown_module in failed event, got %s", attr.Value)
				}
			}
		}
	}
	if failedCount != 1 {
		t.Errorf("expected 1 param_change_failed event, got %d", failedCount)
	}
}

func TestParamRouter_NoRouter_EmitsError(t *testing.T) {
	// Setup: keeper WITHOUT param router set
	k, ctx, mock := setupWithStaking(t, "1000000")
	ms := keeper.NewMsgServerImpl(k)

	// Do NOT set param router — it stays nil

	resp, err := ms.SubmitLIP(ctx, &types.MsgSubmitLIP{
		Proposer:     testAddr("alice"),
		Title:        "No Router Param Change",
		Description:  "Router not wired",
		Category:     types.CategoryParameter,
		InitialStake: "1000000",
		ParamChanges: []*types.ParamChange{
			{Module: "zerone_gov", Key: "voting_period_blocks", Value: "200000"},
		},
	})
	if err != nil {
		t.Fatalf("submit failed: %v", err)
	}
	lipID := resp.LipId

	lip, _ := k.GetLIP(ctx, lipID)
	lip.Stage = types.StatusVoting
	lip.VotingEndBlock = 100
	k.SetLIP(ctx, lip)

	mock.delegations[testAddr("voter1")] = "500000"
	ms.CastVote(ctx, &types.MsgCastVote{
		Voter: testAddr("voter1"), LipId: lipID, Option: types.VoteYes,
	})

	// Should not panic even without router
	k.BeginBlocker(ctx)

	lip, _ = k.GetLIP(ctx, lipID)
	if lip.Stage != types.StatusPassed {
		t.Errorf("expected passed, got %s", lip.Stage)
	}

	// Should emit failure event
	events := ctx.EventManager().Events()
	failedCount := 0
	for _, e := range events {
		if e.Type == "zerone.gov.param_change_failed" {
			failedCount++
		}
	}
	if failedCount != 1 {
		t.Errorf("expected 1 param_change_failed event, got %d", failedCount)
	}
}
