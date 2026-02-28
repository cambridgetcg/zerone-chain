package keeper

import (
	"context"
	"encoding/binary"
	"fmt"

	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/zerone-chain/zerone/x/knowledge/types"
)

// ProcessMetabolism runs one epoch of energy accounting for all active facts.
// Deducts maintenance cost, adds energy income, and transitions fact status
// based on energy levels.
func (k Keeper) ProcessMetabolism(ctx context.Context, epoch uint64) error {
	params, err := k.GetParams(ctx)
	if err != nil {
		return err
	}

	// Collect facts to process (avoid modifying store during iteration)
	var factsToProcess []*types.Fact
	k.IterateFacts(ctx, func(fact *types.Fact) bool {
		// Process all facts that are alive (not already pruned)
		if fact.Status == types.FactStatus_FACT_STATUS_VERIFIED ||
			fact.Status == types.FactStatus_FACT_STATUS_ACTIVE ||
			fact.Status == types.FactStatus_FACT_STATUS_PROVISIONAL ||
			fact.Status == types.FactStatus_FACT_STATUS_AT_RISK ||
			fact.Status == types.FactStatus_FACT_STATUS_EXPIRED {
			factsToProcess = append(factsToProcess, fact)
		}
		return false
	})

	domainCounts := k.CountFactsByDomain(ctx)
	sdkCtx := sdk.UnwrapSDKContext(ctx)

	for _, fact := range factsToProcess {
		// ─── Calculate maintenance cost ───────────────────
		cost := k.calculateMaintenanceCost(fact, params, domainCounts)

		// Apply domain carrying capacity death pressure (R29-1)
		deathMultiplier := k.GetDeathPressureMultiplier(ctx, fact.Domain)
		cost = safeMulDiv(cost, deathMultiplier, BPSCapacity)

		// ─── Calculate energy income ──────────────────────
		income := k.calculateEnergyIncome(ctx, fact, params)

		// ─── Update energy ────────────────────────────────
		newEnergy := fact.Energy + income
		if cost > newEnergy {
			newEnergy = 0
		} else {
			newEnergy -= cost
		}
		if newEnergy > params.MetabolismEnergyCap {
			newEnergy = params.MetabolismEnergyCap
		}

		// ─── State transitions (multi-level thresholds) ──────────
		oldStatus := fact.Status

		if newEnergy >= params.MetabolismActiveThreshold {
			// Healthy — clear any at-risk state
			if fact.AtRiskSinceEpoch > 0 {
				fact.AtRiskSinceEpoch = 0
				fact.Status = types.FactStatus_FACT_STATUS_ACTIVE
			}
		} else if newEnergy < params.MetabolismActiveThreshold {
			// Below active threshold
			if fact.AtRiskSinceEpoch == 0 {
				// Just entered at-risk zone
				fact.AtRiskSinceEpoch = epoch
				fact.Status = types.FactStatus_FACT_STATUS_AT_RISK
			} else {
				// Already at risk — check for expiry/pruning
				atRiskDuration := epoch - fact.AtRiskSinceEpoch
				if atRiskDuration >= params.MetabolismAtRiskEpochs+params.MetabolismExpiredToPrunedEpochs {
					fact.Status = types.FactStatus_FACT_STATUS_PRUNED
				} else if atRiskDuration >= params.MetabolismAtRiskEpochs {
					fact.Status = types.FactStatus_FACT_STATUS_EXPIRED
				}
			}
		}

		fact.Energy = newEnergy
		fact.EnergyLastUpdated = uint64(sdkCtx.BlockHeight())
		if err := k.SetFact(ctx, fact); err != nil {
			k.Logger(ctx).Error("failed to update fact energy", "fact_id", fact.Id, "error", err)
			continue
		}

		// Emit event on status change and update domain stats (R29-1)
		if oldStatus != fact.Status {
			switch {
			case fact.Status == types.FactStatus_FACT_STATUS_AT_RISK:
				k.TransitionDomainFactStatus(ctx, fact.Domain, false) // active → at-risk
			case fact.Status == types.FactStatus_FACT_STATUS_ACTIVE &&
				(oldStatus == types.FactStatus_FACT_STATUS_AT_RISK || oldStatus == types.FactStatus_FACT_STATUS_EXPIRED):
				k.TransitionDomainFactStatus(ctx, fact.Domain, true) // at-risk → active
			case fact.Status == types.FactStatus_FACT_STATUS_EXPIRED ||
				fact.Status == types.FactStatus_FACT_STATUS_PRUNED:
				wasActive := oldStatus == types.FactStatus_FACT_STATUS_ACTIVE ||
					oldStatus == types.FactStatus_FACT_STATUS_VERIFIED ||
					oldStatus == types.FactStatus_FACT_STATUS_PROVISIONAL
				k.DecrementDomainFactCount(ctx, fact.Domain, wasActive, fact.Energy)
			}
			k.emitMetabolismStatusEvent(ctx, fact, oldStatus, epoch)
		}
	}

	// Reset epoch citation counters for all facts
	k.ResetEpochCitationCounters(ctx)

	k.Logger(ctx).Info("metabolism processed",
		"facts_processed", len(factsToProcess),
		"epoch", epoch,
	)

	return nil
}

