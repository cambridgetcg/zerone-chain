package keeper

import (
	"context"

	vestingrewardstypes "github.com/zerone-chain/zerone/x/vesting_rewards/types"
)

// VestingRewardsKnowledgeAdapter wraps the knowledge Keeper to satisfy
// vesting_rewards/types.KnowledgeKeeper. This allows block rewards to be
// coupled to verification throughput (T9 / thesis claim 1).
type VestingRewardsKnowledgeAdapter struct {
	alignmentAdapter *AlignmentKnowledgeAdapter
}

// NewVestingRewardsKnowledgeAdapter returns an adapter for vesting_rewards.
func NewVestingRewardsKnowledgeAdapter(k Keeper) *VestingRewardsKnowledgeAdapter {
	return &VestingRewardsKnowledgeAdapter{alignmentAdapter: NewAlignmentKnowledgeAdapter(k)}
}

// Ensure compile-time interface compliance.
var _ vestingrewardstypes.KnowledgeKeeper = (*VestingRewardsKnowledgeAdapter)(nil)

// GetVerificationRate delegates to the shared accepted-over-terminal calculation.
func (a *VestingRewardsKnowledgeAdapter) GetVerificationRate(ctx context.Context) uint64 {
	return a.alignmentAdapter.GetVerificationRate(ctx)
}
