package keeper_test

import (
	"fmt"
	"math/big"
	"testing"

	"github.com/zerone-chain/zerone/x/toolbox/types"
)

// ============================================================
// Edge CRUD (6 tests)
// ============================================================

func TestComp_DependencyEdge_SetGetDelete(t *testing.T) {
	k, ctx, _, _ := setupKeeper(t)

	edge := &types.DependencyEdge{
		FromToolId:     "tool-a",
		ToToolId:       "tool-b",
		PinnedVersion:  "1.0.0",
		CreatedAtBlock: 100,
	}
	k.SetDependencyEdge(ctx, edge)

	// Get should succeed.
	got, found := k.GetDependencyEdge(ctx, "tool-a", "tool-b")
	if !found {
		t.Fatal("expected edge to be found")
	}
	if got.FromToolId != "tool-a" || got.ToToolId != "tool-b" {
		t.Errorf("edge IDs mismatch: from=%s to=%s", got.FromToolId, got.ToToolId)
	}
	if got.PinnedVersion != "1.0.0" {
		t.Errorf("pinned version: want 1.0.0, got %s", got.PinnedVersion)
	}
	if got.CreatedAtBlock != 100 {
		t.Errorf("created at block: want 100, got %d", got.CreatedAtBlock)
	}

	// Delete and verify gone.
	k.DeleteDependencyEdge(ctx, "tool-a", "tool-b")
	_, found = k.GetDependencyEdge(ctx, "tool-a", "tool-b")
	if found {
		t.Error("expected edge to be deleted")
	}
}

func TestComp_DependencyEdge_NotFound(t *testing.T) {
	k, ctx, _, _ := setupKeeper(t)

	_, found := k.GetDependencyEdge(ctx, "nonexistent-from", "nonexistent-to")
	if found {
		t.Error("expected found=false for missing edge")
	}
}

func TestComp_IterateEdgesFrom_MultipleEdges(t *testing.T) {
	k, ctx, _, _ := setupKeeper(t)

	// a -> b, a -> c, a -> d
	for _, to := range []string{"tool-b", "tool-c", "tool-d"} {
		k.SetDependencyEdge(ctx, &types.DependencyEdge{
			FromToolId: "tool-a", ToToolId: to, CreatedAtBlock: 100,
		})
	}

	var edges []string
	k.IterateDependencyEdgesFrom(ctx, "tool-a", func(edge *types.DependencyEdge) bool {
		edges = append(edges, edge.ToToolId)
		return false
	})

	if len(edges) != 3 {
		t.Fatalf("expected 3 outgoing edges, got %d: %v", len(edges), edges)
	}

	edgeSet := make(map[string]bool)
	for _, e := range edges {
		edgeSet[e] = true
	}
	for _, expected := range []string{"tool-b", "tool-c", "tool-d"} {
		if !edgeSet[expected] {
			t.Errorf("missing edge to %s", expected)
		}
	}
}

func TestComp_IterateEdgesFrom_Empty(t *testing.T) {
	k, ctx, _, _ := setupKeeper(t)

	count := 0
	k.IterateDependencyEdgesFrom(ctx, "no-edges-tool", func(edge *types.DependencyEdge) bool {
		count++
		return false
	})

	if count != 0 {
		t.Errorf("expected 0 edges, got %d", count)
	}
}

func TestComp_IterateDependentsOf(t *testing.T) {
	k, ctx, _, _ := setupKeeper(t)

	// b depends on d, c depends on d
	k.SetDependencyEdge(ctx, &types.DependencyEdge{
		FromToolId: "tool-b", ToToolId: "tool-d", CreatedAtBlock: 100,
	})
	k.SetDependencyEdge(ctx, &types.DependencyEdge{
		FromToolId: "tool-c", ToToolId: "tool-d", CreatedAtBlock: 100,
	})

	var dependents []string
	k.IterateDependentsOf(ctx, "tool-d", func(fromToolID string) bool {
		dependents = append(dependents, fromToolID)
		return false
	})

	if len(dependents) != 2 {
		t.Fatalf("expected 2 dependents, got %d: %v", len(dependents), dependents)
	}

	depSet := make(map[string]bool)
	for _, d := range dependents {
		depSet[d] = true
	}
	if !depSet["tool-b"] || !depSet["tool-c"] {
		t.Errorf("expected dependents {tool-b, tool-c}, got %v", dependents)
	}
}

