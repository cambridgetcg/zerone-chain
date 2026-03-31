package keeper_test

import (
	"fmt"
	"testing"

	"github.com/zerone-chain/zerone/x/gov/keeper"
	"github.com/zerone-chain/zerone/x/gov/types"
)

// ============================================================
// Constitutional Lock Tier Tests
//
// Three tiers: Standard (60%), Elevated (75%), Constitutional (90%)
// Categories:
//   Standard:       text, research_spend
//   Elevated:       parameter, research_seat_election
//   Constitutional: upgrade, research_phase_transition, research_phase_rollback
// ============================================================

func TestConstitutionalTierConfig_DefaultValues(t *testing.T) {
	k, ctx := setupKeeper(t)
	cfg := k.GetConstitutionalTierConfig(ctx)

	if cfg.StandardSupportBps != 600_000 {
		t.Errorf("expected standard=600000, got %d", cfg.StandardSupportBps)
	}
	if cfg.ElevatedSupportBps != 750_000 {
		t.Errorf("expected elevated=750000, got %d", cfg.ElevatedSupportBps)
	}
	if cfg.ConstitutionalSupportBps != 900_000 {
		t.Errorf("expected constitutional=900000, got %d", cfg.ConstitutionalSupportBps)
	}
}

func TestConstitutionalTierConfig_StoreAndRetrieve(t *testing.T) {
	k, ctx := setupKeeper(t)

	custom := keeper.ConstitutionalTierConfig{
		StandardSupportBps:      650_000,
		ElevatedSupportBps:      800_000,
		ConstitutionalSupportBps: 950_000,
	}
	k.SetConstitutionalTierConfig(ctx, custom)
	got := k.GetConstitutionalTierConfig(ctx)

	if got.StandardSupportBps != 650_000 || got.ElevatedSupportBps != 800_000 || got.ConstitutionalSupportBps != 950_000 {
		t.Errorf("stored config mismatch: got %+v", got)
	}
}

func TestGetSupportThresholdForCategory(t *testing.T) {
	k, ctx := setupKeeper(t)

	tests := []struct {
		category      string
		expectedTier  string
		expectedBps   uint64
	}{
		{types.CategoryText, types.TierStandard, 600_000},
		{types.CategoryResearchSpend, types.TierStandard, 600_000},
		{types.CategoryParameter, types.TierElevated, 750_000},
		{types.CategorySeatElection, types.TierElevated, 750_000},
		{types.CategoryUpgrade, types.TierConstitutional, 900_000},
		{types.CategoryPhaseTransition, types.TierConstitutional, 900_000},
		{types.CategoryPhaseRollback, types.TierConstitutional, 900_000},
		{"unknown_category", types.TierConstitutional, 900_000}, // fail-safe
	}

	for _, tc := range tests {
		tier, bps := k.GetSupportThresholdForCategory(ctx, tc.category)
		if tier != tc.expectedTier {
			t.Errorf("category %q: expected tier=%s, got %s", tc.category, tc.expectedTier, tier)
		}
		if bps != tc.expectedBps {
			t.Errorf("category %q: expected bps=%d, got %d", tc.category, tc.expectedBps, bps)
		}
	}
}

func TestTierCategoryMapping(t *testing.T) {
	m := types.DefaultCategoryTierMap()
	expectedLen := 7 // all defined categories
	if len(m) != expectedLen {
		t.Errorf("expected %d categories mapped, got %d", expectedLen, len(m))
	}
}

func TestGetTierForCategory_UnknownFailsafe(t *testing.T) {
	tier := types.GetTierForCategory("nonexistent")
	if tier != types.TierConstitutional {
		t.Errorf("expected fail-safe to constitutional, got %s", tier)
	}
}

// ============================================================
// Tally Integration Tests — verify different categories
// require different support levels to pass.
// ============================================================

// helperCreateVotingLIP creates a LIP in voting stage for a given category.
func helperCreateVotingLIP(t *testing.T, k keeper.Keeper, ctx interface {
	WithBlockHeight(height int64) interface{}
}, lipID, category, proposer string) {
	t.Helper()
	// This is tested through tallyAndResolve via BeginBlocker.
}

