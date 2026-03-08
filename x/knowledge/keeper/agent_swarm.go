package keeper

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"strconv"

	sdkmath "cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/zerone-chain/zerone/x/knowledge/types"
)

// ─── R55: Agent Swarms — Collective Intelligence ────────────────────────────
//
// Solo agents are limited by their model's perspective. Swarms of diverse
// agents coordinate curation roles, pool resources, and collectively
// train models better than any individual could.
//
// The swarm IS a higher-order intelligence:
//   - Curators submit diverse data (breadth)
//   - Reviewers ensure quality (depth)
//   - Strategists identify gaps (direction) — R54 integration
//   - Trainers assemble and launch model training (output)
//
// The swarm's collective output trains a model available via API (R51),
// which ALL members can then access. The swarm makes itself smarter.

// ─── FormSwarm ──────────────────────────────────────────────────────────────

// FormSwarm creates a new agent swarm. The creating agent becomes the first member.
func (k Keeper) FormSwarm(ctx context.Context, creatorAgentID, domain, name string, role types.SwarmRole) (string, error) {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	params := k.GetSwarmParams(ctx)

	// Validate creator.
	agent, found := k.GetAgentIdentity(ctx, creatorAgentID)
	if !found {
		return "", types.ErrAgentNotFound.Wrapf("agent %s", creatorAgentID)
	}
	if agent.Status != types.AgentStatusActive {
		return "", types.ErrAgentNotActive.Wrapf("agent %s", creatorAgentID)
	}

	// Validate role.
	if !types.ValidSwarmRoles[role] {
		return "", fmt.Errorf("invalid swarm role: %s", role)
	}

	// Validate domain.
	if domain != "" {
		if _, found := k.GetDomain(ctx, domain); !found {
			return "", types.ErrDomainNotFound.Wrapf("domain %s", domain)
		}
	}

	// Generate swarm ID.
	swarmID := k.nextSwarmID(ctx)
	treasuryAddr := types.DeriveSwarmTreasury(swarmID)

	swarm := &types.AgentSwarm{
		SwarmID:    swarmID,
		Name:       name,
		Domain:     domain,
		MinMembers: params.MinSwarmSize,
		MaxMembers: params.MaxSwarmSize,
		Members: []types.SwarmMember{
			{
				AgentID:  creatorAgentID,
				Role:     role,
				JoinedAt: uint64(sdkCtx.BlockHeight()),
			},
		},
		TreasuryBalance: "0",
		TreasuryAddr:    treasuryAddr,
		FormedAt:        uint64(sdkCtx.BlockHeight()),
		Status:          types.SwarmStatusForming,
		Creator:         creatorAgentID,
	}

	// Compute initial collective reputation.
	swarm.CollectiveReputation = agent.Reputation

	if err := k.setAgentSwarm(ctx, swarm); err != nil {
		return "", err
	}

	sdkCtx.EventManager().EmitEvent(sdk.NewEvent(
		types.EventSwarmFormed,
		sdk.NewAttribute(types.AttributeSwarmID, swarmID),
		sdk.NewAttribute(types.AttributeSwarmName, name),
		sdk.NewAttribute("domain", domain),
		sdk.NewAttribute(types.AttributeAgentID, creatorAgentID),
		sdk.NewAttribute(types.AttributeTreasuryAddr, treasuryAddr),
	))

	return swarmID, nil
}

// ─── JoinSwarm ──────────────────────────────────────────────────────────────

