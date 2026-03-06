package keeper_test

import (
	"testing"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/stretchr/testify/require"

	"github.com/zerone-chain/zerone/x/knowledge/types"
)

const (
	testWallet  = "zrn1qqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqulc3kt"
	testWallet2 = "zrn1qyqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqyz5mxy"
	testKeyHash = "a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2"
	testKeyHash2 = "b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3"
)

// ─── Test: API key creation binds to wallet ────────────────────────────────

func TestCreateAPIKey(t *testing.T) {
	k, ctx := setupKeeper(t)

	msg := &types.MsgCreateAPIKey{
		Owner:         testWallet,
		KeyHash:       testKeyHash,
		RateLimitTier: "standard",
	}

	resp, err := k.CreateAPIKey(ctx, msg)
	require.NoError(t, err)
	require.Equal(t, testKeyHash, resp.KeyHash)

	// Verify record exists
	record, found := k.GetAPIKeyRecord(ctx, testKeyHash)
	require.True(t, found)
	require.Equal(t, testWallet, record.Wallet)
	require.Equal(t, "standard", record.RateLimitTier)
	require.False(t, record.Revoked)
	require.Equal(t, int64(100), record.CreatedAtBlock)

	// Verify wallet index
	keys := k.GetAPIKeysByWallet(ctx, testWallet)
	require.Len(t, keys, 1)
	require.Equal(t, testKeyHash, keys[0])
}

func TestCreateAPIKey_Duplicate(t *testing.T) {
	k, ctx := setupKeeper(t)

	msg := &types.MsgCreateAPIKey{
		Owner:   testWallet,
		KeyHash: testKeyHash,
	}

	_, err := k.CreateAPIKey(ctx, msg)
	require.NoError(t, err)

	// Second attempt should fail
	_, err = k.CreateAPIKey(ctx, msg)
	require.Error(t, err)
	require.Contains(t, err.Error(), "already registered")
}

func TestCreateAPIKey_DefaultTier(t *testing.T) {
	k, ctx := setupKeeper(t)

	msg := &types.MsgCreateAPIKey{
		Owner:   testWallet,
		KeyHash: testKeyHash,
	}

	_, err := k.CreateAPIKey(ctx, msg)
	require.NoError(t, err)

	record, found := k.GetAPIKeyRecord(ctx, testKeyHash)
	require.True(t, found)
	require.Equal(t, "standard", record.RateLimitTier)
}

// ─── Test: Credit deposit increases balance ────────────────────────────────

func TestDepositAPICredits(t *testing.T) {
	k, ctx, _ := setupKeeperWithBank(t)

	msg := &types.MsgDepositAPICredits{
		Depositor: testWallet,
		Amount:    "1000000", // 1 ZRN
	}

	resp, err := k.DepositAPICredits(ctx, msg)
	require.NoError(t, err)
	require.Equal(t, "1000000", resp.NewBalance)

	// Verify balance
	balance := k.GetAPIBalance(ctx, testWallet)
	require.Equal(t, "1000000", balance.Balance)
	require.Equal(t, "1000000", balance.TotalDeposited)
	require.Equal(t, "0", balance.TotalConsumed)

	// Deposit more
	msg.Amount = "500000"
	resp, err = k.DepositAPICredits(ctx, msg)
	require.NoError(t, err)
	require.Equal(t, "1500000", resp.NewBalance)
}

func TestDepositAPICredits_InsufficientFunds(t *testing.T) {
	k, ctx, bk := setupKeeperWithBank(t)
	bk.failNextSend = true

	msg := &types.MsgDepositAPICredits{
		Depositor: testWallet,
		Amount:    "1000000",
	}

	_, err := k.DepositAPICredits(ctx, msg)
	require.Error(t, err)
	require.Contains(t, err.Error(), "insufficient")
}

// ─── Test: Usage recording deducts correct amount ──────────────────────────

