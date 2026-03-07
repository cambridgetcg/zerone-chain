package keeper

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"

	sdkmath "cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/zerone-chain/zerone/x/knowledge/types"
)

// RecordTraining stores a training attestation on-chain after verifying the
// operator has an active registered enclave.
func (k Keeper) RecordTraining(ctx context.Context, msg *types.MsgRecordTraining) (*types.MsgRecordTrainingResponse, error) {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	kvStore := k.storeService.OpenKVStore(ctx)

	// Verify the operator has an active enclave.
	enclave, err := k.GetEnclave(ctx, msg.Operator)
	if err != nil {
		return nil, types.ErrEnclaveNotRegistered.Wrapf("operator %s: %v", msg.Operator, err)
	}
	if enclave.Status != types.EnclaveStatusActiveStr {
		return nil, types.ErrEnclaveNotActive.Wrapf("enclave status is %s", enclave.Status)
	}

	// Check for duplicate attestation hash.
	key := types.TrainingRecordKey(msg.AttestationHash)
	existing, err := kvStore.Get(key)
	if err != nil {
		return nil, fmt.Errorf("failed to check existing training record: %w", err)
	}
	if existing != nil {
		return nil, types.ErrTrainingRecordExists.Wrapf("attestation %s already recorded", msg.AttestationHash)
	}

	record := types.TrainingRecord{
		Operator:           msg.Operator,
		EnclaveID:          fmt.Sprintf("enclave-%s", msg.Operator),
		AttestationHash:    msg.AttestationHash,
		DatasetFingerprint: msg.DatasetFingerprint,
		DatasetSize:        msg.DatasetSize,
		BaseModel:          msg.BaseModel,
		ModelHash:          msg.ModelHash,
		BenchmarkScore:     msg.BenchmarkScore,
		BlockHeight:        sdkCtx.BlockHeight(),
	}

	bz, err := json.Marshal(record)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal training record: %w", err)
	}

	if err := kvStore.Set(key, bz); err != nil {
		return nil, fmt.Errorf("failed to store training record: %w", err)
	}

	// Index by model hash.
	modelKey := types.TrainingRecordByModelKey(msg.ModelHash, msg.AttestationHash)
	if err := kvStore.Set(modelKey, []byte{0x01}); err != nil {
		return nil, fmt.Errorf("failed to set model index: %w", err)
	}

	sdkCtx.EventManager().EmitEvent(sdk.NewEvent(
		types.EventTrainingRecorded,
		sdk.NewAttribute(types.AttributeEnclaveOperator, msg.Operator),
		sdk.NewAttribute(types.AttributeAttestationHash, msg.AttestationHash),
		sdk.NewAttribute(types.AttributeModelHash, msg.ModelHash),
		sdk.NewAttribute(types.AttributeBaseModel, msg.BaseModel),
		sdk.NewAttribute(types.AttributeDatasetSize, strconv.FormatInt(msg.DatasetSize, 10)),
	))

	// ── Memory system integration (R50/R51) ──────────────────────────────
	// If training TDU IDs are available (from the dataset fingerprint),
	// process the training outcome through the memory consolidation system.
	// The benchmark score is used as the model delta: scores > 0.5 are
	// positive outcomes, < 0.5 are negative.
	fitnessParams := k.GetFitnessDecayParams(ctx)
	currentCycle := uint64(sdkCtx.BlockHeight()) / fitnessParams.GetFitnessEpochBlocks()

	// Benchmark score → delta (centered at 0.5 baseline).
	benchmarkDelta, _ := sdkmath.LegacyNewDecFromStr(fmt.Sprintf("%.18f", msg.BenchmarkScore-0.5))

	// Get TDU IDs from dataset (if shard assignments exist for this fingerprint).
	tduIDs := k.getTDUIDsFromDataset(ctx, msg.DatasetFingerprint)
	if len(tduIDs) > 0 {
		outcome := TrainingOutcome{
			TDUIDs:          tduIDs,
			OverallDelta:    benchmarkDelta,
			CurrentCycle:    currentCycle,
			AttestationHash: msg.AttestationHash,
		}
		// Process asynchronously — errors are non-fatal for the training record.
		if err := k.ProcessTrainingOutcome(ctx, outcome); err != nil {
			sdkCtx.Logger().Error("failed to process training outcome for memory system",
				"attestation_hash", msg.AttestationHash, "error", err)
		}
	}

	return &types.MsgRecordTrainingResponse{
		AttestationHash: msg.AttestationHash,
	}, nil
}

