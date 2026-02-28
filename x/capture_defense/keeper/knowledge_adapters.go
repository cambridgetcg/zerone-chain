package keeper

import (
	"context"

	sdk "github.com/cosmos/cosmos-sdk/types"

	knowledgetypes "github.com/zerone-chain/zerone/x/knowledge/types"
)

// KnowledgeCaptureDefenseAdapter wraps the capture_defense Keeper to satisfy
// the knowledge module's CaptureDefenseKeeper interface.
type KnowledgeCaptureDefenseAdapter struct {
	k Keeper
}

// NewKnowledgeCaptureDefenseAdapter creates a new adapter.
func NewKnowledgeCaptureDefenseAdapter(k Keeper) *KnowledgeCaptureDefenseAdapter {
	return &KnowledgeCaptureDefenseAdapter{k: k}
}

// Ensure compile-time interface compliance.
var _ knowledgetypes.CaptureDefenseKeeper = (*KnowledgeCaptureDefenseAdapter)(nil)

// RecordVerificationHistory records a verification round's results in capture defense history.
func (a *KnowledgeCaptureDefenseAdapter) RecordVerificationHistory(
	goCtx context.Context,
	domain, roundId string,
	validators []string,
	verdicts []bool,
	submitBlocks []uint64,
) {
	ctx := sdk.UnwrapSDKContext(goCtx)
	a.k.RecordVerificationFromKnowledge(ctx, domain, roundId, validators, verdicts, submitBlocks)
}

// UpdateReputation updates a validator's reputation in the capture defense system.
func (a *KnowledgeCaptureDefenseAdapter) UpdateReputation(
	goCtx context.Context,
	validator string,
	domain string,
	stratum string,
	approved bool,
) {
	ctx := sdk.UnwrapSDKContext(goCtx)
	a.k.UpdateReputation(ctx, validator, domain, stratum, approved)
}

// GetDomainCapturePenalty implements knowledgetypes.CaptureDefenseKeeper (R31-1).
func (a *KnowledgeCaptureDefenseAdapter) GetDomainCapturePenalty(
	goCtx context.Context,
	domain string,
) (bool, uint64) {
	ctx := sdk.UnwrapSDKContext(goCtx)
	return a.k.GetDomainCapturePenalty(ctx, domain)
}
