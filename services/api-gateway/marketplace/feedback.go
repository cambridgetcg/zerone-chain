package marketplace

import (
	"fmt"
	"sync"
	"time"
)

// FeedbackStore manages dataset ratings and feedback.
type FeedbackStore struct {
	mu       sync.RWMutex
	ratings  map[string][]*FeedbackRating // datasetID → ratings
	catalog  *Catalog
}

// NewFeedbackStore creates a feedback store.
func NewFeedbackStore(catalog *Catalog) *FeedbackStore {
	return &FeedbackStore{
		ratings: make(map[string][]*FeedbackRating),
		catalog: catalog,
	}
}

// Submit records a buyer's rating for a purchased dataset.
func (fs *FeedbackStore) Submit(purchaseID, datasetID, buyerAddr string, stars int, comment string, weakAreas []string) (*FeedbackRating, error) {
	if stars < 1 || stars > 5 {
		return nil, fmt.Errorf("rating must be 1-5, got %d", stars)
	}

	// Verify dataset exists
	if _, ok := fs.catalog.Get(datasetID); !ok {
		return nil, fmt.Errorf("dataset %s not found", datasetID)
	}

	rating := &FeedbackRating{
		PurchaseID: purchaseID,
		DatasetID:  datasetID,
		BuyerAddr:  buyerAddr,
		Stars:      stars,
		Comment:    comment,
		WeakAreas:  weakAreas,
		CreatedAt:  time.Now(),
	}

	fs.mu.Lock()
	fs.ratings[datasetID] = append(fs.ratings[datasetID], rating)
	fs.mu.Unlock()

	// Update dataset average rating
	fs.updateDatasetRating(datasetID)

	return rating, nil
}

// GetRatings returns all ratings for a dataset.
func (fs *FeedbackStore) GetRatings(datasetID string) []*FeedbackRating {
	fs.mu.RLock()
	defer fs.mu.RUnlock()
	return fs.ratings[datasetID]
}

// WeakAreaSummary returns a frequency count of weak areas for a dataset.
func (fs *FeedbackStore) WeakAreaSummary(datasetID string) map[string]int {
	fs.mu.RLock()
	defer fs.mu.RUnlock()

	counts := make(map[string]int)
	for _, r := range fs.ratings[datasetID] {
		for _, area := range r.WeakAreas {
			counts[area]++
		}
	}
	return counts
}

func (fs *FeedbackStore) updateDatasetRating(datasetID string) {
	fs.mu.RLock()
	ratings := fs.ratings[datasetID]
	fs.mu.RUnlock()

	if len(ratings) == 0 {
		return
	}

	total := 0
	for _, r := range ratings {
		total += r.Stars
	}

	ds, ok := fs.catalog.Get(datasetID)
	if !ok {
		return
	}

	ds.AvgRating = float64(total) / float64(len(ratings))
	ds.RatingCount = int64(len(ratings))
}