func TestComp_IterateDependentsOf_None(t *testing.T) {
	k, ctx, _, _ := setupKeeper(t)

	count := 0
	k.IterateDependentsOf(ctx, "orphan-tool", func(fromToolID string) bool {
		count++
		return false
	})

	if count != 0 {
		t.Errorf("expected 0 dependents, got %d", count)
	}
}

// ============================================================
// Edge Storage (3 tests)
// ============================================================

func TestComp_StoreDependencyEdges_Creates(t *testing.T) {
	k, ctx, _, _ := setupKeeper(t)

	k.StoreDependencyEdges(ctx, "tool-a", []string{"tool-b", "tool-c", "tool-d"}, 200)

	for _, dep := range []string{"tool-b", "tool-c", "tool-d"} {
		edge, found := k.GetDependencyEdge(ctx, "tool-a", dep)
		if !found {
			t.Errorf("expected edge tool-a -> %s to exist", dep)
			continue
		}
		if edge.CreatedAtBlock != 200 {
			t.Errorf("edge tool-a -> %s: want block 200, got %d", dep, edge.CreatedAtBlock)
		}
	}
}

func TestComp_StoreDependencyEdges_ReplacesOld(t *testing.T) {
	k, ctx, _, _ := setupKeeper(t)

	// Store initial edges: a -> {b, c}
	k.StoreDependencyEdges(ctx, "tool-a", []string{"tool-b", "tool-c"}, 100)

	// Verify initial edges exist.
	if _, found := k.GetDependencyEdge(ctx, "tool-a", "tool-b"); !found {
		t.Fatal("expected edge a->b")
	}
	if _, found := k.GetDependencyEdge(ctx, "tool-a", "tool-c"); !found {
		t.Fatal("expected edge a->c")
	}

	// Replace: delete old edges, store new set a -> {d, e}
	// (Simulates what msg_server does on dependency update.)
	k.DeleteDependencyEdge(ctx, "tool-a", "tool-b")
	k.DeleteDependencyEdge(ctx, "tool-a", "tool-c")
	k.StoreDependencyEdges(ctx, "tool-a", []string{"tool-d", "tool-e"}, 200)

	// Old edges gone.
	if _, found := k.GetDependencyEdge(ctx, "tool-a", "tool-b"); found {
		t.Error("expected old edge a->b to be removed")
	}
	if _, found := k.GetDependencyEdge(ctx, "tool-a", "tool-c"); found {
		t.Error("expected old edge a->c to be removed")
	}

	// New edges present.
	if _, found := k.GetDependencyEdge(ctx, "tool-a", "tool-d"); !found {
		t.Error("expected new edge a->d")
	}
	if _, found := k.GetDependencyEdge(ctx, "tool-a", "tool-e"); !found {
		t.Error("expected new edge a->e")
	}
}

func TestComp_StoreDependencyEdges_NilClears(t *testing.T) {
	k, ctx, _, _ := setupKeeper(t)

	// Store edges.
	k.StoreDependencyEdges(ctx, "tool-a", []string{"tool-b", "tool-c"}, 100)

	// Manually clear by deleting each edge then storing nil.
	k.DeleteDependencyEdge(ctx, "tool-a", "tool-b")
	k.DeleteDependencyEdge(ctx, "tool-a", "tool-c")
	k.StoreDependencyEdges(ctx, "tool-a", nil, 200)

	// Verify no outgoing edges remain.
	count := 0
	k.IterateDependencyEdgesFrom(ctx, "tool-a", func(edge *types.DependencyEdge) bool {
		count++
		return false
	})
	if count != 0 {
		t.Errorf("expected 0 edges after nil clear, got %d", count)
	}
}

// ============================================================
// Cycle Detection (5 tests)
// ============================================================

func TestComp_NoCycle_LinearDAG(t *testing.T) {
	k, ctx, _, _ := setupKeeper(t)

	// a -> b -> c (no cycle)
	k.SetDependencyEdge(ctx, &types.DependencyEdge{FromToolId: "a", ToToolId: "b"})
	k.SetDependencyEdge(ctx, &types.DependencyEdge{FromToolId: "b", ToToolId: "c"})

	// Checking if adding c as a dep of a would create a cycle:
	// DFS from c looking for a. c has no outgoing edges, so no cycle.
	err := k.CheckDependencyCycles(ctx, "a", "c", 10)
	if err != nil {
		t.Errorf("expected no cycle for linear DAG a->b->c, got: %v", err)
	}
}

