package keeper

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"sort"

	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/zerone-chain/zerone/x/knowledge/types"
)

// Sentinel errors for typed error dispatch in AssembleToKBundle / BundleToK.
var (
	ErrToKRootFactNotFound  = errors.New("root fact not found")
	ErrToKLeafFactNotFound  = errors.New("leaf fact not found")
	ErrToKInconsistentState = errors.New("inconsistent state")
)

// Version constants for provenance fields that have no keeper getter yet.
// GetTraceSchemaVersion and GetTokenizerVersion do not exist on the keeper;
// these constants represent the current deployed versions.
const (
	tokTraceSchemaVersion = "v1"
	tokTokenizerVersion   = "v0"
)

// tokSerialisationFormatJSONL is the canonical serialisation format tag for
// ToK bundles. Task 9 references this constant when writing JSONL payloads.
const tokSerialisationFormatJSONL = "jsonl_adjacency_v1"

// Domain tags for ToK Merkle commitment. Separate tags prevent set-swap
// collisions: a node-ID set cannot produce the same hash as an edge set.
const (
	tokDomainNodes = "TOK_NODES"
	tokDomainEdges = "TOK_EDGES"
	tokDomainRoot  = "TOK_ROOT"
)

// ComputeToKSnapshotRoot returns a 32-byte Merkle commitment over the
// (sorted node IDs, sorted edges) pair, domain-tagged to prevent
// set-swap collisions. Mirrors the ComputeManifestMerkleRoot pattern
// in training_manifest.go (Wave 7).
//
// Shape:
//
//	sha256( "TOK_ROOT" ||
//	        sha256( "TOK_NODES" || node_0 || node_1 || … ) ||
//	        sha256( "TOK_EDGES" || edge_0_canon || edge_1_canon || … ) )
//
// The root is computed from IDs alone, never from payloads. A trainer
// who has the IDs can re-derive the root without trusting the RPC's
// serialisation. TC2: every view is graph-pinned.
//
// Helpers writeLenString and putUint64 are defined in training_manifest.go
// (same package); sortToKEdges is defined in tok_selector.go (same package).
func ComputeToKSnapshotRoot(nodeIDs []string, edges []*types.ToKEdge) []byte {
	// Defensive copies so callers' slices are not mutated.
	sortedNodes := append([]string{}, nodeIDs...)
	sort.Strings(sortedNodes)
	sortedEdges := append([]*types.ToKEdge{}, edges...)
	sortToKEdges(sortedEdges)

	nodesH := tokDomainHash(tokDomainNodes, func(h interface{ Write([]byte) (int, error) }) {
		for _, id := range sortedNodes {
			writeLenString(h, id)
		}
	})

	edgesH := tokDomainHash(tokDomainEdges, func(h interface{ Write([]byte) (int, error) }) {
		for _, e := range sortedEdges {
			// Length-prefix each field individually to prevent field-boundary
			// collisions. A pipe-concatenated canon would make
			// {FromFactId:"a|b", ToFactId:"c"} indistinguishable from
			// {FromFactId:"a", ToFactId:"b|c"}. writeLenString encodes
			// each field as (uint64-length || bytes), so fields with embedded
			// separators cannot collide with fields at a boundary.
			writeLenString(h, e.FromFactId)
			writeLenString(h, e.ToFactId)
			writeLenString(h, e.Relation)
			writeLenString(h, e.Inference)
		}
	})

	// Root: sha256( len("TOK_ROOT") || "TOK_ROOT" || nodesH || edgesH ).
	// nodesH and edgesH are fixed-length (32 bytes each) so no length prefix
	// is needed for them — fixed-width fields cannot produce ambiguity.
	final := sha256.New()
	writeLenString(final, tokDomainRoot)
	_, _ = final.Write(nodesH)
	_, _ = final.Write(edgesH)
	return final.Sum(nil)
}

// tokDomainHash hashes a domain tag followed by whatever the write callback
// appends. Equivalent to the domainHash helper described in the task plan.
// writeLenString is the length-prefixed write helper from training_manifest.go.
func tokDomainHash(domain string, write func(interface{ Write([]byte) (int, error) })) []byte {
	h := sha256.New()
	writeLenString(h, domain)
	write(h)
	return h.Sum(nil)
}

