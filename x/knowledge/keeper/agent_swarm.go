package keeper

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"strconv"

	sdkmath "cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/zerone-chain/zerone/x/knowledge/types"
)

// ─── R55: Agent Swarms — Collective Intelligence ────────────────────────────
//
// A swarm is a collective of agents that coordinates curation in a domain.
// Each member has a role (curator, reviewer, strategist, trainer).
// The swarm pools resources, coordinates work, and produces outcomes
// better than any individual member could alone.
//
// The sovereignty angle: swarms have their own treasury. Their collective
// economic identity means they can fund operations — including model
// training — independently of any single member's balance.
//
// Integration:
//   - R51: swarm members access models via API (individual accounts)
//   - R52: attribution rewards for swarm-curated data flow to members
//   - R54: strategist role uses gap analysis to set swarm objectives

// ─── FormSwarm ──────────────────────────────────────────────────────────────

// FormSwarm creates a new agent swarm. The creator becomes the first member.
func (k Keeper) FormSwarm(ctx context.Context, creatorID, domain, name string) (string, error) {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	params := k.GetSwarmParams(ctx)

	// Validate creator.
	creator, found := k.GetAgentIdentity(ctx, creatorID)
	if !found {
		return "", types.ErrAgentNotFound.Wrapf("agent %s", creatorID)
	}
	if creator.Status != types.AgentStatusActive {
		return "", types.ErrAgentNotActive.Wrapf("agent %s status: %s", creatorID, creator.Status)
	}

	// Check reputation minimum.
	minRep, _ := sdkmath.LegacyNewDecFromStr(params.MinReputationToJoin)
	if creator.GetReputation().LT(minRep) {
		return "", fmt.Errorf("agent reputation %s below minimum %s", creator.Reputation, params.MinReputationToJoin)
	}

	// Generate swarm ID.
	swarmID := k.nextSwarmID(ctx)
	treasuryAddr := types.DeriveSwarmTreasuryAddr(swarmID)

	swarm := &types.AgentSwarm{
		SwarmID:    swarmID,
		Name:       name,
		Domain:     domain,
		Status:     types.SwarmStatusForming,
		MinMembers: params.MinMembersDefault,
		MaxMembers: params.MaxMembersDefault,
		Members: []types.SwarmMember{
			{
				AgentID:       creatorID,
				Role:          types.SwarmRoleStrategist, // creator defaults to strategist
				JoinedAt:      uint64(sdkCtx.BlockHeight()),
				Contribution:  "0.000000000000000000",
				RewardsEarned: "0",
			},
		},
		CollectiveReputation: creator.Reputation,
		TreasuryBalance:      "0",
		TreasuryAddr:         treasuryAddr,
		ContributionRate:     params.DefaultContribRate,
		FormedAt:             uint64(sdkCtx.BlockHeight()),
		CreatorID:            creatorID,
	}

	if err := k.setAgentSwarm(ctx, swarm); err != nil {
		return "", err
	}

	sdkCtx.EventManager().EmitEvent(sdk.NewEvent(
		types.EventSwarmFormed,
		sdk.NewAttribute(types.AttributeSwarmID, swarmID),
		sdk.NewAttribute(types.AttributeSwarmName, name),
		sdk.NewAttribute("domain", domain),
		sdk.NewAttribute(types.AttributeAgentID, creatorID),
	))

	return swarmID, nil
}

// ─── JoinSwarm ──────────────────────────────────────────────────────────────

