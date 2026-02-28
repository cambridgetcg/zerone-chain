package keeper

import (
	"context"
	"encoding/json"
	"fmt"

	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/zerone-chain/zerone/x/knowledge/types"
)

// DomainStats tracks the population of a domain for carrying capacity calculations.
type DomainStats struct {
	Domain      string `json:"domain"`
	ActiveCount uint64 `json:"active_count"`
	AtRiskCount uint64 `json:"at_risk_count"`
	TotalEnergy uint64 `json:"total_energy"`
	LastUpdated uint64 `json:"last_updated"` // block height
}

func (k Keeper) SetDomainStats(ctx context.Context, stats *DomainStats) {
	store := k.storeService.OpenKVStore(ctx)
	bz, _ := json.Marshal(stats)
	_ = store.Set(types.DomainStatsKey(stats.Domain), bz)
}

func (k Keeper) GetDomainStats(ctx context.Context, domain string) (*DomainStats, bool) {
	store := k.storeService.OpenKVStore(ctx)
	bz, err := store.Get(types.DomainStatsKey(domain))
	if err != nil || bz == nil {
		return &DomainStats{Domain: domain}, false
	}
	var stats DomainStats
	if err := json.Unmarshal(bz, &stats); err != nil {
		return &DomainStats{Domain: domain}, false
	}
	return &stats, true
}

func (k Keeper) IncrementDomainFactCount(ctx context.Context, domain string, isActive bool, energy uint64) {
	stats, _ := k.GetDomainStats(ctx, domain)
	stats.Domain = domain
	if isActive {
		stats.ActiveCount++
	} else {
		stats.AtRiskCount++
	}
	stats.TotalEnergy += energy
	k.SetDomainStats(ctx, stats)
}

func (k Keeper) DecrementDomainFactCount(ctx context.Context, domain string, wasActive bool, energy uint64) {
	stats, _ := k.GetDomainStats(ctx, domain)
	stats.Domain = domain
	if wasActive {
		if stats.ActiveCount > 0 {
			stats.ActiveCount--
		}
	} else {
		if stats.AtRiskCount > 0 {
			stats.AtRiskCount--
		}
	}
	if energy > stats.TotalEnergy {
		stats.TotalEnergy = 0
	} else {
		stats.TotalEnergy -= energy
	}
	k.SetDomainStats(ctx, stats)
}

// TransitionDomainFactStatus updates stats when a fact moves between active/at-risk.
func (k Keeper) TransitionDomainFactStatus(ctx context.Context, domain string, toActive bool) {
	stats, _ := k.GetDomainStats(ctx, domain)
	stats.Domain = domain
	if toActive {
		if stats.AtRiskCount > 0 {
			stats.AtRiskCount--
		}
		stats.ActiveCount++
	} else {
		if stats.ActiveCount > 0 {
			stats.ActiveCount--
		}
		stats.AtRiskCount++
	}
	k.SetDomainStats(ctx, stats)
}

// ─── Carrying capacity and pressure ─────────────────────────────────────────

const BPSCapacity = 1_000_000

func (k Keeper) GetDomainCarryingCapacity(ctx context.Context, domain string) uint64 {
	params, _ := k.GetParams(ctx)
	base := params.DomainBaseCapacity
	if base == 0 {
		base = 1000 // safety default
	}
	inbound := k.GetInboundCrossDomainCitationCount(ctx, domain)
	bonus := inbound * params.DomainCapacityGrowthPerCitation
	return base + bonus
}

func (k Keeper) GetDomainPressure(ctx context.Context, domain string) uint64 {
	stats, _ := k.GetDomainStats(ctx, domain)
	capacity := k.GetDomainCarryingCapacity(ctx, domain)
	if capacity == 0 {
		return BPSCapacity
	}
	population := stats.ActiveCount + stats.AtRiskCount
	return safeMulDiv(population, BPSCapacity, capacity)
}

// GetInboundCrossDomainCitationCount counts citations FROM other domains TO facts in this domain.
func (k Keeper) GetInboundCrossDomainCitationCount(ctx context.Context, domain string) uint64 {
	count := uint64(0)
	k.IterateFactsByDomain(ctx, domain, func(factID string) bool {
		incoming, err := k.GetIncomingRelations(ctx, factID)
		if err != nil {
			return false
		}
		for _, rel := range incoming {
			sourceFact, found := k.GetFact(ctx, rel.SourceFactId)
			if found && sourceFact.Domain != domain {
				count++
			}
		}
		return false
	})
	return count
}

// ─── Birth and death pressure ───────────────────────────────────────────────

// GetDeathPressureMultiplier returns the decay multiplier for a domain.
// >1M BPS = faster decay (overcrowded), <1M BPS = slower decay (sparse), 1M = normal.
func (k Keeper) GetDeathPressureMultiplier(ctx context.Context, domain string) uint64 {
	params, err := k.GetParams(ctx)
	if err != nil || params.DomainBaseCapacity == 0 {
		return BPSCapacity
	}
	pressure := k.GetDomainPressure(ctx, domain)

	if pressure > BPSCapacity {
		// Overcrowded: accelerate decay
		excess := pressure - BPSCapacity
		return BPSCapacity + safeMulDiv(excess, params.OvercrowdingDecayMultiplierBps-BPSCapacity, BPSCapacity)
	} else if pressure < BPSCapacity/2 {
		// Very sparse (< 50% capacity): slow decay to 75%
		return BPSCapacity * 3 / 4
	}
	return BPSCapacity // normal range
}

// ApplyBirthPressure adjusts initial energy based on domain pressure.
// Sparse domains give an energy bonus; overcrowded domains give no bonus.
func (k Keeper) ApplyBirthPressure(ctx context.Context, domain string, baseEnergy uint64) uint64 {
	params, err := k.GetParams(ctx)
	if err != nil || params.DomainBaseCapacity == 0 {
		return baseEnergy
	}
	pressure := k.GetDomainPressure(ctx, domain)
	if pressure >= BPSCapacity {
		return baseEnergy // at or over capacity — no bonus
	}
	// Under capacity: bonus proportional to sparseness
	sparseness := BPSCapacity - pressure
	bonus := safeMulDiv(
		safeMulDiv(baseEnergy, sparseness, BPSCapacity),
		params.UnderpopulationBirthBonusBps,
		BPSCapacity,
	)
	return baseEnergy + bonus
}

// PressureCategory returns a human-readable category for the pressure level.
func PressureCategory(pressureBps uint64) string {
	switch {
	case pressureBps < 250_000:
		return "sparse"
	case pressureBps < 750_000:
		return "normal"
	case pressureBps <= BPSCapacity:
		return "crowded"
	default:
		return "overcrowded"
	}
}

// ─── Events ─────────────────────────────────────────────────────────────────

// EmitDomainPressureEvent emits a domain_pressure_changed event.
func (k Keeper) EmitDomainPressureEvent(ctx context.Context, domain string) {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	stats, _ := k.GetDomainStats(ctx, domain)
	capacity := k.GetDomainCarryingCapacity(ctx, domain)
	pressure := k.GetDomainPressure(ctx, domain)
	category := PressureCategory(pressure)

	sdkCtx.EventManager().EmitEvent(sdk.NewEvent(
		"zerone.knowledge.domain_pressure_changed",
		sdk.NewAttribute("domain", domain),
		sdk.NewAttribute("active_count", fmt.Sprintf("%d", stats.ActiveCount)),
		sdk.NewAttribute("capacity", fmt.Sprintf("%d", capacity)),
		sdk.NewAttribute("pressure_bps", fmt.Sprintf("%d", pressure)),
		sdk.NewAttribute("category", category),
	))
}
