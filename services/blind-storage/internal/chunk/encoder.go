package chunk

import (
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"io"
	"os"

	"github.com/klauspost/reedsolomon"

	"github.com/zerone-chain/zerone/services/blind-storage/internal/crypto"
)

// Header prepended to serialized datasets before chunking.
type Header struct {
	Version     [16]byte // dataset version (null-padded)
	SampleCount uint64
	Format      [8]byte // "jsonl\x00\x00\x00"
	Checksum    [32]byte // SHA-256 of the data payload
	DataSize    uint64   // size of data payload (excluding header)
}

const headerSize = 16 + 8 + 8 + 32 + 8 // 72 bytes

// EncodeResult holds the output of the chunking process.
type EncodeResult struct {
	DataChunks   int
	ParityChunks int
	TotalChunks  int
	ChunkSize    int
	Shards       [][]byte // All shards: data + parity
	MasterKey    []byte
	DatasetID    string
}

// Encode reads a dataset file, applies erasure coding, and encrypts each shard.
func Encode(datasetPath string, datasetID string, k, m int) (*EncodeResult, error) {
	// Read dataset
	data, err := os.ReadFile(datasetPath)
	if err != nil {
		return nil, fmt.Errorf("read dataset: %w", err)
	}

	// Build header
	checksum := sha256.Sum256(data)
	var header Header
	copy(header.Version[:], datasetID)
	copy(header.Format[:], "jsonl")
	header.Checksum = checksum
	header.DataSize = uint64(len(data))

	// Serialize header + data
	headerBytes := make([]byte, headerSize)
	copy(headerBytes[0:16], header.Version[:])
	binary.BigEndian.PutUint64(headerBytes[16:24], header.SampleCount)
	copy(headerBytes[24:32], header.Format[:])
	copy(headerBytes[32:64], header.Checksum[:])
	binary.BigEndian.PutUint64(headerBytes[64:72], header.DataSize)

	blob := append(headerBytes, data...)

	// Pad to align with k shards
	shardSize := (len(blob) + k - 1) / k
	padded := make([]byte, shardSize*k)
	copy(padded, blob)

	// Split into k data shards
	shards := make([][]byte, k+m)
	for i := 0; i < k; i++ {
		shards[i] = padded[i*shardSize : (i+1)*shardSize]
	}
	for i := k; i < k+m; i++ {
		shards[i] = make([]byte, shardSize)
	}

	// Create Reed-Solomon encoder
	enc, err := reedsolomon.New(k, m)
	if err != nil {
		return nil, fmt.Errorf("create encoder: %w", err)
	}

	// Compute parity shards
	if err := enc.Encode(shards); err != nil {
		return nil, fmt.Errorf("erasure encode: %w", err)
	}

	// Generate master key and encrypt each shard
	masterKey, err := crypto.GenerateMasterKey()
	if err != nil {
		return nil, err
	}

	encryptedShards := make([][]byte, len(shards))
	for i, shard := range shards {
		chunkKey, err := crypto.DeriveChunkKey(masterKey, i, datasetID)
		if err != nil {
			return nil, fmt.Errorf("derive key for chunk %d: %w", i, err)
		}
		encrypted, err := crypto.EncryptChunk(chunkKey, shard)
		if err != nil {
			return nil, fmt.Errorf("encrypt chunk %d: %w", i, err)
		}
		encryptedShards[i] = encrypted
	}

	return &EncodeResult{
		DataChunks:   k,
		ParityChunks: m,
		TotalChunks:  k + m,
		ChunkSize:    shardSize,
		Shards:       encryptedShards,
		MasterKey:    masterKey,
		DatasetID:    datasetID,
	}, nil
}