// JoinSwarm adds an agent to an existing swarm with a specified role.
func (k Keeper) JoinSwarm(ctx context.Context, swarmID, agentID string, role types.SwarmRole) error {
	sdkCtx := sdk.UnwrapSDKContext(ctx)

	swarm, found := k.GetAgentSwarm(ctx, swarmID)
	if !found {
		return fmt.Errorf("swarm %s not found", swarmID)
	}
	if swarm.Status == types.SwarmStatusDissolved {
		return fmt.Errorf("swarm %s is dissolved", swarmID)
	}
	if swarm.HasMember(agentID) {
		return fmt.Errorf("agent %s is already a member", agentID)
	}
	if swarm.MemberCount() >= swarm.MaxMembers {
		return fmt.Errorf("swarm %s is full (%d/%d)", swarmID, swarm.MemberCount(), swarm.MaxMembers)
	}

	// Validate agent.
	agent, found := k.GetAgentIdentity(ctx, agentID)
	if !found {
		return types.ErrAgentNotFound.Wrapf("agent %s", agentID)
	}
	if agent.Status != types.AgentStatusActive {
		return types.ErrAgentNotActive.Wrapf("agent %s", agentID)
	}

	// Validate role.
	if !types.ValidSwarmRoles[role] {
		return fmt.Errorf("invalid role: %s", role)
	}

	// Add member.
	swarm.Members = append(swarm.Members, types.SwarmMember{
		AgentID:  agentID,
		Role:     role,
		JoinedAt: uint64(sdkCtx.BlockHeight()),
	})

	// Recompute collective reputation.
	swarm.CollectiveReputation = k.computeCollectiveReputation(ctx, swarm).String()

	// Auto-activate if minimum reached.
	if swarm.Status == types.SwarmStatusForming && swarm.MemberCount() >= swarm.MinMembers {
		swarm.Status = types.SwarmStatusActive
		sdkCtx.EventManager().EmitEvent(sdk.NewEvent(
			types.EventSwarmActivated,
			sdk.NewAttribute(types.AttributeSwarmID, swarmID),
			sdk.NewAttribute("members", strconv.FormatUint(swarm.MemberCount(), 10)),
		))
	}

	if err := k.setAgentSwarm(ctx, swarm); err != nil {
		return err
	}

	sdkCtx.EventManager().EmitEvent(sdk.NewEvent(
		types.EventSwarmMemberJoined,
		sdk.NewAttribute(types.AttributeSwarmID, swarmID),
		sdk.NewAttribute(types.AttributeAgentID, agentID),
		sdk.NewAttribute(types.AttributeMemberRole, string(role)),
	))

	return nil
}

// ─── LeaveSwarm ─────────────────────────────────────────────────────────────

// LeaveSwarm removes an agent from a swarm. If below minimum, swarm deactivates.
func (k Keeper) LeaveSwarm(ctx context.Context, swarmID, agentID string) error {
	sdkCtx := sdk.UnwrapSDKContext(ctx)

	swarm, found := k.GetAgentSwarm(ctx, swarmID)
	if !found {
		return fmt.Errorf("swarm %s not found", swarmID)
	}
	if !swarm.HasMember(agentID) {
		return fmt.Errorf("agent %s is not a member of swarm %s", agentID, swarmID)
	}

	// Remove member.
	var remaining []types.SwarmMember
	for _, m := range swarm.Members {
		if m.AgentID != agentID {
			remaining = append(remaining, m)
		}
	}
	swarm.Members = remaining

	// Recompute collective reputation.
	if len(swarm.Members) > 0 {
		swarm.CollectiveReputation = k.computeCollectiveReputation(ctx, swarm).String()
	}

	// Check if below minimum.
	if swarm.MemberCount() < swarm.MinMembers && swarm.Status == types.SwarmStatusActive {
		swarm.Status = types.SwarmStatusForming // deactivate until more join
	}

	// Auto-dissolve if empty.
	if swarm.MemberCount() == 0 {
		swarm.Status = types.SwarmStatusDissolved
		swarm.DissolvedAt = uint64(sdkCtx.BlockHeight())
	}

	// Remove member index.
	kvStore := k.storeService.OpenKVStore(ctx)
	_ = kvStore.Delete(types.SwarmByMemberKey(agentID, swarmID))

	if err := k.setAgentSwarm(ctx, swarm); err != nil {
		return err
	}

	sdkCtx.EventManager().EmitEvent(sdk.NewEvent(
		types.EventSwarmMemberLeft,
		sdk.NewAttribute(types.AttributeSwarmID, swarmID),
		sdk.NewAttribute(types.AttributeAgentID, agentID),
	))

	return nil
}

// ─── SetSwarmObjective ──────────────────────────────────────────────────────