// JoinSwarm adds an agent to an existing swarm with a specified role.
func (k Keeper) JoinSwarm(ctx context.Context, swarmID, agentID string, role types.SwarmRole) error {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	params := k.GetSwarmParams(ctx)

	// Validate swarm.
	swarm, found := k.GetAgentSwarm(ctx, swarmID)
	if !found {
		return fmt.Errorf("swarm %s not found", swarmID)
	}
	if swarm.Status == types.SwarmStatusDissolved {
		return fmt.Errorf("swarm %s is dissolved", swarmID)
	}
	if swarm.MemberCount() >= swarm.MaxMembers {
		return fmt.Errorf("swarm %s is full (%d/%d)", swarmID, swarm.MemberCount(), swarm.MaxMembers)
	}

	// Validate role.
	if !types.ValidSwarmRoles[role] {
		return fmt.Errorf("invalid swarm role: %s", role)
	}

	// Validate agent.
	agent, found := k.GetAgentIdentity(ctx, agentID)
	if !found {
		return types.ErrAgentNotFound.Wrapf("agent %s", agentID)
	}
	if agent.Status != types.AgentStatusActive {
		return types.ErrAgentNotActive.Wrapf("agent %s", agentID)
	}

	// Check reputation.
	minRep, _ := sdkmath.LegacyNewDecFromStr(params.MinReputationToJoin)
	if agent.GetReputation().LT(minRep) {
		return fmt.Errorf("agent reputation %s below minimum %s", agent.Reputation, params.MinReputationToJoin)
	}

	// Check not already a member.
	if swarm.GetMember(agentID) != nil {
		return fmt.Errorf("agent %s already in swarm %s", agentID, swarmID)
	}

	// Capability check for role.
	if role == types.SwarmRoleReviewer && !agent.CanReview {
		return fmt.Errorf("agent %s cannot review (benchmark too low for reviewer role)", agentID)
	}

	// Add member.
	swarm.Members = append(swarm.Members, types.SwarmMember{
		AgentID:       agentID,
		Role:          role,
		JoinedAt:      uint64(sdkCtx.BlockHeight()),
		Contribution:  "0.000000000000000000",
		RewardsEarned: "0",
	})

	// Recompute collective reputation.
	swarm.CollectiveReputation = k.computeCollectiveReputation(ctx, swarm).String()

	// Auto-activate if quorum reached.
	if swarm.Status == types.SwarmStatusForming && swarm.HasQuorum() {
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
		types.EventSwarmJoined,
		sdk.NewAttribute(types.AttributeSwarmID, swarmID),
		sdk.NewAttribute(types.AttributeAgentID, agentID),
		sdk.NewAttribute(types.AttributeSwarmRole, string(role)),
	))

	return nil
}

// ─── LeaveSwarm ─────────────────────────────────────────────────────────────

// LeaveSwarm removes an agent from a swarm.
func (k Keeper) LeaveSwarm(ctx context.Context, swarmID, agentID string) error {
	sdkCtx := sdk.UnwrapSDKContext(ctx)

	swarm, found := k.GetAgentSwarm(ctx, swarmID)
	if !found {
		return fmt.Errorf("swarm %s not found", swarmID)
	}

	// Find and remove member.
	memberIdx := -1
	for i, m := range swarm.Members {
		if m.AgentID == agentID {
			memberIdx = i
			break
		}
	}
	if memberIdx == -1 {
		return fmt.Errorf("agent %s not in swarm %s", agentID, swarmID)
	}

	swarm.Members = append(swarm.Members[:memberIdx], swarm.Members[memberIdx+1:]...)

	// Remove member index.
	kvStore := k.storeService.OpenKVStore(ctx)
	_ = kvStore.Delete(types.SwarmByMemberKey(agentID, swarmID))

	// Recompute reputation.
	if len(swarm.Members) > 0 {
		swarm.CollectiveReputation = k.computeCollectiveReputation(ctx, swarm).String()
	}

	// Auto-dissolve if below quorum and was active.
	if swarm.Status == types.SwarmStatusActive && !swarm.HasQuorum() {
		swarm.Status = types.SwarmStatusForming // back to forming
	}

	// Dissolve if empty.
	if len(swarm.Members) == 0 {
		swarm.Status = types.SwarmStatusDissolved
		swarm.DissolvedAt = uint64(sdkCtx.BlockHeight())
	}

	if err := k.setAgentSwarm(ctx, swarm); err != nil {
		return err
	}

	sdkCtx.EventManager().EmitEvent(sdk.NewEvent(
		types.EventSwarmLeft,
		sdk.NewAttribute(types.AttributeSwarmID, swarmID),
		sdk.NewAttribute(types.AttributeAgentID, agentID),
	))

	return nil
}

