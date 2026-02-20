package keeper

import (
	"context"

	"github.com/zerone-chain/zerone/x/knowledge/types"
)

// InitGenesis initializes the module state from a genesis state.
func (k Keeper) InitGenesis(ctx context.Context, gs *types.GenesisState) error {
	if gs.Params != nil {
		if err := k.SetParams(ctx, gs.Params); err != nil {
			return err
		}
	}

	for _, domain := range gs.Domains {
		if domain == nil {
			continue
		}
		if err := k.SetDomain(ctx, domain); err != nil {
			return err
		}
	}

	for _, fact := range gs.Facts {
		if fact == nil {
			continue
		}
		if err := k.SetFact(ctx, fact); err != nil {
			return err
		}
	}

	for _, claim := range gs.PendingClaims {
		if claim == nil {
			continue
		}
		if err := k.SetClaim(ctx, claim); err != nil {
			return err
		}
	}

	for _, round := range gs.ActiveRounds {
		if round == nil {
			continue
		}
		if err := k.SetVerificationRound(ctx, round); err != nil {
			return err
		}
	}

	return nil
}

// ExportGenesis exports the current module state as a genesis state.
func (k Keeper) ExportGenesis(ctx context.Context) *types.GenesisState {
	params, err := k.GetParams(ctx)
	if err != nil {
		p := types.DefaultParams()
		params = &p
	}

	var facts []*types.Fact
	k.IterateFacts(ctx, func(fact *types.Fact) bool {
		facts = append(facts, fact)
		return false
	})

	var claims []*types.Claim
	k.IterateClaims(ctx, func(claim *types.Claim) bool {
		claims = append(claims, claim)
		return false
	})

	var domains []*types.Domain
	k.IterateDomains(ctx, func(domain *types.Domain) bool {
		domains = append(domains, domain)
		return false
	})

	var rounds []*types.VerificationRound
	k.IterateActiveRounds(ctx, func(round *types.VerificationRound) bool {
		rounds = append(rounds, round)
		return false
	})

	return &types.GenesisState{
		Params:        params,
		Facts:         facts,
		PendingClaims: claims,
		Domains:       domains,
		ActiveRounds:  rounds,
	}
}
