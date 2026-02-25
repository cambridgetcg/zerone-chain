package keeper

import (
	"context"
	"fmt"
	"sort"

	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/zerone-chain/zerone/x/knowledge/types"
)

// CalculateFitness computes the fitness score for a single fact.
// Returns a value in 0-1,000,000 BPS.
func (k Keeper) CalculateFitness(ctx context.Context, fact *types.Fact, epoch uint64) uint64 {
	params, _ := k.GetParams(ctx)

	// ─── Query rate component ──────────────────────────────
	// Normalize: queries per epoch, capped at 1000 for scaling
	queryRate := min64(fact.QueryCountEpoch, 1000)
	queryScore := safeMulDiv(queryRate, 1_000_000, 1000) // 0-1M based on queries

	// ─── Citation rate component ───────────────────────────
	// 10 incoming citations = max score
	citationScore := min64(fact.IncomingCitationCount*100_000, 1_000_000)

	// ─── Bridge score component ────────────────────────────
	// Already on Fact proto (0-1,000,000)
	bridgeScore := fact.BridgeScore

	// ─── Dependency depth component ────────────────────────
	// How many facts transitively depend on this fact
	depthCount := k.CountTransitiveDependents(ctx, fact.Id, 5) // max depth 5
	depthScore := min64(depthCount*200_000, 1_000_000)         // 5 dependents = max

	// ─── Patronage component ───────────────────────────────
	patronScore := uint64(0)
	if fact.PatronageAmount != "" && fact.PatronageAmount != "0" {
		sdkCtx := sdk.UnwrapSDKContext(ctx)
		if uint64(sdkCtx.BlockHeight()) < fact.PatronageExpiryBlock {
			patronScore = 1_000_000 // Binary: active patronage = full score
		}
	}

	// ─── Uniqueness component ──────────────────────────────
	// Powered by the novelty calculator — common knowledge penalty, overlap, precision/bridge bonuses
	uniqueScore := k.CalculateNovelty(ctx, fact)

	// ─── Age penalty component ─────────────────────────────
	agePenalty := uint64(0)
	if epoch > fact.EpochBorn {
		factAge := epoch - fact.EpochBorn
		if factAge > params.FitnessGraceEpochs {
			// Penalty grows linearly after grace period, capped at full weight
			penaltyEpochs := factAge - params.FitnessGraceEpochs
			agePenalty = min64(penaltyEpochs*50_000, 1_000_000) // 20 epochs past grace = max penalty
		}
		// Cited facts resist aging — each citation reduces penalty by 100k
		if fact.IncomingCitationCount > 0 {
			reduction := min64(fact.IncomingCitationCount*100_000, agePenalty)
			agePenalty -= reduction
		}
	}

	// ─── Weighted sum ──────────────────────────────────────
	fitness := uint64(0)
	fitness += safeMulDiv(queryScore, params.FitnessWeightQueryBps, 1_000_000)
	fitness += safeMulDiv(citationScore, params.FitnessWeightCitationBps, 1_000_000)
	fitness += safeMulDiv(bridgeScore, params.FitnessWeightBridgeBps, 1_000_000)
	fitness += safeMulDiv(depthScore, params.FitnessWeightDepthBps, 1_000_000)
	fitness += safeMulDiv(patronScore, params.FitnessWeightPatronBps, 1_000_000)
	fitness += safeMulDiv(uniqueScore, params.FitnessWeightUniqueBps, 1_000_000)

	// Subtract age penalty
	ageDeduction := safeMulDiv(agePenalty, params.FitnessWeightAgeBps, 1_000_000)
	if ageDeduction > fitness {
		fitness = 0
	} else {
		fitness -= ageDeduction
	}

	// Cap at 1,000,000
	if fitness > 1_000_000 {
		fitness = 1_000_000
	}

	return fitness
}

// UpdateAllFitnessScores recalculates fitness for all verified/active facts
// and resets epoch query counters. Called at fitness epoch boundaries.
func (k Keeper) UpdateAllFitnessScores(ctx context.Context) error {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	height := uint64(sdkCtx.BlockHeight())

	params, err := k.GetParams(ctx)
	if err != nil {
		return err
	}

	epoch := uint64(0)
	if params.FitnessEpochBlocks > 0 {
		epoch = height / params.FitnessEpochBlocks
	}

	// Collect facts to update (avoid modifying store during iteration)
	var factsToUpdate []*types.Fact
	k.IterateFacts(ctx, func(fact *types.Fact) bool {
		// Only score verified or active facts
		if fact.Status == types.FactStatus_FACT_STATUS_VERIFIED ||
			fact.Status == types.FactStatus_FACT_STATUS_ACTIVE ||
			fact.Status == types.FactStatus_FACT_STATUS_PROVISIONAL {
			factsToUpdate = append(factsToUpdate, fact)
		}
		return false
	})

	for _, fact := range factsToUpdate {
		oldFitness := fact.FitnessScore
		newFitness := k.CalculateFitness(ctx, fact, epoch)

		fact.FitnessScore = newFitness
		fact.FitnessUpdatedBlock = height
		fact.QueryCountEpoch = 0 // Reset epoch query counter

		if err := k.SetFact(ctx, fact); err != nil {
			k.Logger(ctx).Error("failed to update fitness", "fact_id", fact.Id, "error", err)
			continue
		}

		// Only emit events for significant changes (>50,000 BPS delta)
		var delta uint64
		if newFitness > oldFitness {
			delta = newFitness - oldFitness
		} else {
			delta = oldFitness - newFitness
		}
		if delta > 50_000 {
			sdkCtx.EventManager().EmitEvent(sdk.NewEvent(
				"zerone.knowledge.fitness_updated",
				sdk.NewAttribute("fact_id", fact.Id),
				sdk.NewAttribute("fitness_score", fmt.Sprintf("%d", newFitness)),
				sdk.NewAttribute("fitness_label", FitnessLabel(newFitness)),
				sdk.NewAttribute("query_count_epoch", fmt.Sprintf("%d", fact.QueryCountEpoch)),
				sdk.NewAttribute("epoch", fmt.Sprintf("%d", epoch)),
			))
		}
	}

	k.Logger(ctx).Info("fitness scores updated",
		"facts_updated", len(factsToUpdate),
		"epoch", epoch,
		"height", height,
	)

	return nil
}

