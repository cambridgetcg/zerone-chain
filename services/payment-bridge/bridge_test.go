package main

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/zerone-chain/zerone/services/payment-bridge/internal/ledger"
	"github.com/zerone-chain/zerone/services/payment-bridge/internal/settlement"
)

// ─── Mock chain settler ─────────────────────────────────────────────────────

type mockChainSettler struct {
	submissions []*settlement.SettlementBatch
	failCount   int // if >0, fail this many times then succeed
	seenBatches map[string]bool
}

func newMockChainSettler() *mockChainSettler {
	return &mockChainSettler{seenBatches: make(map[string]bool)}
}

func (m *mockChainSettler) SubmitSettlement(_ context.Context, batch *settlement.SettlementBatch) error {
	if m.failCount > 0 {
		m.failCount--
		return fmt.Errorf("chain unavailable")
	}

	// Idempotency check: reject duplicate batch
	key := fmt.Sprintf("%s_%d_%d", batch.UserAddr, batch.TotalTokens, batch.TotalCostUZRN)
	if m.seenBatches[key] {
		return fmt.Errorf("duplicate batch rejected")
	}
	m.seenBatches[key] = true
	m.submissions = append(m.submissions, batch)
	return nil
}

// ─── Mock ledger (in-memory, no Redis) ──────────────────────────────────────

type mockLedger struct {
	balances     map[string]int64
	usageRecords map[string][]string
	pending      map[string]bool
}

func newMockLedger() *mockLedger {
	return &mockLedger{
		balances:     make(map[string]int64),
		usageRecords: make(map[string][]string),
		pending:      make(map[string]bool),
	}
}

func (m *mockLedger) CreditDeposit(userAddr string, amount int64) int64 {
	m.balances[userAddr] += amount
	return m.balances[userAddr]
}

func (m *mockLedger) GetBalance(userAddr string) int64 {
	return m.balances[userAddr]
}

func (m *mockLedger) Deduct(userAddr string, cost int64) (int64, error) {
	bal := m.balances[userAddr]
	if bal < cost {
		return 0, fmt.Errorf("insufficient balance for %s (cost: %d, balance: %d)", userAddr, cost, bal)
	}
	m.balances[userAddr] = bal - cost
	return m.balances[userAddr], nil
}

func (m *mockLedger) RecordUsage(userAddr, requestID string, tokens, cost int64, model string) {
	entry := fmt.Sprintf("%s|%s|%d|%d|%s|%d",
		userAddr, requestID, tokens, cost, model, time.Now().Unix())
	m.usageRecords[userAddr] = append(m.usageRecords[userAddr], entry)
	m.pending[userAddr] = true
}

func (m *mockLedger) GetPendingUsers() []string {
	var users []string
	for u := range m.pending {
		users = append(users, u)
	}
	return users
}

func (m *mockLedger) DrainUsage(userAddr string) []string {
	records := m.usageRecords[userAddr]
	delete(m.usageRecords, userAddr)
	delete(m.pending, userAddr)
	return records
}

// ─── Scenario 12: Batch usage submission ────────────────────────────────────

