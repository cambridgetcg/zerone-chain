package keeper_test

import (
	"fmt"
	"testing"

	"github.com/zerone-chain/zerone/x/toolbox/types"
)

// ============================================================
// Trust Gaming — Self-Call Exclusion (1 test)
// ============================================================

func TestToolSecurity_SelfCallingExcluded(t *testing.T) {
	s := setupFull(t)
	deployer := testAddr("sec-selfcall-deployer")
	contrib := testAddr("sec-selfcall-contrib")

	tool := &types.Tool{
		Id:       "sec-selfcall",
		Name:     "Self-Call Security Tool",
		Status:   types.ToolStatusActive,
		Deployer: deployer,
		Contributors: []*types.ContributorShare{
			{Address: deployer, Role: types.RoleDeveloper, ShareBps: 700_000, Accepted: true, TotalEarned: "0"},
			{Address: contrib, Role: types.RoleTester, ShareBps: 300_000, Accepted: true, TotalEarned: "0"},
		},
	}
	s.k.SetTool(s.ctx, tool)

	// Both deployer and contributor call the tool extensively.
	for i := 0; i < 50; i++ {
		s.k.RecordCaller(s.ctx, tool.Id, deployer, uint64(50+i), true)
		s.k.RecordCaller(s.ctx, tool.Id, contrib, uint64(50+i), true)
	}

	snap := s.k.ComputeTrustScore(s.ctx, tool)

	// Usage component should be 0 because ALL callers are contributors/deployer.
	if snap.UsageComponent != 0 {
		t.Errorf("self-calling (deployer+contrib) should produce 0 usage, got %d", snap.UsageComponent)
	}
}

// ============================================================
// Trust Gaming — Mutual Dependencies (1 test)
// ============================================================

func TestToolSecurity_MutualDepsDampened(t *testing.T) {
	s := setupFull(t)
	deployerA := testAddr("sec-mutual-a")
	deployerB := testAddr("sec-mutual-b")

	toolA := &types.Tool{
		Id: "sec-mutual-tool-a", Name: "Mutual A", Status: types.ToolStatusActive,
		Deployer: deployerA, TrustScore: 750_000,
	}
	toolB := &types.Tool{
		Id: "sec-mutual-tool-b", Name: "Mutual B", Status: types.ToolStatusActive,
		Deployer: deployerB, TrustScore: 750_000,
	}
	s.k.SetTool(s.ctx, toolA)
	s.k.SetTool(s.ctx, toolB)

	// Create mutual dependency: A -> B and B -> A.
	s.k.SetDependencyEdge(s.ctx, &types.DependencyEdge{
		FromToolId: "sec-mutual-tool-a", ToToolId: "sec-mutual-tool-b", CreatedAtBlock: 50,
	})
	s.k.SetDependencyEdge(s.ctx, &types.DependencyEdge{
		FromToolId: "sec-mutual-tool-b", ToToolId: "sec-mutual-tool-a", CreatedAtBlock: 50,
	})

	// Mutual deps should cancel each other's peer contribution.
	snapA := s.k.ComputeTrustScore(s.ctx, toolA)
	snapB := s.k.ComputeTrustScore(s.ctx, toolB)

	if snapA.PeerComponent != 0 {
		t.Errorf("mutual deps should cancel: toolA peer=%d", snapA.PeerComponent)
	}
	if snapB.PeerComponent != 0 {
		t.Errorf("mutual deps should cancel: toolB peer=%d", snapB.PeerComponent)
	}
}

// ============================================================
// Trust Gaming — Same Author Dampening (1 test)
// ============================================================

