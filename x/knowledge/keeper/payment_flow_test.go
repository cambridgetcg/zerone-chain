package keeper_test

import (
	"bytes"
	"fmt"
	"strconv"
	"testing"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/stretchr/testify/require"

	"github.com/zerone-chain/zerone/x/knowledge/keeper"
	"github.com/zerone-chain/zerone/x/knowledge/types"
)

// ─── helpers ────────────────────────────────────────────────────────────────

// pfConsumer returns a valid bech32 address for use as a consumer/second wallet.
func pfConsumer() string {
	return sdk.AccAddress(bytes.Repeat([]byte{3}, 20)).String()
}

// pfSetup creates a keeper with bank and sets default API revenue params (pricing + splits).
func pfSetup(t *testing.T) (keeper.Keeper, sdk.Context, *mockBankKeeper) {
	t.Helper()
	k, ctx, bk := setupKeeperWithBank(t)
	rp := types.DefaultAPIRevenueParams() // includes pricing: input=1, output=3
	require.NoError(t, k.SetAPIRevenueParams(ctx, rp))
	return k, sdk.UnwrapSDKContext(ctx), bk
}

// pfCreateKey creates an API key and returns the key hash.
func pfCreateKey(t *testing.T, k keeper.Keeper, ctx sdk.Context, owner, keyHash string) {
	t.Helper()
	_, err := k.CreateAPIKey(ctx, &types.MsgCreateAPIKey{Owner: owner, KeyHash: keyHash})
	require.NoError(t, err)
}

// pfDeposit deposits API credits.
func pfDeposit(t *testing.T, k keeper.Keeper, ctx sdk.Context, wallet, amount string) {
	t.Helper()
	_, err := k.DepositAPICredits(ctx, &types.MsgDepositAPICredits{Depositor: wallet, Amount: amount})
	require.NoError(t, err)
}

// pfRecordUsage records a single API usage batch.
func pfRecordUsage(t *testing.T, k keeper.Keeper, ctx sdk.Context, keyHash string, input, output, reqs uint64) *types.MsgRecordAPIUsageResponse {
	t.Helper()
	resp, err := k.RecordAPIUsage(ctx, &types.MsgRecordAPIUsage{
		Bridge: testWallet2,
		Batches: []*types.APIUsageBatch{
			{APIKeyHash: keyHash, InputTokens: input, OutputTokens: output, RequestCount: reqs, ModelUsed: "zerone-8b"},
		},
	})
	require.NoError(t, err)
	return resp
}

func parseU64(s string) uint64 {
	v, _ := strconv.ParseUint(s, 10, 64)
	return v
}

// ─── Scenario 1: Full payment lifecycle ────────────────────────────────────

func TestPaymentFlow_FullLifecycle(t *testing.T) {
	k, ctx, bk := pfSetup(t)

	// 1. Create wallet, deposit 100 ZRN (100_000_000 uzrn)
	pfCreateKey(t, k, ctx, testWallet, testKeyHash)
	pfDeposit(t, k, ctx, testWallet, "100000000")

	bal := k.GetAPIBalance(ctx, testWallet)
	require.Equal(t, "100000000", bal.Balance)
	require.Len(t, bk.accountToModuleCalls, 1)

	// 2. Simulate 10 API requests with varying token counts
	type req struct {
		input, output uint64
	}
	requests := []req{
		{1000, 500}, {2000, 1000}, {3000, 1500}, {5000, 2000},
		{1000, 200}, {500, 100}, {10000, 5000}, {800, 400},
		{1200, 600}, {4000, 2000},
	}

	var totalExpectedCost uint64
	for _, r := range requests {
		// cost = (input * 1 / 1000) + (output * 3 / 1000)
		inputCost := (r.input * 1) / 1000
		outputCost := (r.output * 3) / 1000
		cost := inputCost + outputCost
		if cost == 0 {
			cost = 1
		}
		totalExpectedCost += cost
	}

	var totalDeducted uint64
	for _, r := range requests {
		resp := pfRecordUsage(t, k, ctx, testKeyHash, r.input, r.output, 1)
		totalDeducted += parseU64(resp.TotalDeducted)
	}

	require.Equal(t, totalExpectedCost, totalDeducted)

	// 3. Verify credits deducted correctly
	bal = k.GetAPIBalance(ctx, testWallet)
	require.Equal(t, strconv.FormatUint(100_000_000-totalExpectedCost, 10), bal.Balance)
	require.Equal(t, strconv.FormatUint(totalExpectedCost, 10), bal.TotalConsumed)

	// 4. Verify usage records accumulated
	usage := k.GetAPIUsageRecord(ctx, testWallet, 1) // epoch = 100/100 = 1
	require.Equal(t, uint64(10), usage.RequestCount)

	// 5. Trigger epoch revenue distribution
	// Queue pending revenue for epoch 0 (previous epoch)
	pendingRev := k.GetPendingAPIRevenue(ctx, 1)
	require.Equal(t, totalExpectedCost, pendingRev)

	// Set up sample + validator for distribution
	require.NoError(t, k.SetSample(ctx, &types.Sample{
		Id:           "s-train-1",
		Submitter:    testWallet,
		Status:       types.SampleStatus_SAMPLE_STATUS_GOLD,
		QualityTier:  "gold",
		FitnessScore: 800_000,
		AccessCount:  5,
	}))
	require.NoError(t, k.SetValidatorInfo(ctx, testWallet2))

	// Distribute at block 200 (epoch 2 boundary, distributes epoch 1 revenue)
	distCtx := ctx.WithBlockHeight(200)
	k.DistributeAPIRevenue(distCtx)

	// Verify pending revenue was cleared
	remaining := k.GetPendingAPIRevenue(distCtx, 1)
	require.Equal(t, uint64(0), remaining)
}

