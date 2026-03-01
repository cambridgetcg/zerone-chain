package types

import "cosmossdk.io/errors"

var (
	ErrQualificationNotFound    = errors.Register(ModuleName, 2, "qualification not found")
	ErrQualificationExists      = errors.Register(ModuleName, 3, "qualification already exists")
	ErrInvalidAmount            = errors.Register(ModuleName, 4, "invalid amount")
	ErrInsufficientStake        = errors.Register(ModuleName, 5, "stake below minimum")
	ErrInvalidDomain            = errors.Register(ModuleName, 6, "invalid domain")
	ErrInvalidValidator         = errors.Register(ModuleName, 7, "invalid validator address")
	ErrNotActive                = errors.Register(ModuleName, 8, "qualification is not active")
	ErrAlreadyExpired           = errors.Register(ModuleName, 9, "qualification has expired")
	ErrRenewalTooEarly          = errors.Register(ModuleName, 10, "renewal window not yet open")
	ErrInsufficientTrackRecord  = errors.Register(ModuleName, 11, "insufficient track record")
	ErrInsufficientAccuracy     = errors.Register(ModuleName, 12, "accuracy below minimum")
	ErrCrossRefNotQualified     = errors.Register(ModuleName, 13, "not qualified in source domain")
	ErrCrossRefWeightTooLow     = errors.Register(ModuleName, 14, "source domain weight below minimum")
	ErrInheritanceInvalidStrata = errors.Register(ModuleName, 15, "inheritance requires lower stratum parent")
	ErrInheritanceNotQualified  = errors.Register(ModuleName, 16, "not qualified in parent domain")
	ErrEndorsementNotFound      = errors.Register(ModuleName, 17, "endorsement not found")
	ErrMaxEndorsements          = errors.Register(ModuleName, 18, "maximum endorsements reached")
	ErrSelfEndorsement          = errors.Register(ModuleName, 19, "cannot endorse own qualification")
	ErrInvalidWeight            = errors.Register(ModuleName, 20, "weight must be 1-100")
	ErrNotValidator             = errors.Register(ModuleName, 21, "address is not a validator")
	ErrStakeLocked              = errors.Register(ModuleName, 22, "stake is still locked")
	ErrUnauthorized             = errors.Register(ModuleName, 23, "unauthorized")
	ErrDepthDistanceTooLarge    = errors.Register(ModuleName, 24, "depth distance exceeds maximum for inheritance")
)
