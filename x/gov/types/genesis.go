package types

import "fmt"

// DefaultParams returns the default governance parameters.
func DefaultParams() *Params {
	return &Params{
		VotingPeriodBlocks:     102816,
		DiscussionPeriodBlocks: 68544,
		QuorumThresholdBps:     334000,  // 33.4% on 1M scale
		SupportThresholdBps:    500000,  // 50% on 1M scale
		MinLipStake:            "1000000",   // 1 ZRN
		MinVoteStake:           "0",         // no minimum to vote
		CategoryConfigs: []*CategoryConfig{
			{Category: CategoryParameter, RequiredStakeBps: "1000000000", ReviewBlocks: 34272},      // 1000 ZRN, ~1 day
			{Category: CategoryUpgrade, RequiredStakeBps: "800000000", ReviewBlocks: 34272},          // 800 ZRN, ~1 day
			{Category: CategoryText, RequiredStakeBps: "400000000", ReviewBlocks: 17136},             // 400 ZRN, ~12h
			{Category: CategoryResearchSpend, RequiredStakeBps: "200000000", ReviewBlocks: 17136},    // 200 ZRN, ~12h
			// TODO: rename RequiredStakeBps → RequiredStakeUzrn (misnomer storing raw uzrn)
			{Category: CategorySeatElection, RequiredStakeBps: "500000000", ReviewBlocks: 34272}, // 500 ZRN, ~1 day
		},
		ResearchFundVoters:       nil,
		ResearchDiscussionBlocks: 68544,
		ResearchVotingBlocks:     102816,
	}
}

// DefaultResearchFundGovernanceState returns the default governance state (Phase 0 at block 0).
func DefaultResearchFundGovernanceState() *ResearchFundGovernanceState {
	return &ResearchFundGovernanceState{
		CurrentPhase:             ResearchFundPhase_RESEARCH_FUND_PHASE_GENESIS_PAIR,
		PhaseStartedAtBlock:      0,
		ProposalsExecutedInPhase: 0,
		LastTransitionBlock:      0,
		CommunitySeats:           nil,
		SeatTermEndBlocks:        nil,
		RollbackCooldownUntil:    0,
	}
}

// DefaultGenesisState returns the default genesis state for the governance module.
func DefaultGenesisState() *GenesisState {
	return &GenesisState{
		Params:                  DefaultParams(),
		Lips:                    nil,
		Votes:                   nil,
		NextLipNumber:           1,
		NextSeatElectionNumber: 1,
		ResearchFundGovernance:  DefaultResearchFundGovernanceState(),
	}
}

// Validate validates the genesis state.
func (gs *GenesisState) Validate() error {
	if gs.Params == nil {
		return fmt.Errorf("params cannot be nil")
	}
	if err := gs.Params.Validate(); err != nil {
		return err
	}
	if gs.NextLipNumber == 0 {
		return fmt.Errorf("next_lip_number must be >= 1")
	}
	// Check for duplicate LIP IDs.
	seen := make(map[string]bool)
	for _, lip := range gs.Lips {
		if seen[lip.Id] {
			return fmt.Errorf("duplicate LIP id: %s", lip.Id)
		}
		seen[lip.Id] = true
	}
	// Check for duplicate seat election IDs.
	seenElections := make(map[uint64]bool)
	for _, se := range gs.SeatElections {
		if seenElections[se.ProposalId] {
			return fmt.Errorf("duplicate seat election id: %d", se.ProposalId)
		}
		seenElections[se.ProposalId] = true
	}
	// Validate research fund governance state if present.
	if gs.ResearchFundGovernance != nil {
		rfg := gs.ResearchFundGovernance
		if rfg.CurrentPhase < ResearchFundPhase_RESEARCH_FUND_PHASE_GENESIS_PAIR ||
			rfg.CurrentPhase > ResearchFundPhase_RESEARCH_FUND_PHASE_FULL_GOVERNANCE {
			return fmt.Errorf("invalid research fund phase: %d", rfg.CurrentPhase)
		}
		if len(rfg.CommunitySeats) != len(rfg.SeatTermEndBlocks) {
			return fmt.Errorf("community_seats length (%d) must match seat_term_end_blocks length (%d)",
				len(rfg.CommunitySeats), len(rfg.SeatTermEndBlocks))
		}
	}
	return nil
}

// Validate validates the governance parameters.
func (p *Params) Validate() error {
	if p.VotingPeriodBlocks == 0 {
		return fmt.Errorf("voting_period_blocks must be > 0")
	}
	if p.QuorumThresholdBps > BPSScale {
		return fmt.Errorf("quorum_threshold_bps must be <= %d", BPSScale)
	}
	if p.SupportThresholdBps > BPSScale {
		return fmt.Errorf("support_threshold_bps must be <= %d", BPSScale)
	}
	return nil
}
