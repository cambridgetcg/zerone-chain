package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var (
	flagEnclaveID      string
	flagBaseModel      string
	flagTargetDomain   string
	flagSnapshotHeight int64
	flagGRPCAddr       string
	flagDomainMix      float64
	flagLoRARank       int
	flagLoRAAlpha      float64
	flagLoRAEpochs     int
)

var rootCmd = &cobra.Command{
	Use:   "training-enclave",
	Short: "TEE training enclave runtime for secure model fine-tuning",
}

var trainCmd = &cobra.Command{
	Use:   "train",
	Short: "Execute full training lifecycle inside TEE enclave",
	Long: `Runs the complete training lifecycle:
  1. COLLECT  — authenticate with validators, collect encrypted shards
  2. PREPARE  — reassemble dataset, apply fitness filter and domain mix
  3. TRAIN    — run LoRA/QLoRA fine-tuning with per-TDU loss tracking
  4. OUTPUT   — export signed model weights, metrics, and benchmark results
  5. DESTROY  — zero all dataset memory, verify destruction, attest

Dataset NEVER touches disk — memory only. If the enclave crashes,
dataset is automatically destroyed (memory freed by OS).`,
	RunE: runTrain,
}

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show current enclave status and configuration",
	RunE:  runStatus,
}

var verifyCmd = &cobra.Command{
	Use:   "verify [attestation-file]",
	Short: "Verify a training attestation file",
	Args:  cobra.ExactArgs(1),
	RunE:  runVerify,
}

func init() {
	trainCmd.Flags().StringVar(&flagEnclaveID, "enclave-id", "", "Enclave identifier")
	trainCmd.Flags().StringVar(&flagBaseModel, "base-model", "llama-3-8b", "Base model for fine-tuning")
	trainCmd.Flags().StringVar(&flagTargetDomain, "domain", "", "Target domain for 70/30 mix")
	trainCmd.Flags().Int64Var(&flagSnapshotHeight, "snapshot-height", 0, "Snapshot block height")
	trainCmd.Flags().StringVar(&flagGRPCAddr, "grpc-addr", "localhost:9090", "Chain gRPC address")
	trainCmd.Flags().Float64Var(&flagDomainMix, "domain-mix", 0.7, "Domain mix ratio (0.0-1.0)")
	trainCmd.Flags().IntVar(&flagLoRARank, "lora-rank", 16, "LoRA rank")
	trainCmd.Flags().Float64Var(&flagLoRAAlpha, "lora-alpha", 32.0, "LoRA alpha")
	trainCmd.Flags().IntVar(&flagLoRAEpochs, "epochs", 3, "Number of training epochs")

	trainCmd.MarkFlagRequired("enclave-id")
	trainCmd.MarkFlagRequired("domain")
	trainCmd.MarkFlagRequired("snapshot-height")

	rootCmd.AddCommand(trainCmd, statusCmd, verifyCmd)
}

func runTrain(cmd *cobra.Command, args []string) error {
	fmt.Printf("training-enclave: starting training lifecycle\n")
	fmt.Printf("  enclave-id:      %s\n", flagEnclaveID)
	fmt.Printf("  base-model:      %s\n", flagBaseModel)
	fmt.Printf("  domain:          %s\n", flagTargetDomain)
	fmt.Printf("  domain-mix:      %.0f%% / %.0f%%\n", flagDomainMix*100, (1-flagDomainMix)*100)
	fmt.Printf("  snapshot-height: %d\n", flagSnapshotHeight)
	fmt.Printf("  grpc-addr:       %s\n", flagGRPCAddr)
	fmt.Printf("  lora-rank:       %d\n", flagLoRARank)
	fmt.Printf("  lora-alpha:      %.1f\n", flagLoRAAlpha)
	fmt.Printf("  epochs:          %d\n", flagLoRAEpochs)
	fmt.Println("  status: not yet connected to live chain (dry-run mode)")
	return nil
}

func runStatus(cmd *cobra.Command, args []string) error {
	fmt.Println("training-enclave: status")
	fmt.Println("  enclave: not initialized (dry-run mode)")
	fmt.Println("  phase: idle")
	fmt.Println("  dataset: not loaded")
	fmt.Println("  model: not trained")
	return nil
}

func runVerify(cmd *cobra.Command, args []string) error {
	file := args[0]
	data, err := os.ReadFile(file)
	if err != nil {
		return fmt.Errorf("failed to read attestation file: %w", err)
	}

	var att TrainingAttestation
	if err := json.Unmarshal(data, &att); err != nil {
		return fmt.Errorf("failed to parse attestation: %w", err)
	}

	fmt.Printf("training-enclave: attestation verification\n")
	fmt.Printf("  enclave-id:    %s\n", att.EnclaveID)
	fmt.Printf("  base-model:    %s\n", att.BaseModel)
	fmt.Printf("  dataset-size:  %d TDUs\n", att.DatasetSize)
	fmt.Printf("  benchmark:     %.4f\n", att.BenchmarkScore)
	fmt.Printf("  model-hash:    %x\n", att.ModelHash[:8])
	fmt.Printf("  start:         %s\n", att.StartTime.Format("2006-01-02T15:04:05Z"))
	fmt.Printf("  end:           %s\n", att.EndTime.Format("2006-01-02T15:04:05Z"))
	fmt.Printf("  has-signature: %v\n", len(att.Signature) > 0)
	fmt.Printf("  has-destruction-proof: %v\n", len(att.DestructionProof) > 0)

	if len(att.Signature) == 0 {
		fmt.Println("  WARNING: attestation is unsigned")
	}

	return nil
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
