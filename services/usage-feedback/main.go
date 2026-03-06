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
	flagEventLog    string
	flagTDUIndex    string
	flagOutputDir   string
	flagTopK        int
	flagMaxDist     int
	flagMinSignals  int
	flagSignalsFile string
)

func main() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

var rootCmd = &cobra.Command{
	Use:   "zerone-usage-feedback",
	Short: "ZERONE Usage Feedback — trace API usage patterns back to training data",
}

var collectCmd = &cobra.Command{
	Use:   "collect",
	Short: "Collect and classify usage events from API gateway logs",
	RunE:  runCollect,
}

var pipelineCmd = &cobra.Command{
	Use:   "pipeline",
	Short: "Run full pipeline: collect → attribute → aggregate → emit",
	RunE:  runPipeline,
}

var reportCmd = &cobra.Command{
	Use:   "report",
	Short: "Print human-readable summary of emitted fitness signals",
	RunE:  runReport,
}

func init() {
	collectCmd.Flags().StringVar(&flagEventLog, "event-log", "", "Path to API gateway event log JSONL")
	collectCmd.Flags().StringVar(&flagOutputDir, "output-dir", "results", "Output directory")
	_ = collectCmd.MarkFlagRequired("event-log")

	pipelineCmd.Flags().StringVar(&flagEventLog, "event-log", "", "Path to API gateway event log JSONL")
	pipelineCmd.Flags().StringVar(&flagTDUIndex, "tdu-index", "", "Path to TDU index JSONL (id + content)")
	pipelineCmd.Flags().StringVar(&flagOutputDir, "output-dir", "results", "Output directory")
	pipelineCmd.Flags().IntVar(&flagTopK, "top-k", 5, "Number of most-similar TDUs per query")
	pipelineCmd.Flags().IntVar(&flagMaxDist, "max-dist", 20, "Maximum hamming distance for attribution")
	pipelineCmd.Flags().IntVar(&flagMinSignals, "min-signals", 3, "Minimum signals per TDU to emit")
	_ = pipelineCmd.MarkFlagRequired("event-log")
	_ = pipelineCmd.MarkFlagRequired("tdu-index")

	reportCmd.Flags().StringVar(&flagSignalsFile, "signals-file", "", "Path to fitness signals JSON")
	_ = reportCmd.MarkFlagRequired("signals-file")

	rootCmd.AddCommand(collectCmd, pipelineCmd, reportCmd)
}

// runCollect parses events and outputs classified signals.
func runCollect(_ *cobra.Command, _ []string) error {
	collector := NewCollector()
	events, err := collector.ParseEventLog(flagEventLog)
	if err != nil {
		return fmt.Errorf("parse event log: %w", err)
	}
	fmt.Printf("Parsed %d usage events\n", len(events))

	// Classify and output
	type classifiedEvent struct {
		UsageEvent
		RawSignal float64 `json:"raw_signal"`
	}

	classified := make([]classifiedEvent, 0, len(events))
	for i := range events {
		sig := collector.EventToSignal(&events[i])
		if sig == 0.0 {
			continue // skip neutral
		}
		classified = append(classified, classifiedEvent{
			UsageEvent: events[i],
			RawSignal:  sig,
		})
	}

	fmt.Printf("Classified %d events with non-zero signals\n", len(classified))

	if err := os.MkdirAll(flagOutputDir, 0755); err != nil {
		return fmt.Errorf("create output dir: %w", err)
	}

	outFile := filepath.Join(flagOutputDir,
		fmt.Sprintf("classified-events-%s.json", time.Now().Format("20060102-150405")))
	data, err := json.MarshalIndent(classified, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal: %w", err)
	}
	if err := os.WriteFile(outFile, data, 0644); err != nil {
		return fmt.Errorf("write: %w", err)
	}

	fmt.Printf("Classified events written to %s\n", outFile)
	return nil
}

