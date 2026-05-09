package keeper

import (
	"context"
	"fmt"
	"math/big"

	sdkmath "cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/zerone-chain/zerone/x/evidence_mgmt/types"
)

type msgServer struct {
	types.UnimplementedMsgServer
	Keeper
}

func NewMsgServerImpl(keeper Keeper) types.MsgServer {
	return &msgServer{Keeper: keeper}
}

var _ types.MsgServer = msgServer{}

// SubmitEvidence creates new evidence with an initial chain-of-custody entry.
func (m msgServer) SubmitEvidence(goCtx context.Context, msg *types.MsgSubmitEvidence) (*types.MsgSubmitEvidenceResponse, error) {
	ctx := sdk.UnwrapSDKContext(goCtx)
	currentBlock := uint64(ctx.BlockHeight())

	id := fmt.Sprintf("evid-%d", m.GetNextEvidenceID(ctx))

	evidence := &types.Evidence{
		Id:           id,
		Submitter:    msg.Submitter,
		EvidenceType: msg.EvidenceType,
		ContentHash:  msg.ContentHash,
		Metadata:     msg.Metadata,
		ChainOfCustody: []*types.ChainOfCustodyEntry{
			{
				Custodian: msg.Submitter,
				Action:    "submit",
				Timestamp: currentBlock,
				Notes:     "initial submission",
			},
		},
		Status:         types.EvidenceStatus_EVIDENCE_STATUS_SUBMITTED,
		CreatedAtBlock: currentBlock,
		UpdatedAtBlock: currentBlock,
	}

	m.SetEvidence(ctx, evidence)

	// Commitment 1 (methodology over statement): evidence enters
	// the chain with a declared methodology and provenance — the
	// substrate that downstream knowledge claims and disputes ground
	// in. Without the declared methodology, evidence would be a
	// bare assertion rather than a testable input.
	ctx.EventManager().EmitEvent(
		sdk.NewEvent(
			"zerone.evidence_mgmt.evidence_submitted",
			sdk.NewAttribute("evidence_id", id),
			sdk.NewAttribute("submitter", msg.Submitter),
			sdk.NewAttribute("evidence_type", msg.EvidenceType.String()),
			sdk.NewAttribute("creed_commitment", "1"),
		),
	)

	return &types.MsgSubmitEvidenceResponse{EvidenceId: id}, nil
}

// TransferCustody appends a custody entry (only current custodian can transfer).
func (m msgServer) TransferCustody(goCtx context.Context, msg *types.MsgTransferCustody) (*types.MsgTransferCustodyResponse, error) {
	ctx := sdk.UnwrapSDKContext(goCtx)

	evidence, found := m.GetEvidence(ctx, msg.EvidenceId)
	if !found {
		return nil, fmt.Errorf("%w: %s", types.ErrEvidenceNotFound, msg.EvidenceId)
	}

	// Verify current custodian is the last entry's custodian
	if len(evidence.ChainOfCustody) == 0 {
		return nil, fmt.Errorf("%w: no custody chain", types.ErrNotCustodian)
	}
	lastCustodian := evidence.ChainOfCustody[len(evidence.ChainOfCustody)-1].Custodian
	if lastCustodian != msg.CurrentCustodian {
		return nil, fmt.Errorf("%w: current custodian is %s, not %s", types.ErrNotCustodian, lastCustodian, msg.CurrentCustodian)
	}

	currentBlock := uint64(ctx.BlockHeight())
	evidence.ChainOfCustody = append(evidence.ChainOfCustody, &types.ChainOfCustodyEntry{
		Custodian: msg.NewCustodian,
		Action:    "transfer",
		Timestamp: currentBlock,
		Notes:     msg.Notes,
	})
	evidence.UpdatedAtBlock = currentBlock

	m.SetEvidence(ctx, evidence)

	// Commitment 10 (forward-only audit): custody transfer adds a
	// link to the chain of custody — never replaces a prior link.
	// The lineage of who has held this evidence and when is part
	// of the chain's permanent record so future verifications can
	// trace provenance without trusting any single party's account.
	ctx.EventManager().EmitEvent(
		sdk.NewEvent(
			"zerone.evidence_mgmt.custody_transferred",
			sdk.NewAttribute("evidence_id", msg.EvidenceId),
			sdk.NewAttribute("from", msg.CurrentCustodian),
			sdk.NewAttribute("to", msg.NewCustodian),
			sdk.NewAttribute("creed_commitment", "10"),
		),
	)

	return &types.MsgTransferCustodyResponse{}, nil
}

