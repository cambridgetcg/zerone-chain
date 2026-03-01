package keeper

import (
	"context"
	"fmt"

	"github.com/cosmos/gogoproto/proto"

	"github.com/zerone-chain/zerone/x/capture_defense/types"
)

// ---------- Params ----------

// SetParams sets module parameters.
func (k Keeper) SetParams(ctx context.Context, params *types.Params) {
	kvStore := k.storeService.OpenKVStore(ctx)
	bz, err := proto.Marshal(params)
	if err != nil {
		panic(fmt.Sprintf("failed to marshal params: %v", err))
	}
	_ = kvStore.Set(types.ParamsKey, bz)
}

// GetParams returns module parameters.
func (k Keeper) GetParams(ctx context.Context) *types.Params {
	kvStore := k.storeService.OpenKVStore(ctx)
	bz, err := kvStore.Get(types.ParamsKey)
	if err != nil || bz == nil {
		return types.DefaultParams()
	}
	var params types.Params
	if err := proto.Unmarshal(bz, &params); err != nil {
		return types.DefaultParams()
	}
	return &params
}

// ---------- GlobalReputation ----------

// SetGlobalReputation stores a global reputation record.
func (k Keeper) SetGlobalReputation(ctx context.Context, r *types.GlobalReputation) {
	kvStore := k.storeService.OpenKVStore(ctx)
	bz, err := proto.Marshal(r)
	if err != nil {
		panic(fmt.Sprintf("failed to marshal global reputation: %v", err))
	}
	_ = kvStore.Set(globalRepKey(r.Validator), bz)
}

// GetGlobalReputation retrieves a global reputation by validator.
func (k Keeper) GetGlobalReputation(ctx context.Context, validator string) (*types.GlobalReputation, bool) {
	kvStore := k.storeService.OpenKVStore(ctx)
	bz, err := kvStore.Get(globalRepKey(validator))
	if err != nil || bz == nil {
		return nil, false
	}
	var r types.GlobalReputation
	if err := proto.Unmarshal(bz, &r); err != nil {
		return nil, false
	}
	return &r, true
}

// DeleteGlobalReputation removes a global reputation record.
func (k Keeper) DeleteGlobalReputation(ctx context.Context, validator string) {
	kvStore := k.storeService.OpenKVStore(ctx)
	_ = kvStore.Delete(globalRepKey(validator))
}

// GetAllGlobalReputations returns all global reputation records.
func (k Keeper) GetAllGlobalReputations(ctx context.Context) []*types.GlobalReputation {
	var reps []*types.GlobalReputation
	k.IterateGlobalReputations(ctx, func(r *types.GlobalReputation) bool {
		reps = append(reps, r)
		return false
	})
	return reps
}

// IterateGlobalReputations iterates all global reputations. Return true from cb to stop.
func (k Keeper) IterateGlobalReputations(ctx context.Context, cb func(*types.GlobalReputation) bool) {
	kvStore := k.storeService.OpenKVStore(ctx)
	iter, err := kvStore.Iterator(types.GlobalReputationKeyPrefix, prefixEndBytes(types.GlobalReputationKeyPrefix))
	if err != nil {
		return
	}
	defer iter.Close()

	for ; iter.Valid(); iter.Next() {
		var r types.GlobalReputation
		if err := proto.Unmarshal(iter.Value(), &r); err != nil {
			continue
		}
		if cb(&r) {
			break
		}
	}
}

// ---------- StratumReputation ----------

// SetStratumReputation stores a stratum reputation record.
func (k Keeper) SetStratumReputation(ctx context.Context, r *types.StratumReputation) {
	kvStore := k.storeService.OpenKVStore(ctx)
	bz, err := proto.Marshal(r)
	if err != nil {
		panic(fmt.Sprintf("failed to marshal stratum reputation: %v", err))
	}
	_ = kvStore.Set(stratumRepKey(r.Stratum, r.Validator), bz)
}

// GetStratumReputation retrieves a stratum reputation by stratum and validator.
func (k Keeper) GetStratumReputation(ctx context.Context, stratum, validator string) (*types.StratumReputation, bool) {
	kvStore := k.storeService.OpenKVStore(ctx)
	bz, err := kvStore.Get(stratumRepKey(stratum, validator))
	if err != nil || bz == nil {
		return nil, false
	}
	var r types.StratumReputation
	if err := proto.Unmarshal(bz, &r); err != nil {
		return nil, false
	}
	return &r, true
}

