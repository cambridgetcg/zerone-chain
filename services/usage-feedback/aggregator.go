package main

import (
	"math"
	"time"
)

// AggregatedSignal holds the accumulated usage correlation signal for a single TDU.
type AggregatedSignal struct {
	TDUID       string  `json:"tdu_id"`
	MeanSignal  float64 `json:"mean_signal"`  // [-1.0, 1.0]
	SignalCount int     `json:"signal_count"`
	Domain      string  `json:"domain,omitempty"`
}

// AggregationConfig controls time window and thresholds.
type AggregationConfig struct {
	WindowDuration    time.Duration // aggregation window (default 1h)
	MinSignalCount    int           // minimum signals per TDU to emit (default 3)
}

// DefaultAggregationConfig returns the default configuration.
func DefaultAggregationConfig() AggregationConfig {
	return AggregationConfig{
		WindowDuration: 1 * time.Hour,
		MinSignalCount: 3,
	}
}

// Aggregator batches and averages attributed signals per TDU over a time window.
type Aggregator struct {
	config AggregationConfig
}

// NewAggregator creates an Aggregator with the given config.
func NewAggregator(config AggregationConfig) *Aggregator {
	return &Aggregator{config: config}
}

// tduAccumulator holds running state for a single TDU.
type tduAccumulator struct {
	sum    float64
	count  int
	domain string
}

// Aggregate takes a batch of attributed signals and produces aggregated per-TDU signals.
// Only TDUs with at least MinSignalCount signals are included in the output.
func (ag *Aggregator) Aggregate(signals []AttributedSignal) []AggregatedSignal {
	accum := make(map[string]*tduAccumulator)

	for _, s := range signals {
		acc, ok := accum[s.TDUID]
		if !ok {
			acc = &tduAccumulator{domain: s.Domain}
			accum[s.TDUID] = acc
		}
		acc.sum += s.Signal
		acc.count++
	}

	var results []AggregatedSignal
	for tduid, acc := range accum {
		if acc.count < ag.config.MinSignalCount {
			continue
		}

		mean := acc.sum / float64(acc.count)
		// Clamp to [-1.0, 1.0]
		mean = math.Max(-1.0, math.Min(1.0, mean))
		mean = math.Round(mean*10000) / 10000

		results = append(results, AggregatedSignal{
			TDUID:       tduid,
			MeanSignal:  mean,
			SignalCount: acc.count,
			Domain:      acc.domain,
		})
	}

	return results
}
