package keeper_test

import (
	"context"
	"fmt"
	"testing"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/zerone-chain/zerone/x/capture_defense/keeper"
	"github.com/zerone-chain/zerone/x/capture_defense/types"
)

// ---------- Mock CaptureChallengeKeeper for auto-challenge tests ----------

type mockChallengeKeeper struct {
	calls []autoChallengCall
}

type autoChallengCall struct {
	domain    string
	riskScore uint64
	hhi       uint64
	evidence  string
}

func newMockChallengeKeeper() *mockChallengeKeeper {
	return &mockChallengeKeeper{}
}

func (m *mockChallengeKeeper) AutoSubmitChallenge(_ context.Context, domain string, riskScore uint64, hhi uint64, evidence string) error {
	m.calls = append(m.calls, autoChallengCall{
		domain:    domain,
		riskScore: riskScore,
		hhi:       hhi,
		evidence:  evidence,
	})
	return nil
}

// ======================================================================
// Test 1: RecordVerificationFromKnowledge stores history
// ======================================================================

func TestRecordVerificationFromKnowledge(t *testing.T) {
	k, ctx := setupKeeper(t)

	k.RecordVerificationFromKnowledge(
		ctx,
		"physics",
		"r1",
		[]string{testAddr(1), testAddr(2)},
		[]bool{true, false},
		nil, // no submit blocks
	)

	entries := k.GetHistoryByDomain(ctx, "physics")
	require.Len(t, entries, 1, "expected exactly 1 history entry for physics")

	entry := entries[0]
	assert.Equal(t, "physics", entry.Domain)
	assert.Equal(t, "r1", entry.RoundId)
	assert.Equal(t, []string{testAddr(1), testAddr(2)}, entry.Validators)
	assert.Equal(t, []bool{true, false}, entry.Verdicts)
	assert.Equal(t, uint64(100), entry.BlockHeight, "expected block height from context")
}

func TestRecordVerificationFromKnowledge_MultipleRounds(t *testing.T) {
	k, ctx := setupKeeper(t)

	for i := 0; i < 3; i++ {
		k.RecordVerificationFromKnowledge(
			ctx,
			"mathematics",
			fmt.Sprintf("round-%d", i),
			[]string{testAddr(1)},
			[]bool{true},
			nil,
		)
	}

	entries := k.GetHistoryByDomain(ctx, "mathematics")
	require.Len(t, entries, 3, "expected 3 history entries")

	// Also verify individual retrieval
	got, found := k.GetVerificationHistory(ctx, "mathematics", "round-1")
	require.True(t, found)
	assert.Equal(t, "mathematics", got.Domain)
	assert.Equal(t, "round-1", got.RoundId)
}

// ======================================================================
// Test 2: UpdateReputation changes global scores
// ======================================================================

func TestUpdateReputation_GlobalScore(t *testing.T) {
	k, ctx := setupKeeper(t)
	validator := testAddr(1)

	// First: approval increases score above initial 500000
	k.UpdateReputation(ctx, validator, "physics", "", true)

	gr, found := k.GetGlobalReputation(ctx, validator)
	require.True(t, found, "global reputation should exist after UpdateReputation")
	assert.Greater(t, gr.Score, uint64(500000), "score should increase after approval")
	scoreAfterApproval := gr.Score

	// Second: rejection decreases score
	k.UpdateReputation(ctx, validator, "physics", "", false)

	gr2, _ := k.GetGlobalReputation(ctx, validator)
	assert.Less(t, gr2.Score, scoreAfterApproval, "score should decrease after rejection")
}

func TestUpdateReputation_DomainAndStratum(t *testing.T) {
	k, ctx := setupKeeper(t)
	validator := testAddr(1)

	k.UpdateReputation(ctx, validator, "biology", "empirical", true)

	// Domain reputation created
	dr, found := k.GetDomainReputation(ctx, "biology", validator)
	require.True(t, found)
	assert.Greater(t, dr.Score, uint64(500000))
	assert.Equal(t, uint64(1), dr.Verifications)

	// Stratum reputation created
	sr, found := k.GetStratumReputation(ctx, "empirical", validator)
	require.True(t, found)
	assert.Greater(t, sr.Score, uint64(500000))
	assert.Equal(t, uint64(1), sr.Verifications)
}

// ======================================================================
// Test 3: AnalyzeCaptureRisk detects monopoly
// ======================================================================

