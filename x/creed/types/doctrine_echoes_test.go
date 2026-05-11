package types_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/zerone-chain/zerone/x/creed/types"
	knowledgetypes "github.com/zerone-chain/zerone/x/knowledge/types"
)

func TestCanonicalDoctrineEchoes_NonEmpty(t *testing.T) {
	require.NotEmpty(t, types.CanonicalDoctrineEchoes,
		"echoes list must include cross-doctrine relations")
}

func TestCanonicalDoctrineEchoes_NoDuplicateEdges(t *testing.T) {
	seen := map[string]bool{}
	for _, e := range types.CanonicalDoctrineEchoes {
		key := e.From + "→" + e.To + "|" + e.Relation.String()
		require.False(t, seen[key], "duplicate echo edge: %s", key)
		seen[key] = true
	}
}

func TestCanonicalDoctrineEchoes_NoSelfReferences(t *testing.T) {
	for _, e := range types.CanonicalDoctrineEchoes {
		require.NotEqual(t, e.From, e.To,
			"echo edges must not be self-references: %s", e.From)
	}
}

func TestCanonicalDoctrineEchoes_AllRelationsSpecified(t *testing.T) {
	for _, e := range types.CanonicalDoctrineEchoes {
		require.NotEmpty(t, e.From, "From must not be empty")
		require.NotEmpty(t, e.To, "To must not be empty")
		require.NotEqual(t, knowledgetypes.RelationType_RELATION_TYPE_UNSPECIFIED, e.Relation,
			"edge %s→%s must specify a relation type", e.From, e.To)
	}
}
