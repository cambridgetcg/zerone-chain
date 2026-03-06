package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"
)

var (
	flagTrainingLog string
	flagBenchBase   string
	flagBenchWith   string
	flagBenchDir    string
	flagOutputDir   string
	flagMethod      string
	flagSignalsFile string
	flagNodeURL     string
	flagChainID     string
	flagFrom        string
	flagBatchSize   int
)

func main() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

var rootCmd = &cobra.Command{
	Use:   "zerone-influence",
	Short: "ZERONE Influence Analysis — trace model quality to training data",
}

var analyzeCmd = &cobra.Command{
	Use:   "analyze",
	Short: "Run influence analysis on training results",
	RunE:  runAnalyze,
}

var emitCmd = &cobra.Command{
	Use:   "emit",
	Short: "Generate fitness signal file from influence analysis results",
	RunE:  runEmit,
}

var reportCmd = &cobra.Command{
	Use:   "report",
	Short: "Print human-readable influence analysis summary",
	RunE:  runReport,
}

func init() {
	analyzeCmd.Flags().StringVar(&flagTrainingLog, "training-log", "", "Path to training log JSONL (per-TDU losses)")
	analyzeCmd.Flags().StringVar(&flagBenchBase, "bench-baseline", "", "Path to baseline benchmark report JSON")
	analyzeCmd.Flags().StringVar(&flagBenchWith, "bench-candidate", "", "Path to candidate benchmark report JSON (after training)")
	analyzeCmd.Flags().StringVar(&flagBenchDir, "bench-dir", "", "Directory of leave-one-out benchmark reports")
	analyzeCmd.Flags().StringVar(&flagOutputDir, "output-dir", "results", "Output directory for analysis results")
	analyzeCmd.Flags().StringVar(&flagMethod, "method", "loss-based", "Analysis method: loss-based | leave-one-out")

	emitCmd.Flags().StringVar(&flagSignalsFile, "analysis-file", "", "Path to influence analysis JSON")
	emitCmd.Flags().StringVar(&flagOutputDir, "output-dir", "results", "Output directory for signals file")
	_ = emitCmd.MarkFlagRequired("analysis-file")

	reportCmd.Flags().StringVar(&flagSignalsFile, "analysis-file", "", "Path to influence analysis JSON")
	_ = reportCmd.MarkFlagRequired("analysis-file")

	rootCmd.AddCommand(analyzeCmd, emitCmd, reportCmd)
}

func runAnalyze(_ *cobra.Command, _ []string) error {
	if flagMethod == "leave-one-out" {
		return runLeaveOneOut()
	}
	return runLossBased()
}

func runLossBased() error {
	if flagTrainingLog == "" {
		return fmt.Errorf("--training-log is required for loss-based analysis")
	}

	tracker := NewLossTracker()
	entries, err := tracker.ParseTrainingLog(flagTrainingLog)
	if err != nil {
		return fmt.Errorf("parse training log: %w", err)
	}
	fmt.Printf("Parsed %d TDU loss entries from training log\n", len(entries))

	analyzer := NewAnalyzer()

	// Load benchmark reports if provided for cross-referencing
	var benchDelta *BenchmarkDelta
	if flagBenchBase != "" && flagBenchWith != "" {
		benchDelta, err = analyzer.LoadBenchmarkDelta(flagBenchBase, flagBenchWith)
		if err != nil {
			return fmt.Errorf("load benchmark delta: %w", err)
		}
		fmt.Printf("Benchmark delta: overall %.4f -> %.4f (delta %+.4f)\n",
			benchDelta.BaselineScore, benchDelta.CandidateScore, benchDelta.OverallDelta)
	}

	results := analyzer.AnalyzeLossBased(entries, benchDelta)
	fmt.Printf("Generated %d influence results\n", len(results))

	report := &InfluenceReport{
		Method:    "loss-based",
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		TDUCount:  len(results),
		Results:   results,
	}

	if benchDelta != nil {
		report.BenchmarkDelta = benchDelta
	}

	// Compute summary stats
	report.ComputeSummary()

	if err := os.MkdirAll(flagOutputDir, 0755); err != nil {
		return fmt.Errorf("create output dir: %w", err)
	}

	outputFile := filepath.Join(flagOutputDir,
		fmt.Sprintf("influence-%s-%s.json", flagMethod, time.Now().Format("20060102-150405")))
	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal report: %w", err)
	}
	if err := os.WriteFile(outputFile, data, 0644); err != nil {
		return fmt.Errorf("write report: %w", err)
	}

	printInfluenceSummary(report)
	fmt.Printf("\nResults saved to %s\n", outputFile)
	return nil
}

func runLeaveOneOut() error {
	if flagBenchBase == "" || flagBenchDir == "" {
		return fmt.Errorf("--bench-baseline and --bench-dir required for leave-one-out")
	}

	analyzer := NewAnalyzer()
	results, benchDelta, err := analyzer.AnalyzeLeaveOneOut(flagBenchBase, flagBenchDir)
	if err != nil {
		return fmt.Errorf("leave-one-out analysis: %w", err)
	}
	fmt.Printf("Generated %d influence results from leave-one-out\n", len(results))

	report := &InfluenceReport{
		Method:         "leave-one-out",
		Timestamp:      time.Now().UTC().Format(time.RFC3339),
		TDUCount:       len(results),
		Results:        results,
		BenchmarkDelta: benchDelta,
	}
	report.ComputeSummary()

	if err := os.MkdirAll(flagOutputDir, 0755); err != nil {
		return fmt.Errorf("create output dir: %w", err)
	}

	outputFile := filepath.Join(flagOutputDir,
		fmt.Sprintf("influence-%s-%s.json", flagMethod, time.Now().Format("20060102-150405")))
	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal report: %w", err)
	}
	if err := os.WriteFile(outputFile, data, 0644); err != nil {
		return fmt.Errorf("write report: %w", err)
	}

	printInfluenceSummary(report)
	fmt.Printf("\nResults saved to %s\n", outputFile)
	return nil
}

