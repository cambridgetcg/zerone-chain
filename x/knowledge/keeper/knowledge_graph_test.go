package keeper_test

import (
	"fmt"
	"testing"

	sdkmath "cosmossdk.io/math"
	"github.com/stretchr/testify/require"

	"github.com/zerone-chain/zerone/x/knowledge/types"
)

// ─── Test: Create Edge — Happy Path ─────────────────────────────────────────

func TestCreateEdge_HappyPath(t *testing.T) {
	k, ctx := setupKeeper(t)

	msg := &types.MsgCreateEdge{
		Creator:  testAddr,
		SourceID: "tdu-001",
		TargetID: "tdu-002",
		EdgeType: types.EdgeTypeReferences,
		Weight:   "0.800000000000000000",
	}

	resp, err := k.CreateEdge(ctx, msg)
	require.NoError(t, err)
	require.NotEmpty(t, resp.EdgeID)

	// Verify stored.
	edge, found := k.GetKnowledgeEdge(ctx, resp.EdgeID)
	require.True(t, found)
	require.Equal(t, "tdu-001", edge.SourceID)
	require.Equal(t, "tdu-002", edge.TargetID)
	require.Equal(t, types.EdgeTypeReferences, edge.EdgeType)
	require.Equal(t, "0.800000000000000000", edge.GetWeight().String())
}

// ─── Test: Create Edge — Default Weight ─────────────────────────────────────

func TestCreateEdge_DefaultWeight(t *testing.T) {
	k, ctx := setupKeeper(t)

	resp, err := k.CreateEdge(ctx, &types.MsgCreateEdge{
		Creator:  testAddr,
		SourceID: "tdu-a",
		TargetID: "tdu-b",
		EdgeType: types.EdgeTypeExtends,
	})
	require.NoError(t, err)

	edge, found := k.GetKnowledgeEdge(ctx, resp.EdgeID)
	require.True(t, found)
	require.Equal(t, "0.500000000000000000", edge.GetWeight().String())
}

// ─── Test: Reject Self-Referential ──────────────────────────────────────────

