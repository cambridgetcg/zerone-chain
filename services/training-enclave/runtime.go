package main

import (
	"crypto/ecdsa"
	"errors"
	"fmt"
	"sync"
	"time"
)

// Phase represents a lifecycle phase of the training enclave.
type Phase int

const (
	PhaseIdle    Phase = iota // not started
	PhaseCollect              // collecting encrypted shards from validators
	PhasePrepare              // reassembling and filtering dataset
	PhaseTrain                // running fine-tuning
	PhaseOutput               // exporting model and metrics
	PhaseDestroy              // zeroing dataset memory
	PhaseDone                 // lifecycle complete
	PhaseFailed               // unrecoverable error
)

// String returns a human-readable name for the phase.
func (p Phase) String() string {
	switch p {
	case PhaseIdle:
		return "idle"
	case PhaseCollect:
		return "collect"
	case PhasePrepare:
		return "prepare"
	case PhaseTrain:
		return "train"
	case PhaseOutput:
		return "output"
	case PhaseDestroy:
		return "destroy"
	case PhaseDone:
		return "done"
	case PhaseFailed:
		return "failed"
	default:
		return "unknown"
	}
}

// validTransitions defines which phase transitions are allowed.
var validTransitions = map[Phase][]Phase{
	PhaseIdle:    {PhaseCollect},
	PhaseCollect: {PhasePrepare, PhaseFailed},
	PhasePrepare: {PhaseTrain, PhaseFailed},
	PhaseTrain:   {PhaseOutput, PhaseFailed},
	PhaseOutput:  {PhaseDestroy, PhaseFailed},
	PhaseDestroy: {PhaseDone, PhaseFailed},
}

// TrainingConfig holds all configuration for a training run.
type TrainingConfig struct {
	EnclaveID      string
	EnclaveKey     *ecdsa.PrivateKey
	Attestation    []byte
	BaseModel      string            // e.g. "llama-3-8b"
	TargetDomain   string            // primary domain for 70/30 mix
	DomainMixRatio float64           // default 0.7 (70% domain, 30% general)
	LoRAConfig     LoRAConfig
	GRPCAddr       string            // chain gRPC address
	SnapshotHeight int64
	Assignments    map[string][]string // validator → TDU IDs
	ExpectedHashes map[string][]byte   // TDU ID → content hash
}

// LoRAConfig holds LoRA/QLoRA fine-tuning hyperparameters.
type LoRAConfig struct {
	Rank           int     `json:"rank"`
	Alpha          float64 `json:"alpha"`
	Dropout        float64 `json:"dropout"`
	LearningRate   float64 `json:"learning_rate"`
	Epochs         int     `json:"epochs"`
	BatchSize      int     `json:"batch_size"`
	WarmupSteps    int     `json:"warmup_steps"`
	MaxSeqLength   int     `json:"max_seq_length"`
}

// DefaultLoRAConfig returns sensible defaults for LoRA fine-tuning.
func DefaultLoRAConfig() LoRAConfig {
	return LoRAConfig{
		Rank:         16,
		Alpha:        32.0,
		Dropout:      0.05,
		LearningRate: 2e-4,
		Epochs:       3,
		BatchSize:    4,
		WarmupSteps:  100,
		MaxSeqLength: 2048,
	}
}

// Runtime manages the complete training lifecycle inside the TEE.
// All dataset memory is held in-process; if the enclave crashes, the OS
// reclaims the memory automatically (dataset destruction by design).
type Runtime struct {
	mu     sync.RWMutex
	phase  Phase
	config *TrainingConfig

	// Phase start times for attestation.
	phaseStart map[Phase]time.Time

	// In-memory dataset (NEVER touches disk).
	dataset *PreparedDataset

	// Training outputs.
	trainingResult *TrainingResult

	// Final attestation.
	attestation *TrainingAttestation
}

// NewRuntime creates a new training enclave runtime.
func NewRuntime(config *TrainingConfig) *Runtime {
	if config.DomainMixRatio == 0 {
		config.DomainMixRatio = 0.7
	}
	return &Runtime{
		phase:      PhaseIdle,
		config:     config,
		phaseStart: make(map[Phase]time.Time),
	}
}

// Phase returns the current lifecycle phase.
func (r *Runtime) Phase() Phase {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.phase
}

// transition moves the runtime to a new phase, enforcing valid transitions.
func (r *Runtime) transition(to Phase) error {
	allowed, ok := validTransitions[r.phase]
	if !ok {
		return fmt.Errorf("no transitions from phase %s", r.phase)
	}
	for _, a := range allowed {
		if a == to {
			r.phase = to
			r.phaseStart[to] = time.Now()
			return nil
		}
	}
	return fmt.Errorf("invalid transition: %s → %s", r.phase, to)
}