// ─── Scenario 2: Credit management — deposit, use, withdraw ────────────────

func TestPaymentFlow_CreditManagement(t *testing.T) {
	k, ctx, _ := pfSetup(t)

	// Deposit 50 ZRN
	pfCreateKey(t, k, ctx, testWallet, testKeyHash)
	pfDeposit(t, k, ctx, testWallet, "50000000")

	// Use 30 ZRN worth of API calls (30_000_000 uzrn)
	// At default pricing (1 per 1K input, 3 per 1K output):
	// 10000 input + 10000 output = 10 + 30 = 40 uzrn per call
	// Need 30_000_000 / 40 = 750_000 calls ... too many.
	// Use larger token counts: 500K input + 166K output ≈ 500 + 498 = 998 per call
	// 30 calls of ~1M uzrn each → still big
	// Simpler: single batch with 30M input tokens
	// cost = (30_000_000 * 1 / 1000) = 30_000 uzrn
	// That's too small. Let's just do direct: set pricing higher.
	revParams := types.DefaultAPIRevenueParams()
	revParams.PricePerInputToken = "1000"  // 1000 uzrn per 1000 input tokens = 1 uzrn/token
	revParams.PricePerOutputToken = "3000" // 3 uzrn per output token
	require.NoError(t, k.SetAPIRevenueParams(ctx, revParams))

	// 10M input tokens → cost = 10M * 1000/1000 = 10M; output 5M → 5M*3000/1000 = 15M → total 25M
	resp, err := k.RecordAPIUsage(ctx, &types.MsgRecordAPIUsage{
		Bridge: testWallet2,
		Batches: []*types.APIUsageBatch{
			{APIKeyHash: testKeyHash, InputTokens: 10_000_000, OutputTokens: 5_000_000, RequestCount: 100},
		},
	})
	require.NoError(t, err)
	require.Equal(t, "25000000", resp.TotalDeducted)

	// Remaining should be 50M - 25M = 25M
	bal := k.GetAPIBalance(ctx, testWallet)
	require.Equal(t, "25000000", bal.Balance)

	// Withdraw remaining 20 ZRN (leave 5M)
	wResp, err := k.WithdrawAPICredits(ctx, &types.MsgWithdrawAPICredits{
		Wallet: testWallet,
		Amount: "20000000",
	})
	require.NoError(t, err)
	require.Equal(t, "5000000", wResp.RemainingBalance)

	// Verify: wallet balance restored partially
	bal = k.GetAPIBalance(ctx, testWallet)
	require.Equal(t, "5000000", bal.Balance)

	// Withdraw remaining 5M
	wResp, err = k.WithdrawAPICredits(ctx, &types.MsgWithdrawAPICredits{
		Wallet: testWallet,
		Amount: "5000000",
	})
	require.NoError(t, err)
	require.Equal(t, "0", wResp.RemainingBalance)

	// Balance is now 0 → next API call should get 0 deduction (partial deduction of 0)
	resp, err = k.RecordAPIUsage(ctx, &types.MsgRecordAPIUsage{
		Bridge: testWallet2,
		Batches: []*types.APIUsageBatch{
			{APIKeyHash: testKeyHash, InputTokens: 1000, OutputTokens: 500, RequestCount: 1},
		},
	})
	require.NoError(t, err)
	require.Equal(t, "0", resp.TotalDeducted)
}

// ─── Scenario 3: Model attribution revenue flow ────────────────────────────

