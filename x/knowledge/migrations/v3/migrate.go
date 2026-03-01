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

// Migrate backfills sensible defaults for any R29 params that are zero
// (from proto default on chain upgrade where new fields decode as 0).
func Migrate(ctx context.Context, ps ParamStore) error {
	params, err := ps.GetParams(ctx)
	if err != nil {
		return err
	}
	defaults := types.DefaultParams()
	changed := false

	// R29-1 carrying capacity
	if params.DomainBaseCapacity == 0 {
		params.DomainBaseCapacity = defaults.DomainBaseCapacity
		changed = true
	}
	if params.DomainCapacityGrowthPerCitation == 0 {
		params.DomainCapacityGrowthPerCitation = defaults.DomainCapacityGrowthPerCitation
		changed = true
	}
	if params.OvercrowdingDecayMultiplierBps == 0 {
		params.OvercrowdingDecayMultiplierBps = defaults.OvercrowdingDecayMultiplierBps
		changed = true
	}
	if params.UnderpopulationBirthBonusBps == 0 {
		params.UnderpopulationBirthBonusBps = defaults.UnderpopulationBirthBonusBps
		changed = true
	}

	// R29-2 epistemic temperature
	if params.EpistemicTemperatureDecayBps == 0 {
		params.EpistemicTemperatureDecayBps = defaults.EpistemicTemperatureDecayBps
		changed = true
	}
	if params.EpistemicConformityCoolingBps == 0 {
		params.EpistemicConformityCoolingBps = defaults.EpistemicConformityCoolingBps
		changed = true
	}
	if params.EpistemicVindicationHeatingBps == 0 {
		params.EpistemicVindicationHeatingBps = defaults.EpistemicVindicationHeatingBps
		changed = true
	}
	if params.EpistemicColdConfidenceCapBps == 0 {
		params.EpistemicColdConfidenceCapBps = defaults.EpistemicColdConfidenceCapBps
		changed = true
	}
	if params.EpistemicHotConfidenceGrowthBps == 0 {
		params.EpistemicHotConfidenceGrowthBps = defaults.EpistemicHotConfidenceGrowthBps
		changed = true
	}
	if params.EpistemicTemperatureWindowBlocks == 0 {
		params.EpistemicTemperatureWindowBlocks = defaults.EpistemicTemperatureWindowBlocks
		changed = true
	}

	// R29-3 domain role elasticity
	if params.RoleElasticityMinCalls == 0 {
		params.RoleElasticityMinCalls = defaults.RoleElasticityMinCalls
		changed = true
	}
	if params.RoleElasticityMaxMultiplierBps == 0 {
		params.RoleElasticityMaxMultiplierBps = defaults.RoleElasticityMaxMultiplierBps
		changed = true
	}
	if params.RoleElasticityMinMultiplierBps == 0 {
		params.RoleElasticityMinMultiplierBps = defaults.RoleElasticityMinMultiplierBps
		changed = true
	}
	if params.RoleElasticityDecayEpochs == 0 {
		params.RoleElasticityDecayEpochs = defaults.RoleElasticityDecayEpochs
		changed = true
	}

	if changed {
		return ps.SetParams(ctx, params)
	}
	return nil
}
