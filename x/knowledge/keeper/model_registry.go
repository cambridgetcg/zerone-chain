package keeper

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strconv"

	sdkmath "cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/zerone-chain/zerone/x/knowledge/types"
)

// ─── PublishModel ───────────────────────────────────────────────────────────

// PublishModel registers a trained model on-chain after quality validation.
// It verifies the linked training record exists, validates quality thresholds,
// assigns a version, builds the lineage chain, and indexes by domain + TDU.
func (k Keeper) PublishModel(ctx context.Context, msg *types.MsgPublishModel) (*types.MsgPublishModelResponse, error) {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	kvStore := k.storeService.OpenKVStore(ctx)

	// Generate deterministic model ID from attestation + model hash.
	idInput := msg.TEEAttestation + ":" + msg.ModelHash
	idHash := sha256.Sum256([]byte(idInput))
	modelID := hex.EncodeToString(idHash[:])

	// Check for duplicate.
	existingKey := types.ModelRecordKey(modelID)
	existing, err := kvStore.Get(existingKey)
	if err != nil {
		return nil, fmt.Errorf("failed to check existing model: %w", err)
	}
	if existing != nil {
		return nil, types.ErrModelAlreadyExists.Wrapf("model %s already registered", modelID)
	}

	// Verify the training record exists.
	_, err = k.GetTrainingRecord(ctx, msg.TrainingRecordID)
	if err != nil {
		return nil, types.ErrTrainingRecordNotFound.Wrapf("training record %s: %v", msg.TrainingRecordID, err)
	}

	// Determine version.
	version := uint64(1)
	if msg.ParentModelID != "" {
		parent, found := k.GetModelRecord(ctx, msg.ParentModelID)
		if !found {
			return nil, types.ErrInvalidModelLineage.Wrapf("parent model %s not found", msg.ParentModelID)
		}
		if parent.Domain != msg.Domain {
			return nil, types.ErrInvalidModelLineage.Wrapf("parent domain %s != %s", parent.Domain, msg.Domain)
		}
		version = parent.Version + 1
	} else {
		// Allocate from domain version counter.
		version, err = k.nextModelVersion(ctx, msg.Domain)
		if err != nil {
			return nil, fmt.Errorf("failed to allocate version: %w", err)
		}
	}

	tduCount := uint64(len(msg.TDUIDs))

	record := types.ModelRecord{
		ModelID:          modelID,
		Name:             msg.Name,
		Domain:           msg.Domain,
		Version:          version,
		ParentModelID:    msg.ParentModelID,
		TrainingRecordID: msg.TrainingRecordID,
		TDUIDs:           msg.TDUIDs,
		DatasetIDs:       msg.DatasetIDs,
		TDUCount:         tduCount,
		BenchmarkScore:   msg.BenchmarkScore,
		BenchmarkDetails: msg.BenchmarkDetails,
		FitnessWeighted:  msg.FitnessWeighted,
		Status:           types.ModelStatusActive,
		Publisher:        msg.Publisher,
		PublishedAt:      sdkCtx.BlockHeight(),
		TEEAttestation:   msg.TEEAttestation,
		ModelHash:        msg.ModelHash,
	}

	// Quality gate.
	if err := record.ValidateQuality(); err != nil {
		return nil, types.ErrModelQualityTooLow.Wrapf("%v", err)
	}

	// Store record.
	if err := k.setModelRecord(ctx, &record); err != nil {
		return nil, err
	}

	// Index: domain → modelID.
	domainKey := types.ModelDomainIndexKey(msg.Domain, modelID)
	if err := kvStore.Set(domainKey, []byte{0x01}); err != nil {
		return nil, fmt.Errorf("failed to set domain index: %w", err)
	}

	// Index: active models.
	activeKey := types.ModelActiveIndexKey(modelID)
	if err := kvStore.Set(activeKey, []byte{0x01}); err != nil {
		return nil, fmt.Errorf("failed to set active index: %w", err)
	}

	// Index: TDU → modelID (reverse index for attribution).
	for _, tduID := range msg.TDUIDs {
		tduKey := types.ModelTDUIndexKey(tduID, modelID)
		if err := kvStore.Set(tduKey, []byte{0x01}); err != nil {
			return nil, fmt.Errorf("failed to set TDU index for %s: %w", tduID, err)
		}
	}

	// Build lineage.
	lineage := types.ModelLineage{
		ModelID:    modelID,
		Generation: 1,
	}
	if msg.ParentModelID != "" {
		parentLineage, found := k.GetModelLineage(ctx, msg.ParentModelID)
		if found {
			lineage.Ancestors = append([]string{msg.ParentModelID}, parentLineage.Ancestors...)
			lineage.Generation = parentLineage.Generation + 1
		} else {
			lineage.Ancestors = []string{msg.ParentModelID}
			lineage.Generation = 2
		}
	}
	if err := k.setModelLineage(ctx, &lineage); err != nil {
		return nil, err
	}

	sdkCtx.EventManager().EmitEvent(sdk.NewEvent(
		types.EventModelPublished,
		sdk.NewAttribute(types.AttributeModelID, modelID),
		sdk.NewAttribute(types.AttributeModelName, msg.Name),
		sdk.NewAttribute(types.AttributeModelDomain, msg.Domain),
		sdk.NewAttribute(types.AttributeModelVersion, strconv.FormatUint(version, 10)),
		sdk.NewAttribute(types.AttributeModelPublisher, msg.Publisher),
	))

	return &types.MsgPublishModelResponse{
		ModelID: modelID,
		Version: version,
	}, nil
}

