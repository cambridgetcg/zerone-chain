package keeper_test

import (
	"testing"

	"github.com/zerone-chain/zerone/x/alignment/keeper"
	"github.com/zerone-chain/zerone/x/alignment/types"
)

// --- TestAdaptivePacing_FullLifecycle ---
// Walks through the complete health-category lifecycle and verifies that
// GetGlobalPacingMultiplier and the GlobalPacing gRPC query return the
// correct creation / analysis BPS multipliers at each stage:
//   healthy → degraded → critical → recovery (healthy) → disabled

func TestAdaptivePacing_FullLifecycle(t *testing.T) {
	k, _, ctx := setupKeeper(t)

	// --- Phase 1: Healthy → neutral multipliers ---
	k.SetState(ctx, &types.AlignmentState{
		Enabled:          true,
		PreviousCategory: types.CategoryHealthy,
	})

	creation, analysis := k.GetGlobalPacingMultiplier(ctx)
	if creation != types.BPS {
		t.Fatalf("healthy: expected creation=%d, got %d", types.BPS, creation)
	}
	if analysis != types.BPS {
		t.Fatalf("healthy: expected analysis=%d, got %d", types.BPS, analysis)
	}

	// --- Phase 2: Transition to degraded → slower creation, faster analysis ---
	k.SetState(ctx, &types.AlignmentState{
		Enabled:          true,
		PreviousCategory: types.CategoryDegraded,
	})

	creation, analysis = k.GetGlobalPacingMultiplier(ctx)
	if creation != 750_000 {
		t.Fatalf("degraded: expected creation=750000, got %d", creation)
	}
	if analysis != 1_500_000 {
		t.Fatalf("degraded: expected analysis=1500000, got %d", analysis)
	}

	// --- Phase 3: Transition to critical → doubled effects ---
	k.SetState(ctx, &types.AlignmentState{
		Enabled:          true,
		PreviousCategory: types.CategoryCritical,
	})

	creation, analysis = k.GetGlobalPacingMultiplier(ctx)
	if creation != 500_000 {
		t.Fatalf("critical: expected creation=500000, got %d", creation)
	}
	if analysis != 2_000_000 {
		t.Fatalf("critical: expected analysis=2000000, got %d", analysis)
	}

	// --- Phase 4: Recovery back to healthy → neutral again ---
	k.SetState(ctx, &types.AlignmentState{
		Enabled:          true,
		PreviousCategory: types.CategoryHealthy,
	})

	creation, analysis = k.GetGlobalPacingMultiplier(ctx)
	if creation != types.BPS {
		t.Fatalf("recovery: expected creation=%d, got %d", types.BPS, creation)
	}
	if analysis != types.BPS {
		t.Fatalf("recovery: expected analysis=%d, got %d", types.BPS, analysis)
	}

	// --- Phase 5: Disabled module → neutral regardless of category ---
	k.SetState(ctx, &types.AlignmentState{
		Enabled:          false,
		PreviousCategory: types.CategoryCritical,
	})

	creation, analysis = k.GetGlobalPacingMultiplier(ctx)
	if creation != types.BPS {
		t.Fatalf("disabled: expected creation=%d, got %d", types.BPS, creation)
	}
	if analysis != types.BPS {
		t.Fatalf("disabled: expected analysis=%d, got %d", types.BPS, analysis)
	}

	// --- Phase 6: Verify gRPC GlobalPacing query returns correct values at each state ---
	qs := keeper.NewQueryServerImpl(k)

	// Re-enable as degraded and query.
	k.SetState(ctx, &types.AlignmentState{
		Enabled:          true,
		PreviousCategory: types.CategoryDegraded,
	})

	resp, err := qs.GlobalPacing(ctx, &types.QueryGlobalPacingRequest{})
	if err != nil {
		t.Fatalf("GlobalPacing query failed: %v", err)
	}
	if resp.HealthCategory != types.CategoryDegraded {
		t.Errorf("query: expected category=%s, got %s", types.CategoryDegraded, resp.HealthCategory)
	}
	if resp.CreationMultiplierBps != 750_000 {
		t.Errorf("query: expected creation=750000, got %d", resp.CreationMultiplierBps)
	}
	if resp.AnalysisMultiplierBps != 1_500_000 {
		t.Errorf("query: expected analysis=1500000, got %d", resp.AnalysisMultiplierBps)
	}

	// Query at critical.
	k.SetState(ctx, &types.AlignmentState{
		Enabled:          true,
		PreviousCategory: types.CategoryCritical,
	})

	resp, err = qs.GlobalPacing(ctx, &types.QueryGlobalPacingRequest{})
	if err != nil {
		t.Fatalf("GlobalPacing query (critical) failed: %v", err)
	}
	if resp.HealthCategory != types.CategoryCritical {
		t.Errorf("query critical: expected category=%s, got %s", types.CategoryCritical, resp.HealthCategory)
	}
	if resp.CreationMultiplierBps != 500_000 {
		t.Errorf("query critical: expected creation=500000, got %d", resp.CreationMultiplierBps)
	}
	if resp.AnalysisMultiplierBps != 2_000_000 {
		t.Errorf("query critical: expected analysis=2000000, got %d", resp.AnalysisMultiplierBps)
	}

	// Query with empty category → defaults to healthy.
	k.SetState(ctx, &types.AlignmentState{
		Enabled:          true,
		PreviousCategory: "",
	})

	resp, err = qs.GlobalPacing(ctx, &types.QueryGlobalPacingRequest{})
	if err != nil {
		t.Fatalf("GlobalPacing query (empty) failed: %v", err)
	}
	if resp.HealthCategory != types.CategoryHealthy {
		t.Errorf("query empty: expected category=%s, got %s", types.CategoryHealthy, resp.HealthCategory)
	}
	if resp.CreationMultiplierBps != types.BPS {
		t.Errorf("query empty: expected creation=%d, got %d", types.BPS, resp.CreationMultiplierBps)
	}
	if resp.AnalysisMultiplierBps != types.BPS {
		t.Errorf("query empty: expected analysis=%d, got %d", types.BPS, resp.AnalysisMultiplierBps)
	}
}

