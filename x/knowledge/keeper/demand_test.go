package keeper_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/zerone-chain/zerone/x/knowledge/types"
)

func TestReportDemand_AuthorizedReporter(t *testing.T) {
	k, ctx := setupKeeper(t)
	setupDefaultDomains(t, k, ctx)

	resp, err := k.ReportDemand(ctx, &types.MsgReportDemand{
		Reporter: "authority", // matches keeper authority
		Reports: []*types.DemandReport{
			{Domain: "science", Subject: "quantum_computing", Queries: 50, Fulfilled: 10, Unfulfilled: 40},
		},
	})
	require.NoError(t, err)
	require.NotNil(t, resp)

	demand, found := k.GetTrainingDemand(ctx, "science", "quantum_computing")
	require.True(t, found)
	require.Equal(t, uint64(50), demand.QueryCount)
	require.Equal(t, uint64(40), demand.UnfulfilledCount)
}

func TestReportDemand_UnauthorizedReporter(t *testing.T) {
	k, ctx := setupKeeper(t)

	_, err := k.ReportDemand(ctx, &types.MsgReportDemand{
		Reporter: testAddr, // not the authority
		Reports: []*types.DemandReport{
			{Domain: "science", Subject: "quantum", Queries: 10, Unfulfilled: 10},
		},
	})
	require.ErrorIs(t, err, types.ErrUnauthorized)
}

func TestReportDemand_DomainNotFound(t *testing.T) {
	k, ctx := setupKeeper(t)

	_, err := k.ReportDemand(ctx, &types.MsgReportDemand{
		Reporter: "authority",
		Reports: []*types.DemandReport{
			{Domain: "nonexistent", Subject: "topic", Queries: 10, Unfulfilled: 10},
		},
	})
	require.ErrorIs(t, err, types.ErrDomainNotFound)
}

func TestReportDemand_UpsertAccumulates(t *testing.T) {
	k, ctx := setupKeeper(t)
	setupDefaultDomains(t, k, ctx)

	_, err := k.ReportDemand(ctx, &types.MsgReportDemand{
		Reporter: "authority",
		Reports: []*types.DemandReport{
			{Domain: "science", Subject: "physics", Queries: 30, Fulfilled: 10, Unfulfilled: 20},
		},
	})
	require.NoError(t, err)

	_, err = k.ReportDemand(ctx, &types.MsgReportDemand{
		Reporter: "authority",
		Reports: []*types.DemandReport{
			{Domain: "science", Subject: "physics", Queries: 20, Fulfilled: 5, Unfulfilled: 15},
		},
	})
	require.NoError(t, err)

	demand, found := k.GetTrainingDemand(ctx, "science", "physics")
	require.True(t, found)
	require.Equal(t, uint64(50), demand.QueryCount)
	require.Equal(t, uint64(15), demand.FulfilledCount)
	require.Equal(t, uint64(35), demand.UnfulfilledCount)
}

func TestReportDemand_AutoBountyCreated(t *testing.T) {
	k, ctx := setupKeeper(t)
	setupDefaultDomains(t, k, ctx)

	params := types.DefaultParams()
	params.AutoBountyThreshold = 50
	require.NoError(t, k.SetParams(ctx, &params))

	_, err := k.ReportDemand(ctx, &types.MsgReportDemand{
		Reporter: "authority",
		Reports: []*types.DemandReport{
			{Domain: "science", Subject: "quantum", Queries: 100, Fulfilled: 20, Unfulfilled: 80},
		},
	})
	require.NoError(t, err)

	bounties := k.GetActiveBounties(ctx, "science")
	require.Len(t, bounties, 1)
	require.Equal(t, "science", bounties[0].Domain)
	require.Equal(t, "quantum", bounties[0].Subject)
	require.Equal(t, params.AutoBountyAmount, bounties[0].RewardAmount)
}

