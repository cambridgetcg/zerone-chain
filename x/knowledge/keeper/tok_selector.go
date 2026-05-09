package keeper

import (
	"context"
	"fmt"
	"sort"

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
	out := &types.ToKSelector{Variant: s.Variant}
	switch v := out.Variant.(type) {
	case *types.ToKSelector_RootedSubtree:
		if v.RootedSubtree.MaxDepth == 0 || v.RootedSubtree.MaxDepth > ToKMaxDepthCap {
			v.RootedSubtree.MaxDepth = ToKMaxDepthCap
		}
	case *types.ToKSelector_AncestorCone:
		if v.AncestorCone.MaxDepth == 0 || v.AncestorCone.MaxDepth > ToKMaxDepthCap {
			v.AncestorCone.MaxDepth = ToKMaxDepthCap
		}
		if v.AncestorCone.MaxPaths == 0 || v.AncestorCone.MaxPaths > ToKMaxPathsCap {
			v.AncestorCone.MaxPaths = ToKMaxPathsCap
		}
	case *types.ToKSelector_Frontier:
		if v.Frontier.Limit == 0 {
			v.Frontier.Limit = ToKFrontierLimit
		}
		if v.Frontier.Limit > ToKFrontierCap {
			v.Frontier.Limit = ToKFrontierCap
		}
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
		edgeKey := rel.SourceFactId + "→" + rel.TargetFactId + "|" + rel.Relation.String()
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

func sortToKEdges(edges []*types.ToKEdge) {
	sort.Slice(edges, func(i, j int) bool {
		if edges[i].FromFactId != edges[j].FromFactId {
			return edges[i].FromFactId < edges[j].FromFactId
		}
		if edges[i].ToFactId != edges[j].ToFactId {
			return edges[i].ToFactId < edges[j].ToFactId
		}
		return edges[i].Relation < edges[j].Relation
	})
}