// ─── DeprecateModel ─────────────────────────────────────────────────────────

// DeprecateModel marks a model as deprecated, removing it from the active index.
func (k Keeper) DeprecateModel(ctx context.Context, modelID string, authority string, reason string) error {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	kvStore := k.storeService.OpenKVStore(ctx)

	record, found := k.GetModelRecord(ctx, modelID)
	if !found {
		return types.ErrModelNotFound.Wrapf("model %s", modelID)
	}

	if record.Status != types.ModelStatusActive {
		return types.ErrModelAlreadyDeprecated.Wrapf("model %s has status %s", modelID, record.Status)
	}

	// Only publisher or governance authority can deprecate.
	if authority != record.Publisher && authority != k.authority {
		return types.ErrUnauthorized.Wrapf("only publisher or governance can deprecate")
	}

	record.Status = types.ModelStatusDeprecated
	record.DeprecatedAt = sdkCtx.BlockHeight()
	record.DeprecationReason = reason

	if err := k.setModelRecord(ctx, &record); err != nil {
		return err
	}

	// Remove from active index.
	activeKey := types.ModelActiveIndexKey(modelID)
	if err := kvStore.Delete(activeKey); err != nil {
		return fmt.Errorf("failed to remove active index: %w", err)
	}

	sdkCtx.EventManager().EmitEvent(sdk.NewEvent(
		types.EventModelDeprecated,
		sdk.NewAttribute(types.AttributeModelID, modelID),
		sdk.NewAttribute(types.AttributeModelStatus, string(types.ModelStatusDeprecated)),
	))

	return nil
}

// ─── SupersedeModel ─────────────────────────────────────────────────────────

// SupersedeModel marks an old model as superseded by a new one.
func (k Keeper) SupersedeModel(ctx context.Context, oldModelID, newModelID string) error {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	kvStore := k.storeService.OpenKVStore(ctx)

	oldRecord, found := k.GetModelRecord(ctx, oldModelID)
	if !found {
		return types.ErrModelNotFound.Wrapf("old model %s", oldModelID)
	}
	if oldRecord.Status != types.ModelStatusActive {
		return types.ErrModelNotActive.Wrapf("old model %s has status %s", oldModelID, oldRecord.Status)
	}

	_, found = k.GetModelRecord(ctx, newModelID)
	if !found {
		return types.ErrModelNotFound.Wrapf("new model %s", newModelID)
	}

	oldRecord.Status = types.ModelStatusSuperseded
	oldRecord.SupersededBy = newModelID
	oldRecord.DeprecatedAt = sdkCtx.BlockHeight()

	if err := k.setModelRecord(ctx, &oldRecord); err != nil {
		return err
	}

	// Remove from active index.
	activeKey := types.ModelActiveIndexKey(oldModelID)
	if err := kvStore.Delete(activeKey); err != nil {
		return fmt.Errorf("failed to remove active index: %w", err)
	}

	sdkCtx.EventManager().EmitEvent(sdk.NewEvent(
		types.EventModelSuperseded,
		sdk.NewAttribute(types.AttributeModelID, oldModelID),
		sdk.NewAttribute("superseded_by", newModelID),
	))

	return nil
}

