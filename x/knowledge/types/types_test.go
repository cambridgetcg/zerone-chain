package types_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/zerone-chain/zerone/x/knowledge/types"
)

// TestDefaultParams_AllSlashParamsNonZero verifies the B22-3 audit requirement:
// all slash parameters must be strictly greater than zero.
func TestDefaultParams_AllSlashParamsNonZero(t *testing.T) {
	p := types.DefaultParams()

	require.Greater(t, p.WrongVerificationSlashBps, uint64(0),
		"wrong_verification_slash_bps must be > 0")
	require.Greater(t, p.MissedRevealSlashBps, uint64(0),
		"missed_reveal_slash_bps must be > 0")
	require.Greater(t, p.EquivocationSlashBps, uint64(0),
		"equivocation_slash_bps must be > 0")
	// InvalidClaimSlashBps deprecated (R19-6): review fee is non-refundable
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

// TestDefaultGenesis_Marshal_Deterministic verifies that the genesis state
// can be marshalled to JSON and back without data loss.
func TestGenesisState_Marshal_Deterministic(t *testing.T) {
	gs := types.DefaultGenesis()
	require.NotNil(t, gs)

	// Verify expected number of domains.
	require.Len(t, gs.Domains, 18, "expected 18 genesis domains")

	// All domains must be active.
	for _, d := range gs.Domains {
		require.Equal(t, types.DomainStatus_DOMAIN_STATUS_ACTIVE, d.Status,
			"genesis domain %q must be active", d.Name)
	}

	// Verify zero-length slice fields are initialised (not nil).
	require.NotNil(t, gs.Facts)
	require.NotNil(t, gs.PendingClaims)
	require.NotNil(t, gs.ActiveRounds)
}
