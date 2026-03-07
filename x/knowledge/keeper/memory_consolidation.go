package keeper

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"

	sdkmath "cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/zerone-chain/zerone/x/knowledge/types"
)

// ─── Memory Consolidation Engine (R50) ──────────────────────────────────────
//
// How biological memory works, applied to training data:
//
//   1. RETRIEVAL STRENGTHENING: Every time a TDU is used in training,
//      its activation record is updated. More activations = stronger trace.
//
//   2. SPACED REPETITION: Activations spread across many cycles are
//      worth more than clustered activations. Quality over quantity.
//
//   3. CONSOLIDATION ("Sleep"): Periodic on-chain events review activation
//      patterns and promote TDUs through memory tiers.
//
//   4. HEBBIAN LEARNING: "Neurons that fire together wire together."
//      TDUs frequently used together in the same training batch form
//      associations that strengthen both.
//
//   5. DECAY PROTECTION: Higher memory tiers decay slower.
//      Canonical data is immune — it's the chain's permanent knowledge.

// ─── RecordActivation ───────────────────────────────────────────────────────

// RecordActivation records that a TDU was retrieved and used in a training run.
// This is the "retrieval strengthening" mechanism — the testing effect.
func (k Keeper) RecordActivation(ctx context.Context, sampleID string, currentCycle uint64, modelDelta sdkmath.LegacyDec) error {
	sdkCtx := sdk.UnwrapSDKContext(ctx)

	record, found := k.GetActivationRecord(ctx, sampleID)
	if !found {
		record = &types.ActivationRecord{
			SampleID:        sampleID,
			FirstActivation: currentCycle,
			MemoryTier:      int(types.MemoryTierWorking),
			CoActivations:   make(map[string]uint64),
		}
	}

	record.TotalActivations++
	record.LastActivation = currentCycle

	// Track unique cycles (spaced repetition).
	cycles := parseCycleList(record.ActivationCycles)
	isNewCycle := true
	for _, c := range cycles {
		if c == currentCycle {
			isNewCycle = false
			break
		}
	}
	if isNewCycle {
		record.UniqueCycles++
		cycles = append(cycles, currentCycle)
		// Keep last 100 cycles for compactness.
		if len(cycles) > 100 {
			cycles = cycles[len(cycles)-100:]
		}
		record.ActivationCycles = formatCycleList(cycles)
	}

	// Track performance correlation.
	zero := sdkmath.LegacyZeroDec()
	if modelDelta.GT(zero) {
		record.PositiveOutcomes++
	} else if modelDelta.LT(zero) {
		record.NegativeOutcomes++
	}

	// Update average model delta (exponential moving average).
	if record.AvgModelDelta == "" {
		record.AvgModelDelta = modelDelta.String()
	} else {
		avgDelta, err := sdkmath.LegacyNewDecFromStr(record.AvgModelDelta)
		if err != nil {
			avgDelta = zero
		}
		// EMA: new_avg = 0.7 * old_avg + 0.3 * new_delta
		emaWeight := sdkmath.LegacyNewDecWithPrec(7, 1)
		newWeight := sdkmath.LegacyNewDecWithPrec(3, 1)
		newAvg := avgDelta.Mul(emaWeight).Add(modelDelta.Mul(newWeight))
		record.AvgModelDelta = newAvg.String()
	}

	// Check if this activation promotes the tier (immediate promotions for low tiers).
	params := k.GetConsolidationParams(ctx)
	k.checkImmediatePromotion(record, &params, currentCycle)

	if err := k.SetActivationRecord(ctx, record); err != nil {
		return err
	}

	sdkCtx.EventManager().EmitEvent(sdk.NewEvent(
		types.EventActivationRecorded,
		sdk.NewAttribute(types.AttributeSampleID, sampleID),
		sdk.NewAttribute(types.AttributeActivationCount, fmt.Sprintf("%d", record.TotalActivations)),
		sdk.NewAttribute(types.AttributeMemoryTier, types.MemoryTier(record.MemoryTier).String()),
	))

	return nil
}

// ─── RecordCoActivation ─────────────────────────────────────────────────────

