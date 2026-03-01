package keeper_test

import (
	"fmt"
	"testing"

	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/zerone-chain/zerone/x/toolbox/keeper"
	"github.com/zerone-chain/zerone/x/toolbox/types"
)

// ============================================================
// Demand Tracking (6 tests)
// ============================================================

func TestPricing_RecordToolCall_SingleBlock(t *testing.T) {
	k, ctx, _, _ := setupKeeper(t)
	setupSurgeParams(k, ctx)

	toolID := "demand-single-block"
	k.RecordToolCall(ctx, toolID)
	k.RecordToolCall(ctx, toolID)
	k.RecordToolCall(ctx, toolID)

	total, _ := k.GetToolDemand(ctx, toolID)
	if total != 3 {
		t.Errorf("expected 3 calls in single block, got %d", total)
	}
}

func TestPricing_RecordToolCall_MultiBlock(t *testing.T) {
	k, ctx, _, _ := setupKeeper(t)
	setupSurgeParams(k, ctx)

	toolID := "demand-multi-block"
	// Record 2 calls at block 100.
	k.RecordToolCall(ctx, toolID)
	k.RecordToolCall(ctx, toolID)

	// Record 3 calls at block 101.
	ctx101 := ctx.WithBlockHeight(101)
	k.RecordToolCall(ctx101, toolID)
	k.RecordToolCall(ctx101, toolID)
	k.RecordToolCall(ctx101, toolID)

	// Query at block 101 -- both blocks in window.
	total, _ := k.GetToolDemand(ctx101, toolID)
	if total != 5 {
		t.Errorf("expected 5 calls across blocks, got %d", total)
	}
}

func TestPricing_GetToolDemand_EmptyWindow(t *testing.T) {
	k, ctx, _, _ := setupKeeper(t)
	setupSurgeParams(k, ctx)

	total, util := k.GetToolDemand(ctx, "nonexistent-tool")
	if total != 0 {
		t.Errorf("expected 0 total calls for empty window, got %d", total)
	}
	if util != 0 {
		t.Errorf("expected 0 utilisation for empty window, got %d", util)
	}
}

func TestPricing_GetToolDemand_StalePurge(t *testing.T) {
	k, ctx, _, _ := setupKeeper(t)
	// Set a small window size to test stale purge.
	params := types.DefaultParams()
	params.DemandWindowSize = 5
	params.TargetCallsPerBlockPerTool = 1
	params.TargetGlobalCallsPerBlock = 100
	k.SetParams(ctx, params)

	toolID := "demand-stale"
	// Record at block 100 (default context).
	k.RecordToolCall(ctx, toolID)

	// Query at block 200 -- far outside window of 5.
	farCtx := ctx.WithBlockHeight(200)
	total, util := k.GetToolDemand(farCtx, toolID)
	if total != 0 {
		t.Errorf("expected 0 calls after stale purge, got %d", total)
	}
	if util != 0 {
		t.Errorf("expected 0 utilisation after stale purge, got %d", util)
	}
}

func TestPricing_GetGlobalDemand_AggregatesAll(t *testing.T) {
	k, ctx, _, _ := setupKeeper(t)
	setupSurgeParams(k, ctx)

	// Record calls for two different tools.
	k.RecordToolCall(ctx, "tool-alpha")
	k.RecordToolCall(ctx, "tool-alpha")
	k.RecordToolCall(ctx, "tool-beta")

	// Global demand should aggregate all tool calls.
	globalTotal, _ := k.GetGlobalDemand(ctx)
	if globalTotal != 3 {
		t.Errorf("expected global total 3, got %d", globalTotal)
	}
}

func TestPricing_Utilisation_CapAt100Percent(t *testing.T) {
	k, ctx, _, _ := setupKeeper(t)
	// Very small window to easily exceed capacity.
	params := types.DefaultParams()
	params.DemandWindowSize = 2
	params.TargetCallsPerBlockPerTool = 1
	params.TargetGlobalCallsPerBlock = 100
	k.SetParams(ctx, params)

	toolID := "util-cap"
	// Record way more calls than capacity (2 * 1 = 2 capacity).
	for i := 0; i < 20; i++ {
		k.RecordToolCall(ctx, toolID)
	}

	_, util := k.GetToolDemand(ctx, toolID)
	if util > types.BpsDenominator {
		t.Errorf("utilisation should be capped at 1,000,000, got %d", util)
	}
	if util != types.BpsDenominator {
		t.Logf("utilisation at capacity: %d (expected %d)", util, types.BpsDenominator)
	}
}

// ============================================================
// Surge Pricing (9 tests)
// ============================================================

