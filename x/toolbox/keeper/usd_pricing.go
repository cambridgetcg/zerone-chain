package keeper

import (
	"context"
	"math/big"
	"strconv"

	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/zerone-chain/zerone/x/toolbox/types"
)

// Default price bound constants (uzrn).
const (
	DefaultMinPricePerCall = uint64(1_000)
	DefaultMaxPricePerCall = uint64(100_000_000)
)

// Pricing mode strings.
const (
	PricingModeFixedUzrn = "fixed_uzrn"
	PricingModeUsdStable = "usd_stable"
)

// GetBasePrice returns the base price in uzrn for a tool call and the pricing mode.
// If the tool has a target_price_usd and the billing keeper is available,
// it computes the ZRN-equivalent. Otherwise, uses the fixed price_per_call.
func (k Keeper) GetBasePrice(ctx context.Context, tool *types.Tool) (uint64, string) {
	fixedPrice := parseUint64(tool.PricePerCall)

	// If no USD target, use fixed ZRN price.
	targetUSD := parseUint64(tool.TargetPriceUsd)
	if targetUSD == 0 {
		return fixedPrice, PricingModeFixedUzrn
	}

	// Attempt oracle price.
	if k.billingKeeper == nil {
		emitOracleUnavailableEvent(ctx, tool.Id, "billing_keeper_nil")
		return fixedPrice, PricingModeFixedUzrn
	}

	zrnPriceUSD, err := k.billingKeeper.GetZRNPriceUSD(ctx)
	if err != nil || zrnPriceUSD == 0 {
		reason := "price_zero"
		if err != nil {
			reason = "oracle_error"
		}
		emitOracleUnavailableEvent(ctx, tool.Id, reason)
		return fixedPrice, PricingModeFixedUzrn // Oracle unavailable, fallback to fixed.
	}

	// Compute: base_uzrn = (targetUSD * 1e6) / zrnPriceUSD
	// Both targetUSD and zrnPriceUSD are in micro-USD (6 decimal).
	numerator := new(big.Int).Mul(
		new(big.Int).SetUint64(targetUSD),
		new(big.Int).SetUint64(1_000_000),
	)
	base := new(big.Int).Div(numerator, new(big.Int).SetUint64(zrnPriceUSD))

	// Clamp to [min, max].
	minPrice := parseUintOrDefault(tool.MinPricePerCall, DefaultMinPricePerCall)
	maxPrice := parseUintOrDefault(tool.MaxPricePerCall, DefaultMaxPricePerCall)

	var result uint64
	if base.IsUint64() {
		result = base.Uint64()
	} else {
		result = ^uint64(0)
	}

	if minPrice > 0 && result < minPrice {
		result = minPrice
	}
	if maxPrice > 0 && result > maxPrice {
		result = maxPrice
	}

	return result, PricingModeUsdStable
}

// CalculateEffectivePrice returns the final price in uzrn after surge and the pricing mode.
func (k Keeper) CalculateEffectivePrice(ctx context.Context, tool *types.Tool) (uint64, string) {
	basePrice, mode := k.GetBasePrice(ctx, tool)
	if basePrice == 0 {
		return 0, mode
	}

	params := k.GetParams(ctx)
	if !params.SurgeEnabled {
		return basePrice, mode
	}

	surgeMultiplier := k.CalculateSurgeMultiplier(ctx, tool)
	effectivePrice := ApplySurge(basePrice, surgeMultiplier)

	// Emit surge event when surge > 1.0×.
	if surgeMultiplier > types.BpsDenominator {
		_, toolUtil := k.GetToolDemand(ctx, tool.Id)
		tier := PricingTier(tool.Category)
		emitSurgeEvent(ctx, tool.Id, basePrice, effectivePrice, surgeMultiplier, toolUtil, tier)
	}

	return effectivePrice, mode
}

// QueryToolPrice returns a full pricing breakdown for a tool.
func (k Keeper) QueryToolPrice(ctx context.Context, toolID string) (*types.QueryToolPriceResponse, error) {
	tool, ok := k.GetTool(ctx, toolID)
	if !ok {
		return nil, types.ErrToolNotFound.Wrapf("tool %s not found", toolID)
	}

	basePrice, mode := k.GetBasePrice(ctx, tool)
	surgeMultiplier := k.CalculateSurgeMultiplier(ctx, tool)
	effectivePrice := ApplySurge(basePrice, surgeMultiplier)

	var zrnPriceUSD uint64
	if k.billingKeeper != nil {
		zrnPriceUSD, _ = k.billingKeeper.GetZRNPriceUSD(ctx)
	}

	return &types.QueryToolPriceResponse{
		BasePriceUzrn:      basePrice,
		SurgeMultiplierBps: surgeMultiplier,
		EffectivePriceUzrn: effectivePrice,
		ZrnPriceUsd:        zrnPriceUSD,
		PricingMode:        mode,
	}, nil
}

// validateUSDPricingFields checks USD pricing field consistency.
func validateUSDPricingFields(targetPriceUsd, minPricePerCall, maxPricePerCall string) error {
	target := parseUint64(targetPriceUsd)
	if target == 0 {
		return nil // Not using USD pricing.
	}
	minP := parseUint64(minPricePerCall)
	maxP := parseUint64(maxPricePerCall)
	if minP > 0 && maxP > 0 && minP > maxP {
		return types.ErrInvalidParams.Wrapf("min_price_per_call (%d) > max_price_per_call (%d)", minP, maxP)
	}
	return nil
}

// enforcePriceCollar rejects any PricePerCall or MaxPricePerCall value that
// exceeds the governance-set absolute ceiling (L5). Protects users from a
// tool deployer monopoly-pricing its domain.
func enforcePriceCollar(params *types.Params, pricePerCall, maxPricePerCall string) error {
	ceilingStr := params.AbsoluteMaxPricePerCallUzrn
	if ceilingStr == "" || ceilingStr == "0" {
		return nil // collar disabled
	}
	ceiling := parseUint64(ceilingStr)
	if ceiling == 0 {
		return nil
	}
	if p := parseUint64(pricePerCall); p > ceiling {
		return types.ErrInvalidParams.Wrapf(
			"price_per_call %d exceeds absolute ceiling %d", p, ceiling)
	}
	if m := parseUint64(maxPricePerCall); m > ceiling {
		return types.ErrInvalidParams.Wrapf(
			"max_price_per_call %d exceeds absolute ceiling %d", m, ceiling)
	}
	return nil
}

// parseUintOrDefault parses a string as uint64, returning defaultVal if empty or invalid.
func parseUintOrDefault(s string, defaultVal uint64) uint64 {
	if s == "" {
		return defaultVal
	}
	v, err := strconv.ParseUint(s, 10, 64)
	if err != nil {
		return defaultVal
	}
	return v
}

// emitOracleUnavailableEvent emits a tool_pricing_oracle_unavailable event.
func emitOracleUnavailableEvent(ctx context.Context, toolID, reason string) {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	sdkCtx.EventManager().EmitEvent(
		sdk.NewEvent(
			"zerone.toolbox.pricing_oracle_unavailable",
			sdk.NewAttribute("tool_id", toolID),
			sdk.NewAttribute("reason", reason),
		),
	)
}
