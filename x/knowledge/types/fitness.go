package types

import (
	"encoding/json"
	"fmt"

	sdkmath "cosmossdk.io/math"
)

// TDULifecycleStatus represents the lifecycle phase of a TDU derived from its fitness score.
type TDULifecycleStatus int

const (
	TDULifecycleCore    TDULifecycleStatus = iota // 0.7–1.0: high-value, earns longevity rewards
	TDULifecycleActive                             // 0.3–0.7: healthy, earns smaller rewards
	TDULifecycleDormant                            // 0.1–0.3: declining, no rewards
	TDULifecyclePruned                             // <0.1: eligible for pruning
)

// Lifecycle thresholds as LegacyDec.
var (
	FitnessThresholdCore    = sdkmath.LegacyNewDecWithPrec(7, 1) // 0.7
	FitnessThresholdActive  = sdkmath.LegacyNewDecWithPrec(3, 1) // 0.3
	FitnessThresholdDormant = sdkmath.LegacyNewDecWithPrec(1, 1) // 0.1
	FitnessInitialScore     = sdkmath.LegacyNewDecWithPrec(5, 1) // 0.5
	FitnessMinScore         = sdkmath.LegacyZeroDec()            // 0.0
	FitnessMaxScore         = sdkmath.LegacyOneDec()             // 1.0
)

// LifecycleStatusFromScore derives the lifecycle status from a fitness score.
func LifecycleStatusFromScore(score sdkmath.LegacyDec) TDULifecycleStatus {
	switch {
	case score.GTE(FitnessThresholdCore):
		return TDULifecycleCore
	case score.GTE(FitnessThresholdActive):
		return TDULifecycleActive
	case score.GTE(FitnessThresholdDormant):
		return TDULifecycleDormant
	default:
		return TDULifecyclePruned
	}
}

// String returns a human-readable name for the lifecycle status.
func (s TDULifecycleStatus) String() string {
	switch s {
	case TDULifecycleCore:
		return "core"
	case TDULifecycleActive:
		return "active"
	case TDULifecycleDormant:
		return "dormant"
	case TDULifecyclePruned:
		return "pruned"
	default:
		return "unknown"
	}
}

// TDUFitnessRecord stores the fitness decay state for a sample (TDU).
// Stored as JSON in the KVStore to avoid proto file changes.
type TDUFitnessRecord struct {
	SampleID        string `json:"sample_id"`
	FitnessScore    string `json:"fitness_score"`     // sdkmath.LegacyDec serialized
	OriginalStake   string `json:"original_stake"`    // sdkmath.Int serialized (uzrn)
	LastSignalCycle uint64 `json:"last_signal_cycle"` // last cycle that received a signal
	CycleCount      uint64 `json:"cycle_count"`       // total cycles since creation
}

// GetFitnessScore parses the stored fitness score.
func (r *TDUFitnessRecord) GetFitnessScore() sdkmath.LegacyDec {
	if r.FitnessScore == "" {
		return FitnessInitialScore
	}
	d, err := sdkmath.LegacyNewDecFromStr(r.FitnessScore)
	if err != nil {
		return FitnessInitialScore
	}
	return d
}

// SetFitnessScore stores the fitness score, clamped to [0.0, 1.0].
func (r *TDUFitnessRecord) SetFitnessScore(score sdkmath.LegacyDec) {
	if score.GT(FitnessMaxScore) {
		score = FitnessMaxScore
	}
	if score.LT(FitnessMinScore) {
		score = FitnessMinScore
	}
	r.FitnessScore = score.String()
}

// GetOriginalStake parses the stored original stake amount.
func (r *TDUFitnessRecord) GetOriginalStake() sdkmath.Int {
	if r.OriginalStake == "" {
		return sdkmath.ZeroInt()
	}
	i, ok := sdkmath.NewIntFromString(r.OriginalStake)
	if !ok {
		return sdkmath.ZeroInt()
	}
	return i
}

// GetLifecycleStatus returns the derived lifecycle status.
func (r *TDUFitnessRecord) GetLifecycleStatus() TDULifecycleStatus {
	return LifecycleStatusFromScore(r.GetFitnessScore())
}

// NewTDUFitnessRecord creates a new fitness record for a freshly accepted sample.
func NewTDUFitnessRecord(sampleID string, originalStake sdkmath.Int, currentCycle uint64) TDUFitnessRecord {
	return TDUFitnessRecord{
		SampleID:        sampleID,
		FitnessScore:    FitnessInitialScore.String(),
		OriginalStake:   originalStake.String(),
		LastSignalCycle: currentCycle,
		CycleCount:      0,
	}
}

// FitnessSignal carries the three input signals for fitness score updates.
// Each signal is in [0.0, 1.0].
type FitnessSignal struct {
	TrainingInfluence sdkmath.LegacyDec // weight: 50% — how much this TDU influenced training
	UsageCorrelation  sdkmath.LegacyDec // weight: 30% — correlation with downstream usage
	Redundancy        sdkmath.LegacyDec // weight: 20% — inverted: 1.0 = unique, 0.0 = fully redundant
}

// Marshal serializes FitnessSignal to a proto-compatible binary format.
func (s *FitnessSignal) Marshal() ([]byte, error) {
	var buf []byte
	buf = protoAppendStringField(buf, 1, s.TrainingInfluence.String())
	buf = protoAppendStringField(buf, 2, s.UsageCorrelation.String())
	buf = protoAppendStringField(buf, 3, s.Redundancy.String())
	return buf, nil
}

