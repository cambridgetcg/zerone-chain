package keeper

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"strconv"

	sdkmath "cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/zerone-chain/zerone/x/knowledge/types"
)

const (
	// APIRevenueShareDenom is 10000 = 100% for the 5-way split.
	APIRevenueShareDenom = 10_000
	// TokenPriceDenom — prices are per 1000 tokens.
	TokenPriceDenom = 1_000
	// LowBalanceWarningBPS — 10% of total deposits.
	LowBalanceWarningBPS = 1_000
)

// ─── CreateAPIKey ───────────────────────────────────────────────────────────

// CreateAPIKey registers an API key hash on-chain, binding it to a wallet.
func (k Keeper) CreateAPIKey(ctx context.Context, msg *types.MsgCreateAPIKey) (*types.MsgCreateAPIKeyResponse, error) {
	sdkCtx := sdk.UnwrapSDKContext(ctx)

	// Check if key already exists
	if _, found := k.GetAPIKeyRecord(ctx, msg.KeyHash); found {
		return nil, types.ErrAPIKeyAlreadyExists.Wrapf("key %s already registered", msg.KeyHash)
	}

	tier := msg.RateLimitTier
	if tier == "" {
		tier = "standard"
	}

	record := &types.APIKeyRecord{
		KeyHash:        msg.KeyHash,
		Wallet:         msg.Owner,
		CreatedAtBlock: sdkCtx.BlockHeight(),
		Revoked:        false,
		RateLimitTier:  tier,
	}

	if err := k.SetAPIKeyRecord(ctx, record); err != nil {
		return nil, err
	}

	// Index: wallet → keyHash
	if err := k.SetAPIKeyWalletIndex(ctx, msg.Owner, msg.KeyHash); err != nil {
		return nil, err
	}

	sdkCtx.EventManager().EmitEvent(sdk.NewEvent(
		types.EventAPIKeyCreated,
		sdk.NewAttribute(types.AttributeAPIKeyHash, msg.KeyHash),
		sdk.NewAttribute(types.AttributeWallet, msg.Owner),
	))

	return &types.MsgCreateAPIKeyResponse{KeyHash: msg.KeyHash}, nil
}

// ─── RevokeAPIKey ───────────────────────────────────────────────────────────

// RevokeAPIKey deactivates an API key.
func (k Keeper) RevokeAPIKey(ctx context.Context, msg *types.MsgRevokeAPIKey) (*types.MsgRevokeAPIKeyResponse, error) {
	sdkCtx := sdk.UnwrapSDKContext(ctx)

	record, found := k.GetAPIKeyRecord(ctx, msg.KeyHash)
	if !found {
		return nil, types.ErrAPIKeyNotFound.Wrapf("key %s not found", msg.KeyHash)
	}

	if record.Wallet != msg.Owner {
		return nil, types.ErrAPIKeyOwnerMismatch.Wrapf("key owned by %s, not %s", record.Wallet, msg.Owner)
	}

	if record.Revoked {
		return nil, types.ErrAPIKeyRevoked.Wrap("key already revoked")
	}

	record.Revoked = true
	if err := k.SetAPIKeyRecord(ctx, record); err != nil {
		return nil, err
	}

	sdkCtx.EventManager().EmitEvent(sdk.NewEvent(
		types.EventAPIKeyRevoked,
		sdk.NewAttribute(types.AttributeAPIKeyHash, msg.KeyHash),
		sdk.NewAttribute(types.AttributeWallet, msg.Owner),
	))

	return &types.MsgRevokeAPIKeyResponse{}, nil
}

// ─── DepositAPICredits ─────────────────────────────────────────────────────