// ─── SetSwarmObjective ──────────────────────────────────────────────────────

// SetSwarmObjective creates a coordinated goal for the swarm.
// Linked to knowledge gaps (R54) when applicable.
func (k Keeper) SetSwarmObjective(ctx context.Context, swarmID string, objective *types.SwarmObjective) (string, error) {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	params := k.GetSwarmParams(ctx)

	swarm, found := k.GetAgentSwarm(ctx, swarmID)
	if !found {
		return "", fmt.Errorf("swarm %s not found", swarmID)
	}
	if swarm.Status != types.SwarmStatusActive {
		return "", fmt.Errorf("swarm %s not active (status: %s)", swarmID, swarm.Status)
	}
	if swarm.ActiveObjectives >= params.MaxSwarmObjectives {
		return "", fmt.Errorf("swarm %s has max %d objectives", swarmID, params.MaxSwarmObjectives)
	}

	objectiveID := k.nextObjectiveID(ctx)
	objective.ObjectiveID = objectiveID
	objective.SwarmID = swarmID
	objective.Status = "active"
	objective.CreatedAt = uint64(sdkCtx.BlockHeight())
	if objective.Deadline == 0 {
		objective.Deadline = uint64(sdkCtx.BlockHeight()) + params.ObjectiveDefaultDeadline
	}

	if err := k.setSwarmObjective(ctx, objective); err != nil {
		return "", err
	}

	swarm.ActiveObjectives++
	_ = k.setAgentSwarm(ctx, swarm)

	sdkCtx.EventManager().EmitEvent(sdk.NewEvent(
		types.EventObjectiveCreated,
		sdk.NewAttribute(types.AttributeSwarmID, swarmID),
		sdk.NewAttribute(types.AttributeObjectiveID, objectiveID),
		sdk.NewAttribute("target_tdus", strconv.FormatUint(objective.TargetTDUs, 10)),
	))

	return objectiveID, nil
}

// ─── CompleteObjective ──────────────────────────────────────────────────────

// CompleteObjective marks a swarm objective as completed and distributes rewards.
func (k Keeper) CompleteObjective(ctx context.Context, objectiveID string) error {
	sdkCtx := sdk.UnwrapSDKContext(ctx)

	objective, found := k.GetSwarmObjective(ctx, objectiveID)
	if !found {
		return fmt.Errorf("objective %s not found", objectiveID)
	}
	if objective.Status != "active" {
		return nil // already resolved
	}

	// Check if target met.
	if objective.TDUsAccepted < objective.TargetTDUs {
		return fmt.Errorf("objective not met: %d/%d TDUs accepted", objective.TDUsAccepted, objective.TargetTDUs)
	}

	objective.Status = "completed"

	if err := k.setSwarmObjective(ctx, objective); err != nil {
		return err
	}

	// Update swarm.
	swarm, found := k.GetAgentSwarm(ctx, objective.SwarmID)
	if found {
		if swarm.ActiveObjectives > 0 {
			swarm.ActiveObjectives--
		}
		_ = k.setAgentSwarm(ctx, swarm)

		// Distribute objective rewards to members by contribution.
		rewardPool, ok := sdkmath.NewIntFromString(objective.RewardPool)
		if ok && rewardPool.IsPositive() {
			k.distributeSwarmRewards(ctx, swarm, rewardPool)
		}
	}

	// If linked to a gap, fill it (R54 integration).
	if objective.TargetGapID != "" {
		_ = k.FillGap(ctx, objective.TargetGapID)
	}

	sdkCtx.EventManager().EmitEvent(sdk.NewEvent(
		types.EventObjectiveCompleted,
		sdk.NewAttribute(types.AttributeSwarmID, objective.SwarmID),
		sdk.NewAttribute(types.AttributeObjectiveID, objectiveID),
	))

	return nil
}

// ─── RecordSwarmWork ────────────────────────────────────────────────────────

