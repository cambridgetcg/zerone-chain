package keeper

import (
	"context"
	"fmt"
	"math/big"

	sdkmath "cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/zerone-chain/zerone/x/capture_challenge/types"
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

// SubmitChallenge creates a new capture challenge.
func (m msgServer) SubmitChallenge(goCtx context.Context, msg *types.MsgSubmitChallenge) (*types.MsgSubmitChallengeResponse, error) {
	ctx := sdk.UnwrapSDKContext(goCtx)

	challengerAddr, err := sdk.AccAddressFromBech32(msg.Challenger)
	if err != nil {
		return nil, fmt.Errorf("invalid challenger address: %w", err)
	}

	// Validate stake meets minimum
	stakeAmt := new(big.Int)
	if _, ok := stakeAmt.SetString(msg.Stake, 10); !ok || stakeAmt.Sign() <= 0 {
		return nil, fmt.Errorf("%w: %s", types.ErrInsufficientStake, msg.Stake)
	}

	params := m.GetParams(ctx)
	minStake := new(big.Int)
	minStake.SetString(params.MinChallengeStake, 10)
	if stakeAmt.Cmp(minStake) < 0 {
		return nil, fmt.Errorf("%w: need %s, got %s (commitment 9: cartel accusations cost something — the cost is the commitment to the claim, not the deterrent against making it)", types.ErrInsufficientStake, params.MinChallengeStake, msg.Stake)
	}

	// Check if domain is paused
	currentBlock := uint64(ctx.BlockHeight())
	if pauseUntil, found := m.GetPausedDomain(ctx, msg.Domain); found {
		if currentBlock < pauseUntil {
			return nil, fmt.Errorf("%w: domain %s paused until block %d", types.ErrDomainPaused, msg.Domain, pauseUntil)
		}
		// Pause expired, clean up
		m.DeletePausedDomain(ctx, msg.Domain)
	}

	// Escrow stake from challenger
	coins := sdk.NewCoins(sdk.NewCoin("uzrn", sdkmath.NewIntFromBigInt(stakeAmt)))
	if err := m.bankKeeper.SendCoinsFromAccountToModule(ctx, challengerAddr, types.ModuleName, coins); err != nil {
		return nil, fmt.Errorf("failed to escrow stake: %w", err)
	}

	// Generate challenge ID
	challengeID := GenerateChallengeID(msg.Challenger, msg.Domain, ctx.BlockHeight())

	// Create challenge with EVIDENCE status
	evidenceDeadline := currentBlock + params.EvidencePeriodBlocks
	reviewDeadline := evidenceDeadline + params.ReviewPeriodBlocks

	challenge := &types.CaptureChallenge{
		Id:                challengeID,
		Challenger:        msg.Challenger,
		Domain:            msg.Domain,
		AccusedValidators: msg.AccusedValidators,
		Stake:             msg.Stake,
		Status:            types.ChallengeStatus_CHALLENGE_STATUS_EVIDENCE,
		Evidence:          []*types.CaptureEvidence{},
		CreatedBlock:      currentBlock,
		EvidenceDeadline:  evidenceDeadline,
		ReviewDeadline:    reviewDeadline,
	}
	m.SetChallenge(ctx, challenge)

	// Set domain index
	m.SetDomainIndex(ctx, msg.Domain, challengeID)

	// Set domain pause
	if params.DomainPauseBlocks > 0 {
		m.SetPausedDomain(ctx, msg.Domain, currentBlock+params.DomainPauseBlocks)
	}

	ctx.EventManager().EmitEvent(
		sdk.NewEvent(
			"zerone.capture_challenge.challenge_submitted",
			sdk.NewAttribute("challenge_id", challengeID),
			sdk.NewAttribute("challenger", msg.Challenger),
			sdk.NewAttribute("domain", msg.Domain),
			sdk.NewAttribute("stake", msg.Stake),
			sdk.NewAttribute("evidence_deadline", fmt.Sprintf("%d", evidenceDeadline)),
			sdk.NewAttribute("review_deadline", fmt.Sprintf("%d", reviewDeadline)),
		),
	)

	return &types.MsgSubmitChallengeResponse{ChallengeId: challengeID}, nil
}

// AddEvidence appends evidence to an existing challenge.
func (m msgServer) AddEvidence(goCtx context.Context, msg *types.MsgAddEvidence) (*types.MsgAddEvidenceResponse, error) {
	ctx := sdk.UnwrapSDKContext(goCtx)

	challenge, found := m.GetChallenge(ctx, msg.ChallengeId)
	if !found {
		return nil, fmt.Errorf("%w: %s", types.ErrChallengeNotFound, msg.ChallengeId)
	}

	// Verify sender is the challenger
	if msg.Challenger != challenge.Challenger {
		return nil, fmt.Errorf("%w: %s", types.ErrNotChallenger, msg.Challenger)
	}

	// Verify status allows evidence
	if challenge.Status != types.ChallengeStatus_CHALLENGE_STATUS_OPEN &&
		challenge.Status != types.ChallengeStatus_CHALLENGE_STATUS_EVIDENCE {
		return nil, fmt.Errorf("%w: status is %s", types.ErrChallengeNotOpen, challenge.Status.String())
	}

	// Verify evidence deadline not passed
	currentBlock := uint64(ctx.BlockHeight())
	if currentBlock >= challenge.EvidenceDeadline {
		return nil, fmt.Errorf("%w: deadline was block %d, current is %d",
			types.ErrEvidenceDeadlinePassed, challenge.EvidenceDeadline, currentBlock)
	}

	// Append evidence entry
	evidence := &types.CaptureEvidence{
		Description:    msg.Description,
		DataHash:       msg.DataHash,
		SubmittedBlock: currentBlock,
	}
	challenge.Evidence = append(challenge.Evidence, evidence)
	m.SetChallenge(ctx, challenge)

	ctx.EventManager().EmitEvent(
		sdk.NewEvent(
			"zerone.capture_challenge.evidence_added",
			sdk.NewAttribute("challenge_id", msg.ChallengeId),
			sdk.NewAttribute("challenger", msg.Challenger),
			sdk.NewAttribute("evidence_count", fmt.Sprintf("%d", len(challenge.Evidence))),
		),
	)

	return &types.MsgAddEvidenceResponse{}, nil
}

// ResolveChallenge settles a challenge (authority-gated).
func (m msgServer) ResolveChallenge(goCtx context.Context, msg *types.MsgResolveChallenge) (*types.MsgResolveChallengeResponse, error) {
	if m.GetAuthority() != msg.Authority {
		return nil, fmt.Errorf("%w: expected %s, got %s", types.ErrUnauthorized, m.GetAuthority(), msg.Authority)
	}

	ctx := sdk.UnwrapSDKContext(goCtx)

	challenge, found := m.GetChallenge(ctx, msg.ChallengeId)
	if !found {
		return nil, fmt.Errorf("%w: %s", types.ErrChallengeNotFound, msg.ChallengeId)
	}

	if challenge.Status != types.ChallengeStatus_CHALLENGE_STATUS_UNDER_REVIEW {
		return nil, fmt.Errorf("%w: expected UNDER_REVIEW, got %s", types.ErrChallengeNotOpen, challenge.Status.String())
	}

	currentBlock := uint64(ctx.BlockHeight())
	scale := new(big.Int).SetUint64(1_000_000)
	params := m.GetParams(ctx)

	stakeAmt := new(big.Int)
	stakeAmt.SetString(challenge.Stake, 10)

	challengerAddr, err := sdk.AccAddressFromBech32(challenge.Challenger)
	if err != nil {
		return nil, fmt.Errorf("invalid challenger address: %w", err)
	}

	var rewardAmount string
	var slashAmount string

	switch msg.Outcome {
	case types.ChallengeOutcome_CHALLENGE_OUTCOME_UPHELD:
		// Reward challenger from bounty pool
		pool, poolFound := m.GetBountyPool(ctx, challenge.Domain)
		reward := new(big.Int)
		if poolFound {
			poolBal := new(big.Int)
			poolBal.SetString(pool.Balance, 10)
			// reward = pool_balance * reward_rate_bps / 1_000_000
			reward.Mul(poolBal, new(big.Int).SetUint64(params.RewardRateBps))
			reward.Div(reward, scale)
			if reward.Sign() > 0 {
				// Send reward from module to challenger
				rewardCoins := sdk.NewCoins(sdk.NewCoin("uzrn", sdkmath.NewIntFromBigInt(reward)))
				if err := m.bankKeeper.SendCoinsFromModuleToAccount(ctx, types.ModuleName, challengerAddr, rewardCoins); err != nil {
					m.Logger(ctx).Error("failed to send reward", "error", err)
					reward.SetInt64(0)
				} else {
					// Update bounty pool balance
					poolBal.Sub(poolBal, reward)
					pool.Balance = poolBal.String()
					m.SetBountyPool(ctx, pool)
				}
			}
		}
		rewardAmount = reward.String()

		// Track slash amounts for accused validators
		var slashes []*types.ValidatorSlash
		totalSlash := new(big.Int)
		for _, validator := range challenge.AccusedValidators {
			// slash_amount = reported_stake * slash_rate_bps / 1_000_000
			// Here we track the slash record; actual slashing is deferred to staking module
			slashAmt := new(big.Int).Mul(stakeAmt, new(big.Int).SetUint64(params.SlashRateBps))
			slashAmt.Div(slashAmt, scale)
			totalSlash.Add(totalSlash, slashAmt)
			slashes = append(slashes, &types.ValidatorSlash{
				Validator:   validator,
				SlashAmount: slashAmt.String(),
				Reason:      msg.Reason,
			})
		}
		challenge.Slashes = slashes
		slashAmount = totalSlash.String()

		// Return challenger stake
		if stakeAmt.Sign() > 0 {
			stakeCoins := sdk.NewCoins(sdk.NewCoin("uzrn", sdkmath.NewIntFromBigInt(stakeAmt)))
			if err := m.bankKeeper.SendCoinsFromModuleToAccount(ctx, types.ModuleName, challengerAddr, stakeCoins); err != nil {
				return nil, fmt.Errorf("failed to return stake: %w", err)
			}
		}

		// Apply capture consequences (R28-8)
		expiryHeight := uint64(ctx.BlockHeight()) + 50000 // ~35 hours temporary

		// Reduce qualification weight for accused validators
		if m.qualificationKeeper != nil {
			for _, accused := range challenge.AccusedValidators {
				if err := m.qualificationKeeper.ReduceQualificationWeight(ctx, accused, challenge.Domain, 500000, expiryHeight); err != nil {
					m.Logger(ctx).Error("failed to reduce qualification weight", "validator", accused, "err", err)
				}
			}
		}

		// Increase verification threshold for the domain temporarily
		if m.knowledgeKeeper != nil {
			if err := m.knowledgeKeeper.IncreaseVerificationThreshold(ctx, challenge.Domain, 2, expiryHeight); err != nil {
				m.Logger(ctx).Error("failed to increase verification threshold", "domain", challenge.Domain, "err", err)
			}

			// Record role impact for domain elasticity (R29-3)
			// NOTE: CaptureChallenge is domain-level (no FactId field), so challenge.Id
			// won't resolve to a fact. This is a forward-compatible placeholder — role impact
			// from capture challenges will activate when fact-level linking is added.
			if err := m.knowledgeKeeper.RecordChallengeRoleImpact(ctx, challenge.Id, challenge.Domain, true); err != nil {
				m.Logger(ctx).Error("failed to record challenge role impact", "domain", challenge.Domain, "err", err)
			}
		}

		// Cartel detection has consequence: this event announces the
		// uphold verdict that downstream consumers (qualification
		// penalty, threshold raise, stake slash) read. Without the
		// announcement, the consequence flow is invisible. See
		// TRUTH_SEEKING.md commitment 9.
		ctx.EventManager().EmitEvent(
			sdk.NewEvent(
				"zerone.capture_challenge.capture_confirmed",
				sdk.NewAttribute("domain", challenge.Domain),
				sdk.NewAttribute("challenge_id", challenge.Id),
				sdk.NewAttribute("creed_commitment", "9"),
			),
		)

	case types.ChallengeOutcome_CHALLENGE_OUTCOME_REJECTED:
		// Route rejected challenger stake to development fund
		if stakeAmt.Sign() > 0 {
			stakeCoins := sdk.NewCoins(sdk.NewCoin("uzrn", sdkmath.NewIntFromBigInt(stakeAmt)))
			if err := m.bankKeeper.SendCoinsFromModuleToModule(ctx, types.ModuleName, "development_fund", stakeCoins); err != nil {
				return nil, fmt.Errorf("failed to route stake to development fund: %w", err)
			}
		}

		// Add slashed stake to bounty pool
		pool, poolFound := m.GetBountyPool(ctx, challenge.Domain)
		if !poolFound {
			pool = &types.DomainBountyPool{
				Domain:  challenge.Domain,
				Balance: "0",
			}
		}
		poolBal := new(big.Int)
		poolBal.SetString(pool.Balance, 10)
		poolBal.Add(poolBal, stakeAmt)
		pool.Balance = poolBal.String()
		m.SetBountyPool(ctx, pool)

		rewardAmount = "0"
		slashAmount = "0"

		// Clear capture flag on rejected challenge (R28-8)
		if m.captureDefenseKeeper != nil {
			m.captureDefenseKeeper.ClearCaptureFlag(ctx, challenge.Domain)
		}

		// Record role impact — challenge rejected means original verifiers were right (R29-3)
		// NOTE: See UPHELD note — challenge.Id is not a fact ID, so this is a no-op placeholder.
		if m.knowledgeKeeper != nil {
			if err := m.knowledgeKeeper.RecordChallengeRoleImpact(ctx, challenge.Id, challenge.Domain, false); err != nil {
				m.Logger(ctx).Error("failed to record challenge role impact", "domain", challenge.Domain, "err", err)
			}
		}

	case types.ChallengeOutcome_CHALLENGE_OUTCOME_PARTIAL:
		// Return half the stake, route the other half to development fund
		halfStake := new(big.Int).Div(stakeAmt, big.NewInt(2))
		remainder := new(big.Int).Sub(stakeAmt, halfStake)

		if halfStake.Sign() > 0 {
			returnCoins := sdk.NewCoins(sdk.NewCoin("uzrn", sdkmath.NewIntFromBigInt(halfStake)))
			if err := m.bankKeeper.SendCoinsFromModuleToAccount(ctx, types.ModuleName, challengerAddr, returnCoins); err != nil {
				return nil, fmt.Errorf("failed to return partial stake: %w", err)
			}
		}
		if remainder.Sign() > 0 {
			devCoins := sdk.NewCoins(sdk.NewCoin("uzrn", sdkmath.NewIntFromBigInt(remainder)))
			if err := m.bankKeeper.SendCoinsFromModuleToModule(ctx, types.ModuleName, "development_fund", devCoins); err != nil {
				return nil, fmt.Errorf("failed to route partial stake to development fund: %w", err)
			}
		}

		// Partial reward from bounty pool
		pool, poolFound := m.GetBountyPool(ctx, challenge.Domain)
		reward := new(big.Int)
		if poolFound {
			poolBal := new(big.Int)
			poolBal.SetString(pool.Balance, 10)
			// Half of the normal reward rate
			reward.Mul(poolBal, new(big.Int).SetUint64(params.RewardRateBps))
			reward.Div(reward, scale)
			reward.Div(reward, big.NewInt(2))
			if reward.Sign() > 0 {
				rewardCoins := sdk.NewCoins(sdk.NewCoin("uzrn", sdkmath.NewIntFromBigInt(reward)))
				if err := m.bankKeeper.SendCoinsFromModuleToAccount(ctx, types.ModuleName, challengerAddr, rewardCoins); err != nil {
					m.Logger(ctx).Error("failed to send partial reward", "error", err)
					reward.SetInt64(0)
				} else {
					poolBal.Sub(poolBal, reward)
					pool.Balance = poolBal.String()
					m.SetBountyPool(ctx, pool)
				}
			}
		}
		rewardAmount = reward.String()
		slashAmount = "0"

	default:
		return nil, fmt.Errorf("%w: %s", types.ErrInvalidOutcome, msg.Outcome.String())
	}

	// Set resolution record
	challenge.Resolution = &types.CaptureResolution{
		Outcome:       msg.Outcome,
		Resolver:      msg.Authority,
		Reason:        msg.Reason,
		ResolvedBlock: currentBlock,
		RewardAmount:  rewardAmount,
		SlashAmount:   slashAmount,
	}
	challenge.Status = types.ChallengeStatus_CHALLENGE_STATUS_RESOLVED
	m.SetChallenge(ctx, challenge)

	ctx.EventManager().EmitEvent(
		sdk.NewEvent(
			"zerone.capture_challenge.challenge_resolved",
			sdk.NewAttribute("challenge_id", msg.ChallengeId),
			sdk.NewAttribute("outcome", msg.Outcome.String()),
			sdk.NewAttribute("reward_amount", rewardAmount),
			sdk.NewAttribute("slash_amount", slashAmount),
		),
	)

	return &types.MsgResolveChallengeResponse{}, nil
}

// FundBountyPool adds tokens to a domain's bounty pool.
func (m msgServer) FundBountyPool(goCtx context.Context, msg *types.MsgFundBountyPool) (*types.MsgFundBountyPoolResponse, error) {
	ctx := sdk.UnwrapSDKContext(goCtx)

	senderAddr, err := sdk.AccAddressFromBech32(msg.Sender)
	if err != nil {
		return nil, fmt.Errorf("invalid sender address: %w", err)
	}

	amount := new(big.Int)
	if _, ok := amount.SetString(msg.Amount, 10); !ok || amount.Sign() <= 0 {
		return nil, fmt.Errorf("invalid amount: %s", msg.Amount)
	}

	// Transfer tokens to module
	coins := sdk.NewCoins(sdk.NewCoin("uzrn", sdkmath.NewIntFromBigInt(amount)))
	if err := m.bankKeeper.SendCoinsFromAccountToModule(ctx, senderAddr, types.ModuleName, coins); err != nil {
		return nil, fmt.Errorf("failed to fund bounty pool: %w", err)
	}

	// Get or create bounty pool
	pool, found := m.GetBountyPool(ctx, msg.Domain)
	if !found {
		pool = &types.DomainBountyPool{
			Domain:  msg.Domain,
			Balance: "0",
		}
	}

	// Add amount to balance
	balance := new(big.Int)
	balance.SetString(pool.Balance, 10)
	balance.Add(balance, amount)
	pool.Balance = balance.String()
	m.SetBountyPool(ctx, pool)

	ctx.EventManager().EmitEvent(
		sdk.NewEvent(
			"zerone.capture_challenge.bounty_pool_funded",
			sdk.NewAttribute("domain", msg.Domain),
			sdk.NewAttribute("sender", msg.Sender),
			sdk.NewAttribute("amount", msg.Amount),
			sdk.NewAttribute("new_balance", pool.Balance),
		),
	)

	return &types.MsgFundBountyPoolResponse{}, nil
}

// UpdateParams handles MsgUpdateParams — governance-gated parameter update.
func (m msgServer) UpdateParams(goCtx context.Context, msg *types.MsgUpdateParams) (*types.MsgUpdateParamsResponse, error) {
	if m.GetAuthority() != msg.Authority {
		return nil, fmt.Errorf("unauthorized: expected %s, got %s", m.GetAuthority(), msg.Authority)
	}

	ctx := sdk.UnwrapSDKContext(goCtx)
	if msg.Params == nil {
		return nil, fmt.Errorf("params cannot be nil")
	}
	if err := msg.Params.Validate(); err != nil {
		return nil, fmt.Errorf("invalid params: %w", err)
	}
	m.SetParams(ctx, msg.Params)

	ctx.EventManager().EmitEvent(
		sdk.NewEvent(
			"zerone.capture_challenge.params_updated",
			sdk.NewAttribute("authority", msg.Authority),
		),
	)

	return &types.MsgUpdateParamsResponse{}, nil
}