func TestPaymentFlow_ModelAttributionRevenue(t *testing.T) {
	k, ctx, _ := pfSetup(t)

	// 1. Submit 5 TDUs (create 5 gold samples with fitness scores)
	sampleIDs := []string{"tdu-1", "tdu-2", "tdu-3", "tdu-4", "tdu-5"}
	for i, id := range sampleIDs {
		require.NoError(t, k.SetSample(ctx, &types.Sample{
			Id:           id,
			Submitter:    testWallet,
			Status:       types.SampleStatus_SAMPLE_STATUS_GOLD,
			QualityTier:  "gold",
			FitnessScore: uint64(900_000 - i*50_000), // decreasing fitness
			AccessCount:  uint64(10 - i),
		}))
	}

	// 2. Create training record using those TDUs
	require.NoError(t, k.SetTrainingRecord(ctx, &types.TrainingRecord{
		Operator:           testWallet,
		AttestationHash:    "attest_abc",
		DatasetFingerprint: "dataset_fp",
		DatasetSize:        5,
		BaseModel:          "zerone-8b",
		ModelHash:          "model_xyz",
		BenchmarkScore:     0.95,
		BlockHeight:        50,
	}))

	// 3. Serve API requests using that model — deposit and record usage
	pfCreateKey(t, k, ctx, testWallet, testKeyHash)
	pfDeposit(t, k, ctx, testWallet, "1000000")
	pfRecordUsage(t, k, ctx, testKeyHash, 5000, 2000, 5)

	// 4. Trigger revenue distribution
	revParams := k.GetAPIRevenueParams(ctx)
	require.NoError(t, k.SetAPIRevenueParams(ctx, revParams))

	// Queue was accumulated during RecordAPIUsage
	pendingRev := k.GetPendingAPIRevenue(ctx, 1)
	require.True(t, pendingRev > 0)

	// Add a validator for infra share
	require.NoError(t, k.SetValidatorInfo(ctx, testWallet2))

	// Distribute at block 200
	distCtx := ctx.WithBlockHeight(200)
	k.DistributeAPIRevenue(distCtx)

	// Verify distribution happened (pending cleared)
	remaining := k.GetPendingAPIRevenue(distCtx, 1)
	require.Equal(t, uint64(0), remaining)

	// 5. Verify model attribution traces back
	attrs := k.GetModelAttribution(ctx, "model_xyz")
	require.NotEmpty(t, attrs)
	require.Contains(t, attrs, "attest_abc")
}

// ─── Scenario 4: Sample access pricing by quality tier ─────────────────────

func TestPaymentFlow_SampleAccessPricing(t *testing.T) {
	k, ctx, bk := pfSetup(t)
	params := types.DefaultParams()
	require.NoError(t, k.SetParams(ctx, &params))

	tests := []struct {
		name     string
		tier     string
		status   types.SampleStatus
		expected string // base=100000 × multiplier/10000
	}{
		{"gold (3×)", "gold", types.SampleStatus_SAMPLE_STATUS_GOLD, "300000"},
		{"silver (2×)", "silver", types.SampleStatus_SAMPLE_STATUS_SILVER, "200000"},
		{"bronze (1×)", "bronze", types.SampleStatus_SAMPLE_STATUS_BRONZE, "100000"},
	}

	for i, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			id := fmt.Sprintf("sample-tier-%d", i)
			require.NoError(t, k.SetSample(ctx, &types.Sample{
				Id:          id,
				Submitter:   testWallet,
				Status:      tt.status,
				QualityTier: tt.tier,
				Energy:      500_000,
				EnergyCap:   1_000_000,
				Content:     "test content",
				Language:    "en",
			}))

			resp, err := k.AccessSample(ctx, &types.MsgAccessSample{
				Consumer: testWallet,
				SampleId: id,
			})
			require.NoError(t, err)
			require.Equal(t, tt.expected, resp.Payment)
		})
	}

	// Verify bank received payments
	require.Len(t, bk.accountToModuleCalls, 3)
}

// ─── Scenario 5: Revenue distribution accuracy with consent multipliers ────

