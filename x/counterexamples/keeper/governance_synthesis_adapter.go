package keeper

import (
	"context"

	govsynthtypes "github.com/zerone-chain/zerone/x/governance_synthesis/types"
	"github.com/zerone-chain/zerone/x/counterexamples/types"
)

// GovernanceSynthesisAdapter aggregates per-domain counterexample
// counts for the Frontier synthesizer's coverage signal. The
// counterexamples module doesn't track parent-fact domains directly;
// the adapter takes a factDomain function (provided by the caller)
// to resolve fact_id → domain.
type GovernanceSynthesisAdapter struct {
	k Keeper
}

func NewGovernanceSynthesisAdapter(k Keeper) *GovernanceSynthesisAdapter {
	return &GovernanceSynthesisAdapter{k: k}
}

// CountCounterexamplesByDomain walks all counterexamples, resolves
// each parent fact's domain via factDomain, and tallies authored +
// validated for the requested domain.
//
// O(N) over total counterexamples per call. For testnet this is
// acceptable; mainnet should add a by-domain index in a future v2
// (the synthesizer is opt-in so this scales like the rest of
// governance_synthesis).
func (a *GovernanceSynthesisAdapter) CountCounterexamplesByDomain(ctx context.Context, domain string, factDomain func(factID string) string) (uint64, uint64) {
	if factDomain == nil {
		return 0, 0
	}
	var authored, validated uint64
	st := a.k.storeService.OpenKVStore(ctx)
	it, err := st.Iterator(types.CounterexampleKeyPrefix, nil)
	if err != nil {
		return 0, 0
	}
	defer it.Close()
	for ; it.Valid(); it.Next() {
		key := it.Key()
		if len(key) < len(types.CounterexampleKeyPrefix) ||
			!bytesEqual(key[:len(types.CounterexampleKeyPrefix)], types.CounterexampleKeyPrefix) {
			break
		}
		var c types.Counterexample
		if err := a.k.cdc.Unmarshal(it.Value(), &c); err != nil {
			continue
		}
		if factDomain(c.FactId) != domain {
			continue
		}
		authored++
		if c.Status == types.CounterexampleStatus_COUNTEREXAMPLE_STATUS_VALIDATED {
			validated++
		}
	}
	return authored, validated
}

var _ govsynthtypes.FrontierCounterexamplesKeeper = (*GovernanceSynthesisAdapter)(nil)
