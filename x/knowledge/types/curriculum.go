package types

import (
	sdkmath "cosmossdk.io/math"
)

// ─── Curriculum Training Types (R49) ────────────────────────────────────────
//
// Curriculum training orders TDUs into structured learning sequences.
// Instead of random sampling, agents train on data in pedagogical order:
// foundations first, then intermediate, then advanced.
//
// Built on top of the knowledge graph's "prerequisite" edges.
// A curriculum is a DAG (directed acyclic graph) of TDUs where edges
// represent "must learn A before B" relationships.

// CurriculumStatus tracks the lifecycle of a curriculum.
type CurriculumStatus string

const (
	CurriculumStatusDraft    CurriculumStatus = "draft"    // being constructed
	CurriculumStatusActive   CurriculumStatus = "active"   // ready for training
	CurriculumStatusArchived CurriculumStatus = "archived" // superseded
)

// Curriculum represents an ordered training sequence for a domain.
type Curriculum struct {
	CurriculumID string           `json:"curriculum_id"`
	Name         string           `json:"name"`
	Domain       string           `json:"domain"`
	Description  string           `json:"description,omitempty"`
	Status       CurriculumStatus `json:"status"`
	Creator      string           `json:"creator"` // address or "protocol"
	Version      uint64           `json:"version"`
	CreatedAt    int64            `json:"created_at"`   // block height
	UpdatedAt    int64            `json:"updated_at"`   // block height

	// Structure: ordered stages containing TDU groups.
	Stages []CurriculumStage `json:"stages"`

	// Quality metrics.
	TotalTDUs      uint64 `json:"total_tdus"`
	AvgFitness     string `json:"avg_fitness,omitempty"`     // average fitness of included TDUs
	CompletionRate string `json:"completion_rate,omitempty"` // % of agents that completed all stages
}

// CurriculumStage is a level in the curriculum (e.g., "foundations", "intermediate").
type CurriculumStage struct {
	StageID      string   `json:"stage_id"`
	Name         string   `json:"name"`
	Order        uint64   `json:"order"`               // sequence position (0-based)
	TDUIDs       []string `json:"tdu_ids"`              // TDUs in this stage
	MinFitness   string   `json:"min_fitness,omitempty"` // minimum fitness to include a TDU
	Prerequisites []string `json:"prerequisites,omitempty"` // stage IDs that must be completed first
}

// CurriculumEnrollment tracks an agent's progress through a curriculum.
type CurriculumEnrollment struct {
	EnrollmentID string `json:"enrollment_id"`
	CurriculumID string `json:"curriculum_id"`
	AgentID      string `json:"agent_id"`
	EnrolledAt   int64  `json:"enrolled_at"` // block height

	// Progress.
	CurrentStage    uint64   `json:"current_stage"`     // index into Stages
	CompletedStages []string `json:"completed_stages"`  // stage IDs completed
	CompletedTDUs   []string `json:"completed_tdus"`    // TDU IDs consumed
	TotalConsumed   uint64   `json:"total_consumed"`

	// Quality tracking.
	AvgScore string `json:"avg_score,omitempty"` // average score on completed TDUs
	Status   string `json:"status"`              // "active", "completed", "dropped"
}

// GetAvgFitness parses the average fitness score.
func (c *Curriculum) GetAvgFitness() sdkmath.LegacyDec {
	if c.AvgFitness == "" {
		return sdkmath.LegacyZeroDec()
	}
	d, err := sdkmath.LegacyNewDecFromStr(c.AvgFitness)
	if err != nil {
		return sdkmath.LegacyZeroDec()
	}
	return d
}

// ─── Events ─────────────────────────────────────────────────────────────────

const (
	EventCurriculumCreated  = "curriculum_created"
	EventCurriculumUpdated  = "curriculum_updated"
	EventCurriculumArchived = "curriculum_archived"
	EventAgentEnrolled      = "agent_enrolled"
	EventStageCompleted     = "curriculum_stage_completed"
	EventCurriculumComplete = "curriculum_completed"

	AttributeCurriculumID = "curriculum_id"
	AttributeStageID      = "stage_id"
	AttributeEnrollmentID = "enrollment_id"
)
