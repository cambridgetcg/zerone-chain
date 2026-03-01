package keeper_test

import (
	"context"
	"testing"

	"github.com/zerone-chain/zerone/x/gov/keeper"
	"github.com/zerone-chain/zerone/x/gov/types"
)

// ---------- Mock AlignmentKeeper ----------

type mockAlignmentKeeper struct {
	healthCategory string
}

func (m *mockAlignmentKeeper) GetHealthCategory(_ context.Context) string {
	return m.healthCategory
}

// ---------- Tests ----------

// Test 2: Wood→Earth — degraded health expedites knowledge param LIPs.
func TestWoodEarth_DegradedHealth_ExpeditesKnowledgeLIP(t *testing.T) {
	k, ctx, _ := setupWithStaking(t, "1000000")

	ak := &mockAlignmentKeeper{healthCategory: "degraded"}
	k.SetAlignmentKeeper(ak)

	// Set discussion period to 0 so last_call→voting transition fires immediately.
	params := k.GetParams(ctx)
	params.DiscussionPeriodBlocks = 0
	k.SetParams(ctx, params)

	baseVotingPeriod := params.VotingPeriodBlocks

	ms := keeper.NewMsgServerImpl(k)

	resp, err := ms.SubmitLIP(ctx, &types.MsgSubmitLIP{
		Proposer:     testAddr("alice"),
		Title:        "Adjust Knowledge Params",
		Description:  "Update verification rate threshold",
		Category:     types.CategoryParameter,
		InitialStake: "1000000",
		ParamChanges: []*types.ParamChange{
			{Module: "knowledge", Key: "min_claim_stake", Value: "500000"},
		},
	})
	if err != nil {
		t.Fatalf("submit failed: %v", err)
	}

	lip, _ := k.GetLIP(ctx, resp.LipId)
	lip.Stage = types.StatusLastCall
	lip.LastCallStartedBlock = 0
	k.SetLIP(ctx, lip)

	k.BeginBlocker(ctx)

	lip, _ = k.GetLIP(ctx, resp.LipId)
	if lip.Stage != types.StatusVoting {
		t.Fatalf("expected voting stage, got %s", lip.Stage)
	}

	expectedEnd := uint64(ctx.BlockHeight()) + baseVotingPeriod/2
	if lip.VotingEndBlock != expectedEnd {
		t.Errorf("expected expedited VotingEndBlock=%d, got %d (base=%d)", expectedEnd, lip.VotingEndBlock, baseVotingPeriod)
	}
}

// Test 3: Wood→Earth — healthy system uses normal voting period.
func TestWoodEarth_HealthySystem_NormalVotingPeriod(t *testing.T) {
	k, ctx, _ := setupWithStaking(t, "1000000")

	ak := &mockAlignmentKeeper{healthCategory: "healthy"}
	k.SetAlignmentKeeper(ak)

	// Set discussion period to 0 so last_call→voting transition fires immediately.
	params := k.GetParams(ctx)
	params.DiscussionPeriodBlocks = 0
	k.SetParams(ctx, params)

	baseVotingPeriod := params.VotingPeriodBlocks

	ms := keeper.NewMsgServerImpl(k)

	resp, err := ms.SubmitLIP(ctx, &types.MsgSubmitLIP{
		Proposer:     testAddr("alice"),
		Title:        "Adjust Knowledge Params",
		Description:  "Same params but healthy system",
		Category:     types.CategoryParameter,
		InitialStake: "1000000",
		ParamChanges: []*types.ParamChange{
			{Module: "knowledge", Key: "min_claim_stake", Value: "500000"},
		},
	})
	if err != nil {
		t.Fatalf("submit failed: %v", err)
	}

	lip, _ := k.GetLIP(ctx, resp.LipId)
	lip.Stage = types.StatusLastCall
	lip.LastCallStartedBlock = 0
	k.SetLIP(ctx, lip)

	k.BeginBlocker(ctx)

	lip, _ = k.GetLIP(ctx, resp.LipId)
	if lip.Stage != types.StatusVoting {
		t.Fatalf("expected voting stage, got %s", lip.Stage)
	}

	expectedEnd := uint64(ctx.BlockHeight()) + baseVotingPeriod
	if lip.VotingEndBlock != expectedEnd {
		t.Errorf("expected normal VotingEndBlock=%d, got %d", expectedEnd, lip.VotingEndBlock)
	}
}