// SetTrainingRecord stores a training record directly (for testing and seed data).
func (k Keeper) SetTrainingRecord(ctx context.Context, record *types.TrainingRecord) error {
	kvStore := k.storeService.OpenKVStore(ctx)
	key := types.TrainingRecordKey(record.AttestationHash)

	bz, err := json.Marshal(record)
	if err != nil {
		return fmt.Errorf("failed to marshal training record: %w", err)
	}
	if err := kvStore.Set(key, bz); err != nil {
		return fmt.Errorf("failed to store training record: %w", err)
	}

	// Index by model hash
	if record.ModelHash != "" {
		modelKey := types.TrainingRecordByModelKey(record.ModelHash, record.AttestationHash)
		if err := kvStore.Set(modelKey, []byte{0x01}); err != nil {
			return fmt.Errorf("failed to set model index: %w", err)
		}
	}
	return nil
}

// getTDUIDsFromDataset retrieves TDU sample IDs associated with a dataset fingerprint.
// Looks up the dataset record by fingerprint, which stores its constituent TDU IDs.
// GetTDUIDsFromDataset retrieves TDU sample IDs associated with a dataset fingerprint.
func (k Keeper) GetTDUIDsFromDataset(ctx context.Context, datasetFingerprint string) []string {
	return k.getTDUIDsFromDataset(ctx, datasetFingerprint)
}

func (k Keeper) getTDUIDsFromDataset(ctx context.Context, datasetFingerprint string) []string {
	if datasetFingerprint == "" {
		return nil
	}

	// Look up the dataset record which maps fingerprint → TDU IDs.
	kvStore := k.storeService.OpenKVStore(ctx)
	key := types.DatasetFingerprintKey(datasetFingerprint)
	bz, err := kvStore.Get(key)
	if err != nil || bz == nil {
		return nil
	}

	var tduIDs []string
	if err := json.Unmarshal(bz, &tduIDs); err != nil {
		return nil
	}

	// Cap at 100 TDUs to bound processing.
	if len(tduIDs) > 100 {
		tduIDs = tduIDs[:100]
	}
	return tduIDs
}

// RegisterDatasetFingerprint stores the mapping from dataset fingerprint → TDU IDs.
// Called when a training dataset is assembled (shard collection phase).
func (k Keeper) RegisterDatasetFingerprint(ctx context.Context, fingerprint string, tduIDs []string) error {
	kvStore := k.storeService.OpenKVStore(ctx)
	bz, err := json.Marshal(tduIDs)
	if err != nil {
		return fmt.Errorf("failed to marshal TDU IDs: %w", err)
	}
	return kvStore.Set(types.DatasetFingerprintKey(fingerprint), bz)
}

// GetTrainingRecord retrieves a training record by attestation hash.
func (k Keeper) GetTrainingRecord(ctx context.Context, attestationHash string) (*types.TrainingRecord, error) {
	kvStore := k.storeService.OpenKVStore(ctx)
	key := types.TrainingRecordKey(attestationHash)

	bz, err := kvStore.Get(key)
	if err != nil {
		return nil, fmt.Errorf("failed to get training record: %w", err)
	}
	if bz == nil {
		return nil, types.ErrTrainingRecordNotFound.Wrapf("no training record for attestation %s", attestationHash)
	}

	var record types.TrainingRecord
	if err := json.Unmarshal(bz, &record); err != nil {
		return nil, fmt.Errorf("failed to unmarshal training record: %w", err)
	}
	return &record, nil
}
