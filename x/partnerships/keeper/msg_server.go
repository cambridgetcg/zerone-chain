package keeper

import (
	"context"
	"fmt"
	"math/big"

	sdkmath "cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/zerone-chain/zerone/x/partnerships/types"
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

// ProposePartnership starts partnership formation.
func (k msgServer) ProposePartnership(goCtx context.Context, msg *types.MsgProposePartnership) (*types.MsgProposePartnershipResponse, error) {
	ctx := sdk.UnwrapSDKContext(goCtx)
	params := k.GetParams(ctx)
	currentBlock := uint64(ctx.BlockHeight())

	// Check for existing non-dissolved partnership between these two
	existingByHuman := k.GetPartnershipsByHuman(ctx, msg.Proposer)
	for _, id := range existingByHuman {
		if p, found := k.GetPartnership(ctx, id); found {
			if (p.HumanAddr == msg.Proposer && p.AgentAddr == msg.Partner) ||
				(p.HumanAddr == msg.Partner && p.AgentAddr == msg.Proposer) {
				if p.Status != types.StatusDissolved {
					return nil, types.ErrPartnershipExists
				}
			}
		}
	}
	existingByAgent := k.GetPartnershipsByAgent(ctx, msg.Proposer)
	for _, id := range existingByAgent {
		if p, found := k.GetPartnership(ctx, id); found {
			if (p.HumanAddr == msg.Proposer && p.AgentAddr == msg.Partner) ||
				(p.HumanAddr == msg.Partner && p.AgentAddr == msg.Proposer) {
				if p.Status != types.StatusDissolved {
					return nil, types.ErrPartnershipExists
				}
			}
		}
	}

	// Validate lock tier
	if msg.ProposedTier > 5 {
		return nil, types.ErrInvalidLockTier
	}
	lockDuration := types.LockTiers[msg.ProposedTier].MinBlocks

	// Escrow initial deposit if provided
	if msg.InitialDeposit != "" {
		depositAmt := new(big.Int)
		if _, ok := depositAmt.SetString(msg.InitialDeposit, 10); ok && depositAmt.Sign() > 0 {
			depositorAddr, err := sdk.AccAddressFromBech32(msg.Proposer)
			if err != nil {
				return nil, fmt.Errorf("invalid proposer address: %w", err)
			}
			coins := sdk.NewCoins(sdk.NewCoin("uzrn", sdkmath.NewIntFromBigInt(depositAmt)))
			if err := k.bankKeeper.SendCoinsFromAccountToModule(ctx, depositorAddr, types.ModuleName, coins); err != nil {
				return nil, fmt.Errorf("failed to escrow deposit: %w", err)
			}
		}
	}

	// Enforce min_partnership_stake
	if params.MinPartnershipStake != "" {
		minStake := new(big.Int)
		if _, ok := minStake.SetString(params.MinPartnershipStake, 10); ok && minStake.Sign() > 0 {
			depositAmt := new(big.Int)
			if msg.InitialDeposit != "" {
				depositAmt.SetString(msg.InitialDeposit, 10)
			}
			if depositAmt.Cmp(minStake) < 0 {
				return nil, fmt.Errorf("%w: minimum stake is %s", types.ErrInsufficientDeposit, params.MinPartnershipStake)
			}
		}
	}

	seq := k.NextSequence(ctx)
	partnershipId := fmt.Sprintf("partnership-%d", seq)

	partnership := &types.Partnership{
		Id:               partnershipId,
		HumanAddr:        msg.Proposer,
		AgentAddr:        msg.Partner,
		Status:           types.StatusPending,
		Tier:             0, // starts at trial
		LockTier:         msg.ProposedTier,
		LockExpiresAt:    currentBlock + lockDuration,
		SplitHumanBps:    params.DefaultHumanSplitBps,
		SplitAgentBps:    params.DefaultAgentSplitBps,
		CommonPotBalance: "0",
		TotalEarned:      "0",
		CooperationScore: 500000, // Start at 50%
		FormedAtBlock:    currentBlock,
	}

	// Add initial deposit to common pot
	if msg.InitialDeposit != "" {
		depositAmt := new(big.Int)
		if _, ok := depositAmt.SetString(msg.InitialDeposit, 10); ok && depositAmt.Sign() > 0 {
			partnership.CommonPotBalance = depositAmt.String()
		}
	}

	k.SetPartnership(ctx, partnership)

	// Store formation record with expiry
	kvStore := k.storeService.OpenKVStore(ctx)
	formationExpiry := currentBlock + params.FormationWindowBlocks
	_ = kvStore.Set(
		append(types.FormationKeyPrefix, []byte(partnershipId)...),
		[]byte(fmt.Sprintf("%d", formationExpiry)),
	)

	ctx.EventManager().EmitEvent(
		sdk.NewEvent(
			"zerone.partnerships.partnership_proposed",
			sdk.NewAttribute("partnership_id", partnershipId),
			sdk.NewAttribute("proposer", msg.Proposer),
			sdk.NewAttribute("partner", msg.Partner),
		),
	)

	return &types.MsgProposePartnershipResponse{PartnershipId: partnershipId}, nil
}

// AcceptPartnership accepts a pending partnership formation.
func (k msgServer) AcceptPartnership(goCtx context.Context, msg *types.MsgAcceptPartnership) (*types.MsgAcceptPartnershipResponse, error) {
	ctx := sdk.UnwrapSDKContext(goCtx)
	currentBlock := uint64(ctx.BlockHeight())

	partnership, found := k.GetPartnership(ctx, msg.PartnershipId)
	if !found {
		return nil, types.ErrPartnershipNotFound
	}

	if partnership.Status != types.StatusPending {
		return nil, types.ErrNotFormingStatus
	}

	// Verify the accepter is the partner (not the proposer)
	if msg.Accepter != partnership.AgentAddr {
		return nil, fmt.Errorf("%w: accepter must be the partner address", types.ErrNotParticipant)
	}

	// Check formation window
	kvStore := k.storeService.OpenKVStore(ctx)
	expiryBz, err := kvStore.Get(append(types.FormationKeyPrefix, []byte(msg.PartnershipId)...))
	if err != nil || expiryBz == nil {
		return nil, types.ErrFormationExpired
	}
	var expiry uint64
	if _, err := fmt.Sscanf(string(expiryBz), "%d", &expiry); err != nil {
		return nil, types.ErrFormationExpired
	}
	if currentBlock > expiry {
		return nil, types.ErrFormationExpired
	}

	// Escrow deposit if provided
	if msg.Deposit != "" {
		depositAmt := new(big.Int)
		if _, ok := depositAmt.SetString(msg.Deposit, 10); ok && depositAmt.Sign() > 0 {
			accepterAddr, err := sdk.AccAddressFromBech32(msg.Accepter)
			if err != nil {
				return nil, fmt.Errorf("invalid accepter address: %w", err)
			}
			coins := sdk.NewCoins(sdk.NewCoin("uzrn", sdkmath.NewIntFromBigInt(depositAmt)))
			if err := k.bankKeeper.SendCoinsFromAccountToModule(ctx, accepterAddr, types.ModuleName, coins); err != nil {
				return nil, fmt.Errorf("failed to escrow deposit: %w", err)
			}
			// Add to common pot
			currentPot := new(big.Int)
			if partnership.CommonPotBalance != "" {
				currentPot.SetString(partnership.CommonPotBalance, 10)
			}
			currentPot.Add(currentPot, depositAmt)
			partnership.CommonPotBalance = currentPot.String()
		}
	}

	// Activate the partnership
	partnership.Status = types.StatusActive
	partnership.FormedAtBlock = currentBlock
	k.SetPartnership(ctx, partnership)

	// Clean up formation record
	_ = kvStore.Delete(append(types.FormationKeyPrefix, []byte(msg.PartnershipId)...))

	// Auto-link home if home keeper is set
	if k.homeKeeper != nil {
		homeIDs := k.homeKeeper.GetHomesByOwner(ctx, partnership.AgentAddr)
		if len(homeIDs) > 0 {
			k.homeKeeper.SetPartnershipOnHome(ctx, homeIDs[0], partnership.Id)
		}
	}

	ctx.EventManager().EmitEvent(
		sdk.NewEvent(
			"zerone.partnerships.partnership_accepted",
			sdk.NewAttribute("partnership_id", msg.PartnershipId),
			sdk.NewAttribute("accepter", msg.Accepter),
		),
	)

	return &types.MsgAcceptPartnershipResponse{}, nil
}

// ProposeConsensusOp proposes a consensus operation.
func (k msgServer) ProposeConsensusOp(goCtx context.Context, msg *types.MsgProposeConsensusOp) (*types.MsgProposeConsensusOpResponse, error) {
	ctx := sdk.UnwrapSDKContext(goCtx)
	currentBlock := uint64(ctx.BlockHeight())

	partnership, found := k.GetPartnership(ctx, msg.PartnershipId)
	if !found {
		return nil, types.ErrPartnershipNotFound
	}
	if partnership.Status != types.StatusActive {
		return nil, fmt.Errorf("%w: partnership must be active for operations", types.ErrInvalidStatus)
	}

	// Verify proposer is a participant
	if msg.Proposer != partnership.HumanAddr && msg.Proposer != partnership.AgentAddr {
		return nil, types.ErrNotParticipant
	}

	// Check for active freeze
	if sf, found := k.GetSafetyFreeze(ctx, msg.PartnershipId); found && sf.ExpiresAt > currentBlock {
		return nil, types.ErrFreezeActive
	}

	// Check rejection cooldown
	if rc, found := k.GetRejectionCooldown(ctx, msg.PartnershipId); found {
		if rc.CooldownEndsAt > currentBlock {
			return nil, fmt.Errorf("%w: cooldown ends at block %d",
				types.ErrCooldownActive, rc.CooldownEndsAt)
		}
	}

	delib := k.CreateDeliberationState(ctx, msg.Amount, partnership.CommonPotBalance, msg.Rationale, "", 0)

	seq := k.NextSequence(ctx)
	opId := fmt.Sprintf("op-%d", seq)

	op := &types.ConsensusOperation{
		Id:            opId,
		PartnershipId: msg.PartnershipId,
		OpType:        msg.OpType,
		ProposedBy:    msg.Proposer,
		Amount:        msg.Amount,
		Status:        types.OpStatusPending,
		Deliberation:  delib,
		CreatedAt:     currentBlock,
	}
	k.SetConsensusOperation(ctx, op)

	ctx.EventManager().EmitEvent(
		sdk.NewEvent(
			"zerone.partnerships.operation_proposed",
			sdk.NewAttribute("operation_id", opId),
			sdk.NewAttribute("partnership_id", msg.PartnershipId),
			sdk.NewAttribute("op_type", msg.OpType),
			sdk.NewAttribute("proposer", msg.Proposer),
			sdk.NewAttribute("amount", msg.Amount),
		),
	)

	return &types.MsgProposeConsensusOpResponse{OperationId: opId}, nil
}

// VoteConsensusOp handles both approval and rejection of consensus operations.
func (k msgServer) VoteConsensusOp(goCtx context.Context, msg *types.MsgVoteConsensusOp) (*types.MsgVoteConsensusOpResponse, error) {
	ctx := sdk.UnwrapSDKContext(goCtx)
	currentBlock := uint64(ctx.BlockHeight())

	op, found := k.GetConsensusOperation(ctx, msg.OperationId)
	if !found {
		return nil, fmt.Errorf("operation not found: %s", msg.OperationId)
	}
	if op.Status != types.OpStatusPending {
		return nil, fmt.Errorf("%w: operation is %s", types.ErrInvalidStatus, op.Status)
	}

	partnership, found := k.GetPartnership(ctx, op.PartnershipId)
	if !found {
		return nil, types.ErrPartnershipNotFound
	}

	// Verify voter is the other partner (not the proposer)
	if msg.Voter == op.ProposedBy {
		return nil, fmt.Errorf("%w: cannot vote on own proposal", types.ErrUnauthorized)
	}
	if msg.Voter != partnership.HumanAddr && msg.Voter != partnership.AgentAddr {
		return nil, types.ErrNotParticipant
	}

	resp := &types.MsgVoteConsensusOpResponse{}

	if msg.Approve {
		// --- APPROVAL ---
		if currentBlock > op.Deliberation.WindowEndsAt {
			op.Status = types.OpStatusExpired
			k.SetConsensusOperation(ctx, op)
			return nil, fmt.Errorf("%w: deliberation window expired", types.ErrInvalidStatus)
		}
		if currentBlock < op.Deliberation.FloorEndsAt {
			return nil, fmt.Errorf("%w: floor period not yet elapsed (ends block %d)",
				types.ErrDeliberationActive, op.Deliberation.FloorEndsAt)
		}

		op.Status = types.OpStatusApproved
		k.SetConsensusOperation(ctx, op)

		// Clear rejection cooldown on approval
		k.DeleteRejectionCooldown(ctx, op.PartnershipId)

		// Execute the operation
		switch op.OpType {
		case "withdraw":
			if op.Amount != "" {
				withdrawAmt := new(big.Int)
				if _, ok := withdrawAmt.SetString(op.Amount, 10); ok && withdrawAmt.Sign() > 0 {
					potBal := new(big.Int)
					if partnership.CommonPotBalance != "" {
						potBal.SetString(partnership.CommonPotBalance, 10)
					}
					if potBal.Cmp(withdrawAmt) >= 0 {
						potBal.Sub(potBal, withdrawAmt)
						partnership.CommonPotBalance = potBal.String()
						k.SetPartnership(ctx, partnership)

						recipientAddr, err := sdk.AccAddressFromBech32(op.ProposedBy)
						if err == nil {
							coins := sdk.NewCoins(sdk.NewCoin("uzrn", sdkmath.NewIntFromBigInt(withdrawAmt)))
							_ = k.bankKeeper.SendCoinsFromModuleToAccount(ctx, types.ModuleName, recipientAddr, coins)
						}
					}
				}
			}
		case "invest":
			// invest adds to pot (funds already deposited)
		case "split_change":
			// split_change would update splits
		case "tier_upgrade":
			// tier_upgrade would update tier
		}

		ctx.EventManager().EmitEvent(
			sdk.NewEvent(
				"zerone.partnerships.operation_approved",
				sdk.NewAttribute("operation_id", msg.OperationId),
				sdk.NewAttribute("approver", msg.Voter),
			),
		)
	} else {
		// --- REJECTION ---
		op.Status = types.OpStatusRejected
		k.SetConsensusOperation(ctx, op)

		// Update rejection cooldown (exponential)
		rc, found := k.GetRejectionCooldown(ctx, op.PartnershipId)
		if !found {
			rc = &types.RejectionCooldown{PartnershipId: op.PartnershipId}
		}
		rc.RejectionCount++
		cooldown := k.CalculateCooldown(ctx, rc.RejectionCount)
		rc.CooldownEndsAt = currentBlock + cooldown
		k.SetRejectionCooldown(ctx, rc)

		// Handle counter-proposal if provided
		if msg.CounterAmount != "" {
			counterAmt := new(big.Int)
			if _, ok := counterAmt.SetString(msg.CounterAmount, 10); ok && counterAmt.Sign() > 0 {
				if err := k.ValidateCounterProposal(ctx, op); err == nil {
					delib := k.CreateDeliberationState(
						ctx,
						msg.CounterAmount,
						partnership.CommonPotBalance,
						msg.Rationale,
						op.Id,
						op.Deliberation.ChainDepth+1,
					)

					seq := k.NextSequence(ctx)
					counterOpId := fmt.Sprintf("op-%d", seq)

					counterOp := &types.ConsensusOperation{
						Id:            counterOpId,
						PartnershipId: op.PartnershipId,
						OpType:        op.OpType,
						ProposedBy:    msg.Voter,
						Amount:        msg.CounterAmount,
						Status:        types.OpStatusPending,
						Deliberation:  delib,
						CreatedAt:     currentBlock,
					}
					k.SetConsensusOperation(ctx, counterOp)

					resp.CounterOperationId = counterOpId
				}
			}
		}

		ctx.EventManager().EmitEvent(
			sdk.NewEvent(
				"zerone.partnerships.operation_rejected",
				sdk.NewAttribute("operation_id", msg.OperationId),
				sdk.NewAttribute("rejecter", msg.Voter),
			),
		)
	}

	return resp, nil
}

// SafetyFreeze triggers a unilateral safety freeze.
func (k msgServer) SafetyFreeze(goCtx context.Context, msg *types.MsgSafetyFreeze) (*types.MsgSafetyFreezeResponse, error) {
	ctx := sdk.UnwrapSDKContext(goCtx)

	partnership, found := k.GetPartnership(ctx, msg.PartnershipId)
	if !found {
		return nil, types.ErrPartnershipNotFound
	}
	if msg.Freezer != partnership.HumanAddr && msg.Freezer != partnership.AgentAddr {
		return nil, types.ErrNotParticipant
	}
	if partnership.Status != types.StatusActive {
		return nil, fmt.Errorf("%w: partnership must be active to freeze", types.ErrInvalidStatus)
	}

	sf, err := k.HandleSafetyFreeze(ctx, msg.PartnershipId, msg.Freezer)
	if err != nil {
		return nil, err
	}

	return &types.MsgSafetyFreezeResponse{ExpiresAt: sf.ExpiresAt}, nil
}

// RaiseCoercionSignal raises a coercion/duress flag.
func (k msgServer) RaiseCoercionSignal(goCtx context.Context, msg *types.MsgRaiseCoercionSignal) (*types.MsgRaiseCoercionSignalResponse, error) {
	ctx := sdk.UnwrapSDKContext(goCtx)

	partnership, found := k.GetPartnership(ctx, msg.PartnershipId)
	if !found {
		return nil, types.ErrPartnershipNotFound
	}
	if msg.Raiser != partnership.HumanAddr && msg.Raiser != partnership.AgentAddr {
		return nil, types.ErrNotParticipant
	}

	cs, err := k.HandleCoercionSignal(ctx, msg.PartnershipId, msg.Raiser)
	if err != nil {
		return nil, err
	}

	return &types.MsgRaiseCoercionSignalResponse{SignalId: cs.SignalId}, nil
}

// InitiateDissolution starts the exit/dissolution process.
func (k msgServer) InitiateDissolution(goCtx context.Context, msg *types.MsgInitiateDissolution) (*types.MsgInitiateDissolutionResponse, error) {
	ctx := sdk.UnwrapSDKContext(goCtx)

	partnership, found := k.GetPartnership(ctx, msg.PartnershipId)
	if !found {
		return nil, types.ErrPartnershipNotFound
	}
	if msg.Initiator != partnership.HumanAddr && msg.Initiator != partnership.AgentAddr {
		return nil, types.ErrNotParticipant
	}

	cooldownEnd, err := k.HandleExit(ctx, msg.PartnershipId, msg.Initiator)
	if err != nil {
		return nil, err
	}

	return &types.MsgInitiateDissolutionResponse{CooldownEnd: cooldownEnd}, nil
}

// CreateSeedPartnership creates a seed (bootstrap) partnership.
func (k msgServer) CreateSeedPartnership(goCtx context.Context, msg *types.MsgCreateSeedPartnership) (*types.MsgCreateSeedPartnershipResponse, error) {
	ctx := sdk.UnwrapSDKContext(goCtx)
	params := k.GetParams(ctx)
	currentBlock := uint64(ctx.BlockHeight())

	// Check seed limit per DID
	if k.CountActiveSeedsByDID(ctx, msg.Human) >= 2 {
		return nil, types.ErrSeedLimitExceeded
	}

	// Escrow human contribution
	if msg.HumanContribution != "" {
		contributionAmt := new(big.Int)
		if _, ok := contributionAmt.SetString(msg.HumanContribution, 10); ok && contributionAmt.Sign() > 0 {
			humanAddr, err := sdk.AccAddressFromBech32(msg.Human)
			if err != nil {
				return nil, fmt.Errorf("invalid human address: %w", err)
			}
			coins := sdk.NewCoins(sdk.NewCoin("uzrn", sdkmath.NewIntFromBigInt(contributionAmt)))
			if err := k.bankKeeper.SendCoinsFromAccountToModule(ctx, humanAddr, types.ModuleName, coins); err != nil {
				return nil, fmt.Errorf("failed to escrow contribution: %w", err)
			}
		}
	}

	seq := k.NextSequence(ctx)
	seedId := fmt.Sprintf("seed-%d", seq)

	potCap := params.SeedCommonPotCap
	if potCap == "" {
		potCap = "100000000"
	}

	sp := &types.SeedPartnership{
		Id:                seedId,
		HumanAddr:         msg.Human,
		AgentAddr:         msg.Agent,
		CreatedAt:         currentBlock,
		ExpiresAt:         currentBlock + params.SeedPartnershipDuration,
		HumanContribution: msg.HumanContribution,
		AgentContribution: "0",
		Status:            "active",
		CommonPotBalance:  msg.HumanContribution,
		CommonPotCap:      potCap,
	}
	k.SetSeedPartnership(ctx, sp)

	ctx.EventManager().EmitEvent(
		sdk.NewEvent(
			"zerone.partnerships.seed_partnership_created",
			sdk.NewAttribute("seed_id", seedId),
			sdk.NewAttribute("human", msg.Human),
			sdk.NewAttribute("agent", msg.Agent),
		),
	)

	return &types.MsgCreateSeedPartnershipResponse{SeedId: seedId}, nil
}

// JoinFormationPool registers an address in the formation pool.
func (k msgServer) JoinFormationPool(goCtx context.Context, msg *types.MsgJoinFormationPool) (*types.MsgJoinFormationPoolResponse, error) {
	ctx := sdk.UnwrapSDKContext(goCtx)
	currentBlock := uint64(ctx.BlockHeight())

	// Check if already in pool
	if _, found := k.GetPoolEntry(ctx, msg.Joiner); found {
		return nil, types.ErrAlreadyInPool
	}

	// Check pool capacity
	if k.CountActivePoolEntries(ctx) >= 222 {
		return nil, types.ErrPoolFull
	}

	// Escrow deposit if provided
	if msg.Deposit != "" {
		depositAmt := new(big.Int)
		if _, ok := depositAmt.SetString(msg.Deposit, 10); ok && depositAmt.Sign() > 0 {
			joinerAddr, err := sdk.AccAddressFromBech32(msg.Joiner)
			if err != nil {
				return nil, fmt.Errorf("invalid joiner address: %w", err)
			}
			coins := sdk.NewCoins(sdk.NewCoin("uzrn", sdkmath.NewIntFromBigInt(depositAmt)))
			if err := k.bankKeeper.SendCoinsFromAccountToModule(ctx, joinerAddr, types.ModuleName, coins); err != nil {
				return nil, fmt.Errorf("failed to escrow pool deposit: %w", err)
			}
		}
	}

	pe := &types.PoolEntry{
		Address:       msg.Joiner,
		Domains:       msg.Domains,
		PreferredRole: msg.PreferredRole,
		RegisteredAt:  currentBlock,
		Deposit:       msg.Deposit,
		ExpiresAt:     currentBlock + 11111,
		Status:        "active",
	}
	k.SetPoolEntry(ctx, pe)

	ctx.EventManager().EmitEvent(
		sdk.NewEvent(
			"zerone.partnerships.joined_formation_pool",
			sdk.NewAttribute("address", msg.Joiner),
		),
	)

	return &types.MsgJoinFormationPoolResponse{}, nil
}

// LeaveFormationPool removes an address from the formation pool.
func (k msgServer) LeaveFormationPool(goCtx context.Context, msg *types.MsgLeaveFormationPool) (*types.MsgLeaveFormationPoolResponse, error) {
	ctx := sdk.UnwrapSDKContext(goCtx)

	pe, found := k.GetPoolEntry(ctx, msg.Leaver)
	if !found {
		return nil, types.ErrNotInPool
	}

	// Refund deposit if any
	if pe.Deposit != "" {
		depositAmt := new(big.Int)
		if _, ok := depositAmt.SetString(pe.Deposit, 10); ok && depositAmt.Sign() > 0 {
			leaverAddr, err := sdk.AccAddressFromBech32(msg.Leaver)
			if err == nil {
				coins := sdk.NewCoins(sdk.NewCoin("uzrn", sdkmath.NewIntFromBigInt(depositAmt)))
				_ = k.bankKeeper.SendCoinsFromModuleToAccount(ctx, types.ModuleName, leaverAddr, coins)
			}
		}
	}

	k.DeletePoolEntry(ctx, msg.Leaver)

	ctx.EventManager().EmitEvent(
		sdk.NewEvent(
			"zerone.partnerships.left_formation_pool",
			sdk.NewAttribute("address", msg.Leaver),
		),
	)

	return &types.MsgLeaveFormationPoolResponse{}, nil
}

// UpdateParams handles governance-gated parameter update.
func (k msgServer) UpdateParams(goCtx context.Context, msg *types.MsgUpdateParams) (*types.MsgUpdateParamsResponse, error) {
	if k.GetAuthority() != msg.Authority {
		return nil, fmt.Errorf("%w: expected %s, got %s",
			types.ErrUnauthorized, k.GetAuthority(), msg.Authority)
	}

	ctx := sdk.UnwrapSDKContext(goCtx)
	if msg.Params == nil {
		return nil, fmt.Errorf("params cannot be nil")
	}
	k.SetParams(ctx, msg.Params)

	ctx.EventManager().EmitEvent(
		sdk.NewEvent(
			"zerone.partnerships.params_updated",
			sdk.NewAttribute("authority", msg.Authority),
		),
	)

	return &types.MsgUpdateParamsResponse{}, nil
}

// ProposeMentorship creates a new mentorship proposal.
func (k msgServer) ProposeMentorship(goCtx context.Context, msg *types.MsgProposeMentorship) (*types.MsgProposeMentorshipResponse, error) {
	ctx := sdk.UnwrapSDKContext(goCtx)
	params := k.GetParams(ctx)

	if msg.Mentor == msg.Mentee {
		return nil, types.ErrSelfMentorship
	}

	if k.CountActiveMentorshipsForMentor(ctx, msg.Mentor) >= int(params.MaxMentorshipsPerMentor) {
		return nil, types.ErrMaxMentorshipsReached
	}

	if _, found := k.GetActiveMentorshipForMentee(ctx, msg.Mentee); found {
		return nil, types.ErrAlreadyMentored
	}

	seq := k.NextSequence(ctx)
	mentorshipId := fmt.Sprintf("mentorship-%d", seq)

	m := &types.Mentorship{
		Id:                  mentorshipId,
		MentorAddr:          msg.Mentor,
		MenteeAddr:          msg.Mentee,
		Domain:              msg.Domain,
		Status:              "proposed",
		DurationBlocks:      msg.DurationBlocks,
		GraduationThreshold: params.GraduationVerifications,
		GraduationClaimsReq: params.GraduationClaims,
	}
	k.SetMentorship(ctx, m)

	ctx.EventManager().EmitEvent(
		sdk.NewEvent("zerone.partnerships.mentorship_proposed",
			sdk.NewAttribute("mentorship_id", mentorshipId),
			sdk.NewAttribute("mentor", msg.Mentor),
			sdk.NewAttribute("mentee", msg.Mentee),
			sdk.NewAttribute("domain", msg.Domain),
		),
	)

	return &types.MsgProposeMentorshipResponse{MentorshipId: mentorshipId}, nil
}

// AcceptMentorship accepts a pending mentorship proposal.
func (k msgServer) AcceptMentorship(goCtx context.Context, msg *types.MsgAcceptMentorship) (*types.MsgAcceptMentorshipResponse, error) {
	ctx := sdk.UnwrapSDKContext(goCtx)

	m, found := k.GetMentorship(ctx, msg.MentorshipId)
	if !found {
		return nil, types.ErrMentorshipNotFound
	}
	if m.Status != "proposed" {
		return nil, types.ErrMentorshipNotProposed
	}
	if m.MenteeAddr != msg.Mentee {
		return nil, types.ErrNotMentorshipParticipant
	}

	m.Status = "active"
	m.StartBlock = uint64(ctx.BlockHeight())
	k.SetMentorship(ctx, m)

	ctx.EventManager().EmitEvent(
		sdk.NewEvent("zerone.partnerships.mentorship_accepted",
			sdk.NewAttribute("mentorship_id", m.Id),
			sdk.NewAttribute("mentor", m.MentorAddr),
			sdk.NewAttribute("mentee", m.MenteeAddr),
		),
	)

	return &types.MsgAcceptMentorshipResponse{}, nil
}

// GraduateMentee manually graduates a mentee from an active mentorship.
func (k msgServer) GraduateMentee(goCtx context.Context, msg *types.MsgGraduateMentee) (*types.MsgGraduateMenteeResponse, error) {
	ctx := sdk.UnwrapSDKContext(goCtx)

	m, found := k.GetMentorship(ctx, msg.MentorshipId)
	if !found {
		return nil, types.ErrMentorshipNotFound
	}
	if m.Status != "active" {
		return nil, types.ErrMentorshipNotActive
	}
	if m.MentorAddr != msg.Mentor {
		return nil, types.ErrNotMentorshipParticipant
	}

	k.graduateMentorship(ctx, m)

	return &types.MsgGraduateMenteeResponse{}, nil
}

// EndMentorship terminates a mentorship early.
func (k msgServer) EndMentorship(goCtx context.Context, msg *types.MsgEndMentorship) (*types.MsgEndMentorshipResponse, error) {
	ctx := sdk.UnwrapSDKContext(goCtx)

	m, found := k.GetMentorship(ctx, msg.MentorshipId)
	if !found {
		return nil, types.ErrMentorshipNotFound
	}
	if m.Status != "proposed" && m.Status != "active" {
		return nil, types.ErrMentorshipNotActive
	}
	if m.MentorAddr != msg.Sender && m.MenteeAddr != msg.Sender {
		return nil, types.ErrNotMentorshipParticipant
	}

	m.Status = "terminated"
	k.SetMentorship(ctx, m)

	ctx.EventManager().EmitEvent(
		sdk.NewEvent("zerone.partnerships.mentorship_terminated",
			sdk.NewAttribute("mentorship_id", m.Id),
			sdk.NewAttribute("terminated_by", msg.Sender),
		),
	)

	return &types.MsgEndMentorshipResponse{}, nil
}

// AcceptFormationMatch accepts a proposed formation match.
func (k msgServer) AcceptFormationMatch(goCtx context.Context, msg *types.MsgAcceptFormationMatch) (*types.MsgAcceptFormationMatchResponse, error) {
	ctx := sdk.UnwrapSDKContext(goCtx)
	params := k.GetParams(ctx)

	fm, found := k.GetFormationMatch(ctx, msg.MatchId)
	if !found {
		return nil, types.ErrMatchNotFound
	}
	if fm.Status != "proposed" {
		return nil, types.ErrMatchNotProposed
	}
	if msg.Accepter != fm.Addr1 && msg.Accepter != fm.Addr2 {
		return nil, types.ErrNotMatchParticipant
	}

	if msg.Accepter == fm.Addr1 {
		fm.Addr1Accepted = true
	} else {
		fm.Addr2Accepted = true
	}

	if fm.Addr1Accepted && fm.Addr2Accepted {
		fm.Status = "accepted"
		k.SetFormationMatch(ctx, fm)

		k.DeletePoolEntry(ctx, fm.Addr1)
		k.DeletePoolEntry(ctx, fm.Addr2)

		currentBlock := uint64(ctx.BlockHeight())
		seq := k.NextSequence(ctx)
		partnershipId := fmt.Sprintf("partnership-%d", seq)

		partnership := &types.Partnership{
			Id:               partnershipId,
			HumanAddr:        fm.Addr1,
			AgentAddr:        fm.Addr2,
			Status:           types.StatusPending,
			Tier:             0,
			LockTier:         0,
			LockExpiresAt:    currentBlock + types.LockTiers[0].MinBlocks,
			SplitHumanBps:    params.DefaultHumanSplitBps,
			SplitAgentBps:    params.DefaultAgentSplitBps,
			CommonPotBalance: "0",
			TotalEarned:      "0",
			CooperationScore: 500000,
			FormedAtBlock:    currentBlock,
		}
		k.SetPartnership(ctx, partnership)

		kvStore := k.storeService.OpenKVStore(ctx)
		formationExpiry := currentBlock + params.FormationWindowBlocks
		_ = kvStore.Set(
			append(types.FormationKeyPrefix, []byte(partnershipId)...),
			[]byte(fmt.Sprintf("%d", formationExpiry)),
		)

		ctx.EventManager().EmitEvent(
			sdk.NewEvent("zerone.partnerships.formation_match_accepted",
				sdk.NewAttribute("match_id", fm.Id),
				sdk.NewAttribute("partnership_id", partnershipId),
			),
		)
	} else {
		k.SetFormationMatch(ctx, fm)

		ctx.EventManager().EmitEvent(
			sdk.NewEvent("zerone.partnerships.formation_match_partial_accept",
				sdk.NewAttribute("match_id", fm.Id),
				sdk.NewAttribute("accepter", msg.Accepter),
			),
		)
	}

	return &types.MsgAcceptFormationMatchResponse{}, nil
}

// DeclineFormationMatch declines a proposed formation match.
func (k msgServer) DeclineFormationMatch(goCtx context.Context, msg *types.MsgDeclineFormationMatch) (*types.MsgDeclineFormationMatchResponse, error) {
	ctx := sdk.UnwrapSDKContext(goCtx)

	fm, found := k.GetFormationMatch(ctx, msg.MatchId)
	if !found {
		return nil, types.ErrMatchNotFound
	}
	if fm.Status != "proposed" {
		return nil, types.ErrMatchNotProposed
	}
	if msg.Decliner != fm.Addr1 && msg.Decliner != fm.Addr2 {
		return nil, types.ErrNotMatchParticipant
	}

	fm.Status = "declined"
	k.SetFormationMatch(ctx, fm)

	if pe, found := k.GetPoolEntry(ctx, fm.Addr1); found {
		pe.MatchedWith = ""
		k.SetPoolEntry(ctx, pe)
	}
	if pe, found := k.GetPoolEntry(ctx, fm.Addr2); found {
		pe.MatchedWith = ""
		k.SetPoolEntry(ctx, pe)
	}

	ctx.EventManager().EmitEvent(
		sdk.NewEvent("zerone.partnerships.formation_match_declined",
			sdk.NewAttribute("match_id", fm.Id),
			sdk.NewAttribute("declined_by", msg.Decliner),
		),
	)

	return &types.MsgDeclineFormationMatchResponse{}, nil
}
