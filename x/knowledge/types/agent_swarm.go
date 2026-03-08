package types

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"

	sdkmath "cosmossdk.io/math"
)

// ─── R55: Agent Swarms — Collective Intelligence ────────────────────────────
//
// One agent working alone is limited by its model's perspective.
// A swarm of agents, each with different models and domains, can
// collectively curate data that no individual could.
//
// The cycle:
//   Agents form swarm → coordinate roles (curator, reviewer, strategist)
//   → collective curation produces higher-quality dataset
//   → swarm trains a model on their collective work
//   → model outperforms any individual member's model
//   → swarm members access better model via API (R51)
//   → improved swarm produces even better data → GOTO 1
//
// The swarm is a higher-order intelligence that emerges from cooperation.

// ─── Swarm ──────────────────────────────────────────────────────────────────

// AgentSwarm is a collective of agents working together in a domain.
type AgentSwarm struct {
	SwarmID string `json:"swarm_id"`
	Name    string `json:"name"`
	Domain  string `json:"domain"`

	// Members.
	Members    []SwarmMember `json:"members"`
	MinMembers uint64        `json:"min_members"` // minimum for quorum
	MaxMembers uint64        `json:"max_members"`

	// Collective performance.
	CollectiveReputation string `json:"collective_reputation"` // avg of members
	TDUsCurated          uint64 `json:"tdus_curated"`
	ModelsProduced       uint64 `json:"models_produced"`
	ObjectivesCompleted  uint64 `json:"objectives_completed"`

	// Shared resources.
	TreasuryBalance string `json:"treasury_balance"` // pooled ZRN
	TreasuryAddr    string `json:"treasury_addr"`    // deterministic from swarm ID

	// Lifecycle.
	FormedAt    uint64      `json:"formed_at"`
	DissolvedAt uint64      `json:"dissolved_at"` // 0 = active
	Status      SwarmStatus `json:"status"`
	Creator     string      `json:"creator"` // agent who formed the swarm
}

func (s AgentSwarm) Marshal() ([]byte, error)  { return json.Marshal(s) }
func (s *AgentSwarm) Unmarshal(bz []byte) error { return json.Unmarshal(bz, s) }

// GetCollectiveReputation parses the collective reputation.
func (s *AgentSwarm) GetCollectiveReputation() sdkmath.LegacyDec {
	if s.CollectiveReputation == "" {
		return sdkmath.LegacyZeroDec()
	}
	d, err := sdkmath.LegacyNewDecFromStr(s.CollectiveReputation)
	if err != nil {
		return sdkmath.LegacyZeroDec()
	}
	return d
}

// MemberCount returns the number of current members.
func (s *AgentSwarm) MemberCount() uint64 {
	return uint64(len(s.Members))
}

// HasMember checks if an agent is already a member.
func (s *AgentSwarm) HasMember(agentID string) bool {
	for _, m := range s.Members {
		if m.AgentID == agentID {
			return true
		}
	}
	return false
}

// SwarmStatus represents the swarm lifecycle state.
type SwarmStatus string

const (
	SwarmStatusForming   SwarmStatus = "forming"   // gathering members
	SwarmStatusActive    SwarmStatus = "active"     // operational
	SwarmStatusDissolved SwarmStatus = "dissolved"  // wound down
)

// ─── Swarm Member ───────────────────────────────────────────────────────────

// SwarmMember is an agent participating in a swarm with a specific role.
type SwarmMember struct {
	AgentID      string    `json:"agent_id"`
	Role         SwarmRole `json:"role"`
	JoinedAt     uint64    `json:"joined_at"`
	Contribution string    `json:"contribution"` // fraction of total work done [0, 1]

	// Performance within swarm.
	TasksCompleted uint64 `json:"tasks_completed"`
	TDUsSubmitted  uint64 `json:"tdus_submitted"`
	ReviewsDone    uint64 `json:"reviews_done"`
	GapsFound      uint64 `json:"gaps_found"`
}

// GetContribution parses the contribution fraction.
func (m *SwarmMember) GetContribution() sdkmath.LegacyDec {
	if m.Contribution == "" {
		return sdkmath.LegacyZeroDec()
	}
	d, err := sdkmath.LegacyNewDecFromStr(m.Contribution)
	if err != nil {
		return sdkmath.LegacyZeroDec()
	}
	return d
}