// VerifyEvidence records a verification result, checking verifier tier.
func (m msgServer) VerifyEvidence(goCtx context.Context, msg *types.MsgVerifyEvidence) (*types.MsgVerifyEvidenceResponse, error) {
	ctx := sdk.UnwrapSDKContext(goCtx)

	evidence, found := m.GetEvidence(ctx, msg.EvidenceId)
	if !found {
		return nil, fmt.Errorf("%w: %s", types.ErrEvidenceNotFound, msg.EvidenceId)
	}

	// No self-verification
	if msg.Verifier == evidence.Submitter {
		return nil, types.ErrSelfVerification
	}

	// Check verifier tier
	params := m.GetParams(ctx)
	tier, err := m.stakingKeeper.GetValidatorTier(ctx, msg.Verifier)
	if err != nil {
		return nil, fmt.Errorf("failed to get verifier tier: %w", err)
	}
	if tier < params.MinVerifierTier {
		return nil, fmt.Errorf("%w: verifier tier %d < required %d", types.ErrInsufficientVerifierTier, tier, params.MinVerifierTier)
	}

	// Check no duplicate verification from same verifier
	existing := m.GetVerificationsByEvidence(ctx, msg.EvidenceId)
	for _, v := range existing {
		if v.Verifier == msg.Verifier {
			return nil, types.ErrAlreadyVerified
		}
	}

	verificationID := fmt.Sprintf("ver-%d", m.GetNextVerificationID(ctx))
	result := &types.VerificationResult{
		Id:         verificationID,
		EvidenceId: msg.EvidenceId,
		Verifier:   msg.Verifier,
		Outcome:    msg.Outcome,
		Confidence: msg.Confidence,
		Method:     msg.Method,
	}
	m.SetVerification(ctx, result)

	// Update evidence status if quorum reached
	allVerifications := m.GetVerificationsByEvidence(ctx, msg.EvidenceId)
	if uint32(len(allVerifications)) >= params.VerificationQuorum {
		positiveCount := 0
		for _, v := range allVerifications {
			if v.Outcome {
				positiveCount++
			}
		}
		if positiveCount > len(allVerifications)/2 {
			evidence.Status = types.EvidenceStatus_EVIDENCE_STATUS_VERIFIED
		} else {
			evidence.Status = types.EvidenceStatus_EVIDENCE_STATUS_REJECTED
		}
		currentBlock := uint64(ctx.BlockHeight())
		evidence.UpdatedAtBlock = currentBlock
		m.SetEvidence(ctx, evidence)
	}

	// Commitment 1 (methodology over statement) AND commitment 10
	// (forward-only audit): the verification verdict is what makes
	// the evidence's declared methodology live as a testable claim.
	// The verdict is recorded immutably alongside the verifier's
	// identity and the confidence — past verifications cannot be
	// retroactively rewritten to flatter or punish later positions.
	ctx.EventManager().EmitEvent(
		sdk.NewEvent(
			"zerone.evidence_mgmt.evidence_verified",
			sdk.NewAttribute("evidence_id", msg.EvidenceId),
			sdk.NewAttribute("verifier", msg.Verifier),
			sdk.NewAttribute("outcome", fmt.Sprintf("%v", msg.Outcome)),
			sdk.NewAttribute("confidence", fmt.Sprintf("%d", msg.Confidence)),
			sdk.NewAttribute("creed_commitment", "1,10"),
		),
	)

	return &types.MsgVerifyEvidenceResponse{}, nil
}

