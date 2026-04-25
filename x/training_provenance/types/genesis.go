package types

// DefaultGenesis returns the empty genesis state. The module owns no
// persistent state — it's a pure read-only synthesizer.
func DefaultGenesis() *GenesisState {
	return &GenesisState{}
}

// Validate is a no-op for the empty genesis.
func (gs *GenesisState) Validate() error {
	return nil
}
