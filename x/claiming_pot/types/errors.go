package types

import "cosmossdk.io/errors"

var (
	ErrPotNotFound         = errors.Register(ModuleName, 2, "pot not found")
	ErrPotNotActive        = errors.Register(ModuleName, 3, "pot is not active")
	ErrIneligible          = errors.Register(ModuleName, 4, "claimant is not eligible")
	ErrAlreadyClaimed      = errors.Register(ModuleName, 5, "already claimed from this pot")
	ErrInsufficientPotFunds = errors.Register(ModuleName, 6, "insufficient pot funds")
	ErrCliffNotReached     = errors.Register(ModuleName, 7, "cliff period not reached")
	ErrBelowMinClaim       = errors.Register(ModuleName, 8, "claim amount below minimum")
	ErrMaxPotsReached      = errors.Register(ModuleName, 9, "maximum active pots reached")
	ErrInvalidConfig       = errors.Register(ModuleName, 10, "invalid pot configuration")
	ErrUnauthorized        = errors.Register(ModuleName, 11, "unauthorized")

	// ErrCapReached is returned when the bootstrap-claim mint pathway is
	// refused because the 222,222,222 ZRN hard cap has been reached
	// (commitment 19: issuance follows participation, and issuance stops
	// when the substrate has issued all it ever will).
	ErrCapReached = errors.Register(ModuleName, 12, "bootstrap mint refused (commitment 19: issuance follows participation, hard cap reached)")
)