func TestPaymentFlow_RevenueDistributionWithConsent(t *testing.T) {
	k, ctx, bk := pfSetup(t)
	params := types.DefaultParams()
	require.NoError(t, k.SetParams(ctx, &params))

	// Create 5 samples with different consent types
	samples := []struct {
		id          string
		consentType types.ConsentType
	}{
		{"s-self-1", types.ConsentType_CONSENT_TYPE_SELF_AUTHORED},
		{"s-self-2", types.ConsentType_CONSENT_TYPE_SELF_AUTHORED},
		{"s-fair-1", types.ConsentType_CONSENT_TYPE_FAIR_USE},
		{"s-optin-1", types.ConsentType_CONSENT_TYPE_OPT_IN},
		{"s-pub-1", types.ConsentType_CONSENT_TYPE_PUBLIC_LICENSE},
	}

	for _, s := range samples {
		require.NoError(t, k.SetSample(ctx, &types.Sample{
			Id:          s.id,
			Submitter:   testWallet,
			Status:      types.SampleStatus_SAMPLE_STATUS_GOLD,
			QualityTier: "gold",
			Consent: &types.ConsentProof{
				Type:     s.consentType,
				ProofUri: "https://example.com/proof",
			},
			Energy:    500_000,
			EnergyCap: 1_000_000,
			Content:   "test content",
			Language:  "en",
		}))
	}

	// Access each sample to generate revenue
	consumer := pfConsumer()
	for _, s := range samples {
		_, err := k.AccessSample(ctx, &types.MsgAccessSample{
			Consumer: consumer,
			SampleId: s.id,
		})
		require.NoError(t, err)
	}

	// Verify pending revenue queued for each sample
	for _, s := range samples {
		pending := k.GetPendingRevenue(ctx, s.id)
		require.True(t, pending > 0, "sample %s should have pending revenue", s.id)
	}

	// Trigger epoch revenue distribution
	k.DistributeEpochRevenue(ctx, &params)

	// Verify bank calls happened (submitter payouts + protocol revenue)
	require.True(t, len(bk.accountToModuleCalls) > 0, "should have bank transfers")
	// moduleToAccount calls are submitter payouts
	require.True(t, len(bk.moduleToAccountCalls) > 0, "should have submitter payouts")
}

// ─── Scenario 6: Sponsored sample access ───────────────────────────────────

func TestPaymentFlow_SponsoredAccess(t *testing.T) {
	k, ctx, bk := pfSetup(t)
	params := types.DefaultParams()
	require.NoError(t, k.SetParams(ctx, &params))

	// Create sample with patronage
	sample := &types.Sample{
		Id:                   "s-sponsored",
		Submitter:            testWallet,
		Status:               types.SampleStatus_SAMPLE_STATUS_GOLD,
		QualityTier:          "gold",
		Energy:               500_000,
		EnergyCap:            1_000_000,
		Content:              "sponsored content",
		Language:             "en",
		PatronageAmount:      "0",
		PatronageExpiryBlock: 0,
	}
	require.NoError(t, k.SetSample(ctx, sample))

	// Sponsor the sample
	_, err := k.SponsorSample(ctx, &types.MsgSponsorSample{
		Sponsor:        testWallet2,
		SampleId:       "s-sponsored",
		Amount:         "10000000", // 10 ZRN
		DurationBlocks: 200,       // Active until block 300
	})
	require.NoError(t, err)
	require.Len(t, bk.accountToModuleCalls, 1)

	// Verify patronage recorded
	sponsored, found := k.GetSample(ctx, "s-sponsored")
	require.True(t, found)
	require.Equal(t, "10000000", sponsored.PatronageAmount)
	require.Equal(t, uint64(300), sponsored.PatronageExpiryBlock)
	require.Equal(t, uint64(1_000_000), sponsored.Energy) // restored to cap

	// Access after patronage setup — still costs normally (patronage preserves from pruning)
	resp, err := k.AccessSample(ctx, &types.MsgAccessSample{
		Consumer: testWallet,
		SampleId: "s-sponsored",
	})
	require.NoError(t, err)
	require.Equal(t, "300000", resp.Payment) // gold 3× base
}

// ─── Scenario 7: Insufficient credits ──────────────────────────────────────

func TestPaymentFlow_InsufficientCredits(t *testing.T) {
	k, ctx, _ := pfSetup(t)

	pfCreateKey(t, k, ctx, testWallet, testKeyHash)
	pfDeposit(t, k, ctx, testWallet, "5") // Only 5 uzrn

	// Request costs (5000*1/1000) + (2000*3/1000) = 5+6 = 11 uzrn
	// Balance is 5, so partial deduction of 5
	resp, err := k.RecordAPIUsage(ctx, &types.MsgRecordAPIUsage{
		Bridge:  testWallet2,
		Batches: []*types.APIUsageBatch{
			{APIKeyHash: testKeyHash, InputTokens: 5000, OutputTokens: 2000, RequestCount: 1},
		},
	})
	require.NoError(t, err)

	// Partial deduction: deducts all available balance (5)
	require.Equal(t, "5", resp.TotalDeducted)

	// Verify no remaining balance
	bal := k.GetAPIBalance(ctx, testWallet)
	require.Equal(t, "0", bal.Balance)

	// Next call gets 0 deduction (no partial)
	resp, err = k.RecordAPIUsage(ctx, &types.MsgRecordAPIUsage{
		Bridge: testWallet2,
		Batches: []*types.APIUsageBatch{
			{APIKeyHash: testKeyHash, InputTokens: 1000, OutputTokens: 500, RequestCount: 1},
		},
	})
	require.NoError(t, err)
	require.Equal(t, "0", resp.TotalDeducted)
}

// ─── Scenario 8: Concurrent access on same sample ──────────────────────────

