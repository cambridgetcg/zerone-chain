package keeper

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"

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

	return &types.MsgRecordTrainingResponse{
		AttestationHash: msg.AttestationHash,
	}, nil
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
