package keeper

import (
	"context"
	"encoding/json"

	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/zerone-chain/zerone/x/knowledge/types"
)

// ─── Vindication Pending (per-fact minority entries) ─────────────────────────

// SetVindicationPending stores the pending vindication entries for a fact as JSON.
func (k Keeper) SetVindicationPending(ctx context.Context, factId string, entries []types.VindicationEntry) error {
	store := k.storeService.OpenKVStore(ctx)
	bz, err := json.Marshal(entries)
	if err != nil {
		return err
	}
	return store.Set(types.VindicationPendingKey(factId), bz)
}

// GetVindicationPending retrieves pending vindication entries for a fact.
// Returns nil if no entries are found.
func (k Keeper) GetVindicationPending(ctx context.Context, factId string) []types.VindicationEntry {
	store := k.storeService.OpenKVStore(ctx)
	bz, err := store.Get(types.VindicationPendingKey(factId))
	if err != nil || bz == nil {
		return nil
	}
	var entries []types.VindicationEntry
	if err := json.Unmarshal(bz, &entries); err != nil {
		return nil
	}
	return entries
}

// DeleteVindicationPending removes all pending vindication entries for a fact.
func (k Keeper) DeleteVindicationPending(ctx context.Context, factId string) {
	store := k.storeService.OpenKVStore(ctx)
	_ = store.Delete(types.VindicationPendingKey(factId))
}

// GetAllVindicationPending iterates all pending vindication entries across all facts.
// Returns a map of factId -> entries. Used by pruning logic in BeginBlocker.
func (k Keeper) GetAllVindicationPending(ctx context.Context) map[string][]types.VindicationEntry {
	store := k.storeService.OpenKVStore(ctx)
	iter, err := store.Iterator(types.VindicationPendingPrefix, prefixEndBytes(types.VindicationPendingPrefix))
	if err != nil {
		return nil
	}
	defer iter.Close()

	result := make(map[string][]types.VindicationEntry)
	for ; iter.Valid(); iter.Next() {
		factId := string(iter.Key()[len(types.VindicationPendingPrefix):])
		var entries []types.VindicationEntry
		if err := json.Unmarshal(iter.Value(), &entries); err != nil {
			continue
		}
		result[factId] = entries
	}
	return result
}

// ─── Vindication Records (immutable per-verifier outcomes) ───────────────────

// SetVindicationRecord stores a vindication record for a specific fact/verifier pair.
func (k Keeper) SetVindicationRecord(ctx context.Context, factId string, record types.VindicationRecord) error {
	store := k.storeService.OpenKVStore(ctx)
	bz, err := json.Marshal(record)
	if err != nil {
		return err
	}
	return store.Set(types.VindicationRecordKey(factId, record.Verifier), bz)
}

// GetVindicationRecord retrieves a vindication record for a specific fact and verifier.
// Returns the record and true if found, or a zero-value record and false if not.
func (k Keeper) GetVindicationRecord(ctx context.Context, factId, verifier string) (types.VindicationRecord, bool) {
	store := k.storeService.OpenKVStore(ctx)
	bz, err := store.Get(types.VindicationRecordKey(factId, verifier))
	if err != nil || bz == nil {
		return types.VindicationRecord{}, false
	}
	var record types.VindicationRecord
	if err := json.Unmarshal(bz, &record); err != nil {
		return types.VindicationRecord{}, false
	}
	return record, true
}

// GetVindicationRecordsForFact returns all vindication records for a given fact.
func (k Keeper) GetVindicationRecordsForFact(ctx context.Context, factId string) []types.VindicationRecord {
	store := k.storeService.OpenKVStore(ctx)
	pfx := types.VindicationRecordPrefixForFact(factId)
	iter, err := store.Iterator(pfx, prefixEndBytes(pfx))
	if err != nil {
		return nil
	}
	defer iter.Close()

	var records []types.VindicationRecord
	for ; iter.Valid(); iter.Next() {
		var record types.VindicationRecord
		if err := json.Unmarshal(iter.Value(), &record); err != nil {
			continue
		}
		records = append(records, record)
	}
	return records
}

// ─── Challenge Disproven Transition ──────────────────────────────────────────

// handleChallengeDisproven transitions the challenged fact to DISPROVEN
// when a challenge claim is accepted. Triggers vindication for the original
// fact's minority voters who were slashed during its verification round.
func (k Keeper) handleChallengeDisproven(ctx context.Context, challengeClaim *types.Claim, newFactId string) {
	if challengeClaim.ProvisionalFactId == "" {
		return
	}

	originalFact, found := k.GetFact(ctx, challengeClaim.ProvisionalFactId)
	if !found {
		return
	}

	// Contradiction check: same domain + explicit challenge link
	if originalFact.Domain != challengeClaim.Domain {
		return
	}

	// Transition to DISPROVEN
	originalFact.Status = types.FactStatus_FACT_STATUS_DISPROVEN
	_ = k.SetFact(ctx, originalFact)

	sdkCtx := sdk.UnwrapSDKContext(ctx)
	sdkCtx.EventManager().EmitEvent(sdk.NewEvent(
		"zerone.knowledge.fact_disproven",
		sdk.NewAttribute("fact_id", originalFact.Id),
		sdk.NewAttribute("disproven_by", newFactId),
		sdk.NewAttribute("challenge_claim_id", challengeClaim.Id),
	))

	// Trigger vindication for the ORIGINAL fact's minority voters
	k.ExecuteVindication(ctx, originalFact.Id, newFactId)
}

// ─── Execute Vindication ─────────────────────────────────────────────────────

// ExecuteVindication refunds minority voters from escrow, slashes the majority,
// distributes a bonus from the majority slash pool, and records immutable
// vindication records. Called when a fact is disproven via challenge.
// Stub: full implementation in next commit (Task 10).
func (k Keeper) ExecuteVindication(ctx context.Context, factId, disprovenBy string) {
	// TODO(R28-1): implement full vindication execution logic
}