func TestRecordAPIUsage(t *testing.T) {
	k, ctx, _ := setupKeeperWithBank(t)

	// Setup: create key + deposit credits
	_, err := k.CreateAPIKey(ctx, &types.MsgCreateAPIKey{
		Owner:   testWallet,
		KeyHash: testKeyHash,
	})
	require.NoError(t, err)

	_, err = k.DepositAPICredits(ctx, &types.MsgDepositAPICredits{
		Depositor: testWallet,
		Amount:    "10000", // 10K uzrn
	})
	require.NoError(t, err)

	// Set pricing params: 1 uzrn per 1000 input, 3 uzrn per 1000 output
	revParams := types.DefaultAPIRevenueParams()
	revParams.PricePerInputToken = "1"
	revParams.PricePerOutputToken = "3"
	_ = k.SetAPIRevenueParams(ctx, revParams)

	// Record usage: 5000 input tokens + 2000 output tokens
	// Cost = (5000 * 1 / 1000) + (2000 * 3 / 1000) = 5 + 6 = 11 uzrn
	msg := &types.MsgRecordAPIUsage{
		Bridge: testWallet2,
		Batches: []*types.APIUsageBatch{
			{
				APIKeyHash:   testKeyHash,
				InputTokens:  5000,
				OutputTokens: 2000,
				RequestCount: 1,
				ModelUsed:    "zerone-8b",
			},
		},
	}

	resp, err := k.RecordAPIUsage(ctx, msg)
	require.NoError(t, err)
	require.Equal(t, "11", resp.TotalDeducted)
	require.Equal(t, uint64(1), resp.BatchesProcessed)

	// Check balance deducted
	balance := k.GetAPIBalance(ctx, testWallet)
	require.Equal(t, "9989", balance.Balance)  // 10000 - 11
	require.Equal(t, "11", balance.TotalConsumed)
}

func TestRecordAPIUsage_MultipleBatches(t *testing.T) {
	k, ctx, _ := setupKeeperWithBank(t)

	// Setup
	_, _ = k.CreateAPIKey(ctx, &types.MsgCreateAPIKey{Owner: testWallet, KeyHash: testKeyHash})
	_, _ = k.CreateAPIKey(ctx, &types.MsgCreateAPIKey{Owner: testWallet, KeyHash: testKeyHash2})
	_, _ = k.DepositAPICredits(ctx, &types.MsgDepositAPICredits{Depositor: testWallet, Amount: "100000"})

	revParams := types.DefaultAPIRevenueParams()
	revParams.PricePerInputToken = "1"
	revParams.PricePerOutputToken = "3"
	_ = k.SetAPIRevenueParams(ctx, revParams)

	msg := &types.MsgRecordAPIUsage{
		Bridge: testWallet2,
		Batches: []*types.APIUsageBatch{
			{APIKeyHash: testKeyHash, InputTokens: 1000, OutputTokens: 1000, RequestCount: 1},
			{APIKeyHash: testKeyHash2, InputTokens: 2000, OutputTokens: 500, RequestCount: 1},
		},
	}

	resp, err := k.RecordAPIUsage(ctx, msg)
	require.NoError(t, err)
	require.Equal(t, uint64(2), resp.BatchesProcessed)

	// Batch 1: (1000*1/1000) + (1000*3/1000) = 1 + 3 = 4
	// Batch 2: (2000*1/1000) + (500*3/1000) = 2 + 1 = 3
	// Total: 7 uzrn
	require.Equal(t, "7", resp.TotalDeducted)
}

// ─── Test: Zero balance blocks further API calls ───────────────────────────

func TestRecordAPIUsage_InsufficientBalance(t *testing.T) {
	k, ctx, _ := setupKeeperWithBank(t)

	// Create key but deposit only 1 uzrn
	_, _ = k.CreateAPIKey(ctx, &types.MsgCreateAPIKey{Owner: testWallet, KeyHash: testKeyHash})
	_, _ = k.DepositAPICredits(ctx, &types.MsgDepositAPICredits{Depositor: testWallet, Amount: "5"})

	revParams := types.DefaultAPIRevenueParams()
	revParams.PricePerInputToken = "1"
	revParams.PricePerOutputToken = "3"
	_ = k.SetAPIRevenueParams(ctx, revParams)

	// First request that costs more than balance — partial deduction
	msg := &types.MsgRecordAPIUsage{
		Bridge: testWallet2,
		Batches: []*types.APIUsageBatch{
			{APIKeyHash: testKeyHash, InputTokens: 10000, OutputTokens: 5000, RequestCount: 1},
		},
	}

	resp, err := k.RecordAPIUsage(ctx, msg)
	require.NoError(t, err)
	require.Equal(t, "5", resp.TotalDeducted) // Only 5 was available

	// Balance is now 0
	balance := k.GetAPIBalance(ctx, testWallet)
	require.Equal(t, "0", balance.Balance)

	// Next call should have 0 deduction (nothing to deduct)
	resp, err = k.RecordAPIUsage(ctx, msg)
	require.NoError(t, err)
	require.Equal(t, "0", resp.TotalDeducted)
}

