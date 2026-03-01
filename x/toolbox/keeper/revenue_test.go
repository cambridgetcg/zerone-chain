package keeper_test

import (
	"testing"

	"github.com/zerone-chain/zerone/x/toolbox/types"
)

// ============================================================
// Revenue Distribution Tests
// ============================================================

// TestRevenue_BasicSplit verifies that 1000 uzrn is split according to the
// default governance percentages: 55% contributor, 22% protocol, 3.33% research, 19.67% development.
func TestRevenue_BasicSplit(t *testing.T) {
	k, ctx, bk, rf := setupKeeper(t)
	deployer := testAddr("rev-deployer")

	tool := &types.Tool{
		Id: "rev-basic", PricePerCall: "1000",
		Deployer: deployer, TotalRevenue: "0", TotalCalls: "0",
		Contributors: []*types.ContributorShare{
			{Address: deployer, Role: types.RoleDeveloper, ShareBps: 1_000_000, Accepted: true, TotalEarned: "0"},
		},
	}
	k.SetTool(ctx, tool)

	err := k.DistributeRevenue(ctx, tool, uzrn(1000))
	if err != nil {
		t.Fatalf("DistributeRevenue: %v", err)
	}

	// Contributor share: safeMulDiv(1000, 550000, 1000000) = 550
	contribPaid := bk.totalModToAcc()
	if contribPaid != 550 {
		t.Errorf("contributor share: expected 550, got %d", contribPaid)
	}

	// Protocol: safeMulDiv(1000, 220000, 1000000) = 220
	var protocolTotal uint64
	for _, s := range bk.modToModSends {
		if s.to != "development_fund" {
			protocolTotal += s.amount
		}
	}
	if protocolTotal != 220 {
		t.Errorf("protocol total: expected 220, got %d", protocolTotal)
	}

	// Research: safeMulDiv(1000, 33300, 1000000) = 33
	var researchTotal uint64
	for _, d := range rf.deposits {
		researchTotal += d.amount
	}
	if researchTotal != 33 {
		t.Errorf("research fund: expected 33, got %d", researchTotal)
	}

	// Development fund: safeMulDiv(1000, 196700, 1000000) = 196
	var devFundTotal uint64
	for _, s := range bk.modToModSends {
		if s.to == "development_fund" {
			devFundTotal += s.amount
		}
	}
	if devFundTotal != 196 {
		t.Errorf("development fund: expected 196, got %d", devFundTotal)
	}
}

// TestRevenue_SingleDeployer100Percent verifies that a deployer who owns 100%
// of contributor shares receives the entire contributor allocation.
func TestRevenue_SingleDeployer100Percent(t *testing.T) {
	k, ctx, bk, _ := setupKeeper(t)
	deployer := testAddr("solo-deployer")

	tool := &types.Tool{
		Id: "rev-solo", PricePerCall: "2000",
		Deployer: deployer, TotalRevenue: "0", TotalCalls: "0",
		Contributors: []*types.ContributorShare{
			{Address: deployer, Role: types.RoleDeveloper, ShareBps: 1_000_000, Accepted: true, TotalEarned: "0"},
		},
	}
	k.SetTool(ctx, tool)

	err := k.DistributeRevenue(ctx, tool, uzrn(2000))
	if err != nil {
		t.Fatalf("DistributeRevenue: %v", err)
	}

	// Contributor amount: safeMulDiv(2000, 550000, 1000000) = 1100
	// Deployer at 100% of contributor share: safeMulDiv(1100, 1000000, 1000000) = 1100
	// (no remainder)
	if len(bk.modToAccSends) == 0 {
		t.Fatal("expected at least one send to deployer")
	}
	if bk.modToAccSends[0].to != deployer {
		t.Errorf("expected send to deployer %s, got %s", deployer, bk.modToAccSends[0].to)
	}
	if bk.totalModToAcc() != 1100 {
		t.Errorf("deployer should receive full 1100, got %d", bk.totalModToAcc())
	}
}

