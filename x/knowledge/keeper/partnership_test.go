package keeper_test

import (
	"context"
	"fmt"
	"testing"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/stretchr/testify/require"

	"github.com/zerone-chain/zerone/x/knowledge/keeper"
	"github.com/zerone-chain/zerone/x/knowledge/types"
)

// ─── Mock PartnershipKeeper ──────────────────────────────────────────────────

type mockPartnershipKeeper struct {
	partnerships map[string]*mockPartnership
	distributed  []distributeRecord
}

type mockPartnership struct {
	humanAddr string
	agentAddr string
	active    bool
	suspended bool
}

type distributeRecord struct {
	PartnershipID string
	Amount        sdk.Coins
	Source        string
}

func newMockPartnershipKeeper() *mockPartnershipKeeper {
	return &mockPartnershipKeeper{
		partnerships: make(map[string]*mockPartnership),
	}
}

func (pk *mockPartnershipKeeper) addPartnership(id, human, agent string, active, suspended bool) {
	pk.partnerships[id] = &mockPartnership{
		humanAddr: human,
		agentAddr: agent,
		active:    active,
		suspended: suspended,
	}
}

func (pk *mockPartnershipKeeper) IsActive(_ context.Context, partnershipId string) (bool, error) {
	p, ok := pk.partnerships[partnershipId]
	if !ok {
		return false, fmt.Errorf("partnership %s not found", partnershipId)
	}
	return p.active, nil
}

func (pk *mockPartnershipKeeper) IsParticipant(_ context.Context, partnershipId string, address string) (bool, error) {
	p, ok := pk.partnerships[partnershipId]
	if !ok {
		return false, fmt.Errorf("partnership %s not found", partnershipId)
	}
	return p.humanAddr == address || p.agentAddr == address, nil
}

func (pk *mockPartnershipKeeper) IsSuspended(_ context.Context, partnershipId string) (bool, error) {
	p, ok := pk.partnerships[partnershipId]
	if !ok {
		return false, fmt.Errorf("partnership %s not found", partnershipId)
	}
	return p.suspended, nil
}

func (pk *mockPartnershipKeeper) GetDomainPartnershipDensity(_ context.Context, _ string) uint64 {
	// Count unique participants across all active partnerships
	unique := make(map[string]bool)
	for _, p := range pk.partnerships {
		if p.active {
			unique[p.humanAddr] = true
			unique[p.agentAddr] = true
		}
	}
	return uint64(len(unique))
}

func (pk *mockPartnershipKeeper) DistributeReward(_ context.Context, partnershipId string, amount sdk.Coins, source string) error {
	p, ok := pk.partnerships[partnershipId]
	if !ok {
		return fmt.Errorf("partnership %s not found", partnershipId)
	}
	if !p.active {
		return fmt.Errorf("partnership %s is not active", partnershipId)
	}
	pk.distributed = append(pk.distributed, distributeRecord{
		PartnershipID: partnershipId,
		Amount:        amount,
		Source:        source,
	})
	return nil
}

// ─── Partnership Validation Tests (SubmitClaim) ─────────────────────────────

func TestSubmitClaim_ValidPartnership(t *testing.T) {
	k, ctx, bk := setupKnowledgeTestWithBank(t)

	human := makeValidBech32Addr("human1")
	agent := makeValidBech32Addr("agent1")
	partnershipID := "partnership-001"

	pk := newMockPartnershipKeeper()
	pk.addPartnership(partnershipID, human, agent, true, false)
	k.SetPartnershipKeeper(pk)

	// Give submitter enough balance
	bk.balances[human] = sdk.NewCoins(sdk.NewInt64Coin("uzrn", 100_000_000_000))

	// Create a domain for the claim
	require.NoError(t, k.SetDomain(ctx, &types.Domain{
		Name:   "general",
		Status: types.DomainStatus_DOMAIN_STATUS_ACTIVE,
	}))

	msg := &types.MsgSubmitClaim{
		Submitter:     human,
		FactContent:   "partnered knowledge claim about biology",
		Domain:        "general",
		Category:      "computational",
		Stake:         "1000000",
		PartnershipId: partnershipID,
	}

	srv := keeper.NewMsgServerImpl(k)
	resp, err := srv.SubmitClaim(ctx, msg)
	require.NoError(t, err)
	require.NotEmpty(t, resp.ClaimId)

	// Verify claim stored with partnership_id
	claim, found := k.GetClaim(ctx, resp.ClaimId)
	require.True(t, found)
	require.Equal(t, partnershipID, claim.PartnershipId)
}