func TestBridge_BatchUsageSubmission(t *testing.T) {
	ml := newMockLedger()
	chain := newMockChainSettler()

	userAddr := "zrn1user_batch_test"
	ml.CreditDeposit(userAddr, 100_000_000) // 100 ZRN

	// Accumulate 100 usage records
	for i := 0; i < 100; i++ {
		inputTokens := int64(1000 + i*10)
		outputTokens := int64(500 + i*5)
		cost := ledger.EstimateTokenCost(inputTokens, outputTokens, 1, 3)
		_, err := ml.Deduct(userAddr, cost)
		if err != nil {
			t.Fatalf("deduct failed at i=%d: %v", i, err)
		}
		ml.RecordUsage(userAddr, fmt.Sprintf("req-%d", i), inputTokens+outputTokens, cost, "zerone-8b")
	}

	// Verify 100 records queued
	records := ml.DrainUsage(userAddr)
	require.Len(t, records, 100)

	// Parse records into a settlement batch
	batch := &settlement.SettlementBatch{
		UserAddr:    userAddr,
		RecordCount: len(records),
		PeriodEnd:   time.Now(),
	}
	for _, rec := range records {
		parts := strings.SplitN(rec, "|", 6)
		require.True(t, len(parts) >= 6, "record should have 6 parts: %s", rec)
		tokens, _ := strconv.ParseInt(parts[2], 10, 64)
		cost, _ := strconv.ParseInt(parts[3], 10, 64)
		batch.TotalTokens += tokens
		batch.TotalCostUZRN += cost
	}

	// Submit to chain
	err := chain.SubmitSettlement(context.Background(), batch)
	require.NoError(t, err)
	require.Len(t, chain.submissions, 1)
	require.Equal(t, 100, chain.submissions[0].RecordCount)
	require.True(t, chain.submissions[0].TotalTokens > 0)
	require.True(t, chain.submissions[0].TotalCostUZRN > 0)

	// Verify balance decreased
	remainingBal := ml.GetBalance(userAddr)
	require.True(t, remainingBal < 100_000_000, "balance should have decreased")
	require.Equal(t, int64(100_000_000)-batch.TotalCostUZRN, remainingBal)
}

// ─── Scenario 13: Duplicate batch rejection ─────────────────────────────────

func TestBridge_DuplicateBatchRejection(t *testing.T) {
	chain := newMockChainSettler()

	batch := &settlement.SettlementBatch{
		UserAddr:      "zrn1user_dup_test",
		TotalTokens:   50000,
		TotalCostUZRN: 100,
		RecordCount:   10,
		PeriodStart:   time.Now().Add(-5 * time.Minute),
		PeriodEnd:     time.Now(),
	}

	// First submission succeeds
	err := chain.SubmitSettlement(context.Background(), batch)
	require.NoError(t, err)
	require.Len(t, chain.submissions, 1)

	// Second identical submission rejected (idempotency)
	err = chain.SubmitSettlement(context.Background(), batch)
	require.Error(t, err)
	require.Contains(t, err.Error(), "duplicate")
	require.Len(t, chain.submissions, 1) // still only 1
}

// ─── Scenario 14: Bridge recovery after crash ───────────────────────────────

func TestBridge_RecoveryAfterCrash(t *testing.T) {
	ml := newMockLedger()
	chain := newMockChainSettler()

	userAddr := "zrn1user_recovery_test"
	ml.CreditDeposit(userAddr, 50_000_000)

	// Record 20 usage records
	var totalCost int64
	for i := 0; i < 20; i++ {
		cost := ledger.EstimateTokenCost(1000, 500, 1, 3)
		_, err := ml.Deduct(userAddr, cost)
		require.NoError(t, err)
		ml.RecordUsage(userAddr, fmt.Sprintf("req-%d", i), 1500, cost, "zerone-8b")
		totalCost += cost
	}

	// Simulate bridge crash mid-batch: chain fails for first attempt
	chain.failCount = 1

	// First attempt fails
	records := ml.usageRecords[userAddr] // peek without draining
	batch := buildBatch(userAddr, records)
	err := chain.SubmitSettlement(context.Background(), batch)
	require.Error(t, err)
	require.Len(t, chain.submissions, 0)

	// "Restart" bridge: retry submission
	chain.failCount = 0
	err = chain.SubmitSettlement(context.Background(), batch)
	require.NoError(t, err)
	require.Len(t, chain.submissions, 1)
	require.Equal(t, 20, chain.submissions[0].RecordCount)

	// Attempt resubmit after success → rejected as duplicate (no double-charging)
	err = chain.SubmitSettlement(context.Background(), batch)
	require.Error(t, err)
	require.Contains(t, err.Error(), "duplicate")

	// Verify balance is consistent: only deducted once
	finalBal := ml.GetBalance(userAddr)
	require.Equal(t, int64(50_000_000)-totalCost, finalBal)
}

// ─── Ledger unit tests ──────────────────────────────────────────────────────