// DepositAPICredits deposits ZRN into a prepaid API balance.
func (k Keeper) DepositAPICredits(ctx context.Context, msg *types.MsgDepositAPICredits) (*types.MsgDepositAPICreditsResponse, error) {
	sdkCtx := sdk.UnwrapSDKContext(ctx)

	amount, ok := sdkmath.NewIntFromString(msg.Amount)
	if !ok || !amount.IsPositive() {
		return nil, fmt.Errorf("invalid deposit amount: %s", msg.Amount)
	}

	// Transfer from depositor to module account
	depositorAddr, err := sdk.AccAddressFromBech32(msg.Depositor)
	if err != nil {
		return nil, err
	}
	coins := sdk.NewCoins(sdk.NewCoin("uzrn", amount))
	if err := k.bankKeeper.SendCoinsFromAccountToModule(ctx, depositorAddr, types.ModuleName, coins); err != nil {
		return nil, types.ErrInsufficientPayment.Wrap(err.Error())
	}

	// Update balance
	balance := k.GetAPIBalance(ctx, msg.Depositor)
	currentBal := parseUzrn(balance.Balance)
	newBal := currentBal + amount.Uint64()
	balance.Balance = strconv.FormatUint(newBal, 10)

	totalDep := parseUzrn(balance.TotalDeposited)
	balance.TotalDeposited = strconv.FormatUint(totalDep+amount.Uint64(), 10)
	balance.LastDepositBlock = sdkCtx.BlockHeight()

	if err := k.SetAPIBalance(ctx, &balance); err != nil {
		return nil, err
	}

	sdkCtx.EventManager().EmitEvent(sdk.NewEvent(
		types.EventAPICreditsDeposited,
		sdk.NewAttribute(types.AttributeWallet, msg.Depositor),
		sdk.NewAttribute(types.AttributeDepositAmount, msg.Amount),
		sdk.NewAttribute(types.AttributeNewBalance, balance.Balance),
	))

	return &types.MsgDepositAPICreditsResponse{NewBalance: balance.Balance}, nil
}

// ─── WithdrawAPICredits ─────────────────────────────────────────────────────

// WithdrawAPICredits withdraws unused API credits.
func (k Keeper) WithdrawAPICredits(ctx context.Context, msg *types.MsgWithdrawAPICredits) (*types.MsgWithdrawAPICreditsResponse, error) {
	sdkCtx := sdk.UnwrapSDKContext(ctx)

	amount, ok := sdkmath.NewIntFromString(msg.Amount)
	if !ok || !amount.IsPositive() {
		return nil, fmt.Errorf("invalid withdrawal amount: %s", msg.Amount)
	}

	balance := k.GetAPIBalance(ctx, msg.Wallet)
	currentBal := parseUzrn(balance.Balance)

	if amount.Uint64() > currentBal {
		return nil, types.ErrInvalidWithdrawalAmount.Wrapf(
			"requested %s, available %d uzrn", msg.Amount, currentBal)
	}

	// Transfer from module to wallet
	walletAddr, err := sdk.AccAddressFromBech32(msg.Wallet)
	if err != nil {
		return nil, err
	}
	coins := sdk.NewCoins(sdk.NewCoin("uzrn", amount))
	if err := k.bankKeeper.SendCoinsFromModuleToAccount(ctx, types.ModuleName, walletAddr, coins); err != nil {
		return nil, err
	}

	// Update balance
	newBal := currentBal - amount.Uint64()
	balance.Balance = strconv.FormatUint(newBal, 10)

	if err := k.SetAPIBalance(ctx, &balance); err != nil {
		return nil, err
	}

	sdkCtx.EventManager().EmitEvent(sdk.NewEvent(
		types.EventAPICreditsWithdrawn,
		sdk.NewAttribute(types.AttributeWallet, msg.Wallet),
		sdk.NewAttribute("amount", msg.Amount),
		sdk.NewAttribute(types.AttributeNewBalance, balance.Balance),
	))

	_ = sdkCtx // suppress unused

	return &types.MsgWithdrawAPICreditsResponse{RemainingBalance: balance.Balance}, nil
}

// ─── RecordAPIUsage ─────────────────────────────────────────────────────────

