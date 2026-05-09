package types

import "cosmossdk.io/errors"

var (
	ErrUnauthorized           = errors.Register(ModuleName, 2, "unauthorized")
	ErrDirectAnchorDisabled   = errors.Register(ModuleName, 3, "direct anchor is disabled — pin must flow through a Creed Amendment LIP")
	ErrEmptyHash              = errors.Register(ModuleName, 4, "canonical_hash must not be empty")
	ErrVersionNotMonotonic    = errors.Register(ModuleName, 5, "pin version must be exactly current+1")
	ErrDuplicateCommitment    = errors.Register(ModuleName, 6, "commitment number appears more than once in pin")
	ErrCommitmentNumberInvalid = errors.Register(ModuleName, 7, "commitment number must be ≥ 1 and form a contiguous 1..N range when archived entries are excluded")
	ErrPinNotFound            = errors.Register(ModuleName, 8, "no pin at requested version")
	ErrCommitmentNotFound     = errors.Register(ModuleName, 9, "no current entry for requested commitment number")
	ErrSourceLIPRequired      = errors.Register(ModuleName, 10, "source_lip is required when direct_anchor_enabled is false")
	ErrInvalidParams          = errors.Register(ModuleName, 11, "invalid params")
)