// Run executes the complete training lifecycle: collect → prepare → train → output → destroy.
// Returns the final TrainingAttestation or an error. On any error, dataset memory
// is destroyed before returning (defense in depth).
func (r *Runtime) Run() (*TrainingAttestation, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	var runErr error
	defer func() {
		// Defense in depth: always attempt destruction on exit, even on success.
		// On crash, the OS frees memory automatically.
		if r.dataset != nil {
			destroyer := &Destroyer{}
			_ = destroyer.DestroyDataset(r.dataset)
			r.dataset = nil
		}
	}()

	// Phase 1: COLLECT
	if err := r.transition(PhaseCollect); err != nil {
		return nil, err
	}
	collected, err := r.runCollect()
	if err != nil {
		runErr = fmt.Errorf("collect phase: %w", err)
		_ = r.transition(PhaseFailed)
		return nil, runErr
	}

	// Phase 2: PREPARE
	if err := r.transition(PhasePrepare); err != nil {
		return nil, err
	}
	dataset, err := r.runPrepare(collected)
	if err != nil {
		runErr = fmt.Errorf("prepare phase: %w", err)
		_ = r.transition(PhaseFailed)
		return nil, runErr
	}
	r.dataset = dataset

	// Phase 3: TRAIN
	if err := r.transition(PhaseTrain); err != nil {
		return nil, err
	}
	result, err := r.runTrain(dataset)
	if err != nil {
		runErr = fmt.Errorf("train phase: %w", err)
		_ = r.transition(PhaseFailed)
		return nil, runErr
	}
	r.trainingResult = result

	// Phase 4: OUTPUT
	if err := r.transition(PhaseOutput); err != nil {
		return nil, err
	}
	attestation, err := r.runOutput(dataset, result)
	if err != nil {
		runErr = fmt.Errorf("output phase: %w", err)
		_ = r.transition(PhaseFailed)
		return nil, runErr
	}

	// Phase 5: DESTROY
	if err := r.transition(PhaseDestroy); err != nil {
		return nil, err
	}
	if err := r.runDestroy(dataset, result, attestation); err != nil {
		runErr = fmt.Errorf("destroy phase: %w", err)
		_ = r.transition(PhaseFailed)
		return nil, runErr
	}

	if err := r.transition(PhaseDone); err != nil {
		return nil, err
	}

	r.attestation = attestation
	return attestation, nil
}

// runCollect authenticates with validators, collects encrypted shards,
// decrypts in enclave memory, and verifies integrity.
func (r *Runtime) runCollect() ([]TDURecord, error) {
	if r.config.Assignments == nil || len(r.config.Assignments) == 0 {
		return nil, errors.New("no shard assignments provided")
	}

	// In the first implementation, we simulate collection by accepting
	// pre-loaded TDU data. In production, this calls the shard-collector
	// service to fetch encrypted shards from validators.
	var records []TDURecord
	for _, tduIDs := range r.config.Assignments {
		for _, id := range tduIDs {
			records = append(records, TDURecord{
				ID:           id,
				Content:      []byte("simulated-content-" + id),
				Domain:       r.config.TargetDomain,
				FitnessScore: 0.5, // default Active — trainable
			})
		}
	}

	if len(records) == 0 {
		return nil, errors.New("no TDUs collected")
	}

	return records, nil
}

// runPrepare reassembles the complete dataset, applies fitness filter and domain mix.
func (r *Runtime) runPrepare(records []TDURecord) (*PreparedDataset, error) {
	prep := &Preparator{
		TargetDomain:   r.config.TargetDomain,
		DomainMixRatio: r.config.DomainMixRatio,
	}
	return prep.Prepare(records)
}

// runTrain executes fine-tuning on the prepared dataset.
func (r *Runtime) runTrain(dataset *PreparedDataset) (*TrainingResult, error) {
	trainer := &Trainer{
		BaseModel:  r.config.BaseModel,
		LoRAConfig: r.config.LoRAConfig,
	}
	return trainer.Train(dataset)
}

// runOutput exports model weights, metrics, and generates the attestation.
func (r *Runtime) runOutput(dataset *PreparedDataset, result *TrainingResult) (*TrainingAttestation, error) {
	outputter := &Outputter{
		EnclaveID:  r.config.EnclaveID,
		EnclaveKey: r.config.EnclaveKey,
	}

	signed, err := outputter.SignOutputs(result)
	if err != nil {
		return nil, err
	}

	gen := &AttestationGenerator{
		EnclaveID:  r.config.EnclaveID,
		EnclaveKey: r.config.EnclaveKey,
	}

	return gen.Generate(dataset, result, signed, r.phaseStart)
}

// runDestroy securely zeroes all dataset memory, intermediate state, and verifies.
func (r *Runtime) runDestroy(dataset *PreparedDataset, result *TrainingResult, attestation *TrainingAttestation) error {
	destroyer := &Destroyer{}

	if err := destroyer.DestroyDataset(dataset); err != nil {
		return fmt.Errorf("dataset destruction: %w", err)
	}

	if err := destroyer.DestroyTrainingState(result); err != nil {
		return fmt.Errorf("training state destruction: %w", err)
	}

	proof, err := destroyer.VerifyDestruction(dataset, result)
	if err != nil {
		return fmt.Errorf("destruction verification: %w", err)
	}

	attestation.DestructionProof = proof
	return nil
}