func TestReportDemand_NoDuplicateAutoBounty(t *testing.T) {
	k, ctx := setupKeeper(t)
	setupDefaultDomains(t, k, ctx)

	params := types.DefaultParams()
	params.AutoBountyThreshold = 10
	require.NoError(t, k.SetParams(ctx, &params))

	for i := 0; i < 2; i++ {
		_, err := k.ReportDemand(ctx, &types.MsgReportDemand{
			Reporter: "authority",
			Reports: []*types.DemandReport{
				{Domain: "science", Subject: "quantum", Queries: 50, Unfulfilled: 50},
			},
		})
		require.NoError(t, err)
	}

	bounties := k.GetActiveBounties(ctx, "science")
	require.Len(t, bounties, 1)
}

func TestReportDemand_BelowThresholdNoBounty(t *testing.T) {
	k, ctx := setupKeeper(t)
	setupDefaultDomains(t, k, ctx)

	_, err := k.ReportDemand(ctx, &types.MsgReportDemand{
		Reporter: "authority",
		Reports: []*types.DemandReport{
			{Domain: "science", Subject: "quantum", Queries: 50, Unfulfilled: 30},
		},
	})
	require.NoError(t, err)

	bounties := k.GetActiveBounties(ctx, "science")
	require.Len(t, bounties, 0)
}

func TestReportDemand_MultipleReports(t *testing.T) {
	k, ctx := setupKeeper(t)
	setupDefaultDomains(t, k, ctx)

	_, err := k.ReportDemand(ctx, &types.MsgReportDemand{
		Reporter: "authority",
		Reports: []*types.DemandReport{
			{Domain: "science", Subject: "physics", Queries: 10, Unfulfilled: 5},
			{Domain: "technology", Subject: "golang", Queries: 20, Unfulfilled: 15},
		},
	})
	require.NoError(t, err)

	d1, found := k.GetTrainingDemand(ctx, "science", "physics")
	require.True(t, found)
	require.Equal(t, uint64(10), d1.QueryCount)

	d2, found := k.GetTrainingDemand(ctx, "technology", "golang")
	require.True(t, found)
	require.Equal(t, uint64(20), d2.QueryCount)
}

// ─── Manual Bounty Funding ──────────────────────────────────────────────────

func TestFundBounty(t *testing.T) {
	k, ctx, _ := setupKeeperWithBank(t)
	setupDefaultDomains(t, k, ctx)

	resp, err := k.FundBounty(ctx, &types.MsgFundBounty{
		Funder:        testAddr,
		Domain:        "science",
		Topic:         "quantum",
		Amount:        "5000000",
		ExpiresBlocks: 50000,
	})
	require.NoError(t, err)
	require.NotEmpty(t, resp.BountyId)

	bounty, found := k.GetDataBounty(ctx, resp.BountyId)
	require.True(t, found)
	require.Equal(t, "science", bounty.Domain)
	require.Equal(t, "quantum", bounty.Subject)
	require.Equal(t, "5000000", bounty.RewardAmount)
	require.False(t, bounty.Claimed)
}

func TestFundBounty_DomainNotFound(t *testing.T) {
	k, ctx := setupKeeper(t)

	_, err := k.FundBounty(ctx, &types.MsgFundBounty{
		Funder: testAddr,
		Domain: "nonexistent",
		Topic:  "topic",
		Amount: "1000000",
	})
	require.ErrorIs(t, err, types.ErrDomainNotFound)
}

func TestFundBounty_InvalidAmount(t *testing.T) {
	k, ctx := setupKeeper(t)
	setupDefaultDomains(t, k, ctx)

	_, err := k.FundBounty(ctx, &types.MsgFundBounty{
		Funder: testAddr,
		Domain: "science",
		Topic:  "topic",
		Amount: "0",
	})
	require.ErrorIs(t, err, types.ErrInsufficientPayment)
}

func TestFundBounty_InsufficientFunds(t *testing.T) {
	k, ctx, bk := setupKeeperWithBank(t)
	setupDefaultDomains(t, k, ctx)

	bk.failNextSend = true
	_, err := k.FundBounty(ctx, &types.MsgFundBounty{
		Funder: testAddr,
		Domain: "science",
		Topic:  "topic",
		Amount: "1000000",
	})
	require.ErrorIs(t, err, types.ErrInsufficientPayment)
}

