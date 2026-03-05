-- Dataset Exporter Schema

CREATE TABLE IF NOT EXISTS samples (
    id TEXT PRIMARY KEY,
    content TEXT NOT NULL,
    sample_type TEXT NOT NULL,
    domain TEXT NOT NULL,
    quality_tier TEXT NOT NULL,
    quality_score INTEGER DEFAULT 0,
    novelty_score INTEGER DEFAULT 0,
    source_uri TEXT DEFAULT '',
    source_platform TEXT DEFAULT '',
    original_author TEXT DEFAULT '',
    language TEXT DEFAULT 'en',
    tags TEXT[] DEFAULT '{}',
    thread_id TEXT DEFAULT '',
    parent_sample_id TEXT DEFAULT '',
    thread_position INTEGER DEFAULT 0,
    synced_at TIMESTAMPTZ DEFAULT NOW(),
    chain_block_height BIGINT DEFAULT 0
);

CREATE INDEX IF NOT EXISTS idx_samples_domain ON samples(domain);
CREATE INDEX IF NOT EXISTS idx_samples_quality_tier ON samples(quality_tier);
CREATE INDEX IF NOT EXISTS idx_samples_sample_type ON samples(sample_type);
CREATE INDEX IF NOT EXISTS idx_samples_language ON samples(language);
CREATE INDEX IF NOT EXISTS idx_samples_thread_id ON samples(thread_id);

CREATE TABLE IF NOT EXISTS dataset_snapshots (
    id SERIAL PRIMARY KEY,
    version TEXT UNIQUE NOT NULL,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    sample_count INTEGER DEFAULT 0,
    domain_filter TEXT,
    quality_filter TEXT,
    type_filter TEXT,
    language_filter TEXT,
    status TEXT DEFAULT 'building'
);

CREATE TABLE IF NOT EXISTS snapshot_samples (
    snapshot_id INTEGER REFERENCES dataset_snapshots(id) ON DELETE CASCADE,
    sample_id TEXT REFERENCES samples(id) ON DELETE CASCADE,
    PRIMARY KEY (snapshot_id, sample_id)
);

CREATE TABLE IF NOT EXISTS sync_state (
    key TEXT PRIMARY KEY,
    value TEXT NOT NULL,
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

-- Track last synced block height
INSERT INTO sync_state (key, value) VALUES ('last_block_height', '0')
ON CONFLICT (key) DO NOTHING;
