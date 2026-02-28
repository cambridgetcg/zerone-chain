package keeper_test

import (
	"context"
	"testing"

	"github.com/zerone-chain/zerone/x/qualification/types"
)

// mockOntologyKeeper implements types.OntologyKeeper for tests.
type mockOntologyKeeper struct {
	depths map[string]uint32
}

func newMockOntologyKeeper() *mockOntologyKeeper {
	return &mockOntologyKeeper{depths: make(map[string]uint32)}
}

func (m *mockOntologyKeeper) setDepth(domain string, depth uint32) {
	m.depths[domain] = depth
}

func (m *mockOntologyKeeper) GetDepthForDomain(_ context.Context, domainName string) (uint32, error) {
	d, ok := m.depths[domainName]
	if !ok {
		return 1, nil // default
	}
	return d, nil
}

func TestCrossRefDiscountScalesWithDepthDiff(t *testing.T) {
	k, ctx, bk, _ := setupKeeper(t)

	ok := newMockOntologyKeeper()
	ok.setDepth("mathematics", 1)
	ok.setDepth("physics", 1)
	ok.setDepth("mechanics", 3)
	k.SetOntologyKeeper(ok)

	val := testAddr("depth-crossref-1")
	bk.setBalance(val, "uzrn", 200_000_000)

	// Qualify in mathematics (depth=1)
	err := k.QualifyByStake(ctx, val, "mathematics", "100000000")
	if err != nil {
		t.Fatalf("QualifyByStake: %v", err)
	}

	// Cross-ref to physics (same depth=1): depth diff = 0, no discount
	err = k.QualifyByCrossReference(ctx, val, "physics", "mathematics")
	if err != nil {
		t.Fatalf("QualifyByCrossReference(depth diff 0): %v", err)
	}
	q, _ := k.GetQualification(ctx, val, "physics")
	// depth diff 0 → discount = 200000 * 0 = 0 → weight = 50 (no discount)
	if q.Weight != 50 {
		t.Errorf("same depth: expected weight 50, got %d", q.Weight)
	}

	// Cross-ref to mechanics (depth=3): depth diff = 2
	err = k.QualifyByCrossReference(ctx, val, "mechanics", "mathematics")
	if err != nil {
		t.Fatalf("QualifyByCrossReference(depth diff 2): %v", err)
	}
	q2, _ := k.GetQualification(ctx, val, "mechanics")
	// depth diff 2 → discount = 200000 * 2 = 400000 (40%) → weight = 50 * 0.6 = 30
	if q2.Weight != 30 {
		t.Errorf("depth diff 2: expected weight 30, got %d", q2.Weight)
	}
}

func TestInheritanceDiscountScalesWithDepthDiff(t *testing.T) {
	k, ctx, _, _ := setupKeeper(t)

	ok := newMockOntologyKeeper()
	ok.setDepth("science", 1)
	ok.setDepth("physics", 2)
	ok.setDepth("mechanics", 3)
	k.SetOntologyKeeper(ok)

	val := testAddr("depth-inherit-1")

	// Parent qualification at depth 1
	k.SetQualification(ctx, &types.DomainQualification{
		Validator: val,
		Domain:    "science",
		Pathway:   types.QualificationPathway_QUALIFICATION_PATHWAY_STAKE,
		Status:    types.QualificationStatus_QUALIFICATION_STATUS_ACTIVE,
		Weight:    100,
	})

	// Inherit to physics (depth 2): diff = 1
	err := k.QualifyByInheritance(ctx, val, "physics", "science")
	if err != nil {
		t.Fatalf("QualifyByInheritance(diff 1): %v", err)
	}
	q, _ := k.GetQualification(ctx, val, "physics")
	// diff 1 → discount = 300000 * 1 = 300000 (30%) → weight = 100 * 0.7 = 70
	if q.Weight != 70 {
		t.Errorf("diff 1: expected weight 70, got %d", q.Weight)
	}

	// Reset and test with depth diff 2
	val2 := testAddr("depth-inherit-2")
	k.SetQualification(ctx, &types.DomainQualification{
		Validator: val2,
		Domain:    "science",
		Pathway:   types.QualificationPathway_QUALIFICATION_PATHWAY_STAKE,
		Status:    types.QualificationStatus_QUALIFICATION_STATUS_ACTIVE,
		Weight:    100,
	})

	err = k.QualifyByInheritance(ctx, val2, "mechanics", "science")
	if err != nil {
		t.Fatalf("QualifyByInheritance(diff 2): %v", err)
	}
	q2, _ := k.GetQualification(ctx, val2, "mechanics")
	// diff 2 → discount = 300000 * 2 = 600000 (60%) → weight = 100 * 0.4 = 40
	if q2.Weight != 40 {
		t.Errorf("diff 2: expected weight 40, got %d", q2.Weight)
	}
}

