package keeper

import (
	"context"
	"fmt"
	"sort"

	protov2 "google.golang.org/protobuf/proto"

	"github.com/zerone-chain/zerone/x/knowledge/types"
)

// ToK chain-side caps. Adjustable via params in a future wave.
const (
	ToKMaxDepthCap   uint32 = 32
	ToKMaxPathsCap   uint32 = 256
	ToKFrontierLimit uint32 = 1024
	ToKFrontierCap   uint32 = 8192
)

// ValidateToKSelector returns an error iff the selector is malformed.
// TC5 (extraction is open) requires that the only refusal classes are
// syntax error, snapshot-out-of-range, and rate-limit. This is the
// syntax-error gate.
func ValidateToKSelector(s *types.ToKSelector) error {
	if s == nil || s.Variant == nil {
		return fmt.Errorf("selector variant required (TC5: extraction is open — but selector must be well-formed)")
	}
	switch v := s.Variant.(type) {
	case *types.ToKSelector_RootedSubtree:
		if v.RootedSubtree == nil || v.RootedSubtree.RootFactId == "" {
			return fmt.Errorf("rooted_subtree.root_fact_id required")
		}
	case *types.ToKSelector_AncestorCone:
		if v.AncestorCone == nil || v.AncestorCone.LeafFactId == "" {
			return fmt.Errorf("ancestor_cone.leaf_fact_id required")
		}
	case *types.ToKSelector_Frontier:
		if v.Frontier == nil || v.Frontier.Domain == "" {
			return fmt.Errorf("frontier.domain required")
		}
	default:
		return fmt.Errorf("selector variant not recognised by this chain version")
	}
	return nil
}

// ValidateAndCapToKSelector validates and applies chain-side caps.
// Returns the capped selector. Caller should pass the capped value
// downstream so caps are applied uniformly.
func ValidateAndCapToKSelector(s *types.ToKSelector) (*types.ToKSelector, error) {
	if err := ValidateToKSelector(s); err != nil {
		return nil, err
	}
	out := &types.ToKSelector{}
	switch v := s.Variant.(type) {
	case *types.ToKSelector_RootedSubtree:
		cp := protov2.Clone(v.RootedSubtree).(*types.RootedSubtreeSelector)
		if cp.MaxDepth == 0 || cp.MaxDepth > ToKMaxDepthCap {
			cp.MaxDepth = ToKMaxDepthCap
		}
		out.Variant = &types.ToKSelector_RootedSubtree{RootedSubtree: cp}
	case *types.ToKSelector_AncestorCone:
		cp := protov2.Clone(v.AncestorCone).(*types.AncestorConeSelector)
		if cp.MaxDepth == 0 || cp.MaxDepth > ToKMaxDepthCap {
			cp.MaxDepth = ToKMaxDepthCap
		}
		if cp.MaxPaths == 0 || cp.MaxPaths > ToKMaxPathsCap {
			cp.MaxPaths = ToKMaxPathsCap
		}
		out.Variant = &types.ToKSelector_AncestorCone{AncestorCone: cp}
	case *types.ToKSelector_Frontier:
		cp := protov2.Clone(v.Frontier).(*types.FrontierSelector)
		if cp.Limit == 0 {
			cp.Limit = ToKFrontierLimit
		}
		if cp.Limit > ToKFrontierCap {
			cp.Limit = ToKFrontierCap
		}
		out.Variant = &types.ToKSelector_Frontier{Frontier: cp}
	}
	return out, nil
}

// GatherRootedSubtree walks descendants from the given root up to max_depth
// and returns sorted node IDs + sorted edges. Re-uses the descendant walk
// already used by grpc_query.go.
func (k Keeper) GatherRootedSubtree(
	ctx context.Context,
	sel *types.RootedSubtreeSelector,
) (nodeIDs []string, edges []*types.ToKEdge, err error) {
	root, found := k.GetFact(ctx, sel.RootFactId)
	if !found {
		return nil, nil, fmt.Errorf("root fact %s not found", sel.RootFactId)
	}
	visited := map[string]bool{root.Id: true}
	edgeSet := map[string]*types.ToKEdge{}
	if err := k.gatherDescendantsRecursive(ctx, root.Id, 1, sel.MaxDepth, visited, edgeSet); err != nil {
		return nil, nil, err
	}
	for id := range visited {
		nodeIDs = append(nodeIDs, id)
	}
	sort.Strings(nodeIDs)
	for _, e := range edgeSet {
		edges = append(edges, e)
	}
	sortToKEdges(edges)
	return nodeIDs, edges, nil
}

