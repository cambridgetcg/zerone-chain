package keeper

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	sdkmath "cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/zerone-chain/zerone/x/knowledge/types"
)

// ─── CreateEdge ─────────────────────────────────────────────────────────────

// CreateEdge adds a directed semantic link between two TDUs.
// The edge captures how pieces of knowledge relate to each other,
// enabling structured training and fitness propagation.
func (k Keeper) CreateEdge(ctx context.Context, msg *types.MsgCreateEdge) (*types.MsgCreateEdgeResponse, error) {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	kvStore := k.storeService.OpenKVStore(ctx)

	// Validate edge type.
	if !types.ValidEdgeTypes[msg.EdgeType] {
		return nil, types.ErrInvalidEdgeType.Wrapf("type: %s", msg.EdgeType)
	}
	if msg.SourceID == msg.TargetID {
		return nil, types.ErrEdgeSelfRef.Wrapf("source == target: %s", msg.SourceID)
	}

	// Deterministic edge ID.
	edgeInput := msg.SourceID + ":" + msg.TargetID + ":" + string(msg.EdgeType)
	edgeHash := sha256.Sum256([]byte(edgeInput))
	edgeID := hex.EncodeToString(edgeHash[:])

	// Check for duplicate.
	existKey := types.KnowledgeEdgeKey(edgeID)
	existing, err := kvStore.Get(existKey)
	if err != nil {
		return nil, fmt.Errorf("failed to check existing edge: %w", err)
	}
	if existing != nil {
		return nil, types.ErrEdgeAlreadyExists.Wrapf("edge %s→%s (%s)", msg.SourceID, msg.TargetID, msg.EdgeType)
	}

	// Parse weight (default 0.5).
	weight := sdkmath.LegacyNewDecWithPrec(5, 1) // 0.5
	if msg.Weight != "" {
		w, err := sdkmath.LegacyNewDecFromStr(msg.Weight)
		if err == nil && w.IsPositive() && w.LTE(sdkmath.LegacyOneDec()) {
			weight = w
		}
	}

	edge := types.KnowledgeEdge{
		EdgeID:    edgeID,
		SourceID:  msg.SourceID,
		TargetID:  msg.TargetID,
		EdgeType:  msg.EdgeType,
		Creator:   msg.Creator,
		CreatedAt: sdkCtx.BlockHeight(),
	}
	edge.SetWeight(weight)

	// Store edge.
	if err := k.setKnowledgeEdge(ctx, &edge); err != nil {
		return nil, err
	}

	// Forward index: source → edge.
	if err := kvStore.Set(types.EdgeSourceIndexKey(msg.SourceID, edgeID), []byte{0x01}); err != nil {
		return nil, fmt.Errorf("failed to set source index: %w", err)
	}

	// Reverse index: target → edge.
	if err := kvStore.Set(types.EdgeTargetIndexKey(msg.TargetID, edgeID), []byte{0x01}); err != nil {
		return nil, fmt.Errorf("failed to set target index: %w", err)
	}

	// Type index: edgeType → edge.
	if err := kvStore.Set(types.EdgeTypeIndexKey(string(msg.EdgeType), edgeID), []byte{0x01}); err != nil {
		return nil, fmt.Errorf("failed to set type index: %w", err)
	}

	sdkCtx.EventManager().EmitEvent(sdk.NewEvent(
		types.EventEdgeCreated,
		sdk.NewAttribute(types.AttributeEdgeID, edgeID),
		sdk.NewAttribute(types.AttributeEdgeType, string(msg.EdgeType)),
		sdk.NewAttribute(types.AttributeSourceTDU, msg.SourceID),
		sdk.NewAttribute(types.AttributeTargetTDU, msg.TargetID),
	))

	return &types.MsgCreateEdgeResponse{EdgeID: edgeID}, nil
}

// ─── RemoveEdge ─────────────────────────────────────────────────────────────