// ─── Test: Revenue distribution splits correctly ───────────────────────────

func TestDistributeAPIRevenue(t *testing.T) {
	k, ctx, _ := setupKeeperWithBank(t)

	// Set API revenue params
	revParams := types.DefaultAPIRevenueParams()
	require.NoError(t, k.SetAPIRevenueParams(ctx, revParams))

	// Queue some revenue in epoch 0
	require.NoError(t, k.SetPendingAPIRevenue(ctx, 0, 10000))

	// Create an active sample for submitter distribution
	require.NoError(t, k.SetSample(ctx, &types.Sample{
		Id:           "sample1",
		Submitter:    testWallet,
		Status:       types.SampleStatus_SAMPLE_STATUS_GOLD,
		QualityTier:  "gold",
		FitnessScore: 800_000,
		AccessCount:  10,
	}))

	// Set a validator for infra distribution
	require.NoError(t, k.SetValidatorInfo(ctx, testWallet2))

	// Trigger distribution at block 100 (epoch 1 boundary)
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	newCtx := sdkCtx.WithBlockHeight(100)
	k.DistributeAPIRevenue(newCtx)

	// Verify pending revenue was cleared
	remaining := k.GetPendingAPIRevenue(newCtx, 0)
	require.Equal(t, uint64(0), remaining)
}

func TestAPIRevenueParamsValidation(t *testing.T) {
	validParams := types.DefaultAPIRevenueParams()
	require.NoError(t, validParams.Validate())

	// Total != 10000 should fail
	invalidParams := types.APIRevenueParams{
		TrainingShareBPS:  5000,
		InfraShareBPS:     2500,
		SubmitterShareBPS: 2000,
		ProtocolShareBPS:  1000,
		ResearchShareBPS:  1000, // sum = 11500, not 10000
	}
	require.Error(t, invalidParams.Validate())
}

// ─── Test: Model attribution traces revenue to correct TDUs ────────────────

func TestModelAttribution(t *testing.T) {
	k, ctx := setupKeeper(t)

	// Create a training record with model hash
	record := &types.TrainingRecord{
		Operator:           testWallet,
		AttestationHash:    "attest123",
		DatasetFingerprint: "dataset_fp",
		DatasetSize:        1000,
		BaseModel:          "zerone-8b",
		ModelHash:          "model_abc",
		BenchmarkScore:     0.92,
		BlockHeight:        50,
	}
	require.NoError(t, k.SetTrainingRecord(ctx, record))

	// Query attribution
	attrs := k.GetModelAttribution(ctx, "model_abc")
	require.NotEmpty(t, attrs)
}

// ─── Test: Withdrawal returns unused credits ───────────────────────────────

func TestWithdrawAPICredits(t *testing.T) {
	k, ctx, _ := setupKeeperWithBank(t)

	// Deposit first
	_, err := k.DepositAPICredits(ctx, &types.MsgDepositAPICredits{
		Depositor: testWallet,
		Amount:    "1000000",
	})
	require.NoError(t, err)

	// Withdraw half
	resp, err := k.WithdrawAPICredits(ctx, &types.MsgWithdrawAPICredits{
		Wallet: testWallet,
		Amount: "500000",
	})
	require.NoError(t, err)
	require.Equal(t, "500000", resp.RemainingBalance)

	// Verify balance
	balance := k.GetAPIBalance(ctx, testWallet)
	require.Equal(t, "500000", balance.Balance)
}