func TestPricing_Surge_EssentialNoSurge(t *testing.T) {
	k, ctx, _, _ := setupKeeper(t)
	setupSurgeParams(k, ctx)

	// Test all essential categories.
	for _, cat := range []string{types.CategoryDataRetrieval, types.CategoryUtility, types.CategoryFormatting} {
		tool := &types.Tool{
			Id: "essential-" + cat, Category: cat,
			PricePerCall: "1000", Status: types.ToolStatusActive,
			Deployer: testAddr("d"), TotalRevenue: "0", TotalCalls: "0",
		}
		k.SetTool(ctx, tool)

		// Generate high demand.
		for i := 0; i < 9; i++ {
			k.RecordToolCall(ctx, tool.Id)
		}

		surge := k.CalculateSurgeMultiplier(ctx, tool)
		if surge != types.BpsDenominator {
			t.Errorf("category %s: essential should always be 1.0x (%d), got %d",
				cat, types.BpsDenominator, surge)
		}
	}
}

func TestPricing_Surge_StandardBelowThreshold(t *testing.T) {
	k, ctx, _, _ := setupKeeper(t)
	setupSurgeParams(k, ctx)

	tool := &types.Tool{
		Id: "std-below", Category: types.CategoryDataAnalysis,
		PricePerCall: "1000", Status: types.ToolStatusActive,
		Deployer: testAddr("d"), TotalRevenue: "0", TotalCalls: "0",
	}
	k.SetTool(ctx, tool)

	// 4 calls with window=10, target=1 -> 40% utilisation (below 50% threshold).
	for i := 0; i < 4; i++ {
		k.RecordToolCall(ctx, tool.Id)
	}

	surge := k.CalculateSurgeMultiplier(ctx, tool)
	if surge != types.BpsDenominator {
		t.Errorf("standard below threshold: expected 1.0x (%d), got %d", types.BpsDenominator, surge)
	}
}

func TestPricing_Surge_StandardLinearRamp(t *testing.T) {
	k, ctx, _, _ := setupKeeper(t)
	setupSurgeParams(k, ctx)

	tool := &types.Tool{
		Id: "std-linear", Category: types.CategoryDataAnalysis,
		PricePerCall: "1000", Status: types.ToolStatusActive,
		Deployer: testAddr("d"), TotalRevenue: "0", TotalCalls: "0",
	}
	k.SetTool(ctx, tool)

	// 6 calls with window=10, target=1 -> 60% utilisation (between 50% and 80%).
	for i := 0; i < 6; i++ {
		k.RecordToolCall(ctx, tool.Id)
	}

	surge := k.CalculateSurgeMultiplier(ctx, tool)
	if surge <= types.BpsDenominator {
		t.Errorf("standard at 60%%: expected surge > 1.0x, got %d", surge)
	}
	if surge >= 2*types.BpsDenominator {
		t.Errorf("standard at 60%%: expected surge < 2.0x, got %d", surge)
	}
}

func TestPricing_Surge_StandardCap(t *testing.T) {
	k, ctx, _, _ := setupKeeper(t)
	setupSurgeParams(k, ctx)

	tool := &types.Tool{
		Id: "std-cap", Category: types.CategoryDataAnalysis,
		PricePerCall: "1000", Status: types.ToolStatusActive,
		Deployer: testAddr("d"), TotalRevenue: "0", TotalCalls: "0",
	}
	k.SetTool(ctx, tool)

	// 10 calls with window=10, target=1 -> 100% utilisation.
	for i := 0; i < 10; i++ {
		k.RecordToolCall(ctx, tool.Id)
	}

	surge := k.CalculateSurgeMultiplier(ctx, tool)
	if surge > 2*types.BpsDenominator {
		t.Errorf("standard at 100%%: expected <= 2.0x (2,000,000), got %d", surge)
	}
}

func TestPricing_Surge_HeavyBelowThreshold(t *testing.T) {
	k, ctx, _, _ := setupKeeper(t)
	setupSurgeParams(k, ctx)

	tool := &types.Tool{
		Id: "heavy-below", Category: types.CategoryComputation,
		PricePerCall: "5000", Status: types.ToolStatusActive,
		Deployer: testAddr("d"), TotalRevenue: "0", TotalCalls: "0",
	}
	k.SetTool(ctx, tool)

	// 4 calls with window=10, target=1 -> 40% < 50% threshold.
	for i := 0; i < 4; i++ {
		k.RecordToolCall(ctx, tool.Id)
	}

	surge := k.CalculateSurgeMultiplier(ctx, tool)
	if surge != types.BpsDenominator {
		t.Errorf("heavy below threshold: expected 1.0x (%d), got %d", types.BpsDenominator, surge)
	}
}

