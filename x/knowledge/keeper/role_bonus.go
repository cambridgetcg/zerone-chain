package keeper

import (
	"context"

	"github.com/zerone-chain/zerone/x/knowledge/types"
)

// ApplyRoleBonusToConfidence applies the claim-type × account-type bonus.
// Returns the boosted confidence (NOT clamped — caller must clamp).
func ApplyRoleBonusToConfidence(confidence uint64, claimType types.ClaimType, accountType string, params *types.Params) uint64 {
	var bonusBps uint64

	switch {
	case claimType == types.ClaimType_CLAIM_TYPE_OBSERVATION && accountType == "human":
		bonusBps = params.HumanEmpiricalBonusBps
	case claimType == types.ClaimType_CLAIM_TYPE_COMPUTATIONAL && accountType == "agent":
		bonusBps = params.AgentComputationalBonusBps
	}

	if bonusBps == 0 {
		return confidence
	}

	return safeMulDiv(confidence, 1_000_000+bonusBps, 1_000_000)
}

// ApplyDualValidationBonus applies the partnership dual-validation bonus.
// Returns the boosted confidence (NOT clamped — caller must clamp).
func ApplyDualValidationBonus(confidence uint64, params *types.Params) uint64 {
	if params.DualValidationBonusBps == 0 {
		return confidence
	}
	return safeMulDiv(confidence, 1_000_000+params.DualValidationBonusBps, 1_000_000)
}

// getAccountType safely looks up account type via ZeroneAuthKeeper.
// Returns "" if keeper is nil or account not found.
func (k Keeper) getAccountType(ctx context.Context, address string) string {
	if k.zeroneAuthKeeper == nil {
		return ""
	}
	accountType, found := k.zeroneAuthKeeper.GetAccountType(ctx, address)
	if !found {
		return ""
	}
	return accountType
}
