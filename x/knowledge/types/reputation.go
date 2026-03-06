package types

import (
	"encoding/json"
	"fmt"

	sdkmath "cosmossdk.io/math"
)

// ReputationDecayParams holds governance-tunable parameters for agent reputation decay.
// Stored as JSON singleton in the KVStore.
type ReputationDecayParams struct {
	// DecayRateBps: basis points of score to decay per interval of inactivity. Default 500 = 5%.
	DecayRateBps uint32 `json:"decay_rate_bps"`
	// DecayIntervalBlocks: number of blocks of inactivity before decay is applied (~1 month).
	DecayIntervalBlocks uint64 `json:"decay_interval_blocks"`
	// FloorRatioBps: minimum score as basis points of peak score. Default 2500 = 25%.
	FloorRatioBps uint32 `json:"floor_ratio_bps"`
}

// DefaultReputationDecayParams returns sensible defaults.
func DefaultReputationDecayParams() ReputationDecayParams {
	return ReputationDecayParams{
		DecayRateBps:        500,    // 5%
		DecayIntervalBlocks: 432000, // ~30 days at 6s blocks
		FloorRatioBps:       2500,   // 25% floor
	}
}

// GetDecayRate returns the decay rate as a LegacyDec (e.g. 500 bps = 0.05).
func (p ReputationDecayParams) GetDecayRate() sdkmath.LegacyDec {
	return sdkmath.LegacyNewDec(int64(p.DecayRateBps)).Quo(sdkmath.LegacyNewDec(10000))
}

// GetFloorRatio returns the floor ratio as a LegacyDec (e.g. 2500 bps = 0.25).
func (p ReputationDecayParams) GetFloorRatio() sdkmath.LegacyDec {
	return sdkmath.LegacyNewDec(int64(p.FloorRatioBps)).Quo(sdkmath.LegacyNewDec(10000))
}

// Validate checks all fields are within valid ranges.
func (p ReputationDecayParams) Validate() error {
	if p.DecayRateBps > 10000 {
		return fmt.Errorf("decay_rate_bps must be <= 10000, got %d", p.DecayRateBps)
	}
	if p.DecayIntervalBlocks == 0 {
		return fmt.Errorf("decay_interval_blocks must be > 0")
	}
	if p.FloorRatioBps > 10000 {
		return fmt.Errorf("floor_ratio_bps must be <= 10000, got %d", p.FloorRatioBps)
	}
	return nil
}

// MarshalJSON marshals to JSON.
func (p ReputationDecayParams) MarshalJSON() ([]byte, error) {
	type alias ReputationDecayParams
	return json.Marshal(alias(p))
}

// UnmarshalJSON unmarshals from JSON.
func (p *ReputationDecayParams) UnmarshalJSON(bz []byte) error {
	type alias ReputationDecayParams
	var a alias
	if err := json.Unmarshal(bz, &a); err != nil {
		return err
	}
	*p = ReputationDecayParams(a)
	return nil
}

// AgentDomainReputation stores an agent's reputation within a specific domain.
// Stored as JSON in the KVStore keyed by agentAddr/domainID.
type AgentDomainReputation struct {
	AgentAddr        string `json:"agent_addr"`
	DomainID         string `json:"domain_id"`
	Score            string `json:"score"`              // sdkmath.LegacyDec serialized
	PeakScore        string `json:"peak_score"`         // sdkmath.LegacyDec serialized
	LastActiveHeight int64  `json:"last_active_height"` // block height of last activity
}

// GetScore parses the stored reputation score.
func (r *AgentDomainReputation) GetScore() sdkmath.LegacyDec {
	if r.Score == "" {
		return sdkmath.LegacyZeroDec()
	}
	d, err := sdkmath.LegacyNewDecFromStr(r.Score)
	if err != nil {
		return sdkmath.LegacyZeroDec()
	}
	return d
}

// SetScore stores the reputation score.
func (r *AgentDomainReputation) SetScore(score sdkmath.LegacyDec) {
	if score.IsNegative() {
		score = sdkmath.LegacyZeroDec()
	}
	r.Score = score.String()
}

// GetPeakScore parses the stored peak reputation score.
func (r *AgentDomainReputation) GetPeakScore() sdkmath.LegacyDec {
	if r.PeakScore == "" {
		return sdkmath.LegacyZeroDec()
	}
	d, err := sdkmath.LegacyNewDecFromStr(r.PeakScore)
	if err != nil {
		return sdkmath.LegacyZeroDec()
	}
	return d
}

// SetPeakScore stores the peak reputation score.
func (r *AgentDomainReputation) SetPeakScore(score sdkmath.LegacyDec) {
	if score.IsNegative() {
		score = sdkmath.LegacyZeroDec()
	}
	r.PeakScore = score.String()
}

// NewAgentDomainReputation creates a new agent domain reputation record.
func NewAgentDomainReputation(agentAddr, domainID string, initialScore sdkmath.LegacyDec, currentHeight int64) AgentDomainReputation {
	return AgentDomainReputation{
		AgentAddr:        agentAddr,
		DomainID:         domainID,
		Score:            initialScore.String(),
		PeakScore:        initialScore.String(),
		LastActiveHeight: currentHeight,
	}
}
