package keeper_test

import (
	"bytes"
	"testing"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/stretchr/testify/require"

	"github.com/zerone-chain/zerone/x/knowledge/types"
)

// sponsor1 returns a valid bech32 address with "zrn" prefix for sponsorship tests.
func sponsor1() string {
	return sdk.AccAddress(bytes.Repeat([]byte{2}, 20)).String()
}

func TestSponsorSample_Success(t *testing.T) {
	sponsor := sponsor1()
	k, ctx, bk := setupKeeperWithBank(t)

	// Create a gold sample with energy 500/1000
	sample := createGoldSample(t, k, ctx, "sample-s1")
	sample.Energy = 500
	sample.EnergyCap = 1000
	sample.EnergyLastUpdated = 50
	require.NoError(t, k.SetSample(ctx, sample))

	msg := &types.MsgSponsorSample{
		Sponsor:        sponsor,
		SampleId:       "sample-s1",
		Amount:         "5000000",
		DurationBlocks: 1000,
	}

	resp, err := k.SponsorSample(ctx, msg)
	require.NoError(t, err)
	require.NotNil(t, resp)

	// Verify updated sample
	updated, found := k.GetSample(ctx, "sample-s1")
	require.True(t, found)
	require.Equal(t, "5000000", updated.PatronageAmount)
	require.Equal(t, uint64(1100), updated.PatronageExpiryBlock) // block 100 + 1000
	require.Equal(t, uint64(1000), updated.Energy)               // restored to cap
	require.Equal(t, uint64(100), updated.EnergyLastUpdated)

	// Verify bank call
	require.Len(t, bk.accountToModuleCalls, 1)
	require.Equal(t, sponsor, bk.accountToModuleCalls[0].from)
	require.Equal(t, types.ModuleName, bk.accountToModuleCalls[0].to)
	require.Equal(t, sdk.NewInt64Coin("uzrn", 5000000), bk.accountToModuleCalls[0].amount[0])
}

func TestSponsorSample_NotFound(t *testing.T) {
	sponsor := sponsor1()
	k, ctx := setupKeeper(t)

	msg := &types.MsgSponsorSample{
		Sponsor:        sponsor,
		SampleId:       "nonexistent",
		Amount:         "1000000",
		DurationBlocks: 100,
	}

	_, err := k.SponsorSample(ctx, msg)
	require.ErrorIs(t, err, types.ErrSampleNotFound)
}

func TestSponsorSample_ExtendExisting(t *testing.T) {
	sponsor := sponsor1()
	k, ctx, _ := setupKeeperWithBank(t)

	// Create sample with existing patronage
	sample := createGoldSample(t, k, ctx, "sample-s2")
	sample.PatronageAmount = "3000000"
	sample.PatronageExpiryBlock = 500 // Already past current block (100), so extends from 500
	sample.Energy = 800
	sample.EnergyCap = 1000
	sample.EnergyLastUpdated = 50
	require.NoError(t, k.SetSample(ctx, sample))

	msg := &types.MsgSponsorSample{
		Sponsor:        sponsor,
		SampleId:       "sample-s2",
		Amount:         "2000000",
		DurationBlocks: 200,
	}

	resp, err := k.SponsorSample(ctx, msg)
	require.NoError(t, err)
	require.NotNil(t, resp)

	updated, found := k.GetSample(ctx, "sample-s2")
	require.True(t, found)
	require.Equal(t, "5000000", updated.PatronageAmount)          // 3M + 2M
	require.Equal(t, uint64(700), updated.PatronageExpiryBlock)   // 500 + 200 (extends from existing)
	require.Equal(t, uint64(1000), updated.Energy)                // restored to cap
}

func TestSponsorSample_PrunedSample(t *testing.T) {
	sponsor := sponsor1()
	k, ctx, _ := setupKeeperWithBank(t)

	// Create a pruned sample — sponsoring should still work (restores it)
	require.NoError(t, k.SetSample(ctx, &types.Sample{
		Id:        "pruned-s1",
		Content:   "pruned content",
		Domain:    "technology",
		Submitter: testAddr,
		Status:    types.SampleStatus_SAMPLE_STATUS_PRUNED,
		EnergyCap: 1000,
	}))

	msg := &types.MsgSponsorSample{
		Sponsor:        sponsor,
		SampleId:       "pruned-s1",
		Amount:         "1000000",
		DurationBlocks: 500,
	}

	resp, err := k.SponsorSample(ctx, msg)
	require.NoError(t, err)
	require.NotNil(t, resp)

	updated, found := k.GetSample(ctx, "pruned-s1")
	require.True(t, found)
	require.Equal(t, "1000000", updated.PatronageAmount)
	require.Equal(t, uint64(600), updated.PatronageExpiryBlock) // 100 + 500
	require.Equal(t, uint64(1000), updated.Energy)              // restored to cap
}

func TestSponsorSample_PaymentFails(t *testing.T) {
	sponsor := sponsor1()
	k, ctx, bk := setupKeeperWithBank(t)

	createGoldSample(t, k, ctx, "sample-s3")

	bk.failNextSend = true

	msg := &types.MsgSponsorSample{
		Sponsor:        sponsor,
		SampleId:       "sample-s3",
		Amount:         "1000000",
		DurationBlocks: 100,
	}

	_, err := k.SponsorSample(ctx, msg)
	require.ErrorIs(t, err, types.ErrInsufficientPayment)
}

func TestSponsorSample_ZeroDuration(t *testing.T) {
	sponsor := sponsor1()
	k, ctx := setupKeeper(t)

	createGoldSample(t, k, ctx, "sample-s4")

	msg := &types.MsgSponsorSample{
		Sponsor:        sponsor,
		SampleId:       "sample-s4",
		Amount:         "1000000",
		DurationBlocks: 0,
	}

	_, err := k.SponsorSample(ctx, msg)
	require.ErrorIs(t, err, types.ErrInvalidChallenge)
}

