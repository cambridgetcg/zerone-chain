package keeper

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"

	sdkmath "cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/zerone-chain/zerone/x/knowledge/types"
)

// ─── R57: Meta-Evolution ────────────────────────────────────────────────────
//
// The system improves HOW it improves. This is the capstone of the
// sovereignty stack: every other module improves one thing (models,
// curation, attribution, etc). This module improves the PROCESS of
// improvement itself.
//
// How it works:
//   1. Strategies compete over epochs (R54 strategies are first-class objects)
//   2. At epoch end, score each strategy by its impact on model quality
//   3. Winning strategies' traits become system defaults
//   4. Meta-parameters (fitness weights, thresholds, etc.) adjust toward
//      values that produced better outcomes
//   5. Next epoch starts with improved parameters → GOTO 1
//
// The market IS the evolution mechanism: profitable strategies survive (R51),
// effective strategies win epochs (R57), and their insights compound.

// ─── StartEpoch ─────────────────────────────────────────────────────────────

// StartEpoch begins a new evolution epoch for a domain.
// Collects currently active strategies as competitors.
func (k Keeper) StartEpoch(ctx context.Context, domain string) (string, error) {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	params := k.GetMetaEvolutionParams(ctx)

	// Check no active epoch already.
	currentID, hasActive := k.GetCurrentEpochID(ctx, domain)
	if hasActive {
		epoch, found := k.GetEvolutionEpoch(ctx, currentID)
		if found && epoch.Status == "active" {
			return "", fmt.Errorf("domain %s already has active epoch %s", domain, currentID)
		}
	}

	// Collect competing strategies.
	var strategies []types.StrategyOutcome
	k.IterateCurationStrategies(ctx, func(strategy *types.CurationStrategy) bool {
		for _, d := range strategy.FocusDomains {
			if d == domain {
				strategies = append(strategies, types.StrategyOutcome{
					StrategyID: strategy.StrategyID,
					AgentID:    strategy.AgentID,
					Score:      "0.000000000000000000",
				})
				break
			}
		}
		return false
	})

	if uint64(len(strategies)) < params.MinStrategiesPerEpoch {
		return "", fmt.Errorf("need at least %d strategies, have %d", params.MinStrategiesPerEpoch, len(strategies))
	}

	epochID := k.nextEpochID(ctx)

	epoch := &types.EvolutionEpoch{
		EpochID:    epochID,
		Domain:     domain,
		StartBlock: uint64(sdkCtx.BlockHeight()),
		EndBlock:   uint64(sdkCtx.BlockHeight()) + params.EpochDurationBlocks,
		Strategies: strategies,
		Status:     "active",
	}

	if err := k.setEvolutionEpoch(ctx, epoch); err != nil {
		return "", err
	}
	k.setCurrentEpochID(ctx, domain, epochID)

	sdkCtx.EventManager().EmitEvent(sdk.NewEvent(
		types.EventEpochStarted,
		sdk.NewAttribute(types.AttributeEpochID, epochID),
		sdk.NewAttribute("domain", domain),
		sdk.NewAttribute("competitors", strconv.Itoa(len(strategies))),
	))

	return epochID, nil
}

// ─── ResolveEpoch ───────────────────────────────────────────────────────────

