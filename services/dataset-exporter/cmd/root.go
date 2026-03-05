package cmd

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/spf13/cobra"

	"github.com/zerone-chain/zerone/services/dataset-exporter/internal/chain"
	"github.com/zerone-chain/zerone/services/dataset-exporter/internal/db"
	"github.com/zerone-chain/zerone/services/dataset-exporter/internal/exporter"
	"github.com/zerone-chain/zerone/services/dataset-exporter/internal/metrics"
	"github.com/zerone-chain/zerone/services/dataset-exporter/internal/snapshot"
)

var (
	flagDSN          string
	flagChainGRPC    string
	flagPollInterval time.Duration
	flagMetricsAddr  string
	flagMigrationSQL string
)

// rootCmd is the base command.
var rootCmd = &cobra.Command{
	Use:   "dataset-exporter",
	Short: "ZERONE Dataset Exporter — sync chain samples to staging DB",
}

func init() {
	rootCmd.PersistentFlags().StringVar(&flagDSN, "dsn", "", "PostgreSQL DSN (default: DATABASE_URL env)")
	rootCmd.PersistentFlags().StringVar(&flagChainGRPC, "chain-grpc", "localhost:9090", "ZERONE chain gRPC endpoint")
	rootCmd.PersistentFlags().DurationVar(&flagPollInterval, "poll-interval", 10*time.Second, "Sync poll interval")
	rootCmd.PersistentFlags().StringVar(&flagMetricsAddr, "metrics-addr", ":9200", "Prometheus metrics listen address")
	rootCmd.PersistentFlags().StringVar(&flagMigrationSQL, "migration", "migrations/001_init.sql", "Path to migration SQL")

	rootCmd.AddCommand(syncCmd)
	rootCmd.AddCommand(snapshotCmd)
}

// Execute runs the root command.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

// ── sync command ─────────────────────────────────────────────────────────────

var syncCmd = &cobra.Command{
	Use:   "sync",
	Short: "Sync approved samples from chain to staging database",
	RunE:  runSync,
}

func runSync(cmd *cobra.Command, _ []string) error {
	database, err := db.New(flagDSN)
	if err != nil {
		return fmt.Errorf("connect db: %w", err)
	}
	defer database.Close()

	if err := database.Migrate(flagMigrationSQL); err != nil {
		return fmt.Errorf("migrate: %w", err)
	}

	chainClient, err := chain.New(flagChainGRPC)
	if err != nil {
		return fmt.Errorf("connect chain: %w", err)
	}
	defer chainClient.Close()

	// Start metrics server
	go func() {
		mux := http.NewServeMux()
		mux.Handle("/metrics", metrics.Handler())
		mux.HandleFunc("/health", metrics.HealthHandler())
		log.Printf("metrics server listening on %s", flagMetricsAddr)
		if err := http.ListenAndServe(flagMetricsAddr, mux); err != nil {
			log.Printf("metrics server error: %v", err)
		}
	}()

	ctx, cancel := context.WithCancel(cmd.Context())
	defer cancel()

	// Graceful shutdown
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		log.Println("received shutdown signal")
		cancel()
	}()

	exp := exporter.New(chainClient, database, flagPollInterval)
	return exp.Run(ctx)
}

// ── snapshot commands ────────────────────────────────────────────────────────

var snapshotCmd = &cobra.Command{
	Use:   "snapshot",
	Short: "Manage dataset snapshots",
}

var (
	flagDomain   string
	flagQuality  string
	flagType     string
	flagLanguage string
	flagFormat   string
)

func init() {
	createCmd := &cobra.Command{
		Use:   "create <version>",
		Short: "Create a new dataset snapshot",
		Args:  cobra.ExactArgs(1),
		RunE:  runSnapshotCreate,
	}
	createCmd.Flags().StringVar(&flagDomain, "domain", "", "Filter by domain")
	createCmd.Flags().StringVar(&flagQuality, "min-quality", "", "Filter by quality tier (gold, silver, bronze)")
	createCmd.Flags().StringVar(&flagType, "type", "", "Filter by sample type")
	createCmd.Flags().StringVar(&flagLanguage, "language", "", "Filter by language")

	listCmd := &cobra.Command{
		Use:   "list",
		Short: "List all dataset snapshots",
		RunE:  runSnapshotList,
	}

	exportCmd := &cobra.Command{
		Use:   "export <version>",
		Short: "Export a snapshot to JSONL",
		Args:  cobra.ExactArgs(1),
		RunE:  runSnapshotExport,
	}
	exportCmd.Flags().StringVar(&flagFormat, "format", "jsonl", "Export format (jsonl)")

	snapshotCmd.AddCommand(createCmd, listCmd, exportCmd)
}

func runSnapshotCreate(_ *cobra.Command, args []string) error {
	database, err := db.New(flagDSN)
	if err != nil {
		return err
	}
	defer database.Close()

	mgr := snapshot.NewManager(database.Pool())
	snap, err := mgr.Create(args[0], flagDomain, flagQuality, flagType, flagLanguage)
	if err != nil {
		return err
	}

	fmt.Printf("Created snapshot %s with %d samples (status: %s)\n",
		snap.Version, snap.SampleCount, snap.Status)
	return nil
}

func runSnapshotList(_ *cobra.Command, _ []string) error {
	database, err := db.New(flagDSN)
	if err != nil {
		return err
	}
	defer database.Close()

	mgr := snapshot.NewManager(database.Pool())
	snaps, err := mgr.List()
	if err != nil {
		return err
	}

	if len(snaps) == 0 {
		fmt.Println("No snapshots found.")
		return nil
	}

	fmt.Printf("%-20s %-10s %-8s %-12s %-12s %-12s %s\n",
		"VERSION", "SAMPLES", "STATUS", "DOMAIN", "QUALITY", "TYPE", "CREATED")
	for _, s := range snaps {
		domain := s.DomainFilter
		if domain == "" {
			domain = "*"
		}
		quality := s.QualityFilter
		if quality == "" {
			quality = "*"
		}
		typ := s.TypeFilter
		if typ == "" {
			typ = "*"
		}
		fmt.Printf("%-20s %-10d %-8s %-12s %-12s %-12s %s\n",
			s.Version, s.SampleCount, s.Status, domain, quality, typ,
			s.CreatedAt.Format(time.RFC3339))
	}
	return nil
}

func runSnapshotExport(_ *cobra.Command, args []string) error {
	database, err := db.New(flagDSN)
	if err != nil {
		return err
	}
	defer database.Close()

	mgr := snapshot.NewManager(database.Pool())
	count, err := mgr.Export(args[0], os.Stdout)
	if err != nil {
		return err
	}

	fmt.Fprintf(os.Stderr, "Exported %d samples from snapshot %s\n", count, args[0])
	return nil
}