// calculateMaintenanceCost returns the energy drain for a fact this epoch.
func (k Keeper) calculateMaintenanceCost(fact *types.Fact, params *types.Params, domainCounts map[string]uint64) uint64 {
	base := params.MetabolismBaseCost

	// Content length factor: scaled by BPS per 100 chars
	contentLen := uint64(len(fact.Content))
	contentFactor := safeMulDiv(base, params.MetabolismContentLengthBps*(contentLen/100), 1_000_000)

	// Domain competition factor: scaled by BPS per 100 facts in domain
	domainCount := domainCounts[fact.Domain]
	competitionFactor := safeMulDiv(base, params.MetabolismDomainCompetitionBps*(domainCount/100), 1_000_000)

	// Add competition tax from niche dynamics
	totalCost := base + contentFactor + competitionFactor + fact.CompetitionTax
	return totalCost
}

// calculateEnergyIncome returns the energy gained this epoch.
func (k Keeper) calculateEnergyIncome(ctx context.Context, fact *types.Fact, params *types.Params) uint64 {
	income := uint64(0)

	// Demand-weighted query energy
	subject := ""
	if fact.Structure != nil {
		subject = fact.Structure.Subject
	}
	demandMultiplier := k.GetDemandMultiplier(ctx, fact.Domain, subject)
	income += fact.QueryCountEpoch * params.MetabolismEnergyPerQuery * demandMultiplier / 1_000_000

	// Citation energy (new citations this epoch)
	newCitations := k.GetNewCitationsThisEpoch(ctx, fact.Id)
	income += newCitations * params.MetabolismEnergyPerCitation

	// Patronage energy
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	if fact.PatronageAmount != "" && fact.PatronageAmount != "0" {
		if uint64(sdkCtx.BlockHeight()) < fact.PatronageExpiryBlock {
			income += params.MetabolismEnergyPerPatronage
		}
	}

	return income
}

// CountFactsByDomain returns a map of domain → active fact count.
func (k Keeper) CountFactsByDomain(ctx context.Context) map[string]uint64 {
	counts := make(map[string]uint64)
	k.IterateFacts(ctx, func(fact *types.Fact) bool {
		if fact.Status == types.FactStatus_FACT_STATUS_VERIFIED ||
			fact.Status == types.FactStatus_FACT_STATUS_ACTIVE ||
			fact.Status == types.FactStatus_FACT_STATUS_PROVISIONAL ||
			fact.Status == types.FactStatus_FACT_STATUS_AT_RISK {
			counts[fact.Domain]++
		}
		return false
	})
	return counts
}

// GetNewCitationsThisEpoch returns the number of new incoming citations for a fact this epoch.
func (k Keeper) GetNewCitationsThisEpoch(ctx context.Context, factID string) uint64 {
	store := k.storeService.OpenKVStore(ctx)
	key := append(append([]byte{}, types.NewCitationsEpochPrefix...), []byte(factID)...)
	bz, err := store.Get(key)
	if err != nil || bz == nil || len(bz) < 8 {
		return 0
	}
	return binary.BigEndian.Uint64(bz)
}

// IncrementNewCitationEpoch increments the new citation counter for a fact in the current epoch.
func (k Keeper) IncrementNewCitationEpoch(ctx context.Context, factID string) {
	store := k.storeService.OpenKVStore(ctx)
	key := append(append([]byte{}, types.NewCitationsEpochPrefix...), []byte(factID)...)
	count := k.GetNewCitationsThisEpoch(ctx, factID) + 1
	bz := make([]byte, 8)
	binary.BigEndian.PutUint64(bz, count)
	_ = store.Set(key, bz)
}