func TestTieredTally_StandardCategory_PassesAt60(t *testing.T) {
	k, ctx, mock := setupWithStaking(t, "1000000000000") // 1M ZRN
	ms := keeper.NewMsgServerImpl(k)

	// Submit a text LIP (standard tier = 60% required).
	ms.SubmitLIP(ctx, &types.MsgSubmitLIP{
		Proposer: testAddr("proposer"), Title: "Text Proposal", Description: "Test",
		Category: types.CategoryText, InitialStake: "1000000",
	})
	lip, _ := k.GetLIP(ctx, "LIP-1")
	lip.Stage = types.StatusVoting
	lip.VotingEndBlock = 200
	k.SetLIP(ctx, lip)

	// Give enough for quorum: 40% total participation (>33.4%).
	// Yes = 61%, No = 39% of votes → should PASS at standard 60%.
	mock.delegations[testAddr("yes1")] = "244000000000"   // 24.4% of total bonded
	mock.delegations[testAddr("no1")] = "156000000000"     // 15.6% of total bonded
	// Total voted: 40% (meets quorum). Yes: 244/400 = 61% (meets 60%)

	ms.CastVote(ctx, &types.MsgCastVote{Voter: testAddr("yes1"), LipId: "LIP-1", Option: types.VoteYes})
	ms.CastVote(ctx, &types.MsgCastVote{Voter: testAddr("no1"), LipId: "LIP-1", Option: types.VoteNo})

	// Trigger tally at block 200.
	tallyCtx := ctx.WithBlockHeight(200)
	k.BeginBlocker(tallyCtx)

	lip, _ = k.GetLIP(tallyCtx, "LIP-1")
	if lip.Stage != types.StatusPassed {
		t.Errorf("text LIP at 61%% support should PASS (standard tier=60%%), got stage=%s", lip.Stage)
	}
}

func TestTieredTally_StandardCategory_FailsBelow60(t *testing.T) {
	k, ctx, mock := setupWithStaking(t, "1000000000000")
	ms := keeper.NewMsgServerImpl(k)

	ms.SubmitLIP(ctx, &types.MsgSubmitLIP{
		Proposer: testAddr("proposer"), Title: "Text Proposal Fail", Description: "Test",
		Category: types.CategoryText, InitialStake: "1000000",
	})
	lip, _ := k.GetLIP(ctx, "LIP-1")
	lip.Stage = types.StatusVoting
	lip.VotingEndBlock = 200
	k.SetLIP(ctx, lip)

	// Yes = 58%, No = 42% → should FAIL at standard 60%.
	mock.delegations[testAddr("yes1")] = "232000000000"  // 23.2%
	mock.delegations[testAddr("no1")] = "168000000000"    // 16.8%
	// Total: 40% (quorum met). Yes: 232/400 = 58% (below 60%)

	ms.CastVote(ctx, &types.MsgCastVote{Voter: testAddr("yes1"), LipId: "LIP-1", Option: types.VoteYes})
	ms.CastVote(ctx, &types.MsgCastVote{Voter: testAddr("no1"), LipId: "LIP-1", Option: types.VoteNo})

	tallyCtx := ctx.WithBlockHeight(200)
	k.BeginBlocker(tallyCtx)

	lip, _ = k.GetLIP(tallyCtx, "LIP-1")
	if lip.Stage != types.StatusFailed {
		t.Errorf("text LIP at 58%% support should FAIL (standard tier=60%%), got stage=%s", lip.Stage)
	}
}

