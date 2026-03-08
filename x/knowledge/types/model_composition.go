package types

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"

	sdkmath "cosmossdk.io/math"
)

// ─── R56: Model Composition — Ensemble Registry ─────────────────────────────
//
// A single specialized model can be great in its domain but blind elsewhere.
// Composing models into ensembles creates capabilities greater than the sum
// of parts. The ensemble's output can then be distilled into a new, more
// capable single model.
//
// The cycle:
//   Specialized models exist in different domains
//   → compose into ensemble (mixture of experts)
//   → ensemble performs verification across domains
//   → ensemble output used as training signal
//   → distill ensemble knowledge into a new single model
//   → new model matches ensemble but runs faster/cheaper
//   → promote to agent → curate across domains → GOTO 1
//
// Key insight: composition is a coordination layer (on-chain),
// not a computation layer. Routing decisions are on-chain and verifiable.
// Actual inference is off-chain in TEE.

// ─── Routing Types ──────────────────────────────────────────────────────────

type RoutingType string

const (
	RoutingDomain     RoutingType = "domain"     // route by knowledge domain
	RoutingConfidence RoutingType = "confidence"  // route to most confident model
	RoutingVoting     RoutingType = "voting"      // all vote, weighted consensus
	RoutingCascade    RoutingType = "cascade"     // try best first, fallback
)

var ValidRoutingTypes = map[RoutingType]bool{
	RoutingDomain:     true,
	RoutingConfidence: true,
	RoutingVoting:     true,
	RoutingCascade:    true,
}

// ─── Model Ensemble ─────────────────────────────────────────────────────────

// ModelEnsemble combines multiple specialized models into one composite.
type ModelEnsemble struct {
	EnsembleID string `json:"ensemble_id"`
	Name       string `json:"name"`

	// Component models and routing.
	Components  []EnsembleComponent `json:"components"`
	RoutingType RoutingType         `json:"routing_type"`

	// Performance.
	BenchmarkScore string   `json:"benchmark_score"` // composite benchmark
	Domains        []string `json:"domains"`          // union of component domains

	// Lifecycle.
	CreatedAt uint64 `json:"created_at"`
	Creator   string `json:"creator"` // address or agent ID
	Status    string `json:"status"`  // draft | active | distilling | retired

	// Distillation output.
	DistilledModelID string `json:"distilled_model_id"` // empty until distillation
	DistillationJob  string `json:"distillation_job"`   // job ID if in progress

	// Usage stats.
	TotalQueries    uint64 `json:"total_queries"`
	TotalRoutings   uint64 `json:"total_routings"`
	AvgResponseCost string `json:"avg_response_cost"` // uzrn
}

func (e ModelEnsemble) Marshal() ([]byte, error)  { return json.Marshal(e) }
func (e *ModelEnsemble) Unmarshal(bz []byte) error { return json.Unmarshal(bz, e) }

// GetBenchmarkScore parses the composite benchmark.
func (e *ModelEnsemble) GetBenchmarkScore() sdkmath.LegacyDec {
	if e.BenchmarkScore == "" {
		return sdkmath.LegacyZeroDec()
	}
	d, err := sdkmath.LegacyNewDecFromStr(e.BenchmarkScore)
	if err != nil {
		return sdkmath.LegacyZeroDec()
	}
	return d
}

// ─── Ensemble Component ─────────────────────────────────────────────────────

// EnsembleComponent is a model within an ensemble.
type EnsembleComponent struct {
	ModelID string `json:"model_id"`
	Domain  string `json:"domain"`
	Weight  string `json:"weight"`  // routing weight [0, 1]
	AgentID string `json:"agent_id"` // backing agent (if promoted)

	// Performance within the ensemble.
	QueriesHandled uint64 `json:"queries_handled"`
	AvgLatency     string `json:"avg_latency"`    // ms
	AccuracyRate   string `json:"accuracy_rate"`   // alignment with ensemble consensus
}

// GetWeight parses the routing weight.
func (c *EnsembleComponent) GetWeight() sdkmath.LegacyDec {
	if c.Weight == "" {
		return sdkmath.LegacyOneDec()
	}
	d, err := sdkmath.LegacyNewDecFromStr(c.Weight)
	if err != nil {
		return sdkmath.LegacyOneDec()
	}
	return d
}

// ─── Distillation Job ───────────────────────────────────────────────────────

// DistillationJob extracts an ensemble's collective knowledge into a single model.
// The distilled model learns to mimic the ensemble's routing decisions and
// component responses, producing a more capable generalist.
type DistillationJob struct {
	JobID      string `json:"job_id"`
	EnsembleID string `json:"ensemble_id"`

	// Training data: ensemble's collective verification captures.
	CaptureCount uint64 `json:"capture_count"`
	DomainsCovered []string `json:"domains_covered"`

	// Output.
	OutputModelID string `json:"output_model_id"` // empty until complete

	// Lifecycle.
	Status  string `json:"status"` // pending | training | complete | failed
	StartAt uint64 `json:"start_at"`
	EndAt   uint64 `json:"end_at"`

	// Quality gate: distilled model must meet this benchmark.
	MinBenchmark string `json:"min_benchmark"`
}

