package tee

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"
)

// ProviderType identifies a TEE provider.
type ProviderType string

const (
	ProviderNitro ProviderType = "nitro"
	ProviderSGX   ProviderType = "sgx"
	ProviderSEV   ProviderType = "sev"
	ProviderMock  ProviderType = "mock"
)

// EnclaveStatus represents the lifecycle state of a registered enclave.
type EnclaveStatus int

const (
	EnclaveStatusActive    EnclaveStatus = iota // Enclave is active and verified
	EnclaveStatusSuspended                      // Temporarily suspended (attestation expired)
	EnclaveStatusRevoked                        // Permanently revoked
)

func (s EnclaveStatus) String() string {
	switch s {
	case EnclaveStatusActive:
		return "active"
	case EnclaveStatusSuspended:
		return "suspended"
	case EnclaveStatusRevoked:
		return "revoked"
	default:
		return fmt.Sprintf("unknown(%d)", int(s))
	}
}

// Measurements captures the identity of enclave code.
type Measurements struct {
	// CodeHash is the SHA-256 of the enclave binary/image.
	CodeHash []byte
	// ConfigHash captures enclave configuration (e.g., Nitro PCR0).
	ConfigHash []byte
	// SignerHash identifies who signed the enclave (e.g., PCR8 in Nitro).
	SignerHash []byte
}

// Validate checks that measurements are non-empty.
func (m *Measurements) Validate() error {
	if len(m.CodeHash) == 0 {
		return fmt.Errorf("code hash must not be empty")
	}
	return nil
}

// Attestation is a provider-agnostic attestation document.
type Attestation struct {
	// Provider identifies which TEE produced this attestation.
	Provider ProviderType
	// Document is the raw attestation document bytes (provider-specific format).
	Document []byte
	// Measurements are the enclave measurements at attestation time.
	Measurements Measurements
	// Timestamp is when the attestation was generated.
	Timestamp time.Time
	// Nonce is an optional challenge for freshness.
	Nonce []byte
	// UserData is arbitrary data bound into the attestation.
	UserData []byte
}

// Hash returns the SHA-256 hash of the attestation document.
func (a *Attestation) Hash() []byte {
	h := sha256.Sum256(a.Document)
	return h[:]
}

// HashHex returns the hex-encoded attestation hash.
func (a *Attestation) HashHex() string {
	return hex.EncodeToString(a.Hash())
}

// IsExpired returns true if the attestation is older than the given TTL.
func (a *Attestation) IsExpired(ttl time.Duration) bool {
	return time.Since(a.Timestamp) > ttl
}

// Validate performs basic structural validation.
func (a *Attestation) Validate() error {
	if a.Provider == "" {
		return fmt.Errorf("provider must not be empty")
	}
	if len(a.Document) == 0 {
		return fmt.Errorf("document must not be empty")
	}
	if a.Timestamp.IsZero() {
		return fmt.Errorf("timestamp must not be zero")
	}
	return a.Measurements.Validate()
}

// AttestationResult holds the outcome of attestation verification.
type AttestationResult struct {
	// Valid is true if the attestation passed all checks.
	Valid bool
	// Measurements extracted from the verified attestation.
	Measurements Measurements
	// Timestamp of the attestation.
	Timestamp time.Time
	// Error if verification failed.
	Error string
}

// RegisteredEnclave represents an enclave registered on-chain.
type RegisteredEnclave struct {
	Operator        string
	Provider        ProviderType
	Measurements    Measurements
	AttestationHash []byte
	RegisteredAt    int64 // block height
	LastVerified    int64 // block height
	Status          EnclaveStatus
}