// Decode collects k shards, decrypts them, and reconstructs the original dataset.
func Decode(encryptedShards [][]byte, masterKey []byte, datasetID string, k, m int) ([]byte, error) {
	// Decrypt available shards
	decryptedShards := make([][]byte, k+m)
	available := 0

	for i, shard := range encryptedShards {
		if shard == nil {
			continue
		}
		chunkKey, err := crypto.DeriveChunkKey(masterKey, i, datasetID)
		if err != nil {
			return nil, fmt.Errorf("derive key for chunk %d: %w", i, err)
		}
		decrypted, err := crypto.DecryptChunk(chunkKey, shard)
		if err != nil {
			// Shard corrupted or wrong key — treat as missing
			continue
		}
		decryptedShards[i] = decrypted
		available++
	}

	if available < k {
		return nil, fmt.Errorf("insufficient shards: have %d, need %d", available, k)
	}

	// Reconstruct using Reed-Solomon
	enc, err := reedsolomon.New(k, m)
	if err != nil {
		return nil, fmt.Errorf("create decoder: %w", err)
	}

	if err := enc.Reconstruct(decryptedShards); err != nil {
		return nil, fmt.Errorf("reconstruct: %w", err)
	}

	// Verify
	ok, err := enc.Verify(decryptedShards)
	if err != nil {
		return nil, fmt.Errorf("verify: %w", err)
	}
	if !ok {
		return nil, fmt.Errorf("verification failed after reconstruction")
	}

	// Join data shards
	var buf []byte
	for i := 0; i < k; i++ {
		buf = append(buf, decryptedShards[i]...)
	}

	// Parse header
	if len(buf) < headerSize {
		return nil, fmt.Errorf("reconstructed data too small for header")
	}

	dataSize := binary.BigEndian.Uint64(buf[64:72])
	if int(dataSize) > len(buf)-headerSize {
		return nil, fmt.Errorf("data size %d exceeds buffer", dataSize)
	}

	payload := buf[headerSize : headerSize+int(dataSize)]

	// Verify checksum
	var expectedChecksum [32]byte
	copy(expectedChecksum[:], buf[32:64])
	actualChecksum := sha256.Sum256(payload)
	if expectedChecksum != actualChecksum {
		return nil, fmt.Errorf("checksum mismatch")
	}

	return payload, nil
}

// WriteShards writes encrypted shards to individual files in a directory.
func WriteShards(shards [][]byte, dir string) error {
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	for i, shard := range shards {
		path := fmt.Sprintf("%s/chunk_%04d.enc", dir, i)
		if err := os.WriteFile(path, shard, 0644); err != nil {
			return fmt.Errorf("write chunk %d: %w", i, err)
		}
	}
	return nil
}

// ReadShards reads encrypted shards from files. Missing files result in nil entries.
func ReadShards(dir string, totalChunks int) ([][]byte, error) {
	shards := make([][]byte, totalChunks)
	for i := 0; i < totalChunks; i++ {
		path := fmt.Sprintf("%s/chunk_%04d.enc", dir, i)
		data, err := os.ReadFile(path)
		if err != nil {
			continue // missing shard
		}
		shards[i] = data
	}
	return shards, nil
}

// MerkleRoot computes the merkle root over chunk hashes.
func MerkleRoot(shards [][]byte) [32]byte {
	hashes := make([][32]byte, len(shards))
	for i, s := range shards {
		hashes[i] = sha256.Sum256(s)
	}
	return merkleFromHashes(hashes)
}

func merkleFromHashes(hashes [][32]byte) [32]byte {
	if len(hashes) == 0 {
		return [32]byte{}
	}
	if len(hashes) == 1 {
		return hashes[0]
	}

	// Pad to even
	if len(hashes)%2 != 0 {
		hashes = append(hashes, hashes[len(hashes)-1])
	}

	var next [][32]byte
	for i := 0; i < len(hashes); i += 2 {
		combined := append(hashes[i][:], hashes[i+1][:]...)
		next = append(next, sha256.Sum256(combined))
	}
	return merkleFromHashes(next)
}

// Ensure imports are used
var _ = io.EOF
