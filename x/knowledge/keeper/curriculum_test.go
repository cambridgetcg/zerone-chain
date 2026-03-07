package keeper_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/zerone-chain/zerone/x/knowledge/keeper"
	"github.com/zerone-chain/zerone/x/knowledge/types"
)

// ─── Helpers ────────────────────────────────────────────────────────────────

func setupCurriculumTest(t *testing.T) (keeper.Keeper, context.Context) {
	t.Helper()
	k, ctx := setupKeeper(t)
	setupDefaultDomains(t, k, ctx)
	return k, ctx
}

func newTestCurriculum(domain string) *types.Curriculum {
	return &types.Curriculum{
		Name:    "Go Programming Curriculum",
		Domain:  domain,
		Creator: testAddr,
		Stages: []types.CurriculumStage{
			{
				StageID: "s0",
				Name:    "foundations",
				TDUIDs:  []string{"tdu-syntax", "tdu-types", "tdu-variables"},
			},
			{
				StageID:       "s1",
				Name:          "intermediate",
				TDUIDs:        []string{"tdu-goroutines", "tdu-channels", "tdu-interfaces"},
				Prerequisites: []string{"s0"},
			},
			{
				StageID:       "s2",
				Name:          "advanced",
				TDUIDs:        []string{"tdu-generics", "tdu-reflection", "tdu-cgo"},
				Prerequisites: []string{"s1"},
			},
		},
	}
}

// ─── Test: Create Curriculum — Happy Path ───────────────────────────────────

func TestCreateCurriculum_HappyPath(t *testing.T) {
	k, ctx := setupCurriculumTest(t)

	curriculum := newTestCurriculum("technology")
	cID, err := k.CreateCurriculum(ctx, curriculum)
	require.NoError(t, err)
	require.NotEmpty(t, cID)

	stored, found := k.GetCurriculum(ctx, cID)
	require.True(t, found)
	require.Equal(t, "Go Programming Curriculum", stored.Name)
	require.Equal(t, "technology", stored.Domain)
	require.Equal(t, types.CurriculumStatusDraft, stored.Status)
	require.Len(t, stored.Stages, 3)
	require.Equal(t, uint64(9), stored.TotalTDUs) // 3+3+3
	require.Equal(t, uint64(1), stored.Version)
}

// ─── Test: Create Curriculum — Empty Stages Rejected ────────────────────────

func TestCreateCurriculum_EmptyStages(t *testing.T) {
	k, ctx := setupCurriculumTest(t)

	_, err := k.CreateCurriculum(ctx, &types.Curriculum{
		Name:   "Empty",
		Domain: "technology",
		Stages: []types.CurriculumStage{},
	})
	require.ErrorIs(t, err, types.ErrCurriculumEmpty)
}

// ─── Test: Create Curriculum — Domain Not Found ─────────────────────────────

func TestCreateCurriculum_DomainNotFound(t *testing.T) {
	k, ctx := setupCurriculumTest(t)

	_, err := k.CreateCurriculum(ctx, &types.Curriculum{
		Name:   "Bad Domain",
		Domain: "nonexistent",
		Stages: []types.CurriculumStage{{Name: "s0", TDUIDs: []string{"tdu-1"}}},
	})
	require.ErrorIs(t, err, types.ErrDomainNotFound)
}

// ─── Test: Create Curriculum — Cycle Detection ──────────────────────────────

func TestCreateCurriculum_CycleDetection(t *testing.T) {
	k, ctx := setupCurriculumTest(t)

	// Stage s1 requires s2, but s2 comes after s1 — this is valid.
	// A cycle would be s0 requires s1 AND s1 requires s0.
	_, err := k.CreateCurriculum(ctx, &types.Curriculum{
		Name:   "Cycle",
		Domain: "technology",
		Stages: []types.CurriculumStage{
			{StageID: "s0", Name: "a", TDUIDs: []string{"tdu-1"}, Prerequisites: []string{"s1"}}, // s0 requires s1 (but s1 is AFTER s0 — cycle!)
			{StageID: "s1", Name: "b", TDUIDs: []string{"tdu-2"}},
		},
	})
	require.ErrorIs(t, err, types.ErrCurriculumCycle)
}

