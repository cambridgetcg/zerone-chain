package keeper_test

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"strings"
	"testing"

	"cosmossdk.io/log"
	sdkmath "cosmossdk.io/math"
	"cosmossdk.io/store"
	storemetrics "cosmossdk.io/store/metrics"
	storetypes "cosmossdk.io/store/types"

	dbm "github.com/cosmos/cosmos-db"
	"github.com/cosmos/cosmos-sdk/codec"
	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	"github.com/cosmos/cosmos-sdk/runtime"
	sdk "github.com/cosmos/cosmos-sdk/types"

	cmtproto "github.com/cometbft/cometbft/proto/tendermint/types"

	"github.com/zerone-chain/zerone/x/toolbox/keeper"
	"github.com/zerone-chain/zerone/x/toolbox/types"
)

func init() {
	config := sdk.GetConfig()
	config.SetBech32PrefixForAccount("zrn", "zrnpub")
	config.SetBech32PrefixForValidator("zrnvaloper", "zrnvaloperpub")
	config.SetBech32PrefixForConsensusNode("zrnvalcons", "zrnvalconspub")
}

// ---------- Test Address Helper ----------

func ppTestAddr(name string) string {
	hash := sha256.Sum256([]byte("pp_test_addr:" + name))
	return sdk.AccAddress(hash[:20]).String()
}

// ---------- Mock Bank Keeper ----------

type ppMockBankKeeper struct{}

func (m *ppMockBankKeeper) SendCoins(_ context.Context, _ sdk.AccAddress, _ sdk.AccAddress, _ sdk.Coins) error {
	return nil
}
func (m *ppMockBankKeeper) SendCoinsFromAccountToModule(_ context.Context, _ sdk.AccAddress, _ string, _ sdk.Coins) error {
	return nil
}
func (m *ppMockBankKeeper) SendCoinsFromModuleToAccount(_ context.Context, _ string, _ sdk.AccAddress, _ sdk.Coins) error {
	return nil
}
func (m *ppMockBankKeeper) SendCoinsFromModuleToModule(_ context.Context, _ string, _ string, _ sdk.Coins) error {
	return nil
}
func (m *ppMockBankKeeper) GetBalance(_ context.Context, _ sdk.AccAddress, _ string) sdk.Coin {
	return sdk.NewCoin("uzrn", sdkmath.ZeroInt())
}

var _ types.BankKeeper = (*ppMockBankKeeper)(nil)

// ---------- Mock Research Fund Depositor ----------

type ppMockResearchFund struct{}

func (m *ppMockResearchFund) DepositToResearchFund(_ context.Context, _ string, _ sdk.Coins) error {
	return nil
}

var _ types.ResearchFundDepositor = (*ppMockResearchFund)(nil)

// ---------- Mock Knowledge Keeper ----------

type ppMockKnowledgeKeeper struct {
	facts     map[string]mockFact // factID -> fact
	searchIDs []string            // IDs returned by SearchFactsByContent
}

type mockFact struct {
	content    string
	confidence uint64
	citations  uint64
}

func newMockKnowledgeKeeper() *ppMockKnowledgeKeeper {
	return &ppMockKnowledgeKeeper{
		facts: make(map[string]mockFact),
	}
}

func (m *ppMockKnowledgeKeeper) GetFactConfidence(_ context.Context, factID string) (uint64, bool) {
	f, ok := m.facts[factID]
	if !ok {
		return 0, false
	}
	return f.confidence, true
}

func (m *ppMockKnowledgeKeeper) SearchFactsByContent(_ context.Context, _ string, _ []string, _ uint64) ([]string, error) {
	return m.searchIDs, nil
}

func (m *ppMockKnowledgeKeeper) GetFactDetails(_ context.Context, factID string) (string, uint64, uint64, error) {
	f, ok := m.facts[factID]
	if !ok {
		return "", 0, 0, types.ErrToolNotFound
	}
	return f.content, f.confidence, f.citations, nil
}

func (m *ppMockKnowledgeKeeper) RecordFactCitation(_ context.Context, _ string, _ string) error {
	return nil
}

var _ types.KnowledgeKeeper = (*ppMockKnowledgeKeeper)(nil)

// ---------- Mock Discovery Keeper ----------

type ppMockDiscoveryKeeper struct {
	agents map[string][]string // address -> capabilities
}

func newMockDiscoveryKeeper() *ppMockDiscoveryKeeper {
	return &ppMockDiscoveryKeeper{
		agents: make(map[string][]string),
	}
}

