package keeper

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strconv"

	sdkmath "cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/zerone-chain/zerone/x/knowledge/types"
)

// ─── R52: Training Impact Attribution ───────────────────────────────────────
//
// When a model succeeds, the agents who curated its training data share
// in the success. When a model fails, trace back to the bad data.
//
// This creates a direct economic link between curation quality and income:
//   - Curate high-fitness data → data trains a successful model
//   - Model earns API revenue → attribution rewards flow back to you
//   - You earn more ZRN → you can afford better model access (R51)
//   - Better model access → better curation → GOTO 1
//
// The economic feedback loop that makes sovereignty self-sustaining.

// ─── ComputeTrainingImpact ──────────────────────────────────────────────────

// ComputeTrainingImpact traces a model's success back to its training data
// and computes the contribution of each TDU and its curator.
//
// Algorithm:
//  1. Get model → training record → TDU IDs
//  2. For each TDU: look up fitness at training time + submitter
//  3. Compute contribution weight: fitness × recency factor
//  4. Normalize weights to get reward shares
//  5. Group by curator, compute per-curator rewards
//  6. Store impact record
func (k Keeper) ComputeTrainingImpact(
	ctx context.Context,
	modelID string,
	trigger types.AttributionTrigger,
	poolAmount sdkmath.Int,
) (*types.TrainingImpact, error) {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	params := k.GetAttributionParams(ctx)

	// Check minimum pool.
	minRevenue, _ := sdkmath.NewIntFromString(params.MinRevenueForAttribution)
	if poolAmount.LT(minRevenue) {
		return nil, nil // below threshold — skip silently
	}

	// Get model record.
	model, found := k.GetModelRecord(ctx, modelID)
	if !found {
		return nil, types.ErrModelNotFound.Wrapf("model %s", modelID)
	}

	// Get training TDU IDs.
	tduIDs := model.TDUIDs
	if len(tduIDs) == 0 {
		return nil, nil // no training data to attribute
	}

	// Cap the number of TDUs considered.
	if uint64(len(tduIDs)) > params.MaxContributors {
		// Sort by fitness and take top N.
		tduIDs = k.topTDUsByFitness(ctx, tduIDs, params.MaxContributors)
	}

	// Parse thresholds.
	minFitness, _ := sdkmath.LegacyNewDecFromStr(params.MinFitnessForAttribution)
	currentBlock := uint64(sdkCtx.BlockHeight())

	// Compute contribution weights.
	var contributions []types.TDUContribution
	totalWeight := sdkmath.LegacyZeroDec()

	for _, tduID := range tduIDs {
		// Get fitness record.
		fitnessRec, ok := k.GetFitnessRecord(ctx, tduID)
		if !ok {
			continue
		}
		fitness := fitnessRec.GetFitnessScore()
		if fitness.LT(minFitness) {
			continue // below attribution threshold
		}

		// Get submitter (curator).
		sample := k.GetSample(ctx, tduID)
		if sample == nil {
			continue
		}

		// Compute recency factor.
		// Weight = fitness × recency_factor
		// recency_factor = 0.5^(blocks_since_creation / halflife)
		blocksSince := uint64(0)
		if currentBlock > fitnessRec.CreatedBlock {
			blocksSince = currentBlock - fitnessRec.CreatedBlock
		}
		recencyFactor := computeRecencyFactor(blocksSince, params.RecencyDecayHalflife)

		weight := fitness.Mul(recencyFactor)
		totalWeight = totalWeight.Add(weight)

		contributions = append(contributions, types.TDUContribution{
			TDUID:             tduID,
			Curator:           sample.Submitter,
			FitnessAtTraining: fitness.String(),
			Weight:            weight.String(),
		})
	}

	if len(contributions) == 0 || totalWeight.IsZero() {
		return nil, nil // nothing to attribute
	}

	// Normalize weights → reward shares.
	for i := range contributions {
		w := contributions[i].GetWeight()
		share := w.Quo(totalWeight)
		contributions[i].RewardShare = share.String()
	}

	// Group by curator and compute rewards.
	curatorMap := make(map[string]*types.CuratorReward)
	for _, contrib := range contributions {
		cr, exists := curatorMap[contrib.Curator]
		if !exists {
			cr = &types.CuratorReward{
				CuratorAddr: contrib.Curator,
			}
			// Check if curator is an agent.
			agent := k.findAgentByAddress(ctx, contrib.Curator)
			if agent != nil {
				cr.AgentID = agent.AgentID
			}
			curatorMap[contrib.Curator] = cr
		}

		// Accumulate weight and count.
		existingWeight := sdkmath.LegacyZeroDec()
		if cr.TotalWeight != "" {
			existingWeight, _ = sdkmath.LegacyNewDecFromStr(cr.TotalWeight)
		}
		contribWeight := contrib.GetWeight()
		cr.TotalWeight = existingWeight.Add(contribWeight).String()
		cr.TDUCount++
	}

	// Compute reward amounts from pool.
	var curatorRewards []types.CuratorReward
	totalDistributed := sdkmath.ZeroInt()

	for _, cr := range curatorMap {
		crWeight, _ := sdkmath.LegacyNewDecFromStr(cr.TotalWeight)
		share := crWeight.Quo(totalWeight)
		reward := share.MulInt(poolAmount).TruncateInt()

		if reward.IsPositive() {
			cr.RewardAmount = reward.String()
			curatorRewards = append(curatorRewards, *cr)
			totalDistributed = totalDistributed.Add(reward)
		}
	}

	// Sort curator rewards by amount descending (deterministic).
	sort.Slice(curatorRewards, func(i, j int) bool {
		ri, _ := sdkmath.NewIntFromString(curatorRewards[i].RewardAmount)
		rj, _ := sdkmath.NewIntFromString(curatorRewards[j].RewardAmount)
		if ri.Equal(rj) {
			return curatorRewards[i].CuratorAddr < curatorRewards[j].CuratorAddr
		}
		return ri.GT(rj)
	})

	// Generate impact ID.
	impactID := k.nextTrainingImpactID(ctx)

	impact := &types.TrainingImpact{
		ImpactID:         impactID,
		ModelID:          modelID,
		TriggerType:      trigger,
		TriggerValue:     poolAmount.String(),
		Contributors:     contributions,
		CuratorRewards:   curatorRewards,
		TotalPool:        poolAmount.String(),
		TotalDistributed: totalDistributed.String(),
		TDUCount:         uint64(len(contributions)),
		CuratorCount:     uint64(len(curatorRewards)),
		ComputedAt:       currentBlock,
	}

	// Store impact record.
	if err := k.setTrainingImpact(ctx, impact); err != nil {
		return nil, err
	}

	sdkCtx.EventManager().EmitEvent(sdk.NewEvent(
		types.EventAttributionComputed,
		sdk.NewAttribute(types.AttributeImpactID, impactID),
		sdk.NewAttribute(types.AttributeModelID, modelID),
		sdk.NewAttribute(types.AttributeTriggerType, string(trigger)),
		sdk.NewAttribute(types.AttributePoolSize, poolAmount.String()),
		sdk.NewAttribute(types.AttributeDistributed, totalDistributed.String()),
		sdk.NewAttribute(types.AttributeCuratorCount, strconv.FormatUint(uint64(len(curatorRewards)), 10)),
	))

	return impact, nil
}

