package types

// TrainingRecord stores the on-chain record of a completed TEE training run.
// Stored as JSON under TrainingRecordPrefix + attestation hash.
type TrainingRecord struct {
	Operator           string  `json:"operator"`
	EnclaveID          string  `json:"enclave_id"`
	AttestationHash    string  `json:"attestation_hash"`    // hex of SHA-256(full attestation JSON)
	DatasetFingerprint string  `json:"dataset_fingerprint"` // hex of dataset hash
	DatasetSize        int64   `json:"dataset_size"`        // number of TDUs used
	BaseModel          string  `json:"base_model"`          // e.g. "llama-3-8b"
	ModelHash          string  `json:"model_hash"`          // hex of LoRA adapter weights hash
	BenchmarkScore     float64 `json:"benchmark_score"`     // overall benchmark score [0,1]
	BlockHeight        int64   `json:"block_height"`        // block at which training was recorded
}

// MsgRecordTraining is the message for recording a training attestation on-chain.
// Manually implemented to avoid proto changes (following MsgAttestStorage pattern).
type MsgRecordTraining struct {
	Operator           string  `json:"operator"`
	AttestationHash    string  `json:"attestation_hash"`
	DatasetFingerprint string  `json:"dataset_fingerprint"`
	DatasetSize        int64   `json:"dataset_size"`
	BaseModel          string  `json:"base_model"`
	ModelHash          string  `json:"model_hash"`
	BenchmarkScore     float64 `json:"benchmark_score"`
}

// ValidateBasic performs stateless validation on MsgRecordTraining.
func (msg *MsgRecordTraining) ValidateBasic() error {
	if msg.Operator == "" {
		return ErrUnauthorized.Wrap("operator is required")
	}
	if msg.AttestationHash == "" {
		return ErrInvalidTrainingAttestation.Wrap("attestation hash is required")
	}
	if msg.ModelHash == "" {
		return ErrInvalidTrainingAttestation.Wrap("model hash is required")
	}
	if msg.BaseModel == "" {
		return ErrInvalidTrainingAttestation.Wrap("base model is required")
	}
	if msg.DatasetSize <= 0 {
		return ErrInvalidTrainingAttestation.Wrap("dataset size must be positive")
	}
	if msg.BenchmarkScore < 0 || msg.BenchmarkScore > 1 {
		return ErrInvalidTrainingAttestation.Wrap("benchmark score must be in [0, 1]")
	}
	return nil
}

// MsgRecordTrainingResponse is the response for MsgRecordTraining.
type MsgRecordTrainingResponse struct {
	AttestationHash string `json:"attestation_hash"`
}

// Training event types.
const (
	EventTrainingRecorded = "training_recorded"

	AttributeAttestationHash    = "attestation_hash"
	AttributeDatasetFingerprint = "dataset_fingerprint"
	AttributeDatasetSize        = "dataset_size"
	AttributeBaseModel          = "base_model"
	AttributeModelHash          = "model_hash"
	AttributeBenchmarkScore     = "benchmark_score"
)
