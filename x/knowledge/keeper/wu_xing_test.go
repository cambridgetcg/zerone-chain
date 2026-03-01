package keeper_test

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/zerone-chain/zerone/x/knowledge/types"
)

// TestWuXing_MetalControlsWood verifies that capture-flagged domains
// have reduced carrying capacity proportional to HHI (R31-1).
func TestWuXing_MetalControlsWood(t *testing.T) {
	k, ctx := setupKnowledgeTest(t)

	// Baseline: unflagged domain has base capacity
	baseCapacity := k.GetDomainCarryingCapacity(ctx, "physics")
	require.Equal(t, uint64(1000), baseCapacity)

	// Flag the domain with 25% HHI (threshold level)
	mockCD := &mockCaptureDefenseForCapacity{
		flagged:   map[string]bool{"physics": true},
		penalties: map[string]uint64{"physics": 250_000},
	}
	k.SetCaptureDefenseKeeper(mockCD)

	flaggedCapacity := k.GetDomainCarryingCapacity(ctx, "physics")
	require.Equal(t, uint64(750), flaggedCapacity) // 25% reduction

	// Pressure should increase with reduced capacity
	for i := 0; i < 5; i++ {
		k.IncrementDomainFactCount(ctx, "physics", true, 100_000)
	}

	pressureFlagged := k.GetDomainPressure(ctx, "physics")

	// Unflag: capacity returns to base
	mockCD.flagged["physics"] = false
	pressureUnflagged := k.GetDomainPressure(ctx, "physics")

	// Pressure should be higher when flagged (smaller capacity, same population)
	require.Greater(t, pressureFlagged, pressureUnflagged)
}

// TestWuXing_WoodControlsEarth verifies that a verification backlog
// degrades the alignment knowledge quality sensor (R31-1).
func TestWuXing_WoodControlsEarth(t *testing.T) {
	k, ctx := setupKnowledgeTest(t)

	// Add 10 active facts
	for i := 0; i < 10; i++ {
		require.NoError(t, k.SetClaim(ctx, &types.Claim{
			Id:     fmt.Sprintf("fact-%d", i),
			Status: types.ClaimStatus_CLAIM_STATUS_ACCEPTED,
			Domain: "physics",
		}))
	}

	// Healthy ratio: 2 pending / 10 active = 20%
	for i := 0; i < 2; i++ {
		require.NoError(t, k.SetClaim(ctx, &types.Claim{
			Id:     fmt.Sprintf("pending-a-%d", i),
			Status: types.ClaimStatus_CLAIM_STATUS_PENDING,
			Domain: "physics",
		}))
	}
	healthyRatio := k.GetPendingVerificationRatio(ctx)
	require.Equal(t, uint64(200_000), healthyRatio) // 20%

	// Add 14 more pending claims → 16 pending / 10 active = 160% (exceeds 150% threshold)
	for i := 0; i < 14; i++ {
		require.NoError(t, k.SetClaim(ctx, &types.Claim{
			Id:     fmt.Sprintf("pending-b-%d", i),
			Status: types.ClaimStatus_CLAIM_STATUS_PENDING,
			Domain: "physics",
		}))
	}
	overloadRatio := k.GetPendingVerificationRatio(ctx)
	require.Equal(t, uint64(1_600_000), overloadRatio) // 160%
	require.Greater(t, overloadRatio, uint64(1_500_000), "should exceed backlog threshold")
}

// TestWuXing_CapturePenaltyEvent verifies event emission on penalized domain (R31-1).
func TestWuXing_CapturePenaltyEvent(t *testing.T) {
	k, ctx := setupKnowledgeTest(t)
	mockCD := &mockCaptureDefenseForCapacity{
		flagged:   map[string]bool{"physics": true},
		penalties: map[string]uint64{"physics": 500_000}, // 50% HHI
	}
	k.SetCaptureDefenseKeeper(mockCD)

	k.EmitCapacityPenaltyEvent(ctx, "physics")

	sdkCtx := sdk.UnwrapSDKContext(ctx)
	events := sdkCtx.EventManager().Events()
	var penaltyEvent *sdk.Event
	for i, e := range events {
		if e.Type == "zerone.knowledge.capacity_penalty_applied" {
			penaltyEvent = &events[i]
		}
	}
	require.NotNil(t, penaltyEvent, "expected capacity_penalty_applied event")

	attrMap := make(map[string]string)
	for _, attr := range penaltyEvent.Attributes {
		attrMap[attr.Key] = attr.Value
	}
	require.Equal(t, "physics", attrMap["domain"])
	require.Equal(t, "1000", attrMap["base_capacity"])
	require.Equal(t, "500", attrMap["effective_capacity"])
	require.Equal(t, "500000", attrMap["capture_penalty_bps"])
	require.Equal(t, "capture_flagged", attrMap["reason"])
}
