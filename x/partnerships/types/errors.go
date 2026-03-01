package types

import "cosmossdk.io/errors"

var (
	ErrPartnershipNotFound  = errors.Register(ModuleName, 2, "partnership not found")
	ErrPartnershipExists    = errors.Register(ModuleName, 3, "partnership already exists between these participants")
	ErrFormationExpired     = errors.Register(ModuleName, 4, "formation window has expired")
	ErrNotParticipant       = errors.Register(ModuleName, 5, "sender is not a participant of this partnership")
	ErrInvalidTier          = errors.Register(ModuleName, 7, "invalid partnership tier")
	ErrInvalidSplit         = errors.Register(ModuleName, 8, "invalid profit split (must sum to 1000000 bps)")
	ErrInsufficientPot      = errors.Register(ModuleName, 9, "insufficient common pot balance")
	ErrCooldownActive       = errors.Register(ModuleName, 12, "rejection cooldown is active")
	ErrFreezeActive         = errors.Register(ModuleName, 13, "safety freeze is currently active")
	ErrMaxFreezesReached    = errors.Register(ModuleName, 14, "maximum freezes per epoch reached")
	ErrCoercionActive       = errors.Register(ModuleName, 15, "coercion signal already active")
	ErrExitInProgress       = errors.Register(ModuleName, 16, "exit is already in progress")
	ErrInvalidStatus        = errors.Register(ModuleName, 17, "invalid partnership status for this operation")
	ErrCounterProposalDepth = errors.Register(ModuleName, 18, "counter-proposal chain depth exceeded")
	ErrNotFormingStatus     = errors.Register(ModuleName, 19, "partnership is not in forming status")
	ErrAlreadyDissolved     = errors.Register(ModuleName, 20, "partnership is already dissolved")
	ErrUnauthorized         = errors.Register(ModuleName, 21, "unauthorized")
	ErrInvalidLockTier      = errors.Register(ModuleName, 22, "invalid lock tier (must be 0-5)")
	ErrLockNotExpired       = errors.Register(ModuleName, 23, "lock period has not yet expired")
	ErrInvalidAmount        = errors.Register(ModuleName, 24, "invalid amount")
	ErrDeliberationActive   = errors.Register(ModuleName, 25, "deliberation is currently active")
	ErrInsufficientDeposit  = errors.Register(ModuleName, 26, "insufficient deposit")

	// Cold-start errors
	ErrSeedLimitExceeded  = errors.Register(ModuleName, 30, "max seed partnerships per DID exceeded")
	ErrSeedExpired        = errors.Register(ModuleName, 31, "seed partnership has expired")
	ErrPoolFull           = errors.Register(ModuleName, 33, "formation pool is full")
	ErrAlreadyInPool      = errors.Register(ModuleName, 39, "already registered in formation pool")
	ErrNotInPool          = errors.Register(ModuleName, 40, "not registered in formation pool")
	ErrSeedPotCapExceeded = errors.Register(ModuleName, 41, "seed partnership common pot cap exceeded")

	// Mentorship errors
	ErrMentorshipNotFound       = errors.Register(ModuleName, 50, "mentorship not found")
	ErrSelfMentorship           = errors.Register(ModuleName, 51, "cannot mentor yourself")
	ErrMaxMentorshipsReached    = errors.Register(ModuleName, 52, "mentor has reached max active mentorships")
	ErrAlreadyMentored          = errors.Register(ModuleName, 53, "mentee already has an active mentorship")
	ErrMentorshipNotProposed    = errors.Register(ModuleName, 54, "mentorship is not in proposed status")
	ErrMentorshipNotActive      = errors.Register(ModuleName, 55, "mentorship is not in active status")
	ErrNotMentorshipParticipant = errors.Register(ModuleName, 56, "sender is not a participant of this mentorship")

	// Formation match errors
	ErrMatchNotFound       = errors.Register(ModuleName, 60, "formation match not found")
	ErrNotMatchParticipant = errors.Register(ModuleName, 61, "sender is not a participant of this match")
	ErrMatchNotProposed    = errors.Register(ModuleName, 62, "match is not in proposed status")

	// Formation freeze errors (R31-3)
	ErrDomainFrozen = errors.Register(ModuleName, 70, "domain is under formation freeze")
)
