package types

// CorrectionOutcome tracks the result of a correction application.
type CorrectionOutcome struct {
	Height      uint64 `json:"height"`
	Dimension   string `json:"dimension"`
	Magnitude   uint64 `json:"magnitude"`
	Direction   string `json:"direction"`
	ScoreBefore uint64 `json:"score_before"`
	ScoreAfter  uint64 `json:"score_after"`
	Successful  bool   `json:"successful"`
}

// GetDimensionScore extracts a dimension's score from DimensionScores by name.
func GetDimensionScore(scores *DimensionScores, dimension string) uint64 {
	switch dimension {
	case DimKnowledgeQuality:
		return scores.KnowledgeQuality
	case DimEconomicStability:
		return scores.EconomicStability
	case DimGovernanceParticipation:
		return scores.GovernanceParticipation
	case DimNetworkSecurity:
		return scores.NetworkSecurity
	case DimStakingRatio:
		return scores.StakingRatio
	default:
		return 0
	}
}

// QueryCorrectionConfidenceRequest is the request for CorrectionConfidence query.
type QueryCorrectionConfidenceRequest struct{}

// QueryCorrectionConfidenceResponse is the response for CorrectionConfidence query.
type QueryCorrectionConfidenceResponse struct {
	ConfidenceBps                uint64               `json:"confidence_bps"`
	TotalCorrections             uint64               `json:"total_corrections"`
	SuccessfulCorrections        uint64               `json:"successful_corrections"`
	EffectiveMaxMagnitude        uint64               `json:"effective_max_magnitude"`
	EffectiveObservationInterval uint64               `json:"effective_observation_interval"`
	RecentOutcomes               []*CorrectionOutcome `json:"recent_outcomes"`
}