func TestAnalyzeCaptureRisk_Monopoly(t *testing.T) {
	k, ctx := setupKeeper(t)

	// Record 10 history entries where only val1 participates
	for i := 0; i < 10; i++ {
		k.RecordVerificationFromKnowledge(
			ctx,
			"captured_domain",
			fmt.Sprintf("round-%d", i),
			[]string{testAddr(1)},
			[]bool{true},
			nil,
		)
	}

	params := k.GetParams(ctx)
	metrics := k.AnalyzeCaptureRisk(ctx, "captured_domain", params)

	require.NotNil(t, metrics, "metrics should not be nil for domain with history")
	assert.True(t, metrics.Flagged, "monopoly domain should be flagged")
	assert.Equal(t, uint64(types.BPSScale), metrics.HerfindahlIndex,
		"HHI for a monopoly should be BPSScale (1,000,000)")
	assert.Equal(t, "captured_domain", metrics.Domain)
	assert.Greater(t, metrics.RiskScore, uint64(0), "risk score should be non-zero")
}

// ======================================================================
// Test 4: AnalyzeCaptureRisk healthy diversity
// ======================================================================

func TestAnalyzeCaptureRisk_Diverse(t *testing.T) {
	k, ctx := setupKeeper(t)

	// Record 10 history entries with 5 different validators evenly (2 rounds each)
	for i := 0; i < 10; i++ {
		valIdx := i % 5 // cycles through val0..val4
		k.RecordVerificationFromKnowledge(
			ctx,
			"diverse_domain",
			fmt.Sprintf("round-%d", i),
			[]string{testAddr(valIdx)},
			[]bool{true},
			nil,
		)
	}

	params := k.GetParams(ctx)
	metrics := k.AnalyzeCaptureRisk(ctx, "diverse_domain", params)

	require.NotNil(t, metrics)
	assert.False(t, metrics.Flagged, "diverse domain should NOT be flagged")

	// HHI for 5 equal validators: 5 * (200,000)^2 / 1,000,000 = 200,000
	// Allow some tolerance
	assert.LessOrEqual(t, metrics.HerfindahlIndex, uint64(250000),
		"HHI should be at or below 250,000 for 5 equal validators")
	assert.GreaterOrEqual(t, metrics.HerfindahlIndex, uint64(150000),
		"HHI should be around 200,000 for 5 equal validators")
}

// ======================================================================
// Test 5: RunAutoAnalysis submits challenge when flagged
// ======================================================================

func TestRunAutoAnalysis_AutoChallenge(t *testing.T) {
	k, ctx := setupKeeper(t)

	// Populate history with monopoly data
	for i := 0; i < 10; i++ {
		k.RecordVerificationFromKnowledge(
			ctx,
			"monopoly_domain",
			fmt.Sprintf("round-%d", i),
			[]string{testAddr(1)},
			[]bool{true},
			nil,
		)
	}

	// Wire a mock challenge keeper that records calls
	mock := newMockChallengeKeeper()
	k.SetChallengeKeeper(mock)

	params := k.GetParams(ctx)
	k.RunAutoAnalysis(ctx, params)

	// Assert: mock received exactly 1 AutoSubmitChallenge call
	require.Len(t, mock.calls, 1, "expected exactly 1 auto-challenge submission")
	assert.Equal(t, "monopoly_domain", mock.calls[0].domain,
		"auto-challenge should target the monopoly domain")
	assert.Equal(t, uint64(types.BPSScale), mock.calls[0].hhi,
		"HHI passed should be BPSScale for monopoly")
	assert.Greater(t, mock.calls[0].riskScore, uint64(0),
		"risk score should be non-zero")
}

func TestRunAutoAnalysis_NoChallengeWhenDiverse(t *testing.T) {
	k, ctx := setupKeeper(t)

	// Diverse domain with 10 validators
	for i := 0; i < 10; i++ {
		k.RecordVerificationFromKnowledge(
			ctx,
			"healthy_domain",
			fmt.Sprintf("round-%d", i),
			[]string{testAddr(i)},
			[]bool{true},
			nil,
		)
	}

	mock := newMockChallengeKeeper()
	k.SetChallengeKeeper(mock)

	params := k.GetParams(ctx)
	k.RunAutoAnalysis(ctx, params)

	assert.Len(t, mock.calls, 0, "no auto-challenge should be submitted for a diverse domain")
}

func TestRunAutoAnalysis_NoChallengeWithoutKeeper(t *testing.T) {
	k, ctx := setupKeeper(t)

	// Monopoly data but no challenge keeper set
	for i := 0; i < 10; i++ {
		k.RecordVerificationFromKnowledge(
			ctx,
			"mono_no_keeper",
			fmt.Sprintf("round-%d", i),
			[]string{testAddr(1)},
			[]bool{true},
			nil,
		)
	}

	// Don't set challenge keeper -- should not panic
	params := k.GetParams(ctx)
	k.RunAutoAnalysis(ctx, params) // should complete without error

	// Metrics should still be stored
	m, found := k.GetCaptureMetrics(ctx, "mono_no_keeper")
	require.True(t, found, "metrics should be stored even without challenge keeper")
	assert.True(t, m.Flagged, "domain should still be flagged")
}

