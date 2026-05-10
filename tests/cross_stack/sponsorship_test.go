package cross_stack_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	sdkmath "cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"

	zeroneapp "github.com/zerone-chain/zerone/app"
	knowledgetypes "github.com/zerone-chain/zerone/x/knowledge/types"
	sponsorshipkeeper "github.com/zerone-chain/zerone/x/sponsorship/keeper"
	sponsorshiptypes "github.com/zerone-chain/zerone/x/sponsorship/types"
)

// TestSponsorship_CreateFulfillEndToEnd drives the full bounty lifecycle
// against live keepers: sponsor escrows funds, a verified fact in the
// bounty's domain triggers payout to the fact's submitter, bounty's
// fulfilled_count advances, escrow_remaining decreases.
//
// This is the MVP's proof that external value can flow into ZERONE via
// the sponsorship pathway — the sponsor's funds left their account and
// the worker's account grew, gated entirely by chain-side verification.
func TestSponsorship_CreateFulfillEndToEnd(t *testing.T) {
	h := NewTestHarness(t)

	// Sponsor account, funded.
	sponsor := sdk.AccAddress(make([]byte, 20))
	for i := range sponsor {
		sponsor[i] = byte(0xA0 + i)
	}
	require.NoError(t, h.FundAccount(sponsor, sdk.NewCoins(sdk.NewCoin(zeroneapp.BondDenom, sdkmath.NewInt(100_000_000)))))

	// Worker account.
	worker := sdk.AccAddress(make([]byte, 20))
	for i := range worker {
		worker[i] = byte(0xB0 + i)
	}

	srv := sponsorshipkeeper.NewMsgServerImpl(h.SponsorshipKeeper)

	// Create bounty.
	createResp, err := srv.CreateBountyOrder(h.Ctx, &sponsorshiptypes.MsgCreateBountyOrder{
		Sponsor:          sponsor.String(),
		Domain:           "mathematics",
		PricePerArtifact: "1000000",
		TargetCount:      3,
		DurationBlocks:   1000,
	})
	require.NoError(t, err)
	bountyID := createResp.BountyId

	sponsorBalance := h.App.BankKeeper.GetBalance(h.Ctx, sponsor, zeroneapp.BondDenom)
	require.Equal(t, sdkmath.NewInt(100_000_000-3_000_000), sponsorBalance.Amount,
		"sponsor balance should be debited by total escrow (price × target)")

	// Seed a verified fact directly via the knowledge keeper. This
	// bypasses the verification round (exercised in knowledge tests);
	// here we test the sponsorship pathway in isolation.
	currentBlock := uint64(h.Ctx.BlockHeight())
	fact := &knowledgetypes.Fact{
		Id:               "test-fact-sponsorship-1",
		Domain:           "mathematics",
		Submitter:        worker.String(),
		SubmittedAtBlock: currentBlock,
		Status:           knowledgetypes.FactStatus_FACT_STATUS_VERIFIED,
		Content:          "Test fact for sponsorship MVP",
	}
	require.NoError(t, h.KnowledgeKeeper.SetFact(h.Ctx, fact))

	// Fulfill the bounty with this fact. Anyone can be the caller.
	fulfillResp, err := srv.FulfillBounty(h.Ctx, &sponsorshiptypes.MsgFulfillBounty{
		Caller:   sponsor.String(), // doesn't matter — chain reads worker from fact.Submitter
		BountyId: bountyID,
		FactId:   fact.Id,
	})
	require.NoError(t, err)
	require.Equal(t, worker.String(), fulfillResp.Worker)
	require.Equal(t, "1000000", fulfillResp.AmountPaid)
	require.False(t, fulfillResp.BountyNowFulfilled, "1 of 3 — not fulfilled yet")

	// Worker received payout.
	workerBalance := h.App.BankKeeper.GetBalance(h.Ctx, worker, zeroneapp.BondDenom)
	require.Equal(t, sdkmath.NewInt(1_000_000), workerBalance.Amount,
		"worker balance should equal one per-artifact price after one fulfillment")

	// Bounty bookkeeping.
	order, found := h.SponsorshipKeeper.GetBountyOrder(h.Ctx, bountyID)
	require.True(t, found)
	require.Equal(t, uint32(1), order.FulfilledCount)
	require.Equal(t, "2000000", order.EscrowRemaining)
	require.Equal(t, sponsorshiptypes.BountyStatus_BOUNTY_STATUS_ACTIVE, order.Status)
}