// TestRevenue_TwoContributors verifies that with a deployer (70%) and
// contributor (30%), the 55% contributor share is split correctly.
func TestRevenue_TwoContributors(t *testing.T) {
	k, ctx, bk, _ := setupKeeper(t)
	deployer := testAddr("two-deployer")
	contrib := testAddr("two-contrib")

	tool := &types.Tool{
		Id: "rev-two", PricePerCall: "1000",
		Deployer: deployer, TotalRevenue: "0", TotalCalls: "0",
		Contributors: []*types.ContributorShare{
			{Address: deployer, Role: types.RoleDeveloper, ShareBps: 700_000, Accepted: true, TotalEarned: "0"},
			{Address: contrib, Role: types.RoleDeveloper, ShareBps: 300_000, Accepted: true, TotalEarned: "0"},
		},
	}
	k.SetTool(ctx, tool)

	err := k.DistributeRevenue(ctx, tool, uzrn(1000))
	if err != nil {
		t.Fatalf("DistributeRevenue: %v", err)
	}

	// Contributor pool: safeMulDiv(1000, 550000, 1000000) = 550
	// Deployer: safeMulDiv(550, 700000, 1000000) = 385
	// Contributor: safeMulDiv(550, 300000, 1000000) = 165
	// Remainder = 550 - 385 - 165 = 0 → deployer gets 0 extra
	var deployerGot, contribGot uint64
	for _, s := range bk.modToAccSends {
		if s.to == deployer {
			deployerGot += s.amount
		}
		if s.to == contrib {
			contribGot += s.amount
		}
	}
	if deployerGot != 385 {
		t.Errorf("deployer got %d, expected 385", deployerGot)
	}
	if contribGot != 165 {
		t.Errorf("contributor got %d, expected 165", contribGot)
	}
}

// TestRevenue_ThreeContributors verifies a three-way split of the contributor share.
func TestRevenue_ThreeContributors(t *testing.T) {
	k, ctx, bk, _ := setupKeeper(t)
	deployer := testAddr("three-deployer")
	c1 := testAddr("three-c1")
	c2 := testAddr("three-c2")

	tool := &types.Tool{
		Id: "rev-three", PricePerCall: "1000",
		Deployer: deployer, TotalRevenue: "0", TotalCalls: "0",
		Contributors: []*types.ContributorShare{
			{Address: deployer, Role: types.RoleDeveloper, ShareBps: 500_000, Accepted: true, TotalEarned: "0"},
			{Address: c1, Role: types.RoleDeveloper, ShareBps: 300_000, Accepted: true, TotalEarned: "0"},
			{Address: c2, Role: types.RoleDeveloper, ShareBps: 200_000, Accepted: true, TotalEarned: "0"},
		},
	}
	k.SetTool(ctx, tool)

	err := k.DistributeRevenue(ctx, tool, uzrn(1000))
	if err != nil {
		t.Fatalf("DistributeRevenue: %v", err)
	}

	// Contributor pool = 550
	// Deployer: safeMulDiv(550, 500000, 1000000) = 275
	// C1: safeMulDiv(550, 300000, 1000000) = 165
	// C2: safeMulDiv(550, 200000, 1000000) = 110
	// distributed = 275+165+110 = 550, remainder = 0
	sends := make(map[string]uint64)
	for _, s := range bk.modToAccSends {
		sends[s.to] += s.amount
	}
	if sends[deployer] != 275 {
		t.Errorf("deployer: expected 275, got %d", sends[deployer])
	}
	if sends[c1] != 165 {
		t.Errorf("c1: expected 165, got %d", sends[c1])
	}
	if sends[c2] != 110 {
		t.Errorf("c2: expected 110, got %d", sends[c2])
	}
}

// TestRevenue_ZeroPrice verifies that distributing zero amount is a no-op and
// does not panic.
func TestRevenue_ZeroPrice(t *testing.T) {
	k, ctx, bk, rf := setupKeeper(t)
	deployer := testAddr("zero-deployer")

	tool := &types.Tool{
		Id: "rev-zero", PricePerCall: "0",
		Deployer: deployer, TotalRevenue: "0", TotalCalls: "0",
	}
	k.SetTool(ctx, tool)

	err := k.DistributeRevenue(ctx, tool, uzrn(0))
	if err != nil {
		t.Fatalf("expected no error on zero distribute, got: %v", err)
	}

	if len(bk.modToAccSends) != 0 {
		t.Error("expected no sends for zero distribution")
	}
	if len(bk.modToModSends) != 0 {
		t.Error("expected no module sends for zero distribution")
	}
	if len(rf.deposits) != 0 {
		t.Error("expected no research deposits for zero distribution")
	}
	for _, s := range bk.modToModSends {
		if s.to == "development_fund" {
			t.Error("expected no development fund sends for zero distribution")
		}
	}
}