func TestCreateEdge_RejectSelfRef(t *testing.T) {
	k, ctx := setupKeeper(t)

	_, err := k.CreateEdge(ctx, &types.MsgCreateEdge{
		Creator:  testAddr,
		SourceID: "tdu-001",
		TargetID: "tdu-001",
		EdgeType: types.EdgeTypeReferences,
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "self-referential")
}

// ─── Test: Reject Duplicate ─────────────────────────────────────────────────

func TestCreateEdge_RejectDuplicate(t *testing.T) {
	k, ctx := setupKeeper(t)

	msg := &types.MsgCreateEdge{
		Creator:  testAddr,
		SourceID: "tdu-001",
		TargetID: "tdu-002",
		EdgeType: types.EdgeTypeReferences,
	}

	_, err := k.CreateEdge(ctx, msg)
	require.NoError(t, err)

	_, err = k.CreateEdge(ctx, msg)
	require.Error(t, err)
	require.Contains(t, err.Error(), "already exists")
}

// ─── Test: Reject Invalid Edge Type ─────────────────────────────────────────

func TestCreateEdge_RejectInvalidType(t *testing.T) {
	k, ctx := setupKeeper(t)

	_, err := k.CreateEdge(ctx, &types.MsgCreateEdge{
		Creator:  testAddr,
		SourceID: "tdu-001",
		TargetID: "tdu-002",
		EdgeType: "invalid_type",
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "invalid edge type")
}

// ─── Test: Remove Edge ──────────────────────────────────────────────────────

func TestRemoveEdge(t *testing.T) {
	k, ctx := setupKeeper(t)

	resp, err := k.CreateEdge(ctx, &types.MsgCreateEdge{
		Creator:  testAddr,
		SourceID: "tdu-001",
		TargetID: "tdu-002",
		EdgeType: types.EdgeTypeReferences,
	})
	require.NoError(t, err)

	err = k.RemoveEdge(ctx, resp.EdgeID, testAddr)
	require.NoError(t, err)

	_, found := k.GetKnowledgeEdge(ctx, resp.EdgeID)
	require.False(t, found)

	// Indexes should be clean.
	outgoing := k.GetOutgoingEdges(ctx, "tdu-001")
	require.Len(t, outgoing, 0)
}

// ─── Test: Remove Edge — Unauthorized ───────────────────────────────────────

func TestRemoveEdge_Unauthorized(t *testing.T) {
	k, ctx := setupKeeper(t)

	resp, err := k.CreateEdge(ctx, &types.MsgCreateEdge{
		Creator:  testAddr,
		SourceID: "tdu-001",
		TargetID: "tdu-002",
		EdgeType: types.EdgeTypeReferences,
	})
	require.NoError(t, err)

	err = k.RemoveEdge(ctx, resp.EdgeID, "zrn1wrongaddress000000000000000000000000")
	require.Error(t, err)
	require.Contains(t, err.Error(), "nauthorized")
}

// ─── Test: Outgoing and Incoming Edges ──────────────────────────────────────

func TestEdgeDirectionality(t *testing.T) {
	k, ctx := setupKeeper(t)

	// A → B (references), A → C (extends), D → A (corrects)
	_, err := k.CreateEdge(ctx, &types.MsgCreateEdge{Creator: testAddr, SourceID: "A", TargetID: "B", EdgeType: types.EdgeTypeReferences})
	require.NoError(t, err)
	_, err = k.CreateEdge(ctx, &types.MsgCreateEdge{Creator: testAddr, SourceID: "A", TargetID: "C", EdgeType: types.EdgeTypeExtends})
	require.NoError(t, err)
	_, err = k.CreateEdge(ctx, &types.MsgCreateEdge{Creator: testAddr, SourceID: "D", TargetID: "A", EdgeType: types.EdgeTypeCorrects})
	require.NoError(t, err)

	// A has 2 outgoing.
	outgoing := k.GetOutgoingEdges(ctx, "A")
	require.Len(t, outgoing, 2)

	// A has 1 incoming.
	incoming := k.GetIncomingEdges(ctx, "A")
	require.Len(t, incoming, 1)
	require.Equal(t, "D", incoming[0].SourceID)

	// B has 0 outgoing, 1 incoming.
	require.Len(t, k.GetOutgoingEdges(ctx, "B"), 0)
	require.Len(t, k.GetIncomingEdges(ctx, "B"), 1)
}

// ─── Test: GetNeighbors ─────────────────────────────────────────────────────

func TestGetNeighbors(t *testing.T) {
	k, ctx := setupKeeper(t)

	// A → B, C → A, A → D
	_, _ = k.CreateEdge(ctx, &types.MsgCreateEdge{Creator: testAddr, SourceID: "A", TargetID: "B", EdgeType: types.EdgeTypeReferences})
	_, _ = k.CreateEdge(ctx, &types.MsgCreateEdge{Creator: testAddr, SourceID: "C", TargetID: "A", EdgeType: types.EdgeTypeExtends})
	_, _ = k.CreateEdge(ctx, &types.MsgCreateEdge{Creator: testAddr, SourceID: "A", TargetID: "D", EdgeType: types.EdgeTypeExemplifies})

	neighbors := k.GetNeighbors(ctx, "A")
	require.Len(t, neighbors, 3) // B, D (outgoing) + C (incoming)
}

// ─── Test: Connectivity ─────────────────────────────────────────────────────

func TestGetConnectivity(t *testing.T) {
	k, ctx := setupKeeper(t)

	_, _ = k.CreateEdge(ctx, &types.MsgCreateEdge{Creator: testAddr, SourceID: "X", TargetID: "Y", EdgeType: types.EdgeTypeReferences})
	_, _ = k.CreateEdge(ctx, &types.MsgCreateEdge{Creator: testAddr, SourceID: "X", TargetID: "Z", EdgeType: types.EdgeTypeExtends})
	_, _ = k.CreateEdge(ctx, &types.MsgCreateEdge{Creator: testAddr, SourceID: "W", TargetID: "X", EdgeType: types.EdgeTypePartOf})

	conn := k.GetConnectivity(ctx, "X")
	require.Equal(t, uint64(3), conn) // 2 out + 1 in
}

// ─── Test: Fitness Propagation ──────────────────────────────────────────────

func TestPropagateFitnessChange(t *testing.T) {
	k, ctx := setupKeeper(t)

	// Create chain: A → B → C.
	_, _ = k.CreateEdge(ctx, &types.MsgCreateEdge{
		Creator: testAddr, SourceID: "prop-A", TargetID: "prop-B",
		EdgeType: types.EdgeTypeReferences, Weight: "1.000000000000000000",
	})
	_, _ = k.CreateEdge(ctx, &types.MsgCreateEdge{
		Creator: testAddr, SourceID: "prop-B", TargetID: "prop-C",
		EdgeType: types.EdgeTypeReferences, Weight: "1.000000000000000000",
	})

	// Set initial fitness for B and C.
	frB := types.NewTDUFitnessRecord("prop-B", sdkmath.NewInt(1000000), 0)
	frB.SetFitnessScore(sdkmath.LegacyNewDecWithPrec(5, 1)) // 0.5
	require.NoError(t, k.SetFitnessRecord(ctx, frB))

	frC := types.NewTDUFitnessRecord("prop-C", sdkmath.NewInt(1000000), 0)
	frC.SetFitnessScore(sdkmath.LegacyNewDecWithPrec(5, 1)) // 0.5
	require.NoError(t, k.SetFitnessRecord(ctx, frC))

	// Propagate +0.2 from A.
	delta := sdkmath.LegacyNewDecWithPrec(2, 1) // 0.2
	signals := k.PropagateFitnessChange(ctx, "prop-A", delta)

	// Should have signals for B and C.
	require.GreaterOrEqual(t, len(signals), 1)

	// B should have received: 0.2 × 1.0 × 0.2 = 0.04 boost.
	frB, found := k.GetFitnessRecord(ctx, "prop-B")
	require.True(t, found)
	bScore := frB.GetFitnessScore()
	require.True(t, bScore.GT(sdkmath.LegacyNewDecWithPrec(5, 1)), "B fitness should increase, got %s", bScore)
}

// ─── Test: Propagation — Zero Delta ─────────────────────────────────────────

func TestPropagateFitnessChange_ZeroDelta(t *testing.T) {
	k, ctx := setupKeeper(t)
	signals := k.PropagateFitnessChange(ctx, "tdu-001", sdkmath.LegacyZeroDec())
	require.Nil(t, signals)
}

// ─── Test: Correction Inverse Propagation ───────────────────────────────────

func TestPropagation_CorrectionInverse(t *testing.T) {
	k, ctx := setupKeeper(t)

	// B corrects A (B → A with type "corrects").
	_, _ = k.CreateEdge(ctx, &types.MsgCreateEdge{
		Creator: testAddr, SourceID: "corr-B", TargetID: "corr-A",
		EdgeType: types.EdgeTypeCorrects, Weight: "1.000000000000000000",
	})

	// Set initial fitness for B (the source of correction).
	frB := types.NewTDUFitnessRecord("corr-B", sdkmath.NewInt(1000000), 0)
	frB.SetFitnessScore(sdkmath.LegacyNewDecWithPrec(5, 1)) // 0.5
	require.NoError(t, k.SetFitnessRecord(ctx, frB))

	// When A improves, B (which corrects A) should get an inverse signal.
	delta := sdkmath.LegacyNewDecWithPrec(3, 1) // 0.3
	signals := k.PropagateFitnessChange(ctx, "corr-A", delta)

	// Should have at least one signal (inverse to corr-B).
	require.GreaterOrEqual(t, len(signals), 1)
}

// ─── Test: Cluster Operations ───────────────────────────────────────────────

func TestClusterCRUD(t *testing.T) {
	k, ctx := setupKeeper(t)

	cluster := &types.KnowledgeCluster{
		ClusterID:   "cluster-go-http",
		Domain:      "code/go",
		Label:       "Go HTTP Handlers",
		MemberIDs:   []string{"tdu-http-1", "tdu-http-2", "tdu-http-3"},
		CoreMembers: []string{"tdu-http-1"},
		EdgeCount:   5,
		AvgFitness:  "0.700000000000000000",
		CreatedAt:   100,
		UpdatedAt:   100,
	}

	err := k.SetCluster(ctx, cluster)
	require.NoError(t, err)

	// Get by ID.
	got, found := k.GetCluster(ctx, "cluster-go-http")
	require.True(t, found)
	require.Equal(t, "Go HTTP Handlers", got.Label)
	require.Len(t, got.MemberIDs, 3)

	// Get by member.
	gotByMember, found := k.GetClusterByMember(ctx, "tdu-http-2")
	require.True(t, found)
	require.Equal(t, "cluster-go-http", gotByMember.ClusterID)

	// Non-existent.
	_, found = k.GetClusterByMember(ctx, "tdu-nonexistent")
	require.False(t, found)
}

// ─── Test: Graph Stats ──────────────────────────────────────────────────────

func TestGraphStats(t *testing.T) {
	k, ctx := setupKeeper(t)

	// Create 3 edges.
	for i, pair := range [][2]string{{"s1", "t1"}, {"s2", "t2"}, {"s3", "t3"}} {
		_ = i
		_, _ = k.CreateEdge(ctx, &types.MsgCreateEdge{
			Creator: testAddr, SourceID: pair[0], TargetID: pair[1],
			EdgeType: types.EdgeTypeReferences,
		})
	}

	// Create 1 cluster.
	_ = k.SetCluster(ctx, &types.KnowledgeCluster{
		ClusterID: "c1", Domain: "code/go", MemberIDs: []string{"s1"},
	})

	edges, clusters := k.GetGraphStats(ctx)
	require.Equal(t, uint64(3), edges)
	require.Equal(t, uint64(1), clusters)
}

// ─── Test: All Edge Types ───────────────────────────────────────────────────

func TestAllEdgeTypes(t *testing.T) {
	k, ctx := setupKeeper(t)

	allTypes := []types.EdgeType{
		types.EdgeTypeReferences, types.EdgeTypeExtends, types.EdgeTypeExemplifies,
		types.EdgeTypeCorrects, types.EdgeTypeSupersedes, types.EdgeTypeContradicts,
		types.EdgeTypePartOf, types.EdgeTypePrerequisite, types.EdgeTypeSimilar,
	}

	for i, et := range allTypes {
		_, err := k.CreateEdge(ctx, &types.MsgCreateEdge{
			Creator:  testAddr,
			SourceID: fmt.Sprintf("src-%d", i),
			TargetID: fmt.Sprintf("tgt-%d", i),
			EdgeType: et,
		})
		require.NoError(t, err, "edge type %s should be valid", et)
	}

	// Query by type.
	refs := k.GetEdgesByType(ctx, types.EdgeTypeReferences)
	require.Len(t, refs, 1)
}
