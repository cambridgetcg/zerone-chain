package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var (
	flagEnclaveID      string
	flagSnapshotHeight int64
	flagTimeout        int
	flagMaxRetries     int
	flagGRPCAddr       string
)

var rootCmd = &cobra.Command{
	Use:   "shard-collector",
	Short: "Secure shard collection service for TEE enclaves",
}

var collectCmd = &cobra.Command{
	Use:   "collect",
	Short: "Collect shards from validators into TEE enclave",
	Long: `Authenticates with validators via attestation exchange, derives
per-session AES-256-GCM keys via ECDH, collects encrypted shard data,
verifies integrity against on-chain hashes, checks replication consistency,
and assembles the complete dataset inside the enclave.`,
	RunE: runCollect,
}

var verifyCmd = &cobra.Command{
	Use:   "verify",
	Short: "Verify integrity of previously collected shards",
	Long:  `Reads shard data from memory and re-verifies content hashes and replication.`,
	RunE:  runVerify,
}

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show current collection status and enclave info",
	RunE:  runStatus,
}

func init() {
	collectCmd.Flags().StringVar(&flagEnclaveID, "enclave-id", "", "Enclave identifier")
	collectCmd.Flags().Int64Var(&flagSnapshotHeight, "snapshot-height", 0, "Snapshot block height")
	collectCmd.Flags().IntVar(&flagTimeout, "timeout", 300, "Per-validator timeout in seconds")
	collectCmd.Flags().IntVar(&flagMaxRetries, "max-retries", 3, "Max retries per validator")
	collectCmd.Flags().StringVar(&flagGRPCAddr, "grpc-addr", "localhost:9090", "Chain gRPC address")

	collectCmd.MarkFlagRequired("enclave-id")
	collectCmd.MarkFlagRequired("snapshot-height")

	rootCmd.AddCommand(collectCmd, verifyCmd, statusCmd)
}

func runCollect(cmd *cobra.Command, args []string) error {
	fmt.Printf("shard-collector: collecting shards for enclave %s at height %d\n",
		flagEnclaveID, flagSnapshotHeight)
	fmt.Printf("  grpc-addr:   %s\n", flagGRPCAddr)
	fmt.Printf("  timeout:     %ds per validator\n", flagTimeout)
	fmt.Printf("  max-retries: %d\n", flagMaxRetries)
	fmt.Println("  status: not yet connected to live chain (dry-run mode)")
	return nil
}

func runVerify(cmd *cobra.Command, args []string) error {
	fmt.Println("shard-collector: verify mode — re-checking collected shard integrity")
	fmt.Println("  status: no shards in memory (dry-run mode)")
	return nil
}

func runStatus(cmd *cobra.Command, args []string) error {
	fmt.Println("shard-collector: status")
	fmt.Println("  enclave: not initialized (dry-run mode)")
	fmt.Println("  collected shards: 0")
	fmt.Println("  verified TDUs: 0")
	return nil
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
