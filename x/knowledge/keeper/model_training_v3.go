package keeper

import (
	"context"
	"fmt"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"google.golang.org/protobuf/proto"

	"github.com/zerone-chain/zerone/x/knowledge/types"
)

// ─── TrainingAttestation (Route B Wave 3c) ───────────────────────────────

// SetTrainingAttestation stores the attestation for a pipeline.
func (k Keeper) SetTrainingAttestation(ctx context.Context, a *types.TrainingAttestation) error {
	if a == nil || a.PipelineId == "" {
		return fmt.Errorf("invalid training attestation")
	}
	store := k.storeService.OpenKVStore(ctx)
	bz, err := marshalOpts.Marshal(a)
	if err != nil {
		return err
	}
	return store.Set(types.TrainingAttestationKey(a.PipelineId), bz)
}

// GetTrainingAttestation fetches the attestation for a pipeline.
func (k Keeper) GetTrainingAttestation(ctx context.Context, pipelineID string) (*types.TrainingAttestation, bool) {
	if pipelineID == "" {
		return nil, false
	}
	store := k.storeService.OpenKVStore(ctx)
	bz, err := store.Get(types.TrainingAttestationKey(pipelineID))
	if err != nil || bz == nil {
		return nil, false
	}
	var a types.TrainingAttestation
	if err := proto.Unmarshal(bz, &a); err != nil {
		return nil, false
	}
	return &a, true
}

// ─── ContributionRecord (Route B Wave 3b) ────────────────────────────────

// SetContributionRecord stores the per-model record and its reverse index.
// The reverse index marks every (fact_id → model_id) pair so FactContributors
// returns in O(n) per fact rather than scanning all models.
func (k Keeper) SetContributionRecord(ctx context.Context, r *types.ContributionRecord) error {
	if r == nil || r.ModelId == "" {
		return fmt.Errorf("invalid contribution record")
	}
	store := k.storeService.OpenKVStore(ctx)
	bz, err := marshalOpts.Marshal(r)
	if err != nil {
		return err
	}
	if err := store.Set(types.ContributionByModelKey(r.ModelId), bz); err != nil {
		return err
	}
	// Reverse index — one marker per (fact, model) pair.
	for _, factID := range r.FactIds {
		if factID == "" {
			continue
		}
		if err := store.Set(types.ContributionByFactKey(factID, r.ModelId), []byte{1}); err != nil {
			return err
		}
	}
	return nil
}

// GetContributionRecord fetches the record for a model.
func (k Keeper) GetContributionRecord(ctx context.Context, modelID string) (*types.ContributionRecord, bool) {
	if modelID == "" {
		return nil, false
	}
	store := k.storeService.OpenKVStore(ctx)
	bz, err := store.Get(types.ContributionByModelKey(modelID))
	if err != nil || bz == nil {
		return nil, false
	}
	var r types.ContributionRecord
	if err := proto.Unmarshal(bz, &r); err != nil {
		return nil, false
	}
	return &r, true
}

// GetModelsThatUsedFact walks the reverse index to return all models that
// used a given fact in training (Route B Wave 3b).
func (k Keeper) GetModelsThatUsedFact(ctx context.Context, factID string) []string {
	if factID == "" {
		return nil
	}
	store := k.storeService.OpenKVStore(ctx)
	prefix := types.ContributionByFactPrefix(factID)
	iter, err := store.Iterator(prefix, prefixEndBytes(prefix))
	if err != nil {
		return nil
	}
	defer iter.Close()
	var out []string
	for ; iter.Valid(); iter.Next() {
		key := iter.Key()
		// key = prefix || modelID
		modelID := string(key[len(prefix):])
		out = append(out, modelID)
	}
	return out
}

// ─── AugmentationBounty (Route B Wave 3e) ────────────────────────────────

// SetAugmentationBounty stores a bounty record.
func (k Keeper) SetAugmentationBounty(ctx context.Context, b *types.AugmentationBounty) error {
	if b == nil || b.Id == "" {
		return fmt.Errorf("invalid augmentation bounty")
	}
	store := k.storeService.OpenKVStore(ctx)
	bz, err := marshalOpts.Marshal(b)
	if err != nil {
		return err
	}
	return store.Set(types.AugmentationBountyKey(b.Id), bz)
}

// GetAugmentationBounty fetches a bounty.
func (k Keeper) GetAugmentationBounty(ctx context.Context, id string) (*types.AugmentationBounty, bool) {
	if id == "" {
		return nil, false
	}
	store := k.storeService.OpenKVStore(ctx)
	bz, err := store.Get(types.AugmentationBountyKey(id))
	if err != nil || bz == nil {
		return nil, false
	}
	var b types.AugmentationBounty
	if err := proto.Unmarshal(bz, &b); err != nil {
		return nil, false
	}
	return &b, true
}

// IterateAugmentationBounties yields every bounty.
func (k Keeper) IterateAugmentationBounties(ctx context.Context, cb func(*types.AugmentationBounty) bool) {
	store := k.storeService.OpenKVStore(ctx)
	iter, err := store.Iterator(types.AugmentationBountyKeyPrefix, prefixEndBytes(types.AugmentationBountyKeyPrefix))
	if err != nil {
		return
	}
	defer iter.Close()
	for ; iter.Valid(); iter.Next() {
		var b types.AugmentationBounty
		if err := proto.Unmarshal(iter.Value(), &b); err != nil {
			continue
		}
		if cb(&b) {
			return
		}
	}
}