// ======================================================================
// Test 6: ClearCaptureFlag works
// ======================================================================

func TestClearCaptureFlag(t *testing.T) {
	k, ctx := setupKeeper(t)

	// Set capture metrics with Flagged=true
	k.SetCaptureMetrics(ctx, &types.CaptureMetrics{
		Domain:          "flagged_domain",
		HerfindahlIndex: 800000,
		RiskScore:       600000,
		Flagged:         true,
		AnalyzedAtBlock: 100,
	})

	// Verify it's flagged
	m, found := k.GetCaptureMetrics(ctx, "flagged_domain")
	require.True(t, found)
	require.True(t, m.Flagged)

	// Clear the flag
	k.ClearCaptureFlag(ctx, "flagged_domain")

	// Verify flag cleared
	m2, found := k.GetCaptureMetrics(ctx, "flagged_domain")
	require.True(t, found, "metrics should still exist after clearing flag")
	assert.False(t, m2.Flagged, "Flagged should be false after ClearCaptureFlag")

	// Other fields preserved
	assert.Equal(t, uint64(800000), m2.HerfindahlIndex)
	assert.Equal(t, uint64(600000), m2.RiskScore)
}

func TestClearCaptureFlag_Nonexistent(t *testing.T) {
	k, ctx := setupKeeper(t)

	// Should not panic on non-existent domain
	k.ClearCaptureFlag(ctx, "nonexistent_domain")

	_, found := k.GetCaptureMetrics(ctx, "nonexistent_domain")
	assert.False(t, found, "no metrics should be created for non-existent domain")
}

// ======================================================================
// Test 7: GetFlaggedDomainCount
// ======================================================================

func TestGetFlaggedDomainCount(t *testing.T) {
	k, ctx := setupKeeper(t)

	// Set 3 domains flagged, 2 not flagged
	k.SetCaptureMetrics(ctx, &types.CaptureMetrics{
		Domain: "domain_a", Flagged: true, HerfindahlIndex: 500000,
	})
	k.SetCaptureMetrics(ctx, &types.CaptureMetrics{
		Domain: "domain_b", Flagged: true, HerfindahlIndex: 600000,
	})
	k.SetCaptureMetrics(ctx, &types.CaptureMetrics{
		Domain: "domain_c", Flagged: true, HerfindahlIndex: 700000,
	})
	k.SetCaptureMetrics(ctx, &types.CaptureMetrics{
		Domain: "domain_d", Flagged: false, HerfindahlIndex: 100000,
	})
	k.SetCaptureMetrics(ctx, &types.CaptureMetrics{
		Domain: "domain_e", Flagged: false, HerfindahlIndex: 50000,
	})

	count := k.GetFlaggedDomainCount(ctx)
	assert.Equal(t, uint64(3), count, "expected 3 flagged domains")
}

func TestGetFlaggedDomainCount_None(t *testing.T) {
	k, ctx := setupKeeper(t)

	count := k.GetFlaggedDomainCount(ctx)
	assert.Equal(t, uint64(0), count, "expected 0 flagged domains when no metrics exist")
}

func TestGetFlaggedDomainCount_AfterClear(t *testing.T) {
	k, ctx := setupKeeper(t)

	k.SetCaptureMetrics(ctx, &types.CaptureMetrics{
		Domain: "flagged_1", Flagged: true,
	})
	k.SetCaptureMetrics(ctx, &types.CaptureMetrics{
		Domain: "flagged_2", Flagged: true,
	})

	assert.Equal(t, uint64(2), k.GetFlaggedDomainCount(ctx))

	// Clear one
	k.ClearCaptureFlag(ctx, "flagged_1")

	assert.Equal(t, uint64(1), k.GetFlaggedDomainCount(ctx),
		"count should decrease after clearing a flag")
}

// ======================================================================
// Test: KnowledgeCaptureDefenseAdapter integration
// ======================================================================

func TestKnowledgeAdapter_RecordAndUpdate(t *testing.T) {
	k, ctx := setupKeeper(t)
	adapter := keeper.NewKnowledgeCaptureDefenseAdapter(k)

	// Use the adapter interface (context.Context, not sdk.Context)
	goCtx := sdk.WrapSDKContext(ctx)

	adapter.RecordVerificationHistory(
		goCtx,
		"chemistry",
		"round-1",
		[]string{testAddr(1), testAddr(2)},
		[]bool{true, true},
		[]uint64{100, 101},
	)

	// Verify via underlying keeper
	entries := k.GetHistoryByDomain(ctx, "chemistry")
	require.Len(t, entries, 1)
	assert.Equal(t, "chemistry", entries[0].Domain)

	// Update reputation via adapter
	adapter.UpdateReputation(goCtx, testAddr(1), "chemistry", "empirical", true)

	gr, found := k.GetGlobalReputation(ctx, testAddr(1))
	require.True(t, found)
	assert.Greater(t, gr.Score, uint64(500000))
}

