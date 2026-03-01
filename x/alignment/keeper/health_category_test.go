package keeper_test

import (
	"testing"

	alignmenttypes "github.com/zerone-chain/zerone/x/alignment/types"
)

func TestGetHealthCategory_ReturnsCategoryFromLatestIndex(t *testing.T) {
	k, _, ctx := setupKeeper(t)

	// Store a health index at height 100 with "degraded" category.
	k.SetHealthIndex(ctx, &alignmenttypes.AlignmentHealthIndex{
		Height:         100,
		CompositeScore: 300000,
		Category:       alignmenttypes.CategoryDegraded,
	})

	got := k.GetHealthCategory(ctx)
	if got != alignmenttypes.CategoryDegraded {
		t.Errorf("expected %q, got %q", alignmenttypes.CategoryDegraded, got)
	}
}

func TestGetHealthCategory_ReturnsHealthyWhenNoData(t *testing.T) {
	k, _, ctx := setupKeeper(t)

	got := k.GetHealthCategory(ctx)
	if got != alignmenttypes.CategoryHealthy {
		t.Errorf("expected %q when no health index exists, got %q", alignmenttypes.CategoryHealthy, got)
	}
}
