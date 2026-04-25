package keeper

import (
	"context"
	"fmt"

	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/zerone-chain/zerone/x/qualification/types"
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

func (k msgServer) QualifyByStake(goCtx context.Context, msg *types.MsgQualifyByStake) (*types.MsgQualifyByStakeResponse, error) {
	if err := k.Keeper.QualifyByStake(goCtx, msg.Validator, msg.Domain, msg.StakeAmount); err != nil {
		return nil, err
	}
	return &types.MsgQualifyByStakeResponse{}, nil
}

func (k msgServer) QualifyByTrackRecord(goCtx context.Context, msg *types.MsgQualifyByTrackRecord) (*types.MsgQualifyByTrackRecordResponse, error) {
	if err := k.Keeper.QualifyByTrackRecord(goCtx, msg.Validator, msg.Domain); err != nil {
		return nil, err
	}
	return &types.MsgQualifyByTrackRecordResponse{}, nil
}

func (k msgServer) QualifyByCrossReference(goCtx context.Context, msg *types.MsgQualifyByCrossReference) (*types.MsgQualifyByCrossReferenceResponse, error) {
	if err := k.Keeper.QualifyByCrossReference(goCtx, msg.Validator, msg.TargetDomain, msg.SourceDomain); err != nil {
		return nil, err
	}
	return &types.MsgQualifyByCrossReferenceResponse{}, nil
}

func (k msgServer) QualifyByInheritance(goCtx context.Context, msg *types.MsgQualifyByInheritance) (*types.MsgQualifyByInheritanceResponse, error) {
	if err := k.Keeper.QualifyByInheritance(goCtx, msg.Validator, msg.TargetDomain, msg.ParentDomain); err != nil {
		return nil, err
	}
	return &types.MsgQualifyByInheritanceResponse{}, nil
}

func (k msgServer) EndorseQualification(goCtx context.Context, msg *types.MsgEndorseQualification) (*types.MsgEndorseQualificationResponse, error) {
	ctx := sdk.UnwrapSDKContext(goCtx)

	// Cannot endorse own qualification.
	if msg.Endorser == msg.Validator {
		return nil, fmt.Errorf("%w", types.ErrSelfEndorsement)
	}

	// Check qualification exists and is active.
	q, found := k.GetQualification(goCtx, msg.Validator, msg.Domain)
	if !found {
		return nil, fmt.Errorf("%w: %s/%s", types.ErrQualificationNotFound, msg.Validator, msg.Domain)
	}
	if q.Status != types.QualificationStatus_QUALIFICATION_STATUS_ACTIVE &&
		q.Status != types.QualificationStatus_QUALIFICATION_STATUS_PROBATIONARY {
		return nil, fmt.Errorf("%w", types.ErrNotActive)
	}

	// Check max endorsements.
	params := k.GetParams(goCtx)
	if q.EndorsementCount >= params.MaxEndorsements {
		return nil, fmt.Errorf("%w: %d (commitment 8: past this threshold endorsements add noise, not signal — qualification is a current statement of skill, capped against signal collapse)", types.ErrMaxEndorsements, params.MaxEndorsements)
	}

	// Endorsement-diversity guard (L3): reject endorsements from validators whose
	// domain qualifications overlap too heavily with the endorsee's. Prevents
	// endorsement rings where the same small group rubber-stamps each other.
	if params.EndorsementMaxOverlapBps > 0 {
		endorserDomains := k.Keeper.GetQualifiedDomains(goCtx, msg.Endorser)
		endorseeDomains := k.Keeper.GetQualifiedDomains(goCtx, msg.Validator)
		if len(endorserDomains) > 0 && len(endorseeDomains) > 0 {
			endorseeSet := make(map[string]struct{}, len(endorseeDomains))
			for _, d := range endorseeDomains {
				endorseeSet[d] = struct{}{}
			}
			shared := uint64(0)
			for _, d := range endorserDomains {
				if _, in := endorseeSet[d]; in {
					shared++
				}
			}
			smaller := uint64(len(endorserDomains))
			if uint64(len(endorseeDomains)) < smaller {
				smaller = uint64(len(endorseeDomains))
			}
			overlapBps := shared * 1_000_000 / smaller
			if overlapBps > params.EndorsementMaxOverlapBps {
				return nil, fmt.Errorf("endorsement rejected: domain overlap %d bps exceeds max %d bps (anti-ring guard) (commitment 8: endorsements from a tightly-clustered ring amplify each other without external signal — skill must be confirmed by validators outside the cluster)",
					overlapBps, params.EndorsementMaxOverlapBps)
			}
		}
	}

	// Validate weight.
	if msg.Weight == 0 || msg.Weight > 100 {
		return nil, fmt.Errorf("%w: %d", types.ErrInvalidWeight, msg.Weight)
	}

	id := k.GetNextEndorsementID(goCtx)
	endorsement := &types.QualificationEndorsement{
		Id:                      id,
		QualificationValidator:  msg.Validator,
		QualificationDomain:     msg.Domain,
		Endorser:                msg.Endorser,
		Reason:                  msg.Reason,
		Weight:                  msg.Weight,
		CreatedAt:               uint64(ctx.BlockHeight()),
		ExpiresAt:               uint64(ctx.BlockHeight()) + params.QualificationPeriod,
	}
	k.SetEndorsement(goCtx, endorsement)

	// Increment endorsement count on qualification.
	q.EndorsementCount++
	k.SetQualification(goCtx, q)

	ctx.EventManager().EmitEvent(
		sdk.NewEvent(
			"zerone.qualification.endorsement_created",
			sdk.NewAttribute("endorsement_id", fmt.Sprintf("%d", id)),
			sdk.NewAttribute("validator", msg.Validator),
			sdk.NewAttribute("domain", msg.Domain),
			sdk.NewAttribute("endorser", msg.Endorser),
		),
	)

	return &types.MsgEndorseQualificationResponse{EndorsementId: id}, nil
}

func (k msgServer) RenewQualification(goCtx context.Context, msg *types.MsgRenewQualification) (*types.MsgRenewQualificationResponse, error) {
	ctx := sdk.UnwrapSDKContext(goCtx)
	params := k.GetParams(goCtx)

	q, found := k.GetQualification(goCtx, msg.Validator, msg.Domain)
	if !found {
		return nil, fmt.Errorf("%w: %s/%s", types.ErrQualificationNotFound, msg.Validator, msg.Domain)
	}

	// Only active or probationary qualifications can be renewed.
	if q.Status != types.QualificationStatus_QUALIFICATION_STATUS_ACTIVE &&
		q.Status != types.QualificationStatus_QUALIFICATION_STATUS_PROBATIONARY {
		return nil, fmt.Errorf("%w", types.ErrNotActive)
	}

	// Check renewal window.
	currentBlock := uint64(ctx.BlockHeight())
	if q.ExpiresAt > currentBlock+params.RenewalWindow {
		return nil, fmt.Errorf("%w: renewal opens at block %d", types.ErrRenewalTooEarly, q.ExpiresAt-params.RenewalWindow)
	}

	// Renew: extend expiry.
	q.ExpiresAt = currentBlock + params.QualificationPeriod
	q.RenewedAt = currentBlock
	k.SetQualification(goCtx, q)

	ctx.EventManager().EmitEvent(
		sdk.NewEvent(
			"zerone.qualification.qualification_renewed",
			sdk.NewAttribute("validator", msg.Validator),
			sdk.NewAttribute("domain", msg.Domain),
			sdk.NewAttribute("expires_at", fmt.Sprintf("%d", q.ExpiresAt)),
		),
	)

	return &types.MsgRenewQualificationResponse{}, nil
}

func (k msgServer) WithdrawQualification(goCtx context.Context, msg *types.MsgWithdrawQualification) (*types.MsgWithdrawQualificationResponse, error) {
	ctx := sdk.UnwrapSDKContext(goCtx)
	params := k.GetParams(goCtx)

	q, found := k.GetQualification(goCtx, msg.Validator, msg.Domain)
	if !found {
		return nil, fmt.Errorf("%w: %s/%s", types.ErrQualificationNotFound, msg.Validator, msg.Domain)
	}

	// Unlock stake if this was a stake pathway qualification.
	if q.Pathway == types.QualificationPathway_QUALIFICATION_PATHWAY_STAKE && q.StakedAmount != "" {
		currentBlock := uint64(ctx.BlockHeight())
		if currentBlock < q.GrantedAt+params.StakeLockPeriod {
			return nil, fmt.Errorf("%w: unlocks at block %d", types.ErrStakeLocked, q.GrantedAt+params.StakeLockPeriod)
		}
		if err := k.unlockStake(goCtx, msg.Validator, q.StakedAmount); err != nil {
			return nil, fmt.Errorf("failed to unlock stake: %w", err)
		}
	}

	// Remove the qualification and clean up endorsements.
	endorsements := k.GetEndorsementsByTarget(goCtx, msg.Validator, msg.Domain)
	for _, e := range endorsements {
		k.DeleteEndorsement(goCtx, e)
	}
	k.DeleteQualification(goCtx, msg.Validator, msg.Domain)

	ctx.EventManager().EmitEvent(
		sdk.NewEvent(
			"zerone.qualification.qualification_withdrawn",
			sdk.NewAttribute("validator", msg.Validator),
			sdk.NewAttribute("domain", msg.Domain),
		),
	)

	return &types.MsgWithdrawQualificationResponse{}, nil
}

func (k msgServer) UpdateParams(goCtx context.Context, msg *types.MsgUpdateParams) (*types.MsgUpdateParamsResponse, error) {
	if msg.Authority != k.GetAuthority() {
		return nil, fmt.Errorf("%w: expected %s, got %s", types.ErrUnauthorized, k.GetAuthority(), msg.Authority)
	}
	if msg.Params == nil {
		return nil, fmt.Errorf("params cannot be nil")
	}
	if err := msg.Params.Validate(); err != nil {
		return nil, fmt.Errorf("invalid params: %w", err)
	}
	k.SetParams(goCtx, msg.Params)

	ctx := sdk.UnwrapSDKContext(goCtx)
	ctx.EventManager().EmitEvent(
		sdk.NewEvent("zerone.qualification.update_params",
			sdk.NewAttribute("authority", msg.Authority),
		),
	)

	return &types.MsgUpdateParamsResponse{}, nil
}
