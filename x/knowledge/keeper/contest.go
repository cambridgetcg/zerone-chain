package keeper

import (
	"context"
	"strconv"

	sdkmath "cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/zerone-chain/zerone/x/knowledge/types"
)

// ContestSample handles a dispute against a validated sample.
func (k Keeper) ContestSample(ctx context.Context, msg *types.MsgContestSample) (*types.MsgContestSampleResponse, error) {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	params, err := k.GetParams(ctx)
	if err != nil {
		return nil, err
	}

	// 1. Verify sample exists
	sample, found := k.GetSample(ctx, msg.SampleId)
	if !found {
		return nil, types.ErrSampleNotFound.Wrapf("sample %q not found", msg.SampleId)
	}

	// 2. Verify sample is active (gold/silver/bronze)
	if !isActiveSampleStatus(sample.Status) {
		return nil, types.ErrInvalidChallenge.Wrapf("sample status %s is not contestable", sample.Status.String())
	}

	// 3. Check not already contested
	if _, contested := k.GetContestRound(ctx, msg.SampleId); contested {
		return nil, types.ErrDuplicateChallenge.Wrap("sample is already under contest")
	}

	// 4. Cannot contest own sample
	if msg.Challenger == sample.Submitter {
		return nil, types.ErrSelfChallenge
	}

	// 5. Validate stake amount
	stakeAmt, ok := sdkmath.NewIntFromString(msg.Stake)
	if !ok || !stakeAmt.IsPositive() {
		return nil, types.ErrInsufficientStake.Wrap("invalid stake amount")
	}

	minStake, _ := sdkmath.NewIntFromString(params.MinSubmissionStake)
	// Consent contests have lower stake requirement (half)
	if msg.ContestType == types.ContestType_CONTEST_TYPE_CONSENT {
		minStake = minStake.Quo(sdkmath.NewInt(2))
		if minStake.IsZero() {
			minStake = sdkmath.OneInt()
		}
	}
	if stakeAmt.LT(minStake) {
		return nil, types.ErrInsufficientStake.Wrapf("stake %s < minimum %s", msg.Stake, minStake.String())
	}

	// 6. Lock challenger's stake
	challengerAddr, _ := sdk.AccAddressFromBech32(msg.Challenger)
	stakeCoin := sdk.NewCoin("uzrn", stakeAmt)
	if err := k.bankKeeper.SendCoinsFromAccountToModule(
		sdkCtx, challengerAddr, types.ModuleName, sdk.NewCoins(stakeCoin),
	); err != nil {
		return nil, types.ErrInsufficientStake.Wrap(err.Error())
	}

	// 7. Create a re-validation submission to anchor the quality round
	subID := k.NextSubmissionID(ctx)
	sub := &types.Submission{
		Id:              subID,
		Submitter:       msg.Challenger,
		Content:         sample.Content,
		SampleType:      sample.SampleType,
		Domain:          sample.Domain,
		SourceUri:       sample.SourceUri,
		SourcePlatform:  sample.SourcePlatform,
		SourceTimestamp:  sample.SourceTimestamp,
		Consent:         sample.Consent,
		OriginalAuthor:  sample.OriginalAuthor,
		License:         sample.License,
		Tags:            sample.Tags,
		Language:        sample.Language,
		Stake:           msg.Stake,
		SubmittedAtBlock: uint64(sdkCtx.BlockHeight()),
		Status:          types.SubmissionStatus_SUBMISSION_STATUS_PENDING_REVIEW,
		ContentHash:     k.ComputeContentHash(sample.Content),
	}
	if err := k.SetSubmission(ctx, sub); err != nil {
		return nil, err
	}

	// 8. Create re-validation round
	roundID, err := k.InitiateQualityRound(ctx, subID, "", []string{})
	if err != nil {
		return nil, err
	}

	// 9. Mark sample as CONTESTED
	prevStatus := sample.Status
	sample.Status = types.SampleStatus_SAMPLE_STATUS_CONTESTED
	if err := k.SetSample(ctx, sample); err != nil {
		return nil, err
	}
	if err := k.SetContestIndex(ctx, msg.SampleId, roundID); err != nil {
		return nil, err
	}

	// 10. Emit event
	sdkCtx.EventManager().EmitEvent(sdk.NewEvent(
		"sample_contested",
		sdk.NewAttribute("sample_id", msg.SampleId),
		sdk.NewAttribute("challenger", msg.Challenger),
		sdk.NewAttribute("contest_type", msg.ContestType.String()),
		sdk.NewAttribute("round_id", roundID),
		sdk.NewAttribute("stake", msg.Stake),
		sdk.NewAttribute("previous_status", prevStatus.String()),
		sdk.NewAttribute("reason", msg.Reason),
		sdk.NewAttribute("block", strconv.FormatInt(sdkCtx.BlockHeight(), 10)),
	))

	return &types.MsgContestSampleResponse{RoundId: roundID}, nil
}

// isActiveSampleStatus returns true if the sample is in a contestable state.
func isActiveSampleStatus(s types.SampleStatus) bool {
	switch s {
	case types.SampleStatus_SAMPLE_STATUS_GOLD,
		types.SampleStatus_SAMPLE_STATUS_SILVER,
		types.SampleStatus_SAMPLE_STATUS_BRONZE:
		return true
	default:
		return false
	}
}