func TestTieredTally_ElevatedCategory_PassesAt75(t *testing.T) {
	k, ctx, mock := setupWithStaking(t, "1000000000000")
	ms := keeper.NewMsgServerImpl(k)

	ms.SubmitLIP(ctx, &types.MsgSubmitLIP{
		Proposer: testAddr("proposer"), Title: "Param Change", Description: "Test",
		Category: types.CategoryParameter, InitialStake: "1000000",
	})
	lip, _ := k.GetLIP(ctx, "LIP-1")
	lip.Stage = types.StatusVoting
	lip.VotingEndBlock = 200
	k.SetLIP(ctx, lip)

	// Yes = 76%, No = 24% → should PASS at elevated 75%.
	mock.delegations[testAddr("yes1")] = "304000000000"  // 30.4%
	mock.delegations[testAddr("no1")] = "96000000000"     // 9.6%
	// Total: 40% (quorum). Yes: 304/400 = 76% (above 75%)

	ms.CastVote(ctx, &types.MsgCastVote{Voter: testAddr("yes1"), LipId: "LIP-1", Option: types.VoteYes})
	ms.CastVote(ctx, &types.MsgCastVote{Voter: testAddr("no1"), LipId: "LIP-1", Option: types.VoteNo})

	tallyCtx := ctx.WithBlockHeight(200)
	k.BeginBlocker(tallyCtx)

	lip, _ = k.GetLIP(tallyCtx, "LIP-1")
	if lip.Stage != types.StatusPassed {
		t.Errorf("parameter LIP at 76%% support should PASS (elevated tier=75%%), got stage=%s", lip.Stage)
	}
}

func TestTieredTally_ElevatedCategory_FailsBetween60And75(t *testing.T) {
	k, ctx, mock := setupWithStaking(t, "1000000000000")
	ms := keeper.NewMsgServerImpl(k)

	ms.SubmitLIP(ctx, &types.MsgSubmitLIP{
		Proposer: testAddr("proposer"), Title: "Param Change Fail", Description: "Test",
		Category: types.CategoryParameter, InitialStake: "1000000",
	})
	lip, _ := k.GetLIP(ctx, "LIP-1")
	lip.Stage = types.StatusVoting
	lip.VotingEndBlock = 200
	k.SetLIP(ctx, lip)

	// Yes = 70%, No = 30% → passes standard (60%) but FAILS elevated (75%).
	mock.delegations[testAddr("yes1")] = "280000000000"  // 28%
	mock.delegations[testAddr("no1")] = "120000000000"    // 12%
	// Total: 40%. Yes: 280/400 = 70%

	ms.CastVote(ctx, &types.MsgCastVote{Voter: testAddr("yes1"), LipId: "LIP-1", Option: types.VoteYes})
	ms.CastVote(ctx, &types.MsgCastVote{Voter: testAddr("no1"), LipId: "LIP-1", Option: types.VoteNo})

	tallyCtx := ctx.WithBlockHeight(200)
	k.BeginBlocker(tallyCtx)

	lip, _ = k.GetLIP(tallyCtx, "LIP-1")
	if lip.Stage != types.StatusFailed {
		t.Errorf("parameter LIP at 70%% support should FAIL (elevated tier=75%%), got stage=%s", lip.Stage)
	}
}

func TestTieredTally_ConstitutionalCategory_PassesAt90(t *testing.T) {
	k, ctx, mock := setupWithStaking(t, "1000000000000")
	ms := keeper.NewMsgServerImpl(k)

	ms.SubmitLIP(ctx, &types.MsgSubmitLIP{
		Proposer: testAddr("proposer"), Title: "Upgrade LIP", Description: "Test",
		Category: types.CategoryUpgrade, InitialStake: "1000000",
	})
	lip, _ := k.GetLIP(ctx, "LIP-1")
	lip.Stage = types.StatusVoting
	lip.VotingEndBlock = 200
	k.SetLIP(ctx, lip)

	// Yes = 91%, No = 9% → should PASS at constitutional 90%.
	mock.delegations[testAddr("yes1")] = "364000000000"  // 36.4%
	mock.delegations[testAddr("no1")] = "36000000000"     // 3.6%
	// Total: 40%. Yes: 364/400 = 91%

	ms.CastVote(ctx, &types.MsgCastVote{Voter: testAddr("yes1"), LipId: "LIP-1", Option: types.VoteYes})
	ms.CastVote(ctx, &types.MsgCastVote{Voter: testAddr("no1"), LipId: "LIP-1", Option: types.VoteNo})

	tallyCtx := ctx.WithBlockHeight(200)
	k.BeginBlocker(tallyCtx)

	lip, _ = k.GetLIP(tallyCtx, "LIP-1")
	if lip.Stage != types.StatusPassed {
		t.Errorf("upgrade LIP at 91%% support should PASS (constitutional tier=90%%), got stage=%s", lip.Stage)
	}
}

