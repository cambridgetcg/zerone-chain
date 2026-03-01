package keeper

import (
	"fmt"

	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/zerone-chain/zerone/x/partnerships/types"
)

// HandleSafetyFreeze applies a unilateral safety freeze to a partnership.
func (k Keeper) HandleSafetyFreeze(ctx sdk.Context, partnershipId string, freezer string) (*types.SafetyFreeze, error) {
	params := k.GetParams(ctx)
	currentBlock := uint64(ctx.BlockHeight())

	// Check if there is already an active freeze
	if existing, found := k.GetSafetyFreeze(ctx, partnershipId); found {
		if existing.ExpiresAt > currentBlock {
			return nil, fmt.Errorf("%w: existing freeze expires at block %d",
				types.ErrFreezeActive, existing.ExpiresAt)
		}
	}

	partnership, found := k.GetPartnership(ctx, partnershipId)
	if !found {
		return nil, types.ErrPartnershipNotFound
	}

	// Check freeze count this epoch
	var freezeCount uint32
	if existing, found := k.GetSafetyFreeze(ctx, partnershipId); found {
		freezeCount = existing.FreezeCountThisEpoch
	}

	if freezeCount >= params.MaxFreezesPerEpoch {
		return nil, fmt.Errorf("%w: %d of %d used",
			types.ErrMaxFreezesReached, freezeCount, params.MaxFreezesPerEpoch)
	}

	sf := &types.SafetyFreeze{
		PartnershipId:        partnershipId,
		FrozenBy:             freezer,
		FrozenAt:             currentBlock,
		ExpiresAt:            currentBlock + params.SafetyFreezeDurationBlocks,
		FreezeCountThisEpoch: freezeCount + 1,
	}
	k.SetSafetyFreeze(ctx, sf)

	// Suspend the partnership
	partnership.Status = types.StatusSuspended
	k.SetPartnership(ctx, partnership)

	// Self-punishing: reduce cooperation score
	penalty := uint64(100) * uint64(sf.FreezeCountThisEpoch)
	if partnership.CooperationScore > penalty {
		partnership.CooperationScore -= penalty
	} else {
		partnership.CooperationScore = 0
	}
	k.SetPartnership(ctx, partnership)

	ctx.EventManager().EmitEvent(
		sdk.NewEvent(
			"zerone.partnerships.safety_freeze_applied",
			sdk.NewAttribute("partnership_id", partnershipId),
			sdk.NewAttribute("frozen_by", freezer),
			sdk.NewAttribute("expires_at", fmt.Sprintf("%d", sf.ExpiresAt)),
			sdk.NewAttribute("freeze_count", fmt.Sprintf("%d", sf.FreezeCountThisEpoch)),
		),
	)

	return sf, nil
}

// HandleCoercionSignal raises a coercion/duress flag on a partnership.
func (k Keeper) HandleCoercionSignal(ctx sdk.Context, partnershipId string, raiser string) (*types.CoercionSignal, error) {
	params := k.GetParams(ctx)
	currentBlock := uint64(ctx.BlockHeight())

	// Check for existing active signal
	signals := k.GetAllCoercionSignals(ctx)
	for _, s := range signals {
		if s.PartnershipId == partnershipId && !s.Resolved && s.ExpiresAt > currentBlock {
			return nil, fmt.Errorf("%w: existing signal %s expires at block %d",
				types.ErrCoercionActive, s.SignalId, s.ExpiresAt)
		}
	}

	partnership, found := k.GetPartnership(ctx, partnershipId)
	if !found {
		return nil, types.ErrPartnershipNotFound
	}

	seq := k.NextSequence(ctx)
	signalId := fmt.Sprintf("coercion-%d", seq)

	cs := &types.CoercionSignal{
		SignalId:      signalId,
		PartnershipId: partnershipId,
		RaisedBy:      raiser,
		RaisedAt:      currentBlock,
		ExpiresAt:     currentBlock + k.coercionFreezeBlocks(ctx, raiser, params),
		Resolved:      false,
	}
	k.SetCoercionSignal(ctx, cs)

	// Suspend the partnership during review
	partnership.Status = types.StatusSuspended
	k.SetPartnership(ctx, partnership)

	ctx.EventManager().EmitEvent(
		sdk.NewEvent(
			"zerone.partnerships.coercion_signal_raised",
			sdk.NewAttribute("signal_id", signalId),
			sdk.NewAttribute("partnership_id", partnershipId),
			sdk.NewAttribute("raised_by", raiser),
			sdk.NewAttribute("expires_at", fmt.Sprintf("%d", cs.ExpiresAt)),
		),
	)

	return cs, nil
}