// Unmarshal deserializes FitnessSignal from proto-compatible binary format.
func (s *FitnessSignal) Unmarshal(dAtA []byte) error {
	offset := 0
	for offset < len(dAtA) {
		fieldNum, wireType, newOffset, err := protoConsumeTag(dAtA, offset)
		if err != nil {
			return err
		}
		offset = newOffset
		switch fieldNum {
		case 1:
			raw, newOff, err := protoConsumeBytes(dAtA, offset)
			if err != nil {
				return err
			}
			s.TrainingInfluence, err = sdkmath.LegacyNewDecFromStr(string(raw))
			if err != nil {
				return err
			}
			offset = newOff
		case 2:
			raw, newOff, err := protoConsumeBytes(dAtA, offset)
			if err != nil {
				return err
			}
			s.UsageCorrelation, err = sdkmath.LegacyNewDecFromStr(string(raw))
			if err != nil {
				return err
			}
			offset = newOff
		case 3:
			raw, newOff, err := protoConsumeBytes(dAtA, offset)
			if err != nil {
				return err
			}
			s.Redundancy, err = sdkmath.LegacyNewDecFromStr(string(raw))
			if err != nil {
				return err
			}
			offset = newOff
		default:
			newOff, err := protoSkipField(dAtA, offset, wireType)
			if err != nil {
				return err
			}
			offset = newOff
		}
	}
	return nil
}

// Signal weights as LegacyDec.
var (
	SignalWeightTraining   = sdkmath.LegacyNewDecWithPrec(5, 1) // 0.5
	SignalWeightUsage      = sdkmath.LegacyNewDecWithPrec(3, 1) // 0.3
	SignalWeightRedundancy = sdkmath.LegacyNewDecWithPrec(2, 1) // 0.2
)

// FitnessDecayParams holds governance-tunable parameters for fitness decay.
type FitnessDecayParams struct {
	// DecayPerCycle: score reduction per cycle when no signal received. Default 0.02.
	DecayPerCycle string `json:"decay_per_cycle"`
	// UnscoredCycleThreshold: cycles without signal before decay starts. Default 5.
	UnscoredCycleThreshold uint64 `json:"unscored_cycle_threshold"`
	// CoreRewardRate: longevity reward multiplier for Core TDUs per cycle. Default 0.01.
	CoreRewardRate string `json:"core_reward_rate"`
	// ActiveRewardRate: longevity reward multiplier for Active TDUs per cycle. Default 0.005.
	ActiveRewardRate string `json:"active_reward_rate"`
	// FitnessEpochBlocks: block interval for fitness epoch processing. Default 100.
	FitnessEpochBlocks uint64 `json:"fitness_epoch_blocks"`
}

// DefaultFitnessDecayParams returns sensible defaults.
func DefaultFitnessDecayParams() FitnessDecayParams {
	return FitnessDecayParams{
		DecayPerCycle:          "0.020000000000000000",
		UnscoredCycleThreshold: 5,
		CoreRewardRate:         "0.010000000000000000",
		ActiveRewardRate:       "0.005000000000000000",
		FitnessEpochBlocks:    100,
	}
}

// GetFitnessEpochBlocks returns the fitness epoch interval, defaulting to 100 if unset.
func (p FitnessDecayParams) GetFitnessEpochBlocks() uint64 {
	if p.FitnessEpochBlocks == 0 {
		return 100
	}
	return p.FitnessEpochBlocks
}

// GetDecayPerCycle parses the decay rate.
func (p FitnessDecayParams) GetDecayPerCycle() sdkmath.LegacyDec {
	d, err := sdkmath.LegacyNewDecFromStr(p.DecayPerCycle)
	if err != nil {
		return sdkmath.LegacyNewDecWithPrec(2, 2) // 0.02
	}
	return d
}

// GetCoreRewardRate parses the core reward rate.
func (p FitnessDecayParams) GetCoreRewardRate() sdkmath.LegacyDec {
	d, err := sdkmath.LegacyNewDecFromStr(p.CoreRewardRate)
	if err != nil {
		return sdkmath.LegacyNewDecWithPrec(1, 2) // 0.01
	}
	return d
}

// GetActiveRewardRate parses the active reward rate.
func (p FitnessDecayParams) GetActiveRewardRate() sdkmath.LegacyDec {
	d, err := sdkmath.LegacyNewDecFromStr(p.ActiveRewardRate)
	if err != nil {
		return sdkmath.LegacyNewDecWithPrec(5, 3) // 0.005
	}
	return d
}

// Validate checks all fields are within valid ranges.
func (p FitnessDecayParams) Validate() error {
	decay := p.GetDecayPerCycle()
	if decay.IsNegative() || decay.GT(sdkmath.LegacyOneDec()) {
		return fmt.Errorf("decay_per_cycle must be in [0, 1], got %s", p.DecayPerCycle)
	}
	coreRate := p.GetCoreRewardRate()
	if coreRate.IsNegative() || coreRate.GT(sdkmath.LegacyOneDec()) {
		return fmt.Errorf("core_reward_rate must be in [0, 1], got %s", p.CoreRewardRate)
	}
	activeRate := p.GetActiveRewardRate()
	if activeRate.IsNegative() || activeRate.GT(sdkmath.LegacyOneDec()) {
		return fmt.Errorf("active_reward_rate must be in [0, 1], got %s", p.ActiveRewardRate)
	}
	return nil
}

// MarshalJSON marshals to JSON.
func (p FitnessDecayParams) MarshalJSON() ([]byte, error) {
	type alias FitnessDecayParams
	return json.Marshal(alias(p))
}

// UnmarshalJSON unmarshals from JSON.
func (p *FitnessDecayParams) UnmarshalJSON(bz []byte) error {
	type alias FitnessDecayParams
	var a alias
	if err := json.Unmarshal(bz, &a); err != nil {
		return err
	}
	*p = FitnessDecayParams(a)
	return nil
}
