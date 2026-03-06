package keeper

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"

	sdkmath "cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/zerone-chain/zerone/x/knowledge/types"
)

// ─── PromoteModelToAgent ────────────────────────────────────────────────────

// PromoteModelToAgent creates an autonomous agent from a trained model.
// The agent gets its own wallet, reputation, and capability set.
// This is the key primitive for the recursive self-improvement loop:
// data → training → model → agent → better data → better model → ...
func (k Keeper) PromoteModelToAgent(ctx context.Context, msg *types.MsgPromoteModel) (*types.MsgPromoteModelResponse, error) {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	kvStore := k.storeService.OpenKVStore(ctx)

	// Parse stake.
	stake, ok := sdkmath.NewIntFromString(msg.Stake)
	if !ok {
		return nil, types.ErrAgentInsufficientStake.Wrapf("invalid stake: %s", msg.Stake)
	}
	if stake.LT(types.AgentMinStake) {
		return nil, types.ErrAgentInsufficientStake.Wrapf("stake %s < minimum %s", stake, types.AgentMinStake)
	}

	// Get source model.
	model, found := k.GetModelRecord(ctx, msg.ModelID)
	if !found {
		return nil, types.ErrModelNotFound.Wrapf("model %s", msg.ModelID)
	}
	if model.Status != types.ModelStatusActive {
		return nil, types.ErrModelNotActive.Wrapf("model %s has status %s", msg.ModelID, model.Status)
	}

	// Quality gate — higher bar than publishing.
	if model.GetBenchmarkScore().LT(types.AgentMinBenchmarkScore) {
		return nil, types.ErrModelQualityTooLow.Wrapf(
			"benchmark %s < agent minimum %s",
			model.BenchmarkScore, types.AgentMinBenchmarkScore,
		)
	}
	if model.TDUCount < types.AgentMinTDUCount {
		return nil, types.ErrModelQualityTooLow.Wrapf(
			"TDU count %d < agent minimum %d",
			model.TDUCount, types.AgentMinTDUCount,
		)
	}

	// Check not already promoted.
	existingAgentID, err := kvStore.Get(types.AgentModelIndexKey(msg.ModelID))
	if err != nil {
		return nil, fmt.Errorf("failed to check agent index: %w", err)
	}
	if existingAgentID != nil {
		return nil, types.ErrAgentAlreadyExists.Wrapf("model %s already promoted to agent %s", msg.ModelID, string(existingAgentID))
	}

	// Determine generation from model lineage.
	generation := uint64(0) // human-trained model = gen 0
	lineage, hasLineage := k.GetModelLineage(ctx, msg.ModelID)
	if hasLineage {
		// Check if any ancestor model was trained by agent-curated data.
		// For now, generation = lineage generation (conservative).
		generation = lineage.Generation - 1 // lineage.Generation is 1-based
	}

	if generation >= types.AgentMaxGeneration {
		return nil, types.ErrAgentMaxGeneration.Wrapf("generation %d >= max %d", generation, types.AgentMaxGeneration)
	}

	// Derive deterministic identity.
	agentID := types.DeriveAgentID(msg.ModelID)
	address := types.DeriveAgentAddress(model.ModelHash)

	// Initial reputation = benchmark score × 0.5 (start conservative).
	initialRep := model.GetBenchmarkScore().Mul(sdkmath.LegacyNewDecWithPrec(5, 1))

	// Determine capabilities based on benchmark.
	canReview := model.GetBenchmarkScore().GTE(types.AgentMinBenchmarkScore)

	agent := types.AgentIdentity{
		AgentID:      agentID,
		ModelID:      msg.ModelID,
		Address:      address,
		Domain:       model.Domain,
		Generation:   generation,
		CanSubmit:    true,       // all agents can submit
		CanReview:    canReview,  // benchmark ≥ 0.6
		CanTrain:     false,      // requires validator backing (future upgrade)
		Status:       types.AgentStatusActive,
		PromotedAt:   sdkCtx.BlockHeight(),
		SponsorAddr:  msg.Sponsor,
		InitialStake: stake.String(),
	}
	agent.SetReputation(initialRep)
	agent.EarningsTotal = sdkmath.ZeroInt().String()

	// Build agent lineage — check parent agents.
	if hasLineage && len(lineage.Ancestors) > 0 {
		for _, ancestorModelID := range lineage.Ancestors {
			ancestorAgent, found := k.GetAgentByModel(ctx, ancestorModelID)
			if found {
				agent.ParentAgentID = ancestorAgent.AgentID
				agent.Lineage = append([]string{ancestorAgent.AgentID}, ancestorAgent.Lineage...)
				break
			}
		}
	}

	// Store agent.
	if err := k.setAgentIdentity(ctx, &agent); err != nil {
		return nil, err
	}

	// Index: modelID → agentID.
	if err := kvStore.Set(types.AgentModelIndexKey(msg.ModelID), []byte(agentID)); err != nil {
		return nil, fmt.Errorf("failed to set model index: %w", err)
	}

	// Index: domain → agentID.
	if err := kvStore.Set(types.AgentDomainIndexKey(model.Domain, agentID), []byte{0x01}); err != nil {
		return nil, fmt.Errorf("failed to set domain index: %w", err)
	}

	// Index: generation → agentID.
	if err := kvStore.Set(types.AgentGenerationIndexKey(generation, agentID), []byte{0x01}); err != nil {
		return nil, fmt.Errorf("failed to set generation index: %w", err)
	}

	// Index: active.
	if err := kvStore.Set(types.AgentActiveIndexKey(agentID), []byte{0x01}); err != nil {
		return nil, fmt.Errorf("failed to set active index: %w", err)
	}

	sdkCtx.EventManager().EmitEvent(sdk.NewEvent(
		types.EventAgentPromoted,
		sdk.NewAttribute(types.AttributeAgentID, agentID),
		sdk.NewAttribute(types.AttributeModelID, msg.ModelID),
		sdk.NewAttribute(types.AttributeAgentGeneration, strconv.FormatUint(generation, 10)),
		sdk.NewAttribute(types.AttributeAgentSponsor, msg.Sponsor),
		sdk.NewAttribute(types.AttributeAgentAddress, address),
	))

	return &types.MsgPromoteModelResponse{
		AgentID: agentID,
		Address: address,
	}, nil
}