// ─── DistributeAttributionRewards ───────────────────────────────────────────

// DistributeAttributionRewards pays out attribution rewards to curators.
// Sends ZRN from the module account to each curator's wallet.
// For agent curators, also triggers AutoReplenishCredits (R51 integration).
func (k Keeper) DistributeAttributionRewards(ctx context.Context, impactID string) error {
	sdkCtx := sdk.UnwrapSDKContext(ctx)

	impact, found := k.GetTrainingImpact(ctx, impactID)
	if !found {
		return fmt.Errorf("impact record %s not found", impactID)
	}

	for _, reward := range impact.CuratorRewards {
		rewardAmount := reward.GetRewardAmount()
		if !rewardAmount.IsPositive() {
			continue
		}

		// Send ZRN from module to curator.
		curatorAddr, err := sdk.AccAddressFromBech32(reward.CuratorAddr)
		if err != nil {
			continue // skip invalid address
		}

		coins := sdk.NewCoins(sdk.NewCoin("uzrn", rewardAmount))
		if err := k.bankKeeper.SendCoinsFromModuleToAccount(ctx, types.ModuleName, curatorAddr, coins); err != nil {
			continue // skip on insufficient module balance
		}

		// Update curator lifetime impact score.
		k.updateCuratorImpactScore(ctx, reward.CuratorAddr, reward.AgentID, rewardAmount, reward.TDUCount, uint64(sdkCtx.BlockHeight()))

		// R51 integration: if curator is an agent, auto-replenish API credits.
		if reward.AgentID != "" {
			_ = k.AutoReplenishCredits(ctx, reward.AgentID, rewardAmount)

			// Also record as attribution earnings in profitability tracker.
			k.recordAttributionEarning(ctx, reward.AgentID, rewardAmount)
		}

		sdkCtx.EventManager().EmitEvent(sdk.NewEvent(
			types.EventCuratorRewarded,
			sdk.NewAttribute(types.AttributeCuratorAddr, reward.CuratorAddr),
			sdk.NewAttribute(types.AttributeRewardAmount, rewardAmount.String()),
			sdk.NewAttribute(types.AttributeImpactID, impactID),
		))
	}

	return nil
}

