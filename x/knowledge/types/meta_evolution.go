package types

import (
	"encoding/json"
	"fmt"

	sdkmath "cosmossdk.io/math"
)

// ─── R57: Meta-Evolution — The System Improves How It Improves ──────────────
//
// The previous modules improve models. This module improves HOW the system
// improves models. Curation strategies, curriculum designs, review criteria,
// bounty sizing — all compete over epochs. Winners propagate. The system
// discovers better ways of self-improving.
//
// The cycle:
//   Multiple strategies compete → track outcomes over epochs
//   → winning strategies amplified, losers adapted
//   → system discovers better curation methods
//   → better methods → better data → better models
//   → better models develop even better strategies → GOTO 1
//
// This is the highest-leverage loop: a 1% improvement in HOW you improve
// compounds faster than a 1% improvement in any single model.

// ─── Evolution Epoch ────────────────────────────────────────────────────────

// EvolutionEpoch tracks a period of strategy competition.
type EvolutionEpoch struct {
	EpochID    string `json:"epoch_id"`
	Domain     string `json:"domain"`
	StartBlock uint64 `json:"start_block"`
	EndBlock   uint64 `json:"end_block"`

	// Competing strategies and their outcomes.
	Strategies []StrategyOutcome `json:"strategies"`

	// Winner.
	WinnerStrategyID string            `json:"winner_strategy_id"`
	WinningTraits    map[string]string `json:"winning_traits"` // what worked

	// Generated insights for next epoch.
	Insights []string `json:"insights"`

	Status string `json:"status"` // active | completed | cancelled
}

func (e EvolutionEpoch) Marshal() ([]byte, error)  { return json.Marshal(e) }
func (e *EvolutionEpoch) Unmarshal(bz []byte) error { return json.Unmarshal(bz, e) }

// ─── Strategy Outcome ───────────────────────────────────────────────────────

// StrategyOutcome records how a strategy performed during an epoch.
type StrategyOutcome struct {
	StrategyID string `json:"strategy_id"`
	AgentID    string `json:"agent_id"`

	// Production metrics.
	TDUsProduced  uint64 `json:"tdus_produced"`
	AvgFitness    string `json:"avg_fitness"`
	GapsFilled    uint64 `json:"gaps_filled"`
	ModelsTrained uint64 `json:"models_trained"`

	// The bottom line: did this strategy's data produce good models?
	ModelPerformance string `json:"model_performance"` // avg benchmark of models trained on this strategy's data

	// Composite score.
	Score string `json:"score"`
}

// GetScore parses the composite score.
func (s *StrategyOutcome) GetScore() sdkmath.LegacyDec {
	if s.Score == "" {
		return sdkmath.LegacyZeroDec()
	}
	d, err := sdkmath.LegacyNewDecFromStr(s.Score)
	if err != nil {
		return sdkmath.LegacyZeroDec()
	}
	return d
}

// ─── Meta-Parameter ─────────────────────────────────────────────────────────

// MetaParameter is a system-level parameter that evolves based on outcomes.
// Examples: fitness scoring weights, review thresholds, bounty sizing.
type MetaParameter struct {
	ParamID      string `json:"param_id"`
	Name         string `json:"name"`
	Domain       string `json:"domain"`         // empty = global
	CurrentValue string `json:"current_value"`

	// History of values and their outcomes.
	History []MetaParamTrial `json:"history"`

	// Bounds: parameter can't drift outside these.
	MinValue string `json:"min_value"`
	MaxValue string `json:"max_value"`

	// Adjustment rate: how much the parameter changes per epoch.
	StepSize string `json:"step_size"` // default: 5% of range

	UpdatedAt uint64 `json:"updated_at"`
}

func (p MetaParameter) Marshal() ([]byte, error)  { return json.Marshal(p) }
func (p *MetaParameter) Unmarshal(bz []byte) error { return json.Unmarshal(bz, p) }

// GetCurrentValue parses the current value.
func (p *MetaParameter) GetCurrentValue() sdkmath.LegacyDec {
	if p.CurrentValue == "" {
		return sdkmath.LegacyZeroDec()
	}
	d, err := sdkmath.LegacyNewDecFromStr(p.CurrentValue)
	if err != nil {
		return sdkmath.LegacyZeroDec()
	}
	return d
}

