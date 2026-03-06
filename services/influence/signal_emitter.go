package main

import (
	"math"
	"time"
)

// FitnessSignalEntry represents a fitness signal ready for on-chain submission.
// Maps to the knowledge module's FitnessSignal type.
type FitnessSignalEntry struct {
	SampleID          string  `json:"sample_id"`
	TrainingInfluence float64 `json:"training_influence"` // [0.0, 1.0]
	UsageCorrelation  float64 `json:"usage_correlation"`  // [0.0, 1.0] — neutral default
	Redundancy        float64 `json:"redundancy"`         // [0.0, 1.0] — neutral default
}

// FitnessSignalBatch is the batch format for on-chain submission.
type FitnessSignalBatch struct {
	Timestamp string               `json:"timestamp"`
	Method    string               `json:"method"`
	Signals   []FitnessSignalEntry `json:"signals"`
	Count     int                  `json:"count"`
}

// SignalEmitter converts influence analysis results to on-chain fitness signals.
type SignalEmitter struct{}

// NewSignalEmitter creates a new signal emitter.
func NewSignalEmitter() *SignalEmitter {
	return &SignalEmitter{}
}

// ConvertToFitnessSignals transforms InfluenceResults into FitnessSignalEntries.
//
// The influence score is in [-1.0, 1.0] but the on-chain TrainingInfluence field
// is in [0.0, 1.0]. Mapping:
//
//	influence -1.0 -> TrainingInfluence 0.0 (maximally harmful)
//	influence  0.0 -> TrainingInfluence 0.5 (neutral)
//	influence +1.0 -> TrainingInfluence 1.0 (maximally helpful)
//
// UsageCorrelation and Redundancy are set to 0.5 (neutral) since this pipeline
// only produces the training influence signal. Other signals come from the
// usage tracking pipeline (R42-3) and redundancy analysis respectively.
func (e *SignalEmitter) ConvertToFitnessSignals(results []InfluenceResult) []FitnessSignalEntry {
	// Deduplicate: for leave-one-out with per-domain results,
	// aggregate to a single overall score per TDU
	aggregated := e.aggregateByTDU(results)

	signals := make([]FitnessSignalEntry, 0, len(aggregated))
	for tduid, score := range aggregated {
		// Map [-1, 1] -> [0, 1]
		trainingInfluence := (score + 1.0) / 2.0
		trainingInfluence = clampSignal(trainingInfluence)

		signals = append(signals, FitnessSignalEntry{
			SampleID:          tduid,
			TrainingInfluence: math.Round(trainingInfluence*10000) / 10000,
			UsageCorrelation:  0.5, // neutral — filled by usage signal pipeline
			Redundancy:        0.5, // neutral — filled by redundancy analysis
		})
	}

	return signals
}

// aggregateByTDU computes a single score per TDU from potentially multiple
// domain-specific results (e.g., leave-one-out produces per-domain scores).
func (e *SignalEmitter) aggregateByTDU(results []InfluenceResult) map[string]float64 {
	sums := make(map[string]float64)
	counts := make(map[string]int)

	for _, r := range results {
		sums[r.TDUID] += r.Score
		counts[r.TDUID]++
	}

	aggregated := make(map[string]float64, len(sums))
	for id, sum := range sums {
		aggregated[id] = sum / float64(counts[id])
	}

	return aggregated
}

// BuildBatch creates a FitnessSignalBatch ready for on-chain submission.
func (e *SignalEmitter) BuildBatch(signals []FitnessSignalEntry) *FitnessSignalBatch {
	return &FitnessSignalBatch{
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		Method:    "training_influence",
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
