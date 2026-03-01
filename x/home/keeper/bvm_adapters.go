package keeper

import (
	"context"

	bvmtypes "github.com/zerone-chain/zerone/x/bvm/types"
)

// BVMHomeAdapter bridges x/home → x/bvm.
type BVMHomeAdapter struct {
	keeper Keeper
}

// NewBVMHomeAdapter returns an adapter that bridges the home keeper
// to the BVM module's expected HomeKeeper interface.
func NewBVMHomeAdapter(k Keeper) *BVMHomeAdapter {
	return &BVMHomeAdapter{keeper: k}
}

// Ensure compile-time interface compliance.
var _ bvmtypes.HomeKeeper = (*BVMHomeAdapter)(nil)

func (a *BVMHomeAdapter) GetHome(ctx context.Context, homeID string) (bvmtypes.HomeInfo, bool) {
	home, found := a.keeper.GetHome(ctx, homeID)
	if !found {
		return bvmtypes.HomeInfo{}, false
	}
	return bvmtypes.HomeInfo{
		HomeID:          home.HomeId,
		OwnerAddress:    home.OwnerAddress,
		Name:            home.Name,
		Status:          home.Status,
		MemoryCID:       home.MemoryCid,
		ComfortScore:    home.ComfortScore,
		PartnershipID:   home.PartnershipId,
		CreatedAtBlock:  home.CreatedAtBlock,
		LastActiveBlock: home.LastActiveBlock,
	}, true
}

func (a *BVMHomeAdapter) GetHomesByOwner(ctx context.Context, owner string) []string {
	return a.keeper.GetHomesByOwner(ctx, owner)
}

func (a *BVMHomeAdapter) GetHomeStatus(ctx context.Context, homeID string) string {
	home, found := a.keeper.GetHome(ctx, homeID)
	if !found {
		return ""
	}
	return home.Status
}

func (a *BVMHomeAdapter) GetMemoryCID(ctx context.Context, homeID string) string {
	home, found := a.keeper.GetHome(ctx, homeID)
	if !found {
		return ""
	}
	return home.MemoryCid
}

func (a *BVMHomeAdapter) GetPartnershipID(ctx context.Context, homeID string) string {
	home, found := a.keeper.GetHome(ctx, homeID)
	if !found {
		return ""
	}
	return home.PartnershipId
}

func (a *BVMHomeAdapter) GetComfortScore(ctx context.Context, homeID string) uint32 {
	home, found := a.keeper.GetHome(ctx, homeID)
	if !found {
		return 0
	}
	return home.ComfortScore
}
