package withdrawal

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/zerone-chain/zerone/services/payment-bridge/internal/ledger"
)

// ChainWithdrawer submits withdrawal transactions on-chain.
type ChainWithdrawer interface {
	SubmitWithdrawal(ctx context.Context, userAddr string, amountUZRN int64) error
}

// Handler processes withdrawal requests with a cooldown period.
type Handler struct {
	ledger   *ledger.Ledger
	chain    ChainWithdrawer
	cooldown time.Duration
}

// NewHandler creates a withdrawal handler.
func NewHandler(l *ledger.Ledger, chain ChainWithdrawer, cooldown time.Duration) *Handler {
	if cooldown == 0 {
		cooldown = 10 * time.Minute
	}
	return &Handler{
		ledger:   l,
		chain:    chain,
		cooldown: cooldown,
	}
}

// RequestWithdrawal initiates a withdrawal for a user.
func (h *Handler) RequestWithdrawal(ctx context.Context, userAddr string, amountUZRN int64) error {
	// Check balance
	bal, err := h.ledger.GetBalance(ctx, userAddr)
	if err != nil {
		return fmt.Errorf("check balance: %w", err)
	}
	if bal < amountUZRN {
		return fmt.Errorf("insufficient balance: have %d uzrn, requested %d uzrn", bal, amountUZRN)
	}

	// Check for pending settlements (cooldown)
	pending, err := h.ledger.GetPendingUsers(ctx)
	if err != nil {
		return fmt.Errorf("check pending: %w", err)
	}
	for _, p := range pending {
		if p == userAddr {
			return fmt.Errorf("withdrawal blocked: pending settlements must clear first (cooldown: %s)", h.cooldown)
		}
	}

	// Deduct from off-chain balance
	_, err = h.ledger.Deduct(ctx, userAddr, amountUZRN)
	if err != nil {
		return fmt.Errorf("deduct for withdrawal: %w", err)
	}

	// Submit on-chain withdrawal
	if err := h.chain.SubmitWithdrawal(ctx, userAddr, amountUZRN); err != nil {
		// Re-credit on failure
		_, creditErr := h.ledger.CreditDeposit(ctx, userAddr, amountUZRN)
		if creditErr != nil {
			log.Printf("CRITICAL: failed to re-credit %s after withdrawal failure: %v", userAddr, creditErr)
		}
		return fmt.Errorf("submit withdrawal: %w", err)
	}

	log.Printf("withdrawal: %s withdrew %d uzrn", userAddr, amountUZRN)
	return nil
}
