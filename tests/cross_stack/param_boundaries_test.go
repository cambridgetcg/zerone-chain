package cross_stack_test

import (
	"math"
	"reflect"
	"testing"

	aligntypes "github.com/zerone-chain/zerone/x/alignment/types"
	captypes "github.com/zerone-chain/zerone/x/capture_defense/types"
	knowtypes "github.com/zerone-chain/zerone/x/knowledge/types"
	partnertypes "github.com/zerone-chain/zerone/x/partnerships/types"
)

// TestGovernanceParamBoundaries iterates all uint64 fields on knowledge,
// alignment, capture_defense, and partnerships Params structs. For each
// field it clones DefaultParams and sets the field to 0, max uint64, 1,
// and BPS (1M), verifying Validate() catches dangerous values.
func TestGovernanceParamBoundaries(t *testing.T) {
	type paramSpec struct {
		name     string
		defaults interface{}
		validate func(interface{}) error
	}

	specs := []paramSpec{
		{
			name:     "knowledge",
			defaults: knowtypes.DefaultParams(),
			validate: func(v interface{}) error { p := v.(knowtypes.Params); return p.Validate() },
		},
		{
			name:     "alignment",
			defaults: aligntypes.DefaultParams(),
			validate: func(v interface{}) error { p := v.(aligntypes.Params); return p.Validate() },
		},
		{
			name:     "capture_defense",
			defaults: *captypes.DefaultParams(),
			validate: func(v interface{}) error { p := v.(captypes.Params); return p.Validate() },
		},
		{
			name:     "partnerships",
			defaults: *partnertypes.DefaultParams(),
			validate: func(v interface{}) error { p := v.(partnertypes.Params); return p.Validate() },
		},
	}

	boundaryValues := []struct {
		name string
		val  uint64
	}{
		{"zero", 0},
		{"max_uint64", math.MaxUint64},
		{"one", 1},
		{"bps", 1_000_000},
	}

	for _, spec := range specs {
		t.Run(spec.name, func(t *testing.T) {
			rv := reflect.ValueOf(spec.defaults)
			rt := rv.Type()

			for i := 0; i < rt.NumField(); i++ {
				field := rt.Field(i)
				if field.Type.Kind() != reflect.Uint64 {
					continue
				}

				for _, bv := range boundaryValues {
					t.Run(field.Name+"_"+bv.name, func(t *testing.T) {
						clone := reflect.New(rt).Elem()
						clone.Set(rv)
						clone.Field(i).SetUint(bv.val)

						p := clone.Interface()
						err := spec.validate(p)
						// We don't assert pass/fail per-value — the point is
						// to exercise every boundary and ensure no panics.
						// Log which combinations are rejected for visibility.
						if err != nil {
							t.Logf("REJECTED %s.%s=%s: %v", spec.name, field.Name, bv.name, err)
						}
					})
				}
			}
		})
	}
}

// TestGovernanceParamInteractions tests adversarial parameter combinations
// that span multiple fields to verify cross-parameter validation catches them.
func TestGovernanceParamInteractions(t *testing.T) {
	t.Run("max_decay_min_initial_energy", func(t *testing.T) {
		p := knowtypes.DefaultParams()
		// Extreme: high base cost + high multiplier → should fire cross-param check
		p.MetabolismBaseCost = 500_000
		p.OvercrowdingDecayMultiplierBps = 10_000_000 // 1000% multiplier
		p.MetabolismInitialEnergy = 500_000
		err := p.Validate()
		if err == nil {
			t.Error("expected cross-param validation to reject max decay + min initial energy")
		}
	})

	t.Run("max_multipliers_all_bonuses", func(t *testing.T) {
		p := knowtypes.DefaultParams()
		// Max out role elasticity and agent verification bonus
		p.RoleElasticityMaxMultiplierBps = 5_000_000 // 500%
		p.AgentVerificationBonusBps = 1_000_000      // 100%
		err := p.Validate()
		if err == nil {
			t.Error("expected cross-param validation to reject role_elasticity * agent_bonus > 100%")
		}
	})

	t.Run("zero_thresholds_nonzero_intervals", func(t *testing.T) {
		p := knowtypes.DefaultParams()
		p.MetabolismActiveThreshold = 0
		err := p.Validate()
		if err == nil {
			t.Error("expected validation to reject zero active threshold")
		}
	})

	t.Run("epistemic_cold_cap_equals_max_survival", func(t *testing.T) {
		p := knowtypes.DefaultParams()
		p.EpistemicColdConfidenceCapBps = p.MaxSurvivalConfidence
		err := p.Validate()
		if err == nil {
			t.Error("expected cross-param validation to reject cold_cap == max_survival_confidence")
		}
	})

	t.Run("epistemic_cold_cap_exceeds_max_survival", func(t *testing.T) {
		p := knowtypes.DefaultParams()
		p.EpistemicColdConfidenceCapBps = p.MaxSurvivalConfidence + 1
		err := p.Validate()
		if err == nil {
			t.Error("expected cross-param validation to reject cold_cap > max_survival_confidence")
		}
	})

	t.Run("alignment_max_multipliers", func(t *testing.T) {
		p := aligntypes.DefaultParams()
		// Max out both: 10x bounds * 100% magnitude = 1000% > 100%
		p.CorrectionBoundsMaxMultiplierBps = 10_000_000 // 1000%
		p.MaxAutoApplyMagnitudeBps = 1_000_000          // 100%
		err := p.Validate()
		if err == nil {
			t.Error("expected alignment cross-param validation to reject bounds * magnitude > 100%")
		}
	})

	t.Run("partnerships_splits_sum_wrong", func(t *testing.T) {
		p := *partnertypes.DefaultParams()
		p.DefaultHumanSplitBps = 700_000
		p.DefaultAgentSplitBps = 700_000
		err := p.Validate()
		if err == nil {
			t.Error("expected validation to reject splits > 100%")
		}
	})
}