func TestWithdrawAPICredits_ExceedsBalance(t *testing.T) {
	k, ctx, _ := setupKeeperWithBank(t)

	// Deposit 1000
	_, _ = k.DepositAPICredits(ctx, &types.MsgDepositAPICredits{
		Depositor: testWallet,
		Amount:    "1000",
	})

	// Try to withdraw 2000
	_, err := k.WithdrawAPICredits(ctx, &types.MsgWithdrawAPICredits{
		Wallet: testWallet,
		Amount: "2000",
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "exceeds balance")
}

// ─── Test: Revoked key cannot be used ──────────────────────────────────────

func TestRevokeAPIKey(t *testing.T) {
	k, ctx := setupKeeper(t)

	// Create key
	_, err := k.CreateAPIKey(ctx, &types.MsgCreateAPIKey{
		Owner:   testWallet,
		KeyHash: testKeyHash,
	})
	require.NoError(t, err)

	// Revoke
	_, err = k.RevokeAPIKey(ctx, &types.MsgRevokeAPIKey{
		Owner:   testWallet,
		KeyHash: testKeyHash,
	})
	require.NoError(t, err)

	// Verify revoked
	record, found := k.GetAPIKeyRecord(ctx, testKeyHash)
	require.True(t, found)
	require.True(t, record.Revoked)

	// Revoking again should fail
	_, err = k.RevokeAPIKey(ctx, &types.MsgRevokeAPIKey{
		Owner:   testWallet,
		KeyHash: testKeyHash,
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "already revoked")
}

func TestRevokeAPIKey_WrongOwner(t *testing.T) {
	k, ctx := setupKeeper(t)

	_, err := k.CreateAPIKey(ctx, &types.MsgCreateAPIKey{
		Owner:   testWallet,
		KeyHash: testKeyHash,
	})
	require.NoError(t, err)

	// Try to revoke with different owner
	_, err = k.RevokeAPIKey(ctx, &types.MsgRevokeAPIKey{
		Owner:   testWallet2,
		KeyHash: testKeyHash,
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "mismatch")
}

func TestRevokedKeySkippedInUsage(t *testing.T) {
	k, ctx, _ := setupKeeperWithBank(t)

	// Create and revoke key
	_, _ = k.CreateAPIKey(ctx, &types.MsgCreateAPIKey{Owner: testWallet, KeyHash: testKeyHash})
	_, _ = k.DepositAPICredits(ctx, &types.MsgDepositAPICredits{Depositor: testWallet, Amount: "100000"})
	_, _ = k.RevokeAPIKey(ctx, &types.MsgRevokeAPIKey{Owner: testWallet, KeyHash: testKeyHash})

	revParams := types.DefaultAPIRevenueParams()
	revParams.PricePerInputToken = "1"
	revParams.PricePerOutputToken = "3"
	_ = k.SetAPIRevenueParams(ctx, revParams)

	// Usage with revoked key should be skipped
	resp, err := k.RecordAPIUsage(ctx, &types.MsgRecordAPIUsage{
		Bridge: testWallet2,
		Batches: []*types.APIUsageBatch{
			{APIKeyHash: testKeyHash, InputTokens: 1000, OutputTokens: 1000, RequestCount: 1},
		},
	})
	require.NoError(t, err)
	require.Equal(t, "0", resp.TotalDeducted)
	require.Equal(t, uint64(0), resp.BatchesProcessed)
}

// ─── Test: Usage epoch tracking ────────────────────────────────────────────

func TestAPIUsageRecordTracking(t *testing.T) {
	k, ctx, _ := setupKeeperWithBank(t)

	_, _ = k.CreateAPIKey(ctx, &types.MsgCreateAPIKey{Owner: testWallet, KeyHash: testKeyHash})
	_, _ = k.DepositAPICredits(ctx, &types.MsgDepositAPICredits{Depositor: testWallet, Amount: "100000"})

	revParams := types.DefaultAPIRevenueParams()
	revParams.PricePerInputToken = "1"
	revParams.PricePerOutputToken = "3"
	_ = k.SetAPIRevenueParams(ctx, revParams)

	_, _ = k.RecordAPIUsage(ctx, &types.MsgRecordAPIUsage{
		Bridge: testWallet2,
		Batches: []*types.APIUsageBatch{
			{APIKeyHash: testKeyHash, InputTokens: 2000, OutputTokens: 1000, RequestCount: 3, ModelUsed: "zerone-8b"},
		},
	})

	// Check epoch usage record (block 100 → epoch 1)
	usage := k.GetAPIUsageRecord(ctx, testWallet, 1)
	require.Equal(t, uint64(2000), usage.InputTokens)
	require.Equal(t, uint64(1000), usage.OutputTokens)
	require.Equal(t, uint64(3), usage.RequestCount)
	require.Equal(t, "zerone-8b", usage.ModelUsed)

	// Get all usage by wallet
	allUsage := k.GetAPIUsageByWallet(ctx, testWallet)
	require.Len(t, allUsage, 1)
}

// ─── Test: Message validation ──────────────────────────────────────────────

func TestMsgCreateAPIKey_ValidateBasic(t *testing.T) {
	// Valid
	msg := &types.MsgCreateAPIKey{Owner: testWallet, KeyHash: testKeyHash}
	require.NoError(t, msg.ValidateBasic())

	// Missing owner
	msg = &types.MsgCreateAPIKey{Owner: "", KeyHash: testKeyHash}
	require.Error(t, msg.ValidateBasic())

	// Missing key hash
	msg = &types.MsgCreateAPIKey{Owner: testWallet, KeyHash: ""}
	require.Error(t, msg.ValidateBasic())

	// Wrong key hash length
	msg = &types.MsgCreateAPIKey{Owner: testWallet, KeyHash: "tooshort"}
	require.Error(t, msg.ValidateBasic())
}

func TestMsgDepositAPICredits_ValidateBasic(t *testing.T) {
	msg := &types.MsgDepositAPICredits{Depositor: testWallet, Amount: "1000"}
	require.NoError(t, msg.ValidateBasic())

	msg = &types.MsgDepositAPICredits{Depositor: testWallet, Amount: "0"}
	require.Error(t, msg.ValidateBasic())
}

func TestMsgRecordAPIUsage_ValidateBasic(t *testing.T) {
	// Valid
	msg := &types.MsgRecordAPIUsage{
		Bridge: testWallet,
		Batches: []*types.APIUsageBatch{
			{APIKeyHash: testKeyHash, InputTokens: 100, OutputTokens: 50, RequestCount: 1},
		},
	}
	require.NoError(t, msg.ValidateBasic())

	// Empty batches
	msg = &types.MsgRecordAPIUsage{Bridge: testWallet, Batches: nil}
	require.Error(t, msg.ValidateBasic())

	// Zero tokens
	msg = &types.MsgRecordAPIUsage{
		Bridge: testWallet,
		Batches: []*types.APIUsageBatch{
			{APIKeyHash: testKeyHash, InputTokens: 0, OutputTokens: 0},
		},
	}
	require.Error(t, msg.ValidateBasic())
}

// ─── Test: Multiple keys per wallet ────────────────────────────────────────

func TestMultipleKeysPerWallet(t *testing.T) {
	k, ctx := setupKeeper(t)

	_, _ = k.CreateAPIKey(ctx, &types.MsgCreateAPIKey{Owner: testWallet, KeyHash: testKeyHash})
	_, _ = k.CreateAPIKey(ctx, &types.MsgCreateAPIKey{Owner: testWallet, KeyHash: testKeyHash2})

	keys := k.GetAPIKeysByWallet(ctx, testWallet)
	require.Len(t, keys, 2)
}

// ─── Test: API Revenue Params CRUD ─────────────────────────────────────────

func TestAPIRevenueParams_CRUD(t *testing.T) {
	k, ctx := setupKeeper(t)

	// Default params
	params := k.GetAPIRevenueParams(ctx)
	require.Equal(t, uint64(4000), params.TrainingShareBPS)

	// Custom params
	custom := types.APIRevenueParams{
		TrainingShareBPS:  3000,
		InfraShareBPS:     3000,
		SubmitterShareBPS: 2000,
		ProtocolShareBPS:  1000,
		ResearchShareBPS:  1000,
	}
	require.NoError(t, k.SetAPIRevenueParams(ctx, custom))

	params = k.GetAPIRevenueParams(ctx)
	require.Equal(t, uint64(3000), params.TrainingShareBPS)
	require.Equal(t, uint64(3000), params.InfraShareBPS)
}

// ─── Test: Iterate API keys and balances ────────────────────────────────────

func TestIterateAPIKeys(t *testing.T) {
	k, ctx := setupKeeper(t)

	_, _ = k.CreateAPIKey(ctx, &types.MsgCreateAPIKey{Owner: testWallet, KeyHash: testKeyHash})
	_, _ = k.CreateAPIKey(ctx, &types.MsgCreateAPIKey{Owner: testWallet2, KeyHash: testKeyHash2})

	var count int
	k.IterateAPIKeys(ctx, func(record *types.APIKeyRecord) bool {
		count++
		return false
	})
	require.Equal(t, 2, count)
}

func TestIterateAPIBalances(t *testing.T) {
	k, ctx, _ := setupKeeperWithBank(t)

	_, _ = k.DepositAPICredits(ctx, &types.MsgDepositAPICredits{Depositor: testWallet, Amount: "1000"})
	_, _ = k.DepositAPICredits(ctx, &types.MsgDepositAPICredits{Depositor: testWallet2, Amount: "2000"})

	var count int
	k.IterateAPIBalances(ctx, func(balance *types.APIBalance) bool {
		count++
		return false
	})
	require.Equal(t, 2, count)
}
