package cross_stack_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	treekeeper "github.com/zerone-chain/zerone/x/tree/keeper"
	treetypes "github.com/zerone-chain/zerone/x/tree/types"
)

// TestScenario4_TreeRevenueDistribution verifies the tree module's revenue
// calculation splits correctly across contributors, treasury, research, and development.
func TestScenario4_TreeRevenueDistribution(t *testing.T) {
	// CalculateRevenue is a pure function — no harness needed for the core math.
	// We still create a harness to verify the keeper is wired correctly.
	h := NewTestHarness(t)
	_ = h // proves tree keeper is accessible

	total := int64(1_000_000)

	// Standard Zerone revenue split:
	//   55% contributors, 22% treasury, 3.33% research, 19.67% development
	contributorsBp := uint32(550_000) // 55%
	treasuryBp := uint32(220_000)     // 22%
	researchBp := uint32(33_300)      // 3.33%
	developmentBp := uint32(196_700)  // 19.67%

	contributors := []*treetypes.ContributorRecord{
		{
			Did:            "did:zrn:contributor_a",
			Role:           "developer",
			JoinedAtBlock:  1,
			TasksCompleted: 7,
			TotalEarned:    "0",
		},
		{
			Did:            "did:zrn:contributor_b",
			Role:           "reviewer",
			JoinedAtBlock:  1,
			TasksCompleted: 3,
			TotalEarned:    "0",
		},
	}

	dist := treekeeper.CalculateRevenue(total, contributorsBp, treasuryBp, researchBp, developmentBp, contributors)

	// --- Verify top-level splits ---

	// ContributorPool: 1,000,000 * 550,000 / 1,000,000 = 550,000
	require.Equal(t, int64(550_000), dist.ContributorPool, "contributor pool")

	// ResearchFund: 1,000,000 * 33,300 / 1,000,000 = 33,300
	require.Equal(t, int64(33_300), dist.ResearchFund, "research fund")

	// DevelopmentFund: 1,000,000 * 196,700 / 1,000,000 = 196,700
	require.Equal(t, int64(196_700), dist.DevelopmentFund, "development fund")

	// Protocol allocation = total - contributors - research - development
	// = 1,000,000 - 550,000 - 33,300 - 196,700 = 220,000
	protocolAllocation := total - dist.ContributorPool - dist.ResearchFund - dist.DevelopmentFund
	require.Equal(t, int64(220_000), protocolAllocation, "protocol allocation")

	// VerificationPool = protocolAllocation * 300,000 / 1,000,000 = 66,000
	require.Equal(t, int64(66_000), dist.VerificationPool, "verification pool (30% of treasury)")

	// ProtocolTreasury = protocolAllocation - VerificationPool = 154,000
	require.Equal(t, int64(154_000), dist.ProtocolTreasury, "protocol treasury")

	// --- Verify conservation: all parts sum to total ---
	totalDistributed := dist.ContributorPool + dist.ResearchFund + dist.ProtocolTreasury + dist.VerificationPool + dist.DevelopmentFund
	require.Equal(t, total, totalDistributed, "total must be conserved")

	// --- Verify contributor shares (weighted by TasksCompleted) ---
	require.Len(t, dist.ContributorShares, 2, "must have 2 contributor shares")

	// Contributors are sorted by DID. "did:zrn:contributor_a" < "did:zrn:contributor_b"
	// Total tasks: 7 + 3 = 10
	// Contributor A (7 tasks): 550,000 * 7 / 10 = 385,000
	// Contributor B (3 tasks): 550,000 - 385,000 = 165,000 (last gets remainder)
	shareA := dist.ContributorShares[0]
	shareB := dist.ContributorShares[1]
	require.Equal(t, "did:zrn:contributor_a", shareA.Address)
	require.Equal(t, "did:zrn:contributor_b", shareB.Address)
	require.Equal(t, int64(385_000), shareA.Amount, "contributor A share (70%)")
	require.Equal(t, int64(165_000), shareB.Amount, "contributor B share (30%)")
	require.Equal(t, dist.ContributorPool, shareA.Amount+shareB.Amount,
		"contributor shares must sum to contributor pool")
}

// TestScenario4_TreeRevenueEqualSplit verifies equal distribution when
// all contributors have zero tasks completed.
func TestScenario4_TreeRevenueEqualSplit(t *testing.T) {
	total := int64(1_000_000)

	contributors := []*treetypes.ContributorRecord{
		{Did: "did:zrn:alice", TasksCompleted: 0},
		{Did: "did:zrn:bob", TasksCompleted: 0},
	}

	dist := treekeeper.CalculateRevenue(total, 550_000, 220_000, 33_300, 196_700, contributors)

	// With zero tasks, shares are distributed equally.
	require.Len(t, dist.ContributorShares, 2)
	// 550,000 / 2 = 275,000 each
	for _, share := range dist.ContributorShares {
		require.Equal(t, int64(275_000), share.Amount, "equal split for %s", share.Address)
	}
}

// TestScenario4_TreeRevenueNoContributors verifies that with no contributors,
// the contributor pool is redirected to the protocol treasury.
func TestScenario4_TreeRevenueNoContributors(t *testing.T) {
	total := int64(1_000_000)

	dist := treekeeper.CalculateRevenue(total, 550_000, 220_000, 33_300, 196_700, nil)

	require.Equal(t, int64(0), dist.ContributorPool, "no contributor pool when no contributors")
	require.Empty(t, dist.ContributorShares)

	// Protocol gets the contributor share redirected.
	// protocolAllocation = 1,000,000 - 550,000 - 33,300 - 196,700 = 220,000
	// verification = 220,000 * 300,000 / 1,000,000 = 66,000
	// treasury = 220,000 - 66,000 = 154,000
	// Then: treasury += contributorPool (550,000) → 154,000 + 550,000 = 704,000
	// And contributorPool = 0
	require.Equal(t, int64(704_000), dist.ProtocolTreasury)

	// Conservation still holds.
	totalDistributed := dist.ContributorPool + dist.ResearchFund + dist.ProtocolTreasury + dist.VerificationPool + dist.DevelopmentFund
	require.Equal(t, total, totalDistributed, "total must be conserved")
}
