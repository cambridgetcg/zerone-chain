package types

// BPS is the basis-point denominator (1,000,000 = 100%).
const BPS = uint64(1_000_000)

// DefaultParams returns the default alignment module parameters.
func DefaultParams() Params {
	return Params{
		ObservationIntervalBlocks:    100,
		WeightKnowledgeQuality:       200_000, // 20%
		WeightEconomicStability:      200_000, // 20%
		WeightGovernanceParticipation: 200_000, // 20%
		WeightNetworkSecurity:        200_000, // 20%
		WeightStakingRatio:           200_000, // 20%
		CriticalThreshold:            200_000, // 20%
		DegradedThreshold:            400_000, // 40%
		HealthyThreshold:             700_000, // 70%
		Enabled:                      true,
		MaxAutoApplyMagnitudeBps:             500_000,   // 50% — conservative testnet default
		CorrectionConfidenceWindowSize:       50,
		CorrectionConfidenceMinSamples:       5,
		MinConfidenceForAutoApply:            200_000,   // 20%
		CorrectionBoundsMinMultiplierBps:     300_000,   // 30%
		CorrectionBoundsMaxMultiplierBps:     2_000_000, // 200%
	}
}

// DefaultGenesis returns the default genesis state.
func DefaultGenesis() *GenesisState {
	params := DefaultParams()
	return &GenesisState{
		Params: &params,
		State: &AlignmentState{
			Enabled: true,
		},
	}
}

// Validate validates the genesis state.
func (gs *GenesisState) Validate() error {
	if gs.Params != nil {
		if err := gs.Params.Validate(); err != nil {
			return err
		}
	}
	return nil
}

// Validate validates the module parameters.
func (p *Params) Validate() error {
	if p.ObservationIntervalBlocks == 0 {
		return ErrInvalidInterval
	}

	weightSum := p.WeightKnowledgeQuality +
		p.WeightEconomicStability +
		p.WeightGovernanceParticipation +
		p.WeightNetworkSecurity +
		p.WeightStakingRatio
	if weightSum != BPS {
		return ErrInvalidWeights
	}

	if p.CriticalThreshold > BPS || p.DegradedThreshold > BPS || p.HealthyThreshold > BPS {
		return ErrInvalidThreshold
	}

	if p.CriticalThreshold >= p.DegradedThreshold || p.DegradedThreshold >= p.HealthyThreshold {
		return ErrThresholdOrder
	}

	if p.MaxAutoApplyMagnitudeBps > BPS {
		return ErrInvalidMaxAutoApply
	}

	if p.CorrectionBoundsMinMultiplierBps > p.CorrectionBoundsMaxMultiplierBps {
		return ErrInvalidConfidenceBounds
	}

	return nil
}
