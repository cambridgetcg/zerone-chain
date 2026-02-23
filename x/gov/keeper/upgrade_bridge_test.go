package keeper_test

import (
	"context"
	"testing"

	"github.com/zerone-chain/zerone/x/gov/keeper"
	"github.com/zerone-chain/zerone/x/gov/types"
)

// ---------- Mock Upgrade Keeper (bridge-specific) ----------

type bridgeMockUpgradeKeeper struct {
	called bool
	plan   *types.UpgradePlan
}

func (m *bridgeMockUpgradeKeeper) ScheduleUpgrade(_ context.Context, plan *types.UpgradePlan) error {
	m.called = true
	m.plan = plan
	return nil
}

// ---------- Upgrade Bridge Integration Tests ----------

func TestUpgradeBridge_FullLifecycle(t *testing.T) {
	// Setup: keeper with mock staking + mock upgrade keeper
	k, ctx, mock := setupWithStaking(t, "1000000")
	ms := keeper.NewMsgServerImpl(k)

	mockUK := &bridgeMockUpgradeKeeper{}
	k.SetUpgradeKeeper(mockUK)

	// 1. Submit upgrade-category LIP
	resp, err := ms.SubmitLIP(ctx, &types.MsgSubmitLIP{
		Proposer:     testAddr("alice"),
		Title:        "v2.0.0 Upgrade",
		Description:  "Major protocol upgrade",
		Category:     types.CategoryUpgrade,
		InitialStake: "1000000",
	})
	if err != nil {
		t.Fatalf("submit failed: %v", err)
	}
	lipID := resp.LipId

	// 2. Attach upgrade plan (name: "v2.0.0", height: 1000)
	_, err = ms.AttachUpgradePlan(ctx, &types.MsgAttachUpgradePlan{
		Proposer:    testAddr("alice"),
		LipId:       lipID,
		UpgradeName: "v2.0.0",
		Height:      1000,
		Info:        "https://github.com/zerone-chain/zerone/releases/v2.0.0",
	})
	if err != nil {
		t.Fatalf("attach upgrade plan failed: %v", err)
	}

	// 3. Stake to meet quorum — put LIP directly into voting
	lip, _ := k.GetLIP(ctx, lipID)
	lip.Stage = types.StatusVoting
	lip.VotingEndBlock = 100 // expires at current block height
	k.SetLIP(ctx, lip)

	// 4. Vote yes to pass support threshold
	mock.delegations[testAddr("voter1")] = "500000" // 50% of total
	_, err = ms.CastVote(ctx, &types.MsgCastVote{
		Voter: testAddr("voter1"), LipId: lipID, Option: types.VoteYes,
	})
	if err != nil {
		t.Fatalf("cast vote failed: %v", err)
	}

	// 5. Advance block height past voting deadline — current height IS the deadline
	// 6. Run BeginBlocker
	k.BeginBlocker(ctx)

	// 7. Assert LIP status is "passed"
	lip, _ = k.GetLIP(ctx, lipID)
	if lip.Stage != types.StatusPassed {
		t.Errorf("expected passed, got %s", lip.Stage)
	}

	// 8. Assert mock upgrade keeper received ScheduleUpgrade call
	//    with name="v2.0.0", height=1000
	if !mockUK.called {
		t.Fatal("ScheduleUpgrade was not called on the mock upgrade keeper")
	}
	if mockUK.plan.Name != "v2.0.0" {
		t.Errorf("scheduled plan name: got %q, want %q", mockUK.plan.Name, "v2.0.0")
	}
	if mockUK.plan.Height != 1000 {
		t.Errorf("scheduled plan height: got %d, want 1000", mockUK.plan.Height)
	}
	if mockUK.plan.Info != "https://github.com/zerone-chain/zerone/releases/v2.0.0" {
		t.Errorf("scheduled plan info mismatch")
	}

	// 9. Assert "zerone.gov.upgrade_scheduled" event emitted
	events := ctx.EventManager().Events()
	found := false
	for _, e := range events {
		if e.Type == "zerone.gov.upgrade_scheduled" {
			found = true
			for _, attr := range e.Attributes {
				switch attr.Key {
				case "lip_id":
					if attr.Value != lipID {
						t.Errorf("event lip_id: got %q, want %q", attr.Value, lipID)
					}
				case "upgrade_name":
					if attr.Value != "v2.0.0" {
						t.Errorf("event upgrade_name: got %q, want %q", attr.Value, "v2.0.0")
					}
				case "height":
					if attr.Value != "1000" {
						t.Errorf("event height: got %q, want %q", attr.Value, "1000")
					}
				}
			}
		}
	}
	if !found {
		t.Error("expected zerone.gov.upgrade_scheduled event to be emitted")
	}
}

