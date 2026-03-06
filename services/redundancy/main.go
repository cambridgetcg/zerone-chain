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
	flagTDUFile     string
	flagOutputDir   string
	flagThreshold   float64
	flagBatchSize   int
	flagSignalsFile string
)

func main() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

var rootCmd = &cobra.Command{
	Use:   "zerone-redundancy",
	Short: "ZERONE Redundancy Detector — identify redundant TDUs and generate decay signals",
}

var scanCmd = &cobra.Command{
	Use:   "scan",
	Short: "Scan TDU corpus for semantic redundancy clusters",
	RunE:  runScan,
}

var emitCmd = &cobra.Command{
	Use:   "emit",
	Short: "Generate fitness signal file from redundancy scan results",
	RunE:  runEmit,
}

var reportCmd = &cobra.Command{
	Use:   "report",
	Short: "Print human-readable redundancy analysis summary",
	RunE:  runReport,
}

func init() {
	scanCmd.Flags().StringVar(&flagTDUFile, "tdu-file", "", "Path to TDU corpus JSONL (id, content, fitness_score, ...)")
	scanCmd.Flags().StringVar(&flagOutputDir, "output-dir", "results", "Output directory for scan results")
	scanCmd.Flags().Float64Var(&flagThreshold, "threshold", 0.85, "Cosine similarity threshold for clustering")
	scanCmd.Flags().IntVar(&flagBatchSize, "batch-size", 100, "Embedding batch size")
	_ = scanCmd.MarkFlagRequired("tdu-file")

	emitCmd.Flags().StringVar(&flagSignalsFile, "scan-file", "", "Path to redundancy scan JSON")
	emitCmd.Flags().StringVar(&flagOutputDir, "output-dir", "results", "Output directory for signals file")
	_ = emitCmd.MarkFlagRequired("scan-file")

	reportCmd.Flags().StringVar(&flagSignalsFile, "scan-file", "", "Path to redundancy scan JSON")
	_ = reportCmd.MarkFlagRequired("scan-file")

	rootCmd.AddCommand(scanCmd, emitCmd, reportCmd)
}

// ─── Scan Results ────────────────────────────────────────────────────────────

// ScanResult is the JSON output from the scan command.
type ScanResult struct {
	Timestamp      string              `json:"timestamp"`
	TDUCount       int                 `json:"tdu_count"`
	ClusterCount   int                 `json:"cluster_count"`
	RedundantCount int                 `json:"redundant_count"`
	Threshold      float64             `json:"threshold"`
	Clusters       []ClusterResult     `json:"clusters"`
	Signals        []FitnessSignalEntry `json:"signals"`
}

// ClusterResult describes a single redundancy cluster.
type ClusterResult struct {
	ID          int            `json:"id"`
	Domain      string         `json:"domain,omitempty"`
	Size        int            `json:"size"`
	CanonicalID string         `json:"canonical_id"`
	Members     []MemberResult `json:"members"`
}

// MemberResult describes a TDU within a cluster.
type MemberResult struct {
	ID                    string  `json:"id"`
	FitnessScore          float64 `json:"fitness_score"`
	IsCorrection          bool    `json:"is_correction,omitempty"`
	IsCanonical           bool    `json:"is_canonical"`
	SimilarityToCanonical float64 `json:"similarity_to_canonical,omitempty"`
}

// ─── Commands ────────────────────────────────────────────────────────────────

func runScan(_ *cobra.Command, _ []string) error {
	// 1. Embed
	embedder := NewEmbedder(flagBatchSize)
	records, err := embedder.LoadAndEmbed(flagTDUFile)
	if err != nil {
		return fmt.Errorf("load and embed: %w", err)
	}
	fmt.Printf("Loaded and embedded %d TDUs\n", len(records))

	// 2. Cluster
	clusterer := NewClusterer(flagThreshold)
	clusters := clusterer.ClusterTDUs(records)
	fmt.Printf("Found %d clusters\n", len(clusters))

	// 3. Rank
	ranker := NewRanker()
	ranked := ranker.RankClusters(clusters)

	// 4. Generate signals
	sigGen := NewSignalGenerator(flagThreshold)
	signals := sigGen.GenerateSignals(ranked)

	// Count redundant TDUs (multi-member clusters)
	redundantCount := 0
	for _, rc := range ranked {
		redundantCount += len(rc.Redundant)
	}

	// Build scan result
	result := ScanResult{
		Timestamp:      time.Now().UTC().Format(time.RFC3339),
		TDUCount:       len(records),
		ClusterCount:   len(clusters),
		RedundantCount: redundantCount,
		Threshold:      flagThreshold,
		Signals:        signals,
	}

	// Build cluster details
	for _, rc := range ranked {
		if len(rc.Cluster.Members) <= 1 {
			continue // skip singletons from report
		}
		cr := ClusterResult{
			ID:          rc.Cluster.ID,
			Domain:      rc.Cluster.Domain,
			Size:        len(rc.Cluster.Members),
			CanonicalID: rc.Canonical.ID,
		}
		for _, m := range rc.Cluster.Members {
			mr := MemberResult{
				ID:           m.ID,
				FitnessScore: m.FitnessScore,
				IsCorrection: m.IsCorrection,
				IsCanonical:  m.ID == rc.Canonical.ID,
			}
			if !mr.IsCanonical {
				mr.SimilarityToCanonical = Similarity(m, rc.Canonical)
			}
			cr.Members = append(cr.Members, mr)
		}
		result.Clusters = append(result.Clusters, cr)
	}

	// Write output
	if err := os.MkdirAll(flagOutputDir, 0755); err != nil {
		return fmt.Errorf("create output dir: %w", err)
	}

	outFile := filepath.Join(flagOutputDir,
		fmt.Sprintf("redundancy-scan-%s.json", time.Now().Format("20060102-150405")))
	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal: %w", err)
	}
	if err := os.WriteFile(outFile, data, 0644); err != nil {
		return fmt.Errorf("write: %w", err)
	}

	printScanSummary(&result)
	fmt.Printf("\nResults saved to %s\n", outFile)
	return nil
}