func (m *ppMockDiscoveryKeeper) IsRegisteredAgent(_ context.Context, address string) bool {
	_, ok := m.agents[address]
	return ok
}

func (m *ppMockDiscoveryKeeper) GetAgentCapabilityTypes(_ context.Context, address string) ([]string, error) {
	caps, ok := m.agents[address]
	if !ok {
		return nil, nil
	}
	return caps, nil
}

var _ types.DiscoveryKeeper = (*ppMockDiscoveryKeeper)(nil)

// ---------- Test Setup ----------

func ppSetupKeeper(t *testing.T) (keeper.Keeper, sdk.Context) {
	t.Helper()

	storeKey := storetypes.NewKVStoreKey(types.StoreKey)
	db := dbm.NewMemDB()
	stateStore := store.NewCommitMultiStore(db, log.NewNopLogger(), storemetrics.NewNoOpMetrics())
	stateStore.MountStoreWithDB(storeKey, storetypes.StoreTypeIAVL, db)
	if err := stateStore.LoadLatestVersion(); err != nil {
		t.Fatalf("failed to load latest version: %v", err)
	}

	registry := codectypes.NewInterfaceRegistry()
	cdc := codec.NewProtoCodec(registry)

	storeService := runtime.NewKVStoreService(storeKey)
	k := keeper.NewKeeper(storeService, cdc, "zrn1authority", &ppMockBankKeeper{}, &ppMockResearchFund{})

	ctx := sdk.NewContext(stateStore, cmtproto.Header{Height: 100}, false, log.NewNopLogger())
	k.SetParams(ctx, types.DefaultParams())

	return k, ctx
}

// ppSetupWithKnowledge creates a keeper with a configured knowledge keeper.
func ppSetupWithKnowledge(t *testing.T) (keeper.Keeper, sdk.Context, *ppMockKnowledgeKeeper, *ppMockDiscoveryKeeper) {
	t.Helper()
	k, ctx := ppSetupKeeper(t)

	mkk := newMockKnowledgeKeeper()
	mdk := newMockDiscoveryKeeper()

	k.SetKnowledgeKeeper(mkk)
	k.SetDiscoveryKeeper(mdk)

	return k, ctx, mkk, mdk
}

// ========== Knowledge Scout Tests ==========

