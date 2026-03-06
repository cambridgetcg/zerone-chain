package main

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// InfluenceResult represents the influence score for a single TDU.
type InfluenceResult struct {
	TDUID     string  `json:"tdu_id"`
	Score     float64 `json:"score"`     // -1.0 to 1.0 (negative = harmful)
	Method    string  `json:"method"`    // "leave-one-out" | "loss-based"
	Benchmark string  `json:"benchmark"` // domain or "overall"
}

// InfluenceReport is the full output of an influence analysis run.
type InfluenceReport struct {
	Method         string          `json:"method"`
	Timestamp      string          `json:"timestamp"`
	TDUCount       int             `json:"tdu_count"`
	Results        []InfluenceResult `json:"results"`
	BenchmarkDelta *BenchmarkDelta `json:"benchmark_delta,omitempty"`
	Summary        *ReportSummary  `json:"summary,omitempty"`
}

// BenchmarkDelta captures the score change between baseline and candidate models.
type BenchmarkDelta struct {
	BaselineScore  float64            `json:"baseline_score"`
	CandidateScore float64            `json:"candidate_score"`
	OverallDelta   float64            `json:"overall_delta"`
	DomainDeltas   map[string]float64 `json:"domain_deltas,omitempty"`
}

// ReportSummary holds aggregate statistics.
type ReportSummary struct {
	MeanScore       float64            `json:"mean_score"`
	MedianScore     float64            `json:"median_score"`
	StdDev          float64            `json:"std_dev"`
	HelpfulCount    int                `json:"helpful_count"`
	HarmfulCount    int                `json:"harmful_count"`
	NeutralCount    int                `json:"neutral_count"`
	DomainBreakdown map[string]float64 `json:"domain_breakdown,omitempty"`
}

// BenchmarkReport mirrors the benchmark suite's report structure.
type BenchmarkReport struct {
	ModelVersion   string             `json:"model_version"`
	DatasetVersion string             `json:"dataset_version"`
	Timestamp      string             `json:"timestamp"`
	OverallScore   float64            `json:"overall_score"`
	DomainScores   map[string]float64 `json:"domain_scores"`
}

// LeaveOneOutReport is a benchmark report with the excluded batch ID.
type LeaveOneOutReport struct {
	BenchmarkReport
	ExcludedBatch string `json:"excluded_batch"`
}

// Analyzer implements influence analysis logic.
type Analyzer struct{}

// NewAnalyzer creates a new analyzer.
func NewAnalyzer() *Analyzer {
	return &Analyzer{}
}

// AnalyzeLossBased computes influence scores from per-TDU training loss entries.
//
// Strategy:
//   - Normalize losses to [0, 1] range (min-max scaling)
//   - Low loss = model learned easily (could be redundant OR foundational)
//   - High loss = model struggled (could be novel OR noisy)
//   - Cross-reference with benchmark delta to determine direction:
//     If benchmark improved: low-loss TDUs get positive scores (foundational)
//     If benchmark worsened: high-loss TDUs get negative scores (noisy data)
//     Without benchmark data: use loss distribution to estimate (median as neutral)
func (a *Analyzer) AnalyzeLossBased(entries []LossEntry, delta *BenchmarkDelta) []InfluenceResult {
	if len(entries) == 0 {
		return nil
	}

	// Find min/max loss for normalization
	minLoss := entries[0].FinalLoss
	maxLoss := entries[0].FinalLoss
	for _, e := range entries {
		if e.FinalLoss < minLoss {
			minLoss = e.FinalLoss
		}
		if e.FinalLoss > maxLoss {
			maxLoss = e.FinalLoss
		}
	}
	lossRange := maxLoss - minLoss
	if lossRange == 0 {
		lossRange = 1.0 // avoid division by zero
	}

	// Compute median loss
	losses := make([]float64, len(entries))
	for i, e := range entries {
		losses[i] = e.FinalLoss
	}
	sort.Float64s(losses)
	medianLoss := losses[len(losses)/2]

	// Determine overall training direction from benchmark
	trainingDirection := 0.0 // neutral
	if delta != nil {
		trainingDirection = delta.OverallDelta
	}

	results := make([]InfluenceResult, 0, len(entries))
	for _, e := range entries {
		// Normalized loss position: 0 = lowest loss, 1 = highest loss
		normalizedLoss := (e.FinalLoss - minLoss) / lossRange

		score := computeLossInfluence(normalizedLoss, e.FinalLoss, medianLoss, trainingDirection)

		// Clamp to [-1, 1]
		score = clamp(score, -1.0, 1.0)

		domain := e.Domain
		if domain == "" {
			domain = "overall"
		}

		results = append(results, InfluenceResult{
			TDUID:     e.TDUID,
			Score:     math.Round(score*10000) / 10000, // 4 decimal places
			Method:    "loss-based",
			Benchmark: domain,
		})
	}

	return results
}

