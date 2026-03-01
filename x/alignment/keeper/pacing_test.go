package keeper_test

import (
	"testing"

	"github.com/zerone-chain/zerone/x/alignment/types"
)

func TestGetGlobalPacingMultiplier(t *testing.T) {
	tests := []struct {
		name               string
		state              *types.AlignmentState
		expectedCreation   uint64
		expectedAnalysis   uint64
	}{
		{
			name: "healthy returns neutral multipliers",
			state: &types.AlignmentState{
				Enabled:          true,
				PreviousCategory: types.CategoryHealthy,
			},
			expectedCreation: types.BPS,
			expectedAnalysis: types.BPS,
		},
		{
			name: "degraded slows creation and speeds analysis",
			state: &types.AlignmentState{
				Enabled:          true,
				PreviousCategory: types.CategoryDegraded,
			},
			expectedCreation: 750_000,
			expectedAnalysis: 1_500_000,
		},
		{
			name: "critical doubles pacing effects",
			state: &types.AlignmentState{
				Enabled:          true,
				PreviousCategory: types.CategoryCritical,
			},
			expectedCreation: 500_000,
			expectedAnalysis: 2_000_000,
		},
		{
			name: "disabled returns neutral regardless of category",
			state: &types.AlignmentState{
				Enabled:          false,
				PreviousCategory: types.CategoryCritical,
			},
			expectedCreation: types.BPS,
			expectedAnalysis: types.BPS,
		},
		{
			name: "empty PreviousCategory returns neutral",
			state: &types.AlignmentState{
				Enabled:          true,
				PreviousCategory: "",
			},
			expectedCreation: types.BPS,
			expectedAnalysis: types.BPS,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			k, _, ctx := setupKeeper(t)
			k.SetState(ctx, tt.state)

			creation, analysis := k.GetGlobalPacingMultiplier(ctx)

			if creation != tt.expectedCreation {
				t.Errorf("creationBps: expected %d, got %d", tt.expectedCreation, creation)
			}
			if analysis != tt.expectedAnalysis {
				t.Errorf("analysisBps: expected %d, got %d", tt.expectedAnalysis, analysis)
			}
		})
	}
}