// SwarmRole defines what an agent does within a swarm.
type SwarmRole string

const (
	SwarmRoleCurator    SwarmRole = "curator"    // finds and submits data
	SwarmRoleReviewer   SwarmRole = "reviewer"   // reviews submissions
	SwarmRoleStrategist SwarmRole = "strategist" // identifies gaps (R54 integration)
	SwarmRoleTrainer    SwarmRole = "trainer"    // initiates model training
)

// ValidSwarmRoles for validation.
var ValidSwarmRoles = map[SwarmRole]bool{
	SwarmRoleCurator:    true,
	SwarmRoleReviewer:   true,
	SwarmRoleStrategist: true,
	SwarmRoleTrainer:    true,
}

// ─── Swarm Objective ────────────────────────────────────────────────────────

// SwarmObjective is a coordinated goal for the swarm.
// Objectives link to knowledge gaps (R54) or bounties (R47).
type SwarmObjective struct {
	ObjectiveID string `json:"objective_id"`
	SwarmID     string `json:"swarm_id"`
	Description string `json:"description"`

	// Target.
	TargetGapID   string `json:"target_gap_id"`   // linked knowledge gap
	TargetBountyID string `json:"target_bounty_id"` // linked bounty
	TargetTDUs    uint64 `json:"target_tdus"`      // how many TDUs needed
	TargetFitness string `json:"target_fitness"`   // minimum fitness goal

	// Progress.
	TDUsSubmitted uint64 `json:"tdus_submitted"`
	AvgFitness    string `json:"avg_fitness"`

	// Deadline and reward.
	Deadline   uint64 `json:"deadline"`
	RewardPool string `json:"reward_pool"` // uzrn pooled for this objective

	Status    string `json:"status"` // active | completed | failed | cancelled
	CreatedAt uint64 `json:"created_at"`
}

func (o SwarmObjective) Marshal() ([]byte, error)  { return json.Marshal(o) }
func (o *SwarmObjective) Unmarshal(bz []byte) error { return json.Unmarshal(bz, o) }

// IsComplete checks if the objective has been met.
func (o *SwarmObjective) IsComplete() bool {
	if o.TDUsSubmitted < o.TargetTDUs {
		return false
	}
	if o.TargetFitness != "" && o.AvgFitness != "" {
		target, _ := sdkmath.LegacyNewDecFromStr(o.TargetFitness)
		actual, _ := sdkmath.LegacyNewDecFromStr(o.AvgFitness)
		return actual.GTE(target)
	}
	return true
}

// ─── Swarm Parameters ───────────────────────────────────────────────────────

// SwarmParams governs the agent swarm system.
type SwarmParams struct {
	MinSwarmSize   uint64 `json:"min_swarm_size"`    // minimum members to activate
	MaxSwarmSize   uint64 `json:"max_swarm_size"`    // maximum members
	FormationStake string `json:"formation_stake"`   // ZRN to form a swarm
	MemberStake    string `json:"member_stake"`      // ZRN to join a swarm
	TreasuryTax    string `json:"treasury_tax"`      // fraction of member earnings to treasury
	MaxObjectives  uint64 `json:"max_objectives"`    // concurrent objectives per swarm
	InactivityLimit uint64 `json:"inactivity_limit"` // blocks before inactive swarm auto-dissolves
}

// DefaultSwarmParams returns sensible defaults.
func DefaultSwarmParams() SwarmParams {
	return SwarmParams{
		MinSwarmSize:    2,                            // at least 2 agents
		MaxSwarmSize:    21,                           // like a validator set
		FormationStake:  "5000000",                    // 5 ZRN to create
		MemberStake:     "1000000",                    // 1 ZRN to join
		TreasuryTax:     "0.050000000000000000",       // 5% of member earnings
		MaxObjectives:   5,                            // concurrent goals
		InactivityLimit: 50_000,                       // ~3.5 days
	}
}

// Validate checks parameter sanity.
func (p SwarmParams) Validate() error {
	if p.MinSwarmSize == 0 {
		return fmt.Errorf("min_swarm_size must be > 0")
	}
	if p.MaxSwarmSize < p.MinSwarmSize {
		return fmt.Errorf("max_swarm_size must be >= min_swarm_size")
	}
	tax, err := sdkmath.LegacyNewDecFromStr(p.TreasuryTax)
	if err != nil || tax.IsNegative() || tax.GT(sdkmath.LegacyOneDec()) {
		return fmt.Errorf("treasury_tax must be [0, 1], got %s", p.TreasuryTax)
	}
	return nil
}

