package types

import "cosmossdk.io/errors"

// Knowledge module sentinel errors.
var (
	// ─── Submission & sample lifecycle (2–13) ───────────────────────────────
	ErrInvalidSubmission   = errors.Register(ModuleName, 2, "invalid submission")
	ErrDuplicateContent    = errors.Register(ModuleName, 3, "duplicate content")
	ErrInvalidConsent      = errors.Register(ModuleName, 4, "invalid consent proof")
	ErrSampleNotFound      = errors.Register(ModuleName, 5, "sample not found")
	ErrSubmissionNotFound  = errors.Register(ModuleName, 6, "submission not found")
	ErrDatasetNotFound     = errors.Register(ModuleName, 7, "dataset not found")
	ErrInvalidQualityScore = errors.Register(ModuleName, 8, "invalid quality score")
	ErrRoundNotFound       = errors.Register(ModuleName, 9, "quality round not found")
	ErrConsentRequired     = errors.Register(ModuleName, 10, "consent proof required")
	ErrContentTooLarge     = errors.Register(ModuleName, 11, "content exceeds max bytes")
	ErrThreadTooLarge      = errors.Register(ModuleName, 12, "thread exceeds max size")
	ErrInsufficientPayment = errors.Register(ModuleName, 13, "insufficient payment")

	// ─── Domain & validation (15–22) ────────────────────────────────────────
	ErrInvalidDomain    = errors.Register(ModuleName, 15, "invalid domain")
	ErrDomainNotFound   = errors.Register(ModuleName, 16, "domain not found")
	ErrDomainExists     = errors.Register(ModuleName, 17, "domain already exists")
	ErrInsufficientStake = errors.Register(ModuleName, 18, "insufficient stake")
	ErrWrongPhase       = errors.Register(ModuleName, 21, "wrong verification phase")
	ErrDeadlinePassed   = errors.Register(ModuleName, 22, "verification deadline has passed")

	// ─── Commit-reveal verification (23–27) ─────────────────────────────────
	ErrDuplicateCommitment  = errors.Register(ModuleName, 23, "duplicate commitment")
	ErrInvalidCommitment    = errors.Register(ModuleName, 24, "invalid commitment hash")
	ErrNoCommitment         = errors.Register(ModuleName, 25, "no commitment found")
	ErrRevealMismatch       = errors.Register(ModuleName, 26, "reveal does not match commitment")
	ErrDuplicateReveal      = errors.Register(ModuleName, 27, "duplicate reveal")

	// ─── Validator selection (30–34) ────────────────────────────────────────
	ErrNotSelectedValidator = errors.Register(ModuleName, 30, "validator not selected for this round")
	ErrAlreadyCommitted     = errors.Register(ModuleName, 31, "validator already committed")
	ErrAlreadyRevealed      = errors.Register(ModuleName, 32, "validator already revealed")
	ErrInvalidVRFProof      = errors.Register(ModuleName, 33, "invalid VRF proof")
	ErrValidatorNotEligible = errors.Register(ModuleName, 34, "validator not eligible for this domain")
	ErrEquivocation         = errors.Register(ModuleName, 35, "equivocation detected")

	// ─── Slashing & authorization (36–38) ───────────────────────────────────
	ErrRoundExpired  = errors.Register(ModuleName, 37, "verification round has expired")
	ErrUnauthorized  = errors.Register(ModuleName, 38, "unauthorized")

	// ─── Contest & challenge (40–47) ────────────────────────────────────────
	ErrInvalidChallenge      = errors.Register(ModuleName, 40, "invalid challenge")
	ErrChallengeWindowClosed = errors.Register(ModuleName, 41, "challenge window has closed")
	ErrSelfChallenge         = errors.Register(ModuleName, 42, "cannot challenge own sample")
	ErrMaxChallengesReached  = errors.Register(ModuleName, 43, "maximum concurrent challenges reached")
	ErrChallengeCooldown     = errors.Register(ModuleName, 44, "challenger is in cooldown period")
	ErrChallengeNotFound     = errors.Register(ModuleName, 45, "challenge not found")
	ErrDuplicateChallenge    = errors.Register(ModuleName, 46, "challenge already exists")
	ErrEvidenceRequired      = errors.Register(ModuleName, 47, "evidence is required for this operation")

	// ─── Research fund (50–51) ──────────────────────────────────────────────
	ErrProposalNotFound = errors.Register(ModuleName, 50, "proposal not found")
	ErrStratumNotFound  = errors.Register(ModuleName, 51, "stratum not found")

	// ─── Domain qualification (60) ──────────────────────────────────────────
	ErrUnqualifiedVerifier = errors.Register(ModuleName, 60, "verifier not qualified for domain")

	// ─── Partnership (70–71) ────────────────────────────────────────────────
	ErrInvalidPartnership = errors.Register(ModuleName, 70, "invalid partnership")
	ErrPartnershipFrozen  = errors.Register(ModuleName, 71, "partnership is frozen")

	// ─── IBC (80) ──────────────────────────────────────────────────────────
	ErrInvalidIBCVersion = errors.Register(ModuleName, 80, "invalid IBC version")

	// ─── Deprecated aliases (keeper migration pending) ──────────────────────
	// TODO(R36-5): remove after keeper migration
	ErrClaimNotFound          = ErrSubmissionNotFound
	ErrFactNotFound           = ErrSampleNotFound
	ErrInvalidClaim           = ErrInvalidSubmission
	ErrClaimTooShort          = ErrContentTooLarge
	ErrInvalidConfidence      = ErrInvalidQualityScore
	ErrDuplicateClaim         = ErrDuplicateContent
	ErrClaimStakeInsufficient = ErrInsufficientStake
	ErrCommitmentMismatch     = ErrRevealMismatch
	ErrVRFSelectionFailed     = ErrInvalidVRFProof
	ErrInvalidCategory        = ErrInvalidDomain
	ErrQueryRateLimited       = errors.Register(ModuleName, 81, "query rate limited")
	ErrRoundNotInCommitPhase  = ErrWrongPhase
	ErrRoundNotInRevealPhase  = ErrWrongPhase

	// Negative knowledge / pruning — retained as aliases for keeper compat
	ErrCounterFactNotFound          = errors.Register(ModuleName, 90, "counter-fact not found")
	ErrCannotContradictSelf         = errors.Register(ModuleName, 91, "cannot contradict own sample")
	ErrContradictionStakeTooLow     = errors.Register(ModuleName, 92, "contradiction stake too low")
	ErrContradictionRateLimited     = errors.Register(ModuleName, 93, "contradiction rate limited")
	ErrGoedelIncompletenessRequired = errors.Register(ModuleName, 94, "Gödelian incompleteness acknowledgment required")
	ErrCannotContradictCounterFact  = errors.Register(ModuleName, 95, "cannot contradict a counter-fact")
	ErrFactAlreadyDisproven         = errors.Register(ModuleName, 96, "fact is already disproven")
	ErrCounterFactAlreadyExists     = errors.Register(ModuleName, 97, "counter-fact already exists for this fact")
	ErrFactNotAtRisk                = errors.Register(ModuleName, 98, "fact is not at-risk")
	ErrFactImmune                   = errors.Register(ModuleName, 99, "fact is immune from pruning")
	ErrInsufficientPatronage        = errors.Register(ModuleName, 100, "insufficient patronage amount")
	ErrFactAlreadyPruned            = errors.Register(ModuleName, 101, "fact is already pruned")
	ErrPatronageExpired             = errors.Register(ModuleName, 102, "patronage has expired")
	ErrPruningDisabled              = errors.Register(ModuleName, 103, "knowledge pruning is disabled")
	ErrFactAlreadyChallenged        = errors.Register(ModuleName, 104, "fact already challenged")
	ErrCannotChallengeFalsified     = errors.Register(ModuleName, 105, "cannot challenge a falsified fact")
	ErrNotProvisional               = errors.Register(ModuleName, 106, "fact is not in provisional state")
	ErrAdversarialDisabled          = errors.Register(ModuleName, 107, "adversarial verification is disabled")
)