// ─── Queries ────────────────────────────────────────────────────────────────

// GetModelRecord retrieves a model record by ID.
func (k Keeper) GetModelRecord(ctx context.Context, modelID string) (types.ModelRecord, bool) {
	kvStore := k.storeService.OpenKVStore(ctx)
	key := types.ModelRecordKey(modelID)

	bz, err := kvStore.Get(key)
	if err != nil || bz == nil {
		return types.ModelRecord{}, false
	}

	var record types.ModelRecord
	if err := json.Unmarshal(bz, &record); err != nil {
		return types.ModelRecord{}, false
	}
	return record, true
}

// GetModelsByDomain returns all models registered under a domain.
func (k Keeper) GetModelsByDomain(ctx context.Context, domain string) []types.ModelRecord {
	kvStore := k.storeService.OpenKVStore(ctx)
	prefix := types.ModelDomainByDomainPrefix(domain)

	var models []types.ModelRecord
	iter, err := kvStore.Iterator(prefix, prefixEndBytes(prefix))
	if err != nil {
		return nil
	}
	defer iter.Close()

	for ; iter.Valid(); iter.Next() {
		// Extract modelID from key: prefix + modelID
		fullKey := iter.Key()
		modelID := string(fullKey[len(prefix):])

		record, found := k.GetModelRecord(ctx, modelID)
		if found {
			models = append(models, record)
		}
	}
	return models
}

// GetActiveModels returns all models with active status.
func (k Keeper) GetActiveModels(ctx context.Context) []types.ModelRecord {
	kvStore := k.storeService.OpenKVStore(ctx)
	prefix := types.ModelActiveIndexPrefix

	var models []types.ModelRecord
	iter, err := kvStore.Iterator(prefix, prefixEndBytes(prefix))
	if err != nil {
		return nil
	}
	defer iter.Close()

	for ; iter.Valid(); iter.Next() {
		modelID := string(iter.Key()[len(prefix):])
		record, found := k.GetModelRecord(ctx, modelID)
		if found {
			models = append(models, record)
		}
	}
	return models
}

// GetModelLineage retrieves the lineage chain for a model.
func (k Keeper) GetModelLineage(ctx context.Context, modelID string) (types.ModelLineage, bool) {
	kvStore := k.storeService.OpenKVStore(ctx)
	key := types.ModelLineageKey(modelID)

	bz, err := kvStore.Get(key)
	if err != nil || bz == nil {
		return types.ModelLineage{}, false
	}

	var lineage types.ModelLineage
	if err := json.Unmarshal(bz, &lineage); err != nil {
		return types.ModelLineage{}, false
	}
	return lineage, true
}

// GetLatestModel returns the most recent active model for a domain (highest version).
func (k Keeper) GetLatestModel(ctx context.Context, domain string) (types.ModelRecord, bool) {
	models := k.GetModelsByDomain(ctx, domain)
	if len(models) == 0 {
		return types.ModelRecord{}, false
	}

	var latest types.ModelRecord
	var latestVersion uint64
	for _, m := range models {
		if m.Status == types.ModelStatusActive && m.Version > latestVersion {
			latest = m
			latestVersion = m.Version
		}
	}
	if latestVersion == 0 {
		return types.ModelRecord{}, false
	}
	return latest, true
}

// ─── Lineage & Attribution ──────────────────────────────────────────────────

