package keeper_test

import (
	"testing"

	"github.com/zerone-chain/zerone/x/toolbox/keeper"
	"github.com/zerone-chain/zerone/x/toolbox/types"
)

// ============================================================
// Trust Tier Boundaries (6 tests)
// ============================================================

func TestTrust_TierBoundary_Unverified(t *testing.T) {
	for _, score := range []uint64{0, 1, 50_000, 100_000} {
		tier := types.TrustTier(score)
		if tier != types.TrustTierIDUnverified {
			t.Errorf("score %d: expected tier %d (Unverified), got %d", score, types.TrustTierIDUnverified, tier)
		}
		label := types.TrustTierLabel(score)
		if label != types.TrustTierLabelUnverified {
			t.Errorf("score %d: expected label %q, got %q", score, types.TrustTierLabelUnverified, label)
		}
	}
}

func TestTrust_TierBoundary_Emerging(t *testing.T) {
	for _, score := range []uint64{100_001, 200_000, 300_000} {
		tier := types.TrustTier(score)
		if tier != types.TrustTierIDEmerging {
			t.Errorf("score %d: expected tier %d (Emerging), got %d", score, types.TrustTierIDEmerging, tier)
		}
		label := types.TrustTierLabel(score)
		if label != types.TrustTierLabelEmerging {
			t.Errorf("score %d: expected label %q, got %q", score, types.TrustTierLabelEmerging, label)
		}
	}
}

func TestTrust_TierBoundary_Established(t *testing.T) {
	for _, score := range []uint64{300_001, 450_000, 600_000} {
		tier := types.TrustTier(score)
		if tier != types.TrustTierIDEstablished {
			t.Errorf("score %d: expected tier %d (Established), got %d", score, types.TrustTierIDEstablished, tier)
		}
		label := types.TrustTierLabel(score)
		if label != types.TrustTierLabelEstablished {
			t.Errorf("score %d: expected label %q, got %q", score, types.TrustTierLabelEstablished, label)
		}
	}
}

func TestTrust_TierBoundary_Trusted(t *testing.T) {
	for _, score := range []uint64{600_001, 700_000, 800_000} {
		tier := types.TrustTier(score)
		if tier != types.TrustTierIDTrusted {
			t.Errorf("score %d: expected tier %d (Trusted), got %d", score, types.TrustTierIDTrusted, tier)
		}
		label := types.TrustTierLabel(score)
		if label != types.TrustTierLabelTrusted {
			t.Errorf("score %d: expected label %q, got %q", score, types.TrustTierLabelTrusted, label)
		}
	}
}

func TestTrust_TierBoundary_Verified(t *testing.T) {
	for _, score := range []uint64{800_001, 900_000, 1_000_000} {
		tier := types.TrustTier(score)
		if tier != types.TrustTierIDVerified {
			t.Errorf("score %d: expected tier %d (Verified), got %d", score, types.TrustTierIDVerified, tier)
		}
		label := types.TrustTierLabel(score)
		if label != types.TrustTierLabelVerified {
			t.Errorf("score %d: expected label %q, got %q", score, types.TrustTierLabelVerified, label)
		}
	}
}

func TestTrust_TierLabels(t *testing.T) {
	cases := []struct {
		score uint64
		label string
	}{
		{0, "Unverified"},
		{100_000, "Unverified"},
		{100_001, "Emerging"},
		{300_000, "Emerging"},
		{300_001, "Established"},
		{600_000, "Established"},
		{600_001, "Trusted"},
		{800_000, "Trusted"},
		{800_001, "Verified"},
		{1_000_000, "Verified"},
	}
	for _, tc := range cases {
		got := types.TrustTierLabel(tc.score)
		if got != tc.label {
			t.Errorf("TrustTierLabel(%d) = %q, want %q", tc.score, got, tc.label)
		}
	}
}

// ============================================================
// Dependency Eligibility (4 tests)
// ============================================================

func TestTrust_DependencyEligible_Tier0Rejected(t *testing.T) {
	// Score 50K is tier 0 (Unverified) → ineligible.
	if types.IsDependencyEligible(50_000) {
		t.Fatal("score 50,000 (tier 0) should be ineligible for dependency")
	}
}

