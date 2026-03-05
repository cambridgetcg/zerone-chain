package marketplace

import (
	"time"
)

// PricingTier defines access levels based on ZRN payment.
type PricingTier string

const (
	TierPreview    PricingTier = "preview"
	TierSlice      PricingTier = "slice"
	TierStandard   PricingTier = "standard"
	TierPremium    PricingTier = "premium"
	TierEnterprise PricingTier = "enterprise"
)

// TierConfig holds configuration for a pricing tier.
type TierConfig struct {
	Tier         PricingTier `json:"tier"`
	PriceUZRN    int64       `json:"price_uzrn"`
	ShamirShares int         `json:"shamir_shares"` // shares released at this tier
	Description  string      `json:"description"`
}

// DefaultTiers returns the standard pricing tiers.
func DefaultTiers() []TierConfig {
	return []TierConfig{
		{TierPreview, 0, 0, "5 low-quality samples, browse only"},
		{TierSlice, 1_000_000, 2, "Random subset of chunks for evaluation"},
		{TierStandard, 10_000_000, 5, "Full dataset, bronze+ quality"},
		{TierPremium, 50_000_000, 8, "Full dataset, silver+ quality including gold"},
		{TierEnterprise, 100_000_000, 10, "Full dataset + ongoing subscription"},
	}
}

// TierForAmount returns the highest tier affordable at the given ZRN amount.
func TierForAmount(amountUZRN int64) PricingTier {
	tiers := DefaultTiers()
	best := TierPreview
	for _, t := range tiers {
		if amountUZRN >= t.PriceUZRN {
			best = t.Tier
		}
	}
	return best
}

// SharesForTier returns the number of Shamir shares released for a tier.
func SharesForTier(tier PricingTier) int {
	for _, t := range DefaultTiers() {
		if t.Tier == tier {
			return t.ShamirShares
		}
	}
	return 0
}

// QualityLevel for dataset samples.
type QualityLevel string

const (
	QualityBronze QualityLevel = "bronze"
	QualitySilver QualityLevel = "silver"
	QualityGold   QualityLevel = "gold"
)

// Dataset represents a curated dataset available in the marketplace.
type Dataset struct {
	ID            string            `json:"id"`
	Name          string            `json:"name"`
	Domain        string            `json:"domain"`
	Description   string            `json:"description"`
	SampleCount   int64             `json:"sample_count"`
	SizeBytes     int64             `json:"size_bytes"`
	QualityStats  map[string]int64  `json:"quality_stats"` // quality level → count
	ChunkCount    int               `json:"chunk_count"`
	ManifestHash  string            `json:"manifest_hash"`
	CuratorAddr   string            `json:"curator_addr"`
	CuratorShare  float64           `json:"curator_share"` // 0.0-1.0
	Pricing       []TierConfig      `json:"pricing"`
	Tags          []string          `json:"tags"`
	Version       string            `json:"version"`
	CreatedAt     time.Time         `json:"created_at"`
	UpdatedAt     time.Time         `json:"updated_at"`
	AvgRating     float64           `json:"avg_rating"`
	RatingCount   int64             `json:"rating_count"`
}

// PreviewSample is a low-quality sample shown in dataset previews.
type PreviewSample struct {
	Index    int    `json:"index"`
	Domain   string `json:"domain"`
	Category string `json:"category"`
	Snippet  string `json:"snippet"` // truncated content
}

// PurchaseStatus tracks the state of a dataset purchase.
type PurchaseStatus string

const (
	PurchasePending   PurchaseStatus = "pending"
	PurchaseConfirmed PurchaseStatus = "confirmed"
	PurchaseComplete  PurchaseStatus = "complete"
	PurchaseFailed    PurchaseStatus = "failed"
	PurchaseExpired   PurchaseStatus = "expired"
)

// Purchase represents a dataset purchase record.
type Purchase struct {
	ID            string         `json:"id"`
	DatasetID     string         `json:"dataset_id"`
	BuyerAddr     string         `json:"buyer_addr"`
	Tier          PricingTier    `json:"tier"`
	AmountUZRN    int64          `json:"amount_uzrn"`
	Status        PurchaseStatus `json:"status"`
	ShamirShares  int            `json:"shamir_shares"`
	AccessTickets []string       `json:"access_tickets,omitempty"`
	WatermarkSeed string         `json:"watermark_seed"`
	ExpiresAt     time.Time      `json:"expires_at"`
	CreatedAt     time.Time      `json:"created_at"`
}

// FeedbackRating is a buyer's rating of a purchased dataset.
type FeedbackRating struct {
	PurchaseID string    `json:"purchase_id"`
	DatasetID  string    `json:"dataset_id"`
	BuyerAddr  string    `json:"buyer_addr"`
	Stars      int       `json:"stars"` // 1-5
	Comment    string    `json:"comment,omitempty"`
	WeakAreas  []string  `json:"weak_areas,omitempty"` // domains/types that were weak
	CreatedAt  time.Time `json:"created_at"`
}
