package keeper_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/zerone-chain/zerone/x/partnerships/types"
)

// TestR31_5_WaterFlowIntegration exercises all three Wu Xing relationships:
//  1. Water → Wood: Mentorship graduation produces knowledge dividends
//  2. Earth → Water: Param changes reset formation matching cycle
//  3. Water → Fire: Social density events (social benefit status)
func TestR31_5_WaterFlowIntegration(t *testing.T) {
	k, ctx, _ := setupKeeper(t)
	// setupKeeper creates context at block 100 and calls SetParams (LastParamUpdateHeight = 100).

	mock := &mockKnowledgeKeeper{}
	k.SetKnowledgeKeeper(mock)

	// Override params with a shorter formation interval for testing.
	params := types.DefaultParams()
	params.FormationMatchIntervalBlocks = 10
	params.SocialSaturationThreshold = 3 // lowered so we can reach it
	ctx = ctx.WithBlockHeight(100)
	k.SetParams(ctx, params) // LastParamUpdateHeight = 100

	// Create an active partnership to start building social density.
	k.SetPartnership(ctx, &types.Partnership{
		Id: "p-1", HumanAddr: "h1", AgentAddr: "a1",
		Status:        types.StatusActive,
		SplitHumanBps: 500000, SplitAgentBps: 500000,
	})

	// ---------------------------------------------------------------
	// Flow 1: Water → Wood (Mentorship Dividend)
	// ---------------------------------------------------------------
	k.SetMentorship(ctx, &types.Mentorship{
		Id: "m-1", MentorAddr: "h2", MenteeAddr: "a2",
		Domain: "physics", Status: "active",
		StartBlock: 100, DurationBlocks: 50,
	})

	// At block 150: currentBlock (150) >= StartBlock (100) + DurationBlocks (50) → graduates.
	ctx = ctx.WithBlockHeight(150)
	k.AutoGraduateMentorships(ctx)

	require.Equal(t, 1, mock.callCount, "Water→Wood: dividend should be called on graduation")
	require.Equal(t, "physics", mock.lastDomain, "Water→Wood: dividend domain must match")
	require.Equal(t, "h2", mock.lastMentor, "Water→Wood: mentor addr must match")
	require.Equal(t, "a2", mock.lastMentee, "Water→Wood: mentee addr must match")

	m, found := k.GetMentorship(ctx, "m-1")
	require.True(t, found)
	require.Equal(t, "graduated", m.Status, "Water→Wood: mentorship status must be graduated")

	// ---------------------------------------------------------------
	// Flow 2: Earth → Water (Param Change Resets Matching Cycle)
	// ---------------------------------------------------------------

	// Seed the formation pool with two compatible entries.
	k.SetPoolEntry(ctx, &types.PoolEntry{
		Address: "pool1", Status: "active",
		Domains: []string{"physics"}, RegisteredAt: 100,
	})
	k.SetPoolEntry(ctx, &types.PoolEntry{
		Address: "pool2", Status: "active",
		Domains: []string{"physics"}, PreferredRole: "agent",
		RegisteredAt: 100,
	})

	// Block 160: (160 - 100) % 10 == 0 → matching runs.
	ctx = ctx.WithBlockHeight(160)
	k.RunFormationMatching(ctx)
	matches := k.GetAllFormationMatches(ctx)
	require.Len(t, matches, 1, "Earth→Water: first match should form at block 160")

	// Seed two more entries for the second match round.
	k.SetPoolEntry(ctx, &types.PoolEntry{
		Address: "pool3", Status: "active",
		Domains: []string{"physics"}, RegisteredAt: 100,
	})
	k.SetPoolEntry(ctx, &types.PoolEntry{
		Address: "pool4", Status: "active",
		Domains: []string{"physics"}, PreferredRole: "agent",
		RegisteredAt: 100,
	})

	// Update params at block 165 — resets cycle base.
	ctx = ctx.WithBlockHeight(165)
	k.SetParams(ctx, params) // LastParamUpdateHeight = 165

	// Block 170: (170 - 165) = 5, 5 % 10 != 0 → should NOT run.
	ctx = ctx.WithBlockHeight(170)
	k.RunFormationMatching(ctx)
	require.Len(t, k.GetAllFormationMatches(ctx), 1,
		"Earth→Water: matching should NOT run at block 170 (cycle reset)")

	// Block 175: (175 - 165) = 10, 10 % 10 == 0 → should run.
	ctx = ctx.WithBlockHeight(175)
	k.RunFormationMatching(ctx)
	require.Len(t, k.GetAllFormationMatches(ctx), 2,
		"Earth→Water: matching should run at block 175 after reset cycle")

	// ---------------------------------------------------------------
	// Flow 3: Water → Fire (Social Benefit Status)
	// ---------------------------------------------------------------
	// At this point we have:
	//   - Partnership p-1: h1, a1 (2 unique participants)
	//   - Graduated mentorship m-1: h2, a2 (graduated, not counted)
	// The graduated mentorship has status "graduated" (not "active"), so
	// only partnership participants count. We need >= 3 unique participants.
	// Add a second partnership to push density to 4 (h1, a1, h3, a3).
	k.SetPartnership(ctx, &types.Partnership{
		Id: "p-2", HumanAddr: "h3", AgentAddr: "a3",
		Status:        types.StatusActive,
		SplitHumanBps: 500000, SplitAgentBps: 500000,
	})

	hasBenefit := k.GetDomainSocialBenefitStatus(ctx, "physics")
	require.True(t, hasBenefit,
		"Water→Fire: social benefit should be active with density >= threshold")

	t.Logf("Water Flow integration complete — all three Wu Xing relationships verified")
}