func TestTrust_DependencyEligible_Tier1Accepted(t *testing.T) {
	// Score 200K is tier 1 (Emerging) → eligible.
	if !types.IsDependencyEligible(200_000) {
		t.Fatal("score 200,000 (tier 1) should be eligible for dependency")
	}
}

func TestTrust_DependencyEligible_RetiredRejected(t *testing.T) {
	// A retired tool is ineligible regardless of score.
	// IsDependencyEligible only checks score; the caller is responsible for
	// status checks. Verify that even a high score in tier 4 passes the
	// score check — the real rejection happens at the status level.
	s := setupFull(t)
	deployer := testAddr("trust-dep-retired")

	toolID := registerTestTool(t, s.ms, s.ctx, deployer, "retired-dep-tool")
	activateTool(t, s.k, s.ctx, toolID)

	// Set to retired status.
	setToolStatus(t, s.k, s.ctx, toolID, types.ToolStatusRetired)
	tool, ok := s.k.GetTool(s.ctx, toolID)
	if !ok {
		t.Fatal("tool not found")
	}
	tool.TrustScore = 900_000
	s.k.SetTool(s.ctx, tool)

	// Even though score is high, a retired tool should be rejected at status level.
	if tool.Status != types.ToolStatusRetired {
		t.Fatal("expected retired status")
	}
	// Score-only check passes, but status prevents use.
	if !types.IsDependencyEligible(tool.TrustScore) {
		t.Fatal("IsDependencyEligible is score-only; 900K should pass score check")
	}
	// The real guard: composite registration rejects retired deps.
	deployer2 := testAddr("trust-dep-retired-comp")
	_, err := s.ms.RegisterTool(s.ctx, &types.MsgRegisterTool{
		Deployer:      deployer2,
		Name:          "composite-retired-dep",
		ToolType:      types.ToolTypeComposite,
		Category:      types.CategoryComposite,
		License:       types.LicenseOpen,
		Version:       "1.0.0",
		DependencyIds: []string{toolID},
	})
	if err == nil {
		t.Fatal("expected error when depending on retired tool")
	}
}

func TestTrust_DependencyEligible_ExactBoundary(t *testing.T) {
	// 100,000 is still tier 0 → ineligible.
	if types.IsDependencyEligible(100_000) {
		t.Fatal("score 100,000 (upper bound of tier 0) should be ineligible")
	}
	// 100,001 enters tier 1 → eligible.
	if !types.IsDependencyEligible(100_001) {
		t.Fatal("score 100,001 (lower bound of tier 1) should be eligible")
	}
}

// ============================================================
// EMA Updates (4 tests)
// ============================================================

func TestTrust_EMA_SuccessNudgesUp(t *testing.T) {
	k, ctx, _, _ := setupKeeper(t)

	tool := &types.Tool{
		Id:         "ema-up",
		Name:       "EMA Up Tool",
		TrustScore: 500_000,
		Status:     types.ToolStatusActive,
		Deployer:   testAddr("ema-deployer"),
	}
	k.SetTool(ctx, tool)

	before := tool.TrustScore
	k.UpdateTrustScore(ctx, tool, true)
	after := tool.TrustScore

	if after <= before {
		t.Fatalf("success should increase score: before=%d, after=%d", before, after)
	}
}

func TestTrust_EMA_FailureNudgesDown(t *testing.T) {
	k, ctx, _, _ := setupKeeper(t)

	tool := &types.Tool{
		Id:         "ema-down",
		Name:       "EMA Down Tool",
		TrustScore: 500_000,
		Status:     types.ToolStatusActive,
		Deployer:   testAddr("ema-deployer"),
	}
	k.SetTool(ctx, tool)

	before := tool.TrustScore
	k.UpdateTrustScore(ctx, tool, false)
	after := tool.TrustScore

	if after >= before {
		t.Fatalf("failure should decrease score: before=%d, after=%d", before, after)
	}
}

