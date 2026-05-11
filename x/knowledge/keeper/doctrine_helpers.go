package keeper

import (
	"context"

	"github.com/zerone-chain/zerone/x/knowledge/types"
)

// SetFactIfAbsent writes a Fact only if no Fact with the same ID
// already exists. Used by LoadDoctrineFacts at genesis (or upgrade)
// to make doctrine import idempotent — re-running the loader does
// not overwrite an existing doctrine Fact, preserving forward-only
// audit (commitment 10).
func (k Keeper) SetFactIfAbsent(ctx context.Context, f *types.Fact) error {
	if _, ok := k.GetFact(ctx, f.Id); ok {
		return nil
	}
	return k.SetFact(ctx, f)
}
