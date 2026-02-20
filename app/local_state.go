package app

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
)

// LocalCommitment holds a validator's local commitment data (salt and verdict)
// that must be retained between the commit and reveal phases.
//
// Persisted to disk so node restarts between commit and reveal phases do not
// cause missed reveal slashes.
type LocalCommitment struct {
	RoundID    string `json:"round_id"`
	Verdict    string `json:"verdict"`    // "accept", "reject", "abstain"
	Confidence uint64 `json:"confidence"` // 0-1000000
	Salt       string `json:"salt"`       // hex-encoded random salt
	Height     uint64 `json:"height"`     // block height when committed
}

const commitmentFileName = "pot_commitments.json"

// LocalCommitmentStore manages validator-local commitment data.
// Thread-safe for concurrent access from ExtendVote calls.
// Write-through persistence: every mutation flushes to disk.
type LocalCommitmentStore struct {
	mu          sync.RWMutex
	commitments map[string]LocalCommitment // roundID → commitment
	filePath    string                     // path to persistence file (empty = in-memory only)
}

// NewLocalCommitmentStore creates a persistent commitment store.
// dataDir is the node's data directory (e.g., ~/.zeroned/data).
// Existing commitments are loaded from disk on creation.
func NewLocalCommitmentStore(dataDir string) *LocalCommitmentStore {
	s := &LocalCommitmentStore{
		commitments: make(map[string]LocalCommitment),
	}
	if dataDir != "" {
		s.filePath = filepath.Join(dataDir, commitmentFileName)
		s.loadFromDisk()
	}
	return s
}

// Store saves a local commitment for a round and persists to disk.
func (s *LocalCommitmentStore) Store(commitment LocalCommitment) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.commitments[commitment.RoundID] = commitment
	s.flushLocked()
}

// Get retrieves a local commitment for a round.
func (s *LocalCommitmentStore) Get(roundID string) (LocalCommitment, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	c, ok := s.commitments[roundID]
	return c, ok
}

// Delete removes a local commitment (after successful reveal or round completion).
func (s *LocalCommitmentStore) Delete(roundID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.commitments, roundID)
	s.flushLocked()
}

// CleanupExpired removes commitments for rounds older than the given height.
func (s *LocalCommitmentStore) CleanupExpired(currentHeight uint64, maxAge uint64) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if currentHeight < maxAge {
		return
	}
	cutoff := currentHeight - maxAge

	changed := false
	for id, c := range s.commitments {
		if c.Height < cutoff {
			delete(s.commitments, id)
			changed = true
		}
	}
	if changed {
		s.flushLocked()
	}
}

// Count returns the number of stored commitments.
func (s *LocalCommitmentStore) Count() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.commitments)
}

// flushLocked writes the current commitments map to disk.
// Must be called with s.mu held (write lock).
func (s *LocalCommitmentStore) flushLocked() {
	if s.filePath == "" {
		return
	}

	data, err := json.Marshal(s.commitments)
	if err != nil {
		return
	}

	// Atomic write: write to temp file then rename to prevent corruption
	// if the node crashes mid-write.
	tmpPath := s.filePath + ".tmp"
	if err := os.WriteFile(tmpPath, data, 0600); err != nil {
		return
	}
	_ = os.Rename(tmpPath, s.filePath)
}

// loadFromDisk loads commitments from the persistence file.
// Called once during construction.
func (s *LocalCommitmentStore) loadFromDisk() {
	if s.filePath == "" {
		return
	}

	data, err := os.ReadFile(s.filePath)
	if err != nil {
		return
	}

	var loaded map[string]LocalCommitment
	if err := json.Unmarshal(data, &loaded); err != nil {
		return
	}
	s.commitments = loaded
}