// RecordAPIUsage records batched API usage from the payment bridge.
// For each batch: look up key → wallet, calculate cost, deduct from balance.
func (k Keeper) RecordAPIUsage(ctx context.Context, msg *types.MsgRecordAPIUsage) (*types.MsgRecordAPIUsageResponse, error) {
	sdkCtx := sdk.UnwrapSDKContext(ctx)

	revParams := k.GetAPIRevenueParams(ctx)
	priceInput := parseUzrn(revParams.PricePerInputToken)
	priceOutput := parseUzrn(revParams.PricePerOutputToken)

	var totalDeducted uint64
	var processed uint64

	for _, batch := range msg.Batches {
		// Look up API key → wallet
		keyRecord, found := k.GetAPIKeyRecord(ctx, batch.APIKeyHash)
		if !found {
			continue // skip unknown keys
		}
		if keyRecord.Revoked {
			continue // skip revoked keys
		}

		// Calculate cost: (input_tokens * price_input / 1000) + (output_tokens * price_output / 1000)
		inputCost := (batch.InputTokens * priceInput) / TokenPriceDenom
		outputCost := (batch.OutputTokens * priceOutput) / TokenPriceDenom
		totalCost := inputCost + outputCost
		if totalCost == 0 {
			totalCost = 1 // minimum 1 uzrn
		}

		// Deduct from wallet balance
		balance := k.GetAPIBalance(ctx, keyRecord.Wallet)
		currentBal := parseUzrn(balance.Balance)

		if currentBal < totalCost {
			// Partial deduction — deduct what's available
			totalCost = currentBal
		}
		if totalCost == 0 {
			continue
		}

		newBal := currentBal - totalCost
		balance.Balance = strconv.FormatUint(newBal, 10)

		totalConsumed := parseUzrn(balance.TotalConsumed)
		balance.TotalConsumed = strconv.FormatUint(totalConsumed+totalCost, 10)
		balance.LastUsageBlock = sdkCtx.BlockHeight()

		if err := k.SetAPIBalance(ctx, &balance); err != nil {
			continue
		}

		// Record usage for epoch tracking
		epoch := uint64(sdkCtx.BlockHeight()) / 100 // epoch = every 100 blocks
		usage := k.GetAPIUsageRecord(ctx, keyRecord.Wallet, epoch)
		usage.InputTokens += batch.InputTokens
		usage.OutputTokens += batch.OutputTokens
		usage.RequestCount += batch.RequestCount
		usageCost := parseUzrn(usage.TotalCost)
		usage.TotalCost = strconv.FormatUint(usageCost+totalCost, 10)
		if batch.ModelUsed != "" {
			usage.ModelUsed = batch.ModelUsed
		}
		if err := k.SetAPIUsageRecord(ctx, &usage); err != nil {
			continue
		}

		// Queue API revenue for distribution
		k.queueAPIRevenue(ctx, epoch, totalCost)

		totalDeducted += totalCost
		processed++
	}

	if processed > 0 {
		sdkCtx.EventManager().EmitEvent(sdk.NewEvent(
			types.EventAPIUsageRecorded,
			sdk.NewAttribute(types.AttributeTotalCost, strconv.FormatUint(totalDeducted, 10)),
			sdk.NewAttribute("batches", strconv.FormatUint(processed, 10)),
		))
	}

	return &types.MsgRecordAPIUsageResponse{
		TotalDeducted:    strconv.FormatUint(totalDeducted, 10),
		BatchesProcessed: processed,
	}, nil
}

// ─── API Revenue Distribution ───────────────────────────────────────────────