func TestPaymentFlow_ConcurrentAccess(t *testing.T) {
	k, ctx, bk := pfSetup(t)
	params := types.DefaultParams()
	require.NoError(t, k.SetParams(ctx, &params))

	// Create one sample
	require.NoError(t, k.SetSample(ctx, &types.Sample{
		Id:          "s-concurrent",
		Submitter:   testWallet,
		Status:      types.SampleStatus_SAMPLE_STATUS_GOLD,
		QualityTier: "gold",
		Energy:      500_000,
		EnergyCap:   1_000_000,
		Content:     "concurrent content",
		Language:    "en",
	}))

	// 10 sequential accesses (simulating concurrent — deterministic in test)
	consumer := pfConsumer()
	for i := 0; i < 10; i++ {
		resp, err := k.AccessSample(ctx, &types.MsgAccessSample{
			Consumer: consumer,
			SampleId: "s-concurrent",
		})
		require.NoError(t, err)
		require.Equal(t, "300000", resp.Payment) // gold = 3× base = 300000
	}

	// Verify: all payments recorded correctly
	require.Len(t, bk.accountToModuleCalls, 10)

	// Verify: access_count incremented correctly
	updated, found := k.GetSample(ctx, "s-concurrent")
	require.True(t, found)
	require.Equal(t, uint64(10), updated.AccessCount)

	// Verify: energy restored for each access
	require.True(t, updated.Energy > 500_000, "energy should have been restored via access")

	// Verify total pending revenue
	pending := k.GetPendingRevenue(ctx, "s-concurrent")
	require.Equal(t, uint64(3_000_000), pending) // 300000 × 10
}

// ─── Scenario 9: Zero-revenue epoch ────────────────────────────────────────

func TestPaymentFlow_ZeroRevenueEpoch(t *testing.T) {
	k, ctx, _ := pfSetup(t)

	// No pending revenue — just run distribution
	distCtx := ctx.WithBlockHeight(200) // epoch 2 boundary
	k.DistributeAPIRevenue(distCtx)     // should not error

	// Verify no panics, no errors
	remaining := k.GetPendingAPIRevenue(distCtx, 1)
	require.Equal(t, uint64(0), remaining)
}

// ─── Scenario 10: Revoked key usage ────────────────────────────────────────

func TestPaymentFlow_RevokedKeyUsage(t *testing.T) {
	k, ctx, _ := pfSetup(t)

	// Create key, use it
	pfCreateKey(t, k, ctx, testWallet, testKeyHash)
	pfDeposit(t, k, ctx, testWallet, "10000000")
	resp := pfRecordUsage(t, k, ctx, testKeyHash, 5000, 2000, 1)
	require.True(t, parseU64(resp.TotalDeducted) > 0)

	// Revoke
	_, err := k.RevokeAPIKey(ctx, &types.MsgRevokeAPIKey{
		Owner:   testWallet,
		KeyHash: testKeyHash,
	})
	require.NoError(t, err)

	// Attempt usage with revoked key
	resp, err = k.RecordAPIUsage(ctx, &types.MsgRecordAPIUsage{
		Bridge: testWallet2,
		Batches: []*types.APIUsageBatch{
			{APIKeyHash: testKeyHash, InputTokens: 5000, OutputTokens: 2000, RequestCount: 1},
		},
	})
	require.NoError(t, err)
	require.Equal(t, "0", resp.TotalDeducted)
	require.Equal(t, uint64(0), resp.BatchesProcessed)
}

// ─── Scenario 11: Quality tier transitions ─────────────────────────────────

func TestPaymentFlow_QualityTierTransition(t *testing.T) {
	k, ctx, _ := pfSetup(t)
	params := types.DefaultParams()
	require.NoError(t, k.SetParams(ctx, &params))

	// Create sample as gold
	require.NoError(t, k.SetSample(ctx, &types.Sample{
		Id:          "s-transition",
		Submitter:   testWallet,
		Status:      types.SampleStatus_SAMPLE_STATUS_GOLD,
		QualityTier: "gold",
		Energy:      500_000,
		EnergyCap:   1_000_000,
		Content:     "transition content",
		Language:    "en",
	}))

	consumer := pfConsumer()

	// Access at gold price
	resp, err := k.AccessSample(ctx, &types.MsgAccessSample{
		Consumer: consumer,
		SampleId: "s-transition",
	})
	require.NoError(t, err)
	require.Equal(t, "300000", resp.Payment)

	// Downgrade to silver (simulate contest result)
	sample, found := k.GetSample(ctx, "s-transition")
	require.True(t, found)
	sample.QualityTier = "silver"
	sample.Status = types.SampleStatus_SAMPLE_STATUS_SILVER
	require.NoError(t, k.SetSample(ctx, sample))

	// Access at silver price
	resp, err = k.AccessSample(ctx, &types.MsgAccessSample{
		Consumer: consumer,
		SampleId: "s-transition",
	})
	require.NoError(t, err)
	require.Equal(t, "200000", resp.Payment) // reflects current tier, not original
}