// ======================================================================
// Test: ChallengeCaptureDefenseAdapter integration
// ======================================================================

func TestChallengeAdapter_GetMetricsAndClearFlag(t *testing.T) {
	k, ctx := setupKeeper(t)
	adapter := keeper.NewChallengeCaptureDefenseAdapter(k)

	// Set metrics
	k.SetCaptureMetrics(ctx, &types.CaptureMetrics{
		Domain:          "physics",
		HerfindahlIndex: 600000,
		RiskScore:       400000,
		Flagged:         true,
		AnalyzedAtBlock: 50,
	})

	goCtx := sdk.WrapSDKContext(ctx)

	// Read through adapter
	data, found := adapter.GetCaptureMetrics(goCtx, "physics")
	require.True(t, found)
	assert.Equal(t, "physics", data.Domain)
	assert.Equal(t, uint64(600000), data.HerfindahlIndex)
	assert.True(t, data.Flagged)

	// Clear flag through adapter
	adapter.ClearCaptureFlag(goCtx, "physics")

	// Verify flag cleared
	data2, found := adapter.GetCaptureMetrics(goCtx, "physics")
	require.True(t, found)
	assert.False(t, data2.Flagged, "flag should be cleared after adapter ClearCaptureFlag")
}

// ======================================================================
// Test: End-to-end knowledge round -> capture defense detection
// ======================================================================

func TestEndToEnd_KnowledgeRoundToCapture(t *testing.T) {
	k, ctx := setupKeeper(t)

	mockChallenge := newMockChallengeKeeper()
	k.SetChallengeKeeper(mockChallenge)

	// Simulate knowledge module calling RecordVerificationFromKnowledge
	// for 10 rounds with a monopoly validator
	for i := 0; i < 10; i++ {
		k.RecordVerificationFromKnowledge(
			ctx, "history_domain",
			fmt.Sprintf("kr-%d", i),
			[]string{testAddr(1)},
			[]bool{true},
			nil,
		)
		// Also update reputation as knowledge would
		k.UpdateReputation(ctx, testAddr(1), "history_domain", "", true)
	}

	// Run auto-analysis (as BeginBlocker would)
	params := k.GetParams(ctx)
	k.RunAutoAnalysis(ctx, params)

	// Verify: domain flagged
	m, found := k.GetCaptureMetrics(ctx, "history_domain")
	require.True(t, found)
	assert.True(t, m.Flagged)

	// Verify: auto-challenge submitted
	require.Len(t, mockChallenge.calls, 1)
	assert.Equal(t, "history_domain", mockChallenge.calls[0].domain)

	// Verify: validator reputation increased
	gr, found := k.GetGlobalReputation(ctx, testAddr(1))
	require.True(t, found)
	assert.Greater(t, gr.Score, uint64(500000),
		"reputation should be above base after 10 approvals")
	assert.Equal(t, uint64(10), gr.TotalVerifications)
}

// ======================================================================
// Test: Multiple domains, only flagged ones trigger challenge
// ======================================================================

func TestRunAutoAnalysis_MultipleDomains_SelectiveFlagging(t *testing.T) {
	k, ctx := setupKeeper(t)

	// Monopoly domain
	for i := 0; i < 10; i++ {
		k.RecordVerificationFromKnowledge(ctx, "mono_domain",
			fmt.Sprintf("m-%d", i), []string{testAddr(1)}, []bool{true}, nil)
	}

	// Diverse domain (10 different validators)
	for i := 0; i < 10; i++ {
		k.RecordVerificationFromKnowledge(ctx, "diverse_domain",
			fmt.Sprintf("d-%d", i), []string{testAddr(i + 10)}, []bool{true}, nil)
	}

	mock := newMockChallengeKeeper()
	k.SetChallengeKeeper(mock)

	params := k.GetParams(ctx)
	k.RunAutoAnalysis(ctx, params)

	// Only the monopoly domain should trigger a challenge
	require.Len(t, mock.calls, 1, "expected exactly 1 auto-challenge (monopoly only)")
	assert.Equal(t, "mono_domain", mock.calls[0].domain)

	// Diverse domain should not be flagged
	dm, found := k.GetCaptureMetrics(ctx, "diverse_domain")
	require.True(t, found)
	assert.False(t, dm.Flagged)
}
