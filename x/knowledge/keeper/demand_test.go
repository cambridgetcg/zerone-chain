package keeper_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/zerone-chain/zerone/x/knowledge/keeper"
	"github.com/zerone-chain/zerone/x/knowledge/types"
)

// ─── Demand Tracking ─────────────────────────────────────────────────────────

func TestDemandTracking_FulfilledQuery(t *testing.T) {
	k, ctx := setupKnowledgeTest(t)

	signal, existed := k.GetOrCreateDemandSignal(ctx, "physics", "quantum entanglement")
	require.False(t, existed)

	signal.QueryCount = 1
	signal.FulfilledCount = 1
	signal.UnfulfilledCount = 0
	require.NoError(t, k.SetDemandSignal(ctx, signal))

	got, found := k.GetDemandSignal(ctx, "physics", "quantum entanglement")
	require.True(t, found)
	require.Equal(t, uint64(1), got.QueryCount)
	require.Equal(t, uint64(1), got.FulfilledCount)
	require.Equal(t, uint64(0), got.UnfulfilledCount)
}

func TestDemandTracking_UnfulfilledQuery(t *testing.T) {
	k, ctx := setupKnowledgeTest(t)

	signal, _ := k.GetOrCreateDemandSignal(ctx, "physics", "dark matter composition")
	signal.QueryCount = 1
	signal.UnfulfilledCount = 1
	signal.EpochUnfulfilled = 1
	require.NoError(t, k.SetDemandSignal(ctx, signal))

	got, found := k.GetDemandSignal(ctx, "physics", "dark matter composition")
	require.True(t, found)
	require.Equal(t, uint64(1), got.UnfulfilledCount)
	require.Equal(t, uint64(1), got.EpochUnfulfilled)
}

// ─── Bounty Creation ─────────────────────────────────────────────────────────

func TestBountyCreation_ThresholdMet(t *testing.T) {
	k, ctx := setupKnowledgeTest(t)

	// Configure params
	params, err := k.GetParams(ctx)
	require.NoError(t, err)
	params.DemandTrackingEnabled = true
	params.DemandBountyThreshold = 100
	params.DemandBountyBaseReward = "10000000"    // 10M uzrn
	params.DemandBountyPerQueryBonus = "100000"   // 100K uzrn
	params.DemandBountyExpiryEpochs = 50
	require.NoError(t, k.SetParams(ctx, params))

	// Create demand signal exceeding threshold
	signal, _ := k.GetOrCreateDemandSignal(ctx, "physics", "quantum error correction")
	signal.EpochUnfulfilled = 150
	require.NoError(t, k.SetDemandSignal(ctx, signal))

	// Process bounties
	require.NoError(t, k.ProcessDemandBounties(ctx, 1))

	// Verify bounty created
	bounties := k.GetActiveBounties(ctx, "physics")
	require.Len(t, bounties, 1)

	// Reward = base (10M) + 150 * 100K = 10M + 15M = 25M
	require.Equal(t, "25000000", bounties[0].RewardAmount)
	require.Equal(t, "physics", bounties[0].Domain)
	require.Equal(t, "quantum error correction", bounties[0].Subject)
	require.False(t, bounties[0].Claimed)
}

func TestBountyCreation_ThresholdNotMet(t *testing.T) {
	k, ctx := setupKnowledgeTest(t)

	params, err := k.GetParams(ctx)
	require.NoError(t, err)
	params.DemandTrackingEnabled = true
	params.DemandBountyThreshold = 100
	require.NoError(t, k.SetParams(ctx, params))

	// Create demand signal below threshold
	signal, _ := k.GetOrCreateDemandSignal(ctx, "physics", "quantum error correction")
	signal.EpochUnfulfilled = 99
	require.NoError(t, k.SetDemandSignal(ctx, signal))

	require.NoError(t, k.ProcessDemandBounties(ctx, 1))

	bounties := k.GetActiveBounties(ctx, "physics")
	require.Len(t, bounties, 0, "no bounty should be created when below threshold")
}

func TestBountyCreation_AlreadyExists(t *testing.T) {
	k, ctx := setupKnowledgeTest(t)

	params, err := k.GetParams(ctx)
	require.NoError(t, err)
	params.DemandTrackingEnabled = true
	params.DemandBountyThreshold = 100
	require.NoError(t, k.SetParams(ctx, params))

	// Create an existing bounty for the domain/subject
	existingBounty := &types.KnowledgeBounty{
		Id:             "existing-bounty-1",
		Domain:         "physics",
		Subject:        "quantum error correction",
		RewardAmount:   "5000000",
		CreatedAtBlock: 50,
		ExpiresAtBlock: 50000,
		Claimed:        false,
	}
	require.NoError(t, k.SetBounty(ctx, existingBounty))

	// Create demand signal exceeding threshold
	signal, _ := k.GetOrCreateDemandSignal(ctx, "physics", "quantum error correction")
	signal.EpochUnfulfilled = 150
	require.NoError(t, k.SetDemandSignal(ctx, signal))

	require.NoError(t, k.ProcessDemandBounties(ctx, 1))

	// Should still have only 1 bounty — no duplicate
	bounties := k.GetActiveBounties(ctx, "physics")
	require.Len(t, bounties, 1)
	require.Equal(t, "existing-bounty-1", bounties[0].Id, "should keep existing bounty, not create duplicate")
}

