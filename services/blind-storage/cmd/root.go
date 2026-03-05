package cmd

import (
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/zerone-chain/zerone/services/blind-storage/internal/chunk"
	"github.com/zerone-chain/zerone/services/blind-storage/internal/manifest"
)

var rootCmd = &cobra.Command{
	Use:   "chunk",
	Short: "ZERONE Chunk Engine — erasure coding and encryption for datasets",
}

func init() {
	rootCmd.AddCommand(encodeCmd, decodeCmd, verifyCmd)
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

var encodeCmd = &cobra.Command{
	Use:   "encode",
	Short: "Encode a dataset with erasure coding and encryption",
	RunE: func(cmd *cobra.Command, args []string) error {
		dataset, _ := cmd.Flags().GetString("dataset")
		k, _ := cmd.Flags().GetInt("k")
		m, _ := cmd.Flags().GetInt("m")
		output, _ := cmd.Flags().GetString("output")

		if dataset == "" || output == "" {
			return fmt.Errorf("--dataset and --output required")
		}

		datasetID := filepath.Base(filepath.Dir(dataset))
		if id, _ := cmd.Flags().GetString("id"); id != "" {
			datasetID = id
		}

		fmt.Printf("Encoding %s (K=%d, M=%d)...\n", dataset, k, m)
		result, err := chunk.Encode(dataset, datasetID, k, m)
		if err != nil {
			return err
		}

		// Write shards
		if err := chunk.WriteShards(result.Shards, output); err != nil {
			return err
		}

		// Generate and save manifest
		man := manifest.Generate(result)
		manifestPath := filepath.Join(output, "manifest.json")
		if err := man.Save(manifestPath); err != nil {
			return err
		}

		// Save master key (in production: use Shamir's Secret Sharing)
		keyPath := filepath.Join(output, "master.key")
		if err := os.WriteFile(keyPath, []byte(hex.EncodeToString(result.MasterKey)), 0600); err != nil {
			return err
		}

		fmt.Printf("Encoded: %d chunks (%d data + %d parity)\n",
			result.TotalChunks, result.DataChunks, result.ParityChunks)
		fmt.Printf("Chunk size: %d bytes\n", result.ChunkSize)
		fmt.Printf("Merkle root: %s\n", man.MerkleRoot)
		fmt.Printf("Master key saved to: %s\n", keyPath)
		fmt.Printf("Manifest saved to: %s\n", manifestPath)
		return nil
	},
}

var decodeCmd = &cobra.Command{
	Use:   "decode",
	Short: "Decode chunks back to original dataset",
	RunE: func(cmd *cobra.Command, args []string) error {
		chunksDir, _ := cmd.Flags().GetString("chunks-dir")
		output, _ := cmd.Flags().GetString("output")
		keyFile, _ := cmd.Flags().GetString("key")

		if chunksDir == "" || output == "" || keyFile == "" {
			return fmt.Errorf("--chunks-dir, --output, and --key required")
		}

		// Load manifest
		man, err := manifest.Load(filepath.Join(chunksDir, "manifest.json"))
		if err != nil {
			return fmt.Errorf("load manifest: %w", err)
		}

		// Load master key
		keyHex, err := os.ReadFile(keyFile)
		if err != nil {
			return fmt.Errorf("read key: %w", err)
		}
		masterKey, err := hex.DecodeString(string(keyHex))
		if err != nil {
			return fmt.Errorf("decode key: %w", err)
		}

		// Read shards
		shards, err := chunk.ReadShards(chunksDir, man.TotalChunks)
		if err != nil {
			return err
		}

		// Count available
		available := 0
		for _, s := range shards {
			if s != nil {
				available++
			}
		}
		fmt.Printf("Found %d/%d chunks (need %d)\n", available, man.TotalChunks, man.DataChunks)

		// Decode
		data, err := chunk.Decode(shards, masterKey, man.DatasetID, man.DataChunks, man.ParityChunks)
		if err != nil {
			return err
		}

		if err := os.WriteFile(output, data, 0644); err != nil {
			return err
		}

		fmt.Printf("Decoded %d bytes to %s\n", len(data), output)
		return nil
	},
}

var verifyCmd = &cobra.Command{
	Use:   "verify",
	Short: "Verify chunks against manifest",
	RunE: func(cmd *cobra.Command, args []string) error {
		manifestPath, _ := cmd.Flags().GetString("manifest")
		if manifestPath == "" {
			return fmt.Errorf("--manifest required")
		}

		man, err := manifest.Load(manifestPath)
		if err != nil {
			return err
		}

		dir := filepath.Dir(manifestPath)
		shards, err := chunk.ReadShards(dir, man.TotalChunks)
		if err != nil {
			return err
		}

		if err := man.Verify(shards); err != nil {
			return fmt.Errorf("verification failed: %w", err)
		}

		available := 0
		for _, s := range shards {
			if s != nil {
				available++
			}
		}

		fmt.Printf("Verification passed: %d/%d chunks valid\n", available, man.TotalChunks)
		return nil
	},
}

func init() {
	encodeCmd.Flags().String("dataset", "", "Path to dataset file (JSONL)")
	encodeCmd.Flags().String("id", "", "Dataset ID (default: parent dir name)")
	encodeCmd.Flags().Int("k", 64, "Number of data chunks")
	encodeCmd.Flags().Int("m", 32, "Number of parity chunks")
	encodeCmd.Flags().String("output", "", "Output directory for chunks")

	decodeCmd.Flags().String("chunks-dir", "", "Directory containing chunks")
	decodeCmd.Flags().String("output", "", "Output file path")
	decodeCmd.Flags().String("key", "", "Path to master key file")

	verifyCmd.Flags().String("manifest", "", "Path to manifest.json")
}
