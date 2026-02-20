package keeper

import (
	"fmt"

	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/zerone-chain/zerone/x/auth/types"
)

// RegisterInvariants registers all auth module invariants.
func RegisterInvariants(ir sdk.InvariantRegistry, k Keeper) {
	ir.RegisterRoute(types.ModuleName, "account-did-parity", AccountDIDParityInvariant(k))
	ir.RegisterRoute(types.ModuleName, "session-count-consistency", SessionCountConsistencyInvariant(k))
	ir.RegisterRoute(types.ModuleName, "params-valid", ParamsValidInvariant(k))
}

// AccountDIDParityInvariant checks that every account has a corresponding DID mapping
// and every DID mapping points to an existing account.
func AccountDIDParityInvariant(k Keeper) sdk.Invariant {
	return func(ctx sdk.Context) (string, bool) {
		var msg string
		broken := false

		k.IterateAccounts(ctx, func(account *types.Account) bool {
			if account.Did == "" {
				return false
			}
			mapping, found := k.GetDIDMapping(ctx, account.Did)
			if !found {
				msg += fmt.Sprintf("account %s has DID %s but no DID mapping exists\n", account.Address, account.Did)
				broken = true
			} else if mapping.Bech32 != account.Address {
				msg += fmt.Sprintf("account %s DID mapping points to %s instead\n", account.Address, mapping.Bech32)
				broken = true
			}
			return false
		})

		return sdk.FormatInvariant(types.ModuleName, "account-did-parity", msg), broken
	}
}

// SessionCountConsistencyInvariant checks that each account's SessionKeyCount
// matches the actual number of stored (non-expired) session keys.
func SessionCountConsistencyInvariant(k Keeper) sdk.Invariant {
	return func(ctx sdk.Context) (string, bool) {
		var msg string
		broken := false
		currentHeight := uint64(ctx.BlockHeight())

		k.IterateAccounts(ctx, func(account *types.Account) bool {
			sessions := k.GetSessionKeysForOwner(ctx, account.Address)
			activeCount := uint32(0)
			for _, s := range sessions {
				if s.ExpiresAtBlock > currentHeight {
					activeCount++
				}
			}
			if account.SessionKeyCount != activeCount {
				msg += fmt.Sprintf("account %s: SessionKeyCount=%d but actual active sessions=%d\n",
					account.Address, account.SessionKeyCount, activeCount)
				broken = true
			}
			return false
		})

		return sdk.FormatInvariant(types.ModuleName, "session-count-consistency", msg), broken
	}
}

// ParamsValidInvariant checks that stored params pass validation.
func ParamsValidInvariant(k Keeper) sdk.Invariant {
	return func(ctx sdk.Context) (string, bool) {
		params := k.GetParams(ctx)
		if err := params.Validate(); err != nil {
			msg := fmt.Sprintf("stored params are invalid: %v\n", err)
			return sdk.FormatInvariant(types.ModuleName, "params-valid", msg), true
		}
		return sdk.FormatInvariant(types.ModuleName, "params-valid", ""), false
	}
}
