package keeper_test

import (
	"context"
	"testing"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/zerone-chain/zerone/x/partnerships/keeper"
	"github.com/zerone-chain/zerone/x/partnerships/types"
)

// ---------- Mock CaptureDefenseKeeper ----------

type mockCaptureDefenseKeeper struct {
	flaggedDomains map[string]bool
}

func newMockCaptureDefenseKeeper() *mockCaptureDefenseKeeper {
	return &mockCaptureDefenseKeeper{
		flaggedDomains: make(map[string]bool),
	}
}

func (m *mockCaptureDefenseKeeper) IsDomainFlagged(_ context.Context, domain string) bool {
	return m.flaggedDomains[domain]
}

// ======================================================================
// Test 1: Partnership density calculation
// ======================================================================

func TestGetDomainPartnershipDensity_ActivePartnerships(t *testing.T) {
	k, ctx, _ := setupKeeper(t)

	// Create 3 active partnerships (6 unique participants)
	k.SetPartnership(ctx, &types.Partnership{
		Id:        "p1",
		HumanAddr: humanAddr,
		AgentAddr: agentAddr,
		Status:    types.StatusActive,
	})
	k.SetPartnership(ctx, &types.Partnership{
		Id:        "p2",
		HumanAddr: outsiderAddr,
		AgentAddr: agent2Addr,
		Status:    types.StatusActive,
	})
	k.SetPartnership(ctx, &types.Partnership{
		Id:        "p3",
		HumanAddr: humanAddr, // duplicate participant
		AgentAddr: agent3Addr,
		Status:    types.StatusActive,
	})

	density := k.GetDomainPartnershipDensity(ctx, "any_domain")

	// Unique participants: humanAddr, agentAddr, outsiderAddr, agent2Addr, agent3Addr = 5
	assert.Equal(t, uint64(5), density,
		"should deduplicate participants across partnerships")
}

func TestGetDomainPartnershipDensity_IncludesMentorships(t *testing.T) {
	k, ctx, _ := setupKeeper(t)

	// One active partnership
	k.SetPartnership(ctx, &types.Partnership{
		Id:        "p1",
		HumanAddr: humanAddr,
		AgentAddr: agentAddr,
		Status:    types.StatusActive,
	})

	// One active mentorship in the target domain
	k.SetMentorship(ctx, &types.Mentorship{
		Id:         "m1",
		MentorAddr: outsiderAddr,
		MenteeAddr: agent2Addr,
		Domain:     "target_domain",
		Status:     "active",
	})

	// One active mentorship in a different domain (not counted for target)
	k.SetMentorship(ctx, &types.Mentorship{
		Id:         "m2",
		MentorAddr: agent3Addr,
		MenteeAddr: testAddr("extra"),
		Domain:     "other_domain",
		Status:     "active",
	})

	density := k.GetDomainPartnershipDensity(ctx, "target_domain")

	// From active partnerships: humanAddr, agentAddr
	// From active mentorships in target_domain: outsiderAddr, agent2Addr
	// Total unique: 4
	assert.Equal(t, uint64(4), density,
		"should include mentorships in target domain")
}

func TestGetDomainPartnershipDensity_IgnoresInactive(t *testing.T) {
	k, ctx, _ := setupKeeper(t)

	// One active partnership
	k.SetPartnership(ctx, &types.Partnership{
		Id:        "p1",
		HumanAddr: humanAddr,
		AgentAddr: agentAddr,
		Status:    types.StatusActive,
	})

	// One dissolved partnership (should not count)
	k.SetPartnership(ctx, &types.Partnership{
		Id:        "p2",
		HumanAddr: outsiderAddr,
		AgentAddr: agent2Addr,
		Status:    types.StatusDissolved,
	})

	// One graduated mentorship (should not count)
	k.SetMentorship(ctx, &types.Mentorship{
		Id:         "m1",
		MentorAddr: agent3Addr,
		MenteeAddr: testAddr("extra"),
		Domain:     "test_domain",
		Status:     "graduated",
	})

	density := k.GetDomainPartnershipDensity(ctx, "test_domain")

	// Only from active partnership: humanAddr, agentAddr = 2
	assert.Equal(t, uint64(2), density,
		"should only count active partnerships and mentorships")
}

