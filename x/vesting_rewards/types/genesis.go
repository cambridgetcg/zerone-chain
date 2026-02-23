package types

import (
	"fmt"

	sdk "github.com/cosmos/cosmos-sdk/types"

	commontypes "github.com/zerone-chain/zerone/x/common/types"
)

// DefaultRevenueSplit returns the default 4-way revenue split.
// contributor 55%, protocol 22%, research 13%, burn 10%.
func DefaultRevenueSplit() *commontypes.RevenueSplit {
	return &commontypes.RevenueSplit{
		ContributorBps: 550000,
		ProtocolBps:    220000,
		ResearchBps:    130000,
		BurnBps:        100000,
	}
}

// DefaultProtocolSubSplit returns the default protocol sub-split.
// citation 50%, verification 30%, treasury 20%.
func DefaultProtocolSubSplit() *commontypes.ProtocolSubSplit {
	return &commontypes.ProtocolSubSplit{
		CitationBps:     500000,
		VerificationBps: 300000,
		TreasuryBps:     200000,
	}
}

// DefaultParams returns default module parameters.
func DefaultParams() *Params {
	return &Params{
		BlockReward:                "10000000",             // 10 ZRN base
		RewardDecayBps:             850000,                 // 0.85x per epoch
		BlocksPerRewardEpoch:       100000,                 // ~2.9 days at 2521ms
		RevenueSplit:               DefaultRevenueSplit(),
		ProtocolSubSplit:           DefaultProtocolSubSplit(),
		FounderShareBps:            70000,                  // 7% of research fund
		FounderAddress:             "",                     // disabled by default
		GovernanceActivationHeight: 0,                      // no sunset
		CategoryRewardConfigs:      DefaultCategoryRewardConfigs(),
		ResearchFundModuleAccount:  ResearchFundModuleName,
		VestingEnabled:             true,
		ReleasedClawbackRate:       3300,                   // 33% of released clawed back
		MinValidatorsForFullReward: 22,
		EmptyBlockRewardRate:       0,                      // 0% for empty blocks (PoT)
		FloorReward:                "100000",               // 0.1 ZRN in uzrn
		InitialFundBalance:         "0",                    // pure PoT
	}
}

// DefaultCategoryRewardConfigs returns per-category block reward multipliers.
func DefaultCategoryRewardConfigs() []*CategoryRewardConfig {
	return []*CategoryRewardConfig{
		{Category: string(CategoryAxiomatic), MultiplierBps: 1200000},     // 1.2x
		{Category: string(CategoryFormalProof), MultiplierBps: 1100000},   // 1.1x
		{Category: string(CategoryOnChain), MultiplierBps: 1000000},       // 1.0x
		{Category: string(CategoryCryptographic), MultiplierBps: 1050000}, // 1.05x
		{Category: string(CategoryComputational), MultiplierBps: 1000000}, // 1.0x
		{Category: string(CategoryPeerReviewed), MultiplierBps: 900000},   // 0.9x
		{Category: string(CategoryReplicated), MultiplierBps: 950000},     // 0.95x
		{Category: string(CategoryOracleFeed), MultiplierBps: 800000},     // 0.8x
		{Category: string(CategoryAttestation), MultiplierBps: 850000},    // 0.85x
		{Category: string(CategoryContested), MultiplierBps: 600000},      // 0.6x
	}
}

// DefaultCategoryConfigs returns release curve configs based on scientometric research.
func DefaultCategoryConfigs() []*CategoryConfig {
	return []*CategoryConfig{
		{Category: string(CategoryAxiomatic), HalfLifeBlocks: 1_111_111, CliffBlocks: 11111, MaxRelease: 950000},
		{Category: string(CategoryFormalProof), HalfLifeBlocks: 555_555, CliffBlocks: 5555, MaxRelease: 920000},
		{Category: string(CategoryOnChain), HalfLifeBlocks: 222_222, CliffBlocks: 1111, MaxRelease: 900000},
		{Category: string(CategoryCryptographic), HalfLifeBlocks: 222_222, CliffBlocks: 3333, MaxRelease: 900000},
		{Category: string(CategoryComputational), HalfLifeBlocks: 333_333, CliffBlocks: 2222, MaxRelease: 880000},
		{Category: string(CategoryPeerReviewed), HalfLifeBlocks: 111_111, CliffBlocks: 5555, MaxRelease: 850000},
		{Category: string(CategoryReplicated), HalfLifeBlocks: 111_111, CliffBlocks: 3333, MaxRelease: 880000},
		{Category: string(CategoryOracleFeed), HalfLifeBlocks: 55_555, CliffBlocks: 555, MaxRelease: 800000},
		{Category: string(CategoryAttestation), HalfLifeBlocks: 77_777, CliffBlocks: 2222, MaxRelease: 800000},
		{Category: string(CategoryContested), HalfLifeBlocks: 22_222, CliffBlocks: 1111, MaxRelease: 600000},
	}
}

