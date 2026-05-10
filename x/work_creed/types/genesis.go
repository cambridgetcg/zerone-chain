package types

import (
	"bytes"
	"fmt"

	"cosmossdk.io/errors"
)

// DefaultGenesis returns the default genesis state — empty at Phase 0
// without genesis pins. The app.go genesis populator inserts the
// inception pins at chain genesis (see Task 19's keeper.InitGenesis).
func DefaultGenesis() *GenesisState {
	return &GenesisState{
		PinnedSubCreeds: []*PinnedSubCreed{},
	}
}

// Validate ensures genesis state invariants hold:
//   - phase numbers in [0, 8]
//   - phase 1 (Knowledge) NEVER pinned in x/work_creed (delegates)
//   - per-phase versions are dense from 1 (each phase has exactly one
//     pin per version, no gaps)
//   - canonical_hash is exactly 32 bytes
//   - phase + version pair is unique
func (g *GenesisState) Validate() error {
	versionsByPhase := map[uint32]map[uint32]bool{}
	for i, p := range g.PinnedSubCreeds {
		if p.Phase > 8 {
			return errors.Wrapf(ErrUnknownPhase, "PinnedSubCreed[%d]: phase %d out of range [0, 8]", i, p.Phase)
		}
		if p.Phase == 1 {
			return fmt.Errorf("PinnedSubCreed[%d]: Knowledge phase delegates to x/creed and must not be pinned here", i)
		}
		if len(p.CanonicalHash) != 32 {
			return fmt.Errorf("PinnedSubCreed[%d]: canonical_hash must be 32 bytes, got %d", i, len(p.CanonicalHash))
		}
		if versionsByPhase[p.Phase] == nil {
			versionsByPhase[p.Phase] = map[uint32]bool{}
		}
		if versionsByPhase[p.Phase][p.Version] {
			return fmt.Errorf("PinnedSubCreed[%d]: duplicate (phase=%d, version=%d)", i, p.Phase, p.Version)
		}
		versionsByPhase[p.Phase][p.Version] = true
	}
	// Density check: for each phase that has any pin, versions must be 1..N dense.
	for phase, versions := range versionsByPhase {
		for v := uint32(1); v <= uint32(len(versions)); v++ {
			if !versions[v] {
				return fmt.Errorf("phase %d missing version %d (must be dense from 1)", phase, v)
			}
		}
	}
	return nil
}

// Equal compares two GenesisState values byte-for-byte over their
// PinnedSubCreed entries.
func (g *GenesisState) Equal(other *GenesisState) bool {
	if len(g.PinnedSubCreeds) != len(other.PinnedSubCreeds) {
		return false
	}
	for i, p := range g.PinnedSubCreeds {
		o := other.PinnedSubCreeds[i]
		if p.Phase != o.Phase ||
			p.PhaseName != o.PhaseName ||
			p.Version != o.Version ||
			!bytes.Equal(p.CanonicalHash, o.CanonicalHash) ||
			p.AnchoredAtBlock != o.AnchoredAtBlock ||
			p.SourceLip != o.SourceLip {
			return false
		}
		if len(p.CommitmentCodes) != len(o.CommitmentCodes) {
			return false
		}
		for j, c := range p.CommitmentCodes {
			if c != o.CommitmentCodes[j] {
				return false
			}
		}
	}
	return true
}
