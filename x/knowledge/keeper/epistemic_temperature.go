package keeper

import (
	"context"
	"encoding/json"
	"fmt"

	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/zerone-chain/zerone/x/knowledge/types"
)

// TemperatureCategory returns a human-readable category for a temperature value.
func TemperatureCategory(temp uint64) string {
	switch {
	case temp < 300_000:
		return "cold"
	case temp < 500_000:
		return "cool"
	case temp <= 700_000:
		return "neutral"
	case temp < 800_000:
		return "warm"
	default:
		return "hot"
	}
}

// emitTemperatureEvent emits an event when epistemic temperature is updated.
func (k Keeper) emitTemperatureEvent(ctx context.Context, domain string, state types.DomainEpistemicState) {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	sdkCtx.EventManager().EmitEvent(sdk.NewEvent(
		"zerone.knowledge.epistemic_temperature_changed",
		sdk.NewAttribute("domain", domain),
		sdk.NewAttribute("temperature_bps", fmt.Sprintf("%d", state.Temperature)),
		sdk.NewAttribute("category", TemperatureCategory(state.Temperature)),
		sdk.NewAttribute("conformity_streak", fmt.Sprintf("%d", state.ConformityStreak)),
		sdk.NewAttribute("recent_vindications", fmt.Sprintf("%d", state.VindicationCount)),
	))
}

// SetDomainEpistemicState stores the epistemic temperature state for a domain.
func (k Keeper) SetDomainEpistemicState(ctx context.Context, state *types.DomainEpistemicState) error {
	store := k.storeService.OpenKVStore(ctx)
	bz, err := json.Marshal(state)
	if err != nil {
		return fmt.Errorf("failed to marshal DomainEpistemicState: %w", err)
	}
	return store.Set(types.EpistemicStateKey(state.Domain), bz)
}

// GetDomainEpistemicState retrieves the epistemic temperature state for a domain.
func (k Keeper) GetDomainEpistemicState(ctx context.Context, domain string) (types.DomainEpistemicState, bool, error) {
	store := k.storeService.OpenKVStore(ctx)
	bz, err := store.Get(types.EpistemicStateKey(domain))
	if err != nil {
		return types.DomainEpistemicState{}, false, err
	}
	if bz == nil {
		return types.DomainEpistemicState{}, false, nil
	}
	var state types.DomainEpistemicState
	if err := json.Unmarshal(bz, &state); err != nil {
		return types.DomainEpistemicState{}, false, fmt.Errorf("failed to unmarshal DomainEpistemicState: %w", err)
	}
	return state, true, nil
}

// GetOrInitDomainEpistemicState returns existing state or creates neutral state.
func (k Keeper) GetOrInitDomainEpistemicState(ctx context.Context, domain string) (types.DomainEpistemicState, error) {
	state, found, err := k.GetDomainEpistemicState(ctx, domain)
	if err != nil {
		return types.DomainEpistemicState{}, err
	}
	if !found {
		return types.DomainEpistemicState{
			Domain:      domain,
			Temperature: NeutralBPS, // 500,000 = neutral
		}, nil
	}
	return state, nil
}

// CountVindicationsInWindow counts distinct vindication events (disproven facts)
// in the given domain within [currentHeight-windowBlocks, currentHeight].
// A vindication event is a fact that was disproven (has vindication records in the window).
func (k Keeper) CountVindicationsInWindow(ctx context.Context, domain string, currentHeight, windowBlocks uint64) uint64 {
	startHeight := uint64(0)
	if currentHeight > windowBlocks {
		startHeight = currentHeight - windowBlocks
	}

	count := uint64(0)
	k.IterateFactsByDomain(ctx, domain, func(factID string) bool {
		records := k.GetVindicationRecordsForFact(ctx, factID)
		for _, rec := range records {
			if rec.VindicatedAt >= startHeight && rec.VindicatedAt <= currentHeight {
				count++ // Count this fact as one vindication event
				break   // Don't double-count multiple verifiers on same fact
			}
		}
		return false
	})
	return count
}