// ─── Test: Activate & Archive ───────────────────────────────────────────────

func TestActivateAndArchiveCurriculum(t *testing.T) {
	k, ctx := setupCurriculumTest(t)

	curriculum := newTestCurriculum("technology")
	cID, err := k.CreateCurriculum(ctx, curriculum)
	require.NoError(t, err)

	// Draft → Active.
	require.NoError(t, k.ActivateCurriculum(ctx, cID))
	stored, _ := k.GetCurriculum(ctx, cID)
	require.Equal(t, types.CurriculumStatusActive, stored.Status)

	// Active → Archived.
	require.NoError(t, k.ArchiveCurriculum(ctx, cID))
	stored, _ = k.GetCurriculum(ctx, cID)
	require.Equal(t, types.CurriculumStatusArchived, stored.Status)
}

// ─── Test: Enroll Agent — Happy Path ────────────────────────────────────────

func TestEnrollAgent_HappyPath(t *testing.T) {
	k, ctx := setupCurriculumTest(t)

	// Create agent.
	require.NoError(t, k.SetAgentIdentity(ctx, &types.AgentIdentity{
		AgentID: "agent-learner", Status: types.AgentStatusActive,
		Address: testAddr, CanSubmit: true, Reputation: "0.5", EarningsTotal: "0",
	}))

	// Create and activate curriculum.
	curriculum := newTestCurriculum("technology")
	cID, _ := k.CreateCurriculum(ctx, curriculum)
	require.NoError(t, k.ActivateCurriculum(ctx, cID))

	// Enroll.
	eID, err := k.EnrollAgent(ctx, cID, "agent-learner")
	require.NoError(t, err)
	require.NotEmpty(t, eID)

	enrollment, found := k.GetEnrollment(ctx, eID)
	require.True(t, found)
	require.Equal(t, cID, enrollment.CurriculumID)
	require.Equal(t, "agent-learner", enrollment.AgentID)
	require.Equal(t, uint64(0), enrollment.CurrentStage)
	require.Equal(t, "active", enrollment.Status)
}

// ─── Test: Enroll Agent — Not Active Curriculum ─────────────────────────────

func TestEnrollAgent_NotActiveCurriculum(t *testing.T) {
	k, ctx := setupCurriculumTest(t)

	require.NoError(t, k.SetAgentIdentity(ctx, &types.AgentIdentity{
		AgentID: "agent-eager", Status: types.AgentStatusActive,
		Address: testAddr, CanSubmit: true, Reputation: "0.5", EarningsTotal: "0",
	}))

	curriculum := newTestCurriculum("technology")
	cID, _ := k.CreateCurriculum(ctx, curriculum) // still draft

	_, err := k.EnrollAgent(ctx, cID, "agent-eager")
	require.ErrorIs(t, err, types.ErrCurriculumNotActive)
}

// ─── Test: Enroll Agent — Duplicate Rejected ────────────────────────────────

func TestEnrollAgent_DuplicateRejected(t *testing.T) {
	k, ctx := setupCurriculumTest(t)

	require.NoError(t, k.SetAgentIdentity(ctx, &types.AgentIdentity{
		AgentID: "agent-dup", Status: types.AgentStatusActive,
		Address: testAddr, CanSubmit: true, Reputation: "0.5", EarningsTotal: "0",
	}))

	curriculum := newTestCurriculum("technology")
	cID, _ := k.CreateCurriculum(ctx, curriculum)
	_ = k.ActivateCurriculum(ctx, cID)

	_, err := k.EnrollAgent(ctx, cID, "agent-dup")
	require.NoError(t, err)

	_, err = k.EnrollAgent(ctx, cID, "agent-dup")
	require.ErrorIs(t, err, types.ErrAlreadyEnrolled)
}

// ─── Test: Advance Stage — Full Progression ─────────────────────────────────