// GetContributingTDUs returns all TDU IDs that contributed to a model's training.
func (k Keeper) GetContributingTDUs(ctx context.Context, modelID string) []string {
	record, found := k.GetModelRecord(ctx, modelID)
	if !found {
		return nil
	}
	return record.TDUIDs
}

// GetModelsByTDU returns all model IDs that used a specific TDU for training.
func (k Keeper) GetModelsByTDU(ctx context.Context, tduID string) []string {
	kvStore := k.storeService.OpenKVStore(ctx)
	prefix := types.ModelTDUByTDUPrefix(tduID)

	var modelIDs []string
	iter, err := kvStore.Iterator(prefix, prefixEndBytes(prefix))
	if err != nil {
		return nil
	}
	defer iter.Close()

	for ; iter.Valid(); iter.Next() {
		modelID := string(iter.Key()[len(prefix):])
		modelIDs = append(modelIDs, modelID)
	}
	return modelIDs
}

// CalculateContributorRewards computes proportional rewards for TDU contributors
// based on fitness × influence. Returns map of submitter address → reward amount.
func (k Keeper) CalculateContributorRewards(ctx context.Context, modelID string, totalReward sdkmath.Int) map[string]sdkmath.Int {
	record, found := k.GetModelRecord(ctx, modelID)
	if !found || totalReward.IsZero() {
		return nil
	}

	type contribution struct {
		submitter string
		weight    sdkmath.LegacyDec
	}

	var contributions []contribution
	totalWeight := sdkmath.LegacyZeroDec()

	for _, tduID := range record.TDUIDs {
		// Get fitness for this TDU.
		fitness := sdkmath.LegacyNewDecWithPrec(5, 1) // default 0.5
		if fitnessRecord, found := k.GetFitnessRecord(ctx, tduID); found {
			fitness = fitnessRecord.GetFitnessScore()
		}

		// Get sample to find submitter.
		sample := k.getSampleSubmitter(ctx, tduID)
		if sample == "" {
			continue
		}

		// Weight = fitness (influence integration TODO: multiply by per-TDU loss signal).
		weight := fitness
		if weight.IsPositive() {
			contributions = append(contributions, contribution{
				submitter: sample,
				weight:    weight,
			})
			totalWeight = totalWeight.Add(weight)
		}
	}

	if totalWeight.IsZero() || len(contributions) == 0 {
		return nil
	}

	rewards := make(map[string]sdkmath.Int)
	totalDec := sdkmath.LegacyNewDecFromInt(totalReward)

	for _, c := range contributions {
		share := c.weight.Quo(totalWeight)
		reward := share.Mul(totalDec).TruncateInt()
		if reward.IsPositive() {
			existing, ok := rewards[c.submitter]
			if ok {
				rewards[c.submitter] = existing.Add(reward)
			} else {
				rewards[c.submitter] = reward
			}
		}
	}

	return rewards
}

// ─── Inference Tracking ─────────────────────────────────────────────────────

// RegisterEndpoint adds an inference endpoint for a model.
func (k Keeper) RegisterModelEndpoint(ctx context.Context, modelID string, endpoint string) error {
	kvStore := k.storeService.OpenKVStore(ctx)

	_, found := k.GetModelRecord(ctx, modelID)
	if !found {
		return types.ErrModelNotFound.Wrapf("model %s", modelID)
	}

	key := types.ModelEndpointKey(modelID, endpoint)
	return kvStore.Set(key, []byte{0x01})
}

// RemoveEndpoint removes an inference endpoint for a model.
func (k Keeper) RemoveModelEndpoint(ctx context.Context, modelID string, endpoint string) error {
	kvStore := k.storeService.OpenKVStore(ctx)
	key := types.ModelEndpointKey(modelID, endpoint)
	return kvStore.Delete(key)
}

