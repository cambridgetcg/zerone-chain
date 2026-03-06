package main

import (
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
)

// TDURecord represents a training data unit in enclave memory.
type TDURecord struct {
	ID           string  `json:"id"`
	Content      []byte  `json:"content"`
	Domain       string  `json:"domain"`
	Hash         []byte  `json:"hash"`
	FitnessScore float64 `json:"fitness_score"` // 0.0–1.0
}

// Fitness thresholds matching on-chain constants from x/knowledge/types/fitness.go.
const (
	FitnessThresholdDormant = 0.3 // below this = dormant
	FitnessThresholdPruned  = 0.1 // below this = pruned
)

// IsDormant returns true if the TDU's fitness score puts it in dormant status.
func (t *TDURecord) IsDormant() bool {
	return t.FitnessScore >= FitnessThresholdPruned && t.FitnessScore < FitnessThresholdDormant
}

// IsPruned returns true if the TDU's fitness score puts it in pruned status.
func (t *TDURecord) IsPruned() bool {
	return t.FitnessScore < FitnessThresholdPruned
}

// IsTrainable returns true if the TDU should be included in training (Core or Active).
func (t *TDURecord) IsTrainable() bool {
	return t.FitnessScore >= FitnessThresholdDormant
}

// TrainingSample is the HuggingFace-compatible training format.
type TrainingSample struct {
	ID       string `json:"id"`
	Text     string `json:"text"`
	Domain   string `json:"domain"`
	Metadata string `json:"metadata,omitempty"`
}

// PreparedDataset holds the filtered, formatted, in-memory dataset.
type PreparedDataset struct {
	Samples            []TrainingSample `json:"samples"`
	DomainSampleCount  int             `json:"domain_sample_count"`
	GeneralSampleCount int             `json:"general_sample_count"`
	TotalTDUs          int             `json:"total_tdus"`           // before filtering
	FilteredCount      int             `json:"filtered_count"`       // removed by fitness filter
	Fingerprint        []byte          `json:"fingerprint"`          // SHA-256 of complete dataset
}

// Preparator handles dataset assembly, fitness filtering, domain mixing,
// and formatting for training.
type Preparator struct {
	TargetDomain   string
	DomainMixRatio float64 // default 0.7 (70% domain, 30% general)
}

// Prepare takes raw TDU records and produces a prepared dataset:
// 1. Filter out Dormant and Pruned TDUs
// 2. Apply domain mix ratio (70% domain / 30% general)
// 3. Format for HuggingFace training
// 4. Compute dataset fingerprint
func (p *Preparator) Prepare(records []TDURecord) (*PreparedDataset, error) {
	if len(records) == 0 {
		return nil, errors.New("no TDU records to prepare")
	}

	totalTDUs := len(records)

	// Step 1: Fitness filter — exclude Dormant and Pruned TDUs.
	var trainable []TDURecord
	filteredCount := 0
	for i := range records {
		if records[i].IsTrainable() {
			trainable = append(trainable, records[i])
		} else {
			filteredCount++
		}
	}

	if len(trainable) == 0 {
		return nil, errors.New("all TDUs filtered by fitness (none trainable)")
	}

	// Step 2: Split by domain for mix ratio.
	var domainTDUs, generalTDUs []TDURecord
	for i := range trainable {
		if trainable[i].Domain == p.TargetDomain {
			domainTDUs = append(domainTDUs, trainable[i])
		} else {
			generalTDUs = append(generalTDUs, trainable[i])
		}
	}

	// Step 3: Apply domain mix ratio.
	selected := p.applyDomainMix(domainTDUs, generalTDUs)

	if len(selected) == 0 {
		// If mix produces nothing, use all trainable TDUs.
		selected = trainable
	}

	// Step 4: Format to HuggingFace training format.
	samples := make([]TrainingSample, 0, len(selected))
	for _, tdu := range selected {
		samples = append(samples, TrainingSample{
			ID:     tdu.ID,
			Text:   string(tdu.Content),
			Domain: tdu.Domain,
		})
	}

	// Step 5: Compute dataset fingerprint (deterministic ordering by ID).
	fingerprint := computeFingerprint(samples)

	domainCount := 0
	generalCount := 0
	for _, s := range samples {
		if s.Domain == p.TargetDomain {
			domainCount++
		} else {
			generalCount++
		}
	}

	return &PreparedDataset{
		Samples:            samples,
		DomainSampleCount:  domainCount,
		GeneralSampleCount: generalCount,
		TotalTDUs:          totalTDUs,
		FilteredCount:      filteredCount,
		Fingerprint:        fingerprint,
	}, nil
}

// applyDomainMix selects TDUs according to the domain mix ratio.
// Target: DomainMixRatio of domain-specific, (1-DomainMixRatio) general.
func (p *Preparator) applyDomainMix(domainTDUs, generalTDUs []TDURecord) []TDURecord {
	total := len(domainTDUs) + len(generalTDUs)
	if total == 0 {
		return nil
	}

	// If only one category exists, use all of it.
	if len(domainTDUs) == 0 {
		return generalTDUs
	}
	if len(generalTDUs) == 0 {
		return domainTDUs
	}

	targetDomain := int(float64(total) * p.DomainMixRatio)
	targetGeneral := total - targetDomain

	// Cap to available.
	if targetDomain > len(domainTDUs) {
		targetDomain = len(domainTDUs)
		targetGeneral = total - targetDomain
	}
	if targetGeneral > len(generalTDUs) {
		targetGeneral = len(generalTDUs)
	}

	result := make([]TDURecord, 0, targetDomain+targetGeneral)
	result = append(result, domainTDUs[:targetDomain]...)
	result = append(result, generalTDUs[:targetGeneral]...)
	return result
}

// computeFingerprint computes SHA-256 over the sorted, serialized dataset.
func computeFingerprint(samples []TrainingSample) []byte {
	// Sort by ID for deterministic fingerprinting.
	sorted := make([]TrainingSample, len(samples))
	copy(sorted, samples)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].ID < sorted[j].ID
	})

	bz, err := json.Marshal(sorted)
	if err != nil {
		// Fallback: hash empty.
		h := sha256.Sum256(nil)
		return h[:]
	}

	h := sha256.Sum256(bz)
	return h[:]
}

// FormatForExport serializes the dataset as JSON lines (JSONL) for training tools.
func FormatForExport(samples []TrainingSample) ([]byte, error) {
	var buf []byte
	for _, s := range samples {
		line, err := json.Marshal(s)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal sample %s: %w", s.ID, err)
		}
		buf = append(buf, line...)
		buf = append(buf, '\n')
	}
	return buf, nil
}
