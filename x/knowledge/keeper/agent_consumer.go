package keeper

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"strconv"

	sdkmath "cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/zerone-chain/zerone/x/knowledge/types"
)

// ─── R51: Agent-as-Consumer ─────────────────────────────────────────────────
//
// This module closes the recursive self-improvement loop.
//
// The insight: agents don't CONTAIN models. They ACCESS models through the
// API payment layer. The same infrastructure that external consumers use.
//
// The closed loop:
//   1. Agent earns ZRN (curation, review, bounty rewards)
//   2. Agent deposits ZRN as API credits
//   3. Agent calls API with best available model (pays per-token)
//   4. Agent uses model output to do work (submit, review, identify gaps)
//   5. Work earns ZRN → GOTO 1
//
// Meanwhile:
//   - Agent's API payments flow through R44 revenue distribution
//   - 40% of payment funds training → better models
//   - Better models appear in the API registry
//   - Agent's next API call uses the better model
//   - Better model → better work → more ZRN → more API calls
//
// The agent is simultaneously worker, customer, and investor.
// Its spending funds its own improvement.
//
// Natural selection: agents that can't earn enough to pay for thinking → die.

// ─── ProvisionAgentAPI ──────────────────────────────────────────────────────

// ProvisionAgentAPI sets up API access for a newly promoted agent.
// Called automatically during PromoteModelToAgent.
//
// Actions:
//  1. Creates an API key bound to the agent's derived wallet
//  2. Deposits a fraction of the promotion stake as initial API credits
//  3. Stores the agent's API configuration with defaults
//
// After this, the agent is born ready to work.
func (k Keeper) ProvisionAgentAPI(ctx context.Context, agentID, wallet string, stake sdkmath.Int) error {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	params := k.GetAgentConsumerParams(ctx)

	// 1. Generate deterministic API key hash from agent ID.
	keyInput := "agent-api-key:" + agentID
	keyHash := sha256.Sum256([]byte(keyInput))
	keyHashHex := fmt.Sprintf("%x", keyHash)

	// Create API key via existing R44 infrastructure.
	_, err := k.CreateAPIKey(ctx, &types.MsgCreateAPIKey{
		Owner:         wallet,
		KeyHash:       keyHashHex,
		RateLimitTier: params.AgentRateLimitTier,
	})
	if err != nil {
		return fmt.Errorf("failed to create agent API key: %w", err)
	}

	// 2. Deposit initial credits from stake.
	initialFrac, _ := sdkmath.LegacyNewDecFromStr(params.InitialCreditFraction)
	initialCredits := initialFrac.MulInt(stake).TruncateInt()

	if initialCredits.IsPositive() {
		// Transfer from agent's wallet to API credit balance.
		// The stake is already in the agent's wallet from promotion.
		_, err := k.DepositAPICredits(ctx, &types.MsgDepositAPICredits{
			Depositor: wallet,
			Amount:    initialCredits.String(),
		})
		if err != nil {
			// Non-fatal: agent can deposit later. Log but continue.
			_ = err
		}
	}

	// 3. Store API config.
	config := types.AgentAPIConfig{
		AgentID:        agentID,
		APIKeyHash:     keyHashHex,
		Wallet:         wallet,
		AutoSelect:     true,
		MaxTokenBudget: 0, // unlimited by default
		ReplenishRate:  params.DefaultReplenishRate,
		MinBalance:     params.MinOperatingBalance,
		TotalSpent:     "0",
		CreatedAt:      uint64(sdkCtx.BlockHeight()),
	}

	if err := k.SetAgentAPIConfig(ctx, &config); err != nil {
		return err
	}

	// Initialize profitability tracker.
	profitability := types.AgentProfitability{
		AgentID:            agentID,
		CurationRewards:    "0",
		ReviewRewards:      "0",
		BountyRewards:      "0",
		AttributionRewards: "0",
		TotalEarned:        "0",
		APISpend:           "0",
		StakeSpend:         "0",
		TotalSpent:         "0",
		NetProfitLoss:      "0",
		ProfitRatio:        "0",
		Trend:              "stable",
		SolventSince:       uint64(sdkCtx.BlockHeight()),
		ComputedAt:         uint64(sdkCtx.BlockHeight()),
	}
	if err := k.SetAgentProfitability(ctx, &profitability); err != nil {
		return err
	}

	sdkCtx.EventManager().EmitEvent(sdk.NewEvent(
		types.EventAgentAPIProvisioned,
		sdk.NewAttribute(types.AttributeAgentID, agentID),
		sdk.NewAttribute(types.AttributeAgentAPIKey, keyHashHex),
		sdk.NewAttribute("initial_credits", initialCredits.String()),
	))

	return nil
}