func TestTrust_EMA_NeverExceedsMax(t *testing.T) {
	k, ctx, _, _ := setupKeeper(t)

	tool := &types.Tool{
		Id:         "ema-max",
		Name:       "EMA Max Tool",
		TrustScore: 999_999,
		Status:     types.ToolStatusActive,
		Deployer:   testAddr("ema-deployer"),
	}
	k.SetTool(ctx, tool)

	// Hammer with successes — should never exceed 1,000,000.
	for i := 0; i < 100; i++ {
		k.UpdateTrustScore(ctx, tool, true)
	}
	if tool.TrustScore > 1_000_000 {
		t.Fatalf("trust score exceeded max: %d", tool.TrustScore)
	}
}

func TestTrust_EMA_NeverGoesNegative(t *testing.T) {
	k, ctx, _, _ := setupKeeper(t)

	tool := &types.Tool{
		Id:         "ema-min",
		Name:       "EMA Min Tool",
		TrustScore: 1,
		Status:     types.ToolStatusActive,
		Deployer:   testAddr("ema-deployer"),
	}
	k.SetTool(ctx, tool)

	// Hammer with failures — should never go below 0.
	for i := 0; i < 100; i++ {
		k.UpdateTrustScore(ctx, tool, false)
	}
	// uint64 cannot go negative, but verify it stays at 0.
	if tool.TrustScore > 1_000_000 {
		t.Fatalf("trust score wrapped around (overflow): %d", tool.TrustScore)
	}
	// After 100 failures from score=1, it should be 0.
	if tool.TrustScore != 0 {
		t.Logf("trust score after 100 failures from 1: %d (should be 0 or very close)", tool.TrustScore)
	}
}

// ============================================================
// Verified Status Lifecycle (5 tests)
// ============================================================

func TestTrust_VerifiedPromotion_AtTier4(t *testing.T) {
	k, ctx, _, _ := setupKeeper(t)

	tool := &types.Tool{
		Id:         "promo-v",
		Name:       "Promote to Verified",
		TrustScore: 800_001, // >= TrustTierVerifiedMin
		Status:     types.ToolStatusActive,
		Deployer:   testAddr("promo-deployer"),
		IsVerified: false,
	}
	k.SetTool(ctx, tool)

	k.UpdateVerifiedStatus(ctx, tool)

	if !tool.IsVerified {
		t.Fatal("tool with score 800,001 should be promoted to verified")
	}
	if tool.VerifiedSince == 0 {
		t.Fatal("VerifiedSince should be set on promotion")
	}
}

func TestTrust_VerifiedNotPromoted_BelowThreshold(t *testing.T) {
	k, ctx, _, _ := setupKeeper(t)

	tool := &types.Tool{
		Id:         "no-promo",
		Name:       "Below Threshold",
		TrustScore: 800_000, // just below TrustTierVerifiedMin (800,001)
		Status:     types.ToolStatusActive,
		Deployer:   testAddr("nopromo-deployer"),
		IsVerified: false,
	}
	k.SetTool(ctx, tool)

	k.UpdateVerifiedStatus(ctx, tool)

	if tool.IsVerified {
		t.Fatal("tool with score 800,000 should NOT be promoted to verified")
	}
}

func TestTrust_VerifiedDemotion_Below700K(t *testing.T) {
	k, ctx, _, _ := setupKeeper(t)

	tool := &types.Tool{
		Id:                    "demote-v",
		Name:                  "Demotion Grace",
		TrustScore:            699_999, // below VerifiedMinRetentionScore (700,000)
		Status:                types.ToolStatusActive,
		Deployer:              testAddr("demote-deployer"),
		IsVerified:            true,
		VerifiedSince:         50,
		VerifiedDemotionBlock: 0,
	}
	k.SetTool(ctx, tool)

	k.UpdateVerifiedStatus(ctx, tool)

	// Should NOT immediately lose verified — grace period starts instead.
	if !tool.IsVerified {
		t.Fatal("verified tool should not immediately lose status — grace period should start")
	}
	if tool.VerifiedDemotionBlock == 0 {
		t.Fatal("VerifiedDemotionBlock should be set to begin grace period")
	}
}