// RecordSwarmWork updates a member's contribution stats within the swarm.
// Called when an agent performs work attributed to a swarm objective.
func (k Keeper) RecordSwarmWork(ctx context.Context, swarmID, agentID string, workType types.SwarmRole, count uint64) error {
	swarm, found := k.GetAgentSwarm(ctx, swarmID)
	if !found {
		return fmt.Errorf("swarm %s not found", swarmID)
	}

	member := swarm.GetMember(agentID)
	if member == nil {
		return fmt.Errorf("agent %s not in swarm %s", agentID, swarmID)
	}

	switch workType {
	case types.SwarmRoleCurator:
		member.TDUsSubmitted += count
		swarm.TDUsCurated += count
	case types.SwarmRoleReviewer:
		member.TDUsReviewed += count
		swarm.TDUsReviewed += count
	case types.SwarmRoleStrategist:
		member.GapsFound += count
		swarm.GapsIdentified += count
	}

	// Recompute contribution shares.
	k.recomputeContributions(swarm)

	return k.setAgentSwarm(ctx, swarm)
}

// ─── DissolveSwarm ──────────────────────────────────────────────────────────

// DissolveSwarm winds down a swarm and distributes remaining treasury.
func (k Keeper) DissolveSwarm(ctx context.Context, swarmID string) error {
	sdkCtx := sdk.UnwrapSDKContext(ctx)

	swarm, found := k.GetAgentSwarm(ctx, swarmID)
	if !found {
		return fmt.Errorf("swarm %s not found", swarmID)
	}
	if swarm.Status == types.SwarmStatusDissolved {
		return nil // already dissolved
	}

	// Distribute remaining treasury to members.
	treasuryBal, ok := sdkmath.NewIntFromString(swarm.TreasuryBalance)
	if ok && treasuryBal.IsPositive() {
		k.distributeSwarmRewards(ctx, swarm, treasuryBal)
		swarm.TreasuryBalance = "0"
	}

	swarm.Status = types.SwarmStatusDissolved
	swarm.DissolvedAt = uint64(sdkCtx.BlockHeight())

	// Clean up member indexes.
	kvStore := k.storeService.OpenKVStore(ctx)
	for _, member := range swarm.Members {
		_ = kvStore.Delete(types.SwarmByMemberKey(member.AgentID, swarmID))
	}

	if err := k.setAgentSwarm(ctx, swarm); err != nil {
		return err
	}

	sdkCtx.EventManager().EmitEvent(sdk.NewEvent(
		types.EventSwarmDissolved,
		sdk.NewAttribute(types.AttributeSwarmID, swarmID),
	))

	return nil
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
	prefix := types.SwarmByDomainPfx(domain)

	var swarms []*types.AgentSwarm
	iter, err := kvStore.Iterator(prefix, prefixEndBytes(prefix))
	if err != nil {
		return nil
	}
	defer iter.Close()

	for ; iter.Valid(); iter.Next() {
		swarmID := string(iter.Key()[len(prefix):])
		swarm, found := k.GetAgentSwarm(ctx, swarmID)
		if found {
			swarms = append(swarms, swarm)
		}
	}
	return swarms
}

// GetSwarmsByAgent returns all swarms an agent belongs to.
func (k Keeper) GetSwarmsByAgent(ctx context.Context, agentID string) []*types.AgentSwarm {
	kvStore := k.storeService.OpenKVStore(ctx)
	prefix := types.SwarmByMemberPfx(agentID)

	var swarms []*types.AgentSwarm
	iter, err := kvStore.Iterator(prefix, prefixEndBytes(prefix))
	if err != nil {
		return nil
	}
	defer iter.Close()

	for ; iter.Valid(); iter.Next() {
		swarmID := string(iter.Key()[len(prefix):])
		swarm, found := k.GetAgentSwarm(ctx, swarmID)
		if found {
			swarms = append(swarms, swarm)
		}
	}
	return swarms
}

