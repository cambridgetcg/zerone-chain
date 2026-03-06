package types

import (
	"fmt"

	sdkmath "cosmossdk.io/math"
)

// ─── Model Status ───────────────────────────────────────────────────────────

// ModelStatus represents the lifecycle state of a registered model.
type ModelStatus string

const (
	ModelStatusActive     ModelStatus = "active"
	ModelStatusDeprecated ModelStatus = "deprecated"
	ModelStatusSuperseded ModelStatus = "superseded"
	ModelStatusFailed     ModelStatus = "failed"
)

// ValidModelStatuses is the set of valid model statuses.
var ValidModelStatuses = map[ModelStatus]bool{
	ModelStatusActive:     true,
	ModelStatusDeprecated: true,
	ModelStatusSuperseded: true,
	ModelStatusFailed:     true,
}

// ─── Benchmark Result ───────────────────────────────────────────────────────

// BenchmarkResult stores a single benchmark evaluation for a model.
type BenchmarkResult struct {
	BenchmarkID string `json:"benchmark_id"`
	Score       string `json:"score"`     // sdkmath.LegacyDec serialized
	Category    string `json:"category"`  // "code", "reasoning", "instruction"
	PassRate    string `json:"pass_rate"` // sdkmath.LegacyDec serialized
}

// GetScore parses the benchmark score.
func (b *BenchmarkResult) GetScore() sdkmath.LegacyDec {
	d, err := sdkmath.LegacyNewDecFromStr(b.Score)
	if err != nil {
		return sdkmath.LegacyZeroDec()
	}
	return d
}

// GetPassRate parses the pass rate.
func (b *BenchmarkResult) GetPassRate() sdkmath.LegacyDec {
	d, err := sdkmath.LegacyNewDecFromStr(b.PassRate)
	if err != nil {
		return sdkmath.LegacyZeroDec()
	}
	return d
}

// ─── Model Record ───────────────────────────────────────────────────────────

// ModelRecord represents a trained model registered on-chain.
// Stored as JSON under ModelRecordPrefix + modelID.
type ModelRecord struct {
	// Identity
	ModelID       string `json:"model_id"`        // SHA-256 of TEE attestation
	Name          string `json:"name"`            // human-readable name
	Domain        string `json:"domain"`          // training domain (e.g. "code/go")
	Version       uint64 `json:"version"`         // monotonic per lineage
	ParentModelID string `json:"parent_model_id"` // previous version (empty for v1)

	// Training Lineage
	TrainingRecordID string   `json:"training_record_id"` // link to TEE training attestation
	TDUIDs           []string `json:"tdu_ids"`            // contributing TDU sample IDs
	DatasetIDs       []string `json:"dataset_ids"`        // dataset IDs used
	TDUCount         uint64   `json:"tdu_count"`          // total TDU count

	// Quality Metrics
	BenchmarkScore   string            `json:"benchmark_score"`   // aggregate score [0,1]
	BenchmarkDetails []BenchmarkResult `json:"benchmark_details"` // per-benchmark
	FitnessWeighted  string            `json:"fitness_weighted"`  // avg fitness of training data

	// Lifecycle
	Status            ModelStatus `json:"status"`
	Publisher         string      `json:"publisher"`          // address that published
	PublishedAt       int64       `json:"published_at"`       // block height
	DeprecatedAt      int64       `json:"deprecated_at"`      // block height (0 if active)
	DeprecationReason string      `json:"deprecation_reason"` // why deprecated
	SupersededBy      string      `json:"superseded_by"`      // model that replaced this one

	// Inference
	InferenceCount uint64 `json:"inference_count"` // total API calls served

	// Attestation
	TEEAttestation string `json:"tee_attestation"` // hex of attestation doc
	ModelHash      string `json:"model_hash"`      // SHA-256 of model weights
}

