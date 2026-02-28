package keeper_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/zerone-chain/zerone/x/knowledge/keeper"
	"github.com/zerone-chain/zerone/x/knowledge/types"
)

// ─── Mock ZeroneAuthKeeper ──────────────────────────────────────────────────

type mockZeroneAuthKeeper struct {
	accounts map[string]string
}

func newMockZeroneAuthKeeper() *mockZeroneAuthKeeper {
	return &mockZeroneAuthKeeper{accounts: make(map[string]string)}
}

func (m *mockZeroneAuthKeeper) GetAccountType(_ context.Context, address string) (string, bool) {
	t, ok := m.accounts[address]
	return t, ok
}

// ─── Claim Confidence Bonus Tests ──────────────────────────────────────────

func TestHumanEmpiricalClaimBonus(t *testing.T) {
	k, ctx := setupKnowledgeTest(t)
	params, _ := k.GetParams(ctx)

	boosted := keeper.ApplyRoleBonusToConfidence(700_000, types.ClaimType_CLAIM_TYPE_OBSERVATION, "human", params)
	// 700,000 * (1,000,000 + 150,000) / 1,000,000 = 805,000
	require.Equal(t, uint64(805_000), boosted)

	_ = k
}

func TestAgentComputationalClaimBonus(t *testing.T) {
	k, ctx := setupKnowledgeTest(t)
	params, _ := k.GetParams(ctx)

	boosted := keeper.ApplyRoleBonusToConfidence(700_000, types.ClaimType_CLAIM_TYPE_COMPUTATIONAL, "agent", params)
	require.Equal(t, uint64(805_000), boosted)

	_ = k
}

func TestNoBonus_HumanComputational(t *testing.T) {
	k, ctx := setupKnowledgeTest(t)
	params, _ := k.GetParams(ctx)

	boosted := keeper.ApplyRoleBonusToConfidence(700_000, types.ClaimType_CLAIM_TYPE_COMPUTATIONAL, "human", params)
	require.Equal(t, uint64(700_000), boosted)

	_ = k
}

func TestNoBonus_AgentObservation(t *testing.T) {
	k, ctx := setupKnowledgeTest(t)
	params, _ := k.GetParams(ctx)

	boosted := keeper.ApplyRoleBonusToConfidence(700_000, types.ClaimType_CLAIM_TYPE_OBSERVATION, "agent", params)
	require.Equal(t, uint64(700_000), boosted)

	_ = k
}

func TestNoBonus_UnknownAccount(t *testing.T) {
	k, ctx := setupKnowledgeTest(t)
	params, _ := k.GetParams(ctx)

	boosted := keeper.ApplyRoleBonusToConfidence(700_000, types.ClaimType_CLAIM_TYPE_OBSERVATION, "", params)
	require.Equal(t, uint64(700_000), boosted)

	_ = k
}

func TestDualValidationBonus(t *testing.T) {
	k, ctx := setupKnowledgeTest(t)
	params, _ := k.GetParams(ctx)

	// Partnership claim: base 700,000 + 25% = 875,000
	boosted := keeper.ApplyDualValidationBonus(700_000, params)
	require.Equal(t, uint64(875_000), boosted)

	_ = k
}

func TestRoleBonusPlusDualValidation(t *testing.T) {
	k, ctx := setupKnowledgeTest(t)
	params, _ := k.GetParams(ctx)

	// Human empirical + partnership: 700,000 * 1.15 = 805,000 → * 1.25 = 1,006,250
	boosted := keeper.ApplyRoleBonusToConfidence(700_000, types.ClaimType_CLAIM_TYPE_OBSERVATION, "human", params)
	require.Equal(t, uint64(805_000), boosted)

	final := keeper.ApplyDualValidationBonus(boosted, params)
	require.Equal(t, uint64(1_006_250), final)

	_ = k
}

func TestRoleBonusDisabledWhenParamZero(t *testing.T) {
	k, ctx := setupKnowledgeTest(t)
	params, _ := k.GetParams(ctx)
	params.HumanEmpiricalBonusBps = 0
	require.NoError(t, k.SetParams(ctx, params))

	newParams, _ := k.GetParams(ctx)
	boosted := keeper.ApplyRoleBonusToConfidence(700_000, types.ClaimType_CLAIM_TYPE_OBSERVATION, "human", newParams)
	require.Equal(t, uint64(700_000), boosted, "bonus disabled when param is 0")
}
