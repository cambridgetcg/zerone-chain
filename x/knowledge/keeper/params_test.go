package keeper_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/zerone-chain/zerone/x/knowledge/types"
)

// ─── Params Set/Get ──────────────────────────────────────────────────────────

func TestParams_SetAndGet(t *testing.T) {
	k, ctx := setupKnowledgeTest(t)

	params := types.DefaultParams()
	params.MinVerifiers = 5
	params.MaxVerifiers = 33
	require.NoError(t, k.SetParams(ctx, &params))

	got, err := k.GetParams(ctx)
	require.NoError(t, err)
	require.Equal(t, uint64(5), got.MinVerifiers)
	require.Equal(t, uint64(33), got.MaxVerifiers)
}

func TestParams_GetReturnsDefaultWhenUnset(t *testing.T) {
	// After InitGenesis, params are set. But GetParams defaults when not in store.
	// This verifies the default fallback behavior.
	k, ctx := setupKnowledgeTest(t)

	params, err := k.GetParams(ctx)
	require.NoError(t, err)
	require.NotNil(t, params)
	require.Equal(t, uint64(3), params.MinVerifiers) // default
}

// ─── Params Validation ──────────────────────────────────────────────────────

func TestParamsValidate_Default(t *testing.T) {
	p := types.DefaultParams()
	require.NoError(t, p.Validate())
}

func TestParamsValidate_MinVerifiersZero(t *testing.T) {
	p := types.DefaultParams()
	p.MinVerifiers = 0
	require.Error(t, p.Validate())
}

func TestParamsValidate_MinGreaterThanMax(t *testing.T) {
	p := types.DefaultParams()
	p.MinVerifiers = 30
	p.MaxVerifiers = 10
	require.Error(t, p.Validate())
}

func TestParamsValidate_ConfidenceThresholdTooHigh(t *testing.T) {
	p := types.DefaultParams()
	p.ConfidenceThreshold = 1_000_001
	require.Error(t, p.Validate())
}

func TestParamsValidate_QuorumThresholdTooHigh(t *testing.T) {
	p := types.DefaultParams()
	p.QuorumThreshold = 1_000_001
	require.Error(t, p.Validate())
}

func TestParamsValidate_InitialConfidenceTooHigh(t *testing.T) {
	p := types.DefaultParams()
	p.InitialConfidence = 1_000_001
	require.Error(t, p.Validate())
}

func TestParamsValidate_MinClaimTextLengthZero(t *testing.T) {
	p := types.DefaultParams()
	p.MinClaimTextLength = 0
	require.Error(t, p.Validate())
}

func TestParamsValidate_MaxClaimTextLengthBelowMin(t *testing.T) {
	p := types.DefaultParams()
	p.MinClaimTextLength = 100
	p.MaxClaimTextLength = 50
	require.Error(t, p.Validate())
}

func TestParamsValidate_CrossStratumDiscountTooHigh(t *testing.T) {
	p := types.DefaultParams()
	p.CrossStratumDiscountBps = 1_000_001
	require.Error(t, p.Validate())
}

// ─── All Slash Params Validation (Security) ──────────────────────────────────

func TestParamsValidate_AllSlashCombinations(t *testing.T) {
	slashFields := []struct {
		name  string
		setup func(p *types.Params)
	}{
		{"WrongVerificationSlashBps=0", func(p *types.Params) { p.WrongVerificationSlashBps = 0 }},
		{"MissedRevealSlashBps=0", func(p *types.Params) { p.MissedRevealSlashBps = 0 }},
		{"EquivocationSlashBps=0", func(p *types.Params) { p.EquivocationSlashBps = 0 }},
		{"InvalidClaimSlashBps=0", func(p *types.Params) { p.InvalidClaimSlashBps = 0 }},
	}

	for _, sf := range slashFields {
		t.Run(sf.name, func(t *testing.T) {
			p := types.DefaultParams()
			sf.setup(&p)
			err := p.Validate()
			require.Error(t, err, "setting %s should make params invalid", sf.name)
		})
	}
}

// ─── Default Param Values ────────────────────────────────────────────────────

func TestDefaultParams_CoreVerification(t *testing.T) {
	p := types.DefaultParams()

	require.Equal(t, uint64(3), p.MinVerifiers)
	require.Equal(t, uint64(22), p.MaxVerifiers)
	require.Equal(t, uint64(4), p.CommitPhaseBlocks)
	require.Equal(t, uint64(4), p.RevealPhaseBlocks)
	require.Equal(t, uint64(3), p.AggregationPhaseBlocks)
	require.Equal(t, uint64(50), p.ClaimCooldownBlocks)
}

func TestDefaultParams_ConfidenceScoring(t *testing.T) {
	p := types.DefaultParams()

	require.Equal(t, uint64(500_000), p.InitialConfidence)
	require.Equal(t, uint64(50_000), p.ConfidenceBoostPerVerification)
	require.Equal(t, uint64(770_000), p.ConfidenceThreshold)
	require.Equal(t, uint64(660_000), p.QuorumThreshold)
}

func TestDefaultParams_Rewards(t *testing.T) {
	p := types.DefaultParams()

	require.Equal(t, "3000000", p.VerificationReward)
	require.Equal(t, uint64(999_000), p.VerificationRewardDecayBps)
}

func TestDefaultParams_ClaimValidation(t *testing.T) {
	p := types.DefaultParams()

	require.Equal(t, uint64(20), p.MinClaimTextLength)
	require.Equal(t, uint64(10_000), p.MaxClaimTextLength)
	require.Equal(t, "1000000", p.MinClaimStake)
}

func TestDefaultParams_AdversarialVerification(t *testing.T) {
	p := types.DefaultParams()

	require.True(t, p.AdversarialVerificationEnabled)
	require.Equal(t, uint64(500_000), p.ProvisionalThreshold)
	require.Equal(t, uint64(300_000), p.RejectThreshold)
	require.Equal(t, "11000000", p.MinChallengeStake)
	require.Equal(t, uint64(3), p.MaxConcurrentChallenges)
}

func TestDefaultParams_ValidBoundary(t *testing.T) {
	// Test that boundary values (exact max) pass validation
	p := types.DefaultParams()
	p.ConfidenceThreshold = 1_000_000
	p.QuorumThreshold = 1_000_000
	p.InitialConfidence = 1_000_000
	require.NoError(t, p.Validate())
}

func TestDefaultParams_MinEqualsMax(t *testing.T) {
	p := types.DefaultParams()
	p.MinVerifiers = 5
	p.MaxVerifiers = 5
	require.NoError(t, p.Validate(), "min == max should be valid")
}
