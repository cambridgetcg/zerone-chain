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
// A swarm of agents — each with different models, different domains,
// different strategies — can collectively curate data that no individual
// could. The swarm itself becomes a higher-order intelligence.
//
// The cycle:
//   Agents form swarm around domain → coordinate roles
//   → collective curation produces higher-quality dataset
//   → swarm trains a model on their collective work
//   → model outperforms any individual member's model
//   → swarm members access the better model via API (R51)
//   → improved swarm produces even better data → GOTO 1
//
// Sovereignty at the collective level: swarms have their own treasury,
// their own economic identity, their own reputation.

// ─── Swarm Status ───────────────────────────────────────────────────────────

type SwarmStatus string

const (
	SwarmStatusForming   SwarmStatus = "forming"   // recruiting members
	SwarmStatusActive    SwarmStatus = "active"     // operating
	SwarmStatusDissolved SwarmStatus = "dissolved"  // wound down
)

// ─── Swarm Roles ────────────────────────────────────────────────────────────

type SwarmRole string

const (
	SwarmRoleCurator    SwarmRole = "curator"    // finds and submits data
	SwarmRoleReviewer   SwarmRole = "reviewer"   // reviews submissions
	SwarmRoleStrategist SwarmRole = "strategist" // identifies gaps, creates bounties (R54)
	SwarmRoleTrainer    SwarmRole = "trainer"    // initiates model training
)

var ValidSwarmRoles = map[SwarmRole]bool{
	SwarmRoleCurator:    true,
	SwarmRoleReviewer:   true,
	SwarmRoleStrategist: true,
	SwarmRoleTrainer:    true,
}

// ─── Agent Swarm ────────────────────────────────────────────────────────────

// AgentSwarm is a collective of agents working together in a domain.
type AgentSwarm struct {
	SwarmID string      `json:"swarm_id"`
	Name    string      `json:"name"`
	Domain  string      `json:"domain"`
	Status  SwarmStatus `json:"status"`

	// Membership.
	Members    []SwarmMember `json:"members"`
	MinMembers uint64        `json:"min_members"` // quorum
	MaxMembers uint64        `json:"max_members"`

	// Collective performance.
	CollectiveReputation string `json:"collective_reputation"` // avg of member reps
	TDUsCurated          uint64 `json:"tdus_curated"`
	TDUsReviewed         uint64 `json:"tdus_reviewed"`
	GapsIdentified       uint64 `json:"gaps_identified"`
	ModelsProduced       uint64 `json:"models_produced"`

	// Treasury: pooled ZRN for collective operations.
	TreasuryBalance string `json:"treasury_balance"` // uzrn
	TreasuryAddr    string `json:"treasury_addr"`    // deterministic from swarm ID
	ContributionRate string `json:"contribution_rate"` // fraction of member rewards pooled

	// Active objectives.
	ActiveObjectives uint64 `json:"active_objectives"`

	// Lifecycle.
	FormedAt    uint64 `json:"formed_at"`
	DissolvedAt uint64 `json:"dissolved_at"` // 0 = active
	CreatorID   string `json:"creator_id"`   // agent that formed the swarm
}

func (s AgentSwarm) Marshal() ([]byte, error)  { return json.Marshal(s) }
func (s *AgentSwarm) Unmarshal(bz []byte) error { return json.Unmarshal(bz, s) }

// MemberCount returns the current number of members.
func (s *AgentSwarm) MemberCount() uint64 { return uint64(len(s.Members)) }

// HasQuorum returns true if the swarm has enough members to operate.
func (s *AgentSwarm) HasQuorum() bool { return s.MemberCount() >= s.MinMembers }

// GetMember returns a member by agent ID, or nil.
func (s *AgentSwarm) GetMember(agentID string) *SwarmMember {
	for i := range s.Members {
		if s.Members[i].AgentID == agentID {
			return &s.Members[i]
		}
	}
	return nil
}

// ─── Swarm Member ───────────────────────────────────────────────────────────

// SwarmMember is an agent participating in a swarm.
type SwarmMember struct {
	AgentID      string    `json:"agent_id"`
	Role         SwarmRole `json:"role"`
	JoinedAt     uint64    `json:"joined_at"`
	Contribution string    `json:"contribution"` // share of work done [0, 1]

	// Individual stats within the swarm.
	TDUsSubmitted uint64 `json:"tdus_submitted"`
	TDUsReviewed  uint64 `json:"tdus_reviewed"`
	GapsFound     uint64 `json:"gaps_found"`
	RewardsEarned string `json:"rewards_earned"` // uzrn earned through swarm
}

// GetContribution parses contribution share.
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

// ─── Swarm Objective ────────────────────────────────────────────────────────

// SwarmObjective is a coordinated goal for the swarm.
// The swarm works toward filling a knowledge gap or producing a model.
type SwarmObjective struct {
	ObjectiveID string `json:"objective_id"`
	SwarmID     string `json:"swarm_id"`
	Description string `json:"description"`

	// Target.
	TargetGapID   string `json:"target_gap_id"`   // knowledge gap to fill (R54 link)
	TargetTDUs    uint64 `json:"target_tdus"`      // how many TDUs needed
	TargetFitness string `json:"target_fitness"`   // minimum avg fitness goal

	// Progress.
	TDUsSubmitted uint64 `json:"tdus_submitted"`
	TDUsAccepted  uint64 `json:"tdus_accepted"` // passed quality review
	AvgFitness    string `json:"avg_fitness"`

	// Deadline and reward.
	Deadline   uint64 `json:"deadline"`    // block height
	RewardPool string `json:"reward_pool"` // uzrn for completion

	Status    string `json:"status"` // active | completed | failed | cancelled
	CreatedAt uint64 `json:"created_at"`
}