func runEmit(_ *cobra.Command, _ []string) error {
	data, err := os.ReadFile(flagSignalsFile)
	if err != nil {
		return fmt.Errorf("read scan file: %w", err)
	}

	var result ScanResult
	if err := json.Unmarshal(data, &result); err != nil {
		return fmt.Errorf("unmarshal scan: %w", err)
	}

	sigGen := NewSignalGenerator(result.Threshold)
	batch := sigGen.BuildBatch(result.Signals)

	if err := os.MkdirAll(flagOutputDir, 0755); err != nil {
		return fmt.Errorf("create output dir: %w", err)
	}

	outFile := filepath.Join(flagOutputDir, "redundancy-fitness-signals.json")
	out, err := json.MarshalIndent(batch, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal signals: %w", err)
	}
	if err := os.WriteFile(outFile, out, 0644); err != nil {
		return fmt.Errorf("write signals: %w", err)
	}

	fmt.Printf("Fitness signals written to %s (%d signals)\n", outFile, batch.Count)
	fmt.Printf("Submit on-chain with:\n")
	fmt.Printf("  zeroned tx knowledge update-fitness-batch --signals-file %s --from fitness-oracle\n", outFile)
	return nil
}

func runReport(_ *cobra.Command, _ []string) error {
	data, err := os.ReadFile(flagSignalsFile)
	if err != nil {
		return fmt.Errorf("read scan file: %w", err)
	}

	var result ScanResult
	if err := json.Unmarshal(data, &result); err != nil {
		return fmt.Errorf("unmarshal scan: %w", err)
	}

	printScanSummary(&result)
	return nil
}

func printScanSummary(result *ScanResult) {
	fmt.Println("===============================================================")
	fmt.Printf("  Redundancy Analysis Report\n")
	fmt.Printf("  Time: %s\n", result.Timestamp)
	fmt.Printf("  Threshold: %.2f cosine similarity\n", result.Threshold)
	fmt.Println("===============================================================")
	fmt.Println()

	fmt.Printf("  Total TDUs:      %d\n", result.TDUCount)
	fmt.Printf("  Clusters:        %d\n", result.ClusterCount)
	fmt.Printf("  Redundant TDUs:  %d\n", result.RedundantCount)
	fmt.Printf("  Signals emitted: %d\n", len(result.Signals))
	fmt.Println()

	if len(result.Clusters) > 0 {
		// Domain breakdown
		domainClusters := make(map[string]int)
		domainRedundant := make(map[string]int)
		for _, c := range result.Clusters {
			d := c.Domain
			if d == "" {
				d = "(unknown)"
			}
			domainClusters[d]++
			domainRedundant[d] += c.Size - 1 // non-canonical count
		}
		fmt.Println("  Clusters by Domain:")
		for d, cnt := range domainClusters {
			fmt.Printf("    %-15s clusters=%d  redundant=%d\n", d, cnt, domainRedundant[d])
		}
		fmt.Println()

		// Largest clusters
		fmt.Println("  Largest Clusters:")
		maxShow := 5
		if maxShow > len(result.Clusters) {
			maxShow = len(result.Clusters)
		}
		// Simple selection sort for top N by size
		sorted := make([]ClusterResult, len(result.Clusters))
		copy(sorted, result.Clusters)
		for i := 0; i < maxShow; i++ {
			for j := i + 1; j < len(sorted); j++ {
				if sorted[j].Size > sorted[i].Size {
					sorted[i], sorted[j] = sorted[j], sorted[i]
				}
			}
		}
		for i := 0; i < maxShow; i++ {
			c := sorted[i]
			fmt.Printf("    cluster-%d: %d TDUs, domain=%s, canonical=%s\n",
				c.ID, c.Size, c.Domain, truncateID(c.CanonicalID))
		}
		fmt.Println()

		// TDUs at risk of pruning (redundancy signal < 0.4)
		var atRisk int
		for _, s := range result.Signals {
			if s.Redundancy < 0.4 {
				atRisk++
			}
		}
		if atRisk > 0 {
			fmt.Printf("  TDUs at risk of pruning (redundancy < 0.4): %d\n", atRisk)
			fmt.Println()
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
