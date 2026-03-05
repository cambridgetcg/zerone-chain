package cmd

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"

	"github.com/zerone-chain/zerone/services/blind-storage/node/internal/access"
	"github.com/zerone-chain/zerone/services/blind-storage/node/internal/discovery"
	"github.com/zerone-chain/zerone/services/blind-storage/node/internal/proof"
	nodestore "github.com/zerone-chain/zerone/services/blind-storage/node/internal/store"
)

var (
	flagAddr     string
	flagDataDir  string
	flagNodeAddr string
)

var rootCmd = &cobra.Command{
	Use:   "storage-node",
	Short: "ZERONE Storage Node — store and serve encrypted dataset chunks",
}

func init() {
	rootCmd.PersistentFlags().StringVar(&flagAddr, "addr", ":9400", "HTTP listen address")
	rootCmd.PersistentFlags().StringVar(&flagDataDir, "data-dir", "/data/chunks", "Chunk storage directory")
	rootCmd.PersistentFlags().StringVar(&flagNodeAddr, "node-addr", "", "This node's on-chain address")

	rootCmd.AddCommand(serveCmd, registerCmd)
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Start the storage node server",
	RunE: func(cmd *cobra.Command, args []string) error {
		store, err := nodestore.New(flagDataDir)
		if err != nil {
			return fmt.Errorf("init store: %w", err)
		}

		registry := discovery.NewRegistry()

		mux := http.NewServeMux()

		// Health endpoint
		mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte(`{"status":"ok"}`))
		})

		// List stored datasets
		mux.HandleFunc("/datasets", func(w http.ResponseWriter, r *http.Request) {
			datasets := store.ListDatasets()
			fmt.Fprintf(w, `{"datasets":%q}`, datasets)
		})

		log.Printf("storage node listening on %s (data: %s)", flagAddr, flagDataDir)

		// Keep imports used
		_ = registry
		_ = proof.Challenge{}
		_ = access.Ticket{}

		ctx, cancel := context.WithCancel(cmd.Context())
		defer cancel()

		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
		go func() {
			<-sigCh
			cancel()
		}()

		server := &http.Server{Addr: flagAddr, Handler: mux}
		go func() {
			<-ctx.Done()
			server.Close()
		}()

		return server.ListenAndServe()
	},
}

var registerCmd = &cobra.Command{
	Use:   "register",
	Short: "Register this node on-chain",
	RunE: func(cmd *cobra.Command, args []string) error {
		// TODO: Submit MsgRegisterStorageNode to chain
		fmt.Println("Registration requires chain connection — use zeroned tx to register")
		return nil
	},
}
