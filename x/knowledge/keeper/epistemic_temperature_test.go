package keeper

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/zerone-chain/zerone/x/knowledge/types"
)

func TestEpistemicState_KeyConstruction(t *testing.T) {
	key := types.EpistemicStateKey("mathematics")
	require.Equal(t, byte(0x53), key[0])
	require.Contains(t, string(key[1:]), "mathematics")
}
