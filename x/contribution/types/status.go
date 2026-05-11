package types

// ValidStatusTransitions defines the allowed status transitions.
// Any transition not in this map is rejected with
// ErrInvalidStatusTransition. Forward-only per truth-seeking
// commitment 10 — no transition moves a status backwards.
var ValidStatusTransitions = map[ContributionStatus]map[ContributionStatus]bool{
	ContributionStatus_STATUS_SUBMITTED: {
		ContributionStatus_STATUS_CLASSIFIED:            true,
		ContributionStatus_STATUS_CLASSIFICATION_FAILED: true,
	},
	ContributionStatus_STATUS_CLASSIFIED: {
		ContributionStatus_STATUS_VERIFIED:            true,
		ContributionStatus_STATUS_VERIFICATION_FAILED: true,
	},
	ContributionStatus_STATUS_VERIFIED: {
		ContributionStatus_STATUS_ADMITTED:         true,
		ContributionStatus_STATUS_ADMISSION_FAILED: true,
	},
	ContributionStatus_STATUS_ADMITTED: {
		ContributionStatus_STATUS_REVOKED: true,
	},
	// Terminal states (no further transitions): REVOKED, *_FAILED.
}

// CanTransition reports whether a status transition from `from` to `to`
// is permitted by the forward-only audit invariant.
func CanTransition(from, to ContributionStatus) bool {
	allowed, ok := ValidStatusTransitions[from]
	if !ok {
		return false
	}
	return allowed[to]
}

// IsTerminal reports whether a status is terminal (no further transitions).
func IsTerminal(s ContributionStatus) bool {
	switch s {
	case ContributionStatus_STATUS_REVOKED,
		ContributionStatus_STATUS_CLASSIFICATION_FAILED,
		ContributionStatus_STATUS_VERIFICATION_FAILED,
		ContributionStatus_STATUS_ADMISSION_FAILED:
		return true
	}
	return false
}

// MinVerificationScoreBps is the minimum verification_score (BPS) for
// a Contribution to transition from CLASSIFIED to VERIFIED. Below this
// threshold, the adapter's Verify result transitions the Contribution
// to VERIFICATION_FAILED. Phase 1 default: 500_000 (50%).
// Governance-tunable via params in Phase 6+; constant at Phase 1.
const MinVerificationScoreBps uint32 = 500_000
