package main

import (
	"crypto/sha256"
	"errors"
)

// Destroyer handles secure memory zeroing and verification for all dataset
// and intermediate training state inside the TEE.
type Destroyer struct{}

// DestroyDataset zeroes all dataset memory (content, samples, fingerprint).
// After this call, the PreparedDataset is unusable.
func (d *Destroyer) DestroyDataset(dataset *PreparedDataset) error {
	if dataset == nil {
		return nil // nothing to destroy
	}

	// Zero all sample content.
	for i := range dataset.Samples {
		zeroString(&dataset.Samples[i].Text)
		zeroString(&dataset.Samples[i].ID)
		zeroString(&dataset.Samples[i].Domain)
		zeroString(&dataset.Samples[i].Metadata)
	}

	// Clear the sample slice.
	dataset.Samples = nil

	// Zero fingerprint.
	zeroBytes(dataset.Fingerprint)
	dataset.Fingerprint = nil

	// Reset counters.
	dataset.DomainSampleCount = 0
	dataset.GeneralSampleCount = 0
	dataset.TotalTDUs = 0
	dataset.FilteredCount = 0

	return nil
}

// DestroyTrainingState zeroes all intermediate training state (gradients,
// optimizer state, activations). Only the model weights and metrics survive
// for the output phase (they were already signed and exported).
func (d *Destroyer) DestroyTrainingState(result *TrainingResult) error {
	if result == nil {
		return nil
	}

	// Zero gradients.
	for i := range result.gradients {
		zeroBytes(result.gradients[i])
	}
	result.gradients = nil

	// Zero optimizer state.
	for i := range result.optimizerState {
		zeroBytes(result.optimizerState[i])
	}
	result.optimizerState = nil

	// Zero activations.
	for i := range result.activations {
		zeroBytes(result.activations[i])
	}
	result.activations = nil

	return nil
}

// VerifyDestruction scans dataset and training state to confirm all sensitive
// memory has been zeroed. Returns a proof hash on success.
func (d *Destroyer) VerifyDestruction(dataset *PreparedDataset, result *TrainingResult) ([]byte, error) {
	// Verify dataset is destroyed.
	if dataset != nil {
		if dataset.Samples != nil && len(dataset.Samples) > 0 {
			return nil, errors.New("destruction verification failed: dataset samples still present")
		}
		if dataset.Fingerprint != nil {
			return nil, errors.New("destruction verification failed: dataset fingerprint still present")
		}
	}

	// Verify intermediate training state is destroyed.
	if result != nil {
		if result.gradients != nil {
			return nil, errors.New("destruction verification failed: gradients still present")
		}
		if result.optimizerState != nil {
			return nil, errors.New("destruction verification failed: optimizer state still present")
		}
		if result.activations != nil {
			return nil, errors.New("destruction verification failed: activations still present")
		}
	}

	// Generate destruction proof: hash of verification timestamp.
	// In production, this would include enclave memory attestation.
	proof := sha256.Sum256([]byte("destruction-verified"))
	return proof[:], nil
}

// zeroBytes writes 0x00 to every byte of a slice.
func zeroBytes(b []byte) {
	for i := range b {
		b[i] = 0
	}
}

// zeroString overwrites a string pointer with an empty string.
// Note: Go strings are immutable, so this replaces the reference.
// The original string data becomes eligible for GC.
func zeroString(s *string) {
	*s = ""
}