// ─── Bounty Claiming ─────────────────────────────────────────────────────────

func TestBountyClaim_MatchingFact(t *testing.T) {
	k, ctx, bk := setupKnowledgeTestWithBank(t)

	// Create bounty
	bounty := &types.KnowledgeBounty{
		Id:             "bounty-match-1",
		Domain:         "physics",
		Subject:        "quantum error correction",
		RewardAmount:   "25000000",
		CreatedAtBlock: 50,
		ExpiresAtBlock: 50000,
		Claimed:        false,
	}
	require.NoError(t, k.SetBounty(ctx, bounty))

	// Create a fact with matching subject in Structure
	submitter := makeValidBech32Addr("submitter1")
	fact := &types.Fact{
		Id:      "fact-bounty-1",
		Content: "Quantum error correction enables fault-tolerant computation",
		Domain:  "physics",
		Status:  types.FactStatus_FACT_STATUS_VERIFIED,
		Structure: &types.ClaimStructure{
			Subject: "quantum error correction",
		},
		Submitter: submitter,
	}
	require.NoError(t, k.SetFact(ctx, fact))

	claim := &types.Claim{
		Id:        "claim-bounty-1",
		Submitter: submitter,
	}

	k.ClaimBountyForFact(ctx, fact, claim)

	// Verify bounty marked as claimed
	updated, found := k.GetBounty(ctx, "bounty-match-1")
	require.True(t, found)
	require.True(t, updated.Claimed)
	require.Equal(t, "fact-bounty-1", updated.ClaimedByFactId)

	// Verify bank send happened
	require.Len(t, bk.sendCalls, 1)
	require.Equal(t, "knowledge", bk.sendCalls[0].from)
	require.Equal(t, submitter, bk.sendCalls[0].to)
}

func TestBountyClaim_WrongSubject(t *testing.T) {
	k, ctx := setupKnowledgeTest(t)

	// Create bounty for "quantum error correction"
	bounty := &types.KnowledgeBounty{
		Id:             "bounty-wrong-1",
		Domain:         "physics",
		Subject:        "quantum error correction",
		RewardAmount:   "25000000",
		CreatedAtBlock: 50,
		ExpiresAtBlock: 50000,
		Claimed:        false,
	}
	require.NoError(t, k.SetBounty(ctx, bounty))

	// Create a fact with DIFFERENT subject
	submitter := makeValidBech32Addr("submitter2")
	fact := &types.Fact{
		Id:      "fact-wrong-1",
		Content: "Classical mechanics describes macroscopic motion",
		Domain:  "physics",
		Status:  types.FactStatus_FACT_STATUS_VERIFIED,
		Structure: &types.ClaimStructure{
			Subject: "classical mechanics",
		},
		Submitter: submitter,
	}
	require.NoError(t, k.SetFact(ctx, fact))

	claim := &types.Claim{
		Id:        "claim-wrong-1",
		Submitter: submitter,
	}

	k.ClaimBountyForFact(ctx, fact, claim)

	// Verify bounty NOT claimed
	updated, found := k.GetBounty(ctx, "bounty-wrong-1")
	require.True(t, found)
	require.False(t, updated.Claimed, "bounty should not be claimed for mismatched subject")
}

// ─── Bounty Expiry ──────────────────────────────────────────────────────────

func TestBountyExpiry(t *testing.T) {
	k, ctx, bk := setupKnowledgeTestWithBank(t)
	// ctx starts at block height 100

	// Create bounty expiring at block 200
	bounty := &types.KnowledgeBounty{
		Id:             "bounty-expire-1",
		Domain:         "physics",
		Subject:        "dark energy equation of state",
		RewardAmount:   "15000000",
		CreatedAtBlock: 50,
		ExpiresAtBlock: 200,
		Claimed:        false,
	}
	require.NoError(t, k.SetBounty(ctx, bounty))

	// Advance past expiry (block 201)
	ctx = advanceBlocks(ctx, 101)

	k.ProcessExpiredBounties(ctx)

	// Verify bounty marked as claimed (expired)
	updated, found := k.GetBounty(ctx, "bounty-expire-1")
	require.True(t, found)
	require.True(t, updated.Claimed, "expired bounty should be marked as claimed")

	// Verify funds returned to protocol_treasury
	require.Len(t, bk.sendCalls, 1)
	require.Equal(t, "knowledge", bk.sendCalls[0].from)
	require.Equal(t, "protocol_treasury", bk.sendCalls[0].to)
}

// ─── Demand Multiplier ──────────────────────────────────────────────────────