// ResetEpochCitationCounters clears all per-epoch citation counters.
func (k Keeper) ResetEpochCitationCounters(ctx context.Context) {
	store := k.storeService.OpenKVStore(ctx)
	iter, err := store.Iterator(types.NewCitationsEpochPrefix, prefixEndBytes(types.NewCitationsEpochPrefix))
	if err != nil {
		return
	}
	defer iter.Close()

	var keysToDelete [][]byte
	for ; iter.Valid(); iter.Next() {
		keysToDelete = append(keysToDelete, iter.Key())
	}
	for _, key := range keysToDelete {
		_ = store.Delete(key)
	}
}

// ApplyPatronageEnergyBoost gives an immediate energy boost when patronage is set.
// Boost is proportional to patronage duration: MetabolismEnergyPerPatronage * epochs / 10.
func (k Keeper) ApplyPatronageEnergyBoost(ctx context.Context, fact *types.Fact, durationBlocks uint64, patronAddr string) {
	params, err := k.GetParams(ctx)
	if err != nil {
		return
	}

	durationEpochs := uint64(1)
	if params.FitnessEpochBlocks > 0 {
		durationEpochs = durationBlocks / params.FitnessEpochBlocks
		if durationEpochs == 0 {
			durationEpochs = 1
		}
	}

	boost := params.MetabolismEnergyPerPatronage * durationEpochs / 10
	if boost == 0 {
		boost = params.MetabolismEnergyPerPatronage // minimum one epoch worth
	}

	// Apply human patronage bonus (R28-5)
	if params.HumanPatronageBonusBps > 0 && patronAddr != "" {
		accountType := k.getAccountType(ctx, patronAddr)
		if accountType == "human" {
			boost = safeMulDiv(boost, 1_000_000+params.HumanPatronageBonusBps, 1_000_000)
		}
	}

	oldStatus := fact.Status
	fact.Energy += boost
	if fact.Energy > params.MetabolismEnergyCap {
		fact.Energy = params.MetabolismEnergyCap
	}

	// Recover from AT_RISK if energy is above active threshold
	if (fact.Status == types.FactStatus_FACT_STATUS_AT_RISK || fact.Status == types.FactStatus_FACT_STATUS_EXPIRED) &&
		fact.Energy >= params.MetabolismActiveThreshold {
		fact.AtRiskSinceEpoch = 0
		fact.Status = types.FactStatus_FACT_STATUS_ACTIVE
	}

	sdkCtx := sdk.UnwrapSDKContext(ctx)
	fact.EnergyLastUpdated = uint64(sdkCtx.BlockHeight())
	_ = k.SetFact(ctx, fact)

	if oldStatus != fact.Status {
		sdkCtx.EventManager().EmitEvent(sdk.NewEvent(
			"zerone.knowledge.fact_status_changed",
			sdk.NewAttribute("fact_id", fact.Id),
			sdk.NewAttribute("old_status", oldStatus.String()),
			sdk.NewAttribute("new_status", fact.Status.String()),
			sdk.NewAttribute("energy", fmt.Sprintf("%d", fact.Energy)),
			sdk.NewAttribute("reason", "patronage_recovery"),
			sdk.NewAttribute("epoch", "0"),
		))
	}
}

// emitMetabolismStatusEvent emits a unified fact_status_changed event.
func (k Keeper) emitMetabolismStatusEvent(ctx context.Context, fact *types.Fact, oldStatus types.FactStatus, epoch uint64) {
	sdkCtx := sdk.UnwrapSDKContext(ctx)

	reason := "decay"
	if fact.Status == types.FactStatus_FACT_STATUS_ACTIVE &&
		(oldStatus == types.FactStatus_FACT_STATUS_AT_RISK || oldStatus == types.FactStatus_FACT_STATUS_EXPIRED) {
		reason = "recovery"
	} else if fact.Status == types.FactStatus_FACT_STATUS_EXPIRED {
		reason = "extinction"
	} else if fact.Status == types.FactStatus_FACT_STATUS_PRUNED {
		reason = "extinction"
	}

	sdkCtx.EventManager().EmitEvent(sdk.NewEvent(
		"zerone.knowledge.fact_status_changed",
		sdk.NewAttribute("fact_id", fact.Id),
		sdk.NewAttribute("old_status", oldStatus.String()),
		sdk.NewAttribute("new_status", fact.Status.String()),
		sdk.NewAttribute("energy", fmt.Sprintf("%d", fact.Energy)),
		sdk.NewAttribute("reason", reason),
		sdk.NewAttribute("epoch", fmt.Sprintf("%d", epoch)),
	))
}