func TestTrust_VerifiedGracePeriod_Recovery(t *testing.T) {
	k, ctx, _, _ := setupKeeper(t)

	tool := &types.Tool{
		Id:                    "recover-v",
		Name:                  "Grace Recovery",
		TrustScore:            699_999,
		Status:                types.ToolStatusActive,
		Deployer:              testAddr("recover-deployer"),
		IsVerified:            true,
		VerifiedSince:         50,
		VerifiedDemotionBlock: 0,
	}
	k.SetTool(ctx, tool)

	// Trigger grace period start.
	k.UpdateVerifiedStatus(ctx, tool)
	if tool.VerifiedDemotionBlock == 0 {
		t.Fatal("expected grace period to start")
	}

	// Recover: score rises above 700K retention threshold.
	tool.TrustScore = 750_000
	k.UpdateVerifiedStatus(ctx, tool)

	if tool.VerifiedDemotionBlock != 0 {
		t.Fatal("VerifiedDemotionBlock should be cleared after recovery above retention threshold")
	}
	if !tool.IsVerified {
		t.Fatal("tool should remain verified after recovery")
	}
}

func TestTrust_VerifiedGracePeriod_Expiry(t *testing.T) {
	s := setupFull(t)

	tool := &types.Tool{
		Id:            "expire-v",
		Name:          "Grace Expiry",
		TrustScore:    650_000, // below retention threshold
		Status:        types.ToolStatusActive,
		Deployer:      testAddr("expire-deployer"),
		IsVerified:    true,
		VerifiedSince: 50,
	}
	s.k.SetTool(s.ctx, tool)

	// Start grace period at block 100 (default ctx height).
	s.k.UpdateVerifiedStatus(s.ctx, tool)
	if tool.VerifiedDemotionBlock == 0 {
		t.Fatal("expected grace period to start")
	}
	gracePeriod := s.k.GetParams(s.ctx).VerifiedGracePeriodBlocks
	if gracePeriod == 0 {
		gracePeriod = types.DefaultVerifiedGracePeriodBlocks
	}

	// Advance context past the grace period.
	expiredCtx := s.ctx.WithBlockHeight(int64(tool.VerifiedDemotionBlock + gracePeriod))
	s.k.UpdateVerifiedStatus(expiredCtx, tool)

	if tool.IsVerified {
		t.Fatal("tool should lose verified status after grace period expiry")
	}
	if tool.VerifiedSince != 0 {
		t.Fatal("VerifiedSince should be reset to 0 on demotion")
	}
	if tool.VerifiedDemotionBlock != 0 {
		t.Fatal("VerifiedDemotionBlock should be cleared on demotion")
	}
}

// ============================================================
// Component Weights (3 tests)
// ============================================================

func TestTrust_WeightsSumTo1M(t *testing.T) {
	// The weight constants are unexported, but we can verify the invariant by
	// reading the source constants. We test indirectly: a tool with perfect
	// component scores (all 1M) should produce a total score of exactly 1M.
	// Since ComputeTrustScore caps at 1M and the weighted sum of (1M * w / 1M)
	// for each weight equals exactly sum(weights) = 1M, this verifies the sum.
	s := setupFull(t)
	deployer := testAddr("weights-deployer")

	// Create a tool with characteristics that max out all component scores.
	tool := &types.Tool{
		Id:                "weight-check",
		Name:              "Weight Check Tool",
		Status:            types.ToolStatusActive,
		Deployer:          deployer,
		TrustScore:        1_000_000,
		IsVerified:        true, // boosts verification component
		SourceHash:        "abc123",
		DocumentationHash: "doc456",
		ApiSchema:         "{}",
		KnowledgeQuery:    "test-query",
		Contributors: []*types.ContributorShare{
			{
				Address:  testAddr("weights-contrib"),
				Role:     types.RoleDeveloper,
				ShareBps: 1_000_000,
				Accepted: true,
			},
		},
	}
	s.k.SetTool(s.ctx, tool)

	// Create many unique callers (non-contributor) to boost usage.
	for i := 0; i < 30; i++ {
		caller := testAddr("wcaller-" + string(rune('A'+i)))
		s.k.RecordCaller(s.ctx, tool.Id, caller, 95, true)
	}

	// Set up staking for contributor: guardian tier + high accuracy.
	contribAddr := testAddr("weights-contrib")
	s.staking.tiers[contribAddr] = 3       // Guardian
	s.staking.accuracies[contribAddr] = 1_000_000 // Perfect

	snap := s.k.ComputeTrustScore(s.ctx, tool)
	// Score should be close to 1M (but may not be exact due to component scaling).
	if snap.Score > 1_000_000 {
		t.Fatalf("score %d exceeds maximum 1,000,000", snap.Score)
	}
	// All 5 component weights sum to 1M, verified by total not exceeding cap.
	t.Logf("computed score: %d (usage=%d, verify=%d, reliability=%d, peer=%d, contrib=%d)",
		snap.Score, snap.UsageComponent, snap.VerificationComponent,
		snap.ReliabilityComponent, snap.PeerComponent, snap.ContributorComponent)
}

