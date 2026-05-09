package keeper

import (
	"crypto/sha256"
	"sort"

	"github.com/zerone-chain/zerone/x/knowledge/types"
)

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
