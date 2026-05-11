package keeper

import (
	"context"

	storetypes "cosmossdk.io/store/types"
	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/zerone-chain/zerone/x/substrate_bridge/types"
)

// CreateLineageEdge writes a LineageEdge after enforcing:
//   - both attestations exist
//   - upstream.SubmittedAtBlock < downstream.SubmittedAtBlock (DAG cycle prevention)
//   - if same submitter, contribution_share_bps <= self_citation_cap_bps
//   - citation_type is specified
//
// Forward/backward indexes maintained.
func (k Keeper) CreateLineageEdge(ctx context.Context, e *types.LineageEdge) error {
	if e == nil || e.UpstreamAttestationId == "" || e.DownstreamAttestationId == "" {
		return types.ErrAttestationNotFound
	}
	if e.CitationType == types.CitationType_CITATION_TYPE_UNSPECIFIED {
		return types.ErrInvalidCitationType
	}

	upstream, foundU := k.GetAttestation(ctx, e.UpstreamAttestationId)
	if !foundU {
		return types.ErrAttestationNotFound
	}
	downstream, foundD := k.GetAttestation(ctx, e.DownstreamAttestationId)
	if !foundD {
		return types.ErrAttestationNotFound
	}

	if upstream.SubmittedAtBlock >= downstream.SubmittedAtBlock {
		return types.ErrLineageCycle
	}

	if upstream.Submitter == downstream.Submitter {
		params := k.GetParams(ctx)
		if e.ContributionShareBps > params.SelfCitationCapBps {
			return types.ErrSelfCitationCapExceeded
		}
	}

	e.UpstreamClassId = upstream.WorkClassId
	e.DownstreamClassId = downstream.WorkClassId
	if e.CreatedAtBlock == 0 {
		e.CreatedAtBlock = uint64(sdk.UnwrapSDKContext(ctx).BlockHeight())
	}
	if e.DepthFromDownstream == 0 {
		e.DepthFromDownstream = 1
	}

	edgeID := types.EdgeID(e.UpstreamAttestationId, e.DownstreamAttestationId)
	store := sdk.UnwrapSDKContext(ctx).KVStore(k.storeKey)
	store.Set(types.LineageEdgeKey(edgeID), k.cdc.MustMarshal(e))
	store.Set(types.LineageByUpstreamKey(e.UpstreamAttestationId, edgeID), []byte{0x01})
	store.Set(types.LineageByDownstreamKey(e.DownstreamAttestationId, edgeID), []byte{0x01})
	return nil
}

func (k Keeper) GetLineageEdge(ctx context.Context, edgeID string) (*types.LineageEdge, bool) {
	store := sdk.UnwrapSDKContext(ctx).KVStore(k.storeKey)
	bz := store.Get(types.LineageEdgeKey(edgeID))
	if bz == nil {
		return nil, false
	}
	var e types.LineageEdge
	if err := k.cdc.Unmarshal(bz, &e); err != nil {
		return nil, false
	}
	return &e, true
}

func (k Keeper) IterateForwardLineage(ctx context.Context, upstreamID string, cb func(*types.LineageEdge) bool) {
	store := sdk.UnwrapSDKContext(ctx).KVStore(k.storeKey)
	prefix := types.LineageByUpstreamPrefixFor(upstreamID)
	iter := storetypes.KVStorePrefixIterator(store, prefix)
	defer iter.Close()
	prefixLen := len(prefix)
	for ; iter.Valid(); iter.Next() {
		edgeID := string(iter.Key()[prefixLen:])
		if e, ok := k.GetLineageEdge(ctx, edgeID); ok {
			if cb(e) {
				return
			}
		}
	}
}

func (k Keeper) IterateBackwardLineage(ctx context.Context, downstreamID string, cb func(*types.LineageEdge) bool) {
	store := sdk.UnwrapSDKContext(ctx).KVStore(k.storeKey)
	prefix := types.LineageByDownstreamPrefixFor(downstreamID)
	iter := storetypes.KVStorePrefixIterator(store, prefix)
	defer iter.Close()
	prefixLen := len(prefix)
	for ; iter.Valid(); iter.Next() {
		edgeID := string(iter.Key()[prefixLen:])
		if e, ok := k.GetLineageEdge(ctx, edgeID); ok {
			if cb(e) {
				return
			}
		}
	}
}

func (k Keeper) GetLineageAccumulator(ctx context.Context, attestationID string) (*types.LineageRoyaltyAccumulator, bool) {
	store := sdk.UnwrapSDKContext(ctx).KVStore(k.storeKey)
	bz := store.Get(types.LineageRoyaltyAccumulatorKey(attestationID))
	if bz == nil {
		return nil, false
	}
	var a types.LineageRoyaltyAccumulator
	if err := k.cdc.Unmarshal(bz, &a); err != nil {
		return nil, false
	}
	return &a, true
}

func (k Keeper) WriteLineageAccumulator(ctx context.Context, a *types.LineageRoyaltyAccumulator) {
	store := sdk.UnwrapSDKContext(ctx).KVStore(k.storeKey)
	store.Set(types.LineageRoyaltyAccumulatorKey(a.AttestationId), k.cdc.MustMarshal(a))
}