func TestSubmitClaim_NonExistentPartnership(t *testing.T) {
	k, ctx, bk := setupKnowledgeTestWithBank(t)

	human := makeValidBech32Addr("human1")
	pk := newMockPartnershipKeeper()
	k.SetPartnershipKeeper(pk)

	bk.balances[human] = sdk.NewCoins(sdk.NewInt64Coin("uzrn", 100_000_000_000))

	msg := &types.MsgSubmitClaim{
		Submitter:     human,
		FactContent:   "claim with fake partnership",
		Domain:        "",
		Category:      "computational",
		Stake:         "1000000",
		PartnershipId: "nonexistent-id",
	}

	srv := keeper.NewMsgServerImpl(k)
	_, err := srv.SubmitClaim(ctx, msg)
	require.Error(t, err)
	require.ErrorContains(t, err, "not active")
}

func TestSubmitClaim_NonParticipant(t *testing.T) {
	k, ctx, bk := setupKnowledgeTestWithBank(t)

	human := makeValidBech32Addr("human1")
	agent := makeValidBech32Addr("agent1")
	outsider := makeValidBech32Addr("outsider1")
	partnershipID := "partnership-001"

	pk := newMockPartnershipKeeper()
	pk.addPartnership(partnershipID, human, agent, true, false)
	k.SetPartnershipKeeper(pk)

	bk.balances[outsider] = sdk.NewCoins(sdk.NewInt64Coin("uzrn", 100_000_000_000))

	msg := &types.MsgSubmitClaim{
		Submitter:     outsider,
		FactContent:   "claim from non-participant",
		Domain:        "",
		Category:      "computational",
		Stake:         "1000000",
		PartnershipId: partnershipID,
	}

	srv := keeper.NewMsgServerImpl(k)
	_, err := srv.SubmitClaim(ctx, msg)
	require.Error(t, err)
	require.ErrorContains(t, err, "not a participant")
}

func TestSubmitClaim_FrozenPartnership(t *testing.T) {
	k, ctx, bk := setupKnowledgeTestWithBank(t)

	human := makeValidBech32Addr("human1")
	agent := makeValidBech32Addr("agent1")
	partnershipID := "partnership-frozen"

	pk := newMockPartnershipKeeper()
	// Active but suspended (coercion freeze)
	pk.addPartnership(partnershipID, human, agent, true, true)
	k.SetPartnershipKeeper(pk)

	bk.balances[human] = sdk.NewCoins(sdk.NewInt64Coin("uzrn", 100_000_000_000))

	msg := &types.MsgSubmitClaim{
		Submitter:     human,
		FactContent:   "claim through frozen partnership",
		Domain:        "",
		Category:      "computational",
		Stake:         "1000000",
		PartnershipId: partnershipID,
	}

	srv := keeper.NewMsgServerImpl(k)
	_, err := srv.SubmitClaim(ctx, msg)
	require.Error(t, err)
	require.ErrorContains(t, err, "frozen")
}

func TestSubmitClaim_EmptyPartnershipId_Unchanged(t *testing.T) {
	k, ctx, bk := setupKnowledgeTestWithBank(t)

	submitter := makeValidBech32Addr("submitter1")
	bk.balances[submitter] = sdk.NewCoins(sdk.NewInt64Coin("uzrn", 100_000_000_000))

	// No partnership keeper set — should still work for solo claims
	msg := &types.MsgSubmitClaim{
		Submitter:     submitter,
		FactContent:   "solo claim without partnership",
		Domain:        "",
		Category:      "computational",
		Stake:         "1000000",
		PartnershipId: "", // empty — solo claim
	}

	srv := keeper.NewMsgServerImpl(k)
	resp, err := srv.SubmitClaim(ctx, msg)
	require.NoError(t, err)
	require.NotEmpty(t, resp.ClaimId)
}

func TestSubmitClaim_PartnershipKeeperNil_BackwardCompat(t *testing.T) {
	k, ctx, bk := setupKnowledgeTestWithBank(t)

	submitter := makeValidBech32Addr("submitter1")
	bk.balances[submitter] = sdk.NewCoins(sdk.NewInt64Coin("uzrn", 100_000_000_000))

	// Partnership keeper NOT set — partnership_id with no keeper should fail gracefully
	msg := &types.MsgSubmitClaim{
		Submitter:     submitter,
		FactContent:   "claim with partnership but no keeper",
		Domain:        "",
		Category:      "computational",
		Stake:         "1000000",
		PartnershipId: "some-partnership",
	}

	srv := keeper.NewMsgServerImpl(k)
	_, err := srv.SubmitClaim(ctx, msg)
	require.Error(t, err)
	require.ErrorContains(t, err, "partnership module not available")
}