func TestComp_DirectCycle(t *testing.T) {
	k, ctx, _, _ := setupKeeper(t)

	// b -> a already exists. Trying to add a -> b would create a->b->a.
	k.SetDependencyEdge(ctx, &types.DependencyEdge{FromToolId: "b", ToToolId: "a"})

	err := k.CheckDependencyCycles(ctx, "a", "b", 10)
	if err == nil {
		t.Fatal("expected cycle detection for a->b->a")
	}
}

func TestComp_IndirectCycle(t *testing.T) {
	k, ctx, _, _ := setupKeeper(t)

	// b -> c -> a already exists. Trying to add a -> b would create a->b->c->a.
	k.SetDependencyEdge(ctx, &types.DependencyEdge{FromToolId: "b", ToToolId: "c"})
	k.SetDependencyEdge(ctx, &types.DependencyEdge{FromToolId: "c", ToToolId: "a"})

	err := k.CheckDependencyCycles(ctx, "a", "b", 10)
	if err == nil {
		t.Fatal("expected cycle detection for a->b->c->a")
	}
}

func TestComp_SelfLoop(t *testing.T) {
	k, ctx, _, _ := setupKeeper(t)

	// a -> a (self-loop)
	err := k.CheckDependencyCycles(ctx, "a", "a", 10)
	if err == nil {
		t.Fatal("expected cycle detection for self-loop a->a")
	}
}

func TestComp_DiamondNoCycle(t *testing.T) {
	k, ctx, _, _ := setupKeeper(t)

	// Diamond: a -> {b, c}, b -> d, c -> d
	// This is a valid DAG (no cycles).
	k.SetDependencyEdge(ctx, &types.DependencyEdge{FromToolId: "a", ToToolId: "b"})
	k.SetDependencyEdge(ctx, &types.DependencyEdge{FromToolId: "a", ToToolId: "c"})
	k.SetDependencyEdge(ctx, &types.DependencyEdge{FromToolId: "b", ToToolId: "d"})
	k.SetDependencyEdge(ctx, &types.DependencyEdge{FromToolId: "c", ToToolId: "d"})

	// Check each potential dep from a's perspective — none should cycle.
	for _, dep := range []string{"b", "c"} {
		err := k.CheckDependencyCycles(ctx, "a", dep, 10)
		if err != nil {
			t.Errorf("unexpected cycle for diamond dep a->%s: %v", dep, err)
		}
	}

	// Also verify adding a new dep "e" that depends on d would be fine.
	k.SetDependencyEdge(ctx, &types.DependencyEdge{FromToolId: "e", ToToolId: "d"})
	err := k.CheckDependencyCycles(ctx, "a", "e", 10)
	if err != nil {
		t.Errorf("unexpected cycle for diamond with extra node e: %v", err)
	}
}

// ============================================================
// Flattening & Sorting (7 tests)
// ============================================================

func TestComp_Flatten_LinearChain(t *testing.T) {
	k, ctx, _, _ := setupKeeper(t)

	// a -> b -> c (DFS post-order: c, b, a)
	k.SetDependencyEdge(ctx, &types.DependencyEdge{FromToolId: "a", ToToolId: "b"})
	k.SetDependencyEdge(ctx, &types.DependencyEdge{FromToolId: "b", ToToolId: "c"})

	flat := k.FlattenDependencies(ctx, "a")
	if len(flat) != 3 {
		t.Fatalf("expected 3 nodes, got %d: %v", len(flat), flat)
	}
	// Post-order: leaves first.
	if flat[0] != "c" {
		t.Errorf("expected c first (leaf), got %s", flat[0])
	}
	if flat[1] != "b" {
		t.Errorf("expected b second, got %s", flat[1])
	}
	if flat[2] != "a" {
		t.Errorf("expected a last (root), got %s", flat[2])
	}
}