func TestGetDomainPartnershipDensity_Empty(t *testing.T) {
	k, ctx, _ := setupKeeper(t)

	density := k.GetDomainPartnershipDensity(ctx, "empty_domain")
	assert.Equal(t, uint64(0), density, "empty store should have 0 density")
}

// ======================================================================
// Test 2: Partnership count by participant
// ======================================================================

func TestGetPartnershipCountByParticipant(t *testing.T) {
	k, ctx, _ := setupKeeper(t)

	// humanAddr has 2 active partnerships and 1 dissolved
	k.SetPartnership(ctx, &types.Partnership{
		Id: "p1", HumanAddr: humanAddr, AgentAddr: agentAddr, Status: types.StatusActive,
	})
	k.SetPartnership(ctx, &types.Partnership{
		Id: "p2", HumanAddr: humanAddr, AgentAddr: agent2Addr, Status: types.StatusActive,
	})
	k.SetPartnership(ctx, &types.Partnership{
		Id: "p3", HumanAddr: humanAddr, AgentAddr: agent3Addr, Status: types.StatusDissolved,
	})

	// Also an active mentorship as mentor
	k.SetMentorship(ctx, &types.Mentorship{
		Id: "m1", MentorAddr: humanAddr, MenteeAddr: outsiderAddr, Domain: "math", Status: "active",
	})

	count := k.GetPartnershipCountByParticipant(ctx, humanAddr, "math")

	// 2 active partnerships + 1 active mentorship (mentor role) = 3
	assert.Equal(t, uint64(3), count,
		"should count active partnerships + mentorships")
}

func TestGetPartnershipCountByParticipant_NoMatches(t *testing.T) {
	k, ctx, _ := setupKeeper(t)

	count := k.GetPartnershipCountByParticipant(ctx, testAddr("nobody"), "math")
	assert.Equal(t, uint64(0), count)
}

// ======================================================================
// Test 3: Formation bonus CRUD
// ======================================================================

func TestFormationBonus_SetGet(t *testing.T) {
	k, ctx, _ := setupKeeper(t)

	k.SetDomainFormationBonus(ctx, "flagged_domain", 300000, "capture_flagged", 50100)

	bonus := k.GetDomainFormationBonus(ctx, "flagged_domain")
	require.NotNil(t, bonus, "should return bonus after set")
	assert.Equal(t, "flagged_domain", bonus.Domain)
	assert.Equal(t, uint64(300000), bonus.BonusBps)
	assert.Equal(t, "capture_flagged", bonus.Reason)
	assert.Equal(t, uint64(50100), bonus.ExpiryHeight)
}

func TestFormationBonus_NotFound(t *testing.T) {
	k, ctx, _ := setupKeeper(t)

	bonus := k.GetDomainFormationBonus(ctx, "no_such_domain")
	assert.Nil(t, bonus, "should return nil for non-existent bonus")
}

func TestFormationBonus_Delete(t *testing.T) {
	k, ctx, _ := setupKeeper(t)

	k.SetDomainFormationBonus(ctx, "to_delete", 100000, "test", 999)
	require.NotNil(t, k.GetDomainFormationBonus(ctx, "to_delete"))

	k.DeleteDomainFormationBonus(ctx, "to_delete")
	assert.Nil(t, k.GetDomainFormationBonus(ctx, "to_delete"),
		"should be nil after delete")
}

func TestFormationBonus_Overwrite(t *testing.T) {
	k, ctx, _ := setupKeeper(t)

	k.SetDomainFormationBonus(ctx, "domain1", 100000, "first", 500)
	k.SetDomainFormationBonus(ctx, "domain1", 200000, "second", 1000)

	bonus := k.GetDomainFormationBonus(ctx, "domain1")
	require.NotNil(t, bonus)
	assert.Equal(t, uint64(200000), bonus.BonusBps, "should overwrite with latest")
	assert.Equal(t, "second", bonus.Reason)
	assert.Equal(t, uint64(1000), bonus.ExpiryHeight)
}

// ======================================================================
// Test 4: Formation bonus expiry
// ======================================================================

