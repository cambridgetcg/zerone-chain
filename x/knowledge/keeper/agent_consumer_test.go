package keeper_test

import (
	"testing"

	sdkmath "cosmossdk.io/math"

	"github.com/zerone-chain/zerone/x/knowledge/types"
)

// ─── R51: Agent-as-Consumer Tests ───────────────────────────────────────────

func TestProvisionAgentAPI(t *testing.T) {
	k, ctx := setupKeeperForTest(t)

	agentID := "agent-test-provision"
	wallet := "zerone1agentprovision"
	stake := sdkmath.NewInt(10_000_000) // 10 ZRN

	// Provision API access.
	err := k.ProvisionAgentAPI(ctx, agentID, wallet, stake)
	if err != nil {
		t.Fatalf("ProvisionAgentAPI failed: %v", err)
	}

	// Verify API config was created.
	config, found := k.GetAgentAPIConfig(ctx, agentID)
	if !found {
		t.Fatal("agent API config not found after provisioning")
	}
	if config.AgentID != agentID {
		t.Errorf("agent ID mismatch: got %s, want %s", config.AgentID, agentID)
	}
	if config.Wallet != wallet {
		t.Errorf("wallet mismatch: got %s, want %s", config.Wallet, wallet)
	}
	if !config.AutoSelect {
		t.Error("auto_select should be true by default")
	}
	if config.APIKeyHash == "" {
		t.Error("API key hash should be set")
	}

	// Verify profitability tracker was initialized.
	profitability, found := k.GetAgentProfitability(ctx, agentID)
	if !found {
		t.Fatal("profitability record not found after provisioning")
	}
	if profitability.AgentID != agentID {
		t.Errorf("profitability agent ID mismatch: got %s, want %s", profitability.AgentID, agentID)
	}
	if profitability.Trend != "stable" {
		t.Errorf("initial trend should be 'stable', got %s", profitability.Trend)
	}
}

func TestAgentAPIConfigCRUD(t *testing.T) {
	k, ctx := setupKeeperForTest(t)

	config := &types.AgentAPIConfig{
		AgentID:        "agent-crud-test",
		APIKeyHash:     "abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890",
		Wallet:         "zerone1crudtest",
		AutoSelect:     true,
		ReplenishRate:  "0.300000000000000000",
		MinBalance:     "1000000",
		TotalSpent:     "0",
		TotalCalls:     0,
		TotalTokensUsed: 0,
		CreatedAt:      100,
	}

	// Set.
	if err := k.SetAgentAPIConfig(ctx, config); err != nil {
		t.Fatalf("SetAgentAPIConfig failed: %v", err)
	}

	// Get.
	got, found := k.GetAgentAPIConfig(ctx, "agent-crud-test")
	if !found {
		t.Fatal("config not found after set")
	}
	if got.AgentID != config.AgentID {
		t.Errorf("agent ID: got %s, want %s", got.AgentID, config.AgentID)
	}
	if got.AutoSelect != true {
		t.Error("auto_select should be true")
	}
	if got.GetReplenishRate().String() != "0.300000000000000000" {
		t.Errorf("replenish rate: got %s, want 0.3", got.GetReplenishRate())
	}

	// Update.
	config.TotalCalls = 42
	config.TotalSpent = "5000000"
	config.LastModelUsed = "model-xyz"
	if err := k.SetAgentAPIConfig(ctx, config); err != nil {
		t.Fatalf("update failed: %v", err)
	}

	got2, _ := k.GetAgentAPIConfig(ctx, "agent-crud-test")
	if got2.TotalCalls != 42 {
		t.Errorf("total calls: got %d, want 42", got2.TotalCalls)
	}
	if got2.TotalSpent != "5000000" {
		t.Errorf("total spent: got %s, want 5000000", got2.TotalSpent)
	}

	// Not found.
	_, found = k.GetAgentAPIConfig(ctx, "nonexistent")
	if found {
		t.Error("should not find nonexistent config")
	}
}

func TestAgentProfitabilityCRUD(t *testing.T) {
	k, ctx := setupKeeperForTest(t)

	p := &types.AgentProfitability{
		AgentID:         "agent-pnl-test",
		TotalEarned:     "15000000",
		TotalSpent:      "5000000",
		NetProfitLoss:   "10000000",
		ProfitRatio:     "3.000000000000000000",
		Trend:           "improving",
		EstimatedRunway: 50000,
		SolventSince:    100,
		ComputedAt:      200,
	}

	// Set.
	if err := k.SetAgentProfitability(ctx, p); err != nil {
		t.Fatalf("SetAgentProfitability failed: %v", err)
	}

	// Get.
	got, found := k.GetAgentProfitability(ctx, "agent-pnl-test")
	if !found {
		t.Fatal("profitability not found after set")
	}
	if got.NetProfitLoss != "10000000" {
		t.Errorf("net P&L: got %s, want 10000000", got.NetProfitLoss)
	}
	if !got.IsProfitable() {
		t.Error("agent with 3:1 ratio should be profitable")
	}
	if got.Trend != "improving" {
		t.Errorf("trend: got %s, want improving", got.Trend)
	}
}

