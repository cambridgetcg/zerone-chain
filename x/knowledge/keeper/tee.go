package keeper

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"

	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/zerone-chain/zerone/x/knowledge/types"
)

// RegisterEnclave registers a new TEE enclave for an operator.
// Validates the provider type, stores the enclave record, and indexes by status.
func (k Keeper) RegisterEnclave(ctx context.Context, operator string, provider string, attestation []byte, measurements []byte) (string, error) {
	sdkCtx := sdk.UnwrapSDKContext(ctx)

	// Validate provider type.
	if !types.ValidTEEProviders[provider] {
		return "", types.ErrInvalidTEEProvider.Wrapf("unknown provider: %s", provider)
	}

	// Validate inputs.
	if len(attestation) == 0 {
		return "", types.ErrInvalidAttestation.Wrap("attestation document must not be empty")
	}
	if len(measurements) == 0 {
		return "", types.ErrTEEMeasurementMismatch.Wrap("measurements must not be empty")
	}

	kvStore := k.storeService.OpenKVStore(ctx)

	// Check if operator already has a registered enclave.
	key := types.EnclaveKey(operator)
	existing, err := kvStore.Get(key)
	if err != nil {
		return "", fmt.Errorf("failed to check existing enclave: %w", err)
	}
	if existing != nil {
		return "", types.ErrEnclaveAlreadyRegistered.Wrapf("operator %s already has a registered enclave", operator)
	}

	// Compute attestation hash (only store hash, not full document).
	attestHash := sha256.Sum256(attestation)

	record := types.EnclaveRecord{
		Operator:        operator,
		Provider:        provider,
		Measurements:    measurements,
		AttestationHash: attestHash[:],
		RegisteredAt:    sdkCtx.BlockHeight(),
		LastVerified:    sdkCtx.BlockHeight(),
		Status:          types.EnclaveStatusActiveStr,
	}

	bz, err := json.Marshal(record)
	if err != nil {
		return "", fmt.Errorf("failed to marshal enclave record: %w", err)
	}

	if err := kvStore.Set(key, bz); err != nil {
		return "", fmt.Errorf("failed to store enclave record: %w", err)
	}

	// Index by status.
	statusKey := types.EnclaveStatusIndexKey(types.EnclaveStatusActiveStr, operator)
	if err := kvStore.Set(statusKey, []byte{0x01}); err != nil {
		return "", fmt.Errorf("failed to set status index: %w", err)
	}

	sdkCtx.EventManager().EmitEvent(sdk.NewEvent(
		types.EventEnclaveRegistered,
		sdk.NewAttribute(types.AttributeEnclaveOperator, operator),
		sdk.NewAttribute(types.AttributeEnclaveProvider, provider),
		sdk.NewAttribute(types.AttributeEnclaveStatus, types.EnclaveStatusActiveStr),
	))

	// Return a deterministic enclave ID based on operator.
	return fmt.Sprintf("enclave-%s", operator), nil
}

// GetEnclave retrieves a registered enclave by operator address.
func (k Keeper) GetEnclave(ctx context.Context, operator string) (*types.EnclaveRecord, error) {
	kvStore := k.storeService.OpenKVStore(ctx)
	key := types.EnclaveKey(operator)

	bz, err := kvStore.Get(key)
	if err != nil {
		return nil, fmt.Errorf("failed to get enclave: %w", err)
	}
	if bz == nil {
		return nil, types.ErrEnclaveNotFound.Wrapf("no enclave for operator %s", operator)
	}

	var record types.EnclaveRecord
	if err := json.Unmarshal(bz, &record); err != nil {
		return nil, fmt.Errorf("failed to unmarshal enclave record: %w", err)
	}
	return &record, nil
}

// VerifyEnclave updates the last-verified block for an active enclave.
func (k Keeper) VerifyEnclave(ctx context.Context, operator string, freshAttestation []byte) (bool, error) {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	kvStore := k.storeService.OpenKVStore(ctx)

	record, err := k.GetEnclave(ctx, operator)
	if err != nil {
		return false, err
	}

	if record.Status != types.EnclaveStatusActiveStr {
		return false, types.ErrEnclaveNotActive.Wrapf("enclave status is %s", record.Status)
	}

	if len(freshAttestation) == 0 {
		return false, types.ErrInvalidAttestation.Wrap("attestation document must not be empty")
	}

	// Update last-verified timestamp and attestation hash.
	attestHash := sha256.Sum256(freshAttestation)
	record.AttestationHash = attestHash[:]
	record.LastVerified = sdkCtx.BlockHeight()

	bz, err := json.Marshal(record)
	if err != nil {
		return false, fmt.Errorf("failed to marshal enclave record: %w", err)
	}

	key := types.EnclaveKey(operator)
	if err := kvStore.Set(key, bz); err != nil {
		return false, fmt.Errorf("failed to update enclave record: %w", err)
	}

	sdkCtx.EventManager().EmitEvent(sdk.NewEvent(
		types.EventEnclaveVerified,
		sdk.NewAttribute(types.AttributeEnclaveOperator, operator),
	))

	return true, nil
}

// SuspendEnclave transitions an enclave to suspended status.
func (k Keeper) SuspendEnclave(ctx context.Context, operator string) error {
	return k.setEnclaveStatus(ctx, operator, types.EnclaveStatusSuspendedStr, types.EventEnclaveSuspended)
}

// RevokeEnclave permanently revokes an enclave.
func (k Keeper) RevokeEnclave(ctx context.Context, operator string) error {
	return k.setEnclaveStatus(ctx, operator, types.EnclaveStatusRevokedStr, types.EventEnclaveRevoked)
}

// setEnclaveStatus is a helper to transition enclave status with index updates.
func (k Keeper) setEnclaveStatus(ctx context.Context, operator, newStatus, eventType string) error {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	kvStore := k.storeService.OpenKVStore(ctx)

	record, err := k.GetEnclave(ctx, operator)
	if err != nil {
		return err
	}

	oldStatus := record.Status

	// Cannot transition from revoked.
	if oldStatus == types.EnclaveStatusRevokedStr {
		return types.ErrEnclaveNotActive.Wrap("cannot change status of a revoked enclave")
	}

	// Remove old status index.
	oldIndexKey := types.EnclaveStatusIndexKey(oldStatus, operator)
	if err := kvStore.Delete(oldIndexKey); err != nil {
		return fmt.Errorf("failed to delete old status index: %w", err)
	}

	// Update record.
	record.Status = newStatus
	bz, err := json.Marshal(record)
	if err != nil {
		return fmt.Errorf("failed to marshal enclave record: %w", err)
	}

	key := types.EnclaveKey(operator)
	if err := kvStore.Set(key, bz); err != nil {
		return fmt.Errorf("failed to update enclave record: %w", err)
	}

	// Set new status index.
	newIndexKey := types.EnclaveStatusIndexKey(newStatus, operator)
	if err := kvStore.Set(newIndexKey, []byte{0x01}); err != nil {
		return fmt.Errorf("failed to set new status index: %w", err)
	}

	sdkCtx.EventManager().EmitEvent(sdk.NewEvent(
		eventType,
		sdk.NewAttribute(types.AttributeEnclaveOperator, operator),
		sdk.NewAttribute(types.AttributeEnclaveStatus, newStatus),
	))

	return nil
}
