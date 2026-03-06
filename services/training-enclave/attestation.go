package main

import (
	"crypto/ecdsa"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

// TrainingAttestation is the cryptographic proof of correct training process.
// This is the primary output that proves to the chain that training was performed
// correctly inside the TEE.
type TrainingAttestation struct {
	EnclaveID          string    `json:"enclave_id"`
	DatasetFingerprint []byte    `json:"dataset_fingerprint"` // SHA-256 of assembled dataset
	DatasetSize        int64     `json:"dataset_size"`        // number of TDUs used
	BaseModel          string    `json:"base_model"`          // e.g. "llama-3-8b"
	TrainingConfig     []byte    `json:"training_config"`     // SHA-256 of hyperparameters
	ModelHash          []byte    `json:"model_hash"`          // SHA-256 of output LoRA weights
	BenchmarkScore     float64   `json:"benchmark_score"`     // overall benchmark score
	StartTime          time.Time `json:"start_time"`
	EndTime            time.Time `json:"end_time"`
	DestructionProof   []byte    `json:"destruction_proof"`   // signed proof that dataset was zeroed
	Signature          []byte    `json:"signature"`           // signed by enclave key
}

// AttestationGenerator creates and signs training attestations.
type AttestationGenerator struct {
	EnclaveID  string
	EnclaveKey *ecdsa.PrivateKey
}

// Generate creates a TrainingAttestation from the completed training run.
func (g *AttestationGenerator) Generate(
	dataset *PreparedDataset,
	result *TrainingResult,
	signed *SignedOutputs,
	phaseTimings map[Phase]time.Time,
) (*TrainingAttestation, error) {
	if dataset == nil {
		return nil, errors.New("dataset required for attestation")
	}
	if result == nil {
		return nil, errors.New("training result required for attestation")
	}

	startTime := phaseTimings[PhaseCollect]
	endTime := time.Now()

	att := &TrainingAttestation{
		EnclaveID:          g.EnclaveID,
		DatasetFingerprint: dataset.Fingerprint,
		DatasetSize:        int64(len(dataset.Samples)),
		BaseModel:          g.EnclaveID, // placeholder until base model tracked
		TrainingConfig:     result.ConfigHash,
		ModelHash:          result.ModelHash,
		BenchmarkScore:     result.BenchmarkScore,
		StartTime:          startTime,
		EndTime:            endTime,
	}

	// Use the base model name from the generator context if available.
	att.BaseModel = g.EnclaveID // will be overridden

	return att, nil
}

// GenerateWithBaseModel creates an attestation with an explicit base model name.
func (g *AttestationGenerator) GenerateWithBaseModel(
	dataset *PreparedDataset,
	result *TrainingResult,
	baseModel string,
	phaseTimings map[Phase]time.Time,
) (*TrainingAttestation, error) {
	if dataset == nil {
		return nil, errors.New("dataset required for attestation")
	}
	if result == nil {
		return nil, errors.New("training result required for attestation")
	}

	startTime := phaseTimings[PhaseCollect]
	endTime := time.Now()

	att := &TrainingAttestation{
		EnclaveID:          g.EnclaveID,
		DatasetFingerprint: dataset.Fingerprint,
		DatasetSize:        int64(len(dataset.Samples)),
		BaseModel:          baseModel,
		TrainingConfig:     result.ConfigHash,
		ModelHash:          result.ModelHash,
		BenchmarkScore:     result.BenchmarkScore,
		StartTime:          startTime,
		EndTime:            endTime,
	}

	return att, nil
}

// Sign signs the attestation with the enclave key and sets the Signature field.
func (g *AttestationGenerator) Sign(att *TrainingAttestation) error {
	if g.EnclaveKey == nil {
		return errors.New("enclave key not available for signing")
	}

	// Serialize attestation without signature for signing.
	sigCopy := att.Signature
	att.Signature = nil
	defer func() {
		if att.Signature == nil {
			att.Signature = sigCopy
		}
	}()

	payload, err := json.Marshal(att)
	if err != nil {
		return fmt.Errorf("failed to serialize attestation for signing: %w", err)
	}

	sig, err := signData(g.EnclaveKey, payload)
	if err != nil {
		return fmt.Errorf("failed to sign attestation: %w", err)
	}

	att.Signature = sig
	return nil
}

// Verify checks the attestation signature against the enclave's public key.
func VerifyAttestation(att *TrainingAttestation, pub *ecdsa.PublicKey) (bool, error) {
	if att == nil {
		return false, errors.New("nil attestation")
	}
	if pub == nil {
		return false, errors.New("nil public key")
	}

	// Extract and clear signature for verification.
	sig := att.Signature
	att.Signature = nil
	defer func() { att.Signature = sig }()

	payload, err := json.Marshal(att)
	if err != nil {
		return false, fmt.Errorf("failed to serialize attestation: %w", err)
	}

	return VerifyOutputSignature(pub, payload, sig), nil
}

// Hash returns the SHA-256 hash of the attestation (used as on-chain identifier).
func (att *TrainingAttestation) Hash() []byte {
	bz, err := json.Marshal(att)
	if err != nil {
		h := sha256.Sum256(nil)
		return h[:]
	}
	h := sha256.Sum256(bz)
	return h[:]
}
