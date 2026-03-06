# T2-1 — Dataset Exporter

## Goal

Build a Go service that connects to a ZERONE chain node, watches for approved training data Samples from the x/knowledge module, and exports them into a staging area ready for training format transformation.

## Context

The x/knowledge module stores Samples on-chain with:
- Content (the actual training data text)
- SampleType (discussion, debate, explanation, tutorial, Q&A, etc.)
- Domain (technical, culture, science, etc.)
- Quality tier (gold, silver, bronze — from QualityRound verdict)
- Metadata (submitter, source, consent, tags, language, thread context)

The exporter needs to:
1. Subscribe to chain events (new Sample approved / status changed to gold/silver/bronze)
2. Fetch full Sample data via gRPC query
3. Store in local staging database with metadata
4. Track dataset versions (snapshots)

## Working Directory

Create a new directory: `/Users/yournameisai/Desktop/zerone/services/dataset-exporter/`

## Deliverables

### 1. Chain Connection

- Connect to ZERONE chain via gRPC (use `x/knowledge` query client)
- Subscribe to EventSampleApproved events (or poll at regular intervals)
- Handle chain reorganizations gracefully

### 2. Sample Fetcher

- Fetch Sample by ID with full content and metadata
- Fetch Samples by domain, quality tier, type
- Batch fetching for initial sync (all approved samples)
- Incremental fetching for ongoing sync (new approvals since last sync)

### 3. Staging Database

- PostgreSQL schema for staging:
  ```sql
  CREATE TABLE samples (
    id TEXT PRIMARY KEY,
    content TEXT NOT NULL,
    sample_type TEXT NOT NULL,
    domain TEXT NOT NULL,
    quality_tier TEXT NOT NULL,  -- gold, silver, bronze
    quality_score INTEGER,
    novelty_score INTEGER,
    source_uri TEXT,
    source_platform TEXT,
    original_author TEXT,
    language TEXT,
    tags TEXT[],
    thread_id TEXT,
    parent_sample_id TEXT,
    thread_position INTEGER,
    synced_at TIMESTAMPTZ DEFAULT NOW(),
    chain_block_height BIGINT
  );
  
  CREATE TABLE dataset_snapshots (
    id SERIAL PRIMARY KEY,
    version TEXT UNIQUE NOT NULL,  -- e.g., "v1.0.0", "v1.1.0"
    created_at TIMESTAMPTZ DEFAULT NOW(),
    sample_count INTEGER,
    domain_filter TEXT,           -- NULL = all domains
    quality_filter TEXT,          -- NULL = all tiers
    type_filter TEXT,             -- NULL = all types
    status TEXT DEFAULT 'building' -- building, ready, training, archived
  );
  
  CREATE TABLE snapshot_samples (
    snapshot_id INTEGER REFERENCES dataset_snapshots(id),
    sample_id TEXT REFERENCES samples(id),
    PRIMARY KEY (snapshot_id, sample_id)
  );
  ```

### 4. Snapshot Manager

- Create dataset snapshots with optional filters (domain, quality tier, type, language)
- Snapshots are immutable once created — new data = new snapshot
- Track snapshot status: building → ready → training → archived
- CLI commands:
  ```
  exporter sync                    # Sync new samples from chain
  exporter snapshot create --domain technical --min-quality silver
  exporter snapshot list
  exporter snapshot export <version> --format jsonl
  ```

### 5. Health & Metrics

- Prometheus metrics: samples synced, sync lag, errors
- Health endpoint for monitoring
- Graceful shutdown

## Output

- Go module at `services/dataset-exporter/`
- Dockerfile for containerized deployment
- README with setup and usage instructions