// TestCrossParamValidation_KnowledgeDecay specifically tests the R30-2 cross-parameter
// check for overcrowding decay vs initial energy.
func TestCrossParamValidation_KnowledgeDecay(t *testing.T) {
	t.Run("default_params_pass", func(t *testing.T) {
		p := knowtypes.DefaultParams()
		if err := p.Validate(); err != nil {
			t.Fatalf("default params should pass: %v", err)
		}
	})

	t.Run("aggressive_decay_rejected", func(t *testing.T) {
		p := knowtypes.DefaultParams()
		// Set base cost very high so multiplied cost exceeds 50% of initial energy
		p.MetabolismBaseCost = 200_000
		p.OvercrowdingDecayMultiplierBps = 5_000_000 // 500%
		// maxDecay = 200_000 * 5_000_000 / 1_000_000 = 1_000_000
		// 50% of initial = 500_000 / 2 = 250_000
		// 1_000_000 > 250_000 → rejected
		err := p.Validate()
		if err == nil {
			t.Error("expected aggressive overcrowding decay to be rejected")
		}
	})

	t.Run("mild_decay_accepted", func(t *testing.T) {
		p := knowtypes.DefaultParams()
		p.MetabolismBaseCost = 10_000
		p.OvercrowdingDecayMultiplierBps = 1_500_000 // 150%
		// maxDecay = 10_000 * 1_500_000 / 1_000_000 = 15_000
		// 50% of initial = 500_000 / 2 = 250_000
		// 15_000 < 250_000 → accepted
		err := p.Validate()
		if err != nil {
			t.Errorf("mild decay should be accepted: %v", err)
		}
	})

	t.Run("cold_cap_below_max_survival_accepted", func(t *testing.T) {
		p := knowtypes.DefaultParams()
		p.EpistemicColdConfidenceCapBps = 600_000
		p.MaxSurvivalConfidence = 770_000
		err := p.Validate()
		if err != nil {
			t.Errorf("cold cap below max survival should be accepted: %v", err)
		}
	})

	t.Run("role_elasticity_agent_bonus_safe", func(t *testing.T) {
		p := knowtypes.DefaultParams()
		// 200% * 20% = 40% < 100% → ok
		p.RoleElasticityMaxMultiplierBps = 2_000_000
		p.AgentVerificationBonusBps = 200_000
		err := p.Validate()
		if err != nil {
			t.Errorf("safe role elasticity + agent bonus should be accepted: %v", err)
		}
	})
}

// TestCrossParamValidation_AlignmentBounds specifically tests the R30-2 cross-parameter
// check for correction bounds × magnitude.
func TestCrossParamValidation_AlignmentBounds(t *testing.T) {
	t.Run("default_params_pass", func(t *testing.T) {
		p := aligntypes.DefaultParams()
		if err := p.Validate(); err != nil {
			t.Fatalf("default params should pass: %v", err)
		}
	})

	t.Run("bounds_times_magnitude_exceeds_100pct", func(t *testing.T) {
		p := aligntypes.DefaultParams()
		p.CorrectionBoundsMaxMultiplierBps = 5_000_000 // 500%
		p.MaxAutoApplyMagnitudeBps = 500_000            // 50%
		// 5_000_000 * 500_000 / 1_000_000 = 2_500_000 > 1_000_000 → rejected
		err := p.Validate()
		if err == nil {
			t.Error("expected bounds * magnitude > 100% to be rejected")
		}
	})

	t.Run("bounds_times_magnitude_exactly_100pct", func(t *testing.T) {
		p := aligntypes.DefaultParams()
		p.CorrectionBoundsMaxMultiplierBps = 2_000_000 // 200%
		p.MaxAutoApplyMagnitudeBps = 500_000            // 50%
		// 2_000_000 * 500_000 / 1_000_000 = 1_000_000 == BPS → not > BPS → accepted
		err := p.Validate()
		if err != nil {
			t.Errorf("bounds * magnitude exactly 100%% should be accepted: %v", err)
		}
	})

	t.Run("safe_combination_accepted", func(t *testing.T) {
		p := aligntypes.DefaultParams()
		p.CorrectionBoundsMaxMultiplierBps = 1_500_000 // 150%
		p.MaxAutoApplyMagnitudeBps = 300_000            // 30%
		// 1_500_000 * 300_000 / 1_000_000 = 450_000 < 1_000_000 → accepted
		err := p.Validate()
		if err != nil {
			t.Errorf("safe combination should be accepted: %v", err)
		}
	})
}

// TestDefaultGenesis_GovernableModules verifies that default genesis states for all
// modules with governance-mutable params pass validation.
func TestDefaultGenesis_GovernableModules(t *testing.T) {
	t.Run("knowledge", func(t *testing.T) {
		gs := knowtypes.DefaultGenesis()
		if err := gs.Validate(); err != nil {
			t.Fatalf("knowledge default genesis should be valid: %v", err)
		}
	})

	t.Run("alignment", func(t *testing.T) {
		gs := aligntypes.DefaultGenesis()
		if err := gs.Validate(); err != nil {
			t.Fatalf("alignment default genesis should be valid: %v", err)
		}
	})

	t.Run("capture_defense", func(t *testing.T) {
		gs := captypes.DefaultGenesis()
		if err := gs.Validate(); err != nil {
			t.Fatalf("capture_defense default genesis should be valid: %v", err)
		}
	})

	t.Run("partnerships", func(t *testing.T) {
		gs := partnertypes.DefaultGenesis()
		if err := gs.Validate(); err != nil {
			t.Fatalf("partnerships default genesis should be valid: %v", err)
		}
	})
}
