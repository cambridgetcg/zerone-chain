package marketplace

import (
	"fmt"
	"strings"
	"sync"
)

// Catalog manages available datasets (in-memory for now).
type Catalog struct {
	mu       sync.RWMutex
	datasets map[string]*Dataset // id → dataset
}

// NewCatalog creates a dataset catalog.
func NewCatalog() *Catalog {
	return &Catalog{datasets: make(map[string]*Dataset)}
}

// Add registers a dataset in the catalog.
func (c *Catalog) Add(ds *Dataset) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if ds.Pricing == nil {
		ds.Pricing = DefaultTiers()
	}
	c.datasets[ds.ID] = ds
}

// Get returns a dataset by ID.
func (c *Catalog) Get(id string) (*Dataset, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	ds, ok := c.datasets[id]
	return ds, ok
}

// ListFilter specifies filters for browsing datasets.
type ListFilter struct {
	Domain     string
	MinQuality string // "bronze", "silver", "gold"
	MinSamples int64
	MaxPrice   int64 // max price in uzrn (0 = no limit)
	Tags       []string
}

// List returns datasets matching the given filters.
func (c *Catalog) List(f ListFilter) []*Dataset {
	c.mu.RLock()
	defer c.mu.RUnlock()

	var result []*Dataset
	for _, ds := range c.datasets {
		if !matchesFilter(ds, f) {
			continue
		}
		result = append(result, ds)
	}
	return result
}

func matchesFilter(ds *Dataset, f ListFilter) bool {
	if f.Domain != "" && !strings.EqualFold(ds.Domain, f.Domain) {
		return false
	}
	if f.MinSamples > 0 && ds.SampleCount < f.MinSamples {
		return false
	}
	if f.MinQuality != "" {
		if !hasMinQuality(ds.QualityStats, f.MinQuality) {
			return false
		}
	}
	if f.MaxPrice > 0 {
		standardPrice := standardTierPrice(ds.Pricing)
		if standardPrice > f.MaxPrice {
			return false
		}
	}
	if len(f.Tags) > 0 {
		if !hasAnyTag(ds.Tags, f.Tags) {
			return false
		}
	}
	return true
}

func hasMinQuality(stats map[string]int64, minQuality string) bool {
	switch minQuality {
	case "gold":
		return stats["gold"] > 0
	case "silver":
		return stats["silver"] > 0 || stats["gold"] > 0
	case "bronze":
		return stats["bronze"] > 0 || stats["silver"] > 0 || stats["gold"] > 0
	}
	return true
}

func standardTierPrice(pricing []TierConfig) int64 {
	for _, p := range pricing {
		if p.Tier == TierStandard {
			return p.PriceUZRN
		}
	}
	return 0
}

func hasAnyTag(tags, wanted []string) bool {
	tagSet := make(map[string]bool, len(tags))
	for _, t := range tags {
		tagSet[strings.ToLower(t)] = true
	}
	for _, w := range wanted {
		if tagSet[strings.ToLower(w)] {
			return true
		}
	}
	return false
}

// Remove deletes a dataset from the catalog.
func (c *Catalog) Remove(id string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if _, ok := c.datasets[id]; !ok {
		return fmt.Errorf("dataset %s not found", id)
	}
	delete(c.datasets, id)
	return nil
}

// Count returns the number of datasets in the catalog.
func (c *Catalog) Count() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.datasets)
}