// RecordCoActivation records that two TDUs were used together in the same training batch.
// Hebbian principle: "neurons that fire together wire together."
func (k Keeper) RecordCoActivation(ctx context.Context, tduIDs []string, currentCycle uint64) error {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	params := k.GetConsolidationParams(ctx)

	for i, idA := range tduIDs {
		recordA, found := k.GetActivationRecord(ctx, idA)
		if !found {
			continue // only track co-activations for TDUs with activation records
		}
		if recordA.CoActivations == nil {
			recordA.CoActivations = make(map[string]uint64)
		}

		for j, idB := range tduIDs {
			if i == j {
				continue
			}
			recordA.CoActivations[idB]++

			// Emit event when Hebbian threshold is crossed.
			if recordA.CoActivations[idB] == params.HebbianThreshold {
				sdkCtx.EventManager().EmitEvent(sdk.NewEvent(
					types.EventHebbianAssociation,
					sdk.NewAttribute(types.AttributeSampleID, idA),
					sdk.NewAttribute(types.AttributeCoActivatedWith, idB),
					sdk.NewAttribute(types.AttributeActivationCount, fmt.Sprintf("%d", recordA.CoActivations[idB])),
				))
			}
		}

		// Cap co-activation map size to prevent unbounded growth.
		if len(recordA.CoActivations) > 50 {
			recordA.CoActivations = pruneCoActivations(recordA.CoActivations, 30)
		}

		_ = k.SetActivationRecord(ctx, recordA)
	}

	return nil
}

// ─── RunConsolidation ───────────────────────────────────────────────────────

// RunConsolidation performs a "sleep cycle" — reviewing all activation records
// and promoting TDUs that meet the criteria for higher memory tiers.
// Called from BeginBlocker at ConsolidationInterval.
func (k Keeper) RunConsolidation(ctx context.Context, currentCycle uint64) (promoted uint64, err error) {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	params := k.GetConsolidationParams(ctx)

	consolidatedMinSpacing, _ := sdkmath.LegacyNewDecFromStr(params.ConsolidatedMinSpacing)
	consolidatedMinPerf, _ := sdkmath.LegacyNewDecFromStr(params.ConsolidatedMinPerformance)
	canonicalMinStrength, _ := sdkmath.LegacyNewDecFromStr(params.CanonicalMinStrength)

	var totalPromoted uint64

	k.IterateActivationRecords(ctx, func(record types.ActivationRecord) bool {
		currentTier := types.MemoryTier(record.MemoryTier)
		newTier := currentTier

		switch currentTier {
		case types.MemoryTierWorking:
			// Working → Active: just needs minimum activations.
			if record.TotalActivations >= params.ActiveMinActivations {
				newTier = types.MemoryTierActive
			}

		case types.MemoryTierActive:
			// Active → Consolidated: needs activations + spacing + performance.
			if record.TotalActivations >= params.ConsolidatedMinActivations &&
				record.GetSpacingFactor().GTE(consolidatedMinSpacing) {
				// Check performance ratio.
				total := record.PositiveOutcomes + record.NegativeOutcomes
				if total > 0 {
					perfRatio := sdkmath.LegacyNewDec(int64(record.PositiveOutcomes)).Quo(
						sdkmath.LegacyNewDec(int64(total)),
					)
					if perfRatio.GTE(consolidatedMinPerf) {
						newTier = types.MemoryTierConsolidated
					}
				}
			}

		case types.MemoryTierConsolidated:
			// Consolidated → Canonical: needs high strength + activations + age.
			age := uint64(0)
			if currentCycle > record.FirstActivation {
				age = currentCycle - record.FirstActivation
			}
			if record.TotalActivations >= params.CanonicalMinActivations &&
				record.GetRetrievalStrength().GTE(canonicalMinStrength) &&
				age >= params.CanonicalMinAge {
				newTier = types.MemoryTierCanonical
			}
		}

		if newTier != currentTier {
			record.MemoryTier = int(newTier)
			if newTier == types.MemoryTierConsolidated && record.ConsolidatedAt == 0 {
				record.ConsolidatedAt = currentCycle
			}
			if newTier == types.MemoryTierCanonical && record.CanonicalAt == 0 {
				record.CanonicalAt = currentCycle
			}

			_ = k.SetActivationRecord(ctx, &record)
			totalPromoted++

			sdkCtx.EventManager().EmitEvent(sdk.NewEvent(
				types.EventTierPromoted,
				sdk.NewAttribute(types.AttributeSampleID, record.SampleID),
				sdk.NewAttribute(types.AttributeMemoryTier, newTier.String()),
				sdk.NewAttribute(types.AttributeRetrievalStrength, record.GetRetrievalStrength().String()),
			))

			// Apply tier-modified decay to fitness record.
			k.applyTierDecayModifier(ctx, record.SampleID, newTier)
		}

		return false // continue iteration
	})

	sdkCtx.EventManager().EmitEvent(sdk.NewEvent(
		types.EventMemoryConsolidated,
		sdk.NewAttribute("promoted_count", fmt.Sprintf("%d", totalPromoted)),
		sdk.NewAttribute("cycle", fmt.Sprintf("%d", currentCycle)),
	))

	return totalPromoted, nil
}

