package types

import "fmt"

// DefaultParams returns the default module parameters (all disabled at genesis).
func DefaultParams() Params {
	return Params{
		EmissionEpochBlocks: 0,  // disabled
		DefaultFeeBps:       "", // unused, reserved
	}
}

// DefaultGenesis returns the default genesis state.
func DefaultGenesis() *GenesisState {
	p := DefaultParams()
	return &GenesisState{
		Params: &p,
	}
}

// Validate validates the genesis state.
func (gs *GenesisState) Validate() error {
	if gs.Params == nil {
		return fmt.Errorf("params must not be nil")
	}
	return gs.Params.Validate()
}

// Validate validates the Params struct.
func (p *Params) Validate() error {
	// All params are currently optional/zero-ok.
	return nil
}
