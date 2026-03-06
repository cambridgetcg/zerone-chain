package types

import (
	"fmt"

	sdkmath "cosmossdk.io/math"
)

// ─── Edge Types ─────────────────────────────────────────────────────────────

// EdgeType defines the semantic relationship between two TDUs.
type EdgeType string

const (
	// Structural edges — how knowledge is organized.
	EdgeTypeReferences  EdgeType = "references"  // A cites/uses B
	EdgeTypeExtends     EdgeType = "extends"     // A builds upon B (deeper treatment)
	EdgeTypeExemplifies EdgeType = "exemplifies" // A is a concrete example of B

	// Corrective edges — knowledge evolution.
	EdgeTypeCorrects    EdgeType = "corrects"    // A fixes errors in B
	EdgeTypeSupersedes  EdgeType = "supersedes"  // A replaces B (updated version)
	EdgeTypeContradicts EdgeType = "contradicts" // A conflicts with B

	// Compositional edges — knowledge structure.
	EdgeTypePartOf      EdgeType = "part_of"     // A is a component of B
	EdgeTypePrerequisite EdgeType = "prerequisite" // A must be understood before B
	EdgeTypeSimilar     EdgeType = "similar"     // A covers similar topic as B (for dedup/clustering)
)

// ValidEdgeTypes is the set of recognized edge types.
var ValidEdgeTypes = map[EdgeType]bool{
	EdgeTypeReferences:   true,
	EdgeTypeExtends:      true,
	EdgeTypeExemplifies:  true,
	EdgeTypeCorrects:     true,
	EdgeTypeSupersedes:   true,
	EdgeTypeContradicts:  true,
	EdgeTypePartOf:       true,
	EdgeTypePrerequisite: true,
	EdgeTypeSimilar:      true,
}

// ─── Knowledge Edge ─────────────────────────────────────────────────────────

// KnowledgeEdge represents a directed semantic link between two TDUs.
// Stored as JSON under KnowledgeEdgePrefix.
type KnowledgeEdge struct {
	EdgeID    string   `json:"edge_id"`    // deterministic: sha256(source:target:type)
	SourceID  string   `json:"source_id"`  // source TDU sample ID
	TargetID  string   `json:"target_id"`  // target TDU sample ID
	EdgeType  EdgeType `json:"edge_type"`  // semantic relationship
	Weight    string   `json:"weight"`     // sdkmath.LegacyDec [0, 1] — strength
	Creator   string   `json:"creator"`    // address that created the edge
	CreatedAt int64    `json:"created_at"` // block height
	Verified  bool     `json:"verified"`   // confirmed by review
}

// GetWeight parses the edge weight.
func (e *KnowledgeEdge) GetWeight() sdkmath.LegacyDec {
	if e.Weight == "" {
		return sdkmath.LegacyNewDecWithPrec(5, 1) // default 0.5
	}
	d, err := sdkmath.LegacyNewDecFromStr(e.Weight)
	if err != nil {
		return sdkmath.LegacyNewDecWithPrec(5, 1)
	}
	return d
}

// SetWeight stores the weight, clamped to [0, 1].
func (e *KnowledgeEdge) SetWeight(w sdkmath.LegacyDec) {
	if w.GT(sdkmath.LegacyOneDec()) {
		w = sdkmath.LegacyOneDec()
	}
	if w.LT(sdkmath.LegacyZeroDec()) {
		w = sdkmath.LegacyZeroDec()
	}
	e.Weight = w.String()
}

// ValidateBasic performs stateless validation.
func (e *KnowledgeEdge) ValidateBasic() error {
	if e.SourceID == "" {
		return fmt.Errorf("source ID is required")
	}
	if e.TargetID == "" {
		return fmt.Errorf("target ID is required")
	}
	if e.SourceID == e.TargetID {
		return fmt.Errorf("self-referential edges not allowed")
	}
	if !ValidEdgeTypes[e.EdgeType] {
		return fmt.Errorf("invalid edge type: %s", e.EdgeType)
	}
	if e.Creator == "" {
		return fmt.Errorf("creator is required")
	}
	return nil
}

// ─── Knowledge Cluster ──────────────────────────────────────────────────────