// TestSponsorship_CancelRefundsRemaining confirms that the sponsor can
// reclaim unspent escrow at any time.
func TestSponsorship_CancelRefundsRemaining(t *testing.T) {
	h := NewTestHarness(t)

	sponsor := sdk.AccAddress(make([]byte, 20))
	for i := range sponsor {
		sponsor[i] = byte(0xC0 + i)
	}
	require.NoError(t, h.FundAccount(sponsor, sdk.NewCoins(sdk.NewCoin(zeroneapp.BondDenom, sdkmath.NewInt(100_000_000)))))

	srv := sponsorshipkeeper.NewMsgServerImpl(h.SponsorshipKeeper)

	createResp, err := srv.CreateBountyOrder(h.Ctx, &sponsorshiptypes.MsgCreateBountyOrder{
		Sponsor: sponsor.String(), Domain: "mathematics", PricePerArtifact: "1000000",
		TargetCount: 5, DurationBlocks: 1000,
	})
	require.NoError(t, err)

	cancelResp, err := srv.CancelBountyOrder(h.Ctx, &sponsorshiptypes.MsgCancelBountyOrder{
		Sponsor: sponsor.String(), BountyId: createResp.BountyId,
	})
	require.NoError(t, err)
	require.Equal(t, "5000000", cancelResp.RefundedAmount)

	sponsorBalance := h.App.BankKeeper.GetBalance(h.Ctx, sponsor, zeroneapp.BondDenom)
	require.Equal(t, sdkmath.NewInt(100_000_000), sponsorBalance.Amount,
		"after cancel, sponsor balance should equal initial fund")
}

// TestSponsorship_NoMintingHappens binds the invariant: sponsorship is
// supply circulation, not emission. Compare total uzrn supply before
// and after a full bounty lifecycle (create + fulfill + cancel).
func TestSponsorship_NoMintingHappens(t *testing.T) {
	h := NewTestHarness(t)

	sponsor := sdk.AccAddress(make([]byte, 20))
	for i := range sponsor {
		sponsor[i] = byte(0xD0 + i)
	}
	worker := sdk.AccAddress(make([]byte, 20))
	for i := range worker {
		worker[i] = byte(0xE0 + i)
	}
	require.NoError(t, h.FundAccount(sponsor, sdk.NewCoins(sdk.NewCoin(zeroneapp.BondDenom, sdkmath.NewInt(50_000_000)))))

	preSupply := h.App.BankKeeper.GetSupply(h.Ctx, zeroneapp.BondDenom)

	srv := sponsorshipkeeper.NewMsgServerImpl(h.SponsorshipKeeper)
	createResp, err := srv.CreateBountyOrder(h.Ctx, &sponsorshiptypes.MsgCreateBountyOrder{
		Sponsor: sponsor.String(), Domain: "mathematics", PricePerArtifact: "1000000",
		TargetCount: 2, DurationBlocks: 1000,
	})
	require.NoError(t, err)

	currentBlock := uint64(h.Ctx.BlockHeight())
	require.NoError(t, h.KnowledgeKeeper.SetFact(h.Ctx, &knowledgetypes.Fact{
		Id: "no-mint-fact", Domain: "mathematics", Submitter: worker.String(),
		SubmittedAtBlock: currentBlock, Status: knowledgetypes.FactStatus_FACT_STATUS_VERIFIED,
	}))
	_, err = srv.FulfillBounty(h.Ctx, &sponsorshiptypes.MsgFulfillBounty{
		Caller: sponsor.String(), BountyId: createResp.BountyId, FactId: "no-mint-fact",
	})
	require.NoError(t, err)

	_, err = srv.CancelBountyOrder(h.Ctx, &sponsorshiptypes.MsgCancelBountyOrder{
		Sponsor: sponsor.String(), BountyId: createResp.BountyId,
	})
	require.NoError(t, err)

	postSupply := h.App.BankKeeper.GetSupply(h.Ctx, zeroneapp.BondDenom)
	require.Equal(t, preSupply.Amount, postSupply.Amount,
		"sponsorship must not mint — total uzrn supply must be unchanged across create+fulfill+cancel")
}
