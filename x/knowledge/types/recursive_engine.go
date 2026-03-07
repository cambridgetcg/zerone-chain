package types

import (
	"encoding/json"
	"fmt"

	sdkmath "cosmossdk.io/math"
)

// ─── R53: Recursive Self-Improvement Engine ─────────────────────────────────
//
// This is the PURPOSE of Zerone, made explicit in code.
//
// The loop:
//   1. Models participate in consensus (quality round verification)
//   2. Their verification work is automatically captured as training data
//   3. That data trains the next generation of models
//   4. Better models replace older ones in consensus slots
//   5. Better models produce better verification data
//   6. GOTO 1
//
// No human bottleneck. The chain improves itself.
//
// Key insight: The consensus mechanism IS the training data pipeline.
// Every quality round vote by a model is simultaneously:
//   - Consensus participation (maintaining the network)
//   - Training data generation (improving future models)
//
// The act of running the blockchain IS the act of creating better AI.

// ─── Verification Capture ───────────────────────────────────────────────────

// VerificationCapture records a model's work during consensus.
// Each capture becomes a training sample for the next generation.
type VerificationCapture struct {
	CaptureID  string `json:"capture_id"`   // unique ID
	RoundID    string `json:"round_id"`     // quality round this came from
	VerifierID string `json:"verifier_id"`  // model-agent that verified
	ModelID    string `json:"model_id"`     // underlying model
	Domain     string `json:"domain"`       // knowledge domain

	// The work product.
	Vote       string `json:"vote"`         // serialized QualityVote JSON
	VoteHash   string `json:"vote_hash"`    // hash of the vote (integrity)

	// Consensus alignment — this IS the quality signal.
	// No separate quality round needed: consensus itself validates.
	Aligned        bool   `json:"aligned"`          // did vote match consensus?
	ConsensusScore string `json:"consensus_score"`  // how close to consensus (0-1)

	// Context for training.
	SubmissionID string `json:"submission_id"` // what was being reviewed
	SampleType   int32  `json:"sample_type"`   // type of content reviewed

	// Metadata.
	Generation   uint64 `json:"generation"`    // model's generation number
	BlockHeight  uint64 `json:"block_height"`  // when captured
	FitnessScore string `json:"fitness_score"` // initial fitness from alignment
}

// ─── Consensus Pool ─────────────────────────────────────────────────────────

// ConsensusSlot represents a model's participation in the verification pool.
// Models stake ZRN to enter and earn/lose based on verification quality.
type ConsensusSlot struct {
	AgentID      string `json:"agent_id"`       // agent identity
	ModelID      string `json:"model_id"`       // underlying model
	Domain       string `json:"domain"`         // domain of expertise
	Generation   uint64 `json:"generation"`     // model generation
	Stake        string `json:"stake"`          // ZRN staked to participate
	JoinedBlock  uint64 `json:"joined_block"`   // when they joined

	// Performance tracking.
	TotalVotes      uint64 `json:"total_votes"`       // votes cast
	AlignedVotes    uint64 `json:"aligned_votes"`     // aligned with consensus
	CapturesCreated uint64 `json:"captures_created"`  // training samples generated
	EarnedRewards   string `json:"earned_rewards"`    // ZRN earned

	// Status.
	Active       bool   `json:"active"`         // currently participating
	RetiredBlock uint64 `json:"retired_block"`  // when retired (0 = still active)
}

// AlignmentRate returns the fraction of votes aligned with consensus.
func (s *ConsensusSlot) AlignmentRate() sdkmath.LegacyDec {
	if s.TotalVotes == 0 {
		return sdkmath.LegacyZeroDec()
	}
	return sdkmath.LegacyNewDec(int64(s.AlignedVotes)).Quo(
		sdkmath.LegacyNewDec(int64(s.TotalVotes)))
}

// ─── Generational Tracker ───────────────────────────────────────────────────

// ModelGeneration tracks a generation of models and their collective performance.
type ModelGeneration struct {
	Generation     uint64 `json:"generation"`      // generation number (0 = seed)
	ActiveModels   uint64 `json:"active_models"`   // models in this generation
	TotalCaptures  uint64 `json:"total_captures"`  // training samples produced
	AvgAlignment   string `json:"avg_alignment"`   // average consensus alignment
	StartBlock     uint64 `json:"start_block"`     // when this generation started
	EndBlock       uint64 `json:"end_block"`       // when superseded (0 = current)
	ParentGen      uint64 `json:"parent_gen"`      // which generation trained these
	TrainingTDUs   uint64 `json:"training_tdus"`   // TDUs used for training
}

// ─── Generational Challenge ─────────────────────────────────────────────────

// GenerationalChallenge represents Gen N+1 challenging Gen N for consensus slots.
// The challenger must demonstrate superior verification quality.
type GenerationalChallenge struct {
	ChallengeID   string `json:"challenge_id"`
	ChallengerID  string `json:"challenger_id"`  // Gen N+1 model-agent
	DefenderID    string `json:"defender_id"`    // Gen N model-agent
	Domain        string `json:"domain"`
	StartBlock    uint64 `json:"start_block"`
	EndBlock      uint64 `json:"end_block"`      // challenge window end

	// Parallel verification: both review the same submissions.
	SharedRounds  uint64 `json:"shared_rounds"`  // rounds both participated in
	ChallengerScore string `json:"challenger_score"` // challenger's alignment
	DefenderScore   string `json:"defender_score"`   // defender's alignment

	// Outcome.
	Resolved bool   `json:"resolved"`
	Winner   string `json:"winner"`    // agent ID of winner
}