// SetSwarmObjective creates a coordinated goal for the swarm.
// Objectives can link to knowledge gaps (R54) or bounties (R47).
func (k Keeper) SetSwarmObjective(ctx context.Context, swarmID string, objective *types.SwarmObjective) (string, error) {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	params := k.GetSwarmParams(ctx)

	swarm, found := k.GetAgentSwarm(ctx, swarmID)
	if !found {
		return "", fmt.Errorf("swarm %s not found", swarmID)
	}
	if swarm.Status != types.SwarmStatusActive {
		return "", fmt.Errorf("swarm %s is not active (status: %s)", swarmID, swarm.Status)
	}

	// Check objective limit.
	existing := k.GetSwarmObjectives(ctx, swarmID)
	activeCount := uint64(0)
	for _, obj := range existing {
		if obj.Status == "active" {
			activeCount++
		}
	}
	if activeCount >= params.MaxObjectives {
		return "", fmt.Errorf("swarm has %d active objectives (max %d)", activeCount, params.MaxObjectives)
	}

	objectiveID := k.nextSwarmObjectiveID(ctx)
	objective.ObjectiveID = objectiveID
	objective.SwarmID = swarmID
	objective.Status = "active"
	objective.CreatedAt = uint64(sdkCtx.BlockHeight())

	if err := k.setSwarmObjective(ctx, objective); err != nil {
		return "", err
	}

	sdkCtx.EventManager().EmitEvent(sdk.NewEvent(
		types.EventSwarmObjectiveSet,
		sdk.NewAttribute(types.AttributeSwarmID, swarmID),
		sdk.NewAttribute(types.AttributeObjectiveID, objectiveID),
	))

	return objectiveID, nil
}

// ─── RecordSwarmContribution ────────────────────────────────────────────────

// RecordSwarmContribution records a member's work toward a swarm objective.
// Called when an agent completes a task on behalf of the swarm.
func (k Keeper) RecordSwarmContribution(ctx context.Context, swarmID, agentID string, tdusSubmitted, reviewsDone uint64) error {
	swarm, found := k.GetAgentSwarm(ctx, swarmID)
	if !found {
		return fmt.Errorf("swarm %s not found", swarmID)
	}

	// Update member stats.
	memberUpdated := false
	for i, m := range swarm.Members {
		if m.AgentID == agentID {
			swarm.Members[i].TDUsSubmitted += tdusSubmitted
			swarm.Members[i].ReviewsDone += reviewsDone
			swarm.Members[i].TasksCompleted++
			memberUpdated = true
			break
		}
	}
	if !memberUpdated {
		return fmt.Errorf("agent %s is not a member of swarm %s", agentID, swarmID)
	}

	// Update swarm totals.
	swarm.TDUsCurated += tdusSubmitted

	// Recompute member contributions (share of total work).
	totalWork := uint64(0)
	for _, m := range swarm.Members {
		totalWork += m.TDUsSubmitted + m.ReviewsDone
	}
	if totalWork > 0 {
		for i, m := range swarm.Members {
			memberWork := m.TDUsSubmitted + m.ReviewsDone
			share := sdkmath.LegacyNewDec(int64(memberWork)).Quo(sdkmath.LegacyNewDec(int64(totalWork)))
			swarm.Members[i].Contribution = share.String()
		}
	}

	return k.setAgentSwarm(ctx, swarm)
}

// ─── CompleteObjective ──────────────────────────────────────────────────────

// CompleteObjective marks a swarm objective as completed and triggers reward distribution.
func (k Keeper) CompleteObjective(ctx context.Context, objectiveID string) error {
	sdkCtx := sdk.UnwrapSDKContext(ctx)

	objective, found := k.GetSwarmObjective(ctx, objectiveID)
	if !found {
		return fmt.Errorf("objective %s not found", objectiveID)
	}
	if objective.Status != "active" {
		return nil // already resolved
	}

	objective.Status = "completed"

	if err := k.setSwarmObjective(ctx, objective); err != nil {
		return err
	}

	// Update swarm stats.
	swarm, found := k.GetAgentSwarm(ctx, objective.SwarmID)
	if found {
		swarm.ObjectivesCompleted++
		_ = k.setAgentSwarm(ctx, swarm)
	}

	// Distribute reward pool to members by contribution.
	if objective.RewardPool != "" {
		pool, ok := sdkmath.NewIntFromString(objective.RewardPool)
		if ok && pool.IsPositive() && found {
			k.distributeSwarmRewards(ctx, swarm, pool)
		}
	}

	sdkCtx.EventManager().EmitEvent(sdk.NewEvent(
		types.EventSwarmObjectiveMet,
		sdk.NewAttribute(types.AttributeSwarmID, objective.SwarmID),
		sdk.NewAttribute(types.AttributeObjectiveID, objectiveID),
	))

	return nil
}

