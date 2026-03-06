package main

import (
	"math"
	"time"
)

// FitnessSignalEntry represents a fitness signal ready for on-chain submission.
// Matches the format from the influence service (R42-2) and usage-feedback (R42-3).
type FitnessSignalEntry struct {
	SampleID          string  `json:"sample_id"`
	TrainingInfluence float64 `json:"training_influence"` // [0.0, 1.0]
	UsageCorrelation  float64 `json:"usage_correlation"`  // [0.0, 1.0]
	Redundancy        float64 `json:"redundancy"`         // [0.0, 1.0] — 1.0=unique, 0.0=redundant
}

// FitnessSignalBatch is the batch format for on-chain submission.
type FitnessSignalBatch struct {
	Timestamp string               `json:"timestamp"`
	Method    string               `json:"method"`
	Signals   []FitnessSignalEntry `json:"signals"`
	Count     int                  `json:"count"`
}

const (
	canonicalBoost = 0.05 // positive signal for canonical TDUs
	neutralSignal  = 0.5  // default for non-redundancy fields
)

// SignalGenerator produces redundancy fitness signals from ranked clusters.
type SignalGenerator struct {
	threshold float64 // similarity threshold used for clustering
}

// NewSignalGenerator creates a new SignalGenerator.
func NewSignalGenerator(threshold float64) *SignalGenerator {
	return &SignalGenerator{threshold: threshold}
}

// GenerateSignals produces FitnessSignalEntries from ranked clusters.
//
// For each cluster:
//   - Single-member clusters: no signal (TDU is unique, stays at neutral)
//   - Canonical TDU: Redundancy = 0.5 + canonicalBoost (rewarded for being best)
//   - Redundant TDUs: Redundancy = 0.5 - (similarity_to_canonical - threshold)
//     Higher similarity to canonical = stronger decay = lower Redundancy value
func (g *SignalGenerator) GenerateSignals(ranked []RankedCluster) []FitnessSignalEntry {
	var signals []FitnessSignalEntry

	for _, rc := range ranked {
		// Skip single-member clusters — no redundancy
		if len(rc.Redundant) == 0 {
			continue
		}

		// Canonical gets a positive boost
		signals = append(signals, FitnessSignalEntry{
			SampleID:          rc.Canonical.ID,
			TrainingInfluence: neutralSignal,
			UsageCorrelation:  neutralSignal,
			Redundancy:        clampSignal(neutralSignal + canonicalBoost),
		})

		// Each redundant TDU gets a decay signal proportional to similarity
		for _, red := range rc.Redundant {
			sim := Similarity(red, rc.Canonical)
			decay := sim - g.threshold
			if decay < 0 {
				decay = 0
			}
			redundancy := neutralSignal - decay
			redundancy = math.Round(redundancy*10000) / 10000

			signals = append(signals, FitnessSignalEntry{
				SampleID:          red.ID,
				TrainingInfluence: neutralSignal,
				UsageCorrelation:  neutralSignal,
				Redundancy:        clampSignal(redundancy),
			})
		}
	}

	return signals
}

// BuildBatch creates a FitnessSignalBatch ready for on-chain submission.
func (g *SignalGenerator) BuildBatch(signals []FitnessSignalEntry) *FitnessSignalBatch {
	return &FitnessSignalBatch{
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		Method:    "redundancy",
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
