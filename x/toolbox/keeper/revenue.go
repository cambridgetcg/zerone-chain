package keeper

import (
	"context"
	"fmt"
	"math/big"

	sdkmath "cosmossdk.io/math"

	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/zerone-chain/zerone/x/toolbox/types"
)

// Revenue routing module names.
const (
	CitationPool      = "knowledge"
	VerificationPool  = "vesting_rewards"
	ProtocolTreasury  = "protocol_treasury"
	ToolboxModuleName = types.ModuleName
	denom             = "uzrn"
)

// uzrnCoin creates a sdk.Coin with the given uzrn amount.
func uzrnCoin(amount uint64) sdk.Coin {
	return sdk.NewCoin(denom, sdkmath.NewInt(int64(amount)))
}

// DistributeRevenue splits collected fees according to governance params.
// Split: ToolRevenueBps → contributors, ProtocolBps → protocol sub-split,
//
//	ResearchBps → research fund, DevelopmentBps → development fund.
func (k Keeper) DistributeRevenue(ctx context.Context, tool *types.Tool, totalAmount sdk.Coin) error {
	if totalAmount.IsZero() {
		return nil
	}
	params := k.GetParams(ctx)
	total := totalAmount.Amount.Uint64()

	// 1. Contributor share.
	contributorAmount := safeMulDiv(total, params.ToolRevenueBps, types.BpsDenominator)
	if contributorAmount > 0 {
		if err := k.distributeContributorShares(ctx, tool, uzrnCoin(contributorAmount)); err != nil {
			return fmt.Errorf("contributor distribution: %w", err)
		}
	}

	// 2. Protocol sub-split.
	protocolAmount := safeMulDiv(total, params.ProtocolBps, types.BpsDenominator)
	if protocolAmount > 0 {
		// Citation pool.
		citationAmt := safeMulDiv(protocolAmount, params.ProtocolCitationBps, types.BpsDenominator)
		if citationAmt > 0 {
			if err := k.bankKeeper.SendCoinsFromModuleToModule(ctx, ToolboxModuleName, CitationPool, sdk.NewCoins(uzrnCoin(citationAmt))); err != nil {
				return fmt.Errorf("citation pool: %w", err)
			}
		}
		// Verification pool.
		verificationAmt := safeMulDiv(protocolAmount, params.ProtocolVerificationBps, types.BpsDenominator)
		if verificationAmt > 0 {
			if err := k.bankKeeper.SendCoinsFromModuleToModule(ctx, ToolboxModuleName, VerificationPool, sdk.NewCoins(uzrnCoin(verificationAmt))); err != nil {
				return fmt.Errorf("verification pool: %w", err)
			}
		}
		// Protocol treasury (remainder).
		treasuryAmt := protocolAmount - citationAmt - verificationAmt
		if treasuryAmt > 0 {
			if err := k.bankKeeper.SendCoinsFromModuleToModule(ctx, ToolboxModuleName, ProtocolTreasury, sdk.NewCoins(uzrnCoin(treasuryAmt))); err != nil {
				return fmt.Errorf("protocol treasury: %w", err)
			}
		}
	}

	// 3. Research fund.
	researchAmount := safeMulDiv(total, params.ResearchBps, types.BpsDenominator)
	if researchAmount > 0 {
		if err := k.researchFund.DepositToResearchFund(ctx, ToolboxModuleName, sdk.NewCoins(uzrnCoin(researchAmount))); err != nil {
			return fmt.Errorf("research fund: %w", err)
		}
	}

	// 4. Development fund.
	devAmount := safeMulDiv(total, params.DevelopmentBps, types.BpsDenominator)
	if devAmount > 0 {
		if err := k.bankKeeper.SendCoinsFromModuleToModule(ctx, ToolboxModuleName, "development_fund", sdk.NewCoins(uzrnCoin(devAmount))); err != nil {
			return fmt.Errorf("development fund: %w", err)
		}
	}

	return nil
}

// distributeContributorShares distributes revenue pro-rata among accepted contributors.
// Remainder goes to the deployer.
func (k Keeper) distributeContributorShares(ctx context.Context, tool *types.Tool, amount sdk.Coin) error {
	if len(tool.Contributors) == 0 {
		// No contributors — send everything to deployer.
		deployerAddr, err := sdk.AccAddressFromBech32(tool.Deployer)
		if err != nil {
			return err
		}
		return k.bankKeeper.SendCoinsFromModuleToAccount(ctx, ToolboxModuleName, deployerAddr, sdk.NewCoins(amount))
	}

	total := amount.Amount.Uint64()
	var distributed uint64

	for _, contrib := range tool.Contributors {
		if !contrib.Accepted {
			continue
		}
		share := safeMulDiv(total, contrib.ShareBps, types.BpsDenominator)
		if share == 0 {
			continue
		}
		addr, err := sdk.AccAddressFromBech32(contrib.Address)
		if err != nil {
			continue
		}
		if err := k.bankKeeper.SendCoinsFromModuleToAccount(ctx, ToolboxModuleName, addr, sdk.NewCoins(uzrnCoin(share))); err != nil {
			return err
		}

		// Track earnings.
		earned := new(big.Int)
		earned.SetString(contrib.TotalEarned, 10)
		earned.Add(earned, new(big.Int).SetUint64(share))
		contrib.TotalEarned = earned.String()

		distributed += share
	}

	// Remainder to deployer.
	remainder := total - distributed
	if remainder > 0 {
		deployerAddr, err := sdk.AccAddressFromBech32(tool.Deployer)
		if err != nil {
			return err
		}
		if err := k.bankKeeper.SendCoinsFromModuleToAccount(ctx, ToolboxModuleName, deployerAddr, sdk.NewCoins(uzrnCoin(remainder))); err != nil {
			return err
		}
	}

	return nil
}

// CollectPayment collects payment from a caller for a tool call.
// Checks free tier first, then slippage, then transfers to the module account.
func (k Keeper) CollectPayment(ctx context.Context, caller string, tool *types.Tool, maxFee uint64) (uint64, bool, error) {
	// Try free tier first.
	if k.TryConsumeFreeCall(ctx, caller, tool) {
		return 0, true, nil
	}

	// Calculate effective price.
	effectivePrice, _ := k.CalculateEffectivePrice(ctx, tool)
	if effectivePrice == 0 {
		return 0, false, nil
	}

	// Slippage check.
	if maxFee > 0 && effectivePrice > maxFee {
		return 0, false, types.ErrFeeTooHigh.Wrapf("effective price %d exceeds max_fee %d", effectivePrice, maxFee)
	}

	// Transfer from caller to module.
	callerAddr, err := sdk.AccAddressFromBech32(caller)
	if err != nil {
		return 0, false, err
	}
	if err := k.bankKeeper.SendCoinsFromAccountToModule(ctx, callerAddr, ToolboxModuleName, sdk.NewCoins(uzrnCoin(effectivePrice))); err != nil {
		return 0, false, types.ErrInsufficientBalance.Wrapf("failed to collect payment: %v", err)
	}

	return effectivePrice, false, nil
}
