package keeper

import (
	"context"
	"encoding/binary"
	"encoding/json"

	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/zerone-chain/zerone/x/alignment/types"
)

// --- Params ---

func (k Keeper) SetParams(ctx context.Context, params *types.Params) {
	st := k.storeService.OpenKVStore(ctx)
	bz, err := json.Marshal(params)
	if err != nil {
		panic("failed to marshal alignment params: " + err.Error())
	}
	if err := st.Set(types.ParamsKey, bz); err != nil {
		panic("failed to set alignment params: " + err.Error())
	}
}

func (k Keeper) GetParams(ctx context.Context) *types.Params {
	st := k.storeService.OpenKVStore(ctx)
	bz, err := st.Get(types.ParamsKey)
	if err != nil || bz == nil {
		params := types.DefaultParams()
		return &params
	}
	var params types.Params
	if err := json.Unmarshal(bz, &params); err != nil {
		p := types.DefaultParams()
		return &p
	}
	return &params
}

// --- State ---

func (k Keeper) SetState(ctx context.Context, state *types.AlignmentState) {
	st := k.storeService.OpenKVStore(ctx)
	bz, err := json.Marshal(state)
	if err != nil {
		panic("failed to marshal alignment state: " + err.Error())
	}
	if err := st.Set(types.StateKey, bz); err != nil {
		panic("failed to set alignment state: " + err.Error())
	}
}

func (k Keeper) GetState(ctx context.Context) *types.AlignmentState {
	st := k.storeService.OpenKVStore(ctx)
	bz, err := st.Get(types.StateKey)
	if err != nil || bz == nil {
		return &types.AlignmentState{Enabled: true}
	}
	var state types.AlignmentState
	if err := json.Unmarshal(bz, &state); err != nil {
		return &types.AlignmentState{Enabled: true}
	}
	return &state
}

// --- Observations ---

func (k Keeper) SetObservation(ctx context.Context, obs *types.AlignmentObservation) {
	st := k.storeService.OpenKVStore(ctx)
	bz, err := json.Marshal(obs)
	if err != nil {
		panic("failed to marshal observation: " + err.Error())
	}
	if err := st.Set(types.ObservationKey(obs.Height), bz); err != nil {
		panic("failed to set observation: " + err.Error())
	}
}

func (k Keeper) GetObservation(ctx context.Context, height uint64) (*types.AlignmentObservation, bool) {
	st := k.storeService.OpenKVStore(ctx)
	bz, err := st.Get(types.ObservationKey(height))
	if err != nil || bz == nil {
		return nil, false
	}
	var obs types.AlignmentObservation
	if err := json.Unmarshal(bz, &obs); err != nil {
		return nil, false
	}
	return &obs, true
}

// --- Scores ---

func (k Keeper) SetScores(ctx context.Context, scores *types.DimensionScores) {
	st := k.storeService.OpenKVStore(ctx)
	bz, err := json.Marshal(scores)
	if err != nil {
		panic("failed to marshal scores: " + err.Error())
	}
	if err := st.Set(types.ScoresKey(scores.Height), bz); err != nil {
		panic("failed to set scores: " + err.Error())
	}
}

func (k Keeper) GetScores(ctx context.Context, height uint64) (*types.DimensionScores, bool) {
	st := k.storeService.OpenKVStore(ctx)
	bz, err := st.Get(types.ScoresKey(height))
	if err != nil || bz == nil {
		return nil, false
	}
	var scores types.DimensionScores
	if err := json.Unmarshal(bz, &scores); err != nil {
		return nil, false
	}
	return &scores, true
}

// --- Health Index ---

func (k Keeper) SetHealthIndex(ctx context.Context, hi *types.AlignmentHealthIndex) {
	st := k.storeService.OpenKVStore(ctx)
	bz, err := json.Marshal(hi)
	if err != nil {
		panic("failed to marshal health index: " + err.Error())
	}
	if err := st.Set(types.HealthIndexKey(hi.Height), bz); err != nil {
		panic("failed to set health index: " + err.Error())
	}
}

func (k Keeper) GetHealthIndex(ctx context.Context, height uint64) (*types.AlignmentHealthIndex, bool) {
	st := k.storeService.OpenKVStore(ctx)
	bz, err := st.Get(types.HealthIndexKey(height))
	if err != nil || bz == nil {
		return nil, false
	}
	var hi types.AlignmentHealthIndex
	if err := json.Unmarshal(bz, &hi); err != nil {
		return nil, false
	}
	return &hi, true
}

