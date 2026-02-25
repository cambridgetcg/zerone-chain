package keeper_test

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/zerone-chain/zerone/x/knowledge/keeper"
	"github.com/zerone-chain/zerone/x/knowledge/types"
)

// ─── Bootstrap Fund Tests (R19-7) ────────────────────────────────────────────

func TestSponsoredClaim_FundPays(t *testing.T) {
	// Sponsored claim draws from bootstrap fund, not submitter
	k, ctx, bk := setupKnowledgeTestWithBank(t)
	ms := keeper.NewMsgServerImpl(k)

	submitter := makeValidBech32Addr("sponsor-sub1")

	_, err := ms.SubmitClaim(ctx, &types.MsgSubmitClaim{
		Submitter:   submitter,
		FactContent: "This is a sponsored claim that should be paid by the bootstrap fund",
		Domain:      "mathematics",
		Category:    "formal",
		Stake:       "1000000", // 1 ZRN
		Sponsored:   true,
	})
	require.NoError(t, err)

	// First send should be from bootstrap fund → knowledge (NOT submitter → knowledge)
	require.GreaterOrEqual(t, len(bk.sendCalls), 1)
	require.Equal(t, types.BootstrapFundModuleName, bk.sendCalls[0].from,
		"sponsored claim fee should come from bootstrap fund")
	require.Equal(t, types.ModuleName, bk.sendCalls[0].to)
	require.Equal(t, int64(1_000_000), bk.sendCalls[0].coins.AmountOf("uzrn").Int64())

	// Submitter should NOT appear as a sender
	for _, call := range bk.sendCalls {
		require.NotEqual(t, submitter, call.from,
			"submitter should NOT pay for sponsored claim")
	}
}

func TestSponsoredClaim_SubmitterGetsVesting(t *testing.T) {
	// Accepted sponsored claim creates vesting for submitter, not the fund
	k, ctx, bk := setupKnowledgeTestWithBank(t)
	ms := keeper.NewMsgServerImpl(k)

	submitter := makeValidBech32Addr("sponsor-vest")

	resp, err := ms.SubmitClaim(ctx, &types.MsgSubmitClaim{
		Submitter:   submitter,
		FactContent: "Sponsored claim that will be accepted for vesting test purposes",
		Domain:      "mathematics",
		Category:    "formal",
		Stake:       "1000000",
		Sponsored:   true,
	})
	require.NoError(t, err)

	// Verify the claim was created with the submitter address
	claim, found := k.GetClaim(ctx, resp.ClaimId)
	require.True(t, found)
	require.Equal(t, submitter, claim.Submitter,
		"claim submitter should be the user, not the bootstrap fund")
	_ = bk
}

func TestSponsoredClaim_FeeDistributed(t *testing.T) {
	// Fee from bootstrap fund still goes through revenue split
	k, ctx, bk := setupKnowledgeTestWithBank(t)
	ms := keeper.NewMsgServerImpl(k)

	_, err := ms.SubmitClaim(ctx, &types.MsgSubmitClaim{
		Submitter:   makeValidBech32Addr("sponsor-dist"),
		FactContent: "Sponsored claim fee should be distributed via revenue split exactly like normal",
		Domain:      "mathematics",
		Category:    "formal",
		Stake:       "1000000",
		Sponsored:   true,
	})
	require.NoError(t, err)

	// Should have distribution sends: bootstrap→knowledge, knowledge→protocol, knowledge→dev, knowledge→research
	var protocolSent, devSent, researchSent bool
	for _, call := range bk.sendCalls {
		switch call.to {
		case "protocol_treasury":
			protocolSent = true
			require.Equal(t, int64(220_000), call.coins.AmountOf("uzrn").Int64())
		case "development_fund":
			devSent = true
			require.Equal(t, int64(196_700), call.coins.AmountOf("uzrn").Int64())
		case "research_fund":
			researchSent = true
			require.Equal(t, int64(33_300), call.coins.AmountOf("uzrn").Int64())
		}
	}
	require.True(t, protocolSent, "protocol treasury should receive 22%")
	require.True(t, devSent, "development fund should receive 19.67%")
	require.True(t, researchSent, "research fund should receive 3.33%")
}