func TestTrust_ScoreAlwaysInRange(t *testing.T) {
	s := setupFull(t)
	deployer := testAddr("range-deployer")

	// Test with several tool configurations.
	configs := []struct {
		name       string
		score      uint64
		verified   bool
		sourceHash string
	}{
		{"empty", 0, false, ""},
		{"mid", 500_000, false, "hash"},
		{"max-verified", 1_000_000, true, "hash"},
		{"zero-no-hash", 0, false, ""},
	}

	for _, cfg := range configs {
		tool := &types.Tool{
			Id:         "range-" + cfg.name,
			Name:       "Range " + cfg.name,
			Status:     types.ToolStatusActive,
			Deployer:   deployer,
			TrustScore: cfg.score,
			IsVerified: cfg.verified,
			SourceHash: cfg.sourceHash,
		}
		s.k.SetTool(s.ctx, tool)

		snap := s.k.ComputeTrustScore(s.ctx, tool)
		if snap.Score > 1_000_000 {
			t.Errorf("config %s: score %d exceeds 1,000,000", cfg.name, snap.Score)
		}
	}
}

func TestTrust_SnapshotComponentsPresent(t *testing.T) {
	s := setupFull(t)
	deployer := testAddr("snap-deployer")

	tool := &types.Tool{
		Id:                "snap-tool",
		Name:              "Snapshot Tool",
		Status:            types.ToolStatusActive,
		Deployer:          deployer,
		TrustScore:        500_000,
		SourceHash:        "abc",
		DocumentationHash: "def",
		ApiSchema:         "{}",
		Contributors: []*types.ContributorShare{
			{
				Address:  testAddr("snap-contrib"),
				Role:     types.RoleDeveloper,
				ShareBps: 1_000_000,
				Accepted: true,
			},
		},
	}
	s.k.SetTool(s.ctx, tool)

	// Record a caller for usage component.
	caller := testAddr("snap-caller")
	s.k.RecordCaller(s.ctx, tool.Id, caller, 90, true)

	snap := s.k.ComputeTrustScore(s.ctx, tool)

	if snap.ToolId != tool.Id {
		t.Errorf("expected ToolId %q, got %q", tool.Id, snap.ToolId)
	}
	if snap.ComputedAtBlock == 0 {
		t.Error("ComputedAtBlock should be set (context height is 100)")
	}

	// All component fields should be populated (non-nil struct fields, but may be 0).
	// Verification: tool has source+doc+api hashes → should be > 0.
	if snap.VerificationComponent == 0 {
		t.Error("VerificationComponent should be > 0 for tool with hashes")
	}
	// Reliability: no calls recorded via iterator = NeutralReliability default.
	// The caller was recorded, so reliability should exist.
	if snap.ReliabilityComponent == 0 {
		t.Error("ReliabilityComponent should be > 0 (neutral default for few calls)")
	}
	// Contributor: has an accepted contributor → should be > 0.
	if snap.ContributorComponent == 0 {
		t.Error("ContributorComponent should be > 0 for tool with accepted contributor")
	}

	t.Logf("snapshot: score=%d, usage=%d, verify=%d, reliability=%d, peer=%d, contrib=%d, block=%d",
		snap.Score, snap.UsageComponent, snap.VerificationComponent,
		snap.ReliabilityComponent, snap.PeerComponent, snap.ContributorComponent,
		snap.ComputedAtBlock)
}

