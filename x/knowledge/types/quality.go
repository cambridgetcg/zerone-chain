package types

// QualityTier represents the quality classification of a sample.
type QualityTier string

const (
	TierGold   QualityTier = "gold"
	TierSilver QualityTier = "silver"
	TierBronze QualityTier = "bronze"
)

// QualityTierFromScore returns the quality tier for a given BPS score
// based on the current params thresholds.
func QualityTierFromScore(score uint64, params *Params) QualityTier {
	switch {
	case score >= params.GoldThreshold:
		return TierGold
	case score >= params.SilverThreshold:
		return TierSilver
	default:
		return TierBronze
	}
}

// QualityVerdictToTier converts a QualityVerdict enum to a QualityTier string.
func QualityVerdictToTier(v QualityVerdict) QualityTier {
	switch v {
	case QualityVerdict_QUALITY_VERDICT_GOLD:
		return TierGold
	case QualityVerdict_QUALITY_VERDICT_SILVER:
		return TierSilver
	case QualityVerdict_QUALITY_VERDICT_BRONZE:
		return TierBronze
	default:
		return ""
	}
}
