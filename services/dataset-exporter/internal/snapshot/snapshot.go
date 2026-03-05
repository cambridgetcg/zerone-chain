package snapshot

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"strings"
	"time"
)

// Snapshot represents an immutable dataset version.
type Snapshot struct {
	ID             int
	Version        string
	CreatedAt      time.Time
	SampleCount    int
	DomainFilter   string
	QualityFilter  string
	TypeFilter     string
	LanguageFilter string
	Status         string
}

// SampleRow is a sample record for export.
type SampleRow struct {
	ID             string   `json:"id"`
	Content        string   `json:"content"`
	SampleType     string   `json:"sample_type"`
	Domain         string   `json:"domain"`
	QualityTier    string   `json:"quality_tier"`
	QualityScore   int      `json:"quality_score"`
	NoveltyScore   int      `json:"novelty_score"`
	SourceURI      string   `json:"source_uri,omitempty"`
	SourcePlatform string   `json:"source_platform,omitempty"`
	OriginalAuthor string   `json:"original_author,omitempty"`
	Language       string   `json:"language"`
	Tags           []string `json:"tags,omitempty"`
	ThreadID       string   `json:"thread_id,omitempty"`
	ParentSampleID string   `json:"parent_sample_id,omitempty"`
	ThreadPosition int      `json:"thread_position,omitempty"`
}

// Manager handles dataset snapshot creation and export.
type Manager struct {
	pool *sql.DB
}

// NewManager creates a snapshot manager from a database pool.
func NewManager(pool *sql.DB) *Manager {
	return &Manager{pool: pool}
}

// Create builds a new snapshot with optional filters.
func (m *Manager) Create(version, domain, quality, sampleType, language string) (*Snapshot, error) {
	// Build filter query for selecting samples
	where := []string{"1=1"}
	args := []interface{}{}
	argN := 1

	if domain != "" {
		where = append(where, fmt.Sprintf("domain = $%d", argN))
		args = append(args, domain)
		argN++
	}
	if quality != "" {
		where = append(where, fmt.Sprintf("quality_tier = $%d", argN))
		args = append(args, quality)
		argN++
	}
	if sampleType != "" {
		where = append(where, fmt.Sprintf("sample_type = $%d", argN))
		args = append(args, sampleType)
		argN++
	}
	if language != "" {
		where = append(where, fmt.Sprintf("language = $%d", argN))
		args = append(args, language)
		argN++
	}

	tx, err := m.pool.Begin()
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	// Create snapshot record
	var snapID int
	err = tx.QueryRow(`
		INSERT INTO dataset_snapshots (version, domain_filter, quality_filter, type_filter, language_filter, status)
		VALUES ($1, NULLIF($2,''), NULLIF($3,''), NULLIF($4,''), NULLIF($5,''), 'building')
		RETURNING id`,
		version, domain, quality, sampleType, language,
	).Scan(&snapID)
	if err != nil {
		return nil, fmt.Errorf("insert snapshot: %w", err)
	}

	// Link matching samples
	query := fmt.Sprintf(`
		INSERT INTO snapshot_samples (snapshot_id, sample_id)
		SELECT $1, id FROM samples WHERE %s`, strings.Join(where, " AND "))
	allArgs := append([]interface{}{snapID}, args...)
	result, err := tx.Exec(query, allArgs...)
	if err != nil {
		return nil, fmt.Errorf("link samples: %w", err)
	}

	count, _ := result.RowsAffected()

	// Update snapshot with count and mark ready
	_, err = tx.Exec(`UPDATE dataset_snapshots SET sample_count = $1, status = 'ready' WHERE id = $2`,
		count, snapID)
	if err != nil {
		return nil, fmt.Errorf("update snapshot: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit: %w", err)
	}

	log.Printf("snapshot: created %s with %d samples", version, count)

	return &Snapshot{
		ID:             snapID,
		Version:        version,
		SampleCount:    int(count),
		DomainFilter:   domain,
		QualityFilter:  quality,
		TypeFilter:     sampleType,
		LanguageFilter: language,
		Status:         "ready",
	}, nil
}

// List returns all snapshots ordered by creation time.
func (m *Manager) List() ([]*Snapshot, error) {
	rows, err := m.pool.Query(`
		SELECT id, version, created_at, sample_count,
			COALESCE(domain_filter,''), COALESCE(quality_filter,''),
			COALESCE(type_filter,''), COALESCE(language_filter,''), status
		FROM dataset_snapshots ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var snaps []*Snapshot
	for rows.Next() {
		s := &Snapshot{}
		if err := rows.Scan(&s.ID, &s.Version, &s.CreatedAt, &s.SampleCount,
			&s.DomainFilter, &s.QualityFilter, &s.TypeFilter,
			&s.LanguageFilter, &s.Status); err != nil {
			return nil, err
		}
		snaps = append(snaps, s)
	}
	return snaps, rows.Err()
}

// Export writes all samples in a snapshot to the writer in JSONL format.
func (m *Manager) Export(version string, w io.Writer) (int, error) {
	rows, err := m.pool.Query(`
		SELECT s.id, s.content, s.sample_type, s.domain, s.quality_tier,
			s.quality_score, s.novelty_score,
			COALESCE(s.source_uri,''), COALESCE(s.source_platform,''),
			COALESCE(s.original_author,''), COALESCE(s.language,'en'),
			COALESCE(s.thread_id,''), COALESCE(s.parent_sample_id,''),
			s.thread_position
		FROM samples s
		JOIN snapshot_samples ss ON ss.sample_id = s.id
		JOIN dataset_snapshots ds ON ds.id = ss.snapshot_id
		WHERE ds.version = $1
		ORDER BY s.domain, s.quality_tier DESC, s.id`, version)
	if err != nil {
		return 0, fmt.Errorf("query snapshot samples: %w", err)
	}
	defer rows.Close()

	enc := json.NewEncoder(w)
	count := 0
	for rows.Next() {
		var sr SampleRow
		if err := rows.Scan(&sr.ID, &sr.Content, &sr.SampleType, &sr.Domain,
			&sr.QualityTier, &sr.QualityScore, &sr.NoveltyScore,
			&sr.SourceURI, &sr.SourcePlatform, &sr.OriginalAuthor,
			&sr.Language, &sr.ThreadID, &sr.ParentSampleID,
			&sr.ThreadPosition); err != nil {
			return count, fmt.Errorf("scan row: %w", err)
		}
		if err := enc.Encode(sr); err != nil {
			return count, fmt.Errorf("encode row: %w", err)
		}
		count++
	}
	return count, rows.Err()
}