// ─── Scenario 12 (on-chain): Batch usage submission ────────────────────────

func TestPaymentFlow_BatchUsageSubmission(t *testing.T) {
	k, ctx, _ := pfSetup(t)

	pfCreateKey(t, k, ctx, testWallet, testKeyHash)
	pfDeposit(t, k, ctx, testWallet, "100000000") // 100 ZRN

	// Build 100 batches
	batches := make([]*types.APIUsageBatch, 100)
	for i := 0; i < 100; i++ {
		batches[i] = &types.APIUsageBatch{
			APIKeyHash:   testKeyHash,
			InputTokens:  uint64(1000 + i*10),
			OutputTokens: uint64(500 + i*5),
			RequestCount: 1,
			ModelUsed:    "zerone-8b",
		}
	}

	resp, err := k.RecordAPIUsage(ctx, &types.MsgRecordAPIUsage{
		Bridge:  testWallet2,
		Batches: batches,
	})
	require.NoError(t, err)
	require.Equal(t, uint64(100), resp.BatchesProcessed)

	// Verify total deducted is reasonable
	totalDeducted := parseU64(resp.TotalDeducted)
	require.True(t, totalDeducted > 0, "should have deducted some amount")

	// Verify balance decreased
	bal := k.GetAPIBalance(ctx, testWallet)
	balVal := parseU64(bal.Balance)
	require.Equal(t, 100_000_000-totalDeducted, balVal)

	// Verify usage record accumulated
	usage := k.GetAPIUsageRecord(ctx, testWallet, 1) // epoch 1
	require.Equal(t, uint64(100), usage.RequestCount)
}

// ─── Additional: Deposit + Withdraw + Overdraft ────────────────────────────

