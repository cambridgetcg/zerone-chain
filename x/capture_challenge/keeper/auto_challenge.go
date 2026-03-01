package keeper

import (
	"context"
	"fmt"

	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/zerone-chain/zerone/x/capture_challenge/types"
)

// AutoSubmitChallenge creates a protocol-initiated challenge when capture defense
// flags a domain. Uses the module account as challenger — no stake escrow needed
// for auto-challenges.
func (k Keeper) AutoSubmitChallenge(ctx context.Context, domain string, riskScore, hhi uint64, evidence string) error {
	sdkCtx := sdk.UnwrapSDKContext(ctx)

	// Check if there's already an active challenge for this domain
	existingIDs := k.GetChallengesByDomain(sdkCtx, domain)
	for _, id := range existingIDs {
		ch, found := k.GetChallenge(sdkCtx, id)
		if !found {
			continue
		}
		if ch.Status == types.ChallengeStatus_CHALLENGE_STATUS_EVIDENCE ||
			ch.Status == types.ChallengeStatus_CHALLENGE_STATUS_OPEN ||
			ch.Status == types.ChallengeStatus_CHALLENGE_STATUS_UNDER_REVIEW {
			return nil // Already has an active challenge
		}
	}

	// Use module account as auto-challenger
	moduleAddr := authtypes.NewModuleAddress(types.ModuleName).String()

	challengeID := GenerateChallengeID(moduleAddr, domain, sdkCtx.BlockHeight())

	params := k.GetParams(sdkCtx)
	height := uint64(sdkCtx.BlockHeight())

	challenge := &types.CaptureChallenge{
		Id:                challengeID,
		Challenger:        moduleAddr,
		Domain:            domain,
		AccusedValidators: []string{},
		Status:            types.ChallengeStatus_CHALLENGE_STATUS_UNDER_REVIEW,
		Stake:             "0",
		EvidenceDeadline:  height,
		ReviewDeadline:    height + params.ReviewPeriodBlocks,
		CreatedBlock:      height,
		Evidence: []*types.CaptureEvidence{
			{
				Description:    evidence,
				DataHash:       fmt.Sprintf("auto:%s:%d", domain, height),
				SubmittedBlock: height,
			},
		},
	}
	k.SetChallenge(sdkCtx, challenge)

	// Maintain domain index for lookups
	k.SetDomainIndex(sdkCtx, domain, challengeID)

	sdkCtx.EventManager().EmitEvent(
		sdk.NewEvent(
			"zerone.capture_challenge.auto_challenge_submitted",
			sdk.NewAttribute("challenge_id", challengeID),
			sdk.NewAttribute("domain", domain),
			sdk.NewAttribute("risk_score", fmt.Sprintf("%d", riskScore)),
			sdk.NewAttribute("hhi", fmt.Sprintf("%d", hhi)),
		),
	)

	return nil
}