// KnowledgeCluster represents a group of semantically related TDUs.
// Clusters emerge from edge density — TDUs with many connections form natural groups.
type KnowledgeCluster struct {
	ClusterID   string   `json:"cluster_id"`
	Domain      string   `json:"domain"`
	Label       string   `json:"label"`       // human-readable topic label
	MemberIDs   []string `json:"member_ids"`  // TDU sample IDs in this cluster
	CoreMembers []string `json:"core_members"` // high-connectivity hub TDUs
	EdgeCount   uint64   `json:"edge_count"`  // total internal edges
	AvgFitness  string   `json:"avg_fitness"` // sdkmath.LegacyDec
	CreatedAt   int64    `json:"created_at"`
	UpdatedAt   int64    `json:"updated_at"`
}

// GetAvgFitness parses the average fitness.
func (c *KnowledgeCluster) GetAvgFitness() sdkmath.LegacyDec {
	if c.AvgFitness == "" {
		return sdkmath.LegacyZeroDec()
	}
	d, err := sdkmath.LegacyNewDecFromStr(c.AvgFitness)
	if err != nil {
		return sdkmath.LegacyZeroDec()
	}
	return d
}

// ─── Propagation Signal ─────────────────────────────────────────────────────

// PropagationSignal carries a fitness signal along a knowledge edge.
// When a TDU's fitness changes, connected TDUs receive attenuated signals.
type PropagationSignal struct {
	SourceTDU  string              `json:"source_tdu"`
	TargetTDU  string              `json:"target_tdu"`
	EdgeType   EdgeType            `json:"edge_type"`
	Delta      sdkmath.LegacyDec   `json:"delta"`      // fitness change in source
	Attenuated sdkmath.LegacyDec   `json:"attenuated"`  // delta × weight × propagation factor
}

// Propagation parameters.
var (
	// PropagationFactor — how much of a fitness change propagates to neighbors.
	PropagationFactor = sdkmath.LegacyNewDecWithPrec(2, 1) // 0.2 (20%)
	// PropagationMaxDepth — max hops for signal propagation.
	PropagationMaxDepth uint64 = 3
	// PropagationMinSignal — signals below this are dropped.
	PropagationMinSignal = sdkmath.LegacyNewDecWithPrec(1, 2) // 0.01
)

// ─── Messages ───────────────────────────────────────────────────────────────

// MsgCreateEdge creates a semantic link between two TDUs.
type MsgCreateEdge struct {
	Creator  string   `json:"creator"`
	SourceID string   `json:"source_id"`
	TargetID string   `json:"target_id"`
	EdgeType EdgeType `json:"edge_type"`
	Weight   string   `json:"weight"` // optional, defaults to 0.5
}

// ValidateBasic validates the message.
func (msg *MsgCreateEdge) ValidateBasic() error {
	if msg.Creator == "" {
		return ErrUnauthorized.Wrap("creator is required")
	}
	if msg.SourceID == "" || msg.TargetID == "" {
		return fmt.Errorf("source and target IDs are required")
	}
	if msg.SourceID == msg.TargetID {
		return fmt.Errorf("self-referential edges not allowed")
	}
	if !ValidEdgeTypes[msg.EdgeType] {
		return fmt.Errorf("invalid edge type: %s", msg.EdgeType)
	}
	return nil
}

// MsgCreateEdgeResponse is returned after creating an edge.
type MsgCreateEdgeResponse struct {
	EdgeID string `json:"edge_id"`
}

// MsgRemoveEdge removes a semantic link.
type MsgRemoveEdge struct {
	Authority string `json:"authority"`
	EdgeID    string `json:"edge_id"`
}

// ─── Events ─────────────────────────────────────────────────────────────────

const (
	EventEdgeCreated       = "knowledge_edge_created"
	EventEdgeRemoved       = "knowledge_edge_removed"
	EventClusterFormed     = "knowledge_cluster_formed"
	EventFitnessPropagated = "fitness_propagated"

	AttributeEdgeID     = "edge_id"
	AttributeEdgeType   = "edge_type"
	AttributeSourceTDU  = "source_tdu"
	AttributeTargetTDU  = "target_tdu"
	AttributeClusterID  = "cluster_id"
)
