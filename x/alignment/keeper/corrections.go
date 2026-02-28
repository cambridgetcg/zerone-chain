package keeper

import (
	"context"
	"fmt"

	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/zerone-chain/zerone/x/alignment/types"
)

// GenerateCorrections checks each dimension against thresholds and produces correction records.
func (k Keeper) GenerateCorrections(ctx context.Context, scores *types.DimensionScores) []*types.CorrectionRecord {
	params := k.GetParams(ctx)
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	height := uint64(sdkCtx.BlockHeight())
	now := sdkCtx.BlockTime().Unix()

	var corrections []*types.CorrectionRecord

	// Knowledge quality: suggest reward increase if low.
	if scores.KnowledgeQuality < params.DegradedThreshold {
		mag := params.DegradedThreshold - scores.KnowledgeQuality
		if scores.KnowledgeQuality < params.CriticalThreshold {
			mag *= 2
		}
		corrections = append(corrections, &types.CorrectionRecord{
			Height:    height,
			Dimension: types.DimKnowledgeQuality,
			Parameter: "knowledge.reward_multiplier",
			Direction: "increase",
			Magnitude: mag,
			Applied:   false,
			Timestamp: now,
		})
	}

	// Economic stability: 2× magnitude correction when critical.
	if scores.EconomicStability < params.DegradedThreshold {
		mag := params.DegradedThreshold - scores.EconomicStability
		if scores.EconomicStability < params.CriticalThreshold {
			mag *= 2
		}
		corrections = append(corrections, &types.CorrectionRecord{
			Height:    height,
			Dimension: types.DimEconomicStability,
			Parameter: "staking.reward_rate",
			Direction: "increase",
			Magnitude: mag,
			Applied:   false,
			Timestamp: now,
		})
	}

	// Governance participation: log only — no automatic correction per prompt.
	// (Intentionally no correction generated for governance.)

	// Network security: suggest slashing severity increase if low.
	if scores.NetworkSecurity < params.DegradedThreshold {
		mag := params.DegradedThreshold - scores.NetworkSecurity
		if scores.NetworkSecurity < params.CriticalThreshold {
			mag *= 2
		}
		corrections = append(corrections, &types.CorrectionRecord{
			Height:    height,
			Dimension: types.DimNetworkSecurity,
			Parameter: "security.slashing_severity",
			Direction: "increase",
			Magnitude: mag,
			Applied:   false,
			Timestamp: now,
		})
	}

	// Staking ratio: suggest reward rate increase if low.
	if scores.StakingRatio < params.DegradedThreshold {
		mag := params.DegradedThreshold - scores.StakingRatio
		if scores.StakingRatio < params.CriticalThreshold {
			mag *= 2
		}
		corrections = append(corrections, &types.CorrectionRecord{
			Height:    height,
			Dimension: types.DimStakingRatio,
			Parameter: "staking.reward_rate",
			Direction: "increase",
			Magnitude: mag,
			Applied:   false,
			Timestamp: now,
		})
	}

	return corrections
}

// ApplyCorrections dispatches corrections to autopoiesis if within bounds.
// Corrections exceeding MaxAutoApplyMagnitudeBps emit a governance-required event.
// Nil-safe: if autopoiesisKeeper is nil, corrections are stored with applied=false.
func (k Keeper) ApplyCorrections(ctx context.Context, corrections []*types.CorrectionRecord) {
	params := k.GetParams(ctx)
	sdkCtx := sdk.UnwrapSDKContext(ctx)

	for _, c := range corrections {
		// Check magnitude bounds.
		if params.MaxAutoApplyMagnitudeBps > 0 && c.Magnitude > params.MaxAutoApplyMagnitudeBps {
			k.Logger(ctx).Info("correction exceeds auto-apply bounds, requires governance",
				"dimension", c.Dimension,
				"parameter", c.Parameter,
				"magnitude", c.Magnitude,
				"max_auto_apply", params.MaxAutoApplyMagnitudeBps,
			)
			sdkCtx.EventManager().EmitEvent(
				sdk.NewEvent("zerone.alignment.correction_governance_required",
					sdk.NewAttribute("dimension", c.Dimension),
					sdk.NewAttribute("parameter", c.Parameter),
					sdk.NewAttribute("direction", c.Direction),
					sdk.NewAttribute("magnitude", fmt.Sprintf("%d", c.Magnitude)),
					sdk.NewAttribute("max_auto_apply", fmt.Sprintf("%d", params.MaxAutoApplyMagnitudeBps)),
				),
			)
			c.Applied = false
			k.AddCorrection(ctx, c)
			continue
		}

		if k.autopoiesisKeeper != nil {
			err := k.autopoiesisKeeper.SuggestAdjustment(ctx, c.Parameter, c.Direction, c.Magnitude)
			if err == nil {
				c.Applied = true
			} else {
				k.Logger(ctx).Error("failed to apply correction",
					"dimension", c.Dimension,
					"parameter", c.Parameter,
					"error", err,
				)
			}
		} else {
			k.Logger(ctx).Info("correction logged (autopoiesis not wired)",
				"dimension", c.Dimension,
				"parameter", c.Parameter,
				"direction", c.Direction,
				"magnitude", c.Magnitude,
			)
		}
		k.AddCorrection(ctx, c)
	}
}