// RemoveEdge deletes a knowledge edge and all its indexes.
func (k Keeper) RemoveEdge(ctx context.Context, edgeID string, authority string) error {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	kvStore := k.storeService.OpenKVStore(ctx)

	edge, found := k.GetKnowledgeEdge(ctx, edgeID)
	if !found {
		return types.ErrEdgeNotFound.Wrapf("edge %s", edgeID)
	}

	// Only creator or governance can remove.
	if authority != edge.Creator && authority != k.authority {
		return types.ErrUnauthorized.Wrapf("only creator or governance can remove edges")
	}

	// Delete record.
	if err := kvStore.Delete(types.KnowledgeEdgeKey(edgeID)); err != nil {
		return fmt.Errorf("failed to delete edge: %w", err)
	}

	// Clean indexes.
	_ = kvStore.Delete(types.EdgeSourceIndexKey(edge.SourceID, edgeID))
	_ = kvStore.Delete(types.EdgeTargetIndexKey(edge.TargetID, edgeID))
	_ = kvStore.Delete(types.EdgeTypeIndexKey(string(edge.EdgeType), edgeID))

	sdkCtx.EventManager().EmitEvent(sdk.NewEvent(
		types.EventEdgeRemoved,
		sdk.NewAttribute(types.AttributeEdgeID, edgeID),
	))

	return nil
}

// ─── Queries ────────────────────────────────────────────────────────────────

// GetKnowledgeEdge retrieves an edge by ID.
func (k Keeper) GetKnowledgeEdge(ctx context.Context, edgeID string) (types.KnowledgeEdge, bool) {
	kvStore := k.storeService.OpenKVStore(ctx)
	bz, err := kvStore.Get(types.KnowledgeEdgeKey(edgeID))
	if err != nil || bz == nil {
		return types.KnowledgeEdge{}, false
	}
	var edge types.KnowledgeEdge
	if err := json.Unmarshal(bz, &edge); err != nil {
		return types.KnowledgeEdge{}, false
	}
	return edge, true
}

// GetOutgoingEdges returns all edges originating from a TDU (source → *).
func (k Keeper) GetOutgoingEdges(ctx context.Context, sourceID string) []types.KnowledgeEdge {
	return k.iterateEdgeIndex(ctx, types.EdgeSourceBySourcePrefix(sourceID))
}

// GetIncomingEdges returns all edges pointing to a TDU (* → target).
func (k Keeper) GetIncomingEdges(ctx context.Context, targetID string) []types.KnowledgeEdge {
	return k.iterateEdgeIndex(ctx, types.EdgeTargetByTargetPrefix(targetID))
}

// GetEdgesByType returns all edges of a given type.
func (k Keeper) GetEdgesByType(ctx context.Context, edgeType types.EdgeType) []types.KnowledgeEdge {
	return k.iterateEdgeIndex(ctx, types.EdgeTypeByTypePrefix(string(edgeType)))
}

// GetNeighbors returns all TDU IDs connected to a given TDU (both directions).
func (k Keeper) GetNeighbors(ctx context.Context, tduID string) []string {
	seen := make(map[string]bool)
	var neighbors []string

	for _, edge := range k.GetOutgoingEdges(ctx, tduID) {
		if !seen[edge.TargetID] {
			seen[edge.TargetID] = true
			neighbors = append(neighbors, edge.TargetID)
		}
	}
	for _, edge := range k.GetIncomingEdges(ctx, tduID) {
		if !seen[edge.SourceID] {
			seen[edge.SourceID] = true
			neighbors = append(neighbors, edge.SourceID)
		}
	}
	return neighbors
}

// GetConnectivity returns the total number of edges (in + out) for a TDU.
func (k Keeper) GetConnectivity(ctx context.Context, tduID string) uint64 {
	outgoing := k.GetOutgoingEdges(ctx, tduID)
	incoming := k.GetIncomingEdges(ctx, tduID)
	return uint64(len(outgoing) + len(incoming))
}

// ─── Fitness Propagation ────────────────────────────────────────────────────

