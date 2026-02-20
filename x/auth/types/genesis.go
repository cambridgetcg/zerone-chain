package types

import "fmt"

// DefaultParams returns default module parameters.
func DefaultParams() Params {
	return Params{
		MaxSessionKeys:                5,
		MaxSessionDuration:            34272, // 1 day in blocks
		KeyRotationCooldown:           111,
		RecoveryDelayBlocks:           1000,  // ~42 min
		ChallengePeriodBlocks:         500,   // ~21 min
		BootstrapEnabled:              false,
		BootstrapAmount:               "0",
		MaxMetadataLength:             1024,
		RequireDid:                    false,
		MaxRecoveryShards:             10,
		RecoveryChallengePeriodBlocks: 500,
		RecoveryExecutionDelayBlocks:  1000,
	}
}

// DefaultGenesis returns default genesis state.
func DefaultGenesis() *GenesisState {
	params := DefaultParams()
	return &GenesisState{
		Params:          &params,
		Accounts:        []*Account{},
		DidMappings:     []*DIDMapping{},
		SessionKeys:     []*SessionKey{},
		RecoveryConfigs: []*RecoveryConfig{},
	}
}

// Validate validates genesis state.
func (gs *GenesisState) Validate() error {
	if gs.Params != nil {
		if err := gs.Params.Validate(); err != nil {
			return fmt.Errorf("invalid params: %w", err)
		}
	}

	seenAddrs := make(map[string]bool)
	seenDIDs := make(map[string]bool)
	for _, acc := range gs.Accounts {
		if acc == nil {
			continue
		}
		if seenAddrs[acc.Address] {
			return fmt.Errorf("duplicate address: %s", acc.Address)
		}
		seenAddrs[acc.Address] = true

		if seenDIDs[acc.Did] {
			return fmt.Errorf("duplicate DID: %s", acc.Did)
		}
		seenDIDs[acc.Did] = true

		if err := ValidateDID(acc.Did); err != nil {
			return fmt.Errorf("invalid DID for account %s: %w", acc.Address, err)
		}
	}

	for _, mapping := range gs.DidMappings {
		if mapping == nil {
			continue
		}
		if err := ValidateDID(mapping.Did); err != nil {
			return fmt.Errorf("invalid DID in mapping: %w", err)
		}
	}

	return nil
}