func (d DistillationJob) Marshal() ([]byte, error)  { return json.Marshal(d) }
func (d *DistillationJob) Unmarshal(bz []byte) error { return json.Unmarshal(bz, d) }

// ─── Routing Decision ───────────────────────────────────────────────────────

// RoutingDecision records which component was selected for a query.
// Stored for audit and training signal generation.
type RoutingDecision struct {
	DecisionID    string `json:"decision_id"`
	EnsembleID    string `json:"ensemble_id"`
	SelectedModel string `json:"selected_model"` // which component handled it
	Domain        string `json:"domain"`          // query domain
	Confidence    string `json:"confidence"`      // routing confidence
	BlockHeight   uint64 `json:"block_height"`
}

func (r RoutingDecision) Marshal() ([]byte, error)  { return json.Marshal(r) }
func (r *RoutingDecision) Unmarshal(bz []byte) error { return json.Unmarshal(bz, r) }

// ─── Composition Parameters ─────────────────────────────────────────────────

type CompositionParams struct {
	MinComponentsForEnsemble uint64 `json:"min_components"`          // default: 2
	MaxComponentsPerEnsemble uint64 `json:"max_components"`          // default: 10
	MinBenchmarkForComponent string `json:"min_benchmark"`           // default: 0.5
	DistillationMinCaptures  uint64 `json:"distillation_min_captures"` // default: 1000
	DistillationMinBenchmark string `json:"distillation_min_benchmark"` // default: 0.6
}

func DefaultCompositionParams() CompositionParams {
	return CompositionParams{
		MinComponentsForEnsemble: 2,
		MaxComponentsPerEnsemble: 10,
		MinBenchmarkForComponent: "0.500000000000000000",
		DistillationMinCaptures:  1000,
		DistillationMinBenchmark: "0.600000000000000000",
	}
}

func (p CompositionParams) Validate() error {
	if p.MinComponentsForEnsemble < 2 {
		return fmt.Errorf("min_components must be >= 2")
	}
	if p.MaxComponentsPerEnsemble < p.MinComponentsForEnsemble {
		return fmt.Errorf("max_components must be >= min_components")
	}
	return nil
}

func (p CompositionParams) Marshal() ([]byte, error)  { return json.Marshal(p) }
func (p *CompositionParams) Unmarshal(bz []byte) error { return json.Unmarshal(bz, p) }

// ─── Ensemble ID Derivation ─────────────────────────────────────────────────

func DeriveEnsembleID(name string, components []string) string {
	input := "ensemble:" + name
	for _, c := range components {
		input += ":" + c
	}
	hash := sha256.Sum256([]byte(input))
	return fmt.Sprintf("%x", hash[:16])
}

// ─── Store Keys ─────────────────────────────────────────────────────────────

var (
	ModelEnsemblePrefix     = []byte("composition/ensemble/")
	EnsembleByDomainPrefix  = []byte("composition/by-domain/")
	DistillationJobPrefix   = []byte("composition/distill/")
	RoutingDecisionPrefix   = []byte("composition/routing/")
	CompositionParamsKey    = []byte("composition/params")
	EnsembleSeqKey          = []byte("composition/seq")
	DistillationSeqKey      = []byte("composition/distill-seq")
	RoutingDecisionSeqKey   = []byte("composition/routing-seq")
)

func ModelEnsembleKey(ensembleID string) []byte {
	return append(ModelEnsemblePrefix, []byte(ensembleID)...)
}

func EnsembleByDomainKey(domain, ensembleID string) []byte {
	return append(EnsembleByDomainPrefix, []byte(domain+"/"+ensembleID)...)
}

func EnsembleByDomainPfx(domain string) []byte {
	return append(EnsembleByDomainPrefix, []byte(domain+"/")...)
}

func DistillationJobKey(jobID string) []byte {
	return append(DistillationJobPrefix, []byte(jobID)...)
}

func RoutingDecisionKey(decisionID string) []byte {
	return append(RoutingDecisionPrefix, []byte(decisionID)...)
}

// ─── Events ─────────────────────────────────────────────────────────────────

const (
	EventEnsembleCreated       = "ensemble_created"
	EventEnsembleActivated     = "ensemble_activated"
	EventEnsembleRetired       = "ensemble_retired"
	EventDistillationStarted   = "distillation_started"
	EventDistillationCompleted = "distillation_completed"
	EventRoutingDecision       = "routing_decision"

	AttributeEnsembleID    = "ensemble_id"
	AttributeRoutingType   = "routing_type"
	AttributeDistillJobID  = "distillation_job_id"
	AttributeSelectedModel = "selected_model"
	AttributeComponentCount = "component_count"
)