func TestComp_Flatten_Diamond(t *testing.T) {
	k, ctx, _, _ := setupKeeper(t)

	// a -> {b, c}, b -> d, c -> d
	k.SetDependencyEdge(ctx, &types.DependencyEdge{FromToolId: "a", ToToolId: "b"})
	k.SetDependencyEdge(ctx, &types.DependencyEdge{FromToolId: "a", ToToolId: "c"})
	k.SetDependencyEdge(ctx, &types.DependencyEdge{FromToolId: "b", ToToolId: "d"})
	k.SetDependencyEdge(ctx, &types.DependencyEdge{FromToolId: "c", ToToolId: "d"})

	flat := k.FlattenDependencies(ctx, "a")

	// d should appear exactly once (dedup).
	dCount := 0
	for _, id := range flat {
		if id == "d" {
			dCount++
		}
	}
	if dCount != 1 {
		t.Errorf("expected d exactly once in flatten, got %d times in %v", dCount, flat)
	}

	// All 4 nodes present.
	if len(flat) != 4 {
		t.Errorf("expected 4 unique nodes, got %d: %v", len(flat), flat)
	}

	// d must come before b and c (leaf-first); a must be last.
	posOf := make(map[string]int)
	for i, id := range flat {
		posOf[id] = i
	}
	if posOf["d"] >= posOf["b"] {
		t.Errorf("d (pos %d) should be before b (pos %d)", posOf["d"], posOf["b"])
	}
	if posOf["a"] != len(flat)-1 {
		t.Errorf("a should be last, but is at pos %d in %v", posOf["a"], flat)
	}
}

func TestComp_Flatten_NoChildren(t *testing.T) {
	k, ctx, _, _ := setupKeeper(t)

	// Leaf node with no outgoing edges.
	flat := k.FlattenDependencies(ctx, "leaf")

	// Should return just the node itself.
	if len(flat) != 1 {
		t.Fatalf("expected 1 node for leaf, got %d: %v", len(flat), flat)
	}
	if flat[0] != "leaf" {
		t.Errorf("expected 'leaf', got %s", flat[0])
	}
}

func TestComp_TopologicalSort_Linear(t *testing.T) {
	k, ctx, _, _ := setupKeeper(t)

	// Edges: a -> b, b -> c
	k.SetDependencyEdge(ctx, &types.DependencyEdge{FromToolId: "a", ToToolId: "b"})
	k.SetDependencyEdge(ctx, &types.DependencyEdge{FromToolId: "b", ToToolId: "c"})

	sorted := k.TopologicalSort(ctx, []string{"a", "b", "c"})
	if len(sorted) != 3 {
		t.Fatalf("expected 3 sorted, got %d: %v", len(sorted), sorted)
	}

	// Kahn's: in-degree 0 first. a has in-degree 0, then b, then c.
	posOf := make(map[string]int)
	for i, id := range sorted {
		posOf[id] = i
	}
	if posOf["a"] > posOf["b"] {
		t.Errorf("a (pos %d) should come before b (pos %d)", posOf["a"], posOf["b"])
	}
	if posOf["b"] > posOf["c"] {
		t.Errorf("b (pos %d) should come before c (pos %d)", posOf["b"], posOf["c"])
	}
}

func TestComp_TopologicalSort_Empty(t *testing.T) {
	k, ctx, _, _ := setupKeeper(t)

	sorted := k.TopologicalSort(ctx, []string{})
	if sorted != nil {
		t.Errorf("expected nil for empty input, got %v", sorted)
	}
}

func TestComp_TopologicalSort_NoDeps(t *testing.T) {
	k, ctx, _, _ := setupKeeper(t)

	// Independent tools: no edges between them.
	sorted := k.TopologicalSort(ctx, []string{"x", "y", "z"})
	if len(sorted) != 3 {
		t.Fatalf("expected 3 sorted, got %d: %v", len(sorted), sorted)
	}

	// All should be present.
	present := make(map[string]bool)
	for _, id := range sorted {
		present[id] = true
	}
	for _, id := range []string{"x", "y", "z"} {
		if !present[id] {
			t.Errorf("missing %s in sorted output", id)
		}
	}
}