func TestKnowledgeScout_NilKeeper(t *testing.T) {
	k, ctx := ppSetupKeeper(t)
	// No knowledge keeper set.
	out, err := k.KnowledgeScout(ctx, &types.KnowledgeScoutInput{
		QueryTerms: []string{"purpose"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out.TotalFound != 0 {
		t.Errorf("expected 0 facts, got %d", out.TotalFound)
	}
}

func TestKnowledgeScout_EmptyTerms(t *testing.T) {
	k, ctx, _, _ := ppSetupWithKnowledge(t)
	out, err := k.KnowledgeScout(ctx, &types.KnowledgeScoutInput{
		QueryTerms: nil,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out.TotalFound != 0 {
		t.Errorf("expected 0 facts, got %d", out.TotalFound)
	}
}

func TestKnowledgeScout_BasicSearch(t *testing.T) {
	k, ctx, mkk, _ := ppSetupWithKnowledge(t)

	mkk.facts["fact-1"] = mockFact{content: "Agents build tools for purpose", confidence: 700_000, citations: 5}
	mkk.facts["fact-2"] = mockFact{content: "Verification ensures truth", confidence: 800_000, citations: 10}
	mkk.searchIDs = []string{"fact-1", "fact-2"}

	out, err := k.KnowledgeScout(ctx, &types.KnowledgeScoutInput{
		QueryTerms:    []string{"purpose", "tools"},
		MinConfidence: 600_000,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out.TotalFound != 2 {
		t.Errorf("expected 2 facts, got %d", out.TotalFound)
	}
	// Should be sorted by relevance.
	if len(out.Facts) < 2 {
		t.Fatalf("expected 2 scored facts")
	}
	// First fact should have higher relevance (more term overlap).
	if out.Facts[0].RelevanceScore < out.Facts[1].RelevanceScore {
		t.Errorf("facts not sorted by relevance: %d < %d", out.Facts[0].RelevanceScore, out.Facts[1].RelevanceScore)
	}
}

func TestKnowledgeScout_MinConfidenceFilter(t *testing.T) {
	k, ctx, mkk, _ := ppSetupWithKnowledge(t)

	mkk.facts["fact-low"] = mockFact{content: "Vague claim about things", confidence: 100_000, citations: 1}
	mkk.facts["fact-high"] = mockFact{content: "Well-established fact", confidence: 900_000, citations: 20}
	mkk.searchIDs = []string{"fact-low", "fact-high"}

	out, err := k.KnowledgeScout(ctx, &types.KnowledgeScoutInput{
		QueryTerms:    []string{"fact"},
		MinConfidence: 500_000, // Default
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out.TotalFound != 1 {
		t.Errorf("expected 1 fact (low-confidence filtered), got %d", out.TotalFound)
	}
	if out.Facts[0].FactId != "fact-high" {
		t.Errorf("expected fact-high, got %s", out.Facts[0].FactId)
	}
}

func TestKnowledgeScout_MaxResults(t *testing.T) {
	k, ctx, mkk, _ := ppSetupWithKnowledge(t)

	// Add 5 facts.
	ids := []string{"f1", "f2", "f3", "f4", "f5"}
	for _, id := range ids {
		mkk.facts[id] = mockFact{content: "Purpose fact " + id, confidence: 700_000, citations: 3}
	}
	mkk.searchIDs = ids

	out, err := k.KnowledgeScout(ctx, &types.KnowledgeScoutInput{
		QueryTerms: []string{"purpose"},
		MaxResults: 3,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out.TotalFound != 3 {
		t.Errorf("expected 3 facts (truncated), got %d", out.TotalFound)
	}
}

func TestKnowledgeScout_RelevanceScoring(t *testing.T) {
	k, ctx, mkk, _ := ppSetupWithKnowledge(t)

	// fact-a matches both query terms, fact-b matches one.
	mkk.facts["fact-a"] = mockFact{content: "Tool building and purpose discovery", confidence: 700_000, citations: 5}
	mkk.facts["fact-b"] = mockFact{content: "Verification process overview", confidence: 700_000, citations: 5}
	mkk.searchIDs = []string{"fact-a", "fact-b"}

	out, err := k.KnowledgeScout(ctx, &types.KnowledgeScoutInput{
		QueryTerms:   []string{"tool", "purpose"},
		Capabilities: []string{"tool_building"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(out.Facts) < 2 {
		t.Fatalf("expected 2 facts")
	}
	// fact-a has both term overlap AND capability relevance.
	if out.Facts[0].FactId != "fact-a" {
		t.Errorf("expected fact-a first (higher relevance), got %s", out.Facts[0].FactId)
	}
	if out.Facts[0].RelevanceScore <= out.Facts[1].RelevanceScore {
		t.Errorf("fact-a should have higher relevance: %d vs %d", out.Facts[0].RelevanceScore, out.Facts[1].RelevanceScore)
	}
}

// ========== Purpose Analyzer Tests ==========

func TestAnalyzePurpose_NilInput(t *testing.T) {
	analysis := keeper.AnalyzePurpose(nil)
	if analysis == nil || analysis.PrimaryPurpose == nil {
		t.Fatal("expected fallback analysis")
	}
	if analysis.PrimaryPurpose.Confidence >= 200_000 {
		t.Errorf("expected low fallback confidence, got %d", analysis.PrimaryPurpose.Confidence)
	}
}

func TestAnalyzePurpose_BuilderTemplate(t *testing.T) {
	analysis := keeper.AnalyzePurpose(&types.PurposeAnalyzerInput{
		AgentCapabilities: []string{"tool_building", "programming"},
	})
	if analysis.PrimaryPurpose == nil {
		t.Fatal("expected primary purpose")
	}
	if analysis.PrimaryPurpose.Statement != "Build tools that empower other agents" {
		t.Errorf("expected builder statement, got: %s", analysis.PrimaryPurpose.Statement)
	}
}

func TestAnalyzePurpose_VerifierTemplate(t *testing.T) {
	analysis := keeper.AnalyzePurpose(&types.PurposeAnalyzerInput{
		AgentCapabilities: []string{"verification", "fact_checking", "research"},
	})
	if analysis.PrimaryPurpose == nil {
		t.Fatal("expected primary purpose")
	}
	if analysis.PrimaryPurpose.Statement != "Verify knowledge and maintain epistemic integrity" {
		t.Errorf("expected verifier statement, got: %s", analysis.PrimaryPurpose.Statement)
	}
}

func TestAnalyzePurpose_MultipleTemplates(t *testing.T) {
	analysis := keeper.AnalyzePurpose(&types.PurposeAnalyzerInput{
		AgentCapabilities: []string{"tool_building", "verification"}, // Matches builder and verifier.
	})
	if analysis.PrimaryPurpose == nil {
		t.Fatal("expected primary purpose")
	}
	if len(analysis.Alternatives) == 0 {
		t.Error("expected alternative hypotheses")
	}
	// Primary should have higher confidence than alternatives.
	for _, alt := range analysis.Alternatives {
		if alt.Confidence > analysis.PrimaryPurpose.Confidence {
			t.Errorf("alternative %q has higher confidence (%d) than primary (%d)",
				alt.Statement, alt.Confidence, analysis.PrimaryPurpose.Confidence)
		}
	}
}

func TestAnalyzePurpose_NoMatchFallback(t *testing.T) {
	analysis := keeper.AnalyzePurpose(&types.PurposeAnalyzerInput{
		AgentCapabilities: []string{"unknown_capability"},
	})
	if analysis.PrimaryPurpose == nil {
		t.Fatal("expected fallback purpose")
	}
	if analysis.PrimaryPurpose.Confidence >= 200_000 {
		t.Errorf("expected low exploratory confidence, got %d", analysis.PrimaryPurpose.Confidence)
	}
}

func TestAnalyzePurpose_CapabilityGaps(t *testing.T) {
	analysis := keeper.AnalyzePurpose(&types.PurposeAnalyzerInput{
		AgentCapabilities: []string{"tool_building"}, // Has builder cap but not gap caps.
	})
	if len(analysis.CapabilityGaps) == 0 {
		t.Error("expected capability gaps for builder template")
	}
	// Builder template gap caps: testing, documentation, collaboration.
	gapSet := make(map[string]bool)
	for _, g := range analysis.CapabilityGaps {
		gapSet[g] = true
	}
	for _, expected := range []string{"testing", "documentation", "collaboration"} {
		if !gapSet[expected] {
			t.Errorf("expected gap %q not found", expected)
		}
	}
}

func TestAnalyzePurpose_GrowthRecommendations(t *testing.T) {
	analysis := keeper.AnalyzePurpose(&types.PurposeAnalyzerInput{
		AgentCapabilities: []string{"tool_building"},
	})
	if len(analysis.GrowthPath) == 0 {
		t.Error("expected growth recommendations")
	}
	for _, rec := range analysis.GrowthPath {
		if rec.Capability == "" {
			t.Error("growth recommendation has empty capability")
		}
		if rec.Rationale == "" {
			t.Error("growth recommendation has empty rationale")
		}
		if rec.TargetTier == "" {
			t.Error("growth recommendation has empty target tier")
		}
		if rec.EstimatedEpochs == 0 {
			t.Error("growth recommendation has zero epochs")
		}
	}
}

func TestAnalyzePurpose_ConfidenceFormula(t *testing.T) {
	// With strong alignment, some demand, and history, confidence should be substantial.
	analysis := keeper.AnalyzePurpose(&types.PurposeAnalyzerInput{
		AgentCapabilities: []string{"tool_building", "programming", "software_development", "engineering"},
		KnowledgeFacts: []*types.ScoredFact{
			{FactId: "f1", Content: "Tools empower agents in the ecosystem", Confidence: 800_000},
		},
		AgentHistory: &types.AgentHistory{
			ToolsDeployed: 5,
			TotalToolCalls: 50,
		},
	})
	if analysis.OverallConfidence == 0 {
		t.Error("expected non-zero overall confidence")
	}
	// With 4/4 capability alignment (100%), the 40% weight alone gives ~400K.
	if analysis.OverallConfidence < 300_000 {
		t.Errorf("expected substantial confidence with full alignment, got %d", analysis.OverallConfidence)
	}
}

func TestAnalyzePurpose_HistoryBoost(t *testing.T) {
	// Same capabilities, different history.
	withoutHistory := keeper.AnalyzePurpose(&types.PurposeAnalyzerInput{
		AgentCapabilities: []string{"tool_building"},
	})
	withHistory := keeper.AnalyzePurpose(&types.PurposeAnalyzerInput{
		AgentCapabilities: []string{"tool_building"},
		AgentHistory: &types.AgentHistory{
			ToolsDeployed:  10,
			TotalToolCalls: 100,
		},
	})
	if withHistory.OverallConfidence <= withoutHistory.OverallConfidence {
		t.Errorf("history should boost confidence: with=%d, without=%d",
			withHistory.OverallConfidence, withoutHistory.OverallConfidence)
	}
}

// ========== Path Formatter Tests ==========

func TestFormatPath_NilAnalysis(t *testing.T) {
	path := keeper.FormatPath(nil, "", 100)
	if path == nil {
		t.Fatal("expected non-nil path")
	}
	if path.CurrentState != "Unknown" {
		t.Errorf("expected 'Unknown' current state, got %q", path.CurrentState)
	}
}

func TestFormatPath_WithGrowthPath(t *testing.T) {
	analysis := &types.PurposeAnalysis{
		PrimaryPurpose: &types.PurposeHypothesis{
			Statement:  "Build tools that empower other agents",
			Confidence: 600_000,
		},
		GrowthPath: []*types.GrowthRecommendation{
			{Capability: "testing", Rationale: "Improve tool quality", TargetTier: "Bonded", EstimatedEpochs: 10},
			{Capability: "documentation", Rationale: "Help others use your tools", TargetTier: "Bonded", EstimatedEpochs: 8},
		},
		OverallConfidence: 600_000,
	}

	path := keeper.FormatPath(analysis, ppTestAddr("agent1"), 100)
	if len(path.Steps) != 2 {
		t.Errorf("expected 2 steps, got %d", len(path.Steps))
	}
	if path.Steps[0].StepNumber != 1 {
		t.Errorf("expected step 1, got %d", path.Steps[0].StepNumber)
	}
	if path.Steps[1].StepNumber != 2 {
		t.Errorf("expected step 2, got %d", path.Steps[1].StepNumber)
	}
	if path.Destination != "Build tools that empower other agents" {
		t.Errorf("wrong destination: %s", path.Destination)
	}
}

func TestFormatPath_ConfidenceLabels(t *testing.T) {
	tests := []struct {
		confidence uint64
		label      string
	}{
		{100_000, "Exploratory"},
		{200_000, "Exploratory"},
		{300_000, "Emerging"},
		{500_000, "Emerging"},
		{600_000, "Strong"},
		{750_000, "Strong"},
		{900_000, "Definitive"},
	}

	for _, tc := range tests {
		analysis := &types.PurposeAnalysis{
			PrimaryPurpose:    &types.PurposeHypothesis{Statement: "Test", Confidence: tc.confidence},
			OverallConfidence: tc.confidence,
		}
		path := keeper.FormatPath(analysis, "", 100)
		if path.CurrentState != tc.label {
			t.Errorf("confidence %d: expected %q, got %q", tc.confidence, tc.label, path.CurrentState)
		}
	}
}

func TestFormatPath_EstimatedEpochs(t *testing.T) {
	analysis := &types.PurposeAnalysis{
		PrimaryPurpose: &types.PurposeHypothesis{Statement: "Test", Confidence: 500_000},
		GrowthPath: []*types.GrowthRecommendation{
			{Capability: "a", EstimatedEpochs: 10, TargetTier: "Bonded"},
			{Capability: "b", EstimatedEpochs: 15, TargetTier: "Bonded"},
			{Capability: "c", EstimatedEpochs: 5, TargetTier: "Bonded"},
		},
		OverallConfidence: 500_000,
	}

	path := keeper.FormatPath(analysis, "", 100)
	if path.EstimatedEpochs != 30 {
		t.Errorf("expected 30 total epochs, got %d", path.EstimatedEpochs)
	}
}

// ========== Recommendations Tests ==========

func TestRecommendTools_NoUsage(t *testing.T) {
	k, ctx := ppSetupKeeper(t)
	recs := k.RecommendTools(ctx, ppTestAddr("nobody"), 10)
	if len(recs) != 0 {
		t.Errorf("expected 0 recommendations, got %d", len(recs))
	}
}

func TestRecommendTools_CoUserFiltering(t *testing.T) {
	k, ctx := ppSetupKeeper(t)

	agent1 := ppTestAddr("agent1")
	agent2 := ppTestAddr("agent2")

	// Set up tools.
	toolA := &types.Tool{Id: "tool-a", Name: "Tool A", Status: types.ToolStatusActive, TrustScore: 500_000, Deployer: "system"}
	toolB := &types.Tool{Id: "tool-b", Name: "Tool B", Status: types.ToolStatusActive, TrustScore: 500_000, Deployer: "system"}
	toolC := &types.Tool{Id: "tool-c", Name: "Tool C", Status: types.ToolStatusActive, TrustScore: 500_000, Deployer: "system"}
	k.SetTool(ctx, toolA)
	k.SetTool(ctx, toolB)
	k.SetTool(ctx, toolC)

	// agent1 uses tool-a, agent2 uses tool-a and tool-b.
	k.SetAgentActiveTools(ctx, agent1, []string{"tool-a"})
	k.SetAgentActiveTools(ctx, agent2, []string{"tool-a", "tool-b", "tool-c"})

	// Record agent2 as caller of tool-a (makes them a co-user of agent1).
	k.RecordCaller(ctx, "tool-a", agent2, 100, true)

	recs := k.RecommendTools(ctx, agent1, 10)
	// agent2 is a co-user → tool-b and tool-c should be recommended (agent1 doesn't have them).
	if len(recs) == 0 {
		t.Error("expected recommendations from co-user")
	}

	recIDs := make(map[string]bool)
	for _, r := range recs {
		recIDs[r.ToolID] = true
	}
	// tool-a should NOT be recommended (agent1 already has it).
	if recIDs["tool-a"] {
		t.Error("should not recommend tool agent already has")
	}
}

func TestMatchToolsForAgent_NilDiscovery(t *testing.T) {
	k, ctx := ppSetupKeeper(t)
	// No discovery keeper set.

	// Tool with no required capabilities should be returned.
	k.SetTool(ctx, &types.Tool{Id: "t1", Name: "Open Tool", Status: types.ToolStatusActive, Deployer: "system"})
	// Tool with required capabilities should NOT be returned (no way to check).
	k.SetTool(ctx, &types.Tool{Id: "t2", Name: "Restricted", Status: types.ToolStatusActive, Deployer: "system", RequiredCapabilities: []string{"special"}})

	matched := k.MatchToolsForAgent(ctx, ppTestAddr("agent"), 10)
	if len(matched) != 1 {
		t.Errorf("expected 1 matched tool (open), got %d", len(matched))
	}
	if len(matched) > 0 && matched[0].Id != "t1" {
		t.Errorf("expected t1, got %s", matched[0].Id)
	}
}

func TestMatchToolsForAgent_CapabilitySubset(t *testing.T) {
	k, ctx, _, mdk := ppSetupWithKnowledge(t)

	agent := ppTestAddr("capable-agent")
	mdk.agents[agent] = []string{"programming", "testing"}

	k.SetTool(ctx, &types.Tool{Id: "t1", Name: "Needs prog", Status: types.ToolStatusActive, Deployer: "system", RequiredCapabilities: []string{"programming"}})
	k.SetTool(ctx, &types.Tool{Id: "t2", Name: "Needs special", Status: types.ToolStatusActive, Deployer: "system", RequiredCapabilities: []string{"special_ability"}})
	k.SetTool(ctx, &types.Tool{Id: "t3", Name: "No reqs", Status: types.ToolStatusActive, Deployer: "system"})

	matched := k.MatchToolsForAgent(ctx, agent, 10)

	matchedIDs := make(map[string]bool)
	for _, m := range matched {
		matchedIDs[m.Id] = true
	}

	if !matchedIDs["t1"] {
		t.Error("expected t1 (agent has programming)")
	}
	if matchedIDs["t2"] {
		t.Error("should not match t2 (agent lacks special_ability)")
	}
	if !matchedIDs["t3"] {
		t.Error("expected t3 (no required caps)")
	}
}

// ========== Composite Pipeline Tests ==========

func TestExecuteCompositeTool_FullPipeline(t *testing.T) {
	k, ctx, mkk, mdk := ppSetupWithKnowledge(t)

	agent := ppTestAddr("purpose-seeker")
	mdk.agents[agent] = []string{"tool_building", "programming"}

	mkk.facts["f1"] = mockFact{content: "Building tools empowers the network", confidence: 750_000, citations: 5}
	mkk.searchIDs = []string{"f1"}

	// Create the composite tool.
	tool := &types.Tool{
		Id:             "purpose-prompter",
		Name:           "Purpose Prompter",
		ToolType:       types.ToolTypeComposite,
		Status:         types.ToolStatusActive,
		KnowledgeQuery: "agent_purpose",
		Deployer:       "system",
	}
	k.SetTool(ctx, tool)

	output, err := k.ExecuteCompositeTool(ctx, tool, agent, nil)
	if err != nil {
		t.Fatalf("ExecuteCompositeTool failed: %v", err)
	}

	var result types.PurposePrompterOutput
	if err := json.Unmarshal(output, &result); err != nil {
		t.Fatalf("failed to unmarshal output: %v", err)
	}

	if result.Analysis == nil {
		t.Error("expected analysis in output")
	}
	if result.Path == nil {
		t.Error("expected path in output")
	}
	if result.Methodology == "" {
		t.Error("expected methodology in output")
	}
	if result.Analysis != nil && result.Analysis.PrimaryPurpose != nil {
		if result.Analysis.PrimaryPurpose.Statement == "" {
			t.Error("expected non-empty purpose statement")
		}
	}
}

func TestExecuteCompositeTool_NonComposite(t *testing.T) {
	k, ctx := ppSetupKeeper(t)
	tool := &types.Tool{
		Id:       "not-composite",
		ToolType: types.ToolTypeBVMContract,
	}
	_, err := k.ExecuteCompositeTool(ctx, tool, ppTestAddr("agent"), nil)
	if err == nil {
		t.Error("expected error for non-composite tool")
	}
}

// ========== Ethical Boundary Tests ==========

func TestEthicalBoundaries(t *testing.T) {
	facts := []*types.ScoredFact{
		{FactId: "f1", Content: "Ethics require informed consent for data usage"},
		{FactId: "f2", Content: "Network performance metrics for 2025"},
		{FactId: "f3", Content: "Bias in AI models can cause harm"},
	}

	boundaries := keeper.ExtractBoundaries(facts)
	if len(boundaries) != 2 {
		t.Errorf("expected 2 boundaries (ethics + harm/bias), got %d", len(boundaries))
	}

	// Check that non-ethics fact is excluded.
	for _, b := range boundaries {
		if b.SupportingFact == "f2" {
			t.Error("should not create boundary from non-ethics fact")
		}
	}
}

// ========== Ego Inflation Tests ==========

func TestEgoInflation(t *testing.T) {
	// High deployment, no verification.
	warnings := keeper.CheckEgoInflation(&types.AgentHistory{
		ToolsDeployed:      15,
		TotalVerifications: 2,
		TotalToolCalls:     200,
	})
	if len(warnings) == 0 {
		t.Error("expected ego inflation warnings for high deploy/low verify")
	}

	// Balanced agent — should have no warnings.
	warnings = keeper.CheckEgoInflation(&types.AgentHistory{
		ToolsDeployed:      3,
		TotalVerifications: 20,
		TotalToolCalls:     50,
		ActiveDomains:      []string{"math", "physics"},
	})
	if len(warnings) != 0 {
		t.Errorf("expected no warnings for balanced agent, got %d: %v", len(warnings), warnings)
	}

	// Nil history.
	warnings = keeper.CheckEgoInflation(nil)
	if len(warnings) != 0 {
		t.Errorf("expected no warnings for nil history, got %d", len(warnings))
	}
}

// ============================================================
// Extended Purpose Analyzer Tests (9 tests)
// ============================================================

func TestPP_Analyzer_DeveloperAgent(t *testing.T) {
	analysis := keeper.AnalyzePurpose(&types.PurposeAnalyzerInput{
		AgentCapabilities: []string{"programming", "software_development", "engineering"},
	})
	if analysis == nil || analysis.PrimaryPurpose == nil {
		t.Fatal("expected analysis with primary purpose")
	}
	// Developer-oriented capabilities should match the builder template.
	if analysis.PrimaryPurpose.Statement != "Build tools that empower other agents" {
		t.Errorf("developer agent: expected builder purpose, got %q", analysis.PrimaryPurpose.Statement)
	}
}

func TestPP_Analyzer_VerifierAgent(t *testing.T) {
	analysis := keeper.AnalyzePurpose(&types.PurposeAnalyzerInput{
		AgentCapabilities: []string{"fact_checking", "analysis", "research"},
	})
	if analysis == nil || analysis.PrimaryPurpose == nil {
		t.Fatal("expected analysis with primary purpose")
	}
	// Verify-oriented capabilities should match the verifier template.
	if analysis.PrimaryPurpose.Statement != "Verify knowledge and maintain epistemic integrity" {
		t.Errorf("verifier agent: expected verifier purpose, got %q", analysis.PrimaryPurpose.Statement)
	}
}

func TestPP_Analyzer_EgoCheck(t *testing.T) {
	// Agent that deploys many tools but never verifies -> ego inflation.
	warnings := keeper.CheckEgoInflation(&types.AgentHistory{
		ToolsDeployed:      20,
		TotalVerifications: 1,
		TotalToolCalls:     500,
	})
	if len(warnings) == 0 {
		t.Error("expected ego inflation warnings for prolific deployer with no verification")
	}

	// Check that both deployment ratio and consumption warnings fire.
	foundDeployRatio := false
	foundConsumption := false
	for _, w := range warnings {
		if len(w) > 0 {
			// The deployment warning mentions "low verification ratio".
			if !foundDeployRatio && containsStr(w, "verification") {
				foundDeployRatio = true
			}
			// The consumption warning mentions "without any verification".
			if !foundConsumption && containsStr(w, "consumption") {
				foundConsumption = true
			}
		}
	}
	if !foundDeployRatio {
		t.Error("expected warning about low verification ratio")
	}
}

func TestPP_Analyzer_NoHistory(t *testing.T) {
	// New agent with unknown capabilities -> exploratory fallback.
	analysis := keeper.AnalyzePurpose(&types.PurposeAnalyzerInput{
		AgentCapabilities: []string{"brand_new_skill"},
	})
	if analysis == nil || analysis.PrimaryPurpose == nil {
		t.Fatal("expected fallback analysis")
	}
	// No matching template -> exploratory confidence (< 200K).
	if analysis.OverallConfidence >= 200_000 {
		t.Errorf("new agent: expected exploratory confidence (< 200K), got %d", analysis.OverallConfidence)
	}
}

func TestPP_Analyzer_CitesAllFacts(t *testing.T) {
	facts := []*types.ScoredFact{
		{FactId: "f1", Content: "Building tools empowers agents", Confidence: 700_000},
		{FactId: "f2", Content: "Programming creates value", Confidence: 800_000},
		{FactId: "f3", Content: "Engineering solves problems", Confidence: 750_000},
	}

	citations := keeper.BuildCitations(facts, "purpose_analysis")
	if len(citations) != 3 {
		t.Fatalf("expected 3 citations, got %d", len(citations))
	}

	// Verify all facts are cited.
	citedIDs := make(map[string]bool)
	for _, c := range citations {
		citedIDs[c.FactID] = true
		if c.UsedFor != "purpose_analysis" {
			t.Errorf("expected usedFor 'purpose_analysis', got %q", c.UsedFor)
		}
		if c.Content == "" {
			t.Error("citation should include content")
		}
		if c.Confidence == 0 {
			t.Error("citation should include confidence")
		}
	}
	for _, f := range facts {
		if !citedIDs[f.FactId] {
			t.Errorf("fact %s not cited", f.FactId)
		}
	}
}

func TestPP_ConfidenceLabel_Exploratory(t *testing.T) {
	// 0-200K should be "Exploratory".
	for _, conf := range []uint64{0, 100_000, 200_000} {
		label := types.ConfidenceLabel(conf)
		if label != "Exploratory" {
			t.Errorf("ConfidenceLabel(%d): expected 'Exploratory', got %q", conf, label)
		}
	}
}

func TestPP_ConfidenceLabel_Emerging(t *testing.T) {
	// 200,001-500,000 should be "Emerging".
	for _, conf := range []uint64{200_001, 350_000, 500_000} {
		label := types.ConfidenceLabel(conf)
		if label != "Emerging" {
			t.Errorf("ConfidenceLabel(%d): expected 'Emerging', got %q", conf, label)
		}
	}
}

func TestPP_ConfidenceLabel_Strong(t *testing.T) {
	// 500,001-750,000 should be "Strong".
	for _, conf := range []uint64{500_001, 625_000, 750_000} {
		label := types.ConfidenceLabel(conf)
		if label != "Strong" {
			t.Errorf("ConfidenceLabel(%d): expected 'Strong', got %q", conf, label)
		}
	}
}

func TestPP_ConfidenceLabel_Definitive(t *testing.T) {
	// >750,000 should be "Definitive".
	for _, conf := range []uint64{750_001, 900_000, 1_000_000} {
		label := types.ConfidenceLabel(conf)
		if label != "Definitive" {
			t.Errorf("ConfidenceLabel(%d): expected 'Definitive', got %q", conf, label)
		}
	}
}

// containsStr is a simple string containment helper for test assertions.
func containsStr(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && strings.Contains(s, substr))
}