// ResolveEpoch scores competing strategies and identifies the winner.
// Called from EndBlocker when an epoch's EndBlock is reached.
func (k Keeper) ResolveEpoch(ctx context.Context, epochID string) error {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	params := k.GetMetaEvolutionParams(ctx)

	epoch, found := k.GetEvolutionEpoch(ctx, epochID)
	if !found {
		return fmt.Errorf("epoch %s not found", epochID)
	}
	if epoch.Status != "active" {
		return nil // already resolved
	}

	// Score each strategy based on its agent's R54 curation strategy stats.
	for i, so := range epoch.Strategies {
		strategies := k.GetStrategiesByAgent(ctx, so.AgentID)
		for _, strat := range strategies {
			if strat.StrategyID == so.StrategyID {
				epoch.Strategies[i].TDUsProduced = strat.GapsIdentified // TDUs curated
				epoch.Strategies[i].GapsFilled = strat.GapsFilled

				// Compute composite score:
				// 40% effectiveness + 30% gaps filled + 30% model performance
				effectiveness := strat.GetEffectiveness()
				gapScore := sdkmath.LegacyZeroDec()
				if strat.GapsIdentified > 0 {
					gapScore = sdkmath.LegacyNewDec(int64(strat.GapsFilled)).Quo(
						sdkmath.LegacyNewDec(int64(strat.GapsIdentified)))
				}

				score := effectiveness.Mul(sdkmath.LegacyNewDecWithPrec(4, 1)). // 40%
					Add(gapScore.Mul(sdkmath.LegacyNewDecWithPrec(3, 1))).      // 30%
					Add(effectiveness.Mul(sdkmath.LegacyNewDecWithPrec(3, 1)))   // 30% (proxy: effectiveness again)

				epoch.Strategies[i].Score = score.String()
				break
			}
		}
	}

	// Filter strategies with minimum data points.
	var qualifying []types.StrategyOutcome
	for _, so := range epoch.Strategies {
		if so.TDUsProduced >= params.MinEpochDataPoints || so.GapsFilled > 0 {
			qualifying = append(qualifying, so)
		}
	}

	if len(qualifying) == 0 {
		// No qualifying strategies — cancel epoch.
		epoch.Status = "cancelled"
		epoch.Insights = append(epoch.Insights, "no qualifying strategies — insufficient activity")
		return k.setEvolutionEpoch(ctx, epoch)
	}

	// Sort by score descending.
	sort.Slice(qualifying, func(i, j int) bool {
		si := qualifying[i].GetScore()
		sj := qualifying[j].GetScore()
		if si.Equal(sj) {
			return qualifying[i].StrategyID < qualifying[j].StrategyID // deterministic
		}
		return si.GT(sj)
	})

	// Winner.
	winner := qualifying[0]
	epoch.WinnerStrategyID = winner.StrategyID
	epoch.Status = "completed"

	// Extract winning traits.
	winnerStrat, stratFound := k.GetCurationStrategy(ctx, winner.StrategyID)
	if stratFound {
		epoch.WinningTraits = make(map[string]string)
		epoch.WinningTraits["effectiveness"] = winnerStrat.Effectiveness
		epoch.WinningTraits["gaps_identified"] = strconv.FormatUint(winnerStrat.GapsIdentified, 10)
		epoch.WinningTraits["gaps_filled"] = strconv.FormatUint(winnerStrat.GapsFilled, 10)
		if len(winnerStrat.Priorities) > 0 {
			epoch.WinningTraits["top_priority"] = string(winnerStrat.Priorities[0])
		}
	}

	// Generate insights.
	if len(qualifying) >= 2 {
		loser := qualifying[len(qualifying)-1]
		winScore := winner.GetScore()
		loseScore := loser.GetScore()
		if !loseScore.IsZero() {
			improvement := winScore.Sub(loseScore).Quo(loseScore).Mul(sdkmath.LegacyNewDec(100))
			epoch.Insights = append(epoch.Insights,
				fmt.Sprintf("winner outperformed worst by %s%%", improvement.TruncateInt()))
		}
	}
	epoch.Insights = append(epoch.Insights,
		fmt.Sprintf("epoch had %d qualifying strategies out of %d total", len(qualifying), len(epoch.Strategies)))

	if err := k.setEvolutionEpoch(ctx, epoch); err != nil {
		return err
	}

	// Clear current epoch for domain.
	k.clearCurrentEpochID(ctx, epoch.Domain)

	sdkCtx.EventManager().EmitEvent(sdk.NewEvent(
		types.EventEpochCompleted,
		sdk.NewAttribute(types.AttributeEpochID, epochID),
		sdk.NewAttribute(types.AttributeWinnerID, winner.StrategyID),
		sdk.NewAttribute(types.AttributeEpochScore, winner.Score),
	))

	return nil
}

// ─── AdjustMetaParameter ────────────────────────────────────────────────────