// MetaParamTrial records one epoch's value and outcome.
type MetaParamTrial struct {
	Value   string `json:"value"`
	EpochID string `json:"epoch_id"`
	Outcome string `json:"outcome"` // measured result
	Better  bool   `json:"better"`  // improved over previous?
}

// ─── Evolution Parameters ───────────────────────────────────────────────────

// MetaEvolutionParams governs the meta-evolution system.
type MetaEvolutionParams struct {
	EpochDurationBlocks  uint64 `json:"epoch_duration_blocks"`   // how long each competition period lasts
	MinStrategiesPerEpoch uint64 `json:"min_strategies_per_epoch"` // minimum competing strategies
	MaxParamHistory      uint64 `json:"max_param_history"`        // trials to keep per parameter
	DefaultStepSize      string `json:"default_step_size"`        // parameter adjustment rate
	MinEpochDataPoints   uint64 `json:"min_epoch_data_points"`    // min TDUs produced to count
}

func DefaultMetaEvolutionParams() MetaEvolutionParams {
	return MetaEvolutionParams{
		EpochDurationBlocks:   10_000, // ~14 hours at 5s blocks
		MinStrategiesPerEpoch: 2,
		MaxParamHistory:       20,
		DefaultStepSize:       "0.050000000000000000", // 5% adjustment
		MinEpochDataPoints:    5,
	}
}

func (p MetaEvolutionParams) Validate() error {
	if p.EpochDurationBlocks == 0 {
		return fmt.Errorf("epoch_duration_blocks must be > 0")
	}
	if p.MinStrategiesPerEpoch == 0 {
		return fmt.Errorf("min_strategies must be > 0")
	}
	step, err := sdkmath.LegacyNewDecFromStr(p.DefaultStepSize)
	if err != nil || step.IsNegative() || step.GT(sdkmath.LegacyOneDec()) {
		return fmt.Errorf("step_size must be [0, 1], got %s", p.DefaultStepSize)
	}
	return nil
}

func (p MetaEvolutionParams) Marshal() ([]byte, error)  { return json.Marshal(p) }
func (p *MetaEvolutionParams) Unmarshal(bz []byte) error { return json.Unmarshal(bz, p) }

// ─── Store Keys ─────────────────────────────────────────────────────────────

var (
	EvolutionEpochPrefix     = []byte("metaevo/epoch/")
	EpochByDomainPrefix      = []byte("metaevo/epoch-domain/")
	MetaParameterPrefix      = []byte("metaevo/param/")
	MetaParamByDomainPrefix  = []byte("metaevo/param-domain/")
	MetaEvolutionParamsKey   = []byte("metaevo/params")
	EvolutionEpochSeqKey     = []byte("metaevo/epoch-seq")
	CurrentEpochByDomainKey  = []byte("metaevo/current-epoch/")
)

func EvolutionEpochKey(epochID string) []byte {
	return append(EvolutionEpochPrefix, []byte(epochID)...)
}

func EpochByDomainKey(domain, epochID string) []byte {
	return append(EpochByDomainPrefix, []byte(domain+"/"+epochID)...)
}

func EpochByDomainPfx(domain string) []byte {
	return append(EpochByDomainPrefix, []byte(domain+"/")...)
}

func MetaParameterKey(paramID string) []byte {
	return append(MetaParameterPrefix, []byte(paramID)...)
}

func MetaParamByDomainKey(domain, paramID string) []byte {
	return append(MetaParamByDomainPrefix, []byte(domain+"/"+paramID)...)
}

func MetaParamByDomainPfx(domain string) []byte {
	return append(MetaParamByDomainPrefix, []byte(domain+"/")...)
}

func CurrentEpochKey(domain string) []byte {
	return append(CurrentEpochByDomainKey, []byte(domain)...)
}

// ─── Events ─────────────────────────────────────────────────────────────────

const (
	EventEpochStarted      = "evolution_epoch_started"
	EventEpochCompleted    = "evolution_epoch_completed"
	EventParamAdjusted     = "meta_param_adjusted"
	EventStrategyWon       = "strategy_won_epoch"

	AttributeEpochID       = "epoch_id"
	AttributeWinnerID      = "winner_id"
	AttributeParamID       = "param_id"
	AttributeOldValue      = "old_value"
	AttributeNewValue      = "new_value"
	AttributeEpochScore    = "epoch_score"
)