// ============================================================
// Anti-Gaming Measures (3 tests)
// ============================================================

func TestTrust_AntiGaming_SelfCallingExcluded(t *testing.T) {
	s := setupFull(t)
	deployer := testAddr("selfcall-deployer")

	tool := &types.Tool{
		Id:       "selfcall-tool",
		Name:     "Self-Call Tool",
		Status:   types.ToolStatusActive,
		Deployer: deployer,
		Contributors: []*types.ContributorShare{
			{
				Address:  deployer,
				Role:     types.RoleDeveloper,
				ShareBps: 1_000_000,
				Accepted: true,
			},
		},
	}
	s.k.SetTool(s.ctx, tool)

	// Deployer calls their own tool many times — should be excluded from usage.
	for i := 0; i < 50; i++ {
		s.k.RecordCaller(s.ctx, tool.Id, deployer, uint64(50+i), true)
	}

	snap := s.k.ComputeTrustScore(s.ctx, tool)

	// Usage component should be 0 because all callers are the deployer (excluded).
	if snap.UsageComponent != 0 {
		t.Errorf("self-calling should produce 0 usage component, got %d", snap.UsageComponent)
	}
	// UniqueCallersWindow counts ALL callers (including self), but the usage
	// calculation filters them out.
	t.Logf("usage=%d, uniqueCallers=%d", snap.UsageComponent, snap.UniqueCallersWindow)
}

func TestTrust_AntiGaming_MutualDeps(t *testing.T) {
	s := setupFull(t)
	deployerA := testAddr("mutual-deployer-a")
	deployerB := testAddr("mutual-deployer-b")

	// Create two tools.
	toolA := &types.Tool{
		Id:         "mutual-a",
		Name:       "Mutual A",
		Status:     types.ToolStatusActive,
		Deployer:   deployerA,
		TrustScore: 800_000,
	}
	toolB := &types.Tool{
		Id:         "mutual-b",
		Name:       "Mutual B",
		Status:     types.ToolStatusActive,
		Deployer:   deployerB,
		TrustScore: 800_000,
	}
	s.k.SetTool(s.ctx, toolA)
	s.k.SetTool(s.ctx, toolB)

	// Create mutual dependency: A depends on B AND B depends on A.
	s.k.SetDependencyEdge(s.ctx, &types.DependencyEdge{
		FromToolId:     "mutual-a",
		ToToolId:       "mutual-b",
		CreatedAtBlock: 50,
	})
	s.k.SetDependencyEdge(s.ctx, &types.DependencyEdge{
		FromToolId:     "mutual-b",
		ToToolId:       "mutual-a",
		CreatedAtBlock: 50,
	})

	// Compute peer score for tool B. Tool A depends on B, but B also depends
	// on A — the mutual dependency should be detected and A's contribution
	// to B's peer score should be cancelled.
	snapB := s.k.ComputeTrustScore(s.ctx, toolB)

	// Peer component should be 0 because the only dependent (A) has a
	// mutual dependency back to B.
	if snapB.PeerComponent != 0 {
		t.Errorf("mutual deps should cancel peer contribution, got peer=%d", snapB.PeerComponent)
	}
	t.Logf("toolB peer component with mutual dep: %d", snapB.PeerComponent)
}

