package keeper_test

import (
	"testing"

	"github.com/zerone-chain/zerone/x/alignment/keeper"
	"github.com/zerone-chain/zerone/x/alignment/types"
)

func TestQueryGlobalPacing(t *testing.T) {
	tests := []struct {
		name               string
		state              *types.AlignmentState
		expectedCategory   string
		expectedCreation   uint64
		expectedAnalysis   uint64
	}{
		{
			name: "healthy state returns neutral multipliers",
			state: &types.AlignmentState{
				Enabled:          true,
				PreviousCategory: types.CategoryHealthy,
			},
			expectedCategory: types.CategoryHealthy,
			expectedCreation: types.BPS,
			expectedAnalysis: types.BPS,
		},
		{
			name: "degraded state slows creation and speeds analysis",
			state: &types.AlignmentState{
				Enabled:          true,
				PreviousCategory: types.CategoryDegraded,
			},
			expectedCategory: types.CategoryDegraded,
			expectedCreation: 750_000,
			expectedAnalysis: 1_500_000,
		},
		{
			name: "critical state doubles pacing effects",
			state: &types.AlignmentState{
				Enabled:          true,
				PreviousCategory: types.CategoryCritical,
			},
			expectedCategory: types.CategoryCritical,
			expectedCreation: 500_000,
			expectedAnalysis: 2_000_000,
		},
		{
			name: "empty category defaults to healthy",
			state: &types.AlignmentState{
				Enabled:          true,
				PreviousCategory: "",
			},
			expectedCategory: types.CategoryHealthy,
			expectedCreation: types.BPS,
			expectedAnalysis: types.BPS,
		},
		{
			name: "disabled state returns neutral multipliers with healthy fallback",
			state: &types.AlignmentState{
				Enabled:          false,
				PreviousCategory: types.CategoryCritical,
			},
			expectedCategory: types.CategoryCritical,
			expectedCreation: types.BPS,
			expectedAnalysis: types.BPS,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			k, _, ctx := setupKeeper(t)
			k.SetState(ctx, tt.state)
			qs := keeper.NewQueryServerImpl(k)

			resp, err := qs.GlobalPacing(ctx, &types.QueryGlobalPacingRequest{})
			if err != nil {
				t.Fatalf("GlobalPacing query failed: %v", err)
			}

			if resp.HealthCategory != tt.expectedCategory {
				t.Errorf("HealthCategory: expected %s, got %s", tt.expectedCategory, resp.HealthCategory)
			}
			if resp.CreationMultiplierBps != tt.expectedCreation {
				t.Errorf("CreationMultiplierBps: expected %d, got %d", tt.expectedCreation, resp.CreationMultiplierBps)
			}
			if resp.AnalysisMultiplierBps != tt.expectedAnalysis {
				t.Errorf("AnalysisMultiplierBps: expected %d, got %d", tt.expectedAnalysis, resp.AnalysisMultiplierBps)
			}
		})
	}
}