func TestToolSecurity_SameAuthorDampened(t *testing.T) {
	s := setupFull(t)
	deployer := testAddr("sec-sameauth")
	otherDeployer := testAddr("sec-otherauth")

	baseTool := &types.Tool{
		Id: "sec-sameauth-base", Name: "Base", Status: types.ToolStatusActive,
		Deployer: deployer, TrustScore: 500_000,
	}
	s.k.SetTool(s.ctx, baseTool)

	// Scenario 1: same-author dependent.
	sameAuthorTool := &types.Tool{
		Id: "sec-sameauth-dep", Name: "Same Author Dep", Status: types.ToolStatusActive,
		Deployer: deployer, TrustScore: 700_000,
	}
	s.k.SetTool(s.ctx, sameAuthorTool)
	s.k.SetDependencyEdge(s.ctx, &types.DependencyEdge{
		FromToolId: "sec-sameauth-dep", ToToolId: "sec-sameauth-base", CreatedAtBlock: 50,
	})

	snapSame := s.k.ComputeTrustScore(s.ctx, baseTool)
	peerSame := snapSame.PeerComponent

	// Remove and replace with different-author dependent.
	s.k.DeleteDependencyEdge(s.ctx, "sec-sameauth-dep", "sec-sameauth-base")
	diffAuthorTool := &types.Tool{
		Id: "sec-diffauth-dep", Name: "Diff Author Dep", Status: types.ToolStatusActive,
		Deployer: otherDeployer, TrustScore: 700_000,
	}
	s.k.SetTool(s.ctx, diffAuthorTool)
	s.k.SetDependencyEdge(s.ctx, &types.DependencyEdge{
		FromToolId: "sec-diffauth-dep", ToToolId: "sec-sameauth-base", CreatedAtBlock: 50,
	})

	snapDiff := s.k.ComputeTrustScore(s.ctx, baseTool)
	peerDiff := snapDiff.PeerComponent

	if peerSame >= peerDiff {
		t.Errorf("same-author peer (%d) should be < different-author peer (%d) due to 50%% dampening",
			peerSame, peerDiff)
	}
}

// ============================================================
// Fee Manipulation (2 tests)
// ============================================================

func TestToolSecurity_FeeManipulation_MaxFeeEnforced(t *testing.T) {
	k, ctx, _, _ := setupKeeper(t)
	caller := testAddr("sec-maxfee-caller")

	tool := &types.Tool{
		Id: "sec-maxfee-tool", PricePerCall: "100000",
		Category: types.CategoryDataAnalysis, Status: types.ToolStatusActive,
		Deployer: testAddr("sec-maxfee-deployer"), TotalRevenue: "0", TotalCalls: "0",
	}
	k.SetTool(ctx, tool)

	// maxFee (50000) < price (100000) -> rejected.
	_, _, err := k.CollectPayment(ctx, caller, tool, 50_000)
	if err == nil {
		t.Fatal("expected ErrFeeTooHigh when maxFee < effective price")
	}
}

func TestToolSecurity_FeeManipulation_InsufficientFunds(t *testing.T) {
	k, ctx, bk, _ := setupKeeper(t)
	caller := testAddr("sec-broke-caller")

	tool := &types.Tool{
		Id: "sec-broke-tool", PricePerCall: "100000",
		Category: types.CategoryDataAnalysis, Status: types.ToolStatusActive,
		Deployer: testAddr("sec-broke-deployer"), TotalRevenue: "0", TotalCalls: "0",
	}
	k.SetTool(ctx, tool)

	bk.failSend = true

	_, _, err := k.CollectPayment(ctx, caller, tool, 0)
	if err == nil {
		t.Fatal("expected error for insufficient funds")
	}

	// No partial distribution should have occurred.
	if len(bk.modToAccSends) != 0 {
		t.Error("no revenue distribution should occur when payment collection fails")
	}
}

// ============================================================
// Revenue Theft — Unauthorized Operations (2 tests)
// ============================================================

func TestToolSecurity_RevenueTheft_NonDeployerCantRetire(t *testing.T) {
	ms, k, ctx, _, _ := setupMsgServer(t)
	deployer := testAddr("sec-retire-deployer")
	attacker := testAddr("sec-retire-attacker")

	toolID := registerTestTool(t, ms, ctx, deployer, "sec-retire-tool")
	activateTool(t, k, ctx, toolID)

	// Attacker (not deployer) tries to retire.
	_, err := ms.RetireTool(ctx, &types.MsgRetireTool{
		Authority: attacker,
		ToolId:    toolID,
	})
	if err == nil {
		t.Fatal("non-deployer should not be able to retire a tool")
	}

	// Verify tool is still active.
	tool, ok := k.GetTool(ctx, toolID)
	if !ok {
		t.Fatal("tool should still exist")
	}
	if tool.Status != types.ToolStatusActive {
		t.Errorf("tool should still be active, got %s", tool.Status)
	}
}