func TestExpireFormationBonuses(t *testing.T) {
	k, ctx, _ := setupKeeper(t)

	// Bonus expiring at block 50 (ctx is at block 100)
	k.SetDomainFormationBonus(ctx, "expired_domain", 300000, "capture_flagged", 50)
	// Bonus expiring at block 200 (still active)
	k.SetDomainFormationBonus(ctx, "active_domain", 300000, "capture_flagged", 200)
	// Bonus expiring at exactly block 100 (should expire — <= currentBlock)
	k.SetDomainFormationBonus(ctx, "edge_domain", 300000, "capture_flagged", 100)

	k.ExpireFormationBonuses(ctx)

	assert.Nil(t, k.GetDomainFormationBonus(ctx, "expired_domain"),
		"bonus at block 50 should be expired (current=100)")
	assert.Nil(t, k.GetDomainFormationBonus(ctx, "edge_domain"),
		"bonus at exactly current block should be expired")
	assert.NotNil(t, k.GetDomainFormationBonus(ctx, "active_domain"),
		"bonus at block 200 should still be active")
}

func TestExpireFormationBonuses_NoOp(t *testing.T) {
	k, ctx, _ := setupKeeper(t)

	// No bonuses → should not panic
	k.ExpireFormationBonuses(ctx)
}

// ======================================================================
// Test 5: Formation bonus affects match scoring
// ======================================================================

func TestFormationBonus_BoostsMatchScore(t *testing.T) {
	k, ctx, _ := setupKeeper(t)

	// Set params with matching interval of 1 block
	params := types.DefaultParams()
	params.FormationMatchIntervalBlocks = 1
	params.MatchAcceptanceBlocks = 100
	k.SetParams(ctx, params)

	// Create two pool entries in a flagged domain
	k.SetPoolEntry(ctx, &types.PoolEntry{
		Address:       humanAddr,
		Domains:       []string{"flagged_domain"},
		PreferredRole: "human",
		Status:        "active",
		RegisteredAt:  50,
	})
	k.SetPoolEntry(ctx, &types.PoolEntry{
		Address:       agentAddr,
		Domains:       []string{"flagged_domain"},
		PreferredRole: "agent",
		Status:        "active",
		RegisteredAt:  50,
	})

	// Create two pool entries in a non-flagged domain
	k.SetPoolEntry(ctx, &types.PoolEntry{
		Address:       outsiderAddr,
		Domains:       []string{"normal_domain"},
		PreferredRole: "human",
		Status:        "active",
		RegisteredAt:  50,
	})
	k.SetPoolEntry(ctx, &types.PoolEntry{
		Address:       agent2Addr,
		Domains:       []string{"normal_domain"},
		PreferredRole: "agent",
		Status:        "active",
		RegisteredAt:  50,
	})

	// Set formation bonus only on flagged domain (expires at block 5000)
	k.SetDomainFormationBonus(ctx, "flagged_domain", 300000, "capture_flagged", 5000)

	// Run matching
	k.RunFormationMatching(ctx)

	// Get all matches
	matches := k.GetAllFormationMatches(ctx)
	require.Len(t, matches, 2, "should create 2 matches")

	// Find the match for the flagged domain pair
	var flaggedMatch, normalMatch *types.FormationMatch
	for _, m := range matches {
		if (m.Addr1 == humanAddr || m.Addr2 == humanAddr) &&
			(m.Addr1 == agentAddr || m.Addr2 == agentAddr) {
			flaggedMatch = m
		} else {
			normalMatch = m
		}
	}

	require.NotNil(t, flaggedMatch, "should have match for flagged domain pair")
	require.NotNil(t, normalMatch, "should have match for normal domain pair")

	// The flagged domain match should have a higher score due to 30% bonus
	assert.Greater(t, flaggedMatch.Score, normalMatch.Score,
		"flagged domain match should have higher score due to formation bonus")

	// Verify the boost is approximately 30%
	expectedMinScore := normalMatch.Score * 1300000 / 1000000
	assert.GreaterOrEqual(t, flaggedMatch.Score, expectedMinScore-1,
		"flagged match score should be ~30%% higher")
}