func TestAdvanceStage_FullProgression(t *testing.T) {
	k, ctx := setupCurriculumTest(t)

	require.NoError(t, k.SetAgentIdentity(ctx, &types.AgentIdentity{
		AgentID: "agent-pro", Status: types.AgentStatusActive,
		Address: testAddr, CanSubmit: true, Reputation: "0.5", EarningsTotal: "0",
	}))

	curriculum := newTestCurriculum("technology")
	cID, _ := k.CreateCurriculum(ctx, curriculum)
	_ = k.ActivateCurriculum(ctx, cID)
	eID, _ := k.EnrollAgent(ctx, cID, "agent-pro")

	// Stage 0: foundations.
	err := k.AdvanceStage(ctx, eID, []string{"tdu-syntax", "tdu-types", "tdu-variables"})
	require.NoError(t, err)

	e, _ := k.GetEnrollment(ctx, eID)
	require.Equal(t, uint64(1), e.CurrentStage)
	require.Contains(t, e.CompletedStages, "s0")
	require.Equal(t, "active", e.Status)

	// Stage 1: intermediate.
	err = k.AdvanceStage(ctx, eID, []string{"tdu-goroutines", "tdu-channels", "tdu-interfaces"})
	require.NoError(t, err)

	e, _ = k.GetEnrollment(ctx, eID)
	require.Equal(t, uint64(2), e.CurrentStage)
	require.Contains(t, e.CompletedStages, "s1")

	// Stage 2: advanced → completes curriculum.
	err = k.AdvanceStage(ctx, eID, []string{"tdu-generics", "tdu-reflection", "tdu-cgo"})
	require.NoError(t, err)

	e, _ = k.GetEnrollment(ctx, eID)
	require.Equal(t, "completed", e.Status)
	require.Equal(t, uint64(9), e.TotalConsumed)
}

// ─── Test: Advance Stage — Prerequisite Not Met ─────────────────────────────

func TestAdvanceStage_PrereqNotMet(t *testing.T) {
	k, ctx := setupCurriculumTest(t)

	require.NoError(t, k.SetAgentIdentity(ctx, &types.AgentIdentity{
		AgentID: "agent-skip", Status: types.AgentStatusActive,
		Address: testAddr, CanSubmit: true, Reputation: "0.5", EarningsTotal: "0",
	}))

	// Curriculum where stage 1 requires stage 0.
	curriculum := newTestCurriculum("technology")
	cID, _ := k.CreateCurriculum(ctx, curriculum)
	_ = k.ActivateCurriculum(ctx, cID)
	eID, _ := k.EnrollAgent(ctx, cID, "agent-skip")

	// Manually advance to stage 1 without completing stage 0.
	e, _ := k.GetEnrollment(ctx, eID)
	e.CurrentStage = 1 // skip stage 0
	k.SetEnrollment(ctx, e)

	// Try to advance stage 1 — should fail because s0 not completed.
	err := k.AdvanceStage(ctx, eID, []string{"tdu-goroutines"})
	require.ErrorIs(t, err, types.ErrStagePrereqNotMet)
}

// ─── Test: Get Next TDUs ────────────────────────────────────────────────────

func TestGetNextTDUs(t *testing.T) {
	k, ctx := setupCurriculumTest(t)

	require.NoError(t, k.SetAgentIdentity(ctx, &types.AgentIdentity{
		AgentID: "agent-next", Status: types.AgentStatusActive,
		Address: testAddr, CanSubmit: true, Reputation: "0.5", EarningsTotal: "0",
	}))

	curriculum := newTestCurriculum("technology")
	cID, _ := k.CreateCurriculum(ctx, curriculum)
	_ = k.ActivateCurriculum(ctx, cID)
	eID, _ := k.EnrollAgent(ctx, cID, "agent-next")

	// First call: should return stage 0 TDUs.
	tdus, stageID, err := k.GetNextTDUs(ctx, eID)
	require.NoError(t, err)
	require.Equal(t, "s0", stageID)
	require.Len(t, tdus, 3)
	require.Contains(t, tdus, "tdu-syntax")
}