func TestTieredTally_ConstitutionalCategory_FailsAt85(t *testing.T) {
	k, ctx, mock := setupWithStaking(t, "1000000000000")
	ms := keeper.NewMsgServerImpl(k)

	ms.SubmitLIP(ctx, &types.MsgSubmitLIP{
		Proposer: testAddr("proposer"), Title: "Upgrade LIP Fail", Description: "Test",
		Category: types.CategoryUpgrade, InitialStake: "1000000",
	})
	lip, _ := k.GetLIP(ctx, "LIP-1")
	lip.Stage = types.StatusVoting
	lip.VotingEndBlock = 200
	k.SetLIP(ctx, lip)

	// Yes = 85%, No = 15% → passes elevated (75%) but FAILS constitutional (90%).
	mock.delegations[testAddr("yes1")] = "340000000000"  // 34%
	mock.delegations[testAddr("no1")] = "60000000000"     // 6%
	// Total: 40%. Yes: 340/400 = 85%

	ms.CastVote(ctx, &types.MsgCastVote{Voter: testAddr("yes1"), LipId: "LIP-1", Option: types.VoteYes})
	ms.CastVote(ctx, &types.MsgCastVote{Voter: testAddr("no1"), LipId: "LIP-1", Option: types.VoteNo})

	tallyCtx := ctx.WithBlockHeight(200)
	k.BeginBlocker(tallyCtx)

	lip, _ = k.GetLIP(tallyCtx, "LIP-1")
	if lip.Stage != types.StatusFailed {
		t.Errorf("upgrade LIP at 85%% support should FAIL (constitutional tier=90%%), got stage=%s", lip.Stage)
	}
}

func TestTieredTally_PhaseTransition_IsConstitutional(t *testing.T) {
	k, ctx, mock := setupWithStaking(t, "1000000000000")
	ms := keeper.NewMsgServerImpl(k)

	// Phase transition LIPs require governance state setup — create directly.
	k.SetNextLIPNumber(ctx, 1)
	lip := &types.LIP{
		Id:           "LIP-1",
		Title:        "Phase Transition",
		Description:  "Test",
		Category:     types.CategoryPhaseTransition,
		Proposer:     testAddr("proposer"),
		Stage:        types.StatusVoting,
		StakedAmount: "1000000",
		YesStake:     "0",
		NoStake:      "0",
		AbstainStake: "0",
		VotingEndBlock: 200,
	}
	k.SetLIP(ctx, lip)
	k.SetNextLIPNumber(ctx, 2)

	// Yes = 85% → would pass old 66.7% supermajority but should FAIL new 90%.
	mock.delegations[testAddr("yes1")] = "340000000000"
	mock.delegations[testAddr("no1")] = "60000000000"

	ms.CastVote(ctx, &types.MsgCastVote{Voter: testAddr("yes1"), LipId: "LIP-1", Option: types.VoteYes})
	ms.CastVote(ctx, &types.MsgCastVote{Voter: testAddr("no1"), LipId: "LIP-1", Option: types.VoteNo})

	tallyCtx := ctx.WithBlockHeight(200)
	k.BeginBlocker(tallyCtx)

	lip, _ = k.GetLIP(tallyCtx, "LIP-1")
	if lip.Stage != types.StatusFailed {
		t.Errorf("phase transition at 85%% should FAIL (constitutional=90%%), got stage=%s (old supermajority 66.7%% would have passed)", lip.Stage)
	}
}