// ─── GetDecayModifiedRate ───────────────────────────────────────────────────

// GetDecayModifiedRate returns the effective decay rate for a TDU, considering
// its memory tier. Consolidated data decays 80% slower; canonical doesn't decay.
func (k Keeper) GetDecayModifiedRate(ctx context.Context, sampleID string, baseDecay sdkmath.LegacyDec) sdkmath.LegacyDec {
	record, found := k.GetActivationRecord(ctx, sampleID)
	if !found {
		return baseDecay // no activation record → full decay
	}
	multiplier := record.DecayMultiplier()
	return baseDecay.Mul(multiplier)
}

// ─── GetHebbianAssociates ───────────────────────────────────────────────────

// GetHebbianAssociates returns TDUs strongly associated with the given TDU
// through co-activation (Hebbian learning). Returns sorted by strength.
func (k Keeper) GetHebbianAssociates(ctx context.Context, sampleID string) []HebbianAssociate {
	record, found := k.GetActivationRecord(ctx, sampleID)
	if !found || len(record.CoActivations) == 0 {
		return nil
	}

	params := k.GetConsolidationParams(ctx)
	var associates []HebbianAssociate
	for tduID, count := range record.CoActivations {
		if count >= params.HebbianThreshold {
			associates = append(associates, HebbianAssociate{
				SampleID:       tduID,
				CoActivations:  count,
			})
		}
	}

	sort.Slice(associates, func(i, j int) bool {
		return associates[i].CoActivations > associates[j].CoActivations
	})

	return associates
}

// HebbianAssociate represents a TDU associated through co-activation.
type HebbianAssociate struct {
	SampleID      string
	CoActivations uint64
}

// ─── Queries ────────────────────────────────────────────────────────────────

// GetActivationRecord retrieves a TDU's activation record.
func (k Keeper) GetActivationRecord(ctx context.Context, sampleID string) (*types.ActivationRecord, bool) {
	kvStore := k.storeService.OpenKVStore(ctx)
	bz, err := kvStore.Get(types.ActivationRecordKey(sampleID))
	if err != nil || bz == nil {
		return nil, false
	}
	var record types.ActivationRecord
	if err := json.Unmarshal(bz, &record); err != nil {
		return nil, false
	}
	return &record, true
}

// SetActivationRecord stores a TDU's activation record.
func (k Keeper) SetActivationRecord(ctx context.Context, record *types.ActivationRecord) error {
	kvStore := k.storeService.OpenKVStore(ctx)
	bz, err := json.Marshal(record)
	if err != nil {
		return fmt.Errorf("failed to marshal activation record: %w", err)
	}
	return kvStore.Set(types.ActivationRecordKey(record.SampleID), bz)
}