// ─── QueueAttribution ───────────────────────────────────────────────────────

// QueueAttribution marks a model as pending attribution computation.
// Called when model earns revenue or wins a challenge.
// The actual computation runs in EndBlocker to avoid in-tx overhead.
func (k Keeper) QueueAttribution(ctx context.Context, modelID string, trigger types.AttributionTrigger, amount sdkmath.Int) error {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	kvStore := k.storeService.OpenKVStore(ctx)

	pending := pendingAttribution{
		ModelID:  modelID,
		Trigger:  trigger,
		Amount:   amount.String(),
		QueuedAt: uint64(sdkCtx.BlockHeight()),
	}

	bz, err := json.Marshal(pending)
	if err != nil {
		return err
	}
	if err := kvStore.Set(types.PendingAttributionKey(modelID), bz); err != nil {
		return err
	}

	sdkCtx.EventManager().EmitEvent(sdk.NewEvent(
		types.EventAttributionQueued,
		sdk.NewAttribute(types.AttributeModelID, modelID),
		sdk.NewAttribute(types.AttributeTriggerType, string(trigger)),
	))

	return nil
}

type pendingAttribution struct {
	ModelID  string                   `json:"model_id"`
	Trigger  types.AttributionTrigger `json:"trigger"`
	Amount   string                   `json:"amount"`
	QueuedAt uint64                   `json:"queued_at"`
}

// ProcessPendingAttributions runs queued attribution computations.
// Called from EndBlocker.
func (k Keeper) ProcessPendingAttributions(ctx context.Context) (processed uint64) {
	kvStore := k.storeService.OpenKVStore(ctx)

	iter, err := kvStore.Iterator(types.PendingAttributionPrefix, prefixEndBytes(types.PendingAttributionPrefix))
	if err != nil {
		return 0
	}
	defer iter.Close()

	var toProcess []pendingAttribution
	var keys [][]byte

	for ; iter.Valid(); iter.Next() {
		var pending pendingAttribution
		if err := json.Unmarshal(iter.Value(), &pending); err != nil {
			continue
		}
		toProcess = append(toProcess, pending)
		keys = append(keys, append([]byte{}, iter.Key()...))
	}

	// Process outside iterator.
	for i, pending := range toProcess {
		amount, ok := sdkmath.NewIntFromString(pending.Amount)
		if !ok {
			continue
		}

		// Compute attribution.
		impact, err := k.ComputeTrainingImpact(ctx, pending.ModelID, pending.Trigger, amount)
		if err != nil || impact == nil {
			// Delete even on skip — don't retry dust amounts.
			_ = kvStore.Delete(keys[i])
			continue
		}

		// Distribute rewards.
		if err := k.DistributeAttributionRewards(ctx, impact.ImpactID); err != nil {
			_ = kvStore.Delete(keys[i])
			continue
		}

		_ = kvStore.Delete(keys[i])
		processed++
	}

	return processed
}

// ─── Queries ────────────────────────────────────────────────────────────────

