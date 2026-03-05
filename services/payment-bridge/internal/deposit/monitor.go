package deposit

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/zerone-chain/zerone/services/payment-bridge/internal/ledger"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// Monitor watches on-chain deposit events and credits the off-chain ledger.
type Monitor struct {
	chainEndpoint string
	ledger        *ledger.Ledger
	conn          *grpc.ClientConn
	pollInterval  time.Duration
}

// NewMonitor creates a deposit monitor.
func NewMonitor(chainEndpoint string, l *ledger.Ledger, pollInterval time.Duration) *Monitor {
	if pollInterval == 0 {
		pollInterval = 5 * time.Second
	}
	return &Monitor{
		chainEndpoint: chainEndpoint,
		ledger:        l,
		pollInterval:  pollInterval,
	}
}

// Start begins monitoring for deposit events.
func (m *Monitor) Start(ctx context.Context) error {
	conn, err := grpc.DialContext(ctx, m.chainEndpoint,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		return fmt.Errorf("connect chain: %w", err)
	}
	m.conn = conn

	log.Printf("deposit monitor: watching %s (poll=%s)", m.chainEndpoint, m.pollInterval)

	ticker := time.NewTicker(m.pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return m.conn.Close()
		case <-ticker.C:
			if err := m.poll(ctx); err != nil {
				log.Printf("deposit monitor poll error: %v", err)
			}
		}
	}
}

func (m *Monitor) poll(ctx context.Context) error {
	// TODO: Query x/billing for new deposit events since last processed block.
	// For each deposit event:
	//   userAddr := event.Depositor
	//   amount := event.Amount.AmountOf("uzrn").Int64()
	//   m.ledger.CreditDeposit(ctx, userAddr, amount)
	return nil
}

// ProcessDeposit manually credits a deposit (for testing / manual operations).
func (m *Monitor) ProcessDeposit(ctx context.Context, userAddr string, amountUZRN int64) (int64, error) {
	return m.ledger.CreditDeposit(ctx, userAddr, amountUZRN)
}
