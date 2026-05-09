package types

// DefaultParams returns sensible defaults for pre-launch and
// development chains. Mainnet genesis SHOULD override Authority
// to the gov module account address and set DirectAnchorEnabled
// to false once x/gov.CategoryCreedAmendment ships.
func DefaultParams() *Params {
	return &Params{
		Authority:           "",  // app-init wires this to the gov module account
		DirectAnchorEnabled: true, // pre-launch and dev chains
	}
}

// DefaultGenesis returns a non-pinned starting state. Real chains
// MUST replace GenesisPin with the canonical Genesis Creed before
// any commitment-citing event passes CI's hash check.
func DefaultGenesis() *GenesisState {
	return &GenesisState{
		Params:     DefaultParams(),
		GenesisPin: nil,
		History:    nil,
	}
}

// Validate checks that the genesis state is internally consistent
// before InitGenesis runs.
func (gs *GenesisState) Validate() error {
	if gs.Params == nil {
		return ErrInvalidParams.Wrap("params must not be nil")
	}
	if gs.GenesisPin != nil {
		if err := validatePin(gs.GenesisPin); err != nil {
			return err
		}
	}
	prevVersion := uint32(0)
	for i, p := range gs.History {
		if p == nil {
			return ErrInvalidParams.Wrapf("history[%d] is nil", i)
		}
		if err := validatePin(p); err != nil {
			return err
		}
		if p.Version <= prevVersion {
			return ErrVersionNotMonotonic.Wrapf("history[%d] version %d not strictly greater than previous %d", i, p.Version, prevVersion)
		}
		prevVersion = p.Version
	}
	if gs.GenesisPin != nil && gs.GenesisPin.Version <= prevVersion {
		return ErrVersionNotMonotonic.Wrapf("genesis_pin version %d must be greater than all history entries (last %d)", gs.GenesisPin.Version, prevVersion)
	}
	return nil
}

// validatePin enforces the structural invariants commitment 10
// names: monotonic version, non-empty hash, unique and contiguous
// commitment numbers (modulo archived entries).
func validatePin(p *PinnedCreed) error {
	if p.Version == 0 {
		return ErrVersionNotMonotonic.Wrap("version must be ≥ 1")
	}
	if len(p.CanonicalHash) == 0 {
		return ErrEmptyHash
	}
	seen := map[uint32]bool{}
	maxNum := uint32(0)
	for _, c := range p.Commitments {
		if c == nil {
			return ErrCommitmentNumberInvalid.Wrap("nil commitment entry")
		}
		if c.Number == 0 {
			return ErrCommitmentNumberInvalid.Wrap("commitment number must be ≥ 1")
		}
		if seen[c.Number] {
			return ErrDuplicateCommitment.Wrapf("commitment %d", c.Number)
		}
		seen[c.Number] = true
		if c.Number > maxNum {
			maxNum = c.Number
		}
	}
	// Numbers 1..maxNum must all be present (archived or active).
	// This prevents accidental drop of a commitment without a
	// corresponding archive transition.
	for n := uint32(1); n <= maxNum; n++ {
		if !seen[n] {
			return ErrCommitmentNumberInvalid.Wrapf("commitment %d missing — archive an entry rather than dropping it", n)
		}
	}
	return nil
}