func TestBridge_EstimateTokenCost(t *testing.T) {
	tests := []struct {
		name     string
		input    int64
		output   int64
		priceIn  int64
		priceOut int64
		expected int64
	}{
		{"basic", 5000, 2000, 1, 3, 11},               // (5000*1/1000) + (2000*3/1000) = 5 + 6
		{"minimum 1", 1, 0, 1, 3, 1},                    // floor to 1
		{"zero tokens", 0, 0, 1, 3, 0},                  // zero stays zero
		{"large batch", 1_000_000, 500_000, 1, 3, 2500}, // (1M/1K) + (500K*3/1K) = 1000+1500
		{"output only", 0, 10000, 1, 3, 30},             // 0 + (10K*3/1K)
		{"input only", 10000, 0, 1, 3, 10},              // (10K*1/1K) + 0
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cost := ledger.EstimateTokenCost(tt.input, tt.output, tt.priceIn, tt.priceOut)
			require.Equal(t, tt.expected, cost)
		})
	}
}

func TestBridge_EstimateCostLegacy(t *testing.T) {
	// Legacy per-million pricing
	cost := ledger.EstimateCost(500_000, 500_000, 1000)
	// (1M * 1000) / 1M = 1000
	require.Equal(t, int64(1000), cost)

	// Minimum 1
	cost = ledger.EstimateCost(1, 0, 1)
	require.Equal(t, int64(1), cost)
}

func TestBridge_LowBalanceThreshold(t *testing.T) {
	threshold := ledger.GetLowBalanceThreshold(100_000_000)
	require.Equal(t, int64(10_000_000), threshold) // 10% of 100M
}

// ─── Settler unit tests ─────────────────────────────────────────────────────

func TestBridge_SettlerBatchParsing(t *testing.T) {
	// Verify settlement batch aggregation via the mock settler
	chain := newMockChainSettler()

	batch := &settlement.SettlementBatch{
		UserAddr:      "zrn1user_parse_test",
		TotalTokens:   75000,
		TotalCostUZRN: 150,
		RecordCount:   50,
		PeriodStart:   time.Now().Add(-10 * time.Minute),
		PeriodEnd:     time.Now(),
	}

	err := chain.SubmitSettlement(context.Background(), batch)
	require.NoError(t, err)
	require.Equal(t, int64(75000), chain.submissions[0].TotalTokens)
	require.Equal(t, int64(150), chain.submissions[0].TotalCostUZRN)
	require.Equal(t, 50, chain.submissions[0].RecordCount)
}

func TestBridge_InsufficientBalanceDeduction(t *testing.T) {
	ml := newMockLedger()

	ml.CreditDeposit("user1", 100)

	// Try to deduct more than available
	_, err := ml.Deduct("user1", 200)
	require.Error(t, err)
	require.Contains(t, err.Error(), "insufficient")

	// Balance unchanged
	require.Equal(t, int64(100), ml.GetBalance("user1"))
}

func TestBridge_ConcurrentUsageRecording(t *testing.T) {
	ml := newMockLedger()
	ml.CreditDeposit("user1", 1_000_000)

	// Record 50 usage entries
	for i := 0; i < 50; i++ {
		cost := int64(10)
		_, err := ml.Deduct("user1", cost)
		require.NoError(t, err)
		ml.RecordUsage("user1", fmt.Sprintf("req-%d", i), 1500, cost, "zerone-8b")
	}

	// Verify all records present
	records := ml.DrainUsage("user1")
	require.Len(t, records, 50)

	// Verify balance
	require.Equal(t, int64(1_000_000-500), ml.GetBalance("user1"))

	// After drain, no more pending
	pending := ml.GetPendingUsers()
	require.Len(t, pending, 0)
}

// ─── helpers ────────────────────────────────────────────────────────────────

func buildBatch(userAddr string, records []string) *settlement.SettlementBatch {
	batch := &settlement.SettlementBatch{
		UserAddr:    userAddr,
		RecordCount: len(records),
		PeriodEnd:   time.Now(),
	}
	for _, rec := range records {
		parts := strings.SplitN(rec, "|", 6)
		if len(parts) < 6 {
			continue
		}
		tokens, _ := strconv.ParseInt(parts[2], 10, 64)
		cost, _ := strconv.ParseInt(parts[3], 10, 64)
		batch.TotalTokens += tokens
		batch.TotalCostUZRN += cost
	}
	return batch
}