// computeLossInfluence derives an influence score from loss metrics.
//
// Intuition:
//   - TDUs with loss near median are "typical" training data — moderate positive influence
//   - TDUs with very low loss may be redundant (model already knew this)
//   - TDUs with very high loss may be noisy/contradictory
//   - When benchmark improves, all data contributed positively (scale by delta)
//   - When benchmark regresses, high-loss data is likely harmful
func computeLossInfluence(normalizedLoss, rawLoss, medianLoss, trainingDirection float64) float64 {
	// Distance from median (0 = at median, higher = further away)
	distFromMedian := math.Abs(rawLoss-medianLoss) / (medianLoss + 1e-8)

	// Base score: centered around 0, positive for near-median, negative for outliers
	// Bell-curve shape: score = exp(-2 * dist^2) mapped to [-0.5, 0.5]
	baseScore := math.Exp(-2*distFromMedian*distFromMedian) - 0.5

	// Adjust by training direction
	if trainingDirection > 0 {
		// Training helped: boost near-median TDUs, still penalize extreme outliers
		baseScore += 0.3 * trainingDirection
	} else if trainingDirection < 0 {
		// Training hurt: penalize high-loss (noisy) TDUs more
		if normalizedLoss > 0.7 {
			baseScore += trainingDirection * normalizedLoss // more negative for higher loss
		} else {
			baseScore += 0.1 * trainingDirection // slight penalty for all
		}
	}

	return baseScore
}

// AnalyzeLeaveOneOut computes influence by comparing benchmark scores
// with and without each TDU batch.
func (a *Analyzer) AnalyzeLeaveOneOut(baselinePath, batchDir string) ([]InfluenceResult, *BenchmarkDelta, error) {
	// Load baseline report
	baseline, err := loadBenchmarkReport(baselinePath)
	if err != nil {
		return nil, nil, fmt.Errorf("load baseline: %w", err)
	}

	// Find all leave-one-out reports in the directory
	entries, err := os.ReadDir(batchDir)
	if err != nil {
		return nil, nil, fmt.Errorf("read batch dir: %w", err)
	}

	var results []InfluenceResult
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}

		path := filepath.Join(batchDir, entry.Name())
		var looReport LeaveOneOutReport
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		if err := json.Unmarshal(data, &looReport); err != nil {
			continue
		}
		if looReport.ExcludedBatch == "" {
			continue
		}

		// Score drop when removing batch = batch had positive influence
		// Score improvement when removing batch = batch had negative influence
		overallDelta := baseline.OverallScore - looReport.OverallScore

		// Normalize: a 10% score drop is a strong positive signal
		// Scale so that typical deltas map to [-1, 1]
		score := clamp(overallDelta*10, -1.0, 1.0)

		results = append(results, InfluenceResult{
			TDUID:     looReport.ExcludedBatch,
			Score:     math.Round(score*10000) / 10000,
			Method:    "leave-one-out",
			Benchmark: "overall",
		})

		// Per-domain influence
		for domain, baseScore := range baseline.DomainScores {
			if looScore, ok := looReport.DomainScores[domain]; ok {
				domainDelta := baseScore - looScore
				domainScore := clamp(domainDelta*10, -1.0, 1.0)

				results = append(results, InfluenceResult{
					TDUID:     looReport.ExcludedBatch,
					Score:     math.Round(domainScore*10000) / 10000,
					Method:    "leave-one-out",
					Benchmark: domain,
				})
			}
		}
	}

	benchDelta := &BenchmarkDelta{
		BaselineScore:  baseline.OverallScore,
		CandidateScore: baseline.OverallScore, // with all data = baseline
		OverallDelta:   0,
		DomainDeltas:   make(map[string]float64),
	}

	return results, benchDelta, nil
}

