package keeper

import (
	"context"

	"google.golang.org/protobuf/proto"

	"github.com/zerone-chain/zerone/x/knowledge/types"
)

// IndexCompletedRound stores completion metadata indexed by verdict block height.
// Called from CompleteRound to enable efficient window-based metrics (R31-2).
func (k Keeper) IndexCompletedRound(ctx context.Context, verdictBlock uint64, roundID string, meta *types.CompletedRoundMeta) error {
	store := k.storeService.OpenKVStore(ctx)
	bz, err := marshalOpts.Marshal(meta)
	if err != nil {
		return err
	}
	return store.Set(types.CompletedRoundKey(verdictBlock, roundID), bz)
}

// CountCompletedRoundsInWindow counts all completed rounds in [height-window, height].
func (k Keeper) CountCompletedRoundsInWindow(ctx context.Context, height, windowBlocks uint64) uint64 {
	var count uint64
	k.iterateCompletedRoundsInWindow(ctx, height, windowBlocks, func(_ *types.CompletedRoundMeta) bool {
		count++
		return false
	})
	return count
}

// CountDisputedRoundsInWindow counts rounds with dissent in [height-window, height].
func (k Keeper) CountDisputedRoundsInWindow(ctx context.Context, height, windowBlocks uint64) uint64 {
	var count uint64
	k.iterateCompletedRoundsInWindow(ctx, height, windowBlocks, func(meta *types.CompletedRoundMeta) bool {
		if meta.HasDissent {
			count++
		}
		return false
	})
	return count
}

// GetAvgRoundDurationInWindow returns the average round duration in [height-window, height].
func (k Keeper) GetAvgRoundDurationInWindow(ctx context.Context, height, windowBlocks uint64) uint64 {
	var total, count uint64
	k.iterateCompletedRoundsInWindow(ctx, height, windowBlocks, func(meta *types.CompletedRoundMeta) bool {
		total += meta.DurationBlocks
		count++
		return false
	})
	if count == 0 {
		return 0
	}
	return total / count
}

// CountCompletedRoundsForDomainInWindow counts completed rounds for a specific domain.
func (k Keeper) CountCompletedRoundsForDomainInWindow(ctx context.Context, domain string, height, windowBlocks uint64) uint64 {
	var count uint64
	k.iterateCompletedRoundsInWindow(ctx, height, windowBlocks, func(meta *types.CompletedRoundMeta) bool {
		if meta.Domain == domain {
			count++
		}
		return false
	})
	return count
}

// iterateCompletedRoundsInWindow iterates all completed round metadata in [startBlock, endBlock].
func (k Keeper) iterateCompletedRoundsInWindow(ctx context.Context, height, windowBlocks uint64, cb func(*types.CompletedRoundMeta) bool) {
	store := k.storeService.OpenKVStore(ctx)

	var startBlock uint64
	if height > windowBlocks {
		startBlock = height - windowBlocks
	}

	startKey := types.CompletedRoundBlockPrefix(startBlock)
	endKey := types.CompletedRoundBlockPrefix(height + 1)

	iter, err := store.Iterator(startKey, endKey)
	if err != nil {
		return
	}
	defer iter.Close()

	for ; iter.Valid(); iter.Next() {
		var meta types.CompletedRoundMeta
		if err := proto.Unmarshal(iter.Value(), &meta); err != nil {
			continue
		}
		if cb(&meta) {
			return
		}
	}
}
