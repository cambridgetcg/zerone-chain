package keeper

import (
	"sort"

	"github.com/zerone-chain/zerone/x/tree/types"
)

const BpDenominator = int64(1000000)
const VerificationPoolShareOfTreasuryBps = int64(300000)
const KnowledgeModuleName = "knowledge"

type RevenueDistribution struct {
	ContributorPool  int64
	ResearchFund     int64
	ProtocolTreasury int64
	VerificationPool int64
	DevelopmentFund  int64

	ContributorShares []ContributorShare
}

type ContributorShare struct {
	Address string
	Amount  int64
}

func CalculateRevenue(
	total int64,
	contributorsBp, treasuryBp, researchBp, developmentBp uint32,
	contributors []*types.ContributorRecord,
) RevenueDistribution {
	if total <= 0 {
		return RevenueDistribution{}
	}

	dist := RevenueDistribution{}
	dist.ContributorPool = total * int64(contributorsBp) / BpDenominator
	dist.ResearchFund = total * int64(researchBp) / BpDenominator
	dist.DevelopmentFund = total * int64(developmentBp) / BpDenominator
	protocolAllocation := total - dist.ContributorPool - dist.ResearchFund - dist.DevelopmentFund
	dist.VerificationPool = protocolAllocation * VerificationPoolShareOfTreasuryBps / BpDenominator
	dist.ProtocolTreasury = protocolAllocation - dist.VerificationPool

	if len(contributors) == 0 {
		dist.ProtocolTreasury += dist.ContributorPool
		dist.ContributorPool = 0
		return dist
	}

	sorted := make([]*types.ContributorRecord, len(contributors))
	copy(sorted, contributors)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Did < sorted[j].Did
	})

	var totalTasks uint32
	for _, c := range sorted {
		totalTasks += c.TasksCompleted
	}

	if totalTasks == 0 {
		perContributor := dist.ContributorPool / int64(len(sorted))
		var distributed int64
		for i, c := range sorted {
			amt := perContributor
			if i == len(sorted)-1 {
				amt = dist.ContributorPool - distributed
			}
			dist.ContributorShares = append(dist.ContributorShares, ContributorShare{
				Address: c.Did,
				Amount:  amt,
			})
			distributed += amt
		}
	} else {
		var distributed int64
		for i, c := range sorted {
			var amt int64
			if i == len(sorted)-1 {
				amt = dist.ContributorPool - distributed
			} else {
				amt = dist.ContributorPool * int64(c.TasksCompleted) / int64(totalTasks)
			}
			dist.ContributorShares = append(dist.ContributorShares, ContributorShare{
				Address: c.Did,
				Amount:  amt,
			})
			distributed += amt
		}
	}

	return dist
}