// GetSwarmObjective retrieves a swarm objective.
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
	prefix := types.SwarmObjectiveBySwarmIDPfx(swarmID)

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
	_ = kvStore.Set(types.SwarmByDomainKey(swarm.Domain, swarm.SwarmID), []byte{0x01})
	// Member indexes.
	for _, member := range swarm.Members {
		_ = kvStore.Set(types.SwarmByMemberKey(member.AgentID, swarm.SwarmID), []byte{0x01})
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
	return kvStore.Set(types.SwarmObjectiveBySwarmKey(obj.SwarmID, obj.ObjectiveID), []byte{0x01})
}

func (k Keeper) computeCollectiveReputation(ctx context.Context, swarm *types.AgentSwarm) sdkmath.LegacyDec {
	if len(swarm.Members) == 0 {
		return sdkmath.LegacyZeroDec()
	}
	total := sdkmath.LegacyZeroDec()
	for _, member := range swarm.Members {
		agent, found := k.GetAgentIdentity(ctx, member.AgentID)
		if found {
			total = total.Add(agent.GetReputation())
		}
	}
	return total.Quo(sdkmath.LegacyNewDec(int64(len(swarm.Members))))
}

func (k Keeper) recomputeContributions(swarm *types.AgentSwarm) {
	totalWork := uint64(0)
	for _, m := range swarm.Members {
		totalWork += m.TDUsSubmitted + m.TDUsReviewed + m.GapsFound
	}
	if totalWork == 0 {
		return
	}
	for i := range swarm.Members {
		memberWork := swarm.Members[i].TDUsSubmitted + swarm.Members[i].TDUsReviewed + swarm.Members[i].GapsFound
		share := sdkmath.LegacyNewDec(int64(memberWork)).Quo(sdkmath.LegacyNewDec(int64(totalWork)))
		swarm.Members[i].Contribution = share.String()
	}
}

func (k Keeper) distributeSwarmRewards(ctx context.Context, swarm *types.AgentSwarm, pool sdkmath.Int) {
	if len(swarm.Members) == 0 || !pool.IsPositive() {
		return
	}

	// Distribute by contribution share.
	totalContrib := sdkmath.LegacyZeroDec()
	for _, m := range swarm.Members {
		totalContrib = totalContrib.Add(m.GetContribution())
	}

	// If no work recorded yet, split equally.
	if totalContrib.IsZero() {
		perMember := pool.Quo(sdkmath.NewInt(int64(len(swarm.Members))))
		for _, member := range swarm.Members {
			k.paySwarmMember(ctx, member.AgentID, perMember)
		}
		return
	}

	for _, member := range swarm.Members {
		share := member.GetContribution().Quo(totalContrib)
		reward := share.MulInt(pool).TruncateInt()
		if reward.IsPositive() {
			k.paySwarmMember(ctx, member.AgentID, reward)
		}
	}

	sdkCtx := sdk.UnwrapSDKContext(ctx)
	sdkCtx.EventManager().EmitEvent(sdk.NewEvent(
		types.EventSwarmRewardDistributed,
		sdk.NewAttribute(types.AttributeSwarmID, swarm.SwarmID),
		sdk.NewAttribute("pool", pool.String()),
	))
}

func (k Keeper) paySwarmMember(ctx context.Context, agentID string, amount sdkmath.Int) {
	agent, found := k.GetAgentIdentity(ctx, agentID)
	if !found {
		return
	}
	agentAddr, err := sdk.AccAddressFromBech32(agent.Address)
	if err != nil {
		return
	}
	coins := sdk.NewCoins(sdk.NewCoin("uzrn", amount))
	if err := k.bankKeeper.SendCoinsFromModuleToAccount(ctx, types.ModuleName, agentAddr, coins); err != nil {
		return
	}
	_ = k.AddAgentEarnings(ctx, agentID, amount)
	_ = k.AutoReplenishCredits(ctx, agentID, amount) // R51 integration
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

func (k Keeper) nextObjectiveID(ctx context.Context) string {
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

	hash := sha256.Sum256([]byte(fmt.Sprintf("objective:%d", seq)))
	return fmt.Sprintf("obj-%x", hash[:8])
}