// DefaultGenesis returns the default genesis state.
func DefaultGenesis() *GenesisState {
	return &GenesisState{
		Params:           DefaultParams(),
		CategoryConfigs:  DefaultCategoryConfigs(),
		VestingSchedules: []*VestingSchedule{},
	}
}

// Validate performs basic genesis state validation.
func (gs *GenesisState) Validate() error {
	if gs.Params != nil {
		if err := ValidateParams(gs.Params); err != nil {
			return err
		}
	}
	if len(gs.CategoryConfigs) == 0 {
		return fmt.Errorf("at least one category config required")
	}
	for _, cfg := range gs.CategoryConfigs {
		if cfg.MaxRelease > 1000000 {
			return fmt.Errorf("max release for %s cannot exceed 100%% (1000000 bps)", cfg.Category)
		}
		if cfg.HalfLifeBlocks == 0 {
			return fmt.Errorf("half-life for %s must be positive", cfg.Category)
		}
	}
	return nil
}

// ValidateParams validates vesting_rewards module parameters.
func ValidateParams(p *Params) error {
	if err := validateRevenueSplit(p.RevenueSplit); err != nil {
		return err
	}
	if err := validateProtocolSubSplit(p.ProtocolSubSplit); err != nil {
		return err
	}
	if p.BlocksPerRewardEpoch == 0 {
		return fmt.Errorf("blocks_per_reward_epoch must be positive")
	}
	if p.RewardDecayBps == 0 {
		return fmt.Errorf("reward_decay_bps must be positive")
	}
	if p.RewardDecayBps > 1000000 {
		return fmt.Errorf("reward_decay_bps cannot exceed 1000000 (1.0)")
	}
	if p.BlockReward == "" || p.BlockReward == "0" {
		return fmt.Errorf("block_reward must be positive")
	}
	if p.FounderShareBps > 1000000 {
		return fmt.Errorf("founder_share_bps cannot exceed 1000000 (100%%)")
	}
	if p.FounderShareBps > 0 && p.FounderAddress != "" {
		if _, err := sdk.AccAddressFromBech32(p.FounderAddress); err != nil {
			return fmt.Errorf("invalid founder_address: %w", err)
		}
	}
	return nil
}

// ValidateFounderShareImmutability ensures governance cannot modify founder share parameters.
// The founder share is a permanent protocol commitment — once set, it cannot be changed.
// Only the initial setting (from empty/zero to a value) is allowed.
func ValidateFounderShareImmutability(current *Params, proposed *Params) error {
	if current == nil || proposed == nil {
		return nil
	}

	// If founder share BPS was already set (>0), it cannot be changed
	if current.FounderShareBps > 0 && proposed.FounderShareBps != current.FounderShareBps {
		return ErrFounderShareImmutable
	}

	// If founder address was already set (non-empty), it cannot be changed
	if current.FounderAddress != "" && proposed.FounderAddress != current.FounderAddress {
		return ErrFounderShareImmutable
	}

	return nil
}

// validateRevenueSplit checks that the revenue split sums to 1,000,000.
func validateRevenueSplit(split *commontypes.RevenueSplit) error {
	if split == nil {
		return nil // defaults used
	}
	total := split.ContributorBps + split.ProtocolBps + split.ResearchBps + split.BurnBps
	if total != 1000000 {
		return fmt.Errorf("revenue split must sum to 1000000, got %d", total)
	}
	return nil
}

// validateProtocolSubSplit checks that the protocol sub-split sums to 1,000,000.
func validateProtocolSubSplit(split *commontypes.ProtocolSubSplit) error {
	if split == nil {
		return nil // defaults used
	}
	total := split.CitationBps + split.VerificationBps + split.TreasuryBps
	if total != 1000000 {
		return fmt.Errorf("protocol sub-split must sum to 1000000, got %d", total)
	}
	return nil
}