func TestSponsorSample_EmitsEvent(t *testing.T) {
	sponsor := sponsor1()
	k, ctx, _ := setupKeeperWithBank(t)

	sample := createGoldSample(t, k, ctx, "sample-s5")
	sample.EnergyCap = 1000
	require.NoError(t, k.SetSample(ctx, sample))

	msg := &types.MsgSponsorSample{
		Sponsor:        sponsor,
		SampleId:       "sample-s5",
		Amount:         "2000000",
		DurationBlocks: 300,
	}

	_, err := k.SponsorSample(ctx, msg)
	require.NoError(t, err)

	sdkCtx := sdk.UnwrapSDKContext(ctx)
	events := sdkCtx.EventManager().Events()

	found := false
	for _, e := range events {
		if e.Type == "sample_sponsored" {
			found = true
			attrMap := make(map[string]string)
			for _, attr := range e.Attributes {
				attrMap[attr.Key] = attr.Value
			}
			require.Equal(t, "sample-s5", attrMap["sample_id"])
			require.Equal(t, sponsor, attrMap["sponsor"])
			require.Equal(t, "2000000", attrMap["amount"])
			require.Equal(t, "300", attrMap["duration_blocks"])
			require.Equal(t, "2000000", attrMap["patronage_total"])
			require.Equal(t, "400", attrMap["expiry_block"]) // 100 + 300
			break
		}
	}
	require.True(t, found, "expected sample_sponsored event")
}

func TestSponsorSample_InvalidAmount(t *testing.T) {
	sponsor := sponsor1()
	k, ctx := setupKeeper(t)

	createGoldSample(t, k, ctx, "sample-s6")

	msg := &types.MsgSponsorSample{
		Sponsor:        sponsor,
		SampleId:       "sample-s6",
		Amount:         "0",
		DurationBlocks: 100,
	}

	_, err := k.SponsorSample(ctx, msg)
	require.ErrorIs(t, err, types.ErrInsufficientPayment)
}

func TestSponsorSample_MultipleSponsorships(t *testing.T) {
	sponsor := sponsor1()
	k, ctx, bk := setupKeeperWithBank(t)

	// Create a gold sample with energy 200/1000
	sample := createGoldSample(t, k, ctx, "sample-ms1")
	sample.Energy = 200
	sample.EnergyCap = 1000
	sample.EnergyLastUpdated = 50
	require.NoError(t, k.SetSample(ctx, sample))

	// First sponsorship: 3M for 500 blocks
	msg1 := &types.MsgSponsorSample{
		Sponsor:        sponsor,
		SampleId:       "sample-ms1",
		Amount:         "3000000",
		DurationBlocks: 500,
	}
	_, err := k.SponsorSample(ctx, msg1)
	require.NoError(t, err)

	s1, found := k.GetSample(ctx, "sample-ms1")
	require.True(t, found)
	require.Equal(t, "3000000", s1.PatronageAmount)
	require.Equal(t, uint64(600), s1.PatronageExpiryBlock)  // 100 + 500
	require.Equal(t, uint64(1000), s1.Energy)                // restored to cap

	// Second sponsorship: 2M for 200 blocks — should accumulate
	msg2 := &types.MsgSponsorSample{
		Sponsor:        sponsor,
		SampleId:       "sample-ms1",
		Amount:         "2000000",
		DurationBlocks: 200,
	}
	_, err = k.SponsorSample(ctx, msg2)
	require.NoError(t, err)

	s2, found := k.GetSample(ctx, "sample-ms1")
	require.True(t, found)
	require.Equal(t, "5000000", s2.PatronageAmount)          // 3M + 2M
	require.Equal(t, uint64(800), s2.PatronageExpiryBlock)   // 600 + 200 (extends from existing expiry > current block)
	require.Equal(t, uint64(1000), s2.Energy)                // still at cap

	// Verify both bank calls happened
	require.Len(t, bk.accountToModuleCalls, 2)
	require.Equal(t, sdk.NewInt64Coin("uzrn", 3000000), bk.accountToModuleCalls[0].amount[0])
	require.Equal(t, sdk.NewInt64Coin("uzrn", 2000000), bk.accountToModuleCalls[1].amount[0])
}

func TestSponsorSample_RestoresEnergy_ZeroCap(t *testing.T) {
	sponsor := sponsor1()
	k, ctx, _ := setupKeeperWithBank(t)

	// Create a gold sample with energy_cap=0 (no energy cap set)
	sample := createGoldSample(t, k, ctx, "sample-zc1")
	sample.Energy = 0
	sample.EnergyCap = 0
	sample.EnergyLastUpdated = 50
	require.NoError(t, k.SetSample(ctx, sample))

	msg := &types.MsgSponsorSample{
		Sponsor:        sponsor,
		SampleId:       "sample-zc1",
		Amount:         "1000000",
		DurationBlocks: 100,
	}

	_, err := k.SponsorSample(ctx, msg)
	require.NoError(t, err)

	updated, found := k.GetSample(ctx, "sample-zc1")
	require.True(t, found)
	require.Equal(t, "1000000", updated.PatronageAmount)
	require.Equal(t, uint64(200), updated.PatronageExpiryBlock)  // 100 + 100
	// Energy stays at 0 because cap is 0 — the condition `if sample.EnergyCap > 0` is false
	require.Equal(t, uint64(0), updated.Energy)
	require.Equal(t, uint64(100), updated.EnergyLastUpdated)
}