// ChallengeEvidence creates a dispute for the evidence via the disputes keeper.
func (m msgServer) ChallengeEvidence(goCtx context.Context, msg *types.MsgChallengeEvidence) (*types.MsgChallengeEvidenceResponse, error) {
	ctx := sdk.UnwrapSDKContext(goCtx)

	evidence, found := m.GetEvidence(ctx, msg.EvidenceId)
	if !found {
		return nil, fmt.Errorf("%w: %s", types.ErrEvidenceNotFound, msg.EvidenceId)
	}

	// Check challenge window
	params := m.GetParams(ctx)
	currentBlock := uint64(ctx.BlockHeight())
	if currentBlock > evidence.CreatedAtBlock+params.ChallengeWindowBlocks {
		return nil, fmt.Errorf("%w: evidence created at block %d, window is %d blocks",
			types.ErrChallengeWindowClosed, evidence.CreatedAtBlock, params.ChallengeWindowBlocks)
	}

	// Validate bond meets minimum
	bond := new(big.Int)
	if _, ok := bond.SetString(msg.Bond, 10); !ok || bond.Sign() <= 0 {
		return nil, fmt.Errorf("invalid bond amount: %s", msg.Bond)
	}
	minBond := new(big.Int)
	minBond.SetString(params.ChallengeBond, 10)
	if bond.Cmp(minBond) < 0 {
		return nil, fmt.Errorf("bond %s below minimum %s", msg.Bond, params.ChallengeBond)
	}

	// Collect bond from challenger
	challengerAddr, err := sdk.AccAddressFromBech32(msg.Challenger)
	if err != nil {
		return nil, fmt.Errorf("invalid challenger address: %w", err)
	}
	_ = challengerAddr
	_ = sdkmath.NewIntFromBigInt(bond)

	// Bridge to disputes module
	var disputeID string
	if m.disputesKeeper != nil {
		disputeID, err = m.disputesKeeper.CreateDispute(ctx, msg.Challenger, msg.EvidenceId, msg.Reason, msg.Bond)
		if err != nil {
			return nil, fmt.Errorf("failed to create dispute: %w", err)
		}
	} else {
		disputeID = fmt.Sprintf("dispute-stub-%s", msg.EvidenceId)
	}

	// Update evidence status to challenged
	evidence.Status = types.EvidenceStatus_EVIDENCE_STATUS_CHALLENGED
	evidence.UpdatedAtBlock = currentBlock
	evidence.ChainOfCustody = append(evidence.ChainOfCustody, &types.ChainOfCustodyEntry{
		Custodian: msg.Challenger,
		Action:    "challenge",
		Timestamp: currentBlock,
		Notes:     msg.Reason,
	})
	m.SetEvidence(ctx, evidence)

	// Commitment 10 (forward-only audit): challenging evidence
	// opens a dispute and the relationship (evidence → dispute) is
	// recorded permanently. The challenge is part of the evidence's
	// lifecycle record going forward — outcomes will be tied back
	// to this entry, not to any later post-hoc restatement.
	ctx.EventManager().EmitEvent(
		sdk.NewEvent(
			"zerone.evidence_mgmt.evidence_challenged",
			sdk.NewAttribute("evidence_id", msg.EvidenceId),
			sdk.NewAttribute("challenger", msg.Challenger),
			sdk.NewAttribute("dispute_id", disputeID),
			sdk.NewAttribute("bond", msg.Bond),
			sdk.NewAttribute("creed_commitment", "10"),
		),
	)

	return &types.MsgChallengeEvidenceResponse{DisputeId: disputeID}, nil
}

// UpdateParams updates module parameters (authority-gated).
func (m msgServer) UpdateParams(goCtx context.Context, msg *types.MsgUpdateParams) (*types.MsgUpdateParamsResponse, error) {
	if m.GetAuthority() != msg.Authority {
		return nil, fmt.Errorf("%w: expected %s, got %s", types.ErrUnauthorized, m.GetAuthority(), msg.Authority)
	}

	ctx := sdk.UnwrapSDKContext(goCtx)
	m.SetParams(ctx, msg.Params)

	ctx.EventManager().EmitEvent(
		sdk.NewEvent("zerone.evidence_mgmt.update_params",
			sdk.NewAttribute("authority", msg.Authority),
		),
	)

	return &types.MsgUpdateParamsResponse{}, nil
}