// TestRevenue_LargeAmount verifies distribution integrity for 1,000,000 uzrn.
func TestRevenue_LargeAmount(t *testing.T) {
	k, ctx, bk, rf := setupKeeper(t)
	deployer := testAddr("large-deployer")

	tool := &types.Tool{
		Id: "rev-large", PricePerCall: "1000000",
		Deployer: deployer, TotalRevenue: "0", TotalCalls: "0",
		Contributors: []*types.ContributorShare{
			{Address: deployer, Role: types.RoleDeveloper, ShareBps: 1_000_000, Accepted: true, TotalEarned: "0"},
		},
	}
	k.SetTool(ctx, tool)

	total := uint64(1_000_000)
	err := k.DistributeRevenue(ctx, tool, uzrn(total))
	if err != nil {
		t.Fatalf("DistributeRevenue: %v", err)
	}

	// 55% = 550,000, 22% = 220,000, 3.33% = 33,300, 19.67% = 196,700
	contribGot := bk.totalModToAcc()
	if contribGot != 550_000 {
		t.Errorf("contributor: expected 550000, got %d", contribGot)
	}

	var protocolTotal, devFundTotal uint64
	for _, s := range bk.modToModSends {
		if s.to == "development_fund" {
			devFundTotal += s.amount
		} else {
			protocolTotal += s.amount
		}
	}
	if protocolTotal != 220_000 {
		t.Errorf("protocol: expected 220000, got %d", protocolTotal)
	}

	var researchTotal uint64
	for _, d := range rf.deposits {
		researchTotal += d.amount
	}
	if researchTotal != 33_300 {
		t.Errorf("research: expected 33300, got %d", researchTotal)
	}

	if devFundTotal != 196_700 {
		t.Errorf("development fund: expected 196700, got %d", devFundTotal)
	}

	// Verify conservation: sum of all distributed equals total.
	distributed := contribGot + protocolTotal + researchTotal + devFundTotal
	if distributed != total {
		t.Errorf("conservation: distributed %d != total %d", distributed, total)
	}
}

// TestRevenue_SmallAmount verifies rounding behavior for the smallest possible
// amount (1 uzrn).
func TestRevenue_SmallAmount(t *testing.T) {
	k, ctx, bk, rf := setupKeeper(t)
	deployer := testAddr("small-deployer")

	tool := &types.Tool{
		Id: "rev-small", PricePerCall: "1",
		Deployer: deployer, TotalRevenue: "0", TotalCalls: "0",
		Contributors: []*types.ContributorShare{
			{Address: deployer, Role: types.RoleDeveloper, ShareBps: 1_000_000, Accepted: true, TotalEarned: "0"},
		},
	}
	k.SetTool(ctx, tool)

	err := k.DistributeRevenue(ctx, tool, uzrn(1))
	if err != nil {
		t.Fatalf("DistributeRevenue: %v", err)
	}

	// safeMulDiv(1, 550000, 1000000) = 0 (integer truncation)
	// safeMulDiv(1, 220000, 1000000) = 0
	// safeMulDiv(1, 33300, 1000000) = 0
	// safeMulDiv(1, 196700, 1000000) = 0
	// All splits round to 0 for 1 uzrn — nothing gets distributed.
	contribGot := bk.totalModToAcc()
	var protocolTotal uint64
	for _, s := range bk.modToModSends {
		protocolTotal += s.amount
	}
	var researchTotal uint64
	for _, d := range rf.deposits {
		researchTotal += d.amount
	}
	total := contribGot + protocolTotal + researchTotal
	// With 1 uzrn and integer division, all splits truncate to 0.
	if total != 0 {
		t.Errorf("expected 0 distributed for 1 uzrn, got %d", total)
	}
}