// IterateActivationRecords iterates all activation records.
func (k Keeper) IterateActivationRecords(ctx context.Context, cb func(record types.ActivationRecord) bool) {
	kvStore := k.storeService.OpenKVStore(ctx)
	prefix := types.ActivationRecordPrefix
	iter, err := kvStore.Iterator(prefix, prefixEndBytes(prefix))
	if err != nil {
		return
	}
	defer iter.Close()
	for ; iter.Valid(); iter.Next() {
		var record types.ActivationRecord
		if err := json.Unmarshal(iter.Value(), &record); err != nil {
			continue
		}
		if cb(record) {
			break
		}
	}
}

// GetConsolidationParams returns the consolidation parameters.
func (k Keeper) GetConsolidationParams(ctx context.Context) types.ConsolidationParams {
	kvStore := k.storeService.OpenKVStore(ctx)
	bz, err := kvStore.Get(types.ConsolidationParamsKey)
	if err != nil || bz == nil {
		return types.DefaultConsolidationParams()
	}
	var params types.ConsolidationParams
	if err := json.Unmarshal(bz, &params); err != nil {
		return types.DefaultConsolidationParams()
	}
	return params
}

// SetConsolidationParams stores the consolidation parameters.
func (k Keeper) SetConsolidationParams(ctx context.Context, params types.ConsolidationParams) error {
	kvStore := k.storeService.OpenKVStore(ctx)
	bz, err := json.Marshal(params)
	if err != nil {
		return fmt.Errorf("failed to marshal consolidation params: %w", err)
	}
	return kvStore.Set(types.ConsolidationParamsKey, bz)
}

// GetTDUsByTier returns all TDU sample IDs at a given memory tier.
func (k Keeper) GetTDUsByTier(ctx context.Context, tier types.MemoryTier) []string {
	var ids []string
	k.IterateActivationRecords(ctx, func(record types.ActivationRecord) bool {
		if types.MemoryTier(record.MemoryTier) == tier {
			ids = append(ids, record.SampleID)
		}
		return false
	})
	return ids
}

// ─── Internal ───────────────────────────────────────────────────────────────

// checkImmediatePromotion handles Working → Active promotion on activation
// (doesn't need to wait for the sleep cycle).
func (k Keeper) checkImmediatePromotion(record *types.ActivationRecord, params *types.ConsolidationParams, currentCycle uint64) {
	if types.MemoryTier(record.MemoryTier) == types.MemoryTierWorking &&
		record.TotalActivations >= params.ActiveMinActivations {
		record.MemoryTier = int(types.MemoryTierActive)
	}
}

// applyTierDecayModifier updates the fitness system to use tier-modified decay.
// This is how consolidation affects the fitness decay system.
func (k Keeper) applyTierDecayModifier(ctx context.Context, sampleID string, tier types.MemoryTier) {
	if tier == types.MemoryTierCanonical {
		// Canonical TDUs get their fitness pinned to at least Core level.
		fitnessRec, found := k.GetFitnessRecord(ctx, sampleID)
		if found {
			score := fitnessRec.GetFitnessScore()
			minCanonicalScore := sdkmath.LegacyNewDecWithPrec(8, 1) // 0.8
			if score.LT(minCanonicalScore) {
				fitnessRec.SetFitnessScore(minCanonicalScore)
				_ = k.SetFitnessRecord(ctx, fitnessRec)
			}
		}
	}
}

// pruneCoActivations keeps only the top-N co-activations by count.
func pruneCoActivations(coAct map[string]uint64, keepN int) map[string]uint64 {
	type entry struct {
		id    string
		count uint64
	}
	entries := make([]entry, 0, len(coAct))
	for id, count := range coAct {
		entries = append(entries, entry{id, count})
	}
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].count > entries[j].count
	})
	result := make(map[string]uint64, keepN)
	for i, e := range entries {
		if i >= keepN {
			break
		}
		result[e.id] = e.count
	}
	return result
}

// parseCycleList deserializes the compact cycle list.
func parseCycleList(s string) []uint64 {
	if s == "" {
		return nil
	}
	var cycles []uint64
	_ = json.Unmarshal([]byte(s), &cycles)
	return cycles
}

// formatCycleList serializes the cycle list compactly.
func formatCycleList(cycles []uint64) string {
	bz, _ := json.Marshal(cycles)
	return string(bz)
}