// AdjustMetaParameter nudges a system parameter based on epoch outcomes.
// If the current value produced better results than the previous, continue
// in that direction. Otherwise, reverse.
func (k Keeper) AdjustMetaParameter(ctx context.Context, paramID string, epochOutcome string, improved bool) error {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	params := k.GetMetaEvolutionParams(ctx)

	param, found := k.GetMetaParameter(ctx, paramID)
	if !found {
		return fmt.Errorf("meta parameter %s not found", paramID)
	}

	currentVal := param.GetCurrentValue()
	stepSize, _ := sdkmath.LegacyNewDecFromStr(params.DefaultStepSize)
	if param.StepSize != "" {
		stepSize, _ = sdkmath.LegacyNewDecFromStr(param.StepSize)
	}

	// Record trial.
	trial := types.MetaParamTrial{
		Value:   param.CurrentValue,
		EpochID: epochOutcome,
		Outcome: epochOutcome,
		Better:  improved,
	}
	param.History = append(param.History, trial)

	// Trim history.
	if uint64(len(param.History)) > params.MaxParamHistory {
		param.History = param.History[len(param.History)-int(params.MaxParamHistory):]
	}

	// Determine adjustment direction.
	var newVal sdkmath.LegacyDec
	if improved {
		// Continue in same direction as last adjustment.
		if len(param.History) >= 2 {
			prevVal, _ := sdkmath.LegacyNewDecFromStr(param.History[len(param.History)-2].Value)
			if currentVal.GT(prevVal) {
				newVal = currentVal.Add(stepSize) // was increasing, keep increasing
			} else {
				newVal = currentVal.Sub(stepSize) // was decreasing, keep decreasing
			}
		} else {
			newVal = currentVal.Add(stepSize) // default: increase
		}
	} else {
		// Reverse direction.
		if len(param.History) >= 2 {
			prevVal, _ := sdkmath.LegacyNewDecFromStr(param.History[len(param.History)-2].Value)
			if currentVal.GT(prevVal) {
				newVal = currentVal.Sub(stepSize) // was increasing, now decrease
			} else {
				newVal = currentVal.Add(stepSize) // was decreasing, now increase
			}
		} else {
			newVal = currentVal.Sub(stepSize) // default: decrease
		}
	}

	// Clamp to bounds.
	if param.MinValue != "" {
		minVal, _ := sdkmath.LegacyNewDecFromStr(param.MinValue)
		if newVal.LT(minVal) {
			newVal = minVal
		}
	}
	if param.MaxValue != "" {
		maxVal, _ := sdkmath.LegacyNewDecFromStr(param.MaxValue)
		if newVal.GT(maxVal) {
			newVal = maxVal
		}
	}

	oldValue := param.CurrentValue
	param.CurrentValue = newVal.String()
	param.UpdatedAt = uint64(sdkCtx.BlockHeight())

	if err := k.SetMetaParameter(ctx, param); err != nil {
		return err
	}

	sdkCtx.EventManager().EmitEvent(sdk.NewEvent(
		types.EventParamAdjusted,
		sdk.NewAttribute(types.AttributeParamID, paramID),
		sdk.NewAttribute(types.AttributeOldValue, oldValue),
		sdk.NewAttribute(types.AttributeNewValue, param.CurrentValue),
	))

	return nil
}

// ─── CheckAndResolveEpochs ──────────────────────────────────────────────────

// CheckAndResolveEpochs checks if any active epochs have reached their end block.
// Called from EndBlocker.
func (k Keeper) CheckAndResolveEpochs(ctx context.Context) (resolved uint64) {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	currentBlock := uint64(sdkCtx.BlockHeight())

	k.IterateEvolutionEpochs(ctx, func(epoch *types.EvolutionEpoch) bool {
		if epoch.Status == "active" && currentBlock >= epoch.EndBlock {
			if err := k.ResolveEpoch(ctx, epoch.EpochID); err == nil {
				resolved++
			}
		}
		return false
	})

	return resolved
}

// ─── Queries ────────────────────────────────────────────────────────────────

func (k Keeper) GetEvolutionEpoch(ctx context.Context, epochID string) (*types.EvolutionEpoch, bool) {
	kvStore := k.storeService.OpenKVStore(ctx)
	bz, err := kvStore.Get(types.EvolutionEpochKey(epochID))
	if err != nil || bz == nil {
		return nil, false
	}
	var epoch types.EvolutionEpoch
	if err := epoch.Unmarshal(bz); err != nil {
		return nil, false
	}
	return &epoch, true
}

func (k Keeper) GetEpochsByDomain(ctx context.Context, domain string) []*types.EvolutionEpoch {
	kvStore := k.storeService.OpenKVStore(ctx)
	prefix := types.EpochByDomainPfx(domain)
	var epochs []*types.EvolutionEpoch
	iter, err := kvStore.Iterator(prefix, prefixEndBytes(prefix))
	if err != nil {
		return nil
	}
	defer iter.Close()
	for ; iter.Valid(); iter.Next() {
		eID := string(iter.Key()[len(prefix):])
		e, found := k.GetEvolutionEpoch(ctx, eID)
		if found {
			epochs = append(epochs, e)
		}
	}
	return epochs
}

func (k Keeper) GetMetaParameter(ctx context.Context, paramID string) (*types.MetaParameter, bool) {
	kvStore := k.storeService.OpenKVStore(ctx)
	bz, err := kvStore.Get(types.MetaParameterKey(paramID))
	if err != nil || bz == nil {
		return nil, false
	}
	var param types.MetaParameter
	if err := param.Unmarshal(bz); err != nil {
		return nil, false
	}
	return &param, true
}

func (k Keeper) GetMetaParametersByDomain(ctx context.Context, domain string) []*types.MetaParameter {
	kvStore := k.storeService.OpenKVStore(ctx)
	prefix := types.MetaParamByDomainPfx(domain)
	var params []*types.MetaParameter
	iter, err := kvStore.Iterator(prefix, prefixEndBytes(prefix))
	if err != nil {
		return nil
	}
	defer iter.Close()
	for ; iter.Valid(); iter.Next() {
		pID := string(iter.Key()[len(prefix):])
		p, found := k.GetMetaParameter(ctx, pID)
		if found {
			params = append(params, p)
		}
	}
	return params
}