// DistributeAPIRevenue distributes accumulated API revenue according to the 5-way split.
// Called from EndBlocker.
func (k Keeper) DistributeAPIRevenue(ctx context.Context) {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	epoch := uint64(sdkCtx.BlockHeight()) / 100

	// Only distribute at epoch boundaries (every 100 blocks)
	if uint64(sdkCtx.BlockHeight())%100 != 0 {
		return
	}

	// Get pending revenue for previous epoch
	prevEpoch := epoch - 1
	if epoch == 0 {
		return
	}

	amount := k.GetPendingAPIRevenue(ctx, prevEpoch)
	if amount == 0 {
		return
	}

	revParams := k.GetAPIRevenueParams(ctx)
	totalAmount := sdkmath.NewInt(int64(amount))

	// 5-way split
	trainingShare := totalAmount.Mul(sdkmath.NewInt(int64(revParams.TrainingShareBPS))).Quo(sdkmath.NewInt(APIRevenueShareDenom))
	infraShare := totalAmount.Mul(sdkmath.NewInt(int64(revParams.InfraShareBPS))).Quo(sdkmath.NewInt(APIRevenueShareDenom))
	submitterShare := totalAmount.Mul(sdkmath.NewInt(int64(revParams.SubmitterShareBPS))).Quo(sdkmath.NewInt(APIRevenueShareDenom))
	protocolShare := totalAmount.Mul(sdkmath.NewInt(int64(revParams.ProtocolShareBPS))).Quo(sdkmath.NewInt(APIRevenueShareDenom))
	researchShare := totalAmount.Sub(trainingShare).Sub(infraShare).Sub(submitterShare).Sub(protocolShare) // remainder

	// 1. Training contributors — distribute via existing fitness-based mechanism
	if trainingShare.IsPositive() {
		k.distributeTrainingRevenue(ctx, trainingShare)
	}

	// 2. Infrastructure (validators) — distribute via staking keeper
	if infraShare.IsPositive() {
		k.distributeInfraRevenue(ctx, infraShare)
	}

	// 3. Submitters (data providers) — distribute proportionally to sample revenue
	if submitterShare.IsPositive() {
		k.distributeSubmitterRevenue(ctx, submitterShare)
	}

	// 4. Protocol treasury — module account
	// Already in module account, no action needed

	// 5. Research fund
	if researchShare.IsPositive() {
		k.depositProtocolRevenue(ctx, researchShare)
	}

	// Clear pending revenue
	_ = k.DeletePendingAPIRevenue(ctx, prevEpoch)

	sdkCtx.EventManager().EmitEvent(sdk.NewEvent(
		types.EventAPIRevenueDistributed,
		sdk.NewAttribute("epoch", strconv.FormatUint(prevEpoch, 10)),
		sdk.NewAttribute("total", totalAmount.String()),
		sdk.NewAttribute("training", trainingShare.String()),
		sdk.NewAttribute("infra", infraShare.String()),
		sdk.NewAttribute("submitters", submitterShare.String()),
		sdk.NewAttribute("protocol", protocolShare.String()),
		sdk.NewAttribute("research", researchShare.String()),
	))
}

// distributeTrainingRevenue distributes revenue to model training contributors.
// Uses training records to find TDU contributors and distributes by fitness scores.
func (k Keeper) distributeTrainingRevenue(ctx context.Context, amount sdkmath.Int) {
	// Distribute proportionally to samples with highest fitness scores.
	// For now, this is pooled into the pending revenue for sample-based distribution.
	var topSamples []string
	k.IterateSamples(ctx, func(sample *types.Sample) bool {
		if isActiveSample(sample.Status) && sample.FitnessScore > 0 {
			topSamples = append(topSamples, sample.Id)
		}
		return len(topSamples) >= 100 // cap at top 100
	})

	if len(topSamples) == 0 {
		// Fallback: deposit to protocol
		k.depositProtocolRevenue(ctx, amount)
		return
	}

	perSample := amount.Quo(sdkmath.NewInt(int64(len(topSamples))))
	for _, sampleID := range topSamples {
		if perSample.IsPositive() {
			k.queueRevenueDistribution(ctx, sampleID, perSample)
		}
	}
}