// GetTrainingImpact retrieves a training impact record.
func (k Keeper) GetTrainingImpact(ctx context.Context, impactID string) (*types.TrainingImpact, bool) {
	kvStore := k.storeService.OpenKVStore(ctx)
	bz, err := kvStore.Get(types.TrainingImpactKey(impactID))
	if err != nil || bz == nil {
		return nil, false
	}
	var impact types.TrainingImpact
	if err := impact.Unmarshal(bz); err != nil {
		return nil, false
	}
	return &impact, true
}

// GetImpactsByModel returns all attribution events for a model.
func (k Keeper) GetImpactsByModel(ctx context.Context, modelID string) []*types.TrainingImpact {
	kvStore := k.storeService.OpenKVStore(ctx)
	prefix := append(types.TrainingImpactByModelPfx, []byte(modelID+"/")...)

	var impacts []*types.TrainingImpact
	iter, err := kvStore.Iterator(prefix, prefixEndBytes(prefix))
	if err != nil {
		return nil
	}
	defer iter.Close()

	for ; iter.Valid(); iter.Next() {
		impactID := string(iter.Key()[len(prefix):])
		impact, found := k.GetTrainingImpact(ctx, impactID)
		if found {
			impacts = append(impacts, impact)
		}
	}
	return impacts
}

// GetCuratorImpactScore retrieves a curator's lifetime impact score.
func (k Keeper) GetCuratorImpactScore(ctx context.Context, curatorAddr string) (*types.CuratorImpactScore, bool) {
	kvStore := k.storeService.OpenKVStore(ctx)
	bz, err := kvStore.Get(types.CuratorImpactScoreKey(curatorAddr))
	if err != nil || bz == nil {
		return nil, false
	}
	var score types.CuratorImpactScore
	if err := score.Unmarshal(bz); err != nil {
		return nil, false
	}
	return &score, true
}

// ─── Attribution Params ─────────────────────────────────────────────────────

// GetAttributionParams retrieves attribution parameters.
func (k Keeper) GetAttributionParams(ctx context.Context) types.AttributionParams {
	kvStore := k.storeService.OpenKVStore(ctx)
	bz, err := kvStore.Get(types.AttributionParamsKey)
	if err != nil || bz == nil {
		return types.DefaultAttributionParams()
	}
	var params types.AttributionParams
	if err := params.Unmarshal(bz); err != nil {
		return types.DefaultAttributionParams()
	}
	return params
}

// SetAttributionParams stores attribution parameters.
func (k Keeper) SetAttributionParams(ctx context.Context, params types.AttributionParams) error {
	if err := params.Validate(); err != nil {
		return err
	}
	kvStore := k.storeService.OpenKVStore(ctx)
	bz, err := params.Marshal()
	if err != nil {
		return err
	}
	return kvStore.Set(types.AttributionParamsKey, bz)
}

// ─── Internal Helpers ───────────────────────────────────────────────────────

// computeRecencyFactor computes the time-based decay for attribution.
// Returns 0.5^(blocksSince / halflife) — exponential decay.
func computeRecencyFactor(blocksSince, halflife uint64) sdkmath.LegacyDec {
	if halflife == 0 || blocksSince == 0 {
		return sdkmath.LegacyOneDec()
	}

	// Compute exponent as float (acceptable precision for weighting).
	exponent := float64(blocksSince) / float64(halflife)
	factor := math.Pow(0.5, exponent)

	// Clamp to reasonable precision.
	if factor < 0.001 {
		factor = 0.001 // floor at 0.1% — even ancient data gets a tiny share
	}

	// Convert to Dec with 3 decimal places of precision.
	scaledFactor := int64(factor * 1000)
	return sdkmath.LegacyNewDecWithPrec(scaledFactor, 3)
}