// IncrementFactQueryCount increments both lifetime and epoch query counters for a fact.
func (k Keeper) IncrementFactQueryCount(ctx context.Context, factID string) {
	fact, found := k.GetFact(ctx, factID)
	if !found {
		return
	}
	fact.QueryCount++
	fact.QueryCountEpoch++
	_ = k.SetFact(ctx, fact)
}

// CountTransitiveDependents counts how many facts transitively depend on this fact
// via incoming relations (REQUIRES, SUPPORTS). Capped at maxDepth to prevent explosion.
func (k Keeper) CountTransitiveDependents(ctx context.Context, factID string, maxDepth uint64) uint64 {
	if maxDepth == 0 {
		return 0
	}

	visited := make(map[string]bool)
	return k.countDependentsRecursive(ctx, factID, maxDepth, visited)
}

func (k Keeper) countDependentsRecursive(ctx context.Context, factID string, depth uint64, visited map[string]bool) uint64 {
	if depth == 0 {
		return 0
	}

	count := uint64(0)
	incoming, err := k.GetIncomingRelations(ctx, factID)
	if err != nil {
		return 0
	}

	for _, rel := range incoming {
		if visited[rel.SourceFactId] {
			continue
		}
		// Only count REQUIRES and SUPPORTS as meaningful dependencies
		if rel.Relation == types.RelationType_RELATION_TYPE_REQUIRES ||
			rel.Relation == types.RelationType_RELATION_TYPE_SUPPORTS {
			visited[rel.SourceFactId] = true
			count++
			count += k.countDependentsRecursive(ctx, rel.SourceFactId, depth-1, visited)
		}
	}

	return count
}

// CalculateUniqueness returns a 0-1,000,000 score measuring how unique a fact is
// within its domain. Inverse of how many facts share the same subject.
func (k Keeper) CalculateUniqueness(ctx context.Context, fact *types.Fact) uint64 {
	// If no structure, assume unique
	if fact.Structure == nil || fact.Structure.Subject == "" {
		return 1_000_000
	}

	// Count facts with the same subject in the same domain
	sameSubject := uint64(0)
	k.IterateFactsByDomain(ctx, fact.Domain, func(factID string) bool {
		if factID == fact.Id {
			return false
		}
		other, found := k.GetFact(ctx, factID)
		if found && other.Structure != nil && other.Structure.Subject == fact.Structure.Subject {
			sameSubject++
		}
		return false
	})

	// No duplicates = full score
	if sameSubject == 0 {
		return 1_000_000
	}

	// Score decreases with more duplicates: 1/(1+n) scaled to BPS
	// 1 dup → 500,000; 2 dups → 333,333; 3 dups → 250,000; etc.
	return safeMulDiv(1_000_000, 1, 1+sameSubject)
}

// FitnessLabel returns a human-readable label for a fitness score.
func FitnessLabel(score uint64) string {
	switch {
	case score >= 800_000:
		return "keystone"
	case score >= 600_000:
		return "thriving"
	case score >= 300_000:
		return "healthy"
	case score >= 100_000:
		return "low"
	default:
		return "critical"
	}
}

// GetFactsByFitness returns facts sorted by fitness score.
func (k Keeper) GetFactsByFitness(ctx context.Context, domain string, minFitness uint64, limit uint64, ascending bool) []*types.Fact {
	var facts []*types.Fact

	iterateFn := func(fact *types.Fact) bool {
		// Only include verified/active facts
		if fact.Status != types.FactStatus_FACT_STATUS_VERIFIED &&
			fact.Status != types.FactStatus_FACT_STATUS_ACTIVE {
			return false
		}
		if fact.FitnessScore < minFitness {
			return false
		}
		if domain != "" && fact.Domain != domain {
			return false
		}
		facts = append(facts, fact)
		return false
	}

	if domain != "" {
		k.IterateFactsByDomain(ctx, domain, func(factID string) bool {
			fact, found := k.GetFact(ctx, factID)
			if found {
				iterateFn(fact)
			}
			return false
		})
	} else {
		k.IterateFacts(ctx, iterateFn)
	}

	// Sort by fitness score
	if ascending {
		sort.Slice(facts, func(i, j int) bool {
			return facts[i].FitnessScore < facts[j].FitnessScore
		})
	} else {
		sort.Slice(facts, func(i, j int) bool {
			return facts[i].FitnessScore > facts[j].FitnessScore
		})
	}

	// Apply limit
	if limit == 0 {
		limit = 50
	}
	if uint64(len(facts)) > limit {
		facts = facts[:limit]
	}

	return facts
}

// min64 returns the smaller of two uint64 values.
func min64(a, b uint64) uint64 {
	if a < b {
		return a
	}
	return b
}
