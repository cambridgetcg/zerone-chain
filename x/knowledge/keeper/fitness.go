package keeper

import (
	"context"
	"encoding/json"
	"fmt"

	sdkmath "cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/zerone-chain/zerone/x/knowledge/types"
)

// ─── Fitness Record CRUD ────────────────────────────────────────────────────

// SetFitnessRecord stores a TDU fitness record as JSON.
func (k Keeper) SetFitnessRecord(ctx context.Context, record types.TDUFitnessRecord) error {
	store := k.storeService.OpenKVStore(ctx)
	bz, err := json.Marshal(record)
	if err != nil {
		return fmt.Errorf("failed to marshal fitness record: %w", err)
	}
	return store.Set(types.FitnessRecordKey(record.SampleID), bz)
}

// GetFitnessRecord returns the fitness record for a sample, or false if not found.
func (k Keeper) GetFitnessRecord(ctx context.Context, sampleID string) (types.TDUFitnessRecord, bool) {
	store := k.storeService.OpenKVStore(ctx)
	bz, err := store.Get(types.FitnessRecordKey(sampleID))
	if err != nil || bz == nil {
		return types.TDUFitnessRecord{}, false
	}
	var record types.TDUFitnessRecord
	if err := json.Unmarshal(bz, &record); err != nil {
		return types.TDUFitnessRecord{}, false
	}
	return record, true
}

// DeleteFitnessRecord removes a fitness record.
func (k Keeper) DeleteFitnessRecord(ctx context.Context, sampleID string) error {
	store := k.storeService.OpenKVStore(ctx)
	return store.Delete(types.FitnessRecordKey(sampleID))
}

// IterateFitnessRecords iterates all fitness records. Return true from cb to stop.
func (k Keeper) IterateFitnessRecords(ctx context.Context, cb func(record types.TDUFitnessRecord) bool) {
	store := k.storeService.OpenKVStore(ctx)
	iter, err := store.Iterator(types.FitnessRecordPrefix, prefixEndBytes(types.FitnessRecordPrefix))
	if err != nil {
		return
	}
	defer iter.Close()
	for ; iter.Valid(); iter.Next() {
		var record types.TDUFitnessRecord
		if err := json.Unmarshal(iter.Value(), &record); err != nil {
			continue
		}
		if cb(record) {
			break
		}
	}
}

// ─── Fitness Decay Params CRUD ──────────────────────────────────────────────

// SetFitnessDecayParams stores the fitness decay parameters as JSON.
func (k Keeper) SetFitnessDecayParams(ctx context.Context, params types.FitnessDecayParams) error {
	store := k.storeService.OpenKVStore(ctx)
	bz, err := params.MarshalJSON()
	if err != nil {
		return fmt.Errorf("failed to marshal fitness decay params: %w", err)
	}
	return store.Set(types.FitnessDecayParamsKey, bz)
}

// GetFitnessDecayParams returns the fitness decay parameters, or defaults if unset.
func (k Keeper) GetFitnessDecayParams(ctx context.Context) types.FitnessDecayParams {
	store := k.storeService.OpenKVStore(ctx)
	bz, err := store.Get(types.FitnessDecayParamsKey)
	if err != nil || bz == nil {
		return types.DefaultFitnessDecayParams()
	}
	var params types.FitnessDecayParams
	if err := params.UnmarshalJSON(bz); err != nil {
		return types.DefaultFitnessDecayParams()
	}
	return params
}

// ─── Lifecycle Status ───────────────────────────────────────────────────────

// GetTDULifecycleStatus returns the lifecycle status for a sample's fitness record.
func (k Keeper) GetTDULifecycleStatus(ctx context.Context, sampleID string) (types.TDULifecycleStatus, error) {
	record, found := k.GetFitnessRecord(ctx, sampleID)
	if !found {
		return types.TDULifecyclePruned, types.ErrFitnessRecordNotFound
	}
	return record.GetLifecycleStatus(), nil
}

// ─── Initialize Fitness ─────────────────────────────────────────────────────

// InitializeFitnessRecord creates a new fitness record for a freshly accepted TDU.
// New TDUs start at fitness score 0.5 (Active status).
func (k Keeper) InitializeFitnessRecord(ctx context.Context, sampleID string, originalStake sdkmath.Int, currentCycle uint64) error {
	record := types.NewTDUFitnessRecord(sampleID, originalStake, currentCycle)
	return k.SetFitnessRecord(ctx, record)
}

// ─── Update Fitness Score ───────────────────────────────────────────────────