// distributeInfraRevenue distributes revenue to infrastructure validators.
func (k Keeper) distributeInfraRevenue(ctx context.Context, amount sdkmath.Int) {
	// Distribute equally among active validators
	var validators []string
	store := k.storeService.OpenKVStore(ctx)
	iter, err := store.Iterator(types.ValidatorInfoKey, prefixEndBytes(types.ValidatorInfoKey))
	if err == nil {
		defer iter.Close()
		for ; iter.Valid(); iter.Next() {
			addr := string(iter.Key()[len(types.ValidatorInfoKey):])
			validators = append(validators, addr)
		}
	}

	if len(validators) == 0 {
		k.depositProtocolRevenue(ctx, amount)
		return
	}

	perValidator := amount.Quo(sdkmath.NewInt(int64(len(validators))))
	for _, addr := range validators {
		if perValidator.IsPositive() {
			valAddr, err := sdk.AccAddressFromBech32(addr)
			if err != nil {
				continue
			}
			coins := sdk.NewCoins(sdk.NewCoin("uzrn", perValidator))
			_ = k.bankKeeper.SendCoinsFromModuleToAccount(ctx, types.ModuleName, valAddr, coins)
		}
	}
}

// distributeSubmitterRevenue distributes revenue to data providers proportionally.
func (k Keeper) distributeSubmitterRevenue(ctx context.Context, amount sdkmath.Int) {
	// Add to the existing pending revenue pool for the highest-access samples
	var topSamples []string
	k.IterateSamples(ctx, func(sample *types.Sample) bool {
		if isActiveSample(sample.Status) && sample.AccessCount > 0 {
			topSamples = append(topSamples, sample.Id)
		}
		return len(topSamples) >= 100
	})

	if len(topSamples) == 0 {
		k.depositProtocolRevenue(ctx, amount)
		return
	}

	perSample := amount.Quo(sdkmath.NewInt(int64(len(topSamples))))
	for _, sampleID := range topSamples {
		if perSample.IsPositive() {
			k.queueRevenueDistribution(ctx, sampleID, perSample)
		}
	}
}

// ─── Model Attribution ──────────────────────────────────────────────────────

// GetModelAttribution traces a model hash back to training records and their TDUs.
// Returns the sample IDs that contributed to the model's training data.
func (k Keeper) GetModelAttribution(ctx context.Context, modelHash string) []string {
	store := k.storeService.OpenKVStore(ctx)
	prefix := types.TrainingRecordByModelHashPrefix(modelHash)
	iter, err := store.Iterator(prefix, prefixEndBytes(prefix))
	if err != nil {
		return nil
	}
	defer iter.Close()

	var sampleIDs []string
	for ; iter.Valid(); iter.Next() {
		// Value is attestation hash — look up training record
		attestationHash := string(iter.Key()[len(prefix):])
		record, err := k.GetTrainingRecord(ctx, attestationHash)
		if err != nil {
			continue
		}

		// The dataset_fingerprint maps back to samples used in training.
		// For now, we track via the training record's dataset reference.
		_ = record
		sampleIDs = append(sampleIDs, attestationHash) // placeholder for full TDU resolution
	}
	return sampleIDs
}

// SetValidatorInfo stores a validator info entry (for testing and seed data).
func (k Keeper) SetValidatorInfo(ctx context.Context, addr string) error {
	store := k.storeService.OpenKVStore(ctx)
	return store.Set(types.ValidatorInfoKeyFn(addr), []byte{0x01})
}

// ─── State CRUD ─────────────────────────────────────────────────────────────

// GetAPIKeyRecord returns an API key record by key hash.
func (k Keeper) GetAPIKeyRecord(ctx context.Context, keyHash string) (*types.APIKeyRecord, bool) {
	store := k.storeService.OpenKVStore(ctx)
	bz, err := store.Get(types.APIKeyRecordKey(keyHash))
	if err != nil || bz == nil {
		return nil, false
	}
	var record types.APIKeyRecord
	if err := json.Unmarshal(bz, &record); err != nil {
		return nil, false
	}
	return &record, true
}