// GetRecentHealthIndices returns the most recent health indices in reverse order.
// Max iteration capped at 10,000 entries.
func (k Keeper) GetRecentHealthIndices(ctx context.Context, limit uint32) []*types.AlignmentHealthIndex {
	if limit == 0 || limit > 100 {
		limit = 20
	}

	st := k.storeService.OpenKVStore(ctx)
	iter, err := st.ReverseIterator(types.HealthIndexKeyPrefix, prefixEndBytes(types.HealthIndexKeyPrefix))
	if err != nil {
		return nil
	}
	defer iter.Close()

	var results []*types.AlignmentHealthIndex
	maxIter := 10_000
	count := 0
	for ; iter.Valid() && uint32(len(results)) < limit && count < maxIter; iter.Next() {
		count++
		var hi types.AlignmentHealthIndex
		if err := json.Unmarshal(iter.Value(), &hi); err != nil {
			continue
		}
		results = append(results, &hi)
	}
	return results
}

// --- Corrections ---

func (k Keeper) AddCorrection(ctx context.Context, correction *types.CorrectionRecord) {
	st := k.storeService.OpenKVStore(ctx)

	// Read and increment correction counter.
	count := k.getCorrectionCount(ctx)
	index := uint32(count)

	bz, err := json.Marshal(correction)
	if err != nil {
		panic("failed to marshal correction: " + err.Error())
	}
	if err := st.Set(types.CorrectionKey(correction.Height, index), bz); err != nil {
		panic("failed to set correction: " + err.Error())
	}

	k.setCorrectionCount(ctx, count+1)
}

func (k Keeper) GetCorrections(ctx context.Context, limit, offset uint32) ([]*types.CorrectionRecord, uint64) {
	st := k.storeService.OpenKVStore(ctx)
	total := k.getCorrectionCount(ctx)

	iter, err := st.Iterator(types.CorrectionKeyPrefix, prefixEndBytes(types.CorrectionKeyPrefix))
	if err != nil {
		return nil, total
	}
	defer iter.Close()

	var corrections []*types.CorrectionRecord
	var idx uint32
	for ; iter.Valid(); iter.Next() {
		if idx < offset {
			idx++
			continue
		}
		if limit > 0 && uint32(len(corrections)) >= limit {
			break
		}
		var c types.CorrectionRecord
		if err := json.Unmarshal(iter.Value(), &c); err != nil {
			continue
		}
		corrections = append(corrections, &c)
		idx++
	}

	return corrections, total
}

func (k Keeper) getCorrectionCount(ctx context.Context) uint64 {
	st := k.storeService.OpenKVStore(ctx)
	bz, err := st.Get(types.CorrectionCountKey)
	if err != nil || bz == nil {
		return 0
	}
	return binary.BigEndian.Uint64(bz)
}

func (k Keeper) setCorrectionCount(ctx context.Context, count uint64) {
	st := k.storeService.OpenKVStore(ctx)
	bz := make([]byte, 8)
	binary.BigEndian.PutUint64(bz, count)
	if err := st.Set(types.CorrectionCountKey, bz); err != nil {
		panic("failed to set correction count: " + err.Error())
	}
}

// --- Correction Outcomes ---

func (k Keeper) SetCorrectionOutcome(ctx context.Context, outcome *types.CorrectionOutcome) {
	st := k.storeService.OpenKVStore(ctx)
	bz, err := json.Marshal(outcome)
	if err != nil {
		panic("failed to marshal correction outcome: " + err.Error())
	}
	if err := st.Set(types.CorrectionOutcomeKey(outcome.Height, outcome.Dimension), bz); err != nil {
		panic("failed to set correction outcome: " + err.Error())
	}
}

func (k Keeper) GetCorrectionOutcome(ctx context.Context, height uint64, dimension string) (*types.CorrectionOutcome, bool) {
	st := k.storeService.OpenKVStore(ctx)
	bz, err := st.Get(types.CorrectionOutcomeKey(height, dimension))
	if err != nil || bz == nil {
		return nil, false
	}
	var outcome types.CorrectionOutcome
	if err := json.Unmarshal(bz, &outcome); err != nil {
		return nil, false
	}
	return &outcome, true
}

// GetCorrectionsAtHeight returns all correction outcomes recorded at a given height.
func (k Keeper) GetCorrectionsAtHeight(ctx context.Context, height uint64) []*types.CorrectionOutcome {
	st := k.storeService.OpenKVStore(ctx)
	prefix := types.CorrectionOutcomeHeightPrefix(height)
	iter, err := st.Iterator(prefix, prefixEndBytes(prefix))
	if err != nil {
		return nil
	}
	defer iter.Close()

	var outcomes []*types.CorrectionOutcome
	for ; iter.Valid(); iter.Next() {
		var o types.CorrectionOutcome
		if json.Unmarshal(iter.Value(), &o) == nil {
			outcomes = append(outcomes, &o)
		}
	}
	return outcomes
}

