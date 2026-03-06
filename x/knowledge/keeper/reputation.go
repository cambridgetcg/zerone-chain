package keeper

import (
	"context"
	"encoding/json"
	"fmt"

	sdkmath "cosmossdk.io/math"

	"github.com/zerone-chain/zerone/x/knowledge/types"
)

// ─── Reputation Decay Params CRUD ───────────────────────────────────────────

// SetReputationDecayParams stores the reputation decay parameters as JSON.
func (k Keeper) SetReputationDecayParams(ctx context.Context, params types.ReputationDecayParams) error {
	store := k.storeService.OpenKVStore(ctx)
	bz, err := params.MarshalJSON()
	if err != nil {
		return fmt.Errorf("failed to marshal reputation decay params: %w", err)
	}
	return store.Set(types.ReputationDecayParamsKey, bz)
}

// GetReputationDecayParams returns the reputation decay parameters, or defaults if unset.
func (k Keeper) GetReputationDecayParams(ctx context.Context) types.ReputationDecayParams {
	store := k.storeService.OpenKVStore(ctx)
	bz, err := store.Get(types.ReputationDecayParamsKey)
	if err != nil || bz == nil {
		return types.DefaultReputationDecayParams()
	}
	var params types.ReputationDecayParams
	if err := params.UnmarshalJSON(bz); err != nil {
		return types.DefaultReputationDecayParams()
	}
	return params
}

// ─── Agent Domain Reputation CRUD ───────────────────────────────────────────

// SetAgentDomainReputation stores an agent's domain reputation as JSON.
func (k Keeper) SetAgentDomainReputation(ctx context.Context, rep types.AgentDomainReputation) error {
	store := k.storeService.OpenKVStore(ctx)
	bz, err := json.Marshal(rep)
	if err != nil {
		return fmt.Errorf("failed to marshal agent domain reputation: %w", err)
	}
	return store.Set(types.AgentDomainReputationKey(rep.AgentAddr, rep.DomainID), bz)
}

// GetAgentDomainReputation returns an agent's domain reputation, or false if not found.
func (k Keeper) GetAgentDomainReputation(ctx context.Context, agentAddr, domainID string) (types.AgentDomainReputation, bool) {
	store := k.storeService.OpenKVStore(ctx)
	bz, err := store.Get(types.AgentDomainReputationKey(agentAddr, domainID))
	if err != nil || bz == nil {
		return types.AgentDomainReputation{}, false
	}
	var rep types.AgentDomainReputation
	if err := json.Unmarshal(bz, &rep); err != nil {
		return types.AgentDomainReputation{}, false
	}
	return rep, true
}

// DeleteAgentDomainReputation removes an agent's domain reputation.
func (k Keeper) DeleteAgentDomainReputation(ctx context.Context, agentAddr, domainID string) error {
	store := k.storeService.OpenKVStore(ctx)
	return store.Delete(types.AgentDomainReputationKey(agentAddr, domainID))
}

// IterateAgentDomainReputations iterates all agent-domain reputation records. Return true from cb to stop.
func (k Keeper) IterateAgentDomainReputations(ctx context.Context, cb func(rep types.AgentDomainReputation) bool) {
	store := k.storeService.OpenKVStore(ctx)
	iter, err := store.Iterator(types.AgentDomainReputationPrefix, prefixEndBytes(types.AgentDomainReputationPrefix))
	if err != nil {
		return
	}
	defer iter.Close()
	for ; iter.Valid(); iter.Next() {
		var rep types.AgentDomainReputation
		if err := json.Unmarshal(iter.Value(), &rep); err != nil {
			continue
		}
		if cb(rep) {
			break
		}
	}
}

// ─── Reputation Operations ──────────────────────────────────────────────────

// UpdateReputation adds or subtracts delta to an agent's domain reputation, tracking peak.
// If the record doesn't exist, it is created with the delta as the initial score.
func (k Keeper) UpdateReputation(ctx context.Context, agentAddr, domainID string, delta sdkmath.LegacyDec, currentHeight int64) error {
	rep, found := k.GetAgentDomainReputation(ctx, agentAddr, domainID)
	if !found {
		// Initialize new record
		initialScore := delta
		if initialScore.IsNegative() {
			initialScore = sdkmath.LegacyZeroDec()
		}
		rep = types.NewAgentDomainReputation(agentAddr, domainID, initialScore, currentHeight)
		return k.SetAgentDomainReputation(ctx, rep)
	}

	newScore := rep.GetScore().Add(delta)
	if newScore.IsNegative() {
		newScore = sdkmath.LegacyZeroDec()
	}
	rep.SetScore(newScore)

	// Track peak
	if newScore.GT(rep.GetPeakScore()) {
		rep.SetPeakScore(newScore)
	}

	// Activity resets inactivity timer
	rep.LastActiveHeight = currentHeight

	return k.SetAgentDomainReputation(ctx, rep)
}

// ResetInactivityTimer resets the inactivity timer for an agent in a domain.
// Called on successful submission or review.
func (k Keeper) ResetInactivityTimer(ctx context.Context, agentAddr, domainID string, currentHeight int64) {
	rep, found := k.GetAgentDomainReputation(ctx, agentAddr, domainID)
	if !found {
		return
	}
	rep.LastActiveHeight = currentHeight
	_ = k.SetAgentDomainReputation(ctx, rep)
}

// ApplyReputationDecay iterates all agent-domain reputation records and applies
// decay to agents inactive for longer than DecayIntervalBlocks. Score is decayed
// by DecayRateBps but never below FloorRatioBps × PeakScore.
func (k Keeper) ApplyReputationDecay(ctx context.Context, currentHeight int64) {
	params := k.GetReputationDecayParams(ctx)
	decayRate := params.GetDecayRate()
	floorRatio := params.GetFloorRatio()
	interval := int64(params.DecayIntervalBlocks)

	if interval <= 0 {
		return
	}

	// Collect records to update (avoid mutating store during iteration)
	var toUpdate []types.AgentDomainReputation
	k.IterateAgentDomainReputations(ctx, func(rep types.AgentDomainReputation) bool {
		inactiveBlocks := currentHeight - rep.LastActiveHeight
		if inactiveBlocks <= interval {
			return false
		}

		score := rep.GetScore()
		if score.IsZero() {
			return false
		}

		// Compute floor: floorRatio × peakScore
		peak := rep.GetPeakScore()
		floor := floorRatio.Mul(peak)

		// Already at or below floor
		if score.LTE(floor) {
			return false
		}

		// Number of full intervals of inactivity
		periods := inactiveBlocks / interval

		// Apply decay: score × (1 - decayRate)^periods
		multiplier := sdkmath.LegacyOneDec().Sub(decayRate)
		for i := int64(0); i < periods; i++ {
			score = score.Mul(multiplier)
		}

		// Enforce floor
		if score.LT(floor) {
			score = floor
		}

		rep.SetScore(score)
		toUpdate = append(toUpdate, rep)
		return false
	})

	for _, rep := range toUpdate {
		_ = k.SetAgentDomainReputation(ctx, rep)
	}
}