// --- TestAdaptivePacing_IntervalCalculations ---
// Table-driven test verifying the BPS-to-interval math.
// Formula: effectiveInterval = baseInterval * BPS / pacingBps
//
// A higher pacingBps means more activity → shorter intervals.
// A lower pacingBps means less activity → longer intervals.

func TestAdaptivePacing_IntervalCalculations(t *testing.T) {
	tests := []struct {
		name               string
		category           string
		baseInterval       uint64
		expectedCreation   uint64 // baseInterval * BPS / creationBps
		expectedAnalysis   uint64 // baseInterval * BPS / analysisBps
	}{
		{
			name:             "healthy base=100 → creation=100, analysis=100",
			category:         types.CategoryHealthy,
			baseInterval:     100,
			expectedCreation: 100, // 100 * 1_000_000 / 1_000_000
			expectedAnalysis: 100, // 100 * 1_000_000 / 1_000_000
		},
		{
			name:             "degraded base=100 → creation=133, analysis=66",
			category:         types.CategoryDegraded,
			baseInterval:     100,
			expectedCreation: 133, // 100 * 1_000_000 / 750_000 = 133.33 → 133
			expectedAnalysis: 66,  // 100 * 1_000_000 / 1_500_000 = 66.66 → 66
		},
		{
			name:             "critical base=100 → creation=200, analysis=50",
			category:         types.CategoryCritical,
			baseInterval:     100,
			expectedCreation: 200, // 100 * 1_000_000 / 500_000
			expectedAnalysis: 50,  // 100 * 1_000_000 / 2_000_000
		},
		{
			name:             "degraded base=1000 → creation=1333, analysis=666",
			category:         types.CategoryDegraded,
			baseInterval:     1000,
			expectedCreation: 1333, // 1000 * 1_000_000 / 750_000 = 1333.33 → 1333
			expectedAnalysis: 666,  // 1000 * 1_000_000 / 1_500_000 = 666.66 → 666
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			k, _, ctx := setupKeeper(t)

			k.SetState(ctx, &types.AlignmentState{
				Enabled:          true,
				PreviousCategory: tt.category,
			})

			creationBps, analysisBps := k.GetGlobalPacingMultiplier(ctx)

			// Apply the interval formula: effectiveInterval = baseInterval * BPS / pacingBps
			creationInterval := tt.baseInterval * types.BPS / creationBps
			analysisInterval := tt.baseInterval * types.BPS / analysisBps

			if creationInterval != tt.expectedCreation {
				t.Errorf("creation interval: expected %d, got %d (base=%d, bps=%d)",
					tt.expectedCreation, creationInterval, tt.baseInterval, creationBps)
			}
			if analysisInterval != tt.expectedAnalysis {
				t.Errorf("analysis interval: expected %d, got %d (base=%d, bps=%d)",
					tt.expectedAnalysis, analysisInterval, tt.baseInterval, analysisBps)
			}
		})
	}
}
