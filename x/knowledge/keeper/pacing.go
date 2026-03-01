package keeper

import (
	"context"
	"encoding/binary"
	"math/big"

	"github.com/zerone-chain/zerone/x/knowledge/types"
)

const pacingBPS = uint64(1_000_000)

// GetEffectiveCooldown returns the claim cooldown adjusted by global health pacing
// and domain carrying capacity pressure.
//
// When the network is healthy (creationBps == 1,000,000), cooldown equals the base param.
// When degraded (creationBps < 1,000,000), cooldown increases proportionally — fewer claims
// are accepted per unit time, matching reduced network throughput.
// Domain pressure can only tighten (increase) the cooldown further.
func (k Keeper) GetEffectiveCooldown(ctx context.Context, domain string) uint64 {
	params, err := k.GetParams(ctx)
	if err != nil {
		return 50 // safe fallback
	}
	baseCooldown := params.ClaimCooldownBlocks

	effectiveCooldown := baseCooldown
	if k.pacingKeeper != nil {
		creationPacing, _ := k.pacingKeeper.GetGlobalPacingMultiplier(ctx)
		if creationPacing > 0 && creationPacing != pacingBPS {
			// Inverse relationship: lower pacing → higher cooldown.
			// e.g. creationPacing = 750_000 (75%) → cooldown = base * 1_000_000 / 750_000 ≈ 133%
			effectiveCooldown = safeMulDiv(baseCooldown, pacingBPS, creationPacing)
		}
	}

	// Domain carrying capacity override: can only tighten (increase) cooldown.
	pressure := k.GetDomainPressure(ctx, domain)
	if pressure > pacingBPS {
		domainCooldown := safeMulDiv(effectiveCooldown, pressure, pacingBPS)
		if domainCooldown > effectiveCooldown {
			effectiveCooldown = domainCooldown
		}
	}

	return effectiveCooldown
}

// GetEffectiveMinReviewFee returns the minimum review fee adjusted by pacing.
//
// When the network is degraded (creationBps < 1,000,000), the fee increases to
// discourage low-value submissions during constrained periods.
func (k Keeper) GetEffectiveMinReviewFee(ctx context.Context) string {
	params, err := k.GetParams(ctx)
	if err != nil {
		return "100000" // safe fallback
	}
	baseStr := params.MinReviewFee
	baseFee, ok := new(big.Int).SetString(baseStr, 10)
	if !ok || baseFee.Sign() <= 0 {
		return baseStr
	}

	if k.pacingKeeper != nil {
		creationPacing, _ := k.pacingKeeper.GetGlobalPacingMultiplier(ctx)
		if creationPacing > 0 && creationPacing < pacingBPS {
			// Inverse relationship: lower pacing → higher fee.
			// e.g. creationPacing = 750_000 → fee = base * 1_000_000 / 750_000 ≈ 133%
			adjusted := new(big.Int).Mul(baseFee, new(big.Int).SetUint64(pacingBPS))
			adjusted.Div(adjusted, new(big.Int).SetUint64(creationPacing))
			return adjusted.String()
		}
	}

	return baseStr
}

// GetLastClaimHeight returns the block height at which a submitter last submitted a claim.
// Returns 0 if the submitter has never submitted.
func (k Keeper) GetLastClaimHeight(ctx context.Context, submitter string) uint64 {
	kvStore := k.storeService.OpenKVStore(ctx)
	key := types.LastClaimHeightKey(submitter)
	bz, err := kvStore.Get(key)
	if err != nil || len(bz) != 8 {
		return 0
	}
	return binary.BigEndian.Uint64(bz)
}

// SetLastClaimHeight records the block height of a submitter's latest claim.
func (k Keeper) SetLastClaimHeight(ctx context.Context, submitter string, height uint64) {
	kvStore := k.storeService.OpenKVStore(ctx)
	key := types.LastClaimHeightKey(submitter)
	bz := make([]byte, 8)
	binary.BigEndian.PutUint64(bz, height)
	_ = kvStore.Set(key, bz)
}