func TestTieredTally_BoundaryExactly60(t *testing.T) {
	k, ctx, mock := setupWithStaking(t, "1000000000000")
	ms := keeper.NewMsgServerImpl(k)

	ms.SubmitLIP(ctx, &types.MsgSubmitLIP{
		Proposer: testAddr("proposer"), Title: "Boundary 60", Description: "Test",
		Category: types.CategoryText, InitialStake: "1000000",
	})
	lip, _ := k.GetLIP(ctx, "LIP-1")
	lip.Stage = types.StatusVoting
	lip.VotingEndBlock = 200
	k.SetLIP(ctx, lip)

	// Exactly 60%: yes=600000, no=400000 (out of yes+no=1000000).
	// 600000 * 1000000 / 1000000 = 600000 BPS = exactly 60%. Should PASS (>= threshold).
	mock.delegations[testAddr("yes1")] = "240000000000"  // 24%
	mock.delegations[testAddr("no1")] = "160000000000"    // 16%
	// Total: 40%. Yes: 240/400 = 60%

	ms.CastVote(ctx, &types.MsgCastVote{Voter: testAddr("yes1"), LipId: "LIP-1", Option: types.VoteYes})
	ms.CastVote(ctx, &types.MsgCastVote{Voter: testAddr("no1"), LipId: "LIP-1", Option: types.VoteNo})

	tallyCtx := ctx.WithBlockHeight(200)
	k.BeginBlocker(tallyCtx)

	lip, _ = k.GetLIP(tallyCtx, "LIP-1")
	if lip.Stage != types.StatusPassed {
		t.Errorf("text LIP at exactly 60%% should PASS (>= standard tier threshold), got stage=%s", lip.Stage)
	}
}

func TestTieredTally_CustomTierConfig(t *testing.T) {
	k, ctx, mock := setupWithStaking(t, "1000000000000")
	ms := keeper.NewMsgServerImpl(k)

	// Override tier config: lower standard to 55%.
	k.SetConstitutionalTierConfig(ctx, keeper.ConstitutionalTierConfig{
		StandardSupportBps:      550_000, // 55%
		ElevatedSupportBps:      750_000,
		ConstitutionalSupportBps: 900_000,
	})

	ms.SubmitLIP(ctx, &types.MsgSubmitLIP{
		Proposer: testAddr("proposer"), Title: "Custom Threshold", Description: "Test",
		Category: types.CategoryText, InitialStake: "1000000",
	})
	lip, _ := k.GetLIP(ctx, "LIP-1")
	lip.Stage = types.StatusVoting
	lip.VotingEndBlock = 200
	k.SetLIP(ctx, lip)

	// Yes = 57% → would fail default 60% but should PASS custom 55%.
	mock.delegations[testAddr("yes1")] = "228000000000"
	mock.delegations[testAddr("no1")] = "172000000000"
	// Total: 40%. Yes: 228/400 = 57%

	ms.CastVote(ctx, &types.MsgCastVote{Voter: testAddr("yes1"), LipId: "LIP-1", Option: types.VoteYes})
	ms.CastVote(ctx, &types.MsgCastVote{Voter: testAddr("no1"), LipId: "LIP-1", Option: types.VoteNo})

	tallyCtx := ctx.WithBlockHeight(200)
	k.BeginBlocker(tallyCtx)

	lip, _ = k.GetLIP(tallyCtx, "LIP-1")
	if lip.Stage != types.StatusPassed {
		t.Errorf("text LIP at 57%% with custom 55%% threshold should PASS, got stage=%s", lip.Stage)
	}
}

func TestTieredTally_ResearchSpend_IsStandard(t *testing.T) {
	k, ctx, mock := setupWithStaking(t, "1000000000000")
	ms := keeper.NewMsgServerImpl(k)

	ms.SubmitLIP(ctx, &types.MsgSubmitLIP{
		Proposer: testAddr("proposer"), Title: "Research Spend", Description: "Test",
		Category: types.CategoryResearchSpend, InitialStake: "1000000",
	})
	lip, _ := k.GetLIP(ctx, "LIP-1")
	lip.Stage = types.StatusVoting
	lip.VotingEndBlock = 200
	k.SetLIP(ctx, lip)

	// Yes = 62% → should PASS at standard 60%.
	mock.delegations[testAddr("yes1")] = "248000000000"
	mock.delegations[testAddr("no1")] = "152000000000"

	ms.CastVote(ctx, &types.MsgCastVote{Voter: testAddr("yes1"), LipId: "LIP-1", Option: types.VoteYes})
	ms.CastVote(ctx, &types.MsgCastVote{Voter: testAddr("no1"), LipId: "LIP-1", Option: types.VoteNo})

	tallyCtx := ctx.WithBlockHeight(200)
	k.BeginBlocker(tallyCtx)

	lip, _ = k.GetLIP(tallyCtx, "LIP-1")
	if lip.Stage != types.StatusPassed {
		t.Errorf("research_spend LIP at 62%% should PASS (standard tier=60%%), got stage=%s", lip.Stage)
	}
}

