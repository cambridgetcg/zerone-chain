package types

import "fmt"

// DefaultParams returns the default module parameters.
func DefaultParams() *Params {
	return &Params{
		MaxKeysPerHome:       20,
		MaxSessionsPerHome:   5,
		SessionTimeoutBlocks: 1000,
		DeadmanMinThreshold:  100,
		DeadmanMaxThreshold:  100000,
		MaxAlertsPerHome:     100,
		HomeCreationFee:      "10000000", // 10 ZRN
		MaxRecoveryAddresses: 5,
	}
}

// DefaultGenesis returns the default genesis state.
func DefaultGenesis() *GenesisState {
	return &GenesisState{
		Params:  DefaultParams(),
		Homes:   []*AgentHome{},
		KeySets: []*HomeKeySet{},
	}
}

// Validate validates the genesis state.
func (gs *GenesisState) Validate() error {
	if gs.Params != nil {
		if err := gs.Params.Validate(); err != nil {
			return err
		}
	}
	homeIDs := make(map[string]bool)
	for _, home := range gs.Homes {
		if homeIDs[home.HomeId] {
			return fmt.Errorf("duplicate home ID: %s", home.HomeId)
		}
		homeIDs[home.HomeId] = true
	}
	return nil
}

// Validate validates the module parameters.
func (p *Params) Validate() error {
	if p.MaxKeysPerHome == 0 {
		return fmt.Errorf("max_keys_per_home must be positive")
	}
	if p.MaxSessionsPerHome == 0 {
		return fmt.Errorf("max_sessions_per_home must be positive")
	}
	if p.SessionTimeoutBlocks == 0 {
		return fmt.Errorf("session_timeout_blocks must be positive")
	}
	if p.DeadmanMinThreshold == 0 {
		return fmt.Errorf("deadman_min_threshold must be positive")
	}
	if p.DeadmanMaxThreshold <= p.DeadmanMinThreshold {
		return fmt.Errorf("deadman_max_threshold must be greater than min")
	}
	if p.MaxAlertsPerHome == 0 {
		return fmt.Errorf("max_alerts_per_home must be positive")
	}
	return nil
}
