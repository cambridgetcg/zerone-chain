package keeper

import (
	"context"
	"encoding/json"
	"fmt"

	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/zerone-chain/zerone/x/capture_defense/types"
)

// ---------- Structural Immunity Params (R29-5) ----------

// GetStructuralImmunityParams returns the structural immunity parameters.
func (k Keeper) GetStructuralImmunityParams(ctx context.Context) *types.StructuralImmunityParams {
	kvStore := k.storeService.OpenKVStore(ctx)
	bz, err := kvStore.Get(types.StructuralImmunityParamsKey)
	if err != nil || bz == nil {
		return types.DefaultStructuralImmunityParams()
	}
	var params types.StructuralImmunityParams
	if err := json.Unmarshal(bz, &params); err != nil {
		return types.DefaultStructuralImmunityParams()
	}
	return &params
}

// SetStructuralImmunityParams stores the structural immunity parameters.
func (k Keeper) SetStructuralImmunityParams(ctx context.Context, params *types.StructuralImmunityParams) {
	kvStore := k.storeService.OpenKVStore(ctx)
	bz, err := json.Marshal(params)
	if err != nil {
		panic(fmt.Sprintf("failed to marshal structural immunity params: %v", err))
	}
	_ = kvStore.Set(types.StructuralImmunityParamsKey, bz)
}

// ---------- Adjusted HHI (R29-5) ----------

// CalculateAdjustedHHI reduces the raw HHI based on partnership density.
// Distributed social structure is natural capture defense.
func (k Keeper) CalculateAdjustedHHI(ctx sdk.Context, domain string, rawHHI uint64) uint64 {
	if k.partnershipsKeeper == nil {
		return rawHHI
	}

	siParams := k.GetStructuralImmunityParams(ctx)
	density := k.partnershipsKeeper.GetDomainPartnershipDensity(ctx, domain)

	if density > 0 {
		reductionBps := density * siParams.PartnershipHHIReductionPerParticipantBps
		if reductionBps > siParams.MaxPartnershipHHIReductionBps {
			reductionBps = siParams.MaxPartnershipHHIReductionBps
		}
		adjusted := rawHHI * (types.BPSScale - reductionBps) / types.BPSScale
		return adjusted
	}

	return rawHHI
}

// ---------- Domain Flagging with Partnership Bonus (R29-5) ----------

// OnDomainFlagged signals the partnerships module that a domain needs new entrants.
func (k Keeper) OnDomainFlagged(ctx sdk.Context, domain string) {
	if k.partnershipsKeeper == nil {
		return
	}

	siParams := k.GetStructuralImmunityParams(ctx)
	expiryHeight := uint64(ctx.BlockHeight()) + siParams.FormationBonusDurationBlocks

	k.partnershipsKeeper.SetDomainFormationBonus(
		ctx,
		domain,
		siParams.CapturedDomainFormationBonusBps,
		"capture_flagged",
		expiryHeight,
	)

	// Commitment 9 (cartel detection has consequence): a flagged
	// domain triggers a structural-immunity formation bonus —
	// partnerships in flagged domains earn higher rewards, breaking
	// up concentration through structure rather than mere
	// punishment. The bonus is the consequence pathway speaking.
	ctx.EventManager().EmitEvent(
		sdk.NewEvent("zerone.capture_defense.domain_formation_bonus_set",
			sdk.NewAttribute("domain", domain),
			sdk.NewAttribute("bonus_bps", fmt.Sprintf("%d", siParams.CapturedDomainFormationBonusBps)),
			sdk.NewAttribute("reason", "capture_flagged"),
			sdk.NewAttribute("expiry_height", fmt.Sprintf("%d", expiryHeight)),
			sdk.NewAttribute("creed_commitment", "9"),
		),
	)
}

// ---------- Structural Reputation Bonus (R29-5) ----------

// CalculateStructuralReputationBonus returns a reputation bonus for validators
// who have active partnerships in a domain. Invested in social structure = higher rep.
func (k Keeper) CalculateStructuralReputationBonus(ctx sdk.Context, validator, domain string) uint64 {
	if k.partnershipsKeeper == nil {
		return 0
	}

	siParams := k.GetStructuralImmunityParams(ctx)
	activeCount := k.partnershipsKeeper.GetPartnershipCountByParticipant(ctx, validator, domain)

	bonus := activeCount * siParams.PartnershipReputationBonusBps
	if bonus > siParams.MaxPartnershipReputationBonusBps {
		bonus = siParams.MaxPartnershipReputationBonusBps
	}
	return bonus
}

// ---------- Accelerated Flag Clearing (R29-5) ----------

// ShouldAccelerateClearFlag returns true if a domain has enough partnership density
// to warrant faster flag clearing.
func (k Keeper) ShouldAccelerateClearFlag(ctx sdk.Context, domain string) bool {
	if k.partnershipsKeeper == nil {
		return false
	}

	metrics, found := k.GetCaptureMetrics(ctx, domain)
	if !found || !metrics.Flagged {
		return false
	}

	siParams := k.GetStructuralImmunityParams(ctx)
	density := k.partnershipsKeeper.GetDomainPartnershipDensity(ctx, domain)
	return density >= siParams.MinDensityForAcceleratedClear
}

// ---------- IsDomainFlagged (R29-5) ----------

// IsDomainFlagged returns whether a domain is currently flagged for capture risk.
// Exported for use by the partnerships adapter.
func (k Keeper) IsDomainFlagged(ctx context.Context, domain string) bool {
	metrics, found := k.GetCaptureMetrics(ctx, domain)
	if !found {
		return false
	}
	return metrics.Flagged
}