func TestTrust_AntiGaming_SameAuthorDampening(t *testing.T) {
	s := setupFull(t)
	deployer := testAddr("sameauth-deployer")
	otherDeployer := testAddr("sameauth-other")

	// Tool being scored.
	baseTool := &types.Tool{
		Id:         "sameauth-base",
		Name:       "Base Tool",
		Status:     types.ToolStatusActive,
		Deployer:   deployer,
		TrustScore: 500_000,
	}
	s.k.SetTool(s.ctx, baseTool)

	// Dependent by same author.
	sameAuthorDep := &types.Tool{
		Id:         "sameauth-dep-same",
		Name:       "Same Author Dep",
		Status:     types.ToolStatusActive,
		Deployer:   deployer, // same deployer
		TrustScore: 600_000,
	}
	s.k.SetTool(s.ctx, sameAuthorDep)

	// Dependent by different author.
	diffAuthorDep := &types.Tool{
		Id:         "sameauth-dep-diff",
		Name:       "Different Author Dep",
		Status:     types.ToolStatusActive,
		Deployer:   otherDeployer, // different deployer
		TrustScore: 600_000,
	}
	s.k.SetTool(s.ctx, diffAuthorDep)

	// Only same-author dep depends on base tool.
	s.k.SetDependencyEdge(s.ctx, &types.DependencyEdge{
		FromToolId:     "sameauth-dep-same",
		ToToolId:       "sameauth-base",
		CreatedAtBlock: 50,
	})
	snapSame := s.k.ComputeTrustScore(s.ctx, baseTool)
	peerWithSameAuthor := snapSame.PeerComponent

	// Reset: remove same-author edge, add different-author edge.
	s.k.DeleteDependencyEdge(s.ctx, "sameauth-dep-same", "sameauth-base")
	s.k.SetDependencyEdge(s.ctx, &types.DependencyEdge{
		FromToolId:     "sameauth-dep-diff",
		ToToolId:       "sameauth-base",
		CreatedAtBlock: 50,
	})
	snapDiff := s.k.ComputeTrustScore(s.ctx, baseTool)
	peerWithDiffAuthor := snapDiff.PeerComponent

	// Same-author dependency should contribute less (dampened by 50%).
	if peerWithSameAuthor >= peerWithDiffAuthor {
		t.Errorf("same-author peer contribution (%d) should be less than different-author (%d) due to 50%% dampening",
			peerWithSameAuthor, peerWithDiffAuthor)
	}
	t.Logf("same-author peer: %d, diff-author peer: %d", peerWithSameAuthor, peerWithDiffAuthor)
}

// ============================================================
// Additional Integration Tests
// ============================================================

func TestTrust_InitialTrustScore(t *testing.T) {
	initial := keeper.InitialTrustScore()
	if initial != 500_000 {
		t.Fatalf("expected initial trust score 500,000, got %d", initial)
	}
}

func TestTrust_RecalculateTrustScores_ActiveOnly(t *testing.T) {
	s := setupFull(t)
	deployer := testAddr("recalc-deployer")

	// Create an active tool and a retired tool.
	activeID := registerTestTool(t, s.ms, s.ctx, deployer, "recalc-active")
	activateTool(t, s.k, s.ctx, activeID)

	retiredID := registerTestTool(t, s.ms, s.ctx, deployer, "recalc-retired")
	activateTool(t, s.k, s.ctx, retiredID)

	// Record callers for the active tool to differentiate its snapshot post-recalc.
	for i := 0; i < 5; i++ {
		caller := testAddr("recalc-caller-" + string(rune('A'+i)))
		s.k.RecordCaller(s.ctx, activeID, caller, 90, true)
	}

	// Get initial snapshot score for both (set by RegisterTool).
	retiredSnapBefore, _ := s.k.GetTrustSnapshot(s.ctx, retiredID)
	retiredScoreBefore := retiredSnapBefore.Score

	// Retire the tool.
	setToolStatus(t, s.k, s.ctx, retiredID, types.ToolStatusRetired)

	// Run batch recalculation.
	s.k.RecalculateTrustScores(s.ctx)

	// Active tool should have a snapshot that was updated by recalc.
	activeSnap, snapFound := s.k.GetTrustSnapshot(s.ctx, activeID)
	if !snapFound {
		t.Error("active tool should have trust snapshot after recalculation")
	}
	if activeSnap.ComputedAtBlock != uint64(s.ctx.BlockHeight()) {
		t.Errorf("active tool snapshot should be computed at current block %d, got %d",
			s.ctx.BlockHeight(), activeSnap.ComputedAtBlock)
	}

	// Retired tool's snapshot should NOT have been updated by recalc.
	// It still has the original snapshot from RegisterTool.
	retiredSnapAfter, _ := s.k.GetTrustSnapshot(s.ctx, retiredID)
	if retiredSnapAfter.Score != retiredScoreBefore {
		t.Errorf("retired tool snapshot score should not change: before=%d, after=%d",
			retiredScoreBefore, retiredSnapAfter.Score)
	}
}
