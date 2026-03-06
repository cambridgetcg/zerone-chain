package main

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"

	"github.com/zerone-chain/zerone/services/benchmark/domains"
)

var (
	flagEndpoint       string
	flagModel          string
	flagDomain         string
	flagDatasetDir     string
	flagOutputDir      string
	flagDatasetVersion string
	flagTimeout        time.Duration
	flagWorkers        int
	flagBaseline       string
	flagCandidate      string
)

func main() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

var rootCmd = &cobra.Command{
	Use:   "zerone-bench",
	Short: "ZERONE Benchmark Suite — evaluate model quality across domains",
}

var runCmd = &cobra.Command{
	Use:   "run",
	Short: "Run benchmark suite against an inference endpoint",
	RunE:  runBenchmark,
}

var compareCmd = &cobra.Command{
	Use:   "compare",
	Short: "Compare two benchmark result files",
	RunE:  runCompare,
}

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List available domains and their case counts",
	RunE:  runList,
}

func init() {
	// Resolve default dataset directory relative to the executable
	defaultDatasetDir := "datasets"
	if exe, err := os.Executable(); err == nil {
		defaultDatasetDir = filepath.Join(filepath.Dir(exe), "datasets")
	}

	runCmd.Flags().StringVar(&flagEndpoint, "endpoint", "http://localhost:8080/v1", "Inference endpoint URL")
	runCmd.Flags().StringVar(&flagModel, "model", "zerone-code-v1", "Model name to benchmark")
	runCmd.Flags().StringVar(&flagDomain, "domain", "", "Run only a specific domain (code, reasoning, instruction)")
	runCmd.Flags().StringVar(&flagDatasetDir, "dataset-dir", defaultDatasetDir, "Path to dataset directory")
	runCmd.Flags().StringVar(&flagOutputDir, "output-dir", "results", "Directory for result files")
	runCmd.Flags().StringVar(&flagDatasetVersion, "dataset-version", "v1", "Dataset version identifier")
	runCmd.Flags().DurationVar(&flagTimeout, "timeout", 30*time.Second, "Per-request timeout")
	runCmd.Flags().IntVar(&flagWorkers, "workers", 4, "Number of concurrent workers")

	compareCmd.Flags().StringVar(&flagBaseline, "baseline", "", "Path to baseline result file")
	compareCmd.Flags().StringVar(&flagCandidate, "candidate", "", "Path to candidate result file")
	_ = compareCmd.MarkFlagRequired("baseline")
	_ = compareCmd.MarkFlagRequired("candidate")

	listCmd.Flags().StringVar(&flagDatasetDir, "dataset-dir", defaultDatasetDir, "Path to dataset directory")

	rootCmd.AddCommand(runCmd, compareCmd, listCmd)
}

func runBenchmark(_ *cobra.Command, _ []string) error {
	// Load benchmark cases
	var cases []domains.BenchCase
	var err error

	if flagDomain != "" {
		cases, err = domains.LoadDomain(flagDatasetDir, flagDomain)
		if err != nil {
			return fmt.Errorf("load domain %s: %w", flagDomain, err)
		}
		fmt.Printf("Loaded %d cases for domain %q\n", len(cases), flagDomain)
	} else {
		cases, err = domains.LoadAll(flagDatasetDir)
		if err != nil {
			return fmt.Errorf("load datasets: %w", err)
		}
		fmt.Printf("Loaded %d cases across all domains\n", len(cases))
	}

	// Create runner and execute
	runner := NewRunner(flagEndpoint, flagModel, flagTimeout, flagWorkers)
	report, err := runner.Run(cases, flagDatasetVersion)
	if err != nil {
		return fmt.Errorf("run benchmark: %w", err)
	}

	// Print summary
	reporter := NewReporter()
	reporter.PrintSummary(report)

	// Save results
	if err := os.MkdirAll(flagOutputDir, 0755); err != nil {
		return fmt.Errorf("create output dir: %w", err)
	}

	outputFile := filepath.Join(flagOutputDir,
		fmt.Sprintf("benchmark-%s-%s.json", flagModel, time.Now().Format("20060102-150405")))
	if err := reporter.WriteJSON(report, outputFile); err != nil {
		return fmt.Errorf("write results: %w", err)
	}
	fmt.Printf("\nResults saved to %s\n", outputFile)

	return nil
}

func runCompare(_ *cobra.Command, _ []string) error {
	reporter := NewReporter()

	baseline, err := reporter.LoadReport(flagBaseline)
	if err != nil {
		return fmt.Errorf("load baseline: %w", err)
	}

	candidate, err := reporter.LoadReport(flagCandidate)
	if err != nil {
		return fmt.Errorf("load candidate: %w", err)
	}

	comparison := reporter.Compare(baseline, candidate)
	fmt.Print(comparison)

	return nil
}

func runList(_ *cobra.Command, _ []string) error {
	for _, domain := range domains.ListDomains() {
		cases, err := domains.LoadDomain(flagDatasetDir, domain)
		if err != nil {
			fmt.Printf("  %-15s (error: %v)\n", domain, err)
			continue
		}

		// Count categories
		cats := make(map[string]int)
		for _, c := range cases {
			cats[c.Category]++
		}

		fmt.Printf("  %-15s %d cases\n", domain, len(cases))
		for cat, count := range cats {
			fmt.Printf("    %-25s %d\n", cat, count)
		}
	}
	return nil
}