func TestGetNextTDUs_ExcludesConsumed(t *testing.T) {
	k, ctx := setupCurriculumTest(t)

	require.NoError(t, k.SetAgentIdentity(ctx, &types.AgentIdentity{
		AgentID: "agent-partial", Status: types.AgentStatusActive,
		Address: testAddr, CanSubmit: true, Reputation: "0.5", EarningsTotal: "0",
	}))

	curriculum := newTestCurriculum("technology")
	cID, _ := k.CreateCurriculum(ctx, curriculum)
	_ = k.ActivateCurriculum(ctx, cID)
	eID, _ := k.EnrollAgent(ctx, cID, "agent-partial")

	// Mark one TDU as consumed.
	e, _ := k.GetEnrollment(ctx, eID)
	e.CompletedTDUs = []string{"tdu-syntax"}
	k.SetEnrollment(ctx, e)

	tdus, _, err := k.GetNextTDUs(ctx, eID)
	require.NoError(t, err)
	require.Len(t, tdus, 2) // 3 minus 1 consumed
	require.NotContains(t, tdus, "tdu-syntax")
}

// ─── Test: Build Curriculum from Graph ──────────────────────────────────────

func TestBuildCurriculumFromGraph(t *testing.T) {
	k, ctx := setupCurriculumTest(t)

	// Create prerequisite edges: syntax → goroutines → generics
	_, _ = k.CreateEdge(ctx, &types.MsgCreateEdge{
		Creator:  testAddr,
		SourceID: "tdu-syntax",
		TargetID: "tdu-goroutines",
		EdgeType: types.EdgeTypePrerequisite,
	})
	_, _ = k.CreateEdge(ctx, &types.MsgCreateEdge{
		Creator:  testAddr,
		SourceID: "tdu-goroutines",
		TargetID: "tdu-generics",
		EdgeType: types.EdgeTypePrerequisite,
	})

	// Also tdu-types has no prerequisites.
	tduIDs := []string{"tdu-syntax", "tdu-goroutines", "tdu-generics", "tdu-types"}

	curriculum, err := k.BuildCurriculumFromGraph(ctx, "technology", "Auto Go Curriculum", testAddr, tduIDs)
	require.NoError(t, err)
	require.NotNil(t, curriculum)

	// Should have 3 stages:
	// Level 0: tdu-syntax, tdu-types (no prereqs)
	// Level 1: tdu-goroutines (requires syntax)
	// Level 2: tdu-generics (requires goroutines)
	require.Len(t, curriculum.Stages, 3)
	require.Equal(t, "foundations", curriculum.Stages[0].Name)
	require.Contains(t, curriculum.Stages[0].TDUIDs, "tdu-syntax")
	require.Contains(t, curriculum.Stages[0].TDUIDs, "tdu-types")
	require.Equal(t, "basics", curriculum.Stages[1].Name)
	require.Contains(t, curriculum.Stages[1].TDUIDs, "tdu-goroutines")
	require.Equal(t, "intermediate", curriculum.Stages[2].Name)
	require.Contains(t, curriculum.Stages[2].TDUIDs, "tdu-generics")
}

func TestBuildCurriculumFromGraph_CycleDetected(t *testing.T) {
	k, ctx := setupCurriculumTest(t)

	// Create a cycle: A → B → A
	_, _ = k.CreateEdge(ctx, &types.MsgCreateEdge{
		Creator: testAddr, SourceID: "tdu-a", TargetID: "tdu-b", EdgeType: types.EdgeTypePrerequisite,
	})
	_, _ = k.CreateEdge(ctx, &types.MsgCreateEdge{
		Creator: testAddr, SourceID: "tdu-b", TargetID: "tdu-a", EdgeType: types.EdgeTypePrerequisite,
	})

	_, err := k.BuildCurriculumFromGraph(ctx, "technology", "Cycle", testAddr, []string{"tdu-a", "tdu-b"})
	require.ErrorIs(t, err, types.ErrCurriculumCycle)
}

// ─── Test: Curricula by Domain ──────────────────────────────────────────────