// GetBenchmarkScore parses the aggregate benchmark score.
func (m *ModelRecord) GetBenchmarkScore() sdkmath.LegacyDec {
	if m.BenchmarkScore == "" {
		return sdkmath.LegacyZeroDec()
	}
	d, err := sdkmath.LegacyNewDecFromStr(m.BenchmarkScore)
	if err != nil {
		return sdkmath.LegacyZeroDec()
	}
	return d
}

// SetBenchmarkScore stores the benchmark score, clamped to [0, 1].
func (m *ModelRecord) SetBenchmarkScore(score sdkmath.LegacyDec) {
	if score.GT(sdkmath.LegacyOneDec()) {
		score = sdkmath.LegacyOneDec()
	}
	if score.LT(sdkmath.LegacyZeroDec()) {
		score = sdkmath.LegacyZeroDec()
	}
	m.BenchmarkScore = score.String()
}

// GetFitnessWeighted parses the weighted fitness score.
func (m *ModelRecord) GetFitnessWeighted() sdkmath.LegacyDec {
	if m.FitnessWeighted == "" {
		return sdkmath.LegacyZeroDec()
	}
	d, err := sdkmath.LegacyNewDecFromStr(m.FitnessWeighted)
	if err != nil {
		return sdkmath.LegacyZeroDec()
	}
	return d
}

// SetFitnessWeighted stores the weighted fitness, clamped to [0, 1].
func (m *ModelRecord) SetFitnessWeighted(score sdkmath.LegacyDec) {
	if score.GT(sdkmath.LegacyOneDec()) {
		score = sdkmath.LegacyOneDec()
	}
	if score.LT(sdkmath.LegacyZeroDec()) {
		score = sdkmath.LegacyZeroDec()
	}
	m.FitnessWeighted = score.String()
}

// ─── Model Lineage ──────────────────────────────────────────────────────────

// ModelLineage tracks the full ancestry of a model through training rounds.
type ModelLineage struct {
	ModelID    string   `json:"model_id"`
	Ancestors  []string `json:"ancestors"`  // ordered: parent, grandparent, ...
	Generation uint64   `json:"generation"` // how many training rounds deep
}

// ─── Quality Thresholds ─────────────────────────────────────────────────────

var (
	// ModelMinBenchmarkScore is the minimum benchmark score to publish a model.
	ModelMinBenchmarkScore = sdkmath.LegacyNewDecWithPrec(3, 1) // 0.3
	// ModelMinFitnessWeighted is the minimum weighted fitness of training data.
	ModelMinFitnessWeighted = sdkmath.LegacyNewDecWithPrec(4, 1) // 0.4
	// ModelMinTDUCount is the minimum number of TDUs used for training.
	ModelMinTDUCount uint64 = 10
)

// ValidateQuality checks that a model meets minimum quality thresholds.
func (m *ModelRecord) ValidateQuality() error {
	if m.GetBenchmarkScore().LT(ModelMinBenchmarkScore) {
		return fmt.Errorf("benchmark score %s below minimum %s", m.BenchmarkScore, ModelMinBenchmarkScore)
	}
	if m.GetFitnessWeighted().LT(ModelMinFitnessWeighted) {
		return fmt.Errorf("fitness weighted %s below minimum %s", m.FitnessWeighted, ModelMinFitnessWeighted)
	}
	if m.TDUCount < ModelMinTDUCount {
		return fmt.Errorf("TDU count %d below minimum %d", m.TDUCount, ModelMinTDUCount)
	}
	if m.ModelHash == "" {
		return fmt.Errorf("model hash is required")
	}
	if m.TEEAttestation == "" {
		return fmt.Errorf("TEE attestation is required")
	}
	return nil
}

