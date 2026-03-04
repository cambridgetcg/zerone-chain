package keeper

import (
	"context"
	"fmt"

	sdkmath "cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/zerone-chain/zerone/x/knowledge/types"
)

const (
	// DefaultBountyExpiryBlocks is the default number of blocks before an auto-bounty expires.
	DefaultBountyExpiryBlocks = 100_000
)

// ReportDemand processes training demand reports from authorized reporters.
// Only the module authority (governance) can report demand.
func (k Keeper) ReportDemand(ctx context.Context, msg *types.MsgReportDemand) (*types.MsgReportDemandResponse, error) {
	sdkCtx := sdk.UnwrapSDKContext(ctx)

	// Only authority can report demand
	if msg.Reporter != k.authority {
		return nil, types.ErrUnauthorized.Wrap("only governance authority can report demand")
	}

	params, err := k.GetParams(ctx)
	if err != nil {
		return nil, err
	}

	for _, report := range msg.Reports {
		// Verify domain exists
		if _, found := k.GetDomain(ctx, report.Domain); !found {
			return nil, types.ErrDomainNotFound.Wrapf("domain %q not found", report.Domain)
		}

		// Upsert training demand
		demand, found := k.GetTrainingDemand(ctx, report.Domain, report.Subject)
		if !found {
			demand = &types.TrainingDemand{
				Domain:  report.Domain,
				Subject: report.Subject,
			}
		}

		demand.QueryCount += report.Queries
		demand.FulfilledCount += report.Fulfilled
		demand.UnfulfilledCount += report.Unfulfilled
		demand.LastQueryBlock = uint64(sdkCtx.BlockHeight())
		demand.EpochQueryCount += report.Queries
		demand.EpochUnfulfilled += report.Unfulfilled

		if err := k.SetTrainingDemand(ctx, demand); err != nil {
			return nil, err
		}

		// Check if auto-bounty threshold reached
		k.checkAndCreateAutoBounty(ctx, demand, params, sdkCtx)
	}

	sdkCtx.EventManager().EmitEvent(sdk.NewEvent(
		"report_demand",
		sdk.NewAttribute("reporter", msg.Reporter),
		sdk.NewAttribute("report_count", fmt.Sprintf("%d", len(msg.Reports))),
	))

	return &types.MsgReportDemandResponse{}, nil
}

// checkAndCreateAutoBounty creates an auto-bounty if unfulfilled demand exceeds threshold.
func (k Keeper) checkAndCreateAutoBounty(ctx context.Context, demand *types.TrainingDemand, params *types.Params, sdkCtx sdk.Context) {
	if params.AutoBountyThreshold == 0 || params.AutoBountyAmount == "" || params.AutoBountyAmount == "0" {
		return
	}

	if demand.UnfulfilledCount < uint64(params.AutoBountyThreshold) {
		return
	}

	// Check if a bounty already exists for this domain/subject
	bounties := k.GetActiveBounties(ctx, demand.Domain)
	for _, b := range bounties {
		if b.Subject == demand.Subject {
			return // bounty already exists
		}
	}

	bountyID := k.NextBountyID(ctx)
	bounty := &types.DataBounty{
		Id:             bountyID,
		Domain:         demand.Domain,
		Subject:        demand.Subject,
		RewardAmount:   params.AutoBountyAmount,
		CreatedAtBlock: uint64(sdkCtx.BlockHeight()),
		ExpiresAtBlock: uint64(sdkCtx.BlockHeight()) + DefaultBountyExpiryBlocks,
		DemandCount:    demand.UnfulfilledCount,
	}

	if err := k.SetDataBounty(ctx, bounty); err != nil {
		return
	}
	_ = k.SetBountyDomainIndex(ctx, demand.Domain, bountyID)

	sdkCtx.EventManager().EmitEvent(sdk.NewEvent(
		"auto_bounty_created",
		sdk.NewAttribute("bounty_id", bountyID),
		sdk.NewAttribute("domain", demand.Domain),
		sdk.NewAttribute("subject", demand.Subject),
		sdk.NewAttribute("reward", params.AutoBountyAmount),
	))
}

