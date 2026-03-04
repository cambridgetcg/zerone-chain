package keeper

import (
	"context"
	"fmt"

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