func (k Keeper) gatherDescendantsRecursive(
	ctx context.Context,
	factID string,
	depth, maxDepth uint32,
	visited map[string]bool,
	edges map[string]*types.ToKEdge,
) error {
	if depth > maxDepth {
		return nil
	}
	incoming, err := k.GetIncomingRelations(ctx, factID)
	if err != nil {
		return err
	}
	for _, rel := range incoming {
		// Only follow support-bearing relations — same filter as walkDescendants.
		// CONTRADICTS and SUPERSEDES must not appear in a descendant bundle.
		switch rel.Relation {
		case types.RelationType_RELATION_TYPE_SUPPORTS,
			types.RelationType_RELATION_TYPE_REQUIRES,
			types.RelationType_RELATION_TYPE_REFINES,
			types.RelationType_RELATION_TYPE_GENERALIZES,
			types.RelationType_RELATION_TYPE_CITES:
		default:
			continue
		}
		// Guard against ghost nodes: skip if the source fact no longer exists.
		if _, ok := k.GetFact(ctx, rel.SourceFactId); !ok {
			continue
		}
		edgeKey := rel.SourceFactId + "->" + rel.TargetFactId + "|" + rel.Relation.String()
		if _, ok := edges[edgeKey]; !ok {
			edges[edgeKey] = &types.ToKEdge{
				FromFactId: rel.SourceFactId,
				ToFactId:   rel.TargetFactId,
				Relation:   rel.Relation.String(),
				Inference:  rel.Inference.String(),
			}
		}
		if !visited[rel.SourceFactId] {
			visited[rel.SourceFactId] = true
			if err := k.gatherDescendantsRecursive(ctx, rel.SourceFactId, depth+1, maxDepth, visited, edges); err != nil {
				return err
			}
		}
	}
	return nil
}

// GatherAncestorCone walks ancestors from the leaf up to max_depth, capped
// at max_paths distinct paths. Mirror of GatherRootedSubtree but follows
// outgoing relations (source→target) instead of incoming.
func (k Keeper) GatherAncestorCone(
	ctx context.Context,
	sel *types.AncestorConeSelector,
) (nodeIDs []string, edges []*types.ToKEdge, err error) {
	leaf, found := k.GetFact(ctx, sel.LeafFactId)
	if !found {
		return nil, nil, fmt.Errorf("leaf fact %s not found", sel.LeafFactId)
	}
	visited := map[string]bool{leaf.Id: true}
	edgeSet := map[string]*types.ToKEdge{}
	pathCount := uint32(0)
	if err := k.gatherAncestorsRecursive(ctx, leaf.Id, 1, sel.MaxDepth, sel.MaxPaths, &pathCount, visited, edgeSet); err != nil {
		return nil, nil, err
	}
	for id := range visited {
		nodeIDs = append(nodeIDs, id)
	}
	sort.Strings(nodeIDs)
	for _, e := range edgeSet {
		edges = append(edges, e)
	}
	sortToKEdges(edges)
	return nodeIDs, edges, nil
}

func (k Keeper) gatherAncestorsRecursive(
	ctx context.Context,
	factID string,
	depth, maxDepth, maxPaths uint32,
	pathCount *uint32,
	visited map[string]bool,
	edges map[string]*types.ToKEdge,
) error {
	if depth > maxDepth || *pathCount >= maxPaths {
		return nil
	}
	// GetFactRelations returns all outgoing relations from factID (source → target).
	outgoing, err := k.GetFactRelations(ctx, factID)
	if err != nil {
		return err
	}
	for _, rel := range outgoing {
		// FILTER: only support-bearing relations — mirror of gatherDescendantsRecursive.
		// CONTRADICTS, SUPERSEDES, UNSPECIFIED, REFORMULATES must not appear in an ancestor bundle.
		switch rel.Relation {
		case types.RelationType_RELATION_TYPE_SUPPORTS,
			types.RelationType_RELATION_TYPE_REQUIRES,
			types.RelationType_RELATION_TYPE_REFINES,
			types.RelationType_RELATION_TYPE_GENERALIZES,
			types.RelationType_RELATION_TYPE_CITES:
		default:
			continue
		}
		// GUARD: skip if target fact missing (ghost node).
		if _, ok := k.GetFact(ctx, rel.TargetFactId); !ok {
			continue
		}
		*pathCount++
		edgeKey := rel.SourceFactId + "->" + rel.TargetFactId + "|" + rel.Relation.String()
		if _, ok := edges[edgeKey]; !ok {
			edges[edgeKey] = &types.ToKEdge{
				FromFactId: rel.SourceFactId,
				ToFactId:   rel.TargetFactId,
				Relation:   rel.Relation.String(),
				Inference:  rel.Inference.String(),
			}
		}
		if !visited[rel.TargetFactId] {
			visited[rel.TargetFactId] = true
			if err := k.gatherAncestorsRecursive(ctx, rel.TargetFactId, depth+1, maxDepth, maxPaths, pathCount, visited, edges); err != nil {
				return err
			}
		}
	}
	return nil
}