// LoadBenchmarkDelta loads two benchmark reports and computes the delta.
func (a *Analyzer) LoadBenchmarkDelta(baselinePath, candidatePath string) (*BenchmarkDelta, error) {
	baseline, err := loadBenchmarkReport(baselinePath)
	if err != nil {
		return nil, fmt.Errorf("load baseline: %w", err)
	}

	candidate, err := loadBenchmarkReport(candidatePath)
	if err != nil {
		return nil, fmt.Errorf("load candidate: %w", err)
	}

	delta := &BenchmarkDelta{
		BaselineScore:  baseline.OverallScore,
		CandidateScore: candidate.OverallScore,
		OverallDelta:   candidate.OverallScore - baseline.OverallScore,
		DomainDeltas:   make(map[string]float64),
	}

	for domain, bScore := range baseline.DomainScores {
		if cScore, ok := candidate.DomainScores[domain]; ok {
			delta.DomainDeltas[domain] = cScore - bScore
		}
	}

	return delta, nil
}

// ComputeSummary populates aggregate statistics on the report.
func (r *InfluenceReport) ComputeSummary() {
	if len(r.Results) == 0 {
		r.Summary = &ReportSummary{}
		return
	}

	scores := make([]float64, len(r.Results))
	domainSums := make(map[string]float64)
	domainCounts := make(map[string]int)
	helpful, harmful, neutral := 0, 0, 0

	for i, res := range r.Results {
		scores[i] = res.Score
		domainSums[res.Benchmark] += res.Score
		domainCounts[res.Benchmark]++

		switch {
		case res.Score > 0:
			helpful++
		case res.Score < 0:
			harmful++
		default:
			neutral++
		}
	}

	// Mean
	sum := 0.0
	for _, s := range scores {
		sum += s
	}
	mean := sum / float64(len(scores))

	// Median
	sort.Float64s(scores)
	median := scores[len(scores)/2]

	// Std dev
	variance := 0.0
	for _, s := range scores {
		d := s - mean
		variance += d * d
	}
	variance /= float64(len(scores))
	stdDev := math.Sqrt(variance)

	// Domain breakdown
	domainAvgs := make(map[string]float64)
	for domain, s := range domainSums {
		domainAvgs[domain] = math.Round(s/float64(domainCounts[domain])*10000) / 10000
	}

	r.Summary = &ReportSummary{
		MeanScore:       math.Round(mean*10000) / 10000,
		MedianScore:     math.Round(median*10000) / 10000,
		StdDev:          math.Round(stdDev*10000) / 10000,
		HelpfulCount:    helpful,
		HarmfulCount:    harmful,
		NeutralCount:    neutral,
		DomainBreakdown: domainAvgs,
	}
}

func loadBenchmarkReport(path string) (*BenchmarkReport, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var report BenchmarkReport
	if err := json.Unmarshal(data, &report); err != nil {
		return nil, err
	}
	return &report, nil
}

func clamp(v, min, max float64) float64 {
	if v < min {
		return min
	}
	if v > max {
		return max
	}
	return v
}
