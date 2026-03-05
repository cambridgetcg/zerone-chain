package manifest

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"

	"github.com/zerone-chain/zerone/services/blind-storage/internal/chunk"
)

// Manifest describes a chunked dataset for distribution.
type Manifest struct {
	DatasetID                string       `json:"dataset_id"`
	TotalChunks              int          `json:"total_chunks"`
	DataChunks               int          `json:"data_chunks"`
	ParityChunks             int          `json:"parity_chunks"`
	ReconstructionThreshold  int          `json:"reconstruction_threshold"`
	ChunkSizeBytes           int          `json:"chunk_size_bytes"`
	TotalSizeBytes           int64        `json:"total_size_bytes"`
	MerkleRoot               string       `json:"merkle_root"`
	Chunks                   []ChunkInfo  `json:"chunks"`
}

// ChunkInfo describes a single chunk.
type ChunkInfo struct {
	Index         int      `json:"index"`
	MerkleHash    string   `json:"merkle_hash"`
	EncryptedSize int      `json:"encrypted_size"`
	AssignedNodes []string `json:"assigned_nodes,omitempty"`
}

// Generate creates a manifest from encode results.
func Generate(result *chunk.EncodeResult) *Manifest {
	merkleRoot := chunk.MerkleRoot(result.Shards)

	chunks := make([]ChunkInfo, len(result.Shards))
	var totalSize int64
	for i, shard := range result.Shards {
		hash := sha256.Sum256(shard)
		chunks[i] = ChunkInfo{
			Index:         i,
			MerkleHash:    hex.EncodeToString(hash[:]),
			EncryptedSize: len(shard),
		}
		totalSize += int64(len(shard))
	}

	return &Manifest{
		DatasetID:               result.DatasetID,
		TotalChunks:             result.TotalChunks,
		DataChunks:              result.DataChunks,
		ParityChunks:            result.ParityChunks,
		ReconstructionThreshold: result.DataChunks,
		ChunkSizeBytes:          result.ChunkSize,
		TotalSizeBytes:          totalSize,
		MerkleRoot:              hex.EncodeToString(merkleRoot[:]),
		Chunks:                  chunks,
	}
}

// Save writes the manifest to a JSON file.
func (m *Manifest) Save(path string) error {
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

// Load reads a manifest from a JSON file.
func Load(path string) (*Manifest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var m Manifest
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, err
	}
	return &m, nil
}

// Verify checks that chunk hashes match the manifest.
func (m *Manifest) Verify(shards [][]byte) error {
	for i, ci := range m.Chunks {
		if i >= len(shards) || shards[i] == nil {
			continue
		}
		hash := sha256.Sum256(shards[i])
		if hex.EncodeToString(hash[:]) != ci.MerkleHash {
			return fmt.Errorf("chunk %d hash mismatch", i)
		}
	}
	return nil
}