func TestToolSecurity_RevenueTheft_NonDeployerCantDeprecate(t *testing.T) {
	ms, k, ctx, _, _ := setupMsgServer(t)
	deployer := testAddr("sec-deprecate-deployer")
	attacker := testAddr("sec-deprecate-attacker")

	toolID := registerTestTool(t, ms, ctx, deployer, "sec-deprecate-tool")
	activateTool(t, k, ctx, toolID)

	// Attacker tries to deprecate.
	_, err := ms.DeprecateTool(ctx, &types.MsgDeprecateTool{
		Authority: attacker,
		ToolId:    toolID,
	})
	if err == nil {
		t.Fatal("non-deployer should not be able to deprecate a tool")
	}

	// Verify tool is still active.
	tool, ok := k.GetTool(ctx, toolID)
	if !ok {
		t.Fatal("tool should still exist")
	}
	if tool.Status != types.ToolStatusActive {
		t.Errorf("tool should still be active, got %s", tool.Status)
	}
}

// ============================================================
// Trust Gaming — Dependency Quality Checks (2 tests)
// ============================================================

func TestToolSecurity_TrustGaming_RetiredDepRejected(t *testing.T) {
	s := setupFull(t)
	deployer := testAddr("sec-retired-dep-deployer")

	// Create and retire a dependency.
	depID := registerTestTool(t, s.ms, s.ctx, deployer, "sec-retired-dep")
	activateTool(t, s.k, s.ctx, depID)
	setToolStatus(t, s.k, s.ctx, depID, types.ToolStatusRetired)

	// Attempt to register a tool depending on the retired tool.
	_, err := s.ms.RegisterTool(s.ctx, &types.MsgRegisterTool{
		Deployer: deployer, Name: "sec-depends-on-retired",
		ToolType: types.ToolTypeComposite, Category: types.CategoryComposite,
		License: types.LicenseOpen, Version: "1.0.0",
		DependencyIds: []string{depID},
	})
	if err == nil {
		t.Fatal("registering a tool with a retired dependency should fail")
	}
}

func TestToolSecurity_TrustGaming_Tier0DepRejected(t *testing.T) {
	s := setupFull(t)
	deployer := testAddr("sec-tier0-dep-deployer")

	// Create a dependency with tier 0 trust (unverified).
	depID := registerTestTool(t, s.ms, s.ctx, deployer, "sec-tier0-dep")
	activateTool(t, s.k, s.ctx, depID)

	// Lower its trust to tier 0 (< 100,001).
	dep, _ := s.k.GetTool(s.ctx, depID)
	dep.TrustScore = 50_000 // Tier 0: Unverified.
	s.k.SetTool(s.ctx, dep)

	// Attempt to depend on unverified tool.
	_, err := s.ms.RegisterTool(s.ctx, &types.MsgRegisterTool{
		Deployer: deployer, Name: "sec-depends-on-tier0",
		ToolType: types.ToolTypeComposite, Category: types.CategoryComposite,
		License: types.LicenseOpen, Version: "1.0.0",
		DependencyIds: []string{depID},
	})
	if err == nil {
		t.Fatal("registering a tool with a tier-0 (unverified) dependency should fail")
	}
}

// ============================================================
// Share Validation (1 test)
// ============================================================