func TestDemandMultiplier_HighDemand(t *testing.T) {
	k, ctx := setupKnowledgeTest(t)

	params, err := k.GetParams(ctx)
	require.NoError(t, err)
	params.DemandMultiplierCap = 10_000_000 // 10x
	require.NoError(t, k.SetParams(ctx, params))

	// Create signal with 50 epoch queries
	signal, _ := k.GetOrCreateDemandSignal(ctx, "physics", "quantum gravity")
	signal.EpochQueryCount = 50
	require.NoError(t, k.SetDemandSignal(ctx, signal))

	// Multiplier = 1,000,000 (base) + (50/10) * 100,000 = 1,000,000 + 500,000 = 1,500,000
	multiplier := k.GetDemandMultiplier(ctx, "physics", "quantum gravity")
	require.Equal(t, uint64(1_500_000), multiplier)
}

func TestDemandMultiplier_Cap(t *testing.T) {
	k, ctx := setupKnowledgeTest(t)

	params, err := k.GetParams(ctx)
	require.NoError(t, err)
	params.DemandMultiplierCap = 10_000_000 // 10x
	require.NoError(t, k.SetParams(ctx, params))

	// Create signal with 1000 epoch queries (would exceed cap without capping)
	signal, _ := k.GetOrCreateDemandSignal(ctx, "physics", "theory of everything")
	signal.EpochQueryCount = 1000
	require.NoError(t, k.SetDemandSignal(ctx, signal))

	// Uncapped: 1,000,000 + (1000/10) * 100,000 = 1,000,000 + 10,000,000 = 11,000,000
	// Capped at 10,000,000
	multiplier := k.GetDemandMultiplier(ctx, "physics", "theory of everything")
	require.Equal(t, uint64(10_000_000), multiplier)
}

// ─── Top Demand Gaps ────────────────────────────────────────────────────────

func TestTopDemandGaps_Sorted(t *testing.T) {
	k, ctx := setupKnowledgeTest(t)

	// Create 4 signals with different UnfulfilledCount values
	subjects := []struct {
		subject          string
		unfulfilledCount uint64
	}{
		{"quantum gravity", 50},
		{"dark matter composition", 200},
		{"consciousness theory", 10},
		{"quantum error correction", 150},
	}
	for _, s := range subjects {
		signal, _ := k.GetOrCreateDemandSignal(ctx, "physics", s.subject)
		signal.UnfulfilledCount = s.unfulfilledCount
		require.NoError(t, k.SetDemandSignal(ctx, signal))
	}

	// Get top 3
	top := k.GetTopDemandGaps(ctx, 3)
	require.Len(t, top, 3)

	// Should be sorted descending by UnfulfilledCount
	require.Equal(t, uint64(200), top[0].UnfulfilledCount, "first should be highest demand")
	require.Equal(t, uint64(150), top[1].UnfulfilledCount, "second should be next highest")
	require.Equal(t, uint64(50), top[2].UnfulfilledCount, "third should be third highest")

	// The one with 10 should be excluded (4th)
	for _, s := range top {
		require.NotEqual(t, uint64(10), s.UnfulfilledCount, "lowest demand should be excluded from top 3")
	}
}

// ─── Report Demand Authorization ─────────────────────────────────────────────

func TestReportDemand_Unauthorized(t *testing.T) {
	k, ctx := setupKnowledgeTest(t)

	// Set authorized reporters
	authorizedAddr := makeValidBech32Addr("authorized1")
	params, err := k.GetParams(ctx)
	require.NoError(t, err)
	params.AuthorizedDemandReporters = []string{authorizedAddr}
	params.DemandTrackingEnabled = true
	require.NoError(t, k.SetParams(ctx, params))

	msgServer := keeper.NewMsgServerImpl(k)

	// Unauthorized reporter should fail
	unauthorizedAddr := makeValidBech32Addr("unauthorized")
	_, err = msgServer.ReportDemand(ctx, &types.MsgReportDemand{
		Reporter: unauthorizedAddr,
		Reports: []*types.DemandReport{
			{
				Domain:      "physics",
				Subject:     "quantum gravity",
				Queries:     10,
				Fulfilled:   3,
				Unfulfilled: 7,
			},
		},
	})
	require.Error(t, err, "unauthorized reporter should be rejected")
	require.Contains(t, err.Error(), "unauthorized")

	// Authorized reporter should succeed
	_, err = msgServer.ReportDemand(ctx, &types.MsgReportDemand{
		Reporter: authorizedAddr,
		Reports: []*types.DemandReport{
			{
				Domain:      "physics",
				Subject:     "quantum gravity",
				Queries:     10,
				Fulfilled:   3,
				Unfulfilled: 7,
			},
		},
	})
	require.NoError(t, err, "authorized reporter should succeed")

	// Verify signal stored
	signal, found := k.GetDemandSignal(ctx, "physics", "quantum gravity")
	require.True(t, found, "demand signal should be stored after authorized report")
	require.Equal(t, uint64(10), signal.QueryCount)
	require.Equal(t, uint64(3), signal.FulfilledCount)
	require.Equal(t, uint64(7), signal.UnfulfilledCount)
	require.Equal(t, uint64(10), signal.EpochQueryCount)
	require.Equal(t, uint64(7), signal.EpochUnfulfilled)
}
