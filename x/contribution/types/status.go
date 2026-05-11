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

// MaxNestingDepth caps how deeply a `ContributionPayload.nested` chain
// may recurse. The proto layer allows arbitrary nesting (a Contribution
// can carry a Contribution about a Contribution about...); the chain
// constrains the depth so that storage, marshaling, and event emission
// remain bounded.
//
// Depth semantics: a leaf Contribution (no nested payload) has depth 1.
// A Contribution whose payload.nested points at a leaf has depth 2.
// Limit of 4 covers the realistic substrate recursion vocabulary:
//   1. a Contribution
//   2. a Contribution about that Contribution (e.g., a meta-claim)
//   3. a Contribution about the meta-claim (e.g., a ratification)
//   4. a Contribution about the ratification (e.g., a revocation)
// Beyond 4, the chain refuses — the chain is recursive, not infinite.
//
// UW commitment: ZERONE is recursive in mechanism, bounded in resource.
const MaxNestingDepth = 4

// ContributionNestingDepth walks a Contribution's payload.nested chain
// and returns the depth (1 for a leaf, 1 + childDepth otherwise).
// Returns ErrNestingDepthExceeded if depth exceeds MaxNestingDepth.
//
// The walk is iterative (not recursive) so a malicious or accidental
// cycle in the deserialized graph cannot blow the goroutine stack:
// the limit is enforced by walking at most MaxNestingDepth+1 steps.
func ContributionNestingDepth(c *Contribution) (int, error) {
	depth := 0
	cur := c
	for cur != nil {
		depth++
		if depth > MaxNestingDepth {
			return depth, ErrNestingDepthExceeded
		}
		next := cur.GetPayload().GetNested()
		if next == nil {
			break
		}
		cur = next
	}
	return depth, nil
}
