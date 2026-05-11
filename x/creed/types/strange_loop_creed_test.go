package types_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/zerone-chain/zerone/x/creed/types"
)

func TestStrangeLoopCommitment_IsIndivisible(t *testing.T) {
	require.Equal(t, "SL", types.StrangeLoopCommitment,
		"SL commitment identifier must not change once shipped")
	require.Equal(t, "ZERONE is a strange loop", types.StrangeLoopStatement,
		"SL statement is doctrinally fixed")
}

func TestCanonicalStrangeLoopMechanisms_Count(t *testing.T) {
	require.Len(t, types.CanonicalStrangeLoopMechanisms, 6,
		"Phase SL-α ships SL-M1 through SL-M6")
}

func TestCanonicalStrangeLoopMechanisms_NumberingDense(t *testing.T) {
	for i, m := range types.CanonicalStrangeLoopMechanisms {
		require.Equal(t, uint32(i+1), m.Number,
			"mechanism numbering must be dense and monotonic; index %d must hold SL-M%d", i, i+1)
	}
}

func TestCanonicalStrangeLoopMechanisms_NamesNonEmpty(t *testing.T) {
	for _, m := range types.CanonicalStrangeLoopMechanisms {
		require.NotEmpty(t, m.Name, "mechanism SL-M%d must have a non-empty name", m.Number)
	}
}

func TestStrangeLoopDomain_Stable(t *testing.T) {
	require.Equal(t, "doctrine_strange_loop", types.StrangeLoopDomain,
		"domain name is doctrinally fixed")
}