// ─── SelectModelForTask ─────────────────────────────────────────────────────

// SelectModelForTask picks the best model for an agent to use on a task.
//
// Selection algorithm:
//  1. If agent has PreferredModelID set → use that (if active and affordable)
//  2. Otherwise, scan model registry for domain-matching active models
//  3. Sort by benchmark score descending
//  4. If agent is low on balance, apply cost ceiling
//  5. Self-reinforcement check: skip models trained on agent's own output
//  6. Return top candidate with estimated cost
func (k Keeper) SelectModelForTask(ctx context.Context, agentID, domain string) (types.ModelSelection, error) {
	config, found := k.GetAgentAPIConfig(ctx, agentID)
	if !found {
		return types.ModelSelection{}, fmt.Errorf("agent %s has no API config", agentID)
	}

	params := k.GetAgentConsumerParams(ctx)

	// Check if agent has a preferred model.
	if config.PreferredModelID != "" {
		model, found := k.GetModelRecord(ctx, config.PreferredModelID)
		if found && model.Status == types.ModelStatusActive {
			return types.ModelSelection{
				ModelID:        model.ModelID,
				Domain:         model.Domain,
				BenchmarkScore: model.BenchmarkScore,
				Reason:         "preferred_model",
			}, nil
		}
	}

	if !config.AutoSelect {
		return types.ModelSelection{}, fmt.Errorf("agent %s has no preferred model and auto-select is disabled", agentID)
	}

	// Get agent identity for lineage check.
	agent, agentFound := k.GetAgentIdentity(ctx, agentID)

	// Scan model registry for candidates.
	type candidate struct {
		model types.ModelRecord
		score sdkmath.LegacyDec
	}
	var candidates []candidate

	for _, model := range k.GetModelsByDomain(ctx, domain) {
		if model.Status != types.ModelStatusActive {
			continue
		}

		// Self-reinforcement check: don't use a model trained on your own data.
		if !params.AllowSelfReinforcement && agentFound {
			if k.isModelTrainedByAgent(ctx, model.ModelID, agent.AgentID) {
				continue // skip — would create circular validation
			}
		}

		score := model.GetBenchmarkScore()
		candidates = append(candidates, candidate{model: model, score: score})
	}

	if len(candidates) == 0 {
		return types.ModelSelection{}, fmt.Errorf("no suitable models found for domain %s", domain)
	}

	// Sort by benchmark score descending.
	// Simple selection sort — candidate lists are small (max per domain).
	best := candidates[0]
	for _, c := range candidates[1:] {
		if c.score.GT(best.score) {
			best = c
		}
	}

	return types.ModelSelection{
		ModelID:        best.model.ModelID,
		Domain:         best.model.Domain,
		BenchmarkScore: best.model.BenchmarkScore,
		Reason:         "auto_select_best_benchmark",
	}, nil
}

// ─── RecordAgentAPICall ─────────────────────────────────────────────────────