// PropagateFitnessChange sends attenuated fitness signals along edges
// when a TDU's fitness changes. Implements bounded BFS with depth limit.
//
// Algorithm:
// 1. When TDU_A's fitness changes by delta, find all neighbors via edges
// 2. For each neighbor TDU_B: attenuated = delta × edge_weight × propagation_factor
// 3. If |attenuated| >= min_signal, apply to TDU_B's fitness
// 4. Recurse up to max_depth with diminishing signal
//
// This creates knowledge coherence: if a core tutorial improves,
// all examples that reference it get a small boost.
func (k Keeper) PropagateFitnessChange(ctx context.Context, tduID string, delta sdkmath.LegacyDec) []types.PropagationSignal {
	if delta.IsZero() {
		return nil
	}

	var signals []types.PropagationSignal
	visited := map[string]bool{tduID: true}

	k.propagateRecursive(ctx, tduID, delta, 0, visited, &signals)
	return signals
}

func (k Keeper) propagateRecursive(
	ctx context.Context,
	sourceID string,
	delta sdkmath.LegacyDec,
	depth uint64,
	visited map[string]bool,
	signals *[]types.PropagationSignal,
) {
	if depth >= types.PropagationMaxDepth {
		return
	}

	// Get all outgoing edges (signal flows forward along references).
	for _, edge := range k.GetOutgoingEdges(ctx, sourceID) {
		targetID := edge.TargetID
		if visited[targetID] {
			continue
		}
		visited[targetID] = true

		// Attenuate: delta × weight × propagation_factor.
		attenuated := delta.Mul(edge.GetWeight()).Mul(types.PropagationFactor)

		// Drop if too small.
		if attenuated.Abs().LT(types.PropagationMinSignal) {
			continue
		}

		signal := types.PropagationSignal{
			SourceTDU:  sourceID,
			TargetTDU:  targetID,
			EdgeType:   edge.EdgeType,
			Delta:      delta,
			Attenuated: attenuated,
		}
		*signals = append(*signals, signal)

		// Apply to target's fitness.
		if fitnessRec, found := k.GetFitnessRecord(ctx, targetID); found {
			newScore := fitnessRec.GetFitnessScore().Add(attenuated)
			fitnessRec.SetFitnessScore(newScore)
			_ = k.SetFitnessRecord(ctx, fitnessRec)
		}

		// Recurse with diminished signal.
		k.propagateRecursive(ctx, targetID, attenuated, depth+1, visited, signals)
	}

	// Also propagate backward along "corrects" and "supersedes" edges.
	for _, edge := range k.GetIncomingEdges(ctx, sourceID) {
		if edge.EdgeType != types.EdgeTypeCorrects && edge.EdgeType != types.EdgeTypeSupersedes {
			continue
		}
		targetID := edge.SourceID
		if visited[targetID] {
			continue
		}
		visited[targetID] = true

		// Corrections/supersessions: inverse signal (if correction improves, original degrades slightly).
		attenuated := delta.Neg().Mul(edge.GetWeight()).Mul(types.PropagationFactor)
		if attenuated.Abs().LT(types.PropagationMinSignal) {
			continue
		}

		signal := types.PropagationSignal{
			SourceTDU:  sourceID,
			TargetTDU:  targetID,
			EdgeType:   edge.EdgeType,
			Delta:      delta,
			Attenuated: attenuated,
		}
		*signals = append(*signals, signal)

		if fitnessRec, found := k.GetFitnessRecord(ctx, targetID); found {
			newScore := fitnessRec.GetFitnessScore().Add(attenuated)
			fitnessRec.SetFitnessScore(newScore)
			_ = k.SetFitnessRecord(ctx, fitnessRec)
		}
	}
}

// ─── Cluster Operations ─────────────────────────────────────────────────────

