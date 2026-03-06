package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"sort"
)

// LossEntry represents per-TDU loss from a training run.
type LossEntry struct {
	TDUID      string  `json:"tdu_id"`
	Domain     string  `json:"domain,omitempty"`
	Category   string  `json:"category,omitempty"`
	InitialLoss float64 `json:"initial_loss"`
	FinalLoss   float64 `json:"final_loss"`
	EpochCount  int     `json:"epoch_count,omitempty"`
	TokenCount  int     `json:"token_count,omitempty"`
}

// TrainingLogEntry is the raw format from the training pipeline's log output.
// Each line in the JSONL file represents one TDU's training metrics.
type TrainingLogEntry struct {
	// Standard fields from training pipeline _meta
	Meta struct {
		ID          string  `json:"id"`
		Domain      string  `json:"domain"`
		QualityTier string  `json:"quality_tier"`
		TokenEstimate int   `json:"token_estimate"`
	} `json:"_meta"`

	// Training metrics
	Loss        float64 `json:"loss"`
	InitialLoss float64 `json:"initial_loss"`
	FinalLoss   float64 `json:"final_loss"`
	Epochs      int     `json:"epochs"`

	// Alternative flat format
	TDUID    string  `json:"tdu_id,omitempty"`
	Domain   string  `json:"domain,omitempty"`
	Category string  `json:"category,omitempty"`
}

// LossTracker handles parsing and aggregation of per-TDU training losses.
type LossTracker struct{}

// NewLossTracker creates a new loss tracker.
func NewLossTracker() *LossTracker {
	return &LossTracker{}
}

// ParseTrainingLog reads a JSONL training log and extracts per-TDU loss entries.
func (t *LossTracker) ParseTrainingLog(path string) ([]LossEntry, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open training log: %w", err)
	}
	defer f.Close()

	var entries []LossEntry
	scanner := bufio.NewScanner(f)

	// Increase buffer size for large lines
	buf := make([]byte, 0, 1024*1024)
	scanner.Buffer(buf, 10*1024*1024)

	lineNum := 0
	for scanner.Scan() {
		lineNum++
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		var raw TrainingLogEntry
		if err := json.Unmarshal(line, &raw); err != nil {
			continue // skip malformed lines
		}

		entry := t.normalizeEntry(raw)
		if entry.TDUID == "" {
			continue // skip entries without TDU ID
		}

		entries = append(entries, entry)
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scan training log: %w", err)
	}

	return entries, nil
}

// normalizeEntry converts a raw log entry to a standardized LossEntry.
func (t *LossTracker) normalizeEntry(raw TrainingLogEntry) LossEntry {
	entry := LossEntry{}

	// Prefer Meta fields, fall back to flat fields
	if raw.Meta.ID != "" {
		entry.TDUID = raw.Meta.ID
	} else if raw.TDUID != "" {
		entry.TDUID = raw.TDUID
	}

	if raw.Meta.Domain != "" {
		entry.Domain = raw.Meta.Domain
	} else if raw.Domain != "" {
		entry.Domain = raw.Domain
	}

	entry.Category = raw.Category

	// Loss values
	if raw.FinalLoss > 0 {
		entry.FinalLoss = raw.FinalLoss
	} else if raw.Loss > 0 {
		entry.FinalLoss = raw.Loss
	}

	if raw.InitialLoss > 0 {
		entry.InitialLoss = raw.InitialLoss
	}

	entry.EpochCount = raw.Epochs
	entry.TokenCount = raw.Meta.TokenEstimate

	return entry
}

// AggregateBatchLoss computes aggregate loss metrics for a batch of TDU IDs.
func (t *LossTracker) AggregateBatchLoss(entries []LossEntry, batchIDs []string) BatchLossStats {
	idSet := make(map[string]bool, len(batchIDs))
	for _, id := range batchIDs {
		idSet[id] = true
	}

	var losses []float64
	for _, e := range entries {
		if idSet[e.TDUID] {
			losses = append(losses, e.FinalLoss)
		}
	}

	if len(losses) == 0 {
		return BatchLossStats{}
	}

	sort.Float64s(losses)
	sum := 0.0
	for _, l := range losses {
		sum += l
	}

	return BatchLossStats{
		Count:      len(losses),
		MeanLoss:   sum / float64(len(losses)),
		MedianLoss: losses[len(losses)/2],
		MinLoss:    losses[0],
		MaxLoss:    losses[len(losses)-1],
	}
}

// BatchLossStats holds aggregate loss metrics for a batch.
type BatchLossStats struct {
	Count      int     `json:"count"`
	MeanLoss   float64 `json:"mean_loss"`
	MedianLoss float64 `json:"median_loss"`
	MinLoss    float64 `json:"min_loss"`
	MaxLoss    float64 `json:"max_loss"`
}