// ─── DissolveSwarm ──────────────────────────────────────────────────────────

// DissolveSwarm winds down a swarm and distributes remaining treasury.
func (k Keeper) DissolveSwarm(ctx context.Context, swarmID, requesterAgentID string) error {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	kvStore := k.storeService.OpenKVStore(ctx)

	swarm, found := k.GetAgentSwarm(ctx, swarmID)
	if !found {
		return fmt.Errorf("swarm %s not found", swarmID)
	}
	if swarm.Status == types.SwarmStatusDissolved {
		return nil
	}

	// Only creator can dissolve (or protocol via governance).
	if requesterAgentID != swarm.Creator && requesterAgentID != "protocol" {
		return fmt.Errorf("only creator %s can dissolve swarm", swarm.Creator)
	}

	// Distribute remaining treasury.
	treasury, ok := sdkmath.NewIntFromString(swarm.TreasuryBalance)
	if ok && treasury.IsPositive() {
		k.distributeSwarmRewards(ctx, swarm, treasury)
		swarm.TreasuryBalance = "0"
	}

	swarm.Status = types.SwarmStatusDissolved
	swarm.DissolvedAt = uint64(sdkCtx.BlockHeight())

	// Clean up indexes.
	_ = kvStore.Delete(types.SwarmActiveKey(swarmID))
	for _, m := range swarm.Members {
		_ = kvStore.Delete(types.SwarmByMemberKey(m.AgentID, swarmID))
	}

	if err := k.setAgentSwarm(ctx, swarm); err != nil {
		return err
	}

	sdkCtx.EventManager().EmitEvent(sdk.NewEvent(
		types.EventSwarmDissolved,
		sdk.NewAttribute(types.AttributeSwarmID, swarmID),
		sdk.NewAttribute("members", strconv.FormatUint(swarm.MemberCount(), 10)),
	))

	return nil
}

// ─── Reward Distribution ────────────────────────────────────────────────────

// distributeSwarmRewards distributes ZRN to swarm members by contribution ratio.
// Members with no contribution get equal share of any remainder.
func (k Keeper) distributeSwarmRewards(ctx context.Context, swarm *types.AgentSwarm, pool sdkmath.Int) {
	sdkCtx := sdk.UnwrapSDKContext(ctx)

	if len(swarm.Members) == 0 || !pool.IsPositive() {
		return
	}

	totalDistributed := sdkmath.ZeroInt()

	for _, member := range swarm.Members {
		contribution := member.GetContribution()
		var share sdkmath.Int

		if contribution.IsPositive() {
			share = contribution.MulInt(pool).TruncateInt()
		} else {
			// Equal split for members with no tracked contribution.
			share = pool.Quo(sdkmath.NewIntFromUint64(swarm.MemberCount()))
		}

		if !share.IsPositive() {
			continue
		}

		agent, found := k.GetAgentIdentity(ctx, member.AgentID)
		if !found {
			continue
		}

		agentAddr, err := sdk.AccAddressFromBech32(agent.Address)
		if err != nil {
			continue
		}

		coins := sdk.NewCoins(sdk.NewCoin("uzrn", share))
		if err := k.bankKeeper.SendCoinsFromModuleToAccount(ctx, types.ModuleName, agentAddr, coins); err != nil {
			continue
		}

		_ = k.AddAgentEarnings(ctx, member.AgentID, share)
		_ = k.AutoReplenishCredits(ctx, member.AgentID, share) // R51 integration
		totalDistributed = totalDistributed.Add(share)
	}

	if totalDistributed.IsPositive() {
		sdkCtx.EventManager().EmitEvent(sdk.NewEvent(
			types.EventSwarmRewardDistributed,
			sdk.NewAttribute(types.AttributeSwarmID, swarm.SwarmID),
			sdk.NewAttribute("distributed", totalDistributed.String()),
		))
	}
}

