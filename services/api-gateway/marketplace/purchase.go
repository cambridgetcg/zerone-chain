package marketplace

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"sync"
	"time"
)

const (
	// PurchaseExpiryBlocks is the number of blocks before access tickets expire.
	PurchaseExpiryBlocks = 14400 // ~24 hours at 6s blocks

	// CuratorDefaultShare is the default curator revenue share.
	CuratorDefaultShare = 0.10 // 10%
)

// PurchaseManager handles the dataset purchase flow.
type PurchaseManager struct {
	mu        sync.RWMutex
	purchases map[string]*Purchase // id → purchase
	catalog   *Catalog
}

// NewPurchaseManager creates a purchase manager.
func NewPurchaseManager(catalog *Catalog) *PurchaseManager {
	return &PurchaseManager{
		purchases: make(map[string]*Purchase),
		catalog:   catalog,
	}
}

// Initiate starts a purchase flow.
func (pm *PurchaseManager) Initiate(buyerAddr, datasetID string, amountUZRN int64) (*Purchase, error) {
	ds, ok := pm.catalog.Get(datasetID)
	if !ok {
		return nil, fmt.Errorf("dataset %s not found", datasetID)
	}

	tier := TierForAmount(amountUZRN)
	shares := SharesForTier(tier)

	// Generate unique purchase ID
	idBytes := make([]byte, 16)
	if _, err := rand.Read(idBytes); err != nil {
		return nil, fmt.Errorf("generate purchase id: %w", err)
	}

	// Generate watermark seed for this buyer
	seedBytes := make([]byte, 16)
	if _, err := rand.Read(seedBytes); err != nil {
		return nil, fmt.Errorf("generate watermark seed: %w", err)
	}

	p := &Purchase{
		ID:            hex.EncodeToString(idBytes),
		DatasetID:     ds.ID,
		BuyerAddr:     buyerAddr,
		Tier:          tier,
		AmountUZRN:    amountUZRN,
		Status:        PurchasePending,
		ShamirShares:  shares,
		WatermarkSeed: hex.EncodeToString(seedBytes),
		ExpiresAt:     time.Now().Add(24 * time.Hour),
		CreatedAt:     time.Now(),
	}

	pm.mu.Lock()
	pm.purchases[p.ID] = p
	pm.mu.Unlock()

	return p, nil
}

// ConfirmPayment confirms that payment was deducted, releasing access.
func (pm *PurchaseManager) ConfirmPayment(purchaseID string) error {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	p, ok := pm.purchases[purchaseID]
	if !ok {
		return fmt.Errorf("purchase %s not found", purchaseID)
	}
	if p.Status != PurchasePending {
		return fmt.Errorf("purchase %s not in pending state: %s", purchaseID, p.Status)
	}

	p.Status = PurchaseConfirmed

	// Generate access tickets for chunk download
	tickets, err := generateAccessTickets(p.ShamirShares)
	if err != nil {
		return fmt.Errorf("generate tickets: %w", err)
	}
	p.AccessTickets = tickets

	return nil
}

// Get returns a purchase by ID.
func (pm *PurchaseManager) Get(purchaseID string) (*Purchase, bool) {
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	p, ok := pm.purchases[purchaseID]
	return p, ok
}

// GetByBuyer returns all purchases for a buyer address.
func (pm *PurchaseManager) GetByBuyer(buyerAddr string) []*Purchase {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	var result []*Purchase
	for _, p := range pm.purchases {
		if p.BuyerAddr == buyerAddr {
			result = append(result, p)
		}
	}
	return result
}

// MarkComplete marks a purchase as fully downloaded.
func (pm *PurchaseManager) MarkComplete(purchaseID string) error {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	p, ok := pm.purchases[purchaseID]
	if !ok {
		return fmt.Errorf("purchase %s not found", purchaseID)
	}
	p.Status = PurchaseComplete
	return nil
}

// ExpireStale marks old pending purchases as expired.
func (pm *PurchaseManager) ExpireStale() int {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	expired := 0
	now := time.Now()
	for _, p := range pm.purchases {
		if p.Status == PurchasePending && now.After(p.ExpiresAt) {
			p.Status = PurchaseExpired
			expired++
		}
	}
	return expired
}

// RevenueBreakdown computes the revenue distribution for a purchase.
func RevenueBreakdown(amountUZRN int64, curatorShare float64) (curator, protocol, research int64) {
	if curatorShare <= 0 {
		curatorShare = CuratorDefaultShare
	}
	curator = int64(float64(amountUZRN) * curatorShare)
	remaining := amountUZRN - curator
	// 70% protocol treasury, 30% research fund
	protocol = remaining * 70 / 100
	research = remaining - protocol
	return
}

func generateAccessTickets(count int) ([]string, error) {
	tickets := make([]string, count)
	for i := range tickets {
		b := make([]byte, 32)
		if _, err := rand.Read(b); err != nil {
			return nil, err
		}
		tickets[i] = hex.EncodeToString(b)
	}
	return tickets, nil
}