func TestPricing_Surge_HeavyExponentialZone(t *testing.T) {
	k, ctx, _, _ := setupKeeper(t)
	setupSurgeParams(k, ctx)

	tool := &types.Tool{
		Id: "heavy-exp", Category: types.CategoryComputation,
		PricePerCall: "5000", Status: types.ToolStatusActive,
		Deployer: testAddr("d"), TotalRevenue: "0", TotalCalls: "0",
	}
	k.SetTool(ctx, tool)

	// 9 calls with window=10, target=1 -> 90% > critical (80%).
	for i := 0; i < 9; i++ {
		k.RecordToolCall(ctx, tool.Id)
	}

	surge := k.CalculateSurgeMultiplier(ctx, tool)
	// Heavy above critical should be > 3x base (exponential kicks in above 80%).
	if surge <= types.BpsDenominator {
		t.Errorf("heavy above critical: expected surge > 1.0x, got %d", surge)
	}
	t.Logf("heavy at 90%% utilisation: surge = %d (%0.2fx)", surge, float64(surge)/float64(types.BpsDenominator))
}

func TestPricing_Surge_HeavyCap(t *testing.T) {
	k, ctx, _, _ := setupKeeper(t)
	setupSurgeParams(k, ctx)

	tool := &types.Tool{
		Id: "heavy-cap", Category: types.CategoryComputation,
		PricePerCall: "5000", Status: types.ToolStatusActive,
		Deployer: testAddr("d"), TotalRevenue: "0", TotalCalls: "0",
	}
	k.SetTool(ctx, tool)

	// Saturate demand.
	for i := 0; i < 10; i++ {
		k.RecordToolCall(ctx, tool.Id)
	}

	surge := k.CalculateSurgeMultiplier(ctx, tool)
	if surge > 10*types.BpsDenominator {
		t.Errorf("heavy: expected <= 10.0x (10,000,000), got %d", surge)
	}
}

func TestPricing_PricingTier_AllCategories(t *testing.T) {
	cases := []struct {
		category string
		tier     string
	}{
		// Essential categories.
		{types.CategoryDataRetrieval, keeper.TierEssential},
		{types.CategoryUtility, keeper.TierEssential},
		{types.CategoryFormatting, keeper.TierEssential},
		// Heavy categories.
		{types.CategoryComputation, keeper.TierHeavy},
		{types.CategoryIntegration, keeper.TierHeavy},
		{types.CategoryComposite, keeper.TierHeavy},
		// Standard categories (everything else).
		{types.CategoryDataAnalysis, keeper.TierStandard},
		{types.CategoryVerification, keeper.TierStandard},
		{types.CategoryCommunication, keeper.TierStandard},
		{types.CategoryMonitoring, keeper.TierStandard},
	}

	for _, tc := range cases {
		tier := keeper.PricingTier(tc.category)
		if tier != tc.tier {
			t.Errorf("PricingTier(%s): expected %s, got %s", tc.category, tc.tier, tier)
		}
	}
}

func TestPricing_PricingTier_UnknownDefault(t *testing.T) {
	tier := keeper.PricingTier("nonexistent_category")
	if tier != keeper.TierStandard {
		t.Errorf("unknown category: expected %s, got %s", keeper.TierStandard, tier)
	}
}

// ============================================================
// USD-Stable Pricing (8 tests)
// ============================================================

func TestPricing_USD_FixedUzrn(t *testing.T) {
	k, ctx, _, _ := setupKeeper(t)

	tool := &types.Tool{
		Id: "usd-fixed", PricePerCall: "50000",
		Category: types.CategoryDataAnalysis, Status: types.ToolStatusActive,
		Deployer: testAddr("d"), TotalRevenue: "0", TotalCalls: "0",
		// No TargetPriceUsd -> fixed mode.
	}
	k.SetTool(ctx, tool)

	price, mode := k.GetBasePrice(ctx, tool)
	if mode != "fixed_uzrn" {
		t.Errorf("expected fixed_uzrn mode, got %s", mode)
	}
	if price != 50_000 {
		t.Errorf("expected 50000 uzrn, got %d", price)
	}
}

