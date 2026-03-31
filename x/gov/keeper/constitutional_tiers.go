package keeper

import (
	"encoding/json"
	"fmt"
	"math/big"

	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/zerone-chain/zerone/x/gov/types"
)

// ConstitutionalTierConfig holds the support thresholds for each tier.
// Stored as JSON-serialized supplementary config (avoids proto codegen blocker).
type ConstitutionalTierConfig struct {
	StandardSupportBps      uint64 `json:"standard_support_bps"`       // 60% default
	ElevatedSupportBps      uint64 `json:"elevated_support_bps"`       // 75% default
	ConstitutionalSupportBps uint64 `json:"constitutional_support_bps"` // 90% default
}

// DefaultConstitutionalTierConfig returns the default tier thresholds.
func DefaultConstitutionalTierConfig() ConstitutionalTierConfig {
	return ConstitutionalTierConfig{
		StandardSupportBps:      types.DefaultStandardSupportBps,
		ElevatedSupportBps:      types.DefaultElevatedSupportBps,
		ConstitutionalSupportBps: types.DefaultConstitutionalSupportBps,
	}
}

// GetConstitutionalTierConfig returns the stored tier config, or defaults if unset.
func (k Keeper) GetConstitutionalTierConfig(ctx sdk.Context) ConstitutionalTierConfig {
	store := ctx.KVStore(k.storeKey)
	bz := store.Get(types.ConstitutionalTierConfigKey)
	if bz == nil {
		return DefaultConstitutionalTierConfig()
	}
	var cfg ConstitutionalTierConfig
	if err := json.Unmarshal(bz, &cfg); err != nil {
		return DefaultConstitutionalTierConfig()
	}
	return cfg
}

// SetConstitutionalTierConfig stores the tier config.
func (k Keeper) SetConstitutionalTierConfig(ctx sdk.Context, cfg ConstitutionalTierConfig) {
	store := ctx.KVStore(k.storeKey)
	bz, err := json.Marshal(cfg)
	if err != nil {
		panic("failed to marshal constitutional tier config: " + err.Error())
	}
	store.Set(types.ConstitutionalTierConfigKey, bz)
}

// GetSupportThresholdForCategory returns the support threshold (BPS) for a LIP category.
// Resolves category → tier → threshold. Falls back to constitutional (90%) for unknown categories.
func (k Keeper) GetSupportThresholdForCategory(ctx sdk.Context, category string) (tier string, thresholdBps uint64) {
	tier = types.GetTierForCategory(category)
	cfg := k.GetConstitutionalTierConfig(ctx)

	switch tier {
	case types.TierStandard:
		thresholdBps = cfg.StandardSupportBps
	case types.TierElevated:
		thresholdBps = cfg.ElevatedSupportBps
	case types.TierConstitutional:
		thresholdBps = cfg.ConstitutionalSupportBps
	default:
		thresholdBps = cfg.ConstitutionalSupportBps // fail-safe
	}
	return tier, thresholdBps
}

// checkQuorumAndTieredSupport checks quorum and the tier-appropriate support threshold.
// This replaces both checkQuorumAndSupport and checkQuorumAndSupermajority with a
// unified tier-aware system.
func (k Keeper) checkQuorumAndTieredSupport(ctx sdk.Context, lip *types.LIP, params *types.Params) (quorumMet bool, passed bool, tier string, thresholdBps uint64) {
	tier, thresholdBps = k.GetSupportThresholdForCategory(ctx, lip.Category)

	yesBig, _ := new(big.Int).SetString(lip.YesStake, 10)
	if yesBig == nil {
		yesBig = big.NewInt(0)
	}
	noBig, _ := new(big.Int).SetString(lip.NoStake, 10)
	if noBig == nil {
		noBig = big.NewInt(0)
	}
	abstainBig, _ := new(big.Int).SetString(lip.AbstainStake, 10)
	if abstainBig == nil {
		abstainBig = big.NewInt(0)
	}

	totalVoted := new(big.Int).Add(yesBig, noBig)
	totalVoted.Add(totalVoted, abstainBig)

	// Get total bonded stake.
	totalBonded := big.NewInt(0)
	if k.stakingKeeper != nil {
		bondedStr, err := k.stakingKeeper.GetTotalBondedStake(ctx)
		if err == nil {
			if tb, ok := new(big.Int).SetString(bondedStr, 10); ok {
				totalBonded = tb
			}
		}
	}

	// Quorum check: (totalVoted * 1_000_000) / totalBonded >= quorumThresholdBps
	if totalBonded.Sign() > 0 {
		actualBps := new(big.Int).Mul(totalVoted, big.NewInt(int64(types.BPSScale)))
		actualBps.Div(actualBps, totalBonded)
		quorumMet = actualBps.Uint64() >= params.QuorumThresholdBps
	}

	// Tiered support check: (yesStake * 1_000_000) / (yesStake + noStake) >= tierThresholdBps
	yesNoTotal := new(big.Int).Add(yesBig, noBig)
	if yesNoTotal.Sign() > 0 {
		supportBps := new(big.Int).Mul(yesBig, big.NewInt(int64(types.BPSScale)))
		supportBps.Div(supportBps, yesNoTotal)
		passed = quorumMet && supportBps.Uint64() >= thresholdBps
	}

	// Emit transparency event: which tier and threshold was applied.
	ctx.EventManager().EmitEvent(
		sdk.NewEvent("zerone.gov.tier_check",
			sdk.NewAttribute("lip_id", lip.Id),
			sdk.NewAttribute("category", lip.Category),
			sdk.NewAttribute("tier", tier),
			sdk.NewAttribute("support_threshold_bps", fmt.Sprintf("%d", thresholdBps)),
			sdk.NewAttribute("quorum_met", fmt.Sprintf("%t", quorumMet)),
			sdk.NewAttribute("passed", fmt.Sprintf("%t", passed)),
		),
	)

	return quorumMet, passed, tier, thresholdBps
}
