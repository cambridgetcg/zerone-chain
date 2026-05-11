package types_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/zerone-chain/zerone/x/contribution/types"
)

func TestCanTransition_ForwardOnly(t *testing.T) {
	// SUBMITTED → CLASSIFIED is allowed.
	require.True(t, types.CanTransition(
		types.ContributionStatus_STATUS_SUBMITTED,
		types.ContributionStatus_STATUS_CLASSIFIED,
	))
	// CLASSIFIED → SUBMITTED is NOT allowed.
	require.False(t, types.CanTransition(
		types.ContributionStatus_STATUS_CLASSIFIED,
		types.ContributionStatus_STATUS_SUBMITTED,
	))
	// VERIFIED → CLASSIFIED is NOT allowed.
	require.False(t, types.CanTransition(
		types.ContributionStatus_STATUS_VERIFIED,
		types.ContributionStatus_STATUS_CLASSIFIED,
	))
}

func TestCanTransition_TerminalStates(t *testing.T) {
	terminals := []types.ContributionStatus{
		types.ContributionStatus_STATUS_REVOKED,
		types.ContributionStatus_STATUS_CLASSIFICATION_FAILED,
		types.ContributionStatus_STATUS_VERIFICATION_FAILED,
		types.ContributionStatus_STATUS_ADMISSION_FAILED,
	}
	for _, term := range terminals {
		// No transition out of any terminal state.
		for s := types.ContributionStatus_STATUS_UNSPECIFIED; s <= types.ContributionStatus_STATUS_ADMISSION_FAILED; s++ {
			require.False(t, types.CanTransition(term, s),
				"terminal %v should not transition to %v", term, s)
		}
	}
}

func TestIsTerminal_ClassifiesCorrectly(t *testing.T) {
	require.True(t, types.IsTerminal(types.ContributionStatus_STATUS_REVOKED))
	require.True(t, types.IsTerminal(types.ContributionStatus_STATUS_CLASSIFICATION_FAILED))
	require.False(t, types.IsTerminal(types.ContributionStatus_STATUS_SUBMITTED))
	require.False(t, types.IsTerminal(types.ContributionStatus_STATUS_VERIFIED))
}

func TestStatusTransitions_HappyPathChain(t *testing.T) {
	// Walk SUBMITTED → CLASSIFIED → VERIFIED → ADMITTED → REVOKED.
	chain := []types.ContributionStatus{
		types.ContributionStatus_STATUS_SUBMITTED,
		types.ContributionStatus_STATUS_CLASSIFIED,
		types.ContributionStatus_STATUS_VERIFIED,
		types.ContributionStatus_STATUS_ADMITTED,
		types.ContributionStatus_STATUS_REVOKED,
	}
	for i := 0; i < len(chain)-1; i++ {
		require.True(t, types.CanTransition(chain[i], chain[i+1]),
			"happy path step %d (%v → %v) must be allowed", i, chain[i], chain[i+1])
	}
}
