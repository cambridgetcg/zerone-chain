package keeper

import (
	"context"
	"fmt"
	"math/big"

	sdkmath "cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/zerone-chain/zerone/x/claiming_pot/types"
)

type msgServer struct {
	types.UnimplementedMsgServer
	Keeper
}

func NewMsgServerImpl(keeper Keeper) types.MsgServer {
	return &msgServer{Keeper: keeper}
}

var _ types.MsgServer = msgServer{}

// CreatePot creates a new claiming pot (authority-gated).
func (m msgServer) CreatePot(goCtx context.Context, msg *types.MsgCreatePot) (*types.MsgCreatePotResponse, error) {
	if m.GetAuthority() != msg.Authority {
		return nil, fmt.Errorf("%w: expected %s, got %s", types.ErrUnauthorized, m.GetAuthority(), msg.Authority)
	}

	ctx := sdk.UnwrapSDKContext(goCtx)

	// Check max active pots
	params := m.GetParams(ctx)
	if uint32(m.CountActivePots(ctx)) >= params.MaxPotsActive {
		return nil, types.ErrMaxPotsReached
	}

	// Validate total amount
	totalAmount := new(big.Int)
	if _, ok := totalAmount.SetString(msg.TotalAmount, 10); !ok || totalAmount.Sign() <= 0 {
		return nil, fmt.Errorf("%w: invalid total_amount: %s", types.ErrInvalidConfig, msg.TotalAmount)
	}

	currentBlock := uint64(ctx.BlockHeight())
	potID := fmt.Sprintf("pot-%d", m.GetNextPotID(ctx))

	pot := &types.ClaimingPot{
		Id:             potID,
		Name:           msg.Name,
		TotalAmount:    msg.TotalAmount,
		ClaimedAmount:  "0",
		Schedule:       msg.Schedule,
		Eligibility:    msg.Eligibility,
		CreatedAtBlock: currentBlock,
		Status:         types.PotStatus_POT_STATUS_ACTIVE,
	}

	m.SetPot(ctx, pot)

	ctx.EventManager().EmitEvent(
		sdk.NewEvent(
			"zerone.claiming_pot.pot_created",
			sdk.NewAttribute("pot_id", potID),
			sdk.NewAttribute("name", msg.Name),
			sdk.NewAttribute("total_amount", msg.TotalAmount),
		),
	)

	return &types.MsgCreatePotResponse{PotId: potID}, nil
}

// Claim processes a claim from an eligible claimant.
func (m msgServer) Claim(goCtx context.Context, msg *types.MsgClaim) (*types.MsgClaimResponse, error) {
	ctx := sdk.UnwrapSDKContext(goCtx)

	pot, found := m.GetPot(ctx, msg.PotId)
	if !found {
		return nil, fmt.Errorf("%w: %s", types.ErrPotNotFound, msg.PotId)
	}

	if pot.Status != types.PotStatus_POT_STATUS_ACTIVE {
		return nil, fmt.Errorf("%w: pot status is %s", types.ErrPotNotActive, pot.Status.String())
	}

	// Check eligibility
	if err := m.checkEligibility(ctx, msg.Claimant, pot); err != nil {
		return nil, err
	}

	// Check not already claimed
	if _, exists := m.GetClaim(ctx, msg.PotId, msg.Claimant); exists {
		return nil, types.ErrAlreadyClaimed
	}

	// Calculate vested-but-unclaimed
	currentBlock := uint64(ctx.BlockHeight())
	claimable := CalculateClaimable(pot, currentBlock)
	if claimable.Sign() <= 0 {
		return nil, types.ErrCliffNotReached
	}

	// Check min claim amount
	params := m.GetParams(ctx)
	minClaim := new(big.Int)
	minClaim.SetString(params.MinClaimAmount, 10)
	if claimable.Cmp(minClaim) < 0 {
		return nil, fmt.Errorf("%w: claimable %s < min %s", types.ErrBelowMinClaim, claimable.String(), params.MinClaimAmount)
	}

	// Check pot has sufficient remaining funds
	totalAmount := new(big.Int)
	totalAmount.SetString(pot.TotalAmount, 10)
	claimedAmount := new(big.Int)
	claimedAmount.SetString(pot.ClaimedAmount, 10)
	remaining := new(big.Int).Sub(totalAmount, claimedAmount)
	if claimable.Cmp(remaining) > 0 {
		claimable = remaining
	}
	if claimable.Sign() <= 0 {
		return nil, types.ErrInsufficientPotFunds
	}

	// Mint into the claiming_pot module account through the chain's
	// single cap-gated mint entry point, then forward to the claimer
	// in the same transaction. The module account is a transient
	// conduit — it never holds funds across blocks.
	//
	// Doctrine (commitment 20, issuance follows participation): the
	// chain mints when an agent participates (this MsgClaim) — not
	// from a pre-funded module account.
	claimantAddr, err := sdk.AccAddressFromBech32(msg.Claimant)
	if err != nil {
		return nil, fmt.Errorf("invalid claimant address: %w", err)
	}

	sdkCtx := sdk.UnwrapSDKContext(ctx)
	actualMinted, err := m.vestingRewardsKeeper.MintWithCap(sdkCtx, types.ModuleName, claimable)
	if err != nil {
		return nil, fmt.Errorf("mint with cap: %w", err)
	}
	if actualMinted.Sign() <= 0 {
		return nil, types.ErrCapReached
	}
	// MintWithCap may have clipped to remaining cap headroom; honour
	// that clip rather than over-promising the claimer.
	if actualMinted.Cmp(claimable) < 0 {
		claimable = actualMinted
	}

	coins := sdk.NewCoins(sdk.NewCoin("uzrn", sdkmath.NewIntFromBigInt(claimable)))
	if err := m.bankKeeper.SendCoinsFromModuleToAccount(ctx, types.ModuleName, claimantAddr, coins); err != nil {
		return nil, fmt.Errorf("forward minted bootstrap claim to claimer: %w", err)
	}

	// Record claim
	claim := &types.Claim{
		PotId:     msg.PotId,
		Claimant:  msg.Claimant,
		Amount:    claimable.String(),
		ClaimedAt: currentBlock,
	}
	m.SetClaim(ctx, claim)

	// Update pot claimed amount
	claimedAmount.Add(claimedAmount, claimable)
	pot.ClaimedAmount = claimedAmount.String()
	if claimedAmount.Cmp(totalAmount) >= 0 {
		pot.Status = types.PotStatus_POT_STATUS_DEPLETED
	}
	m.SetPot(ctx, pot)

	ctx.EventManager().EmitEvent(
		sdk.NewEvent(
			"zerone.claiming_pot.pot_claimed",
			sdk.NewAttribute("pot_id", msg.PotId),
			sdk.NewAttribute("claimant", msg.Claimant),
			sdk.NewAttribute("amount", claimable.String()),
			// Commitment 20: issuance follows participation. Every
			// uzrn this event announces was minted on demand through
			// MintWithCap when the agent called MsgClaim — never
			// pre-funded.
			sdk.NewAttribute("creed_commitment", "20"),
		),
	)

	return &types.MsgClaimResponse{Amount: claimable.String()}, nil
}