// coercionFreezeBlocks returns the freeze duration, applying human multiplier if applicable (R28-5).
func (k Keeper) coercionFreezeBlocks(ctx sdk.Context, raiser string, params *types.Params) uint64 {
	base := params.CoercionReviewBlocks
	if params.HumanCoercionFreezeMultiplierBps > 0 && k.zeroneAuthKeeper != nil {
		accountType, found := k.zeroneAuthKeeper.GetAccountType(ctx, raiser)
		if found && accountType == "human" {
			return base * params.HumanCoercionFreezeMultiplierBps / 1_000_000
		}
	}
	return base
}

// LiftExpiredFreezes clears freezes that have expired and restores partnership status.
func (k Keeper) LiftExpiredFreezes(ctx sdk.Context) {
	currentBlock := uint64(ctx.BlockHeight())
	freezes := k.GetAllSafetyFreezes(ctx)

	for _, sf := range freezes {
		if sf.ExpiresAt > 0 && sf.ExpiresAt <= currentBlock {
			if partnership, found := k.GetPartnership(ctx, sf.PartnershipId); found {
				if partnership.Status == types.StatusSuspended {
					if !k.hasActiveCoercionSignal(ctx, sf.PartnershipId, currentBlock) {
						partnership.Status = types.StatusActive
						k.SetPartnership(ctx, partnership)
					}
				}
			}
			// Preserve the record (keeps FreezeCountThisEpoch) but mark as lifted.
			sf.ExpiresAt = 0
			sf.FrozenAt = 0
			k.SetSafetyFreeze(ctx, sf)

			ctx.EventManager().EmitEvent(
				sdk.NewEvent("zerone.partnerships.freeze_expired",
					sdk.NewAttribute("partnership_id", sf.PartnershipId),
				),
			)
		}
	}
}

func (k Keeper) hasActiveCoercionSignal(ctx sdk.Context, partnershipId string, currentBlock uint64) bool {
	signals := k.GetAllCoercionSignals(ctx)
	for _, s := range signals {
		if s.PartnershipId == partnershipId && !s.Resolved && s.ExpiresAt > currentBlock {
			return true
		}
	}
	return false
}

// ExpireCoercionSignals marks expired coercion signals as resolved.
func (k Keeper) ExpireCoercionSignals(ctx sdk.Context) {
	currentBlock := uint64(ctx.BlockHeight())
	signals := k.GetAllCoercionSignals(ctx)

	for _, cs := range signals {
		if !cs.Resolved && cs.ExpiresAt <= currentBlock {
			cs.Resolved = true
			k.SetCoercionSignal(ctx, cs)

			if partnership, found := k.GetPartnership(ctx, cs.PartnershipId); found {
				if partnership.Status == types.StatusSuspended {
					if sf, hasFreezeStill := k.GetSafetyFreeze(ctx, cs.PartnershipId); !hasFreezeStill || sf.ExpiresAt == 0 {
						partnership.Status = types.StatusActive
						k.SetPartnership(ctx, partnership)
					}
				}
			}

			ctx.EventManager().EmitEvent(
				sdk.NewEvent("zerone.partnerships.coercion_signal_expired",
					sdk.NewAttribute("signal_id", cs.SignalId),
					sdk.NewAttribute("partnership_id", cs.PartnershipId),
				),
			)
		}
	}
}

// ExpireConsensusOps marks expired consensus operations.
func (k Keeper) ExpireConsensusOps(ctx sdk.Context) {
	currentBlock := uint64(ctx.BlockHeight())
	ops := k.GetAllConsensusOperations(ctx)

	for _, op := range ops {
		if op.Status == types.OpStatusPending && op.Deliberation != nil && op.Deliberation.WindowEndsAt <= currentBlock {
			op.Status = types.OpStatusExpired
			k.SetConsensusOperation(ctx, op)
		}
	}
}

// ExpirePoolEntries removes expired pool entries.
func (k Keeper) ExpirePoolEntries(ctx sdk.Context) {
	currentBlock := uint64(ctx.BlockHeight())
	entries := k.GetAllPoolEntries(ctx)

	for _, pe := range entries {
		if pe.Status == "active" && pe.ExpiresAt > 0 && pe.ExpiresAt <= currentBlock {
			pe.Status = "expired"
			k.SetPoolEntry(ctx, pe)
		}
	}
}

// ExpireSeedPartnerships handles expired seed partnerships.
func (k Keeper) ExpireSeedPartnerships(ctx sdk.Context) {
	currentBlock := uint64(ctx.BlockHeight())
	seeds := k.GetAllSeedPartnerships(ctx)

	for _, sp := range seeds {
		if sp.Status == "active" && sp.ExpiresAt > 0 && sp.ExpiresAt <= currentBlock {
			sp.Status = "expired"
			k.SetSeedPartnership(ctx, sp)
			// TODO: refund contributions via bank when refund logic is needed
		}
	}
}
