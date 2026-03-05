package settlement

import (
	"context"
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

	"github.com/zerone-chain/zerone/services/payment-bridge/internal/ledger"
)

// SettlementBatch represents a batch of usage to settle on-chain.
type SettlementBatch struct {
	UserAddr       string
	TotalTokens    int64
	TotalCostUZRN  int64
	RecordCount    int
	PeriodStart    time.Time
	PeriodEnd      time.Time
}

// ChainSettler submits batch settlements to the ZERONE chain.
type ChainSettler interface {
	SubmitSettlement(ctx context.Context, batch *SettlementBatch) error
}

// Settler accumulates usage and periodically settles on-chain.
type Settler struct {
	ledger        *ledger.Ledger
	chain         ChainSettler
	interval      time.Duration
	maxRetries    int
}

// NewSettler creates a batch settler.
func NewSettler(l *ledger.Ledger, chain ChainSettler, interval time.Duration) *Settler {
	if interval == 0 {
		interval = 5 * time.Minute
	}
	return &Settler{
		ledger:     l,
		chain:      chain,
		interval:   interval,
		maxRetries: 3,
	}
}

// Run starts the periodic settlement loop.
func (s *Settler) Run(ctx context.Context) error {
	log.Printf("settler: running every %s", s.interval)

	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			// Final settlement attempt before shutdown
			_ = s.SettleAll(ctx)
			return nil
		case <-ticker.C:
			if err := s.SettleAll(ctx); err != nil {
				log.Printf("settlement cycle error: %v", err)
			}
		}
	}
}

// SettleAll processes all pending users.
func (s *Settler) SettleAll(ctx context.Context) error {
	users, err := s.ledger.GetPendingUsers(ctx)
	if err != nil {
		return fmt.Errorf("get pending users: %w", err)
	}

	if len(users) == 0 {
		return nil
	}

	log.Printf("settler: processing %d users", len(users))
	var lastErr error
	for _, user := range users {
		if err := s.settleUser(ctx, user); err != nil {
			log.Printf("settle %s: %v", user, err)
			lastErr = err
		}
	}
	return lastErr
}

func (s *Settler) settleUser(ctx context.Context, userAddr string) error {
	records, err := s.ledger.DrainUsage(ctx, userAddr)
	if err != nil {
		return fmt.Errorf("drain usage: %w", err)
	}
	if len(records) == 0 {
		return nil
	}

	batch := &SettlementBatch{
		UserAddr:    userAddr,
		RecordCount: len(records),
		PeriodEnd:   time.Now(),
	}

	// Parse records: "userAddr|requestID|tokens|cost|model|timestamp"
	for _, rec := range records {
		parts := strings.SplitN(rec, "|", 6)
		if len(parts) < 6 {
			continue
		}
		tokens, _ := strconv.ParseInt(parts[2], 10, 64)
		cost, _ := strconv.ParseInt(parts[3], 10, 64)
		ts, _ := strconv.ParseInt(parts[5], 10, 64)

		batch.TotalTokens += tokens
		batch.TotalCostUZRN += cost

		t := time.Unix(ts, 0)
		if batch.PeriodStart.IsZero() || t.Before(batch.PeriodStart) {
			batch.PeriodStart = t
		}
	}

	// Submit to chain with retries
	var submitErr error
	for attempt := 0; attempt < s.maxRetries; attempt++ {
		submitErr = s.chain.SubmitSettlement(ctx, batch)
		if submitErr == nil {
			log.Printf("settled %s: %d records, %d tokens, %d uzrn",
				userAddr, batch.RecordCount, batch.TotalTokens, batch.TotalCostUZRN)
			return nil
		}
		backoff := time.Duration(1<<attempt) * time.Second
		log.Printf("settlement attempt %d/%d failed for %s: %v (retry in %s)",
			attempt+1, s.maxRetries, userAddr, submitErr, backoff)
		time.Sleep(backoff)
	}

	return fmt.Errorf("settlement failed after %d retries: %w", s.maxRetries, submitErr)
}
