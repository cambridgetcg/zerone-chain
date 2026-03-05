package keeper

import (
	"context"
	"fmt"
	"math/big"
	"strconv"

	sdkmath "cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/zerone-chain/zerone/x/billing/types"
)

// ── Escrow Deposits ──────────────────────────────────────────────────────────

// DepositEscrow receives ZRN from a user into the module escrow account.
func (k Keeper) DepositEscrow(ctx context.Context, userAddr string, amountUZRN int64) error {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	addr, err := sdk.AccAddressFromBech32(userAddr)
	if err != nil {
		return fmt.Errorf("invalid address: %w", err)
	}

	coins := sdk.NewCoins(sdk.NewInt64Coin("uzrn", amountUZRN))
	if err := k.bankKeeper.SendCoinsFromAccountToModule(sdkCtx, addr, types.ModuleName, coins); err != nil {
		return fmt.Errorf("deposit escrow: %w", err)
	}

	// Update escrow balance in KV
	bal := k.GetEscrowBalance(ctx, userAddr)
	bal += amountUZRN
	k.setEscrowBalance(ctx, userAddr, bal)

	sdkCtx.EventManager().EmitEvent(sdk.NewEvent(
		"zerone.billing.escrow_deposited",
		sdk.NewAttribute("user", userAddr),
		sdk.NewAttribute("amount", strconv.FormatInt(amountUZRN, 10)),
		sdk.NewAttribute("new_balance", strconv.FormatInt(bal, 10)),
	))

	return nil
}

// GetEscrowBalance returns a user's escrow balance.
func (k Keeper) GetEscrowBalance(ctx context.Context, userAddr string) int64 {
	kvStore := k.storeService.OpenKVStore(ctx)
	bz, err := kvStore.Get(types.EscrowKey(userAddr))
	if err != nil || bz == nil {
		return 0
	}
	val, _ := strconv.ParseInt(string(bz), 10, 64)
	return val
}

func (k Keeper) setEscrowBalance(ctx context.Context, userAddr string, balance int64) {
	kvStore := k.storeService.OpenKVStore(ctx)
	_ = kvStore.Set(types.EscrowKey(userAddr), []byte(strconv.FormatInt(balance, 10)))
}

// ── Batch Settlement ─────────────────────────────────────────────────────────

// SettleUsage deducts from a user's escrow and distributes revenue.
func (k Keeper) SettleUsage(ctx context.Context, userAddr string, tokensConsumed int64, costUZRN int64, idempotencyKey string) error {
	sdkCtx := sdk.UnwrapSDKContext(ctx)

	// Idempotency check
	if idempotencyKey != "" {
		kvStore := k.storeService.OpenKVStore(ctx)
		existing, _ := kvStore.Get(types.SettlementKey(idempotencyKey))
		if existing != nil {
			return nil // already settled
		}
		_ = kvStore.Set(types.SettlementKey(idempotencyKey), []byte("1"))
	}

	bal := k.GetEscrowBalance(ctx, userAddr)
	if bal < costUZRN {
		return fmt.Errorf("insufficient escrow: have %d, need %d", bal, costUZRN)
	}

	// Deduct from escrow
	k.setEscrowBalance(ctx, userAddr, bal-costUZRN)

	// Distribute revenue from module account
	costBig := new(big.Int).SetInt64(costUZRN)
	distribution := k.CalculateDistribution(ctx, costBig, nil)

	// Send from module to revenue recipients
	k.distributeSettlementRevenue(sdkCtx, distribution, costUZRN)

	sdkCtx.EventManager().EmitEvent(sdk.NewEvent(
		"zerone.billing.usage_settled",
		sdk.NewAttribute("user", userAddr),
		sdk.NewAttribute("tokens_consumed", strconv.FormatInt(tokensConsumed, 10)),
		sdk.NewAttribute("cost_uzrn", strconv.FormatInt(costUZRN, 10)),
	))

	return nil
}

func (k Keeper) distributeSettlementRevenue(ctx sdk.Context, dist *types.PaymentDistribution, totalUZRN int64) {
	// Protocol treasury share
	treasuryAmt := new(big.Int)
	treasuryAmt.SetString(dist.ProtocolTreasury, 10)
	if treasuryAmt.Sign() > 0 && treasuryAmt.IsInt64() {
		coins := sdk.NewCoins(sdk.NewCoin("uzrn", sdkmath.NewIntFromBigInt(treasuryAmt)))
		if err := k.bankKeeper.SendCoinsFromModuleToModule(ctx, types.ModuleName, "fee_collector", coins); err != nil {
			k.Logger(ctx).Error("settle: treasury share transfer failed", "error", err)
		}
	}

	// Research fund share
	researchAmt := new(big.Int)
	researchAmt.SetString(dist.ResearchShare, 10)
	if researchAmt.Sign() > 0 && researchAmt.IsInt64() {
		coins := sdk.NewCoins(sdk.NewCoin("uzrn", sdkmath.NewIntFromBigInt(researchAmt)))
		if err := k.researchFundDepositor.DepositToResearchFund(ctx, types.ModuleName, coins); err != nil {
			k.Logger(ctx).Error("settle: research fund deposit failed", "error", err)
		}
	}
}

// ── Withdrawal ───────────────────────────────────────────────────────────────

// WithdrawEscrow returns unused escrow balance to the user.
func (k Keeper) WithdrawEscrow(ctx context.Context, userAddr string, amountUZRN int64) error {
	sdkCtx := sdk.UnwrapSDKContext(ctx)

	bal := k.GetEscrowBalance(ctx, userAddr)
	if bal < amountUZRN {
		return fmt.Errorf("insufficient escrow: have %d, requested %d", bal, amountUZRN)
	}

	addr, err := sdk.AccAddressFromBech32(userAddr)
	if err != nil {
		return fmt.Errorf("invalid address: %w", err)
	}

	coins := sdk.NewCoins(sdk.NewInt64Coin("uzrn", amountUZRN))
	if err := k.bankKeeper.SendCoinsFromModuleToAccount(sdkCtx, types.ModuleName, addr, coins); err != nil {
		return fmt.Errorf("withdraw escrow: %w", err)
	}

	k.setEscrowBalance(ctx, userAddr, bal-amountUZRN)

	sdkCtx.EventManager().EmitEvent(sdk.NewEvent(
		"zerone.billing.escrow_withdrawn",
		sdk.NewAttribute("user", userAddr),
		sdk.NewAttribute("amount", strconv.FormatInt(amountUZRN, 10)),
		sdk.NewAttribute("remaining", strconv.FormatInt(bal-amountUZRN, 10)),
	))

	return nil
}
