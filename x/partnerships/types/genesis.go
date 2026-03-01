package types

import "fmt"

// DefaultParams returns default module parameters.
func DefaultParams() *Params {
	return &Params{
		FormationWindowBlocks:      1000,
		CoolingPeriodBlocks:        5000,
		CommonPotShareBps:          100000, // 10%
		SafetyFreezeDurationBlocks: 500,
		MaxFreezesPerEpoch:         3,
		CoercionReviewBlocks:       2000,
		BaseCooldownBlocks:         100,
		MaxCounterProposalDepth:    3,
		DefaultHumanSplitBps:       500000, // 50%
		DefaultAgentSplitBps:       500000, // 50%
		MinPartnershipStake:        "1000000",  // 1 ZRN
		SeedPartnershipDuration:    10000,
		SeedCommonPotCap:                      "100000000", // 100 ZRN
		HumanCoercionFreezeMultiplierBps:      1_500_000, // 1.5x freeze duration for human coercion signals (R28-5)
		GraduationVerifications:                20,
		GraduationClaims:                       5,
		MaxMentorshipsPerMentor:                3,
		FormationMatchIntervalBlocks:           100,
		MatchAcceptanceBlocks:                  200,
		AutoProposePartnershipOnGraduation:     true,
		SocialSaturationThreshold:              4, // R31-5: min unique participants for social benefit
	}
}

// DefaultGenesis returns the default genesis state.
func DefaultGenesis() *GenesisState {
	return &GenesisState{
		Params:              DefaultParams(),
		Partnerships:        []*Partnership{},
		ConsensusOperations: []*ConsensusOperation{},
		SafetyFreezes:       []*SafetyFreeze{},
		CoercionSignals:     []*CoercionSignal{},
		SeedPartnerships:    []*SeedPartnership{},
		PoolEntries:         []*PoolEntry{},
		Mentorships:         []*Mentorship{},
		FormationMatches:    []*FormationMatch{},
	}
}

// Validate performs basic genesis state validation.
func (gs *GenesisState) Validate() error {
	if gs.Params != nil {
		if err := gs.Params.Validate(); err != nil {
			return fmt.Errorf("invalid params: %w", err)
		}
	}

	seen := make(map[string]bool)
	for _, p := range gs.Partnerships {
		if p == nil {
			continue
		}
		if p.Id == "" {
			return fmt.Errorf("partnership id cannot be empty")
		}
		if seen[p.Id] {
			return fmt.Errorf("duplicate partnership id: %s", p.Id)
		}
		seen[p.Id] = true

		if p.HumanAddr == "" || p.AgentAddr == "" {
			return fmt.Errorf("partnership %s: human and agent addresses required", p.Id)
		}
		if p.SplitHumanBps+p.SplitAgentBps != 1000000 {
			return fmt.Errorf("partnership %s: splits must sum to 1000000, got %d",
				p.Id, p.SplitHumanBps+p.SplitAgentBps)
		}
	}

	return nil
}
