package types

import (
	"bytes"
	"fmt"
)

// DefaultGenesis returns the default (empty) genesis state.
func DefaultGenesis() *GenesisState {
	return &GenesisState{
		Contributions: []*Contribution{},
	}
}

// Validate performs basic genesis-state validation:
//   - id is non-empty (32 bytes for sha256)
//   - status is in valid range
//   - id appears at most once
func (g GenesisState) Validate() error {
	seen := map[string]bool{}
	for i, c := range g.Contributions {
		if c == nil {
			return fmt.Errorf("Contributions[%d] is nil", i)
		}
		if len(c.Id) != 32 {
			return fmt.Errorf("Contributions[%d]: id must be 32 bytes (got %d)", i, len(c.Id))
		}
		if c.Status < ContributionStatus_STATUS_UNSPECIFIED || c.Status > ContributionStatus_STATUS_ADMISSION_FAILED {
			return fmt.Errorf("Contributions[%d]: status %d out of range", i, c.Status)
		}
		if c.Class < ContributionClass_KNOWLEDGE_CLAIM || c.Class > ContributionClass_PIPELINE_IMPROVEMENT {
			return fmt.Errorf("Contributions[%d]: class %d out of range", i, c.Class)
		}
		if c.Phase < LifecyclePhase_PHASE_FOUNDATION || c.Phase > LifecyclePhase_PHASE_TOOLS {
			return fmt.Errorf("Contributions[%d]: phase %d out of range", i, c.Phase)
		}
		key := string(c.Id)
		if seen[key] {
			return fmt.Errorf("Contributions[%d]: duplicate id %x", i, c.Id)
		}
		seen[key] = true
	}
	return nil
}

// Equal compares two GenesisState values byte-for-byte.
func (g GenesisState) Equal(other GenesisState) bool {
	if len(g.Contributions) != len(other.Contributions) {
		return false
	}
	for i, a := range g.Contributions {
		b := other.Contributions[i]
		if !bytes.Equal(a.Id, b.Id) || a.Status != b.Status || a.Class != b.Class || a.Phase != b.Phase {
			return false
		}
	}
	return true
}
