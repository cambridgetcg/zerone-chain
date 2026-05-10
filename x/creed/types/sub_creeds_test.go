package types_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/zerone-chain/zerone/x/creed/types"
)

func TestCanonicalSubCreeds_Count(t *testing.T) {
	require.Len(t, types.CanonicalSubCreeds, 9,
		"one entry per lifecycle phase including Knowledge (which has nil commitments)")
}

func TestCanonicalSubCreeds_KnowledgeIsEmpty(t *testing.T) {
	sc, ok := types.SubCreedFor(types.LifecyclePhaseKnowledge)
	require.True(t, ok)
	require.Nil(t, sc.Commitments,
		"Knowledge phase delegates to CanonicalCommitments; sub-creed must be nil")
}

func TestCanonicalSubCreeds_NonKnowledgePhasesHaveThreeAtInception(t *testing.T) {
	for _, sc := range types.CanonicalSubCreeds {
		if sc.Phase == types.LifecyclePhaseKnowledge {
			continue
		}
		require.Len(t, sc.Commitments, 3,
			"phase %d ships 3 seed commitments at inception", sc.Phase)
	}
}

func TestCanonicalSubCreeds_NumberingDenseAndMonotonic(t *testing.T) {
	for _, sc := range types.CanonicalSubCreeds {
		for i, c := range sc.Commitments {
			require.Equal(t, uint32(i+1), c.Number,
				"phase %d commitment index %d must hold number %d", sc.Phase, i, i+1)
			require.NotEmpty(t, c.Code, "commitment code must be non-empty")
			require.NotEmpty(t, c.Name, "commitment name must be non-empty")
		}
	}
}

func TestSubCreedFor_KnownAndUnknown(t *testing.T) {
	_, ok := types.SubCreedFor(types.LifecyclePhaseFoundation)
	require.True(t, ok)
	_, ok = types.SubCreedFor(types.LifecyclePhase(99))
	require.False(t, ok)
}

func TestCanonicalSubCreeds_CodesUnique(t *testing.T) {
	seen := map[string]bool{}
	for _, sc := range types.CanonicalSubCreeds {
		for _, c := range sc.Commitments {
			require.False(t, seen[c.Code],
				"duplicate commitment code %q", c.Code)
			seen[c.Code] = true
		}
	}
	// Sanity: at inception, 8 phases × 3 = 24 codes (Knowledge contributes none)
	require.Len(t, seen, 24)
}