// GetRecentCorrectionOutcomes returns the most recent N evaluated correction outcomes.
func (k Keeper) GetRecentCorrectionOutcomes(ctx context.Context, windowSize uint64) []*types.CorrectionOutcome {
	st := k.storeService.OpenKVStore(ctx)
	iter, err := st.ReverseIterator(types.CorrectionOutcomeKeyPrefix, prefixEndBytes(types.CorrectionOutcomeKeyPrefix))
	if err != nil {
		return nil
	}
	defer iter.Close()

	var outcomes []*types.CorrectionOutcome
	maxIter := 10_000
	count := 0
	for ; iter.Valid() && uint64(len(outcomes)) < windowSize && count < maxIter; iter.Next() {
		count++
		var o types.CorrectionOutcome
		if json.Unmarshal(iter.Value(), &o) == nil {
			if o.ScoreAfter > 0 { // only include evaluated outcomes
				outcomes = append(outcomes, &o)
			}
		}
	}
	return outcomes
}

// --- Genesis ---

func (k Keeper) InitGenesis(ctx context.Context, genState *types.GenesisState) {
	if genState.Params != nil {
		k.SetParams(ctx, genState.Params)
	}
	if genState.State != nil {
		k.SetState(ctx, genState.State)
	}
	for _, obs := range genState.Observations {
		if obs != nil {
			k.SetObservation(ctx, obs)
		}
	}
	for _, s := range genState.Scores {
		if s != nil {
			k.SetScores(ctx, s)
		}
	}
	for _, hi := range genState.HealthIndices {
		if hi != nil {
			k.SetHealthIndex(ctx, hi)
		}
	}
	for _, c := range genState.Corrections {
		if c != nil {
			k.AddCorrection(ctx, c)
		}
	}
}

func (k Keeper) ExportGenesis(ctx context.Context) *types.GenesisState {
	params := k.GetParams(ctx)
	state := k.GetState(ctx)

	var observations []*types.AlignmentObservation
	k.iteratePrefix(ctx, types.ObservationKeyPrefix, func(bz []byte) bool {
		var obs types.AlignmentObservation
		if json.Unmarshal(bz, &obs) == nil {
			observations = append(observations, &obs)
		}
		return false
	})

	var scores []*types.DimensionScores
	k.iteratePrefix(ctx, types.ScoresKeyPrefix, func(bz []byte) bool {
		var s types.DimensionScores
		if json.Unmarshal(bz, &s) == nil {
			scores = append(scores, &s)
		}
		return false
	})

	var healthIndices []*types.AlignmentHealthIndex
	k.iteratePrefix(ctx, types.HealthIndexKeyPrefix, func(bz []byte) bool {
		var hi types.AlignmentHealthIndex
		if json.Unmarshal(bz, &hi) == nil {
			healthIndices = append(healthIndices, &hi)
		}
		return false
	})

	corrections, _ := k.GetCorrections(ctx, 0, 0)

	return &types.GenesisState{
		Params:        params,
		State:         state,
		Observations:  observations,
		Scores:        scores,
		HealthIndices: healthIndices,
		Corrections:   corrections,
	}
}

func (k Keeper) iteratePrefix(ctx context.Context, prefix []byte, cb func(bz []byte) bool) {
	st := k.storeService.OpenKVStore(ctx)
	iter, err := st.Iterator(prefix, prefixEndBytes(prefix))
	if err != nil {
		return
	}
	defer iter.Close()
	for ; iter.Valid(); iter.Next() {
		if cb(iter.Value()) {
			break
		}
	}
}

// IsEnabled returns true if the alignment module is operational.
func (k Keeper) IsEnabled(ctx context.Context) bool {
	params := k.GetParams(ctx)
	state := k.GetState(ctx)
	return params.Enabled && state.Enabled
}

// IsHalted checks if the chain is in emergency halt (nil-safe).
func (k Keeper) IsHalted(ctx context.Context) bool {
	if k.emergencyKeeper == nil {
		return false
	}
	return k.emergencyKeeper.IsHalted(ctx)
}

// GetLastObservationHeight returns the height of the last observation.
func (k Keeper) GetLastObservationHeight(ctx context.Context) uint64 {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	_ = sdkCtx
	state := k.GetState(ctx)
	return state.LastObservationHeight
}