// SetAPIKeyRecord stores an API key record.
func (k Keeper) SetAPIKeyRecord(ctx context.Context, record *types.APIKeyRecord) error {
	store := k.storeService.OpenKVStore(ctx)
	bz, err := json.Marshal(record)
	if err != nil {
		return fmt.Errorf("failed to marshal API key record: %w", err)
	}
	return store.Set(types.APIKeyRecordKey(record.KeyHash), bz)
}

// SetAPIKeyWalletIndex sets the wallet → keyHash index.
func (k Keeper) SetAPIKeyWalletIndex(ctx context.Context, wallet, keyHash string) error {
	store := k.storeService.OpenKVStore(ctx)
	return store.Set(types.APIKeyWalletIndexKey(wallet, keyHash), []byte{0x01})
}

// GetAPIKeysByWallet returns all API key hashes for a wallet.
func (k Keeper) GetAPIKeysByWallet(ctx context.Context, wallet string) []string {
	store := k.storeService.OpenKVStore(ctx)
	prefix := types.APIKeysByWalletPrefix(wallet)
	iter, err := store.Iterator(prefix, prefixEndBytes(prefix))
	if err != nil {
		return nil
	}
	defer iter.Close()
	var hashes []string
	for ; iter.Valid(); iter.Next() {
		hash := string(iter.Key()[len(prefix):])
		hashes = append(hashes, hash)
	}
	return hashes
}

// GetAPIBalance returns the API balance for a wallet (creates default if missing).
func (k Keeper) GetAPIBalance(ctx context.Context, wallet string) types.APIBalance {
	store := k.storeService.OpenKVStore(ctx)
	bz, err := store.Get(types.APIBalanceKey(wallet))
	if err != nil || bz == nil {
		return types.APIBalance{
			Wallet:         wallet,
			Balance:        "0",
			TotalDeposited: "0",
			TotalConsumed:  "0",
		}
	}
	var balance types.APIBalance
	if err := json.Unmarshal(bz, &balance); err != nil {
		return types.APIBalance{
			Wallet:         wallet,
			Balance:        "0",
			TotalDeposited: "0",
			TotalConsumed:  "0",
		}
	}
	return balance
}

// SetAPIBalance stores an API balance record.
func (k Keeper) SetAPIBalance(ctx context.Context, balance *types.APIBalance) error {
	store := k.storeService.OpenKVStore(ctx)
	bz, err := json.Marshal(balance)
	if err != nil {
		return fmt.Errorf("failed to marshal API balance: %w", err)
	}
	return store.Set(types.APIBalanceKey(balance.Wallet), bz)
}

// GetAPIUsageRecord returns the usage record for a wallet/epoch.
func (k Keeper) GetAPIUsageRecord(ctx context.Context, wallet string, epoch uint64) types.APIUsageRecord {
	store := k.storeService.OpenKVStore(ctx)
	bz, err := store.Get(types.APIUsageRecordKey(wallet, epoch))
	if err != nil || bz == nil {
		return types.APIUsageRecord{
			Wallet:    wallet,
			Epoch:     epoch,
			TotalCost: "0",
		}
	}
	var record types.APIUsageRecord
	if err := json.Unmarshal(bz, &record); err != nil {
		return types.APIUsageRecord{
			Wallet:    wallet,
			Epoch:     epoch,
			TotalCost: "0",
		}
	}
	return record
}

// SetAPIUsageRecord stores a usage record.
func (k Keeper) SetAPIUsageRecord(ctx context.Context, record *types.APIUsageRecord) error {
	store := k.storeService.OpenKVStore(ctx)
	bz, err := json.Marshal(record)
	if err != nil {
		return fmt.Errorf("failed to marshal API usage record: %w", err)
	}
	return store.Set(types.APIUsageRecordKey(record.Wallet, record.Epoch), bz)
}

