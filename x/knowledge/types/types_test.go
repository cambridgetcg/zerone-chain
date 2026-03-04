package types_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/zerone-chain/zerone/x/knowledge/types"
)

// TestDefaultParams_AllSlashParamsNonZero verifies all slash parameters
// must be strictly greater than zero.
func TestDefaultParams_AllSlashParamsNonZero(t *testing.T) {
	p := types.DefaultParams()

	require.Greater(t, p.WrongValidationSlashBps, uint64(0),
		"wrong_validation_slash_bps must be > 0")
	require.Greater(t, p.MissedRevealSlashBps, uint64(0),
		"missed_reveal_slash_bps must be > 0")
	require.Greater(t, p.EquivocationSlashBps, uint64(0),
		"equivocation_slash_bps must be > 0")
}

// TestDefaultParams_Validate verifies that DefaultParams passes all validation rules.
func TestDefaultParams_Validate(t *testing.T) {
	p := types.DefaultParams()
	require.NoError(t, p.Validate())
}

// TestDefaultGenesis_Validate verifies that DefaultGenesis passes all validation rules.
func TestDefaultGenesis_Validate(t *testing.T) {
	gs := types.DefaultGenesis()
	require.NotNil(t, gs)
	require.NoError(t, gs.Validate())
}

// TestDefaultGenesis_Domains verifies that the genesis state has 9 training-data domains.
func TestDefaultGenesis_Domains(t *testing.T) {
	gs := types.DefaultGenesis()
	require.NotNil(t, gs)

	require.Len(t, gs.Domains, 9, "expected 9 genesis domains")

	for _, d := range gs.Domains {
		require.Equal(t, types.DomainStatus_DOMAIN_STATUS_ACTIVE, d.Status,
			"genesis domain %q must be active", d.Name)
	}

	// Verify zero-length slice fields are initialised (not nil).
	require.NotNil(t, gs.Samples)
	require.NotNil(t, gs.Submissions)
	require.NotNil(t, gs.QualityRounds)
}