// Test 4: Wood→Earth — non-knowledge LIPs get normal period even during degradation.
func TestWoodEarth_NonKnowledgeLIP_NormalPeriodDuringDegradation(t *testing.T) {
	k, ctx, _ := setupWithStaking(t, "1000000")

	ak := &mockAlignmentKeeper{healthCategory: "degraded"}
	k.SetAlignmentKeeper(ak)

	// Set discussion period to 0 so last_call→voting transition fires immediately.
	params := k.GetParams(ctx)
	params.DiscussionPeriodBlocks = 0
	k.SetParams(ctx, params)

	baseVotingPeriod := params.VotingPeriodBlocks

	ms := keeper.NewMsgServerImpl(k)

	resp, err := ms.SubmitLIP(ctx, &types.MsgSubmitLIP{
		Proposer:     testAddr("alice"),
		Title:        "Adjust Staking Params",
		Description:  "Not knowledge-related",
		Category:     types.CategoryParameter,
		InitialStake: "1000000",
		ParamChanges: []*types.ParamChange{
			{Module: "zerone_staking", Key: "max_validators", Value: "200"},
		},
	})
	if err != nil {
		t.Fatalf("submit failed: %v", err)
	}

	lip, _ := k.GetLIP(ctx, resp.LipId)
	lip.Stage = types.StatusLastCall
	lip.LastCallStartedBlock = 0
	k.SetLIP(ctx, lip)

	k.BeginBlocker(ctx)

	lip, _ = k.GetLIP(ctx, resp.LipId)
	if lip.Stage != types.StatusVoting {
		t.Fatalf("expected voting stage, got %s", lip.Stage)
	}

	expectedEnd := uint64(ctx.BlockHeight()) + baseVotingPeriod
	if lip.VotingEndBlock != expectedEnd {
		t.Errorf("expected normal VotingEndBlock=%d, got %d (should NOT be expedited)", expectedEnd, lip.VotingEndBlock)
	}
}

// Verify manual AdvanceLIPStage also respects expedited voting.
func TestWoodEarth_ManualAdvance_ExpeditesKnowledgeLIP(t *testing.T) {
	k, ctx, _ := setupWithStaking(t, "1000000")

	ak := &mockAlignmentKeeper{healthCategory: "critical"}
	k.SetAlignmentKeeper(ak)

	params := k.GetParams(ctx)
	baseVotingPeriod := params.VotingPeriodBlocks

	// Create msgServer AFTER setting alignment keeper so the copy includes it.
	ms := keeper.NewMsgServerImpl(k)

	resp, err := ms.SubmitLIP(ctx, &types.MsgSubmitLIP{
		Proposer:     testAddr("alice"),
		Title:        "Adjust Alignment Params",
		Description:  "Critical system",
		Category:     types.CategoryParameter,
		InitialStake: "1000000",
		ParamChanges: []*types.ParamChange{
			{Module: "alignment", Key: "critical_threshold", Value: "100000"},
		},
	})
	if err != nil {
		t.Fatalf("submit failed: %v", err)
	}

	lip, _ := k.GetLIP(ctx, resp.LipId)
	lip.Stage = types.StatusLastCall
	lip.LastCallStartedBlock = 0
	k.SetLIP(ctx, lip)

	_, err = ms.AdvanceLIPStage(ctx, &types.MsgAdvanceLIPStage{
		Authority: testAddr("alice"),
		LipId:     resp.LipId,
	})
	if err != nil {
		t.Fatalf("advance failed: %v", err)
	}

	lip, _ = k.GetLIP(ctx, resp.LipId)
	expectedEnd := uint64(ctx.BlockHeight()) + baseVotingPeriod/2
	if lip.VotingEndBlock != expectedEnd {
		t.Errorf("expected expedited VotingEndBlock=%d, got %d", expectedEnd, lip.VotingEndBlock)
	}
}
