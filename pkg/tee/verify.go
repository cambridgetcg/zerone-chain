package tee

import (
	"bytes"
	"fmt"
	"time"
)

const (
	// DefaultAttestationTTL is the default time-to-live for attestation documents.
	DefaultAttestationTTL = 24 * time.Hour
)

// VerifyAttestation performs cross-provider attestation verification.
// It checks structural validity, expiration, provider-specific verification,
// and measurement matching.
func VerifyAttestation(provider TEEProvider, attestation *Attestation, expectedMeasurements *Measurements, ttl time.Duration) (*AttestationResult, error) {
	if attestation == nil {
		return nil, fmt.Errorf("attestation must not be nil")
	}

	// Structural validation.
	if err := attestation.Validate(); err != nil {
		return &AttestationResult{Valid: false, Error: err.Error()}, nil
	}

	// Provider type must match.
	if attestation.Provider != provider.Type() {
		return &AttestationResult{
			Valid: false,
			Error: fmt.Sprintf("provider mismatch: attestation is %s, provider is %s", attestation.Provider, provider.Type()),
		}, nil
	}

	// Check expiration.
	if attestation.IsExpired(ttl) {
		return &AttestationResult{
			Valid: false,
			Error: fmt.Sprintf("attestation expired: generated %s ago, TTL is %s", time.Since(attestation.Timestamp).Round(time.Second), ttl),
		}, nil
	}

	// Provider-specific verification (signature, certificate chain, etc.).
	result, err := provider.Verify(attestation)
	if err != nil {
		return nil, fmt.Errorf("provider verification failed: %w", err)
	}
	if !result.Valid {
		return result, nil
	}

	// Match measurements against expected values (if provided).
	if expectedMeasurements != nil {
		if err := matchMeasurements(&result.Measurements, expectedMeasurements); err != nil {
			return &AttestationResult{
				Valid: false,
				Error: fmt.Sprintf("measurement mismatch: %s", err),
			}, nil
		}
	}

	return result, nil
}

// matchMeasurements checks that actual measurements match the expected ones.
// Only non-empty expected fields are checked, allowing partial matching.
func matchMeasurements(actual, expected *Measurements) error {
	if len(expected.CodeHash) > 0 && !bytes.Equal(actual.CodeHash, expected.CodeHash) {
		return fmt.Errorf("code hash mismatch: got %x, want %x", actual.CodeHash, expected.CodeHash)
	}
	if len(expected.ConfigHash) > 0 && !bytes.Equal(actual.ConfigHash, expected.ConfigHash) {
		return fmt.Errorf("config hash mismatch: got %x, want %x", actual.ConfigHash, expected.ConfigHash)
	}
	if len(expected.SignerHash) > 0 && !bytes.Equal(actual.SignerHash, expected.SignerHash) {
		return fmt.Errorf("signer hash mismatch: got %x, want %x", actual.SignerHash, expected.SignerHash)
	}
	return nil
}
