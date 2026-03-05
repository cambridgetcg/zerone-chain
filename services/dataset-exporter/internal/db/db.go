package db

import (
	"database/sql"
	"fmt"
	"os"
	"time"

	_ "github.com/lib/pq"
)

// DB wraps a PostgreSQL connection pool for the staging database.
type DB struct {
	pool *sql.DB
}

// New creates a new DB connection from environment or explicit DSN.
func New(dsn string) (*DB, error) {
	if dsn == "" {
		dsn = os.Getenv("DATABASE_URL")
	}
	if dsn == "" {
		dsn = "postgres://localhost:5432/dataset_exporter?sslmode=disable"
	}

	pool, err := sql.Open("postgres", dsn)
	if err != nil {
		return nil, fmt.Errorf("open db: %w", err)
	}

	pool.SetMaxOpenConns(25)
	pool.SetMaxIdleConns(5)
	pool.SetConnMaxLifetime(5 * time.Minute)

	if err := pool.Ping(); err != nil {
		return nil, fmt.Errorf("ping db: %w", err)
	}

	return &DB{pool: pool}, nil
}

// Close closes the database connection pool.
func (d *DB) Close() error {
	return d.pool.Close()
}

// Pool returns the underlying sql.DB pool.
func (d *DB) Pool() *sql.DB {
	return d.pool
}

// Migrate runs the migration SQL file.
func (d *DB) Migrate(sqlPath string) error {
	data, err := os.ReadFile(sqlPath)
	if err != nil {
		return fmt.Errorf("read migration: %w", err)
	}
	_, err = d.pool.Exec(string(data))
	if err != nil {
		return fmt.Errorf("exec migration: %w", err)
	}
	return nil
}

// Sample represents a synced training data sample.
type Sample struct {
	ID               string
	Content          string
	SampleType       string
	Domain           string
	QualityTier      string
	QualityScore     int
	NoveltyScore     int
	SourceURI        string
	SourcePlatform   string
	OriginalAuthor   string
	Language         string
	Tags             []string
	ThreadID         string
	ParentSampleID   string
	ThreadPosition   int
	SyncedAt         time.Time
	ChainBlockHeight int64
}

// UpsertSample inserts or updates a sample in the staging database.
func (d *DB) UpsertSample(s *Sample) error {
	_, err := d.pool.Exec(`
		INSERT INTO samples (id, content, sample_type, domain, quality_tier,
			quality_score, novelty_score, source_uri, source_platform,
			original_author, language, tags, thread_id, parent_sample_id,
			thread_position, chain_block_height)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16)
		ON CONFLICT (id) DO UPDATE SET
			content = EXCLUDED.content,
			quality_tier = EXCLUDED.quality_tier,
			quality_score = EXCLUDED.quality_score,
			novelty_score = EXCLUDED.novelty_score,
			synced_at = NOW()`,
		s.ID, s.Content, s.SampleType, s.Domain, s.QualityTier,
		s.QualityScore, s.NoveltyScore, s.SourceURI, s.SourcePlatform,
		s.OriginalAuthor, s.Language, pqArray(s.Tags), s.ThreadID, s.ParentSampleID,
		s.ThreadPosition, s.ChainBlockHeight,
	)
	return err
}

// GetSample retrieves a sample by ID.
func (d *DB) GetSample(id string) (*Sample, error) {
	s := &Sample{}
	err := d.pool.QueryRow(`
		SELECT id, content, sample_type, domain, quality_tier,
			quality_score, novelty_score, source_uri, source_platform,
			original_author, language, thread_id, parent_sample_id,
			thread_position, synced_at, chain_block_height
		FROM samples WHERE id = $1`, id,
	).Scan(&s.ID, &s.Content, &s.SampleType, &s.Domain, &s.QualityTier,
		&s.QualityScore, &s.NoveltyScore, &s.SourceURI, &s.SourcePlatform,
		&s.OriginalAuthor, &s.Language, &s.ThreadID, &s.ParentSampleID,
		&s.ThreadPosition, &s.SyncedAt, &s.ChainBlockHeight,
	)
	if err != nil {
		return nil, err
	}
	return s, nil
}

// CountSamples returns the total number of synced samples.
func (d *DB) CountSamples() (int, error) {
	var count int
	err := d.pool.QueryRow("SELECT COUNT(*) FROM samples").Scan(&count)
	return count, err
}

// GetLastBlockHeight returns the last synced block height.
func (d *DB) GetLastBlockHeight() (int64, error) {
	var val string
	err := d.pool.QueryRow("SELECT value FROM sync_state WHERE key = 'last_block_height'").Scan(&val)
	if err != nil {
		return 0, err
	}
	var height int64
	fmt.Sscanf(val, "%d", &height)
	return height, nil
}

// SetLastBlockHeight updates the last synced block height.
func (d *DB) SetLastBlockHeight(height int64) error {
	_, err := d.pool.Exec(
		"UPDATE sync_state SET value = $1, updated_at = NOW() WHERE key = 'last_block_height'",
		fmt.Sprintf("%d", height),
	)
	return err
}

// pqArray converts a string slice to a PostgreSQL text array literal.
func pqArray(arr []string) string {
	if len(arr) == 0 {
		return "{}"
	}
	result := "{"
	for i, v := range arr {
		if i > 0 {
			result += ","
		}
		result += `"` + v + `"`
	}
	result += "}"
	return result
}