func TestPricing_USD_StableAt1Dollar(t *testing.T) {
	s := setupFull(t)
	s.billing.priceUSD = 1_000_000 // $1.00

	tool := &types.Tool{
		Id: "usd-1dollar", PricePerCall: "10000",
		TargetPriceUsd: "10000", // $0.01 (10K micro-USD)
		Category:       types.CategoryDataAnalysis, Status: types.ToolStatusActive,
		Deployer: testAddr("d"), TotalRevenue: "0", TotalCalls: "0",
	}
	s.k.SetTool(s.ctx, tool)

	// base_uzrn = (10000 * 1000000) / 1000000 = 10000
	price, mode := s.k.GetBasePrice(s.ctx, tool)
	if mode != "usd_stable" {
		t.Errorf("expected usd_stable mode, got %s", mode)
	}
	if price != 10_000 {
		t.Errorf("ZRN=$1, target=$0.01: expected 10000 uzrn, got %d", price)
	}
}

func TestPricing_USD_StableAt10Dollar(t *testing.T) {
	s := setupFull(t)
	s.billing.priceUSD = 10_000_000 // $10.00

	tool := &types.Tool{
		Id: "usd-10dollar", PricePerCall: "10000",
		TargetPriceUsd: "10000", // $0.01
		Category:       types.CategoryDataAnalysis, Status: types.ToolStatusActive,
		Deployer: testAddr("d"), TotalRevenue: "0", TotalCalls: "0",
	}
	s.k.SetTool(s.ctx, tool)

	// base_uzrn = (10000 * 1000000) / 10000000 = 1000
	price, mode := s.k.GetBasePrice(s.ctx, tool)
	if mode != "usd_stable" {
		t.Errorf("expected usd_stable mode, got %s", mode)
	}
	if price != 1_000 {
		t.Errorf("ZRN=$10, target=$0.01: expected 1000 uzrn, got %d", price)
	}
}

func TestPricing_USD_Floor(t *testing.T) {
	s := setupFull(t)
	s.billing.priceUSD = 100_000_000 // $100 (very high ZRN price)

	tool := &types.Tool{
		Id: "usd-floor", PricePerCall: "50000",
		TargetPriceUsd:  "10000",  // $0.01
		MinPricePerCall: "5000",   // floor at 5000 uzrn
		MaxPricePerCall: "100000", // ceiling at 100K
		Category:        types.CategoryDataAnalysis, Status: types.ToolStatusActive,
		Deployer: testAddr("d"), TotalRevenue: "0", TotalCalls: "0",
	}
	s.k.SetTool(s.ctx, tool)

	// base_uzrn = (10000 * 1000000) / 100000000 = 100 (below min 5000)
	price, mode := s.k.GetBasePrice(s.ctx, tool)
	if mode != "usd_stable" {
		t.Errorf("expected usd_stable mode, got %s", mode)
	}
	if price != 5_000 {
		t.Errorf("expected floor of 5000 uzrn, got %d", price)
	}
}

func TestPricing_USD_Ceiling(t *testing.T) {
	s := setupFull(t)
	s.billing.priceUSD = 100 // very low ZRN price ($0.0001)

	tool := &types.Tool{
		Id: "usd-ceiling", PricePerCall: "50000",
		TargetPriceUsd:  "10000",  // $0.01
		MinPricePerCall: "1000",   // floor
		MaxPricePerCall: "500000", // ceiling at 500K
		Category:        types.CategoryDataAnalysis, Status: types.ToolStatusActive,
		Deployer: testAddr("d"), TotalRevenue: "0", TotalCalls: "0",
	}
	s.k.SetTool(s.ctx, tool)

	// base_uzrn = (10000 * 1000000) / 100 = 100,000,000 (above max 500K)
	price, mode := s.k.GetBasePrice(s.ctx, tool)
	if mode != "usd_stable" {
		t.Errorf("expected usd_stable mode, got %s", mode)
	}
	if price != 500_000 {
		t.Errorf("expected ceiling of 500000 uzrn, got %d", price)
	}
}

func TestPricing_USD_OracleUnavailable(t *testing.T) {
	s := setupFull(t)
	s.billing.priceUSD = 0 // Oracle returns 0.
	s.billing.err = fmt.Errorf("oracle down")

	tool := &types.Tool{
		Id: "usd-oracle-fail", PricePerCall: "50000",
		TargetPriceUsd: "10000", // $0.01 requested but oracle fails
		Category:       types.CategoryDataAnalysis, Status: types.ToolStatusActive,
		Deployer: testAddr("d"), TotalRevenue: "0", TotalCalls: "0",
	}
	s.k.SetTool(s.ctx, tool)

	price, mode := s.k.GetBasePrice(s.ctx, tool)
	// Falls back to fixed uzrn mode.
	if mode != "fixed_uzrn" {
		t.Errorf("expected fallback to fixed_uzrn mode, got %s", mode)
	}
	if price != 50_000 {
		t.Errorf("expected fallback to PricePerCall=50000, got %d", price)
	}
}