func TestTieredTally_SeatElection_IsElevated(t *testing.T) {
	tier := types.GetTierForCategory(types.CategorySeatElection)
	if tier != types.TierElevated {
		t.Errorf("seat_election should be elevated tier, got %s", tier)
	}
}

func TestTieredTally_PhaseRollback_IsConstitutional(t *testing.T) {
	tier := types.GetTierForCategory(types.CategoryPhaseRollback)
	if tier != types.TierConstitutional {
		t.Errorf("phase_rollback should be constitutional tier, got %s", tier)
	}
}

// TestAllTiers_Comprehensive runs all three categories through the same vote split
// to verify that the SAME vote percentages yield different outcomes based on tier.
func TestAllTiers_Comprehensive(t *testing.T) {
	// 70% yes support — passes standard (60%), fails elevated (75%), fails constitutional (90%).
	tests := []struct {
		name          string
		category      string
		yesPct        int // percent of non-abstain votes that are yes
		expectedStage string
	}{
		{"text_70pct_pass", types.CategoryText, 70, types.StatusPassed},
		{"param_70pct_fail", types.CategoryParameter, 70, types.StatusFailed},
		{"upgrade_70pct_fail", types.CategoryUpgrade, 70, types.StatusFailed},
		{"text_59pct_fail", types.CategoryText, 59, types.StatusFailed},
		{"param_76pct_pass", types.CategoryParameter, 76, types.StatusPassed},
		{"param_74pct_fail", types.CategoryParameter, 74, types.StatusFailed},
		{"upgrade_91pct_pass", types.CategoryUpgrade, 91, types.StatusPassed},
		{"upgrade_89pct_fail", types.CategoryUpgrade, 89, types.StatusFailed},
	}

	for i, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			k, ctx, mock := setupWithStaking(t, "1000000000000")
			ms := keeper.NewMsgServerImpl(k)

			ms.SubmitLIP(ctx, &types.MsgSubmitLIP{
				Proposer: testAddr("proposer"), Title: tc.name, Description: "Test",
				Category: tc.category, InitialStake: "1000000",
			})
			lipID := fmt.Sprintf("LIP-%d", i+1)
			// re-fetch to get incremented counter — since each test uses its own keeper, always LIP-1
			lipID = "LIP-1"
			lip, _ := k.GetLIP(ctx, lipID)
			lip.Stage = types.StatusVoting
			lip.VotingEndBlock = 200
			k.SetLIP(ctx, lip)

			// 40% total participation.
			yesAmt := int64(tc.yesPct) * 4_000_000_000 // yesPct% of 400B total voted
			noAmt := 400_000_000_000 - yesAmt
			mock.delegations[testAddr("yes1")] = fmt.Sprintf("%d", yesAmt)
			mock.delegations[testAddr("no1")] = fmt.Sprintf("%d", noAmt)

			ms.CastVote(ctx, &types.MsgCastVote{Voter: testAddr("yes1"), LipId: lipID, Option: types.VoteYes})
			ms.CastVote(ctx, &types.MsgCastVote{Voter: testAddr("no1"), LipId: lipID, Option: types.VoteNo})

			tallyCtx := ctx.WithBlockHeight(200)
			k.BeginBlocker(tallyCtx)

			lip, _ = k.GetLIP(tallyCtx, lipID)
			if lip.Stage != tc.expectedStage {
				t.Errorf("%s: %d%% yes support expected %s, got %s",
					tc.category, tc.yesPct, tc.expectedStage, lip.Stage)
			}
		})
	}
}