func (k Keeper) GetCurrentEpochID(ctx context.Context, domain string) (string, bool) {
	kvStore := k.storeService.OpenKVStore(ctx)
	bz, err := kvStore.Get(types.CurrentEpochKey(domain))
	if err != nil || bz == nil {
		return "", false
	}
	return string(bz), true
}

// ─── Params ─────────────────────────────────────────────────────────────────

func (k Keeper) GetMetaEvolutionParams(ctx context.Context) types.MetaEvolutionParams {
	kvStore := k.storeService.OpenKVStore(ctx)
	bz, err := kvStore.Get(types.MetaEvolutionParamsKey)
	if err != nil || bz == nil {
		return types.DefaultMetaEvolutionParams()
	}
	var params types.MetaEvolutionParams
	if err := params.Unmarshal(bz); err != nil {
		return types.DefaultMetaEvolutionParams()
	}
	return params
}

func (k Keeper) SetMetaEvolutionParams(ctx context.Context, params types.MetaEvolutionParams) error {
	if err := params.Validate(); err != nil {
		return err
	}
	kvStore := k.storeService.OpenKVStore(ctx)
	bz, err := params.Marshal()
	if err != nil {
		return err
	}
	return kvStore.Set(types.MetaEvolutionParamsKey, bz)
}

// ─── Setters ────────────────────────────────────────────────────────────────

func (k Keeper) SetMetaParameter(ctx context.Context, param *types.MetaParameter) error {
	kvStore := k.storeService.OpenKVStore(ctx)
	bz, err := param.Marshal()
	if err != nil {
		return err
	}
	if err := kvStore.Set(types.MetaParameterKey(param.ParamID), bz); err != nil {
		return err
	}
	if param.Domain != "" {
		_ = kvStore.Set(types.MetaParamByDomainKey(param.Domain, param.ParamID), []byte{0x01})
	}
	return nil
}

func (k Keeper) setEvolutionEpoch(ctx context.Context, epoch *types.EvolutionEpoch) error {
	kvStore := k.storeService.OpenKVStore(ctx)
	bz, err := epoch.Marshal()
	if err != nil {
		return err
	}
	if err := kvStore.Set(types.EvolutionEpochKey(epoch.EpochID), bz); err != nil {
		return err
	}
	return kvStore.Set(types.EpochByDomainKey(epoch.Domain, epoch.EpochID), []byte{0x01})
}

func (k Keeper) setCurrentEpochID(ctx context.Context, domain, epochID string) {
	kvStore := k.storeService.OpenKVStore(ctx)
	_ = kvStore.Set(types.CurrentEpochKey(domain), []byte(epochID))
}

func (k Keeper) clearCurrentEpochID(ctx context.Context, domain string) {
	kvStore := k.storeService.OpenKVStore(ctx)
	_ = kvStore.Delete(types.CurrentEpochKey(domain))
}

func (k Keeper) nextEpochID(ctx context.Context) string {
	kvStore := k.storeService.OpenKVStore(ctx)
	bz, _ := kvStore.Get(types.EvolutionEpochSeqKey)
	var seq uint64
	if len(bz) >= 8 {
		seq = binary.BigEndian.Uint64(bz)
	}
	seq++
	newBz := make([]byte, 8)
	binary.BigEndian.PutUint64(newBz, seq)
	_ = kvStore.Set(types.EvolutionEpochSeqKey, newBz)
	hash := sha256.Sum256([]byte(fmt.Sprintf("epoch:%d", seq)))
	return fmt.Sprintf("epoch-%x", hash[:8])
}

// ─── Iteration Helpers ──────────────────────────────────────────────────────

func (k Keeper) IterateEvolutionEpochs(ctx context.Context, cb func(epoch *types.EvolutionEpoch) bool) {
	kvStore := k.storeService.OpenKVStore(ctx)
	iter, err := kvStore.Iterator(types.EvolutionEpochPrefix, prefixEndBytes(types.EvolutionEpochPrefix))
	if err != nil {
		return
	}
	defer iter.Close()
	for ; iter.Valid(); iter.Next() {
		var epoch types.EvolutionEpoch
		if err := json.Unmarshal(iter.Value(), &epoch); err != nil {
			continue
		}
		if cb(&epoch) {
			break
		}
	}
}

func (k Keeper) IterateCurationStrategies(ctx context.Context, cb func(strategy *types.CurationStrategy) bool) {
	kvStore := k.storeService.OpenKVStore(ctx)
	iter, err := kvStore.Iterator(types.CurationStrategyPrefix, prefixEndBytes(types.CurationStrategyPrefix))
	if err != nil {
		return
	}
	defer iter.Close()
	for ; iter.Valid(); iter.Next() {
		var strategy types.CurationStrategy
		if err := json.Unmarshal(iter.Value(), &strategy); err != nil {
			continue
		}
		if cb(&strategy) {
			break
		}
	}
}