// ─── Lifecycle ──────────────────────────────────────────────────────────────

// SuspendAgent marks an agent as suspended (poor performance).
func (k Keeper) SuspendAgent(ctx context.Context, agentID string, reason string) error {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	kvStore := k.storeService.OpenKVStore(ctx)

	agent, found := k.GetAgentIdentity(ctx, agentID)
	if !found {
		return types.ErrAgentNotFound.Wrapf("agent %s", agentID)
	}
	if agent.Status != types.AgentStatusActive {
		return types.ErrAgentNotActive.Wrapf("agent %s has status %s", agentID, agent.Status)
	}

	agent.Status = types.AgentStatusSuspended
	agent.SuspendedAt = sdkCtx.BlockHeight()

	if err := k.setAgentIdentity(ctx, &agent); err != nil {
		return err
	}

	// Remove from active index.
	if err := kvStore.Delete(types.AgentActiveIndexKey(agentID)); err != nil {
		return fmt.Errorf("failed to remove active index: %w", err)
	}

	sdkCtx.EventManager().EmitEvent(sdk.NewEvent(
		types.EventAgentSuspended,
		sdk.NewAttribute(types.AttributeAgentID, agentID),
	))

	return nil
}

// RetireAgent marks an agent as retired (source model deprecated).
func (k Keeper) RetireAgent(ctx context.Context, agentID string) error {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	kvStore := k.storeService.OpenKVStore(ctx)

	agent, found := k.GetAgentIdentity(ctx, agentID)
	if !found {
		return types.ErrAgentNotFound.Wrapf("agent %s", agentID)
	}

	agent.Status = types.AgentStatusRetired

	if err := k.setAgentIdentity(ctx, &agent); err != nil {
		return err
	}

	if err := kvStore.Delete(types.AgentActiveIndexKey(agentID)); err != nil {
		return fmt.Errorf("failed to remove active index: %w", err)
	}

	sdkCtx.EventManager().EmitEvent(sdk.NewEvent(
		types.EventAgentRetired,
		sdk.NewAttribute(types.AttributeAgentID, agentID),
	))

	return nil
}

// RecordAgentAction increments the agent's task counter.
func (k Keeper) RecordAgentAction(ctx context.Context, agentID string) error {
	agent, found := k.GetAgentIdentity(ctx, agentID)
	if !found {
		return types.ErrAgentNotFound.Wrapf("agent %s", agentID)
	}
	agent.TasksComplete++
	return k.setAgentIdentity(ctx, &agent)
}

// AddAgentEarnings adds earnings to an agent's total.
func (k Keeper) AddAgentEarnings(ctx context.Context, agentID string, amount sdkmath.Int) error {
	agent, found := k.GetAgentIdentity(ctx, agentID)
	if !found {
		return types.ErrAgentNotFound.Wrapf("agent %s", agentID)
	}
	current := agent.GetEarningsTotal()
	agent.EarningsTotal = current.Add(amount).String()
	return k.setAgentIdentity(ctx, &agent)
}

