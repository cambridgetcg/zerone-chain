package main

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/zerone-chain/zerone/services/benchmark/domains"
)

// Reporter handles result aggregation, output, and comparison.
type Reporter struct{}

// NewReporter creates a new reporter.
func NewReporter() *Reporter {
	return &Reporter{}
}

// Aggregate computes per-domain and overall scores from individual case results.
func (r *Reporter) Aggregate(results []domains.CaseResult, model, datasetVersion string) *domains.BenchmarkReport {
	report := &domains.BenchmarkReport{
		ModelVersion:   model,
		DatasetVersion: datasetVersion,
		Timestamp:      time.Now().UTC().Format(time.RFC3339),
		DomainScores:   make(map[string]float64),
		PerCaseResults: results,
	}

	// Group by domain
	domainResults := make(map[string][]domains.CaseResult)
	for _, res := range results {
		domainResults[res.Domain] = append(domainResults[res.Domain], res)
	}

	// Compute per-domain scores
	totalScore := 0.0
	totalCount := 0
	for domain, dResults := range domainResults {
		sum := 0.0
		for _, res := range dResults {
			sum += res.Score
		}
		avg := sum / float64(len(dResults))
		report.DomainScores[domain] = math.Round(avg*100) / 100

		totalScore += sum
		totalCount += len(dResults)
	}

	// Overall score
	if totalCount > 0 {
		report.OverallScore = math.Round(totalScore/float64(totalCount)*100) / 100
	}

	return report
}

// WriteJSON writes a report to a JSON file.
func (r *Reporter) WriteJSON(report *domains.BenchmarkReport, path string) error {
	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal report: %w", err)
	}
	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("write report: %w", err)
	}
	return nil
}

// LoadReport reads a report from a JSON file.
func (r *Reporter) LoadReport(path string) (*domains.BenchmarkReport, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read report: %w", err)
	}
	var report domains.BenchmarkReport
	if err := json.Unmarshal(data, &report); err != nil {
		return nil, fmt.Errorf("unmarshal report: %w", err)
	}
	return &report, nil
}

// PrintSummary outputs a human-readable summary of the benchmark results.
func (r *Reporter) PrintSummary(report *domains.BenchmarkReport) {
	fmt.Println("═══════════════════════════════════════════════════════════")
	fmt.Printf("  Benchmark Report: %s\n", report.ModelVersion)
	fmt.Printf("  Dataset: %s\n", report.DatasetVersion)
	fmt.Printf("  Time: %s\n", report.Timestamp)
	fmt.Println("═══════════════════════════════════════════════════════════")
	fmt.Println()

	// Domain scores sorted by name
	domainNames := make([]string, 0, len(report.DomainScores))
	for name := range report.DomainScores {
		domainNames = append(domainNames, name)
	}
	sort.Strings(domainNames)

	fmt.Println("  Domain Scores:")
	for _, name := range domainNames {
		score := report.DomainScores[name]
		bar := scoreBar(score, 30)
		fmt.Printf("    %-15s %s %.2f\n", name, bar, score)
	}
	fmt.Println()

	// Category breakdown
	categoryScores := make(map[string][]float64)
	for _, res := range report.PerCaseResults {
		key := fmt.Sprintf("%s/%s", res.Domain, res.Category)
		categoryScores[key] = append(categoryScores[key], res.Score)
	}

	catNames := make([]string, 0, len(categoryScores))
	for name := range categoryScores {
		catNames = append(catNames, name)
	}
	sort.Strings(catNames)

	fmt.Println("  Category Breakdown:")
	for _, name := range catNames {
		scores := categoryScores[name]
		avg := average(scores)
		pass := 0
		for _, s := range scores {
			if s >= 0.5 {
				pass++
			}
		}
		fmt.Printf("    %-30s avg=%.2f  pass=%d/%d\n", name, avg, pass, len(scores))
	}
	fmt.Println()

	// Overall
	fmt.Printf("  Overall Score: %.2f\n", report.OverallScore)
	fmt.Printf("  Total Cases: %d\n", len(report.PerCaseResults))

	pass := 0
	for _, res := range report.PerCaseResults {
		if res.Pass {
			pass++
		}
	}
	fmt.Printf("  Pass Rate: %d/%d (%.1f%%)\n", pass, len(report.PerCaseResults),
		float64(pass)/float64(len(report.PerCaseResults))*100)

	// Latency stats
	if len(report.PerCaseResults) > 0 {
		var totalLatency int64
		minLatency := report.PerCaseResults[0].LatencyMs
		maxLatency := report.PerCaseResults[0].LatencyMs
		for _, res := range report.PerCaseResults {
			totalLatency += res.LatencyMs
			if res.LatencyMs < minLatency {
				minLatency = res.LatencyMs
			}
			if res.LatencyMs > maxLatency {
				maxLatency = res.LatencyMs
			}
		}
		avgLatency := totalLatency / int64(len(report.PerCaseResults))
		fmt.Printf("  Latency: avg=%dms min=%dms max=%dms\n", avgLatency, minLatency, maxLatency)
	}

	fmt.Println("═══════════════════════════════════════════════════════════")
}

