package types

import "cosmossdk.io/errors"

var (
	ErrTokenNotFound       = errors.Register(ModuleName, 2, "token not found")
	ErrInsufficientBalance = errors.Register(ModuleName, 3, "insufficient token balance")
	ErrTokenAlreadyExists  = errors.Register(ModuleName, 4, "token with this symbol already exists")
	ErrUnauthorizedMint    = errors.Register(ModuleName, 5, "sender not authorized to mint this token")
	ErrAllowanceExceeded   = errors.Register(ModuleName, 6, "transfer amount exceeds allowance")
	ErrTokenPaused         = errors.Register(ModuleName, 7, "token is paused")
	ErrNotPausable         = errors.Register(ModuleName, 8, "token does not have pausable feature")
	ErrNotMintable         = errors.Register(ModuleName, 9, "token does not have mintable feature")
	ErrNotBurnable         = errors.Register(ModuleName, 10, "token does not have burnable feature")
	ErrInvalidTransfer     = errors.Register(ModuleName, 11, "invalid transfer")
	ErrSupplyExceeded      = errors.Register(ModuleName, 12, "mint would exceed max supply")
	ErrInvalidAmount       = errors.Register(ModuleName, 13, "invalid amount")
	ErrUnauthorized        = errors.Register(ModuleName, 14, "unauthorized")
	ErrInvalidSymbol       = errors.Register(ModuleName, 15, "invalid symbol: must be 1-16 uppercase alphanumeric characters")
	ErrInvalidName         = errors.Register(ModuleName, 16, "invalid name: must be 1-64 characters")
	ErrSelfTransfer        = errors.Register(ModuleName, 17, "sender and recipient cannot be the same")
	ErrTokenNotPaused      = errors.Register(ModuleName, 18, "token is not paused")

	// Delegation + Wrap errors
	ErrSelfDelegation                 = errors.Register(ModuleName, 20, "cannot delegate power to self")
	ErrNotWrappable                   = errors.Register(ModuleName, 21, "token does not have wrappable feature")
	ErrWrapRecordNotFound             = errors.Register(ModuleName, 22, "wrap record not found for denom")
	ErrInsufficientUndelegatedBalance = errors.Register(ModuleName, 23, "insufficient undelegated balance")

	// Emission errors
	ErrEmissionNotFound     = errors.Register(ModuleName, 30, "emission period not found")
	ErrInvalidEmissionRange = errors.Register(ModuleName, 31, "end_block must be greater than start_block")
	ErrEmissionInactive     = errors.Register(ModuleName, 32, "emission period is already inactive")
)