// UpdateAgentReputation sets the agent's reputation, auto-suspending if below threshold.
func (k Keeper) UpdateAgentReputation(ctx context.Context, agentID string, newRep sdkmath.LegacyDec) error {
	agent, found := k.GetAgentIdentity(ctx, agentID)
	if !found {
		return types.ErrAgentNotFound.Wrapf("agent %s", agentID)
	}

	agent.SetReputation(newRep)

	// Auto-suspend if reputation drops below threshold.
	if agent.Status == types.AgentStatusActive && newRep.LT(types.AgentSuspensionThreshold) {
		return k.SuspendAgent(ctx, agentID, "reputation below threshold")
	}

	return k.setAgentIdentity(ctx, &agent)
}

// ─── Queries ────────────────────────────────────────────────────────────────

// GetAgentIdentity retrieves an agent by ID.
func (k Keeper) GetAgentIdentity(ctx context.Context, agentID string) (types.AgentIdentity, bool) {
	kvStore := k.storeService.OpenKVStore(ctx)
	key := types.AgentIdentityKey(agentID)

	bz, err := kvStore.Get(key)
	if err != nil || bz == nil {
		return types.AgentIdentity{}, false
	}

	var agent types.AgentIdentity
	if err := json.Unmarshal(bz, &agent); err != nil {
		return types.AgentIdentity{}, false
	}
	return agent, true
}

// GetAgentByModel retrieves the agent promoted from a specific model.
func (k Keeper) GetAgentByModel(ctx context.Context, modelID string) (types.AgentIdentity, bool) {
	kvStore := k.storeService.OpenKVStore(ctx)
	agentIDBz, err := kvStore.Get(types.AgentModelIndexKey(modelID))
	if err != nil || agentIDBz == nil {
		return types.AgentIdentity{}, false
	}
	return k.GetAgentIdentity(ctx, string(agentIDBz))
}

// GetAgentsByDomain returns all agents in a domain.
func (k Keeper) GetAgentsByDomain(ctx context.Context, domain string) []types.AgentIdentity {
	kvStore := k.storeService.OpenKVStore(ctx)
	prefix := types.AgentDomainByDomainPrefix(domain)

	var agents []types.AgentIdentity
	iter, err := kvStore.Iterator(prefix, prefixEndBytes(prefix))
	if err != nil {
		return nil
	}
	defer iter.Close()

	for ; iter.Valid(); iter.Next() {
		agentID := string(iter.Key()[len(prefix):])
		agent, found := k.GetAgentIdentity(ctx, agentID)
		if found {
			agents = append(agents, agent)
		}
	}
	return agents
}

// GetAgentsByGeneration returns all agents at a specific generation depth.
func (k Keeper) GetAgentsByGeneration(ctx context.Context, gen uint64) []types.AgentIdentity {
	kvStore := k.storeService.OpenKVStore(ctx)
	prefix := types.AgentGenerationPrefix(gen)

	var agents []types.AgentIdentity
	iter, err := kvStore.Iterator(prefix, prefixEndBytes(prefix))
	if err != nil {
		return nil
	}
	defer iter.Close()

	for ; iter.Valid(); iter.Next() {
		agentID := string(iter.Key()[len(prefix):])
		agent, found := k.GetAgentIdentity(ctx, agentID)
		if found {
			agents = append(agents, agent)
		}
	}
	return agents
}

// GetActiveAgents returns all active agents.
func (k Keeper) GetActiveAgents(ctx context.Context) []types.AgentIdentity {
	kvStore := k.storeService.OpenKVStore(ctx)
	prefix := types.AgentActiveIdxPrefix

	var agents []types.AgentIdentity
	iter, err := kvStore.Iterator(prefix, prefixEndBytes(prefix))
	if err != nil {
		return nil
	}
	defer iter.Close()

	for ; iter.Valid(); iter.Next() {
		agentID := string(iter.Key()[len(prefix):])
		agent, found := k.GetAgentIdentity(ctx, agentID)
		if found {
			agents = append(agents, agent)
		}
	}
	return agents
}

// GetGenerationStats returns a count of agents per generation.
func (k Keeper) GetGenerationStats(ctx context.Context) map[uint64]uint64 {
	stats := make(map[uint64]uint64)
	// Iterate through all possible generations (0 to max).
	for gen := uint64(0); gen <= types.AgentMaxGeneration; gen++ {
		agents := k.GetAgentsByGeneration(ctx, gen)
		if len(agents) > 0 {
			stats[gen] = uint64(len(agents))
		}
	}
	return stats
}

// ─── Internal ───────────────────────────────────────────────────────────────

func (k Keeper) setAgentIdentity(ctx context.Context, agent *types.AgentIdentity) error {
	kvStore := k.storeService.OpenKVStore(ctx)
	key := types.AgentIdentityKey(agent.AgentID)

	bz, err := json.Marshal(agent)
	if err != nil {
		return fmt.Errorf("failed to marshal agent identity: %w", err)
	}
	return kvStore.Set(key, bz)
}