func runEmit(_ *cobra.Command, _ []string) error {
	data, err := os.ReadFile(flagSignalsFile)
	if err != nil {
		return fmt.Errorf("read analysis file: %w", err)
	}

	var report InfluenceReport
	if err := json.Unmarshal(data, &report); err != nil {
		return fmt.Errorf("unmarshal analysis: %w", err)
	}

	emitter := NewSignalEmitter()
	signals := emitter.ConvertToFitnessSignals(report.Results)
	fmt.Printf("Generated %d fitness signals\n", len(signals))

	batch := emitter.BuildBatch(signals)

	if err := os.MkdirAll(flagOutputDir, 0755); err != nil {
		return fmt.Errorf("create output dir: %w", err)
	}

	outputFile := filepath.Join(flagOutputDir, "fitness-signals.json")
	out, err := json.MarshalIndent(batch, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal signals: %w", err)
	}
	if err := os.WriteFile(outputFile, out, 0644); err != nil {
		return fmt.Errorf("write signals: %w", err)
	}

	fmt.Printf("Fitness signals written to %s\n", outputFile)
	fmt.Printf("Submit on-chain with:\n")
	fmt.Printf("  zeroned tx knowledge update-fitness-batch --signals-file %s --from fitness-oracle\n", outputFile)
	return nil
}

func runReport(_ *cobra.Command, _ []string) error {
	data, err := os.ReadFile(flagSignalsFile)
	if err != nil {
		return fmt.Errorf("read analysis file: %w", err)
	}

	var report InfluenceReport
	if err := json.Unmarshal(data, &report); err != nil {
		return fmt.Errorf("unmarshal analysis: %w", err)
	}

	printInfluenceSummary(&report)
	return nil
}

func printInfluenceSummary(report *InfluenceReport) {
	fmt.Println("===============================================================")
	fmt.Printf("  Influence Analysis Report (%s)\n", report.Method)
	fmt.Printf("  Time: %s\n", report.Timestamp)
	fmt.Printf("  TDU Count: %d\n", report.TDUCount)
	fmt.Println("===============================================================")
	fmt.Println()

	if report.Summary != nil {
		fmt.Println("  Summary:")
		fmt.Printf("    Mean Score:   %+.4f\n", report.Summary.MeanScore)
		fmt.Printf("    Median Score: %+.4f\n", report.Summary.MedianScore)
		fmt.Printf("    Std Dev:      %.4f\n", report.Summary.StdDev)
		fmt.Printf("    Helpful:      %d (score > 0)\n", report.Summary.HelpfulCount)
		fmt.Printf("    Harmful:      %d (score < 0)\n", report.Summary.HarmfulCount)
		fmt.Printf("    Neutral:      %d (score = 0)\n", report.Summary.NeutralCount)
		fmt.Println()

		if len(report.Summary.DomainBreakdown) > 0 {
			fmt.Println("  Domain Breakdown:")
			for domain, avg := range report.Summary.DomainBreakdown {
				fmt.Printf("    %-15s avg=%+.4f\n", domain, avg)
			}
			fmt.Println()
		}
	}

	if report.BenchmarkDelta != nil {
		fmt.Println("  Benchmark Delta:")
		fmt.Printf("    Overall: %.4f -> %.4f (%+.4f)\n",
			report.BenchmarkDelta.BaselineScore, report.BenchmarkDelta.CandidateScore,
			report.BenchmarkDelta.OverallDelta)
		for domain, delta := range report.BenchmarkDelta.DomainDeltas {
			fmt.Printf("    %-15s %+.4f\n", domain, delta)
		}
		fmt.Println()
	}

	// Top 5 most helpful
	if len(report.Results) > 0 {
		sorted := sortByScore(report.Results)
		top := 5
		if top > len(sorted) {
			top = len(sorted)
		}
		fmt.Println("  Top Helpful TDUs:")
		for i := 0; i < top; i++ {
			r := sorted[i]
			fmt.Printf("    %s  score=%+.4f  domain=%s  method=%s\n",
				truncateID(r.TDUID), r.Score, r.Benchmark, r.Method)
		}

		fmt.Println()
		fmt.Println("  Top Harmful TDUs:")
		for i := len(sorted) - 1; i >= 0 && i >= len(sorted)-top; i-- {
			r := sorted[i]
			if r.Score >= 0 {
				break
			}
			fmt.Printf("    %s  score=%+.4f  domain=%s  method=%s\n",
				truncateID(r.TDUID), r.Score, r.Benchmark, r.Method)
		}
	}

	fmt.Println("===============================================================")
}

func truncateID(id string) string {
	if len(id) > 12 {
		return id[:12] + "..."
	}
	return id
}

func sortByScore(results []InfluenceResult) []InfluenceResult {
	sorted := make([]InfluenceResult, len(results))
	copy(sorted, results)
	for i := 0; i < len(sorted); i++ {
		for j := i + 1; j < len(sorted); j++ {
			if sorted[j].Score > sorted[i].Score {
				sorted[i], sorted[j] = sorted[j], sorted[i]
			}
		}
	}
	return sorted
}