// runPipeline executes the full usage feedback pipeline.
func runPipeline(_ *cobra.Command, _ []string) error {
	// Step 1: Collect
	collector := NewCollector()
	events, err := collector.ParseEventLog(flagEventLog)
	if err != nil {
		return fmt.Errorf("parse event log: %w", err)
	}
	fmt.Printf("Parsed %d usage events\n", len(events))

	// Step 2: Attribute
	attributor := NewAttributor(flagTopK, flagMaxDist)
	if err := attributor.LoadTDUIndex(flagTDUIndex); err != nil {
		return fmt.Errorf("load TDU index: %w", err)
	}
	fmt.Printf("Loaded %d TDU records for attribution\n", len(attributor.tdus))

	var allAttributed []AttributedSignal
	for i := range events {
		rawSignal := collector.EventToSignal(&events[i])
		if rawSignal == 0.0 {
			continue
		}
		attributed := attributor.Attribute(&events[i], rawSignal)
		allAttributed = append(allAttributed, attributed...)
	}
	fmt.Printf("Generated %d attributed signals\n", len(allAttributed))

	// Step 3: Aggregate
	aggConfig := DefaultAggregationConfig()
	aggConfig.MinSignalCount = flagMinSignals
	aggregator := NewAggregator(aggConfig)
	aggregated := aggregator.Aggregate(allAttributed)
	fmt.Printf("Aggregated to %d TDU signals (min %d signals threshold)\n",
		len(aggregated), flagMinSignals)

	// Step 4: Emit
	emitter := NewEmitter()
	fitnessSignals := emitter.ConvertToFitnessSignals(aggregated)
	batch := emitter.BuildBatch(fitnessSignals)

	if err := os.MkdirAll(flagOutputDir, 0755); err != nil {
		return fmt.Errorf("create output dir: %w", err)
	}

	outFile := filepath.Join(flagOutputDir, "usage-fitness-signals.json")
	data, err := json.MarshalIndent(batch, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal: %w", err)
	}
	if err := os.WriteFile(outFile, data, 0644); err != nil {
		return fmt.Errorf("write: %w", err)
	}

	printPipelineSummary(batch, aggregated)
	fmt.Printf("\nFitness signals written to %s\n", outFile)
	fmt.Printf("Submit on-chain with:\n")
	fmt.Printf("  zeroned tx knowledge update-fitness-batch --signals-file %s --from fitness-oracle\n", outFile)
	return nil
}

func runReport(_ *cobra.Command, _ []string) error {
	data, err := os.ReadFile(flagSignalsFile)
	if err != nil {
		return fmt.Errorf("read signals file: %w", err)
	}

	var batch FitnessSignalBatch
	if err := json.Unmarshal(data, &batch); err != nil {
		return fmt.Errorf("unmarshal: %w", err)
	}

	printPipelineSummary(&batch, nil)
	return nil
}

func printPipelineSummary(batch *FitnessSignalBatch, aggregated []AggregatedSignal) {
	fmt.Println("===============================================================")
	fmt.Printf("  Usage Feedback Fitness Signals (%s)\n", batch.Method)
	fmt.Printf("  Time: %s\n", batch.Timestamp)
	fmt.Printf("  TDU Count: %d\n", batch.Count)
	fmt.Println("===============================================================")
	fmt.Println()

	if len(batch.Signals) > 0 {
		// Compute stats on usage correlation values
		var sum float64
		var minVal, maxVal float64
		minVal = 1.0
		for _, s := range batch.Signals {
			sum += s.UsageCorrelation
			if s.UsageCorrelation < minVal {
				minVal = s.UsageCorrelation
			}
			if s.UsageCorrelation > maxVal {
				maxVal = s.UsageCorrelation
			}
		}
		mean := sum / float64(len(batch.Signals))

		fmt.Println("  Usage Correlation Stats:")
		fmt.Printf("    Mean:  %.4f\n", mean)
		fmt.Printf("    Min:   %.4f\n", minVal)
		fmt.Printf("    Max:   %.4f\n", maxVal)
		fmt.Println()

		// Count positive/negative/neutral
		var pos, neg, neutral int
		for _, s := range batch.Signals {
			switch {
			case s.UsageCorrelation > 0.55:
				pos++
			case s.UsageCorrelation < 0.45:
				neg++
			default:
				neutral++
			}
		}
		fmt.Printf("    Positive (>0.55): %d\n", pos)
		fmt.Printf("    Negative (<0.45): %d\n", neg)
		fmt.Printf("    Neutral:          %d\n", neutral)
		fmt.Println()
	}

	if aggregated != nil && len(aggregated) > 0 {
		fmt.Println("  Aggregation Detail:")
		maxShow := 5
		if maxShow > len(aggregated) {
			maxShow = len(aggregated)
		}
		for i := 0; i < maxShow; i++ {
			a := aggregated[i]
			fmt.Printf("    %s  mean=%+.4f  signals=%d  domain=%s\n",
				truncateID(a.TDUID), a.MeanSignal, a.SignalCount, a.Domain)
		}
		if len(aggregated) > maxShow {
			fmt.Printf("    ... and %d more\n", len(aggregated)-maxShow)
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
