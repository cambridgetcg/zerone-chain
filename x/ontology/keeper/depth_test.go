package keeper_test

import (
	"testing"

	"github.com/zerone-chain/zerone/x/ontology/types"
)

func TestRootDomainHasDepth1(t *testing.T) {
	k, ctx, _ := setupKeeper(t)

	// All default genesis domains should have depth 1
	for _, d := range types.DefaultDomains() {
		domain, found := k.GetDomain(ctx, d.Name)
		if !found {
			t.Fatalf("domain %s not found", d.Name)
		}
		depth, err := k.GetDomainDepth(ctx, domain.Name)
		if err != nil {
			t.Fatalf("GetDomainDepth(%s) error: %v", domain.Name, err)
		}
		if depth != 1 {
			t.Errorf("domain %s: expected depth 1, got %d", domain.Name, depth)
		}
	}
}

func TestChildDomainDepthIsParentPlusOne(t *testing.T) {
	k, ctx, _ := setupKeeper(t)

	// Create a child domain under "physics"
	parentDepth, err := k.ComputeDepth(ctx, "") // root
	if err != nil {
		t.Fatalf("ComputeDepth root: %v", err)
	}
	if parentDepth != 1 {
		t.Fatalf("root depth: expected 1, got %d", parentDepth)
	}

	childDepth, err := k.ComputeDepth(ctx, "physics")
	if err != nil {
		t.Fatalf("ComputeDepth(physics): %v", err)
	}
	if childDepth != 2 {
		t.Fatalf("child of physics: expected depth 2, got %d", childDepth)
	}

	// Set a child domain and verify
	k.SetDomain(ctx, &types.Domain{
		Name:         "mechanics",
		DisplayName:  "Mechanics",
		ParentDomain: "physics",
		Depth:        childDepth,
		Stratum:      uint32(types.StratumEmpirical),
		Status:       "active",
	})

	// Now create a grandchild
	grandchildDepth, err := k.ComputeDepth(ctx, "mechanics")
	if err != nil {
		t.Fatalf("ComputeDepth(mechanics): %v", err)
	}
	if grandchildDepth != 3 {
		t.Fatalf("grandchild: expected depth 3, got %d", grandchildDepth)
	}
}

func TestMaxDepthEnforcement(t *testing.T) {
	k, ctx, _ := setupKeeper(t)

	// Build a chain of domains up to max depth
	domains := []struct {
		name   string
		parent string
		depth  uint32
	}{
		{"level2", "physics", 2},
		{"level3", "level2", 3},
		{"level4", "level3", 4},
		{"level5", "level4", 5},
	}

	for _, d := range domains {
		depth, err := k.ComputeDepth(ctx, d.parent)
		if err != nil {
			t.Fatalf("ComputeDepth(%s): %v", d.parent, err)
		}
		if depth != d.depth {
			t.Fatalf("expected depth %d for child of %s, got %d", d.depth, d.parent, depth)
		}
		k.SetDomain(ctx, &types.Domain{
			Name:         d.name,
			ParentDomain: d.parent,
			Depth:        depth,
			Stratum:      uint32(types.StratumEmpirical),
			Status:       "active",
		})
	}

	// Depth 6 should be rejected
	_, err := k.ComputeDepth(ctx, "level5")
	if err == nil {
		t.Fatal("expected error for depth > 5, got nil")
	}
}

func TestComputeDepthParentNotFound(t *testing.T) {
	k, ctx, _ := setupKeeper(t)

	_, err := k.ComputeDepth(ctx, "nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent parent, got nil")
	}
}

func TestGetDomainDepthLegacyDefault(t *testing.T) {
	k, ctx, _ := setupKeeper(t)

	// Set a domain with depth=0 (legacy)
	k.SetDomain(ctx, &types.Domain{
		Name:    "legacy_domain",
		Depth:   0,
		Stratum: uint32(types.StratumEmpirical),
		Status:  "active",
	})

	depth, err := k.GetDomainDepth(ctx, "legacy_domain")
	if err != nil {
		t.Fatalf("GetDomainDepth error: %v", err)
	}
	if depth != 1 {
		t.Errorf("legacy domain: expected depth 1, got %d", depth)
	}
}

func TestGetDomainDepthNotFound(t *testing.T) {
	k, ctx, _ := setupKeeper(t)

	_, err := k.GetDomainDepth(ctx, "nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent domain, got nil")
	}
}

func TestGenesisDomainDepths(t *testing.T) {
	defaults := types.DefaultDomains()
	for _, d := range defaults {
		if d.Depth != 1 {
			t.Errorf("genesis domain %s: expected depth 1, got %d", d.Name, d.Depth)
		}
	}
}
