package types

import "cosmossdk.io/errors"

var (
	ErrNotEnabled       = errors.Register(ModuleName, 2, "alignment module is not enabled")
	ErrInvalidWeights   = errors.Register(ModuleName, 3, "dimension weights must sum to 1,000,000")
	ErrInvalidThreshold = errors.Register(ModuleName, 4, "threshold must be between 0 and 1,000,000")
	ErrThresholdOrder   = errors.Register(ModuleName, 5, "thresholds must satisfy: critical < degraded < healthy")
	ErrInvalidInterval  = errors.Register(ModuleName, 6, "observation interval must be > 0")
	ErrUnauthorized         = errors.Register(ModuleName, 7, "unauthorized: sender is not module authority")
	ErrInvalidMaxAutoApply      = errors.Register(ModuleName, 8, "max_auto_apply_magnitude_bps exceeds BPS")
	ErrInvalidConfidenceBounds = errors.Register(ModuleName, 9, "min bounds multiplier exceeds max bounds multiplier")
)
