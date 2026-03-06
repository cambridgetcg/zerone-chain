package tee

import "context"

// TEEProvider is the interface that all TEE implementations must satisfy.
type TEEProvider interface {
	// Type returns the provider type identifier.
	Type() ProviderType

	// Attest generates an attestation document proving enclave identity.
	Attest(ctx context.Context) (*Attestation, error)

	// Verify checks a remote attestation document and returns the result.
	Verify(attestation *Attestation) (*AttestationResult, error)

	// GetMeasurements returns the current enclave measurements (code hash, configuration).
	GetMeasurements() (*Measurements, error)

	// Seal encrypts data with the enclave's sealing key.
	Seal(data []byte) ([]byte, error)

	// Unseal decrypts data previously sealed by this enclave.
	Unseal(sealed []byte) ([]byte, error)
}
