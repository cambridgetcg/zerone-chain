package keeper

import (
	"context"
	"strconv"

	sdkmath "cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/zerone-chain/zerone/x/knowledge/types"
)

// SponsorSample handles patronage payments that preserve a sample from pruning.
func (k Keeper) SponsorSample(ctx context.Context, msg *types.MsgSponsorSample) (*types.MsgSponsorSampleResponse, error) {
	sdkCtx := sdk.UnwrapSDKContext(ctx)

	// 1. Verify sample exists
	sample, found := k.GetSample(ctx, msg.SampleId)
	if !found {
		return nil, types.ErrSampleNotFound.Wrapf("sample %q not found", msg.SampleId)
	}

	// 2. Validate amount
	amount, ok := sdkmath.NewIntFromString(msg.Amount)
	if !ok || !amount.IsPositive() {
		return nil, types.ErrInsufficientPayment.Wrap("amount must be positive")
	}

	// 3. Validate duration
	if msg.DurationBlocks == 0 {
		return nil, types.ErrInvalidChallenge.Wrap("duration_blocks must be > 0")
	}

	// 4. Transfer amount from sponsor to module account
	sponsorAddr, _ := sdk.AccAddressFromBech32(msg.Sponsor)
	coin := sdk.NewCoin("uzrn", amount)
	if err := k.bankKeeper.SendCoinsFromAccountToModule(
		sdkCtx, sponsorAddr, types.ModuleName, sdk.NewCoins(coin),
	); err != nil {
		return nil, types.ErrInsufficientPayment.Wrap(err.Error())
	}

	// 5. Update patronage amount (accumulate)
	existingAmount, ok2 := sdkmath.NewIntFromString(sample.PatronageAmount)
	if !ok2 {
		existingAmount = sdkmath.ZeroInt()
	}
	newTotal := existingAmount.Add(amount)
	sample.PatronageAmount = newTotal.String()

	// 6. Set/extend patronage expiry
	block := uint64(sdkCtx.BlockHeight())
	newExpiry := block + msg.DurationBlocks
	if sample.PatronageExpiryBlock > block {
		// Extend from existing expiry
		newExpiry = sample.PatronageExpiryBlock + msg.DurationBlocks
	}
	sample.PatronageExpiryBlock = newExpiry

	// 7. Restore energy to cap
	if sample.EnergyCap > 0 {
		sample.Energy = sample.EnergyCap
	}
	sample.EnergyLastUpdated = block

	// 8. Save
	if err := k.SetSample(ctx, sample); err != nil {
		return nil, err
	}

	// 9. Emit event
	sdkCtx.EventManager().EmitEvent(sdk.NewEvent(
		"sample_sponsored",
		sdk.NewAttribute("sample_id", msg.SampleId),
		sdk.NewAttribute("sponsor", msg.Sponsor),
		sdk.NewAttribute("amount", msg.Amount),
		sdk.NewAttribute("duration_blocks", strconv.FormatUint(msg.DurationBlocks, 10)),
		sdk.NewAttribute("patronage_total", newTotal.String()),
		sdk.NewAttribute("expiry_block", strconv.FormatUint(newExpiry, 10)),
	))

	return &types.MsgSponsorSampleResponse{}, nil
}
