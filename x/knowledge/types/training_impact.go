package types

import (
	"encoding/json"
	"fmt"

	sdkmath "cosmossdk.io/math"
)

// ─── R52: Training Impact Attribution ───────────────────────────────────────
//
// When a model succeeds, trace back to the training data that made it good.
// Reward the agents who curated that data. When a model fails, trace back
// to the bad data. This creates a direct economic link between curation
// quality and rewards.
//
// The feedback loop:
//   Model earns revenue → trace training TDUs → identify curators
//   → distribute attribution rewards (proportional to fitness-weighted contribution)
//   → curators who did well earn more → incentivized to curate even better
//   → better data → better models → more revenue → GOTO 1
//
// This is the economic backbone of sovereignty: agents that curate well
// earn more, which funds more thinking, which funds better curation.

// ─── Training Impact ────────────────────────────────────────────────────────

// TrainingImpact records the contribution of TDUs and their curators
// to a model's performance. Computed when a model earns revenue or
// wins a generational challenge.
type TrainingImpact struct {
	ImpactID string `json:"impact_id"`
	ModelID  string `json:"model_id"`

	// What triggered this attribution?
	TriggerType  AttributionTrigger `json:"trigger_type"`
	TriggerValue string             `json:"trigger_value"` // ZRN amount or score

	// Which TDUs contributed and how much?
	Contributors []TDUContribution `json:"contributors"`

	// Which curators get paid?
	CuratorRewards []CuratorReward `json:"curator_rewards"`

	// Distribution summary.
	TotalPool        string `json:"total_pool"`        // ZRN available for distribution
	TotalDistributed string `json:"total_distributed"` // ZRN actually paid out
	TDUCount         uint64 `json:"tdu_count"`         // TDUs considered
	CuratorCount     uint64 `json:"curator_count"`     // unique curators rewarded

	ComputedAt uint64 `json:"computed_at"` // block height
}

func (t TrainingImpact) Marshal() ([]byte, error)  { return json.Marshal(t) }
func (t *TrainingImpact) Unmarshal(bz []byte) error { return json.Unmarshal(bz, t) }

// AttributionTrigger is what caused the attribution computation.
type AttributionTrigger string

const (
	// TriggerAPIRevenue — model earned API revenue from consumers.
	TriggerAPIRevenue AttributionTrigger = "api_revenue"
	// TriggerChallengeWon — model won a generational challenge.
	TriggerChallengeWon AttributionTrigger = "challenge_won"
	// TriggerBenchmarkImproved — model's benchmark score improved significantly.
	TriggerBenchmarkImproved AttributionTrigger = "benchmark_improved"
	// TriggerPeriodicReview — scheduled periodic attribution (every N epochs).
	TriggerPeriodicReview AttributionTrigger = "periodic_review"
)

// ─── TDU Contribution ───────────────────────────────────────────────────────

// TDUContribution tracks how much a single TDU contributed to a model's success.
type TDUContribution struct {
	TDUID   string `json:"tdu_id"`
	Curator string `json:"curator"` // submitter address

	// Quality at time of training.
	FitnessAtTraining string `json:"fitness_at_training"` // fitness when model was trained

	// Contribution weight: fitness × recency factor.
	// Higher fitness = more credit. More recent = more credit.
	Weight string `json:"weight"`

	// What fraction of the attribution pool this TDU earns.
	RewardShare string `json:"reward_share"` // fraction [0, 1]
}

// GetWeight parses the contribution weight.
func (c *TDUContribution) GetWeight() sdkmath.LegacyDec {
	if c.Weight == "" {
		return sdkmath.LegacyZeroDec()
	}
	d, err := sdkmath.LegacyNewDecFromStr(c.Weight)
	if err != nil {
		return sdkmath.LegacyZeroDec()
	}
	return d
}

// ─── Curator Reward ─────────────────────────────────────────────────────────

// CuratorReward is the payout to a single curator from a training impact event.
type CuratorReward struct {
	CuratorAddr  string `json:"curator_addr"`
	AgentID      string `json:"agent_id"`       // if curator is an agent (empty if human)
	TotalWeight  string `json:"total_weight"`    // sum of their TDU contribution weights
	RewardAmount string `json:"reward_amount"`   // ZRN earned from this attribution
	TDUCount     uint64 `json:"tdu_count"`       // how many of their TDUs contributed
}

// GetRewardAmount parses the reward.
func (r *CuratorReward) GetRewardAmount() sdkmath.Int {
	if r.RewardAmount == "" {
		return sdkmath.ZeroInt()
	}
	i, ok := sdkmath.NewIntFromString(r.RewardAmount)
	if !ok {
		return sdkmath.ZeroInt()
	}
	return i
}

// ─── Curator Lifetime Impact ────────────────────────────────────────────────

// CuratorImpactScore tracks a curator's lifetime contribution to model success.
// This is their "batting average" — how much value their data has created.
type CuratorImpactScore struct {
	CuratorAddr string `json:"curator_addr"`
	AgentID     string `json:"agent_id"` // if agent

	// Lifetime stats.
	TotalAttributions    uint64 `json:"total_attributions"`     // times their data was attributed
	TotalTDUsAttributed  uint64 `json:"total_tdus_attributed"`  // TDUs that contributed to models
	TotalRewardsEarned   string `json:"total_rewards_earned"`   // lifetime ZRN from attribution
	AvgContributionWeight string `json:"avg_contribution_weight"` // average quality of contributions

	// Recent performance.
	RecentAttributions uint64 `json:"recent_attributions"` // last N epochs
	RecentRewards      string `json:"recent_rewards"`

	// Models influenced.
	ModelsInfluenced uint64 `json:"models_influenced"` // distinct models using their data

	UpdatedAt uint64 `json:"updated_at"`
}