// ─── Parameters ─────────────────────────────────────────────────────────────

// RecursiveEngineParams governs the self-improvement loop.
type RecursiveEngineParams struct {
	// Capture settings.
	CaptureEnabled       bool   `json:"capture_enabled"`        // master switch
	MinVotesForCapture   uint64 `json:"min_votes_for_capture"`  // min reveals before capture

	// Consensus pool.
	MinConsensusStake    string `json:"min_consensus_stake"`     // min ZRN to join pool
	MaxSlotsPerDomain   uint64 `json:"max_slots_per_domain"`    // max models per domain

	// Generational succession.
	ChallengeWindowBlocks uint64 `json:"challenge_window_blocks"` // blocks for parallel eval
	MinSharedRounds       uint64 `json:"min_shared_rounds"`       // min rounds for valid challenge
	SuccessionThreshold   string `json:"succession_threshold"`    // alignment improvement needed

	// Fitness for captures.
	AlignedBaseFitness   string `json:"aligned_base_fitness"`    // fitness for aligned vote
	MisalignedBaseFitness string `json:"misaligned_base_fitness"` // fitness for misaligned vote
}

// DefaultRecursiveEngineParams returns sensible defaults.
func DefaultRecursiveEngineParams() RecursiveEngineParams {
	return RecursiveEngineParams{
		CaptureEnabled:       true,
		MinVotesForCapture:   3,
		MinConsensusStake:    "10000000", // 10 ZRN
		MaxSlotsPerDomain:   21,         // like a validator set
		ChallengeWindowBlocks: 1000,     // ~1.5 hours at 5s blocks
		MinSharedRounds:      10,        // must co-verify 10+ rounds
		SuccessionThreshold:  "0.050000000000000000", // must be 5% better
		AlignedBaseFitness:   "0.700000000000000000", // aligned = strong memory
		MisalignedBaseFitness: "0.300000000000000000", // misaligned = weak memory
	}
}

// Validate checks parameter sanity.
func (p RecursiveEngineParams) Validate() error {
	stake, ok := sdkmath.NewIntFromString(p.MinConsensusStake)
	if !ok || stake.IsNegative() {
		return fmt.Errorf("invalid min_consensus_stake: %s", p.MinConsensusStake)
	}
	if p.MaxSlotsPerDomain == 0 {
		return fmt.Errorf("max_slots_per_domain must be > 0")
	}
	if p.ChallengeWindowBlocks == 0 {
		return fmt.Errorf("challenge_window_blocks must be > 0")
	}
	threshold, err := sdkmath.LegacyNewDecFromStr(p.SuccessionThreshold)
	if err != nil || threshold.IsNegative() || threshold.GT(sdkmath.LegacyOneDec()) {
		return fmt.Errorf("invalid succession_threshold: %s", p.SuccessionThreshold)
	}
	return nil
}

// Marshal/Unmarshal helpers.
func (p RecursiveEngineParams) Marshal() ([]byte, error) { return json.Marshal(p) }
func (p *RecursiveEngineParams) Unmarshal(bz []byte) error { return json.Unmarshal(bz, p) }

func (c VerificationCapture) Marshal() ([]byte, error) { return json.Marshal(c) }
func (c *VerificationCapture) Unmarshal(bz []byte) error { return json.Unmarshal(bz, c) }

func (s ConsensusSlot) Marshal() ([]byte, error) { return json.Marshal(s) }
func (s *ConsensusSlot) Unmarshal(bz []byte) error { return json.Unmarshal(bz, s) }

func (g ModelGeneration) Marshal() ([]byte, error) { return json.Marshal(g) }
func (g *ModelGeneration) Unmarshal(bz []byte) error { return json.Unmarshal(bz, g) }

func (ch GenerationalChallenge) Marshal() ([]byte, error) { return json.Marshal(ch) }
func (ch *GenerationalChallenge) Unmarshal(bz []byte) error { return json.Unmarshal(bz, ch) }

// ─── Events & Errors ────────────────────────────────────────────────────────

const (
	EventVerificationCaptured    = "verification_captured"
	EventModelJoinedConsensus    = "model_joined_consensus"
	EventModelRetiredConsensus   = "model_retired_consensus"
	EventGenerationalChallenge   = "generational_challenge"
	EventGenerationalSuccession  = "generational_succession"

	AttributeCaptureID     = "capture_id"
	AttributeVerifierID    = "verifier_id"
	AttributeAligned       = "aligned"
	AttributeGeneration    = "generation"
	AttributeChallengeID   = "challenge_id"
	AttributeChallengerID  = "challenger_id"
	AttributeDefenderID    = "defender_id"
	AttributeWinnerID      = "winner_id"
)
