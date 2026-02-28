package keeper

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/zerone-chain/zerone/x/knowledge/types"
)

// SetDomainEpistemicState stores the epistemic temperature state for a domain.
func (k Keeper) SetDomainEpistemicState(ctx context.Context, state *types.DomainEpistemicState) error {
	store := k.storeService.OpenKVStore(ctx)
	bz, err := json.Marshal(state)
	if err != nil {
		return fmt.Errorf("failed to marshal DomainEpistemicState: %w", err)
	}
	return store.Set(types.EpistemicStateKey(state.Domain), bz)
}

// GetDomainEpistemicState retrieves the epistemic temperature state for a domain.
func (k Keeper) GetDomainEpistemicState(ctx context.Context, domain string) (types.DomainEpistemicState, bool, error) {
	store := k.storeService.OpenKVStore(ctx)
	bz, err := store.Get(types.EpistemicStateKey(domain))
	if err != nil {
		return types.DomainEpistemicState{}, false, err
	}
	if bz == nil {
		return types.DomainEpistemicState{}, false, nil
	}
	var state types.DomainEpistemicState
	if err := json.Unmarshal(bz, &state); err != nil {
		return types.DomainEpistemicState{}, false, fmt.Errorf("failed to unmarshal DomainEpistemicState: %w", err)
	}
	return state, true, nil
}

// GetOrInitDomainEpistemicState returns existing state or creates neutral state.
func (k Keeper) GetOrInitDomainEpistemicState(ctx context.Context, domain string) (types.DomainEpistemicState, error) {
	state, found, err := k.GetDomainEpistemicState(ctx, domain)
	if err != nil {
		return types.DomainEpistemicState{}, err
	}
	if !found {
		return types.DomainEpistemicState{
			Domain:      domain,
			Temperature: NeutralBPS, // 500,000 = neutral
		}, nil
	}
	return state, nil
}