func TestGetCurriculaByDomain(t *testing.T) {
	k, ctx := setupCurriculumTest(t)

	c1 := newTestCurriculum("technology")
	c1.Name = "Go Basics"
	k.CreateCurriculum(ctx, c1)

	c2 := newTestCurriculum("technology")
	c2.Name = "Go Advanced"
	k.CreateCurriculum(ctx, c2)

	c3 := newTestCurriculum("science")
	c3.Name = "Physics Intro"
	k.CreateCurriculum(ctx, c3)

	techCurricula := k.GetCurriculaByDomain(ctx, "technology")
	require.Len(t, techCurricula, 2)

	sciCurricula := k.GetCurriculaByDomain(ctx, "science")
	require.Len(t, sciCurricula, 1)
}

// ─── Test: Curriculum Enrollments Query ─────────────────────────────────────

func TestGetCurriculumEnrollments(t *testing.T) {
	k, ctx := setupCurriculumTest(t)

	for _, id := range []string{"agent-1", "agent-2", "agent-3"} {
		require.NoError(t, k.SetAgentIdentity(ctx, &types.AgentIdentity{
			AgentID: id, Status: types.AgentStatusActive,
			Address: testAddr, CanSubmit: true, Reputation: "0.5", EarningsTotal: "0",
		}))
	}

	curriculum := newTestCurriculum("technology")
	cID, _ := k.CreateCurriculum(ctx, curriculum)
	_ = k.ActivateCurriculum(ctx, cID)

	for _, id := range []string{"agent-1", "agent-2", "agent-3"} {
		_, err := k.EnrollAgent(ctx, cID, id)
		require.NoError(t, err)
	}

	enrollments := k.GetCurriculumEnrollments(ctx, cID)
	require.Len(t, enrollments, 3)
}

// ─── Test: Full Curriculum Lifecycle ────────────────────────────────────────

func TestCurriculumFullLifecycle(t *testing.T) {
	k, ctx := setupCurriculumTest(t)

	// 1. Create agent.
	require.NoError(t, k.SetAgentIdentity(ctx, &types.AgentIdentity{
		AgentID: "agent-full", Status: types.AgentStatusActive,
		Address: testAddr, CanSubmit: true, CanReview: true,
		Reputation: "0.5", EarningsTotal: "0",
	}))

	// 2. Build curriculum from graph.
	_, _ = k.CreateEdge(ctx, &types.MsgCreateEdge{
		Creator: testAddr, SourceID: "tdu-basics", TargetID: "tdu-advanced",
		EdgeType: types.EdgeTypePrerequisite,
	})

	curriculum, err := k.BuildCurriculumFromGraph(ctx, "technology", "Full Test", testAddr, []string{"tdu-basics", "tdu-advanced", "tdu-standalone"})
	require.NoError(t, err)

	// 3. Create and activate.
	cID, err := k.CreateCurriculum(ctx, curriculum)
	require.NoError(t, err)
	require.NoError(t, k.ActivateCurriculum(ctx, cID))

	// 4. Enroll agent.
	eID, err := k.EnrollAgent(ctx, cID, "agent-full")
	require.NoError(t, err)

	// 5. Get next TDUs (stage 0: foundations).
	tdus, stageID, err := k.GetNextTDUs(ctx, eID)
	require.NoError(t, err)
	require.NotEmpty(t, tdus)
	require.NotEmpty(t, stageID)

	// 6. Complete stage 0.
	err = k.AdvanceStage(ctx, eID, tdus)
	require.NoError(t, err)

	// 7. Get next TDUs (stage 1).
	tdus2, _, err := k.GetNextTDUs(ctx, eID)
	require.NoError(t, err)
	require.NotEmpty(t, tdus2)

	// 8. Complete stage 1 → curriculum complete.
	err = k.AdvanceStage(ctx, eID, tdus2)
	require.NoError(t, err)

	e, _ := k.GetEnrollment(ctx, eID)
	require.Equal(t, "completed", e.Status)

	// 9. Archive the curriculum.
	require.NoError(t, k.ArchiveCurriculum(ctx, cID))
	stored, _ := k.GetCurriculum(ctx, cID)
	require.Equal(t, types.CurriculumStatusArchived, stored.Status)
}