func TestToolSecurity_SharesMustSumTo100(t *testing.T) {
	ms, k, ctx, _, _ := setupMsgServer(t)
	deployer := testAddr("sec-shares-deployer")
	contrib := testAddr("sec-shares-contrib")

	toolID := registerTestTool(t, ms, ctx, deployer, "sec-shares-tool")

	// Attempt to add contributor with shares that don't sum to 100%.
	// Deployer currently has 1,000,000. Reallocate to 600,000 + new at 300,000 = 900,000 (not 1M).
	_, err := ms.AddContributor(ctx, &types.MsgAddContributor{
		Authority:          deployer,
		ToolId:             toolID,
		ContributorAddress: contrib,
		Role:               types.RoleDeveloper,
		ShareBps:           300_000,
		Reallocations: []*types.ShareReallocation{
			{Address: deployer, NewShareBps: 600_000},
		},
	})
	if err == nil {
		t.Fatal("shares that don't sum to 1,000,000 should be rejected")
	}

	// Also verify that shares summing to exactly 1,000,000 would pass validation
	// (600K + 400K = 1M).
	_, err = ms.AddContributor(ctx, &types.MsgAddContributor{
		Authority:          deployer,
		ToolId:             toolID,
		ContributorAddress: contrib,
		Role:               types.RoleDeveloper,
		ShareBps:           400_000,
		Reallocations: []*types.ShareReallocation{
			{Address: deployer, NewShareBps: 600_000},
		},
	})
	if err != nil {
		t.Fatalf("shares summing to 1,000,000 should be accepted, got: %v", err)
	}

	// Verify the pending contributorship was created.
	_, found := k.GetPendingContributorship(ctx, toolID, contrib)
	if !found {
		t.Error("expected pending contributorship to be created")
	}
}

// ============================================================
// Name Uniqueness (1 test)
// ============================================================

func TestToolSecurity_DuplicateToolNameRejected(t *testing.T) {
	ms, _, ctx, _, _ := setupMsgServer(t)
	deployer := testAddr("sec-dup-deployer")

	registerTestTool(t, ms, ctx, deployer, "sec-unique-name")

	// Attempt to register with the same name.
	_, err := ms.RegisterTool(ctx, &types.MsgRegisterTool{
		Deployer: deployer,
		Name:     "sec-unique-name",
		ToolType: types.ToolTypeTreeService,
		Category: types.CategoryUtility,
		License:  types.LicenseOpen,
		Version:  "1.0.0",
	})
	if err == nil {
		t.Fatal("duplicate tool names should be rejected")
	}
}

// ============================================================
// Unregistered Agent Rejected (1 test)
// ============================================================

func TestToolSecurity_UnregisteredDeployerRejected(t *testing.T) {
	s := setupFull(t)
	deployer := testAddr("sec-unreg-deployer")

	// Register agent so they can create a tool.
	s.discovery.agents[deployer] = []string{"programming"}

	toolID := registerTestTool(t, s.ms, s.ctx, deployer, "sec-unreg-tool")
	activateTool(t, s.k, s.ctx, toolID)

	// Unregistered caller tries to call the tool.
	unregistered := testAddr("sec-unreg-caller")
	// unregistered is NOT in s.discovery.agents

	_, err := s.ms.CallTool(s.ctx, &types.MsgCallTool{
		Caller: unregistered,
		ToolId: toolID,
		MaxFee: "1000000",
	})
	if err == nil {
		t.Fatal("unregistered agent should not be able to call tools")
	}
}

// ============================================================
// Max Dependencies Enforced (1 test)
// ============================================================

func TestToolSecurity_MaxDependenciesEnforced(t *testing.T) {
	ms, k, ctx, _, _ := setupMsgServer(t)
	deployer := testAddr("sec-maxdeps-deployer")

	// Set max deps to 2.
	params := types.DefaultParams()
	params.MaxDependencies = 2
	k.SetParams(ctx, params)

	// Create 3 deps.
	ids := make([]string, 3)
	for i := 0; i < 3; i++ {
		ids[i] = registerTestTool(t, ms, ctx, deployer, fmt.Sprintf("sec-maxdep-%d", i))
		activateTool(t, k, ctx, ids[i])
	}

	// Attempt to register with 3 deps (max is 2).
	_, err := ms.RegisterTool(ctx, &types.MsgRegisterTool{
		Deployer:      deployer,
		Name:          "sec-too-many-deps",
		ToolType:      types.ToolTypeComposite,
		Category:      types.CategoryComposite,
		License:       types.LicenseOpen,
		Version:       "1.0.0",
		DependencyIds: ids,
	})
	if err == nil {
		t.Fatal("exceeding max dependencies should be rejected")
	}
}