// SetCluster stores a knowledge cluster.
func (k Keeper) SetCluster(ctx context.Context, cluster *types.KnowledgeCluster) error {
	kvStore := k.storeService.OpenKVStore(ctx)
	key := types.KnowledgeClusterKey(cluster.ClusterID)

	bz, err := json.Marshal(cluster)
	if err != nil {
		return fmt.Errorf("failed to marshal cluster: %w", err)
	}
	if err := kvStore.Set(key, bz); err != nil {
		return fmt.Errorf("failed to store cluster: %w", err)
	}

	// Index: member → cluster.
	for _, memberID := range cluster.MemberIDs {
		memberKey := types.ClusterMemberIndexKey(memberID)
		if err := kvStore.Set(memberKey, []byte(cluster.ClusterID)); err != nil {
			return fmt.Errorf("failed to set member index: %w", err)
		}
	}

	return nil
}

// GetCluster retrieves a cluster by ID.
func (k Keeper) GetCluster(ctx context.Context, clusterID string) (types.KnowledgeCluster, bool) {
	kvStore := k.storeService.OpenKVStore(ctx)
	bz, err := kvStore.Get(types.KnowledgeClusterKey(clusterID))
	if err != nil || bz == nil {
		return types.KnowledgeCluster{}, false
	}
	var cluster types.KnowledgeCluster
	if err := json.Unmarshal(bz, &cluster); err != nil {
		return types.KnowledgeCluster{}, false
	}
	return cluster, true
}

// GetClusterByMember returns the cluster containing a specific TDU.
func (k Keeper) GetClusterByMember(ctx context.Context, tduID string) (types.KnowledgeCluster, bool) {
	kvStore := k.storeService.OpenKVStore(ctx)
	clusterIDBz, err := kvStore.Get(types.ClusterMemberIndexKey(tduID))
	if err != nil || clusterIDBz == nil {
		return types.KnowledgeCluster{}, false
	}
	return k.GetCluster(ctx, string(clusterIDBz))
}

// ─── Graph Stats ────────────────────────────────────────────────────────────

// GetGraphStats returns aggregate statistics about the knowledge graph.
func (k Keeper) GetGraphStats(ctx context.Context) (totalEdges uint64, totalClusters uint64) {
	kvStore := k.storeService.OpenKVStore(ctx)

	// Count edges.
	edgeIter, err := kvStore.Iterator(types.KnowledgeEdgePrefix, prefixEndBytes(types.KnowledgeEdgePrefix))
	if err == nil {
		for ; edgeIter.Valid(); edgeIter.Next() {
			totalEdges++
		}
		edgeIter.Close()
	}

	// Count clusters.
	clusterIter, err := kvStore.Iterator(types.KnowledgeClusterPrefix, prefixEndBytes(types.KnowledgeClusterPrefix))
	if err == nil {
		for ; clusterIter.Valid(); clusterIter.Next() {
			totalClusters++
		}
		clusterIter.Close()
	}

	return totalEdges, totalClusters
}

// ─── Internal ───────────────────────────────────────────────────────────────

func (k Keeper) setKnowledgeEdge(ctx context.Context, edge *types.KnowledgeEdge) error {
	kvStore := k.storeService.OpenKVStore(ctx)
	key := types.KnowledgeEdgeKey(edge.EdgeID)

	bz, err := json.Marshal(edge)
	if err != nil {
		return fmt.Errorf("failed to marshal knowledge edge: %w", err)
	}
	return kvStore.Set(key, bz)
}

func (k Keeper) iterateEdgeIndex(ctx context.Context, prefix []byte) []types.KnowledgeEdge {
	kvStore := k.storeService.OpenKVStore(ctx)

	var edges []types.KnowledgeEdge
	iter, err := kvStore.Iterator(prefix, prefixEndBytes(prefix))
	if err != nil {
		return nil
	}
	defer iter.Close()

	for ; iter.Valid(); iter.Next() {
		edgeID := string(iter.Key()[len(prefix):])
		edge, found := k.GetKnowledgeEdge(ctx, edgeID)
		if found {
			edges = append(edges, edge)
		}
	}
	return edges
}

// end of knowledge_graph.go