// ValidateBasic performs stateless validation on a ModelRecord.
func (m *ModelRecord) ValidateBasic() error {
	if m.ModelID == "" {
		return fmt.Errorf("model ID is required")
	}
	if m.Name == "" {
		return fmt.Errorf("model name is required")
	}
	if m.Domain == "" {
		return fmt.Errorf("domain is required")
	}
	if m.Publisher == "" {
		return fmt.Errorf("publisher is required")
	}
	if !ValidModelStatuses[m.Status] {
		return fmt.Errorf("invalid model status: %s", m.Status)
	}
	return nil
}

// ─── Messages ───────────────────────────────────────────────────────────────

// MsgPublishModel is the message for registering a trained model on-chain.
type MsgPublishModel struct {
	Publisher        string            `json:"publisher"`
	Name             string            `json:"name"`
	Domain           string            `json:"domain"`
	ParentModelID    string            `json:"parent_model_id"`
	TrainingRecordID string            `json:"training_record_id"`
	TDUIDs           []string          `json:"tdu_ids"`
	DatasetIDs       []string          `json:"dataset_ids"`
	BenchmarkScore   string            `json:"benchmark_score"`
	BenchmarkDetails []BenchmarkResult `json:"benchmark_details"`
	FitnessWeighted  string            `json:"fitness_weighted"`
	TEEAttestation   string            `json:"tee_attestation"`
	ModelHash        string            `json:"model_hash"`
}

// ValidateBasic performs stateless validation on MsgPublishModel.
func (msg *MsgPublishModel) ValidateBasic() error {
	if msg.Publisher == "" {
		return ErrUnauthorized.Wrap("publisher is required")
	}
	if msg.Name == "" {
		return ErrInvalidTrainingAttestation.Wrap("model name is required")
	}
	if msg.Domain == "" {
		return ErrInvalidTrainingAttestation.Wrap("domain is required")
	}
	if msg.ModelHash == "" {
		return ErrInvalidTrainingAttestation.Wrap("model hash is required")
	}
	if msg.TEEAttestation == "" {
		return ErrInvalidAttestation.Wrap("TEE attestation is required")
	}
	if msg.TrainingRecordID == "" {
		return ErrInvalidTrainingAttestation.Wrap("training record ID is required")
	}
	return nil
}

// MsgPublishModelResponse is returned after publishing a model.
type MsgPublishModelResponse struct {
	ModelID string `json:"model_id"`
	Version uint64 `json:"version"`
}

// MsgDeprecateModel deprecates a model with a reason.
type MsgDeprecateModel struct {
	Authority string `json:"authority"` // publisher or governance
	ModelID   string `json:"model_id"`
	Reason    string `json:"reason"`
}

// ValidateBasic performs stateless validation.
func (msg *MsgDeprecateModel) ValidateBasic() error {
	if msg.Authority == "" {
		return ErrUnauthorized.Wrap("authority is required")
	}
	if msg.ModelID == "" {
		return fmt.Errorf("model ID is required")
	}
	return nil
}

// MsgDeprecateModelResponse is returned after deprecating a model.
type MsgDeprecateModelResponse struct{}

// ─── Events ─────────────────────────────────────────────────────────────────

const (
	EventModelPublished  = "model_published"
	EventModelDeprecated = "model_deprecated"
	EventModelSuperseded = "model_superseded"
	EventInferenceServed = "inference_served"

	AttributeModelID       = "model_id"
	AttributeModelName     = "model_name"
	AttributeModelDomain   = "model_domain"
	AttributeModelVersion  = "model_version"
	AttributeModelStatus   = "model_status"
	AttributeModelPublisher = "model_publisher"
)

// Model registry errors are registered in errors.go to keep code numbering centralized.
// They are re-exported here for documentation:
//   ErrModelNotFound          (170) — model not found
//   ErrModelAlreadyExists     (171) — model already exists
//   ErrModelQualityTooLow     (172) — model quality below threshold
//   ErrModelNotActive         (173) — model is not active
//   ErrModelAlreadyDeprecated (174) — model is already deprecated
//   ErrInvalidModelLineage    (175) — invalid model lineage
