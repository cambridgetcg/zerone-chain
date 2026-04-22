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
// Uses dynamic effective max magnitude based on correction confidence (R29-4).
// Nil-safe: if autopoiesisKeeper is nil, corrections are stored with applied=false.
func (k Keeper) ApplyCorrections(ctx context.Context, corrections []*types.CorrectionRecord) {
	params := k.GetParams(ctx)
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	height := uint64(sdkCtx.BlockHeight())

	// Get current scores for outcome tracking (R29-4).
	currentScores, _ := k.GetScores(ctx, height)

	// Use dynamic effective max magnitude based on correction confidence (R29-4).
	effectiveMax := k.GetEffectiveMaxMagnitude(ctx)

	for _, c := range corrections {
		// Record pre-correction outcome for tracking (R29-4).
		if currentScores != nil {
			outcome := &types.CorrectionOutcome{
				Height:      height,
				Dimension:   c.Dimension,
				Magnitude:   c.Magnitude,
				Direction:   c.Direction,
				ScoreBefore: types.GetDimensionScore(currentScores, c.Dimension),
			}
			k.SetCorrectionOutcome(ctx, outcome)
		}

		// Advisory band (L7): below params.AdvisoryMagnitudeBps, don't forward
		// to autopoiesis — emit an advisory event and record with applied=false.
		// Keeps small deviations from chattering the regulatory layer.
		if params.AdvisoryMagnitudeBps > 0 && c.Magnitude < params.AdvisoryMagnitudeBps {
			sdkCtx.EventManager().EmitEvent(
				sdk.NewEvent("zerone.alignment.correction_advisory",
					sdk.NewAttribute("dimension", c.Dimension),
					sdk.NewAttribute("parameter", c.Parameter),
					sdk.NewAttribute("direction", c.Direction),
					sdk.NewAttribute("magnitude", fmt.Sprintf("%d", c.Magnitude)),
					sdk.NewAttribute("advisory_threshold", fmt.Sprintf("%d", params.AdvisoryMagnitudeBps)),
				),
			)
			c.Applied = false
			k.AddCorrection(ctx, c)
			continue
		}

		// Check magnitude bounds (dynamic via correction confidence).
		if effectiveMax > 0 && c.Magnitude > effectiveMax {
			k.Logger(ctx).Info("correction exceeds auto-apply bounds, requires governance",
				"dimension", c.Dimension,
				"parameter", c.Parameter,
				"magnitude", c.Magnitude,
				"effective_max", effectiveMax,
			)
			sdkCtx.EventManager().EmitEvent(
				sdk.NewEvent("zerone.alignment.correction_governance_required",
					sdk.NewAttribute("dimension", c.Dimension),
					sdk.NewAttribute("parameter", c.Parameter),
					sdk.NewAttribute("direction", c.Direction),
					sdk.NewAttribute("magnitude", fmt.Sprintf("%d", c.Magnitude)),
					sdk.NewAttribute("effective_max", fmt.Sprintf("%d", effectiveMax)),
				),
			)
			c.Applied = false
			k.AddCorrection(ctx, c)
			continue
		} else if effectiveMax == 0 && params.MaxAutoApplyMagnitudeBps > 0 {
			// Confidence too low — all corrections require governance (R29-4).
			k.Logger(ctx).Info("correction confidence too low, all corrections require governance",
				"dimension", c.Dimension,
				"parameter", c.Parameter,
				"magnitude", c.Magnitude,
			)
			sdkCtx.EventManager().EmitEvent(
				sdk.NewEvent("zerone.alignment.correction_governance_required",
					sdk.NewAttribute("dimension", c.Dimension),
					sdk.NewAttribute("parameter", c.Parameter),
					sdk.NewAttribute("direction", c.Direction),
					sdk.NewAttribute("magnitude", fmt.Sprintf("%d", c.Magnitude)),
					sdk.NewAttribute("effective_max", "0"),
					sdk.NewAttribute("reason", "low_confidence"),
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