func TestSubmitClaim_InactivePartnership(t *testing.T) {
	k, ctx, bk := setupKnowledgeTestWithBank(t)

	human := makeValidBech32Addr("human1")
	agent := makeValidBech32Addr("agent1")
	partnershipID := "partnership-dissolved"

	pk := newMockPartnershipKeeper()
	pk.addPartnership(partnershipID, human, agent, false, false)
	k.SetPartnershipKeeper(pk)

	bk.balances[human] = sdk.NewCoins(sdk.NewInt64Coin("uzrn", 100_000_000_000))

	msg := &types.MsgSubmitClaim{
		Submitter:     human,
		FactContent:   "claim through dissolved partnership",
		Domain:        "",
		Category:      "computational",
		Stake:         "1000000",
		PartnershipId: partnershipID,
	}

	srv := keeper.NewMsgServerImpl(k)
	_, err := srv.SubmitClaim(ctx, msg)
	require.Error(t, err)
	require.ErrorContains(t, err, "not active")
}

// ─── Reward Routing Tests ───────────────────────────────────────────────────

func TestRewardRouting_ThroughPartnership(t *testing.T) {
	k, ctx := setupKnowledgeTest(t)

	human := makeValidBech32Addr("human1")
	agent := makeValidBech32Addr("agent1")
	partnershipID := "partnership-001"

	pk := newMockPartnershipKeeper()
	pk.addPartnership(partnershipID, human, agent, true, false)
	k.SetPartnershipKeeper(pk)

	// Create claim with partnership_id
	claim, round := makeTestClaim(t, k, ctx, human,
		"partnered claim content", "general", "computational", "1000000")
	claim.PartnershipId = partnershipID
	require.NoError(t, k.SetClaim(ctx, claim))

	// Complete the round with ACCEPT verdict
	result := &keeper.VerificationResult{
		Verdict:    types.Verdict_VERDICT_ACCEPT,
		Confidence: 800000,
		Rewards: []keeper.VerifierReward{
			{Verifier: makeValidBech32Addr("verifier1"), Amount: 500000},
		},
	}

	err := k.CompleteRound(ctx, round, result)
	require.NoError(t, err)

	// Verify reward was routed through partnership
	require.Len(t, pk.distributed, 1)
	require.Equal(t, partnershipID, pk.distributed[0].PartnershipID)
	require.Equal(t, "knowledge_verification", pk.distributed[0].Source)
	require.Equal(t, "1000000", pk.distributed[0].Amount.AmountOf("uzrn").String())
}

func TestRewardRouting_DirectVesting_NoPartnership(t *testing.T) {
	k, ctx := setupKnowledgeTest(t)

	submitter := makeValidBech32Addr("submitter1")

	pk := newMockPartnershipKeeper()
	k.SetPartnershipKeeper(pk)

	// Create claim without partnership_id
	claim, round := makeTestClaim(t, k, ctx, submitter,
		"solo claim content", "general", "computational", "1000000")
	require.Equal(t, "", claim.PartnershipId) // no partnership

	result := &keeper.VerificationResult{
		Verdict:    types.Verdict_VERDICT_ACCEPT,
		Confidence: 800000,
		Rewards: []keeper.VerifierReward{
			{Verifier: makeValidBech32Addr("verifier1"), Amount: 500000},
		},
	}

	err := k.CompleteRound(ctx, round, result)
	require.NoError(t, err)

	// Verify NO partnership distribution happened
	require.Len(t, pk.distributed, 0)
}

func TestRewardRouting_FallbackOnPartnershipError(t *testing.T) {
	k, ctx := setupKnowledgeTest(t)

	human := makeValidBech32Addr("human1")
	partnershipID := "partnership-broken"

	// Partnership keeper with no partnerships (will error on DistributeReward)
	pk := newMockPartnershipKeeper()
	k.SetPartnershipKeeper(pk)

	// Create claim with partnership_id that will fail distribution
	claim, round := makeTestClaim(t, k, ctx, human,
		"claim with broken partnership", "general", "computational", "1000000")
	claim.PartnershipId = partnershipID
	require.NoError(t, k.SetClaim(ctx, claim))

	result := &keeper.VerificationResult{
		Verdict:    types.Verdict_VERDICT_ACCEPT,
		Confidence: 800000,
		Rewards: []keeper.VerifierReward{
			{Verifier: makeValidBech32Addr("verifier1"), Amount: 500000},
		},
	}

	// Should not error — falls back to vesting
	err := k.CompleteRound(ctx, round, result)
	require.NoError(t, err)

	// No partnership distribution (it failed)
	require.Len(t, pk.distributed, 0)
}
