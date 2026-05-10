package types_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/zerone-chain/zerone/x/creed/types"
)

func TestCanonicalLifecyclePhases_NineAndOrdered(t *testing.T) {
	require.Len(t, types.CanonicalLifecyclePhases, 9,
		"Phase 0 ships exactly 9 lifecycle phases; new phases require doctrine amendment")
	for i, p := range types.CanonicalLifecyclePhases {
		require.Equal(t, types.LifecyclePhase(i), p.Number,
			"phase numbering must be dense and monotonic; index %d must hold phase %d", i, i)
		require.NotEmpty(t, p.Name, "phase name must be non-empty")
	}
}

func TestCanonicalLifecyclePhases_KnowledgeDelegates(t *testing.T) {
	for _, p := range types.CanonicalLifecyclePhases {
		if p.Number == types.LifecyclePhaseKnowledge {
			require.False(t, p.HasSubCreedDoc,
				"Knowledge phase must delegate sub-creed to truth-seeking creed")
			require.Equal(t, "knowledge", p.Name)
			return
		}
	}
	t.Fatal("Knowledge phase not found in CanonicalLifecyclePhases")
}

func TestCanonicalLifecyclePhases_OthersHaveSubCreedDocs(t *testing.T) {
	for _, p := range types.CanonicalLifecyclePhases {
		if p.Number == types.LifecyclePhaseKnowledge {
			continue
		}
		require.True(t, p.HasSubCreedDoc,
			"phase %s (%d) must have a sub-creed doc", p.Name, p.Number)
	}
}

func TestCanonicalLifecyclePhases_NamesMatchExpected(t *testing.T) {
	expected := []string{
		"foundation", "knowledge", "curation", "augmentation",
		"training", "evaluation", "alignment", "substrate", "tools",
	}
	require.Len(t, types.CanonicalLifecyclePhases, len(expected))
	for i, want := range expected {
		require.Equal(t, want, types.CanonicalLifecyclePhases[i].Name,
			"phase index %d name mismatch", i)
	}
}