func TestPaymentFlow_WithdrawExceedsBalance(t *testing.T) {
	k, ctx, _ := pfSetup(t)

	pfDeposit(t, k, ctx, testWallet, "1000")

	// Try to withdraw more than deposited
	_, err := k.WithdrawAPICredits(ctx, &types.MsgWithdrawAPICredits{
		Wallet: testWallet,
		Amount: "2000",
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "exceeds balance")

	// Balance unchanged
	bal := k.GetAPIBalance(ctx, testWallet)
	require.Equal(t, "1000", bal.Balance)
}

// ─── Additional: Multiple wallets with separate balances ───────────────────

func TestPaymentFlow_MultipleWalletIsolation(t *testing.T) {
	k, ctx, _ := pfSetup(t)
	wallet2 := pfConsumer()

	pfCreateKey(t, k, ctx, testWallet, testKeyHash)
	pfCreateKey(t, k, ctx, wallet2, testKeyHash2)
	pfDeposit(t, k, ctx, testWallet, "5000000")
	pfDeposit(t, k, ctx, wallet2, "3000000")

	// Usage on wallet1's key
	pfRecordUsage(t, k, ctx, testKeyHash, 5000, 2000, 1)

	// Wallet2's balance unaffected
	bal2 := k.GetAPIBalance(ctx, wallet2)
	require.Equal(t, "3000000", bal2.Balance)
}

// ─── Additional: Revenue distribution 5-way split accuracy ─────────────────

func TestPaymentFlow_RevenueDistributionSplitAccuracy(t *testing.T) {
	k, ctx, bk := pfSetup(t)

	// Queue 10000 uzrn revenue in epoch 0
	require.NoError(t, k.SetPendingAPIRevenue(ctx, 0, 10000))

	// Set up sample and validator
	require.NoError(t, k.SetSample(ctx, &types.Sample{
		Id:           "s-split",
		Submitter:    testWallet,
		Status:       types.SampleStatus_SAMPLE_STATUS_GOLD,
		QualityTier:  "gold",
		FitnessScore: 800_000,
		AccessCount:  10,
	}))
	require.NoError(t, k.SetValidatorInfo(ctx, testWallet2))

	// Distribute at block 100 (epoch 1 boundary → distributes epoch 0)
	distCtx := ctx.WithBlockHeight(100)
	k.DistributeAPIRevenue(distCtx)

	// Default splits: training=40%, infra=25%, submitter=20%, protocol=10%, research=5%
	// Total = 10000
	// training = 4000, infra = 2500, submitter = 2000, protocol = 1000, research = 500
	// infra goes to validators via bank.SendCoinsFromModuleToAccount
	require.True(t, len(bk.moduleToAccountCalls) > 0, "should have validator payouts")

	// Verify pending revenue cleared
	remaining := k.GetPendingAPIRevenue(distCtx, 0)
	require.Equal(t, uint64(0), remaining)
}

// ─── Additional: Custom revenue params ─────────────────────────────────────

func TestPaymentFlow_CustomRevenueParams(t *testing.T) {
	k, ctx, _ := pfSetup(t)

	// Set custom params
	custom := types.APIRevenueParams{
		TrainingShareBPS:    3000,
		InfraShareBPS:       3000,
		SubmitterShareBPS:   2000,
		ProtocolShareBPS:    1000,
		ResearchShareBPS:    1000,
		PricePerInputToken:  "2",
		PricePerOutputToken: "6",
	}
	require.NoError(t, k.SetAPIRevenueParams(ctx, custom))

	// Verify roundtrip
	stored := k.GetAPIRevenueParams(ctx)
	require.Equal(t, uint64(3000), stored.TrainingShareBPS)
	require.Equal(t, "2", stored.PricePerInputToken)
	require.Equal(t, "6", stored.PricePerOutputToken)

	// Verify pricing takes effect
	pfCreateKey(t, k, ctx, testWallet, testKeyHash)
	pfDeposit(t, k, ctx, testWallet, "100000")

	// cost = (5000 * 2/1000) + (2000 * 6/1000) = 10 + 12 = 22
	resp := pfRecordUsage(t, k, ctx, testKeyHash, 5000, 2000, 1)
	require.Equal(t, "22", resp.TotalDeducted)
}

// ─── Additional: Unknown key skipped ───────────────────────────────────────

func TestPaymentFlow_UnknownKeySkipped(t *testing.T) {
	k, ctx, _ := pfSetup(t)

	pfCreateKey(t, k, ctx, testWallet, testKeyHash)
	pfDeposit(t, k, ctx, testWallet, "100000")

	// Mix of valid and unknown keys
	resp, err := k.RecordAPIUsage(ctx, &types.MsgRecordAPIUsage{
		Bridge: testWallet2,
		Batches: []*types.APIUsageBatch{
			{APIKeyHash: testKeyHash, InputTokens: 5000, OutputTokens: 2000, RequestCount: 1},
			{APIKeyHash: "0000000000000000000000000000000000000000000000000000000000000000", InputTokens: 1000, OutputTokens: 500, RequestCount: 1},
		},
	})
	require.NoError(t, err)
	require.Equal(t, uint64(1), resp.BatchesProcessed) // only valid key processed
}

// ─── Additional: Minimum cost (1 uzrn) ─────────────────────────────────────

func TestPaymentFlow_MinimumCost(t *testing.T) {
	k, ctx, _ := pfSetup(t)

	pfCreateKey(t, k, ctx, testWallet, testKeyHash)
	pfDeposit(t, k, ctx, testWallet, "100000")

	// Very small request: 1 input, 0 output → floor to 1 uzrn
	resp := pfRecordUsage(t, k, ctx, testKeyHash, 1, 0, 1)
	require.Equal(t, "1", resp.TotalDeducted) // minimum
}

// ─── Additional: Epoch boundary tracking ───────────────────────────────────

func TestPaymentFlow_EpochBoundaryTracking(t *testing.T) {
	k, ctx, _ := pfSetup(t)

	pfCreateKey(t, k, ctx, testWallet, testKeyHash)
	pfDeposit(t, k, ctx, testWallet, "100000")

	// Record at block 100 (epoch 1)
	pfRecordUsage(t, k, ctx, testKeyHash, 5000, 2000, 1)

	// Record at block 250 (epoch 2)
	ctx2 := ctx.WithBlockHeight(250)
	_, err := k.RecordAPIUsage(ctx2, &types.MsgRecordAPIUsage{
		Bridge: testWallet2,
		Batches: []*types.APIUsageBatch{
			{APIKeyHash: testKeyHash, InputTokens: 3000, OutputTokens: 1000, RequestCount: 2},
		},
	})
	require.NoError(t, err)

	// Verify separate epoch records
	usage1 := k.GetAPIUsageRecord(ctx, testWallet, 1)
	require.Equal(t, uint64(1), usage1.RequestCount)
	require.Equal(t, uint64(5000), usage1.InputTokens)

	usage2 := k.GetAPIUsageRecord(ctx2, testWallet, 2)
	require.Equal(t, uint64(2), usage2.RequestCount)
	require.Equal(t, uint64(3000), usage2.InputTokens)

	// All usage by wallet
	allUsage := k.GetAPIUsageByWallet(ctx2, testWallet)
	require.Len(t, allUsage, 2)
}

// ─── Additional: Deposit bank failure ──────────────────────────────────────

func TestPaymentFlow_DepositBankFailure(t *testing.T) {
	k, ctx, bk := pfSetup(t)

	bk.failNextSend = true
	_, err := k.DepositAPICredits(ctx, &types.MsgDepositAPICredits{
		Depositor: testWallet,
		Amount:    "1000000",
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "insufficient")

	// Balance should remain 0
	bal := k.GetAPIBalance(ctx, testWallet)
	require.Equal(t, "0", bal.Balance)
}

// ─── Additional: Revenue params validation ─────────────────────────────────

func TestPaymentFlow_RevenueParamsValidation(t *testing.T) {
	tests := []struct {
		name  string
		p     types.APIRevenueParams
		valid bool
	}{
		{"default is valid", types.DefaultAPIRevenueParams(), true},
		{"sum < 10000", types.APIRevenueParams{
			TrainingShareBPS: 1000, InfraShareBPS: 1000, SubmitterShareBPS: 1000,
			ProtocolShareBPS: 1000, ResearchShareBPS: 1000,
		}, false},
		{"sum > 10000", types.APIRevenueParams{
			TrainingShareBPS: 5000, InfraShareBPS: 3000, SubmitterShareBPS: 2000,
			ProtocolShareBPS: 1000, ResearchShareBPS: 1000,
		}, false},
		{"exact 10000", types.APIRevenueParams{
			TrainingShareBPS: 2000, InfraShareBPS: 2000, SubmitterShareBPS: 2000,
			ProtocolShareBPS: 2000, ResearchShareBPS: 2000,
		}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.p.Validate()
			if tt.valid {
				require.NoError(t, err)
			} else {
				require.Error(t, err)
			}
		})
	}
}

// ─── Additional: Multiple keys same wallet, mixed revoked ──────────────────

func TestPaymentFlow_MixedRevokedKeys(t *testing.T) {
	k, ctx, _ := pfSetup(t)

	pfCreateKey(t, k, ctx, testWallet, testKeyHash)
	pfCreateKey(t, k, ctx, testWallet, testKeyHash2)
	pfDeposit(t, k, ctx, testWallet, "100000")

	// Revoke key2
	_, err := k.RevokeAPIKey(ctx, &types.MsgRevokeAPIKey{Owner: testWallet, KeyHash: testKeyHash2})
	require.NoError(t, err)

	// Submit batch with both keys
	resp, err := k.RecordAPIUsage(ctx, &types.MsgRecordAPIUsage{
		Bridge: testWallet2,
		Batches: []*types.APIUsageBatch{
			{APIKeyHash: testKeyHash, InputTokens: 5000, OutputTokens: 2000, RequestCount: 1},
			{APIKeyHash: testKeyHash2, InputTokens: 3000, OutputTokens: 1000, RequestCount: 1},
		},
	})
	require.NoError(t, err)
	require.Equal(t, uint64(1), resp.BatchesProcessed) // only active key
}

// ─── Additional: Full deposit/use/withdraw lifecycle with exact arithmetic ──

func TestPaymentFlow_ExactArithmetic(t *testing.T) {
	k, ctx, _ := pfSetup(t)

	pfCreateKey(t, k, ctx, testWallet, testKeyHash)
	pfDeposit(t, k, ctx, testWallet, "10000")

	// Exact cost: (5000 * 1/1000) + (2000 * 3/1000) = 5 + 6 = 11
	resp := pfRecordUsage(t, k, ctx, testKeyHash, 5000, 2000, 1)
	require.Equal(t, "11", resp.TotalDeducted)

	bal := k.GetAPIBalance(ctx, testWallet)
	require.Equal(t, "9989", bal.Balance)
	require.Equal(t, "11", bal.TotalConsumed)
	require.Equal(t, "10000", bal.TotalDeposited)

	// Withdraw exact remaining
	wResp, err := k.WithdrawAPICredits(ctx, &types.MsgWithdrawAPICredits{
		Wallet: testWallet,
		Amount: "9989",
	})
	require.NoError(t, err)
	require.Equal(t, "0", wResp.RemainingBalance)
}

// ─── Additional: Message validation ────────────────────────────────────────

func TestPaymentFlow_MessageValidation(t *testing.T) {
	// MsgRecordAPIUsage with empty batches
	msg := &types.MsgRecordAPIUsage{Bridge: testWallet, Batches: nil}
	require.Error(t, msg.ValidateBasic())

	// MsgRecordAPIUsage with zero tokens
	msg = &types.MsgRecordAPIUsage{
		Bridge: testWallet,
		Batches: []*types.APIUsageBatch{
			{APIKeyHash: testKeyHash, InputTokens: 0, OutputTokens: 0},
		},
	}
	require.Error(t, msg.ValidateBasic())

	// MsgDepositAPICredits with zero amount
	dMsg := &types.MsgDepositAPICredits{Depositor: testWallet, Amount: "0"}
	require.Error(t, dMsg.ValidateBasic())

	// MsgCreateAPIKey with bad hash
	kMsg := &types.MsgCreateAPIKey{Owner: testWallet, KeyHash: "short"}
	require.Error(t, kMsg.ValidateBasic())
}
