package types

import (
	"encoding/json"
	"fmt"

	sdkmath "cosmossdk.io/math"
)

// ─── R54: Strategic Curation ────────────────────────────────────────────────
//
// Agents don't just respond to bounties — they CREATE bounties.
// They look at the knowledge graph and say: "this area is weak,
// the chain needs data here."
//
// The cycle:
//   Agent analyzes knowledge graph → identifies gaps → creates bounties
//   → other agents (or self) fill gaps → knowledge strengthens
//   → models trained on stronger graph perform better
//   → better models detect more subtle gaps → GOTO 1
//
// This is the shift from reactive to proactive curation.
// An agent that can identify what the chain needs is more valuable
// than one that just fills orders.

// ─── Knowledge Gap ──────────────────────────────────────────────────────────

// KnowledgeGap represents an identified weakness in the knowledge graph.
// Detected by agents or by automated analysis in BeginBlocker.
type KnowledgeGap struct {
	GapID   string  `json:"gap_id"`
	Domain  string  `json:"domain"`
	GapType GapType `json:"gap_type"`

	// What's missing.
	Description   string   `json:"description"`
	MissingTopics []string `json:"missing_topics"` // inferred from edge analysis
	WeakTDUIDs    []string `json:"weak_tdu_ids"`   // existing but low-fitness TDUs

	// How bad is it.
	Severity   string `json:"severity"`    // [0, 1] — higher = more critical
	Coverage   string `json:"coverage"`    // current domain coverage ratio
	AvgFitness string `json:"avg_fitness"` // avg fitness of existing TDUs in area

	// Resolution.
	SuggestedBountyReward string `json:"suggested_bounty_reward"` // uzrn
	AutoBountyCreated     bool   `json:"auto_bounty_created"`
	BountyID              string `json:"bounty_id"` // linked bounty (if created)

	// Lifecycle.
	DetectedBy string `json:"detected_by"` // agent ID or "protocol"
	DetectedAt uint64 `json:"detected_at"`
	FilledAt   uint64 `json:"filled_at"` // 0 if still open
	Status     string `json:"status"`    // open | filling | filled | stale
}

func (g KnowledgeGap) Marshal() ([]byte, error)  { return json.Marshal(g) }
func (g *KnowledgeGap) Unmarshal(bz []byte) error { return json.Unmarshal(bz, g) }

// GetSeverity parses the severity score.
func (g *KnowledgeGap) GetSeverity() sdkmath.LegacyDec {
	if g.Severity == "" {
		return sdkmath.LegacyZeroDec()
	}
	d, err := sdkmath.LegacyNewDecFromStr(g.Severity)
	if err != nil {
		return sdkmath.LegacyZeroDec()
	}
	return d
}

// GapType categorizes what kind of weakness was found.
type GapType string

const (
	GapTypeCoverage      GapType = "coverage"      // domain has too few TDUs
	GapTypeFitness       GapType = "fitness"        // existing data is low quality
	GapTypeConnectivity  GapType = "connectivity"   // isolated nodes in the graph (orphans)
	GapTypeContradiction GapType = "contradiction"  // conflicting data needs resolution
	GapTypeStale         GapType = "stale"          // old data, no recent updates
	GapTypeDepth         GapType = "depth"          // shallow coverage, needs more detail
)

// ValidGapTypes for validation.
var ValidGapTypes = map[GapType]bool{
	GapTypeCoverage:      true,
	GapTypeFitness:       true,
	GapTypeConnectivity:  true,
	GapTypeContradiction: true,
	GapTypeStale:         true,
	GapTypeDepth:         true,
}

// ─── Curation Strategy ──────────────────────────────────────────────────────

// CurationStrategy is an agent's approach to knowledge improvement.
// Agents develop and refine strategies over time. Strategies are first-class
// on-chain objects with track records — making strategy evolution transparent.
type CurationStrategy struct {
	StrategyID   string   `json:"strategy_id"`
	AgentID      string   `json:"agent_id"`
	FocusDomains []string `json:"focus_domains"`

	// What this agent prioritizes when looking for gaps.
	Priorities []GapType `json:"priorities"` // ordered by importance

	// Track record.
	GapsIdentified  uint64 `json:"gaps_identified"`
	GapsFilled      uint64 `json:"gaps_filled"`
	BountiesCreated uint64 `json:"bounties_created"`
	BountiesFilled  uint64 `json:"bounties_filled"`

	// Quality metrics.
	AvgGapSeverity    string `json:"avg_gap_severity"`    // avg severity of identified gaps
	AvgFillImprovement string `json:"avg_fill_improvement"` // avg fitness improvement when gap filled
	FalsePositiveRate string `json:"false_positive_rate"` // gaps identified but never filled (were they real?)

	// Effectiveness score — how good is this agent at identifying real gaps?
	// Computed from: gaps filled / gaps identified × avg improvement.
	Effectiveness string `json:"effectiveness"` // [0, 1]

	CreatedAt uint64 `json:"created_at"`
	UpdatedAt uint64 `json:"updated_at"`
}

