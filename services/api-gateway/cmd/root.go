package cmd

import (
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/spf13/cobra"

	"github.com/zerone-chain/zerone/services/api-gateway/internal/auth"
	"github.com/zerone-chain/zerone/services/api-gateway/internal/handler"
	"github.com/zerone-chain/zerone/services/api-gateway/internal/inference"
	"github.com/zerone-chain/zerone/services/api-gateway/internal/ratelimit"
)

var (
	flagAddr         string
	flagMetricsAddr  string
	flagInferenceURL string
	flagRateLimit    float64
	flagBurst        int64
)

var rootCmd = &cobra.Command{
	Use:   "api-gateway",
	Short: "ZERONE API Gateway — OpenAI-compatible inference API",
}

func init() {
	rootCmd.PersistentFlags().StringVar(&flagAddr, "addr", ":8080", "Gateway listen address")
	rootCmd.PersistentFlags().StringVar(&flagMetricsAddr, "metrics-addr", ":9201", "Metrics listen address")
	rootCmd.PersistentFlags().StringVar(&flagInferenceURL, "inference-url", "http://localhost:8000", "Inference server URL")
	rootCmd.PersistentFlags().Float64Var(&flagRateLimit, "rate-limit", 10, "Requests per second per key")
	rootCmd.PersistentFlags().Int64Var(&flagBurst, "burst", 20, "Max burst requests per key")

	rootCmd.AddCommand(serveCmd)
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Start the API gateway",
	RunE:  runServe,
}

func runServe(_ *cobra.Command, _ []string) error {
	store := auth.NewStore()
	pool := inference.NewPool([]string{flagInferenceURL})
	rl := ratelimit.New(flagRateLimit, flagBurst)

	gw := handler.New(store, pool, rl)

	// Metrics
	go func() {
		mux := http.NewServeMux()
		mux.Handle("/metrics", promhttp.Handler())
		log.Printf("metrics on %s", flagMetricsAddr)
		http.ListenAndServe(flagMetricsAddr, mux)
	}()

	log.Printf("api-gateway listening on %s", flagAddr)

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		log.Println("shutting down")
		os.Exit(0)
	}()

	return http.ListenAndServe(flagAddr, gw)
}