// DeleteStratumReputation removes a stratum reputation record.
func (k Keeper) DeleteStratumReputation(ctx context.Context, stratum, validator string) {
	kvStore := k.storeService.OpenKVStore(ctx)
	_ = kvStore.Delete(stratumRepKey(stratum, validator))
}

// GetAllStratumReputations returns all stratum reputation records.
func (k Keeper) GetAllStratumReputations(ctx context.Context) []*types.StratumReputation {
	var reps []*types.StratumReputation
	k.IterateStratumReputations(ctx, func(r *types.StratumReputation) bool {
		reps = append(reps, r)
		return false
	})
	return reps
}

// IterateStratumReputations iterates all stratum reputations. Return true from cb to stop.
func (k Keeper) IterateStratumReputations(ctx context.Context, cb func(*types.StratumReputation) bool) {
	kvStore := k.storeService.OpenKVStore(ctx)
	iter, err := kvStore.Iterator(types.StratumReputationKeyPrefix, prefixEndBytes(types.StratumReputationKeyPrefix))
	if err != nil {
		return
	}
	defer iter.Close()

	for ; iter.Valid(); iter.Next() {
		var r types.StratumReputation
		if err := proto.Unmarshal(iter.Value(), &r); err != nil {
			continue
		}
		if cb(&r) {
			break
		}
	}
}

// ---------- DomainReputation ----------

// SetDomainReputation stores a domain reputation record.
func (k Keeper) SetDomainReputation(ctx context.Context, r *types.DomainReputation) {
	kvStore := k.storeService.OpenKVStore(ctx)
	bz, err := proto.Marshal(r)
	if err != nil {
		panic(fmt.Sprintf("failed to marshal domain reputation: %v", err))
	}
	_ = kvStore.Set(domainRepKey(r.Domain, r.Validator), bz)
}

// GetDomainReputation retrieves a domain reputation by domain and validator.
func (k Keeper) GetDomainReputation(ctx context.Context, domain, validator string) (*types.DomainReputation, bool) {
	kvStore := k.storeService.OpenKVStore(ctx)
	bz, err := kvStore.Get(domainRepKey(domain, validator))
	if err != nil || bz == nil {
		return nil, false
	}
	var r types.DomainReputation
	if err := proto.Unmarshal(bz, &r); err != nil {
		return nil, false
	}
	return &r, true
}

// DeleteDomainReputation removes a domain reputation record.
func (k Keeper) DeleteDomainReputation(ctx context.Context, domain, validator string) {
	kvStore := k.storeService.OpenKVStore(ctx)
	_ = kvStore.Delete(domainRepKey(domain, validator))
}

// GetAllDomainReputations returns all domain reputation records.
func (k Keeper) GetAllDomainReputations(ctx context.Context) []*types.DomainReputation {
	var reps []*types.DomainReputation
	k.IterateDomainReputations(ctx, func(r *types.DomainReputation) bool {
		reps = append(reps, r)
		return false
	})
	return reps
}

// IterateDomainReputations iterates all domain reputations. Return true from cb to stop.
func (k Keeper) IterateDomainReputations(ctx context.Context, cb func(*types.DomainReputation) bool) {
	kvStore := k.storeService.OpenKVStore(ctx)
	iter, err := kvStore.Iterator(types.DomainReputationKeyPrefix, prefixEndBytes(types.DomainReputationKeyPrefix))
	if err != nil {
		return
	}
	defer iter.Close()

	for ; iter.Valid(); iter.Next() {
		var r types.DomainReputation
		if err := proto.Unmarshal(iter.Value(), &r); err != nil {
			continue
		}
		if cb(&r) {
			break
		}
	}
}

// ---------- CaptureMetrics ----------

// SetCaptureMetrics stores capture metrics for a domain.
func (k Keeper) SetCaptureMetrics(ctx context.Context, m *types.CaptureMetrics) {
	kvStore := k.storeService.OpenKVStore(ctx)
	bz, err := proto.Marshal(m)
	if err != nil {
		panic(fmt.Sprintf("failed to marshal capture metrics: %v", err))
	}
	_ = kvStore.Set(captureMetricsKey(m.Domain), bz)
}

// GetCaptureMetrics retrieves capture metrics by domain.
func (k Keeper) GetCaptureMetrics(ctx context.Context, domain string) (*types.CaptureMetrics, bool) {
	kvStore := k.storeService.OpenKVStore(ctx)
	bz, err := kvStore.Get(captureMetricsKey(domain))
	if err != nil || bz == nil {
		return nil, false
	}
	var m types.CaptureMetrics
	if err := proto.Unmarshal(bz, &m); err != nil {
		return nil, false
	}
	return &m, true
}

