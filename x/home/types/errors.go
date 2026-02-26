package types

import "cosmossdk.io/errors"

var (
	ErrHomeNotFound          = errors.Register(ModuleName, 2, "home not found")
	ErrUnauthorized          = errors.Register(ModuleName, 3, "unauthorized")
	ErrInvalidStatus         = errors.Register(ModuleName, 4, "invalid status transition")
	ErrKeyNotFound           = errors.Register(ModuleName, 5, "key not found")
	ErrKeyAlreadyRegistered  = errors.Register(ModuleName, 6, "key already registered")
	ErrKeyRevoked            = errors.Register(ModuleName, 7, "key has been revoked")
	ErrKeyExpired            = errors.Register(ModuleName, 8, "key has expired")
	ErrMaxKeysReached        = errors.Register(ModuleName, 9, "maximum keys per home reached")
	ErrMaxSessionsReached    = errors.Register(ModuleName, 10, "maximum sessions per home reached")
	ErrSessionNotFound       = errors.Register(ModuleName, 11, "session not found")
	ErrAlertNotFound         = errors.Register(ModuleName, 12, "alert not found")
	ErrInvalidAmount         = errors.Register(ModuleName, 13, "invalid amount")
	ErrSpendingLimitExceeded = errors.Register(ModuleName, 14, "spending limit exceeded")
	ErrInvalidGuardianConfig = errors.Register(ModuleName, 15, "invalid guardian configuration")
	ErrInvalidDeadmanConfig  = errors.Register(ModuleName, 16, "invalid deadman configuration")
	ErrInsufficientFunds     = errors.Register(ModuleName, 17, "insufficient funds for home creation")
	ErrPermissionDenied      = errors.Register(ModuleName, 18, "permission denied")
	ErrMaxAlertsReached      = errors.Register(ModuleName, 19, "maximum alerts per home reached")
	ErrInvalidInput          = errors.Register(ModuleName, 20, "invalid input")
)