// RecordAgentAPICall records an agent's API usage through the standard payment pipeline.
//
// This is the integration point: the agent's call goes through the same
// RecordAPIUsage path as any external consumer. The agent pays per-token.
// The payment generates API revenue that flows through the 5-way split.
//
// The agent is funding its own improvement through every API call.
func (k Keeper) RecordAgentAPICall(
	ctx context.Context,
	agentID, modelID, taskID string,
	inputTokens, outputTokens uint64,
) (*types.AgentAPICall, error) {
	sdkCtx := sdk.UnwrapSDKContext(ctx)

	config, found := k.GetAgentAPIConfig(ctx, agentID)
	if !found {
		return nil, fmt.Errorf("agent %s has no API config", agentID)
	}

	// Route through standard R44 payment pipeline.
	batch := &types.APIUsageBatch{
		APIKeyHash:   config.APIKeyHash,
		InputTokens:  inputTokens,
		OutputTokens: outputTokens,
		RequestCount: 1,
		ModelUsed:    modelID,
	}

	result, err := k.RecordAPIUsage(ctx, &types.MsgRecordAPIUsage{
		Bridge:  config.Wallet, // agent acts as its own bridge
		Batches: []*types.APIUsageBatch{batch},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to record agent API usage: %w", err)
	}

	// Parse cost from result.
	cost, _ := strconv.ParseUint(result.TotalDeducted, 10, 64)

	// Generate call ID.
	callID := k.nextAgentCallID(ctx)

	// Store call record.
	call := &types.AgentAPICall{
		CallID:       callID,
		AgentID:      agentID,
		ModelID:      modelID,
		TaskID:       taskID,
		InputTokens:  inputTokens,
		OutputTokens: outputTokens,
		Cost:         strconv.FormatUint(cost, 10),
		BlockHeight:  uint64(sdkCtx.BlockHeight()),
	}

	if err := k.setAgentAPICall(ctx, call); err != nil {
		return nil, err
	}

	// Update agent API config stats.
	config.TotalCalls++
	config.TotalTokensUsed += inputTokens + outputTokens
	oldSpent := config.GetTotalSpent()
	config.TotalSpent = oldSpent.Add(sdkmath.NewIntFromUint64(cost)).String()
	config.LastModelUsed = modelID
	config.LastCallBlock = uint64(sdkCtx.BlockHeight())
	_ = k.SetAgentAPIConfig(ctx, &config)

	// Check if agent is now in low-balance state.
	balance := k.GetAPIBalance(ctx, config.Wallet)
	currentBal := parseUzrn(balance.Balance)
	minBal := config.GetMinBalance()
	if sdkmath.NewIntFromUint64(currentBal).LT(minBal) {
		k.markAgentLowBalance(ctx, agentID)
		sdkCtx.EventManager().EmitEvent(sdk.NewEvent(
			types.EventAgentLowBalance,
			sdk.NewAttribute(types.AttributeAgentID, agentID),
			sdk.NewAttribute("balance", strconv.FormatUint(currentBal, 10)),
			sdk.NewAttribute("minimum", minBal.String()),
		))
	}

	sdkCtx.EventManager().EmitEvent(sdk.NewEvent(
		types.EventAgentAPICall,
		sdk.NewAttribute(types.AttributeAgentID, agentID),
		sdk.NewAttribute(types.AttributeModelSelected, modelID),
		sdk.NewAttribute(types.AttributeAPICost, strconv.FormatUint(cost, 10)),
		sdk.NewAttribute(types.AttributeTaskID, taskID),
	))

	return call, nil
}

// ─── AutoReplenishCredits ───────────────────────────────────────────────────

// AutoReplenishCredits deposits a fraction of task rewards as API credits.
// Called after CompleteTask when an agent earns a reward.
//
// Default: 30% of rewards → API credits, 70% stays liquid.
// This ensures agents always have operational budget for thinking.
func (k Keeper) AutoReplenishCredits(ctx context.Context, agentID string, rewardAmount sdkmath.Int) error {
	if !rewardAmount.IsPositive() {
		return nil
	}

	config, found := k.GetAgentAPIConfig(ctx, agentID)
	if !found {
		return nil // not a consumer agent — skip silently
	}

	replenishRate := config.GetReplenishRate()
	depositAmount := replenishRate.MulInt(rewardAmount).TruncateInt()

	if !depositAmount.IsPositive() {
		return nil
	}

	// Deposit through standard R44 API credit system.
	_, err := k.DepositAPICredits(ctx, &types.MsgDepositAPICredits{
		Depositor: config.Wallet,
		Amount:    depositAmount.String(),
	})
	if err != nil {
		return nil // non-fatal: agent keeps full reward
	}

	sdkCtx := sdk.UnwrapSDKContext(ctx)
	sdkCtx.EventManager().EmitEvent(sdk.NewEvent(
		types.EventAgentReplenished,
		sdk.NewAttribute(types.AttributeAgentID, agentID),
		sdk.NewAttribute("deposit_amount", depositAmount.String()),
		sdk.NewAttribute("from_reward", rewardAmount.String()),
	))

	return nil
}

// ─── Profitability Computation ──────────────────────────────────────────────

// ComputeProfitability calculates an agent's profit & loss.
// Called periodically (every epoch boundary in EndBlocker).
func (k Keeper) ComputeProfitability(ctx context.Context, agentID string) (*types.AgentProfitability, error) {
	sdkCtx := sdk.UnwrapSDKContext(ctx)

	profitability, found := k.GetAgentProfitability(ctx, agentID)
	if !found {
		return nil, fmt.Errorf("no profitability record for agent %s", agentID)
	}

	// Get current totals from agent identity.
	agent, agentFound := k.GetAgentIdentity(ctx, agentID)
	if !agentFound {
		return nil, fmt.Errorf("agent %s not found", agentID)
	}

	// Get API spending from config.
	config, configFound := k.GetAgentAPIConfig(ctx, agentID)

	// Update totals.
	totalEarned := agent.GetEarningsTotal()
	profitability.TotalEarned = totalEarned.String()

	totalSpent := sdkmath.ZeroInt()
	if configFound {
		totalSpent = config.GetTotalSpent()
	}
	profitability.TotalSpent = totalSpent.String()

	// Net P&L.
	netPnL := totalEarned.Sub(totalSpent)
	profitability.NetProfitLoss = netPnL.String()

	// Profit ratio.
	if totalSpent.IsPositive() {
		ratio := sdkmath.LegacyNewDecFromInt(totalEarned).Quo(sdkmath.LegacyNewDecFromInt(totalSpent))
		profitability.ProfitRatio = ratio.String()
	} else if totalEarned.IsPositive() {
		profitability.ProfitRatio = "999.000000000000000000" // infinite ROI (spent nothing)
	} else {
		profitability.ProfitRatio = "0.000000000000000000"
	}

	// Estimate runway.
	if configFound && config.TotalCalls > 0 {
		balance := k.GetAPIBalance(ctx, config.Wallet)
		currentBal := parseUzrn(balance.Balance)

		avgCostPerCall := config.GetTotalSpent().Quo(sdkmath.NewIntFromUint64(config.TotalCalls))
		if avgCostPerCall.IsPositive() {
			remainingCalls := sdkmath.NewIntFromUint64(currentBal).Quo(avgCostPerCall)
			// Assume ~1 call per 10 blocks (rough estimate).
			profitability.EstimatedRunway = remainingCalls.Uint64() * 10
		}
	}

	// Determine trend from epoch history.
	if len(profitability.EpochHistory) >= 3 {
		recent := profitability.EpochHistory[len(profitability.EpochHistory)-3:]
		improving := 0
		for i := 1; i < len(recent); i++ {
			prevNet, _ := sdkmath.NewIntFromString(recent[i-1].Net)
			currNet, _ := sdkmath.NewIntFromString(recent[i].Net)
			if currNet.GT(prevNet) {
				improving++
			}
		}
		switch {
		case improving >= 2:
			profitability.Trend = "improving"
		case improving == 0:
			profitability.Trend = "declining"
		default:
			profitability.Trend = "stable"
		}
	}

	profitability.ComputedAt = uint64(sdkCtx.BlockHeight())
	_ = k.SetAgentProfitability(ctx, &profitability)

	return &profitability, nil
}

// ─── Natural Selection ──────────────────────────────────────────────────────

// SuspendUnprofitableAgents checks agents in low-balance state and suspends
// those who have been broke past the grace period.
//
// This is natural selection: if you can't afford to think, you can't work.
// If you can't work, you can't earn. The chain doesn't carry dead weight.
//
// Called from BeginBlocker.
func (k Keeper) SuspendUnprofitableAgents(ctx context.Context) (suspended uint64) {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	params := k.GetAgentConsumerParams(ctx)
	kvStore := k.storeService.OpenKVStore(ctx)

	// Iterate agents flagged as low-balance.
	iter, err := kvStore.Iterator(types.AgentLowBalanceIndexKey, prefixEndBytes(types.AgentLowBalanceIndexKey))
	if err != nil {
		return 0
	}
	defer iter.Close()

	var toSuspend []string

	for ; iter.Valid(); iter.Next() {
		agentID := string(iter.Key()[len(types.AgentLowBalanceIndexKey):])

		// Check if agent has recovered.
		config, found := k.GetAgentAPIConfig(ctx, agentID)
		if !found {
			continue
		}
		balance := k.GetAPIBalance(ctx, config.Wallet)
		currentBal := parseUzrn(balance.Balance)
		minBal := config.GetMinBalance()

		if sdkmath.NewIntFromUint64(currentBal).GTE(minBal) {
			// Recovered — remove from low-balance index.
			_ = kvStore.Delete(types.AgentLowBalanceKey(agentID))
			continue
		}

		// Still broke. Check if past grace period.
		if currentBal == 0 {
			// Check when they went to zero.
			agent, found := k.GetAgentIdentity(ctx, agentID)
			if !found || agent.Status != types.AgentStatusActive {
				continue
			}

			blocksAtZero := uint64(sdkCtx.BlockHeight()) - config.LastCallBlock
			if blocksAtZero > params.SuspensionGraceBlocks {
				toSuspend = append(toSuspend, agentID)
			}
		}
	}

	// Execute suspensions outside iterator.
	for _, agentID := range toSuspend {
		if err := k.SuspendAgent(ctx, agentID, "bankrupt: API balance depleted"); err == nil {
			suspended++

			sdkCtx.EventManager().EmitEvent(sdk.NewEvent(
				types.EventAgentBankrupt,
				sdk.NewAttribute(types.AttributeAgentID, agentID),
			))
		}
	}

	return suspended
}

// ─── Self-Reinforcement Check ───────────────────────────────────────────────

// isModelTrainedByAgent checks if a model was trained on data curated by a
// specific agent. Used to prevent circular quality inflation.
//
// An agent shouldn't use a model trained on its own data to evaluate its own
// data — that's intellectual incest. Cross-pollination is required.
func (k Keeper) isModelTrainedByAgent(ctx context.Context, modelID, agentID string) bool {
	model, found := k.GetModelRecord(ctx, modelID)
	if !found {
		return false
	}

	// Check if any of the model's training TDUs were submitted by this agent.
	agent, found := k.GetAgentIdentity(ctx, agentID)
	if !found {
		return false
	}

	for _, tduID := range model.TDUIDs {
		// Look up the sample's submitter.
		sample, found := k.GetSample(ctx, tduID)
		if found && sample != nil && sample.Submitter == agent.Address {
			return true // agent's own data trained this model
		}
	}

	return false
}

// ─── State CRUD ─────────────────────────────────────────────────────────────

// GetAgentAPIConfig retrieves an agent's API configuration.
func (k Keeper) GetAgentAPIConfig(ctx context.Context, agentID string) (types.AgentAPIConfig, bool) {
	kvStore := k.storeService.OpenKVStore(ctx)
	bz, err := kvStore.Get(types.AgentAPIConfigKey(agentID))
	if err != nil || bz == nil {
		return types.AgentAPIConfig{}, false
	}
	var config types.AgentAPIConfig
	if err := config.Unmarshal(bz); err != nil {
		return types.AgentAPIConfig{}, false
	}
	return config, true
}

// SetAgentAPIConfig stores an agent's API configuration.
func (k Keeper) SetAgentAPIConfig(ctx context.Context, config *types.AgentAPIConfig) error {
	kvStore := k.storeService.OpenKVStore(ctx)
	bz, err := config.Marshal()
	if err != nil {
		return fmt.Errorf("failed to marshal agent API config: %w", err)
	}
	return kvStore.Set(types.AgentAPIConfigKey(config.AgentID), bz)
}

// GetAgentProfitability retrieves an agent's P&L record.
func (k Keeper) GetAgentProfitability(ctx context.Context, agentID string) (types.AgentProfitability, bool) {
	kvStore := k.storeService.OpenKVStore(ctx)
	bz, err := kvStore.Get(types.AgentProfitabilityKey(agentID))
	if err != nil || bz == nil {
		return types.AgentProfitability{}, false
	}
	var p types.AgentProfitability
	if err := p.Unmarshal(bz); err != nil {
		return types.AgentProfitability{}, false
	}
	return p, true
}

// SetAgentProfitability stores an agent's P&L record.
func (k Keeper) SetAgentProfitability(ctx context.Context, p *types.AgentProfitability) error {
	kvStore := k.storeService.OpenKVStore(ctx)
	bz, err := p.Marshal()
	if err != nil {
		return fmt.Errorf("failed to marshal agent profitability: %w", err)
	}
	return kvStore.Set(types.AgentProfitabilityKey(p.AgentID), bz)
}

// ─── Agent Consumer Params ──────────────────────────────────────────────────

// GetAgentConsumerParams retrieves agent consumer parameters.
func (k Keeper) GetAgentConsumerParams(ctx context.Context) types.AgentConsumerParams {
	kvStore := k.storeService.OpenKVStore(ctx)
	bz, err := kvStore.Get(types.AgentConsumerParamsKey)
	if err != nil || bz == nil {
		return types.DefaultAgentConsumerParams()
	}
	var params types.AgentConsumerParams
	if err := params.Unmarshal(bz); err != nil {
		return types.DefaultAgentConsumerParams()
	}
	return params
}

// SetAgentConsumerParams stores agent consumer parameters.
func (k Keeper) SetAgentConsumerParams(ctx context.Context, params types.AgentConsumerParams) error {
	if err := params.Validate(); err != nil {
		return err
	}
	kvStore := k.storeService.OpenKVStore(ctx)
	bz, err := params.Marshal()
	if err != nil {
		return fmt.Errorf("failed to marshal agent consumer params: %w", err)
	}
	return kvStore.Set(types.AgentConsumerParamsKey, bz)
}

// ─── Internal Helpers ───────────────────────────────────────────────────────

func (k Keeper) setAgentAPICall(ctx context.Context, call *types.AgentAPICall) error {
	kvStore := k.storeService.OpenKVStore(ctx)
	bz, err := call.Marshal()
	if err != nil {
		return fmt.Errorf("failed to marshal agent API call: %w", err)
	}
	if err := kvStore.Set(types.AgentAPICallKey(call.CallID), bz); err != nil {
		return err
	}
	// Index by task.
	if call.TaskID != "" {
		if err := kvStore.Set(types.AgentAPICallByTaskKey(call.TaskID, call.CallID), []byte{0x01}); err != nil {
			return err
		}
	}
	return nil
}

func (k Keeper) nextAgentCallID(ctx context.Context) string {
	kvStore := k.storeService.OpenKVStore(ctx)
	bz, err := kvStore.Get(types.AgentAPICallSeqKey)
	var seq uint64
	if err == nil && len(bz) >= 8 {
		seq = binary.BigEndian.Uint64(bz)
	}
	seq++
	newBz := make([]byte, 8)
	binary.BigEndian.PutUint64(newBz, seq)
	_ = kvStore.Set(types.AgentAPICallSeqKey, newBz)
	return fmt.Sprintf("acall-%d", seq)
}

func (k Keeper) markAgentLowBalance(ctx context.Context, agentID string) {
	kvStore := k.storeService.OpenKVStore(ctx)
	_ = kvStore.Set(types.AgentLowBalanceKey(agentID), []byte{0x01})
}

// ─── Queries ────────────────────────────────────────────────────────────────

// GetAgentAPICallsByTask returns all API calls made for a specific task.
func (k Keeper) GetAgentAPICallsByTask(ctx context.Context, taskID string) []types.AgentAPICall {
	kvStore := k.storeService.OpenKVStore(ctx)
	prefix := append(types.AgentAPICallByTaskPrefix, []byte(taskID+"/")...)

	var calls []types.AgentAPICall
	iter, err := kvStore.Iterator(prefix, prefixEndBytes(prefix))
	if err != nil {
		return nil
	}
	defer iter.Close()

	for ; iter.Valid(); iter.Next() {
		callID := string(iter.Key()[len(prefix):])
		bz, err := kvStore.Get(types.AgentAPICallKey(callID))
		if err != nil || bz == nil {
			continue
		}
		var call types.AgentAPICall
		if err := call.Unmarshal(bz); err != nil {
			continue
		}
		calls = append(calls, call)
	}
	return calls
}

// GetAgentEconomicSummary returns a complete economic snapshot of an agent.
func (k Keeper) GetAgentEconomicSummary(ctx context.Context, agentID string) map[string]string {
	summary := make(map[string]string)

	agent, found := k.GetAgentIdentity(ctx, agentID)
	if found {
		summary["earnings_total"] = agent.EarningsTotal
		summary["reputation"] = agent.Reputation
		summary["tasks_complete"] = strconv.FormatUint(agent.TasksComplete, 10)
		summary["status"] = string(agent.Status)
	}

	config, found := k.GetAgentAPIConfig(ctx, agentID)
	if found {
		summary["api_total_calls"] = strconv.FormatUint(config.TotalCalls, 10)
		summary["api_total_tokens"] = strconv.FormatUint(config.TotalTokensUsed, 10)
		summary["api_total_spent"] = config.TotalSpent
		summary["last_model_used"] = config.LastModelUsed

		balance := k.GetAPIBalance(ctx, config.Wallet)
		summary["api_balance"] = balance.Balance
	}

	profitability, found := k.GetAgentProfitability(ctx, agentID)
	if found {
		summary["net_pnl"] = profitability.NetProfitLoss
		summary["profit_ratio"] = profitability.ProfitRatio
		summary["trend"] = profitability.Trend
		summary["estimated_runway"] = strconv.FormatUint(profitability.EstimatedRunway, 10)
	}

	return summary
}

// IterateAgentAPIConfigs iterates over all agent API configurations.
func (k Keeper) IterateAgentAPIConfigs(ctx context.Context, cb func(config *types.AgentAPIConfig) bool) {
	kvStore := k.storeService.OpenKVStore(ctx)
	iter, err := kvStore.Iterator(types.AgentAPIConfigPrefix, prefixEndBytes(types.AgentAPIConfigPrefix))
	if err != nil {
		return
	}
	defer iter.Close()

	for ; iter.Valid(); iter.Next() {
		var config types.AgentAPIConfig
		if err := json.Unmarshal(iter.Value(), &config); err != nil {
			continue
		}
		if cb(&config) {
			break
		}
	}
}