func TestInheritanceBlockedAtDepthDiff4(t *testing.T) {
	k, ctx, _, _ := setupKeeper(t)

	ok := newMockOntologyKeeper()
	ok.setDepth("root", 1)
	ok.setDepth("deep", 5)
	k.SetOntologyKeeper(ok)

	val := testAddr("depth-block-1")
	k.SetQualification(ctx, &types.DomainQualification{
		Validator: val,
		Domain:    "root",
		Pathway:   types.QualificationPathway_QUALIFICATION_PATHWAY_STAKE,
		Status:    types.QualificationStatus_QUALIFICATION_STATUS_ACTIVE,
		Weight:    100,
	})

	// Depth diff = 4 > max 3 → should be blocked
	err := k.QualifyByInheritance(ctx, val, "deep", "root")
	if err == nil {
		t.Fatal("expected error for depth diff > 3, got nil")
	}
}

func TestInheritanceAllowedAtDepthDiff3(t *testing.T) {
	k, ctx, _, _ := setupKeeper(t)

	ok := newMockOntologyKeeper()
	ok.setDepth("d1", 1)
	ok.setDepth("d4", 4)
	k.SetOntologyKeeper(ok)

	val := testAddr("depth-allow-3")
	k.SetQualification(ctx, &types.DomainQualification{
		Validator: val,
		Domain:    "d1",
		Pathway:   types.QualificationPathway_QUALIFICATION_PATHWAY_STAKE,
		Status:    types.QualificationStatus_QUALIFICATION_STATUS_ACTIVE,
		Weight:    100,
	})

	// Depth diff = 3 → exactly at limit, should be allowed
	err := k.QualifyByInheritance(ctx, val, "d4", "d1")
	if err != nil {
		t.Fatalf("expected inheritance at depth diff 3, got: %v", err)
	}
}

func TestCrossRefNoMaxDistanceLimit(t *testing.T) {
	k, ctx, _, _ := setupKeeper(t)

	ok := newMockOntologyKeeper()
	ok.setDepth("shallow", 1)
	ok.setDepth("deep", 5)
	k.SetOntologyKeeper(ok)

	val := testAddr("crossref-nomax")
	k.SetQualification(ctx, &types.DomainQualification{
		Validator: val,
		Domain:    "shallow",
		Pathway:   types.QualificationPathway_QUALIFICATION_PATHWAY_STAKE,
		Status:    types.QualificationStatus_QUALIFICATION_STATUS_ACTIVE,
		Weight:    100,
	})

	// Depth diff = 4 → allowed for cross-ref (no max), but steep discount
	err := k.QualifyByCrossReference(ctx, val, "deep", "shallow")
	if err != nil {
		t.Fatalf("cross-ref should allow any depth distance, got: %v", err)
	}
	q, _ := k.GetQualification(ctx, val, "deep")
	// diff 4 → discount = 200000 * 4 = 800000 (80%) → weight = 100 * 0.2 = 20
	if q.Weight != 20 {
		t.Errorf("depth diff 4: expected weight 20, got %d", q.Weight)
	}
}

func TestDepthDiffZeroMeansNoDiscount(t *testing.T) {
	k, ctx, _, _ := setupKeeper(t)

	ok := newMockOntologyKeeper()
	ok.setDepth("domainA", 2)
	ok.setDepth("domainB", 2)
	k.SetOntologyKeeper(ok)

	val := testAddr("depth-zero")
	k.SetQualification(ctx, &types.DomainQualification{
		Validator: val,
		Domain:    "domainA",
		Pathway:   types.QualificationPathway_QUALIFICATION_PATHWAY_STAKE,
		Status:    types.QualificationStatus_QUALIFICATION_STATUS_ACTIVE,
		Weight:    80,
	})

	// Same depth → diff = 0 → no discount
	err := k.QualifyByInheritance(ctx, val, "domainB", "domainA")
	if err != nil {
		t.Fatalf("inheritance at depth diff 0: %v", err)
	}
	q, _ := k.GetQualification(ctx, val, "domainB")
	// diff 0 → discount = 300000 * 0 = 0 → weight = 80
	if q.Weight != 80 {
		t.Errorf("depth diff 0: expected weight 80, got %d", q.Weight)
	}
}