// UpdateEpistemicTemperature recalculates a domain's epistemic temperature.
// Called from BeginBlocker at fitness epoch boundaries.
func (k Keeper) UpdateEpistemicTemperature(ctx context.Context, domain string) error {
	state, err := k.GetOrInitDomainEpistemicState(ctx, domain)
	if err != nil {
		return err
	}
	params, err := k.GetParams(ctx)
	if err != nil {
		return err
	}
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	height := uint64(sdkCtx.BlockHeight())
	neutral := uint64(NeutralBPS) // 500,000

	// 1. Decay toward neutral (500,000)
	if state.Temperature > neutral {
		diff := state.Temperature - neutral
		state.Temperature = neutral + safeMulDiv(diff, params.EpistemicTemperatureDecayBps, BPS)
	} else if state.Temperature < neutral {
		diff := neutral - state.Temperature
		state.Temperature = neutral - safeMulDiv(diff, params.EpistemicTemperatureDecayBps, BPS)
	}

	// 2. Conformity cooling — check current epoch diversity
	epoch := uint64(0)
	if params.FitnessEpochBlocks > 0 {
		epoch = height / params.FitnessEpochBlocks
	}
	rec, found, err := k.GetDomainDiversity(ctx, domain, epoch)
	if err != nil {
		return err
	}
	if found && rec.RoundCount > 0 && rec.AvgEntropy < params.DiversityConformityAlertThreshold {
		state.ConformityStreak++
		// Scale cooling by streak (capped at 10 for max effect)
		streak := state.ConformityStreak
		if streak > 10 {
			streak = 10
		}
		cooling := safeMulDiv(params.EpistemicConformityCoolingBps, streak, 10)
		if state.Temperature > cooling {
			state.Temperature -= cooling
		} else {
			state.Temperature = 0
		}
	} else {
		state.ConformityStreak = 0
	}

	// 3. Vindication heating
	windowBlocks := params.EpistemicTemperatureWindowBlocks
	if windowBlocks == 0 {
		windowBlocks = 10_000
	}
	recentVindications := k.CountVindicationsInWindow(ctx, domain, height, windowBlocks)
	if recentVindications > state.VindicationCount {
		newVindications := recentVindications - state.VindicationCount
		heating := safeMulDiv(params.EpistemicVindicationHeatingBps, newVindications, 1)
		state.Temperature += heating
		if state.Temperature > BPS {
			state.Temperature = BPS
		}
	}
	state.VindicationCount = recentVindications

	state.LastTemperatureUpdate = height

	// Emit temperature event
	k.emitTemperatureEvent(ctx, domain, state)
	return k.SetDomainEpistemicState(ctx, &state)
}

// AdvanceConfidence grows confidence for all active/verified facts by
// ConfidenceGrowthPerEpochBps, modulated by epistemic temperature.
// Called from BeginBlocker at ConfidenceGrowthEpoch intervals.
func (k Keeper) AdvanceConfidence(ctx context.Context) error {
	params, err := k.GetParams(ctx)
	if err != nil {
		return err
	}

	baseGrowthRate := params.ConfidenceGrowthPerEpochBps
	if baseGrowthRate == 0 {
		return nil // growth disabled
	}

	// Cache epistemic states per domain to avoid repeated lookups
	domainGrowthRates := make(map[string]uint64)

	k.IterateFacts(ctx, func(fact *types.Fact) bool {
		// Only grow confidence for active/verified/provisional facts
		switch fact.Status {
		case types.FactStatus_FACT_STATUS_VERIFIED,
			types.FactStatus_FACT_STATUS_ACTIVE,
			types.FactStatus_FACT_STATUS_PROVISIONAL:
		default:
			return false
		}

		growthRate, ok := domainGrowthRates[fact.Domain]
		if !ok {
			growthRate = baseGrowthRate
			epistemicState, found, err := k.GetDomainEpistemicState(ctx, fact.Domain)
			if err == nil && found {
				// Hot domains: confidence grows faster
				if epistemicState.Temperature > 700_000 && params.EpistemicHotConfidenceGrowthBps > 0 {
					growthRate = safeMulDiv(growthRate, params.EpistemicHotConfidenceGrowthBps, BPS)
				}
				// Cold domains: confidence grows slower (50% rate)
				if epistemicState.Temperature < 300_000 {
					growthRate = safeMulDiv(growthRate, 500_000, BPS)
				}
			}
			domainGrowthRates[fact.Domain] = growthRate
		}

		// Apply growth: confidence += confidence * growthRate / BPS
		growth := safeMulDiv(fact.Confidence, growthRate, BPS)
		if growth == 0 {
			growth = 1 // minimum 1 BPS growth per epoch
		}
		fact.Confidence += growth

		// Clamp to effective cap (includes epistemic temperature modulation)
		fact.Confidence = k.ClampConfidence(ctx, fact.Confidence, fact.Domain)

		if setErr := k.SetFact(ctx, fact); setErr != nil {
			return false
		}
		return false
	})

	return nil
}