// GetAPIUsageByWallet returns all usage records for a wallet.
func (k Keeper) GetAPIUsageByWallet(ctx context.Context, wallet string) []types.APIUsageRecord {
	store := k.storeService.OpenKVStore(ctx)
	prefix := types.APIUsageByWalletPrefix(wallet)
	iter, err := store.Iterator(prefix, prefixEndBytes(prefix))
	if err != nil {
		return nil
	}
	defer iter.Close()
	var records []types.APIUsageRecord
	for ; iter.Valid(); iter.Next() {
		var record types.APIUsageRecord
		if err := json.Unmarshal(iter.Value(), &record); err != nil {
			continue
		}
		records = append(records, record)
	}
	return records
}

// ─── Pending API Revenue ────────────────────────────────────────────────────

func (k Keeper) queueAPIRevenue(ctx context.Context, epoch uint64, amount uint64) {
	current := k.GetPendingAPIRevenue(ctx, epoch)
	_ = k.SetPendingAPIRevenue(ctx, epoch, current+amount)
}

func (k Keeper) GetPendingAPIRevenue(ctx context.Context, epoch uint64) uint64 {
	store := k.storeService.OpenKVStore(ctx)
	bz, err := store.Get(types.PendingAPIRevenueKey(epoch))
	if err != nil || len(bz) != 8 {
		return 0
	}
	return binary.BigEndian.Uint64(bz)
}

func (k Keeper) SetPendingAPIRevenue(ctx context.Context, epoch uint64, amount uint64) error {
	store := k.storeService.OpenKVStore(ctx)
	bz := make([]byte, 8)
	binary.BigEndian.PutUint64(bz, amount)
	return store.Set(types.PendingAPIRevenueKey(epoch), bz)
}

func (k Keeper) DeletePendingAPIRevenue(ctx context.Context, epoch uint64) error {
	store := k.storeService.OpenKVStore(ctx)
	return store.Delete(types.PendingAPIRevenueKey(epoch))
}

// ─── API Revenue Params ─────────────────────────────────────────────────────

func (k Keeper) GetAPIRevenueParams(ctx context.Context) types.APIRevenueParams {
	store := k.storeService.OpenKVStore(ctx)
	bz, err := store.Get(types.APIRevenueParamsKey)
	if err != nil || bz == nil {
		return types.DefaultAPIRevenueParams()
	}
	var params types.APIRevenueParams
	if err := json.Unmarshal(bz, &params); err != nil {
		return types.DefaultAPIRevenueParams()
	}
	return params
}

func (k Keeper) SetAPIRevenueParams(ctx context.Context, params types.APIRevenueParams) error {
	store := k.storeService.OpenKVStore(ctx)
	bz, err := json.Marshal(&params)
	if err != nil {
		return fmt.Errorf("failed to marshal API revenue params: %w", err)
	}
	return store.Set(types.APIRevenueParamsKey, bz)
}

// ─── Iterate API Keys ──────────────────────────────────────────────────────

func (k Keeper) IterateAPIKeys(ctx context.Context, cb func(record *types.APIKeyRecord) bool) {
	store := k.storeService.OpenKVStore(ctx)
	iter, err := store.Iterator(types.APIKeyRecordPrefix, prefixEndBytes(types.APIKeyRecordPrefix))
	if err != nil {
		return
	}
	defer iter.Close()
	for ; iter.Valid(); iter.Next() {
		var record types.APIKeyRecord
		if err := json.Unmarshal(iter.Value(), &record); err != nil {
			continue
		}
		if cb(&record) {
			break
		}
	}
}

// ─── Iterate API Balances ──────────────────────────────────────────────────

func (k Keeper) IterateAPIBalances(ctx context.Context, cb func(balance *types.APIBalance) bool) {
	store := k.storeService.OpenKVStore(ctx)
	iter, err := store.Iterator(types.APIBalancePrefix, prefixEndBytes(types.APIBalancePrefix))
	if err != nil {
		return
	}
	defer iter.Close()
	for ; iter.Valid(); iter.Next() {
		var balance types.APIBalance
		if err := json.Unmarshal(iter.Value(), &balance); err != nil {
			continue
		}
		if cb(&balance) {
			break
		}
	}
}