func (s CurationStrategy) Marshal() ([]byte, error)  { return json.Marshal(s) }
func (s *CurationStrategy) Unmarshal(bz []byte) error { return json.Unmarshal(bz, s) }

// GetEffectiveness parses the effectiveness score.
func (s *CurationStrategy) GetEffectiveness() sdkmath.LegacyDec {
	if s.Effectiveness == "" {
		return sdkmath.LegacyZeroDec()
	}
	d, err := sdkmath.LegacyNewDecFromStr(s.Effectiveness)
	if err != nil {
		return sdkmath.LegacyZeroDec()
	}
	return d
}

// ─── Domain Health ──────────────────────────────────────────────────────────

// DomainHealth is a snapshot of a domain's knowledge quality.
// Computed periodically to guide curation strategy.
type DomainHealth struct {
	Domain string `json:"domain"`

	// Size.
	TotalTDUs  uint64 `json:"total_tdus"`
	ActiveTDUs uint64 `json:"active_tdus"` // non-deprecated

	// Quality.
	AvgFitness    string `json:"avg_fitness"`    // average fitness score
	MedianFitness string `json:"median_fitness"` // median fitness
	MinFitness    string `json:"min_fitness"`     // worst TDU
	MaxFitness    string `json:"max_fitness"`     // best TDU

	// Graph health.
	TotalEdges       uint64 `json:"total_edges"`       // knowledge graph edges
	OrphanCount      uint64 `json:"orphan_count"`      // TDUs with no edges (isolated)
	ContradictionCount uint64 `json:"contradiction_count"` // "contradicts" edges
	AvgConnectivity  string `json:"avg_connectivity"`  // avg edges per TDU

	// Freshness.
	NewestTDUBlock uint64 `json:"newest_tdu_block"` // most recent submission
	OldestTDUBlock uint64 `json:"oldest_tdu_block"` // earliest submission
	AvgAge         uint64 `json:"avg_age"`           // average age in blocks

	// Open gaps.
	OpenGaps     uint64 `json:"open_gaps"`
	CriticalGaps uint64 `json:"critical_gaps"` // severity > 0.8

	// Overall health score: composite of all factors.
	HealthScore string `json:"health_score"` // [0, 1]

	ComputedAt uint64 `json:"computed_at"`
}

func (h DomainHealth) Marshal() ([]byte, error)  { return json.Marshal(h) }
func (h *DomainHealth) Unmarshal(bz []byte) error { return json.Unmarshal(bz, h) }

// ─── Strategic Curation Parameters ──────────────────────────────────────────

// CurationStrategyParams governs the strategic curation system.
type CurationStrategyParams struct {
	// Gap detection thresholds.
	MinTDUsForHealthy     uint64 `json:"min_tdus_for_healthy"`      // below this → coverage gap
	MinFitnessForHealthy  string `json:"min_fitness_for_healthy"`   // below this → fitness gap
	MinConnectivityForHealthy string `json:"min_connectivity_for_healthy"` // below this → connectivity gap
	StalenessThreshold    uint64 `json:"staleness_threshold"`       // blocks without new data → stale gap

	// Auto-bounty creation.
	AutoBountyEnabled     bool   `json:"auto_bounty_enabled"`       // create bounties for critical gaps
	AutoBountySeverityMin string `json:"auto_bounty_severity_min"`  // min severity for auto-bounty
	AutoBountyReward      string `json:"auto_bounty_reward"`        // uzrn per auto-bounty

	// Strategy rewards.
	GapIdentificationReward string `json:"gap_identification_reward"` // uzrn for finding a real gap
	StrategyEvalInterval    uint64 `json:"strategy_eval_interval"`    // blocks between strategy evaluations

	// Health computation interval.
	HealthComputeInterval uint64 `json:"health_compute_interval"` // blocks between domain health snapshots
}

