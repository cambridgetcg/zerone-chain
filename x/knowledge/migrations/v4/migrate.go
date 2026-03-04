package v4

import (
	"context"

	"github.com/zerone-chain/zerone/x/knowledge/types"
)

// ParamStore abstracts the keeper methods needed by the v4 migration,
// avoiding an import cycle (migrations/v4 → keeper → migrations/v4).
type ParamStore interface {
	SetParams(ctx context.Context, params *types.Params) error
}

// Migrate performs the v4 migration: fact-claim system → training data protocol.
//
// For testnet: sets fresh default params (quality thresholds, consent multipliers, etc.).
// Old fact/claim state is dropped implicitly — it won't exist on a fresh testnet chain.
//
// For mainnet (future): would need to:
//   - Iterate all existing Facts and convert to Samples (map fields: Content→Content, Domain→Domain, etc.)
//   - Iterate all existing Claims and convert to Submissions
//   - Convert VerificationRounds to QualityRounds
//   - Migrate DemandSignals to TrainingDemand entries
//   - Preserve Bounties (field-compatible)
//   - Recalculate domain stats for sample counts
func Migrate(ctx context.Context, ps ParamStore) error {
	params := types.DefaultParams()
	return ps.SetParams(ctx, &params)
}
