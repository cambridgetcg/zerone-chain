package v3

import (
	"context"

	"github.com/zerone-chain/zerone/x/knowledge/types"
)

// ParamStore abstracts the keeper methods needed by the v3 migration,
// avoiding an import cycle (migrations/v3 → keeper → migrations/v3).
type ParamStore interface {
	GetParams(ctx context.Context) (*types.Params, error)
	SetParams(ctx context.Context, params *types.Params) error
}

// Migrate performs the v3 migration.
//
// Originally backfilled R29 param defaults (carrying capacity, epistemic
// temperature, role elasticity). After the R36 proto rewrite those fields
// no longer exist, so this migration is now a no-op — the v4 migration
// handles the full param reset.
func Migrate(_ context.Context, _ ParamStore) error {
	return nil
}