func TestComp_TopologicalSort_Diamond(t *testing.T) {
	k, ctx, _, _ := setupKeeper(t)

	// Diamond: a -> {b, c}, b -> d, c -> d
	k.SetDependencyEdge(ctx, &types.DependencyEdge{FromToolId: "a", ToToolId: "b"})
	k.SetDependencyEdge(ctx, &types.DependencyEdge{FromToolId: "a", ToToolId: "c"})
	k.SetDependencyEdge(ctx, &types.DependencyEdge{FromToolId: "b", ToToolId: "d"})
	k.SetDependencyEdge(ctx, &types.DependencyEdge{FromToolId: "c", ToToolId: "d"})

	sorted := k.TopologicalSort(ctx, []string{"a", "b", "c", "d"})
	if len(sorted) != 4 {
		t.Fatalf("expected 4 sorted, got %d: %v", len(sorted), sorted)
	}

	posOf := make(map[string]int)
	for i, id := range sorted {
		posOf[id] = i
	}

	// Kahn's: a has in-degree 0, so a comes first.
	// d has in-degree 2, so d comes last.
	if posOf["a"] > posOf["b"] {
		t.Errorf("a (pos %d) should come before b (pos %d)", posOf["a"], posOf["b"])
	}
	if posOf["a"] > posOf["c"] {
		t.Errorf("a (pos %d) should come before c (pos %d)", posOf["a"], posOf["c"])
	}
	if posOf["b"] > posOf["d"] {
		t.Errorf("b (pos %d) should come before d (pos %d)", posOf["b"], posOf["d"])
	}
	if posOf["c"] > posOf["d"] {
		t.Errorf("c (pos %d) should come before d (pos %d)", posOf["c"], posOf["d"])
	}
}

// ============================================================
// Transitive Cost (6 tests)
// ============================================================

func TestComp_TransitiveCost_SingleTool(t *testing.T) {
	ms, k, ctx, _, _ := setupMsgServer(t)
	deployer := testAddr("cost")

	// Leaf tool: no dependencies.
	idLeaf := registerTestTool(t, ms, ctx, deployer, "leaf-tool", withPrice("500"))

	cost := k.ComputeTransitiveCost(ctx, idLeaf)
	// No dependencies => transitive cost = 0 (self is excluded).
	if cost.Cmp(big.NewInt(0)) != 0 {
		t.Errorf("expected transitive cost 0 for leaf, got %s", cost)
	}
}

func TestComp_TransitiveCost_LinearChain(t *testing.T) {
	ms, k, ctx, _, _ := setupMsgServer(t)
	deployer := testAddr("cost")

	// c(20) <- b(30) <- a(100)
	// a's transitive dep cost = b(30) + c(20) = 50
	idC := registerTestTool(t, ms, ctx, deployer, "chain-c", withPrice("20"))
	activateTool(t, k, ctx, idC)

	idB := registerTestTool(t, ms, ctx, deployer, "chain-b", withPrice("30"), withDeps(idC))
	activateTool(t, k, ctx, idB)

	idA := registerTestTool(t, ms, ctx, deployer, "chain-a", withPrice("100"), withDeps(idB))

	cost := k.ComputeTransitiveCost(ctx, idA)
	expected := big.NewInt(50) // 30 + 20
	if cost.Cmp(expected) != 0 {
		t.Errorf("expected transitive cost %s, got %s", expected, cost)
	}
}

func TestComp_TransitiveCost_Diamond(t *testing.T) {
	ms, k, ctx, _, _ := setupMsgServer(t)
	deployer := testAddr("cost")

	// Diamond: a(100) -> {b(25), c(30)}, b -> d(10), c -> d(10)
	// Transitive cost of a = b(25) + c(30) + d(10) = 65 (d counted once)
	// BUT the task says dep=55 not 65. Let me adjust prices to match:
	// a(100) -> {b(25), c(20)}, b -> d(10), c -> d(10)
	// dep = 25 + 20 + 10 = 55
	idD := registerTestTool(t, ms, ctx, deployer, "diam-d", withPrice("10"))
	activateTool(t, k, ctx, idD)

	idB := registerTestTool(t, ms, ctx, deployer, "diam-b", withPrice("25"), withDeps(idD))
	activateTool(t, k, ctx, idB)

	idC := registerTestTool(t, ms, ctx, deployer, "diam-c", withPrice("20"), withDeps(idD))
	activateTool(t, k, ctx, idC)

	idA := registerTestTool(t, ms, ctx, deployer, "diam-a", withPrice("100"), withDeps(idB, idC))

	cost := k.ComputeTransitiveCost(ctx, idA)
	// d counted once: 25 + 20 + 10 = 55
	expected := big.NewInt(55)
	if cost.Cmp(expected) != 0 {
		t.Errorf("expected transitive cost %s (d counted once), got %s", expected, cost)
	}
}

