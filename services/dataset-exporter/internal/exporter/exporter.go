package exporter

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/zerone-chain/zerone/services/dataset-exporter/internal/chain"
	"github.com/zerone-chain/zerone/services/dataset-exporter/internal/db"
	"github.com/zerone-chain/zerone/services/dataset-exporter/internal/metrics"
)

// Exporter syncs approved samples from the ZERONE chain to a staging database.
type Exporter struct {
	chain    *chain.Client
	db       *db.DB
	interval time.Duration
}

// New creates a new Exporter.
func New(chainClient *chain.Client, database *db.DB, pollInterval time.Duration) *Exporter {
	if pollInterval == 0 {
		pollInterval = 10 * time.Second
	}
	return &Exporter{
		chain:    chainClient,
		db:       database,
		interval: pollInterval,
	}
}

// RunOnce performs a single sync cycle: fetch new approved samples and upsert them.
func (e *Exporter) RunOnce(ctx context.Context) error {
	lastHeight, err := e.db.GetLastBlockHeight()
	if err != nil {
		// First run — start from 0
		lastHeight = 0
	}

	samples, newHeight, err := e.chain.FetchApprovedSamples(ctx, lastHeight)
	if err != nil {
		metrics.SyncErrors.Inc()
		return fmt.Errorf("fetch approved samples: %w", err)
	}

	synced := 0
	for _, cs := range samples {
		s := &db.Sample{
			ID:               cs.ID,
			Content:          cs.Content,
			SampleType:       cs.SampleType,
			Domain:           cs.Domain,
			QualityTier:      cs.QualityTier,
			QualityScore:     cs.QualityScore,
			NoveltyScore:     cs.NoveltyScore,
			SourceURI:        cs.SourceURI,
			SourcePlatform:   cs.SourcePlatform,
			OriginalAuthor:   cs.OriginalAuthor,
			Language:         cs.Language,
			Tags:             cs.Tags,
			ThreadID:         cs.ThreadID,
			ParentSampleID:   cs.ParentSampleID,
			ThreadPosition:   cs.ThreadPosition,
			ChainBlockHeight: cs.BlockHeight,
		}
		if err := e.db.UpsertSample(s); err != nil {
			metrics.SyncErrors.Inc()
			log.Printf("upsert sample %s: %v", cs.ID, err)
			continue
		}
		synced++
	}

	if synced > 0 {
		metrics.SamplesSynced.Add(float64(synced))
		log.Printf("synced %d samples up to height %d", synced, newHeight)
	}

	if newHeight > lastHeight {
		if err := e.db.SetLastBlockHeight(newHeight); err != nil {
			return fmt.Errorf("set last block height: %w", err)
		}
		metrics.LastSyncHeight.Set(float64(newHeight))
	}

	count, err := e.db.CountSamples()
	if err == nil {
		metrics.StagedSamplesTotal.Set(float64(count))
	}

	return nil
}

// Run starts the continuous sync loop, polling at the configured interval.
func (e *Exporter) Run(ctx context.Context) error {
	log.Printf("exporter: starting sync loop (interval=%s)", e.interval)

	// Initial sync
	if err := e.RunOnce(ctx); err != nil {
		log.Printf("initial sync error: %v", err)
	}

	ticker := time.NewTicker(e.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			log.Println("exporter: shutting down")
			return nil
		case <-ticker.C:
			if err := e.RunOnce(ctx); err != nil {
				log.Printf("sync error: %v", err)
			}
		}
	}
}