// Compare produces a comparison between a baseline and candidate report.
func (r *Reporter) Compare(baseline, candidate *domains.BenchmarkReport) string {
	var b strings.Builder

	b.WriteString("═══════════════════════════════════════════════════════════\n")
	b.WriteString(fmt.Sprintf("  Comparison: %s vs %s\n", baseline.ModelVersion, candidate.ModelVersion))
	b.WriteString("═══════════════════════════════════════════════════════════\n\n")

	// Overall
	diff := candidate.OverallScore - baseline.OverallScore
	direction := "+"
	if diff < 0 {
		direction = ""
	}
	b.WriteString(fmt.Sprintf("  Overall: %.2f -> %.2f (%s%.2f)\n\n",
		baseline.OverallScore, candidate.OverallScore, direction, diff))

	// Per-domain comparison
	allDomains := make(map[string]bool)
	for d := range baseline.DomainScores {
		allDomains[d] = true
	}
	for d := range candidate.DomainScores {
		allDomains[d] = true
	}

	domainNames := make([]string, 0, len(allDomains))
	for d := range allDomains {
		domainNames = append(domainNames, d)
	}
	sort.Strings(domainNames)

	b.WriteString("  Domain Comparison:\n")
	regressions := 0
	improvements := 0
	for _, d := range domainNames {
		bScore := baseline.DomainScores[d]
		cScore := candidate.DomainScores[d]
		diff := cScore - bScore

		indicator := " "
		if diff > 0.01 {
			indicator = "+"
			improvements++
		} else if diff < -0.01 {
			indicator = "-"
			regressions++
		}

		direction := "+"
		if diff < 0 {
			direction = ""
		}

		b.WriteString(fmt.Sprintf("    %s %-15s %.2f -> %.2f (%s%.2f)\n",
			indicator, d, bScore, cScore, direction, diff))
	}
	b.WriteString("\n")

	b.WriteString(fmt.Sprintf("  Improvements: %d  Regressions: %d\n", improvements, regressions))

	if regressions > 0 {
		b.WriteString("\n  WARNING: Regressions detected!\n")

		// List regressed cases
		baseScores := make(map[string]float64)
		for _, res := range baseline.PerCaseResults {
			baseScores[res.CaseID] = res.Score
		}

		b.WriteString("  Regressed cases:\n")
		for _, res := range candidate.PerCaseResults {
			if bScore, ok := baseScores[res.CaseID]; ok {
				if res.Score < bScore-0.01 {
					b.WriteString(fmt.Sprintf("    %s: %.2f -> %.2f\n", res.CaseID, bScore, res.Score))
				}
			}
		}
	}

	b.WriteString("═══════════════════════════════════════════════════════════\n")
	return b.String()
}

func scoreBar(score float64, width int) string {
	filled := int(score * float64(width))
	if filled > width {
		filled = width
	}
	return "[" + strings.Repeat("█", filled) + strings.Repeat("░", width-filled) + "]"
}

func average(vals []float64) float64 {
	if len(vals) == 0 {
		return 0
	}
	sum := 0.0
	for _, v := range vals {
		sum += v
	}
	return sum / float64(len(vals))
}
