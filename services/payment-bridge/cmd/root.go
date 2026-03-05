package cmd

import (
	"context"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/spf13/cobra"

	"github.com/zerone-chain/zerone/services/payment-bridge/internal/deposit"
	"github.com/zerone-chain/zerone/services/payment-bridge/internal/ledger"
	"github.com/zerone-chain/zerone/services/payment-bridge/internal/settlement"
)

var (
	flagRedisAddr        string
	flagChainGRPC        string
	flagGRPCAddr         string
	flagSettleInterval   time.Duration
	flagDepositPoll      time.Duration
)

var rootCmd = &cobra.Command{
	Use:   "payment-bridge",
	Short: "ZERONE Payment Bridge — off-chain ledger with on-chain settlement",
}

func init() {
	rootCmd.PersistentFlags().StringVar(&flagRedisAddr, "redis", "localhost:6379", "Redis address")
	rootCmd.PersistentFlags().StringVar(&flagChainGRPC, "chain-grpc", "localhost:9090", "ZERONE chain gRPC")
	rootCmd.PersistentFlags().StringVar(&flagGRPCAddr, "grpc-addr", ":9300", "Bridge gRPC listen address")
	rootCmd.PersistentFlags().DurationVar(&flagSettleInterval, "settle-interval", 5*time.Minute, "Settlement interval")
	rootCmd.PersistentFlags().DurationVar(&flagDepositPoll, "deposit-poll", 5*time.Second, "Deposit poll interval")

	rootCmd.AddCommand(serveCmd)
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Start the payment bridge service",
	RunE:  runServe,
}

// noopChainSettler is a placeholder until chain integration is wired.
type noopChainSettler struct{}

func (n *noopChainSettler) SubmitSettlement(ctx context.Context, batch *settlement.SettlementBatch) error {
	log.Printf("noop settler: would settle %d uzrn for %s", batch.TotalCostUZRN, batch.UserAddr)
	return nil
}

func runServe(cmd *cobra.Command, _ []string) error {
	l := ledger.New(flagRedisAddr)
	defer l.Close()

	ctx, cancel := context.WithCancel(cmd.Context())
	defer cancel()

	if err := l.Ping(ctx); err != nil {
		return err
	}
	log.Println("connected to Redis")

	// Start deposit monitor
	dm := deposit.NewMonitor(flagChainGRPC, l, flagDepositPoll)
	go func() {
		if err := dm.Start(ctx); err != nil {
			log.Printf("deposit monitor error: %v", err)
		}
	}()

	// Start settler
	settler := settlement.NewSettler(l, &noopChainSettler{}, flagSettleInterval)
	go func() {
		if err := settler.Run(ctx); err != nil {
			log.Printf("settler error: %v", err)
		}
	}()

	// gRPC listener placeholder
	lis, err := net.Listen("tcp", flagGRPCAddr)
	if err != nil {
		return err
	}
	log.Printf("payment bridge gRPC listening on %s", flagGRPCAddr)
	_ = lis // TODO: register gRPC service

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh
	log.Println("shutting down")
	cancel()
	return lis.Close()
}
