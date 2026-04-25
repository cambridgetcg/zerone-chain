package keeper

import (
	"context"

	cetypes "github.com/zerone-chain/zerone/x/counterexamples/types"
)

// CounterexamplesFactAdapter wraps a knowledge Keeper to expose just
// the FactExists check that x/counterexamples needs to refuse
// counterexamples that anchor to non-existent facts.
type CounterexamplesFactAdapter struct {
	k Keeper
}

func NewCounterexamplesFactAdapter(k Keeper) *CounterexamplesFactAdapter {
	return &CounterexamplesFactAdapter{k: k}
}

// FactExists returns true iff the fact ID resolves to an actual fact.
// Compile-time interface check below.
func (a *CounterexamplesFactAdapter) FactExists(ctx context.Context, factID string) bool {
	if factID == "" {
		return false
	}
	_, ok := a.k.GetFact(ctx, factID)
	return ok
}

var _ cetypes.FactExistenceKeeper = (*CounterexamplesFactAdapter)(nil)