// GetDomainCapturePenalty returns whether a domain is flagged and its HHI as penalty BPS (R31-1).
// Metal controls Wood: penalty scales with HHI concentration.
func (k Keeper) GetDomainCapturePenalty(ctx context.Context, domain string) (bool, uint64) {
	metrics, found := k.GetCaptureMetrics(ctx, domain)
	if !found || !metrics.Flagged {
		return false, 0
	}
	return true, metrics.HerfindahlIndex
}

// DeleteCaptureMetrics removes capture metrics for a domain.
func (k Keeper) DeleteCaptureMetrics(ctx context.Context, domain string) {
	kvStore := k.storeService.OpenKVStore(ctx)
	_ = kvStore.Delete(captureMetricsKey(domain))
}

// GetAllCaptureMetrics returns all capture metrics records.
func (k Keeper) GetAllCaptureMetrics(ctx context.Context) []*types.CaptureMetrics {
	var metrics []*types.CaptureMetrics
	k.IterateCaptureMetrics(ctx, func(m *types.CaptureMetrics) bool {
		metrics = append(metrics, m)
		return false
	})
	return metrics
}

// IterateCaptureMetrics iterates all capture metrics. Return true from cb to stop.
func (k Keeper) IterateCaptureMetrics(ctx context.Context, cb func(*types.CaptureMetrics) bool) {
	kvStore := k.storeService.OpenKVStore(ctx)
	iter, err := kvStore.Iterator(types.CaptureMetricsKeyPrefix, prefixEndBytes(types.CaptureMetricsKeyPrefix))
	if err != nil {
		return
	}
	defer iter.Close()

	for ; iter.Valid(); iter.Next() {
		var m types.CaptureMetrics
		if err := proto.Unmarshal(iter.Value(), &m); err != nil {
			continue
		}
		if cb(&m) {
			break
		}
	}
}

// ClearCaptureFlag unflags a domain by setting Flagged=false on its metrics.
func (k Keeper) ClearCaptureFlag(ctx context.Context, domain string) {
	metrics, found := k.GetCaptureMetrics(ctx, domain)
	if !found {
		return
	}
	metrics.Flagged = false
	k.SetCaptureMetrics(ctx, metrics)
}

// GetFlaggedDomainCount returns the number of domains currently flagged for capture risk.
func (k Keeper) GetFlaggedDomainCount(ctx context.Context) uint64 {
	var count uint64
	k.IterateCaptureMetrics(ctx, func(m *types.CaptureMetrics) bool {
		if m.Flagged {
			count++
		}
		return false
	})
	return count
}

// ---------- VerificationHistory ----------

// SetVerificationHistory stores a verification history entry.
func (k Keeper) SetVerificationHistory(ctx context.Context, entry *types.VerificationHistoryEntry) {
	kvStore := k.storeService.OpenKVStore(ctx)
	bz, err := proto.Marshal(entry)
	if err != nil {
		panic(fmt.Sprintf("failed to marshal verification history: %v", err))
	}
	_ = kvStore.Set(historyKey(entry.Domain, entry.RoundId), bz)
}

// GetVerificationHistory retrieves a verification history entry by domain and round ID.
func (k Keeper) GetVerificationHistory(ctx context.Context, domain, roundId string) (*types.VerificationHistoryEntry, bool) {
	kvStore := k.storeService.OpenKVStore(ctx)
	bz, err := kvStore.Get(historyKey(domain, roundId))
	if err != nil || bz == nil {
		return nil, false
	}
	var entry types.VerificationHistoryEntry
	if err := proto.Unmarshal(bz, &entry); err != nil {
		return nil, false
	}
	return &entry, true
}

// DeleteVerificationHistory removes a verification history entry.
func (k Keeper) DeleteVerificationHistory(ctx context.Context, domain, roundId string) {
	kvStore := k.storeService.OpenKVStore(ctx)
	_ = kvStore.Delete(historyKey(domain, roundId))
}

// GetHistoryByDomain returns all verification history entries for a domain.
func (k Keeper) GetHistoryByDomain(ctx context.Context, domain string) []*types.VerificationHistoryEntry {
	kvStore := k.storeService.OpenKVStore(ctx)
	prefix := historyDomainPrefix(domain)
	iter, err := kvStore.Iterator(prefix, prefixEndBytes(prefix))
	if err != nil {
		return nil
	}
	defer iter.Close()

	var entries []*types.VerificationHistoryEntry
	for ; iter.Valid(); iter.Next() {
		var entry types.VerificationHistoryEntry
		if err := proto.Unmarshal(iter.Value(), &entry); err != nil {
			continue
		}
		entries = append(entries, &entry)
	}
	return entries
}