func (c CuratorImpactScore) Marshal() ([]byte, error)  { return json.Marshal(c) }
func (c *CuratorImpactScore) Unmarshal(bz []byte) error { return json.Unmarshal(bz, c) }

// ─── Attribution Parameters ─────────────────────────────────────────────────

// AttributionParams governs how rewards flow back to curators.
type AttributionParams struct {
	// What fraction of model revenue goes to data attribution?
	// This is ADDITIONAL to the R44 submitter share — it rewards
	// curators specifically for the MODEL's success, not just data access.
	AttributionRate string `json:"attribution_rate"` // default: "0.10" = 10%

	// Minimum revenue before attribution kicks in (prevents dust).
	MinRevenueForAttribution string `json:"min_revenue_for_attribution"` // default: "1000000" = 1 ZRN

	// How many top TDUs to consider per attribution event.
	MaxContributors uint64 `json:"max_contributors"` // default: 100

	// Recency decay: recent data gets more credit.
	// Weight = fitness × recency_factor^(blocks_since_training / decay_halflife)
	RecencyDecayHalflife uint64 `json:"recency_decay_halflife"` // blocks; default: 50000

	// Minimum fitness for a TDU to qualify for attribution.
	MinFitnessForAttribution string `json:"min_fitness_for_attribution"` // default: "0.40"

	// Periodic attribution interval (in epochs = 100 blocks each).
	PeriodicAttributionEpochs uint64 `json:"periodic_attribution_epochs"` // default: 10
}

// DefaultAttributionParams returns sensible defaults.
func DefaultAttributionParams() AttributionParams {
	return AttributionParams{
		AttributionRate:           "0.100000000000000000", // 10% of model revenue
		MinRevenueForAttribution:  "1000000",              // 1 ZRN minimum
		MaxContributors:           100,
		RecencyDecayHalflife:      50_000,                 // ~3.5 days at 5s blocks
		MinFitnessForAttribution:  "0.400000000000000000", // min fitness 0.4
		PeriodicAttributionEpochs: 10,                     // every 1000 blocks
	}
}

// Validate checks parameter sanity.
func (p AttributionParams) Validate() error {
	rate, err := sdkmath.LegacyNewDecFromStr(p.AttributionRate)
	if err != nil || rate.IsNegative() || rate.GT(sdkmath.LegacyOneDec()) {
		return fmt.Errorf("attribution_rate must be [0, 1], got %s", p.AttributionRate)
	}
	if p.MaxContributors == 0 {
		return fmt.Errorf("max_contributors must be > 0")
	}
	minFit, err := sdkmath.LegacyNewDecFromStr(p.MinFitnessForAttribution)
	if err != nil || minFit.IsNegative() || minFit.GT(sdkmath.LegacyOneDec()) {
		return fmt.Errorf("min_fitness must be [0, 1], got %s", p.MinFitnessForAttribution)
	}
	return nil
}

func (p AttributionParams) Marshal() ([]byte, error)  { return json.Marshal(p) }
func (p *AttributionParams) Unmarshal(bz []byte) error { return json.Unmarshal(bz, p) }

// ─── Store Keys ─────────────────────────────────────────────────────────────

var (
	TrainingImpactPrefix       = []byte("attribution/impact/")
	TrainingImpactByModelPfx   = []byte("attribution/by-model/")
	CuratorImpactScorePrefix   = []byte("attribution/curator/")
	AttributionParamsKey       = []byte("attribution/params")
	TrainingImpactSeqKey       = []byte("attribution/seq")
	PendingAttributionPrefix   = []byte("attribution/pending/")
)

// TrainingImpactKey returns the store key for a training impact record.
func TrainingImpactKey(impactID string) []byte {
	return append(TrainingImpactPrefix, []byte(impactID)...)
}

// TrainingImpactByModelKey indexes impacts by model ID.
func TrainingImpactByModelKey(modelID, impactID string) []byte {
	return append(TrainingImpactByModelPfx, []byte(modelID+"/"+impactID)...)
}

// CuratorImpactScoreKey returns the store key for a curator's lifetime score.
func CuratorImpactScoreKey(curatorAddr string) []byte {
	return append(CuratorImpactScorePrefix, []byte(curatorAddr)...)
}

// PendingAttributionKey stores models awaiting attribution computation.
func PendingAttributionKey(modelID string) []byte {
	return append(PendingAttributionPrefix, []byte(modelID)...)
}

// ─── Events ─────────────────────────────────────────────────────────────────

const (
	EventAttributionComputed    = "attribution_computed"
	EventCuratorRewarded        = "curator_rewarded"
	EventAttributionQueued      = "attribution_queued"

	AttributeImpactID      = "impact_id"
	AttributeImpactModelID = "model_id"
	AttributeTriggerType   = "trigger_type"
	AttributePoolSize      = "pool_size"
	AttributeDistributed   = "distributed"
	AttributeCuratorCount  = "curator_count"
	AttributeCuratorAddr   = "curator_addr"
	AttributeRewardAmount  = "reward_amount"
)