// UpdatePotParams updates module parameters (authority-gated).
func (m msgServer) UpdatePotParams(goCtx context.Context, msg *types.MsgUpdatePotParams) (*types.MsgUpdatePotParamsResponse, error) {
	if m.GetAuthority() != msg.Authority {
		return nil, fmt.Errorf("%w: expected %s, got %s", types.ErrUnauthorized, m.GetAuthority(), msg.Authority)
	}

	ctx := sdk.UnwrapSDKContext(goCtx)
	m.SetParams(ctx, msg.Params)

	ctx.EventManager().EmitEvent(
		sdk.NewEvent("zerone.claiming_pot.update_pot_params",
			sdk.NewAttribute("authority", msg.Authority),
		),
	)

	return &types.MsgUpdatePotParamsResponse{}, nil
}

// checkEligibility verifies the claimant meets the pot's eligibility criteria.
func (m msgServer) checkEligibility(ctx context.Context, claimant string, pot *types.ClaimingPot) error {
	if pot.Eligibility == nil {
		return nil // no criteria = everyone eligible
	}

	elig := pot.Eligibility

	// Check whitelist (if non-empty, must be in it)
	if len(elig.Whitelist) > 0 {
		found := false
		for _, addr := range elig.Whitelist {
			if addr == claimant {
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("%w: address not in whitelist", types.ErrIneligible)
		}
	}

	// Check staking tier
	if elig.MinStakingTier > 0 {
		tier, err := m.stakingKeeper.GetValidatorTier(ctx, claimant)
		if err != nil {
			return fmt.Errorf("%w: cannot determine tier: %v", types.ErrIneligible, err)
		}
		if tier < elig.MinStakingTier {
			return fmt.Errorf("%w: tier %d < required %d", types.ErrIneligible, tier, elig.MinStakingTier)
		}
	}

	// Check registration age
	if elig.MinRegistrationAge > 0 {
		regBlock, err := m.authKeeper.GetRegistrationBlock(ctx, claimant)
		if err != nil {
			return fmt.Errorf("%w: cannot determine registration age: %v", types.ErrIneligible, err)
		}
		sdkCtx := sdk.UnwrapSDKContext(ctx)
		currentBlock := uint64(sdkCtx.BlockHeight())
		if regBlock == 0 || currentBlock-regBlock < elig.MinRegistrationAge {
			return fmt.Errorf("%w: registration age insufficient", types.ErrIneligible)
		}
	}

	return nil
}