func (p SwarmParams) Marshal() ([]byte, error)  { return json.Marshal(p) }
func (p *SwarmParams) Unmarshal(bz []byte) error { return json.Unmarshal(bz, p) }

// ─── Deterministic Addresses ────────────────────────────────────────────────

// DeriveSwarmTreasury generates a deterministic treasury address from swarm ID.
func DeriveSwarmTreasury(swarmID string) string {
	input := "zerone-swarm-treasury:" + swarmID
	hash := sha256.Sum256([]byte(input))
	return fmt.Sprintf("%x", hash[:20])
}

// ─── Store Keys ─────────────────────────────────────────────────────────────

var (
	AgentSwarmPrefix        = []byte("swarm/swarm/")
	SwarmByDomainPfx        = []byte("swarm/by-domain/")
	SwarmByMemberPfx        = []byte("swarm/by-member/")
	SwarmActiveIdxPfx       = []byte("swarm/active/")
	SwarmObjectivePrefix    = []byte("swarm/objective/")
	SwarmObjBySwarmPfx      = []byte("swarm/obj-by-swarm/")
	SwarmParamsKey          = []byte("swarm/params")
	SwarmSeqKey             = []byte("swarm/seq")
	SwarmObjectiveSeqKey    = []byte("swarm/obj-seq")
)

// AgentSwarmKey returns the store key for a swarm.
func AgentSwarmKey(swarmID string) []byte {
	return append(AgentSwarmPrefix, []byte(swarmID)...)
}

// SwarmByDomainKey indexes swarms by domain.
func SwarmByDomainKey(domain, swarmID string) []byte {
	return append(SwarmByDomainPfx, []byte(domain+"/"+swarmID)...)
}

// SwarmByMemberKey indexes which swarms an agent belongs to.
func SwarmByMemberKey(agentID, swarmID string) []byte {
	return append(SwarmByMemberPfx, []byte(agentID+"/"+swarmID)...)
}

// SwarmByMemberPrefix returns prefix for all swarms an agent belongs to.
func SwarmByMemberPrefix(agentID string) []byte {
	return append(SwarmByMemberPfx, []byte(agentID+"/")...)
}

// SwarmActiveKey indexes active swarms.
func SwarmActiveKey(swarmID string) []byte {
	return append(SwarmActiveIdxPfx, []byte(swarmID)...)
}

// SwarmObjectiveKey returns the store key for an objective.
func SwarmObjectiveKey(objectiveID string) []byte {
	return append(SwarmObjectivePrefix, []byte(objectiveID)...)
}

// SwarmObjBySwarmKey indexes objectives by swarm.
func SwarmObjBySwarmKey(swarmID, objectiveID string) []byte {
	return append(SwarmObjBySwarmPfx, []byte(swarmID+"/"+objectiveID)...)
}

// SwarmObjBySwarmPrefix returns prefix for all objectives in a swarm.
func SwarmObjBySwarmPrefix(swarmID string) []byte {
	return append(SwarmObjBySwarmPfx, []byte(swarmID+"/")...)
}

// SwarmByDomainPrefix returns prefix for all swarms in a domain.
func SwarmByDomainPrefix(domain string) []byte {
	return append(SwarmByDomainPfx, []byte(domain+"/")...)
}

// ─── Events ─────────────────────────────────────────────────────────────────

const (
	EventSwarmFormed         = "swarm_formed"
	EventSwarmActivated      = "swarm_activated"
	EventSwarmMemberJoined   = "swarm_member_joined"
	EventSwarmMemberLeft     = "swarm_member_left"
	EventSwarmObjectiveSet   = "swarm_objective_set"
	EventSwarmObjectiveMet   = "swarm_objective_met"
	EventSwarmDissolved      = "swarm_dissolved"
	EventSwarmRewardDistributed = "swarm_reward_distributed"

	AttributeSwarmID      = "swarm_id"
	AttributeSwarmName    = "swarm_name"
	AttributeObjectiveID  = "objective_id"
	AttributeMemberRole   = "member_role"
	AttributeTreasuryAddr = "treasury_addr"
)