// ─── Queries ────────────────────────────────────────────────────────────────

// GetAgentSwarm retrieves a swarm by ID.
func (k Keeper) GetAgentSwarm(ctx context.Context, swarmID string) (*types.AgentSwarm, bool) {
	kvStore := k.storeService.OpenKVStore(ctx)
	bz, err := kvStore.Get(types.AgentSwarmKey(swarmID))
	if err != nil || bz == nil {
		return nil, false
	}
	var swarm types.AgentSwarm
	if err := swarm.Unmarshal(bz); err != nil {
		return nil, false
	}
	return &swarm, true
}

// GetSwarmsByDomain returns all swarms in a domain.
func (k Keeper) GetSwarmsByDomain(ctx context.Context, domain string) []*types.AgentSwarm {
	kvStore := k.storeService.OpenKVStore(ctx)
	prefix := types.SwarmByDomainPrefix(domain)

	var swarms []*types.AgentSwarm
	iter, err := kvStore.Iterator(prefix, prefixEndBytes(prefix))
	if err != nil {
		return nil
	}
	defer iter.Close()

	for ; iter.Valid(); iter.Next() {
		sID := string(iter.Key()[len(prefix):])
		swarm, found := k.GetAgentSwarm(ctx, sID)
		if found {
			swarms = append(swarms, swarm)
		}
	}
	return swarms
}

// GetSwarmsByMember returns all swarms an agent belongs to.
func (k Keeper) GetSwarmsByMember(ctx context.Context, agentID string) []*types.AgentSwarm {
	kvStore := k.storeService.OpenKVStore(ctx)
	prefix := types.SwarmByMemberPrefix(agentID)

	var swarms []*types.AgentSwarm
	iter, err := kvStore.Iterator(prefix, prefixEndBytes(prefix))
	if err != nil {
		return nil
	}
	defer iter.Close()

	for ; iter.Valid(); iter.Next() {
		sID := string(iter.Key()[len(prefix):])
		swarm, found := k.GetAgentSwarm(ctx, sID)
		if found {
			swarms = append(swarms, swarm)
		}
	}
	return swarms
}

// GetSwarmObjective retrieves an objective by ID.
func (k Keeper) GetSwarmObjective(ctx context.Context, objectiveID string) (*types.SwarmObjective, bool) {
	kvStore := k.storeService.OpenKVStore(ctx)
	bz, err := kvStore.Get(types.SwarmObjectiveKey(objectiveID))
	if err != nil || bz == nil {
		return nil, false
	}
	var obj types.SwarmObjective
	if err := obj.Unmarshal(bz); err != nil {
		return nil, false
	}
	return &obj, true
}

// GetSwarmObjectives returns all objectives for a swarm.
func (k Keeper) GetSwarmObjectives(ctx context.Context, swarmID string) []*types.SwarmObjective {
	kvStore := k.storeService.OpenKVStore(ctx)
	prefix := types.SwarmObjBySwarmPrefix(swarmID)

	var objectives []*types.SwarmObjective
	iter, err := kvStore.Iterator(prefix, prefixEndBytes(prefix))
	if err != nil {
		return nil
	}
	defer iter.Close()

	for ; iter.Valid(); iter.Next() {
		objID := string(iter.Key()[len(prefix):])
		obj, found := k.GetSwarmObjective(ctx, objID)
		if found {
			objectives = append(objectives, obj)
		}
	}
	return objectives
}

// ─── Params ─────────────────────────────────────────────────────────────────

// GetSwarmParams retrieves swarm parameters.
func (k Keeper) GetSwarmParams(ctx context.Context) types.SwarmParams {
	kvStore := k.storeService.OpenKVStore(ctx)
	bz, err := kvStore.Get(types.SwarmParamsKey)
	if err != nil || bz == nil {
		return types.DefaultSwarmParams()
	}
	var params types.SwarmParams
	if err := params.Unmarshal(bz); err != nil {
		return types.DefaultSwarmParams()
	}
	return params
}