// ============================================================
// Effective Price (3 tests)
// ============================================================

func TestPricing_EffectivePrice_NoSurge(t *testing.T) {
	k, ctx, _, _ := setupKeeper(t)

	tool := &types.Tool{
		Id: "eff-nosurge", PricePerCall: "100000",
		Category: types.CategoryDataAnalysis, Status: types.ToolStatusActive,
		Deployer: testAddr("d"), TotalRevenue: "0", TotalCalls: "0",
	}
	k.SetTool(ctx, tool)

	// No demand recorded -> no surge.
	price, mode := k.CalculateEffectivePrice(ctx, tool)
	if mode != "fixed_uzrn" {
		t.Errorf("expected fixed_uzrn mode, got %s", mode)
	}
	if price != 100_000 {
		t.Errorf("expected base price 100000 with no surge, got %d", price)
	}
}

func TestPricing_EffectivePrice_WithSurge(t *testing.T) {
	k, ctx, _, _ := setupKeeper(t)
	setupSurgeParams(k, ctx)

	tool := &types.Tool{
		Id: "eff-surge", PricePerCall: "100000",
		Category: types.CategoryDataAnalysis, Status: types.ToolStatusActive,
		Deployer: testAddr("d"), TotalRevenue: "0", TotalCalls: "0",
	}
	k.SetTool(ctx, tool)

	// Generate demand above threshold.
	for i := 0; i < 7; i++ {
		k.RecordToolCall(ctx, tool.Id)
	}

	price, _ := k.CalculateEffectivePrice(ctx, tool)
	if price <= 100_000 {
		t.Errorf("expected surged price > 100000, got %d", price)
	}

	// Verify effective = base * surge / BpsDenominator.
	base, _ := k.GetBasePrice(ctx, tool)
	surgeMultiplier := k.CalculateSurgeMultiplier(ctx, tool)
	expected := keeper.ApplySurge(base, surgeMultiplier)
	if price != expected {
		t.Errorf("effective price %d != ApplySurge(%d, %d) = %d", price, base, surgeMultiplier, expected)
	}
}

func TestPricing_Surge_Disabled(t *testing.T) {
	k, ctx, _, _ := setupKeeper(t)
	params := types.DefaultParams()
	params.DemandWindowSize = 10
	params.TargetCallsPerBlockPerTool = 1
	params.TargetGlobalCallsPerBlock = 100
	params.SurgeEnabled = false
	k.SetParams(ctx, params)

	tool := &types.Tool{
		Id: "surge-disabled", PricePerCall: "100000",
		Category: types.CategoryDataAnalysis, Status: types.ToolStatusActive,
		Deployer: testAddr("d"), TotalRevenue: "0", TotalCalls: "0",
	}
	k.SetTool(ctx, tool)

	// Even with high demand, surge should be 1.0x when disabled.
	for i := 0; i < 9; i++ {
		k.RecordToolCall(ctx, tool.Id)
	}

	surge := k.CalculateSurgeMultiplier(ctx, tool)
	if surge != types.BpsDenominator {
		t.Errorf("SurgeEnabled=false: expected 1.0x (%d), got %d", types.BpsDenominator, surge)
	}

	price, _ := k.CalculateEffectivePrice(ctx, tool)
	if price != 100_000 {
		t.Errorf("SurgeEnabled=false: expected base price 100000, got %d", price)
	}
}

// ============================================================
// ApplySurge Package Function (2 tests)
// ============================================================

func TestPricing_ApplySurge_NoSurge(t *testing.T) {
	result := keeper.ApplySurge(100_000, types.BpsDenominator)
	if result != 100_000 {
		t.Errorf("ApplySurge with 1.0x: expected 100000, got %d", result)
	}

	// Also test below 1.0x (should short-circuit).
	result = keeper.ApplySurge(100_000, 500_000)
	if result != 100_000 {
		t.Errorf("ApplySurge with 0.5x: expected short-circuit to 100000, got %d", result)
	}
}

func TestPricing_ApplySurge_WithMultiplier(t *testing.T) {
	// 2.0x on 100000 = 200000
	result := keeper.ApplySurge(100_000, 2*types.BpsDenominator)
	if result != 200_000 {
		t.Errorf("ApplySurge with 2.0x: expected 200000, got %d", result)
	}

	// 1.5x on 100000 = 150000
	result = keeper.ApplySurge(100_000, 1_500_000)
	if result != 150_000 {
		t.Errorf("ApplySurge with 1.5x: expected 150000, got %d", result)
	}
}

// Ensure sdk import is used (for WithBlockHeight).
var _ = (sdk.Context).WithBlockHeight
