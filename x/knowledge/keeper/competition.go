package keeper

import (
	"context"
	"fmt"
	"sort"

	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/zerone-chain/zerone/x/knowledge/types"
)

// ProcessCompetition runs niche-based competition for all active niches.
// For each niche with >1 fact, it ranks by fitness, assigns competition tax
// proportional to the fitness gap with the leader, and force-prunes excess facts.
func (k Keeper) ProcessCompetition(ctx context.Context, epoch uint64) error {
	params, _ := k.GetParams(ctx)

	// Collect all unique niches
	niches := k.GetAllNiches(ctx)

	for _, nicheKey := range niches {
		members := k.GetNicheMembers(ctx, nicheKey)
		if len(members) <= 1 {
			// Sole occupant — no competition
			if len(members) == 1 {
				members[0].NicheLeader = true
				members[0].NicheRank = 1
				members[0].NicheSize = 1
				members[0].CompetitionTax = 0
				_ = k.SetFact(ctx, members[0])
			}
			continue
		}

		// Sort by fitness descending
		sort.Slice(members, func(i, j int) bool {
			return members[i].FitnessScore > members[j].FitnessScore
		})

		leader := members[0]
		leaderFitness := leader.FitnessScore
		if leaderFitness == 0 {
			leaderFitness = 1 // avoid division by zero
		}

		for rank, fact := range members {
			fact.NicheRank = uint64(rank + 1)
			fact.NicheSize = uint64(len(members))
			fact.NicheLeader = (rank == 0)

			if rank == 0 {
				// Leader gets dominance bonus (applied in fitness calculation)
				fact.CompetitionTax = 0
			} else {
				// Competition tax: proportional to fitness gap
				fitnessRatio := safeMulDiv(fact.FitnessScore, 1_000_000, leaderFitness)
				gap := uint64(0)
				if fitnessRatio < 1_000_000 {
					gap = 1_000_000 - fitnessRatio
				}
				fact.CompetitionTax = safeMulDiv(params.MetabolismBaseCost, gap, 1_000_000)
			}

			// Check redundancy threshold
			if !fact.NicheLeader {
				ratio := safeMulDiv(fact.FitnessScore, 1_000_000, leaderFitness)
				if ratio < params.CompetitionRedundancyThresholdBps {
					// Mark as redundant — accelerated decay
					fact.CompetitionTax *= 3 // Triple maintenance for redundant facts
				}
			}

			_ = k.SetFact(ctx, fact)
		}

		// Forced pruning if niche exceeds max size
		if params.CompetitionMaxNicheSize > 0 && uint64(len(members)) > params.CompetitionMaxNicheSize {
			// Prune weakest facts beyond max size
			for i := int(params.CompetitionMaxNicheSize); i < len(members); i++ {
				members[i].Status = types.FactStatus_FACT_STATUS_PRUNED
				members[i].Energy = 0
				_ = k.SetFact(ctx, members[i])

				k.emitNichePruneEvent(ctx, members[i], leader)
			}
		}
	}

	return nil
}

// ProcessSymbiosis applies fitness bonuses for SUPPORTS relationships.
// Facts that SUPPORT healthy facts get a fitness boost, creating stable knowledge clusters.
func (k Keeper) ProcessSymbiosis(ctx context.Context, params *types.Params) {
	// Collect facts to process (avoid modifying store during iteration)
	var factsToProcess []*types.Fact
	k.IterateFacts(ctx, func(fact *types.Fact) bool {
		if fact.Status == types.FactStatus_FACT_STATUS_VERIFIED ||
			fact.Status == types.FactStatus_FACT_STATUS_ACTIVE ||
			fact.Status == types.FactStatus_FACT_STATUS_PROVISIONAL {
			factsToProcess = append(factsToProcess, fact)
		}
		return false
	})

	for _, fact := range factsToProcess {
		relations, err := k.GetRelationsByType(ctx, fact.Id, types.RelationType_RELATION_TYPE_SUPPORTS)
		if err != nil || len(relations) == 0 {
			continue
		}
		symbiosisBonus := uint64(0)
		for _, rel := range relations {
			targetFact, found := k.GetFact(ctx, rel.TargetFactId)
			if found && targetFact.FitnessScore > 500_000 {
				symbiosisBonus += params.CompetitionSymbiosisBonusBps
			}
		}
		if symbiosisBonus > 0 {
			// Add bonus to fitness (capped)
			fact.FitnessScore += safeMulDiv(symbiosisBonus, fact.FitnessScore, 1_000_000)
			if fact.FitnessScore > 1_000_000 {
				fact.FitnessScore = 1_000_000
			}
			_ = k.SetFact(ctx, fact)
		}
	}
}

// emitNichePruneEvent emits an event when a fact is pruned from a niche.
func (k Keeper) emitNichePruneEvent(ctx context.Context, pruned *types.Fact, leader *types.Fact) {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	sdkCtx.EventManager().EmitEvent(sdk.NewEvent(
		"zerone.knowledge.niche_pruned",
		sdk.NewAttribute("pruned_fact_id", pruned.Id),
		sdk.NewAttribute("niche_leader_id", leader.Id),
		sdk.NewAttribute("niche_key", pruned.NicheKey),
		sdk.NewAttribute("fitness", fmt.Sprintf("%d", pruned.FitnessScore)),
	))
}

// emitNicheDisplacementEvent emits an event when a new fact displaces the niche leader.
func (k Keeper) emitNicheDisplacementEvent(ctx context.Context, newLeader *types.Fact, oldLeader *types.Fact, nicheKey string) {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	sdkCtx.EventManager().EmitEvent(sdk.NewEvent(
		"zerone.knowledge.niche_displacement",
		sdk.NewAttribute("new_leader", newLeader.Id),
		sdk.NewAttribute("displaced_fact", oldLeader.Id),
		sdk.NewAttribute("niche_key", nicheKey),
		sdk.NewAttribute("domain", newLeader.Domain),
	))
}
