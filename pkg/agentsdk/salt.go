package agentsdk

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

// SaltStore manages salt and score storage for the commit-reveal protocol.
// Salts are stored as hex-encoded files at <baseDir>/review-salts/<roundID>.salt.
// Scores are stored as JSON at <baseDir>/review-salts/<roundID>.score.
// Thread-safe via mutex.
type SaltStore struct {
	mu      sync.RWMutex
	baseDir string
}

// NewSaltStore creates a salt store rooted at baseDir (typically ~/.zeroned).
func NewSaltStore(baseDir string) *SaltStore {
	return &SaltStore{baseDir: baseDir}
}

// saltDir returns the directory where salts are stored.
func (s *SaltStore) saltDir() string {
	return filepath.Join(s.baseDir, "review-salts")
}

// saltPath returns the file path for a round's salt.
func (s *SaltStore) saltPath(roundID string) string {
	return filepath.Join(s.saltDir(), roundID+".salt")
}

// scorePath returns the file path for a round's score.
func (s *SaltStore) scorePath(roundID string) string {
	return filepath.Join(s.saltDir(), roundID+".score")
}

// GenerateAndStore generates a random 32-byte salt, stores it, and returns it.
func (s *SaltStore) GenerateAndStore(roundID string) ([]byte, error) {
	salt := make([]byte, 32)
	if _, err := rand.Read(salt); err != nil {
		return nil, fmt.Errorf("failed to generate salt: %w", err)
	}
	if err := s.Store(roundID, salt); err != nil {
		return nil, err
	}
	return salt, nil
}

// Store saves a salt for a round.
func (s *SaltStore) Store(roundID string, salt []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	dir := s.saltDir()
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("failed to create salt directory: %w", err)
	}

	hexSalt := hex.EncodeToString(salt)
	if err := os.WriteFile(s.saltPath(roundID), []byte(hexSalt), 0600); err != nil {
		return fmt.Errorf("failed to write salt for round %s: %w", roundID, err)
	}
	return nil
}

// StoreScore saves the review score for a round (for AutoRevealAll).
func (s *SaltStore) StoreScore(roundID string, score ReviewScore) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	dir := s.saltDir()
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("failed to create salt directory: %w", err)
	}

	data, err := json.Marshal(score)
	if err != nil {
		return fmt.Errorf("failed to marshal score: %w", err)
	}
	if err := os.WriteFile(s.scorePath(roundID), data, 0600); err != nil {
		return fmt.Errorf("failed to write score for round %s: %w", roundID, err)
	}
	return nil
}

// Load retrieves the salt for a round.
func (s *SaltStore) Load(roundID string) ([]byte, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	data, err := os.ReadFile(s.saltPath(roundID))
	if err != nil {
		return nil, fmt.Errorf("failed to read salt for round %s: %w", roundID, err)
	}

	salt, err := hex.DecodeString(string(data))
	if err != nil {
		return nil, fmt.Errorf("failed to decode salt for round %s: %w", roundID, err)
	}
	return salt, nil
}

// LoadScore retrieves the stored score for a round.
func (s *SaltStore) LoadScore(roundID string) (*ReviewScore, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	data, err := os.ReadFile(s.scorePath(roundID))
	if err != nil {
		return nil, fmt.Errorf("failed to read score for round %s: %w", roundID, err)
	}

	var score ReviewScore
	if err := json.Unmarshal(data, &score); err != nil {
		return nil, fmt.Errorf("failed to unmarshal score for round %s: %w", roundID, err)
	}
	return &score, nil
}

// Delete removes the salt and score files for a round (after successful reveal).
func (s *SaltStore) Delete(roundID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	saltPath := s.saltPath(roundID)
	if err := os.Remove(saltPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to delete salt for round %s: %w", roundID, err)
	}

	scorePath := s.scorePath(roundID)
	if err := os.Remove(scorePath); err != nil && !os.IsNotExist(err) {
		// Non-fatal: score file may not exist
	}
	return nil
}

// ListPending returns round IDs with stored salts (pending reveal).
func (s *SaltStore) ListPending() ([]string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	entries, err := os.ReadDir(s.saltDir())
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to list salt directory: %w", err)
	}

	var rounds []string
	for _, e := range entries {
		name := e.Name()
		if filepath.Ext(name) == ".salt" {
			rounds = append(rounds, name[:len(name)-5]) // strip .salt
		}
	}
	return rounds, nil
}
