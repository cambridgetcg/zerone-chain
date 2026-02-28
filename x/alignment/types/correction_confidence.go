package types

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
