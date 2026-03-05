package auth

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strings"
	"sync"
	"time"

	"golang.org/x/crypto/bcrypt"
)

// APIKey represents a stored API key.
type APIKey struct {
	ID          string
	KeyHash     string // bcrypt hash
	WalletAddr  string
	CreatedAt   time.Time
	LastUsed    time.Time
	Active      bool
}

// Store manages API keys. In production, backed by PostgreSQL.
type Store struct {
	mu   sync.RWMutex
	keys map[string]*APIKey // key prefix → APIKey
}

// NewStore creates an in-memory key store.
func NewStore() *Store {
	return &Store{keys: make(map[string]*APIKey)}
}

// CreateKey generates a new API key for a wallet address.
// Returns the full key (only shown once) and the stored record.
func (s *Store) CreateKey(walletAddr string) (string, *APIKey, error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", nil, fmt.Errorf("generate key: %w", err)
	}

	fullKey := "zrn_" + hex.EncodeToString(raw)
	prefix := fullKey[:12] // store prefix for lookup

	hash, err := bcrypt.GenerateFromPassword([]byte(fullKey), bcrypt.DefaultCost)
	if err != nil {
		return "", nil, fmt.Errorf("hash key: %w", err)
	}

	key := &APIKey{
		ID:         prefix,
		KeyHash:    string(hash),
		WalletAddr: walletAddr,
		CreatedAt:  time.Now(),
		Active:     true,
	}

	s.mu.Lock()
	s.keys[prefix] = key
	s.mu.Unlock()

	return fullKey, key, nil
}

// Validate checks an API key and returns the associated wallet address.
func (s *Store) Validate(rawKey string) (string, error) {
	if !strings.HasPrefix(rawKey, "zrn_") {
		return "", fmt.Errorf("invalid key format")
	}
	prefix := rawKey[:12]

	s.mu.RLock()
	key, ok := s.keys[prefix]
	s.mu.RUnlock()

	if !ok {
		return "", fmt.Errorf("key not found")
	}
	if !key.Active {
		return "", fmt.Errorf("key revoked")
	}

	if err := bcrypt.CompareHashAndPassword([]byte(key.KeyHash), []byte(rawKey)); err != nil {
		return "", fmt.Errorf("invalid key")
	}

	// Update last used
	s.mu.Lock()
	key.LastUsed = time.Now()
	s.mu.Unlock()

	return key.WalletAddr, nil
}

// Revoke deactivates an API key.
func (s *Store) Revoke(keyID string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if key, ok := s.keys[keyID]; ok {
		key.Active = false
		return true
	}
	return false
}

// ListKeys returns all keys for a wallet address.
func (s *Store) ListKeys(walletAddr string) []*APIKey {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []*APIKey
	for _, k := range s.keys {
		if k.WalletAddr == walletAddr {
			result = append(result, k)
		}
	}
	return result
}

// ExtractBearerToken extracts the token from "Bearer <token>" header value.
func ExtractBearerToken(authHeader string) string {
	if !strings.HasPrefix(authHeader, "Bearer ") {
		return ""
	}
	return strings.TrimPrefix(authHeader, "Bearer ")
}