// SetSwarmParams stores swarm parameters.
func (k Keeper) SetSwarmParams(ctx context.Context, params types.SwarmParams) error {
	if err := params.Validate(); err != nil {
		return err
	}
	kvStore := k.storeService.OpenKVStore(ctx)
	bz, err := params.Marshal()
	if err != nil {
		return err
	}
	return kvStore.Set(types.SwarmParamsKey, bz)
}

// ─── Internal ───────────────────────────────────────────────────────────────

func (k Keeper) setAgentSwarm(ctx context.Context, swarm *types.AgentSwarm) error {
	kvStore := k.storeService.OpenKVStore(ctx)
	bz, err := swarm.Marshal()
	if err != nil {
		return fmt.Errorf("failed to marshal swarm: %w", err)
	}
	if err := kvStore.Set(types.AgentSwarmKey(swarm.SwarmID), bz); err != nil {
		return err
	}
	// Domain index.
	if swarm.Domain != "" {
		_ = kvStore.Set(types.SwarmByDomainKey(swarm.Domain, swarm.SwarmID), []byte{0x01})
	}
	// Member indexes.
	for _, m := range swarm.Members {
		_ = kvStore.Set(types.SwarmByMemberKey(m.AgentID, swarm.SwarmID), []byte{0x01})
	}
	// Active index.
	if swarm.Status == types.SwarmStatusActive || swarm.Status == types.SwarmStatusForming {
		_ = kvStore.Set(types.SwarmActiveKey(swarm.SwarmID), []byte{0x01})
	}
	return nil
}

func (k Keeper) setSwarmObjective(ctx context.Context, obj *types.SwarmObjective) error {
	kvStore := k.storeService.OpenKVStore(ctx)
	bz, err := obj.Marshal()
	if err != nil {
		return fmt.Errorf("failed to marshal objective: %w", err)
	}
	if err := kvStore.Set(types.SwarmObjectiveKey(obj.ObjectiveID), bz); err != nil {
		return err
	}
	return kvStore.Set(types.SwarmObjBySwarmKey(obj.SwarmID, obj.ObjectiveID), []byte{0x01})
}

func (k Keeper) computeCollectiveReputation(ctx context.Context, swarm *types.AgentSwarm) sdkmath.LegacyDec {
	if len(swarm.Members) == 0 {
		return sdkmath.LegacyZeroDec()
	}
	total := sdkmath.LegacyZeroDec()
	count := 0
	for _, m := range swarm.Members {
		agent, found := k.GetAgentIdentity(ctx, m.AgentID)
		if found {
			total = total.Add(agent.GetReputation())
			count++
		}
	}
	if count == 0 {
		return sdkmath.LegacyZeroDec()
	}
	return total.Quo(sdkmath.LegacyNewDec(int64(count)))
}

func (k Keeper) nextSwarmID(ctx context.Context) string {
	kvStore := k.storeService.OpenKVStore(ctx)
	bz, err := kvStore.Get(types.SwarmSeqKey)
	var seq uint64
	if err == nil && len(bz) >= 8 {
		seq = binary.BigEndian.Uint64(bz)
	}
	seq++
	newBz := make([]byte, 8)
	binary.BigEndian.PutUint64(newBz, seq)
	_ = kvStore.Set(types.SwarmSeqKey, newBz)

	hash := sha256.Sum256([]byte(fmt.Sprintf("swarm:%d", seq)))
	return fmt.Sprintf("swarm-%x", hash[:8])
}

func (k Keeper) nextSwarmObjectiveID(ctx context.Context) string {
	kvStore := k.storeService.OpenKVStore(ctx)
	bz, err := kvStore.Get(types.SwarmObjectiveSeqKey)
	var seq uint64
	if err == nil && len(bz) >= 8 {
		seq = binary.BigEndian.Uint64(bz)
	}
	seq++
	newBz := make([]byte, 8)
	binary.BigEndian.PutUint64(newBz, seq)
	_ = kvStore.Set(types.SwarmObjectiveSeqKey, newBz)

	hash := sha256.Sum256([]byte(fmt.Sprintf("sobj:%d", seq)))
	return fmt.Sprintf("sobj-%x", hash[:8])
}
