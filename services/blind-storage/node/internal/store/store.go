package store

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

// ChunkStore manages encrypted chunks on local disk.
type ChunkStore struct {
	mu      sync.RWMutex
	baseDir string
	index   map[string][]int // dataset_id → chunk indices
}

// New creates a chunk store at the given directory.
func New(baseDir string) (*ChunkStore, error) {
	if err := os.MkdirAll(baseDir, 0755); err != nil {
		return nil, err
	}
	return &ChunkStore{
		baseDir: baseDir,
		index:   make(map[string][]int),
	}, nil
}

func (s *ChunkStore) chunkPath(datasetID string, chunkIndex int) string {
	return filepath.Join(s.baseDir, datasetID, fmt.Sprintf("chunk_%04d.enc", chunkIndex))
}

// Store persists an encrypted chunk.
func (s *ChunkStore) Store(datasetID string, chunkIndex int, data []byte) error {
	dir := filepath.Join(s.baseDir, datasetID)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	path := s.chunkPath(datasetID, chunkIndex)
	if err := os.WriteFile(path, data, 0644); err != nil {
		return err
	}

	s.mu.Lock()
	s.index[datasetID] = append(s.index[datasetID], chunkIndex)
	s.mu.Unlock()

	return nil
}

// Retrieve reads a stored chunk.
func (s *ChunkStore) Retrieve(datasetID string, chunkIndex int) ([]byte, error) {
	path := s.chunkPath(datasetID, chunkIndex)
	return os.ReadFile(path)
}

// Has checks if a chunk is stored.
func (s *ChunkStore) Has(datasetID string, chunkIndex int) bool {
	path := s.chunkPath(datasetID, chunkIndex)
	_, err := os.Stat(path)
	return err == nil
}

// Hash returns the SHA-256 hash of a stored chunk.
func (s *ChunkStore) Hash(datasetID string, chunkIndex int) (string, error) {
	data, err := s.Retrieve(datasetID, chunkIndex)
	if err != nil {
		return "", err
	}
	h := sha256.Sum256(data)
	return hex.EncodeToString(h[:]), nil
}

// ListDatasets returns all dataset IDs with stored chunks.
func (s *ChunkStore) ListDatasets() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var ids []string
	for id := range s.index {
		ids = append(ids, id)
	}
	return ids
}

// ChunksFor returns the chunk indices stored for a dataset.
func (s *ChunkStore) ChunksFor(datasetID string) []int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.index[datasetID]
}

// Delete removes a chunk from storage.
func (s *ChunkStore) Delete(datasetID string, chunkIndex int) error {
	path := s.chunkPath(datasetID, chunkIndex)
	return os.Remove(path)
}

// ByteRange reads a specific byte range from a stored chunk.
func (s *ChunkStore) ByteRange(datasetID string, chunkIndex int, offset, length int) ([]byte, error) {
	data, err := s.Retrieve(datasetID, chunkIndex)
	if err != nil {
		return nil, err
	}
	if offset >= len(data) {
		return nil, fmt.Errorf("offset %d beyond chunk size %d", offset, len(data))
	}
	end := offset + length
	if end > len(data) {
		end = len(data)
	}
	return data[offset:end], nil
}