// AssembleToKBundle is the headline ToK extraction primitive. It validates
// the selector, applies caps, gathers IDs, computes the snapshot root,
// materialises payloads, and returns a complete bundle. TC1 (graph is
// the headline) is bound by exposing this through gRPC + CLI as the
// trainer-facing default.
//
// atBlockHeight: pass 0 to use the current block. Non-zero values are
// reserved for historical replay (planned for a future wave). Until the
// keeper supports historical state queries, callers MUST pass 0 — passing
// any other value will record a SnapshotBlock metadata that does NOT
// match the actual state the bundle was built from, breaking TC2's
// re-derivability guarantee.
func (k Keeper) AssembleToKBundle(
	ctx context.Context,
	sel *types.ToKSelector,
	atBlockHeight uint64,
) (*types.ToKBundle, error) {
	capped, err := ValidateAndCapToKSelector(sel)
	if err != nil {
		return nil, err
	}
	nodeIDs, edges, err := k.SelectToKIds(ctx, capped)
	if err != nil {
		return nil, err
	}
	root := ComputeToKSnapshotRoot(nodeIDs, edges)

	// Materialise node payloads.
	var nodes []*types.Fact
	for _, id := range nodeIDs {
		f, ok := k.GetFact(ctx, id)
		if !ok {
			return nil, fmt.Errorf("%w: selected fact %s not found", ErrToKInconsistentState, id)
		}
		nodes = append(nodes, f)
	}

	sdkCtx := sdk.UnwrapSDKContext(ctx)
	if atBlockHeight == 0 {
		atBlockHeight = uint64(sdkCtx.BlockHeight())
	}

	bundle := &types.ToKBundle{
		SnapshotBlock:       atBlockHeight,
		SnapshotRoot:        root,
		IncludedNodeIds:     nodeIDs,
		IncludedEdges:       edges,
		Nodes:               nodes,
		SerialisationFormat: tokSerialisationFormatJSONL,
		Provenance: &types.ToKBundleProvenance{
			ChainId: sdkCtx.ChainID(),
			// GetTraceSchemaVersion / GetTokenizerVersion do not exist on the
			// keeper; use the deployed-version constants until a param or
			// getter is added.
			TraceSchemaVersion:            tokTraceSchemaVersion,
			CanonicalSerialisationVersion: "v1",
			TokenizerVersion:              tokTokenizerVersion,
			SelectorUsed:                  capped,
			CapMaxDepth:                   ToKMaxDepthCap,
			CapMaxPaths:                   ToKMaxPathsCap,
			CapLimit:                      ToKFrontierCap,
		},
	}

	payload, err := SerialiseToK_JSONL(bundle)
	if err != nil {
		return nil, err
	}
	bundle.SerialisedPayload = payload

	sdkCtx.EventManager().EmitEvent(sdk.NewEvent(
		EventTypeToKBundleExtracted,
		sdk.NewAttribute(AttrToKCommitment, "TC0,TC1,TC5"),
		sdk.NewAttribute(AttrToKSelectorKind, selectorKind(capped)),
		sdk.NewAttribute(AttrToKBundleSize, fmt.Sprintf("%d", len(nodeIDs))),
		sdk.NewAttribute(AttrToKSnapshotBlock, fmt.Sprintf("%d", atBlockHeight)),
	))
	sdkCtx.EventManager().EmitEvent(sdk.NewEvent(
		EventTypeToKSnapshotRootPinned,
		sdk.NewAttribute(AttrToKCommitment, "TC0,TC2"),
		sdk.NewAttribute(AttrToKSnapshotRoot, hex.EncodeToString(root)),
	))

	return bundle, nil
}

// SelectToKIds dispatches on the validated selector variant.
func (k Keeper) SelectToKIds(
	ctx context.Context,
	sel *types.ToKSelector,
) (nodeIDs []string, edges []*types.ToKEdge, err error) {
	switch v := sel.Variant.(type) {
	case *types.ToKSelector_RootedSubtree:
		return k.GatherRootedSubtree(ctx, v.RootedSubtree)
	case *types.ToKSelector_AncestorCone:
		return k.GatherAncestorCone(ctx, v.AncestorCone)
	case *types.ToKSelector_Frontier:
		return k.GatherFrontier(ctx, v.Frontier)
	default:
		return nil, nil, fmt.Errorf("selector variant not recognised: %T", sel.Variant)
	}
}

// selectorKind returns a human-readable tag for the selector variant, used as
// the selector_kind event attribute on tok_bundle_extracted events.
func selectorKind(s *types.ToKSelector) string {
	switch s.Variant.(type) {
	case *types.ToKSelector_RootedSubtree:
		return "rooted_subtree"
	case *types.ToKSelector_AncestorCone:
		return "ancestor_cone"
	case *types.ToKSelector_Frontier:
		return "frontier"
	default:
		return "unknown"
	}
}
