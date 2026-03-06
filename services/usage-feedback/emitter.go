package main

import (
	"math"
	"time"
)

// FitnessSignalEntry represents a fitness signal ready for on-chain submission.
// Matches the format from the influence service (R42-2).
type FitnessSignalEntry struct {
	SampleID          string  `json:"sample_id"`
	TrainingInfluence float64 `json:"training_influence"` // [0.0, 1.0]
	UsageCorrelation  float64 `json:"usage_correlation"`  // [0.0, 1.0]
	Redundancy        float64 `json:"redundancy"`         // [0.0, 1.0]
}

// FitnessSignalBatch is the batch format for on-chain submission.
type FitnessSignalBatch struct {
	Timestamp string               `json:"timestamp"`
	Method    string               `json:"method"`
	Signals   []FitnessSignalEntry `json:"signals"`
	Count     int                  `json:"count"`
}

// Emitter converts aggregated usage signals to on-chain fitness signal batches.
type Emitter struct{}

// NewEmitter creates a new Emitter.
func NewEmitter() *Emitter {
	return &Emitter{}
}

// ConvertToFitnessSignals transforms aggregated usage signals into FitnessSignalEntries.
//
// The aggregated mean signal is in [-1.0, 1.0] but the on-chain UsageCorrelation
// field is in [0.0, 1.0]. Mapping:
//
//	mean -1.0 -> UsageCorrelation 0.0 (strong negative usage feedback)
//	mean  0.0 -> UsageCorrelation 0.5 (neutral)
//	mean +1.0 -> UsageCorrelation 1.0 (strong positive usage feedback)
//
// TrainingInfluence and Redundancy are set to 0.5 (neutral) since this pipeline
// only produces the usage correlation signal.
func (e *Emitter) ConvertToFitnessSignals(aggregated []AggregatedSignal) []FitnessSignalEntry {
	signals := make([]FitnessSignalEntry, 0, len(aggregated))

	for _, agg := range aggregated {
		// Map [-1, 1] -> [0, 1]
		usageCorrelation := (agg.MeanSignal + 1.0) / 2.0
		usageCorrelation = clampSignal(usageCorrelation)
		usageCorrelation = math.Round(usageCorrelation*10000) / 10000

		signals = append(signals, FitnessSignalEntry{
			SampleID:          agg.TDUID,
			TrainingInfluence: 0.5, // neutral — filled by training influence pipeline
			UsageCorrelation:  usageCorrelation,
			Redundancy:        0.5, // neutral — filled by redundancy analysis
		})
	}

	return signals
}

// BuildBatch creates a FitnessSignalBatch ready for on-chain submission.
func (e *Emitter) BuildBatch(signals []FitnessSignalEntry) *FitnessSignalBatch {
	return &FitnessSignalBatch{
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		Method:    "usage_correlation",
		Signals:   signals,
		Count:     len(signals),
	}
}

// clampSignal ensures a signal value is in [0.0, 1.0].
func clampSignal(v float64) float64 {
	if v < 0.0 {
		return 0.0
	}
	if v > 1.0 {
		return 1.0
	}
	return v
}
