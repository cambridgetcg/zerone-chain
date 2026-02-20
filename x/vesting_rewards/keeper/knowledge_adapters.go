package keeper

import (
	"context"

	sdk "github.com/cosmos/cosmos-sdk/types"

	knowledgetypes "github.com/zerone-chain/zerone/x/knowledge/types"
)

// VestingRewardsKeeperAdapter wraps the vesting_rewards Keeper to satisfy the
// knowledge module's VestingRewardsKeeper interface by bridging context.Context → sdk.Context.
type VestingRewardsKeeperAdapter struct {
	k Keeper
}

// NewVestingRewardsKeeperAdapter returns an adapter that bridges the vesting rewards keeper
// to the knowledge module's expected interface.
func NewVestingRewardsKeeperAdapter(k Keeper) *VestingRewardsKeeperAdapter {
	return &VestingRewardsKeeperAdapter{k: k}
}

// Ensure compile-time interface compliance.
var _ knowledgetypes.VestingRewardsKeeper = (*VestingRewardsKeeperAdapter)(nil)

func (a *VestingRewardsKeeperAdapter) CreateVestingScheduleFromKnowledge(ctx context.Context, claimID, factID, recipient, totalAmount, epistemicCategory string) error {
	return a.k.CreateVestingScheduleFromKnowledge(sdk.UnwrapSDKContext(ctx), claimID, factID, recipient, totalAmount, epistemicCategory)
}

func (a *VestingRewardsKeeperAdapter) DistributeFalsificationReward(ctx context.Context, counterFactID, targetFactID, recipient, amount string) error {
	return a.k.DistributeFalsificationReward(sdk.UnwrapSDKContext(ctx), counterFactID, targetFactID, recipient, amount)
}

func (a *VestingRewardsKeeperAdapter) GetEpochBlockRewardPool(ctx context.Context, epoch uint64) uint64 {
	return a.k.GetEpochBlockRewardPool(sdk.UnwrapSDKContext(ctx), epoch)
}

func (a *VestingRewardsKeeperAdapter) PauseVestingByClaimId(ctx context.Context, claimID string) error {
	return a.k.PauseVestingByClaimId(sdk.UnwrapSDKContext(ctx), claimID)
}

func (a *VestingRewardsKeeperAdapter) PauseAllVestingByRecipient(ctx context.Context, recipient string) int {
	return a.k.PauseAllVestingByRecipient(sdk.UnwrapSDKContext(ctx), recipient)
}

func (a *VestingRewardsKeeperAdapter) DepositToResearchFund(ctx context.Context, sourceModule string, amount sdk.Coins) error {
	return a.k.DepositToResearchFund(sdk.UnwrapSDKContext(ctx), sourceModule, amount)
}