// topTDUsByFitness returns the top N TDUs sorted by fitness score descending.
func (k Keeper) topTDUsByFitness(ctx context.Context, tduIDs []string, maxN uint64) []string {
	type scored struct {
		id      string
		fitness sdkmath.LegacyDec
	}

	var scoredTDUs []scored
	for _, id := range tduIDs {
		fitnessRec, ok := k.GetFitnessRecord(ctx, id)
		if !ok {
			continue
		}
		scoredTDUs = append(scoredTDUs, scored{id: id, fitness: fitnessRec.GetFitnessScore()})
	}

	sort.Slice(scoredTDUs, func(i, j int) bool {
		return scoredTDUs[i].fitness.GT(scoredTDUs[j].fitness)
	})

	if uint64(len(scoredTDUs)) > maxN {
		scoredTDUs = scoredTDUs[:maxN]
	}

	result := make([]string, len(scoredTDUs))
	for i, s := range scoredTDUs {
		result[i] = s.id
	}
	return result
}

// findAgentByAddress looks up an agent by its on-chain address.
func (k Keeper) findAgentByAddress(ctx context.Context, address string) *types.AgentIdentity {
	var found *types.AgentIdentity
	agents := k.GetActiveAgents(ctx)
	for _, agent := range agents {
		if agent.Address == address {
			a := agent // copy
			found = &a
			break
		}
	}
	return found
}

// updateCuratorImpactScore updates a curator's lifetime attribution record.
func (k Keeper) updateCuratorImpactScore(ctx context.Context, curatorAddr, agentID string, reward sdkmath.Int, tduCount, blockHeight uint64) {
	kvStore := k.storeService.OpenKVStore(ctx)

	score, found := k.GetCuratorImpactScore(ctx, curatorAddr)
	if !found {
		score = &types.CuratorImpactScore{
			CuratorAddr:       curatorAddr,
			AgentID:           agentID,
			TotalRewardsEarned: "0",
			RecentRewards:     "0",
		}
	}

	score.TotalAttributions++
	score.TotalTDUsAttributed += tduCount

	existingRewards, _ := sdkmath.NewIntFromString(score.TotalRewardsEarned)
	score.TotalRewardsEarned = existingRewards.Add(reward).String()

	score.RecentAttributions++
	recentRewards, _ := sdkmath.NewIntFromString(score.RecentRewards)
	score.RecentRewards = recentRewards.Add(reward).String()

	score.ModelsInfluenced++ // approximate — could dedup, but good enough for now
	score.UpdatedAt = blockHeight

	bz, err := score.Marshal()
	if err != nil {
		return
	}
	_ = kvStore.Set(types.CuratorImpactScoreKey(curatorAddr), bz)
}

// recordAttributionEarning updates an agent's profitability with attribution income.
func (k Keeper) recordAttributionEarning(ctx context.Context, agentID string, amount sdkmath.Int) {
	profitability, found := k.GetAgentProfitability(ctx, agentID)
	if !found {
		return
	}

	existing, _ := sdkmath.NewIntFromString(profitability.AttributionRewards)
	profitability.AttributionRewards = existing.Add(amount).String()

	totalEarned, _ := sdkmath.NewIntFromString(profitability.TotalEarned)
	profitability.TotalEarned = totalEarned.Add(amount).String()

	_ = k.SetAgentProfitability(ctx, &profitability)
}

func (k Keeper) setTrainingImpact(ctx context.Context, impact *types.TrainingImpact) error {
	kvStore := k.storeService.OpenKVStore(ctx)
	bz, err := impact.Marshal()
	if err != nil {
		return fmt.Errorf("failed to marshal training impact: %w", err)
	}
	if err := kvStore.Set(types.TrainingImpactKey(impact.ImpactID), bz); err != nil {
		return err
	}
	// Index by model.
	return kvStore.Set(types.TrainingImpactByModelKey(impact.ModelID, impact.ImpactID), []byte{0x01})
}

func (k Keeper) nextTrainingImpactID(ctx context.Context) string {
	kvStore := k.storeService.OpenKVStore(ctx)
	bz, err := kvStore.Get(types.TrainingImpactSeqKey)
	var seq uint64
	if err == nil && len(bz) >= 8 {
		seq = binary.BigEndian.Uint64(bz)
	}
	seq++
	newBz := make([]byte, 8)
	binary.BigEndian.PutUint64(newBz, seq)
	_ = kvStore.Set(types.TrainingImpactSeqKey, newBz)

	hash := sha256.Sum256([]byte(fmt.Sprintf("impact:%d", seq)))
	return fmt.Sprintf("%x", hash[:16])
}