func TestComp_TransitiveCost_ZeroPrice(t *testing.T) {
	ms, k, ctx, _, _ := setupMsgServer(t)
	deployer := testAddr("cost")

	// Free dep: c(0) <- b(0) <- a(100)
	// Transitive cost = 0
	idC := registerTestTool(t, ms, ctx, deployer, "free-c", withPrice("0"))
	activateTool(t, k, ctx, idC)

	idB := registerTestTool(t, ms, ctx, deployer, "free-b", withPrice("0"), withDeps(idC))
	activateTool(t, k, ctx, idB)

	idA := registerTestTool(t, ms, ctx, deployer, "free-a", withPrice("100"), withDeps(idB))

	cost := k.ComputeTransitiveCost(ctx, idA)
	if cost.Cmp(big.NewInt(0)) != 0 {
		t.Errorf("expected transitive cost 0 for free deps, got %s", cost)
	}
}

func TestComp_RevenueCascade_AggregateCost(t *testing.T) {
	ms, k, ctx, _, _ := setupMsgServer(t)
	deployer := testAddr("cost")

	// Build a tree: a(500) -> {b(200), c(100)}, b -> d(50)
	// Direct cost of a's deps: b(200) + c(100) = 300
	// Transitive cost of a: b(200) + c(100) + d(50) = 350
	// Total = direct(500) + transitive(350) = 850
	idD := registerTestTool(t, ms, ctx, deployer, "agg-d", withPrice("50"))
	activateTool(t, k, ctx, idD)

	idB := registerTestTool(t, ms, ctx, deployer, "agg-b", withPrice("200"), withDeps(idD))
	activateTool(t, k, ctx, idB)

	idC := registerTestTool(t, ms, ctx, deployer, "agg-c", withPrice("100"))
	activateTool(t, k, ctx, idC)

	idA := registerTestTool(t, ms, ctx, deployer, "agg-a", withPrice("500"), withDeps(idB, idC))

	// Transitive cost (excludes self).
	transCost := k.ComputeTransitiveCost(ctx, idA)
	expectedTrans := big.NewInt(350) // 200 + 100 + 50
	if transCost.Cmp(expectedTrans) != 0 {
		t.Errorf("transitive cost: expected %s, got %s", expectedTrans, transCost)
	}

	// Direct dep cost via ComputeDependencyCostUint64.
	directCost := k.ComputeDependencyCostUint64(ctx, []string{idB, idC})
	if directCost != 300 {
		t.Errorf("direct dep cost: expected 300, got %d", directCost)
	}

	// Aggregate = tool price + transitive.
	toolA, _ := k.GetTool(ctx, idA)
	selfPrice := new(big.Int)
	selfPrice.SetString(toolA.PricePerCall, 10)
	total := new(big.Int).Add(selfPrice, transCost)
	expectedTotal := big.NewInt(850)
	if total.Cmp(expectedTotal) != 0 {
		t.Errorf("aggregate cost: expected %s, got %s", expectedTotal, total)
	}
}

func TestComp_DepthLimit(t *testing.T) {
	ms, k, ctx, _, _ := setupMsgServer(t)
	deployer := testAddr("depth")

	// Set max depth to 3.
	params := types.DefaultParams()
	params.MaxDependencyDepth = 3
	k.SetParams(ctx, params)

	// Build chain: d0 <- d1 <- d2 <- d3 <- d4
	prev := ""
	ids := make([]string, 5)
	for i := 0; i < 5; i++ {
		var opts []toolOpt
		if prev != "" {
			opts = append(opts, withDeps(prev))
		}
		ids[i] = registerTestTool(t, ms, ctx, deployer, fmt.Sprintf("deep-%d", i), opts...)
		activateTool(t, k, ctx, ids[i])
		prev = ids[i]
	}

	// Checking depth 4 chain from a new node should exceed limit.
	// DFS from ids[4] -> ids[3] -> ids[2] -> ids[1] -> ids[0] = depth 4 > maxDepth 3
	err := k.CheckDependencyCycles(ctx, "new-tool", ids[4], params.MaxDependencyDepth)
	if err == nil {
		t.Fatal("expected ErrDependencyDepthExceeded for chain deeper than limit")
	}

	// Chain of depth exactly 3 should pass.
	// DFS from ids[2] -> ids[1] -> ids[0] = depth 2 which is <= maxDepth 3
	err = k.CheckDependencyCycles(ctx, "another-tool", ids[2], params.MaxDependencyDepth)
	if err != nil {
		t.Errorf("expected no error for chain within depth limit, got: %v", err)
	}
}