func TestFundBounty_DefaultExpiry(t *testing.T) {
	k, ctx, _ := setupKeeperWithBank(t)
	setupDefaultDomains(t, k, ctx)

	resp, err := k.FundBounty(ctx, &types.MsgFundBounty{
		Funder: testAddr,
		Domain: "science",
		Topic:  "topic",
		Amount: "1000000",
	})
	require.NoError(t, err)

	bounty, found := k.GetDataBounty(ctx, resp.BountyId)
	require.True(t, found)
	require.Equal(t, uint64(100)+100_000, bounty.ExpiresAtBlock) // blockHeight(100) + default
}

// ─── Bounty Fulfillment ─────────────────────────────────────────────────────

func TestCheckBountyFulfillment_Match(t *testing.T) {
	k, ctx, bk := setupKeeperWithBank(t)
	setupDefaultDomains(t, k, ctx)

	require.NoError(t, k.SetDataBounty(ctx, &types.DataBounty{
		Id: "b1", Domain: "science", Subject: "quantum", RewardAmount: "5000000", ExpiresAtBlock: 10000,
	}))
	require.NoError(t, k.SetBountyDomainIndex(ctx, "science", "b1"))

	sample := &types.Sample{Id: "s1", Domain: "science", Submitter: testAddr, Topics: []string{"quantum", "physics"}}
	k.CheckBountyFulfillment(ctx, sample)

	bounty, found := k.GetDataBounty(ctx, "b1")
	require.True(t, found)
	require.True(t, bounty.Claimed)
	require.Equal(t, "s1", bounty.ClaimedBySampleId)
	require.Len(t, bk.moduleToAccountCalls, 1)
}

func TestCheckBountyFulfillment_NoMatch(t *testing.T) {
	k, ctx, bk := setupKeeperWithBank(t)
	setupDefaultDomains(t, k, ctx)

	require.NoError(t, k.SetDataBounty(ctx, &types.DataBounty{
		Id: "b1", Domain: "science", Subject: "quantum", RewardAmount: "5000000", ExpiresAtBlock: 10000,
	}))
	require.NoError(t, k.SetBountyDomainIndex(ctx, "science", "b1"))

	sample := &types.Sample{Id: "s1", Domain: "technology", Submitter: testAddr, Topics: []string{"quantum"}}
	k.CheckBountyFulfillment(ctx, sample)

	bounty, _ := k.GetDataBounty(ctx, "b1")
	require.False(t, bounty.Claimed)
	require.Len(t, bk.moduleToAccountCalls, 0)
}

func TestCheckBountyFulfillment_Expired(t *testing.T) {
	k, ctx, bk := setupKeeperWithBank(t)
	setupDefaultDomains(t, k, ctx)

	require.NoError(t, k.SetDataBounty(ctx, &types.DataBounty{
		Id: "b1", Domain: "science", Subject: "quantum", RewardAmount: "5000000", ExpiresAtBlock: 50,
	}))
	require.NoError(t, k.SetBountyDomainIndex(ctx, "science", "b1"))

	sample := &types.Sample{Id: "s1", Domain: "science", Submitter: testAddr, Topics: []string{"quantum"}}
	k.CheckBountyFulfillment(ctx, sample)

	bounty, _ := k.GetDataBounty(ctx, "b1")
	require.False(t, bounty.Claimed)
	require.Len(t, bk.moduleToAccountCalls, 0)
}

func TestCheckBountyFulfillment_SubjectEmpty(t *testing.T) {
	k, ctx, bk := setupKeeperWithBank(t)
	setupDefaultDomains(t, k, ctx)

	require.NoError(t, k.SetDataBounty(ctx, &types.DataBounty{
		Id: "b1", Domain: "science", Subject: "", RewardAmount: "5000000", ExpiresAtBlock: 10000,
	}))
	require.NoError(t, k.SetBountyDomainIndex(ctx, "science", "b1"))

	sample := &types.Sample{Id: "s1", Domain: "science", Submitter: testAddr}
	k.CheckBountyFulfillment(ctx, sample)

	bounty, _ := k.GetDataBounty(ctx, "b1")
	require.True(t, bounty.Claimed)
	require.Len(t, bk.moduleToAccountCalls, 1)
}