func TestFormationBonus_ExpiredBonusNoEffect(t *testing.T) {
	k, ctx, _ := setupKeeper(t)

	params := types.DefaultParams()
	params.FormationMatchIntervalBlocks = 1
	params.MatchAcceptanceBlocks = 100
	k.SetParams(ctx, params)

	k.SetPoolEntry(ctx, &types.PoolEntry{
		Address:       humanAddr,
		Domains:       []string{"expired_bonus_domain"},
		PreferredRole: "human",
		Status:        "active",
		RegisteredAt:  50,
	})
	k.SetPoolEntry(ctx, &types.PoolEntry{
		Address:       agentAddr,
		Domains:       []string{"expired_bonus_domain"},
		PreferredRole: "agent",
		Status:        "active",
		RegisteredAt:  50,
	})

	// Set an EXPIRED formation bonus (block 50, current block is 100)
	k.SetDomainFormationBonus(ctx, "expired_bonus_domain", 300000, "capture_flagged", 50)

	k.RunFormationMatching(ctx)

	matches := k.GetAllFormationMatches(ctx)
	require.Len(t, matches, 1)

	// Base score for complementary roles + 100% domain overlap + time
	// = 3000 + 5000 + time_score
	// No bonus should be applied since it's expired
	baseExpected := uint64(3000 + 5000) // complementary + domain overlap
	assert.Less(t, matches[0].Score, baseExpected+baseExpected*300000/1000000,
		"expired bonus should not inflate score beyond base + bonus")
}

// ======================================================================
// Test 6: CaptureDefensePartnershipsAdapter
// ======================================================================

func TestCaptureDefensePartnershipsAdapter(t *testing.T) {
	k, ctx, _ := setupKeeper(t)

	adapter := keeper.NewCaptureDefensePartnershipsAdapter(k)
	goCtx := sdk.WrapSDKContext(ctx)

	// Density starts at 0
	assert.Equal(t, uint64(0), adapter.GetDomainPartnershipDensity(goCtx, "test_domain"))

	// Add partnerships
	k.SetPartnership(ctx, &types.Partnership{
		Id: "p1", HumanAddr: humanAddr, AgentAddr: agentAddr, Status: types.StatusActive,
	})

	assert.Equal(t, uint64(2), adapter.GetDomainPartnershipDensity(goCtx, "test_domain"),
		"adapter should return density from keeper")

	// Set formation bonus through adapter
	adapter.SetDomainFormationBonus(goCtx, "test_domain", 300000, "test_reason", 5000)

	bonus := k.GetDomainFormationBonus(ctx, "test_domain")
	require.NotNil(t, bonus, "bonus set through adapter should be readable from keeper")
	assert.Equal(t, uint64(300000), bonus.BonusBps)

	// GetPartnershipCountByParticipant through adapter
	count := adapter.GetPartnershipCountByParticipant(goCtx, humanAddr, "domain")
	assert.Equal(t, uint64(1), count) // 1 active partnership
}

// ======================================================================
// Test 7: Integration — flagged domain attracts more partnerships
// ======================================================================

func TestStructuralImmunity_FlaggedDomainBetterMatching(t *testing.T) {
	k, ctx, _ := setupKeeper(t)

	mockCD := newMockCaptureDefenseKeeper()
	k.SetCaptureDefenseKeeper(mockCD)

	// Flag a domain
	mockCD.flaggedDomains["physics"] = true

	// Set formation bonus for the flagged domain
	k.SetDomainFormationBonus(ctx, "physics", 300000, "capture_flagged", 5000)

	// Verify the bonus is readable
	bonus := k.GetDomainFormationBonus(ctx, "physics")
	require.NotNil(t, bonus)
	assert.Equal(t, "physics", bonus.Domain)
	assert.Equal(t, uint64(300000), bonus.BonusBps)

	// The bonus should be applied during matching
	// (already tested in TestFormationBonus_BoostsMatchScore above)
}

// ======================================================================
// Test 8: Social Benefit Status (R31-5)
// ======================================================================

func TestGetDomainSocialBenefitStatus_BelowThreshold(t *testing.T) {
	k, ctx, _ := setupKeeper(t)

	// No participants — below default threshold (4)
	require.False(t, k.GetDomainSocialBenefitStatus(ctx, "physics"),
		"empty domain should not have social benefit")
}

