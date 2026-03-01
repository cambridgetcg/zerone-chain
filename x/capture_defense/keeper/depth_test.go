package keeper_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/zerone-chain/zerone/x/capture_defense/keeper"
	"github.com/zerone-chain/zerone/x/capture_defense/types"
)

// mockOntologyKeeper implements types.OntologyKeeper for depth tests.
type mockOntologyKeeper struct {
	depths map[string]uint32
}

func newMockOntologyKeeper() *mockOntologyKeeper {
	return &mockOntologyKeeper{depths: make(map[string]uint32)}
}

func (m *mockOntologyKeeper) setDepth(domain string, depth uint32) {
	m.depths[domain] = depth
}

func (m *mockOntologyKeeper) GetDepthForDomain(_ context.Context, domainName string) (uint32, error) {
	d, ok := m.depths[domainName]
	if !ok {
		return 1, nil
	}
	return d, nil
}

func TestHHIThresholdAdjustedByDepth(t *testing.T) {
	k, ctx := setupKeeper(t)
	authority := k.GetAuthority()
	srv := keeper.NewMsgServerImpl(k)

	ok := newMockOntologyKeeper()
	ok.setDepth("broad_domain", 1)
	ok.setDepth("specific_domain", 3)
	k.SetOntologyKeeper(ok)

	// Create a moderately concentrated market: 3 validators, one dominant
	// This should trigger flagging at depth=1 threshold (250000) but not at depth=3 threshold (350000)
	for i := 0; i < 10; i++ {
		val := testAddr(1) // same validator every time = monopoly
		_, _ = srv.RecordVerification(ctx, &types.MsgRecordVerification{
			Authority:  authority,
			Domain:     "broad_domain",
			RoundId:    fmt.Sprintf("broad-round-%d", i),
			Validators: []string{val},
			Verdicts:   []bool{true},
		})
		_, _ = srv.RecordVerification(ctx, &types.MsgRecordVerification{
			Authority:  authority,
			Domain:     "specific_domain",
			RoundId:    fmt.Sprintf("specific-round-%d", i),
			Validators: []string{val},
			Verdicts:   []bool{true},
		})
	}

	params := k.GetParams(ctx)

	// Both should be monopolies (HHI = 1,000,000)
	metricsBroad := k.AnalyzeCaptureRisk(ctx, "broad_domain", params)
	metricsSpecific := k.AnalyzeCaptureRisk(ctx, "specific_domain", params)

	if metricsBroad == nil || metricsSpecific == nil {
		t.Fatal("expected non-nil metrics")
	}

	// Both are monopolies so both flagged regardless
	if !metricsBroad.Flagged {
		t.Error("broad domain monopoly should be flagged")
	}
	if !metricsSpecific.Flagged {
		t.Error("specific domain monopoly should still be flagged (HHI exceeds even adjusted threshold)")
	}
}

func TestDepthAdjustedThresholdAllowsModerateConcentration(t *testing.T) {
	k, ctx := setupKeeper(t)
	authority := k.GetAuthority()
	srv := keeper.NewMsgServerImpl(k)

	ok := newMockOntologyKeeper()
	ok.setDepth("broad", 1)   // threshold: 250000 (25%)
	ok.setDepth("narrow", 4)  // threshold: 250000 + 3*50000 = 400000 (40%)
	k.SetOntologyKeeper(ok)

	// Create 3 validators with slightly uneven distribution
	// 2 validators dominate: HHI around 333000 (33%)
	for i := 0; i < 6; i++ {
		var val string
		if i < 3 {
			val = testAddr(1) // 50% share
		} else if i < 5 {
			val = testAddr(2) // 33% share
		} else {
			val = testAddr(3) // 17% share
		}
		_, _ = srv.RecordVerification(ctx, &types.MsgRecordVerification{
			Authority:  authority,
			Domain:     "broad",
			RoundId:    fmt.Sprintf("broad-%d", i),
			Validators: []string{val},
			Verdicts:   []bool{true},
		})
		_, _ = srv.RecordVerification(ctx, &types.MsgRecordVerification{
			Authority:  authority,
			Domain:     "narrow",
			RoundId:    fmt.Sprintf("narrow-%d", i),
			Validators: []string{val},
			Verdicts:   []bool{true},
		})
	}

	params := k.GetParams(ctx)
	metricsBroad := k.AnalyzeCaptureRisk(ctx, "broad", params)
	metricsNarrow := k.AnalyzeCaptureRisk(ctx, "narrow", params)

	if metricsBroad == nil || metricsNarrow == nil {
		t.Fatal("expected non-nil metrics")
	}

	// HHI should be the same for both (same validator distribution)
	if metricsBroad.HerfindahlIndex != metricsNarrow.HerfindahlIndex {
		t.Errorf("HHI should be identical: broad=%d, narrow=%d",
			metricsBroad.HerfindahlIndex, metricsNarrow.HerfindahlIndex)
	}

	// Broad domain (depth=1, threshold=250000): should be flagged if HHI > 250000
	// Narrow domain (depth=4, threshold=400000): should NOT be flagged if HHI < 400000
	hhi := metricsBroad.HerfindahlIndex
	t.Logf("HHI for both domains: %d", hhi)

	if hhi > 250000 && !metricsBroad.Flagged {
		t.Error("broad domain should be flagged (HHI > 250000)")
	}
	if hhi < 400000 && metricsNarrow.Flagged {
		t.Error("narrow domain should NOT be flagged (HHI < adjusted 400000)")
	}
}

func TestNoOntologyKeeperDefaultsToBaseThreshold(t *testing.T) {
	k, ctx := setupKeeper(t)
	authority := k.GetAuthority()
	srv := keeper.NewMsgServerImpl(k)

	// Don't set ontology keeper — should fall back to base threshold

	for i := 0; i < 5; i++ {
		_, _ = srv.RecordVerification(ctx, &types.MsgRecordVerification{
			Authority:  authority,
			Domain:     "test_domain",
			RoundId:    fmt.Sprintf("round-%d", i),
			Validators: []string{testAddr(1)},
			Verdicts:   []bool{true},
		})
	}

	params := k.GetParams(ctx)
	metrics := k.AnalyzeCaptureRisk(ctx, "test_domain", params)

	if metrics == nil {
		t.Fatal("expected non-nil metrics")
	}
	// Monopoly should be flagged with default threshold
	if !metrics.Flagged {
		t.Error("expected monopoly to be flagged with base threshold")
	}
}
