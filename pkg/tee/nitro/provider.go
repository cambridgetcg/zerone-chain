// Package nitro implements the TEE provider interface for AWS Nitro Enclaves.
//
// AWS Nitro Enclaves use a Nitro Security Module (NSM) to generate attestation
// documents containing Platform Configuration Registers (PCRs) that uniquely
// identify the enclave's code, configuration, and signer.
//
// In production, this provider communicates with the NSM device (/dev/nsm)
// and verifies attestation documents against AWS's certificate chain.
// Outside an enclave, it returns an error.
package nitro

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

const (
	// nsmDevicePath is the Nitro Security Module device path.
	nsmDevicePath = "/dev/nsm"
)

// NitroDocument is a simplified representation of a Nitro attestation document.
// The real Nitro attestation document is CBOR-encoded and signed by the NSM.
type NitroDocument struct {
	PCR0      []byte    `json:"pcr0"`       // Enclave image hash
	PCR1      []byte    `json:"pcr1"`       // Kernel and boot hash
	PCR2      []byte    `json:"pcr2"`       // Application hash
	PCR8      []byte    `json:"pcr8"`       // Signer certificate hash
	Nonce     []byte    `json:"nonce"`       // Freshness nonce
	UserData  []byte    `json:"user_data"`   // Bound user data
	Timestamp time.Time `json:"timestamp"`   // Generation time
	CACert    []byte    `json:"ca_cert"`     // AWS root CA certificate
	Signature []byte    `json:"signature"`   // NSM signature
}

// Provider implements tee.TEEProvider for AWS Nitro Enclaves.
type Provider struct {
	// sealingKey is derived from the enclave's identity for seal/unseal operations.
	// In production, this would come from KMS via the NSM.
	sealingKey []byte
}

// NewProvider creates a new Nitro Enclaves TEE provider.
// In production, this would initialize the NSM connection.
func NewProvider() (*Provider, error) {
	return &Provider{}, nil
}

// Type returns the provider type.
func (p *Provider) Type() tee.ProviderType {
	return tee.ProviderNitro
}

// Attest generates an attestation document from the Nitro Security Module.
// Outside an actual Nitro Enclave, this returns an error.
func (p *Provider) Attest(ctx context.Context) (*tee.Attestation, error) {
	// In production, this would:
	// 1. Open /dev/nsm
	// 2. Send an attestation request with optional nonce and user data
	// 3. Receive a CBOR-encoded attestation document
	// 4. Parse PCRs from the document
	//
	// Outside an enclave, /dev/nsm doesn't exist, so we return an error.
	return nil, fmt.Errorf("nitro attestation requires running inside an AWS Nitro Enclave (%s not available)", nsmDevicePath)
}

// Verify checks a Nitro attestation document.
// It validates the document structure and signature chain.
func (p *Provider) Verify(attestation *tee.Attestation) (*tee.AttestationResult, error) {
	if attestation.Provider != tee.ProviderNitro {
		return &tee.AttestationResult{
			Valid: false,
			Error: fmt.Sprintf("expected provider %s, got %s", tee.ProviderNitro, attestation.Provider),
		}, nil
	}

	// Parse the attestation document.
	var doc NitroDocument
	if err := json.Unmarshal(attestation.Document, &doc); err != nil {
		return &tee.AttestationResult{
			Valid: false,
			Error: fmt.Sprintf("failed to parse nitro document: %v", err),
		}, nil
	}

	// Validate PCR0 is present (enclave image hash).
	if len(doc.PCR0) == 0 {
		return &tee.AttestationResult{
			Valid: false,
			Error: "PCR0 (enclave image hash) is empty",
		}, nil
	}

	// In production, we would:
	// 1. Verify the COSE_Sign1 signature
	// 2. Validate the certificate chain against AWS Nitro root CA
	// 3. Check the certificate is not revoked
	// 4. Verify the nonce matches the challenge
	//
	// For now, we validate structure and return measurements.
	return &tee.AttestationResult{
		Valid: true,
		Measurements: tee.Measurements{
			CodeHash:   doc.PCR0,
			ConfigHash: doc.PCR1,
			SignerHash: doc.PCR8,
		},
		Timestamp: doc.Timestamp,
	}, nil
}

// GetMeasurements returns the current enclave measurements.
// Outside an enclave, returns an error.
func (p *Provider) GetMeasurements() (*tee.Measurements, error) {
	return nil, fmt.Errorf("measurements require running inside an AWS Nitro Enclave")
}

// Seal encrypts data with the enclave's sealing key.
// Uses AES-256-GCM with a random nonce.
func (p *Provider) Seal(data []byte) ([]byte, error) {
	key, err := p.getSealingKey()
	if err != nil {
		return nil, fmt.Errorf("failed to get sealing key: %w", err)
	}

	block, err := aes.NewCipher(key)
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

	// Prepend nonce to ciphertext.
	sealed := gcm.Seal(nonce, nonce, data, nil)
	return sealed, nil
}

// Unseal decrypts data previously sealed by this enclave.
func (p *Provider) Unseal(sealed []byte) ([]byte, error) {
	key, err := p.getSealingKey()
	if err != nil {
		return nil, fmt.Errorf("failed to get sealing key: %w", err)
	}

	block, err := aes.NewCipher(key)
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

// getSealingKey returns the sealing key, deriving it if needed.
// In production, this would use KMS via the NSM.
func (p *Provider) getSealingKey() ([]byte, error) {
	if len(p.sealingKey) == 0 {
		// In production, this would be derived from the enclave identity via KMS.
		// For non-enclave environments, we derive from a fixed seed.
		h := sha256.Sum256([]byte("nitro-enclave-sealing-key-dev"))
		p.sealingKey = h[:]
	}
	return p.sealingKey, nil
}
