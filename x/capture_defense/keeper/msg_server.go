package keeper

import (
	"context"
	"fmt"

	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/zerone-chain/zerone/x/capture_defense/types"
)

type msgServer struct {
	types.UnimplementedMsgServer
	Keeper
}

// NewMsgServerImpl returns an implementation of the MsgServer interface.
func NewMsgServerImpl(keeper Keeper) types.MsgServer {
	return &msgServer{Keeper: keeper}
}

var _ types.MsgServer = msgServer{}

// RecordVerification records the results of a verification round and updates reputations.
func (k msgServer) RecordVerification(goCtx context.Context, msg *types.MsgRecordVerification) (*types.MsgRecordVerificationResponse, error) {
	ctx := sdk.UnwrapSDKContext(goCtx)

	// Validate authority
	if msg.Authority != k.GetAuthority() {
		return nil, fmt.Errorf("%w: expected %s, got %s", types.ErrInvalidValidator, k.GetAuthority(), msg.Authority)
	}

	// Validate array lengths
	if len(msg.Validators) != len(msg.Verdicts) {
		return nil, fmt.Errorf("%w: validators(%d) != verdicts(%d)", types.ErrMismatchedArrayLengths, len(msg.Validators), len(msg.Verdicts))
	}
	if len(msg.SubmitBlocks) > 0 && len(msg.SubmitBlocks) != len(msg.Validators) {
		return nil, fmt.Errorf("%w: submit_blocks(%d) != validators(%d)", types.ErrMismatchedArrayLengths, len(msg.SubmitBlocks), len(msg.Validators))
	}

	if msg.Domain == "" {
		return nil, fmt.Errorf("%w: domain is required", types.ErrInvalidDomain)
	}
	if msg.RoundId == "" {
		return nil, fmt.Errorf("round_id is required")
	}
	if len(msg.Validators) == 0 {
		return nil, fmt.Errorf("%w: at least one validator is required", types.ErrInvalidValidator)
	}

	// Record history entry
	entry := &types.VerificationHistoryEntry{
		Domain:       msg.Domain,
		RoundId:      msg.RoundId,
		Validators:   msg.Validators,
		Verdicts:     msg.Verdicts,
		SubmitBlocks: msg.SubmitBlocks,
		BlockHeight:  uint64(ctx.BlockHeight()),
	}
	k.SetVerificationHistory(ctx, entry)

	// Update reputations for each validator
	for i, validator := range msg.Validators {
		approved := msg.Verdicts[i]
		k.UpdateReputation(ctx, validator, msg.Domain, "", approved)
	}

	// Commitment 9 (cartel detection has consequence): the
	// verification record is the upstream substrate that capture
	// detection reads. Without these recorded observations, cartel
	// allegations have no evidence base to consume.
	ctx.EventManager().EmitEvent(
		sdk.NewEvent(
			"zerone.capture_defense.verification_recorded",
			sdk.NewAttribute("domain", msg.Domain),
			sdk.NewAttribute("round_id", msg.RoundId),
			sdk.NewAttribute("validator_count", fmt.Sprintf("%d", len(msg.Validators))),
			sdk.NewAttribute("block_height", fmt.Sprintf("%d", ctx.BlockHeight())),
			sdk.NewAttribute("creed_commitment", "9"),
		),
	)

	return &types.MsgRecordVerificationResponse{}, nil
}

// AnalyzeDomain triggers a capture risk analysis for a specific domain.
func (k msgServer) AnalyzeDomain(goCtx context.Context, msg *types.MsgAnalyzeDomain) (*types.MsgAnalyzeDomainResponse, error) {
	ctx := sdk.UnwrapSDKContext(goCtx)

	if msg.Domain == "" {
		return nil, fmt.Errorf("%w: domain is required", types.ErrInvalidDomain)
	}

	params := k.GetParams(ctx)
	metrics := k.AnalyzeCaptureRisk(ctx, msg.Domain, params)

	if metrics == nil {
		return &types.MsgAnalyzeDomainResponse{}, nil
	}

	// Commitment 9 (cartel detection has consequence): the analysis
	// produces the flagged-domain signal that x/qualification reads
	// to weight panel votes and that x/capture_challenge consumes
	// when scoring allegations. The event makes the analysis result
	// queryable in the same vocabulary as the consequence side.
	ctx.EventManager().EmitEvent(
		sdk.NewEvent(
			"zerone.capture_defense.domain_analyzed",
			sdk.NewAttribute("domain", msg.Domain),
			sdk.NewAttribute("risk_score", fmt.Sprintf("%d", metrics.RiskScore)),
			sdk.NewAttribute("hhi", fmt.Sprintf("%d", metrics.HerfindahlIndex)),
			sdk.NewAttribute("flagged", fmt.Sprintf("%t", metrics.Flagged)),
			sdk.NewAttribute("creed_commitment", "9"),
		),
	)

	return &types.MsgAnalyzeDomainResponse{
		RiskScore: metrics.RiskScore,
		Flagged:   metrics.Flagged,
	}, nil
}

// UpdateParams handles MsgUpdateParams — governance-gated parameter update.
func (k msgServer) UpdateParams(goCtx context.Context, msg *types.MsgUpdateParams) (*types.MsgUpdateParamsResponse, error) {
	if k.GetAuthority() != msg.Authority {
		return nil, fmt.Errorf("unauthorized: expected %s, got %s", k.GetAuthority(), msg.Authority)
	}

	ctx := sdk.UnwrapSDKContext(goCtx)
	if msg.Params == nil {
		return nil, fmt.Errorf("params cannot be nil")
	}
	if err := msg.Params.Validate(); err != nil {
		return nil, fmt.Errorf("invalid params: %w", err)
	}
	k.SetParams(ctx, msg.Params)

	ctx.EventManager().EmitEvent(
		sdk.NewEvent(
			"zerone.capture_defense.params_updated",
			sdk.NewAttribute("authority", msg.Authority),
		),
	)

	return &types.MsgUpdateParamsResponse{}, nil
}