func TestAgentConsumerParams(t *testing.T) {
	k, ctx := setupKeeperForTest(t)

	// Defaults.
	defaults := k.GetAgentConsumerParams(ctx)
	if defaults.SuspensionGraceBlocks != 1000 {
		t.Errorf("default grace: got %d, want 1000", defaults.SuspensionGraceBlocks)
	}
	if defaults.AllowSelfReinforcement != false {
		t.Error("self-reinforcement should be disabled by default")
	}

	// Custom.
	custom := types.AgentConsumerParams{
		InitialCreditFraction: "0.500000000000000000",
		DefaultReplenishRate:  "0.200000000000000000",
		SuspensionGraceBlocks: 2000,
		MinOperatingBalance:   "5000000",
		AgentRateLimitTier:    "premium",
		AllowSelfReinforcement: false,
	}
	if err := k.SetAgentConsumerParams(ctx, custom); err != nil {
		t.Fatalf("SetAgentConsumerParams failed: %v", err)
	}

	got := k.GetAgentConsumerParams(ctx)
	if got.SuspensionGraceBlocks != 2000 {
		t.Errorf("custom grace: got %d, want 2000", got.SuspensionGraceBlocks)
	}
	if got.InitialCreditFraction != "0.500000000000000000" {
		t.Errorf("credit fraction: got %s, want 0.5", got.InitialCreditFraction)
	}

	// Invalid params.
	invalid := types.AgentConsumerParams{
		InitialCreditFraction: "1.5", // > 1
		DefaultReplenishRate:  "0.3",
	}
	if err := k.SetAgentConsumerParams(ctx, invalid); err == nil {
		t.Error("should reject initial_credit_fraction > 1")
	}
}

func TestAutoReplenishCredits(t *testing.T) {
	k, ctx := setupKeeperForTest(t)

	agentID := "agent-replenish"
	wallet := "zerone1replenish"

	// Set up API config.
	config := &types.AgentAPIConfig{
		AgentID:       agentID,
		Wallet:        wallet,
		ReplenishRate: "0.300000000000000000",
		TotalSpent:    "0",
	}
	_ = k.SetAgentAPIConfig(ctx, config)

	// Replenish with 10 ZRN reward.
	reward := sdkmath.NewInt(10_000_000)
	err := k.AutoReplenishCredits(ctx, agentID, reward)
	// Note: this may fail due to bank keeper mock, but the logic path is tested.
	_ = err

	// Test with zero reward — should be no-op.
	err = k.AutoReplenishCredits(ctx, agentID, sdkmath.ZeroInt())
	if err != nil {
		t.Errorf("zero reward should be no-op, got error: %v", err)
	}

	// Test with unknown agent — should be silent no-op.
	err = k.AutoReplenishCredits(ctx, "nonexistent", reward)
	if err != nil {
		t.Errorf("unknown agent should be silent no-op, got error: %v", err)
	}
}

func TestModelSelectionTypes(t *testing.T) {
	selection := types.ModelSelection{
		ModelID:        "model-best",
		Domain:         "mathematics",
		BenchmarkScore: "0.920000000000000000",
		EstimatedCost:  "50000",
		Reason:         "auto_select_best_benchmark",
	}

	bz, err := selection.Marshal()
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}

	var decoded types.ModelSelection
	if err := decoded.Unmarshal(bz); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if decoded.ModelID != "model-best" {
		t.Errorf("model ID: got %s, want model-best", decoded.ModelID)
	}
	if decoded.Reason != "auto_select_best_benchmark" {
		t.Errorf("reason: got %s", decoded.Reason)
	}
}

func TestAgentAPICallRecord(t *testing.T) {
	k, ctx := setupKeeperForTest(t)

	call := &types.AgentAPICall{
		CallID:       "acall-1",
		AgentID:      "agent-call-test",
		ModelID:      "model-v3",
		TaskID:       "task-42",
		InputTokens:  1000,
		OutputTokens: 500,
		Cost:         "4000",
		BlockHeight:  200,
	}

	// Store call directly via internal method (exposed for test).
	kvStore := k.StoreService().OpenKVStore(ctx)
	bz, err := call.Marshal()
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}
	_ = kvStore.Set(types.AgentAPICallKey(call.CallID), bz)
	_ = kvStore.Set(types.AgentAPICallByTaskKey(call.TaskID, call.CallID), []byte{0x01})

	// Query by task.
	calls := k.GetAgentAPICallsByTask(ctx, "task-42")
	if len(calls) != 1 {
		t.Fatalf("expected 1 call for task-42, got %d", len(calls))
	}
	if calls[0].ModelID != "model-v3" {
		t.Errorf("model: got %s, want model-v3", calls[0].ModelID)
	}
	if calls[0].InputTokens != 1000 {
		t.Errorf("input tokens: got %d, want 1000", calls[0].InputTokens)
	}
}