// ─── Augmentation (Route B Wave 3e) ──────────────────────────────────────

// SetAugmentation stores the augmentation record and its two reverse indexes.
func (k Keeper) SetAugmentation(ctx context.Context, a *types.Augmentation) error {
	if a == nil || a.Id == "" {
		return fmt.Errorf("invalid augmentation")
	}
	store := k.storeService.OpenKVStore(ctx)
	bz, err := marshalOpts.Marshal(a)
	if err != nil {
		return err
	}
	if err := store.Set(types.AugmentationKey(a.Id), bz); err != nil {
		return err
	}
	// Reverse indexes: by original fact and by bounty (if any).
	if err := store.Set(types.AugmentationByFactKey(a.OriginalFactId, a.Id), []byte{1}); err != nil {
		return err
	}
	if a.BountyId != "" {
		if err := store.Set(types.AugmentationByBountyKey(a.BountyId, a.Id), []byte{1}); err != nil {
			return err
		}
	}
	return nil
}

// GetAugmentation fetches one by id.
func (k Keeper) GetAugmentation(ctx context.Context, id string) (*types.Augmentation, bool) {
	if id == "" {
		return nil, false
	}
	store := k.storeService.OpenKVStore(ctx)
	bz, err := store.Get(types.AugmentationKey(id))
	if err != nil || bz == nil {
		return nil, false
	}
	var a types.Augmentation
	if err := proto.Unmarshal(bz, &a); err != nil {
		return nil, false
	}
	return &a, true
}

// GetAugmentationsByFact returns all augmentations whose original is factID.
func (k Keeper) GetAugmentationsByFact(ctx context.Context, factID string) []*types.Augmentation {
	if factID == "" {
		return nil
	}
	store := k.storeService.OpenKVStore(ctx)
	prefix := types.AugmentationByFactPrefix(factID)
	iter, err := store.Iterator(prefix, prefixEndBytes(prefix))
	if err != nil {
		return nil
	}
	defer iter.Close()
	var out []*types.Augmentation
	for ; iter.Valid(); iter.Next() {
		key := iter.Key()
		augID := string(key[len(prefix):])
		a, ok := k.GetAugmentation(ctx, augID)
		if ok {
			out = append(out, a)
		}
	}
	return out
}

// GetAugmentationsByBounty returns all augmentations submitted against a bounty.
func (k Keeper) GetAugmentationsByBounty(ctx context.Context, bountyID string) []*types.Augmentation {
	if bountyID == "" {
		return nil
	}
	store := k.storeService.OpenKVStore(ctx)
	prefix := types.AugmentationByBountyPrefix(bountyID)
	iter, err := store.Iterator(prefix, prefixEndBytes(prefix))
	if err != nil {
		return nil
	}
	defer iter.Close()
	var out []*types.Augmentation
	for ; iter.Valid(); iter.Next() {
		key := iter.Key()
		augID := string(key[len(prefix):])
		a, ok := k.GetAugmentation(ctx, augID)
		if ok {
			out = append(out, a)
		}
	}
	return out
}

// ─── Model lineage traversal (Route B Wave 3d) ───────────────────────────

// WalkModelAncestry returns ancestor ModelCards ordered oldest→youngest
// terminating either at a root (predecessor_model_id == "") or at maxDepth.
// truncated=true when we hit maxDepth before reaching a root.
func (k Keeper) WalkModelAncestry(ctx context.Context, modelID string, maxDepth uint32) (ancestry []*types.ModelCard, rootReached bool, truncated bool) {
	if maxDepth == 0 {
		maxDepth = 10
	}
	visited := make(map[string]bool)
	current, ok := k.GetModelCard(ctx, modelID)
	if !ok {
		return nil, false, false
	}
	// Walk backward.
	var chain []*types.ModelCard
	for {
		chain = append(chain, current)
		visited[current.Id] = true
		if current.PredecessorModelId == "" {
			rootReached = true
			break
		}
		if uint32(len(chain)) >= maxDepth {
			truncated = true
			break
		}
		if visited[current.PredecessorModelId] {
			// Cycle — stop defensively.
			break
		}
		parent, ok := k.GetModelCard(ctx, current.PredecessorModelId)
		if !ok {
			// Dangling reference.
			break
		}
		current = parent
	}
	// Reverse so oldest-first.
	for i, j := 0, len(chain)-1; i < j; i, j = i+1, j-1 {
		chain[i], chain[j] = chain[j], chain[i]
	}
	return chain, rootReached, truncated
}

// EmitTrainingAttestationEvent emits the attestation event.
func (k Keeper) EmitTrainingAttestationEvent(ctx context.Context, a *types.TrainingAttestation) {
	if a == nil {
		return
	}
	sdk.UnwrapSDKContext(ctx).EventManager().EmitEvent(sdk.NewEvent(
		"zerone.knowledge.training_attestation_posted",
		sdk.NewAttribute("pipeline_id", a.PipelineId),
		sdk.NewAttribute("attester", a.AttesterAddress),
		sdk.NewAttribute("flops_estimate", fmt.Sprintf("%d", a.FlopsEstimate)),
		sdk.NewAttribute("wallclock_seconds", fmt.Sprintf("%d", a.WallclockSeconds)),
		sdk.NewAttribute("eval_hash", a.EvalHash),
	))
}
