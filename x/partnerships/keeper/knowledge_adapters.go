package keeper

import (
	"context"
	"fmt"
	"math/big"

	sdkmath "cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"

	knowledgetypes "github.com/zerone-chain/zerone/x/knowledge/types"
	"github.com/zerone-chain/zerone/x/partnerships/types"
)

// KnowledgePartnershipAdapter wraps the partnership Keeper to satisfy the
// knowledge module's PartnershipKeeper interface.
type KnowledgePartnershipAdapter struct {
	k Keeper
}

// NewKnowledgePartnershipAdapter returns an adapter that bridges the partnership keeper
// to the knowledge module's PartnershipKeeper interface.
func NewKnowledgePartnershipAdapter(k Keeper) *KnowledgePartnershipAdapter {
	return &KnowledgePartnershipAdapter{k: k}
}

// Ensure compile-time interface compliance.
var _ knowledgetypes.PartnershipKeeper = (*KnowledgePartnershipAdapter)(nil)

func (a *KnowledgePartnershipAdapter) IsActive(ctx context.Context, partnershipId string) (bool, error) {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	p, found := a.k.GetPartnership(sdkCtx, partnershipId)
	if !found {
		return false, fmt.Errorf("partnership %s not found", partnershipId)
	}
	return p.Status == types.StatusActive, nil
}

func (a *KnowledgePartnershipAdapter) IsParticipant(ctx context.Context, partnershipId string, address string) (bool, error) {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	p, found := a.k.GetPartnership(sdkCtx, partnershipId)
	if !found {
		return false, fmt.Errorf("partnership %s not found", partnershipId)
	}
	return p.HumanAddr == address || p.AgentAddr == address, nil
}

func (a *KnowledgePartnershipAdapter) IsSuspended(ctx context.Context, partnershipId string) (bool, error) {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	p, found := a.k.GetPartnership(sdkCtx, partnershipId)
	if !found {
		return false, fmt.Errorf("partnership %s not found", partnershipId)
	}
	return p.Status == types.StatusSuspended, nil
}

func (a *KnowledgePartnershipAdapter) DistributeReward(ctx context.Context, partnershipId string, amount sdk.Coins, source string) error {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	p, found := a.k.GetPartnership(sdkCtx, partnershipId)
	if !found {
		return fmt.Errorf("partnership %s not found", partnershipId)
	}
	if p.Status != types.StatusActive {
		return fmt.Errorf("partnership %s is not active (status: %s)", partnershipId, p.Status)
	}

	// Extract uzrn amount as string for the existing DistributeReward
	uzrnAmount := amount.AmountOf("uzrn")
	if uzrnAmount.IsZero() {
		return nil
	}
	grossAmount := uzrnAmount.String()

	// Calculate splits via existing logic (lock multiplier, common pot, human/agent split)
	humanShare, agentShare, _, err := a.k.DistributeReward(sdkCtx, p, grossAmount)
	if err != nil {
		return fmt.Errorf("partnership reward split failed: %w", err)
	}

	// Send human share from knowledge module to human address
	humanAmt, ok := new(big.Int).SetString(humanShare, 10)
	if ok && humanAmt.Sign() > 0 {
		humanAddr, err := sdk.AccAddressFromBech32(p.HumanAddr)
		if err != nil {
			return fmt.Errorf("invalid human address: %w", err)
		}
		humanCoins := sdk.NewCoins(sdk.NewCoin("uzrn", sdkmath.NewIntFromBigInt(humanAmt)))
		if err := a.k.bankKeeper.SendCoinsFromModuleToAccount(ctx, knowledgetypes.ModuleName, humanAddr, humanCoins); err != nil {
			return fmt.Errorf("failed to send human share: %w", err)
		}
	}

	// Send agent share from knowledge module to agent address
	agentAmt, ok := new(big.Int).SetString(agentShare, 10)
	if ok && agentAmt.Sign() > 0 {
		agentAddr, err := sdk.AccAddressFromBech32(p.AgentAddr)
		if err != nil {
			return fmt.Errorf("invalid agent address: %w", err)
		}
		agentCoins := sdk.NewCoins(sdk.NewCoin("uzrn", sdkmath.NewIntFromBigInt(agentAmt)))
		if err := a.k.bankKeeper.SendCoinsFromModuleToAccount(ctx, knowledgetypes.ModuleName, agentAddr, agentCoins); err != nil {
			return fmt.Errorf("failed to send agent share: %w", err)
		}
	}

	// Emit event for tracking
	sdkCtx.EventManager().EmitEvent(sdk.NewEvent(
		"zerone.partnerships.reward_distributed",
		sdk.NewAttribute("partnership_id", partnershipId),
		sdk.NewAttribute("source", source),
		sdk.NewAttribute("gross_amount", grossAmount),
		sdk.NewAttribute("human_share", humanShare),
		sdk.NewAttribute("agent_share", agentShare),
	))

	return nil
}
