package types

import "cosmossdk.io/errors"

var (
	ErrFactNotFound          = errors.Register(ModuleName, 2, "fact not found")
	ErrInvalidErrorType      = errors.Register(ModuleName, 3, "invalid error type")
	ErrEmptyWrongClaim       = errors.Register(ModuleName, 4, "wrong_claim cannot be empty")
	ErrEmptyReasoning        = errors.Register(ModuleName, 5, "reasoning cannot be empty")
	ErrTextTooLong           = errors.Register(ModuleName, 6, "text exceeds max length")
	ErrCounterexampleNotFound = errors.Register(ModuleName, 7, "counterexample not found")
	ErrAlreadyResolved       = errors.Register(ModuleName, 8, "counterexample already resolved")
	ErrAlreadyVoted          = errors.Register(ModuleName, 9, "validator already voted on this counterexample")
	ErrProposalsDisabled     = errors.Register(ModuleName, 10, "counterexample proposals are disabled")
	ErrInvalidAuthority      = errors.Register(ModuleName, 11, "invalid authority")
	ErrInsufficientBond      = errors.Register(ModuleName, 12, "insufficient bond")
)
