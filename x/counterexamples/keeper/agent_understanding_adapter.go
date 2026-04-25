package keeper

import (
	"context"

	auitypes "github.com/zerone-chain/zerone/x/agent_understanding/types"
	"github.com/zerone-chain/zerone/x/counterexamples/types"
)

// AgentUnderstandingAdapter aggregates counterexample stats by author
// for x/agent_understanding's per-agent profile composer.
type AgentUnderstandingAdapter struct {
	k Keeper
}

func NewAgentUnderstandingAdapter(k Keeper) *AgentUnderstandingAdapter {
	return &AgentUnderstandingAdapter{k: k}
}

// CounterexamplesByAuthor walks all counterexamples and tallies the
// ones authored by `author`. O(N) over all counterexamples; for
// testnet scale this is fine.
func (a *AgentUnderstandingAdapter) CounterexamplesByAuthor(ctx context.Context, author string) (uint64, uint64) {
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
		if c.Author != author {
			continue
		}
		authored++
		if c.Status == types.CounterexampleStatus_COUNTEREXAMPLE_STATUS_VALIDATED {
			validated++
		}
	}
	return authored, validated
}

// CounterexamplesByAuthorAndDomain scopes the per-author tally to
// counterexamples whose parent fact is in the named domain. The
// factDomain function is supplied by the caller (typically the
// knowledge adapter) so the counterexamples module does not have to
// depend on x/knowledge directly.
func (a *AgentUnderstandingAdapter) CounterexamplesByAuthorAndDomain(ctx context.Context, author, domain string, factDomain func(factID string) string) (uint64, uint64) {
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
		if c.Author != author {
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

var _ auitypes.CounterexamplesKeeper = (*AgentUnderstandingAdapter)(nil)