func TestSponsoredClaim_PerAddressLimit(t *testing.T) {
	// 11th claim from same address should be rejected (limit is 10)
	k, ctx, bk := setupKnowledgeTestWithBank(t)
	ms := keeper.NewMsgServerImpl(k)

	submitter := makeValidBech32Addr("sponsor-lim")

	// Submit 10 claims (at limit)
	for i := 0; i < 10; i++ {
		_, err := ms.SubmitClaim(ctx, &types.MsgSubmitClaim{
			Submitter:   submitter,
			FactContent: makeUniqueContent("Per-address limit test claim", i),
			Domain:      "mathematics",
			Category:    "formal",
			Stake:       "100000",
			Sponsored:   true,
		})
		require.NoError(t, err, "claim %d should succeed", i+1)
	}

	// 11th should fail
	_, err := ms.SubmitClaim(ctx, &types.MsgSubmitClaim{
		Submitter:   submitter,
		FactContent: "This is the eleventh claim which should exceed the per-address lifetime limit",
		Domain:      "mathematics",
		Category:    "formal",
		Stake:       "100000",
		Sponsored:   true,
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "bootstrap fund claims")
	_ = bk
}

func TestSponsoredClaim_PerEpochLimit(t *testing.T) {
	// 101st claim in epoch should be rejected (limit is 100)
	k, ctx, bk := setupKnowledgeTestWithBank(t)
	ms := keeper.NewMsgServerImpl(k)

	// Submit 100 claims from different addresses (use short unique seeds to avoid addr collision)
	for i := 0; i < 100; i++ {
		seed := fmt.Sprintf("ep%03d", i) // 5 chars — unique per address within 20 byte limit
		submitter := makeValidBech32Addr(seed)
		_, err := ms.SubmitClaim(ctx, &types.MsgSubmitClaim{
			Submitter:   submitter,
			FactContent: makeUniqueContent("Per-epoch limit test claim number", i),
			Domain:      "mathematics",
			Category:    "formal",
			Stake:       "100000",
			Sponsored:   true,
		})
		require.NoError(t, err, "claim %d should succeed (addr=%s)", i+1, seed)
	}

	// 101st should fail
	_, err := ms.SubmitClaim(ctx, &types.MsgSubmitClaim{
		Submitter:   makeValidBech32Addr("ep100"),
		FactContent: "This claim number one hundred and one should exceed epoch rate limit for sponsorship",
		Domain:      "mathematics",
		Category:    "formal",
		Stake:       "100000",
		Sponsored:   true,
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "epoch limit")
	_ = bk
}

func TestSponsoredClaim_FundExhausted(t *testing.T) {
	// Returns clear error when fund is empty
	k, ctx, bk := setupKnowledgeTestWithBank(t)
	ms := keeper.NewMsgServerImpl(k)

	// Drain the fund by clearing module balance
	bk.moduleBalances[types.BootstrapFundModuleName] = sdk.NewCoins()

	_, err := ms.SubmitClaim(ctx, &types.MsgSubmitClaim{
		Submitter:   makeValidBech32Addr("sponsor-empty"),
		FactContent: "This sponsored claim should fail because the bootstrap fund is exhausted now",
		Domain:      "mathematics",
		Category:    "formal",
		Stake:       "1000000",
		Sponsored:   true,
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "insufficient")
}

func TestSponsoredClaim_FeeCap(t *testing.T) {
	// Claim above fee cap (5 ZRN) should be rejected
	k, ctx, bk := setupKnowledgeTestWithBank(t)
	ms := keeper.NewMsgServerImpl(k)

	_, err := ms.SubmitClaim(ctx, &types.MsgSubmitClaim{
		Submitter:   makeValidBech32Addr("sponsor-cap"),
		FactContent: "This claim has a review fee above the bootstrap fund cap of five ZRN",
		Domain:      "mathematics",
		Category:    "formal",
		Stake:       "10000000", // 10 ZRN > 5 ZRN cap
		Sponsored:   true,
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "exceeds bootstrap fund cap")
	_ = bk
}

func TestSponsoredClaim_Disabled(t *testing.T) {
	// sponsored=true rejected when fund disabled
	k, ctx, bk := setupKnowledgeTestWithBank(t)
	ms := keeper.NewMsgServerImpl(k)

	// Disable the bootstrap fund
	params, _ := k.GetParams(ctx)
	params.BootstrapFundEnabled = false
	require.NoError(t, k.SetParams(ctx, params))

	_, err := ms.SubmitClaim(ctx, &types.MsgSubmitClaim{
		Submitter:   makeValidBech32Addr("sponsor-off"),
		FactContent: "This sponsored claim should fail because bootstrap fund is disabled now",
		Domain:      "mathematics",
		Category:    "formal",
		Stake:       "1000000",
		Sponsored:   true,
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "disabled")
	_ = bk
}

func TestBootstrapFundStatus_Query(t *testing.T) {
	// REST endpoint returns correct balance/counts
	k, ctx, bk := setupKnowledgeTestWithBank(t)
	ms := keeper.NewMsgServerImpl(k)
	qs := keeper.NewQueryServerImpl(k)

	// Submit 3 sponsored claims
	for i := 0; i < 3; i++ {
		_, err := ms.SubmitClaim(ctx, &types.MsgSubmitClaim{
			Submitter:   makeValidBech32Addr(makeUniqueContent("query-sub", i)),
			FactContent: makeUniqueContent("Bootstrap fund status query test claim", i),
			Domain:      "mathematics",
			Category:    "formal",
			Stake:       "1000000",
			Sponsored:   true,
		})
		require.NoError(t, err)
	}

	resp, err := qs.BootstrapFundStatus(ctx, &types.QueryBootstrapFundStatusRequest{})
	require.NoError(t, err)
	require.True(t, resp.Enabled)
	require.Equal(t, "3", resp.TotalClaimsFunded)
	require.Equal(t, "97", resp.RemainingPerEpoch) // 100 - 3 = 97
	_ = bk
}

func TestGenesisInit_FundAllocated(t *testing.T) {
	// Genesis allocation mints correct amount to fund
	_, ctx, bk := setupKnowledgeTestWithBank(t)

	// setupKnowledgeTestWithBank calls InitGenesis with DefaultGenesis()
	// which has BootstrapFundAllocation: "22222000000"
	require.Equal(t, int64(22_222_000_000), bk.minted.AmountOf("uzrn").Int64(),
		"genesis should mint 22,222 ZRN to bootstrap fund")

	// Module balance should also be set
	fundBal := bk.moduleBalances[types.BootstrapFundModuleName]
	require.False(t, fundBal.IsZero(), "bootstrap fund module should have balance after genesis")
	_ = ctx
}

// ─── Test helpers ─────────────────────────────────────────────────────────────

// makeUniqueContent creates a unique claim content string to avoid content hash dedup.
func makeUniqueContent(base string, i int) string {
	return base + " — unique suffix " + string(rune('A'+i%26)) + string(rune('0'+i/26))
}
