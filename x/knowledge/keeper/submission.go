package keeper

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"strconv"

	sdkmath "cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/zerone-chain/zerone/x/knowledge/types"
)

// ComputeContentHash returns the lowercase hex SHA-256 of content.
func (k Keeper) ComputeContentHash(content string) string {
	h := sha256.Sum256([]byte(content))
	return hex.EncodeToString(h[:])
}

// CheckDuplicate returns ErrDuplicateContent if the hash already exists in state.
func (k Keeper) CheckDuplicate(ctx context.Context, contentHash string) error {
	if k.HasContentHash(ctx, contentHash) {
		return types.ErrDuplicateContent
	}
	return nil
}

// ValidateConsent checks that the consent proof is present and has the required fields.
func (k Keeper) ValidateConsent(consent *types.ConsentProof) error {
	if consent == nil {
		return types.ErrConsentRequired
	}
	switch consent.Type {
	case types.ConsentType_CONSENT_TYPE_SELF_AUTHORED:
		return nil
	case types.ConsentType_CONSENT_TYPE_OPT_IN:
		if consent.AuthorSignature == "" && consent.ProofUri == "" {
			return types.ErrInvalidConsent
		}
	case types.ConsentType_CONSENT_TYPE_PUBLIC_LICENSE:
		if consent.ProofUri == "" {
			return types.ErrInvalidConsent
		}
	case types.ConsentType_CONSENT_TYPE_PLATFORM_TOS:
		if consent.ProofUri == "" {
			return types.ErrInvalidConsent
		}
	case types.ConsentType_CONSENT_TYPE_FAIR_USE:
		return nil
	default:
		return types.ErrInvalidConsent
	}
	return nil
}

// SubmitData handles MsgSubmitData — validates, deduplicates, stores, and indexes a submission.
func (k Keeper) SubmitData(ctx context.Context, msg *types.MsgSubmitData) (*types.MsgSubmitDataResponse, error) {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	params, err := k.GetParams(ctx)
	if err != nil {
		return nil, err
	}

	// 1. Validate content size
	if uint64(len(msg.Content)) > params.MaxContentBytes {
		return nil, types.ErrContentTooLarge.Wrapf("content %d bytes exceeds max %d", len(msg.Content), params.MaxContentBytes)
	}

	// 2. Compute content hash
	contentHash := k.ComputeContentHash(msg.Content)

	// 3. Check duplicates
	if err := k.CheckDuplicate(ctx, contentHash); err != nil {
		return nil, err
	}

	// 4. Validate consent
	if err := k.ValidateConsent(msg.Consent); err != nil {
		return nil, err
	}

	// 5. Validate domain exists and is active
	domain, found := k.GetDomain(ctx, msg.Domain)
	if !found {
		return nil, types.ErrDomainNotFound.Wrapf("domain %q not found", msg.Domain)
	}
	if domain.Status != types.DomainStatus_DOMAIN_STATUS_ACTIVE {
		return nil, types.ErrInvalidDomain.Wrapf("domain %q is not active", msg.Domain)
	}

	// 6. Validate and lock stake
	stakeAmt, ok := sdkmath.NewIntFromString(msg.Stake)
	if !ok || stakeAmt.IsNegative() {
		return nil, types.ErrInsufficientStake.Wrap("invalid stake amount")
	}
	minStake, _ := sdkmath.NewIntFromString(params.MinSubmissionStake)
	if stakeAmt.LT(minStake) {
		return nil, types.ErrInsufficientStake.Wrapf("stake %s < minimum %s", msg.Stake, params.MinSubmissionStake)
	}

	// Handle sponsored vs self-funded
	if msg.Sponsored {
		stakeCoin := sdk.NewCoin("uzrn", stakeAmt)
		if err := k.bankKeeper.SendCoinsFromModuleToModule(
			sdkCtx, types.BootstrapFundModuleName, types.ModuleName, sdk.NewCoins(stakeCoin),
		); err != nil {
			return nil, types.ErrInsufficientStake.Wrap("bootstrap fund insufficient")
		}
	} else {
		submitterAddr, _ := sdk.AccAddressFromBech32(msg.Submitter)
		stakeCoin := sdk.NewCoin("uzrn", stakeAmt)
		if err := k.bankKeeper.SendCoinsFromAccountToModule(
			sdkCtx, submitterAddr, types.ModuleName, sdk.NewCoins(stakeCoin),
		); err != nil {
			return nil, types.ErrInsufficientStake.Wrap(err.Error())
		}
	}

	// 7. Create Submission
	submissionID := k.NextSubmissionID(ctx)
	submission := &types.Submission{
		Id:                 submissionID,
		Submitter:          msg.Submitter,
		Content:            msg.Content,
		SampleType:         msg.SampleType,
		Domain:             msg.Domain,
		SourceUri:          msg.SourceUri,
		SourcePlatform:     msg.SourcePlatform,
		SourceTimestamp:    msg.SourceTimestamp,
		ParentSubmissionId: msg.ParentSubmissionId,
		ContextIds:         msg.ContextIds,
		ThreadId:           msg.ThreadId,
		Consent:            msg.Consent,
		OriginalAuthor:     msg.OriginalAuthor,
		License:            msg.License,
		Tags:               msg.Tags,
		Language:           msg.Language,
		Stake:              msg.Stake,
		SubmittedAtBlock:   uint64(sdkCtx.BlockHeight()),
		Status:             types.SubmissionStatus_SUBMISSION_STATUS_PENDING,
		ContentHash:        contentHash,
		Sponsored:          msg.Sponsored,
	}

	// 8. Store + indexes
	if err := k.SetSubmission(ctx, submission); err != nil {
		return nil, err
	}
	if err := k.SetContentHash(ctx, contentHash, submissionID); err != nil {
		return nil, err
	}
	if err := k.SetSubmissionDomainIndex(ctx, msg.Domain, submissionID); err != nil {
		return nil, err
	}
	if err := k.SetSubmissionSubmitterIndex(ctx, msg.Submitter, submissionID); err != nil {
		return nil, err
	}

	// 9-10. TODO(R37-2): Check DataBounty matches + Initiate quality round

	// 11. Emit event
	sdkCtx.EventManager().EmitEvent(sdk.NewEvent(
		"submit_data",
		sdk.NewAttribute("submission_id", submissionID),
		sdk.NewAttribute("submitter", msg.Submitter),
		sdk.NewAttribute("domain", msg.Domain),
		sdk.NewAttribute("content_hash", contentHash),
		sdk.NewAttribute("sponsored", strconv.FormatBool(msg.Sponsored)),
	))

	return &types.MsgSubmitDataResponse{SubmissionId: submissionID}, nil
}