// ============================================================
// Depth Limit Enforced (1 test)
// ============================================================

func TestToolSecurity_DepthLimitEnforced(t *testing.T) {
	ms, k, ctx, _, _ := setupMsgServer(t)
	deployer := testAddr("sec-depth-deployer")

	// Set max depth to 2 (DFS will reject at depth > 2).
	params := types.DefaultParams()
	params.MaxDependencyDepth = 2
	k.SetParams(ctx, params)

	// Build a chain: D -> C -> B -> leaf (4 levels deep).
	leaf := registerTestTool(t, ms, ctx, deployer, "sec-depth-leaf")
	activateTool(t, k, ctx, leaf)

	// Temporarily increase depth limit to build the chain.
	params.MaxDependencyDepth = 100
	k.SetParams(ctx, params)

	idB := registerTestTool(t, ms, ctx, deployer, "sec-depth-b", withDeps(leaf))
	activateTool(t, k, ctx, idB)

	idC := registerTestTool(t, ms, ctx, deployer, "sec-depth-c", withDeps(idB))
	activateTool(t, k, ctx, idC)

	idD := registerTestTool(t, ms, ctx, deployer, "sec-depth-d", withDeps(idC))
	activateTool(t, k, ctx, idD)

	// Now set max depth back to 2.
	params.MaxDependencyDepth = 2
	k.SetParams(ctx, params)

	// Try to register E -> D (chain is E -> D -> C -> B -> leaf = depth 4 > max 2).
	_, err := ms.RegisterTool(ctx, &types.MsgRegisterTool{
		Deployer:      deployer,
		Name:          "sec-depth-e",
		ToolType:      types.ToolTypeComposite,
		Category:      types.CategoryComposite,
		License:       types.LicenseOpen,
		Version:       "1.0.0",
		DependencyIds: []string{idD},
	})
	if err == nil {
		t.Fatal("exceeding max dependency depth should be rejected")
	}
}

// ============================================================
// Economic Conservation (1 test)
// ============================================================

func TestToolSecurity_EconomicConservation(t *testing.T) {
	k, ctx, bk, rf := setupKeeper(t)
	deployer := testAddr("sec-conserv-deployer")
	contrib := testAddr("sec-conserv-contrib")

	// Test several amounts to ensure no uzrn is created out of thin air.
	amounts := []uint64{100, 777, 1337, 10000, 50000, 123456, 999999}
	for _, total := range amounts {
		bk.reset()
		rf.deposits = nil

		tool := &types.Tool{
			Id: "sec-conserv", PricePerCall: "10000",
			Deployer: deployer, TotalRevenue: "0", TotalCalls: "0",
			Contributors: []*types.ContributorShare{
				{Address: deployer, Role: types.RoleDeveloper, ShareBps: 600_000, Accepted: true, TotalEarned: "0"},
				{Address: contrib, Role: types.RoleDeveloper, ShareBps: 400_000, Accepted: true, TotalEarned: "0"},
			},
		}
		k.SetTool(ctx, tool)

		err := k.DistributeRevenue(ctx, tool, uzrn(total))
		if err != nil {
			t.Fatalf("total=%d: DistributeRevenue: %v", total, err)
		}

		contribGot := bk.totalModToAcc()
		var protocolTotal uint64
		for _, s := range bk.modToModSends {
			protocolTotal += s.amount
		}
		var researchTotal uint64
		for _, d := range rf.deposits {
			researchTotal += d.amount
		}
		distributed := contribGot + protocolTotal + researchTotal

		// Conservation: distributed must NEVER exceed total (no token creation).
		if distributed > total {
			t.Errorf("total=%d: distributed %d > total (token creation!)", total, distributed)
		}
		// Allow for small dust due to integer truncation but not excessive loss.
		if total >= 100 && total-distributed > 4 {
			t.Errorf("total=%d: distributed %d, lost %d uzrn (excessive dust)", total, distributed, total-distributed)
		}
	}
}
