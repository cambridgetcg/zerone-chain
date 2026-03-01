package keeper

import (
	"context"
	"fmt"

	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/zerone-chain/zerone/x/capture_defense/types"
)

// UpdateReputation updates all three reputation layers for a validator in a domain.
func (k Keeper) UpdateReputation(ctx sdk.Context, validator string, domain string, stratum string, approved bool) {
	height := uint64(ctx.BlockHeight())

	// --- Global reputation ---
	global, found := k.GetGlobalReputation(ctx, validator)
	if !found {
		global = &types.GlobalReputation{
			Validator:        validator,
			Score:            500000, // 50% initial
			LastUpdatedBlock: height,
		}
	}
	global.TotalVerifications++
	if approved {
		// Increase score: move 1% toward max
		delta := (types.BPSScale - global.Score) / 100
		if delta < 1000 {
			delta = 1000
		}
		global.Score += delta
		if global.Score > types.BPSScale {
			global.Score = types.BPSScale
		}
	} else {
		// Decrease: move 2% toward zero
		delta := global.Score / 50
		if delta < 1000 {
			delta = 1000
		}
		if global.Score > delta {
			global.Score -= delta
		} else {
			global.Score = 0
		}
	}
	global.LastUpdatedBlock = height
	k.SetGlobalReputation(ctx, global)

	// --- Stratum reputation ---
	if stratum != "" {
		sr, found := k.GetStratumReputation(ctx, stratum, validator)
		if !found {
			sr = &types.StratumReputation{
				Validator:        validator,
				Stratum:          stratum,
				Score:            500000,
				LastUpdatedBlock: height,
			}
		}
		sr.Verifications++
		if approved {
			delta := (types.BPSScale - sr.Score) / 100
			if delta < 1000 {
				delta = 1000
			}
			sr.Score += delta
			if sr.Score > types.BPSScale {
				sr.Score = types.BPSScale
			}
		} else {
			delta := sr.Score / 50
			if delta < 1000 {
				delta = 1000
			}
			if sr.Score > delta {
				sr.Score -= delta
			} else {
				sr.Score = 0
			}
		}
		sr.LastUpdatedBlock = height
		k.SetStratumReputation(ctx, sr)
	}

	// --- Domain reputation ---
	if domain != "" {
		dr, found := k.GetDomainReputation(ctx, domain, validator)
		if !found {
			dr = &types.DomainReputation{
				Validator:        validator,
				Domain:           domain,
				Score:            500000,
				LastUpdatedBlock: height,
			}
		}
		dr.Verifications++
		if approved {
			delta := (types.BPSScale - dr.Score) / 100
			if delta < 1000 {
				delta = 1000
			}
			dr.Score += delta
			if dr.Score > types.BPSScale {
				dr.Score = types.BPSScale
			}
		} else {
			delta := dr.Score / 50
			if delta < 1000 {
				delta = 1000
			}
			if dr.Score > delta {
				dr.Score -= delta
			} else {
				dr.Score = 0
			}
		}
		dr.LastUpdatedBlock = height
		k.SetDomainReputation(ctx, dr)
	}
}

// GetEffectiveReputation returns the most specific reputation layer score if
// that layer has enough verifications; otherwise falls back to the next layer.
// Priority: domain > stratum > global.
func (k Keeper) GetEffectiveReputation(ctx sdk.Context, validator, domain, stratum string, params *types.Params) uint64 {
	minVerifications := params.MinVerificationsForScore

	// Try domain-level first
	if domain != "" {
		dr, found := k.GetDomainReputation(ctx, domain, validator)
		if found && dr.Verifications >= minVerifications {
			return dr.Score
		}
	}

	// Try stratum-level
	if stratum != "" {
		sr, found := k.GetStratumReputation(ctx, stratum, validator)
		if found && sr.Verifications >= minVerifications {
			return sr.Score
		}
	}

	// Fall back to global
	gr, found := k.GetGlobalReputation(ctx, validator)
	if found {
		return gr.Score
	}

	// No reputation at all: return base score
	return params.BaseReputationScore
}

// DecayReputation applies exponential decay toward a base score.
// Formula: base + (score - base) * 0.5^(age / halfLife)
// Uses integer approximation: for each full halfLife in age, halve the delta.
func DecayReputation(score, base, age, halfLife uint64) uint64 {
	if halfLife == 0 || age == 0 {
		return score
	}
	if score <= base {
		return score
	}

	delta := score - base
	periods := age / halfLife

	// Apply full half-life periods
	for i := uint64(0); i < periods && delta > 0; i++ {
		delta /= 2
	}

	// Apply fractional half-life: delta * (halfLife - remainder) / halfLife
	remainder := age % halfLife
	if remainder > 0 && delta > 0 {
		// Linear interpolation within the period
		delta = delta * (halfLife - remainder/2) / halfLife
	}

	return base + delta
}