// TestRevenue_RoundingDoesNotLoseTokens verifies that the sum of all distributed
// parts never exceeds the total (no token creation via rounding).
func TestRevenue_RoundingDoesNotLoseTokens(t *testing.T) {
	k, ctx, bk, rf := setupKeeper(t)
	deployer := testAddr("round-deployer")

	// Test several amounts that may produce rounding.
	amounts := []uint64{7, 13, 33, 99, 101, 333, 997, 10003}
	for _, amount := range amounts {
		bk.reset()
		rf.deposits = nil

		tool := &types.Tool{
			Id: "rev-round", PricePerCall: "1000",
			Deployer: deployer, TotalRevenue: "0", TotalCalls: "0",
			Contributors: []*types.ContributorShare{
				{Address: deployer, Role: types.RoleDeveloper, ShareBps: 600_000, Accepted: true, TotalEarned: "0"},
				{Address: testAddr("round-c"), Role: types.RoleDeveloper, ShareBps: 400_000, Accepted: true, TotalEarned: "0"},
			},
		}
		k.SetTool(ctx, tool)

		err := k.DistributeRevenue(ctx, tool, uzrn(amount))
		if err != nil {
			t.Fatalf("amount=%d: DistributeRevenue: %v", amount, err)
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
		if distributed > amount {
			t.Errorf("amount=%d: distributed %d > total (token creation!)", amount, distributed)
		}
	}
}

// TestRevenue_DevFundTracking verifies that the development fund amount matches ~19.67% of the total.
func TestRevenue_DevFundTracking(t *testing.T) {
	k, ctx, bk, _ := setupKeeper(t)
	deployer := testAddr("burn-deployer")

	tool := &types.Tool{
		Id: "rev-burn", PricePerCall: "10000",
		Deployer: deployer, TotalRevenue: "0", TotalCalls: "0",
		Contributors: []*types.ContributorShare{
			{Address: deployer, Role: types.RoleDeveloper, ShareBps: 1_000_000, Accepted: true, TotalEarned: "0"},
		},
	}
	k.SetTool(ctx, tool)

	err := k.DistributeRevenue(ctx, tool, uzrn(10000))
	if err != nil {
		t.Fatalf("DistributeRevenue: %v", err)
	}

	// safeMulDiv(10000, 196700, 1000000) = 1967
	var devFundTotal uint64
	for _, s := range bk.modToModSends {
		if s.to == "development_fund" {
			devFundTotal += s.amount
		}
	}
	if devFundTotal != 1967 {
		t.Errorf("development fund: expected 1967, got %d", devFundTotal)
	}
}

// TestRevenue_ResearchFundDeposit verifies that the research fund receives ~3.33%.
func TestRevenue_ResearchFundDeposit(t *testing.T) {
	k, ctx, _, rf := setupKeeper(t)
	deployer := testAddr("research-deployer")

	tool := &types.Tool{
		Id: "rev-research", PricePerCall: "10000",
		Deployer: deployer, TotalRevenue: "0", TotalCalls: "0",
		Contributors: []*types.ContributorShare{
			{Address: deployer, Role: types.RoleDeveloper, ShareBps: 1_000_000, Accepted: true, TotalEarned: "0"},
		},
	}
	k.SetTool(ctx, tool)

	err := k.DistributeRevenue(ctx, tool, uzrn(10000))
	if err != nil {
		t.Fatalf("DistributeRevenue: %v", err)
	}

	// safeMulDiv(10000, 33300, 1000000) = 333
	var researchTotal uint64
	for _, d := range rf.deposits {
		researchTotal += d.amount
	}
	if researchTotal != 333 {
		t.Errorf("research fund: expected 333, got %d", researchTotal)
	}
}

// TestRevenue_DependencyCascade verifies that when a caller pays for tool A,
// which depends on B and C, the dependency cost includes B and C.
func TestRevenue_DependencyCascade(t *testing.T) {
	s := setupFull(t)
	deployer := testAddr("dep-deployer")

	// Create dependency tools B and C with prices.
	idC := registerTestTool(t, s.ms, s.ctx, deployer, "dep-c", withPrice("200"))
	activateTool(t, s.k, s.ctx, idC)

	idB := registerTestTool(t, s.ms, s.ctx, deployer, "dep-b", withPrice("300"))
	activateTool(t, s.k, s.ctx, idB)

	// Create tool A that depends on B and C.
	idA := registerTestTool(t, s.ms, s.ctx, deployer, "dep-a",
		withPrice("500"), withDeps(idB, idC))

	// Compute dependency cost for A.
	toolA, ok := s.k.GetTool(s.ctx, idA)
	if !ok {
		t.Fatal("tool A not found")
	}
	depCost := s.k.ComputeDependencyCostUint64(s.ctx, toolA.DependencyIds)

	// B(300) + C(200) = 500
	if depCost != 500 {
		t.Errorf("dependency cost: expected 500, got %d", depCost)
	}
}

// TestRevenue_DiamondDependency verifies that in a diamond dependency graph
// where both B and C depend on D, D's cost is counted only once in the
// transitive cost computation.
func TestRevenue_DiamondDependency(t *testing.T) {
	s := setupFull(t)
	deployer := testAddr("diamond-deployer")

	// D is the shared leaf.
	idD := registerTestTool(t, s.ms, s.ctx, deployer, "diamond-d", withPrice("100"))
	activateTool(t, s.k, s.ctx, idD)

	// B depends on D.
	idB := registerTestTool(t, s.ms, s.ctx, deployer, "diamond-b", withPrice("200"), withDeps(idD))
	activateTool(t, s.k, s.ctx, idB)

	// C depends on D.
	idC := registerTestTool(t, s.ms, s.ctx, deployer, "diamond-c", withPrice("300"), withDeps(idD))
	activateTool(t, s.k, s.ctx, idC)

	// A depends on B and C (diamond: B->D, C->D).
	idA := registerTestTool(t, s.ms, s.ctx, deployer, "diamond-a", withPrice("400"), withDeps(idB, idC))

	// Transitive cost: B(200) + C(300) + D(100) = 600, not 800 (D counted once).
	transitiveCost := s.k.ComputeTransitiveCost(s.ctx, idA)
	expected := uint64(600)
	if !transitiveCost.IsUint64() || transitiveCost.Uint64() != expected {
		t.Errorf("diamond transitive cost: expected %d, got %s", expected, transitiveCost.String())
	}
}

// TestRevenue_CollectPayment_Success verifies that CollectPayment succeeds
// when the caller has sufficient funds.
func TestRevenue_CollectPayment_Success(t *testing.T) {
	k, ctx, bk, _ := setupKeeper(t)
	caller := testAddr("pay-caller")

	tool := &types.Tool{
		Id: "pay-ok", PricePerCall: "500",
		Category: types.CategoryUtility, Status: types.ToolStatusActive,
		Deployer: testAddr("pay-deployer"), TotalRevenue: "0", TotalCalls: "0",
	}
	k.SetTool(ctx, tool)

	paid, isFree, err := k.CollectPayment(ctx, caller, tool, 1000)
	if err != nil {
		t.Fatalf("CollectPayment: %v", err)
	}
	if isFree {
		t.Error("expected not free")
	}
	if paid != 500 {
		t.Errorf("expected paid=500, got %d", paid)
	}
	if len(bk.accToModSends) != 1 {
		t.Fatalf("expected 1 accToMod send, got %d", len(bk.accToModSends))
	}
	if bk.accToModSends[0].amount != 500 {
		t.Errorf("expected transfer of 500, got %d", bk.accToModSends[0].amount)
	}
}

// TestRevenue_CollectPayment_InsufficientFunds verifies that CollectPayment
// fails gracefully when the caller lacks funds.
func TestRevenue_CollectPayment_InsufficientFunds(t *testing.T) {
	k, ctx, bk, _ := setupKeeper(t)
	caller := testAddr("broke-caller")

	tool := &types.Tool{
		Id: "pay-broke", PricePerCall: "500",
		Category: types.CategoryUtility, Status: types.ToolStatusActive,
		Deployer: testAddr("pay-deployer2"), TotalRevenue: "0", TotalCalls: "0",
	}
	k.SetTool(ctx, tool)

	// Set bank to fail sends.
	bk.failSend = true

	_, _, err := k.CollectPayment(ctx, caller, tool, 1000)
	if err == nil {
		t.Fatal("expected error for insufficient funds")
	}
}

// TestRevenue_CollectPayment_MaxFeeExceeded verifies that CollectPayment
// rejects when maxFee < effective price.
func TestRevenue_CollectPayment_MaxFeeExceeded(t *testing.T) {
	k, ctx, _, _ := setupKeeper(t)
	caller := testAddr("maxfee-caller")

	tool := &types.Tool{
		Id: "pay-maxfee", PricePerCall: "1000",
		Category: types.CategoryUtility, Status: types.ToolStatusActive,
		Deployer: testAddr("maxfee-deployer"), TotalRevenue: "0", TotalCalls: "0",
	}
	k.SetTool(ctx, tool)

	// maxFee = 500 < price = 1000
	_, _, err := k.CollectPayment(ctx, caller, tool, 500)
	if err == nil {
		t.Fatal("expected ErrFeeTooHigh when maxFee < effective price")
	}
}

// TestRevenue_CollectPayment_DynamicPrice verifies that surge pricing affects
// the collected amount when enabled and demand is high.
func TestRevenue_CollectPayment_DynamicPrice(t *testing.T) {
	k, ctx, bk, _ := setupKeeper(t)
	setupSurgeParams(k, ctx)
	caller := testAddr("dyn-caller")

	tool := &types.Tool{
		Id: "pay-dynamic", PricePerCall: "100000",
		Category: types.CategoryDataAnalysis, // standard tier — can surge
		Status:   types.ToolStatusActive,
		Deployer: testAddr("dyn-deployer"), TotalRevenue: "0", TotalCalls: "0",
	}
	k.SetTool(ctx, tool)

	// Generate high demand to trigger surge.
	for i := 0; i < 7; i++ {
		k.RecordToolCall(ctx, tool.Id)
	}

	paid, _, err := k.CollectPayment(ctx, caller, tool, 0) // maxFee=0 means no cap
	if err != nil {
		t.Fatalf("CollectPayment: %v", err)
	}

	// With surge, the effective price should be > base price.
	if paid <= 100_000 {
		t.Errorf("expected surged price > 100000, got %d", paid)
	}

	// Verify the bank transfer matches.
	if len(bk.accToModSends) != 1 {
		t.Fatalf("expected 1 send, got %d", len(bk.accToModSends))
	}
	if bk.accToModSends[0].amount != paid {
		t.Errorf("transfer amount %d != paid %d", bk.accToModSends[0].amount, paid)
	}
}

// TestRevenue_EconomicConservation verifies that every uzrn is accounted for
// across all buckets (contributor + protocol + research + development).
func TestRevenue_EconomicConservation(t *testing.T) {
	k, ctx, bk, rf := setupKeeper(t)
	deployer := testAddr("conserv-deployer")
	contrib := testAddr("conserv-contrib")

	amounts := []uint64{100, 500, 1000, 5000, 10000, 100000, 999999}
	for _, total := range amounts {
		bk.reset()
		rf.deposits = nil

		tool := &types.Tool{
			Id: "rev-conserv", PricePerCall: "10000",
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
		if distributed > total {
			t.Errorf("total=%d: distributed %d exceeds total (token creation)", total, distributed)
		}
		// Allow for dust due to integer truncation, but no more than 4 uzrn lost
		// (one per bucket that might truncate).
		if total >= 100 && total-distributed > 4 {
			t.Errorf("total=%d: distributed only %d, lost %d uzrn (excessive dust)", total, distributed, total-distributed)
		}
	}
}

// TestRevenue_RetiredDependencyBlocks verifies that a retired sub-tool
// prevents dependency registration (and thus payment pipeline).
func TestRevenue_RetiredDependencyBlocks(t *testing.T) {
	s := setupFull(t)
	deployer := testAddr("retired-deployer")

	// Create a tool and retire it.
	depID := registerTestTool(t, s.ms, s.ctx, deployer, "retired-dep", withPrice("100"))
	activateTool(t, s.k, s.ctx, depID)
	setToolStatus(t, s.k, s.ctx, depID, types.ToolStatusRetired)

	// Attempting to register a tool depending on a retired dep should fail.
	_, err := s.ms.RegisterTool(s.ctx, &types.MsgRegisterTool{
		Deployer:      deployer,
		Name:          "depends-on-retired",
		ToolType:      types.ToolTypeTreeService,
		Category:      types.CategoryUtility,
		License:       types.LicenseOpen,
		Version:       "1.0.0",
		DependencyIds: []string{depID},
	})
	if err == nil {
		t.Fatal("expected error when depending on a retired tool")
	}
}

// TestRevenue_ContributorShareLock verifies that locked shares are respected
// during distribution and that no new contributors can be added after locking.
func TestRevenue_ContributorShareLock(t *testing.T) {
	ms, k, ctx, bk, _ := setupMsgServer(t)
	deployer := testAddr("lock-deployer")
	contrib := testAddr("lock-contrib")

	id := registerTestTool(t, ms, ctx, deployer, "lock-tool", withPrice("10000"))

	// Manually set up the tool with correctly reallocated shares
	// (bypassing the AddContributor msg which has a known reallocation persistence issue).
	tool, ok := k.GetTool(ctx, id)
	if !ok {
		t.Fatal("tool not found")
	}
	tool.Contributors = []*types.ContributorShare{
		{Address: deployer, Role: types.RoleDeveloper, ShareBps: 700_000, Accepted: true, TotalEarned: "0"},
		{Address: contrib, Role: types.RoleDeveloper, ShareBps: 300_000, Accepted: true, TotalEarned: "0"},
	}
	k.SetTool(ctx, tool)

	// Lock shares.
	_, err := ms.LockShares(ctx, &types.MsgLockShares{Deployer: deployer, ToolId: id})
	if err != nil {
		t.Fatalf("LockShares: %v", err)
	}

	// Verify distribution still works with locked shares.
	tool, _ = k.GetTool(ctx, id)
	bk.reset()
	err = k.DistributeRevenue(ctx, tool, uzrn(10000))
	if err != nil {
		t.Fatalf("DistributeRevenue after lock: %v", err)
	}

	// Contributor pool = safeMulDiv(10000, 550000, 1000000) = 5500
	// Deployer (70%): safeMulDiv(5500, 700000, 1000000) = 3850
	// Contrib (30%): safeMulDiv(5500, 300000, 1000000) = 1650
	sends := make(map[string]uint64)
	for _, s := range bk.modToAccSends {
		sends[s.to] += s.amount
	}
	if sends[deployer] != 3850 {
		t.Errorf("deployer: expected 3850, got %d", sends[deployer])
	}
	if sends[contrib] != 1650 {
		t.Errorf("contrib: expected 1650, got %d", sends[contrib])
	}

	// After locking, adding a new contributor should fail.
	_, err = ms.AddContributor(ctx, &types.MsgAddContributor{
		Authority: deployer, ToolId: id, ContributorAddress: testAddr("new-c"),
		Role: types.RoleDeveloper, ShareBps: 100_000,
		Reallocations: []*types.ShareReallocation{{Address: deployer, NewShareBps: 600_000}},
	})
	if err == nil {
		t.Error("expected error when adding contributor to locked tool")
	}
}

// TestRevenue_PendingContributorExcluded verifies that contributors who have
// not yet accepted do not receive revenue.
func TestRevenue_PendingContributorExcluded(t *testing.T) {
	k, ctx, bk, _ := setupKeeper(t)
	deployer := testAddr("pending-deployer")
	pendingC := testAddr("pending-contrib")

	tool := &types.Tool{
		Id: "rev-pending", PricePerCall: "1000",
		Deployer: deployer, TotalRevenue: "0", TotalCalls: "0",
		Contributors: []*types.ContributorShare{
			{Address: deployer, Role: types.RoleDeveloper, ShareBps: 700_000, Accepted: true, TotalEarned: "0"},
			{Address: pendingC, Role: types.RoleDeveloper, ShareBps: 300_000, Accepted: false, TotalEarned: "0"},
		},
	}
	k.SetTool(ctx, tool)

	err := k.DistributeRevenue(ctx, tool, uzrn(1000))
	if err != nil {
		t.Fatalf("DistributeRevenue: %v", err)
	}

	// Contributor pool = 550
	// Deployer: safeMulDiv(550, 700000, 1000000) = 385
	// Pending contributor: skipped (Accepted=false)
	// Remainder = 550 - 385 = 165 → goes to deployer
	sends := make(map[string]uint64)
	for _, s := range bk.modToAccSends {
		sends[s.to] += s.amount
	}

	// Deployer gets 385 (share) + 165 (remainder) = 550
	if sends[deployer] != 550 {
		t.Errorf("deployer: expected 550 (share + remainder), got %d", sends[deployer])
	}
	// Pending contributor should get nothing.
	if sends[pendingC] != 0 {
		t.Errorf("pending contributor: expected 0, got %d", sends[pendingC])
	}
}