// FundBounty creates or adds to a data bounty. Transfers funds from funder to module account.
func (k Keeper) FundBounty(ctx context.Context, msg *types.MsgFundBounty) (*types.MsgFundBountyResponse, error) {
	sdkCtx := sdk.UnwrapSDKContext(ctx)

	// Verify domain exists
	if _, found := k.GetDomain(ctx, msg.Domain); !found {
		return nil, types.ErrDomainNotFound.Wrapf("domain %q not found", msg.Domain)
	}

	// Parse amount
	amount, ok := sdkmath.NewIntFromString(msg.Amount)
	if !ok || !amount.IsPositive() {
		return nil, types.ErrInsufficientPayment.Wrap("invalid amount")
	}

	// Transfer from funder to module
	funderAddr, err := sdk.AccAddressFromBech32(msg.Funder)
	if err != nil {
		return nil, err
	}
	coins := sdk.NewCoins(sdk.NewCoin("uzrn", amount))
	if err := k.bankKeeper.SendCoinsFromAccountToModule(ctx, funderAddr, types.ModuleName, coins); err != nil {
		return nil, types.ErrInsufficientPayment.Wrap(err.Error())
	}

	// Create bounty
	bountyID := k.NextBountyID(ctx)
	expiryBlocks := msg.ExpiresBlocks
	if expiryBlocks == 0 {
		expiryBlocks = DefaultBountyExpiryBlocks
	}

	bounty := &types.DataBounty{
		Id:             bountyID,
		Domain:         msg.Domain,
		Subject:        msg.Topic,
		RewardAmount:   msg.Amount,
		CreatedAtBlock: uint64(sdkCtx.BlockHeight()),
		ExpiresAtBlock: uint64(sdkCtx.BlockHeight()) + expiryBlocks,
	}

	if err := k.SetDataBounty(ctx, bounty); err != nil {
		return nil, err
	}
	_ = k.SetBountyDomainIndex(ctx, msg.Domain, bountyID)

	sdkCtx.EventManager().EmitEvent(sdk.NewEvent(
		"fund_bounty",
		sdk.NewAttribute("bounty_id", bountyID),
		sdk.NewAttribute("funder", msg.Funder),
		sdk.NewAttribute("domain", msg.Domain),
		sdk.NewAttribute("amount", msg.Amount),
	))

	return &types.MsgFundBountyResponse{BountyId: bountyID}, nil
}

// CheckBountyFulfillment checks if a newly created sample fulfills any active bounties.
// Called after a sample is promoted from submission.
func (k Keeper) CheckBountyFulfillment(ctx context.Context, sample *types.Sample) {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	bounties := k.GetActiveBounties(ctx, sample.Domain)

	for _, bounty := range bounties {
		if !matchesBounty(sample, bounty) {
			continue
		}

		// Check not expired
		if uint64(sdkCtx.BlockHeight()) > bounty.ExpiresAtBlock {
			continue
		}

		// Mark bounty as claimed
		bounty.Claimed = true
		bounty.ClaimedBySampleId = sample.Id
		_ = k.SetDataBounty(ctx, bounty)

		// Transfer reward to submitter
		amount, ok := sdkmath.NewIntFromString(bounty.RewardAmount)
		if ok && amount.IsPositive() {
			submitterAddr, err := sdk.AccAddressFromBech32(sample.Submitter)
			if err == nil {
				coins := sdk.NewCoins(sdk.NewCoin("uzrn", amount))
				_ = k.bankKeeper.SendCoinsFromModuleToAccount(ctx, types.ModuleName, submitterAddr, coins)
			}
		}

		sdkCtx.EventManager().EmitEvent(sdk.NewEvent(
			"bounty_fulfilled",
			sdk.NewAttribute("bounty_id", bounty.Id),
			sdk.NewAttribute("sample_id", sample.Id),
			sdk.NewAttribute("submitter", sample.Submitter),
			sdk.NewAttribute("reward", bounty.RewardAmount),
		))

		break // Only claim one bounty per sample
	}
}

// matchesBounty checks if a sample matches a bounty's requirements.
func matchesBounty(sample *types.Sample, bounty *types.DataBounty) bool {
	if sample.Domain != bounty.Domain {
		return false
	}
	// If bounty has a specific subject, check for topic match
	if bounty.Subject != "" {
		found := false
		for _, topic := range sample.Topics {
			if topic == bounty.Subject {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}