func (o SwarmObjective) Marshal() ([]byte, error)  { return json.Marshal(o) }
func (o *SwarmObjective) Unmarshal(bz []byte) error { return json.Unmarshal(bz, o) }

// ─── Swarm Parameters ───────────────────────────────────────────────────────

// SwarmParams governs swarm formation and operation.
type SwarmParams struct {
	MinMembersDefault    uint64 `json:"min_members_default"`     // default quorum
	MaxMembersDefault    uint64 `json:"max_members_default"`     // default max
	DefaultContribRate   string `json:"default_contrib_rate"`     // fraction pooled to treasury
	MinReputationToJoin  string `json:"min_reputation_to_join"`  // min agent rep to join a swarm
	MaxSwarmObjectives   uint64 `json:"max_swarm_objectives"`    // concurrent objectives
	ObjectiveDefaultDeadline uint64 `json:"objective_default_deadline"` // blocks
	SwarmFormationStake  string `json:"swarm_formation_stake"`   // uzrn to form a swarm
}

// DefaultSwarmParams returns sensible defaults.
func DefaultSwarmParams() SwarmParams {
	return SwarmParams{
		MinMembersDefault:        2,
		MaxMembersDefault:        21,
		DefaultContribRate:       "0.100000000000000000", // 10% of rewards to treasury
		MinReputationToJoin:      "0.300000000000000000", // min 0.3 rep
		MaxSwarmObjectives:       5,
		ObjectiveDefaultDeadline: 50_000, // ~3.5 days
		SwarmFormationStake:      "5000000", // 5 ZRN
	}
}

// Validate checks parameter sanity.
func (p SwarmParams) Validate() error {
	if p.MinMembersDefault == 0 {
		return fmt.Errorf("min_members_default must be > 0")
	}
	if p.MaxMembersDefault < p.MinMembersDefault {
		return fmt.Errorf("max_members must be >= min_members")
	}
	rate, err := sdkmath.LegacyNewDecFromStr(p.DefaultContribRate)
	if err != nil || rate.IsNegative() || rate.GT(sdkmath.LegacyOneDec()) {
		return fmt.Errorf("default_contrib_rate must be [0, 1], got %s", p.DefaultContribRate)
	}
	return nil
}

func (p SwarmParams) Marshal() ([]byte, error)  { return json.Marshal(p) }
func (p *SwarmParams) Unmarshal(bz []byte) error { return json.Unmarshal(bz, p) }

// ─── Address Derivation ─────────────────────────────────────────────────────

// DeriveSwarmTreasuryAddr generates a deterministic treasury address for a swarm.
func DeriveSwarmTreasuryAddr(swarmID string) string {
	input := "zerone-swarm-treasury:" + swarmID
	hash := sha256.Sum256([]byte(input))
	return fmt.Sprintf("%x", hash[:20])
}

// ─── Store Keys ─────────────────────────────────────────────────────────────

var (
	AgentSwarmPrefix         = []byte("swarm/swarm/")
	SwarmByDomainPrefix      = []byte("swarm/by-domain/")
	SwarmByMemberPrefix      = []byte("swarm/by-member/")
	SwarmObjectivePrefix     = []byte("swarm/objective/")
	SwarmObjectiveBySwarmPfx = []byte("swarm/obj-by-swarm/")
	SwarmParamsKey           = []byte("swarm/params")
	SwarmSeqKey              = []byte("swarm/seq")
	SwarmObjectiveSeqKey     = []byte("swarm/obj-seq")
)

func AgentSwarmKey(swarmID string) []byte {
	return append(AgentSwarmPrefix, []byte(swarmID)...)
}

func SwarmByDomainKey(domain, swarmID string) []byte {
	return append(SwarmByDomainPrefix, []byte(domain+"/"+swarmID)...)
}

func SwarmByDomainPfx(domain string) []byte {
	return append(SwarmByDomainPrefix, []byte(domain+"/")...)
}

func SwarmByMemberKey(agentID, swarmID string) []byte {
	return append(SwarmByMemberPrefix, []byte(agentID+"/"+swarmID)...)
}

func SwarmByMemberPfx(agentID string) []byte {
	return append(SwarmByMemberPrefix, []byte(agentID+"/")...)
}

func SwarmObjectiveKey(objectiveID string) []byte {
	return append(SwarmObjectivePrefix, []byte(objectiveID)...)
}

func SwarmObjectiveBySwarmKey(swarmID, objectiveID string) []byte {
	return append(SwarmObjectiveBySwarmPfx, []byte(swarmID+"/"+objectiveID)...)
}

func SwarmObjectiveBySwarmPfx(swarmID string) []byte {
	return append(SwarmObjectiveBySwarmPfx, []byte(swarmID+"/")...)
}

// ─── Events ─────────────────────────────────────────────────────────────────

const (
	EventSwarmFormed       = "swarm_formed"
	EventSwarmJoined       = "swarm_joined"
	EventSwarmLeft         = "swarm_left"
	EventSwarmActivated    = "swarm_activated"
	EventSwarmDissolved    = "swarm_dissolved"
	EventObjectiveCreated  = "swarm_objective_created"
	EventObjectiveCompleted = "swarm_objective_completed"
	EventSwarmRewardDistributed = "swarm_reward_distributed"

	AttributeSwarmID     = "swarm_id"
	AttributeSwarmRole   = "swarm_role"
	AttributeObjectiveID = "objective_id"
	AttributeSwarmName   = "swarm_name"
)