// calculateReputationRecoveryRate returns the effective recovery rate for a domain,
// accelerated by verification activity (R31-2: Fire -> Metal).
func (k Keeper) calculateReputationRecoveryRate(ctx context.Context, domain string) uint64 {
	params := k.GetParams(ctx)
	baseRate := params.BaseReputationRecoveryBps
	if baseRate == 0 {
		baseRate = 50_000 // fallback default
	}

	if k.knowledgeKeeper == nil {
		return baseRate
	}

	activity := k.knowledgeKeeper.GetDomainVerificationActivity(ctx, domain)

	maxBonus := params.ActivityRecoveryBonusMaxBps
	if maxBonus == 0 {
		maxBonus = 500_000 // fallback default
	}

	// Activity bonus: scales linearly with verification activity
	activityBonus := activity * maxBonus / types.BPSScale
	recoveryRate := baseRate + (baseRate * activityBonus / types.BPSScale)

	sdkCtx := sdk.UnwrapSDKContext(ctx)
	sdkCtx.EventManager().EmitEvent(sdk.NewEvent(
		"zerone.capture_defense.activity_recovery_bonus",
		sdk.NewAttribute("domain", domain),
		sdk.NewAttribute("verification_activity_bps", fmt.Sprintf("%d", activity)),
		sdk.NewAttribute("recovery_rate_bps", fmt.Sprintf("%d", recoveryRate)),
		sdk.NewAttribute("bonus_bps", fmt.Sprintf("%d", activityBonus)),
	))

	return recoveryRate
}

// DecayAllReputations iterates all reputations and applies decay toward the base score.
func (k Keeper) DecayAllReputations(ctx sdk.Context, params *types.Params) {
	height := uint64(ctx.BlockHeight())
	base := params.BaseReputationScore
	halfLife := params.DecayEpochBlocks

	// Decay global reputations
	k.IterateGlobalReputations(ctx, func(r *types.GlobalReputation) bool {
		age := height - r.LastUpdatedBlock
		if age > 0 {
			r.Score = DecayReputation(r.Score, base, age, halfLife)
			r.LastUpdatedBlock = height
			k.SetGlobalReputation(ctx, r)
		}
		return false
	})

	// Decay stratum reputations
	k.IterateStratumReputations(ctx, func(r *types.StratumReputation) bool {
		age := height - r.LastUpdatedBlock
		if age > 0 {
			r.Score = DecayReputation(r.Score, base, age, halfLife)
			r.LastUpdatedBlock = height
			k.SetStratumReputation(ctx, r)
		}
		return false
	})

	// Decay domain reputations with activity-based recovery (R31-2: Fire -> Metal)
	k.IterateDomainReputations(ctx, func(r *types.DomainReputation) bool {
		age := height - r.LastUpdatedBlock
		if age > 0 {
			// Activity-based recovery: active domains have a higher effective floor
			effectiveBase := base
			if r.Domain != "" {
				recoveryRate := k.calculateReputationRecoveryRate(ctx, r.Domain)
				if recoveryRate > 0 {
					effectiveBase = base + (recoveryRate * (types.BPSScale - base) / types.BPSScale)
					if effectiveBase > types.BPSScale {
						effectiveBase = types.BPSScale
					}
				}
			}
			r.Score = DecayReputation(r.Score, effectiveBase, age, halfLife)
			r.LastUpdatedBlock = height
			k.SetDomainReputation(ctx, r)
		}
		return false
	})
}

// ValidateCrossStratum checks that a set of validators satisfies the cross-stratum
// requirements for a given target stratum. It verifies that for each required
// stratum, at least MinValidatorsPerStratum validators have sufficient reputation.
func (k Keeper) ValidateCrossStratum(ctx sdk.Context, domain, stratum string, validators []string) bool {
	req, found := k.GetCrossStratumRequirement(ctx, stratum)
	if !found {
		// No cross-stratum requirement: pass by default.
		return true
	}

	params := k.GetParams(ctx)

	for _, requiredStratum := range req.RequiredStrata {
		var qualified uint64
		for _, v := range validators {
			sr, found := k.GetStratumReputation(ctx, requiredStratum, v)
			if found && sr.Verifications >= params.MinVerificationsForScore {
				qualified++
			}
		}
		if qualified < req.MinValidatorsPerStratum {
			return false
		}
	}
	return true
}