// GetModelEndpoints returns all registered endpoints for a model.
func (k Keeper) GetModelEndpoints(ctx context.Context, modelID string) []string {
	kvStore := k.storeService.OpenKVStore(ctx)
	prefix := types.ModelEndpointByModelPrefix(modelID)

	var endpoints []string
	iter, err := kvStore.Iterator(prefix, prefixEndBytes(prefix))
	if err != nil {
		return nil
	}
	defer iter.Close()

	for ; iter.Valid(); iter.Next() {
		endpoint := string(iter.Key()[len(prefix):])
		endpoints = append(endpoints, endpoint)
	}
	return endpoints
}

// RecordInference increments the inference counter for a model.
func (k Keeper) RecordInference(ctx context.Context, modelID string) error {
	record, found := k.GetModelRecord(ctx, modelID)
	if !found {
		return types.ErrModelNotFound.Wrapf("model %s", modelID)
	}

	record.InferenceCount++
	return k.setModelRecord(ctx, &record)
}

// ─── Quality Gating ─────────────────────────────────────────────────────────

// UpdateBenchmarkScores updates the benchmark results for a model and
// recomputes the aggregate score.
func (k Keeper) UpdateBenchmarkScores(ctx context.Context, modelID string, results []types.BenchmarkResult) error {
	record, found := k.GetModelRecord(ctx, modelID)
	if !found {
		return types.ErrModelNotFound.Wrapf("model %s", modelID)
	}

	record.BenchmarkDetails = results

	// Recompute aggregate score as mean of all benchmark scores.
	if len(results) > 0 {
		total := sdkmath.LegacyZeroDec()
		for _, r := range results {
			total = total.Add(r.GetScore())
		}
		avg := total.Quo(sdkmath.LegacyNewDec(int64(len(results))))
		record.SetBenchmarkScore(avg)
	}

	return k.setModelRecord(ctx, &record)
}

// ─── Internal helpers ───────────────────────────────────────────────────────

func (k Keeper) setModelRecord(ctx context.Context, record *types.ModelRecord) error {
	kvStore := k.storeService.OpenKVStore(ctx)
	key := types.ModelRecordKey(record.ModelID)

	bz, err := json.Marshal(record)
	if err != nil {
		return fmt.Errorf("failed to marshal model record: %w", err)
	}
	return kvStore.Set(key, bz)
}

func (k Keeper) setModelLineage(ctx context.Context, lineage *types.ModelLineage) error {
	kvStore := k.storeService.OpenKVStore(ctx)
	key := types.ModelLineageKey(lineage.ModelID)

	bz, err := json.Marshal(lineage)
	if err != nil {
		return fmt.Errorf("failed to marshal model lineage: %w", err)
	}
	return kvStore.Set(key, bz)
}

func (k Keeper) nextModelVersion(ctx context.Context, domain string) (uint64, error) {
	kvStore := k.storeService.OpenKVStore(ctx)
	key := types.ModelVersionKey(domain)

	bz, err := kvStore.Get(key)
	if err != nil {
		return 0, fmt.Errorf("failed to get version counter: %w", err)
	}

	var version uint64 = 1
	if bz != nil && len(bz) >= 8 {
		version = binary.BigEndian.Uint64(bz) + 1
	}

	buf := make([]byte, 8)
	binary.BigEndian.PutUint64(buf, version)
	if err := kvStore.Set(key, buf); err != nil {
		return 0, fmt.Errorf("failed to set version counter: %w", err)
	}
	return version, nil
}

// getSampleSubmitter returns the submitter address for a sample ID, or "".
func (k Keeper) getSampleSubmitter(ctx context.Context, sampleID string) string {
	kvStore := k.storeService.OpenKVStore(ctx)
	key := types.SampleKey(sampleID)

	bz, err := kvStore.Get(key)
	if err != nil || bz == nil {
		return ""
	}

	// Sample is stored as proto — try to extract submitter field.
	// For now, return the sampleID as a fallback key since we need
	// proto decode. The integration with Sample proto is done at
	// the msg_server level where we have full access.
	var sample types.Sample
	if err := k.cdc.Unmarshal(bz, &sample); err != nil {
		return ""
	}
	return sample.Submitter
}

// prefixEndBytes is defined in state.go — reused here for prefix iteration.