// GetDomainsWithHistory returns unique domain names that have verification history.
func (k Keeper) GetDomainsWithHistory(ctx context.Context) []string {
	kvStore := k.storeService.OpenKVStore(ctx)
	iter, err := kvStore.Iterator(types.VerificationHistoryKeyPrefix, prefixEndBytes(types.VerificationHistoryKeyPrefix))
	if err != nil {
		return nil
	}
	defer iter.Close()

	seen := make(map[string]bool)
	var domains []string
	for ; iter.Valid(); iter.Next() {
		var entry types.VerificationHistoryEntry
		if err := proto.Unmarshal(iter.Value(), &entry); err != nil {
			continue
		}
		if !seen[entry.Domain] {
			seen[entry.Domain] = true
			domains = append(domains, entry.Domain)
		}
	}
	return domains
}

// ---------- CrossStratumRequirement ----------

// SetCrossStratumRequirement stores a cross-stratum requirement.
func (k Keeper) SetCrossStratumRequirement(ctx context.Context, c *types.CrossStratumRequirement) {
	kvStore := k.storeService.OpenKVStore(ctx)
	bz, err := proto.Marshal(c)
	if err != nil {
		panic(fmt.Sprintf("failed to marshal cross-stratum requirement: %v", err))
	}
	_ = kvStore.Set(crossStratumKey(c.TargetStratum), bz)
}

// GetCrossStratumRequirement retrieves a cross-stratum requirement by target stratum.
func (k Keeper) GetCrossStratumRequirement(ctx context.Context, targetStratum string) (*types.CrossStratumRequirement, bool) {
	kvStore := k.storeService.OpenKVStore(ctx)
	bz, err := kvStore.Get(crossStratumKey(targetStratum))
	if err != nil || bz == nil {
		return nil, false
	}
	var c types.CrossStratumRequirement
	if err := proto.Unmarshal(bz, &c); err != nil {
		return nil, false
	}
	return &c, true
}

// DeleteCrossStratumRequirement removes a cross-stratum requirement.
func (k Keeper) DeleteCrossStratumRequirement(ctx context.Context, targetStratum string) {
	kvStore := k.storeService.OpenKVStore(ctx)
	_ = kvStore.Delete(crossStratumKey(targetStratum))
}

// GetAllCrossStratumRequirements returns all cross-stratum requirements.
func (k Keeper) GetAllCrossStratumRequirements(ctx context.Context) []*types.CrossStratumRequirement {
	kvStore := k.storeService.OpenKVStore(ctx)
	iter, err := kvStore.Iterator(types.CrossStratumKeyPrefix, prefixEndBytes(types.CrossStratumKeyPrefix))
	if err != nil {
		return nil
	}
	defer iter.Close()

	var reqs []*types.CrossStratumRequirement
	for ; iter.Valid(); iter.Next() {
		var c types.CrossStratumRequirement
		if err := proto.Unmarshal(iter.Value(), &c); err != nil {
			continue
		}
		reqs = append(reqs, &c)
	}
	return reqs
}

// ---------- Key Construction Helpers ----------

func globalRepKey(validator string) []byte {
	return append(types.GlobalReputationKeyPrefix, []byte(validator)...)
}

func stratumRepKey(stratum, validator string) []byte {
	return append(types.StratumReputationKeyPrefix, []byte(stratum+"/"+validator)...)
}

func domainRepKey(domain, validator string) []byte {
	return append(types.DomainReputationKeyPrefix, []byte(domain+"/"+validator)...)
}

func captureMetricsKey(domain string) []byte {
	return append(types.CaptureMetricsKeyPrefix, []byte(domain)...)
}

func historyKey(domain, roundId string) []byte {
	return append(types.VerificationHistoryKeyPrefix, []byte(domain+"/"+roundId)...)
}

func historyDomainPrefix(domain string) []byte {
	return append(types.VerificationHistoryKeyPrefix, []byte(domain+"/")...)
}

func crossStratumKey(targetStratum string) []byte {
	return append(types.CrossStratumKeyPrefix, []byte(targetStratum)...)
}

// prefixEndBytes returns the end key for prefix iteration (exclusive).
func prefixEndBytes(prefix []byte) []byte {
	if len(prefix) == 0 {
		return nil
	}
	end := make([]byte, len(prefix))
	copy(end, prefix)
	for i := len(end) - 1; i >= 0; i-- {
		end[i]++
		if end[i] != 0 {
			return end
		}
	}
	return nil
}
