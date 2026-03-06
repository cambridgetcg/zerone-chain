//go:build tee_mock

// Package mock implements a mock TEE provider for testing and development.
// It simulates the attestation flow without requiring TEE hardware.
//
// IMPORTANT: This package is gated behind the "tee_mock" build tag
// and is never available in production builds. The "testing" tag
// allows its use in test files.
package mock

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"time"

	"github.com/zerone-chain/zerone/pkg/tee"
)

// Provider implements tee.TEEProvider with simulated attestation.
type Provider struct {
	measurements tee.Measurements
	sealingKey   []byte
	failVerify   bool      // If true, Verify always returns invalid.
	fixedTime    time.Time // If non-zero, use this instead of time.Now().
}

// Option configures a mock Provider.
type Option func(*Provider)

// WithMeasurements sets the measurements the mock will report.
func WithMeasurements(m tee.Measurements) Option {
	return func(p *Provider) {
		p.measurements = m
	}
}

// WithSealingKey sets a custom sealing key.
func WithSealingKey(key []byte) Option {
	return func(p *Provider) {
		p.sealingKey = key
	}
}

// WithFailVerify makes Verify always return invalid.
func WithFailVerify() Option {
	return func(p *Provider) {
		p.failVerify = true
	}
}

// WithFixedTime overrides the attestation timestamp.
func WithFixedTime(t time.Time) Option {
	return func(p *Provider) {
		p.fixedTime = t
	}
}

// NewProvider creates a mock TEE provider for testing.
func NewProvider(opts ...Option) *Provider {
	// Default measurements (deterministic for reproducible tests).
	defaultCode := sha256.Sum256([]byte("mock-enclave-code-v1"))
	defaultConfig := sha256.Sum256([]byte("mock-enclave-config-v1"))
	defaultSigner := sha256.Sum256([]byte("mock-enclave-signer-v1"))
	defaultKey := sha256.Sum256([]byte("mock-sealing-key"))

	p := &Provider{
		measurements: tee.Measurements{
			CodeHash:   defaultCode[:],
			ConfigHash: defaultConfig[:],
			SignerHash: defaultSigner[:],
		},
		sealingKey: defaultKey[:],
	}
	for _, opt := range opts {
		opt(p)
	}
	return p
}

// Type returns the provider type.
func (p *Provider) Type() tee.ProviderType {
	return tee.ProviderMock
}

// MockDocument is the JSON structure of a mock attestation document.
type MockDocument struct {
	CodeHash   []byte    `json:"code_hash"`
	ConfigHash []byte    `json:"config_hash"`
	SignerHash []byte    `json:"signer_hash"`
	Nonce      []byte    `json:"nonce"`
	UserData   []byte    `json:"user_data"`
	Timestamp  time.Time `json:"timestamp"`
}

// Attest generates a mock attestation document.
func (p *Provider) Attest(ctx context.Context) (*tee.Attestation, error) {
	now := p.now()

	nonce := make([]byte, 32)
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, fmt.Errorf("failed to generate nonce: %w", err)
	}

	doc := MockDocument{
		CodeHash:   p.measurements.CodeHash,
		ConfigHash: p.measurements.ConfigHash,
		SignerHash: p.measurements.SignerHash,
		Nonce:      nonce,
		Timestamp:  now,
	}

	docBytes, err := json.Marshal(doc)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal mock document: %w", err)
	}

	return &tee.Attestation{
		Provider:     tee.ProviderMock,
		Document:     docBytes,
		Measurements: p.measurements,
		Timestamp:    now,
		Nonce:        nonce,
	}, nil
}

// Verify checks a mock attestation document.
func (p *Provider) Verify(attestation *tee.Attestation) (*tee.AttestationResult, error) {
	if p.failVerify {
		return &tee.AttestationResult{
			Valid: false,
			Error: "mock provider configured to fail verification",
		}, nil
	}

	if attestation.Provider != tee.ProviderMock {
		return &tee.AttestationResult{
			Valid: false,
			Error: fmt.Sprintf("expected provider %s, got %s", tee.ProviderMock, attestation.Provider),
		}, nil
	}

	var doc MockDocument
	if err := json.Unmarshal(attestation.Document, &doc); err != nil {
		return &tee.AttestationResult{
			Valid: false,
			Error: fmt.Sprintf("failed to parse mock document: %v", err),
		}, nil
	}

	return &tee.AttestationResult{
		Valid: true,
		Measurements: tee.Measurements{
			CodeHash:   doc.CodeHash,
			ConfigHash: doc.ConfigHash,
			SignerHash: doc.SignerHash,
		},
		Timestamp: doc.Timestamp,
	}, nil
}

// GetMeasurements returns the mock enclave's measurements.
func (p *Provider) GetMeasurements() (*tee.Measurements, error) {
	return &p.measurements, nil
}

// Seal encrypts data using AES-256-GCM with the mock sealing key.
func (p *Provider) Seal(data []byte) ([]byte, error) {
	block, err := aes.NewCipher(p.sealingKey)
	if err != nil {
		return nil, fmt.Errorf("failed to create cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("failed to create GCM: %w", err)
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, fmt.Errorf("failed to generate nonce: %w", err)
	}

	sealed := gcm.Seal(nonce, nonce, data, nil)
	return sealed, nil
}

// Unseal decrypts data previously sealed by this mock provider.
func (p *Provider) Unseal(sealed []byte) ([]byte, error) {
	block, err := aes.NewCipher(p.sealingKey)
	if err != nil {
		return nil, fmt.Errorf("failed to create cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("failed to create GCM: %w", err)
	}

	nonceSize := gcm.NonceSize()
	if len(sealed) < nonceSize {
		return nil, fmt.Errorf("sealed data too short")
	}

	nonce, ciphertext := sealed[:nonceSize], sealed[nonceSize:]
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to unseal: %w", err)
	}

	return plaintext, nil
}

func (p *Provider) now() time.Time {
	if !p.fixedTime.IsZero() {
		return p.fixedTime
	}
	return time.Now()
}