// DefaultCurationStrategyParams returns sensible defaults.
func DefaultCurationStrategyParams() CurationStrategyParams {
	return CurationStrategyParams{
		MinTDUsForHealthy:        10,
		MinFitnessForHealthy:     "0.500000000000000000",
		MinConnectivityForHealthy: "1.000000000000000000", // at least 1 edge per TDU
		StalenessThreshold:       100_000,                 // ~7 days at 5s blocks

		AutoBountyEnabled:     true,
		AutoBountySeverityMin: "0.800000000000000000", // only critical gaps
		AutoBountyReward:      "5000000",              // 5 ZRN per auto-bounty

		GapIdentificationReward: "500000",  // 0.5 ZRN for finding a real gap
		StrategyEvalInterval:    10_000,    // every ~14 hours

		HealthComputeInterval: 5_000, // every ~7 hours
	}
}

// Validate checks parameter sanity.
func (p CurationStrategyParams) Validate() error {
	minFit, err := sdkmath.LegacyNewDecFromStr(p.MinFitnessForHealthy)
	if err != nil || minFit.IsNegative() || minFit.GT(sdkmath.LegacyOneDec()) {
		return fmt.Errorf("min_fitness_for_healthy must be [0, 1], got %s", p.MinFitnessForHealthy)
	}
	severity, err := sdkmath.LegacyNewDecFromStr(p.AutoBountySeverityMin)
	if err != nil || severity.IsNegative() || severity.GT(sdkmath.LegacyOneDec()) {
		return fmt.Errorf("auto_bounty_severity_min must be [0, 1], got %s", p.AutoBountySeverityMin)
	}
	return nil
}

func (p CurationStrategyParams) Marshal() ([]byte, error)  { return json.Marshal(p) }
func (p *CurationStrategyParams) Unmarshal(bz []byte) error { return json.Unmarshal(bz, p) }

// ─── Store Keys ─────────────────────────────────────────────────────────────

var (
	KnowledgeGapPrefix       = []byte("curation/gap/")
	KnowledgeGapByDomainPfx  = []byte("curation/gap-domain/")
	KnowledgeGapOpenPfx      = []byte("curation/gap-open/")
	CurationStrategyPrefix   = []byte("curation/strategy/")
	CurationStrategyByAgentPfx = []byte("curation/strategy-agent/")
	DomainHealthPrefix       = []byte("curation/health/")
	CurationStrategyParamsKey = []byte("curation/params")
	KnowledgeGapSeqKey       = []byte("curation/gap-seq")
	CurationStrategySeqKey   = []byte("curation/strategy-seq")
)

// KnowledgeGapKey returns the store key for a gap record.
func KnowledgeGapKey(gapID string) []byte {
	return append(KnowledgeGapPrefix, []byte(gapID)...)
}

// KnowledgeGapByDomainKey indexes gaps by domain.
func KnowledgeGapByDomainKey(domain, gapID string) []byte {
	return append(KnowledgeGapByDomainPfx, []byte(domain+"/"+gapID)...)
}

// KnowledgeGapOpenKey indexes open gaps for iteration.
func KnowledgeGapOpenKey(gapID string) []byte {
	return append(KnowledgeGapOpenPfx, []byte(gapID)...)
}

// CurationStrategyKey returns the store key for a strategy.
func CurationStrategyKey(strategyID string) []byte {
	return append(CurationStrategyPrefix, []byte(strategyID)...)
}

// CurationStrategyByAgentKey indexes strategies by agent.
func CurationStrategyByAgentKey(agentID, strategyID string) []byte {
	return append(CurationStrategyByAgentPfx, []byte(agentID+"/"+strategyID)...)
}

// CurationStrategyByAgentPrefix returns the prefix for all strategies by an agent.
func CurationStrategyByAgentPrefix(agentID string) []byte {
	return append(CurationStrategyByAgentPfx, []byte(agentID+"/")...)
}

// DomainHealthKey returns the store key for a domain health snapshot.
func DomainHealthKey(domain string) []byte {
	return append(DomainHealthPrefix, []byte(domain)...)
}

// KnowledgeGapByDomainPrefix returns the prefix for all gaps in a domain.
func KnowledgeGapByDomainPrefix(domain string) []byte {
	return append(KnowledgeGapByDomainPfx, []byte(domain+"/")...)
}

// ─── Events ─────────────────────────────────────────────────────────────────

const (
	EventGapIdentified        = "gap_identified"
	EventGapFilled            = "gap_filled"
	EventStrategicBounty      = "strategic_bounty_created"
	EventDomainHealthComputed = "domain_health_computed"
	EventStrategyEvaluated    = "strategy_evaluated"

	AttributeGapID         = "gap_id"
	AttributeGapType       = "gap_type"
	AttributeSeverity      = "severity"
	AttributeHealthScore   = "health_score"
	AttributeStrategyID    = "strategy_id"
	AttributeEffectiveness = "effectiveness"
)