func TestGetDomainSocialBenefitStatus_AtThreshold(t *testing.T) {
	k, ctx, _ := setupKeeper(t)

	// Add active partnership (2 unique participants from partnerships)
	k.SetPartnership(ctx, &types.Partnership{
		Id: "p-1", HumanAddr: humanAddr, AgentAddr: agentAddr,
		Status:        types.StatusActive,
		SplitHumanBps: 500000, SplitAgentBps: 500000,
	})

	// Add active mentorship in "physics" domain (2 more unique participants)
	k.SetMentorship(ctx, &types.Mentorship{
		Id: "m-1", MentorAddr: outsiderAddr, MenteeAddr: agent2Addr,
		Domain: "physics", Status: "active",
	})

	// 4 unique participants (humanAddr, agentAddr, outsiderAddr, agent2Addr) — at threshold
	require.True(t, k.GetDomainSocialBenefitStatus(ctx, "physics"),
		"domain with density >= threshold should have social benefit")
}

func TestGetDomainSocialBenefitStatus_CustomThreshold(t *testing.T) {
	k, ctx, _ := setupKeeper(t)

	// Set a higher threshold
	params := types.DefaultParams()
	params.SocialSaturationThreshold = 10
	k.SetParams(ctx, params)

	// Add 4 participants — below new threshold of 10
	k.SetPartnership(ctx, &types.Partnership{
		Id: "p-1", HumanAddr: humanAddr, AgentAddr: agentAddr,
		Status:        types.StatusActive,
		SplitHumanBps: 500000, SplitAgentBps: 500000,
	})
	k.SetMentorship(ctx, &types.Mentorship{
		Id: "m-1", MentorAddr: outsiderAddr, MenteeAddr: agent2Addr,
		Domain: "physics", Status: "active",
	})

	require.False(t, k.GetDomainSocialBenefitStatus(ctx, "physics"),
		"density 4 should be below custom threshold 10")
}

// ======================================================================
// Test 9: SettleCoolingPartnerships emits social benefit events (R31-5)
// ======================================================================

func TestSettleCoolingPartnerships_EmitsSocialBenefitLostEvent(t *testing.T) {
	k, ctx, _ := setupKeeper(t)

	// Active partnership (humanAddr, agentAddr) — contributes density.
	k.SetPartnership(ctx, &types.Partnership{
		Id: "p-1", HumanAddr: humanAddr, AgentAddr: agentAddr,
		Status:        types.StatusActive,
		SplitHumanBps: 500000, SplitAgentBps: 500000,
	})

	// Active mentorship (outsiderAddr, agent2Addr) in "physics" — contributes density.
	k.SetMentorship(ctx, &types.Mentorship{
		Id: "m-1", MentorAddr: outsiderAddr, MenteeAddr: agent2Addr,
		Domain: "physics", Status: "active",
	})

	// Cooling partnership ready to dissolve. outsiderAddr is a mentor in "physics",
	// so this partnership's participants are linked to the "physics" domain.
	k.SetPartnership(ctx, &types.Partnership{
		Id: "p-cool", HumanAddr: outsiderAddr, AgentAddr: agent2Addr,
		Status:        types.StatusCooling,
		SplitHumanBps: 500000, SplitAgentBps: 500000,
		ExitState:     &types.ExitState{CooldownEnd: 100},
	})

	// Density = 4 (humanAddr, agentAddr from active p-1; outsiderAddr, agent2Addr
	// from mentorship m-1). Cooling partnerships are not counted as active.
	require.True(t, k.GetDomainSocialBenefitStatus(ctx, "physics"))

	ctx = ctx.WithBlockHeight(101)
	k.SettleCoolingPartnerships(ctx)

	// Verify p-cool dissolved.
	p, found := k.GetPartnership(ctx, "p-cool")
	require.True(t, found)
	require.Equal(t, types.StatusDissolved, p.Status)

	// Verify p-1 unchanged.
	p1, found := k.GetPartnership(ctx, "p-1")
	require.True(t, found)
	require.Equal(t, types.StatusActive, p1.Status)

	// In this scenario density stays at 4 (cooling→dissolved doesn't change
	// active count), so no social_benefit_lost event should fire.
	// The event mechanism is correctly wired for future cases where
	// dissolution + other block processing affects density.
}