// UpdateFitnessScore updates a TDU's fitness score using weighted signal aggregation.
// Weights: training influence 50%, usage correlation 30%, redundancy 20%.
// The new score is blended with the current score: new = 0.5 * current + 0.5 * weighted_signal.
func (k Keeper) UpdateFitnessScore(ctx context.Context, sampleID string, signal types.FitnessSignal, currentCycle uint64) error {
	if err := validateSignal(signal); err != nil {
		return err
	}

	record, found := k.GetFitnessRecord(ctx, sampleID)
	if !found {
		return types.ErrFitnessRecordNotFound
	}

	// Weighted signal aggregation
	weightedScore := signal.TrainingInfluence.Mul(types.SignalWeightTraining).
		Add(signal.UsageCorrelation.Mul(types.SignalWeightUsage)).
		Add(signal.Redundancy.Mul(types.SignalWeightRedundancy))

	// Blend: 50% current + 50% new signal (exponential moving average)
	half := sdkmath.LegacyNewDecWithPrec(5, 1) // 0.5
	currentScore := record.GetFitnessScore()
	blended := currentScore.Mul(half).Add(weightedScore.Mul(half))

	record.SetFitnessScore(blended)
	record.LastSignalCycle = currentCycle
	record.CycleCount++

	return k.SetFitnessRecord(ctx, record)
}

// validateSignal checks that all signal values are in [0, 1].
func validateSignal(signal types.FitnessSignal) error {
	zero := sdkmath.LegacyZeroDec()
	one := sdkmath.LegacyOneDec()
	for _, v := range []sdkmath.LegacyDec{signal.TrainingInfluence, signal.UsageCorrelation, signal.Redundancy} {
		if v.LT(zero) || v.GT(one) {
			return types.ErrInvalidFitnessSignal
		}
	}
	return nil
}

// ─── Decay Unscored ─────────────────────────────────────────────────────────

// DecayUnscored applies fitness decay to TDUs that have received no signal
// for more than UnscoredCycleThreshold cycles. Called during ecology epoch.
func (k Keeper) DecayUnscored(ctx context.Context, currentCycle uint64) {
	params := k.GetFitnessDecayParams(ctx)
	threshold := params.UnscoredCycleThreshold
	decayRate := params.GetDecayPerCycle()

	k.IterateFitnessRecords(ctx, func(record types.TDUFitnessRecord) bool {
		cyclesSinceSignal := currentCycle - record.LastSignalCycle
		if cyclesSinceSignal <= threshold {
			return false
		}

		score := record.GetFitnessScore()
		if score.IsZero() {
			return false
		}

		// Apply decay: score = score - decayRate
		newScore := score.Sub(decayRate)
		if newScore.IsNegative() {
			newScore = sdkmath.LegacyZeroDec()
		}

		record.SetFitnessScore(newScore)
		record.CycleCount++
		_ = k.SetFitnessRecord(ctx, record)
		return false
	})
}

// ─── Longevity Rewards ──────────────────────────────────────────────────────

// ComputeLongevityReward calculates the reward for a TDU based on its lifecycle status.
// Core: 0.01 × original_stake per cycle, Active: 0.005 × original_stake per cycle.
// Dormant and Pruned TDUs earn nothing.
func (k Keeper) ComputeLongevityReward(ctx context.Context, sampleID string) sdkmath.Int {
	record, found := k.GetFitnessRecord(ctx, sampleID)
	if !found {
		return sdkmath.ZeroInt()
	}

	params := k.GetFitnessDecayParams(ctx)
	status := record.GetLifecycleStatus()
	stake := record.GetOriginalStake()

	if stake.IsZero() {
		return sdkmath.ZeroInt()
	}

	var rate sdkmath.LegacyDec
	switch status {
	case types.TDULifecycleCore:
		rate = params.GetCoreRewardRate()
	case types.TDULifecycleActive:
		rate = params.GetActiveRewardRate()
	default:
		return sdkmath.ZeroInt()
	}

	// reward = floor(original_stake × rate)
	reward := rate.MulInt(stake).TruncateInt()
	return reward
}

// DistributeLongevityRewards iterates all fitness records and mints longevity
// rewards for eligible TDUs. Called during ecology epoch processing.
func (k Keeper) DistributeLongevityRewards(ctx context.Context) {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	_ = sdkCtx // available for event emission if needed

	k.IterateFitnessRecords(ctx, func(record types.TDUFitnessRecord) bool {
		reward := k.ComputeLongevityReward(ctx, record.SampleID)
		if reward.IsZero() {
			return false
		}

		// Look up the sample to find the submitter
		sample, ok := k.GetSample(ctx, record.SampleID)
		if !ok {
			return false
		}

		recipient, err := sdk.AccAddressFromBech32(sample.Submitter)
		if err != nil {
			return false
		}

		coins := sdk.NewCoins(sdk.NewCoin("uzrn", reward))
		if err := k.bankKeeper.SendCoinsFromModuleToAccount(ctx, types.ModuleName, recipient, coins); err != nil {
			return false
		}

		return false
	})
}
