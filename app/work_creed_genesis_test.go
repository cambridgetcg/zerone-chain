package app_test

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	zeroneapp "github.com/zerone-chain/zerone/app"
	creedtypes "github.com/zerone-chain/zerone/x/creed/types"
	workcreedtypes "github.com/zerone-chain/zerone/x/work_creed/types"
)

// TestLoadInceptionSubCreedPins_FromRepoRoot verifies the helper
// produces a pin for every non-Knowledge phase using the actual
// .sub-creed-hashes shipping with the repo. The resulting genesis
// state must pass the work_creed validator.
func TestLoadInceptionSubCreedPins_FromRepoRoot(t *testing.T) {
	hashPath := filepath.Join("..", ".sub-creed-hashes")

	pins, err := zeroneapp.LoadInceptionSubCreedPins(hashPath)
	require.NoError(t, err)

	// 9 phases minus Knowledge (which delegates to x/creed) = 8 pins.
	require.Len(t, pins, 8, "must have one inception pin per non-Knowledge phase")

	// Each pin's structural invariants.
	for _, p := range pins {
		require.Equal(t, uint32(1), p.Version, "inception pins are always version 1")
		require.Equal(t, uint64(0), p.AnchoredAtBlock, "genesis = block 0")
		require.Empty(t, p.SourceLip, "genesis pins have no LIP")
		require.NotEqual(t, uint32(creedtypes.LifecyclePhaseKnowledge), p.Phase,
			"Knowledge must not be pinned in x/work_creed")
		require.Len(t, p.CanonicalHash, 32, "sha256 = 32 bytes")
		require.NotEmpty(t, p.PhaseName)
		require.NotEmpty(t, p.CommitmentCodes,
			"every pinned phase ships with at least one commitment code")
	}

	// The genesis state composed from these pins must validate.
	gs := &workcreedtypes.GenesisState{PinnedSubCreeds: pins}
	require.NoError(t, gs.Validate(),
		"inception pins must satisfy the work_creed genesis validator")
}

// TestLoadInceptionSubCreedPins_MissingFile reports a clear error.
func TestLoadInceptionSubCreedPins_MissingFile(t *testing.T) {
	_, err := zeroneapp.LoadInceptionSubCreedPins("/nonexistent/.sub-creed-hashes")
	require.Error(t, err)
}
