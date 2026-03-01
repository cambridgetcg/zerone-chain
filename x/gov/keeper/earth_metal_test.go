package keeper_test

import (
	"testing"

	"github.com/zerone-chain/zerone/x/gov/keeper"
	"github.com/zerone-chain/zerone/x/gov/types"
)

// TestEarthMetal_ParamChangeLIP_UpdatesCaptureDefense verifies the Earth→Metal
// governance flow: a parameter-category LIP that changes capture_defense's
// hhi_threshold param is correctly applied when the LIP passes tally.
func TestEarthMetal_ParamChangeLIP_UpdatesCaptureDefense(t *testing.T) {
	// 1. Setup keeper with mock staking and wire a mock param router.
	k, ctx, mock := setupWithStaking(t, "1000000")
	ms := keeper.NewMsgServerImpl(k)

	mockPR := &mockParamRouter{}
	k.SetParamRouter(mockPR)

	// 2. Submit a parameter-category LIP targeting capture_defense hhi_threshold.
	resp, err := ms.SubmitLIP(ctx, &types.MsgSubmitLIP{
		Proposer:     testAddr("proposer"),
		Title:        "Raise HHI Threshold for Capture Defense",
		Description:  "Increase hhi_threshold to 200000 to tighten capture detection",
		Category:     types.CategoryParameter,
		InitialStake: "1000000",
		ParamChanges: []*types.ParamChange{
			{Module: "capture_defense", Key: "hhi_threshold", Value: "200000"},
		},
	})
	if err != nil {
		t.Fatalf("SubmitLIP failed: %v", err)
	}
	lipID := resp.LipId

	// 3. Fast-forward LIP to voting stage with an end block in the past.
	lip, found := k.GetLIP(ctx, lipID)
	if !found {
		t.Fatalf("GetLIP: LIP %s not found", lipID)
	}
	lip.Stage = types.StatusVoting
	lip.VotingEndBlock = 100 // ctx.BlockHeight() == 100, so tally runs immediately
	k.SetLIP(ctx, lip)

	// 4. Give voter1 majority voting power and cast a yes vote.
	mock.delegations[testAddr("voter1")] = "500000" // 50% of 1000000 total bonded
	_, err = ms.CastVote(ctx, &types.MsgCastVote{
		Voter:  testAddr("voter1"),
		LipId:  lipID,
		Option: types.VoteYes,
	})
	if err != nil {
		t.Fatalf("CastVote failed: %v", err)
	}

	// 5. Run BeginBlocker to trigger tally.
	k.BeginBlocker(ctx)

	// 6. Assert the LIP passed.
	lip, found = k.GetLIP(ctx, lipID)
	if !found {
		t.Fatalf("GetLIP after tally: LIP %s not found", lipID)
	}
	if lip.Stage != types.StatusPassed {
		t.Fatalf("expected LIP stage %q, got %q", types.StatusPassed, lip.Stage)
	}

	// 7. Assert the param router received exactly one change with correct module/key/value.
	if len(mockPR.applied) != 1 {
		t.Fatalf("expected 1 param change applied, got %d", len(mockPR.applied))
	}

	change := mockPR.applied[0]
	if change.module != "capture_defense" {
		t.Errorf("expected module %q, got %q", "capture_defense", change.module)
	}
	if change.key != "hhi_threshold" {
		t.Errorf("expected key %q, got %q", "hhi_threshold", change.key)
	}
	if change.value != "200000" {
		t.Errorf("expected value %q, got %q", "200000", change.value)
	}

	// 8. Assert the param_change_applied event was emitted.
	events := ctx.EventManager().Events()
	appliedCount := 0
	for _, e := range events {
		if e.Type == "zerone.gov.param_change_applied" {
			appliedCount++
			// Verify event attributes reference the correct module and key.
			attrs := make(map[string]string)
			for _, attr := range e.Attributes {
				attrs[attr.Key] = attr.Value
			}
			if attrs["module"] != "capture_defense" {
				t.Errorf("event module attr: expected %q, got %q", "capture_defense", attrs["module"])
			}
			if attrs["key"] != "hhi_threshold" {
				t.Errorf("event key attr: expected %q, got %q", "hhi_threshold", attrs["key"])
			}
		}
	}
	if appliedCount != 1 {
		t.Errorf("expected 1 param_change_applied event, got %d", appliedCount)
	}
}