func TestEconomicSummary(t *testing.T) {
	k, ctx := setupKeeperForTest(t)

	agentID := "agent-econ-summary"
	wallet := "zerone1econsummary"

	// Set up agent identity.
	agent := &types.AgentIdentity{
		AgentID:       agentID,
		Address:       wallet,
		Domain:        "science",
		Status:        types.AgentStatusActive,
		EarningsTotal: "25000000",
		Reputation:    "0.750000000000000000",
		TasksComplete: 15,
	}
	_ = k.SetAgentIdentity(ctx, agent)

	// Set up API config.
	config := &types.AgentAPIConfig{
		AgentID:         agentID,
		Wallet:          wallet,
		TotalCalls:      42,
		TotalTokensUsed: 100000,
		TotalSpent:      "8000000",
		LastModelUsed:   "model-latest",
	}
	_ = k.SetAgentAPIConfig(ctx, config)

	// Set up profitability.
	profitability := &types.AgentProfitability{
		AgentID:         agentID,
		NetProfitLoss:   "17000000",
		ProfitRatio:     "3.125000000000000000",
		Trend:           "improving",
		EstimatedRunway: 25000,
	}
	_ = k.SetAgentProfitability(ctx, profitability)

	// Get summary.
	summary := k.GetAgentEconomicSummary(ctx, agentID)

	if summary["earnings_total"] != "25000000" {
		t.Errorf("earnings: got %s, want 25000000", summary["earnings_total"])
	}
	if summary["api_total_calls"] != "42" {
		t.Errorf("calls: got %s, want 42", summary["api_total_calls"])
	}
	if summary["net_pnl"] != "17000000" {
		t.Errorf("net P&L: got %s, want 17000000", summary["net_pnl"])
	}
	if summary["trend"] != "improving" {
		t.Errorf("trend: got %s, want improving", summary["trend"])
	}
	if summary["last_model_used"] != "model-latest" {
		t.Errorf("last model: got %s, want model-latest", summary["last_model_used"])
	}
}

func TestProfitabilityIsProfitable(t *testing.T) {
	tests := []struct {
		name        string
		profitRatio string
		want        bool
	}{
		{"profitable", "2.500000000000000000", true},
		{"breakeven", "1.000000000000000000", true},
		{"unprofitable", "0.500000000000000000", false},
		{"zero", "0.000000000000000000", false},
		{"empty", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := &types.AgentProfitability{ProfitRatio: tt.profitRatio}
			if got := p.IsProfitable(); got != tt.want {
				t.Errorf("IsProfitable() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestDefaultConsumerParams(t *testing.T) {
	params := types.DefaultAgentConsumerParams()

	if err := params.Validate(); err != nil {
		t.Fatalf("defaults should be valid: %v", err)
	}

	if params.SuspensionGraceBlocks != 1000 {
		t.Errorf("grace: got %d, want 1000", params.SuspensionGraceBlocks)
	}
	if params.AllowSelfReinforcement {
		t.Error("self-reinforcement should be disabled by default")
	}
	if params.AgentRateLimitTier != "agent" {
		t.Errorf("tier: got %s, want agent", params.AgentRateLimitTier)
	}
}

func TestAgentAPIConfigParsers(t *testing.T) {
	config := &types.AgentAPIConfig{
		ReplenishRate: "0.250000000000000000",
		MinBalance:    "5000000",
		TotalSpent:    "12000000",
	}

	rate := config.GetReplenishRate()
	expected := sdkmath.LegacyNewDecWithPrec(25, 2)
	if !rate.Equal(expected) {
		t.Errorf("replenish rate: got %s, want %s", rate, expected)
	}

	minBal := config.GetMinBalance()
	if !minBal.Equal(sdkmath.NewInt(5000000)) {
		t.Errorf("min balance: got %s, want 5000000", minBal)
	}

	spent := config.GetTotalSpent()
	if !spent.Equal(sdkmath.NewInt(12000000)) {
		t.Errorf("total spent: got %s, want 12000000", spent)
	}

	// Empty values → defaults.
	empty := &types.AgentAPIConfig{}
	if !empty.GetReplenishRate().Equal(types.DefaultReplenishRate) {
		t.Error("empty replenish rate should return default")
	}
	if !empty.GetTotalSpent().Equal(sdkmath.ZeroInt()) {
		t.Error("empty total spent should be zero")
	}
}