// GatherFrontier returns the most-recently-verified facts in a domain since a
// given block height, together with all edges among the included set.
//
// "Accepted at block" is approximated by Fact.VerifiedAtBlock — the height at
// which the chain accepted a fact into the knowledge graph. Facts with
// VerifiedAtBlock == 0 (never verified) are always excluded; among the rest,
// facts with VerifiedAtBlock < sel.SinceBlock are also excluded.
// Results are capped at sel.Limit (chain cap: ToKFrontierCap).
func (k Keeper) GatherFrontier(
	ctx context.Context,
	sel *types.FrontierSelector,
) (nodeIDs []string, edges []*types.ToKEdge, err error) {
	if sel == nil || sel.Domain == "" {
		return nil, nil, fmt.Errorf("frontier selector requires a non-empty domain")
	}
	limit := int(sel.Limit)
	if limit <= 0 {
		limit = int(ToKFrontierLimit)
	}
	if limit > int(ToKFrontierCap) {
		limit = int(ToKFrontierCap)
	}

	// Collect qualifying facts by iterating the domain index.
	included := map[string]*types.Fact{}
	k.IterateFactsByDomain(ctx, sel.Domain, func(factID string) bool {
		if len(included) >= limit {
			return true // stop iteration
		}
		fact, ok := k.GetFact(ctx, factID)
		if !ok {
			return false // ghost — skip
		}
		// Filter: exclude unverified facts (VerifiedAtBlock == 0) unconditionally.
		if fact.VerifiedAtBlock == 0 {
			return false // never verified — not part of the knowledge substrate
		}
		// Filter: exclude facts older than the since-block cutoff.
		if fact.VerifiedAtBlock < sel.SinceBlock {
			return false
		}
		included[factID] = fact
		return false
	})

	// Build sorted node list.
	for id := range included {
		nodeIDs = append(nodeIDs, id)
	}
	sort.Strings(nodeIDs)

	// Collect inter-set edges: for each included fact, inspect outgoing relations
	// and keep only those whose target is also in the included set.
	edgeSet := map[string]*types.ToKEdge{}
	for _, factID := range nodeIDs {
		relations, relErr := k.GetFactRelations(ctx, factID)
		if relErr != nil {
			continue
		}
		for _, rel := range relations {
			// Only support-bearing relation types (consistent with subtree/cone).
			switch rel.Relation {
			case types.RelationType_RELATION_TYPE_SUPPORTS,
				types.RelationType_RELATION_TYPE_REQUIRES,
				types.RelationType_RELATION_TYPE_REFINES,
				types.RelationType_RELATION_TYPE_GENERALIZES,
				types.RelationType_RELATION_TYPE_CITES:
			default:
				continue
			}
			// Only include edges where the target is also in the frontier set.
			if _, ok := included[rel.TargetFactId]; !ok {
				continue
			}
			edgeKey := rel.SourceFactId + "->" + rel.TargetFactId + "|" + rel.Relation.String()
			if _, ok := edgeSet[edgeKey]; !ok {
				edgeSet[edgeKey] = &types.ToKEdge{
					FromFactId: rel.SourceFactId,
					ToFactId:   rel.TargetFactId,
					Relation:   rel.Relation.String(),
					Inference:  rel.Inference.String(),
				}
			}
		}
	}
	for _, e := range edgeSet {
		edges = append(edges, e)
	}
	sortToKEdges(edges)
	return nodeIDs, edges, nil
}

func sortToKEdges(edges []*types.ToKEdge) {
	sort.Slice(edges, func(i, j int) bool {
		if edges[i].FromFactId != edges[j].FromFactId {
			return edges[i].FromFactId < edges[j].FromFactId
		}
		if edges[i].ToFactId != edges[j].ToFactId {
			return edges[i].ToFactId < edges[j].ToFactId
		}
		if edges[i].Relation != edges[j].Relation {
			return edges[i].Relation < edges[j].Relation
		}
		return edges[i].Inference < edges[j].Inference
	})
}