func TestUpgradeBridge_NonUpgradeLIP_NoSchedule(t *testing.T) {
	k, ctx := setupKeeper(t)
	ms := keeper.NewMsgServerImpl(k)

	// Submit a parameter-category LIP
	ms.SubmitLIP(ctx, &types.MsgSubmitLIP{
		Proposer:     testAddr("alice"),
		Title:        "Param Change",
		Description:  "Change voting period",
		Category:     types.CategoryParameter,
		InitialStake: "1000000",
	})

	// Attach upgrade plan → should fail (wrong category)
	_, err := ms.AttachUpgradePlan(ctx, &types.MsgAttachUpgradePlan{
		Proposer:    testAddr("alice"),
		LipId:       "LIP-1",
		UpgradeName: "v2.0.0",
		Height:      500,
	})
	if err == nil {
		t.Error("expected error when attaching upgrade plan to non-upgrade LIP")
	}
}

func TestUpgradeBridge_FailedLIP_NoSchedule(t *testing.T) {
	// Setup: keeper with mock staking + mock upgrade keeper
	k, ctx, mock := setupWithStaking(t, "1000000")
	ms := keeper.NewMsgServerImpl(k)

	mockUK := &bridgeMockUpgradeKeeper{}
	k.SetUpgradeKeeper(mockUK)

	// Submit upgrade LIP with plan
	resp, err := ms.SubmitLIP(ctx, &types.MsgSubmitLIP{
		Proposer:     testAddr("alice"),
		Title:        "Doomed Upgrade",
		Description:  "Will be rejected",
		Category:     types.CategoryUpgrade,
		InitialStake: "1000000",
	})
	if err != nil {
		t.Fatalf("submit failed: %v", err)
	}
	lipID := resp.LipId

	// Attach upgrade plan
	_, err = ms.AttachUpgradePlan(ctx, &types.MsgAttachUpgradePlan{
		Proposer:    testAddr("alice"),
		LipId:       lipID,
		UpgradeName: "v2.0.0",
		Height:      1000,
	})
	if err != nil {
		t.Fatalf("attach upgrade plan failed: %v", err)
	}

	// Put in voting
	lip, _ := k.GetLIP(ctx, lipID)
	lip.Stage = types.StatusVoting
	lip.VotingEndBlock = 100
	k.SetLIP(ctx, lip)

	// Vote to reject it (vote NO with majority)
	mock.delegations[testAddr("voter1")] = "500000" // 50% of total
	ms.CastVote(ctx, &types.MsgCastVote{
		Voter: testAddr("voter1"), LipId: lipID, Option: types.VoteNo,
	})

	// Run BeginBlocker
	k.BeginBlocker(ctx)

	// Assert LIP failed
	lip, _ = k.GetLIP(ctx, lipID)
	if lip.Stage != types.StatusFailed {
		t.Errorf("expected failed, got %s", lip.Stage)
	}

	// Assert ScheduleUpgrade was NOT called
	if mockUK.called {
		t.Error("ScheduleUpgrade should not be called for a failed LIP")
	}
}
