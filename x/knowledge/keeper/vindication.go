package keeper

import (
	"context"
	"encoding/json"

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
